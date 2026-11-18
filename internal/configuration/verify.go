package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// VerifyResult represents the result of verifying configuration against a device
type VerifyResult struct {
	Match       bool                 `json:"match"`
	Differences []ConfigDifference   `json:"differences"`
	Imported    *DeviceConfiguration `json:"imported"`
	Desired     *DeviceConfiguration `json:"desired"`
	Duration    time.Duration        `json:"duration"`
}

// ApplyVerifyResult combines apply and verify results
type ApplyVerifyResult struct {
	ApplyResult   *ApplyResult  `json:"apply_result"`
	VerifyResult  *VerifyResult `json:"verify_result"`
	ConfigApplied bool          `json:"config_applied"`
	Duration      time.Duration `json:"duration"`
}

// ConfigVerifier handles verification of configurations on devices
type ConfigVerifier struct {
	applier    *ConfigApplier
	converter  ConfigConverter
	comparator *ConfigComparator
	logger     *logging.Logger

	// afterLogin runs once the device accepted a login change. It stores the new credentials
	// and returns the client for every later request, which is the only one the device still
	// answers.
	afterLogin func(ctx context.Context, client ShellyClient, change LoginChange) (ShellyClient, error)
}

// pendingReboot is the reboot an apply owes the device: a write the device only runs after one
// may have landed, and the reboot has not happened yet. It has to outlive a failed apply, because
// the retry reads the written value back as already stored and would see nothing to reboot for.
type pendingReboot struct {
	pending bool             // owed now, from an earlier apply or recorded by this one
	store   func(bool) error // persists the flag
}

// WithPendingReboot makes apply keep a reboot owed across attempts. pending is the stored flag;
// store persists a change to it. The applier records a new obligation before its first write and
// ApplyAndVerify clears it after a reboot succeeded.
func (v *ConfigVerifier) WithPendingReboot(pending bool, store func(bool) error) {
	v.applier.owed = &pendingReboot{pending: pending, store: store}
}

// WithLogin lets apply write the login. afterLogin must store the new credentials and return a
// client that uses them (none when the login is switched off). Without it a login stays unwritten.
func (v *ConfigVerifier) WithLogin(afterLogin func(ctx context.Context, client ShellyClient, change LoginChange) (ShellyClient, error)) {
	v.afterLogin = afterLogin
	v.applier.writeLogin = afterLogin != nil
}

// NewConfigVerifier creates a new configuration verifier
func NewConfigVerifier(converter ConfigConverter, logger *logging.Logger) *ConfigVerifier {
	if logger == nil {
		logger = logging.GetDefault()
	}
	comparator := NewConfigComparator()
	if _, gen2 := converter.(*Gen2Converter); gen2 {
		comparator = NewChannelIDConfigComparator()
	}
	return &ConfigVerifier{
		applier:    NewConfigApplier(converter, logger),
		converter:  converter,
		comparator: comparator,
		logger:     logger,
	}
}

// deviceReadError marks the failure of the device read inside VerifyConfig, apart from a device
// answer that does not convert. Only the read can be silent because the device moved networks.
type deviceReadError struct{ err error }

func (e *deviceReadError) Error() string { return e.err.Error() }
func (e *deviceReadError) Unwrap() error { return e.err }

// stationUnanswered reports whether a failure after a station network change means the device
// stopped answering, so the change is unconfirmed rather than failed. A device that answers with
// an error, or a document that does not convert, is on its network and keeps its error.
func stationUnanswered(applyResult *ApplyResult, err error, fromRead bool) bool {
	return applyResult.stationMoves && fromRead && !deviceAnswered(err)
}

// VerifyConfig imports config from device and compares to desired
func (v *ConfigVerifier) VerifyConfig(ctx context.Context, client ShellyClient, desired *DeviceConfiguration, deviceType string) (*VerifyResult, error) {
	startTime := time.Now()

	result := &VerifyResult{
		Match:       true,
		Differences: []ConfigDifference{},
		Desired:     withoutLoginPassword(desired),
	}

	deviceConfig, err := client.GetConfig(ctx)
	if err != nil {
		return nil, &deviceReadError{fmt.Errorf("failed to get device config: %w", err)}
	}

	imported, err := v.converter.FromAPIConfig(deviceConfig.Raw, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert device config: %w", err)
	}

	result.Imported = imported

	if desired.Auth != nil {
		if err = v.readGen2Login(ctx, client, imported); err != nil {
			return nil, err
		}
	}

	// The login password is write-only. Compare without it, so a missing read-back section never
	// puts the whole desired auth object, password included, into a difference.
	wanted := comparableChannelMaxPower(comparableDesired(withoutWiFiPasswords(withoutLoginPassword(desired))), deviceConfig.Raw, deviceType)
	compareResult := v.comparator.Compare(wanted, imported)
	result.Match = compareResult.Match
	result.Differences = compareResult.Differences
	result.Duration = time.Since(startTime)

	v.logger.WithFields(map[string]any{
		"device_ip":        client.GetIP(),
		"device_type":      deviceType,
		"match":            result.Match,
		"difference_count": len(result.Differences),
		"duration_ms":      result.Duration.Milliseconds(),
		"component":        "config_verifier",
	}).Info("Configuration verification completed")

	return result, nil
}

// withoutLoginPassword returns desired as the verify result reports it: the device login password
// is write-only, so the result never echoes it. The caller's configuration is never changed.
func withoutLoginPassword(desired *DeviceConfiguration) *DeviceConfiguration {
	if desired == nil || desired.Auth == nil || desired.Auth.Password == nil {
		return desired
	}
	shown := *desired
	auth := *desired.Auth
	auth.Password = nil
	shown.Auth = &auth
	return &shown
}

// comparableDesired returns the desired configuration in the shape the Gen1 read-back has.
// A Gen1 device stores the MQTT endpoint as one "host:port" string, and the converter splits
// it into server and port on read-back. A desired server in that combined form, with no
// separate port, is split the same way, so a matching device is not reported as drift.
// The caller's configuration is never changed.
func comparableDesired(desired *DeviceConfiguration) *DeviceConfiguration {
	if desired == nil || desired.MQTT == nil || desired.MQTT.Server == nil || desired.MQTT.Port != nil {
		return desired
	}
	parts := strings.Split(*desired.MQTT.Server, ":")
	if len(parts) != 2 {
		return desired
	}
	var port int
	if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil {
		return desired
	}
	normalized := *desired
	mqtt := *desired.MQTT
	mqtt.Server = StringPtr(parts[0])
	mqtt.Port = IntPtr(port)
	normalized.MQTT = &mqtt
	return &normalized
}

// unconfirmedStation reports a station network change that nothing could confirm: the device
// stopped answering, so it may be on another network. Neither success nor failure.
func (v *ConfigVerifier) unconfirmedStation(result *ApplyVerifyResult) *ApplyVerifyResult {
	result.ApplyResult.Unconfirmed = true
	result.ApplyResult.Warnings = append(result.ApplyResult.Warnings, wifiUnconfirmedWarning)
	return result
}

// ApplyAndVerify applies configuration and then verifies it was applied correctly
func (v *ConfigVerifier) ApplyAndVerify(ctx context.Context, client ShellyClient, config *DeviceConfiguration, deviceType string) (*ApplyVerifyResult, error) {
	startTime := time.Now()

	result := &ApplyVerifyResult{
		ConfigApplied: false,
	}

	applyResult, err := v.applier.ApplyConfig(ctx, client, config, deviceType)
	if err != nil {
		return nil, fmt.Errorf("apply failed: %w", err)
	}
	result.ApplyResult = applyResult

	if !applyResult.Success {
		result.Duration = time.Since(startTime)
		v.logger.WithFields(map[string]any{
			"device_ip":      client.GetIP(),
			"device_type":    deviceType,
			"apply_success":  false,
			"config_applied": false,
			"duration_ms":    result.Duration.Milliseconds(),
			"component":      "config_verifier",
		}).Info("Apply and verify completed (apply failed)")
		return result, nil
	}

	// From here on the device answers only the new credentials, for the reboot and the read-back.
	if applyResult.login != nil && v.afterLogin != nil {
		client, err = v.afterLogin(ctx, client, *applyResult.login)
		if err != nil {
			result.Duration = time.Since(startTime)
			return nil, err
		}
	}

	// A station network change that the device stopped answering after is neither verified nor
	// rebooted: it may be on another network. The reboot stays owed (a.owed) for the next apply.
	if applyResult.Unconfirmed {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	if applyResult.RequiresReboot {
		v.logger.WithFields(map[string]any{
			"device_ip":   client.GetIP(),
			"device_type": deviceType,
			"component":   "config_verifier",
		}).Info("Rebooting device before verification")

		if rebootErr := v.applier.RebootAndWait(ctx, client, 60*time.Second); rebootErr != nil {
			result.Duration = time.Since(startTime)
			if stationUnanswered(applyResult, rebootErr, true) {
				return v.unconfirmedStation(result), nil
			}
			return nil, fmt.Errorf("reboot failed: %w", rebootErr)
		}
		if owed := v.applier.owed; owed != nil {
			if storeErr := owed.store(false); storeErr != nil {
				// A flag left set costs one more reboot on the next apply, so this is not an error.
				applyResult.Warnings = append(applyResult.Warnings,
					"The device rebooted but the pending reboot could not be cleared; the next apply reboots it once more: "+storeErr.Error())
			}
		}
	}

	time.Sleep(500 * time.Millisecond)

	verifyResult, err := v.VerifyConfig(ctx, client, config, deviceType)
	if err != nil {
		result.Duration = time.Since(startTime)
		var readErr *deviceReadError
		if stationUnanswered(applyResult, err, errors.As(err, &readErr)) {
			return v.unconfirmedStation(result), nil
		}
		return nil, fmt.Errorf("verify failed: %w", err)
	}
	result.VerifyResult = verifyResult
	result.ConfigApplied = verifyResult.Match
	result.Duration = time.Since(startTime)

	v.logger.WithFields(map[string]any{
		"device_ip":      client.GetIP(),
		"device_type":    deviceType,
		"apply_success":  applyResult.Success,
		"verify_match":   verifyResult.Match,
		"config_applied": result.ConfigApplied,
		"duration_ms":    result.Duration.Milliseconds(),
		"component":      "config_verifier",
	}).Info("Apply and verify completed")

	return result, nil
}

// VerifyOnly verifies config without applying - just checks if device matches desired
func (v *ConfigVerifier) VerifyOnly(ctx context.Context, client ShellyClient, desired *DeviceConfiguration, deviceType string) (*VerifyResult, error) {
	return v.VerifyConfig(ctx, client, desired, deviceType)
}

// ImportConfig imports configuration from device and returns internal format
func (v *ConfigVerifier) ImportConfig(ctx context.Context, client ShellyClient, deviceType string) (*DeviceConfiguration, json.RawMessage, error) {
	deviceConfig, err := client.GetConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get device config: %w", err)
	}

	imported, err := v.converter.FromAPIConfig(deviceConfig.Raw, deviceType)
	if err != nil {
		return nil, deviceConfig.Raw, fmt.Errorf("failed to convert device config: %w", err)
	}

	v.logger.WithFields(map[string]any{
		"device_ip":   client.GetIP(),
		"device_type": deviceType,
		"component":   "config_verifier",
	}).Info("Configuration imported from device")

	return imported, deviceConfig.Raw, nil
}

// GetDiffReport generates a human-readable diff report
func (v *ConfigVerifier) GetDiffReport(result *VerifyResult) string {
	if result.Match {
		return "Configuration matches device - no differences found."
	}

	report := fmt.Sprintf("Found %d difference(s):\n", len(result.Differences))

	for _, diff := range result.Differences {
		report += fmt.Sprintf("\n[%s] %s\n", diff.Severity, diff.Path)
		report += fmt.Sprintf("  Expected: %v\n", diff.Expected)
		report += fmt.Sprintf("  Actual:   %v\n", diff.Actual)
		if diff.Description != "" {
			report += fmt.Sprintf("  Note: %s\n", diff.Description)
		}
	}

	return report
}

// GetApplyVerifyReport generates a human-readable report for apply+verify
func (v *ConfigVerifier) GetApplyVerifyReport(result *ApplyVerifyResult) string {
	report := "=== Configuration Apply & Verify Report ===\n\n"

	if result.ApplyResult != nil {
		report += "Apply Phase:\n"
		report += fmt.Sprintf("  Settings attempted: %d\n", result.ApplyResult.SettingsCount)
		report += fmt.Sprintf("  Settings applied:   %d\n", result.ApplyResult.AppliedCount)
		report += fmt.Sprintf("  Settings failed:    %d\n", result.ApplyResult.FailedCount)
		report += fmt.Sprintf("  Required reboot:    %v\n", result.ApplyResult.RequiresReboot)

		if len(result.ApplyResult.Failures) > 0 {
			report += "\n  Failures:\n"
			for _, f := range result.ApplyResult.Failures {
				report += fmt.Sprintf("    - %s: %s\n", f.Path, f.Error)
			}
		}
	}

	if result.VerifyResult != nil {
		report += "\nVerify Phase:\n"
		report += fmt.Sprintf("  Match: %v\n", result.VerifyResult.Match)
		report += fmt.Sprintf("  Differences: %d\n", len(result.VerifyResult.Differences))

		if len(result.VerifyResult.Differences) > 0 {
			report += "\n  Differences:\n"
			for _, d := range result.VerifyResult.Differences {
				report += fmt.Sprintf("    - [%s] %s: expected %v, got %v\n",
					d.Severity, d.Path, d.Expected, d.Actual)
			}
		}
	}

	report += fmt.Sprintf("\nFinal Status: config_applied=%v\n", result.ConfigApplied)
	report += fmt.Sprintf("Total Duration: %v\n", result.Duration)

	return report
}

// readGen2Login fills the login of a Gen2 read-back. The device config document holds no login:
// the device reports only whether it is on, in its device info (auth_en). The user is always
// admin and the password is never returned.
func (v *ConfigVerifier) readGen2Login(ctx context.Context, client ShellyClient, imported *DeviceConfiguration) error {
	if _, gen2 := v.converter.(*Gen2Converter); !gen2 {
		return nil
	}
	info, err := client.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to read the device login state: %w", err)
	}
	if info == nil {
		return errors.New("failed to read the device login state: the device reported nothing")
	}
	imported.Auth = &AuthConfiguration{Enable: &info.AuthEn}
	if info.AuthEn {
		user := gen2LoginUser
		imported.Auth.Username = &user
	}
	return nil
}

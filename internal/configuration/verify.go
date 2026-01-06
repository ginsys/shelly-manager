package configuration

import (
	"context"
	"encoding/json"
	"fmt"
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
}

// NewConfigVerifier creates a new configuration verifier
func NewConfigVerifier(converter ConfigConverter, logger *logging.Logger) *ConfigVerifier {
	if logger == nil {
		logger = logging.GetDefault()
	}
	return &ConfigVerifier{
		applier:    NewConfigApplier(converter, logger),
		converter:  converter,
		comparator: NewConfigComparator(),
		logger:     logger,
	}
}

// VerifyConfig imports config from device and compares to desired
func (v *ConfigVerifier) VerifyConfig(ctx context.Context, client ShellyClient, desired *DeviceConfiguration, deviceType string) (*VerifyResult, error) {
	startTime := time.Now()

	result := &VerifyResult{
		Match:       true,
		Differences: []ConfigDifference{},
		Desired:     desired,
	}

	deviceConfig, err := client.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}

	imported, err := v.converter.FromAPIConfig(deviceConfig.Raw, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert device config: %w", err)
	}

	result.Imported = imported

	compareResult := v.comparator.Compare(desired, imported)
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

	if applyResult.RequiresReboot {
		v.logger.WithFields(map[string]any{
			"device_ip":   client.GetIP(),
			"device_type": deviceType,
			"component":   "config_verifier",
		}).Info("Rebooting device before verification")

		if rebootErr := v.applier.RebootAndWait(ctx, client, 60*time.Second); rebootErr != nil {
			result.Duration = time.Since(startTime)
			return nil, fmt.Errorf("reboot failed: %w", rebootErr)
		}
	}

	time.Sleep(500 * time.Millisecond)

	verifyResult, err := v.VerifyConfig(ctx, client, config, deviceType)
	if err != nil {
		result.Duration = time.Since(startTime)
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

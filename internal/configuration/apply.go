package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// ApplyResult represents the result of applying configuration to a device
type ApplyResult struct {
	Success        bool           `json:"success"`         // All settings applied successfully
	SettingsCount  int            `json:"settings_count"`  // Number of settings sent
	AppliedCount   int            `json:"applied_count"`   // Number of settings accepted
	FailedCount    int            `json:"failed_count"`    // Number of settings rejected
	Failures       []ApplyFailure `json:"failures"`        // Details of failures
	RequiresReboot bool           `json:"requires_reboot"` // Device needs reboot to apply
	Warnings       []string       `json:"warnings"`        // Non-fatal issues
	Duration       time.Duration  `json:"duration"`        // Time taken to apply
}

// ApplyFailure represents a single failed setting
type ApplyFailure struct {
	Path  string `json:"path"`  // e.g., "mqtt.server"
	Value string `json:"value"` // What we tried to set
	Error string `json:"error"` // Error from device
}

// ShellyClient defines the interface for communicating with Shelly devices
// This allows for mocking in tests
type ShellyClient interface {
	SetConfig(ctx context.Context, config map[string]interface{}) error
	GetConfig(ctx context.Context) (*shelly.DeviceConfig, error)
	GetInfo(ctx context.Context) (*shelly.DeviceInfo, error)
	Reboot(ctx context.Context) error
	TestConnection(ctx context.Context) error
	GetGeneration() int
	GetIP() string
}

// ConfigApplier handles applying configurations to devices
type ConfigApplier struct {
	converter  ConfigConverter
	comparator *ConfigComparator
	logger     *logging.Logger
}

// NewConfigApplier creates a new configuration applier
func NewConfigApplier(converter ConfigConverter, logger *logging.Logger) *ConfigApplier {
	if logger == nil {
		logger = logging.GetDefault()
	}
	return &ConfigApplier{
		converter:  converter,
		comparator: NewConfigComparator(),
		logger:     logger,
	}
}

// ApplyConfig applies the given configuration to a device via the provided client
func (a *ConfigApplier) ApplyConfig(ctx context.Context, client ShellyClient, config *DeviceConfiguration, deviceType string) (*ApplyResult, error) {
	startTime := time.Now()

	result := &ApplyResult{
		Success:  true,
		Failures: []ApplyFailure{},
		Warnings: []string{},
		Duration: 0,
	}

	// Get current config for comparison (to detect reboot requirements)
	currentConfig, err := client.GetConfig(ctx)
	if err != nil {
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"device_ip": client.GetIP(),
			"component": "config_applier",
		}).Warn("Could not get current config for comparison")
	}

	// Convert internal config to API format
	apiJSON, err := a.converter.ToAPIConfig(config, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert config to API format: %w", err)
	}

	// Parse API JSON to map
	var apiConfig map[string]interface{}
	if err := json.Unmarshal(apiJSON, &apiConfig); err != nil {
		return nil, fmt.Errorf("failed to parse API config: %w", err)
	}

	// Count settings
	result.SettingsCount = a.countSettings(apiConfig)

	// Apply configuration sections. A rejected login is an error, not a failed setting: the caller
	// maps it to "the device rejected the stored credentials".
	if err := a.applyConfigSections(ctx, client, apiConfig, result); err != nil {
		return nil, fmt.Errorf("device rejected the login while applying: %w", err)
	}

	// Check if reboot is required
	if currentConfig != nil {
		result.RequiresReboot = a.detectRebootRequired(currentConfig.Raw, apiJSON, config)
		if result.RequiresReboot {
			result.Warnings = append(result.Warnings,
				"Device reboot required for some settings to take effect")
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = result.FailedCount == 0

	a.logger.WithFields(map[string]any{
		"device_ip":       client.GetIP(),
		"device_type":     deviceType,
		"settings_count":  result.SettingsCount,
		"applied_count":   result.AppliedCount,
		"failed_count":    result.FailedCount,
		"requires_reboot": result.RequiresReboot,
		"duration_ms":     result.Duration.Milliseconds(),
		"component":       "config_applier",
	}).Info("Configuration apply completed")

	return result, nil
}

// applyConfigSections applies each section of the configuration. A section the device refuses
// is recorded in result and the next section is tried, except when the device rejects the login:
// every later write would be rejected too, so that error stops the apply and is returned.
func (a *ConfigApplier) applyConfigSections(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	// For Gen1 devices, we can send settings to /settings endpoint
	// Different setting groups go to different endpoints
	sections := []struct {
		name  string
		apply func(context.Context, ShellyClient, map[string]interface{}, *ApplyResult) error
	}{
		{"main", a.applyMainSettings}, // name, timezone, location, etc.
		{"WiFi", a.applyWiFiSettings},
		{"MQTT", a.applyMQTTSettings},
		{"Cloud", a.applyCloudSettings},
		{"CoIoT", a.applyCoIoTSettings},
		{"Relay", a.applyRelaySettings},
	}
	for _, section := range sections {
		err := section.apply(ctx, client, apiConfig, result)
		if err == nil {
			continue
		}
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "config_applier",
		}).Warn("Failed to apply " + section.name + " settings")
		if shelly.IsAuthError(err) {
			return err
		}
	}
	return nil
}

func (a *ConfigApplier) applyMainSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	mainSettings := make(map[string]interface{})

	// Extract main settings
	mainKeys := []string{"name", "timezone", "lat", "lng", "eco_mode_enabled", "discoverable", "max_power"}
	for _, key := range mainKeys {
		if val, ok := apiConfig[key]; ok {
			mainSettings[key] = val
		}
	}

	// LED settings (inverted booleans)
	if val, ok := apiConfig["led_power_disable"]; ok {
		mainSettings["led_power_disable"] = val
	}
	if val, ok := apiConfig["led_status_disable"]; ok {
		mainSettings["led_status_disable"] = val
	}

	if len(mainSettings) == 0 {
		return nil
	}

	// A custom timezone or location only takes effect once the device's automatic detection
	// is off. The prerequisite is sent with them but is not a desired setting, so it is not
	// counted.
	write := make(map[string]interface{})
	for key, val := range mainSettings {
		write[key] = val
	}
	for _, key := range []string{"timezone", "lat", "lng"} {
		if _, ok := mainSettings[key]; ok {
			write["tzautodetect"] = false
			break
		}
	}

	if err := client.SetConfig(ctx, write); err != nil {
		for key := range mainSettings {
			result.Failures = append(result.Failures, ApplyFailure{
				Path:  key,
				Value: fmt.Sprintf("%v", mainSettings[key]),
				Error: err.Error(),
			})
			result.FailedCount++
		}
		return err
	}

	result.AppliedCount += len(mainSettings)
	return nil
}

func (a *ConfigApplier) applyWiFiSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	wifiSTA, ok := apiConfig["wifi_sta"].(map[string]interface{})
	if !ok || len(wifiSTA) == 0 {
		return nil
	}

	if err := client.SetConfig(ctx, map[string]interface{}{"wifi_sta": wifiSTA}); err != nil {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "wifi_sta",
			Value: fmt.Sprintf("%v", wifiSTA),
			Error: err.Error(),
		})
		result.FailedCount++
		return err
	}

	result.AppliedCount++

	// WiFi AP settings
	if wifiAP, ok := apiConfig["wifi_ap"].(map[string]interface{}); ok && len(wifiAP) > 0 {
		if err := client.SetConfig(ctx, map[string]interface{}{"wifi_ap": wifiAP}); err != nil {
			result.Failures = append(result.Failures, ApplyFailure{
				Path:  "wifi_ap",
				Value: fmt.Sprintf("%v", wifiAP),
				Error: err.Error(),
			})
			result.FailedCount++
		} else {
			result.AppliedCount++
		}
	}

	return nil
}

func (a *ConfigApplier) applyMQTTSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	mqtt, ok := apiConfig["mqtt"].(map[string]interface{})
	if !ok || len(mqtt) == 0 {
		return nil
	}

	form := gen1MQTTForm(mqtt)
	if len(form) == 0 {
		return nil
	}

	if err := client.SetConfig(ctx, form); err != nil {
		// Value names the keys only: the form can hold the broker password.
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "mqtt",
			Value: strings.Join(sortedKeys(form), ","),
			Error: err.Error(),
		})
		result.FailedCount += len(form)
		return err
	}

	result.AppliedCount += len(form)
	return nil
}

// gen1MQTTKeys maps the nested "mqtt" document that Gen1 returns from GET /settings to the
// flat form parameters that POST /settings accepts. The device reads nested maps on output but
// only understands flat mqtt_* parameters on input.
var gen1MQTTKeys = map[string]string{
	"enable":        "mqtt_enable",
	"server":        "mqtt_server",
	"user":          "mqtt_user",
	"pass":          "mqtt_pass",
	"id":            "mqtt_id",
	"clean_session": "mqtt_clean_session",
	"keep_alive":    "mqtt_keep_alive",
}

// gen1MQTTForm converts a nested mqtt document into Gen1 form parameters. Keys it does not know
// are dropped, never sent.
func gen1MQTTForm(mqtt map[string]interface{}) map[string]interface{} {
	form := make(map[string]interface{}, len(mqtt))
	for key, value := range mqtt {
		if formKey, ok := gen1MQTTKeys[key]; ok {
			form[formKey] = value
		}
	}
	return form
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CloudSettingsClient is the cloud surface of a Gen1 client: POST /settings/cloud. A client
// without it cannot write the cloud flag, and apply reports that as a failure instead of falling
// back to a nested "cloud" document on POST /settings, which Gen1 ignores.
type CloudSettingsClient interface {
	SetCloudConfig(ctx context.Context, enabled bool, server string) error
}

var errNoCloudClient = errors.New("client cannot write the cloud setting")

// applyCloudSettings writes the cloud flag. Gen1 reports no cloud server and POST /settings/cloud
// takes none, so only the flag is sent; a desired server is refused earlier (see
// unsupportedApplySettings). The final read-back of the whole configuration confirms the flag.
func (a *ConfigApplier) applyCloudSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	cloud, ok := apiConfig["cloud"].(map[string]interface{})
	if !ok {
		return nil
	}
	enabled, ok := cloud["enabled"].(bool)
	if !ok {
		return nil
	}

	failure := ApplyFailure{Path: "cloud.enabled", Value: fmt.Sprintf("%t", enabled)}
	cloudClient, ok := client.(CloudSettingsClient)
	if !ok {
		failure.Error = errNoCloudClient.Error()
		result.Failures = append(result.Failures, failure)
		result.FailedCount++
		return errNoCloudClient
	}
	if err := cloudClient.SetCloudConfig(ctx, enabled, ""); err != nil {
		failure.Error = err.Error()
		result.Failures = append(result.Failures, failure)
		result.FailedCount++
		return err
	}

	result.AppliedCount++
	return nil
}

func (a *ConfigApplier) applyCoIoTSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	coiot, ok := apiConfig["coiot"].(map[string]interface{})
	if !ok || len(coiot) == 0 {
		return nil
	}

	if err := client.SetConfig(ctx, map[string]interface{}{"coiot": coiot}); err != nil {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "coiot",
			Value: fmt.Sprintf("%v", coiot),
			Error: err.Error(),
		})
		result.FailedCount++
		return err
	}

	result.AppliedCount++
	return nil
}

// RelaySettingsClient is the per-channel relay surface of a Gen1 client: POST and GET on
// /settings/relay/<n>. A client without it cannot apply relay settings, and apply reports that
// as a failure instead of falling back to the nested "relays" document, which Gen1 ignores.
type RelaySettingsClient interface {
	SetRelaySettings(ctx context.Context, channel int, settings map[string]interface{}) error
	GetRelaySettings(ctx context.Context, channel int) (map[string]interface{}, error)
}

var errNoRelayClient = errors.New("client cannot write per-channel relay settings")

// gen1RelayKeys are the relay settings apply writes: the flat form parameters of
// POST /settings/relay/<n>. Anything else in a relay document is refused before the device is
// touched (see unsupportedApplySettings).
var gen1RelayKeys = []string{"name", "default_state", "auto_on", "auto_off"}

func (a *ConfigApplier) applyRelaySettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	relays := relayMaps(apiConfig["relays"])
	if len(relays) == 0 {
		return nil
	}

	relayClient, ok := client.(RelaySettingsClient)
	if !ok {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "relays",
			Error: errNoRelayClient.Error(),
		})
		result.FailedCount++
		return errNoRelayClient
	}

	var firstErr error
	for channel, relay := range relays {
		form := make(map[string]interface{}, len(gen1RelayKeys))
		for _, key := range gen1RelayKeys {
			if value, present := relay[key]; present {
				form[key] = value
			}
		}
		if len(form) == 0 {
			continue
		}

		if err := relayClient.SetRelaySettings(ctx, channel, form); err != nil {
			a.failRelay(result, channel, form, err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}

		// Read back this channel and compare every key just written. A device can answer 200
		// and keep the old value, so a mismatch is a failure of that path.
		live, err := relayClient.GetRelaySettings(ctx, channel)
		if err != nil {
			a.failRelay(result, channel, form, "read-back failed: "+err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}
		for _, key := range sortedKeys(form) {
			if relayValuesEqual(form[key], live[key]) {
				result.AppliedCount++
				continue
			}
			result.Failures = append(result.Failures, ApplyFailure{
				Path:  fmt.Sprintf("relays[%d].%s", channel, key),
				Value: fmt.Sprintf("%v", form[key]),
				Error: fmt.Sprintf("device reports %v after the write", live[key]),
			})
			result.FailedCount++
		}
	}

	return firstErr
}

// failRelay records every key of one channel's form as failed.
func (a *ConfigApplier) failRelay(result *ApplyResult, channel int, form map[string]interface{}, message string) {
	for _, key := range sortedKeys(form) {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  fmt.Sprintf("relays[%d].%s", channel, key),
			Value: fmt.Sprintf("%v", form[key]),
			Error: message,
		})
		result.FailedCount++
	}
}

func firstError(current, next error) error {
	if current != nil {
		return current
	}
	return next
}

// relayMaps returns the relay documents of the converted configuration, in channel order.
func relayMaps(raw interface{}) []map[string]interface{} {
	switch relays := raw.(type) {
	case []map[string]interface{}:
		return relays
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(relays))
		for _, item := range relays {
			relay, _ := item.(map[string]interface{})
			out = append(out, relay)
		}
		return out
	}
	return nil
}

// relayValuesEqual compares a written form value with the JSON the device returns. JSON numbers
// arrive as float64, while the converter holds ints. A string must come back as a string: a
// null or a number that prints the same as the written text is a mismatch.
func relayValuesEqual(written, live interface{}) bool {
	if wf, ok := toFloat(written); ok {
		lf, lok := toFloat(live)
		return lok && wf == lf
	}
	if ws, ok := written.(string); ok {
		ls, lok := live.(string)
		return lok && ws == ls
	}
	return fmt.Sprintf("%v", written) == fmt.Sprintf("%v", live)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// countSettings counts the number of settings being applied
func (a *ConfigApplier) countSettings(config map[string]interface{}) int {
	count := 0
	for key, val := range config {
		if key == "relays" {
			// One setting per relay key apply writes, as applyRelaySettings counts them.
			for _, relay := range relayMaps(val) {
				for _, relayKey := range gen1RelayKeys {
					if _, ok := relay[relayKey]; ok {
						count++
					}
				}
			}
			continue
		}
		switch v := val.(type) {
		case map[string]interface{}:
			count += len(v)
		case []interface{}:
			count += len(v)
		default:
			count++
		}
		_ = key
	}
	return count
}

// detectRebootRequired determines if a reboot is needed
func (a *ConfigApplier) detectRebootRequired(oldConfigJSON json.RawMessage, newConfigJSON json.RawMessage, newConfig *DeviceConfiguration) bool {
	// Settings that require reboot
	rebootSettings := []string{
		"wifi_sta.ssid",
		"wifi_sta.key",
		"wifi_sta.enabled",
		"login.enabled",
		"login.username",
		"login.password",
	}

	// Check if any reboot-required settings changed
	var oldConfig, newAPIConfig map[string]interface{}
	if err := json.Unmarshal(oldConfigJSON, &oldConfig); err != nil {
		return false
	}
	if err := json.Unmarshal(newConfigJSON, &newAPIConfig); err != nil {
		return false
	}

	for _, path := range rebootSettings {
		parts := strings.Split(path, ".")
		if len(parts) != 2 {
			continue
		}

		section, field := parts[0], parts[1]

		oldSection, _ := oldConfig[section].(map[string]interface{})
		newSection, _ := newAPIConfig[section].(map[string]interface{})

		if oldSection == nil || newSection == nil {
			continue
		}

		oldVal := oldSection[field]
		newVal := newSection[field]

		// If new value is set and different from old value
		if newVal != nil && fmt.Sprintf("%v", oldVal) != fmt.Sprintf("%v", newVal) {
			return true
		}
	}

	// Check auth enable change
	if newConfig != nil && newConfig.Auth != nil && newConfig.Auth.Enable != nil {
		return true
	}

	return false
}

// RebootAndWait reboots the device and waits for it to come back online
func (a *ConfigApplier) RebootAndWait(ctx context.Context, client ShellyClient, timeout time.Duration) error {
	a.logger.WithFields(map[string]any{
		"device_ip": client.GetIP(),
		"timeout":   timeout.String(),
		"component": "config_applier",
	}).Info("Rebooting device")

	// Send reboot command
	if err := client.Reboot(ctx); err != nil {
		return fmt.Errorf("failed to send reboot command: %w", err)
	}

	// Wait a bit for device to start rebooting
	time.Sleep(2 * time.Second)

	// Poll for device to come back online
	deadline := time.Now().Add(timeout)
	pollInterval := 3 * time.Second

	for time.Now().Before(deadline) {
		if err := client.TestConnection(ctx); err == nil {
			a.logger.WithFields(map[string]any{
				"device_ip": client.GetIP(),
				"component": "config_applier",
			}).Info("Device back online after reboot")
			return nil
		}
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("device did not come back online within %s", timeout)
}

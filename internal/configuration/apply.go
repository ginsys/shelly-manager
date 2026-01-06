package configuration

import (
	"context"
	"encoding/json"
	"fmt"
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

	// Apply configuration sections
	a.applyConfigSections(ctx, client, apiConfig, result)

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

// applyConfigSections applies each section of the configuration
func (a *ConfigApplier) applyConfigSections(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) {
	// For Gen1 devices, we can send settings to /settings endpoint
	// Different setting groups go to different endpoints

	// Main settings (name, timezone, location, etc.)
	if err := a.applyMainSettings(ctx, client, apiConfig, result); err != nil {
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "config_applier",
		}).Warn("Failed to apply main settings")
	}

	// WiFi settings
	if err := a.applyWiFiSettings(ctx, client, apiConfig, result); err != nil {
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "config_applier",
		}).Warn("Failed to apply WiFi settings")
	}

	// MQTT settings
	if err := a.applyMQTTSettings(ctx, client, apiConfig, result); err != nil {
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "config_applier",
		}).Warn("Failed to apply MQTT settings")
	}

	// Cloud settings
	if err := a.applyCloudSettings(ctx, client, apiConfig, result); err != nil {
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "config_applier",
		}).Warn("Failed to apply Cloud settings")
	}

	// CoIoT settings
	if err := a.applyCoIoTSettings(ctx, client, apiConfig, result); err != nil {
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "config_applier",
		}).Warn("Failed to apply CoIoT settings")
	}

	// Relay settings
	if err := a.applyRelaySettings(ctx, client, apiConfig, result); err != nil {
		a.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "config_applier",
		}).Warn("Failed to apply Relay settings")
	}
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

	if err := client.SetConfig(ctx, mainSettings); err != nil {
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

	if err := client.SetConfig(ctx, map[string]interface{}{"mqtt": mqtt}); err != nil {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "mqtt",
			Value: fmt.Sprintf("%v", mqtt),
			Error: err.Error(),
		})
		result.FailedCount++
		return err
	}

	result.AppliedCount++
	return nil
}

func (a *ConfigApplier) applyCloudSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	cloud, ok := apiConfig["cloud"].(map[string]interface{})
	if !ok || len(cloud) == 0 {
		return nil
	}

	if err := client.SetConfig(ctx, map[string]interface{}{"cloud": cloud}); err != nil {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "cloud",
			Value: fmt.Sprintf("%v", cloud),
			Error: err.Error(),
		})
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

func (a *ConfigApplier) applyRelaySettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	relaysRaw, ok := apiConfig["relays"]
	if !ok || relaysRaw == nil {
		return nil
	}

	// Check if relays is a slice
	relays, ok := relaysRaw.([]interface{})
	if !ok || len(relays) == 0 {
		// Try typed slice format
		if typedRelays, ok := relaysRaw.([]map[string]interface{}); ok && len(typedRelays) > 0 {
			// Convert to []interface{}
			relays = make([]interface{}, len(typedRelays))
			for i, r := range typedRelays {
				relays[i] = r
			}
		} else {
			return nil
		}
	}

	if err := client.SetConfig(ctx, map[string]interface{}{"relays": relays}); err != nil {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "relays",
			Value: fmt.Sprintf("%v", relays),
			Error: err.Error(),
		})
		result.FailedCount++
		return err
	}

	result.AppliedCount++
	return nil
}

// countSettings counts the number of settings being applied
func (a *ConfigApplier) countSettings(config map[string]interface{}) int {
	count := 0
	for key, val := range config {
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

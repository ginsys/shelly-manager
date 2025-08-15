package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
	"gorm.io/gorm"
)

// Service handles configuration management operations
type Service struct {
	db     *gorm.DB
	logger *logging.Logger
}

// NewService creates a new configuration service
func NewService(db *gorm.DB, logger *logging.Logger) *Service {
	// Auto-migrate configuration tables
	db.AutoMigrate(
		&ConfigTemplate{},
		&DeviceConfig{},
		&ConfigHistory{},
	)
	
	return &Service{
		db:     db,
		logger: logger,
	}
}

// ImportFromDevice imports configuration from a physical device
func (s *Service) ImportFromDevice(deviceID uint, client shelly.Client) (*DeviceConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "configuration",
	}).Info("Starting configuration import from device")
	
	// Get device info to determine generation and basic info
	info, err := client.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}
	
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"generation": info.Generation,
		"model": info.Model,
		"component": "configuration",
	}).Debug("Device info retrieved, importing configuration")
	
	// Get comprehensive device configuration
	deviceConfig, err := client.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device configuration: %w", err)
	}
	
	// Use the raw configuration data from the device
	configData := deviceConfig.Raw
	
	// Enhance with additional device metadata
	enhancedConfig := map[string]interface{}{}
	if err := json.Unmarshal(configData, &enhancedConfig); err != nil {
		// If unmarshaling fails, create a basic structure
		enhancedConfig = make(map[string]interface{})
	}
	
	// Determine firmware version (Gen1 uses FW, Gen2+ uses Version)
	firmware := info.Version
	if firmware == "" && info.FW != "" {
		firmware = info.FW
	}
	
	// Determine auth status (Gen1 uses Auth, Gen2+ uses AuthEn)
	authEnabled := info.AuthEn
	if !authEnabled && info.Auth {
		authEnabled = info.Auth
	}
	
	// Add metadata for tracking and identification
	enhancedConfig["_metadata"] = map[string]interface{}{
		"device_id":      deviceID,
		"generation":     info.Generation,
		"model":          info.Model,
		"firmware":       firmware,
		"mac":            info.MAC,
		"imported_at":    time.Now().Format(time.RFC3339),
		"import_source":  "device",
	}
	
	// Add device info if not present in config
	if _, hasDeviceInfo := enhancedConfig["device_info"]; !hasDeviceInfo {
		enhancedConfig["device_info"] = map[string]interface{}{
			"id":         info.ID,
			"model":      info.Model,
			"generation": info.Generation,
			"firmware":   firmware,
			"mac":        info.MAC,
			"auth_en":    authEnabled,
		}
	}
	
	// Re-marshal the enhanced configuration
	configData, err = json.Marshal(enhancedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal enhanced config: %w", err)
	}
	
	// Validate and sanitize the configuration
	configData, err = s.validateAndSanitizeConfig(configData, deviceID)
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	
	// Check if config already exists
	var existingConfig DeviceConfig
	err = s.db.Where("device_id = ?", deviceID).First(&existingConfig).Error
	
	now := time.Now()
	
	if err == gorm.ErrRecordNotFound {
		// Create new config
		newConfig := DeviceConfig{
			DeviceID:   deviceID,
			Config:     configData,
			LastSynced: &now,
			SyncStatus: "synced",
		}
		
		if err := s.db.Create(&newConfig).Error; err != nil {
			return nil, fmt.Errorf("failed to create device config: %w", err)
		}
		
		// Create history entry
		s.createHistory(deviceID, newConfig.ID, "import", nil, configData, "system")
		
		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"config_id": newConfig.ID,
			"config_size": len(configData),
			"component": "configuration",
		}).Info("Successfully imported configuration from device")
		
		return &newConfig, nil
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to query existing config: %w", err)
	}
	
	// Check if configuration has actually changed
	if string(existingConfig.Config) == string(configData) {
		// No changes, just update sync timestamp
		existingConfig.LastSynced = &now
		existingConfig.SyncStatus = "synced"
		
		if err := s.db.Save(&existingConfig).Error; err != nil {
			return nil, fmt.Errorf("failed to update sync timestamp: %w", err)
		}
		
		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"config_id": existingConfig.ID,
			"component": "configuration",
		}).Debug("Configuration unchanged, updated sync timestamp only")
		
		return &existingConfig, nil
	}
	
	// Configuration has changed, update it
	oldConfig := existingConfig.Config
	existingConfig.Config = configData
	existingConfig.LastSynced = &now
	existingConfig.SyncStatus = "synced"
	
	if err := s.db.Save(&existingConfig).Error; err != nil {
		return nil, fmt.Errorf("failed to update device config: %w", err)
	}
	
	// Create history entry
	s.createHistory(deviceID, existingConfig.ID, "import", oldConfig, configData, "system")
	
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"config_id": existingConfig.ID,
		"config_size": len(configData),
		"changes_detected": true,
		"component": "configuration",
	}).Info("Successfully updated configuration from device")
	
	return &existingConfig, nil
}

// GetImportStatus gets the import status for a device
func (s *Service) GetImportStatus(deviceID uint) (*ImportStatus, error) {
	var config DeviceConfig
	err := s.db.Where("device_id = ?", deviceID).First(&config).Error
	
	if err == gorm.ErrRecordNotFound {
		return &ImportStatus{
			DeviceID: deviceID,
			Status:   "not_imported",
			Message:  "No configuration has been imported for this device",
		}, nil
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to check import status: %w", err)
	}
	
	status := &ImportStatus{
		DeviceID:   deviceID,
		ConfigID:   config.ID,
		Status:     config.SyncStatus,
		LastSynced: config.LastSynced,
		UpdatedAt:  config.UpdatedAt,
	}
	
	// Determine status message
	switch config.SyncStatus {
	case "synced":
		status.Message = "Configuration successfully imported and synced"
	case "pending":
		status.Message = "Configuration imported but pending sync to device"
	case "drift":
		status.Message = "Configuration drift detected - device config differs from stored"
	case "error":
		status.Message = "Error occurred during last import/sync operation"
	default:
		status.Message = fmt.Sprintf("Unknown status: %s", config.SyncStatus)
	}
	
	return status, nil
}

// BulkImportFromDevices imports configuration from multiple devices
func (s *Service) BulkImportFromDevices(deviceIDs []uint, clientGetter func(uint) (shelly.Client, error)) (*BulkImportResult, error) {
	result := &BulkImportResult{
		Total:   len(deviceIDs),
		Success: 0,
		Failed:  0,
		Results: make([]ImportResult, 0, len(deviceIDs)),
	}
	
	for _, deviceID := range deviceIDs {
		importResult := ImportResult{
			DeviceID: deviceID,
		}
		
		// Get client for this device
		client, err := clientGetter(deviceID)
		if err != nil {
			importResult.Status = "error"
			importResult.Error = fmt.Sprintf("Failed to create client: %v", err)
			result.Failed++
		} else {
			// Import configuration
			config, err := s.ImportFromDevice(deviceID, client)
			if err != nil {
				importResult.Status = "error"
				importResult.Error = err.Error()
				result.Failed++
			} else {
				importResult.Status = "success"
				importResult.ConfigID = config.ID
				result.Success++
			}
		}
		
		result.Results = append(result.Results, importResult)
	}
	
	s.logger.WithFields(map[string]any{
		"total_devices": result.Total,
		"successful":    result.Success,
		"failed":        result.Failed,
		"component":     "configuration",
	}).Info("Bulk configuration import completed")
	
	return result, nil
}

// ExportToDevice exports configuration to a physical device
func (s *Service) ExportToDevice(deviceID uint, client shelly.Client) error {
	// Get configuration from database
	var config DeviceConfig
	if err := s.db.Where("device_id = ?", deviceID).First(&config).Error; err != nil {
		return fmt.Errorf("configuration not found: %w", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Get device info to determine generation
	info, err := client.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device info: %w", err)
	}
	
	// Apply configuration based on generation
	switch info.Generation {
	case 1:
		// Parse Gen1 config
		var gen1Config Gen1Config
		if err := json.Unmarshal(config.Config, &gen1Config); err != nil {
			return fmt.Errorf("failed to parse Gen1 config: %w", err)
		}
		
		// Apply settings (would need to implement SetSettings in Gen1 client)
		// For now, we'll just update the sync status
		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"component": "configuration",
		}).Warn("Gen1 configuration export not fully implemented")
		
	case 2, 3:
		// Parse Gen2 config
		var gen2Config Gen2Config
		if err := json.Unmarshal(config.Config, &gen2Config); err != nil {
			return fmt.Errorf("failed to parse Gen2+ config: %w", err)
		}
		
		// Apply settings (would need to implement SetConfig in Gen2 client)
		// For now, we'll just update the sync status
		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"component": "configuration",
		}).Warn("Gen2+ configuration export not fully implemented")
		
	default:
		return fmt.Errorf("unsupported device generation: %d", info.Generation)
	}
	
	// Update sync status
	now := time.Now()
	config.LastSynced = &now
	config.SyncStatus = "synced"
	
	if err := s.db.Save(&config).Error; err != nil {
		return fmt.Errorf("failed to update sync status: %w", err)
	}
	
	// Create history entry
	s.createHistory(deviceID, config.ID, "export", nil, config.Config, "system")
	
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "configuration",
	}).Info("Exported configuration to device")
	
	return nil
}

// DetectDrift checks for configuration differences between database and device
func (s *Service) DetectDrift(deviceID uint, client shelly.Client) (*ConfigDrift, error) {
	// Get stored configuration
	var storedConfig DeviceConfig
	if err := s.db.Where("device_id = ?", deviceID).First(&storedConfig).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no stored configuration found for device %d", deviceID)
		}
		return nil, fmt.Errorf("failed to get stored config: %w", err)
	}
	
	// Import current configuration from device
	currentConfig, err := s.ImportFromDevice(deviceID, client)
	if err != nil {
		return nil, fmt.Errorf("failed to import current config: %w", err)
	}
	
	// Compare configurations
	differences := s.compareConfigurations(storedConfig.Config, currentConfig.Config)
	
	if len(differences) == 0 {
		// No drift detected
		storedConfig.SyncStatus = "synced"
		s.db.Save(&storedConfig)
		return nil, nil
	}
	
	// Drift detected
	storedConfig.SyncStatus = "drift"
	s.db.Save(&storedConfig)
	
	// Get device name
	var device database.Device
	s.db.First(&device, deviceID)
	
	drift := &ConfigDrift{
		DeviceID:       deviceID,
		DeviceName:     device.Name,
		LastSynced:     storedConfig.LastSynced,
		DriftDetected:  time.Now(),
		Differences:    differences,
		RequiresAction: true,
	}
	
	s.logger.WithFields(map[string]any{
		"device_id":   deviceID,
		"differences": len(differences),
		"component":   "configuration",
	}).Warn("Configuration drift detected")
	
	return drift, nil
}

// ApplyTemplate applies a configuration template to a device
func (s *Service) ApplyTemplate(deviceID uint, templateID uint, variables map[string]interface{}) error {
	// Get template
	var template ConfigTemplate
	if err := s.db.First(&template, templateID).Error; err != nil {
		return fmt.Errorf("template not found: %w", err)
	}
	
	// Get device to check compatibility
	var device database.Device
	if err := s.db.First(&device, deviceID).Error; err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	
	// Check device type compatibility
	if template.DeviceType != "all" && template.DeviceType != device.Type {
		return fmt.Errorf("template not compatible with device type %s", device.Type)
	}
	
	// Apply variable substitution if needed
	configData := template.Config
	if len(variables) > 0 {
		configData = s.substituteVariables(configData, variables)
	}
	
	// Check if config exists
	var config DeviceConfig
	err := s.db.Where("device_id = ?", deviceID).First(&config).Error
	
	if err == gorm.ErrRecordNotFound {
		// Create new config
		config = DeviceConfig{
			DeviceID:   deviceID,
			TemplateID: &templateID,
			Config:     configData,
			SyncStatus: "pending",
		}
		
		if err := s.db.Create(&config).Error; err != nil {
			return fmt.Errorf("failed to create device config: %w", err)
		}
		
		// Create history entry
		s.createHistory(deviceID, config.ID, "template", nil, configData, "template")
		
	} else if err != nil {
		return fmt.Errorf("failed to query config: %w", err)
	} else {
		// Update existing config
		oldConfig := config.Config
		config.TemplateID = &templateID
		config.Config = configData
		config.SyncStatus = "pending"
		
		if err := s.db.Save(&config).Error; err != nil {
			return fmt.Errorf("failed to update device config: %w", err)
		}
		
		// Create history entry
		s.createHistory(deviceID, config.ID, "template", oldConfig, configData, "template")
	}
	
	s.logger.WithFields(map[string]any{
		"device_id":   deviceID,
		"template_id": templateID,
		"component":   "configuration",
	}).Info("Applied template to device")
	
	return nil
}

// GetDeviceConfig gets the configuration for a device
func (s *Service) GetDeviceConfig(deviceID uint) (*DeviceConfig, error) {
	var config DeviceConfig
	err := s.db.Where("device_id = ?", deviceID).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateDeviceConfig updates the configuration for a device
func (s *Service) UpdateDeviceConfig(deviceID uint, configUpdate map[string]interface{}) error {
	// Get existing config
	var config DeviceConfig
	err := s.db.Where("device_id = ?", deviceID).First(&config).Error
	if err != nil {
		return fmt.Errorf("device config not found: %w", err)
	}

	// Parse existing config
	var existingConfigMap map[string]interface{}
	if err := json.Unmarshal(config.Config, &existingConfigMap); err != nil {
		return fmt.Errorf("failed to parse existing config: %w", err)
	}

	// Merge updates into existing config
	for key, value := range configUpdate {
		existingConfigMap[key] = value
	}

	// Marshal back to JSON
	updatedConfig, err := json.Marshal(existingConfigMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	// Create history entry
	oldConfig := config.Config
	s.createHistory(deviceID, config.ID, "manual", oldConfig, updatedConfig, "user")

	// Update the config
	config.Config = updatedConfig
	config.SyncStatus = "pending"
	now := time.Now()
	config.UpdatedAt = now

	return s.db.Save(&config).Error
}

// UpdateCapabilityConfig updates a specific capability configuration
func (s *Service) UpdateCapabilityConfig(deviceID uint, capability string, capabilityConfig interface{}) error {
	// Get existing config
	var config DeviceConfig
	err := s.db.Where("device_id = ?", deviceID).First(&config).Error
	if err != nil {
		return fmt.Errorf("device config not found: %w", err)
	}

	// Parse existing config
	var configMap map[string]interface{}
	if err := json.Unmarshal(config.Config, &configMap); err != nil {
		return fmt.Errorf("failed to parse existing config: %w", err)
	}

	// Update the specific capability
	configMap[capability] = capabilityConfig

	// Marshal back to JSON
	updatedConfig, err := json.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	// Create history entry
	oldConfig := config.Config
	s.createHistory(deviceID, config.ID, "manual", oldConfig, updatedConfig, "user")

	// Update the config
	config.Config = updatedConfig
	config.SyncStatus = "pending"
	now := time.Now()
	config.UpdatedAt = now

	// Log the update
	s.logger.WithFields(map[string]any{
		"device_id":  deviceID,
		"capability": capability,
		"component":  "configuration",
	}).Info("Updated device capability configuration")

	return s.db.Save(&config).Error
}

// GetTemplates gets all configuration templates
func (s *Service) GetTemplates() ([]ConfigTemplate, error) {
	var templates []ConfigTemplate
	err := s.db.Find(&templates).Error
	return templates, err
}

// CreateTemplate creates a new configuration template
func (s *Service) CreateTemplate(template *ConfigTemplate) error {
	return s.db.Create(template).Error
}

// UpdateTemplate updates an existing template
func (s *Service) UpdateTemplate(template *ConfigTemplate) error {
	return s.db.Save(template).Error
}

// DeleteTemplate deletes a template
func (s *Service) DeleteTemplate(templateID uint) error {
	return s.db.Delete(&ConfigTemplate{}, templateID).Error
}

// GetConfigHistory gets the configuration history for a device
func (s *Service) GetConfigHistory(deviceID uint, limit int) ([]ConfigHistory, error) {
	var history []ConfigHistory
	query := s.db.Where("device_id = ?", deviceID).Order("created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	err := query.Find(&history).Error
	return history, err
}

// compareConfigurations compares two JSON configurations and returns differences
func (s *Service) compareConfigurations(stored, current json.RawMessage) []ConfigDifference {
	var differences []ConfigDifference
	
	var storedMap, currentMap map[string]interface{}
	json.Unmarshal(stored, &storedMap)
	json.Unmarshal(current, &currentMap)
	
	// Compare maps recursively
	s.compareMaps("", storedMap, currentMap, &differences)
	
	return differences
}

// compareMaps recursively compares two maps
func (s *Service) compareMaps(path string, expected, actual map[string]interface{}, differences *[]ConfigDifference) {
	// Check for removed keys
	for key, expectedValue := range expected {
		currentPath := path
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += key
		
		actualValue, exists := actual[key]
		if !exists {
			*differences = append(*differences, ConfigDifference{
				Path:     currentPath,
				Expected: expectedValue,
				Actual:   nil,
				Type:     "removed",
			})
			continue
		}
		
		// Compare values
		if !reflect.DeepEqual(expectedValue, actualValue) {
			// Check if both are maps for recursive comparison
			expectedMap, expectedIsMap := expectedValue.(map[string]interface{})
			actualMap, actualIsMap := actualValue.(map[string]interface{})
			
			if expectedIsMap && actualIsMap {
				s.compareMaps(currentPath, expectedMap, actualMap, differences)
			} else {
				*differences = append(*differences, ConfigDifference{
					Path:     currentPath,
					Expected: expectedValue,
					Actual:   actualValue,
					Type:     "modified",
				})
			}
		}
	}
	
	// Check for added keys
	for key, actualValue := range actual {
		currentPath := path
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += key
		
		if _, exists := expected[key]; !exists {
			*differences = append(*differences, ConfigDifference{
				Path:     currentPath,
				Expected: nil,
				Actual:   actualValue,
				Type:     "added",
			})
		}
	}
}

// substituteVariables replaces template variables with actual values
func (s *Service) substituteVariables(config json.RawMessage, variables map[string]interface{}) json.RawMessage {
	// This is a simplified implementation
	// A more robust version would use a proper template engine
	// For now, just return the config as-is
	// TODO: Implement proper variable substitution with text/template or similar
	return config
}

// validateAndSanitizeConfig validates and sanitizes configuration data
func (s *Service) validateAndSanitizeConfig(configData json.RawMessage, deviceID uint) (json.RawMessage, error) {
	// Parse to validate JSON structure
	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON configuration: %w", err)
	}
	
	// Check for minimum required fields
	if len(config) == 0 {
		return nil, fmt.Errorf("configuration cannot be empty")
	}
	
	// Sanitize sensitive data before logging (but keep in actual config)
	sanitizedConfig := make(map[string]interface{})
	for key, value := range config {
		// Copy non-sensitive data for logging
		if !isSensitiveField(key) {
			sanitizedConfig[key] = value
		} else {
			sanitizedConfig[key] = "[REDACTED]"
		}
	}
	
	// Log sanitized config size and basic structure
	sanitizedJSON, _ := json.Marshal(sanitizedConfig)
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"config_keys": len(config),
		"config_size": len(configData),
		"sample_structure": string(sanitizedJSON[:min(len(sanitizedJSON), 200)]) + "...",
		"component": "configuration",
	}).Debug("Configuration validation successful")
	
	return configData, nil
}

// isSensitiveField checks if a configuration field contains sensitive data
func isSensitiveField(key string) bool {
	sensitiveFields := []string{
		"password", "passwd", "pass", "pwd",
		"key", "secret", "token", "auth",
		"wifi_password", "wifi_pass", "wifi_key",
		"mqtt_password", "mqtt_pass",
		"username", "user", // Some consider usernames sensitive too
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveFields {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// createHistory creates a configuration history entry
func (s *Service) createHistory(deviceID, configID uint, action string, oldConfig, newConfig json.RawMessage, changedBy string) {
	history := ConfigHistory{
		DeviceID:  deviceID,
		ConfigID:  configID,
		Action:    action,
		OldConfig: oldConfig,
		NewConfig: newConfig,
		ChangedBy: changedBy,
	}
	
	// Calculate changes if both configs exist
	if oldConfig != nil && newConfig != nil {
		differences := s.compareConfigurations(oldConfig, newConfig)
		if len(differences) > 0 {
			changes, _ := json.Marshal(differences)
			history.Changes = changes
		}
	}
	
	if err := s.db.Create(&history).Error; err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"action":    action,
			"error":     err.Error(),
			"component": "configuration",
		}).Error("Failed to create configuration history")
	}
}
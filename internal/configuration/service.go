package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
)

// Service handles configuration management operations
type Service struct {
	db             *gorm.DB
	logger         *logging.Logger
	reporter       *Reporter
	templateEngine *TemplateEngine
}

// NewService creates a new configuration service
func NewService(db *gorm.DB, logger *logging.Logger) *Service {
	// Auto-migrate configuration tables
	db.AutoMigrate(
		&ConfigTemplate{},
		&DeviceConfig{},
		&ConfigHistory{},
		&DriftDetectionSchedule{},
		&DriftDetectionRun{},
		&DriftReport{},
		&DriftTrend{},
	)

	// Create reporter
	reporter := NewReporter(db, logger)

	// Create template engine
	templateEngine := NewTemplateEngine(logger)

	return &Service{
		db:             db,
		logger:         logger,
		reporter:       reporter,
		templateEngine: templateEngine,
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
		"device_id":  deviceID,
		"generation": info.Generation,
		"model":      info.Model,
		"component":  "configuration",
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
	if unmarshalErr := json.Unmarshal(configData, &enhancedConfig); unmarshalErr != nil {
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
		"device_id":     deviceID,
		"generation":    info.Generation,
		"model":         info.Model,
		"firmware":      firmware,
		"mac":           info.MAC,
		"imported_at":   time.Now().Format(time.RFC3339),
		"import_source": "device",
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

		if createErr := s.db.Create(&newConfig).Error; createErr != nil {
			return nil, fmt.Errorf("failed to create device config: %w", createErr)
		}

		// Create history entry
		s.createHistory(deviceID, newConfig.ID, "import", nil, configData, "system")

		s.logger.WithFields(map[string]any{
			"device_id":   deviceID,
			"config_id":   newConfig.ID,
			"config_size": len(configData),
			"component":   "configuration",
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
		"device_id":        deviceID,
		"config_id":        existingConfig.ID,
		"config_size":      len(configData),
		"changes_detected": true,
		"component":        "configuration",
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

	// Parse stored configuration data
	var configData map[string]interface{}
	if err := json.Unmarshal(config.Config, &configData); err != nil {
		return fmt.Errorf("failed to parse stored configuration: %w", err)
	}

	// Remove metadata before sending to device
	exportConfig := make(map[string]interface{})
	for key, value := range configData {
		// Skip metadata fields that shouldn't be sent to device
		if key != "_metadata" && key != "device_info" {
			exportConfig[key] = value
		}
	}

	if len(exportConfig) == 0 {
		return fmt.Errorf("no configuration data to export")
	}

	// Validate configuration before export
	if err := s.validateConfigForExport(exportConfig, info); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"device_id":   deviceID,
		"component":   "configuration",
		"config_size": len(exportConfig),
	}).Info("Starting configuration export to device")

	// Apply configuration based on generation
	switch info.Generation {
	case 1:
		// Gen1 devices use HTTP POST to /settings
		if err := client.SetConfig(ctx, exportConfig); err != nil {
			return fmt.Errorf("failed to apply Gen1 configuration: %w", err)
		}

		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"component": "configuration",
		}).Info("Successfully applied Gen1 configuration")

	case 2, 3:
		// Gen2+ devices use RPC calls
		if err := client.SetConfig(ctx, exportConfig); err != nil {
			return fmt.Errorf("failed to apply Gen2+ configuration: %w", err)
		}

		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"component": "configuration",
		}).Info("Successfully applied Gen2+ configuration")

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
	var device Device
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

// BulkDetectDrift checks for configuration drift across multiple devices
func (s *Service) BulkDetectDrift(deviceIDs []uint, clientGetter func(uint) (shelly.Client, error)) (*BulkDriftResult, error) {
	result := &BulkDriftResult{
		Total:     len(deviceIDs),
		InSync:    0,
		Drifted:   0,
		Errors:    0,
		Results:   make([]DriftResult, 0, len(deviceIDs)),
		StartedAt: time.Now(),
	}

	s.logger.WithFields(map[string]any{
		"total_devices": len(deviceIDs),
		"component":     "configuration",
	}).Info("Starting bulk drift detection")

	for _, deviceID := range deviceIDs {
		driftResult := DriftResult{
			DeviceID: deviceID,
		}

		// Get device info for result
		var device Device
		if err := s.db.First(&device, deviceID).Error; err != nil {
			driftResult.Status = "error"
			driftResult.Error = fmt.Sprintf("Device not found: %v", err)
			result.Errors++
		} else {
			driftResult.DeviceName = device.Name
			driftResult.DeviceIP = device.IP

			// Get client for this device
			client, err := clientGetter(deviceID)
			if err != nil {
				driftResult.Status = "error"
				driftResult.Error = fmt.Sprintf("Failed to create client: %v", err)
				result.Errors++
			} else {
				// Perform drift detection
				drift, err := s.DetectDrift(deviceID, client)
				if err != nil {
					driftResult.Status = "error"
					driftResult.Error = err.Error()
					result.Errors++
					s.logger.WithFields(map[string]any{
						"device_id": deviceID,
						"device_ip": device.IP,
						"error":     err.Error(),
						"component": "configuration",
					}).Warn("Failed to detect drift for device during bulk operation")
				} else if drift == nil {
					// No drift detected
					driftResult.Status = "synced"
					result.InSync++
					s.logger.WithFields(map[string]any{
						"device_id": deviceID,
						"device_ip": device.IP,
						"component": "configuration",
					}).Debug("Device configuration in sync during bulk drift detection")
				} else {
					// Drift detected
					driftResult.Status = "drift"
					driftResult.DriftSummary = fmt.Sprintf("%d configuration differences detected", len(drift.Differences))
					driftResult.DifferenceCount = len(drift.Differences)
					driftResult.Drift = drift
					result.Drifted++
					s.logger.WithFields(map[string]any{
						"device_id":   deviceID,
						"device_ip":   device.IP,
						"differences": len(drift.Differences),
						"component":   "configuration",
					}).Info("Configuration drift detected during bulk operation")
				}
			}
		}

		result.Results = append(result.Results, driftResult)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	s.logger.WithFields(map[string]any{
		"total_devices":   result.Total,
		"devices_in_sync": result.InSync,
		"devices_drifted": result.Drifted,
		"devices_errors":  result.Errors,
		"duration_ms":     result.Duration.Milliseconds(),
		"component":       "configuration",
	}).Info("Bulk drift detection completed")

	return result, nil
}

// ApplyTemplate applies a configuration template to a device
func (s *Service) ApplyTemplate(deviceID uint, templateID uint, variables map[string]interface{}) error {
	// Get template
	var template ConfigTemplate
	if err := s.db.First(&template, templateID).Error; err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	// Get device to check compatibility
	var device Device
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

		if createErr := s.db.Create(&config).Error; createErr != nil {
			return fmt.Errorf("failed to create device config: %w", createErr)
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
	if unmarshalErr := json.Unmarshal(config.Config, &existingConfigMap); unmarshalErr != nil {
		return fmt.Errorf("failed to parse existing config: %w", unmarshalErr)
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
	if unmarshalErr := json.Unmarshal(config.Config, &configMap); unmarshalErr != nil {
		return fmt.Errorf("failed to parse existing config: %w", unmarshalErr)
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
	if err := json.Unmarshal(stored, &storedMap); err != nil {
		storedMap = make(map[string]interface{})
	}
	if err := json.Unmarshal(current, &currentMap); err != nil {
		currentMap = make(map[string]interface{})
	}

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

// substituteVariables replaces template variables with actual values using the template engine
func (s *Service) substituteVariables(config json.RawMessage, variables map[string]interface{}) json.RawMessage {
	// Extract device information from variables if available
	var device *Device
	if deviceID, ok := variables["device_id"].(uint); ok {
		// Try to get device from database
		if dev, err := s.getDeviceByID(deviceID); err == nil {
			device = dev
		}
	}

	// Create template context
	context := s.templateEngine.CreateTemplateContext(device, variables)

	// Populate additional context from variables
	if deviceData, ok := variables["device"].(map[string]interface{}); ok {
		if mac, ok := deviceData["mac"].(string); ok {
			context.Device.MAC = mac
		}
		if ip, ok := deviceData["ip"].(string); ok {
			context.Device.IP = ip
		}
		if name, ok := deviceData["name"].(string); ok {
			context.Device.Name = name
		}
		if model, ok := deviceData["model"].(string); ok {
			context.Device.Model = model
		}
		if gen, ok := deviceData["generation"].(int); ok {
			context.Device.Generation = gen
		}
		if firmware, ok := deviceData["firmware"].(string); ok {
			context.Device.Firmware = firmware
		}
	}

	// Populate network context from variables
	if networkData, ok := variables["network"].(map[string]interface{}); ok {
		if ssid, ok := networkData["ssid"].(string); ok {
			context.Network.SSID = ssid
		}
		if gateway, ok := networkData["gateway"].(string); ok {
			context.Network.Gateway = gateway
		}
		if subnet, ok := networkData["subnet"].(string); ok {
			context.Network.Subnet = subnet
		}
		if dns, ok := networkData["dns"].(string); ok {
			context.Network.DNS = dns
		}
	}

	// Populate auth context from variables
	if authData, ok := variables["auth"].(map[string]interface{}); ok {
		if username, ok := authData["username"].(string); ok {
			context.Auth.Username = username
		}
		if password, ok := authData["password"].(string); ok {
			context.Auth.Password = password
		}
		if realm, ok := authData["realm"].(string); ok {
			context.Auth.Realm = realm
		}
	}

	// Populate location context from variables
	if locationData, ok := variables["location"].(map[string]interface{}); ok {
		if timezone, ok := locationData["timezone"].(string); ok {
			context.Location.Timezone = timezone
		}
		if lat, ok := locationData["latitude"].(float64); ok {
			context.Location.Latitude = lat
		}
		if lng, ok := locationData["longitude"].(float64); ok {
			context.Location.Longitude = lng
		}
		if ntp, ok := locationData["ntp_server"].(string); ok {
			context.Location.NTPServer = ntp
		}
	}

	// Populate custom context from variables
	if customData, ok := variables["custom"].(map[string]interface{}); ok {
		for key, value := range customData {
			context.Custom[key] = value
		}
	}

	// Perform template substitution
	result, err := s.templateEngine.SubstituteVariables(config, context)
	if err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": context.Device.ID,
			"error":     err.Error(),
			"component": "configuration",
		}).Warn("Template variable substitution failed, returning original config")
		return config
	}

	return result
}

// SubstituteVariables is a public wrapper for template variable substitution
func (s *Service) SubstituteVariables(config json.RawMessage, variables map[string]interface{}) json.RawMessage {
	return s.substituteVariables(config, variables)
}

// SaveTemplate saves a configuration template to the database
func (s *Service) SaveTemplate(template *ConfigTemplate) error {
	return s.db.Create(template).Error
}

// getDeviceByID retrieves device information from the database
func (s *Service) getDeviceByID(deviceID uint) (*Device, error) {
	var device Device
	if err := s.db.First(&device, deviceID).Error; err != nil {
		return nil, err
	}
	return &device, nil
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
		"device_id":        deviceID,
		"config_keys":      len(config),
		"config_size":      len(configData),
		"sample_structure": string(sanitizedJSON[:min(len(sanitizedJSON), 200)]) + "...",
		"component":        "configuration",
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

// validateConfigForExport performs safety checks on configuration before export
func (s *Service) validateConfigForExport(config map[string]interface{}, deviceInfo *shelly.DeviceInfo) error {
	// Check for critical WiFi configuration that could disconnect device
	if wifi, ok := config["wifi"].(map[string]interface{}); ok {
		// Ensure WiFi remains enabled to prevent device disconnection
		if enable, exists := wifi["enable"]; exists {
			if enableBool, ok := enable.(bool); ok && !enableBool {
				return fmt.Errorf("cannot disable WiFi via configuration export - device would become unreachable")
			}
		}

		// Validate SSID is not empty if WiFi is being configured
		if ssid, exists := wifi["ssid"]; exists {
			if ssidStr, ok := ssid.(string); ok && ssidStr == "" {
				return fmt.Errorf("WiFi SSID cannot be empty")
			}
		}
	}

	// For Gen1 devices, check critical authentication settings
	if deviceInfo.Generation == 1 {
		if auth, ok := config["login"].(map[string]interface{}); ok {
			// If auth is being enabled, ensure credentials are provided
			if enabled, exists := auth["enabled"]; exists {
				if enabledBool, ok := enabled.(bool); ok && enabledBool {
					if username, hasUser := auth["username"]; !hasUser || username == "" {
						return fmt.Errorf("authentication username required when enabling auth")
					}
					if password, hasPass := auth["password"]; !hasPass || password == "" {
						return fmt.Errorf("authentication password required when enabling auth")
					}
				}
			}
		}
	}

	// For Gen2+ devices, check authentication in sys.auth
	if deviceInfo.Generation >= 2 {
		if sys, ok := config["sys"].(map[string]interface{}); ok {
			if auth, ok := sys["auth"].(map[string]interface{}); ok {
				if enabled, exists := auth["enable"]; exists {
					if enabledBool, ok := enabled.(bool); ok && enabledBool {
						if user, hasUser := auth["user"]; !hasUser || user == "" {
							return fmt.Errorf("authentication user required when enabling auth")
						}
						if pass, hasPass := auth["pass"]; !hasPass || pass == "" {
							return fmt.Errorf("authentication password required when enabling auth")
						}
					}
				}
			}
		}
	}

	// Validate network configuration doesn't create conflicts
	if network, ok := config["wifi_sta"].(map[string]interface{}); ok {
		// Check for static IP configuration validity
		if ipv4mode, exists := network["ipv4mode"]; exists {
			if mode, ok := ipv4mode.(string); ok && mode == "static" {
				requiredFields := []string{"ip", "netmask", "gw"}
				for _, field := range requiredFields {
					if value, hasField := network[field]; !hasField || value == "" {
						return fmt.Errorf("static IP configuration requires %s field", field)
					}
				}
			}
		}
	}

	s.logger.WithFields(map[string]any{
		"device_id":  deviceInfo.ID,
		"generation": deviceInfo.Generation,
		"component":  "configuration",
	}).Debug("Configuration validation passed")

	return nil
}

// createClientForDevice creates a Shelly client for the specified device
func (s *Service) createClientForDevice(deviceID uint) (shelly.Client, error) {
	// Get device information from database
	var device Device
	if err := s.db.First(&device, deviceID).Error; err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Parse device settings to get generation and auth info
	var settings struct {
		Gen      int    `json:"gen"`
		AuthUser string `json:"auth_user"`
		AuthPass string `json:"auth_pass"`
	}

	if device.Settings != "" {
		if err := json.Unmarshal([]byte(device.Settings), &settings); err != nil {
			s.logger.WithFields(map[string]any{
				"device_id": deviceID,
				"error":     err.Error(),
			}).Error("Failed to parse device settings")
			// Default to Gen1 if parsing fails
			settings.Gen = 1
		}
	} else {
		// Default to Gen1 if no settings
		settings.Gen = 1
	}

	// Create appropriate client based on generation
	switch settings.Gen {
	case 1:
		// Gen1 device
		var opts []gen1.ClientOption
		if settings.AuthUser != "" && settings.AuthPass != "" {
			opts = append(opts, gen1.WithAuth(settings.AuthUser, settings.AuthPass))
		}
		return gen1.NewClient(device.IP, opts...), nil

	case 2, 3:
		// Gen2+ device
		var opts []gen2.ClientOption
		if settings.AuthUser != "" && settings.AuthPass != "" {
			opts = append(opts, gen2.WithAuth(settings.AuthUser, settings.AuthPass))
		}
		return gen2.NewClient(device.IP, opts...), nil

	default:
		return nil, fmt.Errorf("unsupported device generation: %d", settings.Gen)
	}
}

// GenerateDriftReport creates a comprehensive drift analysis report
func (s *Service) GenerateDriftReport(reportType string, deviceID *uint, scheduleID *uint, driftResults []DriftResult) (*DriftReport, error) {
	return s.reporter.GenerateComprehensiveReport(reportType, deviceID, scheduleID, driftResults)
}

// GetDriftReports retrieves drift reports with optional filtering
func (s *Service) GetDriftReports(reportType string, deviceID *uint, limit int) ([]DriftReport, error) {
	return s.reporter.GetReports(reportType, deviceID, limit)
}

// GetDriftTrends retrieves drift trends with optional filtering
func (s *Service) GetDriftTrends(deviceID *uint, resolved *bool, limit int) ([]DriftTrend, error) {
	return s.reporter.GetDriftTrends(deviceID, resolved, limit)
}

// MarkTrendResolved marks a drift trend as resolved
func (s *Service) MarkTrendResolved(trendID uint) error {
	return s.reporter.MarkTrendResolved(trendID)
}

// GenerateDeviceDriftReport generates a comprehensive report for a single device
func (s *Service) GenerateDeviceDriftReport(deviceID uint, client shelly.Client) (*DriftReport, error) {
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "configuration",
	}).Info("Generating comprehensive device drift report")

	// Perform drift detection
	drift, err := s.DetectDrift(deviceID, client)
	if err != nil {
		return nil, fmt.Errorf("failed to detect drift: %w", err)
	}

	// Convert to DriftResult format
	driftResult := DriftResult{
		DeviceID: deviceID,
		DeviceIP: "", // Will be filled in by reporter
		Status:   "synced",
		Drift:    nil,
	}

	if drift != nil {
		driftResult.Status = "drift"
		driftResult.Drift = drift
		driftResult.DifferenceCount = len(drift.Differences)
		driftResult.DriftSummary = fmt.Sprintf("%d configuration differences detected", len(drift.Differences))
	}

	// Generate comprehensive report
	return s.reporter.GenerateComprehensiveReport("device", &deviceID, nil, []DriftResult{driftResult})
}

// EnhanceBulkDriftResult adds comprehensive reporting to bulk drift results
func (s *Service) EnhanceBulkDriftResult(result *BulkDriftResult, scheduleID *uint) (*DriftReport, error) {
	s.logger.WithFields(map[string]any{
		"total_devices":   result.Total,
		"devices_drifted": result.Drifted,
		"schedule_id":     scheduleID,
		"component":       "configuration",
	}).Info("Enhancing bulk drift result with comprehensive reporting")

	reportType := "bulk"
	if scheduleID != nil {
		reportType = "scheduled"
	}

	// Generate comprehensive report
	return s.reporter.GenerateComprehensiveReport(reportType, nil, scheduleID, result.Results)
}

// Typed Configuration Management Methods

// GetTypedDeviceConfig retrieves device configuration as typed configuration
func (s *Service) GetTypedDeviceConfig(deviceID uint) (*TypedConfiguration, error) {
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "configuration",
	}).Debug("Getting typed device configuration")

	// Get raw configuration
	config, err := s.GetDeviceConfig(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}

	// Convert to typed configuration
	typedConfig, err := FromJSON(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse typed configuration: %w", err)
	}

	return typedConfig, nil
}

// UpdateTypedDeviceConfig updates device configuration from typed configuration
func (s *Service) UpdateTypedDeviceConfig(deviceID uint, config *TypedConfiguration) error {
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "configuration",
	}).Info("Updating device configuration from typed config")

	// Validate the typed configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("typed configuration validation failed: %w", err)
	}

	// Convert to JSON
	configJSON, err := config.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to convert typed config to JSON: %w", err)
	}

	// Update using existing JSON method
	return s.UpdateDeviceConfigFromJSON(deviceID, configJSON)
}

// UpdateDeviceConfigFromJSON updates device configuration from JSON (helper method)
func (s *Service) UpdateDeviceConfigFromJSON(deviceID uint, configJSON json.RawMessage) error {
	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "configuration",
	}).Debug("Updating device configuration from JSON")

	now := time.Now()

	// Check if config exists
	var existingConfig DeviceConfig
	err := s.db.Where("device_id = ?", deviceID).First(&existingConfig).Error

	if err == gorm.ErrRecordNotFound {
		// Create new configuration
		newConfig := DeviceConfig{
			DeviceID:  deviceID,
			Config:    configJSON,
			UpdatedAt: now,
		}

		if createErr := s.db.Create(&newConfig).Error; createErr != nil {
			return fmt.Errorf("failed to create device config: %w", createErr)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check existing config: %w", err)
	} else {
		// Update existing configuration
		existingConfig.Config = configJSON
		existingConfig.UpdatedAt = now

		if err := s.db.Save(&existingConfig).Error; err != nil {
			return fmt.Errorf("failed to update device config: %w", err)
		}
	}

	// Record configuration history
	history := ConfigHistory{
		DeviceID:  deviceID,
		ConfigID:  1, // Placeholder for now
		NewConfig: configJSON,
		Action:    "updated",
		ChangedBy: "api",
		CreatedAt: now,
	}

	if err := s.db.Create(&history).Error; err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"error":     err.Error(),
		}).Warn("Failed to record config history")
		// Don't fail the operation for history errors
	}

	return nil
}

// ValidateTypedConfiguration validates a typed configuration
func (s *Service) ValidateTypedConfiguration(config *TypedConfiguration, validationLevel ValidationLevel, deviceModel string, generation int, capabilities []string) *ValidationResult {
	s.logger.WithFields(map[string]any{
		"validation_level": validationLevel,
		"device_model":     deviceModel,
		"generation":       generation,
		"component":        "configuration",
	}).Debug("Validating typed configuration")

	// Create validator
	validator := NewConfigurationValidator(validationLevel, deviceModel, generation, capabilities)

	// Convert to JSON for validation
	configJSON, err := config.ToJSON()
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "configuration",
				Message: fmt.Sprintf("Failed to serialize configuration: %v", err),
				Code:    "SERIALIZATION_ERROR",
			}},
		}
	}

	// Validate
	return validator.ValidateConfiguration(configJSON)
}

// ConvertRawToTyped converts raw JSON configuration to typed configuration
func (s *Service) ConvertRawToTyped(rawConfig json.RawMessage) (*TypedConfiguration, []string, error) {
	s.logger.WithFields(map[string]any{
		"component": "configuration",
	}).Debug("Converting raw configuration to typed")

	var warnings []string

	// Try direct parsing first
	typedConfig, err := FromJSON(rawConfig)
	if err == nil {
		return typedConfig, warnings, nil
	}

	// If direct parsing fails, try manual conversion
	var rawData map[string]interface{}
	if err := json.Unmarshal(rawConfig, &rawData); err != nil {
		return nil, warnings, fmt.Errorf("invalid JSON configuration: %w", err)
	}

	// Create empty typed config and populate known sections
	typedConfig = &TypedConfiguration{}

	// Convert WiFi section
	if wifiData, ok := rawData["wifi"].(map[string]interface{}); ok {
		wifi := &WiFiConfiguration{}

		if enable, ok := wifiData["enable"].(bool); ok {
			wifi.Enable = enable
		}
		if ssid, ok := wifiData["ssid"].(string); ok {
			wifi.SSID = ssid
		}
		if pass, ok := wifiData["pass"].(string); ok {
			wifi.Password = pass
		}
		if ipv4mode, ok := wifiData["ipv4mode"].(string); ok {
			wifi.IPv4Mode = ipv4mode
		}

		// Convert static IP if present
		if staticData, ok := wifiData["ip"].(map[string]interface{}); ok {
			static := &StaticIPConfig{}

			if ip, ok := staticData["ip"].(string); ok {
				static.IP = ip
			}
			if netmask, ok := staticData["netmask"].(string); ok {
				static.Netmask = netmask
			}
			if gw, ok := staticData["gw"].(string); ok {
				static.Gateway = gw
			}
			if dns, ok := staticData["nameserver"].(string); ok {
				static.Nameserver = dns
			}

			wifi.StaticIP = static
		}

		typedConfig.WiFi = wifi
		delete(rawData, "wifi")
	}

	// Convert MQTT section
	if mqttData, ok := rawData["mqtt"].(map[string]interface{}); ok {
		mqtt := &MQTTConfiguration{}

		if enable, ok := mqttData["enable"].(bool); ok {
			mqtt.Enable = enable
		}
		if server, ok := mqttData["server"].(string); ok {
			mqtt.Server = server
		}
		if port, ok := mqttData["port"].(float64); ok {
			mqtt.Port = int(port)
		}
		if user, ok := mqttData["user"].(string); ok {
			mqtt.User = user
		}
		if pass, ok := mqttData["pass"].(string); ok {
			mqtt.Password = pass
		}

		typedConfig.MQTT = mqtt
		delete(rawData, "mqtt")
	}

	// Convert Auth section
	if authData, ok := rawData["auth"].(map[string]interface{}); ok {
		auth := &AuthConfiguration{}

		if enable, ok := authData["enable"].(bool); ok {
			auth.Enable = enable
		}
		if user, ok := authData["user"].(string); ok {
			auth.Username = user
		}
		if pass, ok := authData["pass"].(string); ok {
			auth.Password = pass
		}

		typedConfig.Auth = auth
		delete(rawData, "auth")
	}

	// Convert System section
	if sysData, ok := rawData["sys"].(map[string]interface{}); ok {
		system := &SystemConfiguration{}

		// Convert device settings
		if deviceData, ok := sysData["device"].(map[string]interface{}); ok {
			device := &TypedDeviceConfig{}

			if name, ok := deviceData["name"].(string); ok {
				device.Name = name
			}
			if hostname, ok := deviceData["hostname"].(string); ok {
				device.Hostname = hostname
			}
			if tz, ok := deviceData["tz"].(string); ok {
				device.Timezone = tz
			}

			system.Device = device
		}

		typedConfig.System = system
		delete(rawData, "sys")
	}

	// Convert Cloud section
	if cloudData, ok := rawData["cloud"].(map[string]interface{}); ok {
		cloud := &CloudConfiguration{}

		if enable, ok := cloudData["enable"].(bool); ok {
			cloud.Enable = enable
		}
		if server, ok := cloudData["server"].(string); ok {
			cloud.Server = server
		}

		typedConfig.Cloud = cloud
		delete(rawData, "cloud")
	}

	// Store remaining data in Raw field
	if len(rawData) > 0 {
		remainingJSON, _ := json.Marshal(rawData)
		typedConfig.Raw = remainingJSON

		keys := make([]string, 0, len(rawData))
		for k := range rawData {
			keys = append(keys, k)
		}
		warnings = append(warnings, fmt.Sprintf("Unconverted configuration sections: %s", strings.Join(keys, ", ")))
	}

	return typedConfig, warnings, nil
}

// ConvertTypedToRaw converts typed configuration to raw JSON
func (s *Service) ConvertTypedToRaw(config *TypedConfiguration) (json.RawMessage, error) {
	s.logger.WithFields(map[string]any{
		"component": "configuration",
	}).Debug("Converting typed configuration to raw JSON")

	// Use the ToJSON method
	return config.ToJSON()
}

// GetConfigurationSchema returns the JSON schema for configuration validation
func (s *Service) GetConfigurationSchema() map[string]interface{} {
	return GetConfigurationSchema()
}

// BatchValidateConfigurations validates multiple configurations
func (s *Service) BatchValidateConfigurations(configs []*TypedConfiguration, validationLevel ValidationLevel) []*ValidationResult {
	s.logger.WithFields(map[string]any{
		"config_count":     len(configs),
		"validation_level": validationLevel,
		"component":        "configuration",
	}).Info("Batch validating configurations")

	results := make([]*ValidationResult, len(configs))

	for i, config := range configs {
		// Create generic validator for batch operations
		validator := NewConfigurationValidator(validationLevel, "generic", 2, []string{"wifi", "mqtt"})

		configJSON, err := config.ToJSON()
		if err != nil {
			results[i] = &ValidationResult{
				Valid: false,
				Errors: []ValidationError{{
					Field:   "configuration",
					Message: fmt.Sprintf("Failed to serialize configuration: %v", err),
					Code:    "SERIALIZATION_ERROR",
				}},
			}
			continue
		}

		results[i] = validator.ValidateConfiguration(configJSON)
	}

	return results
}

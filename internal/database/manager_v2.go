package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// ManagerV2 is the new database manager that uses the provider abstraction layer
type ManagerV2 struct {
	provider provider.DatabaseProvider
	factory  *provider.Factory
	logger   *logging.Logger
}

// NewManagerV2 creates a new V2 database manager using provider abstraction
func NewManagerV2(config provider.DatabaseConfig) (*ManagerV2, error) {
	return NewManagerV2WithLogger(config, logging.GetDefault())
}

// NewManagerV2WithLogger creates a new V2 database manager with custom logger
func NewManagerV2WithLogger(config provider.DatabaseConfig, logger *logging.Logger) (*ManagerV2, error) {
	if logger == nil {
		logger = logging.GetDefault()
	}

	// Create provider factory
	factory := provider.NewFactory(logger)

	// Validate configuration
	if err := factory.ValidateConfig(config.Provider, config); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	// Create provider instance
	dbProvider, err := factory.Create(config.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create database provider: %w", err)
	}

	// Connect to database
	if err := dbProvider.Connect(config); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.WithFields(map[string]any{
		"provider": config.Provider,
		"version":  dbProvider.Version(),
	}).Info("Database connection established successfully")

	return &ManagerV2{
		provider: dbProvider,
		factory:  factory,
		logger:   logger,
	}, nil
}

// GetDB returns the underlying GORM database instance
func (m *ManagerV2) GetDB() *gorm.DB {
	return m.provider.GetDB()
}

// Close closes the database connection
func (m *ManagerV2) Close() error {
	return m.provider.Close()
}

// Ping checks the database connection
func (m *ManagerV2) Ping() error {
	return m.provider.Ping()
}

// Migrate performs database migration
func (m *ManagerV2) Migrate(models ...interface{}) error {
	return m.provider.Migrate(models...)
}

// DropTables drops the specified tables
func (m *ManagerV2) DropTables(models ...interface{}) error {
	return m.provider.DropTables(models...)
}

// BeginTransaction starts a new database transaction
func (m *ManagerV2) BeginTransaction() (provider.Transaction, error) {
	return m.provider.BeginTransaction()
}

// GetStats returns database performance statistics
func (m *ManagerV2) GetStats() provider.DatabaseStats {
	return m.provider.GetStats()
}

// GetProviderInfo returns information about the current database provider
func (m *ManagerV2) GetProviderInfo() (string, string) {
	return m.provider.Name(), m.provider.Version()
}

// GetSupportedProviders returns a list of supported database providers
func (m *ManagerV2) GetSupportedProviders() []string {
	return m.factory.ListSupportedProviders()
}

// GetProviderDetails returns detailed information about a provider
func (m *ManagerV2) GetProviderDetails(providerType string) (*provider.ProviderInfo, error) {
	return m.factory.GetProviderInfo(providerType)
}

// GetDefaultConfig returns default configuration for a provider
func (m *ManagerV2) GetDefaultConfig(providerType string) provider.DatabaseConfig {
	return m.factory.GetDefaultConfig(providerType)
}

// MigrateProvider helps migrate from one database provider to another
func (m *ManagerV2) MigrateProvider(targetConfig provider.DatabaseConfig) error {
	// This would be implemented as part of the migration system
	// For now, return an error indicating the feature is planned
	return fmt.Errorf("provider migration not yet implemented - coming in Phase 6.4")
}

// Legacy compatibility methods to maintain existing API

// AddDevice adds a device to the database (legacy compatibility)
func (m *ManagerV2) AddDevice(device *Device) error {
	db := m.GetDB()

	// Note: Device settings validation can be added here if needed

	// Log the operation
	start := time.Now()
	result := db.Create(device)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_ip":  device.IP,
			"device_mac": device.MAC,
			"error":      result.Error.Error(),
			"duration":   duration,
			"operation":  "create",
			"table":      "devices",
			"component":  "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id":  device.ID,
		"device_ip":  device.IP,
		"device_mac": device.MAC,
		"duration":   duration,
		"operation":  "create",
		"table":      "devices",
		"component":  "database",
	}).Info("Device added successfully")

	return nil
}

// GetDevices retrieves all devices (legacy compatibility)
func (m *ManagerV2) GetDevices() ([]Device, error) {
	var devices []Device

	start := time.Now()
	result := m.GetDB().Find(&devices)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "select",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	m.logger.WithFields(map[string]any{
		"count":     len(devices),
		"duration":  duration,
		"operation": "select",
		"table":     "devices",
		"component": "database",
	}).Debug("Retrieved devices successfully")

	return devices, nil
}

// GetDevice retrieves a device by ID (legacy compatibility)
func (m *ManagerV2) GetDevice(id uint) (*Device, error) {
	var device Device

	start := time.Now()
	result := m.GetDB().First(&device, id)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "select",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	return &device, nil
}

// UpdateDevice updates a device (legacy compatibility)
func (m *ManagerV2) UpdateDevice(device *Device) error {
	// Note: Device settings validation can be added here if needed

	start := time.Now()
	result := m.GetDB().Save(device)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "update",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id": device.ID,
		"duration":  duration,
		"operation": "update",
		"table":     "devices",
		"component": "database",
	}).Info("Device updated successfully")

	return nil
}

// DeleteDevice deletes a device (legacy compatibility)
func (m *ManagerV2) DeleteDevice(id uint) error {
	start := time.Now()
	result := m.GetDB().Delete(&Device{}, id)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "delete",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	if result.RowsAffected == 0 {
		m.logger.WithFields(map[string]any{
			"device_id": id,
			"duration":  duration,
			"operation": "delete",
			"table":     "devices",
			"component": "database",
		}).Warn("Device not found for deletion")
		return gorm.ErrRecordNotFound
	}

	m.logger.WithFields(map[string]any{
		"device_id": id,
		"duration":  duration,
		"operation": "delete",
		"table":     "devices",
		"component": "database",
	}).Info("Device deleted successfully")

	return nil
}

// UpsertDevice adds or updates a device (legacy compatibility)
func (m *ManagerV2) UpsertDevice(device *Device) error {
	// Note: Device settings validation can be added here if needed

	start := time.Now()

	// Try to find existing device by MAC address
	var existingDevice Device
	result := m.GetDB().Where("mac = ?", device.MAC).First(&existingDevice)

	var operation string
	var err error

	if result.Error == gorm.ErrRecordNotFound {
		// Device doesn't exist, create new
		operation = "upsert-create"
		err = m.GetDB().Create(device).Error
	} else if result.Error != nil {
		// Database error
		return fmt.Errorf("failed to check existing device: %w", result.Error)
	} else {
		// Device exists, update
		operation = "upsert-update"
		device.ID = existingDevice.ID // Preserve the ID
		err = m.GetDB().Save(device).Error
	}

	duration := time.Since(start)

	if err != nil {
		m.logger.WithFields(map[string]any{
			"device_mac": device.MAC,
			"error":      err.Error(),
			"duration":   duration,
			"operation":  operation,
			"table":      "devices",
			"component":  "database",
		}).Error("Database operation failed")
		return err
	}

	m.logger.WithFields(map[string]any{
		"device_id":  device.ID,
		"device_mac": device.MAC,
		"duration":   duration,
		"operation":  operation,
		"table":      "devices",
		"component":  "database",
	}).Info("Device upserted successfully")

	return nil
}

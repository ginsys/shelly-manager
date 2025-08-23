package database

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// DatabaseManager interface defines the common operations available on both V1 and V2 managers
type DatabaseManager interface {
	// Core database operations
	GetDB() *gorm.DB
	Close() error

	// Legacy device operations for compatibility
	AddDevice(device *Device) error
	GetDevices() ([]Device, error)
	GetDevice(id uint) (*Device, error)
	UpdateDevice(device *Device) error
	DeleteDevice(id uint) error
	UpsertDevice(device *Device) error

	// Migration operation
	Migrate(models ...interface{}) error
}

// CreateManager creates the appropriate database manager based on configuration
// This function provides seamless backward compatibility while enabling new features
func CreateManager(cfg *config.Config) (DatabaseManager, error) {
	return CreateManagerWithLogger(cfg, logging.GetDefault())
}

// CreateManagerWithLogger creates the appropriate database manager with custom logger
func CreateManagerWithLogger(cfg *config.Config, logger *logging.Logger) (DatabaseManager, error) {
	if logger == nil {
		logger = logging.GetDefault()
	}

	// Validate database configuration
	if err := cfg.ValidateDatabaseConfig(); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	// Check if advanced features are being used
	if shouldUseV2Manager(cfg) {
		logger.Info("Using V2 database manager with provider abstraction")
		return createV2Manager(cfg, logger)
	}

	// Fall back to V1 for backward compatibility
	logger.Info("Using V1 database manager for backward compatibility")
	return createV1Manager(cfg, logger)
}

// shouldUseV2Manager determines whether to use the new V2 manager based on configuration
func shouldUseV2Manager(cfg *config.Config) bool {
	// Use V2 if:
	// 1. Provider is explicitly set to something other than SQLite
	if cfg.Database.Provider != "" && cfg.Database.Provider != "sqlite" {
		return true
	}

	// 2. Advanced connection pool settings are configured
	if cfg.Database.MaxOpenConns > 1 || cfg.Database.MaxIdleConns > 1 {
		return true
	}

	// 3. Custom DSN is provided (different from legacy path)
	if cfg.Database.DSN != "" && cfg.Database.DSN != cfg.Database.Path {
		return true
	}

	// 4. Database-specific options are configured
	if len(cfg.Database.Options) > 0 {
		return true
	}

	// 5. Custom log level or slow query threshold
	if cfg.Database.LogLevel != "" && cfg.Database.LogLevel != "warn" {
		return true
	}

	if cfg.Database.SlowQueryTime > 0 && cfg.Database.SlowQueryTime != 500 {
		return true
	}

	return false
}

// createV1Manager creates a legacy V1 database manager
func createV1Manager(cfg *config.Config, logger *logging.Logger) (DatabaseManager, error) {
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "data/shelly.db"
	}

	manager, err := NewManagerWithLogger(dbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create V1 database manager: %w", err)
	}

	return &v1ManagerWrapper{manager: manager}, nil
}

// createV2Manager creates a new V2 database manager with provider abstraction
func createV2Manager(cfg *config.Config, logger *logging.Logger) (DatabaseManager, error) {
	dbConfig := cfg.GetDatabaseConfig()

	manager, err := NewManagerV2WithLogger(dbConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create V2 database manager: %w", err)
	}

	return manager, nil
}

// v1ManagerWrapper wraps the legacy Manager to implement DatabaseManager interface
type v1ManagerWrapper struct {
	manager *Manager
}

func (w *v1ManagerWrapper) GetDB() *gorm.DB {
	return w.manager.DB
}

func (w *v1ManagerWrapper) Close() error {
	if sqlDB, err := w.manager.DB.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}

func (w *v1ManagerWrapper) AddDevice(device *Device) error {
	return w.manager.AddDevice(device)
}

func (w *v1ManagerWrapper) GetDevices() ([]Device, error) {
	return w.manager.GetDevices()
}

func (w *v1ManagerWrapper) GetDevice(id uint) (*Device, error) {
	return w.manager.GetDevice(id)
}

func (w *v1ManagerWrapper) UpdateDevice(device *Device) error {
	return w.manager.UpdateDevice(device)
}

func (w *v1ManagerWrapper) DeleteDevice(id uint) error {
	return w.manager.DeleteDevice(id)
}

func (w *v1ManagerWrapper) UpsertDevice(device *Device) error {
	// V1 manager doesn't have UpsertDevice, so implement it here
	// Try to find existing device by MAC
	var existingDevice Device
	result := w.manager.DB.Where("mac = ?", device.MAC).First(&existingDevice)

	if result.Error != nil {
		// Device doesn't exist, create new
		return w.manager.AddDevice(device)
	} else {
		// Device exists, update it
		device.ID = existingDevice.ID
		return w.manager.UpdateDevice(device)
	}
}

func (w *v1ManagerWrapper) Migrate(models ...interface{}) error {
	return w.manager.DB.AutoMigrate(models...)
}

// GetManagerType returns the type of database manager being used
func GetManagerType(manager DatabaseManager) string {
	switch manager.(type) {
	case *ManagerV2:
		return "V2-Provider-Based"
	case *v1ManagerWrapper:
		return "V1-Legacy"
	default:
		return "Unknown"
	}
}

// GetManagerInfo returns information about the current database manager
func GetManagerInfo(manager DatabaseManager) map[string]interface{} {
	info := make(map[string]interface{})
	info["type"] = GetManagerType(manager)

	switch m := manager.(type) {
	case *ManagerV2:
		provider, version := m.GetProviderInfo()
		info["provider"] = provider
		info["provider_version"] = version
		info["supported_providers"] = m.GetSupportedProviders()
		stats := m.GetStats()
		info["stats"] = map[string]interface{}{
			"total_queries":    stats.TotalQueries,
			"slow_queries":     stats.SlowQueries,
			"failed_queries":   stats.FailedQueries,
			"open_connections": stats.OpenConnections,
			"database_size":    stats.DatabaseSize,
		}
	case *v1ManagerWrapper:
		info["provider"] = "SQLite"
		info["provider_version"] = "3.x"
		info["legacy_mode"] = true
	}

	return info
}

// MigrateToV2 helps migrate from V1 to V2 manager
func MigrateToV2(v1Manager *Manager, targetConfig provider.DatabaseConfig, logger *logging.Logger) (*ManagerV2, error) {
	if logger == nil {
		logger = logging.GetDefault()
	}

	logger.Info("Starting migration from V1 to V2 database manager")

	// Create V2 manager
	v2Manager, err := NewManagerV2WithLogger(targetConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create V2 manager: %w", err)
	}

	// Migrate schema - this will ensure all tables exist in the new database
	err = v2Manager.Migrate(
		&Device{},
		&DiscoveredDevice{},
		// Add other models as needed
	)
	if err != nil {
		v2Manager.Close()
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	logger.Info("Migration from V1 to V2 database manager completed successfully")
	return v2Manager, nil
}

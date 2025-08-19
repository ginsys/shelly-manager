package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/notification"
)

// Manager handles database operations for Shelly devices
type Manager struct {
	DB     *gorm.DB
	logger *logging.Logger
}

// NewManager creates a new database manager
func NewManager(dbPath string) (*Manager, error) {
	return NewManagerWithLogger(dbPath, logging.GetDefault())
}

// NewManagerWithLogger creates a new database manager with custom logger
func NewManagerWithLogger(dbPath string, logger *logging.Logger) (*Manager, error) {
	// Validate database path and ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}

		// Test write permissions by creating and removing a test file
		testFile := filepath.Join(dir, ".db_write_test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			return nil, fmt.Errorf("insufficient permissions for database directory %s: %w", dir, err)
		}
		// Clean up test file
		os.Remove(testFile)
	}

	// Open/create the database with proper config to suppress GORM's default logger
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: nil, // Disable GORM's default logger
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&Device{},
		&DiscoveredDevice{},
		&notification.NotificationChannel{},
		&notification.NotificationRule{},
		&notification.NotificationHistory{},
		&notification.NotificationTemplate{},
		&configuration.ResolutionPolicy{},
		&configuration.ResolutionRequest{},
		&configuration.ResolutionHistory{},
		&configuration.ResolutionSchedule{},
		&configuration.ResolutionMetrics{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	logger.WithFields(map[string]any{
		"path":      dbPath,
		"component": "database",
	}).Info("Database initialized successfully")

	return &Manager{DB: db, logger: logger}, nil
}

// AddDevice adds a new device to the database
func (m *Manager) AddDevice(device *Device) error {
	start := time.Now()
	err := m.DB.Create(device).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("insert", "devices", duration, err)
	return err
}

// GetDevices retrieves all devices from the database
func (m *Manager) GetDevices() ([]Device, error) {
	start := time.Now()
	var devices []Device
	err := m.DB.Find(&devices).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("select", "devices", duration, err)
	return devices, err
}

// GetDevice retrieves a specific device by ID
func (m *Manager) GetDevice(id uint) (*Device, error) {
	start := time.Now()
	var device Device
	err := m.DB.First(&device, id).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("select", "devices", duration, err)
	return &device, err
}

// UpdateDevice updates an existing device
func (m *Manager) UpdateDevice(device *Device) error {
	start := time.Now()
	err := m.DB.Save(device).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("update", "devices", duration, err)
	return err
}

// DeleteDevice removes a device from the database
func (m *Manager) DeleteDevice(id uint) error {
	start := time.Now()
	err := m.DB.Delete(&Device{}, id).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("delete", "devices", duration, err)
	return err
}

// GetDeviceByMAC retrieves a device by MAC address
func (m *Manager) GetDeviceByMAC(mac string) (*Device, error) {
	start := time.Now()
	var device Device
	err := m.DB.Where("mac = ?", mac).First(&device).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("select", "devices", duration, err)
	return &device, err
}

// DiscoveryUpdate contains fields that should be updated during discovery
type DiscoveryUpdate struct {
	IP       string
	Type     string
	Firmware string
	Status   string
	LastSeen time.Time
}

// UpsertDeviceFromDiscovery updates or creates a device from discovery data
// Preserves existing: name, settings, sync status, created_at
// Updates: IP, type, firmware, status, last_seen
func (m *Manager) UpsertDeviceFromDiscovery(mac string, update DiscoveryUpdate, initialName string) (*Device, error) {
	start := time.Now()

	// Try to find existing device by MAC
	var device Device
	err := m.DB.Where("mac = ?", mac).First(&device).Error

	if err == gorm.ErrRecordNotFound {
		// Create new device
		device = Device{
			MAC:      mac,
			IP:       update.IP,
			Type:     update.Type,
			Name:     initialName,
			Firmware: update.Firmware,
			Status:   update.Status,
			LastSeen: update.LastSeen,
			Settings: "{}",
		}
		err = m.DB.Create(&device).Error
		m.logger.WithFields(map[string]any{
			"mac":       mac,
			"ip":        update.IP,
			"operation": "create",
			"component": "database",
		}).Info("Created new device from discovery")
	} else if err != nil {
		// Database error
		duration := time.Since(start).Microseconds()
		m.logger.LogDBOperation("upsert", "devices", duration, err)
		return nil, err
	} else {
		// Update existing device - only update discovery fields
		device.IP = update.IP
		device.Type = update.Type
		device.Firmware = update.Firmware
		device.Status = update.Status
		device.LastSeen = update.LastSeen
		// Note: Preserve Name, Settings, and other user-configured fields

		err = m.DB.Save(&device).Error
		m.logger.WithFields(map[string]any{
			"mac":       mac,
			"ip":        update.IP,
			"operation": "update",
			"preserved": "name,settings,sync_status",
			"component": "database",
		}).Info("Updated existing device from discovery")
	}

	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("upsert", "devices", duration, err)
	return &device, err
}

// AddDiscoveredDevice adds a new discovered device to the database
func (m *Manager) AddDiscoveredDevice(device *DiscoveredDevice) error {
	start := time.Now()
	err := m.DB.Create(device).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("insert", "discovered_devices", duration, err)
	return err
}

// GetDiscoveredDevices retrieves all discovered devices, optionally filtered by agent ID
func (m *Manager) GetDiscoveredDevices(agentID string) ([]DiscoveredDevice, error) {
	start := time.Now()
	var devices []DiscoveredDevice

	query := m.DB.Where("expires_at > ?", time.Now())
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}

	err := query.Order("discovered DESC").Find(&devices).Error
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("select", "discovered_devices", duration, err)
	return devices, err
}

// UpsertDiscoveredDevice creates or updates a discovered device (by MAC + AgentID)
func (m *Manager) UpsertDiscoveredDevice(device *DiscoveredDevice) error {
	start := time.Now()

	// Try to find existing record by MAC and AgentID
	var existing DiscoveredDevice
	err := m.DB.Where("mac = ? AND agent_id = ?", device.MAC, device.AgentID).First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new record
		err = m.DB.Create(device).Error
		m.logger.WithFields(map[string]any{
			"mac":       device.MAC,
			"agent_id":  device.AgentID,
			"model":     device.Model,
			"operation": "create",
			"component": "database",
		}).Info("Created new discovered device")
	} else if err != nil {
		// Database error
		duration := time.Since(start).Microseconds()
		m.logger.LogDBOperation("upsert", "discovered_devices", duration, err)
		return err
	} else {
		// Update existing record
		device.ID = existing.ID
		device.CreatedAt = existing.CreatedAt
		err = m.DB.Save(device).Error
		m.logger.WithFields(map[string]any{
			"mac":       device.MAC,
			"agent_id":  device.AgentID,
			"model":     device.Model,
			"operation": "update",
			"component": "database",
		}).Info("Updated existing discovered device")
	}

	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("upsert", "discovered_devices", duration, err)
	return err
}

// CleanupExpiredDiscoveredDevices removes discovered devices that have expired
func (m *Manager) CleanupExpiredDiscoveredDevices() (int64, error) {
	start := time.Now()
	result := m.DB.Where("expires_at <= ?", time.Now()).Delete(&DiscoveredDevice{})
	duration := time.Since(start).Microseconds()
	m.logger.LogDBOperation("delete", "discovered_devices", duration, result.Error)

	if result.RowsAffected > 0 {
		m.logger.WithFields(map[string]any{
			"deleted":   result.RowsAffected,
			"component": "database",
		}).Info("Cleaned up expired discovered devices")
	}

	return result.RowsAffected, result.Error
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.DB == nil {
		return nil
	}

	sqlDB, err := m.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying database: %w", err)
	}

	return sqlDB.Close()
}

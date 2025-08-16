package database

import (
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
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}
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

package database

import (
	"fmt"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&Device{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

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
package database

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Manager handles database operations for Shelly devices
type Manager struct {
	DB *gorm.DB
}

// NewManager creates a new database manager
func NewManager(dbPath string) (*Manager, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&Device{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Manager{DB: db}, nil
}

// AddDevice adds a new device to the database
func (m *Manager) AddDevice(device *Device) error {
	return m.DB.Create(device).Error
}

// GetDevices retrieves all devices from the database
func (m *Manager) GetDevices() ([]Device, error) {
	var devices []Device
	err := m.DB.Find(&devices).Error
	return devices, err
}

// GetDevice retrieves a specific device by ID
func (m *Manager) GetDevice(id uint) (*Device, error) {
	var device Device
	err := m.DB.First(&device, id).Error
	return &device, err
}

// UpdateDevice updates an existing device
func (m *Manager) UpdateDevice(device *Device) error {
	return m.DB.Save(device).Error
}

// DeleteDevice removes a device from the database
func (m *Manager) DeleteDevice(id uint) error {
	return m.DB.Delete(&Device{}, id).Error
}

// GetDeviceByMAC retrieves a device by MAC address
func (m *Manager) GetDeviceByMAC(mac string) (*Device, error) {
	var device Device
	err := m.DB.Where("mac = ?", mac).First(&device).Error
	return &device, err
}
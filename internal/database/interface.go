package database

import (
	"gorm.io/gorm"
)

// DatabaseInterface defines the contract for database operations
// This provides a clean abstraction for database operations
type DatabaseInterface interface {
	// Core operations
	GetDB() *gorm.DB
	Close() error

	// Device operations
	AddDevice(device *Device) error
	GetDevices() ([]Device, error)
	GetDevice(id uint) (*Device, error)
	UpdateDevice(device *Device) error
	DeleteDevice(id uint) error
	GetDeviceByMAC(mac string) (*Device, error)
	UpsertDeviceFromDiscovery(mac string, update DiscoveryUpdate, initialName string) (*Device, error)

	// Discovered device operations
	AddDiscoveredDevice(device *DiscoveredDevice) error
	GetDiscoveredDevices(agentID string) ([]DiscoveredDevice, error)
	UpsertDiscoveredDevice(device *DiscoveredDevice) error
	CleanupExpiredDiscoveredDevices() (int64, error)
}

// Ensure Manager implements the interface
var _ DatabaseInterface = (*Manager)(nil)

package database

import (
	"time"

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
	// A writer saves the columns it owns (ginsys/shelly-manager#572). The one save that writes more
	// than a few is the device PUT, UpdateDeviceIfVersion, and it is versioned on updated_at.
	UpdateDeviceIfVersion(device *Device, version time.Time) error
	RecordDeviceSeen(id uint, status string, lastSeen time.Time) error
	UpdateDeviceSettings(id uint, settings string) error
	UpdateDeviceDiscovery(id uint, update DiscoveryUpdate) error
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

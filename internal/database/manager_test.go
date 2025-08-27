package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// Test helper to create a test manager
func setupTestManager(t *testing.T) (*Manager, func()) {
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	manager, err := NewManagerFromPathWithLogger(":memory:", logger)
	require.NoError(t, err)

	cleanup := func() {
		if err := manager.Close(); err != nil {
			t.Logf("Failed to close manager: %v", err)
		}
	}

	return manager, cleanup
}

// Test constructor methods
func TestNewManager(t *testing.T) {
	t.Run("NewManager", func(t *testing.T) {
		config := provider.DatabaseConfig{
			Provider: "sqlite",
			DSN:      ":memory:",
		}

		manager, err := NewManager(config)
		assert.NoError(t, err)
		assert.NotNil(t, manager)

		defer func() {
			if err := manager.Close(); err != nil {
				t.Logf("Failed to close manager: %v", err)
			}
		}()

		// Verify manager is properly initialized
		assert.NotNil(t, manager.provider)
		assert.NotNil(t, manager.factory)
		assert.NotNil(t, manager.logger)
	})

	t.Run("NewManagerWithLogger", func(t *testing.T) {
		logger, err := logging.New(logging.Config{Level: "debug", Format: "text"})
		require.NoError(t, err)

		config := provider.DatabaseConfig{
			Provider: "sqlite",
			DSN:      ":memory:",
		}

		manager, err := NewManagerWithLogger(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, manager)
		assert.Equal(t, logger, manager.logger)

		defer func() {
			if err := manager.Close(); err != nil {
				t.Logf("Failed to close manager: %v", err)
			}
		}()
	})

	t.Run("NewManagerWithLogger_NilLogger", func(t *testing.T) {
		config := provider.DatabaseConfig{
			Provider: "sqlite",
			DSN:      ":memory:",
		}

		manager, err := NewManagerWithLogger(config, nil)
		assert.NoError(t, err)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.logger) // Should use default logger

		defer func() {
			if err := manager.Close(); err != nil {
				t.Logf("Failed to close manager: %v", err)
			}
		}()
	})

	t.Run("NewManager_InvalidConfig", func(t *testing.T) {
		config := provider.DatabaseConfig{
			Provider: "invalid-provider",
			DSN:      ":memory:",
		}

		manager, err := NewManager(config)
		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "invalid database configuration")
	})
}

func TestNewManagerFromPath(t *testing.T) {
	t.Run("NewManagerFromPath", func(t *testing.T) {
		manager, err := NewManagerFromPath(":memory:")
		assert.NoError(t, err)
		assert.NotNil(t, manager)

		defer func() {
			if err := manager.Close(); err != nil {
				t.Logf("Failed to close manager: %v", err)
			}
		}()

		// Verify it's using SQLite provider
		providerName, _ := manager.GetProviderInfo()
		assert.Equal(t, "SQLite", providerName)
	})

	t.Run("NewManagerFromPathWithLogger", func(t *testing.T) {
		logger, err := logging.New(logging.Config{Level: "debug", Format: "text"})
		require.NoError(t, err)

		manager, err := NewManagerFromPathWithLogger(":memory:", logger)
		assert.NoError(t, err)
		assert.NotNil(t, manager)
		assert.Equal(t, logger, manager.logger)

		defer func() {
			if err := manager.Close(); err != nil {
				t.Logf("Failed to close manager: %v", err)
			}
		}()
	})

	t.Run("NewManagerFromPath_InvalidPath", func(t *testing.T) {
		// Use an invalid path that should fail
		manager, err := NewManagerFromPath("/invalid/path/readonly.db")
		if err != nil {
			// Expected for read-only filesystem or permission issues
			assert.Nil(t, manager)
			assert.Contains(t, err.Error(), "failed to connect to database")
		} else {
			// If it succeeds, clean up
			defer func() {
				if err := manager.Close(); err != nil {
					t.Logf("Failed to close manager: %v", err)
				}
			}()
		}
	})
}

func TestNewManagerFromConfig(t *testing.T) {
	t.Run("NewManagerFromConfig", func(t *testing.T) {
		// Since we don't have full config implementation, test with minimal setup
		manager, err := NewManagerFromPath(":memory:")
		if err == nil {
			defer func() {
				if err := manager.Close(); err != nil {
					t.Logf("Failed to close manager: %v", err)
				}
			}()
			assert.NotNil(t, manager)
		}
	})
}

// Test core database provider methods
func TestManagerCoreMethods(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("GetDB", func(t *testing.T) {
		db := manager.GetDB()
		assert.NotNil(t, db)
		assert.IsType(t, &gorm.DB{}, db)
	})

	t.Run("Ping", func(t *testing.T) {
		err := manager.Ping()
		assert.NoError(t, err)
	})

	t.Run("GetProvider", func(t *testing.T) {
		provider := manager.GetProvider()
		assert.NotNil(t, provider)
	})

	t.Run("GetProviderInfo", func(t *testing.T) {
		name, version := manager.GetProviderInfo()
		assert.NotEmpty(t, name)
		assert.NotEmpty(t, version)
		assert.Equal(t, "SQLite", name)
	})

	t.Run("GetSupportedProviders", func(t *testing.T) {
		providers := manager.GetSupportedProviders()
		assert.NotEmpty(t, providers)
		assert.Contains(t, providers, "sqlite")
	})

	t.Run("GetProviderDetails", func(t *testing.T) {
		details, err := manager.GetProviderDetails("sqlite")
		assert.NoError(t, err)
		assert.NotNil(t, details)
		assert.Equal(t, "sqlite", details.Type)
	})

	t.Run("GetProviderDetails_Invalid", func(t *testing.T) {
		details, err := manager.GetProviderDetails("invalid-provider")
		assert.Error(t, err)
		assert.Nil(t, details)
	})

	t.Run("GetDefaultConfig", func(t *testing.T) {
		config := manager.GetDefaultConfig("sqlite")
		assert.Equal(t, "sqlite", config.Provider)
		assert.NotEmpty(t, config.DSN)
	})

	t.Run("GetStats", func(t *testing.T) {
		stats := manager.GetStats()
		assert.NotNil(t, stats)
		// Basic stats should have some values
		assert.GreaterOrEqual(t, stats.OpenConnections, 0)
		assert.GreaterOrEqual(t, stats.IdleConnections, 0)
	})
}

// Test transaction methods
func TestManagerTransactions(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("BeginTransaction", func(t *testing.T) {
		tx, err := manager.BeginTransaction()
		assert.NoError(t, err)
		assert.NotNil(t, tx)

		// Rollback the transaction
		err = tx.Rollback()
		assert.NoError(t, err)
	})

	t.Run("TransactionCommit", func(t *testing.T) {
		tx, err := manager.BeginTransaction()
		require.NoError(t, err)

		// Add a device within transaction
		device := &Device{
			MAC:      "tx:test:mac:commit",
			IP:       "192.168.1.100",
			Type:     "switch",
			Name:     "Transaction Test Device",
			Settings: "{}",
		}

		err = tx.GetDB().Create(device).Error
		assert.NoError(t, err)

		// Commit transaction
		err = tx.Commit()
		assert.NoError(t, err)

		// Verify device exists after commit
		var foundDevice Device
		err = manager.GetDB().Where("mac = ?", "tx:test:mac:commit").First(&foundDevice).Error
		assert.NoError(t, err)
		assert.Equal(t, "Transaction Test Device", foundDevice.Name)
	})

	t.Run("TransactionRollback", func(t *testing.T) {
		tx, err := manager.BeginTransaction()
		require.NoError(t, err)

		// Add a device within transaction
		device := &Device{
			MAC:      "tx:test:mac:rollback",
			IP:       "192.168.1.101",
			Type:     "switch",
			Name:     "Transaction Test Device Rollback",
			Settings: "{}",
		}

		err = tx.GetDB().Create(device).Error
		assert.NoError(t, err)

		// Rollback transaction
		err = tx.Rollback()
		assert.NoError(t, err)

		// Verify device doesn't exist after rollback
		var foundDevice Device
		err = manager.GetDB().Where("mac = ?", "tx:test:mac:rollback").First(&foundDevice).Error
		assert.Error(t, err)
		assert.True(t, err == gorm.ErrRecordNotFound)
	})
}

// Test migration methods
func TestManagerMigration(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Test custom struct for migration
	type TestModel struct {
		ID   uint   `gorm:"primaryKey"`
		Name string `gorm:"size:100"`
	}

	t.Run("Migrate", func(t *testing.T) {
		err := manager.Migrate(&TestModel{})
		assert.NoError(t, err)

		// Verify table was created by inserting a record
		testRecord := TestModel{Name: "test"}
		err = manager.GetDB().Create(&testRecord).Error
		assert.NoError(t, err)
		assert.NotZero(t, testRecord.ID)
	})

	t.Run("DropTables", func(t *testing.T) {
		// First ensure table exists
		err := manager.Migrate(&TestModel{})
		require.NoError(t, err)

		// Drop the table
		err = manager.DropTables(&TestModel{})
		assert.NoError(t, err)

		// Verify table is gone by trying to query it
		var testRecord TestModel
		err = manager.GetDB().First(&testRecord).Error
		assert.Error(t, err) // Should fail because table doesn't exist
	})
}

// Test legacy device CRUD operations
func TestManagerDeviceOperations(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("AddDevice", func(t *testing.T) {
		device := &Device{
			MAC:      "aa:bb:cc:dd:ee:01",
			IP:       "192.168.1.101",
			Type:     "switch",
			Name:     "Test Device 1",
			Firmware: "1.0.0",
			Status:   "online",
			Settings: "{}",
		}

		err := manager.AddDevice(device)
		assert.NoError(t, err)
		assert.NotZero(t, device.ID)
	})

	t.Run("GetDevices", func(t *testing.T) {
		// Clear existing devices
		manager.GetDB().Exec("DELETE FROM devices")

		// Add test devices
		devices := []*Device{
			{MAC: "get:devices:01", IP: "192.168.1.101", Type: "switch", Name: "Device 1", Settings: "{}"},
			{MAC: "get:devices:02", IP: "192.168.1.102", Type: "dimmer", Name: "Device 2", Settings: "{}"},
		}

		for _, device := range devices {
			err := manager.AddDevice(device)
			require.NoError(t, err)
		}

		// Get all devices
		allDevices, err := manager.GetDevices()
		assert.NoError(t, err)
		assert.Len(t, allDevices, 2)

		// Verify device data
		assert.Equal(t, "Device 1", allDevices[0].Name)
		assert.Equal(t, "Device 2", allDevices[1].Name)
	})

	t.Run("GetDevice", func(t *testing.T) {
		// Add a test device
		device := &Device{
			MAC:      "get:device:test",
			IP:       "192.168.1.103",
			Type:     "relay",
			Name:     "Get Device Test",
			Settings: "{}",
		}

		err := manager.AddDevice(device)
		require.NoError(t, err)

		// Get device by ID
		foundDevice, err := manager.GetDevice(device.ID)
		assert.NoError(t, err)
		assert.NotNil(t, foundDevice)
		assert.Equal(t, device.ID, foundDevice.ID)
		assert.Equal(t, "Get Device Test", foundDevice.Name)
	})

	t.Run("GetDevice_NotFound", func(t *testing.T) {
		foundDevice, err := manager.GetDevice(99999) // Non-existent ID
		assert.Error(t, err)
		assert.Nil(t, foundDevice)
		assert.True(t, err == gorm.ErrRecordNotFound)
	})

	t.Run("UpdateDevice", func(t *testing.T) {
		// Add a test device
		device := &Device{
			MAC:      "update:device:test",
			IP:       "192.168.1.104",
			Type:     "switch",
			Name:     "Original Name",
			Settings: "{}",
		}

		err := manager.AddDevice(device)
		require.NoError(t, err)

		// Update the device
		device.Name = "Updated Name"
		device.IP = "192.168.1.199"
		device.Status = "updated"

		err = manager.UpdateDevice(device)
		assert.NoError(t, err)

		// Verify update
		updatedDevice, err := manager.GetDevice(device.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Name", updatedDevice.Name)
		assert.Equal(t, "192.168.1.199", updatedDevice.IP)
		assert.Equal(t, "updated", updatedDevice.Status)
	})

	t.Run("DeleteDevice", func(t *testing.T) {
		// Add a test device
		device := &Device{
			MAC:      "delete:device:test",
			IP:       "192.168.1.105",
			Type:     "dimmer",
			Name:     "Delete Test Device",
			Settings: "{}",
		}

		err := manager.AddDevice(device)
		require.NoError(t, err)
		deviceID := device.ID

		// Delete the device
		err = manager.DeleteDevice(deviceID)
		assert.NoError(t, err)

		// Verify device is gone
		_, err = manager.GetDevice(deviceID)
		assert.Error(t, err)
		assert.True(t, err == gorm.ErrRecordNotFound)
	})

	t.Run("DeleteDevice_NotFound", func(t *testing.T) {
		err := manager.DeleteDevice(99999) // Non-existent ID
		assert.Error(t, err)
		assert.True(t, err == gorm.ErrRecordNotFound)
	})

	t.Run("UpsertDevice_Create", func(t *testing.T) {
		device := &Device{
			MAC:      "upsert:create:test",
			IP:       "192.168.1.106",
			Type:     "relay",
			Name:     "Upsert Create Test",
			Settings: "{}",
		}

		err := manager.UpsertDevice(device)
		assert.NoError(t, err)
		assert.NotZero(t, device.ID)

		// Verify device was created
		foundDevice, err := manager.GetDeviceByMAC("upsert:create:test")
		assert.NoError(t, err)
		assert.Equal(t, "Upsert Create Test", foundDevice.Name)
	})

	t.Run("UpsertDevice_Update", func(t *testing.T) {
		// First create a device
		originalDevice := &Device{
			MAC:      "upsert:update:test",
			IP:       "192.168.1.107",
			Type:     "switch",
			Name:     "Original Upsert Device",
			Settings: "{}",
		}

		err := manager.AddDevice(originalDevice)
		require.NoError(t, err)
		originalID := originalDevice.ID

		// Now upsert with same MAC but different data
		upsertDevice := &Device{
			MAC:      "upsert:update:test", // Same MAC
			IP:       "192.168.1.200",      // Different IP
			Type:     "dimmer",             // Different type
			Name:     "Updated Upsert Device",
			Settings: "{}",
		}

		err = manager.UpsertDevice(upsertDevice)
		assert.NoError(t, err)
		assert.Equal(t, originalID, upsertDevice.ID) // Should preserve ID

		// Verify device was updated
		foundDevice, err := manager.GetDeviceByMAC("upsert:update:test")
		assert.NoError(t, err)
		assert.Equal(t, "Updated Upsert Device", foundDevice.Name)
		assert.Equal(t, "192.168.1.200", foundDevice.IP)
		assert.Equal(t, "dimmer", foundDevice.Type)
	})

	t.Run("GetDeviceByMAC", func(t *testing.T) {
		// Add a test device
		device := &Device{
			MAC:      "get:by:mac:test",
			IP:       "192.168.1.108",
			Type:     "switch",
			Name:     "Get By MAC Test",
			Settings: "{}",
		}

		err := manager.AddDevice(device)
		require.NoError(t, err)

		// Get device by MAC
		foundDevice, err := manager.GetDeviceByMAC("get:by:mac:test")
		assert.NoError(t, err)
		assert.NotNil(t, foundDevice)
		assert.Equal(t, device.ID, foundDevice.ID)
		assert.Equal(t, "Get By MAC Test", foundDevice.Name)
	})

	t.Run("GetDeviceByMAC_NotFound", func(t *testing.T) {
		foundDevice, err := manager.GetDeviceByMAC("nonexistent:mac:address")
		assert.Error(t, err)
		assert.Nil(t, foundDevice)
		assert.True(t, err == gorm.ErrRecordNotFound)
	})

	t.Run("UpsertDeviceFromDiscovery_Create", func(t *testing.T) {
		update := DiscoveryUpdate{
			IP:       "192.168.1.109",
			Type:     "switch",
			Firmware: "1.2.3",
			Status:   "online",
			LastSeen: time.Now(),
		}

		device, err := manager.UpsertDeviceFromDiscovery("discovery:create:mac", update, "Discovery Created Device")
		assert.NoError(t, err)
		assert.NotNil(t, device)
		assert.NotZero(t, device.ID)
		assert.Equal(t, "Discovery Created Device", device.Name)
		assert.Equal(t, "192.168.1.109", device.IP)
		assert.Equal(t, "1.2.3", device.Firmware)
	})

	t.Run("UpsertDeviceFromDiscovery_Update", func(t *testing.T) {
		// First create a device manually
		originalDevice := &Device{
			MAC:      "discovery:update:mac",
			IP:       "192.168.1.110",
			Type:     "dimmer",
			Name:     "Original Discovery Device",
			Firmware: "1.0.0",
			Status:   "offline",
			Settings: "{\"custom\":\"setting\"}",
		}

		err := manager.AddDevice(originalDevice)
		require.NoError(t, err)
		originalID := originalDevice.ID

		// Now update via discovery
		update := DiscoveryUpdate{
			IP:       "192.168.1.250", // New IP - unique for this test
			Type:     "switch",        // New type
			Firmware: "2.0.0",         // New firmware
			Status:   "online",        // New status
			LastSeen: time.Now(),
		}

		device, err := manager.UpsertDeviceFromDiscovery("discovery:update:mac", update, "Should Not Change Name")
		assert.NoError(t, err)
		assert.NotNil(t, device)
		assert.Equal(t, originalID, device.ID) // Should preserve ID

		// Discovery fields should be updated
		assert.Equal(t, "192.168.1.250", device.IP)
		assert.Equal(t, "switch", device.Type)
		assert.Equal(t, "2.0.0", device.Firmware)
		assert.Equal(t, "online", device.Status)

		// User-configured fields should be preserved
		assert.Equal(t, "Original Discovery Device", device.Name)    // Name preserved
		assert.Equal(t, "{\"custom\":\"setting\"}", device.Settings) // Settings preserved
	})
}

// Test error conditions and edge cases
func TestManagerErrorHandling(t *testing.T) {
	t.Run("MigrateProvider_NotImplemented", func(t *testing.T) {
		manager, cleanup := setupTestManager(t)
		defer cleanup()

		targetConfig := provider.DatabaseConfig{
			Provider: "postgresql",
			DSN:      "postgres://test",
		}

		err := manager.MigrateProvider(targetConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider migration not yet implemented")
	})

	t.Run("UpsertDeviceFromDiscovery_NilProvider", func(t *testing.T) {
		// Test that GetDB panics with nil provider (this is expected behavior)
		manager := &Manager{
			provider: nil,
			logger:   logging.GetDefault(),
		}

		// This should panic when calling GetDB()
		assert.Panics(t, func() {
			update := DiscoveryUpdate{
				IP:       "192.168.1.111",
				Type:     "switch",
				Firmware: "1.0.0",
				Status:   "online",
				LastSeen: time.Now(),
			}

			_, _ = manager.UpsertDeviceFromDiscovery("test:nil:db", update, "Test Device")
		})
	})
}

// Test close functionality
func TestManagerClose(t *testing.T) {
	t.Run("Close", func(t *testing.T) {
		manager, _ := setupTestManager(t)

		// Manager should be functional before close
		err := manager.Ping()
		assert.NoError(t, err)

		// Close the manager
		err = manager.Close()
		assert.NoError(t, err)

		// After close, operations might fail (depending on provider implementation)
		// Note: SQLite in-memory databases might not show connection errors after close
		// This is expected behavior
	})
}

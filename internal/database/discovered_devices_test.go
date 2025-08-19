package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestDiscoveredDeviceOperations(t *testing.T) {
	// Setup test database
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	manager, err := NewManagerWithLogger(":memory:", logger)
	require.NoError(t, err)
	defer manager.Close()

	t.Run("AddDiscoveredDevice", func(t *testing.T) {
		device := &DiscoveredDevice{
			MAC:        "aa:bb:cc:dd:ee:ff",
			SSID:       "shellyplus1-112233",
			Model:      "SNSW-001X16EU",
			Generation: 2,
			IP:         "192.168.33.1",
			Signal:     -45,
			AgentID:    "agent-test-1",
			TaskID:     "task-123",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}

		err := manager.AddDiscoveredDevice(device)
		assert.NoError(t, err)
		assert.NotZero(t, device.ID, "Device ID should be set after creation")
	})

	t.Run("UpsertDiscoveredDevice_Create", func(t *testing.T) {
		device := &DiscoveredDevice{
			MAC:        "11:22:33:44:55:66",
			SSID:       "shelly1-aabbcc",
			Model:      "SHSW-1",
			Generation: 1,
			IP:         "192.168.33.2",
			Signal:     -55,
			AgentID:    "agent-test-2",
			TaskID:     "task-456",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}

		err := manager.UpsertDiscoveredDevice(device)
		assert.NoError(t, err)
		assert.NotZero(t, device.ID, "Device ID should be set after creation")
	})

	t.Run("UpsertDiscoveredDevice_Update", func(t *testing.T) {
		// First create a device
		device := &DiscoveredDevice{
			MAC:        "99:88:77:66:55:44",
			SSID:       "shellyplus1-ddeeff",
			Model:      "SNSW-001X16EU",
			Generation: 2,
			IP:         "192.168.33.3",
			Signal:     -40,
			AgentID:    "agent-test-3",
			TaskID:     "task-789",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}

		err := manager.UpsertDiscoveredDevice(device)
		require.NoError(t, err)
		originalID := device.ID

		// Update the same device with different IP and signal
		device.IP = "192.168.33.99"
		device.Signal = -65
		device.Discovered = time.Now()
		device.ExpiresAt = time.Now().Add(12 * time.Hour)

		err = manager.UpsertDiscoveredDevice(device)
		assert.NoError(t, err)
		assert.Equal(t, originalID, device.ID, "Device ID should remain the same after update")

		// Verify the device was updated
		devices, err := manager.GetDiscoveredDevices("agent-test-3")
		assert.NoError(t, err)
		assert.Len(t, devices, 1)
		assert.Equal(t, "192.168.33.99", devices[0].IP)
		assert.Equal(t, -65, devices[0].Signal)
	})

	t.Run("GetDiscoveredDevices_All", func(t *testing.T) {
		// Clear any existing data
		manager.DB.Exec("DELETE FROM discovered_devices")

		// Add multiple devices
		devices := []*DiscoveredDevice{
			{
				MAC:        "aa:bb:cc:11:22:33",
				SSID:       "shellyplus1-112233",
				Model:      "SNSW-001X16EU",
				Generation: 2,
				IP:         "192.168.33.10",
				Signal:     -45,
				AgentID:    "agent-1",
				Discovered: time.Now(),
				ExpiresAt:  time.Now().Add(24 * time.Hour),
			},
			{
				MAC:        "bb:cc:dd:44:55:66",
				SSID:       "shelly1-445566",
				Model:      "SHSW-1",
				Generation: 1,
				IP:         "192.168.33.11",
				Signal:     -50,
				AgentID:    "agent-2",
				Discovered: time.Now(),
				ExpiresAt:  time.Now().Add(24 * time.Hour),
			},
		}

		for _, device := range devices {
			err := manager.AddDiscoveredDevice(device)
			require.NoError(t, err)
		}

		// Get all devices (no agent filter)
		allDevices, err := manager.GetDiscoveredDevices("")
		assert.NoError(t, err)
		assert.Len(t, allDevices, 2)
	})

	t.Run("GetDiscoveredDevices_FilteredByAgent", func(t *testing.T) {
		// Clear any existing data
		manager.DB.Exec("DELETE FROM discovered_devices")

		// Add devices for different agents
		devices := []*DiscoveredDevice{
			{
				MAC:        "aa:bb:cc:11:22:33",
				AgentID:    "agent-filtered-1",
				Discovered: time.Now(),
				ExpiresAt:  time.Now().Add(24 * time.Hour),
			},
			{
				MAC:        "bb:cc:dd:44:55:66",
				AgentID:    "agent-filtered-2",
				Discovered: time.Now(),
				ExpiresAt:  time.Now().Add(24 * time.Hour),
			},
			{
				MAC:        "cc:dd:ee:77:88:99",
				AgentID:    "agent-filtered-1",
				Discovered: time.Now(),
				ExpiresAt:  time.Now().Add(24 * time.Hour),
			},
		}

		for _, device := range devices {
			err := manager.AddDiscoveredDevice(device)
			require.NoError(t, err)
		}

		// Get devices for specific agent
		agent1Devices, err := manager.GetDiscoveredDevices("agent-filtered-1")
		assert.NoError(t, err)
		assert.Len(t, agent1Devices, 2)

		agent2Devices, err := manager.GetDiscoveredDevices("agent-filtered-2")
		assert.NoError(t, err)
		assert.Len(t, agent2Devices, 1)
	})

	t.Run("GetDiscoveredDevices_ExcludesExpired", func(t *testing.T) {
		// Clear any existing data
		manager.DB.Exec("DELETE FROM discovered_devices")

		now := time.Now()

		// Add one valid and one expired device
		validDevice := &DiscoveredDevice{
			MAC:        "valid:device:mac",
			AgentID:    "test-agent",
			Discovered: now,
			ExpiresAt:  now.Add(1 * time.Hour), // Valid for 1 hour
		}

		expiredDevice := &DiscoveredDevice{
			MAC:        "expired:device:mac",
			AgentID:    "test-agent",
			Discovered: now.Add(-2 * time.Hour),
			ExpiresAt:  now.Add(-1 * time.Hour), // Expired 1 hour ago
		}

		err := manager.AddDiscoveredDevice(validDevice)
		require.NoError(t, err)
		err = manager.AddDiscoveredDevice(expiredDevice)
		require.NoError(t, err)

		// GetDiscoveredDevices should only return valid devices
		devices, err := manager.GetDiscoveredDevices("test-agent")
		assert.NoError(t, err)
		assert.Len(t, devices, 1)
		assert.Equal(t, "valid:device:mac", devices[0].MAC)
	})

	t.Run("CleanupExpiredDiscoveredDevices", func(t *testing.T) {
		// Clear any existing data
		manager.DB.Exec("DELETE FROM discovered_devices")

		now := time.Now()

		// Add devices with different expiration times
		devices := []*DiscoveredDevice{
			{
				MAC:        "valid:device:1",
				AgentID:    "test-agent",
				Discovered: now,
				ExpiresAt:  now.Add(1 * time.Hour), // Valid
			},
			{
				MAC:        "expired:device:1",
				AgentID:    "test-agent",
				Discovered: now.Add(-3 * time.Hour),
				ExpiresAt:  now.Add(-1 * time.Hour), // Expired
			},
			{
				MAC:        "expired:device:2",
				AgentID:    "test-agent",
				Discovered: now.Add(-4 * time.Hour),
				ExpiresAt:  now.Add(-2 * time.Hour), // Expired
			},
		}

		for _, device := range devices {
			err := manager.AddDiscoveredDevice(device)
			require.NoError(t, err)
		}

		// Cleanup expired devices
		deleted, err := manager.CleanupExpiredDiscoveredDevices()
		assert.NoError(t, err)
		assert.Equal(t, int64(2), deleted, "Should delete 2 expired devices")

		// Verify only valid device remains
		remainingDevices, err := manager.GetDiscoveredDevices("")
		assert.NoError(t, err)
		assert.Len(t, remainingDevices, 1)
		assert.Equal(t, "valid:device:1", remainingDevices[0].MAC)
	})

	t.Run("CleanupExpiredDiscoveredDevices_NoExpired", func(t *testing.T) {
		// Clear any existing data
		manager.DB.Exec("DELETE FROM discovered_devices")

		// Add only valid devices
		device := &DiscoveredDevice{
			MAC:        "valid:device:only",
			AgentID:    "test-agent",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}

		err := manager.AddDiscoveredDevice(device)
		require.NoError(t, err)

		// Cleanup should delete nothing
		deleted, err := manager.CleanupExpiredDiscoveredDevices()
		assert.NoError(t, err)
		assert.Equal(t, int64(0), deleted, "Should delete 0 devices")

		// Verify device still exists
		validDevices, err := manager.GetDiscoveredDevices("")
		assert.NoError(t, err)
		assert.Len(t, validDevices, 1)
	})
}

func TestDiscoveredDeviceValidation(t *testing.T) {
	// Setup test database
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	manager, err := NewManagerWithLogger(":memory:", logger)
	require.NoError(t, err)
	defer manager.Close()

	t.Run("RequiredFields", func(t *testing.T) {
		// Test that empty MAC or AgentID can still be added (GORM behavior)
		device := &DiscoveredDevice{
			MAC:        "", // Empty MAC
			AgentID:    "", // Empty AgentID
			SSID:       "test-ssid",
			Model:      "test-model",
			Generation: 1,
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}

		err := manager.AddDiscoveredDevice(device)
		assert.NoError(t, err, "GORM allows empty strings by default")
		assert.NotZero(t, device.ID)
	})

	t.Run("ValidCompleteDevice", func(t *testing.T) {
		device := &DiscoveredDevice{
			MAC:        "valid:complete:device",
			SSID:       "shellyplus1-test",
			Model:      "SNSW-001X16EU",
			Generation: 2,
			IP:         "192.168.1.100",
			Signal:     -45,
			AgentID:    "valid-agent",
			TaskID:     "optional-task",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}

		err := manager.AddDiscoveredDevice(device)
		assert.NoError(t, err, "Should succeed with all valid fields")
		assert.NotZero(t, device.ID)
	})
}

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
)

func TestGetClient_EmptySettings(t *testing.T) {
	// Create test database
	db, err := database.NewManager(":memory:")
	require.NoError(t, err)

	// Create service
	service := NewService(db, nil)

	// Create device with empty settings
	device := &database.Device{
		ID:       1,
		IP:       "192.0.2.100", // TEST-NET address
		MAC:      "AA:BB:CC:DD:EE:FF",
		Type:     "Test Device",
		Name:     "Test",
		Settings: "", // Empty settings - this was causing the error
	}

	// Add device to database
	err = db.AddDevice(device)
	require.NoError(t, err)

	// Try to get client - this should not fail anymore
	client, err := service.getClient(device)

	// Should succeed with default settings
	require.NoError(t, err, "getClient should handle empty settings gracefully")
	require.NotNil(t, client, "Client should be created even with empty settings")
}

func TestGetClient_InvalidSettings(t *testing.T) {
	// Create test database
	db, err := database.NewManager(":memory:")
	require.NoError(t, err)

	// Create service
	service := NewService(db, nil)

	// Create device with invalid JSON settings
	device := &database.Device{
		ID:       2,
		IP:       "192.0.2.101", // TEST-NET address
		MAC:      "AA:BB:CC:DD:EE:F0",
		Type:     "Test Device",
		Name:     "Test",
		Settings: `{"model":"SHPLG-S","gen":1,}`, // Invalid JSON with trailing comma
	}

	// Add device to database
	err = db.AddDevice(device)
	require.NoError(t, err)

	// Try to get client - this should fail with a clear error
	client, err := service.getClient(device)

	// Should fail with JSON parsing error
	require.Error(t, err, "getClient should fail with invalid JSON settings")
	require.Nil(t, client, "Client should be nil on error")
	assert.Contains(t, err.Error(), "failed to parse device settings", "Error should mention parsing failure")
}

func TestGetClient_ValidSettings(t *testing.T) {
	// Create test database
	db, err := database.NewManager(":memory:")
	require.NoError(t, err)

	// Create service
	service := NewService(db, nil)

	// Create device with valid settings
	device := &database.Device{
		ID:       3,
		IP:       "192.0.2.102", // TEST-NET address
		MAC:      "AA:BB:CC:DD:EE:F1",
		Type:     "Test Device",
		Name:     "Test",
		Settings: `{"model":"SHPLG-S","gen":1,"auth_enabled":false}`,
	}

	// Add device to database
	err = db.AddDevice(device)
	require.NoError(t, err)

	// Try to get client
	client, err := service.getClient(device)

	// Should succeed
	require.NoError(t, err, "getClient should succeed with valid settings")
	require.NotNil(t, client, "Client should be created with valid settings")
}

func TestGetClient_AuthEnabledSettings(t *testing.T) {
	// Create test database
	db, err := database.NewManager(":memory:")
	require.NoError(t, err)

	// Create service
	service := NewService(db, nil)

	// Create device with auth enabled
	device := &database.Device{
		ID:       4,
		IP:       "192.0.2.103", // TEST-NET address
		MAC:      "AA:BB:CC:DD:EE:F2",
		Type:     "Test Device",
		Name:     "Test",
		Settings: `{"model":"SHPLG-S","gen":1,"auth_enabled":true,"auth_user":"admin","auth_pass":"password"}`,
	}

	// Add device to database
	err = db.AddDevice(device)
	require.NoError(t, err)

	// Try to get client
	client, err := service.getClient(device)

	// Should succeed
	require.NoError(t, err, "getClient should succeed with auth settings")
	require.NotNil(t, client, "Client should be created with auth settings")
}

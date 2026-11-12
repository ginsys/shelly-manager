package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// RecordDeviceSeen stores what a status read observed. It must not move updated_at, because a
// client sends updated_at back as the version of the device it edited (ginsys/shelly-manager#502).
func TestRecordDeviceSeen_StoresTheObservationAndKeepsTheVersion(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{MAC: "AA:BB:CC:DD:EE:01", IP: "192.0.2.1", Name: "lamp", Status: "offline"}
	require.NoError(t, manager.AddDevice(device))
	before, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	// A save moves updated_at to the clock, so wait out the clock's resolution.
	time.Sleep(20 * time.Millisecond)

	seen := time.Now()
	require.NoError(t, manager.RecordDeviceSeen(device.ID, "online", seen))

	after, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "online", after.Status)
	assert.WithinDuration(t, seen, after.LastSeen, time.Second)
	assert.True(t, after.UpdatedAt.Equal(before.UpdatedAt), "updated_at moved from %v to %v", before.UpdatedAt, after.UpdatedAt)
	assert.Equal(t, before.Name, after.Name)
	assert.Equal(t, before.IP, after.IP)
}

func TestRecordDeviceSeen_UnknownDeviceIsNotFound(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	err := manager.RecordDeviceSeen(4242, "online", time.Now())
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

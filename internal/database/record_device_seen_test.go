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

// MySQL reports zero affected rows when the new values equal the stored ones. The same
// observation twice must not look like a missing device. The callback below makes the update
// report zero rows, as MySQL does.
func TestRecordDeviceSeen_AnUnchangedObservationIsNotAMissingDevice(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{MAC: "AA:BB:CC:DD:EE:02", IP: "192.0.2.2", Name: "plug", Status: "offline"}
	require.NoError(t, manager.AddDevice(device))
	seen := time.Now()
	require.NoError(t, manager.RecordDeviceSeen(device.ID, "online", seen))

	const name = "test:report-zero-rows-for-an-unchanged-update"
	require.NoError(t, manager.GetDB().Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
		tx.RowsAffected = 0
	}))
	defer func() { _ = manager.GetDB().Callback().Update().Remove(name) }()

	assert.NoError(t, manager.RecordDeviceSeen(device.ID, "online", seen))
	assert.ErrorIs(t, manager.RecordDeviceSeen(4242, "online", seen), gorm.ErrRecordNotFound)
}

func TestRecordDeviceSeen_UnknownDeviceIsNotFound(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	err := manager.RecordDeviceSeen(4242, "online", time.Now())
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// afterFirstDeviceRead runs hook once, right after the first query on the devices table returns:
// the instant between a writer's read of the row and its write. hook gets a session on the same
// connection, so it also works inside a transaction. The callback is removed when the test ends.
func afterFirstDeviceRead(t *testing.T, db *gorm.DB, hook func(tx *gorm.DB)) {
	t.Helper()
	const name = "test:after-first-device-read"
	fired := false
	require.NoError(t, db.Callback().Query().After("gorm:after_query").Register(name, func(d *gorm.DB) {
		if fired || d.Statement.Table != "devices" {
			return
		}
		fired = true
		hook(d.Session(&gorm.Session{NewDB: true}))
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
}

// The discovery upsert read the row and saved it whole, so another writer's config_applied,
// desired_config, name and settings, committed after that read, were written back as they were read
// (ginsys/shelly-manager#572). It now writes the discovery columns and no other.
func TestUpsertDeviceFromDiscovery_KeepsTheOtherWritersColumns(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC: "AA:BB:CC:DD:EE:31", IP: "192.0.2.31", Name: "lamp", Type: "SHPLG-S", Firmware: "1.0",
		Status: "offline", LastSeen: time.Now().Add(-time.Hour), Settings: `{"a":1}`,
		DesiredConfig: `{"v":1}`, ConfigApplied: true,
	}
	require.NoError(t, manager.AddDevice(device))
	before, err := manager.GetDevice(device.ID)
	require.NoError(t, err)

	afterFirstDeviceRead(t, manager.GetDB(), func(tx *gorm.DB) {
		require.NoError(t, tx.Model(&Device{}).Where("id = ?", device.ID).UpdateColumns(map[string]any{
			"config_applied": false, "desired_config": `{"v":2}`, "name": "renamed", "settings": `{"a":2}`,
		}).Error)
		require.NoError(t, configuration.NewGormConfigRepository(tx, nil).SetDevicePendingReboot(device.ID, true))
	})
	time.Sleep(5 * time.Millisecond)

	seen := time.Now().Truncate(time.Second)
	returned, _, err := manager.UpsertDeviceFromDiscovery(device.MAC,
		DiscoveryUpdate{IP: "192.0.2.32", Type: "SHPLG-S2", Firmware: "1.1", Status: "online", LastSeen: seen}, "ignored")
	require.NoError(t, err)

	got, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.32", got.IP)
	assert.Equal(t, "SHPLG-S2", got.Type)
	assert.Equal(t, "1.1", got.Firmware)
	assert.Equal(t, "online", got.Status)
	assert.WithinDuration(t, seen, got.LastSeen, time.Second)
	assert.False(t, got.ConfigApplied, "config_applied was written over")
	assert.Equal(t, `{"v":2}`, got.DesiredConfig, "desired_config was written over")
	assert.True(t, got.PendingReboot, "pending_reboot was written over")
	assert.Equal(t, "renamed", got.Name, "name was written over")
	assert.Equal(t, `{"a":2}`, got.Settings, "settings were written over")
	assert.True(t, got.UpdatedAt.After(before.UpdatedAt), "updated_at did not move")

	// The returned row is the stored one, not the row that was read before the other writer ran.
	assert.Equal(t, "renamed", returned.Name)
	assert.Equal(t, "192.0.2.32", returned.IP)
	assert.Equal(t, got.UpdatedAt, returned.UpdatedAt)
}

// A discovery that cannot name the model reports no type. The stored type stays: the old upsert
// wrote the empty type over it.
func TestUpsertDeviceFromDiscovery_AnEmptyTypeLeavesTheStoredType(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC: "AA:BB:CC:DD:EE:32", IP: "192.0.2.33", Name: "lamp", Type: "SHPLG-S", Firmware: "1.0",
		Status: "offline", LastSeen: time.Now().Add(-time.Hour), Settings: "{}",
	}
	require.NoError(t, manager.AddDevice(device))

	_, _, err := manager.UpsertDeviceFromDiscovery(device.MAC,
		DiscoveryUpdate{IP: "192.0.2.34", Firmware: "1.2", Status: "online", LastSeen: time.Now()}, "ignored")
	require.NoError(t, err)

	got, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "SHPLG-S", got.Type)
	assert.Equal(t, "192.0.2.34", got.IP)
	assert.Equal(t, "1.2", got.Firmware)
}

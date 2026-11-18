package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// UpdateDeviceSettings and UpdateDeviceDiscovery write the columns they name and no other
// (ginsys/shelly-manager#565). A caller holds a row it read earlier, and another writer may have
// changed config_applied or desired_config since: the stale values must never reach the row.
// checkColumnScopedWrites holds the cases, and any provider can run it.
func checkColumnScopedWrites(t *testing.T, manager *Manager) {
	t.Helper()

	device := &Device{
		MAC: "AA:BB:CC:DD:EE:21", IP: "192.0.2.21", Name: "lamp", Type: "SHPLG-S", Firmware: "1.0",
		Status: "offline", LastSeen: time.Now().Add(-time.Hour), Settings: `{"a":1}`,
		DesiredConfig: `{"v":1}`, ConfigApplied: true,
	}
	require.NoError(t, manager.AddDevice(device))
	stale, err := manager.GetDevice(device.ID)
	require.NoError(t, err)

	// Another writer changes the columns that these methods do not own, after the read above.
	require.NoError(t, manager.GetDB().Model(&Device{}).Where("id = ?", device.ID).
		UpdateColumns(map[string]any{"config_applied": false, "desired_config": `{"v":2}`, "name": "renamed"}).Error)
	// pending_reboot is read-only on the Device model: only the config repository writes it.
	require.NoError(t, configuration.NewGormConfigRepository(manager.GetDB(), nil).SetDevicePendingReboot(device.ID, true))
	// MySQL keeps milliseconds: let the clock move past the insert.
	time.Sleep(5 * time.Millisecond)

	assertOthersKept := func(t *testing.T, got *Device) {
		t.Helper()
		assert.False(t, got.ConfigApplied, "config_applied was written over")
		assert.Equal(t, `{"v":2}`, got.DesiredConfig, "desired_config was written over")
		assert.True(t, got.PendingReboot, "pending_reboot was written over")
		assert.Equal(t, "renamed", got.Name, "name was written over")
	}

	t.Run("settings", func(t *testing.T) {
		require.NoError(t, manager.UpdateDeviceSettings(device.ID, `{"a":2}`))
		got, getErr := manager.GetDevice(device.ID)
		require.NoError(t, getErr)
		assert.Equal(t, `{"a":2}`, got.Settings)
		assert.Equal(t, stale.IP, got.IP)
		assert.Equal(t, stale.Status, got.Status)
		assertOthersKept(t, got)
		// The device PUT is versioned on updated_at. A login change must move it, or a PUT that
		// read the old settings would pass its version check and write them back.
		assert.True(t, got.UpdatedAt.After(stale.UpdatedAt), "updated_at did not move from %v to %v", stale.UpdatedAt, got.UpdatedAt)
	})

	t.Run("discovery", func(t *testing.T) {
		before, getErr := manager.GetDevice(device.ID)
		require.NoError(t, getErr)
		time.Sleep(5 * time.Millisecond)
		seen := time.Now().Truncate(time.Second)
		require.NoError(t, manager.UpdateDeviceDiscovery(device.ID, "192.0.2.22", "online", seen, "1.1"))
		got, getErr := manager.GetDevice(device.ID)
		require.NoError(t, getErr)
		assert.Equal(t, "192.0.2.22", got.IP)
		assert.Equal(t, "online", got.Status)
		assert.Equal(t, "1.1", got.Firmware)
		assert.WithinDuration(t, seen, got.LastSeen, time.Second)
		assert.Equal(t, `{"a":2}`, got.Settings, "settings were written over")
		assertOthersKept(t, got)
		assert.True(t, got.UpdatedAt.After(before.UpdatedAt), "updated_at did not move from %v to %v", before.UpdatedAt, got.UpdatedAt)
	})

	t.Run("missing device", func(t *testing.T) {
		assert.ErrorIs(t, manager.UpdateDeviceSettings(4242, `{}`), gorm.ErrRecordNotFound)
		assert.ErrorIs(t, manager.UpdateDeviceDiscovery(4242, "192.0.2.99", "online", time.Now(), "1"), gorm.ErrRecordNotFound)
	})
}

func TestUpdateDeviceColumns_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkColumnScopedWrites(t, manager)
}

// The same cases on a PostgreSQL and a MySQL server. Each subtest skips when its server is not
// configured; the Database Provider Migration job of the CI supplies both.
func TestUpdateDeviceColumnsProviders(t *testing.T) {
	fixtures := []struct {
		name  string
		setup func(t *testing.T) providerFixture
	}{
		{name: "postgresql", setup: setupPostgreSQLFixture},
		{name: "mysql", setup: setupMySQLFixture},
	}
	for _, spec := range fixtures {
		t.Run(spec.name, func(t *testing.T) {
			fixture := spec.setup(t)
			checkColumnScopedWrites(t, startProviderManager(t, fixture))
		})
	}
}

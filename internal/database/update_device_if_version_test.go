package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// UpdateDeviceIfVersion saves a device row only where updated_at is still the value the caller
// read (ginsys/shelly-manager#509). The compare is done by the database on a time that went through
// the driver twice, so each driver needs a case: SQLite keeps text, MySQL keeps datetime(3) and
// PostgreSQL keeps microseconds. checkVersionedUpdate holds the cases, and any provider can run it.
func checkVersionedUpdate(t *testing.T, manager *Manager) {
	t.Helper()

	device := &Device{MAC: "AA:BB:CC:DD:EE:11", IP: "192.0.2.11", Name: "lamp", Status: "online", LastSeen: time.Now(), Settings: `{"a":1}`}
	require.NoError(t, manager.AddDevice(device))
	read, err := manager.GetDevice(device.ID)
	require.NoError(t, err)

	// MySQL keeps milliseconds, so let the clock move past the insert before the first link.
	time.Sleep(5 * time.Millisecond)

	// A chain of saves, each against the version that the previous read returned. A compare that
	// does not survive the round trip fails on the first or on a later link, depending on how the
	// clock's digits are cut, so one link is not enough.
	for i := 0; i < 8; i++ {
		next := *read
		next.Name = "step"
		if i%2 == 1 {
			next.Name = "" // a zero value must be written too
		}
		next.Settings = `{"step":` + string(rune('0'+i)) + `}`
		require.NoError(t, manager.UpdateDeviceIfVersion(&next, read.UpdatedAt), "link %d of the chain", i)

		saved, getErr := manager.GetDevice(device.ID)
		require.NoError(t, getErr)
		assert.Equal(t, next.Name, saved.Name, "link %d", i)
		assert.Equal(t, next.Settings, saved.Settings, "link %d", i)
		assert.True(t, saved.UpdatedAt.After(read.UpdatedAt), "link %d: updated_at did not move from %v to %v", i, read.UpdatedAt, saved.UpdatedAt)
		assert.True(t, saved.CreatedAt.Equal(read.CreatedAt), "link %d: created_at changed", i)
		time.Sleep(2 * time.Millisecond)
		read = saved
	}

	// A row whose updated_at is ahead of the clock still gets a newer version. This is the case of
	// two saves in one millisecond: an unchanged save on MySQL would count zero rows if the new
	// updated_at equalled the old one. GORM stamps updated_at from its own clock, so the test
	// proves that the adjusted value is the one stored.
	ahead := time.Now().Add(time.Hour)
	require.NoError(t, manager.GetDB().Model(&Device{}).Where("id = ?", device.ID).UpdateColumn("updated_at", ahead).Error)
	aheadRead, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	unchanged := *aheadRead
	require.NoError(t, manager.UpdateDeviceIfVersion(&unchanged, aheadRead.UpdatedAt))
	moved, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.True(t, moved.UpdatedAt.After(aheadRead.UpdatedAt), "updated_at did not move past %v: it is %v", aheadRead.UpdatedAt, moved.UpdatedAt)
	read = moved

	// A version that is no longer stored saves nothing.
	stale := *read
	stale.Name = "from a stale read"
	older := read.UpdatedAt.Add(-time.Hour)
	assert.ErrorIs(t, manager.UpdateDeviceIfVersion(&stale, older), ErrDeviceVersionChanged)
	again, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, read.Name, again.Name)
	assert.True(t, again.UpdatedAt.Equal(read.UpdatedAt))

	// A device that does not exist is not a conflict.
	missing := &Device{ID: 4242, MAC: "AA:BB:CC:DD:EE:99", IP: "192.0.2.99"}
	assert.ErrorIs(t, manager.UpdateDeviceIfVersion(missing, read.UpdatedAt), gorm.ErrRecordNotFound)
}

func TestUpdateDeviceIfVersion_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkVersionedUpdate(t, manager)
}

// The same cases on a PostgreSQL and a MySQL server. Each subtest skips when its server is not
// configured; the Database Provider Migration job of the CI supplies both.
func TestUpdateDeviceIfVersionProviders(t *testing.T) {
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
			checkVersionedUpdate(t, startProviderManager(t, fixture))
		})
	}
}

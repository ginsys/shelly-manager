package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A save whose device carries a zero created_at or last_seen keeps the stored values
// (ginsys/shelly-manager#515). A PUT body that leaves them out decodes to zero times, MySQL rejects
// a zero datetime (error 1292), and SQLite and PostgreSQL would store it. checkZeroTimesKept holds
// the cases, and any provider can run it.
func checkZeroTimesKept(t *testing.T, manager *Manager) {
	t.Helper()

	device := &Device{MAC: "AA:BB:CC:DD:EE:15", IP: "192.0.2.15", Name: "porch", Status: "online", LastSeen: time.Now().Add(-time.Hour), Settings: `{"a":1}`}
	require.NoError(t, manager.AddDevice(device))
	stored, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	require.False(t, stored.CreatedAt.IsZero())
	require.False(t, stored.LastSeen.IsZero())

	// MySQL keeps milliseconds, so let the clock move past the insert.
	time.Sleep(5 * time.Millisecond)

	saves := []struct {
		name string
		save func(d *Device) error
	}{
		{"UpdateDevice", func(d *Device) error { return manager.UpdateDevice(d) }},
		{"UpdateDeviceIfVersion", func(d *Device) error {
			current, getErr := manager.GetDevice(d.ID)
			if getErr != nil {
				return getErr
			}
			return manager.UpdateDeviceIfVersion(d, current.UpdatedAt)
		}},
	}
	for _, save := range saves {
		t.Run(save.name, func(t *testing.T) {
			// Start each case from the same stored times, whatever the previous case wrote.
			require.NoError(t, manager.GetDB().Model(&Device{}).Where("id = ?", device.ID).
				UpdateColumns(map[string]any{"created_at": stored.CreatedAt, "last_seen": stored.LastSeen}).Error)
			before, getErr := manager.GetDevice(device.ID)
			require.NoError(t, getErr)

			// The body of a PUT that sends neither time, and not the updated_at either.
			incoming := &Device{ID: device.ID, MAC: before.MAC, IP: before.IP, Name: "renamed " + save.name, Status: before.Status, Settings: before.Settings}
			require.NoError(t, save.save(incoming))

			after, getErr := manager.GetDevice(device.ID)
			require.NoError(t, getErr)
			assert.Equal(t, "renamed "+save.name, after.Name, "the other fields are saved")
			assert.True(t, after.CreatedAt.Equal(before.CreatedAt), "created_at: stored %v, now %v", before.CreatedAt, after.CreatedAt)
			assert.True(t, after.LastSeen.Equal(before.LastSeen), "last_seen: stored %v, now %v", before.LastSeen, after.LastSeen)
			time.Sleep(2 * time.Millisecond)
		})
	}

	// A time that the request does carry is still saved.
	before, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	seen := time.Now().Add(-time.Minute).Truncate(time.Second)
	edit := *before
	edit.LastSeen = seen
	require.NoError(t, manager.UpdateDevice(&edit))
	after, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.True(t, after.LastSeen.Equal(seen), "last_seen the request carries: want %v, got %v", seen, after.LastSeen)
}

func TestUpdateDeviceZeroTimesKept_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkZeroTimesKept(t, manager)
}

// The same cases on a PostgreSQL and a MySQL server. Each subtest skips when its server is not
// configured; the Database Provider Migration job of the CI supplies both.
func TestUpdateDeviceZeroTimesKeptProviders(t *testing.T) {
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
			checkZeroTimesKept(t, startProviderManager(t, fixture))
		})
	}
}

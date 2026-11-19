package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// addVersionedDevice stores a device whose discovery fields are the ones the tests then report
// again, and returns it as stored, with the version the device PUT would read.
func addVersionedDevice(t *testing.T, manager *Manager, mac string) *Device {
	t.Helper()
	device := &Device{
		MAC: mac, IP: "192.0.2.41", Name: "lamp", Type: "SHPLG-S", Firmware: "1.0",
		Status: "offline", LastSeen: time.Now().Add(-time.Hour), Settings: "{}",
	}
	require.NoError(t, manager.AddDevice(device))
	stored, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	// A version that moved would differ by more than the clock's resolution, so let it pass.
	time.Sleep(5 * time.Millisecond)
	return stored
}

// updated_at is the version of the device PUT (ginsys/shelly-manager#502), and a discovery that
// found nothing new is an observation, not an edit: it must not make a person's open form stale
// (ginsys/shelly-manager#583).
func TestUpdateDeviceDiscovery_UnchangedKeepsTheVersion(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	before := addVersionedDevice(t, manager, "AA:BB:CC:DD:EE:41")

	seen := time.Now().Truncate(time.Second)
	require.NoError(t, manager.UpdateDeviceDiscovery(before.ID,
		DiscoveryUpdate{IP: before.IP, Type: before.Type, Firmware: before.Firmware, Status: "online", LastSeen: seen}))

	got, err := manager.GetDevice(before.ID)
	require.NoError(t, err)
	assert.True(t, before.UpdatedAt.Equal(got.UpdatedAt), "updated_at moved from %v to %v", before.UpdatedAt, got.UpdatedAt)
	assert.Equal(t, "online", got.Status)
	assert.WithinDuration(t, seen, got.LastSeen, time.Second)
}

// An empty type means the discovery could not name the model and the stored type stays, so it is
// no change either.
func TestUpdateDeviceDiscovery_AnEmptyTypeKeepsTheVersion(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	before := addVersionedDevice(t, manager, "AA:BB:CC:DD:EE:42")

	require.NoError(t, manager.UpdateDeviceDiscovery(before.ID,
		DiscoveryUpdate{IP: before.IP, Firmware: before.Firmware, Status: "online", LastSeen: time.Now()}))

	got, err := manager.GetDevice(before.ID)
	require.NoError(t, err)
	assert.True(t, before.UpdatedAt.Equal(got.UpdatedAt), "updated_at moved from %v to %v", before.UpdatedAt, got.UpdatedAt)
	assert.Equal(t, before.Type, got.Type)
}

// A new address, type or firmware is what a client edits, so each one moves the version and is
// stored.
func TestUpdateDeviceDiscovery_AChangeMovesTheVersion(t *testing.T) {
	cases := []struct {
		name   string
		update func(d *Device) DiscoveryUpdate
		check  func(t *testing.T, got *Device)
	}{
		{
			name: "address",
			update: func(d *Device) DiscoveryUpdate {
				return DiscoveryUpdate{IP: "192.0.2.99", Type: d.Type, Firmware: d.Firmware}
			},
			check: func(t *testing.T, got *Device) { assert.Equal(t, "192.0.2.99", got.IP) },
		},
		{
			name: "type",
			update: func(d *Device) DiscoveryUpdate {
				return DiscoveryUpdate{IP: d.IP, Type: "SHPLG-S2", Firmware: d.Firmware}
			},
			check: func(t *testing.T, got *Device) { assert.Equal(t, "SHPLG-S2", got.Type) },
		},
		{
			name: "firmware",
			update: func(d *Device) DiscoveryUpdate {
				return DiscoveryUpdate{IP: d.IP, Type: d.Type, Firmware: "1.1"}
			},
			check: func(t *testing.T, got *Device) { assert.Equal(t, "1.1", got.Firmware) },
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager, cleanup := setupTestManager(t)
			defer cleanup()
			before := addVersionedDevice(t, manager, "AA:BB:CC:DD:EE:5"+string(rune('0'+i)))

			update := tc.update(before)
			update.Status, update.LastSeen = "online", time.Now()
			require.NoError(t, manager.UpdateDeviceDiscovery(before.ID, update))

			got, err := manager.GetDevice(before.ID)
			require.NoError(t, err)
			assert.True(t, got.UpdatedAt.After(before.UpdatedAt), "updated_at did not move")
			tc.check(t, got)
		})
	}
}

// The unchanged path writes status and last_seen and nothing else, so an address that a client
// saved between the discovery's read and its write is not written back.
func TestUpdateDeviceDiscovery_UnchangedKeepsAClientEditCommittedAfterTheRead(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	before := addVersionedDevice(t, manager, "AA:BB:CC:DD:EE:43")

	// The client's version, as the database stored it.
	var edited Device
	afterFirstDeviceRead(t, manager.GetDB(), func(tx *gorm.DB) {
		require.NoError(t, tx.Model(&Device{}).Where("id = ?", before.ID).UpdateColumns(map[string]any{
			"ip": "192.0.2.77", "updated_at": time.Now(),
		}).Error)
		require.NoError(t, tx.First(&edited, before.ID).Error)
	})

	require.NoError(t, manager.UpdateDeviceDiscovery(before.ID,
		DiscoveryUpdate{IP: before.IP, Type: before.Type, Firmware: before.Firmware, Status: "online", LastSeen: time.Now()}))

	got, err := manager.GetDevice(before.ID)
	require.NoError(t, err)
	assert.Equal(t, "192.0.2.77", got.IP, "the client's address was written over")
	assert.True(t, edited.UpdatedAt.Equal(got.UpdatedAt), "the client's version %v was moved to %v", edited.UpdatedAt, got.UpdatedAt)
	assert.Equal(t, "online", got.Status)
}

func TestUpdateDeviceDiscovery_MissingDevice(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	err := manager.UpdateDeviceDiscovery(9999, DiscoveryUpdate{IP: "192.0.2.1", Status: "online", LastSeen: time.Now()})
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// The discovery upsert goes through the same writer, and returns the stored row.
func TestUpsertDeviceFromDiscovery_UnchangedKeepsTheVersion(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	before := addVersionedDevice(t, manager, "AA:BB:CC:DD:EE:44")

	returned, err := manager.UpsertDeviceFromDiscovery(before.MAC,
		DiscoveryUpdate{IP: before.IP, Type: before.Type, Firmware: before.Firmware, Status: "online", LastSeen: time.Now()}, "ignored")
	require.NoError(t, err)

	got, err := manager.GetDevice(before.ID)
	require.NoError(t, err)
	assert.True(t, before.UpdatedAt.Equal(got.UpdatedAt), "updated_at moved from %v to %v", before.UpdatedAt, got.UpdatedAt)
	assert.True(t, got.UpdatedAt.Equal(returned.UpdatedAt), "the returned row is not the stored one")
	assert.Equal(t, "online", returned.Status)
}

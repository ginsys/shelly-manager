package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

func addTestDevice(t *testing.T, manager *Manager, ip, mac string) *Device {
	t.Helper()
	device := &Device{IP: ip, MAC: mac, Type: "SHPLG-S", Name: "plug", Status: "offline"}
	require.NoError(t, manager.AddDevice(device))
	return device
}

// A whole-row Save from a snapshot read before the flag was set (a status write, the plain device
// PUT) must not write false over it, or the retry loses the reboot. Runs on the real schema.
func TestPendingReboot_SurvivesAStaleWholeRowSave(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	device := addTestDevice(t, manager, "192.168.1.100", "A4:CF:12:34:56:78")
	repo := configuration.NewGormConfigRepository(manager.GetDB(), nil)

	stale, err := manager.GetDevice(device.ID) // read before the flag is set
	require.NoError(t, err)

	require.NoError(t, repo.SetDevicePendingReboot(device.ID, true))
	stale.Status = "online"
	require.NoError(t, manager.UpdateDevice(stale)) // the stale whole-row write

	reread, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "online", reread.Status, "control: the stale Save reached the row")

	got, err := repo.GetDevice(device.ID)
	require.NoError(t, err)
	assert.True(t, got.PendingReboot, "the stale Save wrote false over the flag")
}

func TestPendingReboot_StoreSetsAndClearsOneDevice(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	first := addTestDevice(t, manager, "192.168.1.100", "A4:CF:12:34:56:78")
	second := addTestDevice(t, manager, "192.168.1.101", "A4:CF:12:34:56:79")
	repo := configuration.NewGormConfigRepository(manager.GetDB(), nil)

	require.NoError(t, repo.SetDevicePendingReboot(first.ID, true))

	a, err := repo.GetDevice(first.ID)
	require.NoError(t, err)
	b, err := repo.GetDevice(second.ID)
	require.NoError(t, err)
	assert.True(t, a.PendingReboot)
	assert.False(t, b.PendingReboot)

	require.NoError(t, repo.SetDevicePendingReboot(first.ID, false))
	a, err = repo.GetDevice(first.ID)
	require.NoError(t, err)
	assert.False(t, a.PendingReboot)
}

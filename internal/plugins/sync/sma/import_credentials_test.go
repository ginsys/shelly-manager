package sma

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// TestForcedImportOfALegacyArchiveKeepsASavedLogin pins a review finding on
// ginsys/shelly-manager#378. An archive exported while the device held its login in the older
// nested settings.auth shape carries only "auth", redacted. A login saved after the export must
// survive a forced restore of that archive, because the archive can never carry the secret
// itself.
func TestForcedImportOfALegacyArchiveKeepsASavedLogin(t *testing.T) {
	f := newPersistFixture(t)
	require.NoError(t, f.db.AddDevice(&database.Device{
		IP: "192.0.2.30", MAC: "a1b2c3d4e5f6", Type: "Smart Plug", Name: "legacy-login",
		Status: "online", Settings: `{"gen":1,"auth":{"username":"admin","password":"old"}}`,
	}))
	archive := f.export(t)

	var device database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "a1b2c3d4e5f6").First(&device).Error)
	svc := service.NewService(f.db, &config.Config{})
	t.Cleanup(svc.Stop)
	require.NoError(t, svc.UpdateDeviceAuth(device.ID, "admin", "new"))

	_, err := f.importFile(t, archive, sync.ImportOptions{ForceOverwrite: true})
	require.NoError(t, err)

	var restored database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "a1b2c3d4e5f6").First(&restored).Error)
	user, pass, ok := configuration.LegacyDeviceAuth(restored.Settings)
	require.True(t, ok, "the restored settings must still carry a login: %s", restored.Settings)
	assert.Equal(t, "admin", user)
	assert.Equal(t, "new", pass, "the login saved after the export wins over the archive's redaction")
}

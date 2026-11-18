package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	syncengine "github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The GitOps import loads every device once and then saves the rows it changes. It writes a name and
// a type, so it saves those two columns: a row saved whole writes back a config_applied or a
// desired_config that an apply committed after the load (ginsys/shelly-manager#572).
func TestGitOpsImport_SavesOnlyNameAndType(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	stored := &database.Device{
		MAC: "aabbccddee41", IP: "192.0.2.41", Name: "old name", Type: "SHPLG-S", Firmware: "1.0",
		Status: "online", LastSeen: time.Now().Add(-time.Hour), Settings: `{"a":1}`,
		DesiredConfig: `{"v":1}`, ConfigApplied: true,
	}
	require.NoError(t, db.AddDevice(stored))
	before, err := db.GetDevice(stored.ID)
	require.NoError(t, err)

	// Right after the load of the devices, an apply commits its flags, an edit changes the settings.
	const callback = "test:after-gitops-load"
	fired := false
	require.NoError(t, db.GetDB().Callback().Query().After("gorm:after_query").Register(callback, func(d *gorm.DB) {
		if fired || d.Statement.Table != "devices" {
			return
		}
		fired = true
		tx := d.Session(&gorm.Session{NewDB: true})
		require.NoError(t, tx.Model(&database.Device{}).Where("id = ?", stored.ID).UpdateColumns(map[string]any{
			"config_applied": false, "desired_config": `{"v":2}`, "settings": `{"a":2}`,
		}).Error)
		require.NoError(t, configuration.NewGormConfigRepository(tx, nil).SetDevicePendingReboot(stored.ID, true))
	}))
	defer func() { _ = db.GetDB().Callback().Query().Remove(callback) }() // runs before cleanup() closes db
	time.Sleep(5 * time.Millisecond)

	importer := syncengine.NewGitOpsImporter(db, logger)
	result, err := importer.Import(context.Background(), &syncengine.GitOpsData{
		Devices: []syncengine.GitOpsDevice{{MAC: stored.MAC, Name: "new name", Type: "SHPLG-S2"}},
	}, syncengine.GitOpsImportOptions{})
	require.NoError(t, err)
	require.True(t, result.Success, "import errors: %v", result.Errors)
	require.True(t, fired, "the competing write never ran")

	got, err := db.GetDevice(stored.ID)
	require.NoError(t, err)
	assert.Equal(t, "new name", got.Name)
	assert.Equal(t, "SHPLG-S2", got.Type)
	assert.False(t, got.ConfigApplied, "config_applied was written over")
	assert.Equal(t, `{"v":2}`, got.DesiredConfig, "desired_config was written over")
	assert.True(t, got.PendingReboot, "pending_reboot was written over")
	assert.Equal(t, `{"a":2}`, got.Settings, "settings were written over")
	assert.True(t, got.UpdatedAt.After(before.UpdatedAt), "updated_at did not move: a client that read the old name must meet a 409")
}

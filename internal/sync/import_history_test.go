package sync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func importHistoryRows(t *testing.T, db *database.Manager, deviceID uint) []configuration.DesiredConfigHistory {
	t.Helper()
	var rows []configuration.DesiredConfigHistory
	require.NoError(t, db.GetDB().Where("device_id = ?", deviceID).Order("id").Find(&rows).Error)
	return rows
}

func desiredArchive(config map[string]interface{}) *ExportData {
	data := &ExportData{Devices: []DeviceData{{MAC: "aabbccdd0775", IP: "192.0.2.75", Type: "switch", Name: "archived"}}}
	if config != nil {
		data.Devices[0].Configuration = &ConfigurationData{Config: config}
	}
	return data
}

// Each import site leaves one history row per real change, in the import's own transaction.
func TestApplyImport_WritesDesiredHistory(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	require.NoError(t, db.GetDB().AutoMigrate(&configuration.DeviceConfig{}))
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	engine := NewSyncEngine(db, logger)
	ctx := context.Background()

	storedID := func() uint {
		var device database.Device
		require.NoError(t, db.GetDB().Where("mac = ?", "aabbccdd0775").First(&device).Error)
		return device.ID
	}

	t.Run("a created device leaves one row with an empty old side", func(t *testing.T) {
		_, err := engine.ApplyImport(ctx, desiredArchive(map[string]interface{}{"mqtt": map[string]interface{}{"server": "a.example", "pass": "canary-1"}}), ImportOptions{})
		require.NoError(t, err)
		rows := importHistoryRows(t, db, storedID())
		require.Len(t, rows, 1)
		assert.Equal(t, "import", rows[0].Action)
		assert.Equal(t, "system", rows[0].ChangedBy)
		assert.Nil(t, rows[0].OldDesired)
		require.NotNil(t, rows[0].NewDesired)
		assert.NotContains(t, *rows[0].NewDesired, "canary-1")
		assert.Contains(t, *rows[0].NewDesired, "a.example")
	})

	t.Run("an overwrite's old side is the desired text it replaced", func(t *testing.T) {
		id := storedID()
		var before database.Device
		require.NoError(t, db.GetDB().First(&before, id).Error)
		// A device_configs row older than devices.desired_config must not become the old side.
		require.NoError(t, db.GetDB().Create(&configuration.DeviceConfig{DeviceID: id, Config: []byte(`{"legacy":true}`)}).Error)

		_, err := engine.ApplyImport(ctx, desiredArchive(map[string]interface{}{"mqtt": map[string]interface{}{"server": "b.example"}}), ImportOptions{ForceOverwrite: true})
		require.NoError(t, err)
		rows := importHistoryRows(t, db, id)
		require.Len(t, rows, 2)
		require.NotNil(t, rows[1].OldDesired)
		assert.NotContains(t, *rows[1].OldDesired, "legacy")
		assert.Contains(t, *rows[1].OldDesired, "a.example")
		assert.Contains(t, *rows[1].NewDesired, "b.example")
	})

	t.Run("the same desired text writes no row", func(t *testing.T) {
		id := storedID()
		_, err := engine.ApplyImport(ctx, desiredArchive(map[string]interface{}{"mqtt": map[string]interface{}{"server": "b.example"}}), ImportOptions{ForceOverwrite: true})
		require.NoError(t, err)
		assert.Len(t, importHistoryRows(t, db, id), 2)
	})

	t.Run("an archive without a desired text writes no row", func(t *testing.T) {
		id := storedID()
		_, err := engine.ApplyImport(ctx, desiredArchive(nil), ImportOptions{ForceOverwrite: true})
		require.NoError(t, err)
		assert.Len(t, importHistoryRows(t, db, id), 2)
	})

	t.Run("a skipped device writes no row", func(t *testing.T) {
		id := storedID()
		_, err := engine.ApplyImport(ctx, desiredArchive(map[string]interface{}{"mqtt": map[string]interface{}{"server": "c.example"}}), ImportOptions{})
		require.NoError(t, err)
		assert.Len(t, importHistoryRows(t, db, id), 2)
	})
}

// A device a replaced template feeds is recomputed in the import transaction, and the recompute leaves
// its own row, written through the same hook as an API-side recompute.
func TestApplyImport_TemplateRecomputeWritesHistory(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	require.NoError(t, db.GetDB().AutoMigrate(&configuration.DeviceConfig{}))
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	engine := NewSyncEngine(db, logger)

	template := configuration.ConfigTemplate{
		Name: "history-template", Scope: "global", DeviceType: "all",
		Config: []byte(`{"mqtt":{"server":"old.example"}}`), Variables: []byte(`{}`),
	}
	require.NoError(t, db.GetDB().Create(&template).Error)
	device := database.Device{
		MAC: "aabbccdd0776", IP: "192.0.2.76", Type: "switch", Name: "fed",
		TemplateIDs: fmt.Sprintf("[%d]", template.ID), Overrides: "{}", DesiredConfig: "{}",
	}
	require.NoError(t, db.AddDevice(&device))

	archive := &ExportData{Templates: []TemplateData{{
		Name: "history-template", DeviceType: "all", Config: map[string]interface{}{"mqtt": map[string]interface{}{"server": "new.example"}},
		Variables: map[string]interface{}{},
	}}}
	_, err = engine.ApplyImport(context.Background(), archive, ImportOptions{ForceOverwrite: true})
	require.NoError(t, err)

	rows := importHistoryRows(t, db, device.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, "recompute", rows[0].Action)
	require.NotNil(t, rows[0].NewDesired)
	assert.Contains(t, *rows[0].NewDesired, "new.example")
}

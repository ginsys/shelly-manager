package sync_test

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/sma"
	syncengine "github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func TestSyncEngineSMARoundTripIncludesPersistedData(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	configuration.NewService(db.GetDB(), logger)

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	device := database.Device{
		MAC:           "aabbccddeeff",
		IP:            "192.0.2.10",
		Type:          "SHPLG-S",
		Name:          "Kitchen plug",
		Firmware:      "1.0.0",
		Status:        "online",
		LastSeen:      now,
		Settings:      `{"model":"Shelly Plus Plug S","relay":{"enabled":true}}`,
		DesiredConfig: `{"mqtt":{"enable":true}}`,
		ConfigApplied: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	require.NoError(t, db.GetDB().Create(&device).Error)
	fallbackDevice := database.Device{
		MAC:       "112233445566",
		IP:        "192.0.2.11",
		Type:      "SHSW-1",
		Name:      "Hall switch",
		Firmware:  "1.0.0",
		Status:    "online",
		LastSeen:  now,
		Settings:  `{"model":""}`,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.GetDB().Create(&fallbackDevice).Error)
	require.NoError(t, db.GetDB().Create(&configuration.DeviceConfig{
		DeviceID:   device.ID,
		Config:     json.RawMessage(`{"mqtt":{"enable":false}}`),
		SyncStatus: "drift",
		UpdatedAt:  now,
	}).Error)
	template := configuration.ConfigTemplate{
		Name:        "plug-default",
		Description: "Default plug configuration",
		Scope:       "device_type",
		DeviceType:  "SHPLG-S",
		Generation:  1,
		Config:      json.RawMessage(`{"relay":{"default":"off"}}`),
		Variables:   json.RawMessage(`{"room":"kitchen"}`),
		IsDefault:   true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, db.GetDB().Create(&template).Error)

	outputDir := t.TempDir()
	engine := syncengine.NewSyncEngine(db, logger)
	engine.SetExportBaseDir(outputDir)
	require.NoError(t, engine.RegisterPlugin(sma.NewPlugin()))

	exported, err := engine.Export(context.Background(), syncengine.ExportRequest{
		PluginName: "sma",
		Format:     "sma",
		Config: map[string]interface{}{
			"output_path":        outputDir,
			"include_discovered": false,
		},
		Output: syncengine.OutputConfig{Type: "file"},
	})
	require.NoError(t, err)
	require.Equal(t, 3, exported.RecordCount)

	file, err := os.Open(exported.OutputPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	reader, err := gzip.NewReader(file)
	require.NoError(t, err)
	var archive sma.SMAArchive
	require.NoError(t, json.NewDecoder(reader).Decode(&archive))
	require.NoError(t, reader.Close())

	require.Equal(t, "shelly-manager", archive.Metadata.CreatedBy)
	require.Equal(t, "manual", archive.Metadata.ExportType)
	require.Len(t, archive.Devices, 2)
	require.Equal(t, "Shelly Plus Plug S", archive.Devices[0].Model)
	require.NotEqual(t, archive.Devices[0].Type, archive.Devices[0].Model)
	require.Equal(t, true, archive.Devices[0].Settings["relay"].(map[string]interface{})["enabled"])
	require.NotNil(t, archive.Devices[0].Configuration)
	require.Equal(t, true, archive.Devices[0].Configuration.Config["mqtt"].(map[string]interface{})["enable"])
	require.Equal(t, "SHSW-1", archive.Devices[1].Model)
	require.Len(t, archive.Templates, 1)
	require.Equal(t, "off", archive.Templates[0].Config["relay"].(map[string]interface{})["default"])
	require.Equal(t, "kitchen", archive.Templates[0].Variables["room"])

	gzipBytes, err := os.ReadFile(exported.OutputPath)
	require.NoError(t, err)
	imported, err := engine.Import(context.Background(), syncengine.ImportRequest{
		PluginName: "sma",
		Format:     "sma",
		Source:     syncengine.ImportSource{Type: "data", Data: gzipBytes},
		Config:     map[string]interface{}{},
		Options:    syncengine.ImportOptions{DryRun: true, ValidateOnly: true},
	})
	require.NoError(t, err)
	require.Equal(t, 3, imported.RecordsImported)
	require.Len(t, imported.Changes, 3)
}

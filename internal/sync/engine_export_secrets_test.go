package sync_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/jsonexport"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/yamlexport"
	syncengine "github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// These tests pin that no JSON, YAML or GitOps export, and no preview of one, carries a saved
// device secret, and that an operator can still ask for a full export.

const (
	expFlat   = "ex9ort-flat-s3cret"
	expLegacy = "ex9ort-legacy-s3cret"
	expSTA    = "ex9ort-sta-s3cret"
	expMQTT   = "ex9ort-mqtt-s3cret"
	expTmpl   = "ex9ort-template-s3cret"
	expVar    = "ex9ort-variable-s3cret"
	expStored = "ex9ort-stored-s3cret"
	expList   = "ex9ort-list-s3cret"
)

var allExportSecrets = []string{expFlat, expLegacy, expSTA, expMQTT, expTmpl, expVar, expStored, expList}

func exportEngine(t *testing.T) (*syncengine.SyncEngine, string) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	configuration.NewService(db.GetDB(), logger)

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	withDesired := database.Device{
		MAC: "aabbccddeeff", IP: "192.0.2.10", Type: "SHPLG-S", Name: "Kitchen plug", Status: "online",
		LastSeen: now, CreatedAt: now, UpdatedAt: now,
		Settings: `{"model":"SHPLG-S","auth_enabled":true,"auth_user":"admin","auth_pass":"` + expFlat + `",` +
			`"auth":{"username":"admin","password":"` + expLegacy + `"},"wifi_sta":{"ssid":"home","key":"` + expSTA + `"}}`,
		DesiredConfig: `{"mqtt":{"enable":true,"server":"mqtt.example.net","pass":"` + expMQTT + `"}}`,
	}
	require.NoError(t, db.GetDB().Create(&withDesired).Error)
	withStored := database.Device{
		MAC: "112233445566", IP: "192.0.2.11", Type: "SHSW-1", Name: "Hall switch", Status: "online",
		LastSeen: now, CreatedAt: now, UpdatedAt: now, Settings: `{"model":"SHSW-1"}`,
	}
	require.NoError(t, db.GetDB().Create(&withStored).Error)
	require.NoError(t, db.GetDB().Create(&configuration.DeviceConfig{
		DeviceID: withStored.ID, Config: json.RawMessage(`{"auth":{"user":"admin","pass":"` + expStored + `"}}`),
		SyncStatus: "synced", UpdatedAt: now,
	}).Error)
	require.NoError(t, db.GetDB().Create(&configuration.ConfigTemplate{
		Name: "plug-default", Scope: "device_type", DeviceType: "SHPLG-S", Generation: 1, IsDefault: true,
		Config:    json.RawMessage(`{"wifi":{"password":"` + expTmpl + `"},"relay":{"default":"off"}}`),
		Variables: json.RawMessage(`{"room":"kitchen","api_token":"` + expVar + `","api_tokens":["` + expList + `"]}`),
		CreatedAt: now, UpdatedAt: now,
	}).Error)

	engine := syncengine.NewSyncEngine(db, logger)
	baseDir := t.TempDir()
	engine.SetExportBaseDir(baseDir)
	require.NoError(t, engine.RegisterPlugin(jsonexport.NewPlugin()))
	require.NoError(t, engine.RegisterPlugin(yamlexport.NewPlugin()))
	require.NoError(t, engine.RegisterPlugin(gitops.NewPlugin()))
	return engine, baseDir
}

// readAll returns the bytes of a file, or of every file under a directory.
func readAll(t *testing.T, path string) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, readErr := os.ReadFile(p)
		out.Write(b)
		return readErr
	}))
	return out.String()
}

func exportRequest(plugin, format, dir string, extra map[string]interface{}) syncengine.ExportRequest {
	config := map[string]interface{}{"output_path": filepath.Join(dir, plugin+"-out"), "include_templates": true}
	for k, v := range extra {
		config[k] = v
	}
	return syncengine.ExportRequest{
		PluginName: plugin, Format: format, Config: config, Output: syncengine.OutputConfig{Type: "file"},
	}
}

var exportPlugins = []struct{ plugin, format string }{{"json", "json"}, {"yaml", "yaml"}, {"gitops", "yaml"}}

func TestExports_WriteNoSavedSecret(t *testing.T) {
	for _, p := range exportPlugins {
		t.Run(p.plugin, func(t *testing.T) {
			engine, dir := exportEngine(t)
			result, err := engine.Export(context.Background(), exportRequest(p.plugin, p.format, dir, nil))
			require.NoError(t, err)
			require.NotEmpty(t, result.OutputPath)

			text := readAll(t, result.OutputPath)
			for _, s := range allExportSecrets {
				assert.NotContains(t, text, s, "%s export writes the saved secret %s in clear", p.plugin, s)
			}
			assert.Contains(t, text, syncengine.RedactedValue, "a secret that is set shows as the marker")
			assert.Contains(t, text, "home", "the settings around a secret stay")
		})
	}
}

// The operator can still ask for a full export, as the SMA export allows with the same key.
func TestExports_ExcludeSensitiveFalseWritesFullExport(t *testing.T) {
	engine, dir := exportEngine(t)
	request := exportRequest("json", "json", dir, map[string]interface{}{"exclude_sensitive": false})
	result, err := engine.Export(context.Background(), request)
	require.NoError(t, err)

	// The JSON export writes the device settings and the templates, not the device configurations.
	text := readAll(t, result.OutputPath)
	for _, s := range []string{expFlat, expLegacy, expSTA, expTmpl, expVar, expList} {
		assert.Contains(t, text, s)
	}
}

// recordingPlugin keeps what the engine hands it, serialized, for an export and for a preview.
type recordingPlugin struct {
	exported  string
	previewed string
}

func (r *recordingPlugin) Info() syncengine.PluginInfo {
	return syncengine.PluginInfo{
		Name: "recorder", Version: "1.0.0", Description: "records the data it is given", Author: "test",
		License: "MIT", SupportedFormats: []string{"json"}, Category: syncengine.CategoryCustom,
	}
}
func (r *recordingPlugin) ConfigSchema() syncengine.ConfigSchema {
	return syncengine.ConfigSchema{Version: "1.0", Properties: map[string]syncengine.PropertySchema{}}
}
func (r *recordingPlugin) ValidateConfig(map[string]interface{}) error { return nil }
func (r *recordingPlugin) Export(_ context.Context, data *syncengine.ExportData, _ syncengine.ExportConfig) (*syncengine.ExportResult, error) {
	b, err := json.Marshal(data)
	r.exported = string(b)
	return &syncengine.ExportResult{Success: true, RecordCount: len(data.Devices)}, err
}
func (r *recordingPlugin) Preview(_ context.Context, data *syncengine.ExportData, _ syncengine.ExportConfig) (*syncengine.PreviewResult, error) {
	b, err := json.Marshal(data)
	r.previewed = string(b)
	return &syncengine.PreviewResult{Success: true, RecordCount: len(data.Devices)}, err
}
func (r *recordingPlugin) Import(context.Context, syncengine.ImportSource, syncengine.ImportConfig) (*syncengine.ImportResult, error) {
	return nil, nil
}
func (r *recordingPlugin) Capabilities() syncengine.PluginCapabilities {
	return syncengine.PluginCapabilities{SupportedOutputs: []string{"file"}, ConcurrencyLevel: 1}
}
func (r *recordingPlugin) Initialize(*logging.Logger) error { return nil }
func (r *recordingPlugin) Cleanup() error                   { return nil }

// What a plugin receives, in the export and in the preview, carries no saved secret, and the same
// run with exclude_sensitive false does carry them, so the check can fail.
func TestExports_PluginReceivesNoSavedSecret(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]interface{}
		clear bool
	}{
		{"default", nil, false},
		{"exclude_sensitive true", map[string]interface{}{"exclude_sensitive": true}, false},
		{"exclude_sensitive false", map[string]interface{}{"exclude_sensitive": false}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, dir := exportEngine(t)
			recorder := &recordingPlugin{}
			require.NoError(t, engine.RegisterPlugin(recorder))
			request := exportRequest("recorder", "json", dir, tc.extra)

			_, err := engine.Export(context.Background(), request)
			require.NoError(t, err)
			_, err = engine.Preview(context.Background(), request)
			require.NoError(t, err)

			for kind, got := range map[string]string{"export": recorder.exported, "preview": recorder.previewed} {
				require.NotEmpty(t, got, kind)
				for _, s := range []string{expFlat, expLegacy, expSTA, expMQTT, expTmpl, expVar, expStored, expList} {
					if tc.clear {
						assert.Contains(t, got, s, "%s with a full export asked for lacks %s", kind, s)
					} else {
						assert.NotContains(t, got, s, "%s hands the plugin the saved secret %s", kind, s)
					}
				}
			}
		})
	}
}

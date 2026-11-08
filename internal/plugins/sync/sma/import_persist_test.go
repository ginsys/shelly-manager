package sma

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

const (
	persistDesiredConfig = `{"system":{"device":{"name":"tv-office"}},"mqtt":{"enable":true,"server":"broker.local","port":1883}}`
	persistSettings      = `{"model":"SHPLG-S","name":"tv-office"}`
)

// persistFixture is an engine over a real database with the SMA plugin registered.
type persistFixture struct {
	db     *database.Manager
	engine *sync.SyncEngine
	dir    string
}

func newPersistFixture(t *testing.T) *persistFixture {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	// Production start-up migrates the configuration models on top of the database package's own
	// tables (several structs share config_templates, see #280); the test database only has the
	// latter.
	require.NoError(t, db.GetDB().AutoMigrate(&configuration.ConfigTemplate{}, &configuration.DeviceConfig{}))

	dir := t.TempDir()
	engine := sync.NewSyncEngine(db, logger)
	engine.SetExportBaseDir(dir)
	engine.SetImportBaseDir(dir)
	require.NoError(t, engine.RegisterPlugin(NewPlugin()))
	return &persistFixture{db: db, engine: engine, dir: dir}
}

func (f *persistFixture) seed(t *testing.T) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, f.db.AddDevice(&database.Device{
		IP: "192.0.2.10", MAC: "aabbccddeeff", Type: "Smart Plug", Name: "tv-office",
		Firmware: "1.0", Status: "online", LastSeen: now, Settings: persistSettings,
		DesiredConfig: persistDesiredConfig,
	}))
	require.NoError(t, f.db.GetDB().Create(&configuration.ConfigTemplate{
		Name: "plug-defaults", Description: "defaults", Scope: "device_type", DeviceType: "SHPLG-S",
		Generation: 1, Config: json.RawMessage(`{"mqtt":{"enable":true}}`),
		Variables: json.RawMessage(`{"broker":"broker.local"}`), IsDefault: true,
	}).Error)
	require.NoError(t, f.db.GetDB().Create(&database.DiscoveredDevice{
		MAC: "112233445566", SSID: "shelly-1", Model: "SHSW-1", Generation: 1, IP: "192.0.2.20",
		AgentID: "agent-1", Discovered: now, ExpiresAt: now.Add(time.Hour),
	}).Error)
}

func (f *persistFixture) wipe(t *testing.T) {
	t.Helper()
	gdb := f.db.GetDB()
	require.NoError(t, gdb.Where("1 = 1").Delete(&database.Device{}).Error)
	require.NoError(t, gdb.Where("1 = 1").Delete(&configuration.ConfigTemplate{}).Error)
	require.NoError(t, gdb.Where("1 = 1").Delete(&database.DiscoveredDevice{}).Error)
}

func (f *persistFixture) export(t *testing.T) string {
	t.Helper()
	result, err := f.engine.Export(context.Background(), sync.ExportRequest{
		PluginName: "sma", Format: "sma",
		Config: map[string]interface{}{"output_path": f.dir, "include_discovered": true},
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	return result.OutputPath
}

func (f *persistFixture) importFile(t *testing.T, path string, options sync.ImportOptions) (*sync.ImportResult, error) {
	t.Helper()
	return f.engine.Import(context.Background(), sync.ImportRequest{
		PluginName: "sma", Format: "sma",
		Source:  sync.ImportSource{Type: "file", Path: path},
		Options: options,
	})
}

func TestImportRestoresWhatExportWrote(t *testing.T) {
	f := newPersistFixture(t)
	f.seed(t)
	archive := f.export(t)
	f.wipe(t)

	result, err := f.importFile(t, archive, sync.ImportOptions{})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 3, result.RecordsImported, "one device, one template, one discovered device")
	assert.Equal(t, 0, result.RecordsSkipped)
	assert.Len(t, result.Changes, 3)

	var device database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&device).Error)
	assert.Equal(t, "tv-office", device.Name)
	assert.Equal(t, "192.0.2.10", device.IP)
	assert.JSONEq(t, persistSettings, device.Settings)
	assert.JSONEq(t, persistDesiredConfig, device.DesiredConfig, "the desired configuration is the point of the archive")
	assert.False(t, device.ConfigApplied, "a restored configuration has not reached the device")

	var template configuration.ConfigTemplate
	require.NoError(t, f.db.GetDB().Where("name = ?", "plug-defaults").First(&template).Error)
	assert.JSONEq(t, `{"mqtt":{"enable":true}}`, string(template.Config))
	assert.JSONEq(t, `{"broker":"broker.local"}`, string(template.Variables))
	assert.Equal(t, "SHPLG-S", template.DeviceType)
	assert.NotEmpty(t, template.Scope)

	var discovered database.DiscoveredDevice
	require.NoError(t, f.db.GetDB().Where("mac = ?", "112233445566").First(&discovered).Error)
	assert.Equal(t, "agent-1", discovered.AgentID)
	assert.True(t, discovered.ExpiresAt.After(time.Now()), "an imported discovered device must not be born expired")
}

func TestImportSkipsExistingRecordsUnlessForced(t *testing.T) {
	f := newPersistFixture(t)
	f.seed(t)
	archive := f.export(t)

	// Change a record after the export, then import over it.
	require.NoError(t, f.db.GetDB().Model(&database.Device{}).Where("mac = ?", "aabbccddeeff").Update("name", "renamed").Error)

	result, err := f.importFile(t, archive, sync.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, result.RecordsImported)
	assert.Equal(t, 3, result.RecordsSkipped)
	assert.NotEmpty(t, result.Warnings, "skips must be reported, not silent")
	var kept database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&kept).Error)
	assert.Equal(t, "renamed", kept.Name)

	forced, err := f.importFile(t, archive, sync.ImportOptions{ForceOverwrite: true})
	require.NoError(t, err)
	assert.Equal(t, 3, forced.RecordsImported)
	assert.Equal(t, 0, forced.RecordsSkipped)
	var restored database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&restored).Error)
	assert.Equal(t, "tv-office", restored.Name)
}

const persistSecretConfig = `{"mqtt":{"enable":true,"server":"broker.local","password":"s3cret"}}`

// A default export redacts sensitive values. Importing the archive must never store the
// placeholder as a credential.
func TestImportNeverStoresRedactionPlaceholders(t *testing.T) {
	f := newPersistFixture(t)
	f.seed(t)
	require.NoError(t, f.db.GetDB().Model(&database.Device{}).Where("mac = ?", "aabbccddeeff").
		Update("desired_config", persistSecretConfig).Error)
	archive := f.export(t)

	// A new record has no stored value to keep, so the redacted key is left out.
	f.wipe(t)
	fresh, err := f.importFile(t, archive, sync.ImportOptions{})
	require.NoError(t, err)
	var device database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&device).Error)
	assert.JSONEq(t, `{"mqtt":{"enable":true,"server":"broker.local"}}`, device.DesiredConfig)
	assert.NotContains(t, device.DesiredConfig, sync.RedactedValue)
	assert.Contains(t, fresh.Warnings[0], "1 redacted values", "the caller must be told what was left out")

	// An existing record keeps its stored secret when it is replaced.
	require.NoError(t, f.db.GetDB().Model(&database.Device{}).Where("mac = ?", "aabbccddeeff").
		Update("desired_config", persistSecretConfig).Error)
	_, err = f.importFile(t, archive, sync.ImportOptions{ForceOverwrite: true})
	require.NoError(t, err)
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&device).Error)
	assert.JSONEq(t, persistSecretConfig, device.DesiredConfig)
}

// The exporter redacts sensitive keys at any depth, arrays included. A forced import must put
// the stored value back for each one, and must never store the placeholder.
func TestForcedImportKeepsSecretsInsideArrays(t *testing.T) {
	const listed = `{"connections":[{"name":"a","password":"s1"},{"name":"b","password":"s2"}],` +
		`"groups":[[{"name":"c","password":"s3"}]]}`
	f := newPersistFixture(t)
	f.seed(t)
	require.NoError(t, f.db.GetDB().Model(&database.Device{}).Where("mac = ?", "aabbccddeeff").
		Update("desired_config", listed).Error)
	archive := f.export(t)

	var device database.Device
	_, err := f.importFile(t, archive, sync.ImportOptions{ForceOverwrite: true})
	require.NoError(t, err)
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&device).Error)
	assert.JSONEq(t, listed, device.DesiredConfig, "a replacement keeps the stored secret of each list element")

	// A new record has nothing to keep: the placeholder is left out, at any depth, and counted.
	f.wipe(t)
	result, err := f.importFile(t, archive, sync.ImportOptions{})
	require.NoError(t, err)
	var fresh database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&fresh).Error)
	assert.NotContains(t, fresh.DesiredConfig, sync.RedactedValue)
	assert.JSONEq(t, `{"connections":[{"name":"a"},{"name":"b"}],"groups":[[{"name":"c"}]]}`, fresh.DesiredConfig)
	assert.Contains(t, result.Warnings[0], "3 redacted values")
}

// Export falls back to the older device_configs row when the device has no desired configuration,
// so a replacement must look for the stored secrets in the same place.
func TestForcedImportKeepsSecretsFromTheLegacyConfigurationRow(t *testing.T) {
	const legacy = `{"mqtt":{"enable":true,"server":"broker.local","password":"s3cret"}}`
	f := newPersistFixture(t)
	f.seed(t)
	var device database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&device).Error)
	require.NoError(t, f.db.GetDB().Model(&device).Update("desired_config", "{}").Error)
	require.NoError(t, f.db.GetDB().Create(&configuration.DeviceConfig{
		DeviceID: device.ID, Config: json.RawMessage(legacy), SyncStatus: "pending",
	}).Error)
	archive := f.export(t)

	_, err := f.importFile(t, archive, sync.ImportOptions{ForceOverwrite: true})
	require.NoError(t, err)
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&device).Error)
	assert.JSONEq(t, legacy, device.DesiredConfig, "the legacy row's password must survive the replacement")
}

func TestForcedImportKeepsTemplateScopeAndReportsOldValues(t *testing.T) {
	f := newPersistFixture(t)
	f.seed(t)
	archive := f.export(t)
	require.NoError(t, f.db.GetDB().Model(&configuration.ConfigTemplate{}).Where("name = ?", "plug-defaults").
		Update("scope", "group").Error)
	require.NoError(t, f.db.GetDB().Model(&database.Device{}).Where("mac = ?", "aabbccddeeff").
		Update("name", "renamed").Error)

	result, err := f.importFile(t, archive, sync.ImportOptions{ForceOverwrite: true})

	require.NoError(t, err)
	var template configuration.ConfigTemplate
	require.NoError(t, f.db.GetDB().Where("name = ?", "plug-defaults").First(&template).Error)
	assert.Equal(t, "group", template.Scope, "the archive carries no scope, so a replacement keeps the stored one")
	var found bool
	for _, change := range result.Changes {
		if change.Resource == "device" {
			found = true
			assert.Equal(t, "update", change.Type)
			assert.Equal(t, "Device: renamed (Smart Plug)", change.OldValue)
			assert.Equal(t, "Device: tv-office (Smart Plug)", change.NewValue)
		}
	}
	assert.True(t, found, "the device update must be listed")
}

func TestImportWarnsWhenSystemSettingsAreDropped(t *testing.T) {
	archive := &SMAArchive{SystemSettings: SystemSettings{LogLevel: "debug"}}
	warned := importWarnings(archive, sync.ImportOptions{}, &sync.ImportApplyResult{})
	require.Len(t, warned, 1)
	assert.Contains(t, warned[0], "system settings")

	archive.SystemSettings = SystemSettings{LogLevel: defaultSystemLogLevel}
	assert.Empty(t, importWarnings(archive, sync.ImportOptions{}, &sync.ImportApplyResult{}),
		"the settings an export writes by default are not a loss")
}

func TestImportIsAllOrNothing(t *testing.T) {
	f := newPersistFixture(t)
	// A device already holds the IP that the second archive device wants.
	require.NoError(t, f.db.AddDevice(&database.Device{IP: "192.0.2.99", MAC: "000000000001", Name: "holder"}))

	data := validExportData()
	data.Devices = []sync.DeviceData{
		{ID: 1, MAC: "aabbccddeeff", IP: "192.0.2.50", Type: "switch", Name: "fine"},
		{ID: 2, MAC: "aabbccddee00", IP: "192.0.2.99", Type: "switch", Name: "collides"},
	}
	data.Templates = []sync.TemplateData{{Name: "t1", DeviceType: "all", Config: map[string]interface{}{"a": 1.0}}}
	exporter := initializedPlugin(t)
	exporter.SetExportBaseDir(f.dir)
	exported, err := exporter.Export(context.Background(), data, sync.ExportConfig{
		Format: "sma", Config: map[string]interface{}{"output_path": f.dir},
	})
	require.NoError(t, err)

	result, err := f.importFile(t, exported.OutputPath, sync.ImportOptions{})

	require.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "no changes were made")
	var count int64
	require.NoError(t, f.db.GetDB().Model(&database.Device{}).Where("mac = ?", "aabbccddeeff").Count(&count).Error)
	assert.Equal(t, int64(0), count, "the device written before the failure must be rolled back")
	require.NoError(t, f.db.GetDB().Model(&configuration.ConfigTemplate{}).Where("name = ?", "t1").Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestValidateOnlyWritesNothing(t *testing.T) {
	f := newPersistFixture(t)
	f.seed(t)
	archive := f.export(t)
	f.wipe(t)

	result, err := f.importFile(t, archive, sync.ImportOptions{ValidateOnly: true})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, true, result.Metadata["validate_only"])
	var count int64
	require.NoError(t, f.db.GetDB().Model(&database.Device{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestBackupBeforeIsReportedAsIgnored(t *testing.T) {
	f := newPersistFixture(t)
	f.seed(t)
	archive := f.export(t)
	f.wipe(t)

	result, err := f.importFile(t, archive, sync.ImportOptions{BackupBefore: true})

	require.NoError(t, err)
	assert.Contains(t, result.Warnings, "backup_before is not supported and was ignored; no backup was taken")
}

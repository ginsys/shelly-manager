package sma

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

func validExportData() *sync.ExportData {
	now := time.Date(2026, 1, 2, 3, 4, 5, 123, time.UTC)
	return &sync.ExportData{
		Devices: []sync.DeviceData{{
			ID: 1, MAC: "aabbccddeeff", IP: "192.0.2.1", Type: "switch",
			Name: "Kitchen", Model: "Shelly", Firmware: "1", Status: "online",
			LastSeen: now, Settings: nil, CreatedAt: now, UpdatedAt: now,
		}},
		Metadata: sync.ExportMetadata{
			ExportID: "123e4567-e89b-42d3-a456-426614174000", RequestedBy: "shelly-manager",
			ExportType: "manual", SystemVersion: "test", DatabaseType: "sqlite",
		},
		Timestamp: now,
	}
}

func initializedPlugin(t *testing.T) *SMAPlugin {
	t.Helper()
	plugin := NewPlugin().(*SMAPlugin)
	require.NoError(t, plugin.Initialize(logging.GetDefault()))
	return plugin
}

func generatedArchive(t *testing.T) ([]byte, map[string]interface{}) {
	t.Helper()
	plugin := initializedPlugin(t)
	tree, _, err := plugin.prepareArchive(validExportData(), sync.ExportConfig{
		Config: map[string]interface{}{"include_discovered": true},
	})
	require.NoError(t, err)
	data, _, err := finalizeArchiveTree(tree)
	require.NoError(t, err)
	return data, tree
}

func TestGenerateAndDryRunImportRoundTrip(t *testing.T) {
	plugin := initializedPlugin(t)
	output := t.TempDir()
	result, err := plugin.Export(context.Background(), validExportData(), sync.ExportConfig{
		Format: "sma",
		Config: map[string]interface{}{"output_path": output, "compression_level": float64(6)},
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 1, result.RecordCount)

	file, err := os.Open(result.OutputPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	header := make([]byte, 2)
	_, err = io.ReadFull(file, header)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x1f, 0x8b}, header)

	importResult, err := plugin.ImportFromFile(context.Background(), result.OutputPath, sync.ImportConfig{
		Options: sync.ImportOptions{DryRun: true, ValidateOnly: true},
	})
	require.NoError(t, err)
	assert.True(t, importResult.Success)
	assert.Equal(t, 1, importResult.RecordsImported)
}

func TestRawAndGzipNormalizationLimits(t *testing.T) {
	raw := []byte(`{"ok":true}`)
	normalized, err := normalizeSMAInput(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)
	assert.Equal(t, raw, normalized)

	_, err = normalizeSMAInput(bytes.NewReader(raw), int64(len(raw)-1))
	require.Error(t, err)

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err = writer.Write(raw)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	normalized, err = normalizeSMAInput(bytes.NewReader(compressed.Bytes()), int64(len(raw)))
	require.NoError(t, err)
	assert.Equal(t, raw, normalized)

	truncated := compressed.Bytes()[:compressed.Len()-2]
	_, err = normalizeSMAInput(bytes.NewReader(truncated), int64(len(raw)+1))
	require.Error(t, err)

	malformed := []byte{0x1f, 0x8b, 0, 1}
	_, err = normalizeSMAInput(bytes.NewReader(malformed), int64(len(raw)+1))
	require.EqualError(t, err, "malformed gzip input")
}

func TestStrictJSONRejectsDuplicatesSurrogatesAndDepth(t *testing.T) {
	_, err := parseStrictJSON([]byte(`{"a":1,"a":2}`), 64)
	require.ErrorContains(t, err, "duplicate")
	_, err = parseStrictJSON([]byte(`{"a":"\ud800"}`), 64)
	require.ErrorContains(t, err, "surrogate")

	value := `0`
	for range 63 {
		value = `[` + value + `]`
	}
	_, err = parseStrictJSON([]byte(value), 64)
	require.NoError(t, err)
	value = `[` + value + `]`
	_, err = parseStrictJSON([]byte(value), 64)
	require.NoError(t, err)
	_, err = parseStrictJSON([]byte(`[`+value+`]`), 64)
	require.ErrorContains(t, err, "depth")
}

func TestStrictJSONSafeNumberProfile(t *testing.T) {
	for _, value := range []string{
		`5e-324`,
		`9007199254740991`,
		`-9007199254740991`,
	} {
		tree, err := parseStrictJSON([]byte(value), 64)
		require.NoError(t, err, value)
		require.NoError(t, validateSafeNumbers(tree), value)
	}
	for _, value := range []string{
		`1.7976931348623157e308`,
		`1e309`,
		`1e-400`,
		`-1e-400`,
		`9007199254740992`,
		`-9007199254740992`,
	} {
		tree, err := parseStrictJSON([]byte(value), 64)
		require.NoError(t, err, value)
		require.Error(t, validateSafeNumbers(tree), value)
	}
}

func TestChecksumAndClosedSchemaAreEnforced(t *testing.T) {
	data, _ := generatedArchive(t)
	plugin := initializedPlugin(t)
	_, err := plugin.ImportFromData(context.Background(), data, sync.ImportConfig{
		Options: sync.ImportOptions{DryRun: true},
	})
	require.NoError(t, err)

	var tree map[string]interface{}
	tree = nil
	require.NoError(t, json.Unmarshal(data, &tree))
	tree["extra"] = true
	modified, err := json.Marshal(tree)
	require.NoError(t, err)
	_, err = plugin.ImportFromData(context.Background(), modified, sync.ImportConfig{
		Options: sync.ImportOptions{DryRun: true},
	})
	require.ErrorIs(t, err, sync.ErrInvalidImportData)

	tree = nil
	require.NoError(t, json.Unmarshal(data, &tree))
	tree["devices"] = nil
	modified, err = json.Marshal(tree)
	require.NoError(t, err)
	_, err = plugin.ImportFromData(context.Background(), modified, sync.ImportConfig{
		Options: sync.ImportOptions{DryRun: true},
	})
	require.ErrorIs(t, err, sync.ErrInvalidImportData)
	require.ErrorContains(t, err, "non-null array")

	require.NoError(t, json.Unmarshal(data, &tree))
	tree["format_version"] = "2024.1"
	modified, err = json.Marshal(tree)
	require.NoError(t, err)
	_, err = plugin.ImportFromData(context.Background(), modified, sync.ImportConfig{
		Options: sync.ImportOptions{DryRun: true},
	})
	require.ErrorIs(t, err, sync.ErrInvalidImportData)
	require.ErrorContains(t, err, "2026.1")
}

func TestConfigurationJoiningAndNilMapEquivalence(t *testing.T) {
	data := validExportData()
	nested := sync.ConfigurationData{
		DeviceID: 1, Config: nil, SyncStatus: "", UpdatedAt: data.Timestamp,
	}
	data.Devices[0].Configuration = &nested
	data.Configurations = []sync.ConfigurationData{{
		DeviceID: 1, Config: map[string]interface{}{}, SyncStatus: "", UpdatedAt: data.Timestamp,
	}}
	plugin := initializedPlugin(t)
	_, prepared, err := plugin.prepareArchive(data, sync.ExportConfig{})
	require.NoError(t, err)
	assert.Equal(t, 1, prepared.recordCount)

	data.Configurations[0].Config["different"] = true
	_, _, err = plugin.prepareArchive(data, sync.ExportConfig{})
	require.ErrorContains(t, err, "conflicting")
	assert.NotErrorIs(t, err, sync.ErrInvalidExportData)
}

func TestConfigurationJoinRejectsStoredDataInconsistencies(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sync.ExportData)
		match  string
	}{
		{
			name: "duplicate device",
			mutate: func(data *sync.ExportData) {
				data.Devices = append(data.Devices, data.Devices[0])
			},
			match: "duplicate device",
		},
		{
			name: "nested id mismatch",
			mutate: func(data *sync.ExportData) {
				data.Devices[0].Configuration = &sync.ConfigurationData{DeviceID: 2}
			},
			match: "configuration for device",
		},
		{
			name: "duplicate standalone",
			mutate: func(data *sync.ExportData) {
				data.Configurations = []sync.ConfigurationData{{DeviceID: 1}, {DeviceID: 1}}
			},
			match: "duplicate standalone",
		},
		{
			name: "orphan standalone",
			mutate: func(data *sync.ExportData) {
				data.Configurations = []sync.ConfigurationData{{DeviceID: 2}}
			},
			match: "orphan standalone",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := validExportData()
			tt.mutate(data)
			_, _, err := initializedPlugin(t).prepareArchive(data, sync.ExportConfig{})
			require.ErrorContains(t, err, tt.match)
			require.NotErrorIs(t, err, sync.ErrInvalidExportData)
		})
	}
}

func TestDiscoveredOnlyArchiveIsValidAndEmptyIsNot(t *testing.T) {
	data := validExportData()
	data.Devices = nil
	data.DiscoveredDevices = []sync.DiscoveredDeviceData{{
		MAC: "aabb", Discovered: data.Timestamp,
	}}
	plugin := initializedPlugin(t)
	_, prepared, err := plugin.prepareArchive(data, sync.ExportConfig{
		Config: map[string]interface{}{"include_discovered": true},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, prepared.recordCount)

	_, _, err = plugin.prepareArchive(data, sync.ExportConfig{
		Config: map[string]interface{}{"include_discovered": false},
	})
	require.ErrorIs(t, err, sync.ErrInvalidExportData)
}

func TestSafeBooleanDefaultsAndOverrides(t *testing.T) {
	data := validExportData()
	data.Devices[0].Settings = map[string]interface{}{
		"password": "secret",
		"nested":   map[string]interface{}{"api_key": "key"},
	}
	data.DiscoveredDevices = []sync.DiscoveredDeviceData{{
		MAC: "discovered", Discovered: data.Timestamp,
	}}
	plugin := initializedPlugin(t)

	tree, prepared, err := plugin.prepareArchive(data, sync.ExportConfig{})
	require.NoError(t, err)
	require.Equal(t, 2, prepared.recordCount)
	settings := tree["devices"].([]interface{})[0].(map[string]interface{})["settings"].(map[string]interface{})
	require.Equal(t, "[REDACTED]", settings["password"])
	require.Equal(t, "[REDACTED]", settings["nested"].(map[string]interface{})["api_key"])

	tree, prepared, err = plugin.prepareArchive(data, sync.ExportConfig{
		Config: map[string]interface{}{
			"include_discovered": false,
			"exclude_sensitive":  false,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, prepared.recordCount)
	settings = tree["devices"].([]interface{})[0].(map[string]interface{})["settings"].(map[string]interface{})
	require.Equal(t, "secret", settings["password"])
	require.Empty(t, tree["discovered_devices"])

	for _, name := range []string{"include_discovered", "exclude_sensitive"} {
		t.Run(name+"_type", func(t *testing.T) {
			config := map[string]interface{}{name: "true"}
			require.ErrorContains(t, plugin.ValidateConfig(config), "must be a boolean")
			_, _, err := plugin.prepareArchive(data, sync.ExportConfig{Config: config})
			require.ErrorContains(t, err, "must be a boolean")
		})
	}
}

func TestRequiredNilCollectionsAndMapsNormalize(t *testing.T) {
	data := validExportData()
	data.Devices[0].Settings = nil
	tree, _, err := initializedPlugin(t).prepareArchive(data, sync.ExportConfig{})
	require.NoError(t, err)
	require.IsType(t, []interface{}{}, tree["templates"])
	require.IsType(t, []interface{}{}, tree["discovered_devices"])
	device := tree["devices"].([]interface{})[0].(map[string]interface{})
	require.Equal(t, map[string]interface{}{}, device["settings"])
	require.Equal(t, []interface{}{}, tree["plugin_configurations"])
}

func TestPreviewUsesJoinedRecordSemantics(t *testing.T) {
	data := validExportData()
	data.Configurations = []sync.ConfigurationData{{
		DeviceID: 1, Config: map[string]interface{}{}, UpdatedAt: data.Timestamp,
	}}
	preview, err := initializedPlugin(t).Preview(context.Background(), data, sync.ExportConfig{
		Config: map[string]interface{}{"compression_level": float64(9)},
	})
	require.NoError(t, err)
	require.Equal(t, 1, preview.RecordCount)
	require.NotZero(t, preview.EstimatedSize)
}

type countingMarshaler struct {
	calls *int
}

func (value countingMarshaler) MarshalJSON() ([]byte, error) {
	*value.calls++
	return []byte(`{"value":1}`), nil
}

func TestCustomJSONMarshalerIsInvokedExactlyOnce(t *testing.T) {
	calls := 0
	data := validExportData()
	data.Devices[0].Settings = map[string]interface{}{
		"custom": countingMarshaler{calls: &calls},
	}
	_, _, err := initializedPlugin(t).prepareArchive(data, sync.ExportConfig{})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestGeneratedTreeRejectsExcessiveDepthAsOperationalError(t *testing.T) {
	var nested interface{} = "leaf"
	for range 60 {
		nested = []interface{}{nested}
	}
	data := validExportData()
	data.Devices[0].Settings = map[string]interface{}{"deep": nested}
	_, _, err := initializedPlugin(t).prepareArchive(data, sync.ExportConfig{})
	require.NoError(t, err)

	nested = []interface{}{nested}
	data.Devices[0].Settings = map[string]interface{}{"deep": nested}
	_, _, err = initializedPlugin(t).prepareArchive(data, sync.ExportConfig{})
	require.ErrorContains(t, err, "depth")
	require.NotErrorIs(t, err, sync.ErrInvalidExportData)
}

func TestNonDryRunFailsClosed(t *testing.T) {
	data, _ := generatedArchive(t)
	plugin := initializedPlugin(t)
	_, err := plugin.ImportFromData(context.Background(), data, sync.ImportConfig{})
	require.ErrorIs(t, err, sync.ErrImportNotImplemented)
}

func TestValidateConfigHasNoFilesystemSideEffect(t *testing.T) {
	plugin := initializedPlugin(t)
	path := filepath.Join(t.TempDir(), "not-created")
	require.NoError(t, plugin.ValidateConfig(map[string]interface{}{"output_path": path}))
	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestCapabilitiesAdvertiseNormalizedLimit(t *testing.T) {
	assert.Equal(t, defaultNormalizedLimit, initializedPlugin(t).Capabilities().MaxDataSize)
}

func TestConfigSchemaAdvertisesOnlyEffectiveCreationOptions(t *testing.T) {
	properties := initializedPlugin(t).ConfigSchema().Properties
	require.Contains(t, properties, "include_discovered")
	require.Contains(t, properties, "exclude_sensitive")
	require.NotContains(t, properties, "include_checksums")
	require.NotContains(t, properties, "include_network_settings")
	require.NotContains(t, properties, "include_plugin_configs")
	require.NotContains(t, properties, "include_system_settings")
}

type failingAtomicFile struct {
	*os.File
	syncErr  error
	closeErr error
}

func (file *failingAtomicFile) Sync() error {
	if file.syncErr != nil {
		return file.syncErr
	}
	return file.File.Sync()
}

func (file *failingAtomicFile) Close() error {
	err := file.File.Close()
	if file.closeErr != nil {
		return file.closeErr
	}
	return err
}

type failingFileOperations struct {
	syncErr   error
	closeErr  error
	renameErr error
}

func (operations failingFileOperations) CreateTemp(dir, pattern string) (atomicFile, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &failingAtomicFile{File: file, syncErr: operations.syncErr, closeErr: operations.closeErr}, nil
}

func (operations failingFileOperations) Rename(oldPath, newPath string) error {
	if operations.renameErr != nil {
		return operations.renameErr
	}
	return os.Rename(oldPath, newPath)
}

func (failingFileOperations) Remove(path string) error { return os.Remove(path) }

func TestAtomicPublicationCleansUpEveryInjectedFailure(t *testing.T) {
	failure := errors.New("injected failure")
	tests := []struct {
		name       string
		operations failingFileOperations
	}{
		{name: "sync", operations: failingFileOperations{syncErr: failure}},
		{name: "close", operations: failingFileOperations{closeErr: failure}},
		{name: "rename", operations: failingFileOperations{renameErr: failure}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			finalPath := filepath.Join(dir, "archive.sma")
			plugin := initializedPlugin(t)
			plugin.files = tt.operations
			_, err := plugin.publishAtomic(finalPath, []byte(`{"ok":true}`), 6)
			require.ErrorIs(t, err, failure)
			_, statErr := os.Stat(finalPath)
			require.ErrorIs(t, statErr, os.ErrNotExist)
			entries, readErr := os.ReadDir(dir)
			require.NoError(t, readErr)
			for _, entry := range entries {
				require.False(t, strings.HasPrefix(entry.Name(), ".sma-"), entry.Name())
			}
		})
	}
}

func TestSharedCanonicalArchiveAndDigest(t *testing.T) {
	base := filepath.Join("..", "..", "..", "..", "testdata", "sma")
	canonical, err := os.ReadFile(filepath.Join(base, "archive-2026.1.canonical.json"))
	require.NoError(t, err)
	require.NotEmpty(t, canonical)
	require.NotEqual(t, byte('\n'), canonical[len(canonical)-1])

	tree, err := parseStrictJSON(canonical, 64)
	require.NoError(t, err)
	require.NoError(t, validateArchiveTree(tree))
	roundTrip, err := canonicalizeTree(tree)
	require.NoError(t, err)
	require.Equal(t, canonical, roundTrip)

	sidecar, err := os.ReadFile(filepath.Join(base, "archive-2026.1.sha256"))
	require.NoError(t, err)
	root := tree.(map[string]interface{})
	integrity := root["metadata"].(map[string]interface{})["integrity"].(map[string]interface{})
	supplied := integrity["checksum"]
	require.Equal(t, strings.TrimSpace(string(sidecar)), supplied)
	integrity["checksum"] = ""
	checksumInput, err := canonicalizeTree(root)
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(string(sidecar)), checksumBytes(checksumInput))
	integrity["checksum"] = supplied

	result, err := initializedPlugin(t).ImportFromData(context.Background(), canonical, sync.ImportConfig{
		Options: sync.ImportOptions{DryRun: true, ValidateOnly: true},
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.RecordsImported)
}

func TestSharedNumericAndIdentityVectors(t *testing.T) {
	base := filepath.Join("..", "..", "..", "..", "testdata", "sma")
	var numbers []struct {
		Text     string `json:"text"`
		Binary64 string `json:"binary64"`
		Admitted bool   `json:"admitted"`
	}
	raw, err := os.ReadFile(filepath.Join(base, "numeric-vectors.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &numbers))
	for _, vector := range numbers {
		t.Run("number_"+vector.Text, func(t *testing.T) {
			number, _ := strconv.ParseFloat(vector.Text, 64)
			require.Equal(t, vector.Binary64, fmt.Sprintf("%016x", math.Float64bits(number)))
			tree, parseErr := parseStrictJSON([]byte(vector.Text), 64)
			require.NoError(t, parseErr)
			admissionErr := validateSafeNumbers(tree)
			require.Equal(t, vector.Admitted, admissionErr == nil, admissionErr)
		})
	}

	var identities struct {
		Timestamps []struct {
			Value string `json:"value"`
			Valid bool   `json:"valid"`
		} `json:"timestamps"`
		UUIDs []struct {
			Value string `json:"value"`
			Valid bool   `json:"valid"`
		} `json:"uuids"`
	}
	raw, err = os.ReadFile(filepath.Join(base, "identity-vectors.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &identities))
	for _, vector := range identities.Timestamps {
		err := requireTimestamp(vector.Value, "timestamp")
		require.Equal(t, vector.Valid, err == nil, vector.Value)
	}
	for _, vector := range identities.UUIDs {
		require.Equal(t, vector.Valid, isLowerUUID(vector.Value), vector.Value)
	}
}

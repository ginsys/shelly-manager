package sma

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

func TestSMAPlugin_Info(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	info := plugin.Info()

	assert.Equal(t, "sma", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.SupportedFormats, "sma")
	assert.Equal(t, sync.CategoryBackup, info.Category)
}

func TestSMAPlugin_ConfigSchema(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	schema := plugin.ConfigSchema()

	assert.Equal(t, "1.0", schema.Version)
	assert.Contains(t, schema.Properties, "output_path")
	assert.Contains(t, schema.Properties, "compression_level")
	assert.Contains(t, schema.Properties, "include_discovered")
	assert.Contains(t, schema.Properties, "exclude_sensitive")

	// Check default values
	assert.Equal(t, "/var/backups/shelly-manager", schema.Properties["output_path"].Default)
	assert.Equal(t, 6.0, schema.Properties["compression_level"].Default)
	assert.Equal(t, true, schema.Properties["include_discovered"].Default)
	assert.Equal(t, true, schema.Properties["exclude_sensitive"].Default)
}

func TestSMAPlugin_ValidateConfig(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"output_path":       "/tmp/test",
				"compression_level": 6.0,
			},
			wantError: false,
		},
		{
			name: "invalid compression level - too low",
			config: map[string]interface{}{
				"compression_level": 0.0,
			},
			wantError: true,
		},
		{
			name: "invalid compression level - too high",
			config: map[string]interface{}{
				"compression_level": 10.0,
			},
			wantError: true,
		},
		{
			name: "invalid compression level - not number",
			config: map[string]interface{}{
				"compression_level": "invalid",
			},
			wantError: true,
		},
		{
			name: "invalid output_path - not string",
			config: map[string]interface{}{
				"output_path": 123,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.ValidateConfig(tt.config)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSMAPlugin_Export(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Create plugin and initialize
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	// Create test data
	testData := &sync.ExportData{
		Devices: []sync.DeviceData{
			{
				ID:       1,
				MAC:      "AA:BB:CC:DD:EE:FF",
				IP:       "192.168.1.100",
				Type:     "shelly1",
				Name:     "Test Switch",
				Model:    "SHSW-1",
				Firmware: "20231215-111232/v1.14.1-rc1",
				Status:   "online",
				LastSeen: time.Now(),
				Settings: map[string]interface{}{
					"name": "Test Switch",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Templates: []sync.TemplateData{
			{
				ID:          1,
				Name:        "Test Template",
				Description: "A test template",
				DeviceType:  "shelly1",
				Generation:  1,
				Config:      map[string]interface{}{"test": "value"},
				Variables:   map[string]interface{}{"var": "test"},
				IsDefault:   true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
		DiscoveredDevices: []sync.DiscoveredDeviceData{
			{
				MAC:        "BB:CC:DD:EE:FF:AA",
				SSID:       "ShellySwitch-123456",
				Model:      "SHSW-25",
				Generation: 1,
				IP:         "192.168.1.101",
				Signal:     -45,
				AgentID:    "agent-001",
				Discovered: time.Now(),
			},
		},
		Metadata: sync.ExportMetadata{
			ExportID:      "test-export-123",
			RequestedBy:   "test-user",
			ExportType:    "manual",
			TotalDevices:  1,
			TotalConfigs:  1,
			SystemVersion: "v0.5.4-alpha",
			DatabaseType:  "sqlite",
		},
		Timestamp: time.Now(),
	}

	// Create export config
	config := sync.ExportConfig{
		PluginName: "sma",
		Format:     "sma",
		Config: map[string]interface{}{
			"output_path":        tempDir,
			"compression_level":  6.0,
			"include_discovered": true,
			"exclude_sensitive":  true,
		},
	}

	// Perform export
	ctx := context.Background()
	result, err := plugin.Export(ctx, testData, config)

	// Verify result
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "sma", result.PluginName)
	assert.Equal(t, "sma", result.Format)
	assert.Greater(t, result.RecordCount, 0)
	assert.Greater(t, result.FileSize, int64(0))
	assert.NotEmpty(t, result.Checksum)
	assert.Contains(t, result.OutputPath, tempDir)

	// Verify file was created
	assert.FileExists(t, result.OutputPath)

	// Verify file is compressed and contains valid JSON
	file, err := os.Open(result.OutputPath)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	gzipReader, err := gzip.NewReader(file)
	require.NoError(t, err)
	defer func() { _ = gzipReader.Close() }()

	jsonData, err := io.ReadAll(gzipReader)
	require.NoError(t, err)

	var archive SMAArchive
	err = json.Unmarshal(jsonData, &archive)
	require.NoError(t, err)

	// Verify archive structure
	assert.Equal(t, SMAVersion, archive.SMAVersion)
	assert.Equal(t, FormatVersion, archive.FormatVersion)
	assert.Equal(t, testData.Metadata.ExportID, archive.Metadata.ExportID)
	assert.Len(t, archive.Devices, 1)
	assert.Len(t, archive.Templates, 1)
	assert.Len(t, archive.Discovered, 1)
	assert.Equal(t, testData.Devices[0].MAC, archive.Devices[0].MAC)
}

func TestSMAPlugin_Preview(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	testData := &sync.ExportData{
		Devices:           make([]sync.DeviceData, 5),
		Templates:         make([]sync.TemplateData, 2),
		DiscoveredDevices: make([]sync.DiscoveredDeviceData, 3),
		Configurations:    make([]sync.ConfigurationData, 5),
		Metadata: sync.ExportMetadata{
			ExportID:      "preview-test",
			TotalDevices:  5,
			TotalConfigs:  5,
			SystemVersion: "v0.5.4-alpha",
		},
	}

	config := sync.ExportConfig{
		Config: map[string]interface{}{
			"include_discovered": true,
			"compression_level":  6.0,
		},
	}

	ctx := context.Background()
	result, err := plugin.Preview(ctx, testData, config)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, result.RecordCount, 0)
	assert.Greater(t, result.EstimatedSize, int64(0))
	assert.NotEmpty(t, result.SampleData)

	preview := string(result.SampleData)
	assert.Contains(t, preview, "SMA Archive Preview")
	assert.Contains(t, preview, "Devices: 5")
	assert.Contains(t, preview, "Templates: 2")
	assert.Contains(t, preview, "Discovered Devices: 3")
	assert.Contains(t, preview, SMAVersion)
}

func TestSMAPlugin_ImportFromData_DryRun(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	// Create test SMA archive
	archive := SMAArchive{
		SMAVersion:    SMAVersion,
		FormatVersion: FormatVersion,
		Metadata: SMAMetadata{
			ExportID:  "test-import",
			CreatedAt: time.Now(),
			CreatedBy: "test-user",
			Integrity: IntegrityInfo{
				RecordCount: 3,
			},
		},
		Devices: []sync.DeviceData{
			{
				MAC:  "AA:BB:CC:DD:EE:FF",
				Name: "Test Device",
				Type: "shelly1",
			},
		},
		Templates: []sync.TemplateData{
			{
				Name:       "Test Template",
				DeviceType: "shelly1",
			},
		},
		Discovered: []sync.DiscoveredDeviceData{
			{
				MAC:   "BB:CC:DD:EE:FF:AA",
				Model: "SHSW-25",
			},
		},
	}

	jsonData, err := json.Marshal(archive)
	require.NoError(t, err)

	config := sync.ImportConfig{
		Options: sync.ImportOptions{
			DryRun: true,
		},
	}

	ctx := context.Background()
	result, err := plugin.ImportFromData(ctx, jsonData, config)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "sma", result.PluginName)
	assert.Equal(t, "sma", result.Format)
	assert.Equal(t, 3, result.RecordsImported)
	assert.Len(t, result.Changes, 3)
	assert.Contains(t, result.Warnings[0], "dry run")
}

func TestSMAPlugin_ValidateSMAFormat(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	tests := []struct {
		name      string
		archive   SMAArchive
		wantError bool
	}{
		{
			name: "valid archive",
			archive: SMAArchive{
				SMAVersion:    "1.0",
				FormatVersion: "2024.1",
				Metadata: SMAMetadata{
					ExportID:  "test",
					CreatedAt: time.Now(),
				},
				Devices: []sync.DeviceData{
					{MAC: "AA:BB:CC:DD:EE:FF"},
				},
			},
			wantError: false,
		},
		{
			name: "missing SMA version",
			archive: SMAArchive{
				FormatVersion: "2024.1",
				Metadata: SMAMetadata{
					ExportID:  "test",
					CreatedAt: time.Now(),
				},
			},
			wantError: true,
		},
		{
			name: "unsupported SMA version",
			archive: SMAArchive{
				SMAVersion:    "2.0",
				FormatVersion: "2024.1",
				Metadata: SMAMetadata{
					ExportID:  "test",
					CreatedAt: time.Now(),
				},
			},
			wantError: true,
		},
		{
			name: "missing export ID",
			archive: SMAArchive{
				SMAVersion:    "1.0",
				FormatVersion: "2024.1",
				Metadata: SMAMetadata{
					CreatedAt: time.Now(),
				},
			},
			wantError: true,
		},
		{
			name: "empty archive",
			archive: SMAArchive{
				SMAVersion:    "1.0",
				FormatVersion: "2024.1",
				Metadata: SMAMetadata{
					ExportID:  "test",
					CreatedAt: time.Now(),
				},
				Devices:   []sync.DeviceData{},
				Templates: []sync.TemplateData{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.validateSMAFormat(&tt.archive)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSMAPlugin_ExcludeSensitiveData(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	archive := SMAArchive{
		NetworkSettings: &NetworkSettings{
			MQTTConfig: &MQTTConfig{
				Username: "secret-username",
			},
		},
		PluginConfigs: []PluginConfiguration{
			{
				PluginName: "test-plugin",
				Config: map[string]interface{}{
					"api_key":  "secret-key",
					"password": "secret-password",
					"username": "normal-username",
				},
			},
		},
		Devices: []sync.DeviceData{
			{
				Settings: map[string]interface{}{
					"wifi_password": "secret-wifi",
					"device_name":   "normal-name",
				},
			},
		},
	}

	plugin.excludeSensitiveData(&archive)

	// Check that sensitive fields were redacted
	assert.Equal(t, "[REDACTED]", archive.NetworkSettings.MQTTConfig.Username)
	assert.Equal(t, "[REDACTED]", archive.PluginConfigs[0].Config["api_key"])
	assert.Equal(t, "[REDACTED]", archive.PluginConfigs[0].Config["password"])
	assert.Equal(t, "normal-username", archive.PluginConfigs[0].Config["username"]) // Not sensitive
	assert.Equal(t, "[REDACTED]", archive.Devices[0].Settings["wifi_password"])
	assert.Equal(t, "normal-name", archive.Devices[0].Settings["device_name"]) // Not sensitive
}

func TestSMAPlugin_IsSensitiveField(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	sensitiveFields := []string{
		"password", "passwd", "pwd", "secret", "key", "token",
		"api_key", "apikey", "auth", "credential", "private",
		"PASSWORD", "Secret", "API_KEY", "wifi_password",
	}

	normalFields := []string{
		"username", "name", "id", "type", "status",
		"hostname", "port", "timeout", "enabled",
	}

	for _, field := range sensitiveFields {
		assert.True(t, plugin.isSensitiveField(field),
			"Field '%s' should be considered sensitive", field)
	}

	for _, field := range normalFields {
		assert.False(t, plugin.isSensitiveField(field),
			"Field '%s' should not be considered sensitive", field)
	}
}

func TestSMAPlugin_Capabilities(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	caps := plugin.Capabilities()

	assert.False(t, caps.SupportsIncremental)
	assert.True(t, caps.SupportsScheduling)
	assert.False(t, caps.RequiresAuthentication)
	assert.Contains(t, caps.SupportedOutputs, "file")
	assert.Greater(t, caps.MaxDataSize, int64(0))
	assert.Equal(t, 1, caps.ConcurrencyLevel)
}

func TestSMAPlugin_ImportFromFile_FileNotExists(t *testing.T) {
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	config := sync.ImportConfig{}
	ctx := context.Background()

	result, err := plugin.ImportFromFile(ctx, "/nonexistent/file.sma", config)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestSMAPlugin_Integration_ExportImport(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create plugin
	plugin := NewPlugin().(*SMAPlugin)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	err := plugin.Initialize(logger)
	require.NoError(t, err)

	// Create test data
	originalData := &sync.ExportData{
		Devices: []sync.DeviceData{
			{
				MAC:  "AA:BB:CC:DD:EE:FF",
				Name: "Integration Test Device",
				Type: "shelly1",
			},
		},
		Templates: []sync.TemplateData{
			{
				Name:       "Integration Test Template",
				DeviceType: "shelly1",
			},
		},
		Metadata: sync.ExportMetadata{
			ExportID:      "integration-test",
			RequestedBy:   "test",
			ExportType:    "manual",
			SystemVersion: "v0.5.4-alpha",
			DatabaseType:  "sqlite",
		},
		Timestamp: time.Now(),
	}

	// Export
	exportConfig := sync.ExportConfig{
		PluginName: "sma",
		Format:     "sma",
		Config: map[string]interface{}{
			"output_path":        tempDir,
			"compression_level":  6.0,
			"include_discovered": false,
		},
	}

	ctx := context.Background()
	exportResult, err := plugin.Export(ctx, originalData, exportConfig)
	require.NoError(t, err)
	require.True(t, exportResult.Success)

	// Import (dry run)
	importConfig := sync.ImportConfig{
		Options: sync.ImportOptions{
			DryRun: true,
		},
	}

	importResult, err := plugin.ImportFromFile(ctx, exportResult.OutputPath, importConfig)
	require.NoError(t, err)
	require.True(t, importResult.Success)

	// Verify import detected correct number of records
	assert.Equal(t, 2, importResult.RecordsImported) // 1 device + 1 template
	assert.Len(t, importResult.Changes, 2)

	// Verify changes contain expected resources
	var deviceChange, templateChange *sync.ImportChange
	for _, change := range importResult.Changes {
		switch change.Resource {
		case "device":
			deviceChange = &change
		case "template":
			templateChange = &change
		}
	}

	require.NotNil(t, deviceChange)
	require.NotNil(t, templateChange)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", deviceChange.ResourceID)
	assert.Equal(t, "Integration Test Template", templateChange.ResourceID)
}

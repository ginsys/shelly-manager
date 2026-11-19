package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Mock Shelly client for testing
type mockShellyClient struct {
	mock.Mock
}

func (m *mockShellyClient) GetInfo(ctx context.Context) (*shelly.DeviceInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*shelly.DeviceInfo), args.Error(1)
}

func (m *mockShellyClient) GetStatus(ctx context.Context) (*shelly.DeviceStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*shelly.DeviceStatus), args.Error(1)
}

func (m *mockShellyClient) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*shelly.DeviceConfig), args.Error(1)
}

func (m *mockShellyClient) SetConfig(ctx context.Context, config map[string]interface{}) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockShellyClient) SetAuth(ctx context.Context, username, password string) error {
	args := m.Called(ctx, username, password)
	return args.Error(0)
}

func (m *mockShellyClient) ResetAuth(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockShellyClient) SetSwitch(ctx context.Context, channel int, on bool) error {
	args := m.Called(ctx, channel, on)
	return args.Error(0)
}

func (m *mockShellyClient) SetBrightness(ctx context.Context, channel int, brightness int) error {
	args := m.Called(ctx, channel, brightness)
	return args.Error(0)
}

func (m *mockShellyClient) SetColorRGB(ctx context.Context, channel int, r, g, b uint8) error {
	args := m.Called(ctx, channel, r, g, b)
	return args.Error(0)
}

func (m *mockShellyClient) SetColorTemp(ctx context.Context, channel int, temp int) error {
	args := m.Called(ctx, channel, temp)
	return args.Error(0)
}

// Roller Shutter Operations
func (m *mockShellyClient) SetRollerPosition(ctx context.Context, channel int, position int) error {
	args := m.Called(ctx, channel, position)
	return args.Error(0)
}

func (m *mockShellyClient) OpenRoller(ctx context.Context, channel int) error {
	args := m.Called(ctx, channel)
	return args.Error(0)
}

func (m *mockShellyClient) CloseRoller(ctx context.Context, channel int) error {
	args := m.Called(ctx, channel)
	return args.Error(0)
}

func (m *mockShellyClient) StopRoller(ctx context.Context, channel int) error {
	args := m.Called(ctx, channel)
	return args.Error(0)
}

// Advanced Settings
func (m *mockShellyClient) SetRelaySettings(ctx context.Context, channel int, settings map[string]interface{}) error {
	args := m.Called(ctx, channel, settings)
	return args.Error(0)
}

func (m *mockShellyClient) SetLightSettings(ctx context.Context, channel int, settings map[string]interface{}) error {
	args := m.Called(ctx, channel, settings)
	return args.Error(0)
}

func (m *mockShellyClient) SetInputSettings(ctx context.Context, input int, settings map[string]interface{}) error {
	args := m.Called(ctx, input, settings)
	return args.Error(0)
}

func (m *mockShellyClient) SetLEDSettings(ctx context.Context, settings map[string]interface{}) error {
	args := m.Called(ctx, settings)
	return args.Error(0)
}

// RGBW Operations
func (m *mockShellyClient) SetWhiteChannel(ctx context.Context, channel int, brightness int, temp int) error {
	args := m.Called(ctx, channel, brightness, temp)
	return args.Error(0)
}

func (m *mockShellyClient) SetColorMode(ctx context.Context, mode string) error {
	args := m.Called(ctx, mode)
	return args.Error(0)
}

func (m *mockShellyClient) Reboot(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockShellyClient) FactoryReset(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockShellyClient) CheckUpdate(ctx context.Context) (*shelly.UpdateInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*shelly.UpdateInfo), args.Error(1)
}

func (m *mockShellyClient) PerformUpdate(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockShellyClient) GetMetrics(ctx context.Context) (*shelly.DeviceMetrics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*shelly.DeviceMetrics), args.Error(1)
}

func (m *mockShellyClient) GetEnergyData(ctx context.Context, channel int) (*shelly.EnergyData, error) {
	args := m.Called(ctx, channel)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*shelly.EnergyData), args.Error(1)
}

func (m *mockShellyClient) TestConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockShellyClient) GetGeneration() int {
	args := m.Called()
	return args.Int(0)
}

func (m *mockShellyClient) GetIP() string {
	args := m.Called()
	return args.String(0)
}

// Test helpers
func setupTestService(t *testing.T) (*Service, *gorm.DB) {
	db := setupTestDB(t)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text"})
	service := NewService(db, logger)
	return service, db
}

func createTestDevice(t *testing.T, db *gorm.DB, id uint, name, deviceType string) {
	device := &Device{
		ID:   id,
		Name: name,
		Type: deviceType,
		IP:   "192.168.1.100",
		MAC:  "AA:BB:CC:DD:EE:FF",
	}
	err := db.Create(device).Error
	require.NoError(t, err)
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text"})

	service := NewService(db, logger)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.db)
	assert.Equal(t, logger, service.logger)

	// Verify tables were migrated
	var tableCount int64
	db.Table("config_templates").Count(&tableCount)
	db.Table("device_configs").Count(&tableCount)
	db.Table("config_histories").Count(&tableCount)
}

func TestImportFromDevice_Gen1(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")

	mockClient := new(mockShellyClient)

	// Setup mock expectations
	deviceInfo := &shelly.DeviceInfo{
		ID:         "shelly1-123456",
		Type:       "SHSW-1",
		MAC:        "AA:BB:CC:DD:EE:FF",
		Generation: 1,
		Model:      "SHSW-1",
	}

	deviceConfig := &shelly.DeviceConfig{
		Name: "shelly1-123456",
		WiFi: &shelly.WiFiConfig{
			Enable: true,
			SSID:   "TestNetwork",
			IP:     "192.168.1.100",
		},
		Raw: json.RawMessage(`{"name":"shelly1-123456","wifi":{"enable":true,"ssid":"TestNetwork","ip":"192.168.1.100"}}`),
	}

	mockClient.On("GetInfo", mock.Anything).Return(deviceInfo, nil)
	mockClient.On("GetConfig", mock.Anything).Return(deviceConfig, nil)

	// Test import
	config, err := service.ImportFromDevice(1, mockClient)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, uint(1), config.DeviceID)
	assert.Equal(t, "synced", config.SyncStatus)
	assert.NotNil(t, config.LastSynced)

	// Verify configuration content contains the raw config data
	var configData map[string]interface{}
	err = json.Unmarshal(config.Config, &configData)
	require.NoError(t, err)

	// Check that metadata was added
	metadata, exists := configData["_metadata"].(map[string]interface{})
	assert.True(t, exists)
	assert.Equal(t, float64(1), metadata["device_id"])
	assert.Equal(t, deviceInfo.Generation, int(metadata["generation"].(float64)))
	assert.Equal(t, deviceInfo.Model, metadata["model"])

	// Verify history was created
	var history []ConfigHistory
	db.Where("device_id = ?", 1).Find(&history)
	assert.Len(t, history, 1)
	assert.Equal(t, "import", history[0].Action)
	assert.Equal(t, "system", history[0].ChangedBy)

	mockClient.AssertExpectations(t)
}

func TestImportFromDevice_Gen2(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-25")

	mockClient := new(mockShellyClient)

	// Setup mock expectations for Gen2+
	deviceInfo := &shelly.DeviceInfo{
		ID:         "shellyplus1-123456",
		Type:       "SHSW-25",
		MAC:        "AA:BB:CC:DD:EE:FF",
		Generation: 2,
		Model:      "SHSW-25",
	}

	deviceConfig := &shelly.DeviceConfig{
		Name: "shellyplus1-123456",
		WiFi: &shelly.WiFiConfig{
			Enable: true,
			SSID:   "TestNetwork2",
			IP:     "192.168.1.200",
		},
		Raw: json.RawMessage(`{"name":"shellyplus1-123456","wifi":{"enable":true,"ssid":"TestNetwork2","ip":"192.168.1.200"}}`),
	}

	mockClient.On("GetInfo", mock.Anything).Return(deviceInfo, nil)
	mockClient.On("GetConfig", mock.Anything).Return(deviceConfig, nil)

	// Test import
	config, err := service.ImportFromDevice(1, mockClient)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, uint(1), config.DeviceID)
	assert.Equal(t, "synced", config.SyncStatus)
	assert.NotNil(t, config.LastSynced)

	// Verify configuration content contains the raw config data
	var configData map[string]interface{}
	err = json.Unmarshal(config.Config, &configData)
	require.NoError(t, err)

	// Check that metadata was added
	metadata, exists := configData["_metadata"].(map[string]interface{})
	assert.True(t, exists)
	assert.Equal(t, float64(1), metadata["device_id"])
	assert.Equal(t, deviceInfo.Generation, int(metadata["generation"].(float64)))
	assert.Equal(t, deviceInfo.Model, metadata["model"])

	mockClient.AssertExpectations(t)
}

func TestImportFromDevice_UpdateExisting(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")

	// Create existing config
	existingConfig := &DeviceConfig{
		DeviceID:   1,
		Config:     json.RawMessage(`{"old": "config"}`),
		SyncStatus: "drift",
	}
	err := db.Create(existingConfig).Error
	require.NoError(t, err)

	mockClient := new(mockShellyClient)

	// Setup mock expectations
	deviceInfo := &shelly.DeviceInfo{
		ID:         "shelly1-123456",
		Type:       "SHSW-1",
		MAC:        "AA:BB:CC:DD:EE:FF",
		Generation: 1,
		Model:      "SHSW-1",
	}

	deviceConfig := &shelly.DeviceConfig{
		Name: "shelly1-123456",
		WiFi: &shelly.WiFiConfig{
			Enable: true,
			SSID:   "UpdatedNetwork",
			IP:     "192.168.1.150",
		},
		Raw: json.RawMessage(`{"name":"shelly1-123456","wifi":{"enable":true,"ssid":"UpdatedNetwork","ip":"192.168.1.150"}}`),
	}

	mockClient.On("GetInfo", mock.Anything).Return(deviceInfo, nil)
	mockClient.On("GetConfig", mock.Anything).Return(deviceConfig, nil)

	// Test import
	config, err := service.ImportFromDevice(1, mockClient)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, existingConfig.ID, config.ID) // Should update existing
	assert.Equal(t, "synced", config.SyncStatus)

	// Verify history was created with old config
	var history []ConfigHistory
	db.Where("device_id = ?", 1).Find(&history)
	assert.Len(t, history, 1)
	assert.Equal(t, "import", history[0].Action)
	assert.JSONEq(t, `{"old": "config"}`, string(history[0].OldConfig))

	mockClient.AssertExpectations(t)
}

func TestImportFromDevice_Errors(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*mockShellyClient)
		expectedError string
	}{
		{
			name: "GetInfo error",
			setupMock: func(m *mockShellyClient) {
				m.On("GetInfo", mock.Anything).Return(nil, errors.New("connection failed"))
			},
			expectedError: "failed to get device info",
		},
		{
			name: "GetConfig error",
			setupMock: func(m *mockShellyClient) {
				deviceInfo := &shelly.DeviceInfo{
					ID:         "shelly1-123456",
					Generation: 1,
				}
				m.On("GetInfo", mock.Anything).Return(deviceInfo, nil)
				m.On("GetConfig", mock.Anything).Return(nil, errors.New("config error"))
			},
			expectedError: "failed to get device configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, db := setupTestService(t)
			createTestDevice(t, db, 1, "Test Device", "SHSW-1")

			mockClient := new(mockShellyClient)
			tt.setupMock(mockClient)

			config, err := service.ImportFromDevice(1, mockClient)

			assert.Nil(t, config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			mockClient.AssertExpectations(t)
		})
	}
}

// driftCheckState is what a drift check must leave alone: the stored row and its history.
type driftCheckState struct {
	row       DeviceConfig
	histories int64
}

func readDriftCheckState(t *testing.T, db *gorm.DB, deviceID uint) driftCheckState {
	t.Helper()
	var s driftCheckState
	require.NoError(t, db.Where("device_id = ?", deviceID).First(&s.row).Error)
	require.NoError(t, db.Model(&ConfigHistory{}).Where("device_id = ?", deviceID).Count(&s.histories).Error)
	return s
}

// requireBaselineUntouched fails unless the drift check left the stored config, its sync time,
// its update time and the history exactly as they were. Only the sync status may move, to want.
func requireBaselineUntouched(t *testing.T, before, after driftCheckState, wantStatus string) {
	t.Helper()
	assert.Equal(t, wantStatus, after.row.SyncStatus)
	assert.Equal(t, string(before.row.Config), string(after.row.Config), "stored config was rewritten")
	assert.Equal(t, before.row.LastSynced, after.row.LastSynced, "last_synced was re-stamped")
	assert.True(t, before.row.UpdatedAt.Equal(after.row.UpdatedAt), "updated_at moved: %v -> %v",
		before.row.UpdatedAt, after.row.UpdatedAt)
	assert.Equal(t, before.histories, after.histories, "a drift check appended a history row")
}

// TestCompareConfigurations_ReportsMetadata locks in that the audit comparator (used by
// createHistory) reports a metadata-only change.
func TestCompareConfigurations_ReportsMetadata(t *testing.T) {
	service, _ := setupTestService(t)

	stored := json.RawMessage(`{"name": "dev", "_metadata": {"imported_at": "2020-01-01T00:00:00Z"}}`)
	current := json.RawMessage(`{"name": "dev", "_metadata": {"imported_at": "2025-01-01T00:00:00Z"}}`)

	audit := service.compareConfigurations(stored, current)
	require.Len(t, audit, 1, "audit comparison must still report the metadata change")
	assert.Contains(t, audit[0].Path, "_metadata")
}

func TestApplyTemplate(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")

	// Create template
	template := &ConfigTemplate{
		Name:       "Test Template",
		DeviceType: "SHSW-1",
		Generation: 1,
		Config:     json.RawMessage(`{"template": "config", "wifi": {"ssid": "TemplateNetwork"}}`),
	}
	err := db.Create(template).Error
	require.NoError(t, err)

	// Apply template
	err = service.ApplyTemplate(1, template.ID, nil)

	require.NoError(t, err)

	// Verify config was created
	var config DeviceConfig
	db.Where("device_id = ?", 1).First(&config)
	assert.Equal(t, uint(1), config.DeviceID)
	assert.NotNil(t, config.TemplateID)
	assert.Equal(t, template.ID, *config.TemplateID)
	assert.Equal(t, "pending", config.SyncStatus)
	assert.JSONEq(t, string(template.Config), string(config.Config))

	// Verify history was created
	var history []ConfigHistory
	db.Where("device_id = ?", 1).Find(&history)
	assert.Len(t, history, 1)
	assert.Equal(t, "template", history[0].Action)
	assert.Equal(t, "template", history[0].ChangedBy)
}

func TestApplyTemplate_UpdateExisting(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")

	// Create existing config
	existingConfig := &DeviceConfig{
		DeviceID:   1,
		Config:     json.RawMessage(`{"old": "config"}`),
		SyncStatus: "synced",
	}
	err := db.Create(existingConfig).Error
	require.NoError(t, err)

	// Create template
	template := &ConfigTemplate{
		Name:       "Test Template",
		DeviceType: "SHSW-1",
		Config:     json.RawMessage(`{"new": "template"}`),
	}
	err = db.Create(template).Error
	require.NoError(t, err)

	// Apply template
	err = service.ApplyTemplate(1, template.ID, nil)

	require.NoError(t, err)

	// Verify config was updated
	var config DeviceConfig
	db.Where("device_id = ?", 1).First(&config)
	assert.Equal(t, existingConfig.ID, config.ID)
	assert.JSONEq(t, string(template.Config), string(config.Config))

	// Verify history contains old config
	var history []ConfigHistory
	db.Where("device_id = ?", 1).Find(&history)
	assert.Len(t, history, 1)
	assert.JSONEq(t, `{"old": "config"}`, string(history[0].OldConfig))
}

func TestApplyTemplate_IncompatibleType(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")

	// Create template for different device type
	template := &ConfigTemplate{
		Name:       "Wrong Template",
		DeviceType: "SHPLG-S", // Different type
		Config:     json.RawMessage(`{}`),
	}
	err := db.Create(template).Error
	require.NoError(t, err)

	// Try to apply incompatible template
	err = service.ApplyTemplate(1, template.ID, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not compatible with device type")
}

func TestApplyTemplate_WithVariables(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")

	// Create template with variables
	template := &ConfigTemplate{
		Name:       "Variable Template",
		DeviceType: "all",
		Config:     json.RawMessage(`{"name": "{{device_name}}", "keepalive": "{{keepalive}}"}`),
		Variables:  json.RawMessage(`[{"name": "device_name", "type": "string"}, {"name": "keepalive", "type": "number"}]`),
	}
	err := db.Create(template).Error
	require.NoError(t, err)

	// Apply template with variables
	variables := map[string]interface{}{
		"device_name": "MyDevice",
		"keepalive":   60,
	}
	err = service.ApplyTemplate(1, template.ID, variables)

	require.NoError(t, err)

	// Note: Variable substitution is not fully implemented yet
	// This test verifies the flow works without errors
}

func TestGetDeviceConfig(t *testing.T) {
	service, db := setupTestService(t)

	// Create config
	config := &DeviceConfig{
		DeviceID:   1,
		Config:     json.RawMessage(`{"test": "config"}`),
		SyncStatus: "synced",
	}
	err := db.Create(config).Error
	require.NoError(t, err)

	// Get config
	result, err := service.GetDeviceConfig(1)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, config.ID, result.ID)
	assert.Equal(t, config.DeviceID, result.DeviceID)
	assert.JSONEq(t, string(config.Config), string(result.Config))
}

func TestGetDeviceConfig_NotFound(t *testing.T) {
	service, _ := setupTestService(t)

	result, err := service.GetDeviceConfig(999)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestTemplateOperations(t *testing.T) {
	service, db := setupTestService(t)

	t.Run("CreateTemplate", func(t *testing.T) {
		// Scope is required: a template stored without one cannot be migrated
		// later (#275), so the write path now rejects it.
		template := &ConfigTemplate{
			Name:       "New Template",
			Scope:      ScopeDeviceType,
			DeviceType: "SHSW-1",
			Config:     json.RawMessage(`{"test": "create"}`),
		}

		err := service.CreateTemplate(template)

		require.NoError(t, err)
		assert.NotZero(t, template.ID)

		// Verify in database
		var saved ConfigTemplate
		db.First(&saved, template.ID)
		assert.Equal(t, template.Name, saved.Name)
	})

	t.Run("GetTemplates", func(t *testing.T) {
		// Create multiple templates
		templates := []ConfigTemplate{
			{Name: "Template1", DeviceType: "SHSW-1", Config: json.RawMessage(`{}`)},
			{Name: "Template2", DeviceType: "SHPLG-S", Config: json.RawMessage(`{}`)},
			{Name: "Template3", DeviceType: "all", Config: json.RawMessage(`{}`)},
		}
		for _, tmpl := range templates {
			db.Create(&tmpl)
		}

		// Get all templates
		result, err := service.GetTemplates()

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result), 3)
	})

	t.Run("UpdateTemplate", func(t *testing.T) {
		// Create template
		template := &ConfigTemplate{
			Name:       "Update Test",
			Scope:      ScopeDeviceType,
			DeviceType: "SHSW-1",
			Config:     json.RawMessage(`{"original": true}`),
		}
		db.Create(template)

		// Update template
		template.Name = "Updated Name"
		template.Config = json.RawMessage(`{"updated": true}`)

		err := service.UpdateTemplate(template)

		require.NoError(t, err)

		// Verify update
		var saved ConfigTemplate
		db.First(&saved, template.ID)
		assert.Equal(t, "Updated Name", saved.Name)
		assert.JSONEq(t, `{"updated": true}`, string(saved.Config))
	})

	t.Run("DeleteTemplate", func(t *testing.T) {
		// Create template
		template := &ConfigTemplate{
			Name:       "Delete Test",
			DeviceType: "SHSW-1",
			Config:     json.RawMessage(`{}`),
		}
		db.Create(template)

		// Delete template
		err := service.DeleteTemplate(template.ID)

		require.NoError(t, err)

		// Verify deletion
		var count int64
		db.Model(&ConfigTemplate{}).Where("id = ?", template.ID).Count(&count)
		assert.Zero(t, count)
	})
}

func TestGetConfigHistory(t *testing.T) {
	service, db := setupTestService(t)

	// Create multiple history entries
	deviceID := uint(1)
	for i := 0; i < 5; i++ {
		history := &ConfigHistory{
			DeviceID:  deviceID,
			ConfigID:  uint(i + 1),
			Action:    "import",
			NewConfig: json.RawMessage(`{}`),
			ChangedBy: "system",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Hour),
		}
		db.Create(history)
	}

	t.Run("GetAll", func(t *testing.T) {
		history, err := service.GetConfigHistory(deviceID, 0)

		require.NoError(t, err)
		assert.Len(t, history, 5)

		// Verify ordering (newest first)
		for i := 0; i < len(history)-1; i++ {
			assert.True(t, history[i].CreatedAt.After(history[i+1].CreatedAt))
		}
	})

	t.Run("GetWithLimit", func(t *testing.T) {
		history, err := service.GetConfigHistory(deviceID, 3)

		require.NoError(t, err)
		assert.Len(t, history, 3)
	})

	t.Run("NoHistory", func(t *testing.T) {
		history, err := service.GetConfigHistory(999, 0)

		require.NoError(t, err)
		assert.Empty(t, history)
	})
}

func TestCompareConfigurations(t *testing.T) {
	service, _ := setupTestService(t)

	tests := []struct {
		name        string
		stored      json.RawMessage
		current     json.RawMessage
		expectDiffs []string // Expected diff types
	}{
		{
			name:        "No differences",
			stored:      json.RawMessage(`{"wifi": {"ssid": "Network"}, "mqtt": {"enabled": true}}`),
			current:     json.RawMessage(`{"wifi": {"ssid": "Network"}, "mqtt": {"enabled": true}}`),
			expectDiffs: []string{},
		},
		{
			name:        "Modified field",
			stored:      json.RawMessage(`{"wifi": {"ssid": "OldNetwork"}}`),
			current:     json.RawMessage(`{"wifi": {"ssid": "NewNetwork"}}`),
			expectDiffs: []string{"modified"},
		},
		{
			name:        "Added field",
			stored:      json.RawMessage(`{"wifi": {"ssid": "Network"}}`),
			current:     json.RawMessage(`{"wifi": {"ssid": "Network"}, "mqtt": {"enabled": true}}`),
			expectDiffs: []string{"added"},
		},
		{
			name:        "Removed field",
			stored:      json.RawMessage(`{"wifi": {"ssid": "Network"}, "mqtt": {"enabled": true}}`),
			current:     json.RawMessage(`{"wifi": {"ssid": "Network"}}`),
			expectDiffs: []string{"removed"},
		},
		{
			name:        "Multiple differences",
			stored:      json.RawMessage(`{"wifi": {"ssid": "Old", "pass": "secret"}, "name": "OldName"}`),
			current:     json.RawMessage(`{"wifi": {"ssid": "New"}, "name": "NewName", "cloud": true}`),
			expectDiffs: []string{"modified", "modified", "removed", "added"},
		},
		{
			name:        "Nested differences",
			stored:      json.RawMessage(`{"sys": {"device": {"name": "Old", "location": {"tz": "UTC"}}}}`),
			current:     json.RawMessage(`{"sys": {"device": {"name": "New", "location": {"tz": "EST", "lat": 40.7}}}}`),
			expectDiffs: []string{"modified", "modified", "added"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differences := service.compareConfigurations(tt.stored, tt.current)

			assert.Len(t, differences, len(tt.expectDiffs))

			// Verify diff types match
			for i, diff := range differences {
				if i < len(tt.expectDiffs) {
					assert.Contains(t, tt.expectDiffs, diff.Type)
				}
			}
		})
	}
}

func TestCompareMaps_DetailedPaths(t *testing.T) {
	service, _ := setupTestService(t)

	expected := map[string]interface{}{
		"wifi": map[string]interface{}{
			"ssid": "Network1",
			"pass": "secret",
		},
		"mqtt": map[string]interface{}{
			"enabled": true,
			"server":  "mqtt.example.com",
		},
	}

	actual := map[string]interface{}{
		"wifi": map[string]interface{}{
			"ssid": "Network2",
			// "pass" is removed
		},
		"mqtt": map[string]interface{}{
			"enabled": false, // modified
			"server":  "mqtt.example.com",
			"port":    1883, // added
		},
	}

	var differences []ConfigDifference
	service.compareMaps("", expected, actual, &differences)

	// Verify specific paths
	pathMap := make(map[string]ConfigDifference)
	for _, diff := range differences {
		pathMap[diff.Path] = diff
	}

	assert.Contains(t, pathMap, "wifi.ssid")
	assert.Equal(t, "modified", pathMap["wifi.ssid"].Type)

	assert.Contains(t, pathMap, "wifi.pass")
	assert.Equal(t, "removed", pathMap["wifi.pass"].Type)

	assert.Contains(t, pathMap, "mqtt.enabled")
	assert.Equal(t, "modified", pathMap["mqtt.enabled"].Type)

	assert.Contains(t, pathMap, "mqtt.port")
	assert.Equal(t, "added", pathMap["mqtt.port"].Type)
}

func TestCreateHistory(t *testing.T) {
	service, db := setupTestService(t)

	oldConfig := json.RawMessage(`{"old": "config"}`)
	newConfig := json.RawMessage(`{"new": "config"}`)

	// Call createHistory directly (normally private)
	service.createHistory(1, 1, "test", oldConfig, newConfig, "user")

	// Verify history was created
	var history ConfigHistory
	db.Where("device_id = ? AND action = ?", 1, "test").First(&history)

	assert.Equal(t, uint(1), history.DeviceID)
	assert.Equal(t, uint(1), history.ConfigID)
	assert.Equal(t, "test", history.Action)
	assert.Equal(t, "user", history.ChangedBy)
	assert.JSONEq(t, string(oldConfig), string(history.OldConfig))
	assert.JSONEq(t, string(newConfig), string(history.NewConfig))
	assert.NotNil(t, history.Changes) // Should contain the diff
}

func TestCreateHistory_WithoutOldConfig(t *testing.T) {
	service, db := setupTestService(t)

	newConfig := json.RawMessage(`{"new": "config"}`)

	// Call createHistory without old config
	service.createHistory(1, 1, "import", nil, newConfig, "system")

	// Verify history was created
	var history ConfigHistory
	db.Where("device_id = ? AND action = ?", 1, "import").First(&history)

	assert.Equal(t, uint(1), history.DeviceID)
	assert.Nil(t, history.OldConfig)
	assert.JSONEq(t, string(newConfig), string(history.NewConfig))
	assert.Nil(t, history.Changes) // No diff when old config is nil
}

func TestSubstituteVariables(t *testing.T) {
	service, _ := setupTestService(t)

	config := json.RawMessage(`{"name": "{{device_name}}", "port": "{{port}}"}`)
	variables := map[string]interface{}{
		"device_name": "MyDevice",
		"port":        8080,
	}

	// Note: Current implementation just returns config as-is
	// This test documents the expected behavior when implemented
	result := service.substituteVariables(config, variables)

	// For now, it should return the same config
	assert.JSONEq(t, string(config), string(result))

	// TODO: When implemented, should verify variable substitution:
	// expected := `{"name": "MyDevice", "port": 8080}`
	// assert.JSONEq(t, expected, string(result))
}

func TestConcurrentOperations(t *testing.T) {
	service, db := setupTestService(t)

	// Create multiple devices with unique IPs
	for i := 1; i <= 5; i++ {
		device := &Device{
			ID:   uint(i),
			Name: fmt.Sprintf("Device%d", i),
			Type: "SHSW-1",
			IP:   fmt.Sprintf("192.168.1.%d", 100+i),
			MAC:  fmt.Sprintf("AA:BB:CC:DD:EE:%02X", i),
		}
		err := db.Create(device).Error
		require.NoError(t, err)
	}

	// Create template
	template := &ConfigTemplate{
		Name:       "Concurrent Template",
		DeviceType: "SHSW-1",
		Config:     json.RawMessage(`{"concurrent": true}`),
	}
	err := db.Create(template).Error
	require.NoError(t, err)

	// Apply template to devices sequentially (concurrent DB operations can be problematic with SQLite in-memory)
	for i := 1; i <= 5; i++ {
		err := service.ApplyTemplate(uint(i), template.ID, nil)
		assert.NoError(t, err)
	}

	// Verify all configs were created
	var count int64
	db.Model(&DeviceConfig{}).Count(&count)
	assert.Equal(t, int64(5), count)
}

// TestBulkDetectDrift tests bulk drift detection functionality
func TestBulkDetectDrift(t *testing.T) {
	service, db := setupTestService(t)

	// Create test devices first with unique IPs
	device1 := &Device{
		ID:   1,
		Name: "Device1",
		Type: "SHSW-1",
		IP:   "192.168.1.101",
		MAC:  "AA:BB:CC:DD:EE:01",
	}
	device2 := &Device{
		ID:   2,
		Name: "Device2",
		Type: "SHSW-1",
		IP:   "192.168.1.102",
		MAC:  "AA:BB:CC:DD:EE:02",
	}
	require.NoError(t, db.Create(device1).Error)
	require.NoError(t, db.Create(device2).Error)

	// Drift compares with the desired configuration, which this package's devices table does not
	// hold, so a mock repository serves it.
	repo := newMockRepository()
	repo.devices[1] = &ServiceDevice{ID: 1, MAC: "AA:BB:CC:DD:EE:01", DesiredConfig: desiredNamed(t, "Device1")}
	repo.devices[2] = &ServiceDevice{ID: 2, MAC: "AA:BB:CC:DD:EE:02", DesiredConfig: desiredNamed(t, "Device2")}
	service.ConfigurationSvc = NewConfigurationService(repo, nil, nil)

	// Device 1 holds its desired configuration; device 2 was renamed after the apply.
	mockClient1 := driftClient("Device1", "AA:BB:CC:DD:EE:01")
	mockClient2 := driftClient("Renamed", "AA:BB:CC:DD:EE:02")

	// Client getter function
	clientGetter := func(deviceID uint) (shelly.Client, error) {
		switch deviceID {
		case 1:
			return mockClient1, nil
		case 2:
			return mockClient2, nil
		default:
			return nil, fmt.Errorf("device not found")
		}
	}

	// Test bulk drift detection
	result, err := service.BulkDetectDrift([]uint{1, 2}, clientGetter, false)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify results
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 1, result.InSync)  // Device 1 holds its desired configuration
	assert.Equal(t, 1, result.Drifted) // Device 2 left it
	assert.Equal(t, 0, result.Errors)
	assert.Len(t, result.Results, 2)

	// Check device 1 result (in sync)
	device1Result := result.Results[0]
	assert.Equal(t, uint(1), device1Result.DeviceID)
	assert.Equal(t, "synced", device1Result.Status)
	assert.Equal(t, 0, device1Result.DifferenceCount)
	assert.Nil(t, device1Result.Drift)

	// Check device 2 result (drift detected)
	device2Result := result.Results[1]
	assert.Equal(t, uint(2), device2Result.DeviceID)
	assert.Equal(t, "drift", device2Result.Status)
	assert.Greater(t, device2Result.DifferenceCount, 0)
	assert.NotNil(t, device2Result.Drift)
	assert.Contains(t, device2Result.DriftSummary, "configuration differences detected")

	// Verify timing information
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.CompletedAt.After(result.StartedAt))
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestBulkDetectDrift_WithErrors(t *testing.T) {
	service, db := setupTestService(t)

	// Create test device first with unique IP
	device1 := &Device{
		ID:   1,
		Name: "Device1",
		Type: "SHSW-1",
		IP:   "192.168.1.103",
		MAC:  "AA:BB:CC:DD:EE:03",
	}
	require.NoError(t, db.Create(device1).Error)

	repo := newMockRepository()
	repo.devices[1] = &ServiceDevice{ID: 1, MAC: "AA:BB:CC:DD:EE:03", DesiredConfig: desiredNamed(t, "Device1")}
	service.ConfigurationSvc = NewConfigurationService(repo, nil, nil)

	// Client getter that returns errors for some devices
	clientGetter := func(deviceID uint) (shelly.Client, error) {
		switch deviceID {
		case 1:
			// Valid client
			return driftClient("Device1", "AA:BB:CC:DD:EE:03"), nil
		case 2:
			// Client creation error
			return nil, fmt.Errorf("client creation failed")
		case 999:
			// Device not found (will be handled by BulkDetectDrift)
			return nil, fmt.Errorf("device not found")
		default:
			return nil, fmt.Errorf("unknown device")
		}
	}

	// Test with mixed success/error scenarios
	result, err := service.BulkDetectDrift([]uint{1, 2, 999}, clientGetter, false)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify error handling
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 1, result.InSync)  // Device 1 holds its desired configuration
	assert.Equal(t, 0, result.Drifted) // No drift
	assert.Equal(t, 2, result.Errors)  // Devices 2 and 999 failed
	assert.Len(t, result.Results, 3)

	// Check successful device (in sync)
	successResult := result.Results[0]
	assert.Equal(t, uint(1), successResult.DeviceID)
	assert.Equal(t, "synced", successResult.Status)
	assert.Empty(t, successResult.Error)

	// Check client error device
	clientErrorResult := result.Results[1]
	assert.Equal(t, uint(2), clientErrorResult.DeviceID)
	assert.Equal(t, "error", clientErrorResult.Status)
	assert.Contains(t, clientErrorResult.Error, "Device not found")

	// Check device not found error
	notFoundResult := result.Results[2]
	assert.Equal(t, uint(999), notFoundResult.DeviceID)
	assert.Equal(t, "error", notFoundResult.Status)
	assert.Contains(t, notFoundResult.Error, "Device not found")
}

// With stopOnError, the devices after the first error are skipped and their client is never asked for.
func TestBulkDetectDrift_StopOnErrorSkipsTheRest(t *testing.T) {
	service, db := setupTestService(t)
	for _, id := range []uint{1, 2} {
		require.NoError(t, db.Create(&Device{
			ID:   id,
			Name: fmt.Sprintf("Device%d", id),
			Type: "SHSW-1",
			IP:   fmt.Sprintf("192.168.1.%d", 110+id),
			MAC:  fmt.Sprintf("AA:BB:CC:DD:EE:%02d", 10+id),
		}).Error)
	}

	// Both devices have a desired configuration, so each one's client is asked for.
	repo := newMockRepository()
	repo.devices[1] = &ServiceDevice{ID: 1, MAC: "AA:BB:CC:DD:EE:11", DesiredConfig: desiredNamed(t, "Device1")}
	repo.devices[2] = &ServiceDevice{ID: 2, MAC: "AA:BB:CC:DD:EE:12", DesiredConfig: desiredNamed(t, "Device2")}
	service.ConfigurationSvc = NewConfigurationService(repo, nil, nil)

	var asked []uint
	clientGetter := func(deviceID uint) (shelly.Client, error) {
		asked = append(asked, deviceID)
		return nil, fmt.Errorf("client creation failed")
	}

	result, err := service.BulkDetectDrift([]uint{1, 2}, clientGetter, true)
	require.NoError(t, err)

	assert.Equal(t, []uint{1}, asked, "the device after the first error must not be contacted")
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 1, result.Errors)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, result.Results, 2)
	assert.Equal(t, "error", result.Results[0].Status)
	assert.Equal(t, BulkStatusSkipped, result.Results[1].Status)

	asked = nil
	result, err = service.BulkDetectDrift([]uint{1, 2}, clientGetter, false)
	require.NoError(t, err)
	assert.Equal(t, []uint{1, 2}, asked, "without stopOnError every device is tried")
	assert.Equal(t, 2, result.Errors)
	assert.Equal(t, 0, result.Skipped)
}

func TestBulkDetectDrift_EmptyDeviceList(t *testing.T) {
	service, _ := setupTestService(t)

	clientGetter := func(deviceID uint) (shelly.Client, error) {
		return nil, fmt.Errorf("should not be called")
	}

	// Test with empty device list
	result, err := service.BulkDetectDrift([]uint{}, clientGetter, false)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, 0, result.Total)
	assert.Equal(t, 0, result.InSync)
	assert.Equal(t, 0, result.Drifted)
	assert.Equal(t, 0, result.Errors)
	assert.Len(t, result.Results, 0)
}

func TestServiceWithNilDB(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text"})

	// NewService with nil DB will panic due to AutoMigrate
	// This is expected behavior
	assert.Panics(t, func() {
		_ = NewService(nil, logger)
	})
}

func TestServiceWithNilLogger(t *testing.T) {
	db := setupTestDB(t)

	// Should not panic
	assert.NotPanics(t, func() {
		_ = NewService(db, nil)
	})
}

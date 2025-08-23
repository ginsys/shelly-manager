package export

import (
	"context"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// MockPlugin implements ExportPlugin for testing
type MockPlugin struct {
	name         string
	formats      []string
	initError    error
	exportError  error
	configError  error
	previewError error
}

func (m *MockPlugin) Info() PluginInfo {
	return PluginInfo{
		Name:             m.name,
		Version:          "1.0.0",
		Description:      "Mock plugin for testing",
		Author:           "Test Suite",
		License:          "MIT",
		SupportedFormats: m.formats,
		Tags:             []string{"test", "mock"},
		Category:         CategoryCustom,
	}
}

func (m *MockPlugin) ConfigSchema() ConfigSchema {
	return ConfigSchema{
		Version:    "1.0",
		Properties: map[string]PropertySchema{},
		Required:   []string{},
	}
}

func (m *MockPlugin) ValidateConfig(config map[string]interface{}) error {
	return m.configError
}

func (m *MockPlugin) Export(ctx context.Context, data *ExportData, config ExportConfig) (*ExportResult, error) {
	if m.exportError != nil {
		return nil, m.exportError
	}

	return &ExportResult{
		Success:     true,
		RecordCount: len(data.Devices),
		FileSize:    1024,
		Duration:    time.Millisecond * 100,
		CreatedAt:   time.Now(),
	}, nil
}

func (m *MockPlugin) Preview(ctx context.Context, data *ExportData, config ExportConfig) (*PreviewResult, error) {
	if m.previewError != nil {
		return nil, m.previewError
	}

	return &PreviewResult{
		Success:       true,
		SampleData:    []byte("mock preview data"),
		RecordCount:   len(data.Devices),
		EstimatedSize: 1024,
	}, nil
}

func (m *MockPlugin) Capabilities() PluginCapabilities {
	return PluginCapabilities{
		SupportsIncremental:    false,
		SupportsScheduling:     true,
		RequiresAuthentication: false,
		SupportedOutputs:       []string{"file"},
		MaxDataSize:            1024 * 1024,
		ConcurrencyLevel:       1,
	}
}

func (m *MockPlugin) Initialize(logger *logging.Logger) error {
	return m.initError
}

func (m *MockPlugin) Cleanup() error {
	return nil
}

// MockDatabaseManager implements DatabaseManagerInterface for testing
type MockDatabaseManager struct {
	provider provider.DatabaseProvider
	mockDB   *MockGormDB
}

func (m *MockDatabaseManager) GetProvider() provider.DatabaseProvider {
	return m.provider
}

func (m *MockDatabaseManager) GetDB() interface{} {
	return nil // Always return nil for testing
}

func (m *MockDatabaseManager) Close() error {
	return nil
}

// MockGormDB simulates gorm.DB for testing
type MockGormDB struct {
	devices []database.Device
	error   error
}

func (m *MockGormDB) WithContext(ctx context.Context) *MockGormDB {
	return m
}

func (m *MockGormDB) Find(dest interface{}) *MockGormDB {
	if devices, ok := dest.(*[]database.Device); ok {
		*devices = m.devices
	}
	return m
}

func (m *MockGormDB) Where(query interface{}, args ...interface{}) *MockGormDB {
	return m
}

func (m *MockGormDB) Error() error {
	return m.error
}

// createMockDatabase creates a database manager for testing
func createMockDatabase() DatabaseManagerInterface {
	return &MockDatabaseManager{
		mockDB: nil, // Return nil to trigger testing mode
	}
}

func TestNewExportEngine(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()

	engine := NewExportEngine(mockDB, logger)

	if engine == nil {
		t.Fatal("NewExportEngine returned nil")
	}

	if engine.dbManager != mockDB {
		t.Error("Engine dbManager not set correctly")
	}

	if engine.logger != logger {
		t.Error("Engine logger not set correctly")
	}

	if len(engine.plugins) != 0 {
		t.Error("Engine should start with no plugins")
	}
}

func TestExportEngine_RegisterPlugin(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()
	engine := NewExportEngine(mockDB, logger)

	// Test successful registration
	mockPlugin := &MockPlugin{
		name:    "test-plugin",
		formats: []string{"json"},
	}

	err := engine.RegisterPlugin(mockPlugin)
	if err != nil {
		t.Fatalf("Failed to register plugin: %v", err)
	}

	// Test duplicate registration
	err = engine.RegisterPlugin(mockPlugin)
	if err == nil {
		t.Error("Should not allow duplicate plugin registration")
	}

	// Test plugin with empty name
	emptyNamePlugin := &MockPlugin{name: ""}
	err = engine.RegisterPlugin(emptyNamePlugin)
	if err == nil {
		t.Error("Should not allow plugin with empty name")
	}

	// Test plugin initialization error
	initErrorPlugin := &MockPlugin{
		name:      "error-plugin",
		initError: &PluginError{"initialization failed"},
	}
	err = engine.RegisterPlugin(initErrorPlugin)
	if err == nil {
		t.Error("Should fail when plugin initialization fails")
	}
}

func TestExportEngine_ListPlugins(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()
	engine := NewExportEngine(mockDB, logger)

	// Test empty list
	plugins := engine.ListPlugins()
	if len(plugins) != 0 {
		t.Error("Should return empty list when no plugins registered")
	}

	// Add some plugins
	plugin1 := &MockPlugin{name: "plugin1", formats: []string{"json"}}
	plugin2 := &MockPlugin{name: "plugin2", formats: []string{"yaml"}}

	engine.RegisterPlugin(plugin1)
	engine.RegisterPlugin(plugin2)

	plugins = engine.ListPlugins()
	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(plugins))
	}

	// Check plugin names are present
	names := make(map[string]bool)
	for _, plugin := range plugins {
		names[plugin.Name] = true
	}

	if !names["plugin1"] || !names["plugin2"] {
		t.Error("Plugin names not found in list")
	}
}

func TestExportEngine_GetPlugin(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()
	engine := NewExportEngine(mockDB, logger)

	// Test getting non-existent plugin
	_, err := engine.GetPlugin("nonexistent")
	if err == nil {
		t.Error("Should return error for non-existent plugin")
	}

	// Register and get plugin
	mockPlugin := &MockPlugin{name: "test-plugin", formats: []string{"json"}}
	engine.RegisterPlugin(mockPlugin)

	plugin, err := engine.GetPlugin("test-plugin")
	if err != nil {
		t.Fatalf("Failed to get plugin: %v", err)
	}

	if plugin.Info().Name != "test-plugin" {
		t.Error("Got wrong plugin")
	}
}

func TestExportEngine_ValidateExport(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()
	engine := NewExportEngine(mockDB, logger)

	mockPlugin := &MockPlugin{
		name:    "test-plugin",
		formats: []string{"json", "yaml"},
	}
	engine.RegisterPlugin(mockPlugin)

	// Test valid request
	request := ExportRequest{
		PluginName: "test-plugin",
		Format:     "json",
		Config:     map[string]interface{}{},
	}

	err := engine.ValidateExport(request)
	if err != nil {
		t.Errorf("Valid request should not return error: %v", err)
	}

	// Test invalid plugin
	request.PluginName = "nonexistent"
	err = engine.ValidateExport(request)
	if err == nil {
		t.Error("Should return error for invalid plugin")
	}

	// Test unsupported format
	request.PluginName = "test-plugin"
	request.Format = "unsupported"
	err = engine.ValidateExport(request)
	if err == nil {
		t.Error("Should return error for unsupported format")
	}

	// Test config validation error
	mockPlugin.configError = &PluginError{"invalid config"}
	request.Format = "json"
	err = engine.ValidateExport(request)
	if err == nil {
		t.Error("Should return error when config validation fails")
	}
}

func TestExportEngine_Export(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()
	engine := NewExportEngine(mockDB, logger)

	mockPlugin := &MockPlugin{
		name:    "test-plugin",
		formats: []string{"json"},
	}
	engine.RegisterPlugin(mockPlugin)

	request := ExportRequest{
		PluginName: "test-plugin",
		Format:     "json",
		Config:     map[string]interface{}{},
		Filters:    ExportFilters{},
		Options:    ExportOptions{},
	}

	ctx := context.Background()
	result, err := engine.Export(ctx, request)

	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if !result.Success {
		t.Error("Export should succeed")
	}

	if result.PluginName != "test-plugin" {
		t.Error("Result should contain plugin name")
	}

	if result.ExportID == "" {
		t.Error("Result should contain export ID")
	}
}

func TestExportEngine_Preview(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()
	engine := NewExportEngine(mockDB, logger)

	mockPlugin := &MockPlugin{
		name:    "test-plugin",
		formats: []string{"json"},
	}
	engine.RegisterPlugin(mockPlugin)

	request := ExportRequest{
		PluginName: "test-plugin",
		Format:     "json",
		Config:     map[string]interface{}{},
	}

	ctx := context.Background()
	result, err := engine.Preview(ctx, request)

	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if !result.Success {
		t.Error("Preview should succeed")
	}

	if len(result.SampleData) == 0 {
		t.Error("Preview should contain sample data")
	}
}

func TestExportEngine_Shutdown(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := createMockDatabase()
	engine := NewExportEngine(mockDB, logger)

	// Add some plugins
	plugin1 := &MockPlugin{name: "plugin1", formats: []string{"json"}}
	plugin2 := &MockPlugin{name: "plugin2", formats: []string{"yaml"}}

	engine.RegisterPlugin(plugin1)
	engine.RegisterPlugin(plugin2)

	err := engine.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// PluginError is a simple error type for testing
type PluginError struct {
	message string
}

func (e *PluginError) Error() string {
	return e.message
}

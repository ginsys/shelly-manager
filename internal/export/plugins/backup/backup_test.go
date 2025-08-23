package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/export"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// MockBackupProvider implements provider.BackupProvider for testing
type MockBackupProvider struct {
	createBackupResult   *provider.BackupResult
	createBackupError    error
	restoreBackupResult  *provider.RestoreResult
	restoreBackupError   error
	validateBackupResult *provider.ValidationResult
	validateBackupError  error
}

func (m *MockBackupProvider) CreateBackup(ctx context.Context, config provider.BackupConfig) (*provider.BackupResult, error) {
	if m.createBackupError != nil {
		return nil, m.createBackupError
	}
	if m.createBackupResult != nil {
		return m.createBackupResult, nil
	}
	return &provider.BackupResult{
		Success:     true,
		BackupID:    "test-backup-123",
		BackupPath:  config.BackupPath,
		BackupType:  config.BackupType,
		StartTime:   time.Now().Add(-time.Minute),
		EndTime:     time.Now(),
		Duration:    time.Minute,
		Size:        1024,
		RecordCount: 10,
		TableCount:  3,
		Checksum:    "test-checksum",
	}, nil
}

func (m *MockBackupProvider) RestoreBackup(ctx context.Context, config provider.RestoreConfig) (*provider.RestoreResult, error) {
	if m.restoreBackupError != nil {
		return nil, m.restoreBackupError
	}
	if m.restoreBackupResult != nil {
		return m.restoreBackupResult, nil
	}
	return &provider.RestoreResult{
		Success:         true,
		RestoreID:       "test-restore-123",
		BackupPath:      config.BackupPath,
		StartTime:       time.Now().Add(-time.Minute),
		EndTime:         time.Now(),
		Duration:        time.Minute,
		TablesRestored:  []string{"devices", "configurations"},
		RecordsRestored: 10,
	}, nil
}

func (m *MockBackupProvider) ValidateBackup(ctx context.Context, backupPath string) (*provider.ValidationResult, error) {
	if m.validateBackupError != nil {
		return nil, m.validateBackupError
	}
	if m.validateBackupResult != nil {
		return m.validateBackupResult, nil
	}
	return &provider.ValidationResult{
		Valid:         true,
		BackupID:      "test-backup-123",
		BackupType:    provider.BackupTypeFull,
		Size:          1024,
		RecordCount:   10,
		ChecksumValid: true,
	}, nil
}

func (m *MockBackupProvider) ListBackups() ([]provider.BackupInfo, error) {
	return []provider.BackupInfo{}, nil
}

func (m *MockBackupProvider) DeleteBackup(backupID string) error {
	return nil
}

// Embed provider.DatabaseProvider methods (we'll only implement what we need)
func (m *MockBackupProvider) Connect(config provider.DatabaseConfig) error { return nil }
func (m *MockBackupProvider) Close() error                                 { return nil }
func (m *MockBackupProvider) Ping() error                                  { return nil }
func (m *MockBackupProvider) Migrate(models ...interface{}) error          { return nil }
func (m *MockBackupProvider) DropTables(models ...interface{}) error       { return nil }
func (m *MockBackupProvider) BeginTransaction() (provider.Transaction, error) {
	return nil, nil
}
func (m *MockBackupProvider) GetDB() *gorm.DB { return nil }
func (m *MockBackupProvider) GetStats() provider.DatabaseStats {
	return provider.DatabaseStats{}
}
func (m *MockBackupProvider) SetLogger(logger *logging.Logger) {}
func (m *MockBackupProvider) Name() string                     { return "mock" }
func (m *MockBackupProvider) Version() string                  { return "1.0.0" }

// MockDatabaseManager implements the interface for testing
type MockDatabaseManager struct {
	provider provider.DatabaseProvider
}

func (m *MockDatabaseManager) GetProvider() provider.DatabaseProvider {
	return m.provider
}

func (m *MockDatabaseManager) GetDB() interface{} {
	return nil
}

func (m *MockDatabaseManager) Close() error {
	return nil
}

func TestBackupExporter_Info(t *testing.T) {
	mockDB := &MockDatabaseManager{}
	exporter := NewBackupExporter(mockDB)

	info := exporter.Info()

	if info.Name != "backup" {
		t.Errorf("Expected name 'backup', got '%s'", info.Name)
	}

	if info.Category != export.CategoryBackup {
		t.Errorf("Expected category %v, got %v", export.CategoryBackup, info.Category)
	}

	if len(info.SupportedFormats) == 0 {
		t.Error("Should support at least one format")
	}
}

func TestBackupExporter_ConfigSchema(t *testing.T) {
	mockDB := &MockDatabaseManager{}
	exporter := NewBackupExporter(mockDB)

	schema := exporter.ConfigSchema()

	if schema.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", schema.Version)
	}

	if len(schema.Properties) == 0 {
		t.Error("Schema should have properties")
	}

	// Check for expected properties
	if _, exists := schema.Properties["output_path"]; !exists {
		t.Error("Schema should have 'output_path' property")
	}

	if _, exists := schema.Properties["compression"]; !exists {
		t.Error("Schema should have 'compression' property")
	}
}

func TestBackupExporter_ValidateConfig(t *testing.T) {
	mockDB := &MockDatabaseManager{}
	exporter := NewBackupExporter(mockDB)

	// Test valid config
	validConfig := map[string]interface{}{
		"output_path": "/tmp/test-backup",
		"compression": true,
		"backup_type": "full",
	}

	err := exporter.ValidateConfig(validConfig)
	if err != nil {
		t.Errorf("Valid config should not return error: %v", err)
	}

	// Test invalid backup_type
	invalidConfig := map[string]interface{}{
		"backup_type": "invalid_type",
	}

	err = exporter.ValidateConfig(invalidConfig)
	if err == nil {
		t.Error("Invalid backup_type should return error")
	}

	// Test non-string output_path
	invalidPathConfig := map[string]interface{}{
		"output_path": 123,
	}

	err = exporter.ValidateConfig(invalidPathConfig)
	if err == nil {
		t.Error("Non-string output_path should return error")
	}
}

func TestBackupExporter_Export(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "backup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup mock
	mockProvider := &MockBackupProvider{}
	mockDB := &MockDatabaseManager{provider: mockProvider}
	exporter := NewBackupExporter(mockDB)
	exporter.Initialize(logging.GetDefault())

	// Create test data
	testData := &export.ExportData{
		Devices: []export.DeviceData{
			{ID: 1, Name: "Test Device", MAC: "00:11:22:33:44:55"},
		},
		Metadata: export.ExportMetadata{
			ExportID:      "test-export-123",
			SystemVersion: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	// Create export config
	config := export.ExportConfig{
		Format: "sma",
		Config: map[string]interface{}{
			"output_path": tmpDir,
			"compression": true,
			"backup_type": "full",
		},
	}

	ctx := context.Background()
	result, err := exporter.Export(ctx, testData, config)

	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if !result.Success {
		t.Error("Export should succeed")
	}

	if result.RecordCount != 10 { // From mock
		t.Errorf("Expected record count 10, got %d", result.RecordCount)
	}

	if result.OutputPath == "" {
		t.Error("Output path should be set")
	}
}

func TestBackupExporter_ExportWithProviderError(t *testing.T) {
	// Setup mock with error
	mockProvider := &MockBackupProvider{
		createBackupError: &BackupError{"backup failed"},
	}
	mockDB := &MockDatabaseManager{provider: mockProvider}
	exporter := NewBackupExporter(mockDB)
	exporter.Initialize(logging.GetDefault())

	testData := &export.ExportData{
		Devices:   []export.DeviceData{},
		Metadata:  export.ExportMetadata{},
		Timestamp: time.Now(),
	}

	config := export.ExportConfig{
		Format: "sma",
		Config: map[string]interface{}{
			"output_path": "/tmp",
		},
	}

	ctx := context.Background()
	result, err := exporter.Export(ctx, testData, config)

	if err == nil {
		t.Error("Should return error when backup provider fails")
	}

	if result != nil && result.Success {
		t.Error("Result should indicate failure")
	}
}

func TestBackupExporter_Preview(t *testing.T) {
	mockProvider := &MockBackupProvider{}
	mockDB := &MockDatabaseManager{provider: mockProvider}
	exporter := NewBackupExporter(mockDB)
	exporter.Initialize(logging.GetDefault())

	testData := &export.ExportData{
		Devices: []export.DeviceData{
			{ID: 1, Name: "Device 1"},
			{ID: 2, Name: "Device 2"},
		},
		Configurations: []export.ConfigurationData{
			{DeviceID: 1},
		},
		Templates: []export.TemplateData{
			{ID: 1, Name: "Template 1"},
		},
	}

	config := export.ExportConfig{
		Config: map[string]interface{}{
			"compression": true,
		},
	}

	ctx := context.Background()
	result, err := exporter.Preview(ctx, testData, config)

	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if !result.Success {
		t.Error("Preview should succeed")
	}

	if result.RecordCount != 4 { // 2 devices + 1 config + 1 template
		t.Errorf("Expected record count 4, got %d", result.RecordCount)
	}

	if len(result.SampleData) == 0 {
		t.Error("Preview should contain sample data")
	}

	// With compression, estimated size should be smaller
	if result.EstimatedSize >= 4*500 { // 4 records * 500 bytes average
		t.Error("Compressed size should be smaller than uncompressed")
	}
}

func TestBackupExporter_Capabilities(t *testing.T) {
	mockDB := &MockDatabaseManager{}
	exporter := NewBackupExporter(mockDB)

	caps := exporter.Capabilities()

	if !caps.SupportsIncremental {
		t.Error("Backup should support incremental backups")
	}

	if !caps.SupportsScheduling {
		t.Error("Backup should support scheduling")
	}

	if caps.RequiresAuthentication {
		t.Error("Backup should not require authentication")
	}

	if caps.ConcurrencyLevel != 1 {
		t.Error("Backup should have concurrency level 1")
	}
}

func TestBackupExporter_RestoreBackup(t *testing.T) {
	// Create temporary backup file
	tmpDir, err := os.MkdirTemp("", "restore_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backupPath := filepath.Join(tmpDir, "test-backup.sma")

	// Create dummy backup file
	if err := os.WriteFile(backupPath, []byte("dummy backup data"), 0644); err != nil {
		t.Fatalf("Failed to create backup file: %v", err)
	}

	mockProvider := &MockBackupProvider{}
	mockDB := &MockDatabaseManager{provider: mockProvider}
	exporter := NewBackupExporter(mockDB)
	exporter.Initialize(logging.GetDefault())

	ctx := context.Background()
	options := map[string]interface{}{
		"dry_run":       false,
		"preserve_data": true,
	}

	result, err := exporter.RestoreBackup(ctx, backupPath, options)

	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if !result.Success {
		t.Error("Restore should succeed")
	}

	if result.RecordsImported != 10 { // From mock
		t.Errorf("Expected 10 records imported, got %d", result.RecordsImported)
	}
}

func TestBackupExporter_ValidateBackup(t *testing.T) {
	// Create temporary backup file
	tmpDir, err := os.MkdirTemp("", "validate_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backupPath := filepath.Join(tmpDir, "test-backup.sma")

	// Create dummy backup file
	if err := os.WriteFile(backupPath, []byte("dummy backup data"), 0644); err != nil {
		t.Fatalf("Failed to create backup file: %v", err)
	}

	mockProvider := &MockBackupProvider{}
	mockDB := &MockDatabaseManager{provider: mockProvider}
	exporter := NewBackupExporter(mockDB)
	exporter.Initialize(logging.GetDefault())

	ctx := context.Background()
	result, err := exporter.ValidateBackup(ctx, backupPath)

	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !result.Valid {
		t.Error("Backup should be valid")
	}

	if result.RecordCount != 10 {
		t.Errorf("Expected 10 records, got %d", result.RecordCount)
	}

	if !result.ChecksumValid {
		t.Error("Checksum should be valid")
	}
}

// BackupError is a simple error type for testing
type BackupError struct {
	message string
}

func (e *BackupError) Error() string {
	return e.message
}

package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// MockDBForImport implements DatabaseManagerInterface for import testing
type MockDBForImport struct {
	provider provider.DatabaseProvider
}

func (db *MockDBForImport) GetProvider() provider.DatabaseProvider {
	return db.provider
}

func (db *MockDBForImport) GetDB() *gorm.DB {
	return nil // Return nil for testing mode
}

func (db *MockDBForImport) Close() error {
	return nil
}

func TestNewImportEngine(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}
	mockExportEngine := NewExportEngine(mockDB, logger)

	importEngine := NewImportEngine(mockExportEngine, mockDB, logger)

	if importEngine == nil {
		t.Fatal("NewImportEngine returned nil")
	}

	if importEngine.exportEngine != mockExportEngine {
		t.Error("Import engine export engine not set correctly")
	}

	if importEngine.dbManager != mockDB {
		t.Error("Import engine dbManager not set correctly")
	}

	if importEngine.logger != logger {
		t.Error("Import engine logger not set correctly")
	}
}

func TestImportEngine_ImportGitOps(t *testing.T) {
	// Create temporary GitOps structure
	tmpDir, err := os.MkdirTemp("", "gitops_import_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create GitOps structure
	err = createTestGitOpsStructure(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create test GitOps structure: %v", err)
	}

	// Setup mocks
	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}
	mockExportEngine := NewExportEngine(mockDB, logger)
	importEngine := NewImportEngine(mockExportEngine, mockDB, logger)

	// Create import request
	request := ImportRequest{
		PluginName: "gitops",
		Format:     "yaml",
		Source: ImportSource{
			Type: "file",
			Path: tmpDir,
		},
		Options: ImportOptions{
			DryRun: true, // Start with dry run
		},
	}

	ctx := context.Background()
	result, err := importEngine.Import(ctx, request)

	if err != nil {
		t.Fatalf("GitOps import failed: %v", err)
	}

	if !result.Success {
		t.Error("GitOps import should succeed")
	}

	if len(result.Changes) == 0 {
		t.Error("Should detect changes in dry run mode")
	}

	// Check that we have the expected import ID
	if result.ImportID == "" {
		t.Error("Result should contain import ID")
	}
}

func TestImportEngine_PreviewImport(t *testing.T) {
	// Create temporary GitOps structure
	tmpDir, err := os.MkdirTemp("", "gitops_preview_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create GitOps structure
	err = createTestGitOpsStructure(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create test GitOps structure: %v", err)
	}

	// Setup mocks
	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}
	mockExportEngine := NewExportEngine(mockDB, logger)
	importEngine := NewImportEngine(mockExportEngine, mockDB, logger)

	// Create preview request
	request := ImportRequest{
		PluginName: "gitops",
		Format:     "yaml",
		Source: ImportSource{
			Type: "file",
			Path: tmpDir,
		},
	}

	ctx := context.Background()
	result, err := importEngine.PreviewImport(ctx, request)

	if err != nil {
		t.Fatalf("GitOps preview failed: %v", err)
	}

	if !result.Success {
		t.Error("GitOps preview should succeed")
	}

	// Preview should force dry run and validate only
	if len(result.Changes) == 0 {
		t.Error("Preview should show changes that would be made")
	}
}

func TestImportEngine_ImportUnsupportedPlugin(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}
	mockExportEngine := NewExportEngine(mockDB, logger)
	importEngine := NewImportEngine(mockExportEngine, mockDB, logger)

	request := ImportRequest{
		PluginName: "unsupported-plugin",
		Format:     "unknown",
		Source: ImportSource{
			Type: "file",
			Path: "/tmp/test",
		},
	}

	ctx := context.Background()
	_, err := importEngine.Import(ctx, request)

	if err == nil {
		t.Error("Should return error for unsupported plugin")
	}
}

func TestNewGitOpsImporter(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}

	importer := NewGitOpsImporter(mockDB, logger)

	if importer == nil {
		t.Fatal("NewGitOpsImporter returned nil")
	}

	if importer.dbManager != mockDB {
		t.Error("Importer dbManager not set correctly")
	}

	if importer.logger != logger {
		t.Error("Importer logger not set correctly")
	}
}

func TestGitOpsImporter_LoadGitOpsStructure(t *testing.T) {
	// Create temporary GitOps structure
	tmpDir, err := os.MkdirTemp("", "load_structure_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create GitOps structure
	err = createTestGitOpsStructure(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create test GitOps structure: %v", err)
	}

	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}
	importer := NewGitOpsImporter(mockDB, logger)

	data, err := importer.LoadGitOpsStructure(tmpDir)

	if err != nil {
		t.Fatalf("Failed to load GitOps structure: %v", err)
	}

	if data == nil {
		t.Fatal("GitOps data should not be nil")
	}

	// Check common config was loaded
	if data.CommonConfig == nil {
		t.Error("Common config should be loaded")
	}

	// Check groups were loaded
	if len(data.Groups) == 0 {
		t.Error("Groups should be loaded")
	}

	// Check devices were flattened
	if len(data.Devices) == 0 {
		t.Error("Devices should be flattened from groups")
	}
}

func TestGitOpsImporter_MergeConfigs(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}
	importer := NewGitOpsImporter(mockDB, logger)

	common := map[string]interface{}{
		"wifi": map[string]interface{}{
			"ssid":     "CommonNetwork",
			"password": "common123",
		},
		"mqtt": map[string]interface{}{
			"enabled": true,
			"port":    1883,
		},
	}

	group := map[string]interface{}{
		"wifi": map[string]interface{}{
			"ssid": "GroupNetwork", // Override common
		},
		"mqtt": map[string]interface{}{
			"topic_prefix": "group/prefix",
		},
	}

	device := map[string]interface{}{
		"name": "Test Device",
		"wifi": map[string]interface{}{
			"password": "device123", // Override common and group
		},
	}

	result := importer.mergeConfigs(common, group, device)

	// Check that device overrides took effect
	if wifi, ok := result["wifi"].(map[string]interface{}); ok {
		if wifi["ssid"] != "GroupNetwork" {
			t.Error("Group WiFi SSID should override common")
		}
		if wifi["password"] != "device123" {
			t.Error("Device WiFi password should override all others")
		}
	} else {
		t.Error("Merged config should have wifi section")
	}

	// Check that non-conflicting values are preserved
	if mqtt, ok := result["mqtt"].(map[string]interface{}); ok {
		if mqtt["enabled"] != true {
			t.Error("Common MQTT enabled should be preserved")
		}
		if mqtt["port"] != 1883 {
			t.Error("Common MQTT port should be preserved")
		}
		if mqtt["topic_prefix"] != "group/prefix" {
			t.Error("Group MQTT topic_prefix should be preserved")
		}
	} else {
		t.Error("Merged config should have mqtt section")
	}

	// Check device-specific values
	if result["name"] != "Test Device" {
		t.Error("Device name should be preserved")
	}
}

func TestGitOpsImporter_PreviewChanges(t *testing.T) {
	logger := logging.GetDefault()
	mockDB := &MockDBForImport{}
	importer := NewGitOpsImporter(mockDB, logger)

	gitopsData := &GitOpsData{
		Devices: []GitOpsDevice{
			{
				Name: "New Device Name", // Different from existing
				MAC:  "00:11:22:33:44:55",
				Type: "SHSW-1",
				MergedConfig: map[string]interface{}{
					"some": "config",
				},
			},
			{
				Name: "Completely New Device",
				MAC:  "00:11:22:33:44:66", // New MAC
				Type: "SHPLG-S",
				MergedConfig: map[string]interface{}{
					"new": "config",
				},
			},
		},
	}

	ctx := context.Background()
	changes := importer.PreviewChanges(ctx, gitopsData)

	if len(changes) == 0 {
		t.Error("Should detect changes")
	}

	// Should have at least one update and one create
	hasUpdate := false
	hasCreate := false

	for _, change := range changes {
		if change.Type == "update" {
			hasUpdate = true
		}
		if change.Type == "create" {
			hasCreate = true
		}
	}

	if !hasUpdate {
		t.Error("Should detect device update")
	}

	if !hasCreate {
		t.Error("Should detect device creation")
	}
}

// Helper function to create test GitOps structure
func createTestGitOpsStructure(baseDir string) error {
	// Create common.yaml
	commonConfig := map[string]interface{}{
		"wifi": map[string]interface{}{
			"ssid":     "{{ .Global.wifi_ssid }}",
			"password": "{{ .Global.wifi_password }}",
		},
		"mqtt": map[string]interface{}{
			"enabled": true,
			"server":  "192.168.1.100",
		},
	}

	commonPath := filepath.Join(baseDir, "common.yaml")
	if err := writeYAMLFile(commonPath, commonConfig); err != nil {
		return err
	}

	// Create groups structure
	groupsDir := filepath.Join(baseDir, "groups", "living-room")
	if err := os.MkdirAll(groupsDir, 0755); err != nil {
		return err
	}

	// Create group config
	groupConfig := map[string]interface{}{
		"mqtt": map[string]interface{}{
			"topic_prefix": "living-room",
		},
	}

	groupConfigPath := filepath.Join(groupsDir, "group.yaml")
	if err := writeYAMLFile(groupConfigPath, groupConfig); err != nil {
		return err
	}

	// Create device type structure
	relayDir := filepath.Join(groupsDir, "relay")
	if err := os.MkdirAll(relayDir, 0755); err != nil {
		return err
	}

	// Create type common config
	typeConfig := map[string]interface{}{
		"relay": map[string]interface{}{
			"auto_on": false,
		},
	}

	typeConfigPath := filepath.Join(relayDir, "common.yaml")
	if err := writeYAMLFile(typeConfigPath, typeConfig); err != nil {
		return err
	}

	// Create device config
	deviceConfig := map[string]interface{}{
		"name": "Living Room Light",
		"mac":  "00:11:22:33:44:55",
		"type": "SHSW-1",
		"relay": map[string]interface{}{
			"default_state": "off",
		},
	}

	devicePath := filepath.Join(relayDir, "ceiling-light.yaml")
	if err := writeYAMLFile(devicePath, deviceConfig); err != nil {
		return err
	}

	return nil
}

// Helper function to write YAML file
func writeYAMLFile(filePath string, data interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer func() { _ = encoder.Close() }()

	return encoder.Encode(data)
}

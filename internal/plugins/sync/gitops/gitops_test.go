package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

func TestGitOpsExporter_Info(t *testing.T) {
	exporter := NewGitOpsExporter()

	info := exporter.Info()

	if info.Name != "gitops" {
		t.Errorf("Expected name 'gitops', got '%s'", info.Name)
	}

	if info.Category != sync.CategoryGitOps {
		t.Errorf("Expected category %v, got %v", sync.CategoryGitOps, info.Category)
	}

	if len(info.SupportedFormats) == 0 {
		t.Error("Should support at least one format")
	}

	// Check that yaml format is supported
	yamlSupported := false
	for _, format := range info.SupportedFormats {
		if format == "yaml" {
			yamlSupported = true
			break
		}
	}
	if !yamlSupported {
		t.Error("Should support YAML format")
	}
}

func TestGitOpsExporter_ConfigSchema(t *testing.T) {
	exporter := NewGitOpsExporter()

	schema := exporter.ConfigSchema()

	if schema.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", schema.Version)
	}

	if len(schema.Properties) == 0 {
		t.Error("Schema should have properties")
	}

	// Check for expected properties
	expectedProps := []string{"output_path", "group_by", "include_common", "include_templates"}
	for _, prop := range expectedProps {
		if _, exists := schema.Properties[prop]; !exists {
			t.Errorf("Schema should have '%s' property", prop)
		}
	}
}

func TestGitOpsExporter_ValidateConfig(t *testing.T) {
	exporter := NewGitOpsExporter()

	// Test valid config
	validConfig := map[string]interface{}{
		"output_path":       "/tmp/test-gitops",
		"group_by":          "location",
		"include_common":    true,
		"include_templates": true,
	}

	err := exporter.ValidateConfig(validConfig)
	if err != nil {
		t.Errorf("Valid config should not return error: %v", err)
	}

	// Test invalid group_by
	invalidConfig := map[string]interface{}{
		"group_by": "invalid_grouping",
	}

	err = exporter.ValidateConfig(invalidConfig)
	if err == nil {
		t.Error("Invalid group_by should return error")
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

func TestGitOpsExporter_GroupDevices(t *testing.T) {
	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	devices := []sync.DeviceData{
		{ID: 1, Name: "Living Room Light", Type: "SHSW-1", MAC: "00:11:22:33:44:55"},
		{ID: 2, Name: "Kitchen Switch", Type: "SHSW-1", MAC: "00:11:22:33:44:66"},
		{ID: 3, Name: "Outdoor Plug", Type: "SHPLG-S", MAC: "00:11:22:33:44:77"},
		{ID: 4, Name: "Random Device", Type: "SHSW-1", MAC: "00:11:22:33:44:88"},
	}

	// Test location-based grouping
	groups := exporter.groupDevices(devices, "location", nil)

	// Should have groups for "living", "kitchen", "outdoor"
	if len(groups) < 3 {
		t.Error("Should have at least 3 location-based groups")
	}

	if _, exists := groups["living"]; !exists {
		t.Error("Should have 'living' group")
	}

	if _, exists := groups["kitchen"]; !exists {
		t.Error("Should have 'kitchen' group")
	}

	if _, exists := groups["outdoor"]; !exists {
		t.Error("Should have 'outdoor' group")
	}

	// Test type-based grouping
	typeGroups := exporter.groupDevices(devices, "type", nil)

	if _, exists := typeGroups["shsw-1"]; !exists {
		t.Error("Should have 'shsw-1' type group")
	}

	if _, exists := typeGroups["shplg-s"]; !exists {
		t.Error("Should have 'shplg-s' type group")
	}

	if len(typeGroups["shsw-1"]) != 3 {
		t.Errorf("Expected 3 devices in 'shsw-1' group, got %d", len(typeGroups["shsw-1"]))
	}
}

func TestGitOpsExporter_GroupDevicesByType(t *testing.T) {
	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	devices := []sync.DeviceData{
		{ID: 1, Name: "Device 1", Type: "SHSW-1"},
		{ID: 2, Name: "Device 2", Type: "SHSW-1"},
		{ID: 3, Name: "Device 3", Type: "SHPLG-S"},
	}

	typeGroups := exporter.groupDevicesByType(devices)

	if len(typeGroups) != 2 {
		t.Errorf("Expected 2 type groups, got %d", len(typeGroups))
	}

	if len(typeGroups["shsw-1"]) != 2 {
		t.Errorf("Expected 2 devices in 'shsw-1' group, got %d", len(typeGroups["shsw-1"]))
	}

	if len(typeGroups["shplg-s"]) != 1 {
		t.Errorf("Expected 1 device in 'shplg-s' group, got %d", len(typeGroups["shplg-s"]))
	}
}

func TestGitOpsExporter_ExtractLocationFromName(t *testing.T) {
	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	testCases := []struct {
		name     string
		expected string
	}{
		{"Living Room Light", "living"},
		{"Kitchen Switch", "kitchen"},
		{"Bedroom Dimmer", "bedroom"},
		{"Outdoor Plug", "outdoor"},
		{"Garage Door", "garage"},
		{"Office Light", "office"},
		{"Random Device", ""}, // No location keyword
	}

	for _, tc := range testCases {
		result := exporter.extractLocationFromName(tc.name)
		if result != tc.expected {
			t.Errorf("For name '%s', expected '%s', got '%s'", tc.name, tc.expected, result)
		}
	}
}

func TestGitOpsExporter_GenerateCommonConfig(t *testing.T) {
	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	testData := &sync.ExportData{
		Metadata: sync.ExportMetadata{
			SystemVersion: "1.0.0",
			TotalDevices:  5,
		},
		Timestamp: time.Now(),
	}

	commonConfig := exporter.generateCommonConfig(testData)

	// Check that common config has expected structure
	if wifi, ok := commonConfig["wifi"].(map[string]interface{}); ok {
		if wifi["ssid"] != "{{ .Global.wifi_ssid }}" {
			t.Error("WiFi SSID should use template variable")
		}
	} else {
		t.Error("Common config should have wifi section")
	}

	if mqtt, ok := commonConfig["mqtt"].(map[string]interface{}); ok {
		if mqtt["enabled"] != true {
			t.Error("MQTT should be enabled by default")
		}
	} else {
		t.Error("Common config should have mqtt section")
	}

	if metadata, ok := commonConfig["metadata"].(map[string]interface{}); ok {
		if metadata["system_version"] != "1.0.0" {
			t.Error("Metadata should contain system version")
		}
	} else {
		t.Error("Common config should have metadata section")
	}
}

func TestGitOpsExporter_GenerateDeviceConfig(t *testing.T) {
	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	device := sync.DeviceData{
		ID:       1,
		Name:     "Test Device",
		MAC:      "00:11:22:33:44:55",
		Type:     "SHSW-1",
		Firmware: "1.0.0",
		Settings: map[string]interface{}{
			"name":       "Test Device",
			"relay_mode": "switch",
			"created_at": "2023-01-01T00:00:00Z",
			"updated_at": "2023-01-01T00:00:00Z",
			"id":         1,
		},
	}

	excludeFields := []string{"id", "created_at", "updated_at"}

	deviceConfig := exporter.generateDeviceConfig(device, excludeFields)

	// Check basic fields
	if deviceConfig["name"] != "Test Device" {
		t.Error("Device config should contain name")
	}

	if deviceConfig["mac"] != "00:11:22:33:44:55" {
		t.Error("Device config should contain MAC")
	}

	if deviceConfig["type"] != "SHSW-1" {
		t.Error("Device config should contain type")
	}

	// Check settings filtering
	if settings, ok := deviceConfig["settings"].(map[string]interface{}); ok {
		// Should include relay_mode
		if settings["relay_mode"] != "switch" {
			t.Error("Settings should include relay_mode")
		}

		// Should exclude id, created_at, updated_at
		if _, exists := settings["id"]; exists {
			t.Error("Settings should not include excluded field 'id'")
		}

		if _, exists := settings["created_at"]; exists {
			t.Error("Settings should not include excluded field 'created_at'")
		}

		if _, exists := settings["updated_at"]; exists {
			t.Error("Settings should not include excluded field 'updated_at'")
		}
	} else {
		t.Error("Device config should have settings section")
	}
}

func TestGitOpsExporter_SanitizeFilename(t *testing.T) {
	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	testCases := []struct {
		input    string
		expected string
	}{
		{"Living Room Light", "living-room-light"},
		{"Kitchen/Switch", "kitchen-switch"},
		{"Device:With:Colons", "device-with-colons"},
		{"Device*With*Stars", "devicewithstars"},
		{"Device\"With\"Quotes", "devicewithquotes"},
		{"UPPERCASE", "uppercase"},
		{"Mixed CaSe", "mixed-case"},
	}

	for _, tc := range testCases {
		result := exporter.sanitizeFilename(tc.input)
		if result != tc.expected {
			t.Errorf("For input '%s', expected '%s', got '%s'", tc.input, tc.expected, result)
		}
	}
}

func TestGitOpsExporter_Export(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "gitops_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	// Create test data
	testData := &sync.ExportData{
		Devices: []sync.DeviceData{
			{ID: 1, Name: "Living Room Light", Type: "SHSW-1", MAC: "00:11:22:33:44:55"},
			{ID: 2, Name: "Kitchen Switch", Type: "SHSW-1", MAC: "00:11:22:33:44:66"},
		},
		Templates: []sync.TemplateData{
			{ID: 1, Name: "Basic Switch", DeviceType: "SHSW-1", IsDefault: true},
		},
		Metadata: sync.ExportMetadata{
			ExportID:      "test-export-123",
			SystemVersion: "1.0.0",
			TotalDevices:  2,
		},
		Timestamp: time.Now(),
	}

	// Create export config
	config := sync.ExportConfig{
		Format: "yaml",
		Config: map[string]interface{}{
			"output_path":       tmpDir,
			"group_by":          "location",
			"include_common":    true,
			"include_templates": true,
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

	if result.OutputPath != tmpDir {
		t.Errorf("Expected output path %s, got %s", tmpDir, result.OutputPath)
	}

	// Check that files were created
	commonPath := filepath.Join(tmpDir, "common.yaml")
	if _, err := os.Stat(commonPath); os.IsNotExist(err) {
		t.Error("common.yaml should be created")
	}

	summaryPath := filepath.Join(tmpDir, "export-summary.yaml")
	if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
		t.Error("export-summary.yaml should be created")
	}

	// Check that group directories were created
	groupsPath := filepath.Join(tmpDir, "groups")
	if _, err := os.Stat(groupsPath); os.IsNotExist(err) {
		t.Error("groups directory should be created")
	}

	// Check that template files were created
	templatesPath := filepath.Join(tmpDir, "templates")
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		t.Error("templates directory should be created")
	}
}

func TestGitOpsExporter_Preview(t *testing.T) {
	exporter := NewGitOpsExporter()
	exporter.Initialize(logging.GetDefault())

	testData := &sync.ExportData{
		Devices: []sync.DeviceData{
			{ID: 1, Name: "Living Room Light", Type: "SHSW-1"},
			{ID: 2, Name: "Kitchen Switch", Type: "SHSW-1"},
			{ID: 3, Name: "Random Device", Type: "SHPLG-S"},
		},
		Templates: []sync.TemplateData{
			{ID: 1, Name: "Template 1"},
		},
	}

	config := sync.ExportConfig{
		Config: map[string]interface{}{
			"group_by":          "location",
			"include_templates": true,
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

	if len(result.SampleData) == 0 {
		t.Error("Preview should contain sample data")
	}

	// Should have reasonable estimate of files
	if result.RecordCount < 3 { // At least common, summary, and some device files
		t.Errorf("Expected at least 3 files, got %d", result.RecordCount)
	}

	if result.EstimatedSize <= 0 {
		t.Error("Estimated size should be positive")
	}

	// Check that preview contains structure information
	previewStr := string(result.SampleData)
	if !containsString(previewStr, "GitOps Export Structure Preview") {
		t.Error("Preview should contain structure information")
	}
}

func TestGitOpsExporter_Capabilities(t *testing.T) {
	exporter := NewGitOpsExporter()

	caps := exporter.Capabilities()

	if caps.SupportsIncremental {
		t.Error("GitOps should not support incremental exports")
	}

	if !caps.SupportsScheduling {
		t.Error("GitOps should support scheduling")
	}

	if caps.RequiresAuthentication {
		t.Error("GitOps should not require authentication")
	}

	if caps.ConcurrencyLevel != 1 {
		t.Error("GitOps should have concurrency level 1")
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || containsString(s[1:], substr) || (len(s) > 0 && s[:len(substr)] == substr))
}

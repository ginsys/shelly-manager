package jsonexport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

func TestPlugin_Info(t *testing.T) {
	p := NewPlugin()
	info := p.Info()

	if info.Name != "json" {
		t.Errorf("Expected name 'json', got '%s'", info.Name)
	}

	if info.Category != sync.CategoryCustom {
		t.Errorf("Expected category %v, got %v", sync.CategoryCustom, info.Category)
	}

	if info.Version == "" {
		t.Error("Expected non-empty version")
	}
}

func TestPlugin_ConfigSchema(t *testing.T) {
	p := NewPlugin()
	schema := p.ConfigSchema()

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

	if _, exists := schema.Properties["pretty"]; !exists {
		t.Error("Schema should have 'pretty' property")
	}

	if _, exists := schema.Properties["compression"]; !exists {
		t.Error("Schema should have 'compression' property")
	}
}

func TestPlugin_Export_Success(t *testing.T) {
	p := NewPlugin()
	if err := p.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test data
	data := &sync.ExportData{
		Devices: []sync.DeviceData{
			{ID: 1, Name: "Test Device", Type: "shelly1"},
		},
		Templates: []sync.TemplateData{
			{ID: 1, Name: "Test Template"},
		},
	}

	// Temp directory for output
	tmpDir := t.TempDir()

	config := sync.ExportConfig{
		Format: "json",
		Config: map[string]interface{}{
			"output_path": tmpDir,
			"pretty":      true,
		},
	}

	result, err := p.Export(context.Background(), data, config)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Errorf("Output file not created: %s", result.OutputPath)
	}

	// Verify JSON structure
	content, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(content, &envelope); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify structure
	if _, ok := envelope["devices"]; !ok {
		t.Error("Expected 'devices' field in JSON output")
	}
	if _, ok := envelope["templates"]; !ok {
		t.Error("Expected 'templates' field in JSON output")
	}
}

func TestPlugin_Export_Compression(t *testing.T) {
	tests := []struct {
		name            string
		compressionAlgo string
		expectExt       string
	}{
		{"gzip compression", "gzip", ".json.gz"},
		{"zip compression", "zip", ".json.zip"},
		{"no compression", "none", ".json"},
		{"default to none", "", ".json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlugin()
			if err := p.Initialize(logging.GetDefault()); err != nil {
				t.Fatalf("Initialize failed: %v", err)
			}

			data := &sync.ExportData{
				Devices: []sync.DeviceData{{ID: 1}},
			}

			tmpDir := t.TempDir()
			config := sync.ExportConfig{
				Format: "json",
				Config: map[string]interface{}{
					"output_path":      tmpDir,
					"compression":      tt.compressionAlgo != "" && tt.compressionAlgo != "none",
					"compression_algo": tt.compressionAlgo,
				},
			}

			result, err := p.Export(context.Background(), data, config)
			if err != nil {
				t.Fatalf("Export failed: %v", err)
			}

			// Verify file extension
			ext := filepath.Ext(result.OutputPath)
			if tt.compressionAlgo == "zip" || tt.compressionAlgo == "gzip" {
				// For .json.zip and .json.gz, we need to check both extensions
				ext = filepath.Ext(result.OutputPath[:len(result.OutputPath)-len(ext)]) + ext
			}
			if ext != tt.expectExt {
				t.Errorf("Expected extension %s, got %s (path: %s)", tt.expectExt, ext, result.OutputPath)
			}

			// Verify file exists
			if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
				t.Errorf("Output file not created: %s", result.OutputPath)
			}
		})
	}
}

func TestPlugin_Export_InvalidPath(t *testing.T) {
	p := NewPlugin()
	if err := p.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	data := &sync.ExportData{
		Devices: []sync.DeviceData{{ID: 1}},
	}

	config := sync.ExportConfig{
		Format: "json",
		Config: map[string]interface{}{
			"output_path": "/nonexistent/invalid/path",
		},
	}

	_, err := p.Export(context.Background(), data, config)
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestPlugin_Preview(t *testing.T) {
	p := NewPlugin()
	if err := p.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	testData := &sync.ExportData{
		Devices: []sync.DeviceData{
			{ID: 1, Name: "Device 1"},
			{ID: 2, Name: "Device 2"},
		},
		Templates: []sync.TemplateData{
			{ID: 1, Name: "Template 1"},
		},
		DiscoveredDevices: []sync.DiscoveredDeviceData{
			{MAC: "AA:BB:CC:DD:EE:FF"},
		},
	}

	config := sync.ExportConfig{
		Config: map[string]interface{}{
			"include_discovered": true,
		},
	}

	ctx := context.Background()
	result, err := p.Preview(ctx, testData, config)

	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if !result.Success {
		t.Error("Preview should succeed")
	}

	// Total: 2 devices + 1 template + 1 discovered = 4
	if result.RecordCount != 4 {
		t.Errorf("Expected record count 4, got %d", result.RecordCount)
	}

	if result.EstimatedSize <= 0 {
		t.Error("Estimated size should be positive")
	}
}

func TestPlugin_Capabilities(t *testing.T) {
	p := NewPlugin()
	caps := p.Capabilities()

	if !caps.SupportsScheduling {
		t.Error("Plugin should support scheduling")
	}

	if len(caps.SupportedOutputs) == 0 {
		t.Error("Plugin should have supported outputs")
	}

	if caps.ConcurrencyLevel != 1 {
		t.Errorf("Expected concurrency level 1, got %d", caps.ConcurrencyLevel)
	}
}

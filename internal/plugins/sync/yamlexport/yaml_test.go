package yamlexport

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

func TestPlugin_Metadata(t *testing.T) {
	p := NewPlugin()

	// Test Info
	info := p.Info()
	if info.Name != "yaml" {
		t.Errorf("Expected name 'yaml', got '%s'", info.Name)
	}
	if info.Version == "" {
		t.Error("Expected non-empty version")
	}
	if info.Category != sync.CategoryCustom {
		t.Errorf("Expected category CategoryCustom, got %v", info.Category)
	}

	// Test ConfigSchema
	schema := p.ConfigSchema()
	if schema.Version != "1.0" {
		t.Errorf("Expected schema version '1.0', got '%s'", schema.Version)
	}
	if _, ok := schema.Properties["output_path"]; !ok {
		t.Error("Expected 'output_path' property in schema")
	}
	if _, ok := schema.Properties["compression_algo"]; !ok {
		t.Error("Expected 'compression_algo' property in schema")
	}

	// Test Capabilities
	caps := p.Capabilities()
	if !caps.SupportsScheduling {
		t.Error("Expected plugin to support scheduling")
	}
}

func TestPlugin_Initialize(t *testing.T) {
	p := NewPlugin()
	logger := logging.GetDefault()

	err := p.Initialize(logger)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test Cleanup
	err = p.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func TestPlugin_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError bool
	}{
		{
			name:      "empty config",
			config:    map[string]interface{}{},
			wantError: false,
		},
		{
			name: "valid output path",
			config: map[string]interface{}{
				"output_path": t.TempDir(),
			},
			wantError: false,
		},
		{
			name: "invalid output path",
			config: map[string]interface{}{
				"output_path": "/nonexistent/invalid/path/that/cannot/be/created",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlugin()
			err := p.ValidateConfig(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestPlugin_Export_Success(t *testing.T) {
	p := NewPlugin().(*Plugin)
	if err := p.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test data
	data := &sync.ExportData{
		Devices: []sync.DeviceData{
			{
				ID:       1,
				MAC:      "AA:BB:CC:DD:EE:FF",
				IP:       "192.168.1.100",
				Type:     "shelly1",
				Name:     "Test Device",
				Model:    "SHSW-1",
				Firmware: "1.0.0",
				Status:   "online",
				LastSeen: time.Now(),
				Settings: map[string]interface{}{"key": "value"},
			},
		},
		Templates: []sync.TemplateData{
			{
				ID:          1,
				Name:        "Test Template",
				Description: "A test template",
				DeviceType:  "shelly1",
				Config:      map[string]interface{}{"setting": "value"},
			},
		},
		Metadata: sync.ExportMetadata{
			ExportID:   "test-export-123",
			ExportType: "manual",
		},
	}

	// Temp directory for output
	tmpDir := t.TempDir()

	config := sync.ExportConfig{
		Format: "yaml",
		Config: map[string]interface{}{
			"output_path": tmpDir,
		},
	}

	result, err := p.Export(context.Background(), data, config)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify result
	if !result.Success {
		t.Error("Expected export to succeed")
	}
	if result.RecordCount != 1 {
		t.Errorf("Expected record count 1, got %d", result.RecordCount)
	}
	if result.OutputPath == "" {
		t.Error("Expected non-empty output path")
	}
	if result.Checksum == "" {
		t.Error("Expected non-empty checksum")
	}

	// Verify file exists
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Errorf("Output file not created: %s", result.OutputPath)
	}

	// Verify YAML structure
	content, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var envelope yamlEnvelope
	if err := yaml.Unmarshal(content, &envelope); err != nil {
		t.Fatalf("Output is not valid YAML: %v", err)
	}

	// Verify structure
	if len(envelope.Devices) != 1 {
		t.Errorf("Expected 1 device in YAML, got %d", len(envelope.Devices))
	}
	if len(envelope.Templates) != 1 {
		t.Errorf("Expected 1 template in YAML, got %d", len(envelope.Templates))
	}
	if envelope.Metadata.ExportID != "test-export-123" {
		t.Errorf("Expected export ID 'test-export-123', got '%s'", envelope.Metadata.ExportID)
	}
	if envelope.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", envelope.Version)
	}
}

func TestPlugin_Export_Compression(t *testing.T) {
	tests := []struct {
		name            string
		compression     bool
		compressionAlgo string
		expectExt       string
	}{
		{"no compression", false, "", ".yaml"},
		{"gzip compression", true, "gzip", ".yaml.gz"},
		{"zip compression", true, "zip", ".yaml.zip"},
		{"default to gzip", true, "", ".yaml.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlugin().(*Plugin)
			_ = p.Initialize(logging.GetDefault())

			data := &sync.ExportData{
				Devices: []sync.DeviceData{
					{ID: 1, Name: "Test", Type: "shelly1"},
				},
				Metadata: sync.ExportMetadata{
					ExportID: "test",
				},
			}

			tmpDir := t.TempDir()
			config := sync.ExportConfig{
				Format: "yaml",
				Config: map[string]interface{}{
					"output_path":      tmpDir,
					"compression":      tt.compression,
					"compression_algo": tt.compressionAlgo,
				},
			}

			result, err := p.Export(context.Background(), data, config)
			if err != nil {
				t.Fatalf("Export failed: %v", err)
			}

			// Verify file extension
			base := filepath.Base(result.OutputPath)
			if tt.expectExt == ".yaml.zip" || tt.expectExt == ".yaml.gz" {
				// For double extensions, check the last two parts
				if !hasDoubleSuffix(base, tt.expectExt) {
					t.Errorf("Expected file with extension %s, got %s", tt.expectExt, result.OutputPath)
				}
			} else {
				ext := filepath.Ext(result.OutputPath)
				if ext != tt.expectExt {
					t.Errorf("Expected extension %s, got %s", tt.expectExt, ext)
				}
			}

			// Verify file exists
			if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
				t.Errorf("Output file not created: %s", result.OutputPath)
			}

			// Verify file size is reasonable
			if result.FileSize == 0 {
				t.Error("Expected non-zero file size")
			}
		})
	}
}

// hasDoubleSuffix checks if filename ends with double extension like .yaml.gz
func hasDoubleSuffix(filename, suffix string) bool {
	return len(filename) >= len(suffix) && filename[len(filename)-len(suffix):] == suffix
}

func TestPlugin_Export_InvalidPath(t *testing.T) {
	p := NewPlugin().(*Plugin)
	_ = p.Initialize(logging.GetDefault())

	data := &sync.ExportData{
		Devices: []sync.DeviceData{{ID: 1, Name: "Test"}},
		Metadata: sync.ExportMetadata{
			ExportID: "test",
		},
	}

	config := sync.ExportConfig{
		Format: "yaml",
		Config: map[string]interface{}{
			"output_path": "/nonexistent/invalid/path/that/cannot/be/created",
		},
	}

	_, err := p.Export(context.Background(), data, config)
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestPlugin_Export_IncludeDiscovered(t *testing.T) {
	p := NewPlugin().(*Plugin)
	_ = p.Initialize(logging.GetDefault())

	data := &sync.ExportData{
		Devices: []sync.DeviceData{
			{ID: 1, Name: "Device1", Type: "shelly1"},
		},
		DiscoveredDevices: []sync.DiscoveredDeviceData{
			{IP: "192.168.1.10", MAC: "11:22:33:44:55:66", Model: "SHSW-1", Generation: 1},
		},
		Metadata: sync.ExportMetadata{
			ExportID: "test",
		},
	}

	tmpDir := t.TempDir()

	// Test with include_discovered = true
	t.Run("with discovered devices", func(t *testing.T) {
		config := sync.ExportConfig{
			Format: "yaml",
			Config: map[string]interface{}{
				"output_path":        tmpDir,
				"include_discovered": true,
			},
		}

		result, err := p.Export(context.Background(), data, config)
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		content, err := os.ReadFile(result.OutputPath)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		var envelope yamlEnvelope
		if err := yaml.Unmarshal(content, &envelope); err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		if len(envelope.Discovered) != 1 {
			t.Errorf("Expected 1 discovered device, got %d", len(envelope.Discovered))
		}
	})

	// Test with include_discovered = false
	t.Run("without discovered devices", func(t *testing.T) {
		config := sync.ExportConfig{
			Format: "yaml",
			Config: map[string]interface{}{
				"output_path":        tmpDir,
				"include_discovered": false,
			},
		}

		result, err := p.Export(context.Background(), data, config)
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		content, err := os.ReadFile(result.OutputPath)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		var envelope yamlEnvelope
		if err := yaml.Unmarshal(content, &envelope); err != nil {
			t.Fatalf("Failed to unmarshal YAML: %v", err)
		}

		if len(envelope.Discovered) != 0 {
			t.Errorf("Expected 0 discovered devices, got %d", len(envelope.Discovered))
		}
	})
}

func TestPlugin_Preview(t *testing.T) {
	p := NewPlugin().(*Plugin)
	_ = p.Initialize(logging.GetDefault())

	data := &sync.ExportData{
		Devices: []sync.DeviceData{
			{ID: 1, Name: "Device1"},
			{ID: 2, Name: "Device2"},
		},
		Templates: []sync.TemplateData{
			{ID: 1, Name: "Template1"},
		},
		Metadata: sync.ExportMetadata{
			ExportID: "test",
		},
	}

	config := sync.ExportConfig{
		Format: "yaml",
	}

	result, err := p.Preview(context.Background(), data, config)
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected preview to succeed")
	}
	if result.RecordCount != 3 {
		t.Errorf("Expected record count 3, got %d", result.RecordCount)
	}
	if result.EstimatedSize == 0 {
		t.Error("Expected non-zero estimated size")
	}
}

func TestPlugin_Import_NotImplemented(t *testing.T) {
	p := NewPlugin().(*Plugin)
	_ = p.Initialize(logging.GetDefault())

	source := sync.ImportSource{
		Type: "file",
		Path: "/tmp/test.yaml",
	}

	config := sync.ImportConfig{
		Format: "yaml",
	}

	_, err := p.Import(context.Background(), source, config)
	if err == nil {
		t.Error("Expected Import to return not implemented error")
	}
}

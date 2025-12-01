package yamlexport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"gopkg.in/yaml.v3"
)

func TestPlugin_Info(t *testing.T) {
	p := NewPlugin()
	info := p.Info()
	if info.Name != "yaml" {
		t.Errorf("Expected name 'yaml', got '%s'", info.Name)
	}
	if info.Category != sync.CategoryCustom {
		t.Errorf("Expected category %v, got %v", sync.CategoryCustom, info.Category)
	}
	if info.Version == "" {
		t.Error("Expected non-empty version")
	}
	found := false
	for _, f := range info.SupportedFormats {
		if f == "yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'yaml' in SupportedFormats")
	}
}

func TestPlugin_Export_Success(t *testing.T) {
	p := NewPlugin()
	if err := p.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	data := &sync.ExportData{
		Devices:   []sync.DeviceData{{ID: 1, Name: "Test Device", Type: "shelly1"}},
		Templates: []sync.TemplateData{{ID: 1, Name: "Test Template"}},
	}
	tmpDir := t.TempDir()
	cfg := sync.ExportConfig{Format: "yaml", Config: map[string]interface{}{"output_path": tmpDir}}

	result, err := p.Export(context.Background(), data, cfg)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("Expected successful result, got: %#v", result)
	}
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Fatalf("Output file not created: %s", result.OutputPath)
	}

	content, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var envelope map[string]interface{}
	if err := yaml.Unmarshal(content, &envelope); err != nil {
		t.Fatalf("Output is not valid YAML: %v", err)
	}
	if _, ok := envelope["devices"]; !ok {
		t.Error("Expected 'devices' field in YAML output")
	}
	if _, ok := envelope["templates"]; !ok {
		t.Error("Expected 'templates' field in YAML output")
	}
}

func TestPlugin_Export_Compression(t *testing.T) {
	tests := []struct {
		name        string
		compression bool
		algo        string
		expectExt   string
	}{
		{"gzip compression", true, "gzip", ".yaml.gz"},
		{"zip compression", true, "zip", ".yaml.zip"},
		{"no compression", false, "gzip", ".yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlugin()
			_ = p.Initialize(logging.GetDefault())

			data := &sync.ExportData{Devices: []sync.DeviceData{{ID: 1}}}
			tmpDir := t.TempDir()
			cfg := sync.ExportConfig{
				Format: "yaml",
				Config: map[string]interface{}{
					"output_path":      tmpDir,
					"compression":      tt.compression,
					"compression_algo": tt.algo,
				},
			}

			result, err := p.Export(context.Background(), data, cfg)
			if err != nil {
				t.Fatalf("Export failed: %v", err)
			}
			if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
				t.Fatalf("Output file not created: %s", result.OutputPath)
			}

			if !strings.HasSuffix(result.OutputPath, tt.expectExt) {
				t.Errorf("Expected extension %s, got %s", tt.expectExt, filepath.Ext(result.OutputPath))
			}
		})
	}
}

func TestPlugin_Export_InvalidPath(t *testing.T) {
	p := NewPlugin()
	_ = p.Initialize(logging.GetDefault())

	data := &sync.ExportData{Devices: []sync.DeviceData{{ID: 1}}}
	cfg := sync.ExportConfig{
		Format: "yaml",
		Config: map[string]interface{}{
			"output_path": "/root/invalid/path",
		},
	}

	if _, err := p.Export(context.Background(), data, cfg); err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestPlugin_Preview_And_Import(t *testing.T) {
	p := NewPlugin()
	_ = p.Initialize(logging.GetDefault())

	data := &sync.ExportData{
		Devices:   []sync.DeviceData{{ID: 1}, {ID: 2}},
		Templates: []sync.TemplateData{{ID: 1}},
		DiscoveredDevices: []sync.DiscoveredDeviceData{{
			MAC:        "00:11:22:33:44:55",
			SSID:       "TestWiFi",
			Model:      "SHSW-1",
			Generation: 1,
			IP:         "192.0.2.11",
			Signal:     -40,
			AgentID:    "agent-2",
			Discovered: time.Now(),
		}},
	}
	prev, err := p.Preview(context.Background(), data, sync.ExportConfig{Format: "yaml"})
	if err != nil {
		t.Fatalf("Preview failed: %v", err)
	}
	if !prev.Success || prev.RecordCount <= 0 {
		t.Error("Preview should be successful with positive record count")
	}

	if _, err := p.Import(context.Background(), sync.ImportSource{}, sync.ImportConfig{}); err == nil {
		t.Error("Expected error for unimplemented import")
	}
}

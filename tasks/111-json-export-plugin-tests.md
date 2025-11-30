# JSON Export Plugin Tests

**Priority**: CRITICAL - Blocks Commit
**Status**: not-started
**Effort**: 45 minutes

## Context

The JSON export plugin (`internal/plugins/sync/jsonexport/json.go`) lacks test coverage. Tests are required to ensure the plugin works correctly before committing the export/import consolidation changes.

## Success Criteria

- [ ] 4+ passing tests
- [ ] >60% coverage of json.go
- [ ] No race conditions (`go test -race`)

## Implementation

Create `internal/plugins/sync/jsonexport/json_test.go`:

```go
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

func TestPlugin_Metadata(t *testing.T) {
    p := NewPlugin()

    if p.Name() != "json" {
        t.Errorf("Expected name 'json', got '%s'", p.Name())
    }

    if p.Type() != sync.PluginTypeExport {
        t.Errorf("Expected type PluginTypeExport, got %v", p.Type())
    }

    if p.Version() == "" {
        t.Error("Expected non-empty version")
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
            {ID: "test-device-1", Name: "Test Device", Type: "shelly1"},
        },
        Templates: []sync.TemplateData{
            {ID: "test-template", Name: "Test Template"},
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
        name           string
        compressionAlgo string
        expectExt      string
    }{
        {"gzip compression", "gzip", ".json.gz"},
        {"zip compression", "zip", ".json.zip"},
        {"no compression", "none", ".json"},
        {"default to none", "", ".json"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := NewPlugin()
            p.Initialize(logging.GetDefault())

            data := &sync.ExportData{
                Devices: []sync.DeviceData{{ID: "test"}},
            }

            tmpDir := t.TempDir()
            config := sync.ExportConfig{
                Format: "json",
                Config: map[string]interface{}{
                    "output_path":      tmpDir,
                    "compression_algo": tt.compressionAlgo,
                },
            }

            result, err := p.Export(context.Background(), data, config)
            if err != nil {
                t.Fatalf("Export failed: %v", err)
            }

            // Verify file extension
            ext := filepath.Ext(result.OutputPath)
            if tt.compressionAlgo == "zip" {
                ext = filepath.Ext(result.OutputPath[:len(result.OutputPath)-len(ext)]) + ext
            }
            if ext != tt.expectExt {
                t.Errorf("Expected extension %s, got %s", tt.expectExt, ext)
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
    p.Initialize(logging.GetDefault())

    data := &sync.ExportData{
        Devices: []sync.DeviceData{{ID: "test"}},
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
```

## Validation

```bash
# Run tests
go test -v ./internal/plugins/sync/jsonexport/

# Check coverage
go test -cover ./internal/plugins/sync/jsonexport/
# Target: >60% coverage

# Run with race detector
go test -race ./internal/plugins/sync/jsonexport/
```

## Dependencies

- Task 101 (formatting) - completed

## References

- `internal/plugins/sync/backup/backup_test.go` for patterns

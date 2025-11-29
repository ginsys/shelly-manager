package jsonexport

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// Plugin implements a simple JSON content export (not a DB snapshot)
type Plugin struct {
	logger *logging.Logger
}

func NewPlugin() sync.SyncPlugin { return &Plugin{} }

func (p *Plugin) Info() sync.PluginInfo {
	return sync.PluginInfo{
		Name:        "json",
		Version:     "1.0.0",
		Description: "Export system data (devices, templates, discovered) to a single JSON file",
		Author:      "Shelly Manager Team",
		License:     "MIT",
		SupportedFormats: []string{
			"json",
		},
		Tags:     []string{"content", "json", "export"},
		Category: sync.CategoryCustom,
	}
}

func (p *Plugin) ConfigSchema() sync.ConfigSchema {
	return sync.ConfigSchema{
		Version: "1.0",
		Properties: map[string]sync.PropertySchema{
			"output_path":        {Type: "string", Description: "Directory for export files", Default: "./data/exports"},
			"pretty":             {Type: "boolean", Description: "Pretty-print JSON", Default: true},
			"include_discovered": {Type: "boolean", Description: "Include discovered devices", Default: true},
			"compression":        {Type: "boolean", Description: "Enable compression", Default: false},
			"compression_algo":   {Type: "string", Description: "Compression algorithm (gzip|zip)", Default: "gzip", Enum: []interface{}{"gzip", "zip"}},
		},
		Required: []string{},
	}
}

func (p *Plugin) ValidateConfig(config map[string]interface{}) error {
	if v, ok := config["output_path"].(string); ok && v != "" {
		if err := os.MkdirAll(v, 0755); err != nil {
			return fmt.Errorf("invalid output_path: %w", err)
		}
	}
	return nil
}

// jsonEnvelope defines a simple top-level structure for the JSON export
type jsonEnvelope struct {
	Metadata   sync.ExportMetadata         `json:"metadata"`
	Devices    []sync.DeviceData           `json:"devices"`
	Templates  []sync.TemplateData         `json:"templates"`
	Discovered []sync.DiscoveredDeviceData `json:"discovered_devices,omitempty"`
	CreatedAt  time.Time                   `json:"created_at"`
	Version    string                      `json:"version"`
}

func (p *Plugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	start := time.Now()
	outputPath, _ := config.Config["output_path"].(string)
	if outputPath == "" {
		outputPath = "./data/exports"
	}
	pretty, _ := config.Config["pretty"].(bool)
	includeDiscovered, _ := config.Config["include_discovered"].(bool)
	compression, _ := config.Config["compression"].(bool)
	algo := "gzip"
	if v, ok := config.Config["compression_algo"].(string); ok && v != "" {
		algo = v
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	exportID := uuid.New().String()[:8]
	ts := time.Now().Format("20060102-150405")
	baseName := fmt.Sprintf("shelly-export-%s-%s.json", ts, exportID)
	path := filepath.Join(outputPath, baseName)

	env := jsonEnvelope{
		Metadata:  data.Metadata,
		Devices:   data.Devices,
		Templates: data.Templates,
		CreatedAt: start,
		Version:   "1.0",
	}
	if includeDiscovered {
		env.Discovered = data.DiscoveredDevices
	}

	// Produce JSON bytes
	var buf []byte
	var err error
	if pretty {
		buf, err = json.MarshalIndent(&env, "", "  ")
	} else {
		buf, err = json.Marshal(&env)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}

	// Write with optional compression
	if compression {
		switch algo {
		case "zip":
			zipPath := filepath.Join(outputPath, fmt.Sprintf("shelly-export-%s-%s.json.zip", ts, exportID))
			if err := writeZipSingle(zipPath, baseName, buf); err != nil {
				return nil, err
			}
			path = zipPath
		default: // gzip
			gzPath := filepath.Join(outputPath, fmt.Sprintf("shelly-export-%s-%s.json.gz", ts, exportID))
			if err := writeGzip(gzPath, buf); err != nil {
				return nil, err
			}
			path = gzPath
		}
	} else {
		if err := os.WriteFile(path, buf, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	}

	fi, _ := os.Stat(path)
	sum, _ := fileSHA256(path)

	if p.logger != nil {
		p.logger.Info("JSON export completed", "path", path, "size", func() int64 {
			if fi != nil {
				return fi.Size()
			}
			return 0
		}(), "devices", len(env.Devices))
	}

	return &sync.ExportResult{
		Success:     true,
		OutputPath:  path,
		RecordCount: len(env.Devices),
		FileSize: func() int64 {
			if fi != nil {
				return fi.Size()
			}
			return 0
		}(),
		Checksum: sum,
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"export_id": data.Metadata.ExportID,
			"format":    "json",
		},
	}, nil
}

func (p *Plugin) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	total := len(data.Devices) + len(data.Templates) + len(data.DiscoveredDevices)
	// rough size
	size := int64(total) * 800
	return &sync.PreviewResult{Success: true, RecordCount: total, EstimatedSize: size}, nil
}

func (p *Plugin) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	return nil, fmt.Errorf("json import is not implemented")
}

func (p *Plugin) Capabilities() sync.PluginCapabilities {
	return sync.PluginCapabilities{SupportsScheduling: true, SupportedOutputs: []string{"file"}, ConcurrencyLevel: 1}
}

func (p *Plugin) Initialize(logger *logging.Logger) error { p.logger = logger; return nil }
func (p *Plugin) Cleanup() error                          { return nil }

// helpers
func writeGzip(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	if _, err := gz.Write(data); err != nil {
		_ = gz.Close()
		return fmt.Errorf("failed to write gzip: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip: %w", err)
	}
	return f.Sync()
}

func writeZipSingle(path string, entryName string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: entryName, Method: zip.Deflate}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		_ = zw.Close()
		return fmt.Errorf("failed to create zip entry: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		_ = zw.Close()
		return fmt.Errorf("failed to write zip entry: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close zip: %w", err)
	}
	return f.Sync()
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

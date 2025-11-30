package yamlexport

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/security"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// Plugin implements a simple YAML content export (not a DB snapshot)
type Plugin struct {
	logger  *logging.Logger
	baseDir string // Base directory for path validation
}

func NewPlugin() sync.SyncPlugin { return &Plugin{} }

func (p *Plugin) Info() sync.PluginInfo {
	return sync.PluginInfo{
		Name:        "yaml",
		Version:     "1.0.0",
		Description: "Export system data (devices, templates, discovered) to a single YAML file",
		Author:      "Shelly Manager Team",
		License:     "MIT",
		SupportedFormats: []string{
			"yaml",
		},
		Tags:     []string{"content", "yaml", "export"},
		Category: sync.CategoryCustom,
	}
}

func (p *Plugin) ConfigSchema() sync.ConfigSchema {
	return sync.ConfigSchema{
		Version: "1.0",
		Properties: map[string]sync.PropertySchema{
			"output_path":        {Type: "string", Description: "Directory for export files", Default: "./data/exports"},
			"include_discovered": {Type: "boolean", Description: "Include discovered devices", Default: true},
			"compression":        {Type: "boolean", Description: "Enable compression", Default: false},
			"compression_algo":   {Type: "string", Description: "Compression algorithm (gzip|zip)", Default: "gzip", Enum: []interface{}{"gzip", "zip"}},
		},
		Required: []string{},
	}
}

func (p *Plugin) ValidateConfig(config map[string]interface{}) error {
	if v, ok := config["output_path"].(string); ok && v != "" {
		// Validate path is within allowed base directory
		if p.baseDir != "" {
			if _, err := security.ValidatePath(p.baseDir, v); err != nil {
				return fmt.Errorf("invalid output_path: %w", err)
			}
		}
		if err := os.MkdirAll(v, 0755); err != nil {
			return fmt.Errorf("invalid output_path: %w", err)
		}
	}
	return nil
}

// SetBaseDir sets the base directory for path validation
func (p *Plugin) SetBaseDir(baseDir string) {
	p.baseDir = baseDir
}

// yamlEnvelope mirrors the JSON exporter structure
type yamlEnvelope struct {
	Metadata   sync.ExportMetadata         `yaml:"metadata"`
	Devices    []sync.DeviceData           `yaml:"devices"`
	Templates  []sync.TemplateData         `yaml:"templates"`
	Discovered []sync.DiscoveredDeviceData `yaml:"discovered_devices,omitempty"`
	CreatedAt  time.Time                   `yaml:"created_at"`
	Version    string                      `yaml:"version"`
}

func (p *Plugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	start := time.Now()
	outputPath, _ := config.Config["output_path"].(string)
	if outputPath == "" {
		outputPath = "./data/exports"
	}
	includeDiscovered, _ := config.Config["include_discovered"].(bool)
	compression, _ := config.Config["compression"].(bool)
	algo := "gzip"
	if v, ok := config.Config["compression_algo"].(string); ok && v != "" {
		algo = v
	}

	// Validate output path against base directory to prevent path traversal
	if p.baseDir != "" {
		validatedPath, err := security.ValidatePath(p.baseDir, outputPath)
		if err != nil {
			return nil, fmt.Errorf("path validation failed: %w", err)
		}
		outputPath = validatedPath
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	exportID := uuid.New().String()[:8]
	ts := time.Now().Format("20060102-150405")
	// Sanitize the filename components to prevent injection
	baseName := fmt.Sprintf("shelly-export-%s-%s.yaml", security.SanitizeFilename(ts), security.SanitizeFilename(exportID))
	path := filepath.Join(outputPath, baseName)

	env := yamlEnvelope{
		Metadata:  data.Metadata,
		Devices:   data.Devices,
		Templates: data.Templates,
		CreatedAt: start,
		Version:   "1.0",
	}
	if includeDiscovered {
		env.Discovered = data.DiscoveredDevices
	}

	b, err := yaml.Marshal(&env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal yaml: %w", err)
	}
	if compression {
		switch algo {
		case "zip":
			zipPath := filepath.Join(outputPath, fmt.Sprintf("shelly-export-%s-%s.yaml.zip", ts, exportID))
			if err := writeZipSingle(zipPath, baseName, b); err != nil {
				return nil, err
			}
			path = zipPath
		default:
			gzPath := filepath.Join(outputPath, fmt.Sprintf("shelly-export-%s-%s.yaml.gz", ts, exportID))
			if err := writeGzip(gzPath, b); err != nil {
				return nil, err
			}
			path = gzPath
		}
	} else {
		if err := os.WriteFile(path, b, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	}

	fi, _ := os.Stat(path)
	sum, _ := fileSHA256(path)

	if p.logger != nil {
		p.logger.Info("YAML export completed", "path", path, "size", func() int64 {
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
			"format":    "yaml",
		},
	}, nil
}

func (p *Plugin) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	total := len(data.Devices) + len(data.Templates) + len(data.DiscoveredDevices)
	size := int64(total) * 900
	return &sync.PreviewResult{Success: true, RecordCount: total, EstimatedSize: size}, nil
}

func (p *Plugin) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	return nil, fmt.Errorf("yaml import is not implemented")
}

func (p *Plugin) Capabilities() sync.PluginCapabilities {
	return sync.PluginCapabilities{SupportsScheduling: true, SupportedOutputs: []string{"file"}, ConcurrencyLevel: 1}
}

func (p *Plugin) Initialize(logger *logging.Logger) error { p.logger = logger; return nil }
func (p *Plugin) Cleanup() error                          { return nil }

// shared helpers with json plugin style
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

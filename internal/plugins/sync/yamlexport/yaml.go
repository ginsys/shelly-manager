package yamlexport

import (
	"context"
	"fmt"
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
			"output_path":        {Type: "string", Description: "Directory under the export root (empty is the root itself)", Default: ""},
			"include_discovered": {Type: "boolean", Description: "Include discovered devices", Default: true},
			"compression":        {Type: "boolean", Description: "Enable compression", Default: false},
			"compression_algo":   {Type: "string", Description: "Compression algorithm (gzip|zip)", Default: "gzip", Enum: []interface{}{"gzip", "zip"}},
		},
		Required: []string{},
	}
}

func (p *Plugin) ValidateConfig(config map[string]interface{}) error {
	if v, ok := config["output_path"].(string); ok && v != "" {
		if _, err := sync.ResolveUnderRoot(p.baseDir, v); err != nil {
			return fmt.Errorf("invalid output_path: %w", err)
		}
	}
	return nil
}

// SetBaseDir sets the export root; empty means sync.DefaultExportRoot
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

	// The output directory lies inside the export root: an empty path is the root itself
	outputPath, err := sync.ResolveUnderRoot(p.baseDir, outputPath)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// From here every write goes through the opened export root, so a link swapped in after the check
	// above cannot redirect it.
	root, rel, err := sync.OpenOutputRoot(p.baseDir, outputPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return p.exportInto(root, rel, outputPath, data, config, start)
}

// exportInto writes the export below rel in the opened root. outputPath is the resolved absolute output
// directory, for the result and the log only; no write uses it.
func (p *Plugin) exportInto(root *os.Root, rel, outputPath string, data *sync.ExportData, config sync.ExportConfig, start time.Time) (*sync.ExportResult, error) {
	includeDiscovered, _ := config.Config["include_discovered"].(bool)
	compression, _ := config.Config["compression"].(bool)
	algo := "gzip"
	if v, ok := config.Config["compression_algo"].(string); ok && v != "" {
		algo = v
	}

	if err := root.MkdirAll(rel, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	exportID := uuid.New().String()[:8]
	ts := time.Now().Format("20060102-150405")
	// Sanitize the filename components to prevent injection
	baseName := fmt.Sprintf("shelly-export-%s-%s.yaml", security.SanitizeFilename(ts), security.SanitizeFilename(exportID))
	name := baseName // the file's name in the output directory

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
			name = fmt.Sprintf("shelly-export-%s-%s.yaml.zip", ts, exportID)
			if err := sync.WriteZipSingle(root, filepath.Join(rel, name), baseName, b); err != nil {
				return nil, err
			}
		default:
			name = fmt.Sprintf("shelly-export-%s-%s.yaml.gz", ts, exportID)
			if err := sync.WriteGzip(root, filepath.Join(rel, name), b); err != nil {
				return nil, err
			}
		}
	} else {
		if err := root.WriteFile(filepath.Join(rel, name), b, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	}

	path := filepath.Join(outputPath, name)
	fi, _ := root.Stat(filepath.Join(rel, name))
	sum, _ := sync.FileSHA256(root, filepath.Join(rel, name))

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
	return sync.PluginCapabilities{
		SupportedOutputs: []string{"file"},
		MaxDataSize:      100 * 1024 * 1024,
		ConcurrencyLevel: 1,
	}
}

func (p *Plugin) Initialize(logger *logging.Logger) error { p.logger = logger; return nil }
func (p *Plugin) Cleanup() error                          { return nil }

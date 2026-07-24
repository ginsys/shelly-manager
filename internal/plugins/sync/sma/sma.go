package sma

import (
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/security"
	"github.com/ginsys/shelly-manager/internal/sync"
)

const (
	FormatVersion           = "2026.1"
	defaultNormalizedLimit  = int64(100 * 1024 * 1024)
	defaultCompressionLevel = 6
)

// SMAPlugin implements the registry-backed SMA export/import format.
type SMAPlugin struct {
	logger          *logging.Logger
	baseDir         string
	normalizedLimit int64
	files           fileOperations
}

func NewPlugin() sync.SyncPlugin {
	return &SMAPlugin{
		normalizedLimit: defaultNormalizedLimit,
		files:           osFileOperations{},
	}
}

// SMAArchive is the closed 2026.1 wire model. Optional nested objects use
// pointers so they are omitted instead of being serialized as null.
type SMAArchive struct {
	FormatVersion  string                      `json:"format_version"`
	Metadata       SMAMetadata                 `json:"metadata"`
	Devices        []sync.DeviceData           `json:"devices"`
	Templates      []sync.TemplateData         `json:"templates"`
	Discovered     []sync.DiscoveredDeviceData `json:"discovered_devices"`
	Network        NetworkSettings             `json:"network_settings"`
	PluginConfigs  []PluginConfiguration       `json:"plugin_configurations"`
	SystemSettings SystemSettings              `json:"system_settings"`
}

type SMAMetadata struct {
	ExportID   string        `json:"export_id"`
	CreatedAt  time.Time     `json:"created_at"`
	CreatedBy  string        `json:"created_by"`
	ExportType string        `json:"export_type"`
	SystemInfo SystemInfo    `json:"system_info"`
	Integrity  IntegrityInfo `json:"integrity"`
}

type SystemInfo struct {
	Version          string  `json:"version"`
	DatabaseType     string  `json:"database_type"`
	Hostname         string  `json:"hostname"`
	TotalSizeBytes   int64   `json:"total_size_bytes"`
	CompressionRatio float64 `json:"compression_ratio"`
}

type IntegrityInfo struct {
	Checksum    string `json:"checksum"`
	RecordCount int    `json:"record_count"`
	FileCount   int    `json:"file_count"`
}

type NetworkSettings struct {
	WiFiNetworks []WiFiNetwork `json:"wifi_networks"`
	NTPServers   []string      `json:"ntp_servers"`
	MQTTConfig   *MQTTConfig   `json:"mqtt_config,omitempty"`
}

type WiFiNetwork struct {
	SSID     string `json:"ssid"`
	Security string `json:"security"`
	Priority int    `json:"priority"`
}

type MQTTConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Retain   bool   `json:"retain"`
	QoS      int    `json:"qos"`
}

type PluginConfiguration struct {
	PluginName string                 `json:"plugin_name"`
	Version    string                 `json:"version"`
	Config     map[string]interface{} `json:"config"`
	Enabled    bool                   `json:"enabled"`
}

type SystemSettings struct {
	LogLevel         string                 `json:"log_level"`
	APISettings      map[string]interface{} `json:"api_settings"`
	DatabaseSettings map[string]interface{} `json:"database_settings"`
}

func (s *SMAPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{
		Name:             "sma",
		Version:          FormatVersion,
		Description:      "Shelly Management Archive with gzip compression and JCS integrity",
		Author:           "Shelly Manager Team",
		License:          "MIT",
		SupportedFormats: []string{"sma"},
		Tags:             []string{"archive", "complete", "compressed", "structured"},
		Category:         sync.CategoryBackup,
	}
}

func (s *SMAPlugin) ConfigSchema() sync.ConfigSchema {
	min, max := 1.0, 9.0
	return sync.ConfigSchema{
		Version: FormatVersion,
		Properties: map[string]sync.PropertySchema{
			"output_path": {
				Type: "string", Description: "Directory path for SMA files",
				Default: "/var/backups/shelly-manager",
			},
			"compression_level": {
				Type: "number", Description: "Gzip compression level (1-9)",
				Default: float64(defaultCompressionLevel), Minimum: &min, Maximum: &max,
			},
			"include_discovered": {
				Type: "boolean", Description: "Include discovered devices", Default: true,
			},
			"exclude_sensitive": {
				Type: "boolean", Description: "Redact sensitive map values", Default: true,
			},
		},
		Required: []string{},
	}
}

// ValidateConfig is deliberately side-effect free. Directory creation belongs
// to Export, after all engine and plugin validation has succeeded.
func (s *SMAPlugin) ValidateConfig(config map[string]interface{}) error {
	if value, ok := config["output_path"]; ok {
		path, ok := value.(string)
		if !ok {
			return fmt.Errorf("output_path must be a string")
		}
		if s.baseDir != "" {
			if _, err := security.ValidatePath(s.baseDir, path); err != nil {
				return fmt.Errorf("invalid output_path: %w", err)
			}
		}
	}
	if value, ok := config["compression_level"]; ok {
		level, ok := value.(float64)
		if !ok || level != float64(int(level)) {
			return fmt.Errorf("compression_level must be an integer")
		}
		if level < 1 || level > 9 {
			return fmt.Errorf("compression_level must be between 1 and 9")
		}
	}
	for _, name := range []string{"include_discovered", "exclude_sensitive"} {
		if _, err := effectiveBool(config, name, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *SMAPlugin) SetBaseDir(baseDir string) {
	s.baseDir = baseDir
}

func (s *SMAPlugin) Export(_ context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	start := time.Now()
	tree, prepared, err := s.prepareArchive(data, config)
	if err != nil {
		return nil, err
	}
	canonical, checksum, err := finalizeArchiveTree(tree)
	if err != nil {
		return nil, fmt.Errorf("materialize SMA archive: %w", err)
	}

	outputPath, _ := config.Config["output_path"].(string)
	if outputPath == "" {
		outputPath = "/tmp/shelly-sma"
	}
	if s.baseDir != "" {
		outputPath, err = security.ValidatePath(s.baseDir, outputPath)
		if err != nil {
			return nil, fmt.Errorf("path validation failed: %w", err)
		}
	}
	if mkdirErr := os.MkdirAll(outputPath, 0o755); mkdirErr != nil {
		return nil, fmt.Errorf("create output directory: %w", mkdirErr)
	}

	level := defaultCompressionLevel
	if configured, ok := config.Config["compression_level"].(float64); ok && configured != 0 {
		level = int(configured)
	}
	filename := fmt.Sprintf("shelly-archive-%s-%s.sma",
		security.SanitizeFilename(start.UTC().Format("20060102-150405")),
		security.SanitizeFilename(uuid.NewString()[:8]))
	finalPath := filepath.Join(outputPath, filename)
	fileSize, err := s.publishAtomic(finalPath, canonical, level)
	if err != nil {
		return nil, err
	}

	return &sync.ExportResult{
		Success:     true,
		ExportID:    prepared.exportID,
		PluginName:  "sma",
		Format:      "sma",
		OutputPath:  finalPath,
		RecordCount: prepared.recordCount,
		FileSize:    fileSize,
		Checksum:    checksum,
		Duration:    time.Since(start),
		Metadata: map[string]interface{}{
			"format_version":    FormatVersion,
			"compression_level": level,
			"uncompressed_size": len(canonical),
		},
		CreatedAt: time.Now(),
	}, nil
}

func (s *SMAPlugin) Preview(_ context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	tree, prepared, err := s.prepareArchive(data, config)
	if err != nil {
		return nil, err
	}
	canonical, _, err := finalizeArchiveTree(tree)
	if err != nil {
		return nil, fmt.Errorf("materialize SMA preview: %w", err)
	}
	level := defaultCompressionLevel
	if configured, ok := config.Config["compression_level"].(float64); ok && configured != 0 {
		level = int(configured)
	}
	var compressed strings.Builder
	// A counting buffer would be marginally cheaper, but retaining this tiny
	// preview compression keeps the estimate exact and deterministic.
	writer, err := gzip.NewWriterLevel(&compressed, level)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(canonical); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	sample := fmt.Sprintf("SMA %s preview\nDevices: %d\nTemplates: %d\nDiscovered Devices: %d",
		FormatVersion, prepared.deviceCount, prepared.templateCount, prepared.discoveredCount)
	return &sync.PreviewResult{
		Success:       true,
		SampleData:    []byte(sample),
		RecordCount:   prepared.recordCount,
		EstimatedSize: int64(compressed.Len()),
	}, nil
}

func (s *SMAPlugin) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	switch source.Type {
	case "file":
		return s.ImportFromFile(ctx, source.Path, config)
	case "data":
		return s.ImportFromData(ctx, source.Data, config)
	default:
		return nil, fmt.Errorf("%w: unsupported SMA source type %q", sync.ErrInvalidImportData, source.Type)
	}
}

func (s *SMAPlugin) Capabilities() sync.PluginCapabilities {
	return sync.PluginCapabilities{
		SupportedOutputs: []string{"file"},
		MaxDataSize:      defaultNormalizedLimit,
		ConcurrencyLevel: 1,
	}
}

func (s *SMAPlugin) Initialize(logger *logging.Logger) error {
	s.logger = logger
	if s.normalizedLimit <= 0 {
		s.normalizedLimit = defaultNormalizedLimit
	}
	if s.files == nil {
		s.files = osFileOperations{}
	}
	return nil
}

func (s *SMAPlugin) Cleanup() error { return nil }

func (s *SMAPlugin) isSensitiveField(name string) bool {
	lower := strings.ToLower(name)
	for _, fragment := range []string{
		"password", "passwd", "pwd", "secret", "key", "token",
		"api_key", "apikey", "auth", "credential", "private",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

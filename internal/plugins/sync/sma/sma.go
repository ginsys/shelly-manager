package sma

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
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
	SMAVersion    = "1.0"
	FormatVersion = "2024.1"
)

// SMAPlugin implements the SyncPlugin interface for SMA format operations
type SMAPlugin struct {
	logger  *logging.Logger
	baseDir string // Base directory for path validation
}

// NewPlugin creates a new SMA plugin (for registry)
func NewPlugin() sync.SyncPlugin {
	return &SMAPlugin{}
}

// SMAArchive represents the complete structure of an SMA file
type SMAArchive struct {
	SMAVersion      string                      `json:"sma_version"`
	FormatVersion   string                      `json:"format_version"`
	Metadata        SMAMetadata                 `json:"metadata"`
	Devices         []sync.DeviceData           `json:"devices"`
	Templates       []sync.TemplateData         `json:"templates"`
	Discovered      []sync.DiscoveredDeviceData `json:"discovered_devices,omitempty"`
	NetworkSettings *NetworkSettings            `json:"network_settings,omitempty"`
	PluginConfigs   []PluginConfiguration       `json:"plugin_configurations,omitempty"`
	SystemSettings  *SystemSettings             `json:"system_settings,omitempty"`
}

// SMAMetadata contains metadata about the SMA archive
type SMAMetadata struct {
	ExportID   string        `json:"export_id"`
	CreatedAt  time.Time     `json:"created_at"`
	CreatedBy  string        `json:"created_by"`
	ExportType string        `json:"export_type"` // "manual", "scheduled", "api"
	SystemInfo SystemInfo    `json:"system_info"`
	Integrity  IntegrityInfo `json:"integrity"`
}

// SystemInfo contains information about the source system
type SystemInfo struct {
	Version          string  `json:"version"`
	DatabaseType     string  `json:"database_type"`
	Hostname         string  `json:"hostname"`
	TotalSizeBytes   int64   `json:"total_size_bytes"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// IntegrityInfo contains integrity verification data
type IntegrityInfo struct {
	Checksum    string `json:"checksum"`
	RecordCount int    `json:"record_count"`
	FileCount   int    `json:"file_count"`
}

// NetworkSettings contains network configuration
type NetworkSettings struct {
	WiFiNetworks []WiFiNetwork `json:"wifi_networks,omitempty"`
	MQTTConfig   *MQTTConfig   `json:"mqtt_config,omitempty"`
	NTPServers   []string      `json:"ntp_servers,omitempty"`
}

// WiFiNetwork represents a WiFi network configuration
type WiFiNetwork struct {
	SSID     string `json:"ssid"`
	Security string `json:"security"`
	Priority int    `json:"priority"`
}

// MQTTConfig represents MQTT configuration
type MQTTConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Retain   bool   `json:"retain"`
	QoS      int    `json:"qos"`
}

// PluginConfiguration represents plugin configuration
type PluginConfiguration struct {
	PluginName string                 `json:"plugin_name"`
	Version    string                 `json:"version"`
	Config     map[string]interface{} `json:"config"`
	Enabled    bool                   `json:"enabled"`
}

// SystemSettings represents system-level settings
type SystemSettings struct {
	LogLevel         string                 `json:"log_level"`
	APISettings      map[string]interface{} `json:"api_settings,omitempty"`
	DatabaseSettings map[string]interface{} `json:"database_settings,omitempty"`
}

// Info returns plugin information
func (s *SMAPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{
		Name:        "sma",
		Version:     "1.0.0",
		Description: "Shelly Management Archive (SMA) format - comprehensive export/import with compression and integrity verification",
		Author:      "Shelly Manager Team",
		License:     "MIT",
		SupportedFormats: []string{
			"sma", // Shelly Management Archive
		},
		Tags:     []string{"archive", "complete", "compressed", "structured"},
		Category: sync.CategoryBackup,
	}
}

// ConfigSchema returns the configuration schema
func (s *SMAPlugin) ConfigSchema() sync.ConfigSchema {
	return sync.ConfigSchema{
		Version: "1.0",
		Properties: map[string]sync.PropertySchema{
			"output_path": {
				Type:        "string",
				Description: "Directory path for SMA files",
				Default:     "/var/backups/shelly-manager",
			},
			"compression_level": {
				Type:        "number",
				Description: "Gzip compression level (1-9, 6 recommended)",
				Default:     6.0,
				Minimum:     &[]float64{1}[0],
				Maximum:     &[]float64{9}[0],
			},
			"include_discovered": {
				Type:        "boolean",
				Description: "Include discovered devices in archive",
				Default:     true,
			},
			"include_network_settings": {
				Type:        "boolean",
				Description: "Include network configuration in archive",
				Default:     false,
			},
			"include_plugin_configs": {
				Type:        "boolean",
				Description: "Include plugin configurations in archive",
				Default:     true,
			},
			"include_system_settings": {
				Type:        "boolean",
				Description: "Include system settings in archive",
				Default:     false,
			},
			"exclude_sensitive": {
				Type:        "boolean",
				Description: "Exclude sensitive data like passwords and API keys",
				Default:     true,
			},
		},
		Required: []string{},
		Examples: []map[string]interface{}{
			{
				"output_path":              "/backup/sma",
				"compression_level":        6,
				"include_discovered":       true,
				"include_network_settings": false,
				"include_plugin_configs":   true,
				"include_system_settings":  false,
				"exclude_sensitive":        true,
			},
		},
	}
}

// ValidateConfig validates the plugin configuration
func (s *SMAPlugin) ValidateConfig(config map[string]interface{}) error {
	if outputPath, exists := config["output_path"]; exists {
		if path, ok := outputPath.(string); ok {
			// Validate path is within allowed base directory
			if s.baseDir != "" {
				if _, err := security.ValidatePath(s.baseDir, path); err != nil {
					return fmt.Errorf("invalid output_path: %w", err)
				}
			}
			// Check if directory exists or can be created
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("invalid output_path: cannot create directory %s: %w", path, err)
			}
		} else {
			return fmt.Errorf("output_path must be a string")
		}
	}

	if compressionLevel, exists := config["compression_level"]; exists {
		if level, ok := compressionLevel.(float64); ok {
			if level < 1 || level > 9 {
				return fmt.Errorf("compression_level must be between 1 and 9, got %f", level)
			}
		} else {
			return fmt.Errorf("compression_level must be a number")
		}
	}

	return nil
}

// SetBaseDir sets the base directory for path validation
func (s *SMAPlugin) SetBaseDir(baseDir string) {
	s.baseDir = baseDir
}

// Export creates an SMA archive from the provided data
func (s *SMAPlugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	startTime := time.Now()

	s.logger.Info("Starting SMA export",
		"format", config.Format,
		"devices", len(data.Devices),
		"templates", len(data.Templates),
		"discovered", len(data.DiscoveredDevices),
	)

	// Parse configuration
	outputPath, _ := config.Config["output_path"].(string)
	if outputPath == "" {
		outputPath = "/tmp/shelly-sma"
	}

	compressionLevel, _ := config.Config["compression_level"].(float64)
	if compressionLevel == 0 {
		compressionLevel = 6
	}

	includeDiscovered, _ := config.Config["include_discovered"].(bool)
	includeNetworkSettings, _ := config.Config["include_network_settings"].(bool)
	includePluginConfigs, _ := config.Config["include_plugin_configs"].(bool)
	includeSystemSettings, _ := config.Config["include_system_settings"].(bool)
	excludeSensitive, _ := config.Config["exclude_sensitive"].(bool)

	// Validate output path against base directory to prevent path traversal
	if s.baseDir != "" {
		validatedPath, err := security.ValidatePath(s.baseDir, outputPath)
		if err != nil {
			return nil, fmt.Errorf("path validation failed: %w", err)
		}
		outputPath = validatedPath
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate unique filename with sanitized components
	exportID := uuid.New().String()[:8]
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("shelly-archive-%s-%s.sma", security.SanitizeFilename(timestamp), security.SanitizeFilename(exportID))
	archivePath := filepath.Join(outputPath, filename)

	// Create SMA archive structure
	archive := SMAArchive{
		SMAVersion:    SMAVersion,
		FormatVersion: FormatVersion,
		Metadata: SMAMetadata{
			ExportID:   data.Metadata.ExportID,
			CreatedAt:  startTime,
			CreatedBy:  data.Metadata.RequestedBy,
			ExportType: data.Metadata.ExportType,
			SystemInfo: SystemInfo{
				Version:      data.Metadata.SystemVersion,
				DatabaseType: data.Metadata.DatabaseType,
				Hostname:     "localhost", // TODO: Get from system
			},
		},
		Devices:   data.Devices,
		Templates: data.Templates,
	}

	// Add optional sections based on configuration
	if includeDiscovered {
		archive.Discovered = data.DiscoveredDevices
	}

	if includeNetworkSettings {
		archive.NetworkSettings = s.extractNetworkSettings()
	}

	if includePluginConfigs {
		archive.PluginConfigs = s.extractPluginConfigurations()
	}

	if includeSystemSettings {
		archive.SystemSettings = s.extractSystemSettings()
	}

	// Apply sensitive data exclusion if requested
	if excludeSensitive {
		s.excludeSensitiveData(&archive)
	}

	// Calculate record counts
	recordCount := len(archive.Devices) + len(archive.Templates) + len(archive.Discovered)

	// Update integrity information first (before calculating checksum)
	archive.Metadata.Integrity = IntegrityInfo{
		Checksum:    "", // Will be calculated after marshaling
		RecordCount: recordCount,
		FileCount:   1,
	}

	// Marshal to JSON (final version)
	jsonData, err := json.MarshalIndent(archive, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal archive to JSON: %w", err)
	}

	// Calculate checksum of final JSON data
	hash := sha256.Sum256(jsonData)
	checksum := fmt.Sprintf("sha256:%x", hash)

	// Update the checksum in the archive metadata (in memory only for result metadata)
	archive.Metadata.Integrity.Checksum = checksum

	// Write compressed file
	file, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.logger.Warn("Failed to close archive file", "error", err)
		}
	}()

	// Create gzip writer with specified compression level
	gzipWriter, err := gzip.NewWriterLevel(file, int(compressionLevel))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			s.logger.Warn("Failed to close gzip writer", "error", err)
		}
	}()

	// Write compressed data
	if _, err := gzipWriter.Write(jsonData); err != nil {
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}

	// Close gzip writer to ensure all data is written
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("failed to close archive file: %w", err)
	}

	// Get final file info
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get archive file info: %w", err)
	}

	compressedSize := fileInfo.Size()
	compressionRatio := float64(compressedSize) / float64(len(jsonData))

	// Update system info with actual sizes
	archive.Metadata.SystemInfo.TotalSizeBytes = compressedSize
	archive.Metadata.SystemInfo.CompressionRatio = compressionRatio

	s.logger.Info("SMA export completed",
		"path", archivePath,
		"uncompressed_size", len(jsonData),
		"compressed_size", compressedSize,
		"compression_ratio", compressionRatio,
		"records", recordCount,
		"duration", time.Since(startTime),
	)

	return &sync.ExportResult{
		Success:     true,
		ExportID:    data.Metadata.ExportID,
		PluginName:  "sma",
		Format:      "sma",
		OutputPath:  archivePath,
		RecordCount: recordCount,
		FileSize:    compressedSize,
		Checksum:    checksum,
		Duration:    time.Since(startTime),
		Metadata: map[string]interface{}{
			"sma_version":        SMAVersion,
			"format_version":     FormatVersion,
			"compression_level":  compressionLevel,
			"compression_ratio":  compressionRatio,
			"uncompressed_size":  len(jsonData),
			"include_discovered": includeDiscovered,
			"exclude_sensitive":  excludeSensitive,
		},
		CreatedAt: time.Now(),
	}, nil
}

// Preview generates a preview of what would be exported
func (s *SMAPlugin) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	includeDiscovered, _ := config.Config["include_discovered"].(bool)
	includeNetworkSettings, _ := config.Config["include_network_settings"].(bool)
	includePluginConfigs, _ := config.Config["include_plugin_configs"].(bool)
	includeSystemSettings, _ := config.Config["include_system_settings"].(bool)
	excludeSensitive, _ := config.Config["exclude_sensitive"].(bool)
	compressionLevel, _ := config.Config["compression_level"].(float64)
	if compressionLevel == 0 {
		compressionLevel = 6
	}

	// Calculate record counts
	deviceCount := len(data.Devices)
	configCount := len(data.Configurations)
	templateCount := len(data.Templates)
	discoveredCount := 0
	if includeDiscovered {
		discoveredCount = len(data.DiscoveredDevices)
	}

	totalRecords := deviceCount + configCount + templateCount + discoveredCount

	// Estimate size (rough calculation)
	avgRecordSize := int64(800) // bytes per record for SMA format
	estimatedUncompressedSize := int64(totalRecords) * avgRecordSize
	estimatedCompressedSize := int64(float64(estimatedUncompressedSize) * 0.35) // ~35% compression ratio

	// Generate preview text
	var sections []string
	sections = append(sections, fmt.Sprintf("- Devices: %d", deviceCount))
	sections = append(sections, fmt.Sprintf("- Configurations: %d", configCount))
	sections = append(sections, fmt.Sprintf("- Templates: %d", templateCount))

	if includeDiscovered {
		sections = append(sections, fmt.Sprintf("- Discovered Devices: %d", discoveredCount))
	}
	if includeNetworkSettings {
		sections = append(sections, "- Network Settings: Yes")
	}
	if includePluginConfigs {
		sections = append(sections, "- Plugin Configurations: Yes")
	}
	if includeSystemSettings {
		sections = append(sections, "- System Settings: Yes")
	}

	sampleData := fmt.Sprintf(`SMA Archive Preview:
%s
- SMA Version: %s
- Format Version: %s
- Compression Level: %.0f
- Exclude Sensitive Data: %v
- Estimated Uncompressed Size: %d bytes
- Estimated Compressed Size: %d bytes
- Estimated Compression Ratio: 35%%`,
		strings.Join(sections, "\n"),
		SMAVersion,
		FormatVersion,
		compressionLevel,
		excludeSensitive,
		estimatedUncompressedSize,
		estimatedCompressedSize)

	return &sync.PreviewResult{
		Success:       true,
		SampleData:    []byte(sampleData),
		RecordCount:   totalRecords,
		EstimatedSize: estimatedCompressedSize,
	}, nil
}

// Import restores data from an SMA archive
func (s *SMAPlugin) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	switch source.Type {
	case "file":
		return s.ImportFromFile(ctx, source.Path, config)
	case "data":
		return s.ImportFromData(ctx, source.Data, config)
	case "url":
		return nil, fmt.Errorf("URL-based SMA import not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported import source type: %s", source.Type)
	}
}

// Capabilities returns plugin capabilities
func (s *SMAPlugin) Capabilities() sync.PluginCapabilities {
	return sync.PluginCapabilities{
		SupportsIncremental:    false, // Full archives only
		SupportsScheduling:     true,
		RequiresAuthentication: false,
		SupportedOutputs:       []string{"file"},
		MaxDataSize:            1024 * 1024 * 1024 * 5, // 5GB
		ConcurrencyLevel:       1,                      // Sequential processing for integrity
	}
}

// Initialize initializes the plugin
func (s *SMAPlugin) Initialize(logger *logging.Logger) error {
	s.logger = logger
	s.logger.Info("Initialized SMA plugin")
	return nil
}

// Cleanup cleans up plugin resources
func (s *SMAPlugin) Cleanup() error {
	s.logger.Info("Cleaning up SMA plugin")
	return nil
}

// Helper methods

// extractNetworkSettings extracts network settings (placeholder implementation)
func (s *SMAPlugin) extractNetworkSettings() *NetworkSettings {
	// This would be implemented based on actual system configuration
	// For now, returning a placeholder
	return &NetworkSettings{
		WiFiNetworks: []WiFiNetwork{},
		MQTTConfig:   nil,
		NTPServers:   []string{"pool.ntp.org"},
	}
}

// extractPluginConfigurations extracts plugin configurations (placeholder implementation)
func (s *SMAPlugin) extractPluginConfigurations() []PluginConfiguration {
	// This would be implemented based on actual plugin registry
	// For now, returning empty slice
	return []PluginConfiguration{}
}

// extractSystemSettings extracts system settings (placeholder implementation)
func (s *SMAPlugin) extractSystemSettings() *SystemSettings {
	// This would be implemented based on actual system configuration
	// For now, returning a placeholder
	return &SystemSettings{
		LogLevel: "info",
		APISettings: map[string]interface{}{
			"rate_limit":   100,
			"cors_enabled": true,
		},
	}
}

// excludeSensitiveData removes sensitive information from the archive
func (s *SMAPlugin) excludeSensitiveData(archive *SMAArchive) {
	// Remove sensitive data from network settings
	if archive.NetworkSettings != nil && archive.NetworkSettings.MQTTConfig != nil {
		archive.NetworkSettings.MQTTConfig.Username = "[REDACTED]"
	}

	// Remove sensitive data from plugin configurations
	for i := range archive.PluginConfigs {
		config := &archive.PluginConfigs[i]
		for key := range config.Config {
			if s.isSensitiveField(key) {
				config.Config[key] = "[REDACTED]"
			}
		}
	}

	// Remove sensitive data from devices (if any settings contain passwords)
	for i := range archive.Devices {
		device := &archive.Devices[i]
		for key := range device.Settings {
			if s.isSensitiveField(key) {
				device.Settings[key] = "[REDACTED]"
			}
		}
	}
}

// isSensitiveField checks if a field contains sensitive data
func (s *SMAPlugin) isSensitiveField(fieldName string) bool {
	sensitiveFields := []string{
		"password", "passwd", "pwd", "secret", "key", "token",
		"api_key", "apikey", "auth", "credential", "private",
	}

	lowerField := strings.ToLower(fieldName)
	for _, sensitive := range sensitiveFields {
		if strings.Contains(lowerField, sensitive) {
			return true
		}
	}
	return false
}

package backup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// DatabaseManagerInterface defines the interface we need from database.Manager
type DatabaseManagerInterface interface {
	GetProvider() provider.DatabaseProvider
	GetDB() interface{}
	Close() error
}

// defaultBackupDir is the backup root when sync.export_base_dir is not set.
const defaultBackupDir = "./data/backups"

// BackupPlugin implements the SyncPlugin interface for database backups
type BackupPlugin struct {
	dbManager     DatabaseManagerInterface
	logger        *logging.Logger
	exportBaseDir string
}

// SetDatabaseManager injects the database manager dependency.
func (b *BackupPlugin) SetDatabaseManager(dbManager DatabaseManagerInterface) {
	b.dbManager = dbManager
}

// SetExportBaseDir receives the engine's export root (sync.export_base_dir). Backups are written
// under it; when it is empty they are written under ./data/backups.
func (b *BackupPlugin) SetExportBaseDir(dir string) {
	b.exportBaseDir = dir
}

// resolveOutputDir returns the absolute directory a backup is written to. An empty output_path is
// the backup root. A relative one joins under a configured root, and is relative to the working
// directory otherwise (the engine has already made it absolute). Anything that is not inside the
// root after cleaning is refused.
func (b *BackupPlugin) resolveOutputDir(outputPath string) (string, error) {
	root := defaultBackupDir
	if b.exportBaseDir != "" {
		root = b.exportBaseDir
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve the backup directory: %w", err)
	}
	dir := absRoot
	if outputPath != "" {
		dir = filepath.Clean(outputPath)
		if b.exportBaseDir != "" && !filepath.IsAbs(dir) {
			dir = filepath.Join(absRoot, dir)
		}
	}
	dir, err = insideRoot(absRoot, dir)
	if err != nil {
		return "", fmt.Errorf("output_path must be inside the backup directory %s", absRoot)
	}
	return dir, nil
}

// insideRoot returns path as an absolute path when it is absRoot or lies under it, and an error
// otherwise. It reads no file system state: a symlink inside the root is trusted host configuration.
func insideRoot(absRoot, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%q is outside %q", path, absRoot)
	}
	return filepath.Join(absRoot, rel), nil
}

// NewPlugin creates a new backup plugin (for registry)
func NewPlugin() sync.SyncPlugin {
	return &BackupPlugin{}
}

// NewBackupExporter creates a new backup exporter (backward compatibility)
func NewBackupExporter(dbManager DatabaseManagerInterface) *BackupPlugin {
	return &BackupPlugin{
		dbManager: dbManager,
	}
}

// Info returns plugin information
func (b *BackupPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{
		Name:        "backup",
		Version:     "1.0.0",
		Description: "Export complete system backup using database abstraction layer",
		Author:      "Shelly Manager Team",
		License:     "MIT",
		SupportedFormats: []string{
			// Single-file provider snapshot. Compression is controlled via config (gzip on/off)
			"sqlite",
		},
		Tags:     []string{"backup", "database", "system"},
		Category: sync.CategoryBackup,
	}
}

// ConfigSchema returns the configuration schema
func (b *BackupPlugin) ConfigSchema() sync.ConfigSchema {
	return sync.ConfigSchema{
		Version: "1.0",
		Properties: map[string]sync.PropertySchema{
			"output_path": {
				Type:        "string",
				Description: "Directory for backup files; must be inside sync.export_base_dir when set, else inside ./data/backups",
				Default:     "./data/backups",
			},
			"compression": {
				Type:        "boolean",
				Description: "Enable gzip compression for backup files",
				Default:     true,
			},
			"compression_algo": {
				Type:        "string",
				Description: "Compression algorithm to use when compression is enabled (gzip|zip)",
				Default:     "gzip",
				Enum:        []interface{}{"gzip", "zip"},
			},
			"include_discovered": {
				Type:        "boolean",
				Description: "Include discovered devices in backup",
				Default:     true,
			},
			"backup_type": {
				Type:        "string",
				Description: "Type of backup to perform",
				Default:     "full",
				Enum:        []interface{}{"full", "incremental", "differential"},
			},
		},
		Required: []string{},
		Examples: []map[string]interface{}{
			{
				"output_path":        "./data/backups",
				"compression":        true,
				"include_discovered": true,
				"backup_type":        "full",
			},
		},
	}
}

// ValidateConfig validates the plugin configuration
func (b *BackupPlugin) ValidateConfig(config map[string]interface{}) error {
	if outputPath, exists := config["output_path"]; exists {
		path, ok := outputPath.(string)
		if !ok {
			return fmt.Errorf("output_path must be a string")
		}
		if _, err := b.resolveOutputDir(path); err != nil {
			return err
		}
	}

	if backupType, exists := config["backup_type"]; exists {
		if bt, ok := backupType.(string); ok {
			validTypes := map[string]bool{
				"full":         true,
				"incremental":  true,
				"differential": true,
			}
			if !validTypes[bt] {
				return fmt.Errorf("invalid backup_type: %s", bt)
			}
		} else {
			return fmt.Errorf("backup_type must be a string")
		}
	}

	return nil
}

// Export performs the backup export
func (b *BackupPlugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	startTime := time.Now()

	if b != nil && b.logger != nil {
		b.logger.Info("Starting backup export",
			"format", config.Format,
			"devices", len(data.Devices),
		)
	}

	// Get backup provider from database manager
	if b == nil || b.dbManager == nil {
		return nil, fmt.Errorf("backup plugin is not initialized with a database manager")
	}
	dbProvider := b.dbManager.GetProvider()
	backupProvider, ok := dbProvider.(provider.BackupProvider)
	if !ok {
		return nil, fmt.Errorf("database provider does not support backup operations")
	}

	// Parse configuration
	requested, _ := config.Config["output_path"].(string)
	outputPath, err := b.resolveOutputDir(requested)
	if err != nil {
		return nil, err
	}

	compression, _ := config.Config["compression"].(bool)
	backupType, _ := config.Config["backup_type"].(string)
	if backupType == "" {
		backupType = "full"
	}

	// Ensure output directory exists
	if err = os.MkdirAll(outputPath, 0755); err != nil {
		if b.logger != nil {
			b.logger.WithFields(map[string]any{"path": outputPath, "error": err}).Error("Failed to create output directory for backup")
		}
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate unique backup filename
	backupID := uuid.New().String()[:8]
	timestamp := time.Now().Format("20060102-150405")

	var filename string
	if compression {
		algo := "gzip"
		if v, ok := config.Config["compression_algo"].(string); ok && v != "" {
			algo = strings.ToLower(v)
		}
		if algo == "zip" {
			filename = fmt.Sprintf("shelly-backup-%s-%s.sqlite.zip", timestamp, backupID)
		} else {
			filename = fmt.Sprintf("shelly-backup-%s-%s.sqlite.gz", timestamp, backupID)
		}
	} else {
		filename = fmt.Sprintf("shelly-backup-%s-%s.sqlite", timestamp, backupID)
	}

	backupPath := filepath.Join(outputPath, filename)

	// Create backup using database provider
	backupConfig := provider.BackupConfig{
		BackupPath:  backupPath,
		BackupType:  provider.BackupType(backupType),
		Compression: compression,
		Options:     make(map[string]string),
	}

	// Add metadata to backup options
	backupConfig.Options["export_id"] = data.Metadata.ExportID
	backupConfig.Options["system_version"] = data.Metadata.SystemVersion
	backupConfig.Options["timestamp"] = data.Timestamp.Format(time.RFC3339)

	backupResult, err := backupProvider.CreateBackup(ctx, backupConfig)
	if err != nil {
		return nil, fmt.Errorf("backup operation failed: %w", err)
	}

	if !backupResult.Success {
		return &sync.ExportResult{
			Success:  false,
			Duration: time.Since(startTime),
			Errors:   []string{backupResult.Error},
		}, fmt.Errorf("backup failed: %s", backupResult.Error)
	}

	// Calculate file size and checksum
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		b.logger.Warn("Failed to get backup file info", "error", err)
	}

	var fileSize int64
	var checksum string
	if fileInfo != nil {
		fileSize = fileInfo.Size()
		if sum, sumErr := b.calculateChecksum(backupPath); sumErr != nil {
			if b.logger != nil {
				b.logger.Warn("Failed to calculate checksum for backup file", "error", sumErr)
			}
		} else {
			checksum = sum
		}
	}

	if b != nil && b.logger != nil {
		b.logger.Info("Backup export completed",
			"path", backupPath,
			"size", fileSize,
			"duration", backupResult.Duration,
			"records", backupResult.RecordCount,
		)
	}

	// Resolve provider name safely
	providerName := "unknown"
	if b.dbManager != nil && b.dbManager.GetProvider() != nil {
		providerName = b.dbManager.GetProvider().Name()
	}
	// Logical records as device count if provider doesn't report
	logicalRecords := len(data.Devices)
	if backupResult.RecordCount > 0 {
		logicalRecords = int(backupResult.RecordCount)
	}
	md := map[string]interface{}{
		"backup_id":   backupResult.BackupID,
		"backup_type": string(backupResult.BackupType),
		"table_count": backupResult.TableCount,
		"provider":    providerName,
		"compressed":  compression,
	}
	if v, ok := config.Config["name"].(string); ok && v != "" {
		md["name"] = v
	}
	if v, ok := config.Config["description"].(string); ok && v != "" {
		md["description"] = v
	}
	var warnings []string
	switch strings.ToLower(config.Format) {
	case "json", "yaml", "zip":
		warnings = append(warnings, fmt.Sprintf("%s format not supported for SQLite; created SQLite DB copy instead", strings.ToUpper(config.Format)))
	}
	return &sync.ExportResult{
		Success:     true,
		OutputPath:  backupPath,
		RecordCount: logicalRecords,
		FileSize:    fileSize,
		Checksum:    checksum,
		Duration:    time.Since(startTime),
		Warnings:    append(backupResult.Warnings, warnings...),
		Metadata:    md,
	}, nil
}

// Preview generates a preview of what would be backed up
func (b *BackupPlugin) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	// Resolve provider name safely
	providerName := "unknown"
	if b != nil && b.dbManager != nil && b.dbManager.GetProvider() != nil {
		providerName = b.dbManager.GetProvider().Name()
	}
	// Get rough size estimates
	deviceCount := len(data.Devices)
	configCount := len(data.Configurations)
	templateCount := len(data.Templates)
	discoveredCount := len(data.DiscoveredDevices)

	totalRecords := deviceCount + configCount + templateCount + discoveredCount

	// Rough size estimation (average record size)
	avgRecordSize := int64(500) // bytes per record
	estimatedSize := int64(totalRecords) * avgRecordSize

	compression, _ := config.Config["compression"].(bool)
	if compression {
		estimatedSize = estimatedSize / 3 // Rough compression ratio
	}

	sampleData := fmt.Sprintf(`Backup Preview:
- Devices: %d
- Configurations: %d  
- Templates: %d
- Discovered Devices: %d
- Database Provider: %s
- Compression: %v
- Estimated Size: %d bytes`,
		deviceCount, configCount, templateCount, discoveredCount,
		providerName, compression, estimatedSize)

	return &sync.PreviewResult{
		Success:       true,
		SampleData:    []byte(sampleData),
		RecordCount:   totalRecords,
		EstimatedSize: estimatedSize,
	}, nil
}

// Import refuses every source. POST /import and POST /import/preview would reach it with a
// client-named path, the preview included; a backup is restored by id through
// POST /api/v1/import/restore, which stages it for the next start.
func (b *BackupPlugin) Import(_ context.Context, _ sync.ImportSource, _ sync.ImportConfig) (*sync.ImportResult, error) {
	return nil, fmt.Errorf("restoring a backup through the import route is not offered; stage it with POST /api/v1/import/restore: %w",
		sync.ErrImportNotImplemented)
}

// Capabilities returns plugin capabilities
func (b *BackupPlugin) Capabilities() sync.PluginCapabilities {
	return sync.PluginCapabilities{
		SupportsIncremental:    true,
		RequiresAuthentication: false,
		SupportedOutputs:       []string{"file"},
		MaxDataSize:            1024 * 1024 * 1024 * 10, // 10GB
		ConcurrencyLevel:       1,                       // Database backups should be sequential
	}
}

// Initialize initializes the plugin
func (b *BackupPlugin) Initialize(logger *logging.Logger) error {
	b.logger = logger
	b.logger.Info("Initialized backup exporter plugin")
	return nil
}

// Cleanup cleans up plugin resources
func (b *BackupPlugin) Cleanup() error {
	b.logger.Info("Cleaning up backup exporter plugin")
	return nil
}

// ValidateBackup validates a backup file
func (b *BackupPlugin) ValidateBackup(ctx context.Context, backupPath string) (*provider.ValidationResult, error) {
	if b != nil && b.logger != nil {
		b.logger.Info("Validating backup file", "path", backupPath)
	}

	// Get backup provider
	if b == nil || b.dbManager == nil {
		return nil, fmt.Errorf("backup plugin is not initialized with a database manager")
	}
	dbProvider := b.dbManager.GetProvider()
	backupProvider, ok := dbProvider.(provider.BackupProvider)
	if !ok {
		return nil, fmt.Errorf("database provider does not support backup validation")
	}

	return backupProvider.ValidateBackup(ctx, backupPath)
}

// calculateChecksum calculates SHA256 checksum of a file
func (b *BackupPlugin) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log but continue - file is still usable
			_ = err
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

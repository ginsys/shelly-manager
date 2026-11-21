package backup

import (
	"context"
	"errors"
	"fmt"
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

// DefaultBackupDir is the backup root when sync.export_base_dir is not set.
const DefaultBackupDir = "./data/backups"

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

// backupRoot is the backup root: the configured export root, else DefaultBackupDir. It is always passed
// explicitly, because an empty root would mean the export root.
func (b *BackupPlugin) backupRoot() string {
	if b.exportBaseDir != "" {
		return b.exportBaseDir
	}
	return DefaultBackupDir
}

// resolveOutputDir returns the absolute directory a backup is written to. An empty output_path is
// the backup root. A relative one joins under a configured root, and is relative to the working
// directory otherwise. Anything that is not inside the root is refused by the shared check
// (sync.ResolveUnderRoot), which also follows symbolic links of the deepest existing ancestor.
// The root is created first (it is configuration, never request text), because that check needs
// an existing root; that is the only thing this creates, and ValidateConfig reaches it too.
func (b *BackupPlugin) resolveOutputDir(outputPath string) (string, error) {
	absRoot, err := filepath.Abs(b.backupRoot())
	if err != nil {
		return "", fmt.Errorf("resolve the backup directory: %w", err)
	}
	if err = sync.EnsureExportRoot(absRoot); err != nil {
		return "", fmt.Errorf("prepare the backup directory %s: %w", absRoot, err)
	}
	if b.exportBaseDir == "" && outputPath != "" && !filepath.IsAbs(outputPath) {
		// Relative to the working directory, not joined under the root.
		if outputPath, err = filepath.Abs(outputPath); err != nil {
			return "", fmt.Errorf("output_path must be inside the backup directory %s", absRoot)
		}
	}
	dir, err := sync.ResolveUnderRoot(absRoot, outputPath)
	if err != nil {
		return "", fmt.Errorf("output_path must be inside the backup directory %s", absRoot)
	}
	return dir, nil
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
				Default:     DefaultBackupDir,
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
				"output_path":        DefaultBackupDir,
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

	// The directory, the file, its size and its checksum go through the opened backup root.
	root, rel, err := sync.OpenOutputRoot(b.backupRoot(), outputPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

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

	backupResult, err := b.writeBackup(ctx, backupProvider, root, rel, filename, backupConfig)
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

	// The provider counted and hashed the bytes it wrote to the file opened through the root.
	fileSize := backupResult.Size
	checksum := backupResult.Checksum

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

// writeBackup creates the backup file at rel/name below the root and has the provider write it there, so
// the provider never writes the file by path. A file left by a failed backup is removed through the root.
func (b *BackupPlugin) writeBackup(ctx context.Context, bp provider.BackupProvider, root *os.Root, rel, name string, cfg provider.BackupConfig) (*provider.BackupResult, error) {
	if err := root.MkdirAll(rel, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	file := filepath.Join(rel, name)
	f, err := root.Create(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}
	cfg.Output = f
	result, err := bp.CreateBackup(ctx, cfg)
	if err == nil && result == nil {
		err = errors.New("the database provider returned no backup result")
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil || !result.Success {
		if rerr := root.Remove(file); rerr != nil {
			err = errors.Join(err, fmt.Errorf("the failed backup file could not be removed: %w", rerr))
		}
	}
	return result, err
}

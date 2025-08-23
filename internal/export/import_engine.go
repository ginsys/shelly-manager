package export

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// ImportEngine manages import operations using existing export plugins
type ImportEngine struct {
	exportEngine *ExportEngine
	dbManager    DatabaseManagerInterface
	logger       *logging.Logger
}

// NewImportEngine creates a new import engine
func NewImportEngine(exportEngine *ExportEngine, dbManager DatabaseManagerInterface, logger *logging.Logger) *ImportEngine {
	return &ImportEngine{
		exportEngine: exportEngine,
		dbManager:    dbManager,
		logger:       logger,
	}
}

// Import performs an import operation
func (i *ImportEngine) Import(ctx context.Context, request ImportRequest) (*ImportResult, error) {
	startTime := time.Now()
	importID := uuid.New().String()

	i.logger.Info("Starting import operation",
		"import_id", importID,
		"plugin", request.PluginName,
		"format", request.Format,
		"dry_run", request.Options.DryRun,
	)

	// Handle different plugin types
	switch request.PluginName {
	case "backup":
		return i.importBackup(ctx, request, importID, startTime)
	case "gitops":
		return i.importGitOps(ctx, request, importID, startTime)
	default:
		return nil, fmt.Errorf("import not supported for plugin: %s", request.PluginName)
	}
}

// PreviewImport generates a preview of what would be imported
func (i *ImportEngine) PreviewImport(ctx context.Context, request ImportRequest) (*ImportResult, error) {
	// Force dry run for preview
	previewRequest := request
	previewRequest.Options.DryRun = true
	previewRequest.Options.ValidateOnly = true

	return i.Import(ctx, previewRequest)
}

// importBackup handles backup file imports using the backup plugin
func (i *ImportEngine) importBackup(ctx context.Context, request ImportRequest, importID string, startTime time.Time) (*ImportResult, error) {
	// For now, backup import is handled using the database provider directly
	// TODO: Integrate with proper backup plugin interface when available

	// Get backup provider from database manager
	dbProvider := i.dbManager.GetProvider()
	backupProvider, ok := dbProvider.(provider.BackupProvider)
	if !ok {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{"database provider does not support backup operations"},
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, fmt.Errorf("database provider does not support backup operations")
	}

	// Determine source path
	var sourcePath string
	switch request.Source.Type {
	case "file":
		sourcePath = request.Source.Path
	case "url":
		return nil, fmt.Errorf("URL source not yet supported for backup import")
	case "data":
		return nil, fmt.Errorf("data source not yet supported for backup import")
	default:
		return nil, fmt.Errorf("invalid source type: %s", request.Source.Type)
	}

	// If validation only, validate the backup file
	if request.Options.ValidateOnly {
		validation, err := backupProvider.ValidateBackup(ctx, sourcePath)
		if err != nil {
			return &ImportResult{
				Success:   false,
				ImportID:  importID,
				Errors:    []string{fmt.Sprintf("backup validation failed: %v", err)},
				Duration:  time.Since(startTime),
				CreatedAt: time.Now(),
			}, err
		}

		return &ImportResult{
			Success:         validation.Valid,
			ImportID:        importID,
			RecordsImported: int(validation.RecordCount),
			Duration:        time.Since(startTime),
			Errors:          validation.Errors,
			Warnings:        validation.Warnings,
			Metadata: map[string]interface{}{
				"validation_only": true,
				"backup_valid":    validation.Valid,
				"backup_size":     validation.Size,
				"backup_type":     string(validation.BackupType),
			},
			CreatedAt: time.Now(),
		}, nil
	}

	// Prepare restore configuration
	restoreConfig := provider.RestoreConfig{
		BackupPath:   sourcePath,
		DryRun:       request.Options.DryRun,
		PreserveData: !request.Options.ForceOverwrite,
	}

	// Perform the restore
	restoreResult, err := backupProvider.RestoreBackup(ctx, restoreConfig)
	if err != nil {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{err.Error()},
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	return &ImportResult{
		Success:         restoreResult.Success,
		ImportID:        importID,
		PluginName:      request.PluginName,
		Format:          request.Format,
		RecordsImported: int(restoreResult.RecordsRestored),
		Duration:        time.Since(startTime),
		Changes:         []ImportChange{}, // Backup restore doesn't track individual changes
		Errors:          []string{restoreResult.Error},
		Warnings:        restoreResult.Warnings,
		Metadata: map[string]interface{}{
			"restore_id":      restoreResult.RestoreID,
			"tables_restored": restoreResult.TablesRestored,
			"dry_run":         request.Options.DryRun,
		},
		CreatedAt: time.Now(),
	}, nil
}

// importGitOps handles GitOps YAML imports
func (i *ImportEngine) importGitOps(ctx context.Context, request ImportRequest, importID string, startTime time.Time) (*ImportResult, error) {
	i.logger.Info("Starting GitOps import",
		"import_id", importID,
		"dry_run", request.Options.DryRun,
	)

	// Determine source path
	var sourcePath string
	switch request.Source.Type {
	case "file":
		sourcePath = request.Source.Path
	case "url":
		return nil, fmt.Errorf("URL source not yet supported for GitOps import")
	case "data":
		return nil, fmt.Errorf("data source not yet supported for GitOps import")
	default:
		return nil, fmt.Errorf("invalid source type: %s", request.Source.Type)
	}

	// Create GitOps importer
	gitopsImporter := NewGitOpsImporter(i.dbManager, i.logger)

	// Load and validate GitOps structure
	gitopsData, err := gitopsImporter.LoadGitOpsStructure(sourcePath)
	if err != nil {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{fmt.Sprintf("failed to load GitOps structure: %v", err)},
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// If validation only, return success with preview
	if request.Options.ValidateOnly {
		changes := gitopsImporter.PreviewChanges(ctx, gitopsData)
		return &ImportResult{
			Success:         true,
			ImportID:        importID,
			RecordsImported: 0,
			Changes:         changes,
			Duration:        time.Since(startTime),
			Metadata: map[string]interface{}{
				"validation_only": true,
				"total_devices":   len(gitopsData.Devices),
				"total_changes":   len(changes),
			},
			CreatedAt: time.Now(),
		}, nil
	}

	// Perform the import
	importResult, err := gitopsImporter.Import(ctx, gitopsData, GitOpsImportOptions{
		DryRun:         request.Options.DryRun,
		ForceOverwrite: request.Options.ForceOverwrite,
		BackupBefore:   request.Options.BackupBefore,
	})
	if err != nil {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{err.Error()},
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Convert GitOps import result to generic import result
	return &ImportResult{
		Success:         importResult.Success,
		ImportID:        importID,
		PluginName:      request.PluginName,
		Format:          request.Format,
		RecordsImported: importResult.DevicesImported,
		RecordsSkipped:  importResult.DevicesSkipped,
		Duration:        time.Since(startTime),
		Changes:         importResult.Changes,
		Errors:          importResult.Errors,
		Warnings:        importResult.Warnings,
		Metadata: map[string]interface{}{
			"devices_imported": importResult.DevicesImported,
			"devices_skipped":  importResult.DevicesSkipped,
			"configs_applied":  importResult.ConfigsApplied,
			"dry_run":          request.Options.DryRun,
		},
		CreatedAt: time.Now(),
	}, nil
}

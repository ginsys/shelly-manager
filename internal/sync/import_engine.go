package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

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

// importBackup refuses every backup import.
func (i *ImportEngine) importBackup(_ context.Context, _ ImportRequest, importID string, startTime time.Time) (*ImportResult, error) {
	// A client-named path never reaches the database: a backup is restored by id through the
	// restore routes, which stage it for the next start (ginsys/shelly-manager#748).
	err := fmt.Errorf("restoring a backup through the import engine is not offered; stage it with POST /api/v1/import/restore: %w", ErrImportNotImplemented)
	return &ImportResult{
		Success:   false,
		ImportID:  importID,
		Errors:    []string{err.Error()},
		Duration:  time.Since(startTime),
		CreatedAt: time.Now(),
	}, err
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

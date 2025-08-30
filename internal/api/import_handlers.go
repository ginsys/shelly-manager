package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// ImportHandlers provides HTTP handlers for import operations
type ImportHandlers struct {
	syncEngine *sync.SyncEngine
	logger     *logging.Logger
}

// NewImportHandlers creates new import handlers
func NewImportHandlers(syncEngine *sync.SyncEngine, logger *logging.Logger) *ImportHandlers {
	return &ImportHandlers{
		syncEngine: syncEngine,
		logger:     logger,
	}
}

// AddImportRoutes adds import routes to the router
func (ih *ImportHandlers) AddImportRoutes(api *mux.Router) {
	// Backup import endpoints
	api.HandleFunc("/import/backup", ih.RestoreBackup).Methods("POST")
	api.HandleFunc("/import/backup/validate", ih.ValidateBackup).Methods("POST")

	// GitOps import endpoints
	api.HandleFunc("/import/gitops", ih.ImportGitOps).Methods("POST")
	api.HandleFunc("/import/gitops/preview", ih.PreviewGitOpsImport).Methods("POST")

	// Generic import endpoints
	api.HandleFunc("/import", ih.Import).Methods("POST")
	api.HandleFunc("/import/preview", ih.PreviewImport).Methods("POST")
	api.HandleFunc("/import/{id}", ih.GetImportResult).Methods("GET")
}

// RestoreBackup restores a backup file
func (ih *ImportHandlers) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	ih.logger.Info("Restore backup request")

	var requestBody struct {
		BackupPath string                 `json:"backup_path"`
		Config     map[string]interface{} `json:"config"`
		Options    sync.ImportOptions     `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		ih.logger.Error("Invalid request body", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}

	if requestBody.BackupPath == "" {
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "backup_path is required")
		return
	}

	// Create import request
	importRequest := sync.ImportRequest{
		PluginName: "backup",
		Format:     "sma",
		Source: sync.ImportSource{
			Type: "file",
			Path: requestBody.BackupPath,
		},
		Config:  requestBody.Config,
		Options: requestBody.Options,
	}

	// Perform the import
	result, err := ih.syncEngine.Import(r.Context(), importRequest)
	if err != nil {
		ih.logger.Error("Backup restore failed", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, result)
}

// ValidateBackup validates a backup file without importing it
func (ih *ImportHandlers) ValidateBackup(w http.ResponseWriter, r *http.Request) {
	ih.logger.Info("Validate backup request")

	var requestBody struct {
		BackupPath string `json:"backup_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		ih.logger.Error("Invalid request body", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}

	if requestBody.BackupPath == "" {
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "backup_path is required")
		return
	}

	// Create validation-only import request
	importRequest := sync.ImportRequest{
		PluginName: "backup",
		Format:     "sma",
		Source: sync.ImportSource{
			Type: "file",
			Path: requestBody.BackupPath,
		},
		Options: sync.ImportOptions{
			ValidateOnly: true,
		},
	}

	// Perform validation
	result, err := ih.syncEngine.Import(r.Context(), importRequest)
	if err != nil {
		ih.logger.Error("Backup validation failed", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, result)
}

// ImportGitOps imports a GitOps configuration
func (ih *ImportHandlers) ImportGitOps(w http.ResponseWriter, r *http.Request) {
	ih.logger.Info("GitOps import request")

	var requestBody struct {
		SourcePath string                 `json:"source_path"`
		Config     map[string]interface{} `json:"config"`
		Options    sync.ImportOptions     `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		ih.logger.Error("Invalid request body", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}

	if requestBody.SourcePath == "" {
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "source_path is required")
		return
	}

	// Create import request
	importRequest := sync.ImportRequest{
		PluginName: "gitops",
		Format:     "yaml",
		Source: sync.ImportSource{
			Type: "file",
			Path: requestBody.SourcePath,
		},
		Config:  requestBody.Config,
		Options: requestBody.Options,
	}

	// Perform the import
	result, err := ih.syncEngine.Import(r.Context(), importRequest)
	if err != nil {
		ih.logger.Error("GitOps import failed", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, result)
}

// PreviewGitOpsImport generates a preview of GitOps import changes
func (ih *ImportHandlers) PreviewGitOpsImport(w http.ResponseWriter, r *http.Request) {
	ih.logger.Info("GitOps import preview request")

	var requestBody struct {
		SourcePath string                 `json:"source_path"`
		Config     map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		ih.logger.Error("Invalid request body", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}

	if requestBody.SourcePath == "" {
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "source_path is required")
		return
	}

	// Create preview import request
	importRequest := sync.ImportRequest{
		PluginName: "gitops",
		Format:     "yaml",
		Source: sync.ImportSource{
			Type: "file",
			Path: requestBody.SourcePath,
		},
		Config: requestBody.Config,
		Options: sync.ImportOptions{
			DryRun:       true,
			ValidateOnly: true,
		},
	}

	// Generate preview by running in dry run mode
	result, err := ih.syncEngine.Import(r.Context(), importRequest)
	if err != nil {
		ih.logger.Error("GitOps import preview failed", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, map[string]interface{}{
		"preview":       result,
		"changes_count": len(result.Changes),
		"will_create":   ih.countChangesByType(result.Changes, "create"),
		"will_update":   ih.countChangesByType(result.Changes, "update"),
		"will_delete":   ih.countChangesByType(result.Changes, "delete"),
	})
}

// Import performs a generic import using any plugin
func (ih *ImportHandlers) Import(w http.ResponseWriter, r *http.Request) {
	ih.logger.Info("Generic import request")

	var importRequest sync.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&importRequest); err != nil {
		ih.logger.Error("Invalid import request", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "Invalid import request")
		return
	}

	// Validate required fields
	if importRequest.PluginName == "" {
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "plugin_name is required")
		return
	}

	if importRequest.Source.Type == "" {
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "source.type is required")
		return
	}

	// Perform the import
	result, err := ih.syncEngine.Import(r.Context(), importRequest)
	if err != nil {
		ih.logger.Error("Import failed", "plugin", importRequest.PluginName, "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, result)
}

// PreviewImport generates a preview of what would be imported
func (ih *ImportHandlers) PreviewImport(w http.ResponseWriter, r *http.Request) {
	ih.logger.Info("Import preview request")

	var importRequest sync.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&importRequest); err != nil {
		ih.logger.Error("Invalid import request", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "Invalid import request")
		return
	}

	// Generate preview by running in dry run mode
	result, err := ih.syncEngine.Import(r.Context(), importRequest)
	if err != nil {
		ih.logger.Error("Import preview failed", "plugin", importRequest.PluginName, "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, map[string]interface{}{
		"preview":       result,
		"changes_count": len(result.Changes),
		"summary": map[string]int{
			"will_create": ih.countChangesByType(result.Changes, "create"),
			"will_update": ih.countChangesByType(result.Changes, "update"),
			"will_delete": ih.countChangesByType(result.Changes, "delete"),
		},
	})
}

// GetImportResult returns the result of an import operation
func (ih *ImportHandlers) GetImportResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	importID := vars["id"]

	if res, ok := ih.syncEngine.GetImportResult(importID); ok {
		apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, res)
		return
	}
	apiresp.NewResponseWriter(ih.logger).WriteNotFoundError(w, r, "Import result")
}

// Helper functions

func (ih *ImportHandlers) countChangesByType(changes []sync.ImportChange, changeType string) int {
	count := 0
	for _, change := range changes {
		if change.Type == changeType {
			count++
		}
	}
	return count
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// SyncHandlers provides HTTP handlers for sync operations (export/import)
type SyncHandlers struct {
	syncEngine *sync.SyncEngine
	logger     *logging.Logger
}

// ExportHandlers provides backward compatibility
type ExportHandlers = SyncHandlers

// NewSyncHandlers creates new sync handlers
func NewSyncHandlers(syncEngine *sync.SyncEngine, logger *logging.Logger) *SyncHandlers {
	return &SyncHandlers{
		syncEngine: syncEngine,
		logger:     logger,
	}
}

// NewExportHandlers creates new export handlers (backward compatibility)
func NewExportHandlers(exportEngine *sync.ExportEngine, logger *logging.Logger) *ExportHandlers {
	return (*ExportHandlers)(NewSyncHandlers((*sync.SyncEngine)(exportEngine), logger))
}

// AddExportRoutes adds export routes to the router
func (eh *SyncHandlers) AddExportRoutes(api *mux.Router) {
	// Plugin management endpoints
	api.HandleFunc("/export/plugins", eh.ListPlugins).Methods("GET")
	api.HandleFunc("/export/plugins/{name}", eh.GetPlugin).Methods("GET")
	api.HandleFunc("/export/plugins/{name}/schema", eh.GetPluginSchema).Methods("GET")

	// Export endpoints
	api.HandleFunc("/export/backup", eh.CreateBackup).Methods("POST")
	api.HandleFunc("/export/backup/{id}", eh.GetExportResult).Methods("GET")
	api.HandleFunc("/export/backup/{id}/download", eh.DownloadBackup).Methods("GET")

	api.HandleFunc("/export/gitops", eh.CreateGitOpsExport).Methods("POST")
	api.HandleFunc("/export/gitops/{id}/download", eh.DownloadGitOpsExport).Methods("GET")

	// Generic export endpoints
	api.HandleFunc("/export", eh.Export).Methods("POST")
	api.HandleFunc("/export/{id}", eh.GetExportResult).Methods("GET")
	api.HandleFunc("/export/preview", eh.PreviewExport).Methods("POST")

	// Import endpoints
	api.HandleFunc("/import", eh.Import).Methods("POST")
	api.HandleFunc("/import/{id}", eh.GetImportResult).Methods("GET")
}

// ListPlugins returns all available export plugins
func (eh *SyncHandlers) ListPlugins(w http.ResponseWriter, r *http.Request) {
	eh.logger.Info("Listing export plugins")

	plugins := eh.syncEngine.ListPlugins()
	rw := apiresp.NewResponseWriter(eh.logger)
	rw.WriteSuccess(w, r, map[string]interface{}{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// GetPlugin returns information about a specific plugin
func (eh *SyncHandlers) GetPlugin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pluginName := vars["name"]

	plugin, err := eh.syncEngine.GetPlugin(pluginName)
	if err != nil {
		eh.logger.Error("Plugin not found", "name", pluginName, "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteNotFoundError(w, r, fmt.Sprintf("plugin %s", pluginName))
		return
	}

	info := plugin.Info()
	capabilities := plugin.Capabilities()

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, map[string]interface{}{
		"info":         info,
		"capabilities": capabilities,
	})
}

// GetPluginSchema returns the configuration schema for a plugin
func (eh *SyncHandlers) GetPluginSchema(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pluginName := vars["name"]

	plugin, err := eh.syncEngine.GetPlugin(pluginName)
	if err != nil {
		eh.logger.Error("Plugin not found", "name", pluginName, "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteNotFoundError(w, r, fmt.Sprintf("plugin %s", pluginName))
		return
	}

	schema := plugin.ConfigSchema()

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, schema)
}

// CreateBackup creates a new backup export
func (eh *SyncHandlers) CreateBackup(w http.ResponseWriter, r *http.Request) {
	eh.logger.Info("Creating backup export")

	var requestBody struct {
		Config  map[string]interface{} `json:"config"`
		Filters sync.ExportFilters     `json:"filters"`
		Options sync.ExportOptions     `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		eh.logger.Error("Invalid request body", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}

	// Create export request
	exportRequest := sync.ExportRequest{
		PluginName: "backup",
		Format:     "sma", // Default to Shelly Manager Archive format
		Config:     requestBody.Config,
		Filters:    requestBody.Filters,
		Output: sync.OutputConfig{
			Type: "file",
		},
		Options: requestBody.Options,
	}

	// Perform the export
	result, err := eh.syncEngine.Export(r.Context(), exportRequest)
	if err != nil {
		eh.logger.Error("Backup export failed", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// CreateGitOpsExport creates a new GitOps export
func (eh *SyncHandlers) CreateGitOpsExport(w http.ResponseWriter, r *http.Request) {
	eh.logger.Info("Creating GitOps export")

	var requestBody struct {
		Config  map[string]interface{} `json:"config"`
		Filters sync.ExportFilters     `json:"filters"`
		Options sync.ExportOptions     `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		eh.logger.Error("Invalid request body", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}

	// Create export request
	exportRequest := sync.ExportRequest{
		PluginName: "gitops",
		Format:     "yaml",
		Config:     requestBody.Config,
		Filters:    requestBody.Filters,
		Output: sync.OutputConfig{
			Type: "file",
		},
		Options: requestBody.Options,
	}

	// Perform the export
	result, err := eh.syncEngine.Export(r.Context(), exportRequest)
	if err != nil {
		eh.logger.Error("GitOps export failed", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// Export performs a generic export using any plugin
func (eh *SyncHandlers) Export(w http.ResponseWriter, r *http.Request) {
	eh.logger.Info("Generic export request")

	var exportRequest sync.ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&exportRequest); err != nil {
		eh.logger.Error("Invalid export request", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid export request")
		return
	}

	// Validate the export request
	if err := eh.syncEngine.ValidateExport(exportRequest); err != nil {
		eh.logger.Error("Export validation failed", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Perform the export
	result, err := eh.syncEngine.Export(r.Context(), exportRequest)
	if err != nil {
		eh.logger.Error("Export failed", "plugin", exportRequest.PluginName, "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// PreviewExport generates a preview of what would be exported
func (eh *SyncHandlers) PreviewExport(w http.ResponseWriter, r *http.Request) {
	eh.logger.Info("Export preview request")

	var exportRequest sync.ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&exportRequest); err != nil {
		eh.logger.Error("Invalid export request", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid export request")
		return
	}

	// Generate preview
	preview, err := eh.syncEngine.Preview(r.Context(), exportRequest)
	if err != nil {
		eh.logger.Error("Export preview failed", "plugin", exportRequest.PluginName, "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, preview)
}

// GetExportResult returns the result of an export operation
func (eh *SyncHandlers) GetExportResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exportID := vars["id"]

	// TODO: Implement export result storage and retrieval
	// For now, return a placeholder response
	eh.logger.Info("Getting export result", "export_id", exportID)

	apiresp.NewResponseWriter(eh.logger).WriteError(w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented, "Export result retrieval not yet implemented", nil)
}

// DownloadBackup serves a backup file for download
func (eh *SyncHandlers) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	backupID := vars["id"]

	// TODO: Implement backup file serving
	// This should:
	// 1. Validate the backup ID
	// 2. Check permissions
	// 3. Serve the file with appropriate headers

	eh.logger.Info("Downloading backup", "backup_id", backupID)

	apiresp.NewResponseWriter(eh.logger).WriteError(w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented, "Backup download not yet implemented", nil)
}

// DownloadGitOpsExport serves a GitOps export for download
func (eh *SyncHandlers) DownloadGitOpsExport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exportID := vars["id"]

	// TODO: Implement GitOps export file serving
	eh.logger.Info("Downloading GitOps export", "export_id", exportID)

	apiresp.NewResponseWriter(eh.logger).WriteError(w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented, "GitOps export download not yet implemented", nil)
}

// Import performs a generic import using any plugin
func (eh *SyncHandlers) Import(w http.ResponseWriter, r *http.Request) {
	eh.logger.Info("Generic import request")

	var importRequest sync.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&importRequest); err != nil {
		eh.logger.Error("Invalid import request", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid import request")
		return
	}

	// Perform the import
	result, err := eh.syncEngine.Import(r.Context(), importRequest)
	if err != nil {
		eh.logger.Error("Import failed", "plugin", importRequest.PluginName, "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}

	eh.logger.Info("Import completed successfully",
		"plugin", importRequest.PluginName,
		"imported", result.RecordsImported,
		"skipped", result.RecordsSkipped,
	)

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// GetImportResult returns the result of an import operation
func (eh *SyncHandlers) GetImportResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	importID := vars["id"]

	// TODO: Implement import result storage and retrieval
	// For now, return a placeholder response
	eh.logger.Info("Getting import result", "import_id", importID)

	apiresp.NewResponseWriter(eh.logger).WriteError(w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented, "Import result retrieval not yet implemented", nil)
}

// Utility functions

// Deprecated: use standardized response writer in handlers above.

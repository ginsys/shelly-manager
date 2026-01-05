package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// SyncHandlers provides HTTP handlers for sync operations (export/import)
type SyncHandlers struct {
	syncEngine *sync.SyncEngine
	logger     *logging.Logger

	// Security controls
	adminAPIKey   string
	exportBaseDir string
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

// SetAdminAPIKey sets an optional admin API key for RBAC guarding of sensitive endpoints.
// When set, endpoints in this handler require either `Authorization: Bearer <key>` or `X-API-Key: <key>`.
func (eh *SyncHandlers) SetAdminAPIKey(key string) {
	eh.adminAPIKey = key
}

// SetExportBaseDir sets an optional base directory for export downloads.
// When set, any served file must be within this directory (after path resolution).
func (eh *SyncHandlers) SetExportBaseDir(dir string) {
	eh.exportBaseDir = dir
}

// requireAdmin checks admin credentials if configured. It writes a standardized
// error response and returns false when access is denied.
func (eh *SyncHandlers) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if eh.adminAPIKey == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	xKey := r.Header.Get("X-API-Key")
	keyOK := false
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		keyOK = token == eh.adminAPIKey
	}
	if !keyOK && xKey != "" {
		keyOK = xKey == eh.adminAPIKey
	}
	if !keyOK {
		eh.logger.WithFields(map[string]any{
			"path":      r.URL.Path,
			"method":    r.Method,
			"component": "rbac",
			"event":     "access_denied",
		}).Warn("Admin RBAC check failed")
		apiresp.NewResponseWriter(eh.logger).WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Admin authorization required", nil)
		return false
	}
	return true
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
	api.HandleFunc("/export/backup/{id}", eh.DeleteBackup).Methods("DELETE")

	// Backward/alias routes for UI compatibility with expected payload shapes
	api.HandleFunc("/export/backups", eh.ListBackupsCompat).Methods("GET")
	api.HandleFunc("/export/backup-statistics", eh.GetBackupStatisticsCompat).Methods("GET")

	// JSON export endpoints
	api.HandleFunc("/export/json", eh.CreateJSONExport).Methods("POST")
	api.HandleFunc("/export/json/{id}/download", eh.DownloadExport).Methods("GET")

	// SMA export endpoints
	api.HandleFunc("/export/sma", eh.CreateSMAExport).Methods("POST")
	api.HandleFunc("/export/sma/{id}/download", eh.DownloadExport).Methods("GET")

	// YAML export endpoints (single-file content export)
	api.HandleFunc("/export/yaml", eh.CreateYAMLExport).Methods("POST")
	api.HandleFunc("/export/yaml/{id}/download", eh.DownloadExport).Methods("GET")

	api.HandleFunc("/export/gitops", eh.CreateGitOpsExport).Methods("POST")
	api.HandleFunc("/export/gitops/{id}/download", eh.DownloadGitOpsExport).Methods("GET")

	// Scheduling endpoints (register before generic /export/{id} to avoid collisions)
	api.HandleFunc("/export/schedules", eh.ListSchedules).Methods("GET")
	api.HandleFunc("/export/schedules", eh.CreateSchedule).Methods("POST")
	api.HandleFunc("/export/schedules/{id}", eh.GetSchedule).Methods("GET")
	api.HandleFunc("/export/schedules/{id}", eh.UpdateSchedule).Methods("PUT")
	api.HandleFunc("/export/schedules/{id}", eh.DeleteSchedule).Methods("DELETE")
	api.HandleFunc("/export/schedules/{id}/run", eh.RunSchedule).Methods("POST")

	// History & statistics endpoints
	api.HandleFunc("/export/history", eh.ListExportHistory).Methods("GET")
	api.HandleFunc("/export/history/{id}", eh.GetExportHistory).Methods("GET")
	api.HandleFunc("/export/statistics", eh.GetExportStatistics).Methods("GET")

	// Generic export endpoints (after history to avoid route collisions)
	api.HandleFunc("/export", eh.Export).Methods("POST")
	api.HandleFunc("/export/preview", eh.PreviewExport).Methods("POST")
	api.HandleFunc("/export/{id}", eh.GetExportResult).Methods("GET")
	api.HandleFunc("/export/{id}/download", eh.DownloadExport).Methods("GET")
}

// ListPlugins returns all available export plugins
func (eh *SyncHandlers) ListPlugins(w http.ResponseWriter, r *http.Request) {
	eh.logger.Info("Listing export plugins")

	infos := eh.syncEngine.ListPlugins()
	total := len(infos)
	pageSize := apiresp.GetQueryParamInt(r, "page_size", 0)
	page := apiresp.GetQueryParamInt(r, "page", 1)
	start, end := 0, total
	if pageSize > 0 {
		if page < 1 {
			page = 1
		}
		start = (page - 1) * pageSize
		if start > total {
			start = total
		}
		end = start + pageSize
		if end > total {
			end = total
		}
	} else {
		page = 1
		pageSize = total
	}
	pageInfos := infos
	if start != 0 || end != total {
		pageInfos = infos[start:end]
	}

	type pluginStatus struct {
		Available  bool   `json:"available"`
		Configured bool   `json:"configured"`
		Enabled    bool   `json:"enabled"`
		Error      string `json:"error,omitempty"`
		LastUsed   string `json:"last_used,omitempty"`
	}
	type pluginDTO struct {
		Name         string       `json:"name"`
		DisplayName  string       `json:"display_name"`
		Description  string       `json:"description"`
		Version      string       `json:"version"`
		Category     string       `json:"category"`
		Capabilities []string     `json:"capabilities"`
		Status       pluginStatus `json:"status"`
	}

	dtos := make([]pluginDTO, 0, len(pageInfos))
	categoriesCount := map[string]int{}
	for _, pi := range pageInfos {
		display := pi.Name
		if display == "" {
			display = pi.Description
		}
		dto := pluginDTO{
			Name:         pi.Name,
			DisplayName:  display,
			Description:  pi.Description,
			Version:      pi.Version,
			Category:     string(pi.Category),
			Capabilities: pi.SupportedFormats,
			Status:       pluginStatus{Available: true, Configured: true, Enabled: true},
		}
		dtos = append(dtos, dto)
		categoriesCount[dto.Category]++
	}

	type categoryDTO struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		PluginCount int    `json:"plugin_count"`
		Plugins     []any  `json:"plugins"`
	}
	cats := make([]categoryDTO, 0, len(categoriesCount))
	for name, count := range categoriesCount {
		// Simple display: Title case using x/text/cases
		disp := cases.Title(language.Und).String(name)
		cats = append(cats, categoryDTO{Name: name, DisplayName: disp, Description: "", PluginCount: count, Plugins: nil})
	}

	totalPages := 1
	if pageSize > 0 {
		if total%pageSize == 0 {
			totalPages = total / pageSize
		} else {
			totalPages = (total / pageSize) + 1
		}
		if total == 0 {
			totalPages = 1
		}
	}
	meta := &apiresp.Metadata{
		Page:  &apiresp.PaginationMeta{Page: page, PageSize: pageSize, TotalPages: totalPages, HasNext: page < totalPages, HasPrev: page > 1},
		Count: intPtr(len(dtos)), TotalCount: intPtr(total),
	}

	apiresp.NewResponseWriter(eh.logger).WriteSuccessWithMeta(w, r, map[string]interface{}{
		"plugins":    dtos,
		"categories": cats,
	}, meta)
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
	if !eh.requireAdmin(w, r) {
		return
	}
	eh.logger.Info("Creating backup export")

	var requestBody struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Format      string                 `json:"format"`
		Devices     []uint                 `json:"devices"`
		Config      map[string]interface{} `json:"config"`
		Filters     sync.ExportFilters     `json:"filters"`
		Options     sync.ExportOptions     `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		eh.logger.Error("Invalid request body", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}

	// Normalize and enrich config
	if requestBody.Config == nil {
		requestBody.Config = map[string]interface{}{}
	}
	if requestBody.Name != "" {
		requestBody.Config["name"] = requestBody.Name
	}
	if requestBody.Description != "" {
		requestBody.Config["description"] = requestBody.Description
	}
	// Apply device filters if provided
	if len(requestBody.Devices) > 0 {
		requestBody.Filters.DeviceIDs = requestBody.Devices
	}
	// Create export request
	exportRequest := sync.ExportRequest{
		PluginName: "backup",
		Format: func() string {
			if requestBody.Format != "" {
				return requestBody.Format
			}
			return "sma"
		}(),
		Config:  requestBody.Config,
		Filters: requestBody.Filters,
		Output:  sync.OutputConfig{Type: "file"},
		Options: requestBody.Options,
	}

	// Perform the export
	result, err := eh.syncEngine.Export(r.Context(), exportRequest)
	if err != nil {
		// Map unsupported operation to 501 for clearer UX
		msg := err.Error()
		if strings.Contains(msg, "does not support backup operations") {
			apiresp.NewResponseWriter(eh.logger).WriteError(
				w, r, http.StatusNotImplemented,
				apiresp.ErrCodeNotImplemented,
				"Backup not supported by current database provider",
				map[string]any{"detail": msg},
			)
			return
		}
		eh.logger.Error("Backup export failed", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	_ = eh.syncEngine.SaveExportHistory(r.Context(), exportRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// CreateGitOpsExport creates a new GitOps export
func (eh *SyncHandlers) CreateGitOpsExport(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
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
	_ = eh.syncEngine.SaveExportHistory(r.Context(), exportRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// CreateJSONExport creates a JSON content export (not DB snapshot)
func (eh *SyncHandlers) CreateJSONExport(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	eh.logger.Info("Creating JSON export")
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
	exportRequest := sync.ExportRequest{
		PluginName: "json",
		Format:     "json",
		Config:     requestBody.Config,
		Filters:    requestBody.Filters,
		Output:     sync.OutputConfig{Type: "file"},
		Options:    requestBody.Options,
	}
	result, err := eh.syncEngine.Export(r.Context(), exportRequest)
	if err != nil {
		eh.logger.Error("JSON export failed", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	_ = eh.syncEngine.SaveExportHistory(r.Context(), exportRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// CreateSMAExport creates a new SMA archive export
func (eh *SyncHandlers) CreateSMAExport(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	eh.logger.Info("Creating SMA export")
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
	exportRequest := sync.ExportRequest{
		PluginName: "sma",
		Format:     "sma",
		Config:     requestBody.Config,
		Filters:    requestBody.Filters,
		Output:     sync.OutputConfig{Type: "file"},
		Options:    requestBody.Options,
	}
	result, err := eh.syncEngine.Export(r.Context(), exportRequest)
	if err != nil {
		eh.logger.Error("SMA export failed", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	_ = eh.syncEngine.SaveExportHistory(r.Context(), exportRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// CreateYAMLExport creates a single-file YAML content export
func (eh *SyncHandlers) CreateYAMLExport(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	eh.logger.Info("Creating YAML export")
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
	exportRequest := sync.ExportRequest{
		PluginName: "yaml",
		Format:     "yaml",
		Config:     requestBody.Config,
		Filters:    requestBody.Filters,
		Output:     sync.OutputConfig{Type: "file"},
		Options:    requestBody.Options,
	}
	result, err := eh.syncEngine.Export(r.Context(), exportRequest)
	if err != nil {
		eh.logger.Error("YAML export failed", "error", err)
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	_ = eh.syncEngine.SaveExportHistory(r.Context(), exportRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// Export performs a generic export using any plugin
func (eh *SyncHandlers) Export(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
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
	_ = eh.syncEngine.SaveExportHistory(r.Context(), exportRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, result)
}

// PreviewExport generates a preview of what would be exported
func (eh *SyncHandlers) PreviewExport(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
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

	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, map[string]interface{}{
		"preview": preview,
		"summary": map[string]interface{}{
			"record_count":   preview.RecordCount,
			"estimated_size": preview.EstimatedSize,
		},
	})
}

// GetExportResult returns the result of an export operation
func (eh *SyncHandlers) GetExportResult(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	vars := mux.Vars(r)
	exportID := vars["id"]

	if res, ok := eh.syncEngine.GetExportResult(exportID); ok {
		apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, res)
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteNotFoundError(w, r, "Export result")
}

// DownloadBackup serves a backup file for download
func (eh *SyncHandlers) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	vars := mux.Vars(r)
	backupID := vars["id"]
	eh.serveExportByID(w, r, backupID)
}

// DownloadGitOpsExport serves a GitOps export for download
func (eh *SyncHandlers) DownloadGitOpsExport(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	vars := mux.Vars(r)
	exportID := vars["id"]
	eh.serveExportByID(w, r, exportID)
}

// DownloadExport serves a generic export file for download
func (eh *SyncHandlers) DownloadExport(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	vars := mux.Vars(r)
	exportID := vars["id"]
	eh.serveExportByID(w, r, exportID)
}

// DeleteBackup deletes a backup export record and its file
func (eh *SyncHandlers) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Missing export ID")
		return
	}
	// Remove both history and file
	_ = eh.syncEngine.DeleteExport(r.Context(), id, true)
	apiresp.NewResponseWriter(eh.logger).WriteNoContent(w, r)
}

// serveExportByID fetches an export by ID and serves its file (if available)
func (eh *SyncHandlers) serveExportByID(w http.ResponseWriter, r *http.Request, id string) {
	rw := apiresp.NewResponseWriter(eh.logger)
	res, ok := eh.syncEngine.GetExportResult(id)
	if !ok || res == nil {
		// Fallback to persisted history for file path
		if rec, err := eh.syncEngine.GetExportHistory(r.Context(), id); err == nil && rec != nil {
			if rec.FilePath != "" {
				if eh.exportBaseDir != "" {
					absBase, _ := filepath.Abs(eh.exportBaseDir)
					absFile, _ := filepath.Abs(rec.FilePath)
					rel, err := filepath.Rel(absBase, absFile)
					if err != nil || strings.HasPrefix(rel, "..") {
						rw.WriteError(w, r, http.StatusForbidden, apiresp.ErrCodeForbidden, "Download path not allowed", nil)
						return
					}
				}
				// Set a helpful filename for download
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(rec.FilePath)))
				setContentTypeForPath(w, rec.FilePath)
				http.ServeFile(w, r, rec.FilePath)
				return
			}
		}
		rw.WriteNotFoundError(w, r, "Export result")
		return
	}
	if res.OutputPath == "" {
		rw.WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeBadRequest, "No output file available for this export", nil)
		return
	}
	// If a base dir is configured, ensure the file is within it
	if eh.exportBaseDir != "" {
		absBase, _ := filepath.Abs(eh.exportBaseDir)
		absFile, _ := filepath.Abs(res.OutputPath)
		rel, err := filepath.Rel(absBase, absFile)
		if err != nil || strings.HasPrefix(rel, "..") {
			rw.WriteError(w, r, http.StatusForbidden, apiresp.ErrCodeForbidden, "Download path not allowed", nil)
			return
		}
	}
	// Set download filename and content type
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(res.OutputPath)))
	setContentTypeForPath(w, res.OutputPath)
	http.ServeFile(w, r, res.OutputPath)
}

// setContentTypeForPath sets Content-Type header based on file extension for better UX
func setContentTypeForPath(w http.ResponseWriter, path string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".gz":
		w.Header().Set("Content-Type", "application/gzip")
	case ".zip":
		w.Header().Set("Content-Type", "application/zip")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".yaml", ".yml":
		w.Header().Set("Content-Type", "application/yaml")
	case ".sma":
		// SMA is a gzip-compressed archive format
		w.Header().Set("Content-Type", "application/gzip")
	case ".sqlite":
		w.Header().Set("Content-Type", "application/octet-stream")
	default:
		// leave default; http.ServeFile may infer
	}
}

// Scheduling handlers

// ListSchedules returns all export schedules
func (eh *SyncHandlers) ListSchedules(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	schedules := eh.syncEngine.ListSchedules()
	total := len(schedules)
	pageSize := apiresp.GetQueryParamInt(r, "page_size", 0)
	page := apiresp.GetQueryParamInt(r, "page", 1)
	start, end := 0, total
	if pageSize > 0 {
		if page < 1 {
			page = 1
		}
		start = (page - 1) * pageSize
		if start > total {
			start = total
		}
		end = start + pageSize
		if end > total {
			end = total
		}
	} else {
		page = 1
		pageSize = total
	}
	pageSchedules := schedules
	if start != 0 || end != total {
		pageSchedules = schedules[start:end]
	}
	totalPages := 1
	if pageSize > 0 {
		if total%pageSize == 0 {
			totalPages = total / pageSize
		} else {
			totalPages = (total / pageSize) + 1
		}
		if total == 0 {
			totalPages = 1
		}
	}
	meta := &apiresp.Metadata{
		Page: &apiresp.PaginationMeta{
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
		Count:      intPtr(len(pageSchedules)),
		TotalCount: intPtr(total),
	}
	apiresp.NewResponseWriter(eh.logger).WriteSuccessWithMeta(w, r, map[string]interface{}{
		"schedules": pageSchedules,
	}, meta)
}

// local helper
// intPtr is defined in handlers.go; duplicate here avoided.

// CreateSchedule creates a new export schedule
func (eh *SyncHandlers) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	var req sync.ExportScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}
	sch, err := eh.syncEngine.CreateSchedule(req)
	if err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, err.Error())
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteCreated(w, r, sch)
}

// GetSchedule returns a schedule by ID
func (eh *SyncHandlers) GetSchedule(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	id := mux.Vars(r)["id"]
	if sch, ok := eh.syncEngine.GetSchedule(id); ok {
		apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, sch)
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteNotFoundError(w, r, "Export schedule")
}

// UpdateSchedule updates a schedule
func (eh *SyncHandlers) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	id := mux.Vars(r)["id"]
	var req sync.ExportScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, "Invalid request body")
		return
	}
	sch, err := eh.syncEngine.UpdateSchedule(id, req)
	if err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, err.Error())
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, sch)
}

// DeleteSchedule deletes a schedule
func (eh *SyncHandlers) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	id := mux.Vars(r)["id"]
	if err := eh.syncEngine.DeleteSchedule(id); err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, err.Error())
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, map[string]string{"status": "deleted"})
}

// RunSchedule triggers a schedule immediately
func (eh *SyncHandlers) RunSchedule(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	id := mux.Vars(r)["id"]
	res, err := eh.syncEngine.RunSchedule(r.Context(), id)
	if err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteValidationError(w, r, err.Error())
		return
	}
	// Best-effort record keeping; construct minimal request context
	_ = eh.syncEngine.SaveExportHistory(r.Context(), sync.ExportRequest{PluginName: res.PluginName, Format: res.Format}, res, requesterFrom(r))
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, res)
}

// Import performs a generic import using any plugin
func (eh *SyncHandlers) Import(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
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
	if !eh.requireAdmin(w, r) {
		return
	}
	vars := mux.Vars(r)
	importID := vars["id"]

	if res, ok := eh.syncEngine.GetImportResult(importID); ok {
		apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, res)
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteNotFoundError(w, r, "Import result")
}

// Utility functions

// Deprecated: use standardized response writer in handlers above.
// Export history & statistics
func (eh *SyncHandlers) ListExportHistory(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	// Pagination params
	q := r.URL.Query()
	page := parseIntDefault(q.Get("page"), 1)
	pageSize := parseIntDefault(q.Get("page_size"), 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	plugin := q.Get("plugin")
	successParam := q.Get("success")
	var success *bool
	if successParam != "" {
		v := strings.ToLower(successParam)
		b := v == "true" || v == "1" || v == "yes"
		success = &b
	}

	items, total, err := eh.syncEngine.ListExportHistory(r.Context(), page, pageSize, plugin, success)
	if err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	builder := apiresp.NewResponseBuilder(eh.logger).WithPagination(page, pageSize, total)
	resp := builder.Success(map[string]interface{}{"history": items})
	apiresp.NewResponseWriter(eh.logger).WriteSuccessWithMeta(w, r, resp.Data, resp.Meta)
}

func (eh *SyncHandlers) GetExportHistory(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	id := mux.Vars(r)["id"]
	rec, err := eh.syncEngine.GetExportHistory(r.Context(), id)
	if err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	if rec == nil {
		apiresp.NewResponseWriter(eh.logger).WriteNotFoundError(w, r, "Export history")
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, rec)
}

func (eh *SyncHandlers) GetExportStatistics(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	stats, err := eh.syncEngine.GetExportStatistics(r.Context())
	if err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, stats)
}

// ListBackupsCompat provides a compatibility layer for UI expecting { backups: [...] }
func (eh *SyncHandlers) ListBackupsCompat(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	page := parseIntDefault(q.Get("page"), 1)
	pageSize := parseIntDefault(q.Get("page_size"), 20)
	successParam := q.Get("success")
	var success *bool
	if successParam != "" {
		v := strings.ToLower(successParam)
		b := v == "true" || v == "1" || v == "yes"
		success = &b
	}
	formatFilter := strings.ToLower(strings.TrimSpace(q.Get("format")))

	items, total, err := eh.syncEngine.ListExportHistory(r.Context(), page, pageSize, "backup", success)
	if err != nil {
		apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
		return
	}
	// Map to UI BackupItem shape
	type BackupItem struct {
		ID          uint      `json:"id"`
		BackupID    string    `json:"backup_id"`
		Name        string    `json:"name"`
		Description string    `json:"description,omitempty"`
		Format      string    `json:"format"`
		DeviceCount int       `json:"device_count"`
		FileSize    int64     `json:"file_size,omitempty"`
		Checksum    string    `json:"checksum,omitempty"`
		Encrypted   bool      `json:"encrypted"`
		Success     bool      `json:"success"`
		Error       string    `json:"error_message,omitempty"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   string    `json:"created_by,omitempty"`
	}
	backups := make([]BackupItem, 0, len(items))
	for _, it := range items {
		if formatFilter != "" && strings.ToLower(it.Format) != formatFilter {
			continue
		}
		backups = append(backups, BackupItem{
			ID:       it.ID,
			BackupID: it.ExportID,
			Name: func() string {
				if it.Name != "" {
					return it.Name
				}
				return "backup"
			}(),
			Format:      it.Format,
			DeviceCount: it.RecordCount,
			FileSize:    it.FileSize,
			Encrypted:   false,
			Success:     it.Success,
			Error:       it.ErrorMessage,
			CreatedAt:   it.CreatedAt,
			CreatedBy:   it.RequestedBy,
		})
	}
	builder := apiresp.NewResponseBuilder(eh.logger).WithPagination(page, pageSize, total)
	resp := builder.Success(map[string]interface{}{"backups": backups})
	apiresp.NewResponseWriter(eh.logger).WriteSuccessWithMeta(w, r, resp.Data, resp.Meta)
}

// GetBackupStatisticsCompat returns backup-only statistics in expected UI shape
func (eh *SyncHandlers) GetBackupStatisticsCompat(w http.ResponseWriter, r *http.Request) {
	if !eh.requireAdmin(w, r) {
		return
	}
	// Aggregate over a reasonable window; iterate pages to collect all backup history
	page := 1
	pageSize := 200
	totalSize := int64(0)
	totalCount := 0
	successCount := 0
	byFormat := map[string]int{}
	var last time.Time

	for {
		items, total, err := eh.syncEngine.ListExportHistory(r.Context(), page, pageSize, "backup", nil)
		if err != nil {
			apiresp.NewResponseWriter(eh.logger).WriteInternalError(w, r, err)
			return
		}
		for _, it := range items {
			totalCount++
			if it.Success {
				successCount++
			}
			totalSize += it.FileSize
			byFormat[strings.ToLower(it.Format)]++
			if it.CreatedAt.After(last) {
				last = it.CreatedAt
			}
		}
		// Break when we've covered all items or no more pages
		if page*pageSize >= total || len(items) == 0 {
			break
		}
		page++
		if page > 50 { // safety cap
			break
		}
	}
	failureCount := totalCount - successCount
	data := map[string]interface{}{
		"total":      totalCount,
		"success":    successCount,
		"failure":    failureCount,
		"total_size": totalSize,
		"by_format":  byFormat,
	}
	if !last.IsZero() {
		data["last_backup"] = last.Format(time.RFC3339)
	}
	apiresp.NewResponseWriter(eh.logger).WriteSuccess(w, r, data)
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// requesterFrom extracts a best-effort identifier for auditing purposes
func requesterFrom(r *http.Request) string {
	if v := r.Header.Get("X-User-ID"); v != "" {
		return v
	}
	if v := r.Header.Get("X-User"); v != "" {
		return v
	}
	if v := r.Header.Get("Authorization"); v != "" {
		return v
	}
	return r.RemoteAddr
}

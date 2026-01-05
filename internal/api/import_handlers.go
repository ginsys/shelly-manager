package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// ImportHandlers provides HTTP handlers for import operations
type ImportHandlers struct {
	syncEngine *sync.SyncEngine
	logger     *logging.Logger

	adminAPIKey string
}

// NewImportHandlers creates new import handlers
func NewImportHandlers(syncEngine *sync.SyncEngine, logger *logging.Logger) *ImportHandlers {
	return &ImportHandlers{
		syncEngine: syncEngine,
		logger:     logger,
	}
}

// SetAdminAPIKey sets an optional admin API key for RBAC guarding of sensitive endpoints.
func (ih *ImportHandlers) SetAdminAPIKey(key string) {
	ih.adminAPIKey = key
}

func (ih *ImportHandlers) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if ih.adminAPIKey == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	xKey := r.Header.Get("X-API-Key")
	ok := strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == ih.adminAPIKey
	if !ok && xKey != "" && xKey == ih.adminAPIKey {
		ok = true
	}
	if !ok {
		apiresp.NewResponseWriter(ih.logger).WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Admin authorization required", nil)
		return false
	}
	return true
}

// AddImportRoutes adds import routes to the router
func (ih *ImportHandlers) AddImportRoutes(api *mux.Router) {
	// Backup import endpoints
	api.HandleFunc("/import/backup", ih.RestoreBackup).Methods("POST")
	api.HandleFunc("/import/backup/validate", ih.ValidateBackup).Methods("POST")

	// GitOps import endpoints
	api.HandleFunc("/import/gitops", ih.ImportGitOps).Methods("POST")
	api.HandleFunc("/import/gitops/preview", ih.PreviewGitOpsImport).Methods("POST")

	// History & statistics
	api.HandleFunc("/import/history", ih.ListImportHistory).Methods("GET")
	api.HandleFunc("/import/history/{id}", ih.GetImportHistory).Methods("GET")
	api.HandleFunc("/import/statistics", ih.GetImportStatistics).Methods("GET")

	// Generic import endpoints (after history to avoid route collisions)
	api.HandleFunc("/import", ih.Import).Methods("POST")
	api.HandleFunc("/import/preview", ih.PreviewImport).Methods("POST")
	api.HandleFunc("/import/{id}", ih.GetImportResult).Methods("GET")
}

// RestoreBackup restores a backup file
func (ih *ImportHandlers) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
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
	_ = ih.syncEngine.SaveImportHistory(r.Context(), importRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, result)
}

// ValidateBackup validates a backup file without importing it
func (ih *ImportHandlers) ValidateBackup(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
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
	if !ih.requireAdmin(w, r) {
		return
	}
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
	_ = ih.syncEngine.SaveImportHistory(r.Context(), importRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, result)
}

// PreviewGitOpsImport generates a preview of GitOps import changes
func (ih *ImportHandlers) PreviewGitOpsImport(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
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
	if !ih.requireAdmin(w, r) {
		return
	}
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
	_ = ih.syncEngine.SaveImportHistory(r.Context(), importRequest, result, requesterFrom(r))
	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, result)
}

// PreviewImport generates a preview of what would be imported
func (ih *ImportHandlers) PreviewImport(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	ih.logger.Info("Import preview request")

	var importRequest sync.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&importRequest); err != nil {
		ih.logger.Error("Invalid import request", "error", err)
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, "Invalid import request")
		return
	}

	// Force dry-run + validate-only to avoid side effects
	importRequest.Options.DryRun = true
	importRequest.Options.ValidateOnly = true

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
	if !ih.requireAdmin(w, r) {
		return
	}
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

// History & statistics endpoints
func (ih *ImportHandlers) ListImportHistory(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
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

	items, total, err := ih.syncEngine.ListImportHistory(r.Context(), page, pageSize, plugin, success)
	if err != nil {
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}
	builder := apiresp.NewResponseBuilder(ih.logger).WithPagination(page, pageSize, total)
	resp := builder.Success(map[string]interface{}{"history": items})
	apiresp.NewResponseWriter(ih.logger).WriteSuccessWithMeta(w, r, resp.Data, resp.Meta)
}

func (ih *ImportHandlers) GetImportHistory(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	id := mux.Vars(r)["id"]
	rec, err := ih.syncEngine.GetImportHistory(r.Context(), id)
	if err != nil {
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}
	if rec == nil {
		apiresp.NewResponseWriter(ih.logger).WriteNotFoundError(w, r, "Import history")
		return
	}
	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, rec)
}

func (ih *ImportHandlers) GetImportStatistics(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	stats, err := ih.syncEngine.GetImportStatistics(r.Context())
	if err != nil {
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}
	apiresp.NewResponseWriter(ih.logger).WriteSuccess(w, r, stats)
}

// requesterFrom extracts a best-effort identifier for auditing purposes
// requesterFrom provided in sync_handlers.go (same package)

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/security"
	"github.com/ginsys/shelly-manager/internal/security/apikey"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// ImportHandlers provides HTTP handlers for import operations
type ImportHandlers struct {
	syncEngine *sync.SyncEngine
	logger     *logging.Logger

	adminKey apikey.Key
	// exportBaseDir, when set, is the directory stored backups must lie in. It is the same
	// directory export downloads are confined to (export.output_directory).
	exportBaseDir string
}

// NewImportHandlers creates new import handlers
func NewImportHandlers(syncEngine *sync.SyncEngine, logger *logging.Logger) *ImportHandlers {
	return &ImportHandlers{
		syncEngine: syncEngine,
		logger:     logger,
	}
}

// SetExportBaseDir confines the backups the restore preview will open to dir.
func (ih *ImportHandlers) SetExportBaseDir(dir string) {
	ih.exportBaseDir = dir
}

// SetAdminAPIKey sets an optional admin API key for RBAC guarding of sensitive endpoints.
func (ih *ImportHandlers) SetAdminAPIKey(key string) {
	ih.adminKey.Set(key)
}

func (ih *ImportHandlers) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	key := ih.adminKey.Get()
	if key == "" {
		return true
	}
	if !apikey.Authorized(r, key) {
		apiresp.NewResponseWriter(ih.logger).WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Admin authorization required", nil)
		return false
	}
	return true
}

// AddImportRoutes adds import routes to the router
func (ih *ImportHandlers) AddImportRoutes(api *mux.Router) {
	// Backup check: a stored backup is named by its export id, and is never restored
	api.HandleFunc("/import/restore-preview", ih.PreviewRestore).Methods("POST")

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

// backupFileName is the name the backup plugin gives its files. A stored path is only used when its
// base name has this shape, so a history row that names any other file is refused.
var backupFileName = regexp.MustCompile(`^shelly-backup-\d{8}-\d{6}-[0-9a-f]{8}\.sqlite(\.gz|\.zip)?$`)

// backupValidator is what the verify route needs from the backup plugin.
type backupValidator interface {
	ValidateBackup(ctx context.Context, backupPath string) (*provider.ValidationResult, error)
}

// backupFile returns the absolute path of a stored backup, or false when the file is not one this
// server wrote: wrong name, outside the export directory, or not a regular file (a symlink or a
// directory). The path comes from a database row, which is data, so it is checked like input.
func (ih *ImportHandlers) backupFile(stored string) (string, bool) {
	if !backupFileName.MatchString(filepath.Base(stored)) {
		return "", false
	}
	path, err := filepath.Abs(stored)
	if err != nil {
		return "", false
	}
	if ih.exportBaseDir != "" {
		if path, err = security.ValidatePath(ih.exportBaseDir, path); err != nil {
			return "", false
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return path, true
}

// PreviewRestore checks a stored backup without restoring it. The client names the backup by its
// export id only; the path comes from the export history, so no request field reaches the file
// system. Restoring is not offered: it replaces the live database and is not safe today.
func (ih *ImportHandlers) PreviewRestore(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	rw := apiresp.NewResponseWriter(ih.logger)

	var requestBody struct {
		BackupID string `json:"backup_id"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&requestBody); err != nil {
		rw.WriteValidationError(w, r, "Invalid request body: only backup_id is accepted")
		return
	}
	if requestBody.BackupID == "" {
		rw.WriteValidationError(w, r, "backup_id is required")
		return
	}

	rec, err := ih.syncEngine.GetExportHistory(r.Context(), requestBody.BackupID)
	if err != nil {
		ih.logger.Error("Restore preview: history read failed", "error", err)
		rw.WriteInternalError(w, r, err)
		return
	}
	if rec == nil || rec.PluginName != "backup" {
		rw.WriteNotFoundError(w, r, "Backup")
		return
	}
	if !rec.Success {
		rw.WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeBadRequest, "This backup did not complete, so it cannot be checked", nil)
		return
	}
	path, ok := ih.backupFile(rec.FilePath)
	if !ok {
		ih.logger.Warn("Restore preview: stored backup file refused", "export_id", rec.ExportID)
		rw.WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeBadRequest, "The backup file is not available", nil)
		return
	}
	plugin, err := ih.syncEngine.GetPlugin("backup")
	validator, isValidator := plugin.(backupValidator)
	if err != nil || !isValidator {
		rw.WriteError(w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented, "Backup checking is not available", nil)
		return
	}
	res, err := validator.ValidateBackup(r.Context(), path)
	if err != nil {
		ih.logger.Error("Restore preview: validation failed", "error", err)
		rw.WriteInternalError(w, r, err)
		return
	}

	// The provider checks that the file exists, is not empty and can be opened, and nothing more.
	warnings := append([]string{"Only the file's presence and size were checked; its contents were not read."}, res.Warnings...)
	rw.WriteSuccess(w, r, map[string]interface{}{
		"backup_id":    rec.ExportID,
		"name":         rec.Name,
		"valid":        res.Valid,
		"file_size":    res.Size,
		"record_count": rec.RecordCount,
		"created_at":   rec.CreatedAt,
		"errors":       res.Errors,
		"warnings":     warnings,
	})
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
		if result != nil {
			_ = ih.syncEngine.SaveImportHistory(r.Context(), importRequest, result, requesterFrom(r))
		}
		ih.logger.Error("GitOps import failed", "error", err)
		ih.writeSyncError(w, r, err)
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
		ih.writeSyncError(w, r, err)
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
		if result != nil {
			_ = ih.syncEngine.SaveImportHistory(r.Context(), importRequest, result, requesterFrom(r))
		}
		ih.logger.Error("Import failed", "plugin", importRequest.PluginName, "error", err)
		ih.writeSyncError(w, r, err)
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
		ih.writeSyncError(w, r, err)
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

func (ih *ImportHandlers) writeSyncError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, sync.ErrInvalidPluginConfig),
		errors.Is(err, sync.ErrPluginNotFound),
		errors.Is(err, sync.ErrUnsupportedFormat),
		errors.Is(err, sync.ErrInvalidImportData),
		errors.Is(err, sync.ErrInvalidExportData):
		apiresp.NewResponseWriter(ih.logger).WriteValidationError(w, r, err.Error())
	case errors.Is(err, sync.ErrImportNotImplemented):
		apiresp.NewResponseWriter(ih.logger).WriteError(
			w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented, err.Error(), nil,
		)
	default:
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
	}
}

// GetImportResult returns the result of an import operation
func (ih *ImportHandlers) GetImportResult(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	vars := mux.Vars(r)
	importID := vars["id"]

	res, err := ih.syncEngine.FindImportResult(r.Context(), importID)
	if err != nil {
		apiresp.NewResponseWriter(ih.logger).WriteInternalError(w, r, err)
		return
	}
	if res != nil {
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

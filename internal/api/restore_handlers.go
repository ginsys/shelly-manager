package api

import (
	"encoding/json"
	"errors"
	"net/http"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/database/restore"
)

// SetRestoreDatabase tells the restore routes which database a restore replaces. For any database
// but a SQLite file the routes stay off and answer 501.
func (ih *ImportHandlers) SetRestoreDatabase(cfg provider.DatabaseConfig) {
	ih.restoreLive, _ = database.RestoreTarget(cfg)
}

// restoreAvailable answers 501 and returns false when the database is not a SQLite file.
func (ih *ImportHandlers) restoreAvailable(w http.ResponseWriter, r *http.Request) bool {
	if ih.restoreLive == "" {
		apiresp.NewResponseWriter(ih.logger).WriteError(w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented,
			"Restore is available for SQLite databases only", nil)
		return false
	}
	return true
}

// StageRestore checks a stored backup and stages it, so that the server applies it at its next start.
// The client names the backup by its export id only; the path comes from the export history, as in
// PreviewRestore. A dry run runs the checks and stages nothing. The live database is never touched.
func (ih *ImportHandlers) StageRestore(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	if !ih.restoreAvailable(w, r) {
		return
	}
	rw := apiresp.NewResponseWriter(ih.logger)

	var requestBody struct {
		BackupID string `json:"backup_id"`
		DryRun   bool   `json:"dry_run"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&requestBody); err != nil {
		rw.WriteValidationError(w, r, "Invalid request body: only backup_id and dry_run are accepted")
		return
	}
	if requestBody.BackupID == "" {
		rw.WriteValidationError(w, r, "backup_id is required")
		return
	}

	rec, path, ok := ih.storedBackup(w, r, requestBody.BackupID, "Restore")
	if !ok {
		return
	}
	report, err := restore.Stage(r.Context(), ih.restoreLive, path, rec.ExportID, requestBody.DryRun)
	var refused *restore.CheckError
	if errors.As(err, &refused) {
		ih.logger.Warn("Restore: backup refused", "export_id", rec.ExportID, "reason", refused.Reason)
		rw.WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeBadRequest, refused.Reason, nil)
		return
	}
	if err != nil {
		ih.logger.Error("Restore: staging failed", "export_id", rec.ExportID, "error", err)
		rw.WriteInternalError(w, r, err)
		return
	}
	if requestBody.DryRun {
		rw.WriteSuccess(w, r, map[string]interface{}{
			"backup_id": report.BackupID,
			"dry_run":   true,
			"valid":     true,
			"file_size": report.Size,
		})
		return
	}
	rw.WriteAccepted(w, r, map[string]interface{}{
		"backup_id": report.BackupID,
		"file_size": report.Size,
		"applies":   "at the next start",
	})
}

// GetRestoreStatus reports the staged restore, if any, and what the last start did with one.
func (ih *ImportHandlers) GetRestoreStatus(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	if !ih.restoreAvailable(w, r) {
		return
	}
	rw := apiresp.NewResponseWriter(ih.logger)
	pending, err := restore.Pending(ih.restoreLive)
	if err != nil {
		ih.logger.Error("Restore: pending read failed", "error", err)
		rw.WriteInternalError(w, r, err)
		return
	}
	last, err := restore.LastResult(ih.restoreLive)
	if err != nil {
		ih.logger.Error("Restore: result read failed", "error", err)
		rw.WriteInternalError(w, r, err)
		return
	}
	rw.WriteSuccess(w, r, map[string]interface{}{"pending": pending, "last_result": last})
}

// CancelRestore removes a staged restore before it is applied.
func (ih *ImportHandlers) CancelRestore(w http.ResponseWriter, r *http.Request) {
	if !ih.requireAdmin(w, r) {
		return
	}
	if !ih.restoreAvailable(w, r) {
		return
	}
	rw := apiresp.NewResponseWriter(ih.logger)
	cancelled, err := restore.Cancel(ih.restoreLive)
	if err != nil {
		ih.logger.Error("Restore: cancel failed", "error", err)
		rw.WriteInternalError(w, r, err)
		return
	}
	if !cancelled {
		rw.WriteNotFoundError(w, r, "Pending restore")
		return
	}
	rw.WriteNoContent(w, r)
}

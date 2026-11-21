package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/redact"
)

// DesiredHistoryEntry is one version of a device's desired configuration. NewDesired is the stored text,
// masked again on the way out, and null where no text could be stored.
type DesiredHistoryEntry struct {
	ID           uint      `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Action       string    `json:"action"`
	ChangedBy    string    `json:"changed_by"`
	RestoredFrom *uint     `json:"restored_from"`
	NewDesired   *string   `json:"new_desired"`
}

// rollbackRequest is the body of a rollback. Secrets is keyed by the paths a 409 listed.
type rollbackRequest struct {
	HistoryID uint            `json:"history_id"`
	Secrets   json.RawMessage `json:"secrets"`
}

// secretsOf reads the secrets of a request: absent is none, and anything but an object (null included)
// is an error.
func (r rollbackRequest) secretsOf() (map[string]any, bool) {
	if len(r.Secrets) == 0 {
		return nil, true
	}
	var out map[string]any
	if json.Unmarshal(r.Secrets, &out) != nil || out == nil {
		return nil, false
	}
	return out, true
}

// GetDeviceDesiredHistory lists the versions of a device's desired configuration, newest first.
func (h *Handler) GetDeviceDesiredHistory(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	id, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}
	rows, err := h.ConfigService.ConfigurationSvc.ListDesiredHistory(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	entries := make([]DesiredHistoryEntry, 0, len(rows))
	for _, row := range rows {
		entry := DesiredHistoryEntry{
			ID: row.ID, CreatedAt: row.CreatedAt, Action: row.Action,
			ChangedBy: row.ChangedBy, RestoredFrom: row.RestoredFrom,
		}
		if row.NewDesired != nil {
			masked := maskJSONText(*row.NewDesired)
			entry.NewDesired = &masked
		}
		entries = append(entries, entry)
	}
	rw.WriteSuccess(w, r, map[string]any{"history": entries})
}

// RollbackDeviceDesiredConfig makes a stored version the device's desired configuration. It contacts no
// device: the device stays as it is until an apply. A secret the version cannot supply is refused with
// the list of paths to enter, and the body's secrets are never logged, echoed or put in an error.
func (h *Handler) RollbackDeviceDesiredConfig(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	id, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// The decode error is never put in the answer or a log line: it can quote the body.
	raw, readErr := io.ReadAll(r.Body)
	req, decoded := decodeRollbackRequest(raw)
	if readErr != nil || redact.HasRepeatedMember(raw) || !decoded {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid JSON request body", nil)
		return
	}

	supplied, valid := req.secretsOf()
	if !valid {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid JSON request body", nil)
		return
	}

	result, err := h.ConfigService.ConfigurationSvc.RollbackDesired(uint(id), req.HistoryID, supplied)
	var refusal *configuration.RollbackError
	switch {
	case err == nil:
		message := fmt.Sprintf("Desired configuration set to version %d. The device is unchanged until you apply it.", result.VersionID)
		if result.NoChange {
			message = fmt.Sprintf("Desired configuration already is version %d. Nothing was written.", result.VersionID)
		}
		rw.WriteSuccess(w, r, map[string]any{
			"version_id":            result.VersionID,
			"no_change":             result.NoChange,
			"detached_template_ids": append([]uint{}, result.DetachedTemplateIDs...),
			"message":               message,
		})
	case errors.Is(err, configuration.ErrDeviceNotFound):
		rw.WriteNotFoundError(w, r, "Device")
	case errors.Is(err, configuration.ErrHistoryNotFound):
		rw.WriteNotFoundError(w, r, "Desired configuration version")
	case errors.Is(err, configuration.ErrDesiredChanged):
		rw.WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "The desired configuration changed while the rollback was prepared. Read it again and retry.", nil)
	case errors.Is(err, configuration.ErrVersionNotRestorable):
		rw.WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeConfigurationError, err.Error(), nil)
	case errors.As(err, &refusal):
		h.writeRollbackRefusal(rw, w, r, refusal)
	default:
		rw.WriteInternalError(w, r, err)
	}
}

// writeRollbackRefusal answers a refusal with paths and no value.
func (h *Handler) writeRollbackRefusal(rw *apiresp.ResponseWriter, w http.ResponseWriter, r *http.Request, refusal *configuration.RollbackError) {
	details := map[string]any{"paths": append([]string{}, refusal.Paths...)}
	switch refusal.Kind {
	case configuration.RollbackNeedsSecrets:
		rw.WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "This version holds secrets that history does not keep. Enter them for the listed paths and retry.", details)
	case configuration.RollbackNotRestorable:
		rw.WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeConfigurationError,
			"This version holds a masked value at a path that takes no secret, so it cannot be restored. Nothing can be entered for it.", details)
	case configuration.RollbackBadSecrets:
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest,
			"The secrets entered are not an answer to the list: a key off the list, or an empty value, the mask or a value of the wrong type.", nil)
	default:
		rw.WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeConfigurationError,
			"This version would change when the configuration is recomputed, so it cannot be restored as it is.", details)
	}
}

// decodeRollbackRequest reads the body: one object with a history_id and only the members of a request,
// so a misspelt secrets member is a 400 and never a silent 409 again.
func decodeRollbackRequest(raw []byte) (rollbackRequest, bool) {
	var req rollbackRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var extra json.RawMessage
	if dec.Decode(&req) != nil || dec.Decode(&extra) != io.EOF {
		return req, false
	}
	return req, req.HistoryID != 0
}

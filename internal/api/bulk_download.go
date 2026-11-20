package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// maxBulkDownloadDevices is the most devices one bulk-download names. The file is built in memory
// from the database, so the cap bounds the answer, not device contact: 200 fully populated
// configurations are about 1 MiB (TestBulkDownload_TheAnswerAtTheCapIsSmall). Bulk-apply keeps its
// cap of 10, because it contacts devices one after another.
const maxBulkDownloadDevices = 200

// Entry statuses of the downloaded file.
const (
	bulkDownloadOK    = "ok"
	bulkDownloadError = "error"
)

// bulkDownloadEntry is one device of the file. Config is the desired configuration with every
// secret masked (maskSecrets); an entry that could not be read carries a fixed Error text and no
// Config, never the text of the underlying error.
type bulkDownloadEntry struct {
	DeviceID   uint            `json:"device_id"`
	DeviceName string          `json:"device_name,omitempty"`
	MAC        string          `json:"mac,omitempty"`
	Status     string          `json:"status"`
	Config     json.RawMessage `json:"config,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type bulkDownloadFile struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Devices     []bulkDownloadEntry `json:"devices"`
}

// BulkDownloadConfigs handles POST /api/v1/config/bulk-download. It answers one JSON file with the
// desired configuration of each named device, secrets masked. It reads the database and never
// contacts a device.
func (h *Handler) BulkDownloadConfigs(w http.ResponseWriter, r *http.Request) {
	// Admin before any parsing or read.
	if !h.requireAdmin(w, r) {
		return
	}
	selection, ok := h.decodeBulkRequest(w, r)
	if !ok {
		return
	}
	rw := h.responseWriter()
	if selection.StopOnError {
		rw.WriteValidationError(w, r, "options.stop_on_error does not apply to bulk-download: a device that cannot be read is an entry, not a stop")
		return
	}
	if len(selection.IDs) > maxBulkDownloadDevices {
		rw.WriteValidationError(w, r, fmt.Sprintf("bulk-download takes at most %d devices, got %d", maxBulkDownloadDevices, len(selection.IDs)))
		return
	}

	now := time.Now().UTC()
	file := bulkDownloadFile{GeneratedAt: now, Devices: make([]bulkDownloadEntry, 0, len(selection.IDs))}
	for _, id := range selection.IDs {
		file.Devices = append(file.Devices, h.bulkDownloadEntry(id))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"shelly-configs-%s.json\"", now.Format("20060102-150405")))
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(file); err != nil {
		h.logger.WithFields(map[string]any{"error": err.Error()}).Warn("Failed to write the bulk-download file")
	}
}

// bulkDownloadEntry reads one device. Everything about its configuration is decided on the one
// stored value read with the device: no desired configuration, a configuration the typed model would
// truncate, and the configuration itself. A device with an empty desired configuration is reported
// as having none and not as having no settings.
func (h *Handler) bulkDownloadEntry(id uint) bulkDownloadEntry {
	entry := bulkDownloadEntry{DeviceID: id, Status: bulkDownloadError}
	device, err := h.Service.DB.GetDevice(id)
	if err != nil {
		entry.Error = "device not found"
		return entry
	}
	entry.DeviceName = device.Name
	entry.MAC = device.MAC

	config, err := configuration.StoredDesiredConfig(device.DesiredConfig)
	switch {
	case errors.Is(err, configuration.ErrNoDesiredConfig):
		entry.Error = "no desired configuration is set for this device"
		return entry
	case errors.Is(err, configuration.ErrUnsupportedSettings):
		entry.Error = "the stored configuration has settings this version does not know, so it cannot be downloaded completely"
		return entry
	case err != nil:
		return h.bulkDownloadUnreadable(entry, err)
	}
	maskSecrets(config)
	// Encoded here, per device: a stored value that decodes but cannot be encoded again (a time
	// zone offset Go reads and refuses to write) fails this entry and not the whole file.
	encoded, err := json.Marshal(config)
	if err != nil {
		return h.bulkDownloadUnreadable(entry, err)
	}
	entry.Status = bulkDownloadOK
	entry.Config = encoded
	return entry
}

// bulkDownloadUnreadable gives an entry the fixed text. The error may quote the stored bytes, so it
// goes to the log and not to the file.
func (h *Handler) bulkDownloadUnreadable(entry bulkDownloadEntry, err error) bulkDownloadEntry {
	h.logger.WithFields(map[string]any{
		"device_id": entry.DeviceID,
		"error":     err.Error(),
	}).Warn("Failed to read the desired configuration for bulk-download")
	entry.Error = "the configuration could not be read"
	return entry
}

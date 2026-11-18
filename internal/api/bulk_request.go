package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// The three bulk config routes (bulk-import, bulk-drift-detect and
// bulk-drift-detect-enhanced) act on the devices the request names and on no others. The body is
//
//	{"device_ids": [1, 2, 3], "options": {"stop_on_error": false}}
//
// A request that names no device, carries a field the server does not read, or is not one JSON
// object is refused with 400 before any device is contacted.

const maxBulkRequestBytes = 1 << 20

// bulkSelection is a validated bulk request: the devices to act on, once each and in request
// order, and whether the first failure ends the run.
type bulkSelection struct {
	IDs         []uint
	StopOnError bool
}

type bulkRequestBody struct {
	DeviceIDs []int64 `json:"device_ids"`
	Options   struct {
		StopOnError bool `json:"stop_on_error"`
	} `json:"options"`
}

// decodeBulkRequest reads and validates the body. On a refusal it has written the 400 and returns
// false.
func (h *Handler) decodeBulkRequest(w http.ResponseWriter, r *http.Request) (bulkSelection, bool) {
	selection, err := parseBulkRequest(http.MaxBytesReader(w, r.Body, maxBulkRequestBytes))
	if err != nil {
		h.responseWriter().WriteValidationError(w, r, err.Error())
		return bulkSelection{}, false
	}
	return selection, true
}

func parseBulkRequest(body io.Reader) (bulkSelection, error) {
	var req bulkRequestBody
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return bulkSelection{}, errors.New(`a JSON body naming the devices is required, e.g. {"device_ids":[1,2]}`)
		}
		return bulkSelection{}, fmt.Errorf("invalid request body: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return bulkSelection{}, errors.New("invalid request body: more than one JSON value")
	}
	if len(req.DeviceIDs) == 0 {
		return bulkSelection{}, errors.New("device_ids must name at least one device")
	}

	selection := bulkSelection{StopOnError: req.Options.StopOnError}
	seen := make(map[int64]bool, len(req.DeviceIDs))
	for _, id := range req.DeviceIDs {
		if id <= 0 {
			return bulkSelection{}, fmt.Errorf("device_ids must be positive device ids, got %d", id)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		selection.IDs = append(selection.IDs, uint(id))
	}
	return selection, nil
}

// runBulkConfig applies do to each selected device in order and reports one result per device.
// An id that names no device is that device's failure and the others still run. With
// StopOnError, the devices after the first failure are reported skipped and are not contacted.
// do returns the stored configuration id where the operation has one.
func (h *Handler) runBulkConfig(sel bulkSelection, operation string, do func(id uint) (uint, error)) *configuration.BulkImportResult {
	result := &configuration.BulkImportResult{
		Total:   len(sel.IDs),
		Results: make([]configuration.ImportResult, 0, len(sel.IDs)),
	}

	for _, id := range sel.IDs {
		entry := configuration.ImportResult{DeviceID: id}
		device, lookupErr := h.Service.DB.GetDevice(id)
		if lookupErr == nil {
			entry.DeviceName = device.Name
			entry.DeviceIP = device.IP
		}

		switch {
		case sel.StopOnError && result.Failed > 0:
			entry.Status = configuration.BulkStatusSkipped
			result.Skipped++
		case lookupErr != nil:
			entry.Status = configuration.BulkStatusFailed
			entry.Error = fmt.Sprintf("device not found: %v", lookupErr)
			result.Failed++
		default:
			configID, err := do(id)
			if err != nil {
				entry.Status = configuration.BulkStatusFailed
				entry.Error = err.Error()
				result.Failed++
				h.logger.WithFields(map[string]any{
					"device_id": id,
					"device_ip": entry.DeviceIP,
					"error":     err.Error(),
				}).Warn("Failed to run bulk " + operation + " for device config")
			} else {
				entry.Status = configuration.BulkStatusSuccess
				entry.ConfigID = configID
				result.Success++
				h.logger.WithFields(map[string]any{
					"device_id": id,
					"device_ip": entry.DeviceIP,
				}).Info("Successfully ran bulk " + operation + " for device config")
			}
		}

		result.Results = append(result.Results, entry)
	}
	return result
}

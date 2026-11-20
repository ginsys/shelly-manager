package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

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
	if err := decodeBulkJSON(body, &req); err != nil {
		return bulkSelection{}, err
	}
	return req.selection()
}

// decodeBulkJSON reads the one JSON object of a bulk request into into, refusing a field it does not
// have and anything after the object.
func decodeBulkJSON(body io.Reader, into any) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New(`a JSON body naming the devices is required, e.g. {"device_ids":[1,2]}`)
		}
		return fmt.Errorf("invalid request body: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid request body: more than one JSON value")
	}
	return nil
}

// selection validates the device list: at least one positive id, each device once, in request order.
func (req bulkRequestBody) selection() (bulkSelection, error) {
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

// bulkReadMargin is how long before the request deadline a bulk read stops starting devices. A device
// that is dead (nothing listening) takes 9 s to give up on the fake devices of ginsys/shelly-manager#730,
// so a device started just inside the budget ends before the deadline and the answer is written in time.
// A proposal from that one reading, not a bound; a variable so a test can shorten it.
var bulkReadMargin = 10 * time.Second

// bulkReadContext is the context a bulk read runs under: the request's, ended bulkReadMargin before its
// deadline, so the run stops starting devices in time to answer. A request with no deadline keeps its
// own context. A client that leaves ends the run at the next device boundary, whatever the deadline.
func bulkReadContext(r *http.Request) (context.Context, context.CancelFunc) {
	if deadline, ok := r.Context().Deadline(); ok {
		return context.WithDeadline(r.Context(), deadline.Add(-bulkReadMargin))
	}
	return r.Context(), func() {}
}

// bulkReadTimeLimited says whether a drift run left a device unread because the call's time limit was
// reached. The comprehensive report leaves skipped devices out, so such a run answers the basic result
// that names every device.
func bulkReadTimeLimited(result *configuration.BulkDriftResult) bool {
	for _, entry := range result.Results {
		if entry.Status == configuration.BulkStatusSkipped && entry.Error == configuration.ReasonCallEnded {
			return true
		}
	}
	return false
}

// runBulkConfig applies do to each selected device in order and reports one result per device.
// An id that names no device is that device's failure and the others still run. With
// StopOnError, the devices after the first failure are reported skipped and are not contacted.
// do returns the stored configuration id where the operation has one.
func (h *Handler) runBulkConfig(ctx context.Context, sel bulkSelection, operation string, do func(id uint) (uint, error)) *configuration.BulkImportResult {
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
		case ctx.Err() != nil:
			// The call's time limit was reached or the client left: no device is started.
			entry.Status = configuration.BulkStatusSkipped
			entry.Error = configuration.ReasonCallEnded
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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// POST /config/bulk-apply stores one partial device configuration as the overrides of each named
// device and applies it. The only call that reaches a device is the per-device apply
// (ShellyService.ApplyDesiredConfig), once for each named device and for none other: the request has
// no tag, group or template member, and the patch never touches a template.
//
//	{"device_ids": [1, 2], "config": {"system": {"device": {"name": "x"}}}, "options": {"stop_on_error": false}}
//
// Every selected device is checked against its own model before the first write, so a JSON that one
// device cannot take is written to none. The call is not atomic after that: device 3 can be applied
// while device 4 is refused, and nothing is rolled back. The overrides stay stored when the apply
// does not succeed, and the entry says so. The call has a time limit (bulkApplyBudget): the devices
// not started by then, or after the request ended, are skipped and not touched.

// maxBulkApplyDevices is the most devices one bulk-apply names. A call lasts as long as its devices
// do, so the cap bounds the time, not the size. It is a proposal, not a measurement.
const maxBulkApplyDevices = 10

// The call has a time limit of its own, because 10 devices written one after another outlast the 30 s
// that every other protected route is given (ginsys/shelly-manager#725). The deadline is the route's
// share of the request middleware, found by its name (routeDeadline). The budget is the part of it
// the run may use: no device is started once it is spent, so the answer is written before the
// deadline. A device that ignores its context, or a step before the first write that stalls, can still run past it, and the 408 then means that the
// outcome is unknown. Both are proposals, not measurements, like the cap. Variables so a test can
// shorten them.
var (
	bulkApplyDeadline = 5 * time.Minute
	bulkApplyBudget   = bulkApplyDeadline - time.Minute
)

// Entry statuses.
const (
	bulkApplyApplied     = "applied"     // written and read back equal
	bulkApplyPending     = "pending"     // written and read back, but the desired configuration moved meanwhile
	bulkApplyUnconfirmed = "unconfirmed" // written; the device stopped answering after a network change
	bulkApplyRefused     = "refused"     // nothing was written to the device
	bulkApplyFailed      = "failed"      // the call did not end in a recorded, verified write
	bulkApplySkipped     = "skipped"     // not contacted
)

// The reasons are fixed texts: they name what happened and never a value or the text of a device
// error, which can echo what it was sent. An error text goes to the log.
const (
	reasonNotFound        = "device not found"
	reasonUnreadable      = "the device record could not be read; nothing was changed"
	reasonNotStored       = "could not store the configuration"
	reasonOverridesUnread = "the stored overrides could not be read; nothing was changed"
	reasonInvalid         = "the settings are not valid for this device model; nothing was stored or written"
	reasonBlocked         = "not written: the settings are not valid for another selected device"
	reasonStopped         = "not contacted: an earlier device did not end applied and stop_on_error is set"
	reasonCallEnded       = configuration.ReasonCallEnded
	reasonPending         = "the desired configuration changed while it was applied; the device stays pending"
	reasonUnconfirmed     = "the device stopped answering after a network change, so the write was not verified; the device stays pending"
	reasonRefusedWrite    = "the device refused a write; the device stays pending"
	reasonDifferent       = "the device holds a different value than asked; the device stays pending"
	reasonRejectedLogin   = "the device rejected the stored login; the device stays pending"
	reasonNotRecorded     = "the apply did not end in a verified, recorded write; the device stays pending"
	refusedAfterStoreTag  = "nothing was written to the device, the new settings are stored and the device stays pending"
)

// bulkApplyEntry is one device of the answer. Stored says whether the new overrides are stored; it is
// null where that is not known (the storage call failed part way).
type bulkApplyEntry struct {
	DeviceID       uint     `json:"device_id"`
	DeviceName     string   `json:"device_name,omitempty"`
	DeviceIP       string   `json:"device_ip,omitempty"`
	Status         string   `json:"status"`
	Stored         *bool    `json:"stored"`
	Reason         string   `json:"reason,omitempty"`
	Failures       []string `json:"failures,omitempty"`
	RequiresReboot bool     `json:"requires_reboot"`
}

type bulkApplyResult struct {
	Total       int              `json:"total"`
	Applied     int              `json:"applied"`
	Pending     int              `json:"pending"`
	Unconfirmed int              `json:"unconfirmed"`
	Refused     int              `json:"refused"`
	Failed      int              `json:"failed"`
	Skipped     int              `json:"skipped"`
	Results     []bulkApplyEntry `json:"results"`
}

type bulkApplyRequestBody struct {
	bulkRequestBody
	Config json.RawMessage `json:"config"`
}

// BulkApplyConfigs handles POST /api/v1/config/bulk-apply.
func (h *Handler) BulkApplyConfigs(w http.ResponseWriter, r *http.Request) {
	// Admin before any parsing, read or write: the route writes to physical devices.
	if !h.requireAdmin(w, r) {
		return
	}
	rw := h.responseWriter()

	var req bulkApplyRequestBody
	if err := decodeBulkJSON(http.MaxBytesReader(w, r.Body, maxBulkRequestBytes), &req); err != nil {
		rw.WriteValidationError(w, r, err.Error())
		return
	}
	selection, err := req.selection()
	if err != nil {
		rw.WriteValidationError(w, r, err.Error())
		return
	}
	if len(selection.IDs) > maxBulkApplyDevices {
		rw.WriteValidationError(w, r, fmt.Sprintf("bulk-apply takes at most %d devices, got %d", maxBulkApplyDevices, len(selection.IDs)))
		return
	}
	patch, _, err := decodeBulkApplyConfig(req.Config)
	if err != nil {
		rw.WriteValidationError(w, r, err.Error())
		return
	}

	entries, blocked := h.bulkApplyCheck(selection.IDs, patch)
	if blocked {
		entries = bulkApplyBlocked(entries)
	} else {
		// The run's own context outlives the request: a client that leaves, or the deadline of the
		// request, never cuts a device in the middle of a write. Only the budget does.
		// The budget counts from the request's own deadline when it has one, so a body that arrived late
		// leaves the run less time and the answer is still written before the deadline.
		budget := bulkApplyBudget
		if dl, ok := r.Context().Deadline(); ok {
			budget = min(budget, time.Until(dl)-(bulkApplyDeadline-bulkApplyBudget))
		}
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), budget)
		defer cancel()
		entries = h.bulkApplyRun(r.Context(), runCtx, entries, req.Config, selection.StopOnError)
	}

	result := bulkApplyResult{Total: len(entries), Results: entries}
	for _, e := range entries {
		switch e.Status {
		case bulkApplyApplied:
			result.Applied++
		case bulkApplyPending:
			result.Pending++
		case bulkApplyUnconfirmed:
			result.Unconfirmed++
		case bulkApplyRefused:
			result.Refused++
		case bulkApplyFailed:
			result.Failed++
		case bulkApplySkipped:
			result.Skipped++
		}
	}
	if r.Context().Err() != nil {
		// Nobody reads this answer: the client left or the request deadline passed.
		h.logger.WithFields(map[string]any{
			"applied": result.Applied, "pending": result.Pending, "unconfirmed": result.Unconfirmed,
			"refused": result.Refused, "failed": result.Failed, "skipped": result.Skipped,
		}).Warn("The request ended before bulk-apply was answered; the devices were handled as counted")
	}
	rw.WriteSuccess(w, r, result)
}

// decodeBulkApplyConfig reads the config member as a partial device configuration: an object with at
// least one member, whose fields the typed model all knows, and the paths sent as null, which remove
// an override as on the PATCH route. It is strict where the PATCH route is not: a misspelled key
// decodes to nothing, the patch would be empty and the apply would write the stored configuration
// under an "applied" that says nothing about what was asked. A fresh value comes back on every call,
// because restoreMaskedSecrets changes the one it is given.
func decodeBulkApplyConfig(raw json.RawMessage) (*configuration.DeviceConfiguration, [][]string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || len(bytes.TrimSpace(trimmed[1:len(trimmed)-1])) == 0 {
		return nil, nil, errors.New("config must be a JSON object naming at least one setting, as the body of PATCH /devices/{id}/overrides/new")
	}
	var patch configuration.DeviceConfiguration
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&patch); err != nil {
		if name, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			return nil, nil, fmt.Errorf("config has a field this version does not know: %s", name)
		}
		return nil, nil, errors.New("config is not a valid partial device configuration")
	}
	removed, err := configuration.NullPaths(trimmed)
	if err != nil {
		return nil, nil, errors.New("config is not a valid partial device configuration")
	}
	return &patch, removed, nil
}

// bulkApplyCheck is pass 1: it reads each selected device and checks the patch against its model,
// writing nothing. A device that is not found, or cannot be read, is refused and blocks no other. A
// device the patch is invalid for is refused too, and blocks every write of the request. An entry
// with no status yet is a valid device that pass 2 completes.
func (h *Handler) bulkApplyCheck(ids []uint, patch *configuration.DeviceConfiguration) (entries []bulkApplyEntry, blocked bool) {
	entries = make([]bulkApplyEntry, 0, len(ids))
	for _, id := range ids {
		device, err := h.DB.GetDevice(id)
		if err != nil {
			reason := reasonNotFound
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				h.logBulkApply(id, "", err, "Could not read the device for bulk-apply")
				reason = reasonUnreadable
			}
			entries = append(entries, bulkApplyRefusedEntry(id, reason))
			continue
		}
		entry := bulkApplyEntry{DeviceID: id, DeviceName: device.Name, DeviceIP: device.IP}
		fieldErrs, err := validateOverrideBody(service.DeviceModel(device), patch)
		if err != nil {
			h.logBulkApply(id, device.IP, err, "Could not check the bulk-apply settings against the device model")
		}
		if err != nil || len(fieldErrs) > 0 {
			entry = bulkApplyStoredEntry(entry, bulkApplyRefused, false, reasonInvalid)
			for _, f := range fieldErrs {
				entry.Failures = append(entry.Failures, f.Path+": not valid for this device model")
			}
			blocked = true
		}
		entries = append(entries, entry)
	}
	return entries, blocked
}

// bulkApplyBlocked answers a request that pass 1 stopped: the invalid and missing devices keep their
// refusal, every valid device is skipped, and nothing was written.
func bulkApplyBlocked(entries []bulkApplyEntry) []bulkApplyEntry {
	for i, entry := range entries {
		if entry.Status == "" {
			entries[i] = bulkApplyStoredEntry(entry, bulkApplySkipped, false, reasonBlocked)
		}
	}
	return entries
}

// bulkApplyRun is pass 2: for each device in order, store the patch as the device's overrides, then
// apply. With stop_on_error, the first device that does not end applied ends the run and the
// devices after it are skipped and not contacted. The run also ends when the request has ended or
// the budget is spent: no later device is started, so none has its overrides stored, and the one
// in flight finishes on runCtx.
func (h *Handler) bulkApplyRun(reqCtx, runCtx context.Context, entries []bulkApplyEntry, raw json.RawMessage, stopOnError bool) []bulkApplyEntry {
	stopped := false
	for i, entry := range entries {
		switch {
		case stopped:
			// After the stop nothing is contacted, a device refused in pass 1 included.
			entries[i] = bulkApplyStoredEntry(entry, bulkApplySkipped, false, reasonStopped)
		case entry.Status != "":
			// Refused in pass 1: not found or not readable. It keeps its entry.
		case reqCtx.Err() != nil || runCtx.Err() != nil:
			entries[i] = bulkApplyStoredEntry(entry, bulkApplySkipped, false, reasonCallEnded)
		default:
			entries[i] = h.bulkApplyDevice(runCtx, entry, raw)
		}
		// A device not reached because the call ended is no earlier failure to stop on: the
		// devices after it say the same thing.
		if stopOnError && entries[i].Status != bulkApplyApplied && entries[i].Reason != reasonCallEnded {
			stopped = true
		}
	}
	return entries
}

// bulkApplyDevice stores the patch for one device, as PatchDeviceNewOverrides does, and applies.
func (h *Handler) bulkApplyDevice(ctx context.Context, entry bulkApplyEntry, raw json.RawMessage) bulkApplyEntry {
	id := entry.DeviceID
	patch, removed, err := decodeBulkApplyConfig(raw)
	if err != nil {
		// A guard: the same bytes decoded in the handler, so this cannot fail today, and a later
		// change must not be able to store an unchecked patch.
		return bulkApplyStoredEntry(entry, bulkApplyRefused, false, reasonInvalid)
	}
	// A secret sent back as the mask leaves the stored one as it is. Without the stored overrides the
	// mask would become an empty secret, and this route applies it to the device in the same call, so
	// an unreadable record is a refusal and not the PATCH route's fallback.
	stored, err := h.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	if err != nil {
		h.logBulkApply(id, entry.DeviceIP, err, "Could not read the stored overrides for bulk-apply")
		return bulkApplyStoredEntry(entry, bulkApplyFailed, false, reasonOverridesUnread)
	}
	restoreMaskedSecrets(patch, stored)
	keepOmittedActionURLs(patch, stored, matchSecretNulls(removed))

	if err = h.ConfigService.ConfigurationSvc.PatchDeviceOverrides(id, patch, removed); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			return bulkApplyStoredEntry(entry, bulkApplyRefused, false, reasonNotFound)
		}
		// The overrides may or may not be stored, and the desired configuration is not: unknown.
		h.logBulkApply(id, entry.DeviceIP, err, "Could not store the bulk-apply overrides")
		failed := bulkApplyStoredEntry(entry, bulkApplyFailed, false, reasonNotStored)
		failed.Stored = nil
		return failed
	}

	result, applyErr := h.Service.ApplyDesiredConfig(ctx, id)
	if applyErr != nil {
		h.logBulkApply(id, entry.DeviceIP, applyErr, "Bulk-apply could not apply the configuration")
	}
	return bulkApplyOutcome(entry, result, applyErr)
}

// bulkApplyOutcome maps the result of the per-device apply, run after the overrides were stored, to
// the entry. Each refusal says that the device was not written to and that the new settings are
// stored and the device stays pending: "refused" alone reads as "nothing changed".
func bulkApplyOutcome(entry bulkApplyEntry, result *configuration.ApplyVerifyResult, err error) bulkApplyEntry {
	if err != nil {
		switch {
		case errors.Is(err, configuration.ErrUnsupportedGeneration):
			return bulkApplyStoredEntry(entry, bulkApplyRefused, true,
				"this device generation cannot be configured by the manager; "+refusedAfterStoreTag)
		case errors.Is(err, configuration.ErrUnsupportedSettings):
			return bulkApplyStoredEntry(entry, bulkApplyRefused, true,
				"the device does not support one of the settings; "+refusedAfterStoreTag)
		case errors.Is(err, configuration.ErrDeviceIdentity):
			return bulkApplyStoredEntry(entry, bulkApplyRefused, true,
				"the device at the stored address is not the stored device; "+refusedAfterStoreTag)
		case errors.Is(err, configuration.ErrApplyInProgress):
			return bulkApplyStoredEntry(entry, bulkApplyRefused, true,
				"another apply to this device is running; "+refusedAfterStoreTag)
		case shelly.IsAuthError(err):
			return bulkApplyStoredEntry(entry, bulkApplyFailed, true, reasonRejectedLogin)
		}
		return bulkApplyStoredEntry(entry, bulkApplyFailed, true, reasonNotRecorded)
	}
	if result == nil {
		return bulkApplyStoredEntry(entry, bulkApplyFailed, true, reasonNotRecorded)
	}

	apply, verify := result.ApplyResult, result.VerifyResult
	if apply != nil {
		entry.RequiresReboot = apply.RequiresReboot
	}
	switch {
	case apply != nil && apply.Unconfirmed:
		return bulkApplyStoredEntry(entry, bulkApplyUnconfirmed, true, reasonUnconfirmed)
	case result.ConfigApplied:
		return bulkApplyStoredEntry(entry, bulkApplyApplied, true, "")
	case apply != nil && apply.Success && verify != nil && verify.Match:
		// Written and equal on read-back; the only thing missing is the applied flag, which the
		// apply leaves unset when the desired configuration changed while it ran.
		return bulkApplyStoredEntry(entry, bulkApplyPending, true, reasonPending)
	}

	entry = bulkApplyStoredEntry(entry, bulkApplyFailed, true, reasonNotRecorded)
	entry.Failures = bulkApplyFailures(result)
	switch {
	case apply != nil && len(apply.Failures) > 0:
		entry.Reason = reasonRefusedWrite
	case verify != nil && len(verify.Differences) > 0:
		entry.Reason = reasonDifferent
	}
	return entry
}

// bulkApplyFailures lists the settings by path only: the device's own error text and the values on
// both sides stay out, unlike applyFailureMessages, which prints a read-back difference's values.
func bulkApplyFailures(result *configuration.ApplyVerifyResult) []string {
	var failures []string
	if result.ApplyResult != nil {
		for _, f := range result.ApplyResult.Failures {
			failures = append(failures, f.Path+": the device refused the write")
		}
	}
	if result.VerifyResult != nil {
		for _, d := range result.VerifyResult.Differences {
			failures = append(failures, d.Path+": the device holds a different value")
		}
	}
	return failures
}

func bulkApplyStoredEntry(entry bulkApplyEntry, status string, stored bool, reason string) bulkApplyEntry {
	entry.Status = status
	entry.Stored = &stored
	entry.Reason = reason
	return entry
}

// bulkApplyRefusedEntry is the entry of a device that is not there to be written to.
func bulkApplyRefusedEntry(id uint, reason string) bulkApplyEntry {
	return bulkApplyStoredEntry(bulkApplyEntry{DeviceID: id}, bulkApplyRefused, false, reason)
}

func (h *Handler) logBulkApply(id uint, ip string, err error, msg string) {
	h.logger.WithFields(map[string]any{
		"device_id": id,
		"device_ip": ip,
		"error":     err.Error(),
	}).Warn(msg)
}

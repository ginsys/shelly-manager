package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
)

var (
	_ = json.Marshal
)

type SetDeviceTemplatesRequest struct {
	TemplateIDs []uint `json:"template_ids"`
}

type DeviceTemplatesResponse struct {
	Templates   []TemplateResponse `json:"templates"`
	TemplateIDs []uint             `json:"template_ids"`
}

type DeviceOverridesRequest struct {
	Overrides *configuration.DeviceConfiguration `json:"overrides"`
}

type DesiredConfigResponse struct {
	Config  *configuration.DeviceConfiguration `json:"config"`
	Sources map[string]string                  `json:"sources"`
	// Inherited is what the device falls back to without its overrides: the template layers
	// alone, with the template each value comes from. A field that a template and an override
	// both set shows only the override in Config. Secrets that are set show as a mask.
	Inherited        *configuration.DeviceConfiguration `json:"inherited"`
	InheritedSources map[string]string                  `json:"inherited_sources"`
}

type ConfigApplyResponse struct {
	Success        bool     `json:"success"`
	AppliedCount   int      `json:"applied_count"`
	FailedCount    int      `json:"failed_count"`
	RequiresReboot bool     `json:"requires_reboot"`
	Failures       []string `json:"failures,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type ConfigStatusResponse struct {
	DeviceID       uint   `json:"device_id"`
	ConfigApplied  bool   `json:"config_applied"`
	HasOverrides   bool   `json:"has_overrides"`
	TemplateCount  int    `json:"template_count"`
	LastApplied    string `json:"last_applied,omitempty"`
	PendingChanges bool   `json:"pending_changes"`
}

type ConfigVerifyResponse struct {
	Match       bool                  `json:"match"`
	Differences []ConfigDifferenceDTO `json:"differences,omitempty"`
}

type ConfigDifferenceDTO struct {
	Path     string `json:"path"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
}

func (h *Handler) GetDeviceNewTemplates(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	templates, err := h.ConfigService.ConfigurationSvc.GetDeviceTemplates(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"device_id": id,
			"component": "api",
		}).Error("Failed to get device templates")
		rw.WriteInternalError(w, r, err)
		return
	}

	templateIDs := make([]uint, len(templates))
	responses := make([]TemplateResponse, len(templates))
	for i, tmpl := range templates {
		templateIDs[i] = tmpl.ID
		responses[i] = templateToResponse(&tmpl)
	}

	rw.WriteSuccess(w, r, DeviceTemplatesResponse{
		Templates:   responses,
		TemplateIDs: templateIDs,
	})
}

func (h *Handler) SetDeviceNewTemplates(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	var req SetDeviceTemplatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.ConfigService.ConfigurationSvc.SetDeviceTemplates(uint(id), req.TemplateIDs); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		if errors.Is(err, configuration.ErrTemplateIDsNotFound) {
			rw.WriteValidationError(w, r, err.Error())
			return
		}
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"device_id": id,
			"component": "api",
		}).Error("Failed to set device templates")
		rw.WriteInternalError(w, r, err)
		return
	}

	templates, _ := h.ConfigService.ConfigurationSvc.GetDeviceTemplates(uint(id))
	desiredConfig, sources, _ := h.ConfigService.ConfigurationSvc.GetDesiredConfig(uint(id))
	inherited, inheritedSources, _ := h.ConfigService.ConfigurationSvc.GetInheritedConfig(uint(id))
	if inherited != nil {
		maskSecrets(inherited)
	}

	templateIDs := make([]uint, len(templates))
	responses := make([]TemplateResponse, len(templates))
	for i, tmpl := range templates {
		templateIDs[i] = tmpl.ID
		responses[i] = templateToResponse(&tmpl)
	}

	rw.WriteSuccess(w, r, map[string]any{
		"templates":         responses,
		"template_ids":      templateIDs,
		"desired_config":    desiredConfig,
		"sources":           sources,
		"inherited":         inherited,
		"inherited_sources": inheritedSources,
	})
}

func (h *Handler) AddDeviceNewTemplate(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	deviceID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	templateID, err := strconv.ParseUint(vars["templateId"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid template ID", nil)
		return
	}

	position := -1
	if posStr := r.URL.Query().Get("position"); posStr != "" {
		if pos, parseErr := strconv.Atoi(posStr); parseErr == nil {
			position = pos
		}
	}

	if err := h.ConfigService.ConfigurationSvc.AddTemplateToDevice(uint(deviceID), uint(templateID), position); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		if errors.Is(err, configuration.ErrTemplateNotFound) {
			rw.WriteNotFoundError(w, r, "Template")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	templates, _ := h.ConfigService.ConfigurationSvc.GetDeviceTemplates(uint(deviceID))
	responses := make([]TemplateResponse, len(templates))
	for i, tmpl := range templates {
		responses[i] = templateToResponse(&tmpl)
	}

	rw.WriteSuccess(w, r, map[string]any{"templates": responses})
}

func (h *Handler) RemoveDeviceNewTemplate(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	deviceID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	templateID, err := strconv.ParseUint(vars["templateId"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid template ID", nil)
		return
	}

	if err := h.ConfigService.ConfigurationSvc.RemoveTemplateFromDevice(uint(deviceID), uint(templateID)); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	templates, _ := h.ConfigService.ConfigurationSvc.GetDeviceTemplates(uint(deviceID))
	responses := make([]TemplateResponse, len(templates))
	for i, tmpl := range templates {
		responses[i] = templateToResponse(&tmpl)
	}

	rw.WriteSuccess(w, r, map[string]any{"templates": responses})
}

func (h *Handler) GetDeviceNewOverrides(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	overrides, err := h.ConfigService.ConfigurationSvc.GetDeviceOverrides(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	if overrides != nil {
		redactSecrets(overrides)
	}

	rw.WriteSuccess(w, r, map[string]any{"overrides": overrides})
}

func (h *Handler) SetDeviceNewOverrides(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	var overrides configuration.DeviceConfiguration
	if err := json.NewDecoder(r.Body).Decode(&overrides); err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.ConfigService.ConfigurationSvc.SetDeviceOverrides(uint(id), &overrides); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	savedOverrides, _ := h.ConfigService.ConfigurationSvc.GetDeviceOverrides(uint(id))
	desiredConfig, _, _ := h.ConfigService.ConfigurationSvc.GetDesiredConfig(uint(id))

	if savedOverrides != nil {
		redactSecrets(savedOverrides)
	}
	if desiredConfig != nil {
		redactSecrets(desiredConfig)
	}

	rw.WriteSuccess(w, r, map[string]any{
		"overrides":      savedOverrides,
		"desired_config": desiredConfig,
	})
}

func (h *Handler) PatchDeviceNewOverrides(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	var patch configuration.DeviceConfiguration
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.ConfigService.ConfigurationSvc.PatchDeviceOverrides(uint(id), &patch); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	overrides, _ := h.ConfigService.ConfigurationSvc.GetDeviceOverrides(uint(id))
	if overrides != nil {
		redactSecrets(overrides)
	}

	rw.WriteSuccess(w, r, map[string]any{"overrides": overrides})
}

func (h *Handler) DeleteDeviceNewOverrides(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	if err := h.ConfigService.ConfigurationSvc.ClearDeviceOverrides(uint(id)); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	rw.WriteNoContent(w, r)
}

func (h *Handler) GetDeviceDesiredConfig(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	config, sources, err := h.ConfigService.ConfigurationSvc.GetDesiredConfig(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	inherited, inheritedSources, err := h.ConfigService.ConfigurationSvc.GetInheritedConfig(uint(id))
	if err != nil {
		rw.WriteInternalError(w, r, err)
		return
	}

	if config != nil {
		redactSecrets(config)
	}
	maskSecrets(inherited)

	rw.WriteSuccess(w, r, DesiredConfigResponse{
		Config:           config,
		Sources:          sources,
		Inherited:        inherited,
		InheritedSources: inheritedSources,
	})
}

func (h *Handler) GetDeviceNewConfigStatus(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	status, err := h.ConfigService.ConfigurationSvc.GetConfigStatus(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	rw.WriteSuccess(w, r, ConfigStatusResponse{
		DeviceID:       status.DeviceID,
		ConfigApplied:  status.ConfigApplied,
		HasOverrides:   status.HasOverrides,
		TemplateCount:  status.TemplateCount,
		LastApplied:    status.LastUpdated.Format("2006-01-02T15:04:05Z"),
		PendingChanges: !status.ConfigApplied,
	})
}

func (h *Handler) ApplyDeviceNewConfig(w http.ResponseWriter, r *http.Request) {
	// Apply writes to the physical device (for example the MQTT broker), so it needs the admin key.
	if !h.requireAdmin(w, r) {
		return
	}
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	_, err = h.DB.GetDevice(uint(id))
	if err != nil {
		rw.WriteNotFoundError(w, r, "Device")
		return
	}

	result, err := h.Service.ApplyDesiredConfig(r.Context(), uint(id))
	if err != nil {
		h.writeConfigDeviceError(w, r, err)
		return
	}

	resp := ConfigApplyResponse{
		Success:  result.ConfigApplied,
		Failures: applyFailureMessages(result),
	}
	if result.ApplyResult != nil {
		resp.AppliedCount = result.ApplyResult.AppliedCount
		resp.FailedCount = result.ApplyResult.FailedCount
		resp.RequiresReboot = result.ApplyResult.RequiresReboot
		resp.Warnings = result.ApplyResult.Warnings
	}

	rw.WriteSuccess(w, r, resp)
}

// applyFailureMessages lists the settings the device refused and the read-back differences.
// A refused write names the setting only: the device's own error text can echo the value it
// was sent, such as the MQTT password, so it goes to the server log and never to the caller.
func applyFailureMessages(result *configuration.ApplyVerifyResult) []string {
	failures := []string{}
	if result.ApplyResult != nil {
		for _, f := range result.ApplyResult.Failures {
			failures = append(failures, f.Path+": the device refused the write")
		}
	}
	if result.VerifyResult != nil {
		for _, d := range result.VerifyResult.Differences {
			path, expected, actual := redactDifference(d)
			failures = append(failures,
				fmt.Sprintf("verify %s: expected %v, device has %v", path, expected, actual))
		}
	}
	return failures
}

// writeConfigDeviceError maps errors from talking to a device to API responses.
func (h *Handler) writeConfigDeviceError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, configuration.ErrUnsupportedGeneration) {
		h.responseWriter().WriteError(w, r, http.StatusNotImplemented, apiresp.ErrCodeNotImplemented, err.Error(), nil)
		return
	}
	if errors.Is(err, configuration.ErrUnsupportedSettings) {
		h.responseWriter().WriteError(w, r, http.StatusUnprocessableEntity, apiresp.ErrCodeValidationFailed, err.Error(), nil)
		return
	}
	h.writeDeviceReadError(w, r, err)
}

func (h *Handler) VerifyDeviceNewConfig(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	_, err = h.DB.GetDevice(uint(id))
	if err != nil {
		rw.WriteNotFoundError(w, r, "Device")
		return
	}

	result, err := h.Service.VerifyDesiredConfig(r.Context(), uint(id))
	if err != nil {
		h.writeConfigDeviceError(w, r, err)
		return
	}

	diffs := make([]ConfigDifferenceDTO, 0, len(result.Differences))
	for _, d := range result.Differences {
		path, expected, actual := redactDifference(d)
		diffs = append(diffs, ConfigDifferenceDTO{Path: path, Expected: expected, Actual: actual})
	}

	rw.WriteSuccess(w, r, ConfigVerifyResponse{Match: result.Match, Differences: diffs})
}

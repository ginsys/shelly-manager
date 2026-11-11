package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
)

// Template CRUD request/response types

// CreateTemplateRequest represents a request to create a new template
type CreateTemplateRequest struct {
	Name        string                             `json:"name"`
	Description string                             `json:"description,omitempty"`
	Scope       string                             `json:"scope"`
	DeviceType  string                             `json:"device_type,omitempty"`
	Config      *configuration.DeviceConfiguration `json:"config"`
}

// UpdateTemplateRequest represents a request to update a template
type UpdateTemplateRequest struct {
	Name        string                             `json:"name,omitempty"`
	Description string                             `json:"description,omitempty"`
	Config      *configuration.DeviceConfiguration `json:"config,omitempty"`
}

// TemplateResponse represents a template in API responses
type TemplateResponse struct {
	ID          uint                               `json:"id"`
	Name        string                             `json:"name"`
	Description string                             `json:"description,omitempty"`
	Scope       string                             `json:"scope"`
	DeviceType  string                             `json:"device_type,omitempty"`
	Config      *configuration.DeviceConfiguration `json:"config"`
	CreatedAt   string                             `json:"created_at"`
	UpdatedAt   string                             `json:"updated_at"`
	// Secrets redaction indicators
	HasWiFiPassword *bool `json:"has_wifi_password,omitempty"`
	HasMQTTPassword *bool `json:"has_mqtt_password,omitempty"`
	HasAuthPassword *bool `json:"has_auth_password,omitempty"`
}

// ListTemplatesResponse represents the response for listing templates
type ListTemplatesResponse struct {
	Templates []TemplateResponse `json:"templates"`
}

// GetNewConfigTemplates handles GET /api/v1/config/templates/new
// This endpoint uses the new ConfigurationService
func (h *Handler) GetNewConfigTemplates(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	// Get scope from query params
	scope := r.URL.Query().Get("scope")

	templates, err := h.ConfigService.ConfigurationSvc.ListTemplates(scope)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"scope":     scope,
			"component": "api",
		}).Error("Failed to list templates")
		rw.WriteInternalError(w, r, err)
		return
	}

	// Convert to response format with secret redaction
	responses := make([]TemplateResponse, 0, len(templates))
	for _, tmpl := range templates {
		resp := templateToResponse(&tmpl)
		responses = append(responses, resp)
	}

	rw.WriteSuccess(w, r, ListTemplatesResponse{Templates: responses})
}

// GetDeviceTypes handles GET /api/v1/config/device-types: the models a device-type template can name.
func (h *Handler) GetDeviceTypes(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.responseWriter().WriteSuccess(w, r, map[string]any{
		"device_types": configuration.SupportedDeviceTypeInfos(),
	})
}

// writeUnknownDeviceType refuses a device-type template that names a model outside the supported
// list, in the same field-error shape the config checks use. It returns true when it refused.
// The check lives here and not in ValidateTemplateScope, which the startup migration and import
// also run: they must never refuse a template that was stored before this check existed.
func writeUnknownDeviceType(rw *apiresp.ResponseWriter, w http.ResponseWriter, r *http.Request, deviceType string) bool {
	if configuration.IsSupportedDeviceType(deviceType) {
		return false
	}
	rw.WriteValidationError(w, r, map[string]any{"fields": []configuration.FieldError{{
		Path:    "device_type",
		Message: "Unknown device type " + deviceType + ". Pick one from GET /api/v1/config/device-types",
	}}})
	return true
}

// CreateNewConfigTemplate handles POST /api/v1/config/templates/new
func (h *Handler) CreateNewConfigTemplate(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	raw, head, err := readTemplateBody(r)
	if err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}
	// An empty type keeps the service's own refusal (ErrDeviceTypeRequired).
	if head.Scope == configuration.ScopeDeviceType && head.DeviceType != "" &&
		writeUnknownDeviceType(rw, w, r, head.DeviceType) {
		return
	}
	if writeConfigFieldErrors(rw, w, r, schemaDeviceType(head.Scope, head.DeviceType), raw, head,
		reflect.TypeOf(CreateTemplateRequest{})) {
		return
	}
	var req CreateTemplateRequest
	if err = json.Unmarshal(raw, &req); err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Name) == "" {
		rw.WriteValidationError(w, r, "name is required")
		return
	}
	if req.Scope == "" {
		rw.WriteValidationError(w, r, "scope is required (global, group, or device_type)")
		return
	}
	if req.Config == nil {
		rw.WriteValidationError(w, r, "config is required")
		return
	}

	// Marshal config to JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		rw.WriteInternalError(w, r, err)
		return
	}

	template := &configuration.ServiceConfigTemplate{
		Name:        req.Name,
		Description: req.Description,
		Scope:       req.Scope,
		DeviceType:  req.DeviceType,
		Config:      configJSON,
	}

	if err := h.ConfigService.ConfigurationSvc.CreateTemplate(template); err != nil {
		if errors.Is(err, configuration.ErrInvalidScope) {
			rw.WriteValidationError(w, r, err.Error())
			return
		}
		if errors.Is(err, configuration.ErrDeviceTypeRequired) {
			rw.WriteValidationError(w, r, err.Error())
			return
		}
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"name":      req.Name,
			"component": "api",
		}).Error("Failed to create template")
		rw.WriteInternalError(w, r, err)
		return
	}

	h.logger.WithFields(map[string]any{
		"template_id":   template.ID,
		"template_name": template.Name,
		"scope":         template.Scope,
		"component":     "api",
	}).Info("Template created via API")

	rw.WriteCreated(w, r, map[string]any{
		"template": templateToResponse(template),
	})
}

// GetNewConfigTemplate handles GET /api/v1/config/templates/new/{id}
func (h *Handler) GetNewConfigTemplate(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid template ID", nil)
		return
	}

	template, err := h.ConfigService.ConfigurationSvc.GetTemplate(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrTemplateNotFound) {
			rw.WriteNotFoundError(w, r, "Template")
			return
		}
		h.logger.WithFields(map[string]any{
			"error":       err.Error(),
			"template_id": id,
			"component":   "api",
		}).Error("Failed to get template")
		rw.WriteInternalError(w, r, err)
		return
	}

	rw.WriteSuccess(w, r, map[string]any{
		"template": templateToResponse(template),
	})
}

// UpdateNewConfigTemplate handles PUT /api/v1/config/templates/new/{id}
func (h *Handler) UpdateNewConfigTemplate(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid template ID", nil)
		return
	}

	// Get existing template
	existing, err := h.ConfigService.ConfigurationSvc.GetTemplate(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrTemplateNotFound) {
			rw.WriteNotFoundError(w, r, "Template")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	raw, head, err := readTemplateBody(r)
	if err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}
	// Only a type that differs from the stored one is checked, so a template stored with a type
	// outside the list stays editable. The update keeps the stored type either way.
	if existing.Scope == configuration.ScopeDeviceType && head.DeviceType != "" &&
		head.DeviceType != existing.DeviceType && writeUnknownDeviceType(rw, w, r, head.DeviceType) {
		return
	}
	if writeConfigFieldErrors(rw, w, r, schemaDeviceType(existing.Scope, existing.DeviceType), raw, head,
		reflect.TypeOf(UpdateTemplateRequest{})) {
		return
	}
	var req UpdateTemplateRequest
	if err = json.Unmarshal(raw, &req); err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	// Apply updates
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Config != nil {
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			rw.WriteInternalError(w, r, err)
			return
		}
		existing.Config = configJSON
	}

	if err := h.ConfigService.ConfigurationSvc.UpdateTemplate(existing); err != nil {
		h.logger.WithFields(map[string]any{
			"error":       err.Error(),
			"template_id": id,
			"component":   "api",
		}).Error("Failed to update template")
		rw.WriteInternalError(w, r, err)
		return
	}

	// Get count of affected devices
	affected, _ := h.ConfigService.ConfigurationSvc.GetAffectedDevices(uint(id))

	h.logger.WithFields(map[string]any{
		"template_id":      id,
		"template_name":    existing.Name,
		"affected_devices": len(affected),
		"component":        "api",
	}).Info("Template updated via API")

	rw.WriteSuccess(w, r, map[string]any{
		"template":         templateToResponse(existing),
		"affected_devices": len(affected),
	})
}

// DeleteNewConfigTemplate handles DELETE /api/v1/config/templates/new/{id}
func (h *Handler) DeleteNewConfigTemplate(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid template ID", nil)
		return
	}

	if err := h.ConfigService.ConfigurationSvc.DeleteTemplate(uint(id)); err != nil {
		if errors.Is(err, configuration.ErrTemplateNotFound) {
			rw.WriteNotFoundError(w, r, "Template")
			return
		}
		if errors.Is(err, configuration.ErrTemplateAssigned) {
			// Get the affected devices for the error details
			affected, _ := h.ConfigService.ConfigurationSvc.GetAffectedDevices(uint(id))
			rw.WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict,
				"Cannot delete template: assigned to devices",
				map[string]any{"device_count": len(affected)})
			return
		}
		h.logger.WithFields(map[string]any{
			"error":       err.Error(),
			"template_id": id,
			"component":   "api",
		}).Error("Failed to delete template")
		rw.WriteInternalError(w, r, err)
		return
	}

	h.logger.WithFields(map[string]any{
		"template_id": id,
		"component":   "api",
	}).Info("Template deleted via API")

	rw.WriteNoContent(w, r)
}

// maxTemplateBodyBytes bounds a template create or update request body.
const maxTemplateBodyBytes = 1 << 20

// templateHead holds the fields of a template request that schema validation needs. Config is
// the raw body, before DeviceConfiguration decoding drops keys or refuses a wrong type.
type templateHead struct {
	Scope      string                 `json:"scope"`
	DeviceType string                 `json:"device_type"`
	Config     map[string]interface{} `json:"config"`
	// RawConfig is the config value as sent, for the key check.
	RawConfig json.RawMessage `json:"-"`
}

// readTemplateBody reads a template request body. It returns the raw bytes, for the typed
// request, and its head, for schema validation.
func readTemplateBody(r *http.Request) ([]byte, templateHead, error) {
	var head templateHead
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxTemplateBodyBytes+1))
	if err != nil {
		return nil, head, err
	}
	if len(raw) > maxTemplateBodyBytes {
		return nil, head, errors.New("request body too large")
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, head, err
	}
	var rawConfig struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(raw, &rawConfig); err != nil {
		return nil, head, err
	}
	head.RawConfig = rawConfig.Config
	return raw, head, nil
}

// schemaDeviceType names the device schema a template body is checked against: the template's
// model for a device_type template, every section for a global or group template.
func schemaDeviceType(scope, deviceType string) string {
	if scope == configuration.ScopeDeviceType {
		return deviceType
	}
	return ""
}

// writeConfigFieldErrors checks a template request body. It refuses a repeated or miscased
// request key, then the keys of the config against the stored configuration, which is the body
// that CreateTemplateRequest and UpdateTemplateRequest decode and store, then the values of
// config against the device schema of deviceType. If anything fails, it writes a validation
// error whose details list each field, and returns true.
func writeConfigFieldErrors(rw *apiresp.ResponseWriter, w http.ResponseWriter, r *http.Request,
	deviceType string, raw []byte, head templateHead, request reflect.Type) bool {
	fieldErrs := configuration.CheckRequestKeys(raw, request)
	fieldErrs = append(fieldErrs, configuration.CheckConfigKeys(head.RawConfig)...)
	fieldErrs = append(fieldErrs, configuration.ValidateConfigBody(deviceType, head.Config)...)
	if len(fieldErrs) == 0 {
		return false
	}
	sort.SliceStable(fieldErrs, func(i, j int) bool { return fieldErrs[i].Path < fieldErrs[j].Path })
	rw.WriteValidationError(w, r, map[string]any{"fields": fieldErrs})
	return true
}

// templateToResponse converts a ServiceConfigTemplate to TemplateResponse
// This function handles secret redaction
func templateToResponse(tmpl *configuration.ServiceConfigTemplate) TemplateResponse {
	resp := TemplateResponse{
		ID:          tmpl.ID,
		Name:        tmpl.Name,
		Description: tmpl.Description,
		Scope:       tmpl.Scope,
		DeviceType:  tmpl.DeviceType,
		CreatedAt:   tmpl.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   tmpl.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	// Parse config and redact secrets
	if len(tmpl.Config) > 0 {
		var config configuration.DeviceConfiguration
		if err := json.Unmarshal(tmpl.Config, &config); err == nil {
			// Check for secrets and set indicators
			hasWiFiPass := hasWiFiPassword(&config)
			hasMQTTPass := hasMQTTPassword(&config)
			hasAuthPass := hasAuthPassword(&config)

			if hasWiFiPass {
				resp.HasWiFiPassword = &hasWiFiPass
			}
			if hasMQTTPass {
				resp.HasMQTTPassword = &hasMQTTPass
			}
			if hasAuthPass {
				resp.HasAuthPassword = &hasAuthPass
			}

			// Redact secrets before returning
			redactSecrets(&config)
			resp.Config = &config
		}
	}

	return resp
}

func hasWiFiPassword(config *configuration.DeviceConfiguration) bool {
	if config.WiFi == nil {
		return false
	}
	if config.WiFi.Password != nil && *config.WiFi.Password != "" {
		return true
	}
	if config.WiFi.AccessPoint != nil && config.WiFi.AccessPoint.Password != nil && *config.WiFi.AccessPoint.Password != "" {
		return true
	}
	return false
}

// hasMQTTPassword checks if config has MQTT password set
func hasMQTTPassword(config *configuration.DeviceConfiguration) bool {
	if config.MQTT == nil {
		return false
	}
	return config.MQTT.Password != nil && *config.MQTT.Password != ""
}

// hasAuthPassword checks if config has auth password set
func hasAuthPassword(config *configuration.DeviceConfiguration) bool {
	if config.Auth == nil {
		return false
	}
	return config.Auth.Password != nil && *config.Auth.Password != ""
}

// secretMask stands in for a secret that is set. It lets a client tell "has a password" from "has
// none" (a form that requires one) without ever receiving the value.
const secretMask = "********"

// secretSlot reads and writes one secret of a configuration. The fields differ in type (a pointer
// in most sections, a plain string in the Network part), so a slot hides that. Setting "" clears it.
type secretSlot struct {
	get func() string
	set func(string)
}

func ptrSlot(p **string) secretSlot {
	return secretSlot{
		get: func() string {
			if *p == nil {
				return ""
			}
			return **p
		},
		set: func(v string) {
			if v == "" {
				*p = nil
				return
			}
			*p = &v
		},
	}
}

func strSlot(p *string) secretSlot {
	return secretSlot{get: func() string { return *p }, set: func(v string) { *p = v }}
}

// secretSlots lists every secret of a configuration by its JSON path. A section that is absent has
// no slot. Redacting, masking and restoring all walk this one list, so a new secret field is added
// in one place, and secret_invariant_test.go fails for a field named like a secret that is not here.
func secretSlots(config *configuration.DeviceConfiguration) map[string]secretSlot {
	slots := map[string]secretSlot{}
	if config == nil {
		return slots
	}
	if config.WiFi != nil {
		slots["wifi.password"] = ptrSlot(&config.WiFi.Password)
		if ap := config.WiFi.AccessPoint; ap != nil {
			slots["wifi.ap.pass"] = ptrSlot(&ap.Password)
			slots["wifi.ap.key"] = ptrSlot(&ap.Key)
		}
	}
	if config.MQTT != nil {
		slots["mqtt.pass"] = ptrSlot(&config.MQTT.Password)
	}
	if config.Auth != nil {
		slots["auth.pass"] = ptrSlot(&config.Auth.Password)
	}
	if config.Network != nil && config.Network.WiFi != nil {
		if sta := config.Network.WiFi.STA; sta != nil {
			slots["network.wifi.sta.pass"] = strSlot(&sta.Password)
		}
		if ap := config.Network.WiFi.AP; ap != nil {
			slots["network.wifi.ap.pass"] = strSlot(&ap.Password)
		}
	}
	return slots
}

// maskSecrets replaces each secret that is set with secretMask. A secret that is absent or empty
// stays as it is.
func maskSecrets(config *configuration.DeviceConfiguration) {
	for _, slot := range secretSlots(config) {
		if slot.get() != "" {
			slot.set(secretMask)
		}
	}
}

// redactSecrets removes every secret.
func redactSecrets(config *configuration.DeviceConfiguration) {
	for _, slot := range secretSlots(config) {
		slot.set("")
	}
}

// restoreMaskedSecrets answers a client that sends back what maskSecrets gave it. A secret that
// arrives as the mask becomes the stored value, or nothing when none is stored: the mask itself is
// never stored. Any other value is a new secret and stays as sent.
func restoreMaskedSecrets(incoming, stored *configuration.DeviceConfiguration) {
	storedSlots := secretSlots(stored)
	for path, slot := range secretSlots(incoming) {
		if slot.get() != secretMask {
			continue
		}
		kept := ""
		if old, ok := storedSlots[path]; ok {
			kept = old.get()
		}
		slot.set(kept)
	}
}

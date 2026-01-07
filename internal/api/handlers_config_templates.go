package api

import (
	"encoding/json"
	"errors"
	"net/http"
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

// CreateNewConfigTemplate handles POST /api/v1/config/templates/new
func (h *Handler) CreateNewConfigTemplate(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	var req UpdateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

func redactSecrets(config *configuration.DeviceConfiguration) {
	if config.WiFi != nil {
		config.WiFi.Password = nil
		if config.WiFi.AccessPoint != nil {
			config.WiFi.AccessPoint.Password = nil
		}
	}
	if config.MQTT != nil {
		config.MQTT.Password = nil
	}
	if config.Auth != nil {
		config.Auth.Password = nil
	}
}

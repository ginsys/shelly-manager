package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/service"
)

// TypedConfigurationRequest represents a typed configuration request
type TypedConfigurationRequest struct {
	Configuration   *configuration.TypedConfiguration `json:"configuration"`
	ValidationLevel string                            `json:"validation_level,omitempty"` // basic, strict, production
	DeviceModel     string                            `json:"device_model,omitempty"`
	Generation      int                               `json:"generation,omitempty"`
	Capabilities    []string                          `json:"capabilities,omitempty"`
}

// TypedConfigurationResponse represents a typed configuration response
type TypedConfigurationResponse struct {
	Configuration    *configuration.TypedConfiguration `json:"configuration"`
	ValidationResult *configuration.ValidationResult   `json:"validation,omitempty"`
	ConversionInfo   *ConfigurationConversionInfo      `json:"conversion_info,omitempty"`
}

// ConfigurationConversionInfo provides information about configuration conversion
type ConfigurationConversionInfo struct {
	HasTypedConfig bool     `json:"has_typed_config"`
	HasRawConfig   bool     `json:"has_raw_config"`
	ConvertedFrom  string   `json:"converted_from,omitempty"` // "raw" or "typed"
	Warnings       []string `json:"warnings,omitempty"`
}

// GetTypedDeviceConfig handles GET /api/v1/devices/{id}/config/typed
func (h *Handler) GetTypedDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Get device info for validation context
	device, err := h.DB.GetDevice(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.responseWriter().WriteNotFoundError(w, r, "Device")
		} else {
			h.responseWriter().WriteInternalError(w, r, err)
		}
		return
	}

	// Get device configuration (may be raw JSON or typed)
	rawConfig, err := h.Service.GetDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	// Convert to typed configuration
	typedConfig, conversionInfo, err := h.convertToTypedConfig(rawConfig.Config, device)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to convert to typed config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to convert configuration: %v", err),
		})
		return
	}

	// Validate the configuration
	validationLevel := r.URL.Query().Get("validation_level")
	if validationLevel == "" {
		validationLevel = "basic"
	}

	validator := h.createValidator(validationLevel, device)
	configJSON, _ := typedConfig.ToJSON()
	validationResult := validator.ValidateConfiguration(configJSON)

	// The configuration was validated with its real values, and is sent masked, with the template
	// fragments that the validation messages quote masked too.
	scrubValidation(validationResult)
	maskTypedSecrets(typedConfig)

	response := TypedConfigurationResponse{
		Configuration:    typedConfig,
		ValidationResult: validationResult,
		ConversionInfo:   conversionInfo,
	}

	// Wrap response with success for frontend compatibility
	h.responseWriter().WriteSuccess(w, r, response)
}

// GetTypedDeviceConfigNormalized handles GET /api/v1/devices/{id}/config/typed/normalized
// Returns the saved typed configuration in normalized format for comparison
func (h *Handler) GetTypedDeviceConfigNormalized(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "Invalid device ID",
		})
		return
	}

	// Get device info for validation context
	device, err := h.DB.GetDevice(uint(id))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == gorm.ErrRecordNotFound {
			w.WriteHeader(http.StatusNotFound)
			h.writeJSON(w, map[string]interface{}{
				"success": false,
				"error":   "Device not found",
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			h.writeJSON(w, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}
		return
	}

	// Get device configuration (may be raw JSON or typed)
	rawConfig, err := h.Service.GetDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device config for normalization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Convert to typed configuration
	typedConfig, _, err := h.convertToTypedConfig(rawConfig.Config, device)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to convert to typed config for normalization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to convert configuration: %v", err),
		})
		return
	}

	// Normalize the typed configuration, with its secrets masked
	maskTypedSecrets(typedConfig)
	normalizer := NewConfigNormalizer()
	normalized := normalizer.NormalizeTypedConfig(typedConfig)

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, map[string]interface{}{
		"success":       true,
		"configuration": normalized,
	})
}

// UpdateTypedDeviceConfig handles PUT /api/v1/devices/{id}/config/typed
func (h *Handler) UpdateTypedDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Decode request. The raw body is kept: a decoded struct cannot tell a secret sent as null
	// from one left out.
	raw, readErr := io.ReadAll(r.Body)
	var req TypedConfigurationRequest
	if readErr != nil || json.Unmarshal(raw, &req) != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Configuration == nil {
		http.Error(w, "Configuration is required", http.StatusBadRequest)
		return
	}

	// Get device info for validation context
	device, err := h.DB.GetDevice(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Device not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// The client read the configuration masked: a mask or an omitted secret keeps the stored
	// value, and only an explicit "" or null removes it. This happens before the configuration is
	// validated and stored.
	removed, nullErr := wrappedNullSecretPaths(raw, "configuration")
	if nullErr != nil {
		http.Error(w, "Invalid JSON: "+nullErr.Error(), http.StatusBadRequest)
		return
	}
	if restoreErr := restoreTypedSecrets(req.Configuration, h.storedTypedConfig(uint(id), device), removed); restoreErr != nil {
		http.Error(w, restoreErr.Error(), http.StatusBadRequest)
		return
	}

	// Validate the configuration
	validationLevel := req.ValidationLevel
	if validationLevel == "" {
		validationLevel = "basic"
	}

	validator := h.createValidator(validationLevel, device)
	configJSON, err := req.Configuration.ToJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize configuration: %v", err), http.StatusInternalServerError)
		return
	}

	validationResult := validator.ValidateConfiguration(configJSON)
	scrubValidation(validationResult)
	if !validationResult.Valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		h.writeJSON(w, map[string]interface{}{
			"error":      "Configuration validation failed",
			"validation": validationResult,
		})
		return
	}

	// Update device configuration
	err = h.Service.ConfigSvc.UpdateDeviceConfigFromJSON(uint(id), configJSON)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update device config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// The stored configuration is the one just written: the answer carries it masked.
	maskTypedSecrets(req.Configuration)
	response := TypedConfigurationResponse{
		Configuration:    req.Configuration,
		ValidationResult: validationResult,
	}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, response)
}

// ValidateTypedConfig handles POST /api/v1/configuration/validate-typed
func (h *Handler) ValidateTypedConfig(w http.ResponseWriter, r *http.Request) {
	var req TypedConfigurationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Configuration == nil {
		http.Error(w, "Configuration is required", http.StatusBadRequest)
		return
	}

	// Create validator with provided context
	validationLevel := req.ValidationLevel
	if validationLevel == "" {
		validationLevel = "basic"
	}

	var validator *configuration.ConfigurationValidator
	if req.DeviceModel != "" {
		// Create mock device for validation context
		device := &database.Device{
			Type: req.DeviceModel,
		}
		if req.Generation > 0 {
			device.Firmware = fmt.Sprintf("v%d.0.0", req.Generation)
		}
		validator = h.createValidator(validationLevel, device)
	} else {
		validator = h.createGenericValidator(validationLevel, req.Capabilities)
	}

	// Validate the configuration
	configJSON, err := req.Configuration.ToJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize configuration: %v", err), http.StatusInternalServerError)
		return
	}

	validationResult := validator.ValidateConfiguration(configJSON)

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, validationResult)
}

// ConvertConfigToTyped handles POST /api/v1/configuration/convert-to-typed
func (h *Handler) ConvertConfigToTyped(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Configuration json.RawMessage `json:"configuration"`
		DeviceModel   string          `json:"device_model,omitempty"`
		Generation    int             `json:"generation,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Create mock device for conversion context
	device := &database.Device{}
	if req.DeviceModel != "" {
		device.Type = req.DeviceModel
	}
	if req.Generation > 0 {
		device.Firmware = fmt.Sprintf("v%d.0.0", req.Generation)
	}

	// Convert to typed configuration
	typedConfig, conversionInfo, err := h.convertToTypedConfig(req.Configuration, device)
	if err != nil {
		h.responseWriter().WriteInternalError(w, r, fmt.Errorf("failed to convert configuration: %w", err))
		return
	}

	response := TypedConfigurationResponse{
		Configuration:  typedConfig,
		ConversionInfo: conversionInfo,
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// ConvertTypedToRaw handles POST /api/v1/configuration/convert-to-raw
func (h *Handler) ConvertTypedToRaw(w http.ResponseWriter, r *http.Request) {
	var req TypedConfigurationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if req.Configuration == nil {
		h.responseWriter().WriteValidationError(w, r, "Configuration is required")
		return
	}

	// Convert to raw JSON
	rawJSON, err := req.Configuration.ToJSON()
	if err != nil {
		h.responseWriter().WriteInternalError(w, r, fmt.Errorf("failed to convert to raw JSON: %w", err))
		return
	}

	response := map[string]interface{}{
		"configuration": rawJSON,
		"conversion_info": ConfigurationConversionInfo{
			HasTypedConfig: true,
			HasRawConfig:   true,
			ConvertedFrom:  "typed",
		},
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// GetConfigurationSchema handles GET /api/v1/config/schema.
//
// With ?device_type=<model> or ?device_id=<id> it returns the schema of the configuration stored
// for that model (configuration.GetDeviceConfigurationSchema): only the model's sections, only
// stored fields, and each field marked x-applicable when apply can write it. device_id resolves
// the model the way apply does. scope=global or scope=group returns the schema of such a template
// (configuration.GetTemplateConfigurationSchema). Without a parameter it returns the generic
// schema. A parameter that is present but empty, a malformed query string, or more than one of
// the three parameters is a 400.
func (h *Handler) GetConfigurationSchema(w http.ResponseWriter, r *http.Request) {
	// r.URL.Query() drops a malformed pair without a word, so "?device_id=%ZZ" would read as no
	// parameter and return the generic schema. Parse the raw query and refuse it instead.
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest,
			"Invalid query string", nil)
		return
	}
	deviceType := query.Get("device_type")

	if query.Has("device_type") && deviceType == "" {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest,
			"device_type must not be empty", nil)
		return
	}
	if query.Has("scope") {
		scope := query.Get("scope")
		if query.Has("device_type") || query.Has("device_id") {
			h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest,
				"Use scope, device_type or device_id, not more than one", nil)
			return
		}
		if scope != configuration.ScopeGlobal && scope != configuration.ScopeGroup {
			h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest,
				"scope must be global or group; a device_type template uses device_type", nil)
			return
		}
		h.responseWriter().WriteSuccess(w, r, configuration.GetTemplateConfigurationSchema())
		return
	}
	if query.Has("device_id") {
		idParam := query.Get("device_id")
		if query.Has("device_type") {
			h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest,
				"Use device_type or device_id, not both", nil)
			return
		}
		id, err := strconv.ParseUint(idParam, 10, 32)
		if err != nil {
			h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
			return
		}
		device, err := h.DB.GetDevice(uint(id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				h.responseWriter().WriteNotFoundError(w, r, "Device")
			} else {
				h.responseWriter().WriteInternalError(w, r, err)
			}
			return
		}
		deviceType = service.DeviceModel(device)
		if deviceType == "" {
			// An empty model would read as "no parameter" below and return the generic schema,
			// whose fields are not all stored. A device without a model gets the unsupported-model
			// schema instead.
			deviceType = "unknown"
		}
	}

	if deviceType == "" {
		h.responseWriter().WriteSuccess(w, r, configuration.GetConfigurationSchema())
		return
	}
	h.responseWriter().WriteSuccess(w, r, configuration.GetDeviceConfigurationSchema(deviceType))
}

// BulkValidateConfigs handles POST /api/v1/configuration/bulk-validate
func (h *Handler) BulkValidateConfigs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Configurations  []TypedConfigurationRequest `json:"configurations"`
		ValidationLevel string                      `json:"validation_level,omitempty"`
		StopOnError     bool                        `json:"stop_on_error,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if len(req.Configurations) == 0 {
		h.responseWriter().WriteValidationError(w, r, "No configurations provided")
		return
	}

	validationLevel := req.ValidationLevel
	if validationLevel == "" {
		validationLevel = "basic"
	}

	results := make([]struct {
		Index            int                             `json:"index"`
		ValidationResult *configuration.ValidationResult `json:"validation"`
		Error            string                          `json:"error,omitempty"`
	}, 0, len(req.Configurations))

	for i, configReq := range req.Configurations {
		if configReq.Configuration == nil {
			results = append(results, struct {
				Index            int                             `json:"index"`
				ValidationResult *configuration.ValidationResult `json:"validation"`
				Error            string                          `json:"error,omitempty"`
			}{
				Index: i,
				Error: "Configuration is required",
			})

			if req.StopOnError {
				break
			}
			continue
		}

		// Create validator
		var validator *configuration.ConfigurationValidator
		if configReq.DeviceModel != "" {
			device := &database.Device{Type: configReq.DeviceModel}
			if configReq.Generation > 0 {
				device.Firmware = fmt.Sprintf("v%d.0.0", configReq.Generation)
			}
			validator = h.createValidator(validationLevel, device)
		} else {
			validator = h.createGenericValidator(validationLevel, configReq.Capabilities)
		}

		// Validate
		configJSON, err := configReq.Configuration.ToJSON()
		if err != nil {
			results = append(results, struct {
				Index            int                             `json:"index"`
				ValidationResult *configuration.ValidationResult `json:"validation"`
				Error            string                          `json:"error,omitempty"`
			}{
				Index: i,
				Error: fmt.Sprintf("Failed to serialize configuration: %v", err),
			})

			if req.StopOnError {
				break
			}
			continue
		}

		validationResult := validator.ValidateConfiguration(configJSON)
		results = append(results, struct {
			Index            int                             `json:"index"`
			ValidationResult *configuration.ValidationResult `json:"validation"`
			Error            string                          `json:"error,omitempty"`
		}{
			Index:            i,
			ValidationResult: validationResult,
		})

		if req.StopOnError && !validationResult.Valid {
			break
		}
	}

	// Calculate summary
	valid := 0
	invalid := 0
	for _, result := range results {
		if result.Error != "" || (result.ValidationResult != nil && !result.ValidationResult.Valid) {
			invalid++
		} else {
			valid++
		}
	}

	response := map[string]interface{}{
		"summary": map[string]interface{}{
			"total":   len(req.Configurations),
			"valid":   valid,
			"invalid": invalid,
		},
		"results": results,
	}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, response)
}

// GetDeviceCapabilities handles GET /api/v1/devices/{id}/capabilities
func (h *Handler) GetDeviceCapabilities(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Get device from database
	device, err := h.DB.GetDevice(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Device not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Extract model and generation from device settings
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(device.Settings), &settings); err != nil {
		http.Error(w, "Invalid device settings", http.StatusInternalServerError)
		return
	}

	model := device.Type // fallback to device type
	if modelStr, ok := settings["model"].(string); ok && modelStr != "" {
		model = modelStr
	}

	generation := h.extractGeneration(device.Firmware)
	if genFloat, ok := settings["gen"].(float64); ok {
		generation = int(genFloat)
	}

	capabilities := h.getDeviceCapabilities(model, generation)

	response := struct {
		DeviceID     uint     `json:"device_id"`
		DeviceModel  string   `json:"device_model"`
		Generation   int      `json:"generation"`
		Capabilities []string `json:"capabilities"`
	}{
		DeviceID:     device.ID,
		DeviceModel:  model,
		Generation:   generation,
		Capabilities: capabilities,
	}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, response)
}

// Helper methods

// convertToTypedConfig converts raw JSON configuration to typed configuration
func (h *Handler) convertToTypedConfig(rawConfig json.RawMessage, device *database.Device) (*configuration.TypedConfiguration, *ConfigurationConversionInfo, error) {
	conversionInfo := &ConfigurationConversionInfo{
		HasRawConfig: len(rawConfig) > 0,
	}

	// Try to parse as typed configuration first
	typedConfig, err := configuration.FromJSON(rawConfig)
	if err == nil {
		// Successfully parsed as typed config
		conversionInfo.HasTypedConfig = true
		return typedConfig, conversionInfo, nil
	}

	// If typed parsing failed, try to extract known sections from raw JSON
	var rawData map[string]interface{}
	if err := json.Unmarshal(rawConfig, &rawData); err != nil {
		return nil, conversionInfo, fmt.Errorf("invalid JSON configuration: %w", err)
	}

	// Convert from raw JSON to typed configuration
	typedConfig = &configuration.TypedConfiguration{}
	conversionInfo.ConvertedFrom = "raw"
	conversionInfo.Warnings = []string{}

	// Convert WiFi settings - handle both unified "wifi" and separate "wifi_sta"/"wifi_ap" structures
	wifiCombined := make(map[string]interface{})

	// Check for unified WiFi configuration
	if wifiData, ok := rawData["wifi"].(map[string]interface{}); ok {
		for k, v := range wifiData {
			wifiCombined[k] = v
		}
	}

	// Check for separate wifi_sta configuration
	if staData, ok := rawData["wifi_sta"].(map[string]interface{}); ok {
		wifiCombined["wifi_sta"] = staData
	}

	// Check for separate wifi_ap configuration
	if apData, ok := rawData["wifi_ap"].(map[string]interface{}); ok {
		wifiCombined["wifi_ap"] = apData
	}

	// Convert combined WiFi data if we have any
	if len(wifiCombined) > 0 {
		wifi, warnings := h.convertWiFiConfig(wifiCombined)
		if wifi != nil {
			typedConfig.WiFi = wifi
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert MQTT settings
	if mqttData, ok := rawData["mqtt"].(map[string]interface{}); ok {
		mqtt, warnings := h.convertMQTTConfig(mqttData)
		if mqtt != nil {
			typedConfig.MQTT = mqtt
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert Auth settings - handle both "auth" and "login" (Shelly devices use "login")
	var authData map[string]interface{}
	if data, ok := rawData["auth"].(map[string]interface{}); ok {
		authData = data
	} else if data, ok := rawData["login"].(map[string]interface{}); ok {
		authData = data
	}

	if authData != nil {
		auth, warnings := h.convertAuthConfig(authData)
		if auth != nil {
			typedConfig.Auth = auth
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert System settings - Shelly devices store most settings at root level
	system, warnings := h.convertSystemConfig(rawData)
	if system != nil {
		typedConfig.System = system
		conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
	}

	// Convert Cloud settings
	if cloudData, ok := rawData["cloud"].(map[string]interface{}); ok {
		cloud, warnings := h.convertCloudConfig(cloudData)
		if cloud != nil {
			typedConfig.Cloud = cloud
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert CoIoT settings
	if coiotData, ok := rawData["coiot"].(map[string]interface{}); ok {
		coiot, warnings := h.convertCoIoTConfig(coiotData)
		if coiot != nil {
			typedConfig.CoIoT = coiot
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Extract model from device settings for accurate capability detection
	var settings map[string]interface{}
	model := device.Type // fallback to device type
	generation := h.extractGeneration(device.Firmware)

	if err := json.Unmarshal([]byte(device.Settings), &settings); err == nil {
		if modelStr, ok := settings["model"].(string); ok && modelStr != "" {
			model = modelStr
		}
		if genFloat, ok := settings["gen"].(float64); ok {
			generation = int(genFloat)
		}
	}

	// Convert capability-specific configurations based on device model
	deviceCapabilities := h.getDeviceCapabilities(model, generation)

	// Convert Relay configuration
	if contains(deviceCapabilities, "relay") {
		if relay, warnings := h.convertRelayConfig(rawData, device.Type); relay != nil {
			typedConfig.Relay = relay
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert Power Metering configuration
	if contains(deviceCapabilities, "power_metering") {
		if power, warnings := h.convertPowerMeteringConfig(rawData); power != nil {
			typedConfig.PowerMetering = power
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert Dimming configuration
	if contains(deviceCapabilities, "dimming") {
		if dimming, warnings := h.convertDimmingConfig(rawData); dimming != nil {
			typedConfig.Dimming = dimming
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert Roller configuration
	if contains(deviceCapabilities, "roller") {
		if roller, warnings := h.convertRollerConfig(rawData); roller != nil {
			typedConfig.Roller = roller
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert Input configuration
	if contains(deviceCapabilities, "input") {
		if input, warnings := h.convertInputConfig(rawData); input != nil {
			typedConfig.Input = input
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert LED configuration
	if contains(deviceCapabilities, "led") {
		if led, warnings := h.convertLEDConfig(rawData); led != nil {
			typedConfig.LED = led
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert Color configuration
	if contains(deviceCapabilities, "color") || contains(deviceCapabilities, "rgbw") {
		if color, warnings := h.convertColorConfig(rawData); color != nil {
			typedConfig.Color = color
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Store unconverted settings in Raw field
	filteredRaw := make(map[string]interface{})
	knownSections := map[string]bool{
		"wifi": true, "wifi_sta": true, "wifi_ap": true, "mqtt": true, "auth": true, "login": true,
		"sys": true, "cloud": true, "device": true, "name": true, "tz": true, "timezone": true, "sntp": true,
		"lat": true, "lng": true, "discoverable": true, "eco_mode": true, "coiot": true,
		// Capability-specific sections
		"relay": true, "relays": true, "relay_0": true, "relay_1": true,
		"light": true, "lights": true, "light_0": true, "light_1": true,
		"meter": true, "meters": true, "meter_0": true, "meter_1": true,
		"roller": true, "rollers": true, "roller_0": true,
		"input": true, "inputs": true, "input_0": true, "input_1": true, "input_2": true,
		"led": true, "led_status_disable": true, "led_power_disable": true,
		"color": true, "white": true, "effect": true, "effects": true,
		"schedule": true, "schedules": true, "actions": true,
		"ext_power": true, "ext_sensors": true, "temperature": true, "overtemp": true,
		"max_power": true, "longpush_time": true, "multipush_time": true,
		"mode": true, "default_state": true, "btn_type": true, "swap": true,
		"calibrated": true, "positioning": true, "safety_switch": true, "obstacle_mode": true,
		"fade_rate": true, "brightness": true, "transition": true, "night_mode": true,
	}

	for key, value := range rawData {
		if !knownSections[key] {
			filteredRaw[key] = value
		}
	}

	if len(filteredRaw) > 0 {
		rawJSON, _ := json.Marshal(filteredRaw)
		typedConfig.Raw = rawJSON
		conversionInfo.Warnings = append(conversionInfo.Warnings,
			fmt.Sprintf("Unconverted settings stored in raw field: %v", strings.Join(getKeys(filteredRaw), ", ")))
	}

	conversionInfo.HasTypedConfig = true
	return typedConfig, conversionInfo, nil
}

// createValidator creates a configuration validator for the given device
func (h *Handler) createValidator(validationLevel string, device *database.Device) *configuration.ConfigurationValidator {
	var level configuration.ValidationLevel
	switch validationLevel {
	case "strict":
		level = configuration.ValidationLevelStrict
	case "production":
		level = configuration.ValidationLevelProduction
	default:
		level = configuration.ValidationLevelBasic
	}

	deviceModel := device.Type
	generation := h.extractGeneration(device.Firmware)
	capabilities := h.getDeviceCapabilities(device.Type, generation)

	return configuration.NewConfigurationValidator(level, deviceModel, generation, capabilities)
}

// createGenericValidator creates a generic configuration validator
func (h *Handler) createGenericValidator(validationLevel string, capabilities []string) *configuration.ConfigurationValidator {
	var level configuration.ValidationLevel
	switch validationLevel {
	case "strict":
		level = configuration.ValidationLevelStrict
	case "production":
		level = configuration.ValidationLevelProduction
	default:
		level = configuration.ValidationLevelBasic
	}

	if capabilities == nil {
		capabilities = []string{"wifi", "mqtt"} // Default capabilities
	}

	return configuration.NewConfigurationValidator(level, "generic", 2, capabilities)
}

// extractGeneration extracts generation from firmware version
func (h *Handler) extractGeneration(firmware string) int {
	if strings.Contains(firmware, "v1.") || strings.Contains(firmware, "1.") {
		return 1
	}
	return 2 // Default to Gen2+
}

// getDeviceCapabilities returns device capabilities based on model and generation
func (h *Handler) getDeviceCapabilities(model string, generation int) []string {
	capabilities := []string{"wifi"}

	// Every generation has MQTT, cloud and login settings. Gen1 writes the first two through
	// /settings/mqtt and /settings/cloud, and the device login through /settings/login.
	capabilities = append(capabilities, "mqtt", "cloud", "auth")

	// Model-specific capabilities
	switch {
	case strings.Contains(model, "SHSW"):
		capabilities = append(capabilities, "relay", "power_metering")
	case strings.Contains(model, "SHDM"):
		capabilities = append(capabilities, "dimming", "power_metering")
	case strings.Contains(model, "SHPLG"):
		capabilities = append(capabilities, "relay", "power_metering")
	case strings.Contains(model, "SHRGBW"):
		capabilities = append(capabilities, "rgbw", "dimming")
	case strings.Contains(model, "SHHT"):
		capabilities = append(capabilities, "humidity", "temperature")
	case strings.Contains(model, "SHIX3"):
		capabilities = append(capabilities, "input")
	}

	if configuration.HasRollerMode(model) {
		capabilities = append(capabilities, "roller")
	}

	// Generation-specific capabilities
	if generation >= 2 {
		capabilities = append(capabilities, "ble", "ethernet")
	}

	return capabilities
}

// Helper conversion functions

// passwordField reads a password under its typed name, or under the device's own name where the
// typed name is not in the map. A typed name that is present wins even when its value is not a
// string, so a null "pass" never lets the device spelling take its place.
func passwordField(data map[string]interface{}, typed, device string) (string, bool) {
	if v, present := data[typed]; present {
		s, ok := v.(string)
		return s, ok
	}
	s, ok := data[device].(string)
	return s, ok
}

func (h *Handler) convertWiFiConfig(data map[string]interface{}) (*configuration.WiFiConfiguration, []string) {
	wifi := &configuration.WiFiConfiguration{}
	var warnings []string

	if staData, ok := data["wifi_sta"].(map[string]interface{}); ok {
		if enable, ok := staData["enable"].(bool); ok {
			wifi.Enable = configuration.BoolPtr(enable)
		} else if enabled, ok := staData["enabled"].(bool); ok {
			wifi.Enable = configuration.BoolPtr(enabled)
		}
		if ssid, ok := staData["ssid"].(string); ok {
			wifi.SSID = configuration.StringPtr(ssid)
		}
		// The device spells the station password "key"; "pass" wins where a configuration has both.
		if pass, ok := passwordField(staData, "pass", "key"); ok {
			wifi.Password = configuration.StringPtr(pass)
		}
		if ipv4mode, ok := staData["ipv4mode"].(string); ok {
			wifi.IPv4Mode = configuration.StringPtr(ipv4mode)
		} else if ipv4method, ok := staData["ipv4_method"].(string); ok {
			wifi.IPv4Mode = configuration.StringPtr(ipv4method)
		}

		if staticData, ok := staData["ip"].(map[string]interface{}); ok {
			static := &configuration.StaticIPConfig{}
			if ip, ok := staticData["ip"].(string); ok {
				static.IP = configuration.StringPtr(ip)
			}
			if netmask, ok := staticData["netmask"].(string); ok {
				static.Netmask = configuration.StringPtr(netmask)
			}
			if gw, ok := staticData["gw"].(string); ok {
				static.Gateway = configuration.StringPtr(gw)
			}
			if dns, ok := staticData["nameserver"].(string); ok {
				static.Nameserver = configuration.StringPtr(dns)
			}
			wifi.StaticIP = static
		}
	} else {
		if enable, ok := data["enable"].(bool); ok {
			wifi.Enable = configuration.BoolPtr(enable)
		}
		if ssid, ok := data["ssid"].(string); ok {
			wifi.SSID = configuration.StringPtr(ssid)
		}
		if pass, ok := data["pass"].(string); ok {
			wifi.Password = configuration.StringPtr(pass)
		}
		if ipv4mode, ok := data["ipv4mode"].(string); ok {
			wifi.IPv4Mode = configuration.StringPtr(ipv4mode)
		}

		// Convert static IP if present
		if staticData, ok := data["ip"].(map[string]interface{}); ok {
			static := &configuration.StaticIPConfig{}
			if ip, ok := staticData["ip"].(string); ok {
				static.IP = configuration.StringPtr(ip)
			}
			if netmask, ok := staticData["netmask"].(string); ok {
				static.Netmask = configuration.StringPtr(netmask)
			}
			if gw, ok := staticData["gw"].(string); ok {
				static.Gateway = configuration.StringPtr(gw)
			}
			if dns, ok := staticData["nameserver"].(string); ok {
				static.Nameserver = configuration.StringPtr(dns)
			}
			wifi.StaticIP = static
		}
	}

	// Handle access point configuration (wifi_ap)
	if apData, ok := data["wifi_ap"].(map[string]interface{}); ok {
		ap := &configuration.AccessPointConfig{}

		// Try both "enable" and "enabled" (Shelly devices use "enabled")
		if enable, ok := apData["enable"].(bool); ok {
			ap.Enable = configuration.BoolPtr(enable)
		} else if enabled, ok := apData["enabled"].(bool); ok {
			ap.Enable = configuration.BoolPtr(enabled)
		}
		if ssid, ok := apData["ssid"].(string); ok {
			ap.SSID = configuration.StringPtr(ssid)
		}
		if pass, ok := apData["pass"].(string); ok {
			ap.Password = configuration.StringPtr(pass)
		}
		if key, ok := apData["key"].(string); ok {
			ap.Key = configuration.StringPtr(key)
		}

		wifi.AccessPoint = ap
	}

	return wifi, warnings
}

func (h *Handler) convertMQTTConfig(data map[string]interface{}) (*configuration.MQTTConfiguration, []string) {
	mqtt := &configuration.MQTTConfiguration{}
	var warnings []string

	if enable, ok := data["enable"].(bool); ok {
		mqtt.Enable = configuration.BoolPtr(enable)
	}

	if server, ok := data["server"].(string); ok {
		mqtt.Server = configuration.StringPtr(server)
	}

	if port, ok := data["port"].(float64); ok {
		mqtt.Port = configuration.IntPtr(int(port))
	}

	if user, ok := data["user"].(string); ok {
		mqtt.User = configuration.StringPtr(user)
	}

	if pass, ok := data["pass"].(string); ok {
		mqtt.Password = configuration.StringPtr(pass)
	}

	if clientID, ok := data["id"].(string); ok {
		mqtt.ClientID = configuration.StringPtr(clientID)
	}

	if keepAlive, ok := data["keep_alive"].(float64); ok {
		mqtt.KeepAlive = configuration.IntPtr(int(keepAlive))
	}

	return mqtt, warnings
}

func (h *Handler) convertAuthConfig(data map[string]interface{}) (*configuration.AuthConfiguration, []string) {
	auth := &configuration.AuthConfiguration{}
	var warnings []string

	// Handle both "enable" and "enabled" (Shelly devices use "enabled")
	if enable, ok := data["enable"].(bool); ok {
		auth.Enable = configuration.BoolPtr(enable)
	} else if enabled, ok := data["enabled"].(bool); ok {
		auth.Enable = configuration.BoolPtr(enabled)
	}

	// Handle both "user" and "username" (Shelly devices use "username")
	if user, ok := data["user"].(string); ok {
		auth.Username = configuration.StringPtr(user)
	} else if username, ok := data["username"].(string); ok {
		auth.Username = configuration.StringPtr(username)
	}

	// The device spells the login password "password"; "pass" wins where a configuration has both.
	if pass, ok := passwordField(data, "pass", "password"); ok {
		auth.Password = configuration.StringPtr(pass)
	}

	if realm, ok := data["realm"].(string); ok {
		auth.Realm = configuration.StringPtr(realm)
	}

	return auth, warnings
}

func (h *Handler) convertSystemConfig(data map[string]interface{}) (*configuration.SystemConfiguration, []string) {
	system := &configuration.SystemConfiguration{}
	var warnings []string

	// Convert device settings - Shelly devices have mixed structure
	device := &configuration.TypedDeviceConfig{}
	hasDeviceData := false

	// Device name from root level "name" field
	if name, ok := data["name"].(string); ok {
		device.Name = configuration.StringPtr(name)
		hasDeviceData = true
	}

	// Device info from "device" object
	if deviceData, ok := data["device"].(map[string]interface{}); ok {
		if hostname, ok := deviceData["hostname"].(string); ok {
			device.Hostname = configuration.StringPtr(hostname)
			hasDeviceData = true
		}
		if mac, ok := deviceData["mac"].(string); ok {
			device.MAC = configuration.StringPtr(mac)
			hasDeviceData = true
		}
		if deviceType, ok := deviceData["type"].(string); ok {
			device.Profile = configuration.StringPtr(deviceType)
			hasDeviceData = true
		}
	}

	// Timezone from root level "tz" or "timezone" field
	if tz, ok := data["tz"].(string); ok {
		device.Timezone = configuration.StringPtr(tz)
		hasDeviceData = true
	} else if timezone, ok := data["timezone"].(string); ok {
		device.Timezone = configuration.StringPtr(timezone)
		hasDeviceData = true
	}

	// Discoverable from root level
	if discoverable, ok := data["discoverable"].(bool); ok {
		device.Discoverable = configuration.BoolPtr(discoverable)
		hasDeviceData = true
	}

	// Eco mode from root level
	if ecoMode, ok := data["eco_mode"].(bool); ok {
		device.EcoMode = configuration.BoolPtr(ecoMode)
		hasDeviceData = true
	}

	if hasDeviceData {
		system.Device = device
	}

	// Convert location settings from root level lat/lng
	if lat, ok := data["lat"].(float64); ok {
		if lng, ok := data["lng"].(float64); ok {
			location := &configuration.LocationConfig{}
			location.Latitude = configuration.Float64Ptr(lat)
			location.Longitude = configuration.Float64Ptr(lng)
			system.Location = location
		}
	}

	// Convert SNTP settings from root level "sntp" object
	if sntpData, ok := data["sntp"].(map[string]interface{}); ok {
		// Only include SNTP config if it's enabled and has a server
		if enabled, ok := sntpData["enabled"].(bool); ok && enabled {
			if server, ok := sntpData["server"].(string); ok && server != "" {
				sntp := &configuration.SNTPConfig{}
				sntp.Server = server
				system.SNTP = sntp
			}
		}
	}

	return system, warnings
}

func (h *Handler) convertCloudConfig(data map[string]interface{}) (*configuration.CloudConfiguration, []string) {
	cloud := &configuration.CloudConfiguration{}
	var warnings []string

	// Handle both "enable" and "enabled" (Shelly devices use "enabled")
	if enable, ok := data["enable"].(bool); ok {
		cloud.Enable = configuration.BoolPtr(enable)
	} else if enabled, ok := data["enabled"].(bool); ok {
		cloud.Enable = configuration.BoolPtr(enabled)
	}

	if server, ok := data["server"].(string); ok {
		cloud.Server = configuration.StringPtr(server)
	}

	return cloud, warnings
}

// Capability conversion functions

func (h *Handler) convertCoIoTConfig(data map[string]interface{}) (*configuration.CoIoTConfig, []string) {
	coiot := &configuration.CoIoTConfig{}
	var warnings []string

	if enabled, ok := data["enabled"].(bool); ok {
		coiot.Enabled = enabled
	}
	if server, ok := data["server"].(string); ok {
		coiot.Server = server
	}
	if port, ok := data["port"].(float64); ok {
		coiot.Port = int(port)
	}
	if period, ok := data["period"].(float64); ok {
		coiot.Period = int(period)
	}

	return coiot, warnings
}

func (h *Handler) convertRelayConfig(data map[string]interface{}, deviceType string) (*configuration.RelayConfig, []string) {
	relay := &configuration.RelayConfig{}
	var warnings []string

	// Check for single relay configuration
	if relayData, ok := data["relay"].(map[string]interface{}); ok {
		if defaultState, ok := relayData["default_state"].(string); ok {
			relay.DefaultState = configuration.StringPtr(defaultState)
		}
		if btnType, ok := relayData["btn_type"].(string); ok {
			relay.ButtonType = configuration.StringPtr(btnType)
		}
		if autoOn, ok := relayData["auto_on"].(float64); ok && autoOn > 0 {
			autoOnInt := int(autoOn)
			relay.AutoOn = &autoOnInt
		}
		if autoOff, ok := relayData["auto_off"].(float64); ok && autoOff > 0 {
			autoOffInt := int(autoOff)
			relay.AutoOff = &autoOffInt
		}
		if hasTimer, ok := relayData["has_timer"].(bool); ok {
			relay.HasTimer = configuration.BoolPtr(hasTimer)
		}
	}

	// Check for "relays" array format (most common format)
	var relayConfigs []configuration.SingleRelayConfig
	if relaysData, ok := data["relays"].([]interface{}); ok {
		for i, relayData := range relaysData {
			if relayMap, ok := relayData.(map[string]interface{}); ok {
				singleRelay := configuration.SingleRelayConfig{
					ID: i,
				}

				if name, ok := relayMap["name"].(string); ok && name != "" {
					singleRelay.Name = configuration.StringPtr(name)
				} else if appType, ok := relayMap["appliance_type"].(string); ok {
					singleRelay.Name = configuration.StringPtr(appType)
				}

				if defaultState, ok := relayMap["default_state"].(string); ok {
					singleRelay.DefaultState = configuration.StringPtr(defaultState)
					// Also set global default state from first relay
					if i == 0 && configuration.StringVal(relay.DefaultState, "") == "" {
						relay.DefaultState = configuration.StringPtr(defaultState)
					}
				}
				if autoOn, ok := relayMap["auto_on"].(float64); ok {
					autoOnInt := int(autoOn)
					singleRelay.AutoOn = &autoOnInt
					// Also set global auto_on from first relay
					if i == 0 && relay.AutoOn == nil {
						relay.AutoOn = &autoOnInt
					}
				}
				if autoOff, ok := relayMap["auto_off"].(float64); ok {
					autoOffInt := int(autoOff)
					singleRelay.AutoOff = &autoOffInt
					// Also set global auto_off from first relay
					if i == 0 && relay.AutoOff == nil {
						relay.AutoOff = &autoOffInt
					}
				}
				if schedule, ok := relayMap["schedule"].(bool); ok {
					singleRelay.Schedule = configuration.BoolPtr(schedule)
				}
				if btnType, ok := relayMap["btn_type"].(string); ok {
					// Set global button type from first relay
					if i == 0 && configuration.StringVal(relay.ButtonType, "") == "" {
						relay.ButtonType = configuration.StringPtr(btnType)
					}
				}
				if hasTimer, ok := relayMap["has_timer"].(bool); ok {
					// Set global has_timer from first relay
					if i == 0 {
						relay.HasTimer = configuration.BoolPtr(hasTimer)
					}
				}

				relayConfigs = append(relayConfigs, singleRelay)
			}
		}
	}

	// Check for multi-relay devices (SHSW-25, etc.) - relay_0, relay_1 format
	if len(relayConfigs) == 0 {
		for i := 0; i < 2; i++ {
			relayKey := fmt.Sprintf("relay_%d", i)
			if relayData, ok := data[relayKey].(map[string]interface{}); ok {
				singleRelay := configuration.SingleRelayConfig{
					ID: i,
				}
				if name, ok := relayData["name"].(string); ok {
					singleRelay.Name = configuration.StringPtr(name)
				}
				if defaultState, ok := relayData["default_state"].(string); ok {
					singleRelay.DefaultState = configuration.StringPtr(defaultState)
				}
				if autoOn, ok := relayData["auto_on"].(float64); ok && autoOn > 0 {
					autoOnInt := int(autoOn)
					singleRelay.AutoOn = &autoOnInt
				}
				if autoOff, ok := relayData["auto_off"].(float64); ok && autoOff > 0 {
					autoOffInt := int(autoOff)
					singleRelay.AutoOff = &autoOffInt
				}
				if schedule, ok := relayData["schedule"].(bool); ok {
					singleRelay.Schedule = configuration.BoolPtr(schedule)
				}
				relayConfigs = append(relayConfigs, singleRelay)
			}
		}
	}

	if len(relayConfigs) > 0 {
		relay.Relays = relayConfigs
	}

	// Fall back to global settings if no specific relay config found
	if configuration.StringVal(relay.DefaultState, "") == "" {
		if defaultState, ok := data["default_state"].(string); ok {
			relay.DefaultState = configuration.StringPtr(defaultState)
		}
	}

	// Ensure we always have at least a default state to prevent empty object
	if configuration.StringVal(relay.DefaultState, "") == "" {
		relay.DefaultState = configuration.StringPtr("off") // Sensible default
	}

	return relay, warnings
}

func (h *Handler) convertPowerMeteringConfig(data map[string]interface{}) (*configuration.PowerMeteringConfig, []string) {
	power := &configuration.PowerMeteringConfig{}
	var warnings []string

	if maxPower, ok := data["max_power"].(float64); ok && maxPower > 0 {
		maxPowerInt := int(maxPower)
		power.MaxPower = &maxPowerInt
	}
	if maxVoltage, ok := data["max_voltage"].(float64); ok && maxVoltage > 0 {
		maxVoltageInt := int(maxVoltage)
		power.MaxVoltage = &maxVoltageInt
	}
	if maxCurrent, ok := data["max_current"].(float64); ok && maxCurrent > 0 {
		power.MaxCurrent = &maxCurrent
	}
	if protection, ok := data["protection_action"].(string); ok {
		power.ProtectionAction = configuration.StringPtr(protection)
	}
	if correction, ok := data["power_correction"].(float64); ok {
		power.PowerCorrection = configuration.Float64Ptr(correction)
	} else {
		power.PowerCorrection = configuration.Float64Ptr(1.0) // Default multiplier
	}
	if period, ok := data["reporting_period"].(float64); ok {
		power.ReportingPeriod = configuration.IntPtr(int(period))
	}
	if costPerKWh, ok := data["cost_per_kwh"].(float64); ok && costPerKWh > 0 {
		power.CostPerKWh = &costPerKWh
	}
	if currency, ok := data["currency"].(string); ok {
		power.Currency = configuration.StringPtr(currency)
	}

	return power, warnings
}

func (h *Handler) convertDimmingConfig(data map[string]interface{}) (*configuration.DimmingConfig, []string) {
	dimming := &configuration.DimmingConfig{}
	var warnings []string

	// Check for light configuration
	if lightData, ok := data["light"].(map[string]interface{}); ok {
		data = lightData // Use light data as primary source
	} else if light0Data, ok := data["light_0"].(map[string]interface{}); ok {
		data = light0Data // Use first light channel
	}

	if minBrightness, ok := data["min_brightness"].(float64); ok {
		dimming.MinBrightness = configuration.IntPtr(int(minBrightness))
	} else {
		dimming.MinBrightness = configuration.IntPtr(1)
	}
	if maxBrightness, ok := data["max_brightness"].(float64); ok {
		dimming.MaxBrightness = configuration.IntPtr(int(maxBrightness))
	} else {
		dimming.MaxBrightness = configuration.IntPtr(100)
	}
	if defaultBrightness, ok := data["default_brightness"].(float64); ok {
		dimming.DefaultBrightness = configuration.IntPtr(int(defaultBrightness))
	} else {
		dimming.DefaultBrightness = configuration.IntPtr(100)
	}
	if defaultState, ok := data["default_state"].(bool); ok {
		dimming.DefaultState = configuration.BoolPtr(defaultState)
	}
	if fadeRate, ok := data["fade_rate"].(float64); ok {
		dimming.FadeRate = configuration.IntPtr(int(fadeRate))
	}
	if transition, ok := data["transition"].(float64); ok {
		dimming.TransitionTime = configuration.IntPtr(int(transition))
	}
	if leadingEdge, ok := data["leading_edge"].(bool); ok {
		dimming.LeadingEdge = configuration.BoolPtr(leadingEdge)
	}
	if nightMode, ok := data["night_mode"].(bool); ok {
		dimming.NightModeEnabled = configuration.BoolPtr(nightMode)
	}
	if nightBrightness, ok := data["night_mode_brightness"].(float64); ok {
		dimming.NightModeBrightness = configuration.IntPtr(int(nightBrightness))
	}
	if nightStart, ok := data["night_mode_start"].(string); ok {
		dimming.NightModeStart = configuration.StringPtr(nightStart)
	}
	if nightEnd, ok := data["night_mode_end"].(string); ok {
		dimming.NightModeEnd = configuration.StringPtr(nightEnd)
	}

	return dimming, warnings
}

func (h *Handler) convertRollerConfig(data map[string]interface{}) (*configuration.RollerConfig, []string) {
	roller := &configuration.RollerConfig{}
	var warnings []string

	// Check for roller configuration
	if rollerData, ok := data["roller"].(map[string]interface{}); ok {
		data = rollerData
	} else if roller0Data, ok := data["roller_0"].(map[string]interface{}); ok {
		data = roller0Data
	}

	if motorDirection, ok := data["motor_direction"].(string); ok {
		roller.MotorDirection = configuration.StringPtr(motorDirection)
	}
	if maxOpenTime, ok := data["max_open_time"].(float64); ok {
		roller.MaxOpenTime = configuration.IntPtr(int(maxOpenTime))
	}
	if maxCloseTime, ok := data["max_close_time"].(float64); ok {
		roller.MaxCloseTime = configuration.IntPtr(int(maxCloseTime))
	}
	if defaultPos, ok := data["default_position"].(float64); ok {
		defaultPosInt := int(defaultPos)
		roller.DefaultPosition = &defaultPosInt
	}
	if currentPos, ok := data["current_position"].(float64); ok {
		roller.CurrentPosition = configuration.IntPtr(int(currentPos))
	}
	if positioning, ok := data["positioning"].(bool); ok {
		roller.PositioningEnabled = configuration.BoolPtr(positioning)
	}
	if obstacle, ok := data["obstacle_detection"].(bool); ok {
		roller.ObstacleDetection = configuration.BoolPtr(obstacle)
	}
	if obstaclePower, ok := data["obstacle_power"].(float64); ok && obstaclePower > 0 {
		obstaclePowerInt := int(obstaclePower)
		roller.ObstaclePower = &obstaclePowerInt
	}
	if safetySwitch, ok := data["safety_switch"].(bool); ok {
		roller.SafetySwitch = configuration.BoolPtr(safetySwitch)
	}
	if swap, ok := data["swap"].(bool); ok {
		roller.SwapInputs = configuration.BoolPtr(swap)
	}
	if inputMode, ok := data["input_mode"].(string); ok {
		roller.InputMode = configuration.StringPtr(inputMode)
	}
	if holdTime, ok := data["button_hold_time"].(float64); ok {
		roller.ButtonHoldTime = configuration.IntPtr(int(holdTime))
	}

	return roller, warnings
}

func (h *Handler) convertInputConfig(data map[string]interface{}) (*configuration.InputConfig, []string) {
	input := &configuration.InputConfig{}
	var warnings []string

	// Check for inputs array configuration (for multi-input devices like SHIX3-1)
	if inputsData, ok := data["inputs"].([]interface{}); ok {
		for i, inputData := range inputsData {
			if inputMap, ok := inputData.(map[string]interface{}); ok {
				singleInput := configuration.SingleInputConfig{ID: i}

				// Extract individual input properties
				if name, ok := inputMap["name"].(string); ok && name != "" {
					singleInput.Name = configuration.StringPtr(name)
				}
				if btnType, ok := inputMap["btn_type"].(string); ok {
					singleInput.Type = configuration.StringPtr("button") // Default type
					singleInput.Mode = configuration.StringPtr(btnType)
				}
				if btnReverse, ok := inputMap["btn_reverse"].(float64); ok {
					singleInput.Inverted = configuration.BoolPtr(btnReverse != 0)
				}

				if longPushDuration, ok := data["longpush_duration_ms"].(map[string]interface{}); ok {
					if _, ok := longPushDuration["min"].(float64); ok {
						singleInput.LongPushAction = configuration.StringPtr("long")
					}
				}

				input.Inputs = append(input.Inputs, singleInput)
			}
		}

		// Set global input properties from timing configurations
		if longPushDuration, ok := data["longpush_duration_ms"].(map[string]interface{}); ok {
			if minDuration, ok := longPushDuration["min"].(float64); ok {
				input.LongPushTime = configuration.IntPtr(int(minDuration))
			}
		}
		if multiPushTime, ok := data["multipush_time_between_pushes_ms"].(map[string]interface{}); ok {
			if maxTime, ok := multiPushTime["max"].(float64); ok {
				input.MultiPushTime = configuration.IntPtr(int(maxTime))
			}
		}

		return input, warnings
	}

	// Check for single input configuration
	if inputData, ok := data["input"].(map[string]interface{}); ok {
		data = inputData
	} else if input0Data, ok := data["input_0"].(map[string]interface{}); ok {
		data = input0Data
	}

	if inputType, ok := data["type"].(string); ok {
		input.Type = configuration.StringPtr(inputType)
	}
	if mode, ok := data["mode"].(string); ok {
		input.Mode = configuration.StringPtr(mode)
	}
	if inverted, ok := data["inverted"].(bool); ok {
		input.Inverted = configuration.BoolPtr(inverted)
	}
	if debounce, ok := data["debounce_time"].(float64); ok {
		input.DebounceTime = configuration.IntPtr(int(debounce))
	}
	if longPush, ok := data["longpush_time"].(float64); ok {
		input.LongPushTime = configuration.IntPtr(int(longPush))
	}
	if multiPush, ok := data["multipush_time"].(float64); ok {
		input.MultiPushTime = configuration.IntPtr(int(multiPush))
	}
	if singleAction, ok := data["single_push_action"].(string); ok {
		input.SinglePushAction = configuration.StringPtr(singleAction)
	}
	if doubleAction, ok := data["double_push_action"].(string); ok {
		input.DoublePushAction = configuration.StringPtr(doubleAction)
	}
	if longAction, ok := data["long_push_action"].(string); ok {
		input.LongPushAction = configuration.StringPtr(longAction)
	}

	return input, warnings
}

func (h *Handler) convertLEDConfig(data map[string]interface{}) (*configuration.LEDConfig, []string) {
	led := &configuration.LEDConfig{}
	var warnings []string

	if enabled, ok := data["led_status_disable"].(bool); ok {
		led.Enabled = configuration.BoolPtr(!enabled)
	}
	if mode, ok := data["led_mode"].(string); ok {
		led.Mode = configuration.StringPtr(mode)
	}
	if brightness, ok := data["led_brightness"].(float64); ok {
		led.Brightness = configuration.IntPtr(int(brightness))
	}
	if nightMode, ok := data["led_night_mode"].(bool); ok {
		led.NightModeEnabled = configuration.BoolPtr(nightMode)
	}
	if powerIndication, ok := data["led_power_disable"].(bool); ok {
		led.PowerIndication = configuration.BoolPtr(!powerIndication)
	}

	return led, warnings
}

func (h *Handler) convertColorConfig(data map[string]interface{}) (*configuration.ColorConfig, []string) {
	color := &configuration.ColorConfig{}
	var warnings []string

	// Check for color configuration
	if colorData, ok := data["color"].(map[string]interface{}); ok {
		data = colorData
	} else if color0Data, ok := data["color_0"].(map[string]interface{}); ok {
		data = color0Data
	}

	if mode, ok := data["mode"].(string); ok {
		color.Mode = mode
	}
	if effectsEnabled, ok := data["effects_enabled"].(bool); ok {
		color.EffectsEnabled = effectsEnabled
	}
	if activeEffect, ok := data["active_effect"].(float64); ok {
		activeEffectInt := int(activeEffect)
		color.ActiveEffect = &activeEffectInt
	}
	if effectSpeed, ok := data["effect_speed"].(float64); ok {
		color.EffectSpeed = int(effectSpeed)
	}
	if redCal, ok := data["red_calibration"].(float64); ok {
		color.RedCalibration = redCal
	} else {
		color.RedCalibration = 1.0
	}
	if greenCal, ok := data["green_calibration"].(float64); ok {
		color.GreenCalibration = greenCal
	} else {
		color.GreenCalibration = 1.0
	}
	if blueCal, ok := data["blue_calibration"].(float64); ok {
		color.BlueCalibration = blueCal
	} else {
		color.BlueCalibration = 1.0
	}
	if whiteCal, ok := data["white_calibration"].(float64); ok {
		color.WhiteCalibration = whiteCal
	} else {
		color.WhiteCalibration = 1.0
	}

	// Default color if specified
	if defaultColor, ok := data["default_color"].(map[string]interface{}); ok {
		if r, rOk := defaultColor["r"].(float64); rOk {
			if g, gOk := defaultColor["g"].(float64); gOk {
				if b, bOk := defaultColor["b"].(float64); bOk {
					color.DefaultColor = &configuration.Color{
						Red:   int(r),
						Green: int(g),
						Blue:  int(b),
					}
				}
			}
		}
	}

	return color, warnings
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getKeys returns the keys of a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
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
		}).Error("Failed to get device config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
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

	response := TypedConfigurationResponse{
		Configuration:    typedConfig,
		ValidationResult: validationResult,
		ConversionInfo:   conversionInfo,
	}

	// Wrap response with success for frontend compatibility
	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, map[string]interface{}{
		"success":       true,
		"configuration": response,
	})
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

	// Normalize the typed configuration
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

	// Decode request
	var req TypedConfigurationRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
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
		http.Error(w, fmt.Sprintf("Failed to convert configuration: %v", err), http.StatusInternalServerError)
		return
	}

	response := TypedConfigurationResponse{
		Configuration:  typedConfig,
		ConversionInfo: conversionInfo,
	}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, response)
}

// ConvertTypedToRaw handles POST /api/v1/configuration/convert-to-raw
func (h *Handler) ConvertTypedToRaw(w http.ResponseWriter, r *http.Request) {
	var req TypedConfigurationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Configuration == nil {
		http.Error(w, "Configuration is required", http.StatusBadRequest)
		return
	}

	// Convert to raw JSON
	rawJSON, err := req.Configuration.ToJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to convert to raw JSON: %v", err), http.StatusInternalServerError)
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

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, response)
}

// GetConfigurationSchema handles GET /api/v1/configuration/schema
func (h *Handler) GetConfigurationSchema(w http.ResponseWriter, r *http.Request) {
	schema := configuration.GetConfigurationSchema()

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, schema)
}

// BulkValidateConfigs handles POST /api/v1/configuration/bulk-validate
func (h *Handler) BulkValidateConfigs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Configurations  []TypedConfigurationRequest `json:"configurations"`
		ValidationLevel string                      `json:"validation_level,omitempty"`
		StopOnError     bool                        `json:"stop_on_error,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Configurations) == 0 {
		http.Error(w, "No configurations provided", http.StatusBadRequest)
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

	// All devices support basic capabilities
	if generation >= 2 {
		capabilities = append(capabilities, "mqtt", "cloud", "auth")
	}

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

	// Generation-specific capabilities
	if generation >= 2 {
		capabilities = append(capabilities, "ble", "ethernet")
	}

	return capabilities
}

// Helper conversion functions

func (h *Handler) convertWiFiConfig(data map[string]interface{}) (*configuration.WiFiConfiguration, []string) {
	wifi := &configuration.WiFiConfiguration{}
	var warnings []string

	// Handle station mode WiFi configuration (wifi_sta)
	if staData, ok := data["wifi_sta"].(map[string]interface{}); ok {
		// Try both "enable" and "enabled" (Shelly devices use "enabled")
		if enable, ok := staData["enable"].(bool); ok {
			wifi.Enable = enable
		} else if enabled, ok := staData["enabled"].(bool); ok {
			wifi.Enable = enabled
		}
		if ssid, ok := staData["ssid"].(string); ok {
			wifi.SSID = ssid
		}
		if pass, ok := staData["pass"].(string); ok {
			wifi.Password = pass
		}
		// Handle both "ipv4mode" and "ipv4_method" (Shelly devices use "ipv4_method")
		if ipv4mode, ok := staData["ipv4mode"].(string); ok {
			wifi.IPv4Mode = ipv4mode
		} else if ipv4method, ok := staData["ipv4_method"].(string); ok {
			wifi.IPv4Mode = ipv4method
		}

		// Convert static IP if present
		if staticData, ok := staData["ip"].(map[string]interface{}); ok {
			static := &configuration.StaticIPConfig{}
			if ip, ok := staticData["ip"].(string); ok {
				static.IP = ip
			}
			if netmask, ok := staticData["netmask"].(string); ok {
				static.Netmask = netmask
			}
			if gw, ok := staticData["gw"].(string); ok {
				static.Gateway = gw
			}
			if dns, ok := staticData["nameserver"].(string); ok {
				static.Nameserver = dns
			}
			wifi.StaticIP = static
		}
	} else {
		// Fallback to direct WiFi fields for older format
		if enable, ok := data["enable"].(bool); ok {
			wifi.Enable = enable
		}
		if ssid, ok := data["ssid"].(string); ok {
			wifi.SSID = ssid
		}
		if pass, ok := data["pass"].(string); ok {
			wifi.Password = pass
		}
		if ipv4mode, ok := data["ipv4mode"].(string); ok {
			wifi.IPv4Mode = ipv4mode
		}

		// Convert static IP if present
		if staticData, ok := data["ip"].(map[string]interface{}); ok {
			static := &configuration.StaticIPConfig{}
			if ip, ok := staticData["ip"].(string); ok {
				static.IP = ip
			}
			if netmask, ok := staticData["netmask"].(string); ok {
				static.Netmask = netmask
			}
			if gw, ok := staticData["gw"].(string); ok {
				static.Gateway = gw
			}
			if dns, ok := staticData["nameserver"].(string); ok {
				static.Nameserver = dns
			}
			wifi.StaticIP = static
		}
	}

	// Handle access point configuration (wifi_ap)
	if apData, ok := data["wifi_ap"].(map[string]interface{}); ok {
		ap := &configuration.AccessPointConfig{}

		// Try both "enable" and "enabled" (Shelly devices use "enabled")
		if enable, ok := apData["enable"].(bool); ok {
			ap.Enable = enable
		} else if enabled, ok := apData["enabled"].(bool); ok {
			ap.Enable = enabled
		}
		if ssid, ok := apData["ssid"].(string); ok {
			ap.SSID = ssid
		}
		if pass, ok := apData["pass"].(string); ok {
			ap.Password = pass
		}
		if key, ok := apData["key"].(string); ok {
			ap.Key = key
		}

		wifi.AccessPoint = ap
	}

	return wifi, warnings
}

func (h *Handler) convertMQTTConfig(data map[string]interface{}) (*configuration.MQTTConfiguration, []string) {
	mqtt := &configuration.MQTTConfiguration{}
	var warnings []string

	if enable, ok := data["enable"].(bool); ok {
		mqtt.Enable = enable
	}

	if server, ok := data["server"].(string); ok {
		mqtt.Server = server
	}

	if port, ok := data["port"].(float64); ok {
		mqtt.Port = int(port)
	}

	if user, ok := data["user"].(string); ok {
		mqtt.User = user
	}

	if pass, ok := data["pass"].(string); ok {
		mqtt.Password = pass
	}

	if clientID, ok := data["id"].(string); ok {
		mqtt.ClientID = clientID
	}

	if keepAlive, ok := data["keep_alive"].(float64); ok {
		mqtt.KeepAlive = int(keepAlive)
	}

	return mqtt, warnings
}

func (h *Handler) convertAuthConfig(data map[string]interface{}) (*configuration.AuthConfiguration, []string) {
	auth := &configuration.AuthConfiguration{}
	var warnings []string

	// Handle both "enable" and "enabled" (Shelly devices use "enabled")
	if enable, ok := data["enable"].(bool); ok {
		auth.Enable = enable
	} else if enabled, ok := data["enabled"].(bool); ok {
		auth.Enable = enabled
	}

	// Handle both "user" and "username" (Shelly devices use "username")
	if user, ok := data["user"].(string); ok {
		auth.Username = user
	} else if username, ok := data["username"].(string); ok {
		auth.Username = username
	}

	if pass, ok := data["pass"].(string); ok {
		auth.Password = pass
	}

	if realm, ok := data["realm"].(string); ok {
		auth.Realm = realm
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
		device.Name = name
		hasDeviceData = true
	}

	// Device info from "device" object
	if deviceData, ok := data["device"].(map[string]interface{}); ok {
		if hostname, ok := deviceData["hostname"].(string); ok {
			device.Hostname = hostname
			hasDeviceData = true
		}
		if mac, ok := deviceData["mac"].(string); ok {
			device.MAC = mac
			hasDeviceData = true
		}
		if deviceType, ok := deviceData["type"].(string); ok {
			device.Profile = deviceType
			hasDeviceData = true
		}
	}

	// Timezone from root level "tz" or "timezone" field
	if tz, ok := data["tz"].(string); ok {
		device.Timezone = tz
		hasDeviceData = true
	} else if timezone, ok := data["timezone"].(string); ok {
		device.Timezone = timezone
		hasDeviceData = true
	}

	// Discoverable from root level
	if discoverable, ok := data["discoverable"].(bool); ok {
		device.Discoverable = discoverable
		hasDeviceData = true
	}

	// Eco mode from root level
	if ecoMode, ok := data["eco_mode"].(bool); ok {
		device.EcoMode = ecoMode
		hasDeviceData = true
	}

	if hasDeviceData {
		system.Device = device
	}

	// Convert location settings from root level lat/lng
	if lat, ok := data["lat"].(float64); ok {
		if lng, ok := data["lng"].(float64); ok {
			location := &configuration.LocationConfig{}
			location.Latitude = lat
			location.Longitude = lng
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
		cloud.Enable = enable
	} else if enabled, ok := data["enabled"].(bool); ok {
		cloud.Enable = enabled
	}

	if server, ok := data["server"].(string); ok {
		cloud.Server = server
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
			relay.DefaultState = defaultState
		}
		if btnType, ok := relayData["btn_type"].(string); ok {
			relay.ButtonType = btnType
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
			relay.HasTimer = hasTimer
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
					singleRelay.Name = name
				} else if appType, ok := relayMap["appliance_type"].(string); ok {
					singleRelay.Name = appType
				}

				if defaultState, ok := relayMap["default_state"].(string); ok {
					singleRelay.DefaultState = defaultState
					// Also set global default state from first relay
					if i == 0 && relay.DefaultState == "" {
						relay.DefaultState = defaultState
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
					singleRelay.Schedule = schedule
				}
				if btnType, ok := relayMap["btn_type"].(string); ok {
					// Set global button type from first relay
					if i == 0 && relay.ButtonType == "" {
						relay.ButtonType = btnType
					}
				}
				if hasTimer, ok := relayMap["has_timer"].(bool); ok {
					// Set global has_timer from first relay
					if i == 0 {
						relay.HasTimer = hasTimer
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
					singleRelay.Name = name
				}
				if defaultState, ok := relayData["default_state"].(string); ok {
					singleRelay.DefaultState = defaultState
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
					singleRelay.Schedule = schedule
				}
				relayConfigs = append(relayConfigs, singleRelay)
			}
		}
	}

	if len(relayConfigs) > 0 {
		relay.Relays = relayConfigs
	}

	// Fall back to global settings if no specific relay config found
	if relay.DefaultState == "" {
		if defaultState, ok := data["default_state"].(string); ok {
			relay.DefaultState = defaultState
		}
	}

	// Ensure we always have at least a default state to prevent empty object
	if relay.DefaultState == "" {
		relay.DefaultState = "off" // Sensible default
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
		power.ProtectionAction = protection
	}
	if correction, ok := data["power_correction"].(float64); ok {
		power.PowerCorrection = correction
	} else {
		power.PowerCorrection = 1.0 // Default multiplier
	}
	if period, ok := data["reporting_period"].(float64); ok {
		power.ReportingPeriod = int(period)
	}
	if costPerKWh, ok := data["cost_per_kwh"].(float64); ok && costPerKWh > 0 {
		power.CostPerKWh = &costPerKWh
	}
	if currency, ok := data["currency"].(string); ok {
		power.Currency = currency
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
		dimming.MinBrightness = int(minBrightness)
	} else {
		dimming.MinBrightness = 1 // Default minimum
	}
	if maxBrightness, ok := data["max_brightness"].(float64); ok {
		dimming.MaxBrightness = int(maxBrightness)
	} else {
		dimming.MaxBrightness = 100 // Default maximum
	}
	if defaultBrightness, ok := data["default_brightness"].(float64); ok {
		dimming.DefaultBrightness = int(defaultBrightness)
	} else {
		dimming.DefaultBrightness = 100 // Default
	}
	if defaultState, ok := data["default_state"].(bool); ok {
		dimming.DefaultState = defaultState
	}
	if fadeRate, ok := data["fade_rate"].(float64); ok {
		dimming.FadeRate = int(fadeRate)
	}
	if transition, ok := data["transition"].(float64); ok {
		dimming.TransitionTime = int(transition)
	}
	if leadingEdge, ok := data["leading_edge"].(bool); ok {
		dimming.LeadingEdge = leadingEdge
	}
	if nightMode, ok := data["night_mode"].(bool); ok {
		dimming.NightModeEnabled = nightMode
	}
	if nightBrightness, ok := data["night_mode_brightness"].(float64); ok {
		dimming.NightModeBrightness = int(nightBrightness)
	}
	if nightStart, ok := data["night_mode_start"].(string); ok {
		dimming.NightModeStart = nightStart
	}
	if nightEnd, ok := data["night_mode_end"].(string); ok {
		dimming.NightModeEnd = nightEnd
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
		roller.MotorDirection = motorDirection
	}
	if maxOpenTime, ok := data["max_open_time"].(float64); ok {
		roller.MaxOpenTime = int(maxOpenTime)
	}
	if maxCloseTime, ok := data["max_close_time"].(float64); ok {
		roller.MaxCloseTime = int(maxCloseTime)
	}
	if defaultPos, ok := data["default_position"].(float64); ok {
		defaultPosInt := int(defaultPos)
		roller.DefaultPosition = &defaultPosInt
	}
	if currentPos, ok := data["current_position"].(float64); ok {
		roller.CurrentPosition = int(currentPos)
	}
	if positioning, ok := data["positioning"].(bool); ok {
		roller.PositioningEnabled = positioning
	}
	if obstacle, ok := data["obstacle_detection"].(bool); ok {
		roller.ObstacleDetection = obstacle
	}
	if obstaclePower, ok := data["obstacle_power"].(float64); ok && obstaclePower > 0 {
		obstaclePowerInt := int(obstaclePower)
		roller.ObstaclePower = &obstaclePowerInt
	}
	if safetySwitch, ok := data["safety_switch"].(bool); ok {
		roller.SafetySwitch = safetySwitch
	}
	if swap, ok := data["swap"].(bool); ok {
		roller.SwapInputs = swap
	}
	if inputMode, ok := data["input_mode"].(string); ok {
		roller.InputMode = inputMode
	}
	if holdTime, ok := data["button_hold_time"].(float64); ok {
		roller.ButtonHoldTime = int(holdTime)
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
					singleInput.Name = name
				}
				if btnType, ok := inputMap["btn_type"].(string); ok {
					singleInput.Type = "button" // Default type
					singleInput.Mode = btnType
				}
				if btnReverse, ok := inputMap["btn_reverse"].(float64); ok {
					singleInput.Inverted = btnReverse != 0
				}

				// Add timing settings from global config
				if longPushDuration, ok := data["longpush_duration_ms"].(map[string]interface{}); ok {
					if _, ok := longPushDuration["min"].(float64); ok {
						singleInput.LongPushAction = "long" // Default action
					}
				}

				input.Inputs = append(input.Inputs, singleInput)
			}
		}

		// Set global input properties from timing configurations
		if longPushDuration, ok := data["longpush_duration_ms"].(map[string]interface{}); ok {
			if minDuration, ok := longPushDuration["min"].(float64); ok {
				input.LongPushTime = int(minDuration)
			}
		}
		if multiPushTime, ok := data["multipush_time_between_pushes_ms"].(map[string]interface{}); ok {
			if maxTime, ok := multiPushTime["max"].(float64); ok {
				input.MultiPushTime = int(maxTime)
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
		input.Type = inputType
	}
	if mode, ok := data["mode"].(string); ok {
		input.Mode = mode
	}
	if inverted, ok := data["inverted"].(bool); ok {
		input.Inverted = inverted
	}
	if debounce, ok := data["debounce_time"].(float64); ok {
		input.DebounceTime = int(debounce)
	}
	if longPush, ok := data["longpush_time"].(float64); ok {
		input.LongPushTime = int(longPush)
	}
	if multiPush, ok := data["multipush_time"].(float64); ok {
		input.MultiPushTime = int(multiPush)
	}
	if singleAction, ok := data["single_push_action"].(string); ok {
		input.SinglePushAction = singleAction
	}
	if doubleAction, ok := data["double_push_action"].(string); ok {
		input.DoublePushAction = doubleAction
	}
	if longAction, ok := data["long_push_action"].(string); ok {
		input.LongPushAction = longAction
	}

	return input, warnings
}

func (h *Handler) convertLEDConfig(data map[string]interface{}) (*configuration.LEDConfig, []string) {
	led := &configuration.LEDConfig{}
	var warnings []string

	if enabled, ok := data["led_status_disable"].(bool); ok {
		led.Enabled = !enabled // led_status_disable is the inverse
	}
	if mode, ok := data["led_mode"].(string); ok {
		led.Mode = mode
	}
	if brightness, ok := data["led_brightness"].(float64); ok {
		led.Brightness = int(brightness)
	}
	if nightMode, ok := data["led_night_mode"].(bool); ok {
		led.NightModeEnabled = nightMode
	}
	if powerIndication, ok := data["led_power_disable"].(bool); ok {
		led.PowerIndication = !powerIndication // led_power_disable is the inverse
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

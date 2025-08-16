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
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
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

	// Get device configuration (may be raw JSON or typed)
	rawConfig, err := h.Service.GetDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to typed configuration
	typedConfig, conversionInfo, err := h.convertToTypedConfig(rawConfig.Config, device)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to convert to typed config")
		http.Error(w, fmt.Sprintf("Failed to convert configuration: %v", err), http.StatusInternalServerError)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		json.NewEncoder(w).Encode(map[string]interface{}{
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
	json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(validationResult)
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
	json.NewEncoder(w).Encode(response)
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
	json.NewEncoder(w).Encode(response)
}

// GetConfigurationSchema handles GET /api/v1/configuration/schema
func (h *Handler) GetConfigurationSchema(w http.ResponseWriter, r *http.Request) {
	schema := configuration.GetConfigurationSchema()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
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
	json.NewEncoder(w).Encode(response)
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

	// Convert WiFi settings
	if wifiData, ok := rawData["wifi"].(map[string]interface{}); ok {
		wifi, warnings := h.convertWiFiConfig(wifiData)
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

	// Convert Auth settings
	if authData, ok := rawData["auth"].(map[string]interface{}); ok {
		auth, warnings := h.convertAuthConfig(authData)
		if auth != nil {
			typedConfig.Auth = auth
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert System settings
	if sysData, ok := rawData["sys"].(map[string]interface{}); ok {
		system, warnings := h.convertSystemConfig(sysData)
		if system != nil {
			typedConfig.System = system
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Convert Cloud settings
	if cloudData, ok := rawData["cloud"].(map[string]interface{}); ok {
		cloud, warnings := h.convertCloudConfig(cloudData)
		if cloud != nil {
			typedConfig.Cloud = cloud
			conversionInfo.Warnings = append(conversionInfo.Warnings, warnings...)
		}
	}

	// Store unconverted settings in Raw field
	filteredRaw := make(map[string]interface{})
	knownSections := map[string]bool{
		"wifi": true, "mqtt": true, "auth": true, "sys": true, "cloud": true,
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

	if enable, ok := data["enable"].(bool); ok {
		auth.Enable = enable
	}

	if user, ok := data["user"].(string); ok {
		auth.Username = user
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

	// Convert device settings
	if deviceData, ok := data["device"].(map[string]interface{}); ok {
		device := &configuration.TypedDeviceConfig{}

		if name, ok := deviceData["name"].(string); ok {
			device.Name = name
		}
		if hostname, ok := deviceData["hostname"].(string); ok {
			device.Hostname = hostname
		}
		if tz, ok := deviceData["tz"].(string); ok {
			device.Timezone = tz
		}

		system.Device = device
	}

	// Convert location settings
	if locData, ok := data["location"].(map[string]interface{}); ok {
		location := &configuration.LocationConfig{}

		if tz, ok := locData["tz"].(string); ok {
			location.Timezone = tz
		}
		if lat, ok := locData["lat"].(float64); ok {
			location.Latitude = lat
		}
		if lng, ok := locData["lng"].(float64); ok {
			location.Longitude = lng
		}

		system.Location = location
	}

	return system, warnings
}

func (h *Handler) convertCloudConfig(data map[string]interface{}) (*configuration.CloudConfiguration, []string) {
	cloud := &configuration.CloudConfiguration{}
	var warnings []string

	if enable, ok := data["enable"].(bool); ok {
		cloud.Enable = enable
	}

	if server, ok := data["server"].(string); ok {
		cloud.Server = server
	}

	return cloud, warnings
}

// getKeys returns the keys of a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

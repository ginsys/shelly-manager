package configuration

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
)

// ValidationLevel represents the strictness of validation
type ValidationLevel int

const (
	ValidationLevelBasic ValidationLevel = iota
	ValidationLevelStrict
	ValidationLevelProduction
)

// ValidationResult contains validation results
type ValidationResult struct {
	Valid    bool                `json:"valid"`
	Errors   []ValidationError   `json:"errors,omitempty"`
	Warnings []ValidationWarning `json:"warnings,omitempty"`
	Info     []ValidationInfo    `json:"info,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationInfo represents validation information
type ValidationInfo struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ConfigurationValidator provides comprehensive validation for device configurations
type ConfigurationValidator struct {
	level        ValidationLevel
	deviceModel  string
	generation   int
	capabilities []string
}

// NewConfigurationValidator creates a new configuration validator
func NewConfigurationValidator(level ValidationLevel, deviceModel string, generation int, capabilities []string) *ConfigurationValidator {
	return &ConfigurationValidator{
		level:        level,
		deviceModel:  deviceModel,
		generation:   generation,
		capabilities: capabilities,
	}
}

// ValidateConfiguration performs comprehensive validation of a device configuration
func (v *ConfigurationValidator) ValidateConfiguration(config json.RawMessage) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Info:     []ValidationInfo{},
	}

	// Check for templates first, before JSON parsing
	configStr := string(config)
	hasTemplates := containsTemplateVars(configStr)

	if hasTemplates {
		// If configuration contains templates, validate them
		v.validateTemplates(config, result)
		// For template configs, we can't do full typed validation
		// So we do basic safety checks instead
		v.validateTemplateBasics(config, result)
		// Also try to do partial validation on non-template parts
		v.validatePartialConfig(config, result)
		return result
	}

	// Parse as typed configuration
	typedConfig, err := FromJSON(config)
	if err != nil {
		// If typed parsing fails, try raw JSON validation
		return v.validateRawJSON(config)
	}

	// Validate typed configuration
	if err := typedConfig.Validate(); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "configuration",
			Message: err.Error(),
			Code:    "TYPED_VALIDATION_FAILED",
		})
	}

	// Perform specific validation checks
	v.validateWiFi(typedConfig.WiFi, result)
	v.validateMQTT(typedConfig.MQTT, result)
	v.validateAuth(typedConfig.Auth, result)
	v.validateSystem(typedConfig.System, result)
	v.validateNetwork(typedConfig.Network, result)
	v.validateCloud(typedConfig.Cloud, result)
	v.validateLocation(typedConfig.Location, result)

	// Perform device-specific validation
	v.validateDeviceCompatibility(typedConfig, result)

	// Perform safety checks
	v.performSafetyChecks(typedConfig, result)

	// Production-level checks
	if v.level >= ValidationLevelProduction {
		v.performProductionChecks(typedConfig, result)
	}

	return result
}

// validateRawJSON validates raw JSON configuration
func (v *ConfigurationValidator) validateRawJSON(config json.RawMessage) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Info:     []ValidationInfo{},
	}

	// Validate JSON syntax
	var rawData map[string]interface{}
	if err := json.Unmarshal(config, &rawData); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "json",
			Message: fmt.Sprintf("Invalid JSON syntax: %v", err),
			Code:    "INVALID_JSON",
		})
		return result
	}

	// Validate known dangerous settings
	v.validateDangerousSettings(rawData, result)

	// Validate basic network settings
	v.validateBasicNetworkSettings(rawData, result)

	return result
}

// validateWiFi validates WiFi configuration
func (v *ConfigurationValidator) validateWiFi(wifi *WiFiConfiguration, result *ValidationResult) {
	if wifi == nil {
		return
	}

	// Check if WiFi is supported
	if !v.hasCapability("wifi") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "wifi",
			Message: "WiFi configuration specified but device does not support WiFi",
			Code:    "WIFI_NOT_SUPPORTED",
		})
	}

	// Validate SSID strength
	if wifi.Enable != nil && *wifi.Enable && wifi.SSID != nil && *wifi.SSID != "" {
		if len(*wifi.SSID) < 3 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "wifi.ssid",
				Message: "SSID is very short, may cause connection issues",
				Code:    "SHORT_SSID",
			})
		}

		if strings.ContainsAny(*wifi.SSID, `"'<>&`) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "wifi.ssid",
				Message: "SSID contains special characters that may cause issues",
				Code:    "SPECIAL_CHARS_SSID",
			})
		}
	}

	// Validate password strength
	if wifi.Enable != nil && *wifi.Enable && wifi.Password != nil && *wifi.Password != "" {
		if len(*wifi.Password) < 8 {
			if v.level >= ValidationLevelStrict {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "wifi.password",
					Message: "WiFi password must be at least 8 characters",
					Code:    "WEAK_WIFI_PASSWORD",
				})
				result.Valid = false
			} else {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Field:   "wifi.password",
					Message: "WiFi password is shorter than recommended (8+ characters)",
					Code:    "WEAK_WIFI_PASSWORD",
				})
			}
		}
	}

	if wifi.StaticIP != nil {
		v.validateIPConfiguration(wifi.StaticIP, "wifi.static_ip", result)
	}

	if wifi.AccessPoint != nil && wifi.AccessPoint.Enable != nil && *wifi.AccessPoint.Enable && wifi.Enable != nil && *wifi.Enable {
		result.Info = append(result.Info, ValidationInfo{
			Field:   "wifi",
			Message: "Both STA and AP modes enabled - device will act as WiFi repeater",
			Code:    "WIFI_REPEATER_MODE",
		})
	}
}

// validateMQTT validates MQTT configuration
func (v *ConfigurationValidator) validateMQTT(mqtt *MQTTConfiguration, result *ValidationResult) {
	if mqtt == nil || mqtt.Enable == nil || !*mqtt.Enable {
		return
	}

	// Check if MQTT is supported
	if !v.hasCapability("mqtt") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "mqtt",
			Message: "MQTT configuration specified but device may not support MQTT",
			Code:    "MQTT_NOT_SUPPORTED",
		})
	}

	// Validate server connectivity (basic checks)
	if mqtt.Server != nil && *mqtt.Server != "" {
		server := *mqtt.Server
		// Check for localhost/private addresses in production
		if v.level >= ValidationLevelProduction {
			if strings.Contains(server, "localhost") || strings.Contains(server, "127.0.0.1") {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Field:   "mqtt.server",
					Message: "Using localhost MQTT server may not work in production",
					Code:    "LOCALHOST_MQTT_SERVER",
				})
			}
		}

		// Check for default credentials
		if mqtt.User != nil && mqtt.Password != nil && *mqtt.User == "admin" && *mqtt.Password == "admin" {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "mqtt.credentials",
				Message: "Using default MQTT credentials is insecure",
				Code:    "DEFAULT_MQTT_CREDENTIALS",
			})
		}
	}

	// Validate topic prefix
	if mqtt.TopicPrefix != nil && *mqtt.TopicPrefix != "" {
		prefix := *mqtt.TopicPrefix
		if strings.Contains(prefix, "#") || strings.Contains(prefix, "+") {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "mqtt.topic_prefix",
				Message: "Topic prefix cannot contain wildcards (# or +)",
				Code:    "INVALID_TOPIC_PREFIX",
			})
			result.Valid = false
		}
	}

	// Validate keep alive settings
	if mqtt.KeepAlive != nil && *mqtt.KeepAlive > 0 && *mqtt.KeepAlive < 30 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "mqtt.keep_alive",
			Message: "Very short keep alive interval may cause frequent reconnections",
			Code:    "SHORT_KEEPALIVE",
		})
	}
}

// validateAuth validates authentication configuration
func (v *ConfigurationValidator) validateAuth(auth *AuthConfiguration, result *ValidationResult) {
	if auth == nil || auth.Enable == nil || !*auth.Enable {
		if v.level >= ValidationLevelProduction && (auth == nil || auth.Enable == nil || !*auth.Enable) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "auth",
				Message: "Authentication is disabled - device will be accessible without credentials",
				Code:    "AUTH_DISABLED",
			})
		}
		return
	}

	// Validate username
	if auth.Username != nil {
		username := *auth.Username
		if username == "admin" || username == "user" || username == "root" {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "auth.username",
				Message: "Using common username - consider using a unique username",
				Code:    "COMMON_USERNAME",
			})
		}
	}

	// Validate password strength
	if auth.Password != nil && *auth.Password != "" {
		password := *auth.Password
		warnings := v.validatePasswordStrength(password)
		for _, warning := range warnings {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "auth.password",
				Message: warning,
				Code:    "WEAK_PASSWORD",
			})
		}

		// Check for default passwords
		defaultPasswords := []string{"admin", "password", "123456", "12345678", "shelly"}
		for _, defPass := range defaultPasswords {
			if password == defPass {
				if v.level >= ValidationLevelStrict {
					result.Errors = append(result.Errors, ValidationError{
						Field:   "auth.password",
						Message: "Default or common password detected - security risk",
						Code:    "DEFAULT_PASSWORD",
					})
					result.Valid = false
				} else {
					result.Warnings = append(result.Warnings, ValidationWarning{
						Field:   "auth.password",
						Message: "Default or common password detected - security risk",
						Code:    "DEFAULT_PASSWORD",
					})
				}
				break
			}
		}
	}
}

// validateSystem validates system configuration
func (v *ConfigurationValidator) validateSystem(system *SystemConfiguration, result *ValidationResult) {
	if system == nil {
		return
	}

	// Validate device configuration
	if system.Device != nil {
		// Check hostname validity
		if system.Device.Hostname != nil && *system.Device.Hostname != "" {
			if !isValidHostname(*system.Device.Hostname) {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "system.device.hostname",
					Message: "Invalid hostname format",
					Code:    "INVALID_HOSTNAME",
				})
				result.Valid = false
			}
		}

		// Check device name
		if system.Device.Name != nil && *system.Device.Name != "" {
			name := *system.Device.Name
			if len(name) > 64 {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "system.device.name",
					Message: "Device name too long (max 64 characters)",
					Code:    "DEVICE_NAME_TOO_LONG",
				})
				result.Valid = false
			}

			// Check for problematic characters
			if strings.ContainsAny(name, `"'<>&\n\r\t`) {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Field:   "system.device.name",
					Message: "Device name contains special characters that may cause issues",
					Code:    "SPECIAL_CHARS_DEVICE_NAME",
				})
			}
		}

		// Validate timezone
		if system.Device.Timezone != nil && *system.Device.Timezone != "" {
			if _, err := time.LoadLocation(*system.Device.Timezone); err != nil {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Field:   "system.device.timezone",
					Message: "Unknown timezone identifier",
					Code:    "UNKNOWN_TIMEZONE",
				})
			}
		}

		// Validate coordinates
		if len(system.Device.LatLon) == 2 {
			lat, lng := system.Device.LatLon[0], system.Device.LatLon[1]
			if lat == 0 && lng == 0 {
				result.Info = append(result.Info, ValidationInfo{
					Field:   "system.device.lat_lon",
					Message: "Coordinates set to null island (0,0) - verify location",
					Code:    "NULL_ISLAND_COORDINATES",
				})
			}
		}
	}

	if system.Location != nil {
		lat := Float64Val(system.Location.Latitude, 0)
		lng := Float64Val(system.Location.Longitude, 0)
		if lat == 0 && lng == 0 {
			result.Info = append(result.Info, ValidationInfo{
				Field:   "system.location",
				Message: "Location set to null island (0,0) - verify coordinates",
				Code:    "NULL_ISLAND_LOCATION",
			})
		}
	}
}

// validateNetwork validates network configuration
func (v *ConfigurationValidator) validateNetwork(network *NetworkConfiguration, result *ValidationResult) {
	if network == nil {
		return
	}

	// TODO(task-601): NetworkConfiguration needs pointer field updates
	// Temporarily skip network validation - needs full pointer conversion
	_ = network
}

// validateCloud validates cloud configuration
func (v *ConfigurationValidator) validateCloud(cloud *CloudConfiguration, result *ValidationResult) {
	if cloud == nil {
		return
	}

	if cloud.Enable != nil && *cloud.Enable {
		// Warn about cloud connectivity in production
		if v.level >= ValidationLevelProduction {
			result.Info = append(result.Info, ValidationInfo{
				Field:   "cloud",
				Message: "Cloud connectivity enabled - device will connect to external servers",
				Code:    "CLOUD_ENABLED",
			})
		}

		// Validate server URL
		if cloud.Server != nil && *cloud.Server != "" {
			if _, err := url.Parse(*cloud.Server); err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "cloud.server",
					Message: "Invalid cloud server URL",
					Code:    "INVALID_CLOUD_URL",
				})
				result.Valid = false
			}
		}
	}
}

// validateLocation validates location configuration
func (v *ConfigurationValidator) validateLocation(location *LocationConfiguration, result *ValidationResult) {
	if location == nil {
		return
	}

	if location.Timezone != nil && *location.Timezone != "" {
		if _, err := time.LoadLocation(*location.Timezone); err != nil {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "location.timezone",
				Message: "Unknown timezone identifier",
				Code:    "UNKNOWN_TIMEZONE",
			})
		}
	}

	lat := Float64Val(location.Latitude, 0)
	lng := Float64Val(location.Longitude, 0)
	if lat == 0 && lng == 0 {
		result.Info = append(result.Info, ValidationInfo{
			Field:   "location",
			Message: "Location set to null island (0,0) - verify coordinates",
			Code:    "NULL_ISLAND_LOCATION",
		})
	}
}

// validateDeviceCompatibility validates configuration against device capabilities
func (v *ConfigurationValidator) validateDeviceCompatibility(config *TypedConfiguration, result *ValidationResult) {
	// Generation-specific checks
	if v.generation == 1 {
		// Gen1 devices have different capabilities
		if config.System != nil && config.System.Device != nil && config.System.Device.BleConfig != nil {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "system.device.ble",
				Message: "BLE configuration specified but Gen1 devices do not support BLE",
				Code:    "BLE_NOT_SUPPORTED_GEN1",
			})
		}
	}

	// Model-specific checks
	switch v.deviceModel {
	case "SHSW-1", "SHSW-L", "SHSW-PM":
		// Switch devices
		if config.System != nil && config.System.Device != nil && config.System.Device.EcoMode != nil && *config.System.Device.EcoMode {
			result.Info = append(result.Info, ValidationInfo{
				Field:   "system.device.eco_mode",
				Message: "Eco mode enabled on switch device - may affect responsiveness",
				Code:    "ECO_MODE_SWITCH",
			})
		}
	case "SHDM-1", "SHDM-2":
		// Dimmer devices
		// Dimmer-specific validations
	case "SHPLG-S", "SHPLG-1":
		// Plug devices
		// Plug-specific validations
	}
}

// performSafetyChecks performs safety-related validation
func (v *ConfigurationValidator) performSafetyChecks(config *TypedConfiguration, result *ValidationResult) {
	if config.System != nil && config.System.Debug != nil && config.System.Debug.Level > 2 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "system.debug.level",
			Message: "High debug level enabled - may impact performance and expose sensitive information",
			Code:    "HIGH_DEBUG_LEVEL",
		})
	}

	// Check for open access points
	if config.WiFi != nil && config.WiFi.AccessPoint != nil && config.WiFi.AccessPoint.Enable != nil && *config.WiFi.AccessPoint.Enable {
		password := StringVal(config.WiFi.AccessPoint.Password, "")
		if password == "" || len(password) < 8 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "wifi.ap.password",
				Message: "Access point has weak or no password - security risk",
				Code:    "OPEN_ACCESS_POINT",
			})
		}
	}

	// Check for disabled authentication with external connectivity
	authDisabled := config.Auth == nil || config.Auth.Enable == nil || !*config.Auth.Enable
	hasExternalConnectivity := (config.Cloud != nil && config.Cloud.Enable != nil && *config.Cloud.Enable) ||
		(config.MQTT != nil && config.MQTT.Enable != nil && *config.MQTT.Enable)

	if authDisabled && hasExternalConnectivity && v.level >= ValidationLevelStrict {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "security",
			Message: "Authentication disabled while external connectivity is enabled - security risk",
			Code:    "AUTH_DISABLED_EXTERNAL_ACCESS",
		})
	}
}

// performProductionChecks performs production-specific validation
func (v *ConfigurationValidator) performProductionChecks(config *TypedConfiguration, result *ValidationResult) {
	// Check for development/test settings
	if config.System != nil && config.System.Device != nil && config.System.Device.Name != nil {
		name := strings.ToLower(*config.System.Device.Name)
		if strings.Contains(name, "test") || strings.Contains(name, "dev") {
			result.Info = append(result.Info, ValidationInfo{
				Field:   "system.device.name",
				Message: "Device name suggests development/test environment",
				Code:    "DEV_DEVICE_NAME",
			})
		}
	}

	// Check for auto-update settings
	if config.System != nil && config.System.Device != nil && config.System.Device.FWAutoUpdate != nil && !*config.System.Device.FWAutoUpdate {
		result.Info = append(result.Info, ValidationInfo{
			Field:   "system.device.fw_auto_update",
			Message: "Firmware auto-update disabled - manual updates required",
			Code:    "AUTO_UPDATE_DISABLED",
		})
	}

	// Check cloud settings
	if config.Cloud != nil && config.Cloud.Enable != nil && *config.Cloud.Enable {
		result.Info = append(result.Info, ValidationInfo{
			Field:   "cloud",
			Message: "Cloud connectivity enabled in production - verify data privacy requirements",
			Code:    "CLOUD_PRODUCTION",
		})
	}
}

// Helper methods

// validateIPConfiguration validates IP configuration
func (v *ConfigurationValidator) validateIPConfiguration(ipConfig *StaticIPConfig, fieldPrefix string, result *ValidationResult) {
	if ipConfig == nil {
		return
	}

	ip := net.ParseIP(StringVal(ipConfig.IP, ""))
	gateway := net.ParseIP(StringVal(ipConfig.Gateway, ""))
	netmask := net.ParseIP(StringVal(ipConfig.Netmask, ""))

	if ip == nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fieldPrefix + ".ip",
			Message: "Invalid IP address",
			Code:    "INVALID_IP",
		})
		result.Valid = false
		return
	}

	if gateway == nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fieldPrefix + ".gateway",
			Message: "Invalid gateway address",
			Code:    "INVALID_GATEWAY",
		})
		result.Valid = false
		return
	}

	if netmask == nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   fieldPrefix + ".netmask",
			Message: "Invalid netmask",
			Code:    "INVALID_NETMASK",
		})
		result.Valid = false
		return
	}

	// Check if IP and gateway are in the same subnet
	if !v.isIPInSameSubnet(ip, gateway, netmask) {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   fieldPrefix,
			Message: "IP address and gateway are not in the same subnet",
			Code:    "IP_GATEWAY_SUBNET_MISMATCH",
		})
	}

	// Check for private IP ranges in production
	if v.level >= ValidationLevelProduction && !v.isPrivateIP(ip) {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   fieldPrefix + ".ip",
			Message: "Using public IP address - verify network configuration",
			Code:    "PUBLIC_IP_USAGE",
		})
	}
}

// validatePasswordStrength validates password strength
func (v *ConfigurationValidator) validatePasswordStrength(password string) []string {
	var warnings []string

	if len(password) < 8 {
		warnings = append(warnings, "Password is shorter than 8 characters")
	}

	if len(password) < 12 && v.level >= ValidationLevelStrict {
		warnings = append(warnings, "Password should be at least 12 characters for better security")
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	complexity := 0
	if hasUpper {
		complexity++
	}
	if hasLower {
		complexity++
	}
	if hasDigit {
		complexity++
	}
	if hasSpecial {
		complexity++
	}

	if complexity < 3 {
		warnings = append(warnings, "Password should contain at least 3 of: uppercase, lowercase, digits, special characters")
	}

	// Check for common patterns - detect 3+ repeated characters
	if hasRepeatedChars(password, 3) {
		warnings = append(warnings, "Password contains repeated characters")
	}

	if regexp.MustCompile(`(012|123|234|345|456|567|678|789|890|abc|bcd|cde|def)`).MatchString(strings.ToLower(password)) {
		warnings = append(warnings, "Password contains sequential characters")
	}

	return warnings
}

// validateDangerousSettings validates potentially dangerous raw settings
func (v *ConfigurationValidator) validateDangerousSettings(config map[string]interface{}, result *ValidationResult) {
	// Check for dangerous debug settings
	if debug, ok := config["debug"].(map[string]interface{}); ok {
		if level, ok := debug["level"].(float64); ok && level > 3 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "debug.level",
				Message: "Very high debug level - may expose sensitive information",
				Code:    "HIGH_DEBUG_LEVEL",
			})
		}
	}

	// Check for open network settings
	if wifi, ok := config["wifi"].(map[string]interface{}); ok {
		if ap, ok := wifi["ap"].(map[string]interface{}); ok {
			if enable, ok := ap["enable"].(bool); ok && enable {
				if pass, ok := ap["pass"].(string); !ok || len(pass) < 8 {
					result.Warnings = append(result.Warnings, ValidationWarning{
						Field:   "wifi.ap.pass",
						Message: "Access point has weak or no password",
						Code:    "WEAK_AP_PASSWORD",
					})
				}
			}
		}
	}
}

// validateBasicNetworkSettings validates basic network settings in raw JSON
func (v *ConfigurationValidator) validateBasicNetworkSettings(config map[string]interface{}, result *ValidationResult) {
	// Validate WiFi SSID
	if wifi, ok := config["wifi"].(map[string]interface{}); ok {
		if ssid, ok := wifi["ssid"].(string); ok {
			if len(ssid) > 32 {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "wifi.ssid",
					Message: "SSID too long (max 32 characters)",
					Code:    "SSID_TOO_LONG",
				})
				result.Valid = false
			}
		}
	}

	// Validate IP addresses in static configuration
	v.validateRawIPConfig(config, "wifi", result)
	v.validateRawIPConfig(config, "eth", result)
}

// validateRawIPConfig validates IP configuration in raw JSON
func (v *ConfigurationValidator) validateRawIPConfig(config map[string]interface{}, interfaceName string, result *ValidationResult) {
	if iface, ok := config[interfaceName].(map[string]interface{}); ok {
		if ip, ok := iface["ip"].(map[string]interface{}); ok {
			if ipAddr, ok := ip["ip"].(string); ok {
				if net.ParseIP(ipAddr) == nil {
					result.Errors = append(result.Errors, ValidationError{
						Field:   fmt.Sprintf("%s.ip.ip", interfaceName),
						Message: "Invalid IP address",
						Code:    "INVALID_IP",
					})
					result.Valid = false
				}
			}

			if gateway, ok := ip["gw"].(string); ok {
				if net.ParseIP(gateway) == nil {
					result.Errors = append(result.Errors, ValidationError{
						Field:   fmt.Sprintf("%s.ip.gw", interfaceName),
						Message: "Invalid gateway address",
						Code:    "INVALID_GATEWAY",
					})
					result.Valid = false
				}
			}
		}
	}
}

// Helper utility methods

// hasCapability checks if device has a specific capability
func (v *ConfigurationValidator) hasCapability(capability string) bool {
	for _, cap := range v.capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// isIPInSameSubnet checks if IP and gateway are in the same subnet
func (v *ConfigurationValidator) isIPInSameSubnet(ip, gateway, netmask net.IP) bool {
	mask := net.IPMask(netmask.To4())
	ipNet := &net.IPNet{IP: ip.Mask(mask), Mask: mask}
	return ipNet.Contains(gateway)
}

// isPrivateIP checks if IP is in private range
func (v *ConfigurationValidator) isPrivateIP(ip net.IP) bool {
	// RFC 1918 private ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// GetValidationSummary returns a summary of validation results
func (r *ValidationResult) GetValidationSummary() string {
	if r.Valid {
		if len(r.Warnings) == 0 && len(r.Info) == 0 {
			return "Configuration is valid with no issues"
		}
		return fmt.Sprintf("Configuration is valid with %d warnings and %d info messages", len(r.Warnings), len(r.Info))
	}
	return fmt.Sprintf("Configuration is invalid with %d errors, %d warnings", len(r.Errors), len(r.Warnings))
}

// hasRepeatedChars checks if a string contains n or more consecutive repeated characters
func hasRepeatedChars(s string, n int) bool {
	if len(s) < n {
		return false
	}

	count := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			count++
			if count >= n {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

// validateTemplates validates template syntax in configuration
func (v *ConfigurationValidator) validateTemplates(config json.RawMessage, result *ValidationResult) {
	configStr := string(config)

	// Check if configuration contains template variables
	if !containsTemplateVars(configStr) {
		return
	}

	// Extract template variables
	templateVars := extractTemplateVars(configStr)
	if len(templateVars) == 0 {
		return
	}

	// Create template functions for validation
	funcMap := template.FuncMap{}

	// Add safe Sprig functions
	sprigFuncs := sprig.TxtFuncMap()
	for name, fn := range sprigFuncs {
		if isSafeTemplateFunction(name) {
			funcMap[name] = fn
		}
	}

	// Add custom IoT functions
	customFunctions := map[string]interface{}{
		"macColon":        func(string) string { return "" },
		"macDash":         func(string) string { return "" },
		"macNone":         func(string) string { return "" },
		"macLast4":        func(string) string { return "" },
		"macLast6":        func(string) string { return "" },
		"deviceShortName": func(string, string) string { return "" },
		"deviceUnique":    func(string, string) string { return "" },
		"networkName":     func(string) string { return "" },
		"hostName":        func(string) string { return "" },
		"env":             func(string) string { return "" },
		"envOr":           func(string, string) string { return "" },
		"empty":           func(interface{}) bool { return false },
		"requiredMsg":     func(interface{}, string) (interface{}, error) { return nil, nil },
	}

	for name, fn := range customFunctions {
		funcMap[name] = fn
	}

	// For complex templates (with if/else/end), skip detailed syntax validation
	// Focus on security and variable validation instead
	hasComplexTemplates := strings.Contains(configStr, "{{if") ||
		strings.Contains(configStr, "{{else") ||
		strings.Contains(configStr, "{{end")

	if !hasComplexTemplates {
		// Validate individual simple template expressions
		for _, tmplVar := range templateVars {
			// Skip syntax validation if this template contains dangerous functions
			// The dangerous function validation will catch these
			containsDangerous := strings.Contains(tmplVar, "exec ") || strings.Contains(tmplVar, "exec\"") || strings.Contains(tmplVar, "exec(") ||
				strings.Contains(tmplVar, "shell ") || strings.Contains(tmplVar, "shell\"") || strings.Contains(tmplVar, "shell(") ||
				strings.Contains(tmplVar, "command ") || strings.Contains(tmplVar, "command\"") || strings.Contains(tmplVar, "command(") ||
				strings.Contains(tmplVar, "readFile ") || strings.Contains(tmplVar, "readFile\"") || strings.Contains(tmplVar, "readFile(") ||
				strings.Contains(tmplVar, "writeFile ") || strings.Contains(tmplVar, "writeFile\"") || strings.Contains(tmplVar, "writeFile(")

			// Also skip templates with JSON-escaped quotes which cause parsing issues
			hasEscapedQuotes := strings.Contains(tmplVar, "\\\"")

			if containsDangerous || hasEscapedQuotes {
				continue // Skip syntax validation for problematic templates
			}

			// Create a simple template to validate each expression
			testTemplate := tmplVar
			_, err := template.New("validation").Funcs(funcMap).Parse(testTemplate)
			if err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Field:   "template_syntax",
					Message: fmt.Sprintf("Invalid template syntax: %v", err),
					Code:    "TEMPLATE_SYNTAX_ERROR",
				})
				continue // Continue checking other templates
			}
		}
	}

	// Validate template variable references
	v.validateTemplateVariables(templateVars, result)
}

// validateTemplateVariables validates that template variables are properly formed
func (v *ConfigurationValidator) validateTemplateVariables(templateVars []string, result *ValidationResult) {
	// Track reported custom variables to avoid duplicates
	reportedCustomVars := make(map[string]bool)
	reportedEnvVars := make(map[string]bool)

	for _, tmplVar := range templateVars {
		// Check for potentially dangerous template patterns (function calls)
		dangerousPatterns := []string{
			"exec ", "exec\"", "exec(",
			"shell ", "shell\"", "shell(",
			"command ", "command\"", "command(",
			"readFile ", "readFile\"", "readFile(",
			"writeFile ", "writeFile\"", "writeFile(",
			"httpGet ", "httpGet\"", "httpGet(",
			"httpPost ", "httpPost\"", "httpPost(",
		}

		for _, pattern := range dangerousPatterns {
			if strings.Contains(tmplVar, pattern) {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "template_variables",
					Message: fmt.Sprintf("Template contains potentially dangerous function: %s", tmplVar),
					Code:    "DANGEROUS_TEMPLATE_FUNCTION",
				})
				result.Valid = false
				break // Only report once per template variable
			}
		}

		// Check for undefined variable references
		if strings.Contains(tmplVar, ".Custom.") {
			// Extract custom variable name
			parts := strings.Split(tmplVar, ".Custom.")
			if len(parts) > 1 {
				customVar := strings.Split(parts[1], " ")[0]
				customVar = strings.Split(customVar, ")")[0]
				customVar = strings.Split(customVar, "|")[0]
				customVar = strings.TrimSpace(customVar)

				if customVar != "" && !reportedCustomVars[customVar] {
					reportedCustomVars[customVar] = true
					result.Warnings = append(result.Warnings, ValidationWarning{
						Field:   "template_variables",
						Message: fmt.Sprintf("Custom variable referenced: %s - ensure it's provided during substitution", customVar),
						Code:    "CUSTOM_VARIABLE_REFERENCE",
					})
				}
			}
		}

		// Check for required environment variables
		if strings.Contains(tmplVar, "env ") || strings.Contains(tmplVar, "envOr ") {
			// Extract environment variable name for deduplication
			envVar := tmplVar
			if !reportedEnvVars[envVar] {
				reportedEnvVars[envVar] = true
				result.Info = append(result.Info, ValidationInfo{
					Field:   "template_variables",
					Message: fmt.Sprintf("Template references environment variables: %s", tmplVar),
					Code:    "ENV_VARIABLE_REFERENCE",
				})
			}
		}
	}
}

// isSafeTemplateFunction checks if a Sprig function is safe to use
func isSafeTemplateFunction(funcName string) bool {
	// List of dangerous functions to exclude
	dangerousFunctions := []string{
		"readFile", "writeFile", "glob",
		"exec", "shell", "command",
		"httpGet", "httpPost", "httpPut", "httpDelete",
		"getHostByName", "env", // We provide our own env function
	}

	for _, dangerous := range dangerousFunctions {
		if funcName == dangerous {
			return false
		}
	}
	return true
}

// validatePartialConfig performs partial validation on template configurations
// focusing on literal values that can be validated without template substitution
func (v *ConfigurationValidator) validatePartialConfig(config json.RawMessage, result *ValidationResult) {
	// Try to parse as JSON to extract literal values
	var configMap map[string]interface{}
	if err := json.Unmarshal(config, &configMap); err != nil {
		// If we can't parse as JSON (maybe templates break it), skip partial validation
		return
	}

	// Validate literal password values if present
	if wifi, ok := configMap["wifi"].(map[string]interface{}); ok {
		if password, ok := wifi["password"].(string); ok {
			// Only validate if it's not a template
			if !containsTemplateVars(password) {
				if len(password) < 8 {
					result.Warnings = append(result.Warnings, ValidationWarning{
						Field:   "wifi.password",
						Message: "WiFi password is weak (less than 8 characters)",
						Code:    "WEAK_WIFI_PASSWORD",
					})
				}
			}
		}
	}

	// Validate auth settings
	if auth, ok := configMap["auth"].(map[string]interface{}); ok {
		if enable, ok := auth["enable"].(bool); ok && !enable {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "auth.enable",
				Message: "Device authentication is disabled - consider enabling for security",
				Code:    "AUTH_DISABLED",
			})
		}
	}
}

// validateTemplateBasics performs basic validation for template configurations
func (v *ConfigurationValidator) validateTemplateBasics(config json.RawMessage, result *ValidationResult) {
	// For template configurations, we can only do basic structural checks
	// since the JSON may not be valid until after template processing

	configStr := string(config)

	// Check for basic JSON structure (balanced braces)
	braceCount := 0
	inString := false
	escaped := false

	for i, char := range configStr {
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' && !escaped {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount < 0 {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   "json_structure",
						Message: fmt.Sprintf("Unmatched closing brace at position %d", i),
						Code:    "INVALID_JSON_STRUCTURE",
					})
					return
				}
			}
		}
	}

	if braceCount != 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "json_structure",
			Message: fmt.Sprintf("Unmatched opening braces: %d", braceCount),
			Code:    "INVALID_JSON_STRUCTURE",
		})
	}
}

// Note: Template helper functions containsTemplateVars and extractTemplateVars
// are defined in template_engine.go to avoid duplication

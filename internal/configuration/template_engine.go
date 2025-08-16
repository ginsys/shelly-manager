package configuration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TemplateEngine handles variable substitution in device configurations
type TemplateEngine struct {
	logger *logging.Logger
	funcs  template.FuncMap
}

// TemplateContext contains variables available for template substitution
type TemplateContext struct {
	// Device-specific variables
	Device struct {
		ID         uint   `json:"id"`
		MAC        string `json:"mac"`
		IP         string `json:"ip"`
		Name       string `json:"name"`
		Model      string `json:"model"`
		Generation int    `json:"generation"`
		Firmware   string `json:"firmware"`
	} `json:"device"`

	// Network information
	Network struct {
		SSID    string `json:"ssid"`
		Gateway string `json:"gateway"`
		Subnet  string `json:"subnet"`
		DNS     string `json:"dns"`
	} `json:"network"`

	// System variables
	System struct {
		Timestamp   time.Time `json:"timestamp"`
		ConfigHash  string    `json:"config_hash"`
		Environment string    `json:"environment"`
		Version     string    `json:"version"`
	} `json:"system"`

	// Custom variables from configuration templates
	Custom map[string]interface{} `json:"custom"`

	// Authentication credentials
	Auth struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Realm    string `json:"realm"`
	} `json:"auth"`

	// Location and time
	Location struct {
		Timezone  string  `json:"timezone"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		NTPServer string  `json:"ntp_server"`
	} `json:"location"`
}

// NewTemplateEngine creates a new template engine with built-in functions
func NewTemplateEngine(logger *logging.Logger) *TemplateEngine {
	engine := &TemplateEngine{
		logger: logger,
		funcs:  make(template.FuncMap),
	}

	// Add built-in template functions
	engine.addBuiltinFunctions()

	return engine
}

// addBuiltinFunctions adds useful template functions for device configuration
func (te *TemplateEngine) addBuiltinFunctions() {
	te.funcs = template.FuncMap{
		// String manipulation
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"title":     strings.Title,
		"trim":      strings.TrimSpace,
		"replace":   strings.ReplaceAll,
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,

		// MAC address formatting
		"macColon": formatMACColon,
		"macDash":  formatMACDash,
		"macNone":  formatMACNone,
		"macLast4": getMACLast4,
		"macLast6": getMACLast6,

		// IP address manipulation
		"ipOctets": getIPOctets,
		"ipLast":   getLastIPOctet,

		// Device naming helpers
		"deviceShortName": generateDeviceShortName,
		"deviceUnique":    generateUniqueDeviceName,

		// Time and date formatting
		"now":        time.Now,
		"formatTime": formatTime,
		"timestamp":  func() int64 { return time.Now().Unix() },

		// Validation helpers
		"required": requireValue,
		"default":  defaultValue,

		// Network helpers
		"networkName": generateNetworkName,
		"hostName":    generateHostName,

		// JSON helpers
		"toJson":   toJSON,
		"fromJson": fromJSON,

		// Math helpers
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
	}
}

// SubstituteVariables performs template variable substitution on configuration data
func (te *TemplateEngine) SubstituteVariables(configData json.RawMessage, context *TemplateContext) (json.RawMessage, error) {
	// Convert JSON to string for template processing
	configStr := string(configData)

	// Check if there are any template variables to substitute
	if !containsTemplateVars(configStr) {
		te.logger.WithFields(map[string]any{
			"device_id": context.Device.ID,
			"component": "template",
		}).Debug("No template variables found in configuration")
		return configData, nil
	}

	te.logger.WithFields(map[string]any{
		"device_id":     context.Device.ID,
		"device_mac":    context.Device.MAC,
		"template_vars": extractTemplateVars(configStr),
		"component":     "template",
	}).Info("Processing template variables in configuration")

	// Create and parse template
	tmpl, err := template.New("config").Funcs(te.funcs).Parse(configStr)
	if err != nil {
		te.logger.WithFields(map[string]any{
			"device_id": context.Device.ID,
			"error":     err.Error(),
			"component": "template",
		}).Error("Failed to parse configuration template")
		return nil, fmt.Errorf("template parsing error: %w", err)
	}

	// Execute template with context
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		te.logger.WithFields(map[string]any{
			"device_id": context.Device.ID,
			"error":     err.Error(),
			"component": "template",
		}).Error("Failed to execute configuration template")
		return nil, fmt.Errorf("template execution error: %w", err)
	}

	result := buf.Bytes()

	// Validate that result is still valid JSON
	if !json.Valid(result) {
		te.logger.WithFields(map[string]any{
			"device_id": context.Device.ID,
			"result":    string(result),
			"component": "template",
		}).Error("Template substitution resulted in invalid JSON")
		return nil, fmt.Errorf("template substitution resulted in invalid JSON")
	}

	te.logger.WithFields(map[string]any{
		"device_id":     context.Device.ID,
		"original_size": len(configData),
		"result_size":   len(result),
		"component":     "template",
	}).Info("Template variable substitution completed successfully")

	return json.RawMessage(result), nil
}

// CreateTemplateContext creates a template context from device and system information
func (te *TemplateEngine) CreateTemplateContext(device *Device, variables map[string]interface{}) *TemplateContext {
	context := &TemplateContext{
		Custom: make(map[string]interface{}),
	}

	// Populate device information
	if device != nil {
		context.Device.ID = device.ID
		context.Device.MAC = device.MAC
		context.Device.IP = device.IP
		context.Device.Name = device.Name

		// Extract model, generation, and firmware from settings JSON
		if device.Settings != "" {
			var settings map[string]interface{}
			if err := json.Unmarshal([]byte(device.Settings), &settings); err == nil {
				if model, ok := settings["model"].(string); ok {
					context.Device.Model = model
				}
				if gen, ok := settings["gen"].(float64); ok {
					context.Device.Generation = int(gen)
				}
				if firmware, ok := settings["fw_id"].(string); ok {
					context.Device.Firmware = firmware
				} else if firmware, ok := settings["firmware"].(string); ok {
					context.Device.Firmware = firmware
				}
			}
		}

		// Use Type as Model if Model is not available
		if context.Device.Model == "" {
			context.Device.Model = device.Type
		}
	}

	// Populate system information
	context.System.Timestamp = time.Now()
	context.System.Environment = "production" // Could be configurable
	context.System.Version = "1.0.0"          // Could be injected from build

	// Populate custom variables
	if variables != nil {
		for key, value := range variables {
			context.Custom[key] = value
		}
	}

	// Set default values for network if not provided
	if context.Network.DNS == "" {
		context.Network.DNS = "8.8.8.8"
	}

	// Set default timezone if not provided
	if context.Location.Timezone == "" {
		context.Location.Timezone = "UTC"
	}
	if context.Location.NTPServer == "" {
		context.Location.NTPServer = "pool.ntp.org"
	}

	return context
}

// Template helper functions

func containsTemplateVars(text string) bool {
	// Check for {{ }} template syntax
	matched, _ := regexp.MatchString(`\{\{.*\}\}`, text)
	return matched
}

func extractTemplateVars(text string) []string {
	re := regexp.MustCompile(`\{\{[^}]+\}\}`)
	matches := re.FindAllString(text, -1)

	// Remove duplicates
	seen := make(map[string]bool)
	result := []string{}
	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			result = append(result, match)
		}
	}

	return result
}

func formatMACColon(mac string) string {
	// Convert MAC to standard colon format
	mac = strings.ReplaceAll(mac, "-", ":")
	mac = strings.ReplaceAll(mac, ".", "")
	if len(mac) == 12 {
		// Add colons to 12-character MAC
		return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			mac[0:2], mac[2:4], mac[4:6], mac[6:8], mac[8:10], mac[10:12])
	}
	return mac
}

func formatMACDash(mac string) string {
	return strings.ReplaceAll(formatMACColon(mac), ":", "-")
}

func formatMACNone(mac string) string {
	return strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", "")
}

func getMACLast4(mac string) string {
	clean := formatMACNone(mac)
	if len(clean) >= 4 {
		return clean[len(clean)-4:]
	}
	return clean
}

func getMACLast6(mac string) string {
	clean := formatMACNone(mac)
	if len(clean) >= 6 {
		return clean[len(clean)-6:]
	}
	return clean
}

func getIPOctets(ip string) []string {
	return strings.Split(ip, ".")
}

func getLastIPOctet(ip string) string {
	octets := strings.Split(ip, ".")
	if len(octets) > 0 {
		return octets[len(octets)-1]
	}
	return ""
}

func generateDeviceShortName(model, mac string) string {
	shortMAC := getMACLast4(mac)
	// Clean up model name to remove generation-specific parts
	cleanModel := model
	if strings.Contains(model, "-") {
		parts := strings.Split(model, "-")
		if len(parts) > 0 {
			cleanModel = parts[0]
		}
	}
	return fmt.Sprintf("%s-%s", cleanModel, shortMAC)
}

func generateUniqueDeviceName(model, mac string) string {
	shortMAC := getMACLast6(mac)
	return fmt.Sprintf("%s-%s", model, shortMAC)
}

func generateNetworkName(ssid string) string {
	// Sanitize SSID for use in device names
	name := strings.ReplaceAll(ssid, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return strings.ToLower(name)
}

func generateHostName(deviceName string) string {
	// Convert device name to valid hostname
	hostname := strings.ToLower(deviceName)
	hostname = strings.ReplaceAll(hostname, " ", "-")
	hostname = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(hostname, "")
	return hostname
}

func formatTime(format string, t time.Time) string {
	return t.Format(format)
}

func requireValue(value interface{}) (interface{}, error) {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.String && value.(string) == "") {
		return nil, fmt.Errorf("required value is missing or empty")
	}
	return value, nil
}

func defaultValue(value, defaultVal interface{}) interface{} {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.String && value.(string) == "") {
		return defaultVal
	}
	return value
}

func toJSON(value interface{}) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func fromJSON(jsonStr string) (interface{}, error) {
	var result interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	return result, err
}

// ValidateTemplate validates a template string without executing it
func (te *TemplateEngine) ValidateTemplate(templateStr string) error {
	_, err := template.New("validation").Funcs(te.funcs).Parse(templateStr)
	return err
}

// GetAvailableFunctions returns a list of available template functions
func (te *TemplateEngine) GetAvailableFunctions() []string {
	functions := make([]string, 0, len(te.funcs))
	for name := range te.funcs {
		functions = append(functions, name)
	}
	return functions
}

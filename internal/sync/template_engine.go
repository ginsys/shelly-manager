package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// AdvancedTemplateEngine extends the basic template engine with export-specific functions
type AdvancedTemplateEngine struct {
	logger       *logging.Logger
	funcs        template.FuncMap
	cache        map[string]*template.Template
	externalAPIs map[string]ExternalAPIFunc
}

// ExternalAPIFunc represents a function that can fetch data from external APIs
type ExternalAPIFunc func(params map[string]interface{}) (interface{}, error)

// TemplateConfig defines a template with its metadata and content
type TemplateConfig struct {
	Name         string                 `yaml:"name" json:"name"`
	Description  string                 `yaml:"description" json:"description"`
	Version      string                 `yaml:"version" json:"version"`
	Templates    map[string]string      `yaml:"templates" json:"templates"`
	Functions    map[string]string      `yaml:"functions,omitempty" json:"functions,omitempty"`
	Variables    map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`
	Validation   ValidationRules        `yaml:"validation,omitempty" json:"validation,omitempty"`
	Dependencies []string               `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
}

// ValidationRules defines validation rules for template output
type ValidationRules struct {
	RequiredFields []string `yaml:"required_fields,omitempty" json:"required_fields,omitempty"`
	MaxRecords     int      `yaml:"max_records,omitempty" json:"max_records,omitempty"`
	MinRecords     int      `yaml:"min_records,omitempty" json:"min_records,omitempty"`
	OutputFormat   string   `yaml:"output_format,omitempty" json:"output_format,omitempty"`
}

// NewAdvancedTemplateEngine creates a new advanced template engine
func NewAdvancedTemplateEngine(logger *logging.Logger) *AdvancedTemplateEngine {
	engine := &AdvancedTemplateEngine{
		logger:       logger,
		funcs:        make(template.FuncMap),
		cache:        make(map[string]*template.Template),
		externalAPIs: make(map[string]ExternalAPIFunc),
	}

	engine.addBuiltinFunctions()
	engine.addExportFunctions()
	return engine
}

// addBuiltinFunctions adds standard and Sprig template functions
func (ate *AdvancedTemplateEngine) addBuiltinFunctions() {
	// Start with safe Sprig functions
	safeFunctions := ate.getSafeSprigFunctions()
	for name, fn := range safeFunctions {
		ate.funcs[name] = fn
	}

	// Add basic utility functions
	basicFunctions := template.FuncMap{
		// String manipulation
		"sanitize": sanitizeString,
		"truncate": func(arg1, arg2 interface{}) string {
			var s string
			var length int
			var ok bool

			// Try to determine which argument is which
			if s, ok = arg1.(string); ok {
				// First arg is string, second should be length
				if length, ok = arg2.(int); ok {
					if len(s) <= length {
						return s
					}
					return s[:length]
				}
			} else if length, ok = arg1.(int); ok {
				// First arg is length, second should be string (pipeline case)
				if s, ok = arg2.(string); ok {
					if len(s) <= length {
						return s
					}
					return s[:length]
				}
			}
			return ""
		},
		"padLeft":      padStringLeft,
		"padRight":     padStringRight,
		"regexMatch":   regexMatch,
		"regexReplace": regexReplace,

		// Number formatting
		"formatInt":   formatInteger,
		"formatFloat": formatFloat,
		"parseNumber": parseNumber,

		// Date/time functions
		"timestamp":  getCurrentTimestamp,
		"formatTime": formatTimestamp,
		"timeAdd":    addTimeToTimestamp,
		"timeSub":    subtractTimeFromTimestamp,

		// Collection operations
		"sortBy":   sortByField,
		"groupBy":  groupByField,
		"filterBy": filterByCondition,
		"distinct": getDistinctValues,

		// JSON operations
		"jsonMarshal":   jsonMarshal,
		"jsonUnmarshal": jsonUnmarshal,
		"jsonPath":      jsonPath,
		"jsonSet":       jsonSet,
	}

	for name, fn := range basicFunctions {
		ate.funcs[name] = fn
	}
}

// addExportFunctions adds export-specific template functions
func (ate *AdvancedTemplateEngine) addExportFunctions() {
	exportFunctions := template.FuncMap{
		// Network operations
		"networkAddress":  getNetworkAddress,
		"broadcastAddr":   getBroadcastAddress,
		"subnetMask":      getSubnetMask,
		"isInNetwork":     isIPInNetwork,
		"resolveHostname": resolveHostnameToIP,
		"pingHost":        pingHost,

		// MAC address operations
		"macToEUI64":      macToEUI64,
		"macOUI":          getMACOUI,
		"macManufacturer": getMACManufacturer,
		"macValidate":     validateMAC,
		"macGenerate":     generateMAC,

		// Device operations
		"deviceType":            determineDeviceType,
		"checkDeviceCapability": checkDeviceCapability,
		"deviceCapability":      checkDeviceCapability,
		"deviceGroup":           assignDeviceGroup,
		"devicePriority":        assignDevicePriority,

		// Configuration generation
		"dhcpConfig":   generateDHCPConfig,
		"dnsConfig":    generateDNSConfig,
		"firewallRule": generateFirewallRule,
		"routingEntry": generateRoutingEntry,

		// Data transformation
		"csvFormat":  formatAsCSV,
		"xmlFormat":  formatAsXML,
		"yamlFormat": formatAsYAML,
		"iniFormat":  formatAsINI,
		"tomlFormat": formatAsTOML,

		// Conditional logic
		"ifThen":     conditionalValue,
		"ifThenElse": conditionalValueWithElse,
		"switchCase": switchCaseValue,
		"coalesce":   coalesceValues,

		// Loop constructs
		"range":   rangeValues,
		"forEach": forEachItem,
		"repeat":  repeatValue,

		// External data sources
		"apiCall":  ate.callExternalAPI,
		"dbQuery":  ate.executeDatabaseQuery,
		"fileRead": readFileContent,

		// Validation functions
		"validate": validateValue,
		"required": requireValue,
		"oneOf":    oneOfValues,
		"between":  betweenValues,

		// Encoding/Decoding
		"base64Encode": base64Encode,
		"base64Decode": base64Decode,
		"urlEncode":    urlEncode,
		"urlDecode":    urlDecode,
		"htmlEscape":   htmlEscape,
		"htmlUnescape": htmlUnescape,
	}

	for name, fn := range exportFunctions {
		ate.funcs[name] = fn
	}
}

// RenderTemplate renders a template with the given data
func (ate *AdvancedTemplateEngine) RenderTemplate(templateContent string, data interface{}) (string, error) {
	// Create template with custom functions
	tmpl, err := template.New("export").Funcs(ate.funcs).Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderTemplateConfig renders a template configuration
func (ate *AdvancedTemplateEngine) RenderTemplateConfig(config TemplateConfig, templateName string, data interface{}) (string, error) {
	templateContent, exists := config.Templates[templateName]
	if !exists {
		return "", fmt.Errorf("template '%s' not found in configuration", templateName)
	}

	// Add template variables to data
	if len(config.Variables) > 0 {
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			// Convert to map if it's not already
			jsonData, err := json.Marshal(data)
			if err != nil {
				return "", fmt.Errorf("failed to marshal data: %w", err)
			}

			dataMap = make(map[string]interface{})
			if err := json.Unmarshal(jsonData, &dataMap); err != nil {
				return "", fmt.Errorf("failed to unmarshal data: %w", err)
			}
		}

		// Add template variables
		for key, value := range config.Variables {
			dataMap[key] = value
		}
		data = dataMap
	}

	result, err := ate.RenderTemplate(templateContent, data)
	if err != nil {
		return "", err
	}

	// Validate result if rules are defined
	if err := ate.validateResult(result, config.Validation); err != nil {
		return "", fmt.Errorf("template validation failed: %w", err)
	}

	return result, nil
}

// RegisterExternalAPI registers an external API function
func (ate *AdvancedTemplateEngine) RegisterExternalAPI(name string, fn ExternalAPIFunc) {
	ate.externalAPIs[name] = fn
}

// Template function implementations

// getSafeSprigFunctions returns safe Sprig functions (excluding dangerous ones)
func (ate *AdvancedTemplateEngine) getSafeSprigFunctions() template.FuncMap {
	// Get all Sprig functions
	allFunctions := sprig.TxtFuncMap()

	// Remove dangerous functions
	dangerousFunctions := []string{
		"env",           // Environment access
		"expandenv",     // Environment expansion
		"exec",          // Command execution
		"genPrivateKey", // Key generation
		"getHostByName", // Network access
	}

	for _, dangerous := range dangerousFunctions {
		delete(allFunctions, dangerous)
	}

	return allFunctions
}

// String manipulation functions
func sanitizeString(s string) string {
	// Remove or replace characters that might be problematic
	reg := regexp.MustCompile(`[^\w\s\-\.]`)
	return reg.ReplaceAllString(s, "")
}

func padStringLeft(s string, width int, pad string) string {
	if len(s) >= width {
		return s
	}
	padding := strings.Repeat(pad, (width-len(s))/len(pad)+1)
	return padding[:width-len(s)] + s
}

func padStringRight(s string, width int, pad string) string {
	if len(s) >= width {
		return s
	}
	padding := strings.Repeat(pad, (width-len(s))/len(pad)+1)
	return s + padding[:width-len(s)]
}

func regexMatch(pattern, s string) bool {
	matched, _ := regexp.MatchString(pattern, s)
	return matched
}

func regexReplace(pattern, replacement, s string) string {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return s
	}
	return reg.ReplaceAllString(s, replacement)
}

// Network functions
func getNetworkAddress(ip, mask string) string {
	ipAddr := net.ParseIP(ip)
	maskAddr := net.ParseIP(mask)
	if ipAddr == nil || maskAddr == nil {
		return ""
	}

	network := ipAddr.Mask(net.IPMask(maskAddr.To4()))
	return network.String()
}

func getBroadcastAddress(ip, mask string) string {
	ipAddr := net.ParseIP(ip)
	maskAddr := net.ParseIP(mask)
	if ipAddr == nil || maskAddr == nil {
		return ""
	}

	network := ipAddr.Mask(net.IPMask(maskAddr.To4()))
	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = network[i] | ^maskAddr.To4()[i]
	}
	return broadcast.String()
}

func getSubnetMask(cidr string) string {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}

	mask := network.Mask
	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
}

func isIPInNetwork(ip, cidr string) bool {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false
	}

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	return network.Contains(ipAddr)
}

func resolveHostnameToIP(hostname string) string {
	ips, err := net.LookupIP(hostname)
	if err != nil || len(ips) == 0 {
		return ""
	}
	return ips[0].String()
}

func pingHost(hostname string) bool {
	// This is a placeholder - actual ping implementation would require system calls
	_, err := net.LookupIP(hostname)
	return err == nil
}

// MAC address functions
func macToEUI64(mac string) string {
	// Convert MAC address to EUI-64 format
	normalized := strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", "")
	if len(normalized) != 12 {
		return ""
	}

	return fmt.Sprintf("%s:%s:%s:ff:fe:%s:%s:%s",
		normalized[0:2], normalized[2:4], normalized[4:6],
		normalized[6:8], normalized[8:10], normalized[10:12])
}

func getMACOUI(mac string) string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", "")
	if len(normalized) < 6 {
		return ""
	}
	return strings.ToUpper(normalized[0:6])
}

func getMACManufacturer(mac string) string {
	// This would typically lookup OUI in a database
	// For now, return OUI with common manufacturers
	oui := getMACOUI(mac)
	manufacturers := map[string]string{
		"8CAAB5": "Allterco Robotics Ltd (Shelly)",
		"C45BBE": "Shelly Devices",
		"000000": "Unknown",
	}

	if manufacturer, exists := manufacturers[oui]; exists {
		return manufacturer
	}
	return "Unknown (" + oui + ")"
}

func validateMAC(mac string) bool {
	_, err := net.ParseMAC(mac)
	return err == nil
}

func generateMAC(oui string) string {
	// Generate random MAC with specific OUI
	if len(oui) != 6 {
		oui = "000000"
	}

	// Generate random lower 3 bytes
	return fmt.Sprintf("%s:%02x:%02x:%02x",
		oui[0:2]+":"+oui[2:4]+":"+oui[4:6],
		time.Now().Unix()%256,
		time.Now().UnixNano()%256,
		time.Now().Nanosecond()%256)
}

// Device classification functions
func determineDeviceType(model, name string) string {
	model = strings.ToLower(model)
	name = strings.ToLower(name)

	if strings.Contains(model, "plus1") || strings.Contains(name, "switch") {
		return "switch"
	} else if strings.Contains(model, "plus2") || strings.Contains(name, "relay") {
		return "relay"
	} else if strings.Contains(model, "dimmer") || strings.Contains(name, "dimmer") {
		return "dimmer"
	} else if strings.Contains(model, "plug") || strings.Contains(name, "plug") {
		return "plug"
	}
	return "unknown"
}

func checkDeviceCapability(deviceType, capability string) bool {
	capabilities := map[string][]string{
		"switch": {"on_off", "power_monitoring"},
		"relay":  {"on_off", "power_monitoring", "dual_channel"},
		"dimmer": {"on_off", "dimming", "power_monitoring"},
		"plug":   {"on_off", "power_monitoring"},
	}

	if caps, exists := capabilities[deviceType]; exists {
		for _, cap := range caps {
			if cap == capability {
				return true
			}
		}
	}
	return false
}

// Configuration generation functions
func generateDHCPConfig(devices []interface{}) string {
	var config strings.Builder
	for _, device := range devices {
		if deviceMap, ok := device.(map[string]interface{}); ok {
			mac := getString(deviceMap, "mac")
			ip := getString(deviceMap, "ip")
			hostname := getString(deviceMap, "hostname")

			if mac != "" && ip != "" && hostname != "" {
				config.WriteString(fmt.Sprintf("host %s { hardware ethernet %s; fixed-address %s; }\n",
					hostname, mac, ip))
			}
		}
	}
	return config.String()
}

// Conditional logic functions
func conditionalValue(condition bool, trueValue interface{}) interface{} {
	if condition {
		return trueValue
	}
	return ""
}

func conditionalValueWithElse(condition bool, trueValue, falseValue interface{}) interface{} {
	if condition {
		return trueValue
	}
	return falseValue
}

// External API and database functions (placeholders)
func (ate *AdvancedTemplateEngine) callExternalAPI(apiName string, params map[string]interface{}) (interface{}, error) {
	if fn, exists := ate.externalAPIs[apiName]; exists {
		return fn(params)
	}
	return nil, fmt.Errorf("external API '%s' not registered", apiName)
}

func (ate *AdvancedTemplateEngine) executeDatabaseQuery(query string, params ...interface{}) (interface{}, error) {
	// Placeholder for database query execution
	return nil, fmt.Errorf("database query not implemented")
}

// Utility functions
func getString(m map[string]interface{}, key string) string {
	if value, exists := m[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func getCurrentTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

func formatTimestamp(t interface{}, format string) string {
	switch v := t.(type) {
	case time.Time:
		return v.Format(format)
	case string:
		if parsedTime, err := time.Parse(time.RFC3339, v); err == nil {
			return parsedTime.Format(format)
		}
		return v
	default:
		return fmt.Sprintf("%v", t)
	}
}

func jsonMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func jsonUnmarshal(s string) interface{} {
	var result interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil
	}
	return result
}

// Formatting functions
func formatAsCSV(data []interface{}) string {
	// Implementation for CSV formatting
	return "CSV format not implemented"
}

func formatAsXML(data interface{}) string {
	// Implementation for XML formatting
	return "XML format not implemented"
}

func formatAsYAML(data interface{}) string {
	// Implementation for YAML formatting
	return "YAML format not implemented"
}

// Additional utility functions would be implemented here
func validateValue(value interface{}, rules map[string]interface{}) bool {
	return true // Placeholder
}

func base64Encode(s string) string {
	// Implementation would use base64 encoding
	return s
}

func (ate *AdvancedTemplateEngine) validateResult(result string, rules ValidationRules) error {
	// Implement validation logic here
	return nil
}

// Additional function stubs for completeness
var (
	formatInteger             = func(i int) string { return strconv.Itoa(i) }
	formatFloat               = func(f float64) string { return fmt.Sprintf("%.2f", f) }
	parseNumber               = func(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
	addTimeToTimestamp        = func(t time.Time, d time.Duration) time.Time { return t.Add(d) }
	subtractTimeFromTimestamp = func(t time.Time, d time.Duration) time.Time { return t.Add(-d) }
	sortByField               = func(slice []interface{}, field string) []interface{} { return slice }
	groupByField              = func(slice []interface{}, field string) map[string][]interface{} { return nil }
	filterByCondition         = func(slice []interface{}, condition func(interface{}) bool) []interface{} { return slice }
	getDistinctValues         = func(slice []interface{}) []interface{} { return slice }
	jsonPath                  = func(data interface{}, path string) interface{} { return nil }
	jsonSet                   = func(data interface{}, path string, value interface{}) interface{} { return data }
	generateDNSConfig         = func(devices []interface{}) string { return "" }
	generateFirewallRule      = func(rule map[string]interface{}) string { return "" }
	generateRoutingEntry      = func(route map[string]interface{}) string { return "" }
	formatAsINI               = func(data interface{}) string { return "" }
	formatAsTOML              = func(data interface{}) string { return "" }
	switchCaseValue           = func(value interface{}, cases map[interface{}]interface{}) interface{} { return nil }
	coalesceValues            = func(values ...interface{}) interface{} { return nil }
	rangeValues               = func(start, end int) []int {
		var result []int
		for i := start; i <= end; i++ {
			result = append(result, i)
		}
		return result
	}
	forEachItem = func(items []interface{}, fn func(interface{}) interface{}) []interface{} { return items }
	repeatValue = func(value interface{}, count int) []interface{} {
		result := make([]interface{}, count)
		for i := range result {
			result[i] = value
		}
		return result
	}
	readFileContent      = func(filename string) string { return "" }
	requireValue         = func(value interface{}) interface{} { return value }
	oneOfValues          = func(value interface{}, options []interface{}) bool { return true }
	betweenValues        = func(value, min, max interface{}) bool { return true }
	base64Decode         = func(s string) string { return s }
	urlEncode            = func(s string) string { return s }
	urlDecode            = func(s string) string { return s }
	htmlEscape           = func(s string) string { return s }
	htmlUnescape         = func(s string) string { return s }
	assignDeviceGroup    = func(device map[string]interface{}) string { return "default" }
	assignDevicePriority = func(device map[string]interface{}) int { return 1 }
)

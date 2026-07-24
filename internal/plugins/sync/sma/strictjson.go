package sma

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxSafeInteger = int64(9007199254740991)

var (
	lowerUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	timestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$`)
	checksumPattern  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type strictParser struct {
	data     []byte
	position int
	maxDepth int
}

func parseStrictJSON(data []byte, maxDepth int) (interface{}, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid UTF-8")
	}
	parser := strictParser{data: data, maxDepth: maxDepth}
	parser.skipSpace()
	value, err := parser.parseValue(0)
	if err != nil {
		return nil, err
	}
	parser.skipSpace()
	if parser.position != len(parser.data) {
		return nil, fmt.Errorf("trailing JSON value at byte %d", parser.position)
	}
	return value, nil
}

func (p *strictParser) parseValue(depth int) (interface{}, error) {
	if p.position >= len(p.data) {
		return nil, fmt.Errorf("unexpected end of JSON")
	}
	switch p.data[p.position] {
	case '{':
		return p.parseObject(depth + 1)
	case '[':
		return p.parseArray(depth + 1)
	case '"':
		return p.parseString()
	case 't':
		return p.parseLiteral("true", true)
	case 'f':
		return p.parseLiteral("false", false)
	case 'n':
		return p.parseLiteral("null", nil)
	default:
		if p.data[p.position] == '-' || isDigit(p.data[p.position]) {
			return p.parseNumber()
		}
		return nil, fmt.Errorf("unexpected byte %q at %d", p.data[p.position], p.position)
	}
}

func (p *strictParser) parseObject(depth int) (interface{}, error) {
	if depth > p.maxDepth {
		return nil, fmt.Errorf("maximum JSON depth %d exceeded", p.maxDepth)
	}
	p.position++
	p.skipSpace()
	result := map[string]interface{}{}
	if p.consume('}') {
		return result, nil
	}
	for {
		if p.position >= len(p.data) || p.data[p.position] != '"' {
			return nil, fmt.Errorf("object name must be a string at byte %d", p.position)
		}
		nameValue, err := p.parseString()
		if err != nil {
			return nil, err
		}
		name := nameValue.(string)
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("duplicate object name %q", name)
		}
		p.skipSpace()
		if !p.consume(':') {
			return nil, fmt.Errorf("missing colon after object name %q", name)
		}
		p.skipSpace()
		value, err := p.parseValue(depth)
		if err != nil {
			return nil, err
		}
		result[name] = value
		p.skipSpace()
		if p.consume('}') {
			return result, nil
		}
		if !p.consume(',') {
			return nil, fmt.Errorf("missing comma in object at byte %d", p.position)
		}
		p.skipSpace()
	}
}

func (p *strictParser) parseArray(depth int) (interface{}, error) {
	if depth > p.maxDepth {
		return nil, fmt.Errorf("maximum JSON depth %d exceeded", p.maxDepth)
	}
	p.position++
	p.skipSpace()
	result := []interface{}{}
	if p.consume(']') {
		return result, nil
	}
	for {
		value, err := p.parseValue(depth)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
		p.skipSpace()
		if p.consume(']') {
			return result, nil
		}
		if !p.consume(',') {
			return nil, fmt.Errorf("missing comma in array at byte %d", p.position)
		}
		p.skipSpace()
	}
}

func (p *strictParser) parseString() (interface{}, error) {
	start := p.position
	p.position++
	for p.position < len(p.data) {
		current := p.data[p.position]
		if current == '"' {
			p.position++
			var decoded string
			if err := json.Unmarshal(p.data[start:p.position], &decoded); err != nil {
				return nil, fmt.Errorf("invalid JSON string: %w", err)
			}
			return decoded, nil
		}
		if current < 0x20 {
			return nil, fmt.Errorf("unescaped control character in string")
		}
		if current != '\\' {
			p.position++
			continue
		}
		p.position++
		if p.position >= len(p.data) {
			return nil, fmt.Errorf("unterminated string escape")
		}
		escape := p.data[p.position]
		if strings.ContainsRune(`"\/bfnrt`, rune(escape)) {
			p.position++
			continue
		}
		if escape != 'u' {
			return nil, fmt.Errorf("invalid string escape")
		}
		code, err := p.readUnicodeEscape()
		if err != nil {
			return nil, err
		}
		if code >= 0xD800 && code <= 0xDBFF {
			if p.position+1 >= len(p.data) || p.data[p.position] != '\\' || p.data[p.position+1] != 'u' {
				return nil, fmt.Errorf("lone high surrogate")
			}
			p.position++
			low, err := p.readUnicodeEscape()
			if err != nil {
				return nil, err
			}
			if low < 0xDC00 || low > 0xDFFF {
				return nil, fmt.Errorf("high surrogate is not followed by a low surrogate")
			}
		} else if code >= 0xDC00 && code <= 0xDFFF {
			return nil, fmt.Errorf("lone low surrogate")
		}
	}
	return nil, fmt.Errorf("unterminated string")
}

func (p *strictParser) readUnicodeEscape() (uint16, error) {
	// Position points at the u.
	if p.position+5 > len(p.data) {
		return 0, fmt.Errorf("short Unicode escape")
	}
	value, err := strconv.ParseUint(string(p.data[p.position+1:p.position+5]), 16, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid Unicode escape")
	}
	p.position += 5
	return uint16(value), nil
}

func (p *strictParser) parseNumber() (interface{}, error) {
	start := p.position
	if p.consume('-') && p.position >= len(p.data) {
		return nil, fmt.Errorf("incomplete number")
	}
	if p.consume('0') {
		if p.position < len(p.data) && isDigit(p.data[p.position]) {
			return nil, fmt.Errorf("leading zero in number")
		}
	} else {
		if p.position >= len(p.data) || p.data[p.position] < '1' || p.data[p.position] > '9' {
			return nil, fmt.Errorf("invalid number")
		}
		for p.position < len(p.data) && isDigit(p.data[p.position]) {
			p.position++
		}
	}
	if p.consume('.') {
		if p.position >= len(p.data) || !isDigit(p.data[p.position]) {
			return nil, fmt.Errorf("fraction requires digits")
		}
		for p.position < len(p.data) && isDigit(p.data[p.position]) {
			p.position++
		}
	}
	if p.position < len(p.data) && (p.data[p.position] == 'e' || p.data[p.position] == 'E') {
		p.position++
		if p.position < len(p.data) && (p.data[p.position] == '+' || p.data[p.position] == '-') {
			p.position++
		}
		if p.position >= len(p.data) || !isDigit(p.data[p.position]) {
			return nil, fmt.Errorf("exponent requires digits")
		}
		for p.position < len(p.data) && isDigit(p.data[p.position]) {
			p.position++
		}
	}
	return json.Number(string(p.data[start:p.position])), nil
}

func (p *strictParser) parseLiteral(text string, value interface{}) (interface{}, error) {
	if p.position+len(text) > len(p.data) || string(p.data[p.position:p.position+len(text)]) != text {
		return nil, fmt.Errorf("invalid literal at byte %d", p.position)
	}
	p.position += len(text)
	return value, nil
}

func (p *strictParser) skipSpace() {
	for p.position < len(p.data) {
		switch p.data[p.position] {
		case ' ', '\n', '\r', '\t':
			p.position++
		default:
			return
		}
	}
}

func (p *strictParser) consume(expected byte) bool {
	if p.position < len(p.data) && p.data[p.position] == expected {
		p.position++
		return true
	}
	return false
}

func isDigit(value byte) bool { return value >= '0' && value <= '9' }

func validateSafeNumbers(value interface{}) error {
	switch current := value.(type) {
	case json.Number:
		number, err := strconv.ParseFloat(current.String(), 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return fmt.Errorf("number %q is outside binary64", current)
		}
		if number == 0 && !numericTextIsZero(current.String()) {
			return fmt.Errorf("number %q underflows to zero", current)
		}
		if math.Trunc(number) == number && math.Abs(number) > float64(maxSafeInteger) {
			return fmt.Errorf("integer-valued number %q is outside the safe range", current)
		}
	case float64:
		if math.IsInf(current, 0) || math.IsNaN(current) {
			return fmt.Errorf("number must be finite")
		}
		if math.Trunc(current) == current && math.Abs(current) > float64(maxSafeInteger) {
			return fmt.Errorf("integer-valued number is outside the safe range")
		}
	case map[string]interface{}:
		for name, child := range current {
			if err := validateSafeNumbers(child); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	case []interface{}:
		for index, child := range current {
			if err := validateSafeNumbers(child); err != nil {
				return fmt.Errorf("[%d]: %w", index, err)
			}
		}
	}
	return nil
}

func validateGeneratedTree(value interface{}, maxDepth int) error {
	var walk func(interface{}, int) error
	walk = func(current interface{}, depth int) error {
		switch typed := current.(type) {
		case string:
			if !utf8.ValidString(typed) {
				return fmt.Errorf("invalid UTF-8 string")
			}
		case map[string]interface{}:
			if depth > maxDepth {
				return fmt.Errorf("maximum JSON depth %d exceeded", maxDepth)
			}
			for name, child := range typed {
				if !utf8.ValidString(name) {
					return fmt.Errorf("invalid UTF-8 object name")
				}
				if err := walk(child, depth+1); err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
			}
		case []interface{}:
			if depth > maxDepth {
				return fmt.Errorf("maximum JSON depth %d exceeded", maxDepth)
			}
			for index, child := range typed {
				if err := walk(child, depth+1); err != nil {
					return fmt.Errorf("[%d]: %w", index, err)
				}
			}
		}
		return nil
	}
	if err := walk(value, 1); err != nil {
		return err
	}
	return validateSafeNumbers(value)
}

func numericTextIsZero(value string) bool {
	value = strings.TrimPrefix(value, "-")
	mantissa := value
	if index := strings.IndexAny(mantissa, "eE"); index >= 0 {
		mantissa = mantissa[:index]
	}
	mantissa = strings.ReplaceAll(mantissa, ".", "")
	mantissa = strings.TrimLeft(mantissa, "0")
	return mantissa == ""
}

func validateArchiveTree(value interface{}) error {
	root, rootErr := requireObject(value, "root")
	if rootErr != nil {
		return rootErr
	}
	if err := exactKeys(root, []string{
		"format_version", "metadata", "devices", "templates", "discovered_devices",
		"network_settings", "plugin_configurations", "system_settings",
	}, nil); err != nil {
		return err
	}
	if version, err := requireString(root["format_version"], "format_version", false); err != nil {
		return err
	} else if version != FormatVersion {
		return fmt.Errorf("unsupported format_version %q", version)
	}
	if err := validateSafeNumbers(root); err != nil {
		return err
	}
	metadata, metadataErr := requireObject(root["metadata"], "metadata")
	if metadataErr != nil {
		return metadataErr
	}
	if err := validateMetadata(metadata); err != nil {
		return err
	}
	devices, devicesErr := requireArray(root["devices"], "devices")
	if devicesErr != nil {
		return devicesErr
	}
	for i, item := range devices {
		object, objectErr := requireObject(item, fmt.Sprintf("devices[%d]", i))
		if objectErr != nil {
			return objectErr
		}
		if err := validateDevice(object); err != nil {
			return fmt.Errorf("devices[%d]: %w", i, err)
		}
	}
	templates, templatesErr := requireArray(root["templates"], "templates")
	if templatesErr != nil {
		return templatesErr
	}
	for i, item := range templates {
		object, objectErr := requireObject(item, fmt.Sprintf("templates[%d]", i))
		if objectErr != nil {
			return objectErr
		}
		if err := validateTemplate(object); err != nil {
			return fmt.Errorf("templates[%d]: %w", i, err)
		}
	}
	discovered, discoveredErr := requireArray(root["discovered_devices"], "discovered_devices")
	if discoveredErr != nil {
		return discoveredErr
	}
	for i, item := range discovered {
		object, objectErr := requireObject(item, fmt.Sprintf("discovered_devices[%d]", i))
		if objectErr != nil {
			return objectErr
		}
		if err := validateDiscovered(object); err != nil {
			return fmt.Errorf("discovered_devices[%d]: %w", i, err)
		}
	}
	if len(devices)+len(templates)+len(discovered) == 0 {
		return fmt.Errorf("archive must contain a device, template, or discovered device")
	}
	if err := validateNetwork(root["network_settings"]); err != nil {
		return err
	}
	plugins, pluginsErr := requireArray(root["plugin_configurations"], "plugin_configurations")
	if pluginsErr != nil {
		return pluginsErr
	}
	for i, item := range plugins {
		object, objectErr := requireObject(item, fmt.Sprintf("plugin_configurations[%d]", i))
		if objectErr != nil {
			return objectErr
		}
		if err := exactKeys(object, []string{"plugin_name", "version", "config", "enabled"}, nil); err != nil {
			return err
		}
		if _, err := requireString(object["plugin_name"], "plugin_name", true); err != nil {
			return err
		}
		if _, err := requireString(object["version"], "version", true); err != nil {
			return err
		}
		if _, err := requireObject(object["config"], "config"); err != nil {
			return err
		}
		if _, ok := object["enabled"].(bool); !ok {
			return fmt.Errorf("enabled must be a boolean")
		}
	}
	return validateSystemSettings(root["system_settings"])
}

func validateMetadata(object map[string]interface{}) error {
	if err := exactKeys(object, []string{
		"export_id", "created_at", "created_by", "export_type", "system_info", "integrity",
	}, nil); err != nil {
		return err
	}
	exportID, exportIDErr := requireString(object["export_id"], "export_id", false)
	if exportIDErr != nil {
		return exportIDErr
	}
	if !isLowerUUID(exportID) {
		return fmt.Errorf("export_id must be a lowercase UUID")
	}
	if err := requireTimestamp(object["created_at"], "created_at"); err != nil {
		return err
	}
	if _, err := requireString(object["created_by"], "created_by", false); err != nil {
		return err
	}
	exportType, exportTypeErr := requireString(object["export_type"], "export_type", false)
	if exportTypeErr != nil || (exportType != "manual" && exportType != "api") {
		return fmt.Errorf("export_type must be manual or api")
	}
	system, systemErr := requireObject(object["system_info"], "system_info")
	if systemErr != nil {
		return systemErr
	}
	if err := exactKeys(system, []string{
		"version", "database_type", "hostname", "total_size_bytes", "compression_ratio",
	}, nil); err != nil {
		return err
	}
	if _, err := requireString(system["version"], "version", false); err != nil {
		return err
	}
	databaseType, databaseTypeErr := requireString(system["database_type"], "database_type", false)
	if databaseTypeErr != nil || (databaseType != "sqlite" && databaseType != "postgresql" && databaseType != "mysql") {
		return fmt.Errorf("database_type must be sqlite, postgresql, or mysql")
	}
	if _, err := requireString(system["hostname"], "hostname", false); err != nil {
		return err
	}
	if !numberIsExactly(system["total_size_bytes"], 0) || !numberIsExactly(system["compression_ratio"], 0) {
		return fmt.Errorf("system size and compression ratio must be zero")
	}
	integrity, integrityErr := requireObject(object["integrity"], "integrity")
	if integrityErr != nil {
		return integrityErr
	}
	if err := exactKeys(integrity, []string{"checksum", "record_count", "file_count"}, nil); err != nil {
		return err
	}
	checksum, checksumErr := requireString(integrity["checksum"], "checksum", true)
	if checksumErr != nil {
		return checksumErr
	}
	if checksum != "" && !checksumPattern.MatchString(checksum) {
		return fmt.Errorf("invalid checksum")
	}
	if _, err := requireSafeInteger(integrity["record_count"], "record_count", false); err != nil {
		return err
	}
	fileCount, fileCountErr := requireSafeInteger(integrity["file_count"], "file_count", false)
	if fileCountErr != nil || fileCount != 1 {
		return fmt.Errorf("file_count must be 1")
	}
	return nil
}

func validateDevice(object map[string]interface{}) error {
	required := []string{
		"id", "mac", "ip", "type", "name", "model", "firmware", "status",
		"last_seen", "settings", "created_at", "updated_at",
	}
	if err := exactKeys(object, required, []string{"configuration"}); err != nil {
		return err
	}
	if _, err := requireSafeInteger(object["id"], "id", false); err != nil {
		return err
	}
	for _, name := range []string{"mac", "ip", "type", "name", "model", "firmware", "status"} {
		if _, err := requireString(object[name], name, true); err != nil {
			return err
		}
	}
	for _, name := range []string{"last_seen", "created_at", "updated_at"} {
		if err := requireTimestamp(object[name], name); err != nil {
			return err
		}
	}
	if _, err := requireObject(object["settings"], "settings"); err != nil {
		return err
	}
	if configuration, present := object["configuration"]; present {
		return validateConfiguration(configuration)
	}
	return nil
}

func validateConfiguration(value interface{}) error {
	object, err := requireObject(value, "configuration")
	if err != nil {
		return err
	}
	if err := exactKeys(object,
		[]string{"device_id", "config", "sync_status", "updated_at"},
		[]string{"template_id", "last_synced"}); err != nil {
		return err
	}
	if _, err := requireSafeInteger(object["device_id"], "device_id", false); err != nil {
		return err
	}
	if value, ok := object["template_id"]; ok {
		if _, err := requireSafeInteger(value, "template_id", false); err != nil {
			return err
		}
	}
	if _, err := requireObject(object["config"], "config"); err != nil {
		return err
	}
	if _, err := requireString(object["sync_status"], "sync_status", true); err != nil {
		return err
	}
	if value, ok := object["last_synced"]; ok {
		if err := requireTimestamp(value, "last_synced"); err != nil {
			return err
		}
	}
	return requireTimestamp(object["updated_at"], "updated_at")
}

func validateTemplate(object map[string]interface{}) error {
	required := []string{
		"id", "generation", "name", "description", "device_type", "config", "variables",
		"is_default", "created_at", "updated_at",
	}
	if err := exactKeys(object, required, nil); err != nil {
		return err
	}
	for _, name := range []string{"id", "generation"} {
		if _, err := requireSafeInteger(object[name], name, false); err != nil {
			return err
		}
	}
	for _, name := range []string{"name", "description", "device_type"} {
		if _, err := requireString(object[name], name, true); err != nil {
			return err
		}
	}
	for _, name := range []string{"config", "variables"} {
		if _, err := requireObject(object[name], name); err != nil {
			return err
		}
	}
	if _, ok := object["is_default"].(bool); !ok {
		return fmt.Errorf("is_default must be a boolean")
	}
	if err := requireTimestamp(object["created_at"], "created_at"); err != nil {
		return err
	}
	return requireTimestamp(object["updated_at"], "updated_at")
}

func validateDiscovered(object map[string]interface{}) error {
	if err := exactKeys(object,
		[]string{"mac", "ssid", "model", "ip", "agent_id", "generation", "signal", "discovered"},
		nil); err != nil {
		return err
	}
	for _, name := range []string{"mac", "ssid", "model", "ip", "agent_id"} {
		if _, err := requireString(object[name], name, true); err != nil {
			return err
		}
	}
	if _, err := requireSafeInteger(object["generation"], "generation", false); err != nil {
		return err
	}
	if _, err := requireSafeInteger(object["signal"], "signal", true); err != nil {
		return err
	}
	return requireTimestamp(object["discovered"], "discovered")
}

func validateNetwork(value interface{}) error {
	object, objectErr := requireObject(value, "network_settings")
	if objectErr != nil {
		return objectErr
	}
	if err := exactKeys(object, []string{"wifi_networks", "ntp_servers"}, []string{"mqtt_config"}); err != nil {
		return err
	}
	wifi, wifiErr := requireArray(object["wifi_networks"], "wifi_networks")
	if wifiErr != nil {
		return wifiErr
	}
	for i, value := range wifi {
		entry, entryErr := requireObject(value, fmt.Sprintf("wifi_networks[%d]", i))
		if entryErr != nil {
			return entryErr
		}
		if err := exactKeys(entry, []string{"ssid", "security", "priority"}, nil); err != nil {
			return err
		}
		if _, err := requireString(entry["ssid"], "ssid", true); err != nil {
			return err
		}
		if _, err := requireString(entry["security"], "security", true); err != nil {
			return err
		}
		if _, err := requireSafeInteger(entry["priority"], "priority", false); err != nil {
			return err
		}
	}
	servers, serversErr := requireArray(object["ntp_servers"], "ntp_servers")
	if serversErr != nil {
		return serversErr
	}
	for _, server := range servers {
		if _, err := requireString(server, "ntp server", true); err != nil {
			return err
		}
	}
	if mqttValue, present := object["mqtt_config"]; present {
		mqtt, mqttErr := requireObject(mqttValue, "mqtt_config")
		if mqttErr != nil {
			return mqttErr
		}
		if err := exactKeys(mqtt, []string{"server", "username", "port", "retain", "qos"}, nil); err != nil {
			return err
		}
		if _, err := requireString(mqtt["server"], "server", true); err != nil {
			return err
		}
		if _, err := requireString(mqtt["username"], "username", true); err != nil {
			return err
		}
		port, portErr := requireSafeInteger(mqtt["port"], "port", false)
		if portErr != nil || port < 1 || port > 65535 {
			return fmt.Errorf("port must be in 1..65535")
		}
		qos, qosErr := requireSafeInteger(mqtt["qos"], "qos", false)
		if qosErr != nil || qos < 0 || qos > 2 {
			return fmt.Errorf("qos must be in 0..2")
		}
		if _, ok := mqtt["retain"].(bool); !ok {
			return fmt.Errorf("retain must be a boolean")
		}
	}
	return nil
}

func validateSystemSettings(value interface{}) error {
	object, objectErr := requireObject(value, "system_settings")
	if objectErr != nil {
		return objectErr
	}
	if err := exactKeys(object, []string{"log_level", "api_settings", "database_settings"}, nil); err != nil {
		return err
	}
	if _, err := requireString(object["log_level"], "log_level", false); err != nil {
		return err
	}
	if _, err := requireObject(object["api_settings"], "api_settings"); err != nil {
		return err
	}
	_, databaseSettingsErr := requireObject(object["database_settings"], "database_settings")
	return databaseSettingsErr
}

func exactKeys(object map[string]interface{}, required, optional []string) error {
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, name := range required {
		allowed[name] = true
		if _, present := object[name]; !present {
			return fmt.Errorf("missing required field %s", name)
		}
	}
	for _, name := range optional {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unknown field %s", name)
		}
	}
	return nil
}

func requireObject(value interface{}, name string) (map[string]interface{}, error) {
	result, ok := value.(map[string]interface{})
	if !ok || result == nil {
		return nil, fmt.Errorf("%s must be a non-null object", name)
	}
	return result, nil
}

func requireArray(value interface{}, name string) ([]interface{}, error) {
	result, ok := value.([]interface{})
	if !ok || result == nil {
		return nil, fmt.Errorf("%s must be a non-null array", name)
	}
	return result, nil
}

func requireString(value interface{}, name string, allowEmpty bool) (string, error) {
	result, ok := value.(string)
	if !ok || (!allowEmpty && result == "") {
		return "", fmt.Errorf("%s must be %sa string", name, map[bool]string{true: "", false: "a non-empty "}[allowEmpty])
	}
	return result, nil
}

func requireTimestamp(value interface{}, name string) error {
	text, err := requireString(value, name, false)
	if err != nil {
		return err
	}
	if !timestampPattern.MatchString(text) {
		return fmt.Errorf("%s must be a canonical UTC timestamp", name)
	}
	if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
		return fmt.Errorf("%s must be a real calendar instant", name)
	}
	return nil
}

func requireSafeInteger(value interface{}, name string, signed bool) (int64, error) {
	var result int64
	switch number := value.(type) {
	case json.Number:
		if strings.ContainsAny(number.String(), ".eE") {
			float, err := strconv.ParseFloat(number.String(), 64)
			if err != nil || math.Trunc(float) != float || math.Abs(float) > float64(maxSafeInteger) {
				return 0, fmt.Errorf("%s must be a safe integer", name)
			}
			result = int64(float)
		} else {
			parsed, err := strconv.ParseInt(number.String(), 10, 64)
			if err != nil || parsed < -maxSafeInteger || parsed > maxSafeInteger {
				return 0, fmt.Errorf("%s must be a safe integer", name)
			}
			result = parsed
		}
	case int:
		result = int64(number)
	case int64:
		result = number
	case uint:
		if uint64(number) > uint64(maxSafeInteger) {
			return 0, fmt.Errorf("%s must be a safe integer", name)
		}
		result = int64(number)
	case float64:
		if math.Trunc(number) != number || math.Abs(number) > float64(maxSafeInteger) {
			return 0, fmt.Errorf("%s must be a safe integer", name)
		}
		result = int64(number)
	default:
		return 0, fmt.Errorf("%s must be a safe integer", name)
	}
	if !signed && result < 0 {
		return 0, fmt.Errorf("%s must be non-negative", name)
	}
	return result, nil
}

func numberIsExactly(value interface{}, expected float64) bool {
	switch number := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseFloat(number.String(), 64)
		return err == nil && parsed == expected
	case int:
		return float64(number) == expected
	case int64:
		return float64(number) == expected
	case uint:
		return float64(number) == expected
	case float64:
		return number == expected
	default:
		return false
	}
}

func isLowerUUID(value string) bool {
	if !lowerUUIDPattern.MatchString(value) {
		return false
	}
	_, err := uuid.Parse(value)
	return err == nil
}

func checksumBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return fmt.Sprintf("sha256:%x", digest)
}

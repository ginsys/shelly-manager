package configuration

import (
	"fmt"
	"math"
	"net/netip"
	"regexp"
	"sort"
	"strings"
)

// FieldError is one field of a configuration body that does not match its schema. Path is the
// dotted path of the field, as in the desired-configuration sources map (for example
// "relay.relays.0.auto_off").
type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidateConfigBody checks a stored configuration body (a template or a device override)
// against the device schema of deviceType (GetDeviceConfigurationSchema). An empty deviceType
// uses the schema of every section, as GetTemplateConfigurationSchema does for global and group
// templates.
//
// It checks the type, minimum, maximum, maxLength, pattern and enum of each value, as the UI form
// does (ui/src/utils/config-form.ts validateValue). A top-level section that the model does not
// have is an error. Other keys that the schema does not describe are not checked, because
// DeviceConfiguration stores some fields that the schema leaves out. The errors are sorted by
// path.
func ValidateConfigBody(deviceType string, body map[string]interface{}) []FieldError {
	schema := GetDeviceConfigurationSchema(deviceType)
	sections, _ := schema["properties"].(map[string]interface{})
	every, _ := GetDeviceConfigurationSchema("")["properties"].(map[string]interface{})

	var errs []FieldError
	for name, value := range body {
		section, ok := sections[name].(map[string]interface{})
		if !ok {
			if _, known := every[name]; known {
				errs = append(errs, FieldError{Path: name, Message: fmt.Sprintf("%s has no %s settings", deviceType, name)})
			}
			continue
		}
		errs = append(errs, validateNode(section, value, name)...)
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Path < errs[j].Path })
	return errs
}

func validateNode(schema map[string]interface{}, value interface{}, path string) []FieldError {
	if value == nil {
		return nil
	}
	fail := func(msg string) []FieldError { return []FieldError{{Path: path, Message: msg}} }

	switch schema["type"] {
	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fail("Must be an object")
		}
		props, _ := schema["properties"].(map[string]interface{})
		var errs []FieldError
		for key, child := range obj {
			if childSchema, ok := props[key].(map[string]interface{}); ok {
				errs = append(errs, validateNode(childSchema, child, path+"."+key)...)
			}
		}
		return errs
	case "array":
		list, ok := value.([]interface{})
		if !ok {
			return fail("Must be a JSON list")
		}
		items, ok := schema["items"].(map[string]interface{})
		if !ok {
			return nil
		}
		var errs []FieldError
		for i, item := range list {
			errs = append(errs, validateNode(items, item, fmt.Sprintf("%s.%d", path, i))...)
		}
		return errs
	case "integer":
		n, ok := value.(float64)
		if !ok || n != math.Trunc(n) {
			return fail("Must be a whole number")
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fail("Must be a number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fail("Must be true or false")
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fail("Must be text")
		}
	}

	if n, ok := value.(float64); ok {
		if min, ok := schemaNumber(schema["minimum"]); ok && n < min {
			return fail(fmt.Sprintf("Must be at least %v", schema["minimum"]))
		}
		if max, ok := schemaNumber(schema["maximum"]); ok && n > max {
			return fail(fmt.Sprintf("Must be at most %v", schema["maximum"]))
		}
	}
	if s, ok := value.(string); ok {
		if maxLen, ok := schemaNumber(schema["maxLength"]); ok && float64(len([]rune(s))) > maxLen {
			return fail(fmt.Sprintf("Must be at most %v characters", schema["maxLength"]))
		}
		if pattern, ok := schema["pattern"].(string); ok && pattern != "" {
			re, err := regexp.Compile("^(?:" + pattern + ")$")
			if err == nil && !re.MatchString(s) {
				return fail("Does not match the required format")
			}
		}
		// An empty string reads as "not set": apply, not the schema, refuses a static address mode
		// that has no address.
		if schema["format"] == "ipv4" && s != "" && !isIPv4(s) {
			return fail("Must be an IPv4 address")
		}
	}
	if options := schemaEnum(schema["enum"]); options != nil {
		for _, option := range options {
			if option == value {
				return nil
			}
		}
		parts := make([]string, len(options))
		for i, option := range options {
			parts[i] = fmt.Sprint(option)
		}
		return fail("Must be one of: " + strings.Join(parts, ", "))
	}
	return nil
}

// isIPv4 reports whether s is a dotted IPv4 address: four decimal parts, no sign, no leading
// zero, no space, no IPv6 form. The form's check (ui/src/utils/config-form.ts) holds the same rule.
func isIPv4(s string) bool {
	addr, err := netip.ParseAddr(s)
	return err == nil && addr.Is4()
}

// schemaNumber reads a schema bound, which the schema builders write as an int or a float64.
func schemaNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// schemaEnum reads an enum list as JSON values, so a decoded body value compares equal to it.
func schemaEnum(v interface{}) []interface{} {
	switch list := v.(type) {
	case []interface{}:
		return list
	case []string:
		out := make([]interface{}, len(list))
		for i, s := range list {
			out[i] = s
		}
		return out
	default:
		return nil
	}
}

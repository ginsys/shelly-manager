package configuration

import (
	"fmt"
	"math"
	"reflect"
	"strings"
)

// CompareResult holds the result of comparing two configurations
type CompareResult struct {
	Match       bool               `json:"match"`
	Differences []ConfigDifference `json:"differences"`
}

// FieldCompareRule defines how a specific field should be compared
type FieldCompareRule struct {
	Path        string                        // Field path pattern (supports wildcards)
	SkipCompare bool                          // Don't compare this field
	Tolerance   float64                       // For numeric fields
	Normalize   func(interface{}) interface{} // Normalize before compare
	Severity    string                        // "critical", "warning", "info" (default: critical)
	Category    string                        // "security", "network", "device", "system", "metadata"
}

// defaultCompareRules defines the default comparison rules
var defaultCompareRules = []FieldCompareRule{
	// Skip read-only fields
	{Path: "system.mac", SkipCompare: true},
	{Path: "system.firmware", SkipCompare: true},
	{Path: "system.fw_id", SkipCompare: true},

	// Tolerance for coordinates (about 10 meters)
	{Path: "location.latitude", Tolerance: 0.0001, Category: "system"},
	{Path: "location.longitude", Tolerance: 0.0001, Category: "system"},
	{Path: "location.lat", Tolerance: 0.0001, Category: "system"},
	{Path: "location.lng", Tolerance: 0.0001, Category: "system"},

	// Warnings for non-critical fields
	{Path: "system.device.name", Severity: "warning", Category: "metadata"},
	{Path: "led.*", Severity: "warning", Category: "device"},
}

// ConfigComparator compares device configurations
type ConfigComparator struct {
	rules []FieldCompareRule
}

// NewConfigComparator creates a new comparator with default rules
func NewConfigComparator() *ConfigComparator {
	return &ConfigComparator{
		rules: defaultCompareRules,
	}
}

// NewConfigComparatorWithRules creates a comparator with custom rules
func NewConfigComparatorWithRules(rules []FieldCompareRule) *ConfigComparator {
	return &ConfigComparator{
		rules: rules,
	}
}

// Compare compares expected (desired) with actual (from device) configurations
func (c *ConfigComparator) Compare(expected, actual *DeviceConfiguration) *CompareResult {
	result := &CompareResult{
		Match:       true,
		Differences: []ConfigDifference{},
	}

	if expected == nil && actual == nil {
		return result
	}

	if expected == nil || actual == nil {
		result.Match = false
		result.Differences = append(result.Differences, ConfigDifference{
			Path:     "",
			Expected: expected,
			Actual:   actual,
			Type:     "modified",
			Severity: "critical",
			Category: "system",
		})
		return result
	}

	// Compare using reflection
	c.compareStruct(reflect.ValueOf(expected).Elem(), reflect.ValueOf(actual).Elem(), "", result)

	return result
}

func (c *ConfigComparator) compareStruct(expected, actual reflect.Value, prefix string, result *CompareResult) {
	if !expected.IsValid() || !actual.IsValid() {
		return
	}

	expectedType := expected.Type()

	for i := 0; i < expected.NumField(); i++ {
		field := expectedType.Field(i)
		if !field.IsExported() {
			continue
		}

		// Get JSON tag for field name
		fieldName := c.getFieldName(field)
		path := fieldName
		if prefix != "" {
			path = prefix + "." + fieldName
		}

		// Check if this field should be skipped
		rule := c.findRule(path)
		if rule != nil && rule.SkipCompare {
			continue
		}

		expectedField := expected.Field(i)
		actualField := actual.Field(i)

		c.compareFields(expectedField, actualField, path, rule, result)
	}
}

func (c *ConfigComparator) compareFields(expected, actual reflect.Value, path string, rule *FieldCompareRule, result *CompareResult) {
	// Handle nil pointers
	if expected.Kind() == reflect.Ptr {
		if expected.IsNil() {
			// Expected is nil (inherit), don't compare
			return
		}
		if actual.IsNil() {
			// Expected has value but actual is nil
			c.addDifference(result, path, expected.Elem().Interface(), nil, "removed", rule)
			return
		}
		// Both non-nil, compare pointed values
		c.compareFields(expected.Elem(), actual.Elem(), path, rule, result)
		return
	}

	// Handle struct fields
	if expected.Kind() == reflect.Struct {
		c.compareStruct(expected, actual, path, result)
		return
	}

	// Handle slices
	if expected.Kind() == reflect.Slice {
		c.compareSlices(expected, actual, path, rule, result)
		return
	}

	// Compare primitive values
	if !c.valuesEqual(expected.Interface(), actual.Interface(), rule) {
		c.addDifference(result, path, expected.Interface(), actual.Interface(), "modified", rule)
	}
}

func (c *ConfigComparator) compareSlices(expected, actual reflect.Value, path string, rule *FieldCompareRule, result *CompareResult) {
	expectedLen := expected.Len()
	actualLen := actual.Len()

	// Compare elements up to the shorter length
	maxLen := expectedLen
	if actualLen < maxLen {
		maxLen = actualLen
	}

	for i := 0; i < maxLen; i++ {
		elemPath := fmt.Sprintf("%s.%d", path, i)
		elemRule := c.findRule(elemPath)
		if elemRule == nil {
			elemRule = rule
		}
		c.compareFields(expected.Index(i), actual.Index(i), elemPath, elemRule, result)
	}

	// Report extra elements in expected (missing from actual)
	for i := actualLen; i < expectedLen; i++ {
		elemPath := fmt.Sprintf("%s.%d", path, i)
		c.addDifference(result, elemPath, expected.Index(i).Interface(), nil, "removed", rule)
	}
}

func (c *ConfigComparator) valuesEqual(expected, actual interface{}, rule *FieldCompareRule) bool {
	// Apply normalization if specified
	if rule != nil && rule.Normalize != nil {
		expected = rule.Normalize(expected)
		actual = rule.Normalize(actual)
	}

	// Handle numeric tolerance
	if rule != nil && rule.Tolerance > 0 {
		expectedFloat, ok1 := toFloat64(expected)
		actualFloat, ok2 := toFloat64(actual)
		if ok1 && ok2 {
			return math.Abs(expectedFloat-actualFloat) <= rule.Tolerance
		}
	}

	// String comparison
	if expectedStr, ok := expected.(string); ok {
		actualStr, ok := actual.(string)
		if !ok {
			return false
		}
		return expectedStr == actualStr
	}

	// Default deep equality
	return reflect.DeepEqual(expected, actual)
}

func (c *ConfigComparator) addDifference(result *CompareResult, path string, expected, actual interface{}, diffType string, rule *FieldCompareRule) {
	severity := "critical"
	category := "device"

	if rule != nil {
		if rule.Severity != "" {
			severity = rule.Severity
		}
		if rule.Category != "" {
			category = rule.Category
		}
	}

	// Infer category from path if not specified
	if category == "device" {
		category = c.inferCategory(path)
	}

	result.Match = false
	result.Differences = append(result.Differences, ConfigDifference{
		Path:        path,
		Expected:    expected,
		Actual:      actual,
		Type:        diffType,
		Severity:    severity,
		Category:    category,
		Description: fmt.Sprintf("Value mismatch at %s", path),
	})
}

func (c *ConfigComparator) inferCategory(path string) string {
	lowerPath := strings.ToLower(path)

	switch {
	case strings.HasPrefix(lowerPath, "wifi"):
		return "network"
	case strings.HasPrefix(lowerPath, "mqtt"):
		return "network"
	case strings.HasPrefix(lowerPath, "auth"):
		return "security"
	case strings.HasPrefix(lowerPath, "cloud"):
		return "network"
	case strings.HasPrefix(lowerPath, "coiot"):
		return "network"
	case strings.HasPrefix(lowerPath, "system"):
		return "system"
	case strings.HasPrefix(lowerPath, "location"):
		return "system"
	case strings.HasPrefix(lowerPath, "relay"):
		return "device"
	case strings.HasPrefix(lowerPath, "input"):
		return "device"
	case strings.HasPrefix(lowerPath, "led"):
		return "device"
	case strings.HasPrefix(lowerPath, "power"):
		return "device"
	default:
		return "device"
	}
}

func (c *ConfigComparator) findRule(path string) *FieldCompareRule {
	for i := range c.rules {
		if c.pathMatches(path, c.rules[i].Path) {
			return &c.rules[i]
		}
	}
	return nil
}

func (c *ConfigComparator) pathMatches(path, pattern string) bool {
	// Exact match
	if path == pattern {
		return true
	}

	// Wildcard matching
	patternParts := strings.Split(pattern, ".")
	pathParts := strings.Split(path, ".")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i, pp := range patternParts {
		if pp == "*" {
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}

	return true
}

func (c *ConfigComparator) getFieldName(field reflect.StructField) string {
	// Try JSON tag first
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" && jsonTag != "-" {
		// Handle "fieldname,omitempty" format
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "" {
			return parts[0]
		}
	}

	// Fall back to field name in lowercase
	return strings.ToLower(field.Name)
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case *float64:
		if val != nil {
			return *val, true
		}
		return 0, false
	case *int:
		if val != nil {
			return float64(*val), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// HasErrors returns true if there are any critical-level differences
func (r *CompareResult) HasErrors() bool {
	for _, diff := range r.Differences {
		if diff.Severity == "critical" {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of critical-level differences
func (r *CompareResult) ErrorCount() int {
	count := 0
	for _, diff := range r.Differences {
		if diff.Severity == "critical" {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level differences
func (r *CompareResult) WarningCount() int {
	count := 0
	for _, diff := range r.Differences {
		if diff.Severity == "warning" {
			count++
		}
	}
	return count
}

package configuration

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// schemaLeaves returns every leaf field of a schema node by dotted path ("[]" for array items).
func schemaLeaves(path string, node map[string]interface{}, out map[string]map[string]interface{}) {
	if props, ok := node["properties"].(map[string]interface{}); ok {
		for name, child := range props {
			childPath := name
			if path != "" {
				childPath = path + "." + name
			}
			schemaLeaves(childPath, child.(map[string]interface{}), out)
		}
		return
	}
	if items, ok := node["items"].(map[string]interface{}); ok {
		if _, isObject := items["properties"]; isObject {
			schemaLeaves(path+"[]", items, out)
			return
		}
	}
	out[path] = node
}

// jsonFields maps each JSON field name of struct t to its Go type.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fields[name] = f.Type
	}
	return fields
}

// structPaths returns every leaf JSON path of a Go type, in the same notation.
func structPaths(path string, t reflect.Type, out map[string]bool) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		elem := t.Elem()
		for elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct {
			structPaths(path+"[]", elem, out)
			return
		}
		out[path] = true
		return
	}
	if t.Kind() != reflect.Struct || t.PkgPath() != reflect.TypeOf(DeviceConfiguration{}).PkgPath() {
		out[path] = true
		return
	}
	for name, ft := range jsonFields(t) {
		childPath := name
		if path != "" {
			childPath = path + "." + name
		}
		structPaths(childPath, ft, out)
	}
}

func sectionNames(schema map[string]interface{}) []string {
	var names []string
	for name := range schema["properties"].(map[string]interface{}) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestDeviceSchemaHoldsOnlyStoredFields(t *testing.T) {
	stored := map[string]bool{}
	structPaths("", reflect.TypeOf(DeviceConfiguration{}), stored)

	types := append(NewGen1Converter(nil).SupportedDeviceTypes(), "SNSW-001X16EU", "")
	for _, deviceType := range types {
		leaves := map[string]map[string]interface{}{}
		schemaLeaves("", GetDeviceConfigurationSchema(deviceType), leaves)
		if len(leaves) == 0 {
			t.Fatalf("%q: schema has no fields", deviceType)
		}
		for path := range leaves {
			if !stored[path] {
				t.Errorf("%q: schema field %s is not stored by DeviceConfiguration", deviceType, path)
			}
		}
	}
}

func TestDeviceSchemaSectionsFollowTheModel(t *testing.T) {
	cases := map[string][]string{
		"SHPLG-S": {"auth", "cloud", "coiot", "led", "location", "mqtt", "network", "power_metering", "relay", "system", "wifi"},
		"SHSW-1":  {"auth", "cloud", "coiot", "input", "location", "mqtt", "network", "relay", "system", "wifi"},
		"SHSW-25": {"auth", "cloud", "coiot", "input", "location", "mqtt", "network", "power_metering", "relay", "roller", "system", "wifi"},
		"SHIX3-1": {"auth", "cloud", "coiot", "input", "location", "mqtt", "network", "system", "wifi"},
		"unknown": {"auth", "cloud", "coiot", "dimming", "input", "led", "location", "mqtt", "network", "power_metering", "relay", "roller", "system", "wifi"},
	}
	for deviceType, want := range cases {
		schema := GetDeviceConfigurationSchema(deviceType)
		if got := sectionNames(schema); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: sections %v, want %v", deviceType, got, want)
		}
		if schema[SchemaKeyDeviceType] != deviceType {
			t.Errorf("%s: %s = %v", deviceType, SchemaKeyDeviceType, schema[SchemaKeyDeviceType])
		}
	}
	if GetDeviceConfigurationSchema("SHPLG-S")[SchemaKeyApplySupported] != true {
		t.Error("SHPLG-S must be apply-supported")
	}
	if GetDeviceConfigurationSchema("unknown")[SchemaKeyApplySupported] != false {
		t.Error("an unknown model must not be apply-supported")
	}
}

func TestDeviceSchemaUnsupportedModelHasNoApplicableField(t *testing.T) {
	leaves := map[string]map[string]interface{}{}
	schemaLeaves("", GetDeviceConfigurationSchema("SNSW-001X16EU"), leaves)
	for path, leaf := range leaves {
		if leaf[SchemaKeyApplicable] != false {
			t.Errorf("%s: %s = %v, want false", path, SchemaKeyApplicable, leaf[SchemaKeyApplicable])
		}
	}
}

// sampleValue returns a value the schema accepts for a leaf.
func sampleValue(leaf map[string]interface{}) interface{} {
	if enum, ok := leaf["enum"].([]string); ok && len(enum) > 0 {
		return enum[0]
	}
	switch leaf["type"] {
	case "boolean":
		return true
	case "integer":
		if minimum, ok := leaf["minimum"].(int); ok && minimum > 1 {
			return minimum
		}
		return 1
	case "number":
		return 1.5
	case "array":
		return []interface{}{}
	default:
		if leaf["format"] == "ipv4" {
			return "10.0.0.1"
		}
		return "sample"
	}
}

// configWithLeaf builds a DeviceConfiguration with only path set.
func configWithLeaf(t *testing.T, path string, value interface{}) *DeviceConfiguration {
	t.Helper()
	root := map[string]interface{}{}
	node := root
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		if strings.HasSuffix(part, "[]") {
			item := map[string]interface{}{"id": 0}
			node[strings.TrimSuffix(part, "[]")] = []interface{}{item}
			node = item
			continue
		}
		child := map[string]interface{}{}
		node[part] = child
		node = child
	}
	node[parts[len(parts)-1]] = value
	if path == "mqtt.port" {
		node["server"] = "broker.local"
	}
	// A login is applied as a whole: the flag with a username and a password.
	if path == "auth.enable" || path == "auth.user" || path == "auth.pass" {
		for key, companion := range map[string]interface{}{"enable": true, "user": "admin", "pass": "secret"} {
			if _, set := node[key]; !set {
				node[key] = companion
			}
		}
	}

	raw, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("%s: marshal: %v", path, err)
	}
	var cfg DeviceConfiguration
	if err = json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("%s: unmarshal %s: %v", path, raw, err)
	}
	return &cfg
}

// TestDeviceSchemaApplicableMatchesApply sets each schema field alone and asks apply's own
// refusal check whether it would write it. A field is applicable in the schema exactly when
// apply accepts it.
func TestDeviceSchemaApplicableMatchesApply(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, deviceType := range converter.SupportedDeviceTypes() {
		leaves := map[string]map[string]interface{}{}
		schemaLeaves("", GetDeviceConfigurationSchema(deviceType), leaves)
		applicable := 0
		for path, leaf := range leaves {
			if strings.HasSuffix(path, "[].id") {
				continue // an array item's index, not a setting
			}
			cfg := configWithLeaf(t, path, sampleValue(leaf))
			unsupported, err := unsupportedApplySettings(converter, cfg, deviceType, true)
			if err != nil {
				t.Fatalf("%s %s: %v", deviceType, path, err)
			}
			accepted := len(unsupported) == 0
			marked := leaf[SchemaKeyApplicable] == true
			if accepted != marked {
				t.Errorf("%s %s: schema %s=%v, apply accepts=%v (unsupported %v)",
					deviceType, path, SchemaKeyApplicable, marked, accepted, unsupported)
			}
			if marked {
				applicable++
			}
		}
		if applicable == 0 {
			t.Errorf("%s: no applicable field, the comparison proves nothing", deviceType)
		}
	}
}

// writeOnlyApplicablePaths names the applicable fields that verify skips on purpose. A Gen1 device
// never returns the value, so apply writes it and no read-back can confirm it. Every other
// applicable field is compared on verify.
var writeOnlyApplicablePaths = map[string]bool{
	"mqtt.pass": true,
	"auth.pass": true,
}

// TestDeviceSchemaApplicableFieldsAreVerifiedOrNamed fails when verify skips an applicable field
// that is not named above, and when a named field is no longer both applicable and skipped.
func TestDeviceSchemaApplicableFieldsAreVerifiedOrNamed(t *testing.T) {
	comparator := NewConfigComparator()
	// The comparator checks a rule for every section before it descends into it, so a skip rule
	// on an ancestor ("location") skips every field below it. Test the path and each ancestor.
	skippedByVerify := func(path string) bool {
		parts := strings.Split(path, ".")
		for n := 1; n <= len(parts); n++ {
			rule := comparator.findRule(strings.Join(parts[:n], "."))
			if rule != nil && rule.SkipCompare {
				return true
			}
		}
		return false
	}
	for path := range gen1ApplicablePaths {
		skipped := skippedByVerify(path)
		if skipped && !writeOnlyApplicablePaths[path] {
			t.Errorf("%s is applicable but verify skips it; compare it or name it as write-only", path)
		}
		if !skipped && writeOnlyApplicablePaths[path] {
			t.Errorf("%s is named write-only but verify now compares it; drop it from the list", path)
		}
	}
	for path := range writeOnlyApplicablePaths {
		if !gen1ApplicablePaths[path] {
			t.Errorf("%s is named write-only but is not an applicable path", path)
		}
	}
}

func TestDeviceSchemaPlugApplicableFields(t *testing.T) {
	leaves := map[string]map[string]interface{}{}
	schemaLeaves("", GetDeviceConfigurationSchema("SHPLG-S"), leaves)
	var got []string
	for path, leaf := range leaves {
		if leaf[SchemaKeyApplicable] == true {
			got = append(got, path)
		}
	}
	sort.Strings(got)
	var want []string
	for path := range gen1ApplicablePaths {
		// A plug has no roller section, so a roller path is applicable on another model only.
		if _, onPlug := leaves[path]; onPlug {
			want = append(want, path)
		}
	}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SHPLG-S applicable fields\n got %v\nwant %v", got, want)
	}
}

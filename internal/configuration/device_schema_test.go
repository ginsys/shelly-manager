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
		"SHPLG-S": {"actions", "auth", "cloud", "coiot", "led", "location", "mqtt", "network", "power_metering", "relay", "system", "wifi"},
		"SHSW-1":  {"actions", "auth", "cloud", "coiot", "input", "location", "mqtt", "network", "relay", "system", "wifi"},
		"SHSW-25": {"actions", "auth", "cloud", "coiot", "input", "location", "mqtt", "network", "power_metering", "relay", "roller", "system", "wifi"},
		// The i3 has no actions section in this slice: apply refuses it whole.
		"SHIX3-1": {"auth", "cloud", "coiot", "input", "location", "mqtt", "network", "system", "wifi"},
		// A Dimmer has light channels and no relay; it has the status LED and its device-level group.
		"SHDM-1": {"auth", "cloud", "coiot", "dimming", "led", "light", "location", "mqtt", "network", "system", "wifi"},
		"SHDM-2": {"auth", "cloud", "coiot", "dimming", "led", "light", "location", "mqtt", "network", "system", "wifi"},
		// An RGBW2 has light channels (color: one, white: four) and no relay, and no Dimmer group.
		"SHRGBW2": {"auth", "cloud", "coiot", "light", "location", "mqtt", "network", "system", "wifi"},
		"unknown": {"actions", "auth", "cloud", "coiot", "dimming", "input", "led", "light", "location", "mqtt", "network", "power_metering", "relay", "roller", "system", "wifi"},
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

// A plug keeps both of its LED keys: the Dimmer's status-LED-only section changes nothing for it.
func TestDeviceSchemaPlugKeepsBothLEDLeaves(t *testing.T) {
	for _, model := range []string{"SHPLG-S", "SHPLG-1"} {
		leaves := map[string]map[string]interface{}{}
		schemaLeaves("", GetDeviceConfigurationSchema(model), leaves)
		for _, path := range []string{"led.power_indication", "led.network_indication"} {
			if leaf, ok := leaves[path]; !ok || leaf[SchemaKeyApplicable] != true {
				t.Errorf("%s: %s = %v, want an applicable leaf", model, path, leaf)
			}
		}
	}
}

// The Dimmer's light fields are applicable, and so are the five device-level dimming fields apply
// writes on POST /settings, except that the Dimmer 1 has no leading_edge leaf (apply always refuses
// it there). Every other dimming leaf is not applicable. The Dimmer has the status LED
// and no power LED, so its led section holds network_indication and no power_indication, while the
// plug keeps both.
func TestDeviceSchemaDimmerMarksLightAndFiveDimmingFieldsApplicable(t *testing.T) {
	written := map[string]bool{
		"dimming.min_brightness": true, "dimming.fade_rate": true, "dimming.transition": true,
		"dimming.zcross_debounce": true, "dimming.leading_edge": true,
	}
	for _, model := range []string{"SHDM-1", "SHDM-2"} {
		leaves := map[string]map[string]interface{}{}
		schemaLeaves("", GetDeviceConfigurationSchema(model), leaves)
		for path := range written {
			leaf, ok := leaves[path]
			if path == "dimming.leading_edge" && model == "SHDM-1" {
				// The Dimmer 1 never reports pulse_mode_detected, so apply always refuses the field there.
				if ok {
					t.Errorf("%s: %s must not be in the schema, apply always refuses it", model, path)
				}
				continue
			}
			if !ok {
				t.Errorf("%s: no schema leaf %s", model, path)
			} else if leaf[SchemaKeyApplicable] != true {
				t.Errorf("%s: %s is not applicable", model, path)
			}
		}
		if leaf, ok := leaves["led.network_indication"]; !ok || leaf[SchemaKeyApplicable] != true {
			t.Errorf("%s: led.network_indication = %v, want an applicable leaf", model, leaf)
		}
		if _, ok := leaves["led.power_indication"]; ok {
			t.Errorf("%s: a Dimmer has no power LED", model)
		}
		for path, leaf := range leaves {
			if strings.HasPrefix(path, "dimming.") && !written[path] && leaf[SchemaKeyApplicable] != false {
				t.Errorf("%s: %s = %v, want false", model, path, leaf[SchemaKeyApplicable])
			}
		}
		for _, field := range gen1LightKeys {
			path := "light.lights[]." + field
			leaf, ok := leaves[path]
			if !ok {
				t.Errorf("%s: no schema leaf %s", model, path)
				continue
			}
			if leaf[SchemaKeyApplicable] != true {
				t.Errorf("%s: %s is not applicable", model, path)
			}
		}
		if _, ok := leaves["light.lights[].name"]; ok {
			t.Errorf("%s: a light name is an attribute, not a parameter", model)
		}
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
		if step, ok := leaf["multipleOf"].(int); ok && step > 0 {
			return step
		}
		if minimum, ok := leaf["minimum"].(int); ok && minimum > 1 {
			return minimum
		}
		if maximum, ok := leaf["maximum"].(int); ok && maximum < 1 {
			return maximum // a range below zero, such as a signal threshold in dBm
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

// actionsLeafConfig builds one action entry of a model with a single field of the schema set from the
// leaf. An action is identified by its name and index, so the entry always holds a name the model
// has and index 0, and the leaf under test overrides one of them.
func actionsLeafConfig(t *testing.T, deviceType, field string, leaf map[string]interface{}) *DeviceConfiguration {
	t.Helper()
	row, ok := gen1ActionTable(deviceType)
	if !ok {
		return &DeviceConfiguration{Actions: &[]ActionConfig{{Name: "btn_on_url"}}}
	}
	entry := ActionConfig{Name: row.names[0]}
	switch field {
	case "name":
		entry.Name = sampleValue(leaf).(string)
	case "index":
		entry.Index = leaf["maximum"].(int)
	case "enabled":
		entry.Enabled = BoolPtr(true)
	case "secret_urls":
		entry.SecretURLs = &[]string{"http://192.168.1.4/on"}
	default:
		t.Fatalf("actions[].%s has no sample", field)
	}
	return &DeviceConfiguration{Actions: &[]ActionConfig{entry}}
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
			if strings.HasPrefix(path, "actions[].") {
				cfg = actionsLeafConfig(t, deviceType, strings.TrimPrefix(path, "actions[]."), leaf)
			}
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
	"mqtt.pass":     true,
	"auth.pass":     true,
	"wifi.password": true,
	"wifi.ap.pass":  true,
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
		// A plug has no roller or input section, so those paths are applicable on another model only.
		if _, onPlug := leaves[path]; onPlug {
			want = append(want, path)
		}
	}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SHPLG-S applicable fields\n got %v\nwant %v", got, want)
	}
}

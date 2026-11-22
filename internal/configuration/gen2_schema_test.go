package configuration

import (
	"sort"
	"strings"
	"testing"
)

const gen2TestModel = "SNSW-001X16EU"

func gen2SchemaLeaves(t *testing.T) map[string]map[string]interface{} {
	t.Helper()
	leaves := map[string]map[string]interface{}{}
	schemaLeaves("", GetDeviceConfigurationSchemaFor(gen2TestModel, 2), leaves)
	return leaves
}

// gen2Samples are the values a leaf is probed with: every word of an enum, else one sample.
func gen2Samples(leaf map[string]interface{}) []interface{} {
	if enum, ok := leaf["enum"].([]string); ok && len(enum) > 0 {
		values := make([]interface{}, len(enum))
		for i, word := range enum {
			values[i] = word
		}
		return values
	}
	if leaf["type"] == "boolean" {
		return []interface{}{true, false}
	}
	// A number is probed at both bounds the schema states, so a range wider than apply's fails.
	if leaf["type"] == "integer" || leaf["type"] == "number" {
		values := []interface{}{sampleValue(leaf)}
		for _, bound := range []string{"minimum", "maximum"} {
			switch v := leaf[bound].(type) {
			case int, float64:
				values = append(values, v)
			}
		}
		return values
	}
	if leaf["pattern"] == gen2ClockTime.String() {
		return []interface{}{"12:00"}
	}
	return []interface{}{sampleValue(leaf)}
}

// gen2OneSidedPaths are the fields apply accepts for some values only and the schema leaves unmarked
// on purpose: apply refuses wifi.enable false, so the form must not offer the switch.
var gen2OneSidedPaths = map[string]bool{"wifi.enable": true}

// TestGen2DeviceSchemaApplicableMatchesApply sets each field of the Gen2 schema alone, and each word of
// an enum in turn, and asks the Gen2 refusal check whether it would write it. A field is applicable
// exactly when apply accepts every value probed, and a field that is not applicable has no accepted one.
func TestGen2DeviceSchemaApplicableMatchesApply(t *testing.T) {
	converter := NewGen2Converter(nil)
	applicable := 0
	for path, leaf := range gen2SchemaLeaves(t) {
		if strings.HasSuffix(path, "[].id") {
			continue // an array item's index, not a setting
		}
		accepted, refused := 0, 0
		for _, value := range gen2Samples(leaf) {
			unsupported, err := unsupportedGen2Settings(converter, gen2ProbeConfig(t, path, value), true)
			if err != nil {
				t.Fatalf("%s=%v: %v", path, value, err)
			}
			if len(unsupported) == 0 {
				accepted++
			} else {
				refused++
			}
		}
		marked := leaf[SchemaKeyApplicable] == true
		switch {
		case marked && refused > 0:
			t.Errorf("%s: marked applicable, but apply refuses %d of %d values", path, refused, accepted+refused)
		case gen2OneSidedPaths[path] && (marked || accepted == 0 || refused == 0):
			t.Errorf("%s: named one-sided, but it is marked or apply no longer accepts only some values", path)
		case !marked && accepted > 0 && !gen2OneSidedPaths[path]:
			t.Errorf("%s: marked not applicable, but apply accepts %d of %d values", path, accepted, accepted+refused)
		}
		if marked {
			applicable++
		}
	}
	if applicable == 0 {
		t.Error("no applicable field, the comparison proves nothing")
	}
}

// gen2ProbeConfig is configWithLeaf with the companions a Gen2 write needs next to one setting.
func gen2ProbeConfig(t *testing.T, path string, value interface{}) *DeviceConfiguration {
	t.Helper()
	if path == "auth.enable" && value == false {
		return &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(false)}} // no login to name
	}
	cfg := configWithLeaf(t, path, value)
	switch path {
	case "mqtt.pass":
		cfg.MQTT.User = StringPtr("user") // the device takes the password only with the user
	case "light.lights[].night_mode_start":
		cfg.Light.Lights[0].NightModeEnd = StringPtr("13:00") // the window is set as a pair
	case "light.lights[].night_mode_end":
		cfg.Light.Lights[0].NightModeStart = StringPtr("12:00")
	}
	return cfg
}

// TestGen2ApplicablePathsAreSchemaLeaves ties the derived and listed paths to the schema: each is a
// leaf of it, and each key of the maps the Gen2 check reads has one, so a key with no leaf fails.
func TestGen2ApplicablePathsAreSchemaLeaves(t *testing.T) {
	leaves := gen2SchemaLeaves(t)
	for path := range gen2ApplicablePaths {
		if _, ok := leaves[path]; !ok {
			t.Errorf("%s is applicable on Gen2 and is not a leaf of the Gen2 schema", path)
		}
	}
	for _, source := range []struct {
		prefixes []string
		keyed    map[string]bool
	}{
		// gen2ChannelFields mixes switch and input keys, so a key needs a leaf under one of the two.
		{[]string{"relay.relays[].", "input.inputs[]."}, gen2ChannelFields},
		{[]string{"light.lights[]."}, gen2LightFields}, {[]string{"roller."}, gen2RollerFields},
	} {
		for key := range source.keyed {
			held := false
			for _, prefix := range source.prefixes {
				_, ok := leaves[prefix+key]
				held = held || ok
			}
			if !held {
				t.Errorf("%v%s is a Gen2 key and no schema leaf holds it", source.prefixes, key)
			}
		}
	}
}

func TestGen2DeviceSchemaSections(t *testing.T) {
	sections, _ := GetDeviceConfigurationSchemaFor(gen2TestModel, 2)["properties"].(map[string]interface{})
	var got []string
	for name := range sections {
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{"auth", "cloud", "input", "light", "location", "mqtt", "network", "relay", "roller", "system", "wifi"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("sections = %v, want %v", got, want)
	}
	if GetDeviceConfigurationSchemaFor(gen2TestModel, 2)[SchemaKeyApplySupported] != true {
		t.Error("a Gen2 device is supported")
	}
	if GetDeviceConfigurationSchemaFor(gen2TestModel, 0)[SchemaKeyApplySupported] != false {
		t.Error("a model with no generation stays unsupported")
	}
}

// A field the form accepts and apply refuses is a mismatch: the Gen2 login has one user, and the cover
// takes 1 to 300 seconds, a range the shared roller schema leaves open at the top (and at 0 for the delay).
func TestGen2DeviceSchemaTakesTheGen2LoginAndCoverRanges(t *testing.T) {
	leaves := gen2SchemaLeaves(t)
	if enum, _ := leaves["auth.user"]["enum"].([]string); len(enum) != 1 || enum[0] != gen2LoginUser {
		t.Errorf("auth.user enum = %v, want only %q", enum, gen2LoginUser)
	}
	for _, name := range []string{"max_open_time", "max_close_time", "obstacle_delay"} {
		leaf := leaves["roller."+name]
		if leaf["minimum"] != gen2CoverSecondsMin || leaf["maximum"] != gen2CoverSecondsMax {
			t.Errorf("roller.%s range = %v to %v, want %d to %d", name, leaf["minimum"], leaf["maximum"], gen2CoverSecondsMin, gen2CoverSecondsMax)
		}
	}
	gen1 := map[string]map[string]interface{}{}
	schemaLeaves("", GetDeviceConfigurationSchema("SHSW-25"), gen1)
	if gen1["roller.obstacle_delay"]["minimum"] != 0 || gen1["roller.max_open_time"]["maximum"] != nil {
		t.Error("the Gen1 roller schema must keep its own range")
	}
}

func TestTemplateSchemaMarksGen2OnlyFields(t *testing.T) {
	leaves := map[string]map[string]interface{}{}
	schemaLeaves("", GetTemplateConfigurationSchema(), leaves)
	for _, path := range []string{"relay.relays[].voltage_limit", "light.lights[].in_mode", "input.inputs[].factory_reset"} {
		leaf, ok := leaves[path]
		if !ok || leaf[SchemaKeyApplicable] != true {
			t.Errorf("%s: a field only Gen2 writes must be applicable in a template (leaf %v)", path, leaf)
		}
	}
	if leaves["power_metering.max_power"][SchemaKeyApplicable] != true {
		t.Error("a Gen1 field stays applicable in a template")
	}
}

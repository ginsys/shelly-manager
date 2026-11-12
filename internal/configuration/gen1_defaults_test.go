package configuration

import (
	"encoding/json"
	"sort"
	"testing"
)

// gen1NoDocumentedDefault lists every applied path whose default the Gen1 API reference does not
// state. The reference gives the values of relay default_state or the range of the CoIoT period,
// and gives no default for them, so these fields show none. A path leaves this list only by gaining
// a documented entry in gen1FieldDefaults, with its source named there.
var gen1NoDocumentedDefault = []string{
	"system.device.name", "system.device.eco_mode", "system.device.discoverable",
	"location.tz", "location.lat", "location.lng",
	"power_metering.max_power",
	"led.power_indication", "led.network_indication",
	"mqtt.enable", "mqtt.server", "mqtt.user", "mqtt.pass", "mqtt.id",
	"mqtt.clean_session", "mqtt.keep_alive",
	"cloud.enable",
	"coiot.enable", "coiot.update_period", "coiot.peer",
	"auth.user", "auth.pass",
	"relay.relays[].name", "relay.relays[].default_state", "relay.relays[].auto_on",
	"relay.relays[].auto_off", "relay.relays[].appliance_type", "relay.relays[].schedule",
	"relay.relays[].schedule_rules", "relay.relays[].max_power",
	"roller.max_open_time", "roller.max_close_time", "roller.motor_direction",
	"roller.swap_inputs", "roller.input_mode",
	"roller.button_reverse", "roller.button_type", "roller.default_state", "roller.ends_delay",
	"roller.obstacle_action", "roller.obstacle_delay", "roller.obstacle_mode", "roller.obstacle_power",
	"roller.off_power", "roller.safety_action", "roller.safety_allowed_on_trigger", "roller.safety_mode",
	"roller.schedule", "roller.schedule_rules",
	"input.inputs[].mode", "input.inputs[].inverted",
	"wifi.enable", "wifi.ssid", "wifi.password", "wifi.ipv4mode",
	"wifi.static_ip.ip", "wifi.static_ip.netmask", "wifi.static_ip.gw", "wifi.static_ip.nameserver",
	"wifi.ap.enable", "wifi.ap.pass",
}

// The form shows 1883 as a placeholder for an empty port. Apply must not turn that into a value:
// a server with no port is written as the bare host, never "host:" or "host:0".
func TestGen1WritesAServerWithNoPortAsTheBareHost(t *testing.T) {
	apiJSON, err := NewGen1Converter(nil).ToAPIConfig(&DeviceConfiguration{MQTT: &MQTTConfiguration{Server: StringPtr("broker.local")}}, "SHPLG-S")
	if err != nil {
		t.Fatal(err)
	}
	var gen1 map[string]map[string]interface{}
	if err = json.Unmarshal(apiJSON, &gen1); err != nil {
		t.Fatal(err)
	}
	if got := gen1["mqtt"]["server"]; got != "broker.local" {
		t.Errorf("mqtt server = %v, want the bare host broker.local", got)
	}
}

// Every applied path either has a documented default or is named as having none, so a new applied
// field cannot arrive without one of the two, and a default cannot be invented for a field.
func TestGen1AppliedFieldsHaveADefaultOrAreNamed(t *testing.T) {
	none := map[string]bool{}
	for _, path := range gen1NoDocumentedDefault {
		if none[path] {
			t.Errorf("%s is listed twice as having no documented default", path)
		}
		none[path] = true
	}
	var missing []string
	for path := range gen1ApplicablePaths {
		_, hasDefault := gen1FieldDefaults[path]
		switch {
		case hasDefault && none[path]:
			t.Errorf("%s has a documented default and is also listed as having none", path)
		case !hasDefault && !none[path]:
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("applied paths with neither a default in gen1FieldDefaults nor a place in gen1NoDocumentedDefault: %v", missing)
	}
	for path := range none {
		if !gen1ApplicablePaths[path] {
			t.Errorf("%s is listed as having no documented default but is not an applied path", path)
		}
	}
	for path := range gen1FieldDefaults {
		if !gen1ApplicablePaths[path] {
			t.Errorf("gen1FieldDefaults names %s, which apply never writes", path)
		}
	}
}

// A form shows a default as a placeholder and a "Use default" link, so a default on a field apply
// never writes tells the user that something happens which does not, and an applied sibling with
// none looks like the field that does nothing. The schema therefore carries a default on an applied
// field only, and only the documented one.
func TestDeviceSchemaDefaultsSitOnlyOnAppliedFieldsAndAreDocumented(t *testing.T) {
	schemas := map[string]map[string]interface{}{"template": GetTemplateConfigurationSchema()}
	for _, deviceType := range []string{"SHSW-25", "SHPLG-S", "SHSW-1", "SHIX3-1", "NOT-A-MODEL"} {
		schemas[deviceType] = GetDeviceConfigurationSchema(deviceType)
	}
	for deviceType, schema := range schemas {
		leaves := map[string]map[string]interface{}{}
		schemaLeaves("", schema, leaves)
		if len(leaves) == 0 {
			t.Fatalf("model %q: the schema has no leaf, so the check ran on nothing", deviceType)
		}
		shown := 0
		for path, leaf := range leaves {
			value, hasDefault := leaf["default"]
			applicable, _ := leaf[SchemaKeyApplicable].(bool)
			want, documented := gen1FieldDefaults[path]
			switch {
			case hasDefault && !applicable:
				t.Errorf("model %q: %s shows the default %v but apply never writes it", deviceType, path, value)
			case hasDefault && (!documented || value != want):
				t.Errorf("model %q: %s shows the default %v, which the Gen1 reference does not state", deviceType, path, value)
			case !hasDefault && applicable && documented:
				t.Errorf("model %q: %s is applied and has the documented default %v but shows none", deviceType, path, want)
			}
			if hasDefault {
				shown++
			}
		}
		// The control: the template schema applies every documented field, so it must show all of them.
		if deviceType == "template" && shown != len(gen1FieldDefaults) {
			t.Errorf("template schema shows %d defaults, want the %d documented ones", shown, len(gen1FieldDefaults))
		}
	}
}

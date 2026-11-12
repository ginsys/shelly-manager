package configuration

import (
	"encoding/json"
	"os"
	"testing"
)

type hostCases struct {
	Hostname hostCaseList `json:"hostname"`
	Hostport hostCaseList `json:"hostport"`
	Endpoint hostCaseList `json:"endpoint"`
}

type hostCaseList struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

// loadHostCases reads the list that the config form's test reads too, so the two sides cannot
// disagree about a value without one of the tests failing.
func loadHostCases(t *testing.T) hostCases {
	t.Helper()
	const path = "../../testdata/config/hosts.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var cases hostCases
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if len(cases.Hostname.Valid) == 0 || len(cases.Hostname.Invalid) == 0 ||
		len(cases.Hostport.Valid) == 0 || len(cases.Hostport.Invalid) == 0 ||
		len(cases.Endpoint.Valid) == 0 || len(cases.Endpoint.Invalid) == 0 {
		t.Fatalf("%s must list valid and invalid values for hostname, hostport and endpoint", path)
	}
	return cases
}

// fieldErrors validates a one-field body and returns the messages by path.
func fieldErrors(t *testing.T, section, key, value string, nested ...string) map[string]string {
	t.Helper()
	var leaf interface{} = map[string]interface{}{key: value}
	for i := len(nested) - 1; i >= 0; i-- {
		leaf = map[string]interface{}{nested[i]: leaf}
	}
	body := map[string]interface{}{section: leaf}
	got := map[string]string{}
	for _, e := range ValidateConfigBody("", body) {
		got[e.Path] = e.Message
	}
	return got
}

func TestValidTimeServerOnTheSharedList(t *testing.T) {
	cases := loadHostCases(t)
	for _, s := range cases.Hostname.Valid {
		if !validTimeServer(s) {
			t.Errorf("validTimeServer(%q) = false, want true", s)
		}
	}
	for _, s := range cases.Hostname.Invalid {
		if validTimeServer(s) {
			t.Errorf("validTimeServer(%q) = true, want false", s)
		}
	}
}

// A name whose last label is all digits is a mistyped address, so it must be a valid IPv4 address.
func TestValidTimeServerRefusesAMistypedAddress(t *testing.T) {
	for _, s := range []string{"192.168.1.300", "300.1.1.1", "1.2.3", "10.0.0.256", "example.123"} {
		if validTimeServer(s) {
			t.Errorf("validTimeServer(%q) = true, want false", s)
		}
	}
	for _, s := range []string{"192.168.1.30", "ntp1.example.com", "1.example.com"} {
		if !validTimeServer(s) {
			t.Errorf("validTimeServer(%q) = false, want true", s)
		}
	}
}

// The schema rule and the apply rule are one function: for every value on the shared list, the
// time server field answers as validTimeServer does, and an empty value passes.
func TestTimeServerFieldFollowsValidTimeServer(t *testing.T) {
	cases := loadHostCases(t)
	const name = "system.sntp.server"
	const msg = "Must be a host name or an IPv4 address"
	for _, s := range cases.Hostname.Valid {
		if got := fieldErrors(t, "system", "server", s, "sntp"); len(got) != 0 {
			t.Errorf("%s %q: want no error, got %v", name, s, got)
		}
	}
	for _, s := range cases.Hostname.Invalid {
		got := fieldErrors(t, "system", "server", s, "sntp")
		if len(got) != 1 || got[name] != msg {
			t.Errorf("%s %q: want %q, got %v", name, s, msg, got)
		}
	}
	if got := fieldErrors(t, "system", "server", "", "sntp"); len(got) != 0 {
		t.Errorf("%s empty: want no error, got %v", name, got)
	}
}

// A Gen1 device stores the MQTT broker as host:port and apply accepts that form
// (TestApplyDesiredConfig_MQTTCombinedHostPortVerifies), so a stored value of either form must
// still save: the field takes a host as validTimeServer reads it, with or without a port.
func TestMQTTServerFieldTakesAHostWithOrWithoutAPort(t *testing.T) {
	cases := loadHostCases(t)
	const msg = "Must be a host name or an IP address, optionally followed by :port (an IPv6 address takes its port in the Port field)"
	for _, s := range append(append([]string{}, cases.Endpoint.Valid...), cases.Hostname.Valid...) {
		if got := fieldErrors(t, "mqtt", "server", s); len(got) != 0 {
			t.Errorf("mqtt.server %q: want no error, got %v", s, got)
		}
	}
	for _, s := range cases.Endpoint.Invalid {
		got := fieldErrors(t, "mqtt", "server", s)
		if len(got) != 1 || got["mqtt.server"] != msg {
			t.Errorf("mqtt.server %q: want %q, got %v", s, msg, got)
		}
	}
	if got := fieldErrors(t, "mqtt", "server", ""); len(got) != 0 {
		t.Errorf("mqtt.server empty: want no error, got %v", got)
	}
}

// The form check and the Gen2 apply both judge mqtt.server, so no value on the shared list may be
// saved by the form and then refused at apply, and no one-string value that Gen2 apply refuses
// may pass the form. (A host:port beside a separate port is a two-field case that a one-field
// check cannot see; the Gen2 apply refuses it on its own.)
func TestMQTTServerFieldAgreesWithTheGen2Apply(t *testing.T) {
	cases := loadHostCases(t)
	converter := NewGen2Converter(nil)
	refused := 0
	values := append(append([]string{}, cases.Endpoint.Valid...), cases.Endpoint.Invalid...)
	for _, s := range values {
		unsupported, err := unsupportedGen2Settings(converter, &DeviceConfiguration{MQTT: &MQTTConfiguration{Server: StringPtr(s)}})
		if err != nil {
			continue // a value the Gen2 converter cannot even convert is not accepted by apply
		}
		refusedAtApply := false
		for _, path := range unsupported {
			refusedAtApply = refusedAtApply || path == "mqtt.server"
		}
		savedByForm := len(fieldErrors(t, "mqtt", "server", s)) == 0
		if refusedAtApply {
			refused++
		}
		if refusedAtApply && savedByForm {
			t.Errorf("mqtt.server %q: the form saves it and Gen2 apply refuses it", s)
		}
	}
	// The control: the list holds a bracketed one-string broker, which Gen2 apply refuses, so a
	// zero here means the loop above never saw a refusal and proved nothing.
	if refused == 0 {
		t.Error("no value on the list was refused by Gen2 apply: the agreement check ran on nothing")
	}
}

func TestCoIoTPeerAcceptsEmptyOrHostAndPort(t *testing.T) {
	cases := loadHostCases(t)
	const msg = "Must be host:port, with a host name or IPv4 address and a port from 1 to 65535"
	for _, s := range cases.Hostport.Valid {
		if got := fieldErrors(t, "coiot", "peer", s); len(got) != 0 {
			t.Errorf("coiot.peer %q: want no error, got %v", s, got)
		}
	}
	for _, s := range cases.Hostport.Invalid {
		got := fieldErrors(t, "coiot", "peer", s)
		if len(got) != 1 || got["coiot.peer"] != msg {
			t.Errorf("coiot.peer %q: want %q, got %v", s, msg, got)
		}
	}
	if got := fieldErrors(t, "coiot", "peer", ""); len(got) != 0 {
		t.Errorf("coiot.peer empty: want no error (multicast), got %v", got)
	}
}

// The three fields carry the format that the form reads from the schema.
func TestHostFieldsCarryTheirFormat(t *testing.T) {
	sections := GetDeviceConfigurationSchema("")["properties"].(map[string]interface{})
	field := func(section string, path ...string) map[string]interface{} {
		node := sections[section].(map[string]interface{})
		for _, p := range path {
			node = node["properties"].(map[string]interface{})[p].(map[string]interface{})
		}
		return node
	}
	for name, want := range map[string]struct {
		node   map[string]interface{}
		format string
	}{
		"system.sntp.server": {field("system", "sntp", "server"), "hostname"},
		"mqtt.server":        {field("mqtt", "server"), "endpoint"},
		"coiot.peer":         {field("coiot", "peer"), "hostport"},
	} {
		if want.node["format"] != want.format {
			t.Errorf("%s: format %v, want %s", name, want.node["format"], want.format)
		}
	}
}

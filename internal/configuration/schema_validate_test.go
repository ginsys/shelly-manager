package configuration

import (
	"encoding/json"
	"testing"
)

func decodeBody(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(s), &body); err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return body
}

func TestValidateConfigBodyAcceptsAValidPlugBody(t *testing.T) {
	body := decodeBody(t, `{
		"power_metering": {"max_power": 1400},
		"led": {"power_disable": false},
		"relay": {"default_state": "last", "auto_off": 3600, "relays": [{"auto_off": 60, "default_state": "on"}]},
		"network": {"anything": "is not checked"}
	}`)
	if errs := ValidateConfigBody("SHPLG-S", body); len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
}

func TestValidateConfigBodyReportsEachField(t *testing.T) {
	body := decodeBody(t, `{
		"power_metering": {"max_power": -5},
		"relay": {"default_state": "sometimes", "auto_off": 1.5, "relays": [{"auto_off": "soon"}]},
		"system": {"device": {"name": 7}},
		"roller": {"max_time": 10}
	}`)
	got := map[string]string{}
	for _, e := range ValidateConfigBody("SHPLG-S", body) {
		got[e.Path] = e.Message
	}
	want := map[string]string{
		"power_metering.max_power": "Must be at least 0",
		"relay.default_state":      "Must be one of: off, on, last, switch",
		"relay.auto_off":           "Must be a whole number",
		"relay.relays.0.auto_off":  "Must be a whole number",
		"system.device.name":       "Must be text",
		"roller":                   "SHPLG-S has no roller settings",
	}
	for path, msg := range want {
		if got[path] != msg {
			t.Errorf("%s: got %q, want %q", path, got[path], msg)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d errors %v, want %d", len(got), got, len(want))
	}
}

func TestValidateConfigBodyGlobalSchemaHasEverySection(t *testing.T) {
	body := decodeBody(t, `{"roller": {"max_time": 10}, "dimming": {"default_state": true}}`)
	if errs := ValidateConfigBody("", body); len(errs) != 0 {
		t.Fatalf("a global template may hold every section, got %v", errs)
	}
	if errs := ValidateConfigBody("", decodeBody(t, `{"dimming": {"default_state": "on"}}`)); len(errs) != 1 {
		t.Fatalf("want one type error, got %v", errs)
	}
}

func TestValidateConfigBodyChecksLengthAndPattern(t *testing.T) {
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	body := map[string]interface{}{"auth": map[string]interface{}{"user": string(long)}}
	errs := ValidateConfigBody("SHPLG-S", body)
	if len(errs) != 1 || errs[0].Message != "Must be at most 64 characters" {
		t.Fatalf("want a length error, got %v", errs)
	}
}

func TestValidateConfigBodyChecksTheIPv4Format(t *testing.T) {
	body := decodeBody(t, `{
		"wifi": {"static_ip": {"ip": "300.1.1.1", "netmask": "255.255.255.0", "gw": "fe80::1", "nameserver": "dns.example"}},
		"network": {
			"eth": {"ip": {"ip": "192.168.1.256"}},
			"wifi": {
				"sta": {"ip": {"gw": "1.2.3"}, "nameserver": "8.8.8.8x"},
				"ap": {"range_start": "10.0.0.0.1", "range_end": " 10.0.0.9"}
			}
		}
	}`)
	got := map[string]string{}
	for _, e := range ValidateConfigBody("SHPLG-S", body) {
		got[e.Path] = e.Message
	}
	const msg = "Must be an IPv4 address"
	want := map[string]string{
		"wifi.static_ip.ip":           msg,
		"wifi.static_ip.gw":           msg,
		"wifi.static_ip.nameserver":   msg,
		"network.eth.ip.ip":           msg,
		"network.wifi.sta.ip.gw":      msg,
		"network.wifi.sta.nameserver": msg,
		"network.wifi.ap.range_start": msg,
		"network.wifi.ap.range_end":   msg,
	}
	for path, m := range want {
		if got[path] != m {
			t.Errorf("%s: got %q, want %q", path, got[path], m)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d errors %v, want %d", len(got), got, len(want))
	}
}

func TestValidateConfigBodyAcceptsAnIPv4AddressAndAnEmptyOne(t *testing.T) {
	body := decodeBody(t, `{
		"wifi": {"static_ip": {"ip": "192.0.2.50", "netmask": "255.255.255.0", "gw": "192.0.2.1", "nameserver": ""}},
		"network": {"eth": {"ip": {"ip": "10.0.0.2", "netmask": "", "gw": "10.0.0.1"}}}
	}`)
	if errs := ValidateConfigBody("SHPLG-S", body); len(errs) != 0 {
		t.Fatalf("want no errors, got %v", errs)
	}
}

// Every address field of the stored network section carries format ipv4, so the form and the
// server refuse the same values there as they do for wifi.static_ip.
func TestDeviceSchemaNetworkAddressFieldsCarryTheIPv4Format(t *testing.T) {
	sections := GetDeviceConfigurationSchema("SHPLG-S")["properties"].(map[string]interface{})
	network := sections["network"].(map[string]interface{})
	var missing []string
	var walk func(node map[string]interface{}, path string)
	walk = func(node map[string]interface{}, path string) {
		props, _ := node["properties"].(map[string]interface{})
		for name, child := range props {
			c := child.(map[string]interface{})
			p := path + "." + name
			if c["type"] == "object" {
				walk(c, p)
				continue
			}
			isAddress := name == "ip" || name == "netmask" || name == "gw" || name == "nameserver" ||
				name == "range_start" || name == "range_end"
			if isAddress && c["format"] != "ipv4" {
				missing = append(missing, p)
			}
		}
	}
	walk(network, "network")
	if len(missing) != 0 {
		t.Errorf("address fields without format ipv4: %v", missing)
	}
}

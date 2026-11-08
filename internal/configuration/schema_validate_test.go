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

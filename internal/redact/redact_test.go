package redact

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestIsKey(t *testing.T) {
	for _, key := range []string{"pass", "password", "auth_pass", "psk", "client_secret", "api_token", "key", "wifi_key", "PASS"} {
		if !IsKey(key) {
			t.Errorf("%q should name a secret", key)
		}
	}
	for _, key := range []string{"ssid", "user", "server", "keep_alive", "model", "keys_total"} {
		if IsKey(key) {
			t.Errorf("%q should not name a secret", key)
		}
	}
}

// A secret-named key that holds a list or an object is secret as a whole.
func TestMask_SecretContainerIsMaskedWhole(t *testing.T) {
	var tree any
	text := `{"api_tokens":["one","two",{"v":"three","n":7,"on":true}],"tokens":{"a":"x","b":["y"]},` +
		`"keep":["visible"],"secrets":[]}`
	if err := json.Unmarshal([]byte(text), &tree); err != nil {
		t.Fatal(err)
	}
	Mask(tree, "[M]")
	want := `{"api_tokens":["[M]","[M]",{"v":"[M]","n":"[M]","on":true}],"tokens":{"a":"[M]","b":["[M]"]},` +
		`"keep":["visible"],"secrets":[]}`
	var wantTree any
	if err := json.Unmarshal([]byte(want), &wantTree); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tree, wantTree) {
		got, _ := json.Marshal(tree)
		t.Errorf("masked tree = %s\nwant %s", got, want)
	}
}

func TestMask(t *testing.T) {
	var tree any
	text := `{"mqtt":{"pass":"s1","user":"u","port":1883},"wifi_sta":{"key":"s2","ssid":"home"},` +
		`"list":[{"token":12345678},{"label":"keep"}],"auth_pass":"","password_set":true,` +
		`"pass":{"nested_pass":"s3"}}`
	if err := json.Unmarshal([]byte(text), &tree); err != nil {
		t.Fatal(err)
	}
	Mask(tree, "[M]")
	want := `{"mqtt":{"pass":"[M]","user":"u","port":1883},"wifi_sta":{"key":"[M]","ssid":"home"},` +
		`"list":[{"token":"[M]"},{"label":"keep"}],"auth_pass":"","password_set":true,` +
		`"pass":{"nested_pass":"[M]"}}`
	var wantTree any
	if err := json.Unmarshal([]byte(want), &wantTree); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tree, wantTree) {
		got, _ := json.Marshal(tree)
		t.Errorf("masked tree = %s\nwant %s", got, want)
	}
}

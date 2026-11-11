package configuration

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bodyFromSchema builds a body that holds every property path of an object schema, with a null
// at each leaf. A list is left out: its items are checked by the key check's own cases.
func bodyFromSchema(schema map[string]interface{}) map[string]interface{} {
	props, _ := schema["properties"].(map[string]interface{})
	out := map[string]interface{}{}
	for key, raw := range props {
		child, _ := raw.(map[string]interface{})
		if child["type"] == "object" {
			out[key] = bodyFromSchema(child)
		} else {
			out[key] = nil
		}
	}
	return out
}

// Every key the form can produce must be a key of the stored configuration, or the key check
// would refuse a body that the form built from the schema.
func TestEverySchemaKeyIsAStoredKey(t *testing.T) {
	for _, deviceType := range []string{"", "SHPLG-S", "SHSW-1", "SHSW-25"} {
		raw, err := json.Marshal(bodyFromSchema(GetDeviceConfigurationSchema(deviceType)))
		require.NoError(t, err)
		assert.Empty(t, CheckConfigKeys(raw), "schema of %q", deviceType)
	}
}

func TestCheckConfigKeysNamesTheKey(t *testing.T) {
	cases := map[string]map[string]string{
		`{"relay":{"auto_off":5}}`:              {},
		`{"Relay":{}}`:                          {"Relay": `Unknown key "Relay"; the key is "relay"`},
		`{"relay":{"AUTO_OFF":1}}`:              {"relay.AUTO_OFF": `Unknown key "AUTO_OFF"; the key is "auto_off"`},
		`{"relay":{"auto_of":1}}`:               {"relay.auto_of": `Unknown key "auto_of"`},
		`{"relay":{},"relay":{}}`:               {"relay": `Duplicate key "relay"`},
		`{"relay":{"auto_off":1,"auto_off":2}}`: {"relay.auto_off": `Duplicate key "auto_off"`},
		`null`:                                  {},
		`{"relay":null}`:                        {},
		`{"mqtt":{"enable":true,"nope":1}}`:     {"mqtt.nope": `Unknown key "nope"`},
	}
	for body, want := range cases {
		got := map[string]string{}
		for _, e := range CheckConfigKeys([]byte(body)) {
			got[e.Path] = e.Message
		}
		assert.Equal(t, want, got, body)
	}
}

func TestCheckRequestKeysLeavesUnknownKeysAlone(t *testing.T) {
	type request struct {
		Name   string `json:"name"`
		Config string `json:"config"`
	}
	got := map[string]string{}
	for _, e := range CheckRequestKeys([]byte(`{"name":"a","Name":"b","scope":"x","config":1,"config":2}`), reflect.TypeOf(request{})) {
		got[e.Path] = e.Message
	}
	assert.Equal(t, map[string]string{
		"Name":   `Unknown key "Name"; the key is "name"`,
		"config": `Duplicate key "config"`,
	}, got)
}

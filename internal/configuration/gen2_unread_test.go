package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UnreadSettings names what FromAPIConfig does not read, by trying each setting: one that changes
// the result when it is removed or changed is read, whatever its value.
func TestGen2Converter_UnreadSettings(t *testing.T) {
	doc := `{
		"sys":{"device":{"name":"n","mac":"AA","eco_mode":false},"cfg_rev":3,"ui_data":{}},
		"mqtt":{"enable":false,"ssl_ca":null,"status_ntf":false},
		"switch:0":{"id":0,"auto_on":false,"auto_on_delay":60,"auto_off":true,"auto_off_delay":30,"power_limit":10},
		"wifi":{"ap":{"enable":false,"is_open":true}},
		"ws":{"enable":false,"server":null},
		"modbus":{"pass":"s3cret"}
	}`
	paths, rest, err := NewGen2Converter(nil).UnreadSettings(json.RawMessage(doc))
	require.NoError(t, err)

	// A read setting at its zero value (eco_mode, enable, status_ntf, auto_on) is not reported: removing
	// it and leaving it out read the same, but changing it does not. A top-level key with no read
	// setting is named as the key. A delay with its timer off has no typed home, so it is reported.
	assert.Equal(t, []string{
		"modbus", "mqtt.ssl_ca", "switch:0.auto_on_delay", "switch:0.id", "switch:0.power_limit",
		"sys.cfg_rev", "sys.device.mac", "wifi.ap.is_open", "ws",
	}, paths)

	restJSON, err := json.Marshal(rest)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"sys":{"device":{"mac":"AA"},"cfg_rev":3},
		"mqtt":{"ssl_ca":null},
		"switch:0":{"id":0,"auto_on_delay":60,"power_limit":10},
		"wifi":{"ap":{"is_open":true}},
		"ws":{"enable":false,"server":null},
		"modbus":{"pass":"s3cret"}
	}`, string(restJSON), "the rest holds every unread setting with its value, and no read one, and no empty object")
}

func TestGen2Converter_UnreadSettingsRefusesAnythingButAnObject(t *testing.T) {
	_, _, err := NewGen2Converter(nil).UnreadSettings(json.RawMessage(`[1]`))
	assert.Error(t, err)
}

// input_id changes what is read (it picks the input that takes the switch's in_mode) but has no typed
// home, so it stays in the rest: a typed save must not lose it.
func TestGen2Converter_UnreadSettingsKeepsInputID(t *testing.T) {
	doc := `{"sys":{},"switch:0":{"id":0,"in_mode":"flip","input_id":1},"input:0":{"id":0},"input:1":{"id":1}}`
	paths, rest, err := NewGen2Converter(nil).UnreadSettings(json.RawMessage(doc))
	require.NoError(t, err)
	assert.Contains(t, paths, "switch:0.input_id")
	assert.Equal(t, float64(1), rest["switch:0"].(map[string]interface{})["input_id"])
}

// An enabled auto timer with a delay that is not a whole number above 0 reads as unset, so the flag has
// no typed value: it stays in the rest beside its delay, and a typed save loses neither.
func TestGen2Converter_UnreadSettingsKeepsATimerWithoutATypedValue(t *testing.T) {
	doc := `{"sys":{},"switch:0":{"id":0,"auto_off":true,"auto_off_delay":0.5,"auto_on":true,"auto_on_delay":30}}`
	paths, rest, err := NewGen2Converter(nil).UnreadSettings(json.RawMessage(doc))
	require.NoError(t, err)
	assert.Equal(t, []string{"switch:0.auto_off", "switch:0.auto_off_delay", "switch:0.id"}, paths)
	assert.Equal(t, map[string]interface{}{"auto_off": true, "auto_off_delay": 0.5, "id": float64(0)}, rest["switch:0"])
}

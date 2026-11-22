package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UnreadKeys names the top-level keys that FromAPIConfig reads nothing from, by the same rule that
// decides which keys the apply path writes. A key that is read is not named, whatever its sub-keys hold.
func TestGen2Converter_UnreadKeys(t *testing.T) {
	doc := `{
		"sys":{"device":{"name":"n"},"cfg_rev":3},
		"mqtt":{"enable":false,"ssl_ca":null},
		"cloud":{},
		"wifi":{"sta1":{"ssid":null}},
		"switch:0":{"id":0},"switch:12":{"id":12},"input:0":{"id":0},
		"ws":{"enable":false},"script:1":{"id":1},"ble":{},"modbus":{"pass":"s3cret"},
		"switch":{},"input:x":{}
	}`
	keys, err := NewGen2Converter(nil).UnreadKeys(json.RawMessage(doc))
	require.NoError(t, err)
	assert.Equal(t, []string{"input:x", "modbus", "script:1", "switch", "ws"}, keys, "ble is read since the Pro models' Bluetooth RPC is written")
}

func TestGen2Converter_UnreadKeysRefusesAnythingButAnObject(t *testing.T) {
	for _, in := range []string{`[1]`, `"x"`, `null`, `{`} {
		_, err := NewGen2Converter(nil).UnreadKeys(json.RawMessage(in))
		if in == `null` {
			// null decodes to an empty map: no key, no error.
			require.NoError(t, err)
			continue
		}
		assert.Error(t, err, in)
	}
}

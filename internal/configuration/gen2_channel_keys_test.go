package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Odd channel keys of a Gen2 document (ginsys/shelly-manager#802). A channel is the canonical key
// "switch:N" or "input:N" with an object value; anything else is a key that nothing is read from.

func TestGen2ChannelID_CanonicalSpellingOnly(t *testing.T) {
	for _, tc := range []struct {
		key string
		id  int
		ok  bool
	}{
		{"switch:0", 0, true},
		{"switch:12", 12, true},
		{"switch:007", 0, false},
		{"switch:01", 0, false},
		{"switch:00", 0, false},
		{"switch:+1", 0, false},
		{"switch:-1", 0, false},
		{"switch: 1", 0, false},
		{"switch:", 0, false},
		{"switch:1.0", 0, false},
		{"input:3", 0, false}, // another kind
	} {
		id, ok := gen2ChannelID(tc.key, "switch")
		assert.Equal(t, tc.ok, ok, tc.key)
		assert.Equal(t, tc.id, id, tc.key)
	}
	id, ok := gen2ChannelID("input:3", "input")
	assert.True(t, ok)
	assert.Equal(t, 3, id)
}

func TestGen2KeyApplies_NeedsAnObject(t *testing.T) {
	object := map[string]interface{}{}
	for _, key := range []string{"sys", "mqtt", "cloud", "wifi", "switch:0", "input:4"} {
		assert.True(t, gen2KeyApplies(key, object), key)
	}
	for _, value := range []interface{}{nil, "x", 1.0, true, []interface{}{}} {
		for _, key := range []string{"sys", "switch:0", "input:0"} {
			assert.False(t, gen2KeyApplies(key, value), "%s: %v", key, value)
		}
	}
	for _, key := range []string{"switch:007", "switch:+1", "ws", "switch", "input:x"} {
		assert.False(t, gen2KeyApplies(key, object), key)
	}
}

func TestGen2ChannelIDs_ListsAnIdOnce(t *testing.T) {
	doc := map[string]interface{}{
		"switch:1": map[string]interface{}{}, "switch:01": map[string]interface{}{},
		"switch:+1": map[string]interface{}{}, "switch:0": map[string]interface{}{},
		"input:2": map[string]interface{}{},
	}
	assert.Equal(t, []int{0, 1}, gen2ChannelIDs(doc, "switch"))
	assert.Equal(t, []int{2}, gen2ChannelIDs(doc, "input"))
}

func TestGen2Converter_OddChannelKeysAreUnreadAndReadNothing(t *testing.T) {
	doc := `{
		"switch:0":{"id":0,"name":"real"},
		"switch:1":{"id":1,"name":"one"},"switch:01":{"id":1,"name":"dup"},
		"switch:007":{"name":"x"},"switch:+2":{"name":"y"},
		"input:0":null,"sys":"text","mqtt":[],"cloud":{}
	}`
	keys, err := NewGen2Converter(nil).UnreadKeys(json.RawMessage(doc))
	require.NoError(t, err)
	assert.Equal(t, []string{"input:0", "mqtt", "switch:+2", "switch:007", "switch:01", "sys"}, keys)

	config, err := NewGen2Converter(nil).FromAPIConfig(json.RawMessage(doc), "SNSW-001P16EU")
	require.NoError(t, err)
	require.NotNil(t, config.Relay)
	require.Len(t, config.Relay.Relays, 2, "one relay for switch:0 and one for switch:1, none for the spellings")
	assert.Equal(t, 0, config.Relay.Relays[0].ID)
	assert.Equal(t, 1, config.Relay.Relays[1].ID)
	assert.Equal(t, "one", *config.Relay.Relays[1].Name)
}

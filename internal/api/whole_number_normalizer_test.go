package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// The normalizer reads an MQTT keep_alive and a timezone offset as whole numbers only. The two entry
// points differ on a fraction on purpose of the code that was already there: the raw one marks the
// timezone fields handled up front, so a fraction is dropped; the typed one marks a field handled only
// when it read it, so a fraction stays in the raw remainder.
func TestNormalizeRawConfig_KeepAliveAndTimezoneOffsetReadAsWholeNumbersOnly(t *testing.T) {
	n := NewConfigNormalizer()

	keepAlive := func(v interface{}) int {
		raw := map[string]interface{}{"mqtt": map[string]interface{}{"server": "broker"}}
		if v != nil {
			raw["mqtt"].(map[string]interface{})["keep_alive"] = v
		}
		return n.NormalizeRawConfig(raw).MQTT.KeepAlive
	}
	offset := func(v interface{}) (interface{}, bool) {
		raw := map[string]interface{}{}
		if v != nil {
			raw["tz_utc_offset"] = v
		}
		got := n.NormalizeRawConfig(raw)
		_, extra := got.Raw.Additional["tz_utc_offset"]
		return intPtrValue(got.Time.TZUtcOffset), extra
	}

	assert.Equal(t, 7, keepAlive(float64(7)))
	assert.Equal(t, 0, keepAlive(nil))
	whole, extra := offset(float64(19800))
	assert.Equal(t, 19800, whole)
	assert.False(t, extra)
	absent, extra := offset(nil)
	assert.Nil(t, absent)
	assert.False(t, extra)

	for _, tc := range notWhole {
		assert.Equal(t, 0, keepAlive(tc.value), "keep_alive: "+tc.name)
		got, extra := offset(tc.value)
		assert.Nil(t, got, "tz_utc_offset: "+tc.name)
		assert.False(t, extra, "tz_utc_offset: "+tc.name+" is dropped on the raw path")
	}
}

func TestNormalizeTypedConfig_TimezoneOffsetReadsAsWholeNumberOnly(t *testing.T) {
	n := NewConfigNormalizer()
	read := func(raw map[string]interface{}) (interface{}, interface{}) {
		doc, err := json.Marshal(raw)
		require.NoError(t, err)
		got := n.NormalizeTypedConfig(&configuration.TypedConfiguration{Raw: doc})
		return intPtrValue(got.Time.TZUtcOffset), got.Raw.Additional["tz_utc_offset"]
	}

	whole, extra := read(map[string]interface{}{"tz_utc_offset": float64(19800)})
	assert.Equal(t, 19800, whole)
	assert.Nil(t, extra)

	absent, extra := read(map[string]interface{}{"name": "x"})
	assert.Nil(t, absent)
	assert.Nil(t, extra)

	for _, tc := range notWhole {
		got, extra := read(map[string]interface{}{"tz_utc_offset": tc.value})
		assert.Nil(t, got, tc.name)
		assert.Equal(t, tc.value, extra, tc.name+": the value stays in the raw remainder, so a comparison sees it")
	}
}

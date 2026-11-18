package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// notWhole lists the values an auto_on, auto_off or max_power read must treat as no value: a fraction,
// and a number no int holds exactly.
var notWhole = []struct {
	name  string
	value float64
}{
	{"fractional", 7.5},
	{"fractional below one", 0.5},
	{"out of range high", 1e300},
	{"out of range low", -1e300},
	{"one past the exact integers", 9007199254740992},
}

func intPtrValue(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// An imported relay's auto_on and auto_off keep a whole number and drop a fraction, in each of the three
// layouts the import reads, so a stored 7 never stands for a device's 7.5.
func TestConvertRelayConfig_AutoOnOffReadAsWholeNumbersOnly(t *testing.T) {
	h := &Handler{}
	layouts := []struct {
		name string
		// zeroKept says whether a whole 0 is stored (true) or read as unset (false): the single relay
		// and relay_N layouts have always skipped a value of 0 or less.
		zeroKept bool
		build    func(key string, entry map[string]interface{}) map[string]interface{}
		pick     func(r *configuration.RelayConfig, key string) *int
	}{
		{
			name: "single relay object",
			build: func(_ string, entry map[string]interface{}) map[string]interface{} {
				return map[string]interface{}{"relay": entry}
			},
			pick: func(r *configuration.RelayConfig, key string) *int {
				if key == "auto_on" {
					return r.AutoOn
				}
				return r.AutoOff
			},
		},
		{
			name:     "relays array, per channel",
			zeroKept: true,
			build: func(_ string, entry map[string]interface{}) map[string]interface{} {
				return map[string]interface{}{"relays": []interface{}{entry}}
			},
			pick: func(r *configuration.RelayConfig, key string) *int {
				if len(r.Relays) != 1 {
					return nil
				}
				if key == "auto_on" {
					return r.Relays[0].AutoOn
				}
				return r.Relays[0].AutoOff
			},
		},
		{
			name:     "relays array, copy on the relay",
			zeroKept: true,
			build: func(_ string, entry map[string]interface{}) map[string]interface{} {
				return map[string]interface{}{"relays": []interface{}{entry}}
			},
			pick: func(r *configuration.RelayConfig, key string) *int {
				if key == "auto_on" {
					return r.AutoOn
				}
				return r.AutoOff
			},
		},
		{
			name: "relay_0 object",
			build: func(_ string, entry map[string]interface{}) map[string]interface{} {
				return map[string]interface{}{"relay_0": entry}
			},
			pick: func(r *configuration.RelayConfig, key string) *int {
				if len(r.Relays) != 1 {
					return nil
				}
				if key == "auto_on" {
					return r.Relays[0].AutoOn
				}
				return r.Relays[0].AutoOff
			},
		},
	}

	for _, layout := range layouts {
		for _, key := range []string{"auto_on", "auto_off"} {
			t.Run(layout.name+"/"+key, func(t *testing.T) {
				read := func(entry map[string]interface{}) interface{} {
					relay, _ := h.convertRelayConfig(layout.build(key, entry), "SHSW-1")
					return intPtrValue(layout.pick(relay, key))
				}

				assert.Equal(t, 300, read(map[string]interface{}{key: float64(300)}), "a whole number is kept")
				assert.Nil(t, read(map[string]interface{}{"name": "Lamp"}), "an absent key stays unset")
				if layout.zeroKept {
					assert.Equal(t, 0, read(map[string]interface{}{key: float64(0)}), "a whole 0 is kept")
				} else {
					assert.Nil(t, read(map[string]interface{}{key: float64(0)}), "a 0 is read as unset")
				}
				for _, tc := range notWhole {
					assert.Nil(t, read(map[string]interface{}{key: tc.value}), tc.name)
				}
			})
		}
	}
}

// A channel's max_power on an import keeps a whole number and drops a fraction or a number no int holds.
func TestConvertRelayConfig_ChannelMaxPowerReadsAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(entry map[string]interface{}) interface{} {
		relay, _ := h.convertRelayConfig(map[string]interface{}{"relays": []interface{}{entry}}, "SHSW-25")
		require.Len(t, relay.Relays, 1)
		return intPtrValue(relay.Relays[0].MaxPower)
	}

	assert.Equal(t, 1200, read(map[string]interface{}{"max_power": float64(1200)}))
	assert.Equal(t, 0, read(map[string]interface{}{"max_power": float64(0)}))
	assert.Nil(t, read(map[string]interface{}{"name": "Lamp"}))
	for _, tc := range notWhole {
		assert.Nil(t, read(map[string]interface{}{"max_power": tc.value}), tc.name)
	}
}

// An imported power-metering max_power keeps a whole number above 0 and drops a fraction or a number
// no int holds.
func TestConvertPowerMeteringConfig_MaxPowerReadsAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(data map[string]interface{}) interface{} {
		power, _ := h.convertPowerMeteringConfig(data)
		return intPtrValue(power.MaxPower)
	}

	assert.Equal(t, 2000, read(map[string]interface{}{"max_power": float64(2000)}))
	assert.Nil(t, read(map[string]interface{}{}))
	assert.Nil(t, read(map[string]interface{}{"max_power": float64(0)}))
	for _, tc := range notWhole {
		assert.Nil(t, read(map[string]interface{}{"max_power": tc.value}), tc.name)
	}
}

// A normalized relay carries auto_on and auto_off as the device holds them: a fraction is no value (0),
// never its truncation, and the fraction stays visible to a comparison.
func TestNormalizeRawConfig_RelayAutoOnOffAndMaxPowerReadAsWholeNumbersOnly(t *testing.T) {
	n := NewConfigNormalizer()

	for _, key := range []string{"auto_on", "auto_off"} {
		t.Run(key, func(t *testing.T) {
			read := func(entry map[string]interface{}) int {
				got := n.NormalizeRawConfig(map[string]interface{}{"relays": []interface{}{entry}})
				require.NotNil(t, got.Relay)
				if key == "auto_on" {
					return got.Relay.AutoOn
				}
				return got.Relay.AutoOff
			}

			assert.Equal(t, 300, read(map[string]interface{}{key: float64(300)}))
			assert.Equal(t, 0, read(map[string]interface{}{key: float64(0)}))
			assert.Equal(t, 0, read(map[string]interface{}{"ison": true}))
			for _, tc := range notWhole {
				assert.Equal(t, 0, read(map[string]interface{}{key: tc.value}), tc.name)
			}
		})
	}

	t.Run("max_power", func(t *testing.T) {
		read := func(raw map[string]interface{}) (interface{}, interface{}) {
			got := n.NormalizeRawConfig(raw)
			return intPtrValue(got.Raw.MaxPower), got.Raw.Additional["max_power"]
		}

		whole, extra := read(map[string]interface{}{"max_power": float64(3500)})
		assert.Equal(t, 3500, whole)
		assert.Nil(t, extra)

		absent, extra := read(map[string]interface{}{})
		assert.Nil(t, absent)
		assert.Nil(t, extra)

		for _, tc := range notWhole {
			got, extra := read(map[string]interface{}{"max_power": tc.value})
			assert.Nil(t, got, tc.name)
			assert.Equal(t, tc.value, extra, tc.name+": the value stays in the raw remainder, so a comparison sees it")
		}
	})
}

// The same holds for the normalized form of a stored typed configuration's raw document.
func TestNormalizeTypedConfig_RawMaxPowerReadsAsWholeNumberOnly(t *testing.T) {
	n := NewConfigNormalizer()
	read := func(raw map[string]interface{}) (interface{}, interface{}) {
		doc, err := json.Marshal(raw)
		require.NoError(t, err)
		got := n.NormalizeTypedConfig(&configuration.TypedConfiguration{Raw: doc})
		return intPtrValue(got.Raw.MaxPower), got.Raw.Additional["max_power"]
	}

	whole, extra := read(map[string]interface{}{"max_power": float64(3500)})
	assert.Equal(t, 3500, whole)
	assert.Nil(t, extra)

	absent, extra := read(map[string]interface{}{"name": "x"})
	assert.Nil(t, absent)
	assert.Nil(t, extra)

	for _, tc := range notWhole {
		got, extra := read(map[string]interface{}{"max_power": tc.value})
		assert.Nil(t, got, tc.name)
		assert.Equal(t, tc.value, extra, tc.name+": the value stays in the raw remainder, so a comparison sees it")
	}
}

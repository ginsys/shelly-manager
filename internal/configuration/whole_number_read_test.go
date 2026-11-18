package configuration

import (
	"encoding/json"
	"testing"
)

// notWholeNumbers lists the values an auto_on, auto_off or max_power read must treat as no value: a
// fraction, and a number no int holds exactly.
var notWholeNumbers = []struct {
	name  string
	value float64
}{
	{"fractional", 7.5},
	{"fractional below one", 0.5},
	{"out of range high", 1e300},
	{"out of range low", -1e300},
	{"one past the exact integers", 9007199254740992},
}

func intAt(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// A Gen1 relay keeps a whole auto_on, auto_off and max_power and reads a fraction, or a number no int
// holds, as no value, so a stored 7 never stands for a device's 7.5.
func TestGen1Converter_FromAPIConfig_RelayNumbersReadAsWholeNumbersOnly(t *testing.T) {
	converter := NewGen1Converter(nil)

	for _, key := range []string{"auto_on", "auto_off", "max_power"} {
		t.Run(key, func(t *testing.T) {
			read := func(entry map[string]interface{}) interface{} {
				config, err := converter.FromAPIConfig(mustJSON(t, map[string]interface{}{
					"relays": []interface{}{entry},
				}), "SHSW-PM")
				if err != nil {
					t.Fatalf("FromAPIConfig: %v", err)
				}
				if config.Relay == nil || len(config.Relay.Relays) != 1 {
					t.Fatalf("want one relay, got %+v", config.Relay)
				}
				channel := config.Relay.Relays[0]
				switch key {
				case "auto_on":
					return intAt(channel.AutoOn)
				case "auto_off":
					return intAt(channel.AutoOff)
				}
				return intAt(channel.MaxPower)
			}

			if got := read(map[string]interface{}{key: float64(300)}); got != 300 {
				t.Errorf("whole: got %v, want 300", got)
			}
			if got := read(map[string]interface{}{key: float64(0)}); got != 0 {
				t.Errorf("whole zero: got %v, want 0", got)
			}
			if got := read(map[string]interface{}{"name": "Lamp"}); got != nil {
				t.Errorf("absent: got %v, want unset", got)
			}
			for _, tc := range notWholeNumbers {
				if got := read(map[string]interface{}{key: tc.value}); got != nil {
					t.Errorf("%s: got %v, want unset", tc.name, got)
				}
			}
		})
	}
}

// The model-wide max_power of a Gen1 device is a whole number or unset, and the per-channel value of a
// Shelly 2.5 in relay mode is the common one only when every channel reports the same whole number.
func TestGen1Converter_FromAPIConfig_PowerMeteringMaxPowerReadsAsWholeNumberOnly(t *testing.T) {
	converter := NewGen1Converter(nil)
	read := func(deviceType string, settings map[string]interface{}) interface{} {
		config, err := converter.FromAPIConfig(mustJSON(t, settings), deviceType)
		if err != nil {
			t.Fatalf("FromAPIConfig: %v", err)
		}
		if config.PowerMetering == nil {
			return nil
		}
		return intAt(config.PowerMetering.MaxPower)
	}

	t.Run("model-wide", func(t *testing.T) {
		if got := read("SHSW-PM", map[string]interface{}{"max_power": float64(2000)}); got != 2000 {
			t.Errorf("whole: got %v, want 2000", got)
		}
		if got := read("SHSW-PM", map[string]interface{}{}); got != nil {
			t.Errorf("absent: got %v, want unset", got)
		}
		for _, tc := range notWholeNumbers {
			if got := read("SHSW-PM", map[string]interface{}{"max_power": tc.value}); got != nil {
				t.Errorf("%s: got %v, want unset", tc.name, got)
			}
		}
	})

	t.Run("common channel value", func(t *testing.T) {
		channels := func(first, second interface{}) map[string]interface{} {
			return map[string]interface{}{
				"mode": "relay",
				"relays": []interface{}{
					map[string]interface{}{"max_power": first},
					map[string]interface{}{"max_power": second},
				},
			}
		}

		if got := read("SHSW-25", channels(float64(1500), float64(1500))); got != 1500 {
			t.Errorf("channels agree on a whole number: got %v, want 1500", got)
		}
		if got := read("SHSW-25", channels(float64(1500), float64(1600))); got != nil {
			t.Errorf("channels differ: got %v, want unset", got)
		}
		for _, tc := range notWholeNumbers {
			if got := read("SHSW-25", channels(tc.value, tc.value)); got != nil {
				t.Errorf("%s on both channels: got %v, want unset", tc.name, got)
			}
		}
		// 7.5 and 7 truncate to the same int, but they are not the same value.
		if got := read("SHSW-25", channels(7.5, float64(7))); got != nil {
			t.Errorf("a fraction beside its truncation: got %v, want unset", got)
		}
	})
}

// A raw relay document converts to the typed form with whole numbers kept and fractions, or numbers no
// int holds, left unset, on the relay and on each channel.
func TestConvertRawToTyped_RelayNumbersReadAsWholeNumbersOnly(t *testing.T) {
	service := createTestService(t)

	for _, key := range []string{"auto_on", "auto_off", "max_power"} {
		t.Run(key, func(t *testing.T) {
			// On the relay, and for auto_on and auto_off on the channel as well. Both skip a value of 0 or less.
			read := func(entry map[string]interface{}) (onRelay, onChannel interface{}) {
				typed, _, err := service.ConvertRawToTyped(mustJSON(t, map[string]interface{}{
					"relays": []interface{}{entry},
				}))
				if err != nil {
					t.Fatalf("ConvertRawToTyped: %v", err)
				}
				if typed.Relay == nil {
					t.Fatal("relay section is nil")
				}
				var channel *SingleRelayConfig
				if len(typed.Relay.Relays) == 1 {
					channel = &typed.Relay.Relays[0]
				}
				switch key {
				case "auto_on":
					onRelay = intAt(typed.Relay.AutoOn)
					if channel != nil {
						onChannel = intAt(channel.AutoOn)
					}
				case "auto_off":
					onRelay = intAt(typed.Relay.AutoOff)
					if channel != nil {
						onChannel = intAt(channel.AutoOff)
					}
				default:
					onRelay = intAt(typed.Relay.MaxPowerLimit)
				}
				return onRelay, onChannel
			}

			relay, channel := read(map[string]interface{}{key: float64(300)})
			if relay != 300 {
				t.Errorf("whole on the relay: got %v, want 300", relay)
			}
			if key != "max_power" && channel != 300 {
				t.Errorf("whole on the channel: got %v, want 300", channel)
			}

			relay, channel = read(map[string]interface{}{"name": "Lamp"})
			if relay != nil || channel != nil {
				t.Errorf("absent: got %v and %v, want unset", relay, channel)
			}

			relay, channel = read(map[string]interface{}{key: float64(0)})
			if relay != nil || channel != nil {
				t.Errorf("zero: got %v and %v, want unset (a value of 0 or less is skipped)", relay, channel)
			}

			for _, tc := range notWholeNumbers {
				relay, channel = read(map[string]interface{}{key: tc.value})
				if relay != nil || channel != nil {
					t.Errorf("%s: got %v and %v, want unset", tc.name, relay, channel)
				}
			}
		})
	}
}

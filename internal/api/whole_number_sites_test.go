package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// siteCase is one configuration field read from a device document. build puts v (nil: leave the key
// out) into the document the converter takes, read returns the field it fills, none is the result of
// an absent key and of a value that is not a whole number, and whole is the result of 7.
type siteCase struct {
	name  string
	build func(v interface{}) map[string]interface{}
	read  func(map[string]interface{}) interface{}
	whole interface{}
	none  interface{}
}

func flat(key string) func(v interface{}) map[string]interface{} {
	return func(v interface{}) map[string]interface{} {
		if v == nil {
			return map[string]interface{}{}
		}
		return map[string]interface{}{key: v}
	}
}

func nested(outer, inner string) func(v interface{}) map[string]interface{} {
	return func(v interface{}) map[string]interface{} {
		if v == nil {
			return map[string]interface{}{}
		}
		return map[string]interface{}{outer: map[string]interface{}{inner: v}}
	}
}

// runSites checks every site over the four value shapes: whole, absent, a fraction, and a number no
// int holds exactly. A fraction used to be cut to its whole part.
func runSites(t *testing.T, cases []siteCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.whole, c.read(c.build(float64(7))), "a whole number is read as it is")
			assert.Equal(t, c.none, c.read(c.build(nil)), "an absent key is no value")
			for _, tc := range notWhole {
				assert.Equal(t, c.none, c.read(c.build(tc.value)), tc.name)
			}
		})
	}
}

func TestConvertMQTTConfig_NumbersReadAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(pick func(*configuration.MQTTConfiguration) *int) func(map[string]interface{}) interface{} {
		return func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertMQTTConfig(d)
			return intPtrValue(pick(cfg))
		}
	}
	runSites(t, []siteCase{
		{"port", flat("port"), read(func(c *configuration.MQTTConfiguration) *int { return c.Port }), 7, nil},
		{"keep_alive", flat("keep_alive"), read(func(c *configuration.MQTTConfiguration) *int { return c.KeepAlive }), 7, nil},
	})
}

func TestConvertCoIoTConfig_NumbersReadAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(pick func(*configuration.CoIoTConfig) int) func(map[string]interface{}) interface{} {
		return func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertCoIoTConfig(d)
			return pick(cfg)
		}
	}
	runSites(t, []siteCase{
		{"port", flat("port"), read(func(c *configuration.CoIoTConfig) int { return c.Port }), 7, 0},
		{"period", flat("period"), read(func(c *configuration.CoIoTConfig) int { return c.Period }), 7, 0},
	})
}

func TestConvertPowerMeteringConfig_NumbersReadAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(pick func(*configuration.PowerMeteringConfig) *int) func(map[string]interface{}) interface{} {
		return func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertPowerMeteringConfig(d)
			return intPtrValue(pick(cfg))
		}
	}
	runSites(t, []siteCase{
		{"max_voltage", flat("max_voltage"), read(func(c *configuration.PowerMeteringConfig) *int { return c.MaxVoltage }), 7, nil},
		{"reporting_period", flat("reporting_period"), read(func(c *configuration.PowerMeteringConfig) *int { return c.ReportingPeriod }), 7, nil},
	})

	// max_voltage keeps its guard: a whole zero is no value.
	cfg, _ := h.convertPowerMeteringConfig(map[string]interface{}{"max_voltage": float64(0)})
	assert.Nil(t, cfg.MaxVoltage)
}

// The three brightness limits keep their defaults when the device value is not a whole number.
func TestConvertDimmingConfig_NumbersReadAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(pick func(*configuration.DimmingConfig) *int) func(map[string]interface{}) interface{} {
		return func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertDimmingConfig(d)
			return intPtrValue(pick(cfg))
		}
	}
	runSites(t, []siteCase{
		{"min_brightness", flat("min_brightness"), read(func(c *configuration.DimmingConfig) *int { return c.MinBrightness }), 7, 1},
		{"max_brightness", flat("max_brightness"), read(func(c *configuration.DimmingConfig) *int { return c.MaxBrightness }), 7, 100},
		{"default_brightness", flat("default_brightness"), read(func(c *configuration.DimmingConfig) *int { return c.DefaultBrightness }), 7, 100},
		{"fade_rate", flat("fade_rate"), read(func(c *configuration.DimmingConfig) *int { return c.FadeRate }), 7, nil},
		{"transition", flat("transition"), read(func(c *configuration.DimmingConfig) *int { return c.TransitionTime }), 7, nil},
		{"night_mode_brightness", flat("night_mode_brightness"), read(func(c *configuration.DimmingConfig) *int { return c.NightModeBrightness }), 7, nil},
	})
}

func TestConvertRollerConfig_NumbersReadAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(pick func(*configuration.RollerConfig) *int) func(map[string]interface{}) interface{} {
		return func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertRollerConfig(d)
			return intPtrValue(pick(cfg))
		}
	}
	runSites(t, []siteCase{
		{"max_open_time", flat("max_open_time"), read(func(c *configuration.RollerConfig) *int { return c.MaxOpenTime }), 7, nil},
		{"max_close_time", flat("max_close_time"), read(func(c *configuration.RollerConfig) *int { return c.MaxCloseTime }), 7, nil},
		{"default_position", flat("default_position"), read(func(c *configuration.RollerConfig) *int { return c.DefaultPosition }), 7, nil},
		{"current_position", flat("current_position"), read(func(c *configuration.RollerConfig) *int { return c.CurrentPosition }), 7, nil},
		{"obstacle_power", flat("obstacle_power"), read(func(c *configuration.RollerConfig) *int { return c.ObstaclePower }), 7, nil},
		{"button_hold_time", flat("button_hold_time"), read(func(c *configuration.RollerConfig) *int { return c.ButtonHoldTime }), 7, nil},
	})

	// obstacle_power keeps its guard: a whole zero is no value.
	cfg, _ := h.convertRollerConfig(map[string]interface{}{"obstacle_power": float64(0)})
	assert.Nil(t, cfg.ObstaclePower)
}

func TestConvertInputConfig_NumbersReadAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	read := func(pick func(*configuration.InputConfig) *int) func(map[string]interface{}) interface{} {
		return func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertInputConfig(d)
			return intPtrValue(pick(cfg))
		}
	}
	// The multi-input layout reads the global timings from the nested durations.
	withInputs := func(inner func(v interface{}) map[string]interface{}) func(v interface{}) map[string]interface{} {
		return func(v interface{}) map[string]interface{} {
			d := inner(v)
			d["inputs"] = []interface{}{map[string]interface{}{"name": "in"}}
			return d
		}
	}
	runSites(t, []siteCase{
		{"debounce_time", flat("debounce_time"), read(func(c *configuration.InputConfig) *int { return c.DebounceTime }), 7, nil},
		{"longpush_time", flat("longpush_time"), read(func(c *configuration.InputConfig) *int { return c.LongPushTime }), 7, nil},
		{"multipush_time", flat("multipush_time"), read(func(c *configuration.InputConfig) *int { return c.MultiPushTime }), 7, nil},
		{"longpush_duration_ms.min", withInputs(nested("longpush_duration_ms", "min")), read(func(c *configuration.InputConfig) *int { return c.LongPushTime }), 7, nil},
		{"multipush_time_between_pushes_ms.max", withInputs(nested("multipush_time_between_pushes_ms", "max")), read(func(c *configuration.InputConfig) *int { return c.MultiPushTime }), 7, nil},
	})
}

// A channel is marked "long" only when its long-push duration is a number the device setting can
// carry: a fraction gives neither the action nor the time.
func TestConvertInputConfig_LongPushActionFollowsAWholeDuration(t *testing.T) {
	h := &Handler{}
	action := func(v interface{}) interface{} {
		cfg, _ := h.convertInputConfig(map[string]interface{}{
			"inputs":               []interface{}{map[string]interface{}{"name": "in"}},
			"longpush_duration_ms": map[string]interface{}{"min": v},
		})
		require.Len(t, cfg.Inputs, 1)
		if cfg.Inputs[0].LongPushAction == nil {
			return nil
		}
		return *cfg.Inputs[0].LongPushAction
	}

	assert.Equal(t, "long", action(float64(800)))
	for _, tc := range notWhole {
		assert.Nil(t, action(tc.value), tc.name)
	}
}

func TestConvertLEDConfig_BrightnessReadsAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	runSites(t, []siteCase{
		{"led_brightness", flat("led_brightness"), func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertLEDConfig(d)
			return intPtrValue(cfg.Brightness)
		}, 7, nil},
	})
}

func TestConvertColorConfig_NumbersReadAsWholeNumberOnly(t *testing.T) {
	h := &Handler{}
	runSites(t, []siteCase{
		{"active_effect", flat("active_effect"), func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertColorConfig(d)
			return intPtrValue(cfg.ActiveEffect)
		}, 7, nil},
		{"effect_speed", flat("effect_speed"), func(d map[string]interface{}) interface{} {
			cfg, _ := h.convertColorConfig(d)
			return cfg.EffectSpeed
		}, 7, 0},
	})

	// The default colour needs all three channels to be whole numbers; one that is not drops it.
	colour := func(r, g, b interface{}) *configuration.Color {
		cfg, _ := h.convertColorConfig(map[string]interface{}{
			"default_color": map[string]interface{}{"r": r, "g": g, "b": b},
		})
		return cfg.DefaultColor
	}
	assert.Equal(t, &configuration.Color{Red: 1, Green: 2, Blue: 3}, colour(float64(1), float64(2), float64(3)))
	for _, tc := range notWhole {
		assert.Nil(t, colour(tc.value, float64(2), float64(3)), "red: "+tc.name)
		assert.Nil(t, colour(float64(1), tc.value, float64(3)), "green: "+tc.name)
		assert.Nil(t, colour(float64(1), float64(2), tc.value), "blue: "+tc.name)
	}
}

// A device's "gen" is read the same way for the capabilities route and for the typed import: a
// fraction is no value, so the generation comes from the firmware as it does when "gen" is missing.
func TestGenerationFromSettings_ReadsAWholeNumberOnly(t *testing.T) {
	assert.Equal(t, 2, generationFromSettings(map[string]interface{}{"gen": float64(2)}, 1))
	assert.Equal(t, 1, generationFromSettings(map[string]interface{}{}, 1), "an absent key keeps the fallback")
	assert.Equal(t, 1, generationFromSettings(nil, 1), "no settings keep the fallback")
	for _, tc := range notWhole {
		assert.Equal(t, 1, generationFromSettings(map[string]interface{}{"gen": tc.value}, 1), tc.name)
	}
}

func TestGetDeviceCapabilities_GenerationReadsAWholeNumberOnly(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()

	next := 0
	caps := func(gen interface{}) []string {
		next++
		device := testutil.TestDevice()
		device.IP = fmt.Sprintf("192.0.2.%d", next)
		device.MAC = fmt.Sprintf("AABBCCDDEE%02X", next)
		device.Firmware = "v1.14.0"
		settings, err := json.Marshal(map[string]interface{}{"model": "SHSW-1", "gen": gen})
		require.NoError(t, err)
		device.Settings = string(settings)
		require.NoError(t, h.DB.AddDevice(device))

		w := serve(h.GetDeviceCapabilities, "GET", "/api/v1/devices/1/capabilities", idString(device), "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			Capabilities []string `json:"capabilities"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return body.Capabilities
	}

	assert.Contains(t, caps(float64(2)), "ble", "a whole gen of 2 wins over the firmware")
	assert.NotContains(t, caps(2.5), "ble", "a fractional gen is no value: the firmware says Gen1")
}

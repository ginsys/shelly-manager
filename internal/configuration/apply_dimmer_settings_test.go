package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

func dimmingConfig(d DimmingConfig) *DeviceConfiguration { return &DeviceConfiguration{Dimming: &d} }

// A Dimmer's device-level settings are flat parameters of POST /settings, read back from GET
// /settings: the service writes them in one form and the verify reads them equal.
func TestApplyDimmerSettings_FadeRateIsWrittenAndReadBack(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{
		FadeRate: IntPtr(3), TransitionTime: IntPtr(200), MinBrightness: IntPtr(10), ZcrossDebounce: IntPtr(100),
	}))
	dev := newDimmerDevice()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.posted, 1)
	assert.Equal(t, map[string]interface{}{
		"fade_rate": float64(3), "transition": float64(200), "min_brightness": float64(10), "zcross_debounce": float64(100),
	}, dev.posted[0])
	assert.Empty(t, dev.lightWrites)
	assert.Empty(t, dev.relayWrites)
}

// Each of the four numbers on its own is written and read back, and only the ones set are sent.
func TestApplyDimmerSettings_EachNumberAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		d    DimmingConfig
		key  string
		want float64
	}{
		{"fade_rate", DimmingConfig{FadeRate: IntPtr(5)}, "fade_rate", 5},
		{"transition", DimmingConfig{TransitionTime: IntPtr(0)}, "transition", 0},
		{"min_brightness", DimmingConfig{MinBrightness: IntPtr(50)}, "min_brightness", 50},
		{"zcross_debounce", DimmingConfig{ZcrossDebounce: IntPtr(50)}, "zcross_debounce", 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "dimmer")
			setDesired(t, repo, dimmingConfig(tc.d))
			dev := newDimmerDevice()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-2")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
			require.Len(t, dev.posted, 1)
			assert.Equal(t, map[string]interface{}{tc.key: tc.want}, dev.posted[0])
		})
	}
}

// A value outside the page's ranges is refused before any write, and the range edges are accepted.
func TestUnsupportedDimmingValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		d    DimmingConfig
		want []string
	}{
		{"transition at 0", DimmingConfig{TransitionTime: IntPtr(0)}, nil},
		{"transition at 5000", DimmingConfig{TransitionTime: IntPtr(5000)}, nil},
		{"transition above", DimmingConfig{TransitionTime: IntPtr(5001)}, []string{"dimming.transition"}},
		{"transition below", DimmingConfig{TransitionTime: IntPtr(-1)}, []string{"dimming.transition"}},
		{"fade_rate at 1", DimmingConfig{FadeRate: IntPtr(1)}, nil},
		{"fade_rate at 5", DimmingConfig{FadeRate: IntPtr(5)}, nil},
		{"fade_rate zero (the page's example value)", DimmingConfig{FadeRate: IntPtr(0)}, []string{"dimming.fade_rate"}},
		{"fade_rate above", DimmingConfig{FadeRate: IntPtr(6)}, []string{"dimming.fade_rate"}},
		{"min_brightness at 0", DimmingConfig{MinBrightness: IntPtr(0)}, nil},
		{"min_brightness at 50", DimmingConfig{MinBrightness: IntPtr(50)}, nil},
		{"min_brightness above", DimmingConfig{MinBrightness: IntPtr(51)}, []string{"dimming.min_brightness"}},
		{"min_brightness below", DimmingConfig{MinBrightness: IntPtr(-1)}, []string{"dimming.min_brightness"}},
		{"zcross_debounce at 50", DimmingConfig{ZcrossDebounce: IntPtr(50)}, nil},
		{"zcross_debounce at 150", DimmingConfig{ZcrossDebounce: IntPtr(150)}, nil},
		{"zcross_debounce below", DimmingConfig{ZcrossDebounce: IntPtr(49)}, []string{"dimming.zcross_debounce"}},
		{"zcross_debounce above", DimmingConfig{ZcrossDebounce: IntPtr(151)}, []string{"dimming.zcross_debounce"}},
		{"unset", DimmingConfig{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, unsupportedDimmingValues(&tc.d))
		})
	}
}

func TestApplyDimmerSettings_OutOfRangeIsRefusedBeforeAnyWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{FadeRate: IntPtr(3), TransitionTime: IntPtr(9000)}))
	dev := newDimmerDevice()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "dimming.transition")
	assert.NotContains(t, err.Error(), "dimming.fade_rate", "an in-range value is not refused")
	assert.Equal(t, 0, dev.setCalls, "nothing may be written")
}

// pulse_mode is written (1 leading edge, 2 trailing edge) only when the device reports that it has
// detected the pulse mode. Anything else is refused before any write: the page does not say what an
// absent or false flag permits, so the write is not left to the device.
func TestApplyDimmerSettings_PulseModeIsWrittenWhenTheDeviceReportsDetection(t *testing.T) {
	for _, tc := range []struct {
		name    string
		leading bool
		want    float64
	}{
		{"leading edge", true, 1},
		{"trailing edge", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "dimmer")
			setDesired(t, repo, dimmingConfig(DimmingConfig{LeadingEdge: BoolPtr(tc.leading)}))
			dev := newDimmerDevice()
			dev.settings["pulse_mode_detected"] = true

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-2")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
			require.Len(t, dev.posted, 1)
			assert.Equal(t, map[string]interface{}{"pulse_mode": tc.want}, dev.posted[0])
			assert.False(t, result.ApplyResult.RequiresReboot, "the reference names no reboot after a pulse_mode change")
		})
	}
}

func TestApplyDimmerSettings_PulseModeIsRefusedWithoutDetection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		model    string
		settings map[string]interface{}
	}{
		{"flag false", "SHDM-2", map[string]interface{}{"pulse_mode_detected": false, "pulse_mode": float64(0)}},
		{"flag absent, desired value equals the live one (a Dimmer 1 never reports it)", "SHDM-1", map[string]interface{}{"pulse_mode": float64(1)}},
		{"flag absent on a Dimmer 2", "SHDM-2", map[string]interface{}{}},
		{"desired value equals the live one, flag false", "SHDM-2", map[string]interface{}{"pulse_mode_detected": false, "pulse_mode": float64(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dev := newDimmerDevice()
			for k, v := range tc.settings {
				dev.settings[k] = v
			}
			// A change to another setting rides the same POST, and is refused with it.
			config := dimmingConfig(DimmingConfig{LeadingEdge: BoolPtr(true), FadeRate: IntPtr(2)})

			result, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), dev, config, tc.model)

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "pulse_mode")
			assert.Equal(t, 0, dev.setCalls, "nothing may be written, the fade rate included")
		})
	}
}

// pulse_mode_detected is only one value away from "yes": a device that cannot be read cannot say.
type unreadableDimmer struct{ *dimmerDevice }

func (unreadableDimmer) GetConfig(context.Context) (*shelly.DeviceConfig, error) {
	return nil, errors.New("connection refused")
}

func TestApplyDimmerSettings_PulseModeIsRefusedWhenTheDeviceCannotBeRead(t *testing.T) {
	dev := unreadableDimmer{newDimmerDevice()}

	result, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), dev,
		dimmingConfig(DimmingConfig{LeadingEdge: BoolPtr(false)}), "SHDM-2")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, dev.setCalls)
}

// The converter reads pulse_mode 1 and 2 as the stored edge, and any other value (the page's example
// shows 0 before the mode is detected) as no value at all.
// The dimming section belongs to the Dimmer and not to every light model: the RGBW2 will have light
// channels and none of these parameters. No such model is supported yet, so this pins the predicate
// directly and the converter on a model name that is a light model without being a Dimmer: a
// desired dimming key is not exported, so the round trip refuses it before any write.
func TestGen1Converter_DimmingIsForTheDimmerNotForEveryLightModel(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, model := range []string{"SHDM-1", "SHDM-2"} {
		assert.True(t, converter.hasDimmerSettings(model), model)
		assert.True(t, converter.hasLight(model), "a Dimmer is a light model: "+model)
	}
	for _, model := range []string{"SHRGBW2", "SHBLB-1", "SHCB-1", "SHPLG-S", "SHSW-1", ""} {
		assert.False(t, converter.hasDimmerSettings(model), model)
	}

	apiJSON, err := converter.ToAPIConfig(&DeviceConfiguration{Dimming: &DimmingConfig{
		FadeRate: IntPtr(3), TransitionTime: IntPtr(200), MinBrightness: IntPtr(10),
		ZcrossDebounce: IntPtr(100), LeadingEdge: BoolPtr(true),
	}}, "SHRGBW2")
	require.NoError(t, err)
	var gen1 map[string]interface{}
	require.NoError(t, json.Unmarshal(apiJSON, &gen1))
	for _, key := range append([]string{}, gen1DimmerKeys...) {
		assert.NotContains(t, gen1, key, "a model that is not a Dimmer is sent no dimming key")
	}

	got, err := converter.FromAPIConfig([]byte(`{"fade_rate":3,"transition":200,"min_brightness":10,"zcross_debounce":100,"pulse_mode":1}`), "SHRGBW2")
	require.NoError(t, err)
	assert.Nil(t, got.Dimming, "a model that is not a Dimmer reads no dimming section")
}

func TestGen1Converter_PulseModeRoundTrip(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, tc := range []struct {
		raw  string
		want *bool
	}{
		{`{"pulse_mode":1}`, BoolPtr(true)},
		{`{"pulse_mode":2}`, BoolPtr(false)},
		{`{"pulse_mode":0}`, nil},
		{`{}`, nil},
	} {
		got, err := converter.FromAPIConfig([]byte(tc.raw), "SHDM-2")
		require.NoError(t, err)
		if tc.want == nil {
			assert.True(t, got.Dimming == nil || got.Dimming.LeadingEdge == nil, tc.raw)
			continue
		}
		require.NotNil(t, got.Dimming, tc.raw)
		assert.Equal(t, tc.want, got.Dimming.LeadingEdge, tc.raw)
	}
}

// calibrate and calibrate_cancel are actions, not settings: no desired field carries them, so no POST
// ever does.
func TestApplyDimmerSettings_ActionsAreNeverWritten(t *testing.T) {
	dev := newDimmerDevice()
	dev.settings["pulse_mode_detected"] = true
	config := dimmingConfig(DimmingConfig{
		FadeRate: IntPtr(2), MinBrightness: IntPtr(5), TransitionTime: IntPtr(100), ZcrossDebounce: IntPtr(90), LeadingEdge: BoolPtr(true),
	})

	_, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), dev, config, "SHDM-2")

	require.NoError(t, err)
	for _, form := range dev.posted {
		for _, key := range []string{"calibrate", "calibrate_cancel", "pulse_mode_detected", "bypass", "load_autodetect"} {
			assert.NotContains(t, form, key)
		}
	}
}

// The status LED is a Dimmer key too (led_status_disable, stored inverted as network_indication). The
// power LED is a Plug key: the Dimmer's page has none, so a desired one is refused.
func TestApplyDimmerSettings_StatusLEDIsWrittenAndPowerLEDRefused(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, &DeviceConfiguration{LED: &LEDConfig{NetworkIndication: BoolPtr(false)}})
	dev := newDimmerDevice()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.posted, 1)
	assert.Equal(t, map[string]interface{}{"led_status_disable": true}, dev.posted[0])

	setDesired(t, repo, &DeviceConfiguration{LED: &LEDConfig{PowerIndication: BoolPtr(false)}})
	dev = newDimmerDevice()
	result, err = svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")
	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "led", "the section that came back empty is named")
	assert.Equal(t, 0, dev.setCalls)
}

// A dimming field with no /settings parameter is refused before any write, never dropped in silence:
// the converter exports none of them, so the round trip reports it.
func TestApplyDimmerSettings_FieldsWithoutAParameterAreRefused(t *testing.T) {
	for name, d := range map[string]DimmingConfig{
		"max_brightness":     {MaxBrightness: IntPtr(80)},
		"default_brightness": {DefaultBrightness: IntPtr(50)},
		"default_state":      {DefaultState: BoolPtr(true)},
		"min_dim_level":      {MinDimLevel: IntPtr(5)},
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "dimmer")
			setDesired(t, repo, dimmingConfig(d))
			dev := newDimmerDevice()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Nil(t, result)
			assert.Equal(t, 0, dev.setCalls)
		})
	}
}

// A model that is not a Dimmer has no such keys: a desired dimming section is refused there, and
// the plugs keep both of their LED keys.
func TestApplyDimmerSettings_OtherModelsKeepTheirResults(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "plug")
	setDesired(t, repo, dimmingConfig(DimmingConfig{FadeRate: IntPtr(3)}))
	dev := newFakeGen1Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "dimming", "the section the model does not have is named")
	assert.Equal(t, 0, dev.setCalls)

	setDesired(t, repo, &DeviceConfiguration{LED: &LEDConfig{PowerIndication: BoolPtr(false), NetworkIndication: BoolPtr(false)}})
	dev = newFakeGen1Device()
	result, err = svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.posted, 1)
	assert.Equal(t, map[string]interface{}{"led_power_disable": true, "led_status_disable": true}, dev.posted[0])
}

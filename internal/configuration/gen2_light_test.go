package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGen2DeviceWithLight is a dimmer: light:0 holds the keys that the Shelly Light page's GetConfig
// example shows. The page says warmup, button_fade_rate and the limits are present only on some
// devices ("if applicable"), so a test of the live read removes a key to model a device that lacks it.
func fakeGen2DeviceWithLight() *fakeGen2Device {
	d := newFakeGen2Device()
	d.doc["light:0"] = map[string]interface{}{
		"id": 0.0, "name": nil, "initial_state": "restore_last",
		"auto_on": false, "auto_on_delay": 60.0, "auto_off": false, "auto_off_delay": 60.0,
		"transition_duration": 3.0, "min_brightness_on_toggle": 3.0, "button_fade_rate": 3.0,
		"button_presets": map[string]interface{}{"button_doublepush": map[string]interface{}{"brightness": 100.0}},
		"night_mode":     map[string]interface{}{"enable": false, "brightness": 50.0, "active_between": []interface{}{"22:00", "06:00"}},
		"warmup":         map[string]interface{}{"enable": false, "brightness": 10.0, "time_ms": 100.0},
	}
	return d
}

func lightDesired() *DeviceConfiguration {
	return &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{
		{ID: 0, Name: StringPtr("hall"), DefaultState: StringPtr("last"), AutoOn: IntPtr(30), AutoOff: IntPtr(0)},
	}}}
}

// A desired light channel on a dimmer that reports light:0 is written with one Light.SetConfig that
// carries only the keys the configuration sets, then read back. Before this slice the whole light
// section was refused, because the Gen2 converter had no mapping for it.
func TestApplyDesiredConfig_Gen2LightWritesChannel(t *testing.T) {
	svc, repo := newGen2Service(t, lightDesired())
	dev := fakeGen2DeviceWithLight()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "the light is read back as the stored light")
	assert.True(t, repo.devices[1].ConfigApplied)
	assert.Equal(t, 0, dev.legacyCalls)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "Light.SetConfig", dev.calls[0].method)
	assert.Equal(t, 0, dev.calls[0].id)
	assert.Equal(t, map[string]interface{}{
		"name": "hall", "initial_state": "restore_last", "auto_on": true, "auto_on_delay": 30.0, "auto_off": false,
	}, dev.calls[0].config)
	assert.Equal(t, 0, result.ApplyResult.FailedCount)
	assert.False(t, result.ApplyResult.RequiresReboot, "the page shows no restart_required")
}

func lightBody(t *testing.T, light SingleLightConfig) map[string]interface{} {
	t.Helper()
	raw, err := NewGen2Converter(nil).ToAPIConfig(&DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{light}}}, "SNDM-00100WW")
	require.NoError(t, err)
	var doc map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc["light:0"]
}

// The nested objects carry the sub-keys that are set and nothing else, as the cover does.
func TestGen2Converter_LightNestedObjectsCarryOnlyTheKeysSet(t *testing.T) {
	cases := map[string]struct {
		light SingleLightConfig
		want  map[string]interface{}
	}{
		"night enable alone": {SingleLightConfig{NightMode: BoolPtr(true)}, map[string]interface{}{"night_mode": map[string]interface{}{"enable": true}}},
		"night span alone": {SingleLightConfig{NightModeStart: StringPtr("22:00"), NightModeEnd: StringPtr("06:30")},
			map[string]interface{}{"night_mode": map[string]interface{}{"active_between": []interface{}{"22:00", "06:30"}}}},
		"night brightness alone": {SingleLightConfig{NightModeBrightness: IntPtr(20)}, map[string]interface{}{"night_mode": map[string]interface{}{"brightness": 20.0}}},
		"warmup time alone":      {SingleLightConfig{WarmupTime: IntPtr(200)}, map[string]interface{}{"warmup": map[string]interface{}{"time_ms": 200.0}}},
		"warmup all": {SingleLightConfig{WarmupEnabled: BoolPtr(true), WarmupBrightness: IntPtr(50), WarmupTime: IntPtr(100)},
			map[string]interface{}{"warmup": map[string]interface{}{"enable": true, "brightness": 50.0, "time_ms": 100.0}}},
		"flat keys": {SingleLightConfig{TransitionDuration: Float64Ptr(1.5), MinBrightnessOnToggle: IntPtr(5), ButtonFadeRate: IntPtr(2)},
			map[string]interface{}{"transition_duration": 1.5, "min_brightness_on_toggle": 5.0, "button_fade_rate": 2.0}},
		"a lone end of the span is not written": {SingleLightConfig{NightModeEnd: StringPtr("06:30")}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, lightBody(t, tc.light))
		})
	}
}

func TestGen2Converter_ReadsTheLight(t *testing.T) {
	device := fakeGen2DeviceWithLight()
	light := device.doc["light:0"].(map[string]interface{})
	light["name"], light["auto_on"], light["auto_on_delay"] = "hall", true, 45.0
	light["warmup"] = map[string]interface{}{"enable": true, "brightness": 40.0, "time_ms": 300.0}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNDM-00100WW")

	require.NoError(t, err)
	require.NotNil(t, got.Light)
	require.Len(t, got.Light.Lights, 1)
	l := got.Light.Lights[0]
	assert.Equal(t, "hall", *l.Name)
	assert.Equal(t, "restore_last", *l.DefaultState)
	assert.Equal(t, 45, *l.AutoOn)
	assert.Equal(t, 0, *l.AutoOff, "auto_off false is 0, whatever delay the device keeps")
	assert.Equal(t, 3.0, *l.TransitionDuration)
	assert.Equal(t, 3, *l.MinBrightnessOnToggle)
	assert.Equal(t, 3, *l.ButtonFadeRate)
	assert.False(t, *l.NightMode)
	assert.Equal(t, 50, *l.NightModeBrightness)
	assert.Equal(t, "22:00", *l.NightModeStart)
	assert.Equal(t, "06:00", *l.NightModeEnd)
	assert.True(t, *l.WarmupEnabled)
	assert.Equal(t, 40, *l.WarmupBrightness)
	assert.Equal(t, 300, *l.WarmupTime)
}

// What the device reports that has no stored value is left unread, never guessed.
func TestGen2Converter_LightReadBackNeverInventsAValue(t *testing.T) {
	device := fakeGen2DeviceWithLight()
	light := device.doc["light:0"].(map[string]interface{})
	light["min_brightness_on_toggle"] = 2.5
	light["night_mode"] = map[string]interface{}{"enable": true, "active_between": []interface{}{"22:00"}}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNDM-00100WW")

	require.NoError(t, err)
	l := got.Light.Lights[0]
	assert.Nil(t, l.MinBrightnessOnToggle, "a fraction is never truncated")
	assert.Nil(t, l.NightModeStart, "a span of one end names no start")
	assert.Nil(t, l.NightModeEnd)
	assert.True(t, *l.NightMode)
}

// Only the keys a desired light sets are sent, and the nested objects send only their sub-keys.
func TestApplyDesiredConfig_Gen2LightSendsOnlyTheKeysSet(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{
		{ID: 0, NightModeBrightness: IntPtr(30), WarmupTime: IntPtr(250)},
	}}})
	dev := fakeGen2DeviceWithLight()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{
		"night_mode": map[string]interface{}{"brightness": 30.0},
		"warmup":     map[string]interface{}{"time_ms": 250.0},
	}, dev.calls[0].config)
	assert.Equal(t, 2, result.ApplyResult.AppliedCount, "one per leaf")
}

// The Gen2 apply sends no profile change: a desired light on a device with no light:0 is refused
// before any write.
func TestApplyDesiredConfig_Gen2RefusesALightOnADeviceWithoutOne(t *testing.T) {
	svc, repo := newGen2Service(t, lightDesired())
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "light:0")
	assert.Empty(t, dev.calls)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// A key that the live light:0 does not report, at any depth, is refused before any write; a key that
// is present with null is reported.
func TestApplyDesiredConfig_Gen2RefusesAKeyTheDeviceDoesNotReport(t *testing.T) {
	cases := map[string]struct {
		remove func(light map[string]interface{})
		want   string
	}{
		"warmup object":   {func(l map[string]interface{}) { delete(l, "warmup") }, "light:0.warmup.time_ms"},
		"warmup sub-key":  {func(l map[string]interface{}) { delete(l["warmup"].(map[string]interface{}), "time_ms") }, "light:0.warmup.time_ms"},
		"button fade":     {func(l map[string]interface{}) { delete(l, "button_fade_rate") }, "light:0.button_fade_rate"},
		"night span":      {func(l map[string]interface{}) { delete(l["night_mode"].(map[string]interface{}), "active_between") }, "light:0.night_mode.active_between"},
		"transition time": {func(l map[string]interface{}) { delete(l, "transition_duration") }, "light:0.transition_duration"},
	}
	desired := &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{{
		ID: 0, WarmupTime: IntPtr(200), ButtonFadeRate: IntPtr(2), TransitionDuration: Float64Ptr(1),
		NightModeStart: StringPtr("21:00"), NightModeEnd: StringPtr("07:00"),
	}}}}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, desired)
			dev := fakeGen2DeviceWithLight()
			tc.remove(dev.doc["light:0"].(map[string]interface{}))

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.want)
			assert.Empty(t, dev.calls)
		})
	}

	t.Run("null is reported", func(t *testing.T) {
		svc, _ := newGen2Service(t, desired)
		dev := fakeGen2DeviceWithLight()
		dev.doc["light:0"].(map[string]interface{})["button_fade_rate"] = nil

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

		require.NoError(t, err)
		assert.True(t, result.ConfigApplied)
	})
}

// A word outside the page's lists, a value outside its ranges and a key with no Light.SetConfig
// counterpart are refused before the device is touched.
func TestApplyDesiredConfig_Gen2RefusesLightValuesOutOfRange(t *testing.T) {
	cases := map[string]struct {
		light SingleLightConfig
		paths []string
	}{
		"switch state":          {SingleLightConfig{DefaultState: StringPtr("switch")}, []string{"light.lights.0.default_state"}},
		"unknown state":         {SingleLightConfig{DefaultState: StringPtr("dim")}, []string{"light.lights.0.default_state"}},
		"negative timer":        {SingleLightConfig{AutoOn: IntPtr(-1)}, []string{"light.lights.0.auto_on"}},
		"negative transition":   {SingleLightConfig{TransitionDuration: Float64Ptr(-0.5)}, []string{"light.lights.0.transition_duration"}},
		"min brightness high":   {SingleLightConfig{MinBrightnessOnToggle: IntPtr(101)}, []string{"light.lights.0.min_brightness_on_toggle"}},
		"min brightness low":    {SingleLightConfig{MinBrightnessOnToggle: IntPtr(-1)}, []string{"light.lights.0.min_brightness_on_toggle"}},
		"fade rate zero":        {SingleLightConfig{ButtonFadeRate: IntPtr(0)}, []string{"light.lights.0.button_fade_rate"}},
		"fade rate six":         {SingleLightConfig{ButtonFadeRate: IntPtr(6)}, []string{"light.lights.0.button_fade_rate"}},
		"night brightness high": {SingleLightConfig{NightModeBrightness: IntPtr(101)}, []string{"light.lights.0.night_mode_brightness"}},
		"warmup brightness low": {SingleLightConfig{WarmupBrightness: IntPtr(9)}, []string{"light.lights.0.warmup_brightness"}},
		"warmup time short":     {SingleLightConfig{WarmupTime: IntPtr(19)}, []string{"light.lights.0.warmup_time"}},
		"warmup time long":      {SingleLightConfig{WarmupTime: IntPtr(1001)}, []string{"light.lights.0.warmup_time"}},
		"span with one end":     {SingleLightConfig{NightModeStart: StringPtr("22:00")}, []string{"light.lights.0.night_mode_start"}},
		"span not a clock":      {SingleLightConfig{NightModeStart: StringPtr("24:00"), NightModeEnd: StringPtr("06:00")}, []string{"light.lights.0.night_mode_end", "light.lights.0.night_mode_start"}},
		"span without zero":     {SingleLightConfig{NightModeStart: StringPtr("6:00"), NightModeEnd: StringPtr("07:00")}, []string{"light.lights.0.night_mode_end", "light.lights.0.night_mode_start"}},
		"gen1 effect":           {SingleLightConfig{Effect: IntPtr(1)}, []string{"light.lights.0.effect"}},
		"gen1 button type":      {SingleLightConfig{ButtonType: StringPtr("toggle")}, []string{"light.lights.0.btn_type"}},
		"gen1 schedule":         {SingleLightConfig{Schedule: BoolPtr(true)}, []string{"light.lights.0.schedule"}},
		"gen1 transition":       {SingleLightConfig{Transition: IntPtr(500)}, []string{"light.lights.0.transition"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{tc.light}}})
			dev := &coverReads{fakeGen2Device: fakeGen2DeviceWithLight()}

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			for _, path := range tc.paths {
				assert.Contains(t, err.Error(), path)
			}
			assert.Empty(t, dev.calls)
			assert.Equal(t, 0, dev.reads, "refused before the device was read")
		})
	}
}

func TestApplyDesiredConfig_Gen2AcceptsTheLightBounds(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{{
		ID: 0, DefaultState: StringPtr("toggle"), AutoOn: IntPtr(1), MinBrightnessOnToggle: IntPtr(100), ButtonFadeRate: IntPtr(5),
		NightModeBrightness: IntPtr(0), WarmupBrightness: IntPtr(100), WarmupTime: IntPtr(1000),
		NightModeStart: StringPtr("00:00"), NightModeEnd: StringPtr("23:59"),
	}}}})
	dev := fakeGen2DeviceWithLight()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
}

// A repeated or negative channel id is named at its place in the desired list.
func TestApplyDesiredConfig_Gen2RefusesARepeatedLightId(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{
		{ID: 0, Name: StringPtr("a")}, {ID: 0, Name: StringPtr("b")}, {ID: -1},
	}}})
	dev := fakeGen2DeviceWithLight()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "light.lights.1.id")
	assert.Contains(t, err.Error(), "light.lights.2.id")
	assert.Empty(t, dev.calls)
}

// The page shows no restart_required for Light.SetConfig, so the device's own answer decides.
func TestApplyDesiredConfig_Gen2LightHonoursTheRestartAnswer(t *testing.T) {
	for _, answer := range []bool{false, true} {
		t.Run(fmt.Sprintf("answer %v", answer), func(t *testing.T) {
			svc, _ := newGen2Service(t, lightDesired())
			dev := fakeGen2DeviceWithLight()
			dev.restart = answer

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

			require.NoError(t, err)
			assert.Equal(t, answer, result.ApplyResult.RequiresReboot)
		})
	}
}

// A Gen2-only light key on a Gen1 Dimmer has no Gen1 key: the round trip refuses it.
func TestGen1DimmerRefusesTheGen2LightKeys(t *testing.T) {
	desired := &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{{
		ID: 0, TransitionDuration: Float64Ptr(1), ButtonFadeRate: IntPtr(2), WarmupTime: IntPtr(200), NightMode: BoolPtr(true),
	}}}}

	unsupported, err := unsupportedApplySettings(NewGen1Converter(nil), desired, "SHDM-1", true)

	require.NoError(t, err)
	assert.NotEmpty(t, unsupported)
	for _, path := range unsupported {
		assert.Contains(t, path, "light", path)
	}
}

// One word per meaning across the two generations: Gen1 last is Gen2 restore_last on a light, and
// a Gen1 Dimmer's own words stay as they were.
func TestGen2LightInitialStateWordIsOneValue(t *testing.T) {
	assert.Equal(t, "restore_last", gen2LightInitialStateWord("last"))
	assert.Equal(t, "toggle", gen2LightInitialStateWord("toggle"))
	assert.Equal(t, "switch", gen2LightInitialStateWord("switch"), "a light has no word for switch")
	assert.Equal(t, 5, gen2LightInitialStateWord(5))
}

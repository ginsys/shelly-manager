package configuration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bulbDevice is a Gen1 Bulb as the apply sees it: the Dimmer's fake (one /settings/light/<n> route
// that stores into the "lights" array of GET /settings) with a mode that a write changes. events lists
// the mode and light writes in order.
type bulbDevice struct {
	*dimmerDevice
	events []string
}

func newBulbDevice(mode string) *bulbDevice {
	d := &bulbDevice{dimmerDevice: newDimmerDevice()}
	d.settings["mode"] = mode
	d.settings["lights"] = []interface{}{map[string]interface{}{}}
	return d
}

func (d *bulbDevice) SetConfig(ctx context.Context, cfg map[string]interface{}) error {
	if want, ok := cfg["mode"]; ok {
		d.events = append(d.events, fmt.Sprintf("mode=%v", want))
	}
	return d.dimmerDevice.SetConfig(ctx, cfg)
}

func (d *bulbDevice) SetLightSettings(ctx context.Context, channel int, form map[string]interface{}) error {
	d.events = append(d.events, fmt.Sprintf("light/%d", channel))
	return d.dimmerDevice.SetLightSettings(ctx, channel, form)
}

// nothingWritten says no write of any kind reached the device.
func (d *bulbDevice) nothingWritten(t *testing.T) {
	t.Helper()
	assert.Equal(t, 0, d.setCalls, "nothing may be written to /settings")
	assert.Empty(t, d.lightWrites, "nothing may reach /settings/light/<n>")
	assert.Empty(t, d.relayWrites, "nothing may reach /settings/relay/<n>")
}

func bulbLights(lights ...SingleLightConfig) *DeviceConfiguration {
	for i := range lights {
		lights[i].ID = i
	}
	return &DeviceConfiguration{Light: &LightConfig{Lights: lights}}
}

func withBulbMode(config *DeviceConfiguration, mode string) *DeviceConfiguration {
	config.System = &SystemConfiguration{Device: &TypedDeviceConfig{Mode: StringPtr(mode)}}
	return config
}

// The page's /settings/light/0 parameters are written as one form POST and read back from the same
// path; reset is an action and is never sent.
func TestApplyBulbLight_IsWrittenAndReadBack(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "bulb")
	rules := []string{"0800-0123456-on", "2200-0123456-off"}
	setDesired(t, repo, bulbLights(SingleLightConfig{
		DefaultState: StringPtr("last"), AutoOn: IntPtr(5), AutoOff: IntPtr(60), Effect: IntPtr(3),
		Schedule: BoolPtr(true), ScheduleRules: &rules,
	}))
	dev := newBulbDevice("white")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.lightWrites, 1)
	assert.Equal(t, 0, dev.lightWrites[0].channel)
	assert.Equal(t, map[string]interface{}{
		"default_state": "last", "auto_on": float64(5), "auto_off": float64(60), "effect": float64(3),
		"schedule": true, "schedule_rules": []interface{}{"0800-0123456-on", "2200-0123456-off"},
	}, dev.lightWrites[0].form)
	assert.NotContains(t, dev.lightWrites[0].form, "reset")
	assert.Equal(t, []string{"light/0"}, dev.events, "no mode is written when it already holds")
}

func TestApplyBulbLight_EachKeyAloneIsWritten(t *testing.T) {
	rules := []string{"0800-0123456-on"}
	cases := map[string]struct {
		light SingleLightConfig
		form  map[string]interface{}
	}{
		"default_state": {SingleLightConfig{DefaultState: StringPtr("off")}, map[string]interface{}{"default_state": "off"}},
		"auto_on":       {SingleLightConfig{AutoOn: IntPtr(7)}, map[string]interface{}{"auto_on": float64(7)}},
		"auto_off":      {SingleLightConfig{AutoOff: IntPtr(9)}, map[string]interface{}{"auto_off": float64(9)}},
		"effect":        {SingleLightConfig{Effect: IntPtr(6)}, map[string]interface{}{"effect": float64(6)}},
		"schedule":      {SingleLightConfig{Schedule: BoolPtr(false)}, map[string]interface{}{"schedule": false}},
		"rules":         {SingleLightConfig{ScheduleRules: &rules}, map[string]interface{}{"schedule_rules": []interface{}{"0800-0123456-on"}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "bulb")
			setDesired(t, repo, bulbLights(tc.light))
			dev := newBulbDevice("color")

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied)
			require.Len(t, dev.lightWrites, 1)
			assert.Equal(t, tc.form, dev.lightWrites[0].form)
		})
	}
}

// A device that answers 200 and keeps the old value fails at that key's path.
func TestApplyBulbLight_ReadBackMismatchFails(t *testing.T) {
	dev := newBulbDevice("white")
	dev.lightLive = map[int]map[string]interface{}{0: {"effect": float64(0), "auto_off": float64(60)}}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	apiConfig := bulbLights(SingleLightConfig{Effect: IntPtr(3), AutoOff: IntPtr(60)})

	result, err := applier.ApplyConfig(context.Background(), dev, apiConfig, "SHBLB-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "lights[0].effect", result.Failures[0].Path)
}

// The page lists effect as a parameter of /settings/light/0 and marks only red, green, blue, white and
// gain as color-mode: effect is accepted in white mode too (unconfirmed on a device).
func TestApplyBulbLight_EffectIsAcceptedInBothModes(t *testing.T) {
	for _, mode := range []string{"white", "color"} {
		t.Run(mode, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "bulb")
			setDesired(t, repo, bulbLights(SingleLightConfig{Effect: IntPtr(1)}))
			dev := newBulbDevice(mode)

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied)
			require.Len(t, dev.lightWrites, 1)
		})
	}
}

// A desired mode that differs from the reported one is written first, then read back, and the light
// follows it.
func TestApplyBulbMode_IsWrittenFirstAndReadBack(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "bulb")
	setDesired(t, repo, withBulbMode(bulbLights(SingleLightConfig{Effect: IntPtr(2)}), "color"))
	dev := newBulbDevice("white")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, []string{"mode=color", "light/0"}, dev.events)
	assert.Equal(t, "color", dev.settings["mode"])
}

// A mode the device already reports is not written again.
func TestApplyBulbMode_IsNotRewrittenWhenItHolds(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "bulb")
	setDesired(t, repo, withBulbMode(bulbLights(SingleLightConfig{Effect: IntPtr(2)}), "white"))
	dev := newBulbDevice("white")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, []string{"light/0"}, dev.events)
}

// A device that accepts the mode and does not report it stops the apply: no light is written.
func TestApplyBulbMode_ThatDoesNotHoldStopsTheLaterSections(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "bulb")
	setDesired(t, repo, withBulbMode(bulbLights(SingleLightConfig{Effect: IntPtr(2)}), "color"))
	dev := newBulbDevice("white")
	dev.ignoreWrites = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotEmpty(t, result.ApplyResult.Failures)
	assert.Equal(t, "system.device.mode", result.ApplyResult.Failures[0].Path)
	assert.Empty(t, dev.lightWrites)
	assert.Equal(t, []string{"mode=color"}, dev.events)
}

// A word the page does not list is refused before any write.
func TestApplyBulbMode_UnknownWordIsRefusedBeforeAnyWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "bulb")
	setDesired(t, repo, withBulbMode(bulbLights(SingleLightConfig{Effect: IntPtr(2)}), "rainbow"))
	dev := newBulbDevice("white")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "system.device.mode")
	dev.nothingWritten(t)
}

// The settings keys do not depend on the mode (RULING-20261122-1836-381s9), so the mode word alone,
// with no light, is written and read back too.
func TestApplyBulbMode_AloneIsWritten(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "bulb")
	setDesired(t, repo, withBulbMode(&DeviceConfiguration{}, "color"))
	dev := newBulbDevice("white")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, []string{"mode=color"}, dev.events)
}

// Effect 0 to 6 is the page's list; the edges just outside, a Dimmer word, a negative timer, a bad
// rule, a second channel and the keys the Bulb's page does not have are refused before any write.
func TestApplyBulbLight_RefusesValuesTheDeviceDoesNotTake(t *testing.T) {
	badRules := []string{"0800-0123456-on,2200-0123456-off"}
	cases := map[string]struct {
		lights []SingleLightConfig
		path   string
	}{
		"effect -1":                 {[]SingleLightConfig{{Effect: IntPtr(-1)}}, "light.lights[0].effect"},
		"effect 7":                  {[]SingleLightConfig{{Effect: IntPtr(7)}}, "light.lights[0].effect"},
		"default_state switch":      {[]SingleLightConfig{{DefaultState: StringPtr("switch")}}, "light.lights[0].default_state"},
		"negative auto_on":          {[]SingleLightConfig{{AutoOn: IntPtr(-1)}}, "light.lights[0].auto_on"},
		"negative auto_off":         {[]SingleLightConfig{{AutoOff: IntPtr(-1)}}, "light.lights[0].auto_off"},
		"a schedule rule, a comma":  {[]SingleLightConfig{{ScheduleRules: &badRules}}, "light.lights[0].schedule_rules"},
		"a second channel":          {[]SingleLightConfig{{}, {Effect: IntPtr(1)}}, "light.lights[1]"},
		"name (not on the page)":    {[]SingleLightConfig{{Name: StringPtr("lamp")}}, "light.lights[0].name"},
		"transition":                {[]SingleLightConfig{{Transition: IntPtr(500)}}, "light.lights[0].transition"},
		"btn_reverse":               {[]SingleLightConfig{{ButtonReverse: BoolPtr(true)}}, "light.lights[0].btn_reverse"},
		"btn_type (a Dimmer key)":   {[]SingleLightConfig{{ButtonType: StringPtr("toggle")}}, "light.lights[0].btn_type"},
		"btn_debounce (a Dimmer)":   {[]SingleLightConfig{{ButtonDebounce: IntPtr(80)}}, "light.lights[0].btn_debounce"},
		"swap_inputs (a Dimmer)":    {[]SingleLightConfig{{SwapInputs: BoolPtr(true)}}, "light.lights[0].swap_inputs"},
		"effect 7 with a good rest": {[]SingleLightConfig{{DefaultState: StringPtr("on"), Effect: IntPtr(7)}}, "light.lights[0].effect"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "bulb")
			setDesired(t, repo, bulbLights(tc.lights...))
			dev := newBulbDevice("white")

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), tc.path)
			dev.nothingWritten(t)
		})
	}
}

func TestApplyBulbLight_EdgesOfTheEffectListAreAccepted(t *testing.T) {
	for _, effect := range []int{0, 6} {
		t.Run(fmt.Sprint(effect), func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "bulb")
			setDesired(t, repo, bulbLights(SingleLightConfig{Effect: IntPtr(effect)}))
			dev := newBulbDevice("color")

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied)
		})
	}
}

// The Bulb has no relay and no Dimmer group: a desired relay or dimming section is refused before any
// write, and hasDimmerSettings stays Dimmer-only.
func TestApplyBulb_RefusesRelayAndDimmingSections(t *testing.T) {
	cases := map[string]*DeviceConfiguration{
		"relay":   {Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, AutoOff: IntPtr(60)}}}},
		"dimming": {Dimming: &DimmingConfig{MinBrightness: IntPtr(5)}},
	}
	for name, desired := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "bulb")
			setDesired(t, repo, desired)
			dev := newBulbDevice("white")

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBLB-1")

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), name)
			dev.nothingWritten(t)
		})
	}
	converter := NewGen1Converter(nil)
	assert.False(t, converter.hasDimmerSettings("SHBLB-1"))
	assert.False(t, converter.hasRelay("SHBLB-1"))
	assert.True(t, converter.hasLight("SHBLB-1"))
}

// The Duo and the Vintage stay refused as models: a desired light on either is not applied.
func TestApplyBulb_DuoAndVintageStayRefused(t *testing.T) {
	supported := NewGen1Converter(nil).SupportedDeviceTypes()
	assert.Contains(t, supported, "SHBLB-1")
	assert.NotContains(t, supported, "SHBDUO-1")
	assert.NotContains(t, supported, "SHVIN-1")
	for _, model := range []string{"SHBDUO-1", "SHVIN-1"} {
		t.Run(model, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "bulb")
			setDesired(t, repo, bulbLights(SingleLightConfig{Effect: IntPtr(1)}))
			dev := newBulbDevice("white")

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

			require.Error(t, err)
			dev.nothingWritten(t)
		})
	}
}

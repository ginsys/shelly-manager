package configuration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rgbw2Device is a Gen1 RGBW2 as the apply sees it: a fakeGen1Device that keeps its mode between a
// write and a read and holds one channel in color mode and four in white mode. POST and GET on
// /settings/color/<n> and /settings/white/<n> store into the "lights" array of GET /settings, which
// is where the device reports its channels. events lists every mode and channel write in order.
type rgbw2Device struct {
	*fakeGen1Device
	events      []string
	colorWrites []relayWrite
	whiteWrites []relayWrite
}

func newRGBW2Device(mode string) *rgbw2Device {
	d := &rgbw2Device{fakeGen1Device: newFakeGen1Device()}
	d.settings["mode"] = mode
	channels := 1
	if mode == "white" {
		channels = 4
	}
	lights := make([]interface{}, channels)
	for i := range lights {
		lights[i] = map[string]interface{}{}
	}
	d.settings["lights"] = lights
	return d
}

func (d *rgbw2Device) SetConfig(ctx context.Context, cfg map[string]interface{}) error {
	if want, ok := cfg["mode"]; ok {
		d.events = append(d.events, fmt.Sprintf("mode=%v", want))
	}
	return d.fakeGen1Device.SetConfig(ctx, cfg)
}

func (d *rgbw2Device) lightDoc(channel int) map[string]interface{} {
	lights, _ := d.settings["lights"].([]interface{})
	for len(lights) <= channel {
		lights = append(lights, map[string]interface{}{})
	}
	d.settings["lights"] = lights
	doc, _ := lights[channel].(map[string]interface{})
	return doc
}

func (d *rgbw2Device) store(channel int, form map[string]interface{}) {
	doc := d.lightDoc(channel)
	for k, v := range form {
		// A device returns JSON numbers as float64.
		if n, ok := v.(int); ok {
			v = float64(n)
		}
		doc[k] = v
	}
}

func (d *rgbw2Device) SetColorSettings(_ context.Context, channel int, form map[string]interface{}) error {
	d.events = append(d.events, fmt.Sprintf("color/%d", channel))
	d.colorWrites = append(d.colorWrites, relayWrite{channel: channel, form: form})
	d.store(channel, form)
	return nil
}

func (d *rgbw2Device) GetColorSettings(_ context.Context, channel int) (map[string]interface{}, error) {
	return d.lightDoc(channel), nil
}

func (d *rgbw2Device) SetWhiteSettings(_ context.Context, channel int, form map[string]interface{}) error {
	d.events = append(d.events, fmt.Sprintf("white/%d", channel))
	d.whiteWrites = append(d.whiteWrites, relayWrite{channel: channel, form: form})
	d.store(channel, form)
	return nil
}

func (d *rgbw2Device) GetWhiteSettings(_ context.Context, channel int) (map[string]interface{}, error) {
	return d.lightDoc(channel), nil
}

// nothingWritten says no write of any kind reached the device.
func (d *rgbw2Device) nothingWritten(t *testing.T) {
	t.Helper()
	assert.Equal(t, 0, d.setCalls, "nothing may be written to /settings")
	assert.Empty(t, d.colorWrites, "nothing may reach /settings/color/<n>")
	assert.Empty(t, d.whiteWrites, "nothing may reach /settings/white/<n>")
	assert.Empty(t, d.relayWrites, "nothing may reach /settings/relay/<n>")
}

func rgbw2Lights(lights ...SingleLightConfig) *DeviceConfiguration {
	for i := range lights {
		lights[i].ID = i
	}
	return &DeviceConfiguration{Light: &LightConfig{Lights: lights}}
}

// A desired color channel is a form POST to /settings/color/0 with only the keys that are set, then a
// read of the same path. The mode already holds, so no mode is written.
func TestApplyDesiredConfig_Gen1RGBW2WritesColor(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(SingleLightConfig{DefaultState: StringPtr("last"), AutoOff: IntPtr(60), ButtonType: StringPtr("toggle")}))
	dev := newRGBW2Device("color")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.colorWrites, 1)
	assert.Equal(t, 0, dev.colorWrites[0].channel)
	assert.Equal(t, map[string]interface{}{"default_state": "last", "auto_off": float64(60), "btn_type": "toggle"}, dev.colorWrites[0].form)
	assert.Empty(t, dev.whiteWrites)
	assert.Equal(t, []string{"color/0"}, dev.events, "no mode is written when it already holds")
}

// The four white channels are written to /settings/white/0 to 3. The page says btn_type and btn_reverse
// are shared and visible only on channel 0, so they are written there only.
func TestApplyDesiredConfig_Gen1RGBW2WritesFourWhiteChannels(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(
		SingleLightConfig{DefaultState: StringPtr("off"), AutoOff: IntPtr(10), ButtonType: StringPtr("momentary")},
		SingleLightConfig{DefaultState: StringPtr("on")},
		SingleLightConfig{AutoOn: IntPtr(5)},
		SingleLightConfig{DefaultState: StringPtr("last")},
	))
	dev := newRGBW2Device("white")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.whiteWrites, 4)
	assert.Equal(t, map[string]interface{}{"default_state": "off", "auto_off": float64(10), "btn_type": "momentary"}, dev.whiteWrites[0].form)
	assert.Equal(t, map[string]interface{}{"default_state": "on"}, dev.whiteWrites[1].form)
	assert.Equal(t, map[string]interface{}{"auto_on": float64(5)}, dev.whiteWrites[2].form)
	assert.Equal(t, map[string]interface{}{"default_state": "last"}, dev.whiteWrites[3].form)
	assert.Empty(t, dev.colorWrites)
}

// btn_type and btn_reverse on a white channel other than 0 are refused, with the path, before any write.
func TestApplyDesiredConfig_Gen1RGBW2RefusesSharedButtonKeysOnWhite1To3(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(
		SingleLightConfig{DefaultState: StringPtr("on")},
		SingleLightConfig{},
		SingleLightConfig{ButtonType: StringPtr("toggle")},
	))
	dev := newRGBW2Device("white")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "light.lights[2].btn_type")
	dev.nothingWritten(t)
}

// The mode guard: a desired section for the mode the device is not in is refused before any write.
// In color mode there is one channel, so a second light is another mode's section.
func TestApplyDesiredConfig_Gen1RGBW2RefusesLightsOfTheOtherMode(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(SingleLightConfig{DefaultState: StringPtr("on")}, SingleLightConfig{DefaultState: StringPtr("on")}))
	dev := newRGBW2Device("color")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "light.lights[1]")
	assert.Contains(t, err.Error(), "color")
	dev.nothingWritten(t)
}

// A desired mode that differs from the live one is written first, alone, and the channel sections
// that follow are the new mode's: four white channels on a device that was in color mode.
func TestApplyDesiredConfig_Gen1RGBW2SwitchesTheModeFirst(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	desired := rgbw2Lights(SingleLightConfig{DefaultState: StringPtr("on")}, SingleLightConfig{AutoOff: IntPtr(30)})
	desired.System = &SystemConfiguration{Device: &TypedDeviceConfig{Mode: StringPtr("white")}}
	setDesired(t, repo, desired)
	dev := newRGBW2Device("color")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, []string{"mode=white", "white/0", "white/1"}, dev.events)
	assert.Empty(t, dev.colorWrites)
}

// A device whose mode cannot be read, and a desired configuration that states none, leaves the
// channel section unconfirmable: it is refused, not written to whichever endpoint is guessed.
func TestApplyDesiredConfig_Gen1RGBW2RefusesWhenTheModeIsUnknown(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(SingleLightConfig{DefaultState: StringPtr("on")}))
	dev := newRGBW2Device("color")
	delete(dev.settings, "mode")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "mode")
	dev.nothingWritten(t)
}

// Values outside the page's lists and ranges are refused before any write, for both modes.
func TestApplyDesiredConfig_Gen1RGBW2RefusesValuesTheDeviceDoesNotTake(t *testing.T) {
	rules := []string{"0000-0123456-on,0100-0123456-off"}
	cases := map[string]struct {
		mode  string
		light SingleLightConfig
		path  string
	}{
		"default_state switch (a Dimmer word)": {"color", SingleLightConfig{DefaultState: StringPtr("switch")}, "light.lights[0].default_state"},
		"btn_type one_button (a Dimmer word)":  {"color", SingleLightConfig{ButtonType: StringPtr("one_button")}, "light.lights[0].btn_type"},
		"negative auto_on":                     {"color", SingleLightConfig{AutoOn: IntPtr(-1)}, "light.lights[0].auto_on"},
		"a schedule rule with a comma":         {"white", SingleLightConfig{ScheduleRules: &rules}, "light.lights[0].schedule_rules"},
		"debounce is a Dimmer key":             {"color", SingleLightConfig{ButtonDebounce: IntPtr(80)}, "light.lights[0].btn_debounce"},
		"swap_inputs is a Dimmer key":          {"white", SingleLightConfig{SwapInputs: BoolPtr(true)}, "light.lights[0].swap_inputs"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "rgbw2")
			setDesired(t, repo, rgbw2Lights(tc.light))
			dev := newRGBW2Device(tc.mode)

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), tc.path)
			dev.nothingWritten(t)
		})
	}
}

// An RGBW2 has no relay: its channels are /settings/color/0 and /settings/white/<n>.
func TestApplyDesiredConfig_Gen1RGBW2RefusesARelay(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, AutoOff: IntPtr(60)}}}})
	dev := newRGBW2Device("color")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "relay")
	dev.nothingWritten(t)
}

// The Dimmer's device-level group is not an RGBW2's: a desired dimming section is refused, and the
// schema of the model holds no such section.
func TestApplyDesiredConfig_Gen1RGBW2RefusesADimmingSection(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, &DeviceConfiguration{Dimming: &DimmingConfig{MinBrightness: IntPtr(5)}})
	dev := newRGBW2Device("color")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "dimming")
	dev.nothingWritten(t)

	sections, _ := GetDeviceConfigurationSchema("SHRGBW2")["properties"].(map[string]interface{})
	assert.NotContains(t, sections, "dimming")
	assert.Contains(t, sections, "light")
	assert.NotContains(t, sections, "relay")
}

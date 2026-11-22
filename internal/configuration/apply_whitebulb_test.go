package configuration

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Gen1 Duo and Vintage pages (read, not confirmed on a device): one white channel, no mode and no
// effect, /settings/light/0 with name, default_state, auto_on, auto_off, schedule and schedule_rules,
// and /settings/night_mode with enabled, start_time, end_time and brightness 0 to 100.
var whiteBulbModels = []string{"SHBDUO-1", "SHVIN-1"}

// whiteBulb is a Duo or Vintage as the apply sees it: one light channel in the "lights" array of GET
// /settings, a night_mode object a POST on /settings/night_mode writes, and a mode that is always white.
type whiteBulb struct {
	*nightDimmer
}

func newWhiteBulb() *whiteBulb {
	d := &whiteBulb{nightDimmer: newNightDimmer()}
	d.settings["mode"] = "white"
	d.settings["lights"] = []interface{}{map[string]interface{}{}}
	return d
}

// nothingWritten says no write of any kind reached the device.
func (d *whiteBulb) nothingWritten(t *testing.T) {
	t.Helper()
	assert.Equal(t, 0, d.setCalls, "nothing may be written to /settings")
	assert.Empty(t, d.lightWrites, "nothing may reach /settings/light/<n>")
	assert.Empty(t, d.nightPosts, "nothing may reach /settings/night_mode")
	assert.Empty(t, d.warmPosts, "nothing may reach /settings/warm_up")
}

func whiteBulbNight(night DimmingConfig, lights ...SingleLightConfig) *DeviceConfiguration {
	config := bulbLights(lights...)
	if len(lights) == 0 {
		config = &DeviceConfiguration{}
	}
	config.Dimming = &night
	return config
}

// The page's /settings/light/0 parameters, name included, are written as one form POST and read back
// from the same path; reset is an action and is never sent.
func TestApplyWhiteBulbLight_IsWrittenAndReadBack(t *testing.T) {
	for _, model := range whiteBulbModels {
		t.Run(model, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "lamp")
			rules := []string{"0800-0123456-on", "2200-0123456-off"}
			setDesired(t, repo, bulbLights(SingleLightConfig{
				Name: StringPtr("hall"), DefaultState: StringPtr("last"), AutoOn: IntPtr(5), AutoOff: IntPtr(60),
				Schedule: BoolPtr(true), ScheduleRules: &rules,
			}))
			dev := newWhiteBulb()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
			require.Len(t, dev.lightWrites, 1)
			assert.Equal(t, 0, dev.lightWrites[0].channel)
			assert.Equal(t, map[string]interface{}{
				"name": "hall", "default_state": "last", "auto_on": float64(5), "auto_off": float64(60),
				"schedule": true, "schedule_rules": []interface{}{"0800-0123456-on", "2200-0123456-off"},
			}, dev.lightWrites[0].form)
			assert.NotContains(t, dev.lightWrites[0].form, "reset")
		})
	}
}

func TestApplyWhiteBulbLight_EachKeyAloneIsWritten(t *testing.T) {
	rules := []string{"0800-0123456-on"}
	cases := map[string]struct {
		light SingleLightConfig
		form  map[string]interface{}
	}{
		"name":          {SingleLightConfig{Name: StringPtr("desk")}, map[string]interface{}{"name": "desk"}},
		"default_state": {SingleLightConfig{DefaultState: StringPtr("off")}, map[string]interface{}{"default_state": "off"}},
		"auto_on":       {SingleLightConfig{AutoOn: IntPtr(7)}, map[string]interface{}{"auto_on": float64(7)}},
		"auto_off":      {SingleLightConfig{AutoOff: IntPtr(9)}, map[string]interface{}{"auto_off": float64(9)}},
		"schedule":      {SingleLightConfig{Schedule: BoolPtr(false)}, map[string]interface{}{"schedule": false}},
		"rules":         {SingleLightConfig{ScheduleRules: &rules}, map[string]interface{}{"schedule_rules": []interface{}{"0800-0123456-on"}}},
	}
	for _, model := range whiteBulbModels {
		for name, tc := range cases {
			t.Run(model+"/"+name, func(t *testing.T) {
				svc, repo := newServiceWithDesiredName(t, "lamp")
				setDesired(t, repo, bulbLights(tc.light))
				dev := newWhiteBulb()

				result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

				require.NoError(t, err)
				assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
				require.Len(t, dev.lightWrites, 1)
				assert.Equal(t, tc.form, dev.lightWrites[0].form)
			})
		}
	}
}

// A device that answers 200 and keeps the old name fails at that key's path.
func TestApplyWhiteBulbLight_ReadBackMismatchFails(t *testing.T) {
	for _, model := range whiteBulbModels {
		t.Run(model, func(t *testing.T) {
			dev := newWhiteBulb()
			dev.lightLive = map[int]map[string]interface{}{0: {"name": "old", "auto_off": float64(60)}}
			applier := NewConfigApplier(NewGen1Converter(nil), nil)
			apiConfig := bulbLights(SingleLightConfig{Name: StringPtr("new"), AutoOff: IntPtr(60)})

			result, err := applier.ApplyConfig(context.Background(), dev, apiConfig, model)

			require.NoError(t, err)
			assert.False(t, result.Success)
			require.Len(t, result.Failures, 1)
			assert.Equal(t, "lights[0].name", result.Failures[0].Path)
		})
	}
}

// The pages list no mode parameter and no effect, so neither is written, and a mode word is refused
// like any other on a model with no mode.
func TestApplyWhiteBulb_ModeAndEffectAreRefusedBeforeAnyWrite(t *testing.T) {
	cases := map[string]struct {
		config *DeviceConfiguration
		path   string
	}{
		"effect":            {bulbLights(SingleLightConfig{Effect: IntPtr(1)}), "light.lights[0].effect"},
		"mode color":        {withBulbMode(bulbLights(SingleLightConfig{DefaultState: StringPtr("on")}), "color"), "system.device.mode"},
		"mode white (same)": {withBulbMode(bulbLights(SingleLightConfig{DefaultState: StringPtr("on")}), "white"), "system.device.mode"},
	}
	for _, model := range whiteBulbModels {
		for name, tc := range cases {
			t.Run(model+"/"+name, func(t *testing.T) {
				svc, repo := newServiceWithDesiredName(t, "lamp")
				setDesired(t, repo, tc.config)
				dev := newWhiteBulb()

				_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

				require.ErrorIs(t, err, ErrUnsupportedSettings)
				assert.Contains(t, err.Error(), tc.path)
				dev.nothingWritten(t)
			})
		}
	}
}

// The keys the pages do not list, a second channel, a bad state, a negative timer and a bad rule are
// refused before any write.
func TestApplyWhiteBulbLight_RefusesValuesTheDeviceDoesNotTake(t *testing.T) {
	badRules := []string{"0800-0123456-on,2200-0123456-off"}
	cases := map[string]struct {
		lights []SingleLightConfig
		path   string
	}{
		"default_state switch": {[]SingleLightConfig{{DefaultState: StringPtr("switch")}}, "light.lights[0].default_state"},
		"negative auto_on":     {[]SingleLightConfig{{AutoOn: IntPtr(-1)}}, "light.lights[0].auto_on"},
		"negative auto_off":    {[]SingleLightConfig{{AutoOff: IntPtr(-1)}}, "light.lights[0].auto_off"},
		"a rule with a comma":  {[]SingleLightConfig{{ScheduleRules: &badRules}}, "light.lights[0].schedule_rules"},
		"a second channel":     {[]SingleLightConfig{{}, {Name: StringPtr("two")}}, "light.lights[1]"},
		"transition":           {[]SingleLightConfig{{Transition: IntPtr(500)}}, "light.lights[0].transition"},
		"btn_reverse":          {[]SingleLightConfig{{ButtonReverse: BoolPtr(true)}}, "light.lights[0].btn_reverse"},
		"btn_type":             {[]SingleLightConfig{{ButtonType: StringPtr("toggle")}}, "light.lights[0].btn_type"},
		"btn_debounce":         {[]SingleLightConfig{{ButtonDebounce: IntPtr(80)}}, "light.lights[0].btn_debounce"},
		"swap_inputs":          {[]SingleLightConfig{{SwapInputs: BoolPtr(true)}}, "light.lights[0].swap_inputs"},
	}
	for _, model := range whiteBulbModels {
		for name, tc := range cases {
			t.Run(model+"/"+name, func(t *testing.T) {
				svc, repo := newServiceWithDesiredName(t, "lamp")
				setDesired(t, repo, bulbLights(tc.lights...))
				dev := newWhiteBulb()

				_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

				require.ErrorIs(t, err, ErrUnsupportedSettings)
				assert.Contains(t, err.Error(), tc.path)
				dev.nothingWritten(t)
			})
		}
	}
}

// Night mode is not a parameter of POST /settings: its four keys go to POST /settings/night_mode and
// are read back from the night_mode object of GET /settings. Brightness 0 is the page's own example.
func TestApplyWhiteBulbNightMode_IsWrittenAndReadBack(t *testing.T) {
	for _, model := range whiteBulbModels {
		for _, brightness := range []int{0, 20, 100} {
			t.Run(model, func(t *testing.T) {
				svc, repo := newServiceWithDesiredName(t, "lamp")
				setDesired(t, repo, whiteBulbNight(DimmingConfig{
					NightModeEnabled: BoolPtr(true), NightModeStart: StringPtr("22:00"), NightModeEnd: StringPtr("06:00"),
					NightModeBrightness: IntPtr(brightness),
				}))
				dev := newWhiteBulb()

				result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

				require.NoError(t, err)
				assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
				require.Len(t, dev.nightPosts, 1)
				assert.Equal(t, map[string]interface{}{
					"enabled": true, "start_time": "22:00", "end_time": "06:00", "brightness": float64(brightness),
				}, dev.nightPosts[0])
				assert.Empty(t, dev.posted, "night mode is not a parameter of POST /settings")
				assert.Empty(t, dev.warmPosts)
			})
		}
	}
}

func TestApplyWhiteBulbNightMode_AndLightAreWrittenTogether(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "lamp")
	setDesired(t, repo, whiteBulbNight(DimmingConfig{NightModeEnabled: BoolPtr(false)}, SingleLightConfig{Name: StringPtr("hall")}))
	dev := newWhiteBulb()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHBDUO-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.lightWrites, 1)
	require.Len(t, dev.nightPosts, 1)
	assert.Equal(t, map[string]interface{}{"enabled": false}, dev.nightPosts[0])
}

// The page's bounds: hh:mm times and brightness 0 to 100. Every value outside is refused before any
// write.
func TestApplyWhiteBulbNightMode_OutOfRangeIsRefusedBeforeAnyWrite(t *testing.T) {
	cases := map[string]struct {
		night DimmingConfig
		path  string
	}{
		"brightness -1":  {DimmingConfig{NightModeBrightness: IntPtr(-1)}, "dimming.night_mode_brightness"},
		"brightness 101": {DimmingConfig{NightModeBrightness: IntPtr(101)}, "dimming.night_mode_brightness"},
		"start 24:00":    {DimmingConfig{NightModeStart: StringPtr("24:00")}, "dimming.night_mode_start"},
		"end 12:60":      {DimmingConfig{NightModeEnd: StringPtr("12:60")}, "dimming.night_mode_end"},
		"end one digit":  {DimmingConfig{NightModeEnd: StringPtr("6:00")}, "dimming.night_mode_end"},
	}
	for _, model := range whiteBulbModels {
		for name, tc := range cases {
			t.Run(model+"/"+name, func(t *testing.T) {
				svc, repo := newServiceWithDesiredName(t, "lamp")
				setDesired(t, repo, whiteBulbNight(tc.night))
				dev := newWhiteBulb()

				_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

				require.ErrorIs(t, err, ErrUnsupportedSettings)
				assert.Contains(t, err.Error(), tc.path)
				dev.nothingWritten(t)
			})
		}
	}
}

// The Duo and Vintage pages have no warm-up and the flat parameters of a Dimmer (fade_rate and the
// rest, but not transition, which they list) are not theirs: the round trip refuses them before any
// write, as it does on the Bulb.
func TestApplyWhiteBulbDimming_OnlyTheNightObjectIsTaken(t *testing.T) {
	cases := map[string]DimmingConfig{
		"fade_rate":        {FadeRate: IntPtr(3)},
		"min_brightness":   {MinBrightness: IntPtr(10)},
		"warm-up enabled":  {WarmupEnabled: BoolPtr(true), WarmupBrightness: IntPtr(50), WarmupTime: IntPtr(100)},
		"night and a flat": {NightModeEnabled: BoolPtr(true), FadeRate: IntPtr(3)},
	}
	for _, model := range whiteBulbModels {
		for name, dimming := range cases {
			t.Run(model+"/"+name, func(t *testing.T) {
				svc, repo := newServiceWithDesiredName(t, "lamp")
				setDesired(t, repo, whiteBulbNight(dimming))
				dev := newWhiteBulb()

				_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

				require.ErrorIs(t, err, ErrUnsupportedSettings)
				dev.nothingWritten(t)
			})
		}
	}
}

// Night mode belongs to the Dimmer, Duo and Vintage: every other model refuses it before any write.
func TestApplyNightMode_OnOtherModelsIsRefused(t *testing.T) {
	for _, model := range []string{"SHBLB-1", "SHRGBW2", "SHSW-1"} {
		t.Run(model, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "lamp")
			setDesired(t, repo, whiteBulbNight(DimmingConfig{NightModeEnabled: BoolPtr(true)}))
			dev := newWhiteBulb()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), "dimming")
			dev.nothingWritten(t)
		})
	}
}

// reset is an action, never a setting: the key list apply sends holds neither it nor a runtime key.
func TestApplyWhiteBulbLight_KeyListHoldsOnlyTheSixSettings(t *testing.T) {
	assert.ElementsMatch(t, []string{"name", "default_state", "auto_on", "auto_off", "schedule", "schedule_rules"}, gen1WhiteBulbLightKeys)
}

// The schema of the Duo and Vintage: the six page keys with name and no effect, the three default
// states, no mode, and a dimming section of the four night-mode leaves and the transition, brightness
// from 0.
func TestDeviceSchemaWhiteBulbHoldsItsOwnKeys(t *testing.T) {
	for _, model := range whiteBulbModels {
		t.Run(model, func(t *testing.T) {
			leaves := map[string]map[string]interface{}{}
			schemaLeaves("", GetDeviceConfigurationSchema(model), leaves)

			for _, key := range gen1WhiteBulbLightKeys {
				leaf, ok := leaves["light.lights[]."+key]
				require.True(t, ok, "no schema leaf %s", key)
				assert.Equal(t, true, leaf[SchemaKeyApplicable], key)
			}
			assert.Equal(t, []string{"on", "off", "last"}, leaves["light.lights[].default_state"]["enum"])
			for _, key := range []string{"effect", "transition", "btn_reverse", "btn_type", "btn_debounce", "swap_inputs"} {
				assert.NotContains(t, leaves, "light.lights[]."+key)
			}
			assert.NotContains(t, leaves, "system.device.mode", "the page has no mode parameter")

			var dimming []string
			for path := range leaves {
				if strings.HasPrefix(path, "dimming.") {
					dimming = append(dimming, path)
				}
			}
			assert.ElementsMatch(t, []string{
				"dimming.night_mode", "dimming.night_mode_brightness", "dimming.night_mode_start", "dimming.night_mode_end",
				"dimming.transition",
			}, dimming)
			for _, path := range dimming {
				assert.Equal(t, true, leaves[path][SchemaKeyApplicable], path)
			}
			assert.EqualValues(t, 0, leaves["dimming.night_mode_brightness"]["minimum"])
		})
	}
}

// The Dimmer keeps its night brightness from 1 and the whole group, and the Bulb keeps its effect and
// no name.
func TestDeviceSchemaOtherModelsAreUnchangedByTheWhiteBulbs(t *testing.T) {
	dimmer := map[string]map[string]interface{}{}
	schemaLeaves("", GetDeviceConfigurationSchema("SHDM-2"), dimmer)
	assert.EqualValues(t, 1, dimmer["dimming.night_mode_brightness"]["minimum"])
	assert.Contains(t, dimmer, "dimming.fade_rate")
	assert.NotContains(t, dimmer, "light.lights[].effect")

	bulb := map[string]map[string]interface{}{}
	schemaLeaves("", GetDeviceConfigurationSchema("SHBLB-1"), bulb)
	assert.Contains(t, bulb, "light.lights[].effect")
	assert.NotContains(t, bulb, "light.lights[].name")
}

// A template with no model holds the widest night brightness bound, so a group of Duos can carry 0;
// a model's own schema narrows it: the Dimmer to 1, the Duo and Vintage keep 0.
func TestValidateConfigBodyNightBrightnessBoundFollowsTheModel(t *testing.T) {
	body := func(brightness int) map[string]interface{} {
		return map[string]interface{}{"dimming": map[string]interface{}{"night_mode_brightness": float64(brightness)}}
	}
	cases := []struct {
		model      string
		brightness int
		valid      bool
	}{
		{"", 0, true}, {"", 101, false},
		{"SHBDUO-1", 0, true}, {"SHVIN-1", 0, true}, {"SHBDUO-1", 101, false},
		{"SHDM-1", 0, false}, {"SHDM-2", 1, true}, {"SHDM-2", 101, false},
	}
	for _, tc := range cases {
		errs := ValidateConfigBody(tc.model, body(tc.brightness))
		assert.Equal(t, tc.valid, len(errs) == 0, "model %q brightness %d: %v", tc.model, tc.brightness, errs)
	}
}

// The Duo and Vintage are not Dimmers: hasDimmerSettings stays the Dimmers' alone, and night mode is
// the one section they share with them.
func TestWhiteBulbPredicates(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, model := range whiteBulbModels {
		assert.False(t, converter.hasDimmerSettings(model), model)
		assert.False(t, converter.hasRelay(model), model)
		assert.True(t, converter.hasLight(model), model)
		assert.True(t, converter.hasNightMode(model), model)
		assert.True(t, hasLightName(model), model)
		assert.False(t, hasLightEffect(model), model)
		assert.Empty(t, gen1DeviceModes(model), model)
	}
	for _, model := range []string{"SHDM-1", "SHDM-2"} {
		assert.True(t, converter.hasDimmerSettings(model), model)
		assert.True(t, converter.hasNightMode(model), model)
		assert.False(t, isGen1WhiteBulb(model), model)
	}
}

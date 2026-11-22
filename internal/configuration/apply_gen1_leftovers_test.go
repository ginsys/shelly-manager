package configuration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Dimmer page's example shows "fade_rate": 0 beside the range 1..5, so a device can report 0.
// A configuration imported from such a device must apply again: the 0 is read as no value, as the
// pulse_mode 0 of the same page is, and a desired 0 stays refused.
func TestApplyDimmerFadeRate_ImportedZeroAppliesAgain(t *testing.T) {
	dev := newDimmerDevice()
	dev.settings["fade_rate"] = float64(0)
	dev.settings["min_brightness"] = float64(20)
	raw, err := json.Marshal(dev.settings)
	require.NoError(t, err)

	imported, err := NewGen1Converter(nil).FromAPIConfig(raw, "SHDM-1")
	require.NoError(t, err)
	require.NotNil(t, imported.Dimming)
	assert.Nil(t, imported.Dimming.FadeRate, "a fade_rate outside 1..5 is no value")
	require.NotNil(t, imported.Dimming.MinBrightness, "the other values are kept")

	edited := *imported.Dimming
	edited.MinBrightness = IntPtr(30)
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(edited))

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.posted, 1)
	assert.Equal(t, map[string]interface{}{"min_brightness": float64(30)}, dev.posted[0])
}

func inputOnlyConfig(mode string) *DeviceConfiguration {
	config := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("follow")}}}}
	if mode != "" {
		config = withMode(config, mode)
	}
	return config
}

// In roller mode the inputs of a Shelly 2.5 are the roller's (roller.button_type and the other roller
// keys): the page lists the relay endpoint's btn_type under relay mode and says input events are only
// detected there. A desired input is refused before any write, naming the path.
func TestApplyConfig_ARollerModeInputIsRefusedBeforeAnyWrite(t *testing.T) {
	for _, tc := range []struct {
		name    string
		live    string
		desired string
	}{
		{"the device runs roller mode", "roller", ""},
		{"roller mode is desired", "relay", "roller"},
		{"roller mode is desired and the device runs it", "roller", "roller"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newModeDevice(tc.live)

			_, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), d, inputOnlyConfig(tc.desired), "SHSW-25")

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), "input.inputs[0]")
			assert.Contains(t, err.Error(), "roller.button_type")
			assert.Equal(t, []string{"get"}, d.events, "only the read before any write")
		})
	}
}

// A template with an input, applied to a roller-mode 2.5, used to write the relay endpoint and is now
// refused as a whole: the service applies the merged desired configuration, which is where a group or
// global template's input arrives, so this is the template path.
func TestApplyDesiredConfig_ATemplateInputOnARollerModeDeviceIsRefused(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "shutter")
	config := inputOnlyConfig("")
	config.System = &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("shutter")}}
	setDesired(t, repo, config)
	d := newModeDevice("roller")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, d, "SHSW-25")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Nil(t, result)
	assert.NotContains(t, d.events, "relay")
	assert.NotContains(t, d.events, "settings")
	assert.False(t, repo.devices[1].ConfigApplied)
}

// What the rule leaves alone: relay mode (also when the apply switches to it first), a mode the
// device does not report as roller, and the models that have no roller mode.
func TestApplyConfig_InputsAreStillWrittenWhereRollerModeIsNotTheMode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		model      string
		live       string
		desired    string
		wantEvents []string
	}{
		{"relay mode", "SHSW-25", "relay", "", []string{"get", "relay"}},
		{"roller now, relay desired: the mode is written first", "SHSW-25", "roller", "relay", []string{"get", "mode=relay", "get", "relay"}},
		{"a mode the device does not report as roller", "SHSW-25", "", "", []string{"get", "relay"}},
		{"a Shelly 1 reports no roller mode", "SHSW-1", "roller", "", []string{"get", "relay"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newModeDevice(tc.live)

			result, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), d, inputOnlyConfig(tc.desired), tc.model)

			require.NoError(t, err)
			assert.True(t, result.Success, "failures: %+v", result.Failures)
			assert.Equal(t, tc.wantEvents, d.events)
		})
	}
}

// The relays of a 2.5 report a button type in either mode, so an import from one in roller mode would
// carry an input section that the rule above refuses on the next apply: it reads none.
func TestGen1Converter_ARollerModeDeviceReadsNoInputs(t *testing.T) {
	converter := NewGen1Converter(nil)
	relays := `"relays":[{"btn_type":"momentary","btn_reverse":0},{"btn_type":"toggle","btn_reverse":1}]`
	for _, tc := range []struct {
		name  string
		raw   string
		model string
		want  int
	}{
		{"2.5 in relay mode", `{"mode":"relay",` + relays + `}`, "SHSW-25", 2},
		{"2.5 in roller mode", `{"mode":"roller",` + relays + `,"rollers":[{}]}`, "SHSW-25", 0},
		{"2.5 with no mode reported", `{` + relays + `}`, "SHSW-25", 2},
		{"Shelly 1", `{"relays":[{"btn_type":"momentary"}]}`, "SHSW-1", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := converter.FromAPIConfig([]byte(tc.raw), tc.model)
			require.NoError(t, err)
			if tc.want == 0 {
				assert.Nil(t, got.Input)
				return
			}
			require.NotNil(t, got.Input)
			assert.Len(t, got.Input.Inputs, tc.want)
		})
	}
}

// The Duo and Vintage pages list transition (0..5000 ms) as a parameter of POST /settings, beside a
// night mode. It is written and read back like the Dimmer's, and nothing else of the Dimmer's group
// reaches them: hasDimmerSettings stays the Dimmers' and the round trip refuses the rest.
func TestApplyWhiteBulbTransition_IsWrittenAndReadBack(t *testing.T) {
	for _, model := range whiteBulbModels {
		t.Run(model, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "lamp")
			setDesired(t, repo, whiteBulbNight(DimmingConfig{TransitionTime: IntPtr(800), NightModeEnabled: BoolPtr(true)}))
			dev := newWhiteBulb()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
			require.Len(t, dev.posted, 1)
			assert.Equal(t, map[string]interface{}{"transition": float64(800)}, dev.posted[0])
			assert.Len(t, dev.nightPosts, 1, "the night mode keeps its own endpoint")
		})
	}
}

func TestApplyWhiteBulbTransition_OutOfRangeIsRefusedBeforeAnyWrite(t *testing.T) {
	for _, model := range whiteBulbModels {
		for _, value := range []int{-1, 5001} {
			t.Run(model, func(t *testing.T) {
				svc, repo := newServiceWithDesiredName(t, "lamp")
				setDesired(t, repo, whiteBulbNight(DimmingConfig{TransitionTime: IntPtr(value)}))
				dev := newWhiteBulb()

				_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

				require.ErrorIs(t, err, ErrUnsupportedSettings)
				assert.Contains(t, err.Error(), "dimming.transition")
				dev.nothingWritten(t)
			})
		}
	}
}

// A configuration imported from a Duo or Vintage that reports a transition applies again.
func TestApplyWhiteBulbTransition_ImportedValueAppliesAgain(t *testing.T) {
	for _, model := range whiteBulbModels {
		t.Run(model, func(t *testing.T) {
			dev := newWhiteBulb()
			dev.settings["transition"] = float64(1000)
			raw, err := json.Marshal(dev.settings)
			require.NoError(t, err)
			imported, err := NewGen1Converter(nil).FromAPIConfig(raw, model)
			require.NoError(t, err)
			require.NotNil(t, imported.Dimming)
			require.NotNil(t, imported.Dimming.TransitionTime, "the device's transition is read")
			assert.Equal(t, 1000, *imported.Dimming.TransitionTime)

			edited := *imported.Dimming
			edited.TransitionTime = IntPtr(2000)
			svc, repo := newServiceWithDesiredName(t, "lamp")
			setDesired(t, repo, whiteBulbNight(edited))

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
			assert.Equal(t, []map[string]interface{}{{"transition": float64(2000)}}, dev.posted)
		})
	}
}

// Only transition is added: the Duo and Vintage are not Dimmers, and the Bulb and the RGBW2 gain nothing.
func TestGen1Converter_TransitionOfTheWhiteBulbsOnly(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, model := range whiteBulbModels {
		assert.False(t, converter.hasDimmerSettings(model), model)
		assert.True(t, converter.hasTransition(model), model)
	}
	for _, model := range []string{"SHDM-1", "SHDM-2"} {
		assert.True(t, converter.hasTransition(model), model)
	}
	for _, model := range []string{"SHBLB-1", "SHRGBW2", "SHSW-1", ""} {
		assert.False(t, converter.hasTransition(model), model)
	}

	raw := `{"transition":500,"fade_rate":3,"min_brightness":10,"zcross_debounce":100,"warm_up":{"enabled":true},"night_mode":{"enabled":true}}`
	for _, model := range whiteBulbModels {
		got, err := converter.FromAPIConfig([]byte(raw), model)
		require.NoError(t, err)
		require.NotNil(t, got.Dimming, model)
		assert.Equal(t, DimmingConfig{TransitionTime: IntPtr(500), NightModeEnabled: BoolPtr(true)}, *got.Dimming, model)
	}
}

func TestGen1Converter_FadeRateIsKeptOnlyInTheRange(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, tc := range []struct {
		raw  string
		want *int
	}{
		{`{"fade_rate":1}`, IntPtr(1)},
		{`{"fade_rate":5}`, IntPtr(5)},
		{`{"fade_rate":0}`, nil},
		{`{"fade_rate":6}`, nil},
	} {
		got, err := converter.FromAPIConfig([]byte(tc.raw), "SHDM-2")
		require.NoError(t, err)
		if tc.want == nil {
			assert.True(t, got.Dimming == nil || got.Dimming.FadeRate == nil, tc.raw)
			continue
		}
		require.NotNil(t, got.Dimming, tc.raw)
		assert.Equal(t, tc.want, got.Dimming.FadeRate, tc.raw)
	}
}

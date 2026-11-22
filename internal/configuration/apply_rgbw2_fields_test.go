package configuration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The color channel's own keys: name, effect (0 to 4), transition (0 to 5000 ms) and btn_reverse. They
// sit in a second file because the first one compiles on code that has none of these fields.
func TestApplyDesiredConfig_Gen1RGBW2WritesEffectTransitionNameAndReverse(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(SingleLightConfig{
		Name: StringPtr("strip"), Effect: IntPtr(3), Transition: IntPtr(1500), ButtonReverse: BoolPtr(true),
	}))
	dev := newRGBW2Device("color")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.colorWrites, 1)
	assert.Equal(t, map[string]interface{}{
		"name": "strip", "effect": float64(3), "transition": float64(1500), "btn_reverse": true,
	}, dev.colorWrites[0].form)
}

// A written btn_reverse bool is a read of 0 or 1: the read back must not call that a mismatch.
func TestApplyDesiredConfig_Gen1RGBW2ButtonReverseMatchesANumberRead(t *testing.T) {
	for name, read := range map[string]interface{}{"number 1": float64(1), "bool": true} {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "rgbw2")
			setDesired(t, repo, rgbw2Lights(SingleLightConfig{ButtonReverse: BoolPtr(true)}))
			dev := &rgbw2ReadsBack{rgbw2Device: newRGBW2Device("color"), read: read}

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied)
		})
	}
}

// rgbw2ReadsBack answers the color read with btn_reverse in the given JSON shape.
type rgbw2ReadsBack struct {
	*rgbw2Device
	read interface{}
}

func (d *rgbw2ReadsBack) GetColorSettings(_ context.Context, _ int) (map[string]interface{}, error) {
	return map[string]interface{}{"btn_reverse": d.read}, nil
}

// effect is a color-mode key: a white channel has none. transition and name are on both modes.
func TestApplyDesiredConfig_Gen1RGBW2RefusesEffectOnWhite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(SingleLightConfig{Transition: IntPtr(500)}, SingleLightConfig{Effect: IntPtr(1)}))
	dev := newRGBW2Device("white")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "light.lights[1].effect")
	dev.nothingWritten(t)
}

func TestApplyDesiredConfig_Gen1RGBW2WritesTransitionAndNameOnWhite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(SingleLightConfig{Name: StringPtr("a"), Transition: IntPtr(500)}, SingleLightConfig{Transition: IntPtr(0)}))
	dev := newRGBW2Device("white")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.NoError(t, err)
	require.Len(t, dev.whiteWrites, 2)
	assert.Equal(t, map[string]interface{}{"name": "a", "transition": float64(500)}, dev.whiteWrites[0].form)
	assert.Equal(t, map[string]interface{}{"transition": float64(0)}, dev.whiteWrites[1].form)
}

// btn_reverse is shared by the four white channels and visible only on channel 0.
func TestApplyDesiredConfig_Gen1RGBW2RefusesButtonReverseOnWhite1To3(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "rgbw2")
	setDesired(t, repo, rgbw2Lights(SingleLightConfig{}, SingleLightConfig{ButtonReverse: BoolPtr(true)}))
	dev := newRGBW2Device("white")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "light.lights[1].btn_reverse")
	dev.nothingWritten(t)
}

// Effect 0 to 4 and transition 0 to 5000 are the page's ranges; the edges just outside are refused.
func TestApplyDesiredConfig_Gen1RGBW2RefusesEffectAndTransitionOutOfRange(t *testing.T) {
	cases := map[string]struct {
		light SingleLightConfig
		path  string
	}{
		"effect 5":        {SingleLightConfig{Effect: IntPtr(5)}, "light.lights[0].effect"},
		"effect -1":       {SingleLightConfig{Effect: IntPtr(-1)}, "light.lights[0].effect"},
		"transition 5001": {SingleLightConfig{Transition: IntPtr(5001)}, "light.lights[0].transition"},
		"transition -1":   {SingleLightConfig{Transition: IntPtr(-1)}, "light.lights[0].transition"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "rgbw2")
			setDesired(t, repo, rgbw2Lights(tc.light))
			dev := newRGBW2Device("color")

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHRGBW2")

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), tc.path)
			dev.nothingWritten(t)
		})
	}
}

// The Dimmer has none of the RGBW2's own keys: effect and name stay refused there.
func TestApplyDesiredConfig_DimmerStillRefusesRGBW2Keys(t *testing.T) {
	for name, light := range map[string]SingleLightConfig{
		"effect": {Effect: IntPtr(1)},
		"name":   {Name: StringPtr("x")},
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "dimmer")
			setDesired(t, repo, rgbw2Lights(light))
			dev := newDimmerDevice()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Equal(t, 0, dev.setCalls)
		})
	}
}

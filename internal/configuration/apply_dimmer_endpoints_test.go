package configuration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nightDimmer is a Dimmer that also takes POST on /settings/night_mode and /settings/warm_up. A
// write is stored in the top-level "night_mode" and "warm_up" objects of GET /settings, where a
// Dimmer reports them.
type nightDimmer struct {
	*dimmerDevice
	nightPosts []map[string]interface{}
	warmPosts  []map[string]interface{}
}

func newNightDimmer() *nightDimmer { return &nightDimmer{dimmerDevice: newDimmerDevice()} }

func (d *nightDimmer) store(object string, form map[string]interface{}) {
	doc, _ := d.settings[object].(map[string]interface{})
	if doc == nil {
		doc = map[string]interface{}{}
	}
	for k, v := range form {
		// A device returns JSON numbers as float64.
		if n, ok := v.(int); ok {
			v = float64(n)
		}
		doc[k] = v
	}
	d.settings[object] = doc
}

func (d *nightDimmer) SetNightMode(_ context.Context, form map[string]interface{}) error {
	d.nightPosts = append(d.nightPosts, form)
	d.store("night_mode", form)
	return nil
}

func (d *nightDimmer) SetWarmUp(_ context.Context, form map[string]interface{}) error {
	d.warmPosts = append(d.warmPosts, form)
	d.store("warm_up", form)
	return nil
}

// Night mode is not a parameter of POST /settings: its four keys go to POST /settings/night_mode and
// are read back from the night_mode object of GET /settings.
func TestApplyDimmerNightMode_IsWrittenAndReadBack(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{
		NightModeEnabled: BoolPtr(true), NightModeStart: StringPtr("22:00"), NightModeEnd: StringPtr("06:00"),
		NightModeBrightness: IntPtr(20),
	}))
	dev := newNightDimmer()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.nightPosts, 1)
	assert.Equal(t, map[string]interface{}{
		"enabled": true, "start_time": "22:00", "end_time": "06:00", "brightness": float64(20),
	}, dev.nightPosts[0])
	assert.Empty(t, dev.warmPosts)
	assert.Empty(t, dev.posted, "night mode is not a parameter of POST /settings")
}

// Every bound of the page is refused before any write, and each edge is accepted.
func TestUnsupportedNightWarmup(t *testing.T) {
	on, off := BoolPtr(true), BoolPtr(false)
	for _, tc := range []struct {
		name string
		d    DimmingConfig
		want []string
	}{
		{"night brightness 1", DimmingConfig{NightModeBrightness: IntPtr(1)}, nil},
		{"night brightness 100", DimmingConfig{NightModeBrightness: IntPtr(100)}, nil},
		{"night brightness 0", DimmingConfig{NightModeBrightness: IntPtr(0)}, []string{"dimming.night_mode_brightness"}},
		{"night brightness 101", DimmingConfig{NightModeBrightness: IntPtr(101)}, []string{"dimming.night_mode_brightness"}},
		{"time 00:00", DimmingConfig{NightModeStart: StringPtr("00:00")}, nil},
		{"time 23:59", DimmingConfig{NightModeEnd: StringPtr("23:59")}, nil},
		{"time 24:00", DimmingConfig{NightModeStart: StringPtr("24:00")}, []string{"dimming.night_mode_start"}},
		{"time 12:60", DimmingConfig{NightModeEnd: StringPtr("12:60")}, []string{"dimming.night_mode_end"}},
		{"time without minutes", DimmingConfig{NightModeStart: StringPtr("22")}, []string{"dimming.night_mode_start"}},
		{"time with one-digit hour", DimmingConfig{NightModeStart: StringPtr("6:00")}, []string{"dimming.night_mode_start"}},
		{"time empty", DimmingConfig{NightModeEnd: StringPtr("")}, []string{"dimming.night_mode_end"}},
		{"window across midnight", DimmingConfig{NightModeStart: StringPtr("22:00"), NightModeEnd: StringPtr("06:00")}, nil},
		{"warm-up brightness 10", DimmingConfig{WarmupEnabled: on, WarmupBrightness: IntPtr(10)}, nil},
		{"warm-up brightness 9", DimmingConfig{WarmupEnabled: on, WarmupBrightness: IntPtr(9)}, []string{"dimming.warmup_brightness"}},
		{"warm-up brightness 101", DimmingConfig{WarmupBrightness: IntPtr(101)}, []string{"dimming.warmup_brightness"}},
		{"warm-up time 20", DimmingConfig{WarmupTime: IntPtr(20)}, nil},
		{"warm-up time 200", DimmingConfig{WarmupTime: IntPtr(200)}, nil},
		{"warm-up time 19", DimmingConfig{WarmupTime: IntPtr(19)}, []string{"dimming.warmup_time"}},
		{"warm-up time 201 (the changelog's 1000 ms is unconfirmed)", DimmingConfig{WarmupTime: IntPtr(201)}, []string{"dimming.warmup_time"}},
		{"enabled true, brightness 0", DimmingConfig{WarmupEnabled: on, WarmupBrightness: IntPtr(0)}, []string{"dimming.warmup_brightness"}},
		{"enabled off, the page's example", DimmingConfig{WarmupEnabled: off, WarmupBrightness: IntPtr(0), WarmupTime: IntPtr(20)}, nil},
		{"enabled off, time out of range", DimmingConfig{WarmupEnabled: off, WarmupTime: IntPtr(5000)}, nil},
		{"enabled off does not excuse night mode", DimmingConfig{WarmupEnabled: off, NightModeBrightness: IntPtr(0)}, []string{"dimming.night_mode_brightness"}},
		{"unset", DimmingConfig{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.ElementsMatch(t, tc.want, unsupportedNightWarmup(&tc.d, "SHDM-1"))
		})
	}
}

func TestApplyDimmerNightMode_OutOfRangeIsRefusedBeforeAnyWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{
		NightModeEnabled: BoolPtr(true), NightModeStart: StringPtr("25:00"), NightModeBrightness: IntPtr(101), FadeRate: IntPtr(3),
	}))
	dev := newNightDimmer()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "dimming.night_mode_start")
	assert.Contains(t, err.Error(), "dimming.night_mode_brightness")
	assert.Equal(t, 0, dev.setCalls)
	assert.Empty(t, dev.nightPosts)
}

// The warm-up's enabled flag and the keys the page confirms are written to /settings/warm_up and
// read back.
func TestApplyDimmerWarmup_IsWrittenAndReadBack(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{
		WarmupEnabled: BoolPtr(true), WarmupBrightness: IntPtr(50), WarmupTime: IntPtr(100),
	}))
	dev := newNightDimmer()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-2")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.warmPosts, 1)
	assert.Equal(t, map[string]interface{}{"enabled": true, "brightness": float64(50), "time": float64(100)}, dev.warmPosts[0])
	assert.Empty(t, dev.nightPosts)
	assert.Empty(t, dev.posted)
	assert.False(t, result.ApplyResult.RequiresReboot, "the page names no reboot after either write")
	assert.Equal(t, 3, result.ApplyResult.AppliedCount, "three keys written and read back")
}

// The page's own warm-up example is enabled 0, brightness 0: a config imported from a fresh Dimmer
// applies, and only the enabled flag is written.
func TestApplyDimmerWarmup_SwitchedOffWritesOnlyEnabled(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{
		WarmupEnabled: BoolPtr(false), WarmupBrightness: IntPtr(0), WarmupTime: IntPtr(20),
	}))
	dev := newNightDimmer()
	// The device keeps its own brightness, which is no drift for a value apply never sent.
	dev.settings["warm_up"] = map[string]interface{}{"enabled": true, "brightness": float64(50), "time": float64(100)}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	require.Len(t, dev.warmPosts, 1)
	assert.Equal(t, map[string]interface{}{"enabled": false}, dev.warmPosts[0])
	assert.Equal(t, 1, result.ApplyResult.SettingsCount, "only the enabled flag was sent")
	assert.Equal(t, 1, result.ApplyResult.AppliedCount)
}

func TestApplyDimmerWarmup_EnabledWithBrightnessZeroIsRefusedBeforeAnyWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{WarmupEnabled: BoolPtr(true), WarmupBrightness: IntPtr(0), WarmupTime: IntPtr(20)}))
	dev := newNightDimmer()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "dimming.warmup_brightness")
	assert.Empty(t, dev.warmPosts)
}

// The page's own warm-up example, enabled 0 with brightness 0, passes the schema check that guards
// every template and override write; the night mode bounds are the schema's.
func TestValidateConfigBody_WarmupOffExampleAndNightBounds(t *testing.T) {
	example := map[string]interface{}{"dimming": map[string]interface{}{
		"warmup_enabled": false, "warmup_brightness": float64(0), "warmup_time": float64(20),
		"night_mode_brightness": float64(1), "night_mode_start": "22:00", "night_mode_end": "06:00",
	}}
	assert.Empty(t, ValidateConfigBody("SHDM-1", example))

	bad := map[string]interface{}{"dimming": map[string]interface{}{"night_mode_brightness": float64(0), "night_mode_start": "24:00"}}
	assert.Len(t, ValidateConfigBody("SHDM-2", bad), 2)
}

// A device that answers 200 and keeps the old value fails at the path of the key.
type forgetfulNightDimmer struct{ *nightDimmer }

func (d forgetfulNightDimmer) SetNightMode(ctx context.Context, form map[string]interface{}) error {
	kept := map[string]interface{}{}
	for k, v := range form {
		if k != "start_time" {
			kept[k] = v
		}
	}
	return d.nightDimmer.SetNightMode(ctx, kept)
}

func TestApplyDimmerNightMode_ADeviceThatKeepsTheOldValueFailsAtItsPath(t *testing.T) {
	dev := forgetfulNightDimmer{newNightDimmer()}
	dev.settings["night_mode"] = map[string]interface{}{"start_time": "21:00"}
	config := dimmingConfig(DimmingConfig{NightModeStart: StringPtr("22:00"), NightModeBrightness: IntPtr(20)})

	result, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), dev, config, "SHDM-1")

	require.NoError(t, err)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "night_mode.start_time", result.Failures[0].Path)
	assert.Equal(t, 1, result.AppliedCount, "the brightness was kept")
	assert.Equal(t, 1, result.FailedCount)
}

// Only a Dimmer takes the two endpoints: another model is refused before any write.
func TestApplyDimmerNightMode_IsForTheDimmerOnly(t *testing.T) {
	for _, model := range []string{"SHPLG-S", "SHSW-1", "SHRGBW2", "SHUNKNOWN-9"} {
		t.Run(model, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "other")
			setDesired(t, repo, dimmingConfig(DimmingConfig{NightModeBrightness: IntPtr(20), WarmupEnabled: BoolPtr(true)}))
			dev := newNightDimmer()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, model)

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Nil(t, result)
			assert.Equal(t, 0, dev.setCalls)
			assert.Empty(t, dev.nightPosts)
			assert.Empty(t, dev.warmPosts)
		})
	}
}

// A client without the two endpoints fails the section instead of dropping the objects.
func TestApplyDimmerNightMode_AClientWithoutTheEndpointsFailsTheSection(t *testing.T) {
	dev := newDimmerDevice()
	config := dimmingConfig(DimmingConfig{NightModeBrightness: IntPtr(20)})

	result, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), dev, config, "SHDM-1")

	// A section failure is a failure in the result, as for a light, never an error that hides the rest.
	require.NoError(t, err)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "dimming", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, errNoDimmerEndpointsClient.Error())
	assert.Equal(t, 1, result.FailedCount)
}

// Neither object is a parameter of POST /settings.
func TestGen1Converter_NightModeAndWarmupAreNotSettingsParameters(t *testing.T) {
	apiJSON, err := NewGen1Converter(nil).ToAPIConfig(&DeviceConfiguration{Dimming: &DimmingConfig{
		FadeRate: IntPtr(3), NightModeEnabled: BoolPtr(true), WarmupEnabled: BoolPtr(false),
	}}, "SHDM-1")
	require.NoError(t, err)
	var gen1 map[string]interface{}
	require.NoError(t, json.Unmarshal(apiJSON, &gen1))
	assert.Contains(t, gen1, "night_mode", "the converted document carries it for the endpoint")

	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, dimmingConfig(DimmingConfig{FadeRate: IntPtr(3), NightModeEnabled: BoolPtr(true)}))
	dev := newNightDimmer()
	_, err = svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")
	require.NoError(t, err)
	require.Len(t, dev.posted, 1)
	assert.NotContains(t, dev.posted[0], "night_mode")
	assert.NotContains(t, dev.posted[0], "warm_up")
}

// The converter reads both top-level objects, a flag as bool or 0/1, and stores an out-of-range value
// as the device reports it. The copy of night_mode inside a light is not read.
func TestGen1Converter_ReadsNightModeAndWarmup(t *testing.T) {
	got, err := NewGen1Converter(nil).FromAPIConfig([]byte(`{
		"night_mode":{"enabled":true,"start_time":"22:00","end_time":"06:00","brightness":20},
		"warm_up":{"enabled":0,"brightness":0,"time":20},
		"lights":[{"night_mode":{"enabled":true,"start_time":"01:00","end_time":"02:00","brightness":99}}]}`), "SHDM-1")
	require.NoError(t, err)
	require.NotNil(t, got.Dimming)
	d := got.Dimming
	assert.Equal(t, BoolPtr(true), d.NightModeEnabled)
	assert.Equal(t, StringPtr("22:00"), d.NightModeStart)
	assert.Equal(t, StringPtr("06:00"), d.NightModeEnd)
	assert.Equal(t, IntPtr(20), d.NightModeBrightness)
	assert.Equal(t, BoolPtr(false), d.WarmupEnabled, "0 is a switched-off flag")
	assert.Equal(t, IntPtr(0), d.WarmupBrightness, "stored as reported")
	assert.Equal(t, IntPtr(20), d.WarmupTime)

	other, err := NewGen1Converter(nil).FromAPIConfig([]byte(`{"night_mode":{"enabled":true},"warm_up":{"time":50}}`), "SHSW-1")
	require.NoError(t, err)
	assert.Nil(t, other.Dimming, "a model that is not a Dimmer reads no dimming section")
}

// Verify reports a night-mode difference at its path.
func TestVerifyConfig_NightModeDifferenceIsReportedAtItsPath(t *testing.T) {
	desired := dimmingConfig(DimmingConfig{NightModeStart: StringPtr("22:00"), NightModeBrightness: IntPtr(20)})

	matched := verifyRelay(t, "SHDM-1", `{"night_mode":{"enabled":false,"start_time":"22:00","end_time":"06:00","brightness":20}}`, desired)
	assert.True(t, matched.Match, "differences: %+v", matched.Differences)

	drifted := verifyRelay(t, "SHDM-1", `{"night_mode":{"start_time":"21:00","brightness":20}}`, desired)
	assert.False(t, drifted.Match)
	var paths []string
	for _, difference := range drifted.Differences {
		paths = append(paths, difference.Path)
	}
	assert.Contains(t, strings.Join(paths, " "), "night_mode_start")
}

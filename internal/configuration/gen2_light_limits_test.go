package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGen2DeviceWithLightLimits is the dimmer of fakeGen2DeviceWithLight with the limit keys and the
// input mode that the Light page lists as present "if applicable".
func fakeGen2DeviceWithLightLimits() *fakeGen2Device {
	d := fakeGen2DeviceWithLight()
	light := d.doc["light:0"].(map[string]interface{})
	light["power_limit"], light["voltage_limit"], light["undervoltage_limit"] = 200.0, 280.0, 180.0
	light["current_limit"], light["in_mode"] = 1.5, "follow"
	return d
}

func limitsDesired(light SingleLightConfig) *DeviceConfiguration {
	return &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{light}}}
}

// The four limits and the input mode are written with one Light.SetConfig and read back. Before this
// slice every one of them was refused, because the Gen2 light converter had no key for it.
func TestApplyDesiredConfig_Gen2LightWritesLimitsAndInMode(t *testing.T) {
	svc, repo := newGen2Service(t, limitsDesired(SingleLightConfig{
		ID: 0, MaxPower: IntPtr(150), VoltageLimit: IntPtr(250), UndervoltageLimit: IntPtr(190),
		CurrentLimit: Float64Ptr(2.5), InMode: StringPtr("detached"),
	}))
	dev := fakeGen2DeviceWithLightLimits()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "the limits and the mode are read back as stored")
	assert.True(t, repo.devices[1].ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "Light.SetConfig", dev.calls[0].method)
	assert.Equal(t, map[string]interface{}{
		"power_limit": 150.0, "voltage_limit": 250.0, "undervoltage_limit": 190.0, "current_limit": 2.5, "in_mode": "detached",
	}, dev.calls[0].config)
}

// The Light page gives no range and no pair rule, so only a negative limit is refused, and an
// undervoltage limit above the voltage limit is written: the device decides.
func TestApplyDesiredConfig_Gen2LightLimitsRefuseOnlyANegativeValue(t *testing.T) {
	negative := map[string]SingleLightConfig{
		"power":        {MaxPower: IntPtr(-1)},
		"voltage":      {VoltageLimit: IntPtr(-1)},
		"undervoltage": {UndervoltageLimit: IntPtr(-1)},
		"current":      {CurrentLimit: Float64Ptr(-0.1)},
	}
	paths := map[string]string{"power": "max_power", "voltage": "voltage_limit", "undervoltage": "undervoltage_limit", "current": "current_limit"}
	for name, light := range negative {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, limitsDesired(light))
			dev := &coverReads{fakeGen2Device: fakeGen2DeviceWithLightLimits()}

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), "light.lights.0."+paths[name])
			assert.Empty(t, dev.calls)
			assert.Equal(t, 0, dev.reads, "refused before the device was read")
		})
	}

	t.Run("undervoltage above voltage is written", func(t *testing.T) {
		svc, _ := newGen2Service(t, limitsDesired(SingleLightConfig{VoltageLimit: IntPtr(100), UndervoltageLimit: IntPtr(200)}))
		dev := fakeGen2DeviceWithLightLimits()

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

		require.NoError(t, err)
		require.Len(t, dev.calls, 1)
		assert.Equal(t, map[string]interface{}{"voltage_limit": 100.0, "undervoltage_limit": 200.0}, dev.calls[0].config)
	})
}

// The six words of the page are written as they are; any other word is refused before any write.
func TestApplyDesiredConfig_Gen2LightInModeWords(t *testing.T) {
	for _, word := range []string{"follow", "flip", "activate", "detached", "dim", "dual_dim"} {
		t.Run(word, func(t *testing.T) {
			svc, _ := newGen2Service(t, limitsDesired(SingleLightConfig{InMode: StringPtr(word)}))
			dev := fakeGen2DeviceWithLightLimits()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied)
			require.Len(t, dev.calls, 1)
			assert.Equal(t, map[string]interface{}{"in_mode": word}, dev.calls[0].config)
		})
	}
	for _, word := range []string{"momentary", "cycle", "toggle", "", "Follow"} {
		t.Run("refused "+word, func(t *testing.T) {
			svc, _ := newGen2Service(t, limitsDesired(SingleLightConfig{InMode: StringPtr(word)}))
			dev := &coverReads{fakeGen2Device: fakeGen2DeviceWithLightLimits()}

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), "light.lights.0.in_mode")
			assert.Empty(t, dev.calls)
			assert.Equal(t, 0, dev.reads)
		})
	}
}

// A light of a device that does not report a limit or the input mode is refused before any write,
// with the device key beside it.
func TestApplyDesiredConfig_Gen2LightRefusesALimitTheDeviceDoesNotReport(t *testing.T) {
	cases := map[string]struct {
		light SingleLightConfig
		key   string
	}{
		"power limit":   {SingleLightConfig{MaxPower: IntPtr(100)}, "light:0.power_limit"},
		"current limit": {SingleLightConfig{CurrentLimit: Float64Ptr(1)}, "light:0.current_limit"},
		"input mode":    {SingleLightConfig{InMode: StringPtr("follow")}, "light:0.in_mode"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, limitsDesired(tc.light))
			dev := fakeGen2DeviceWithLight()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-00100WW")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.key)
			assert.Empty(t, dev.calls)
		})
	}
}

// A null, a fraction in a whole-number limit and a missing key are read as unset, never as a value.
func TestGen2Converter_LightLimitsReadBackNeverInventAValue(t *testing.T) {
	raw, err := json.Marshal(map[string]interface{}{"light:0": map[string]interface{}{
		"id": 0.0, "power_limit": nil, "voltage_limit": 250.5, "current_limit": 2.5, "in_mode": "dual_dim",
	}})
	require.NoError(t, err)

	config, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNDM-00100WW")

	require.NoError(t, err)
	require.NotNil(t, config.Light)
	light := config.Light.Lights[0]
	assert.Nil(t, light.MaxPower, "null reads as unset")
	assert.Nil(t, light.VoltageLimit, "a fraction of a volt reads as unset")
	assert.Nil(t, light.UndervoltageLimit)
	assert.Equal(t, Float64Ptr(2.5), light.CurrentLimit)
	assert.Equal(t, StringPtr("dual_dim"), light.InMode)
}

// None of the five keys has a Gen1 counterpart on the light, so the Dimmer refuses them.
func TestGen1DimmerRefusesTheGen2LightLimitsAndMode(t *testing.T) {
	for name, light := range map[string]SingleLightConfig{
		"power": {MaxPower: IntPtr(100)}, "voltage": {VoltageLimit: IntPtr(250)}, "undervoltage": {UndervoltageLimit: IntPtr(190)},
		"current": {CurrentLimit: Float64Ptr(1)}, "mode": {InMode: StringPtr("follow")},
	} {
		t.Run(name, func(t *testing.T) {
			unsupported, err := unsupportedApplySettings(NewGen1Converter(nil), limitsDesired(light), "SHDM-1", true)

			require.NoError(t, err)
			assert.NotEmpty(t, unsupported)
		})
	}
}

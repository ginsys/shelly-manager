package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func floatPtr(v float64) *float64 { return &v }

func relayOf(r SingleRelayConfig) *DeviceConfiguration {
	return &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{r}}}
}

// A desired max_power is the overpower threshold in watts. On a Gen2 switch it is power_limit.
func TestApplyDesiredConfig_Gen2WritesThePowerLimit(t *testing.T) {
	svc, _ := newGen2Service(t, relayOf(SingleRelayConfig{ID: 0, MaxPower: IntPtr(1500)}))
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "power_limit is read back as max_power")
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "Switch.SetConfig", dev.calls[0].method)
	assert.Equal(t, map[string]interface{}{"power_limit": float64(1500)}, dev.calls[0].config, "only the keys set")
}

func TestApplyDesiredConfig_Gen2WritesTheOtherSwitchLimits(t *testing.T) {
	desired := relayOf(SingleRelayConfig{
		ID: 0, VoltageLimit: IntPtr(250), UndervoltageLimit: IntPtr(200), CurrentLimit: floatPtr(10.5),
		InLocked: BoolPtr(true), AutorecoverVoltageErrors: BoolPtr(true), PowerRestoreDelay: IntPtr(30),
	})
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{
		"voltage_limit": float64(250), "undervoltage_limit": float64(200), "current_limit": 10.5,
		"in_locked": true, "autorecover_voltage_errors": true,
		"power_restore_delay_enable": true, "power_restore_delay": float64(30),
	}, dev.calls[0].config)
}

// Zero seconds is off: only the enable key is written, as for the auto timers.
func TestApplyDesiredConfig_Gen2ZeroSecondsSwitchesThePowerRestoreDelayOff(t *testing.T) {
	svc, _ := newGen2Service(t, relayOf(SingleRelayConfig{ID: 0, PowerRestoreDelay: IntPtr(0)}))
	dev := fakeGen2DeviceWithChannels()
	delete(dev.doc["switch:0"].(map[string]interface{}), "power_restore_delay")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{"power_restore_delay_enable": false}, dev.calls[0].config)
}

// A null, a missing key and a value with a fraction have no stored value, so a desired number never
// matches them.
func TestGen2Converter_ReadsNoLimitForAValueItCannotStore(t *testing.T) {
	read := func(section map[string]interface{}) SingleRelayConfig {
		raw, err := json.Marshal(map[string]interface{}{"switch:0": section})
		require.NoError(t, err)
		got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")
		require.NoError(t, err)
		require.NotNil(t, got.Relay)
		return got.Relay.Relays[0]
	}

	whole := read(map[string]interface{}{"id": 0, "power_limit": 1500.0, "voltage_limit": 250.0, "undervoltage_limit": 0.0, "current_limit": 10.5})
	require.NotNil(t, whole.MaxPower)
	assert.Equal(t, 1500, *whole.MaxPower)
	assert.Equal(t, 250, *whole.VoltageLimit)
	assert.Equal(t, 0, *whole.UndervoltageLimit)
	assert.Equal(t, 10.5, *whole.CurrentLimit)

	unset := read(map[string]interface{}{"id": 0, "power_limit": nil, "voltage_limit": 250.5, "undervoltage_limit": nil, "current_limit": nil, "in_locked": nil})
	assert.Nil(t, unset.MaxPower, "null is the device default")
	assert.Nil(t, unset.VoltageLimit, "a fraction of a volt has no stored form")
	assert.Nil(t, unset.UndervoltageLimit)
	assert.Nil(t, unset.CurrentLimit)
	assert.Nil(t, unset.InLocked)

	delays := map[string]map[string]interface{}{
		"off":              {"power_restore_delay_enable": false, "power_restore_delay": 60.0},
		"on":               {"power_restore_delay_enable": true, "power_restore_delay": 45.0},
		"on with null":     {"power_restore_delay_enable": true, "power_restore_delay": nil},
		"on with fraction": {"power_restore_delay_enable": true, "power_restore_delay": 4.5},
		"on above 600":     {"power_restore_delay_enable": true, "power_restore_delay": 601.0},
		"no enable key":    {"power_restore_delay": 45.0},
	}
	want := map[string]*int{"off": IntPtr(0), "on": IntPtr(45)}
	for name, section := range delays {
		section["id"] = 0
		assert.Equal(t, want[name], read(section).PowerRestoreDelay, name)
	}
}

// A limit that the device refuses is a failed row, never an error of the apply, and the write to the
// next channel still goes out.
func TestApplyDesiredConfig_Gen2FailsALimitTheDeviceRefuses(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, MaxPower: IntPtr(5000)},
		{ID: 1, Name: StringPtr("lamp")},
	}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()
	dev.limitMax = map[string]float64{"power_limit": 3680}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err, "a refusal of the device is not a transport error")
	assert.False(t, result.ConfigApplied)
	assert.False(t, result.ApplyResult.Success)
	assert.Equal(t, 1, result.ApplyResult.FailedCount)
	require.Len(t, result.ApplyResult.Failures, 1)
	assert.Equal(t, "switch:0.power_limit", result.ApplyResult.Failures[0].Path)
	assert.Contains(t, result.ApplyResult.Failures[0].Error, "power_limit")
	require.Len(t, dev.calls, 2, "switch 1 is written after the refusal of switch 0")
	assert.Equal(t, "lamp", dev.doc["switch:1"].(map[string]interface{})["name"])
}

func TestApplyDesiredConfig_Gen2WritesInputEnableAndFactoryReset(t *testing.T) {
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Enable: BoolPtr(false), FactoryReset: BoolPtr(true)}}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "Input.SetConfig", dev.calls[0].method)
	assert.Equal(t, map[string]interface{}{"enable": false, "factory_reset": true}, dev.calls[0].config)
}

// Gen1 keeps its meaning: max_power is the Gen1 key, and a Gen2 only key is refused on a Gen1 model.
func TestGen1KeepsMaxPowerAndRefusesTheGen2Limits(t *testing.T) {
	raw, err := NewGen1Converter(nil).ToAPIConfig(relayOf(SingleRelayConfig{ID: 0, MaxPower: IntPtr(1500)}), "SHSW-1")
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"max_power":1500`)
	assert.NotContains(t, string(raw), "power_limit")

	for name, relay := range map[string]SingleRelayConfig{
		"current_limit":              {ID: 0, CurrentLimit: floatPtr(5)},
		"voltage_limit":              {ID: 0, VoltageLimit: IntPtr(250)},
		"undervoltage_limit":         {ID: 0, UndervoltageLimit: IntPtr(180)},
		"in_locked":                  {ID: 0, InLocked: BoolPtr(true)},
		"autorecover_voltage_errors": {ID: 0, AutorecoverVoltageErrors: BoolPtr(true)},
		"power_restore_delay":        {ID: 0, PowerRestoreDelay: IntPtr(30)},
	} {
		refused, refusedErr := unsupportedApplySettings(NewGen1Converter(nil), relayOf(relay), "SHSW-1", true)
		require.NoError(t, refusedErr, name)
		assert.Contains(t, refused, "relay.relays.0."+name)
	}
	unsupported, err := unsupportedApplySettings(NewGen1Converter(nil),
		&DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Enable: BoolPtr(true), FactoryReset: BoolPtr(true)}}}}, "SHSW-1", true)
	require.NoError(t, err)
	assert.Contains(t, unsupported, "input.inputs.0.enable")
	assert.Contains(t, unsupported, "input.inputs.0.factory_reset")
}

// A setting the device cannot take, or the live document does not carry, is refused before the first
// write. A key that is present with null is reported by the device: it is written.
func TestApplyDesiredConfig_Gen2RefusesALimitBeforeAnyWrite(t *testing.T) {
	missing := func(section, key string) func(*fakeGen2Device) {
		return func(d *fakeGen2Device) { delete(d.doc[section].(map[string]interface{}), key) }
	}
	cases := map[string]struct {
		desired *DeviceConfiguration
		prepare func(*fakeGen2Device)
		want    []string
	}{
		"power limit not reported": {
			relayOf(SingleRelayConfig{ID: 0, MaxPower: IntPtr(1500)}), missing("switch:0", "power_limit"),
			[]string{"relay.relays.0.max_power (power_limit)"},
		},
		"voltage limit not reported": {
			relayOf(SingleRelayConfig{ID: 0, VoltageLimit: IntPtr(250)}), missing("switch:0", "voltage_limit"),
			[]string{"relay.relays.0.voltage_limit (voltage_limit)"},
		},
		"current limit not reported": {
			relayOf(SingleRelayConfig{ID: 0, CurrentLimit: floatPtr(5)}), missing("switch:0", "current_limit"),
			[]string{"relay.relays.0.current_limit (current_limit)"},
		},
		"in_locked not reported": {
			relayOf(SingleRelayConfig{ID: 0, InLocked: BoolPtr(true)}), missing("switch:0", "in_locked"),
			[]string{"relay.relays.0.in_locked (in_locked)"},
		},
		"restore delay with no enable key": {
			relayOf(SingleRelayConfig{ID: 0, PowerRestoreDelay: IntPtr(30)}), missing("switch:0", "power_restore_delay_enable"),
			[]string{"relay.relays.0.power_restore_delay (power_restore_delay_enable)"},
		},
		"restore delay above 0 with no delay key": {
			relayOf(SingleRelayConfig{ID: 0, PowerRestoreDelay: IntPtr(30)}), missing("switch:0", "power_restore_delay"),
			[]string{"relay.relays.0.power_restore_delay (power_restore_delay)"},
		},
		"input enable not reported": {
			&DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Enable: BoolPtr(false)}}}}, missing("input:0", "enable"),
			[]string{"input.inputs.0.enable (enable)"},
		},
		"factory reset not reported": {
			&DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, FactoryReset: BoolPtr(true)}}}}, missing("input:0", "factory_reset"),
			[]string{"input.inputs.0.factory_reset (factory_reset)"},
		},
		"channel the device does not have": {
			relayOf(SingleRelayConfig{ID: 3, MaxPower: IntPtr(1500)}), nil,
			[]string{"relay.relays.0.max_power (power_limit)"},
		},
		"device cannot be read": {
			relayOf(SingleRelayConfig{ID: 0, MaxPower: IntPtr(1500)}), func(d *fakeGen2Device) { d.getErr = errors.New("connection refused") },
			[]string{"connection refused"},
		},
		"negative limit": {
			relayOf(SingleRelayConfig{ID: 0, VoltageLimit: IntPtr(-1)}), nil, []string{"relay.relays.0.voltage_limit"},
		},
		"negative power limit": {
			relayOf(SingleRelayConfig{ID: 0, MaxPower: IntPtr(-5)}), nil, []string{"relay.relays.0.max_power"},
		},
		"negative current limit": {
			relayOf(SingleRelayConfig{ID: 0, CurrentLimit: floatPtr(-0.5)}), nil, []string{"relay.relays.0.current_limit"},
		},
		"restore delay above 600": {
			relayOf(SingleRelayConfig{ID: 0, PowerRestoreDelay: IntPtr(601)}), nil, []string{"relay.relays.0.power_restore_delay"},
		},
		"negative restore delay": {
			relayOf(SingleRelayConfig{ID: 0, PowerRestoreDelay: IntPtr(-1)}), nil, []string{"relay.relays.0.power_restore_delay"},
		},
		"undervoltage above voltage": {
			relayOf(SingleRelayConfig{ID: 0, UndervoltageLimit: IntPtr(300), VoltageLimit: IntPtr(250)}), nil,
			[]string{"relay.relays.0.undervoltage_limit", "relay.relays.0.voltage_limit"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, tc.desired)
			dev := fakeGen2DeviceWithChannels()
			if tc.prepare != nil {
				tc.prepare(dev)
			}

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			for _, want := range tc.want {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, dev.calls, "nothing is written")
		})
	}
}

// An undervoltage limit equal to the voltage limit passes the pair check, and with only one half
// desired the device decides.
func TestApplyDesiredConfig_Gen2TakesALimitPairThatFitsAndLeavesOneHalfToTheDevice(t *testing.T) {
	for name, relay := range map[string]SingleRelayConfig{
		"equal":              {ID: 0, UndervoltageLimit: IntPtr(250), VoltageLimit: IntPtr(250)},
		"undervoltage alone": {ID: 0, UndervoltageLimit: IntPtr(300)},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, relayOf(relay))
			dev := fakeGen2DeviceWithChannels()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied)
			assert.Len(t, dev.calls, 1)
		})
	}
}

// One read of the device serves every check made before the first write.
func TestApplyDesiredConfig_Gen2ReadsTheDeviceOnceBeforeTheFirstWrite(t *testing.T) {
	readsBefore := func(desired *DeviceConfiguration) int {
		svc, _ := newGen2Service(t, desired)
		dev := fakeGen2DeviceWithChannels()
		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")
		require.NoError(t, err)
		return dev.getsBeforeFirstWrite
	}
	mode := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("momentary")}}}}
	both := &DeviceConfiguration{
		Input: mode.Input,
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, VoltageLimit: IntPtr(250)}}},
	}

	one := readsBefore(mode)

	assert.Equal(t, one, readsBefore(both), "a limit adds no read to an input mode")
	assert.Equal(t, one, readsBefore(relayOf(SingleRelayConfig{ID: 0, VoltageLimit: IntPtr(250)})), "a limit alone reads the device as often")
}

// The page says the safety switch works only when in_mode is single. Enabling it is refused under any
// other effective mode, and so is a dual input mode under a switch that is already enabled.
func TestApplyDesiredConfig_Gen2RefusesADualCoverInputModeUnderAnEnabledSafetySwitch(t *testing.T) {
	enabled := func(d *fakeGen2Device) {
		d.doc["cover:0"].(map[string]interface{})["safety_switch"].(map[string]interface{})["enable"] = true
	}
	noSwitch := func(d *fakeGen2Device) { delete(d.doc["cover:0"].(map[string]interface{}), "safety_switch") }
	cases := map[string]struct {
		roller  RollerConfig
		prepare func(*fakeGen2Device)
		refused []string
		not     string
	}{
		"dual under an enabled switch":       {RollerConfig{InputMode: StringPtr("dual")}, enabled, []string{"roller.input_mode"}, ""},
		"dual with only a safety action":     {RollerConfig{InputMode: StringPtr("dual"), SafetyAction: StringPtr("pause")}, enabled, []string{"roller.input_mode"}, ""},
		"dual beside an enabling mode":       {RollerConfig{InputMode: StringPtr("dual"), SafetyMode: StringPtr("while_opening")}, enabled, []string{"roller.safety_mode"}, "roller.input_mode"},
		"dual beside an enabling mode, off":  {RollerConfig{InputMode: StringPtr("dual"), SafetyMode: StringPtr("while_opening")}, nil, []string{"roller.safety_mode"}, "roller.input_mode"},
		"dual beside a disabled mode":        {RollerConfig{InputMode: StringPtr("dual"), SafetyMode: StringPtr("disabled")}, enabled, nil, ""},
		"dual under a disabled switch":       {RollerConfig{InputMode: StringPtr("dual")}, nil, nil, ""},
		"dual on a device with no switch":    {RollerConfig{InputMode: StringPtr("dual")}, noSwitch, nil, ""},
		"single under an enabled switch":     {RollerConfig{InputMode: StringPtr("single")}, enabled, nil, ""},
		"no input mode under an enabled one": {RollerConfig{MaxOpenTime: IntPtr(30)}, enabled, nil, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			roller := tc.roller
			svc, _ := newGen2Service(t, &DeviceConfiguration{Roller: &roller})
			dev := fakeGen2DeviceWithCover()
			if tc.prepare != nil {
				tc.prepare(dev)
			}

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

			if tc.refused == nil {
				require.NoError(t, err)
				assert.True(t, result.ConfigApplied)
				return
			}
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			for _, path := range tc.refused {
				assert.Contains(t, err.Error(), path)
			}
			if tc.not != "" {
				assert.NotContains(t, err.Error(), tc.not, "the guard of the enabling mode names its own path only")
			}
			assert.Empty(t, dev.calls, "nothing is written")
		})
	}
}

// A cover write, an input mode and a limit are checked with one read of the device.
func TestApplyDesiredConfig_Gen2ReadsTheDeviceOnceForACoverAnInputModeAndALimit(t *testing.T) {
	readsBefore := func(desired *DeviceConfiguration) int {
		svc, _ := newGen2Service(t, desired)
		dev := fakeGen2DeviceWithCover()
		// A cover-profile device reports no switch of its own: this one reports a switch and an input so that the
		// input mode and the limit have a section to be written to.
		dev.doc["switch:0"] = map[string]interface{}{"id": 0.0, "in_mode": "follow", "voltage_limit": 280.0}
		dev.doc["input:0"] = map[string]interface{}{"id": 0.0, "type": "switch", "enable": true}
		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")
		require.NoError(t, err)
		return dev.getsBeforeFirstWrite
	}
	cover := &RollerConfig{MaxOpenTime: IntPtr(30)}
	all := &DeviceConfiguration{
		Roller: cover,
		Input:  &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("flip")}}},
		Relay:  &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, VoltageLimit: IntPtr(250)}}},
	}

	one := readsBefore(&DeviceConfiguration{Roller: cover})

	assert.Equal(t, 1, one, "a cover write alone reads the device once")
	assert.Equal(t, one, readsBefore(all), "an input mode and a limit add no read to a cover write")
}

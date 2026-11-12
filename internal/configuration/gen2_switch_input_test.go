package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// switchInputDesired is a two-channel desired configuration written with Gen1 words where a Gen1
// template would use them.
func switchInputDesired() *DeviceConfiguration {
	return &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{
			{ID: 0, Name: StringPtr("pump"), DefaultState: StringPtr("last"), AutoOn: IntPtr(30), AutoOff: IntPtr(0)},
			{ID: 1, DefaultState: StringPtr("switch")},
		}},
		Input: &InputConfig{Inputs: []SingleInputConfig{
			{ID: 0, Name: StringPtr("wall"), Type: StringPtr("switch"), Mode: StringPtr("toggle"), Inverted: BoolPtr(true)},
			{ID: 1, Mode: StringPtr("edge")},
		}},
	}
}

func fakeGen2DeviceWithChannels() *fakeGen2Device {
	d := newFakeGen2Device()
	for id, key := range []string{"0", "1"} {
		d.doc["switch:"+key] = map[string]interface{}{
			"id": float64(id), "name": nil, "in_mode": "momentary", "initial_state": "off",
			"auto_on": false, "auto_on_delay": 60.0, "auto_off": false, "auto_off_delay": 60.0,
		}
		d.doc["input:"+key] = map[string]interface{}{"id": float64(id), "name": nil, "type": "button", "enable": true, "invert": false}
	}
	return d
}

func TestGen2Converter_ReadsSwitchAndInput(t *testing.T) {
	device := newFakeGen2Device()
	device.doc["switch:0"] = map[string]interface{}{
		"id": 0.0, "name": "pump", "in_mode": "follow", "initial_state": "restore_last",
		"auto_on": true, "auto_on_delay": 30.0, "auto_off": false, "auto_off_delay": 60.0,
	}
	device.doc["input:0"] = map[string]interface{}{"id": 0.0, "name": "wall", "type": "switch", "enable": true, "invert": true}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	require.NotNil(t, got.Relay)
	require.Len(t, got.Relay.Relays, 1)
	relay := got.Relay.Relays[0]
	assert.Equal(t, 0, relay.ID)
	assert.Equal(t, "pump", *relay.Name)
	assert.Equal(t, "restore_last", *relay.DefaultState)
	assert.Equal(t, 30, *relay.AutoOn, "auto_on true with a delay is that many seconds")
	assert.Equal(t, 0, *relay.AutoOff, "auto_off false is 0, whatever delay the device keeps")
	require.NotNil(t, got.Input)
	require.Len(t, got.Input.Inputs, 1)
	input := got.Input.Inputs[0]
	assert.Equal(t, "wall", *input.Name)
	assert.Equal(t, "switch", *input.Type)
	assert.Equal(t, "follow", *input.Mode, "in_mode of the switch is the mode of its input")
	assert.True(t, *input.Inverted)
}

func TestGen2Converter_WritesTheSwitchAndInputShape(t *testing.T) {
	raw, err := NewGen2Converter(nil).ToAPIConfig(switchInputDesired(), "SNSW-001X16EU")

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"input:0":{"name":"wall","type":"switch","invert":true},
		"switch:0":{"name":"pump","initial_state":"restore_last","auto_on":true,"auto_on_delay":30,
			"auto_off":false,"in_mode":"follow"},
		"switch:1":{"initial_state":"match_input","in_mode":"flip"}}`, string(raw))
}

// Each Gen1 word that has the same meaning on a Gen2 device is written as the Gen2 word.
func TestGen2Converter_MapsTheGen1Words(t *testing.T) {
	cases := []struct {
		name    string
		desired *DeviceConfiguration
		section string
		key     string
		want    string
	}{
		{"last", &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, DefaultState: StringPtr("last")}}}}, "switch:0", "initial_state", "restore_last"},
		{"switch", &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, DefaultState: StringPtr("switch")}}}}, "switch:0", "initial_state", "match_input"},
		{"toggle", &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("toggle")}}}}, "switch:0", "in_mode", "follow"},
		{"edge", &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("edge")}}}}, "switch:0", "in_mode", "flip"},
		{"on", &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, DefaultState: StringPtr("on")}}}}, "switch:0", "initial_state", "on"},
		{"momentary", &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("momentary")}}}}, "switch:0", "in_mode", "momentary"},
		{"detached", &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("detached")}}}}, "switch:0", "in_mode", "detached"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := NewGen2Converter(nil).ToAPIConfig(tc.desired, "SNSW-001X16EU")

			require.NoError(t, err)
			var doc map[string]map[string]interface{}
			require.NoError(t, json.Unmarshal(raw, &doc))
			assert.Equal(t, tc.want, doc[tc.section][tc.key])
		})
	}
}

// A value that no Gen2 enum names is never written.
func TestGen2Converter_WritesNoValueOutsideTheEnum(t *testing.T) {
	desired := &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, DefaultState: StringPtr("restart")}}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("action"), Type: StringPtr("analog")}}},
	}

	raw, err := NewGen2Converter(nil).ToAPIConfig(desired, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(raw))
}

func TestApplyDesiredConfig_Gen2WritesSwitchAndInput(t *testing.T) {
	svc, repo := newGen2Service(t, switchInputDesired())
	dev := fakeGen2DeviceWithChannels()
	dev.restart = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify matches: the Gen1 words are read back as their Gen2 meaning")
	assert.True(t, repo.devices[1].ConfigApplied)
	require.Len(t, dev.calls, 3, "input 1 has no key of its own: its mode is written with switch 1")
	assert.Equal(t, "Input.SetConfig", dev.calls[0].method)
	assert.Equal(t, 0, dev.calls[0].id)
	assert.Equal(t, "Switch.SetConfig", dev.calls[1].method)
	assert.Equal(t, 0, dev.calls[1].id)
	assert.Equal(t, "Switch.SetConfig", dev.calls[2].method)
	assert.Equal(t, 1, dev.calls[2].id)
	assert.Equal(t, 11, result.ApplyResult.AppliedCount, "three input keys, six keys of switch 0, two of switch 1")
	assert.Equal(t, 1, dev.reboots, "one reboot, because an answer asked for one")
}

func TestApplyDesiredConfig_Gen2WritesChannelsAfterTheSections(t *testing.T) {
	desired := switchInputDesired()
	desired.Cloud = &CloudConfiguration{Enable: BoolPtr(false)}
	desired.System = &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	var methods []string
	for _, call := range dev.calls {
		methods = append(methods, call.method)
	}
	assert.Equal(t, []string{"Sys.SetConfig", "Cloud.SetConfig", "Input.SetConfig", "Switch.SetConfig", "Switch.SetConfig"}, methods)
	assert.Equal(t, 0, dev.reboots, "no answer asked for a restart")
}

// Auto on and auto off are two keys on a Gen2 device. Zero seconds is off, and no delay is sent.
func TestApplyDesiredConfig_Gen2ZeroSecondsSwitchesAnAutoTimerOff(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, AutoOn: IntPtr(0), AutoOff: IntPtr(90)}}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{"auto_on": false, "auto_off": true, "auto_off_delay": float64(90)}, dev.calls[0].config)
}

// A setting with no Gen2 key, or a value no Gen2 enum names, is refused before anything is written.
func TestApplyDesiredConfig_Gen2RefusesSwitchAndInputSettingsItCannotWrite(t *testing.T) {
	relay := func(r SingleRelayConfig) *DeviceConfiguration {
		r.Name = StringPtr("pump")
		return &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{r}}}
	}
	input := func(i SingleInputConfig) *DeviceConfiguration {
		i.Name = StringPtr("wall")
		return &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{i}}}
	}
	cases := map[string]struct {
		desired *DeviceConfiguration
		path    string
	}{
		"initial state":       {relay(SingleRelayConfig{DefaultState: StringPtr("restart")}), "relay.relays.0.default_state"},
		"negative auto on":    {relay(SingleRelayConfig{AutoOn: IntPtr(-1)}), "relay.relays.0.auto_on"},
		"negative auto off":   {relay(SingleRelayConfig{AutoOff: IntPtr(-5)}), "relay.relays.0.auto_off"},
		"schedule":            {relay(SingleRelayConfig{Schedule: BoolPtr(true)}), "relay.relays.0.schedule"},
		"gen1 action mode":    {input(SingleInputConfig{Mode: StringPtr("action")}), "input.inputs.0.mode"},
		"analog type":         {input(SingleInputConfig{Type: StringPtr("analog")}), "input.inputs.0.type"},
		"count type":          {input(SingleInputConfig{Type: StringPtr("count")}), "input.inputs.0.type"},
		"single push action":  {input(SingleInputConfig{SinglePushAction: StringPtr("toggle")}), "input.inputs.0.single_push_action"},
		"long push action":    {input(SingleInputConfig{LongPushAction: StringPtr("toggle")}), "input.inputs.0.long_push_action"},
		"top level auto on":   {&DeviceConfiguration{Relay: &RelayConfig{AutoOn: IntPtr(30), Relays: []SingleRelayConfig{{ID: 0, Name: StringPtr("pump")}}}}, "relay.auto_on"},
		"top level debounce":  {&DeviceConfiguration{Input: &InputConfig{DebounceTime: IntPtr(50), Inputs: []SingleInputConfig{{ID: 0, Name: StringPtr("wall")}}}}, "input.debounce_time"},
		"top level btn type":  {&DeviceConfiguration{Relay: &RelayConfig{ButtonType: StringPtr("toggle"), Relays: []SingleRelayConfig{{ID: 0, Name: StringPtr("pump")}}}}, "relay.btn_type"},
		"top level push time": {&DeviceConfiguration{Input: &InputConfig{LongPushTime: IntPtr(800), Inputs: []SingleInputConfig{{ID: 0, Name: StringPtr("wall")}}}}, "input.long_push_time"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, tc.desired)
			dev := fakeGen2DeviceWithChannels()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls)
		})
	}
}

// A device with one switch is not written to for a second one it does not report.
func TestApplyDesiredConfig_Gen2ReportsAChannelTheDeviceDoesNotHave(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 3, Name: StringPtr("ghost")}}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()
	dev.refuse = errors.New("no such component")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotEmpty(t, result.ApplyResult.Failures)
	assert.Equal(t, "switch:3.name", result.ApplyResult.Failures[0].Path)
}

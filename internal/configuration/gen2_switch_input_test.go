package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
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

// fakeGen2DeviceWithChannels holds two channels. Input 0 is a button that drives switch 0 in
// momentary mode, and input 1 is a switch that drives switch 1 in follow mode, so that a mode of
// each kind has the input type it needs where the test changes only one of the two.
func fakeGen2DeviceWithChannels() *fakeGen2Device {
	d := newFakeGen2Device()
	for id, kind := range []string{"button", "switch"} {
		key := strconv.Itoa(id)
		mode := map[string]string{"button": "momentary", "switch": "follow"}[kind]
		d.doc["switch:"+key] = map[string]interface{}{
			"id": float64(id), "name": nil, "in_mode": mode, "initial_state": "off",
			"auto_on": false, "auto_on_delay": 60.0, "auto_off": false, "auto_off_delay": 60.0,
		}
		d.doc["input:"+key] = map[string]interface{}{"id": float64(id), "name": nil, "type": kind, "enable": true, "invert": false}
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
	dev.strict = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "Switch.SetConfig", dev.calls[0].method)
	assert.Equal(t, 3, dev.calls[0].id, "the write names switch 3, and the device holds only 0 and 1")
	require.NotEmpty(t, result.ApplyResult.Failures)
	assert.Equal(t, "switch:3.name", result.ApplyResult.Failures[0].Path)
	assert.Contains(t, result.ApplyResult.Failures[0].Error, "no such component: switch:3")
}

// A desired channel is matched with the device's channel of the same id, never the one at the same
// position in the list.
func TestApplyDesiredConfig_Gen2VerifiesChannelsByID(t *testing.T) {
	desired := &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, Name: StringPtr("pump")}}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 1, Name: StringPtr("wall"), Inverted: BoolPtr(true)}}},
	}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "channel 1 is compared with channel 1")
	assert.Empty(t, result.VerifyResult.Differences)
}

func TestApplyDesiredConfig_Gen2ReportsADifferenceInTheChannelOfThatID(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, Name: StringPtr("pump")}}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()
	dev.ignoreWrites = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.Len(t, result.VerifyResult.Differences, 1)
	assert.Equal(t, "relay.relays.0.name", result.VerifyResult.Differences[0].Path)
}

// A delay that is not a whole number of seconds above 0 has no stored value, so a desired number
// reports a difference and a disabled timer is never matched by an enabled one.
func TestGen2Converter_ReadsNoTimerForADelayItCannotStore(t *testing.T) {
	cases := map[string]float64{"fraction": 30.9, "half a second": 0.5, "zero": 0, "negative": -1}
	for name, delay := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]interface{}{
				"switch:0": map[string]interface{}{"id": 0, "auto_off": true, "auto_off_delay": delay},
			})
			require.NoError(t, err)

			got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

			require.NoError(t, err)
			require.NotNil(t, got.Relay)
			assert.Nil(t, got.Relay.Relays[0].AutoOff)
			for _, seconds := range []int{0, 1, 30} {
				desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, AutoOff: IntPtr(seconds)}}}}
				assert.False(t, NewConfigComparator().Compare(desired, got).Match, "desired %d s", seconds)
			}
		})
	}
}

// A channel that sets only what no Gen2 key carries is still refused, with a path for each setting.
func TestApplyDesiredConfig_Gen2NamesEachSettingOfAChannelThatSetsNothingElse(t *testing.T) {
	cases := map[string]struct {
		desired *DeviceConfiguration
		paths   []string
	}{
		"relay": {
			&DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, Schedule: BoolPtr(true), MaxPower: IntPtr(100)}}}},
			[]string{"relay.relays.0.schedule", "relay.relays.0.max_power"},
		},
		"input": {
			&DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, SinglePushAction: StringPtr("toggle"), LongPushAction: StringPtr("toggle")}}}},
			[]string{"input.inputs.0.single_push_action", "input.inputs.0.long_push_action"},
		},
		"top level relay": {
			&DeviceConfiguration{Relay: &RelayConfig{AutoOn: IntPtr(30)}},
			[]string{"relay.auto_on"},
		},
		"top level input": {
			&DeviceConfiguration{Input: &InputConfig{DebounceTime: IntPtr(50)}},
			[]string{"input.debounce_time"},
		},
		"unmapped value only": {
			&DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("action")}}}},
			[]string{"input.inputs.0.mode"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, tc.desired)
			dev := fakeGen2DeviceWithChannels()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			for _, path := range tc.paths {
				assert.Contains(t, err.Error(), path)
			}
			assert.Empty(t, dev.calls)
		})
	}
}

// The fake device refuses a mode that the input type cannot drive, so a write of the mode before
// the type fails here as it does on a device.
func TestApplyDesiredConfig_Gen2WritesTheInputTypeBeforeTheModeThatNeedsIt(t *testing.T) {
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Type: StringPtr("switch"), Mode: StringPtr("flip")}}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 2)
	assert.Equal(t, "Input.SetConfig", dev.calls[0].method)
	assert.Equal(t, "Switch.SetConfig", dev.calls[1].method)
}

func TestApplyDesiredConfig_Gen2FailsAModeThatTheInputTypeCannotDrive(t *testing.T) {
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("flip")}}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotEmpty(t, result.ApplyResult.Failures)
	assert.Contains(t, result.ApplyResult.Failures[0].Error, "needs a switch input")
}

// A switch that takes another input (input_id) runs its in_mode on that input, so the mode is read
// as the mode of that input and never as the mode of the input with the switch's own number.
func TestGen2Converter_ReadsTheModeOnTheInputTheSwitchTakes(t *testing.T) {
	raw, err := json.Marshal(map[string]interface{}{
		"switch:0": map[string]interface{}{"id": 0, "in_mode": "flip", "input_id": 1},
		"input:0":  map[string]interface{}{"id": 0, "type": "switch"},
		"input:1":  map[string]interface{}{"id": 1, "type": "switch"},
	})
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	require.Len(t, got.Input.Inputs, 2)
	assert.Nil(t, got.Input.Inputs[0].Mode, "input 0 does not drive switch 0")
	require.NotNil(t, got.Input.Inputs[1].Mode)
	assert.Equal(t, "flip", *got.Input.Inputs[1].Mode)
}

// The mode of input 0 is written to switch 0, which runs the input named by its input_id. When that
// is another input, the write would change the wrong input's behavior, and a later verify cannot
// undo it. Apply refuses before the first write and leaves the applied flag as it was.
func TestApplyDesiredConfig_Gen2RefusesAnInputModeForAnotherSelectedInput(t *testing.T) {
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{
		{ID: 0, Type: StringPtr("switch"), Mode: StringPtr("flip")},
		{ID: 1, Name: StringPtr("door")},
	}}}
	svc, repo := newGen2Service(t, desired)
	repo.devices[1].ConfigApplied = true
	dev := fakeGen2DeviceWithChannels()
	dev.doc["switch:0"].(map[string]interface{})["input_id"] = 1.0

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "input.inputs.0.mode")
	assert.Empty(t, dev.calls, "nothing is written")
	assert.True(t, repo.devices[1].ConfigApplied, "the device still holds what the flag said")
}

// A switch that takes its own input, or names no input_id at all, is written as before.
func TestApplyDesiredConfig_Gen2WritesAnInputModeWhenTheSwitchTakesItsOwnInput(t *testing.T) {
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Type: StringPtr("switch"), Mode: StringPtr("flip")}}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()
	dev.doc["switch:0"].(map[string]interface{})["input_id"] = 0.0

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
}

// The merge of a template and a sparse override pairs channels by position and keeps the id of the
// override, so a channel that only the template sets can end up under another channel's id. A list
// that holds one id twice cannot say which channel a setting belongs to, so apply refuses it.
func TestApplyDesiredConfig_Gen2RefusesARepeatedChannelID(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 1, AutoOn: IntPtr(30), Name: StringPtr("lamp")},
		{ID: 1, AutoOff: IntPtr(60)},
	}}}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "relay.relays.1.id")
	assert.Empty(t, dev.calls, "nothing is written")
}

// An entry that sets nothing still holds its id. Verify compares the desired list as it stands, so a
// repeated id left in it makes verify fall back to positional compare, and a write that went through
// would then read as a missing channel. The repeat is refused whether or not the first entry sets
// something.
func TestUnsupportedGen2Settings_NamesARepeatedIDWhereTheFirstEntrySetsNothing(t *testing.T) {
	desired := &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0}, {ID: 0, Name: StringPtr("pump")}}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 2}, {ID: 2, Name: StringPtr("door")}}},
	}

	paths, err := unsupportedGen2Settings(NewGen2Converter(nil), desired)

	require.NoError(t, err)
	assert.Equal(t, []string{"input.inputs.1.id", "relay.relays.1.id"}, paths)
}

func TestUnsupportedGen2Settings_NamesARepeatedInputID(t *testing.T) {
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{
		{ID: 0, Name: StringPtr("a")},
		{ID: 1, Name: StringPtr("b")},
		{ID: 0, Name: StringPtr("c")},
	}}}

	paths, err := unsupportedGen2Settings(NewGen2Converter(nil), desired)

	require.NoError(t, err)
	assert.Equal(t, []string{"input.inputs.2.id"}, paths)
}

func TestConfigComparator_PairsChannelsByIDOnlyOnRequest(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, Name: StringPtr("b")}}}}
	actual := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, Name: StringPtr("a")}, {ID: 1, Name: StringPtr("b")},
	}}}

	assert.True(t, NewChannelIDConfigComparator().Compare(desired, actual).Match, "id 1 is compared with id 1")
	assert.False(t, NewConfigComparator().Compare(desired, actual).Match, "position 0 is compared with position 0")
}

// A Gen1 channel's id is its position, so Gen1 verify stays positional: desired channel 1 is not
// verified against the Gen1 device's channel 0 by id.
func TestVerifyConfig_Gen1StaysPositional(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{"relays":[{"name":"a"},{"name":"b"}]}`)
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, Name: StringPtr("b")}}}}

	result, err := NewConfigVerifier(NewGen1Converter(nil), nil).VerifyConfig(context.Background(), client, desired, "SHSW-25")

	require.NoError(t, err)
	assert.False(t, result.Match, "position 0 holds a, not b")
	var paths []string
	for _, difference := range result.Differences {
		paths = append(paths, difference.Path)
	}
	assert.Contains(t, paths, "relay.relays.0.name")
}

// The mode lives on the input, so the read-back may only build an input the device reports. A
// switch's in_mode alone does not make input 0 exist.
func TestVerifyConfig_Gen2ReportsAnInputTheDeviceDoesNotHave(t *testing.T) {
	dev := &fakeGen2Device{doc: map[string]interface{}{"switch:0": map[string]interface{}{"in_mode": "detached"}}}
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("detached")}}}}

	result, err := NewConfigVerifier(NewGen2Converter(nil), nil).VerifyConfig(context.Background(), dev, desired, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.Match, "the device has no input 0")
}

// A template can store the Gen2 initial states, and a Gen1 device has no such words. Apply refuses
// them before it writes anything.
func TestApplyDesiredConfig_Gen1RefusesTheGen2InitialStates(t *testing.T) {
	for _, state := range []string{"restore_last", "match_input"} {
		t.Run(state, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "tv-office")
			setDesired(t, repo, &DeviceConfiguration{
				System: mergeName(nil),
				Relay:  &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, DefaultState: StringPtr(state)}}},
			})
			dev := newFakeGen1Device()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHSW-1")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), "relay.relays[0].default_state")
			assert.Equal(t, 0, dev.setCalls, "the supported name must not be written either")
		})
	}
}

// A refusal path names the channel's place in the desired list, even after a channel with nothing
// to write is left out of the check.
func TestUnsupportedGen2Settings_NamesAnOutOfRangeTimerAtItsOwnIndex(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0}, {ID: 1, AutoOn: IntPtr(2147483648)},
	}}}

	unsupported, err := unsupportedGen2Settings(NewGen2Converter(nil), desired)

	require.NoError(t, err)
	assert.Equal(t, []string{"relay.relays.1.auto_on"}, unsupported)
}

// A channel with nothing to write is left out of the check, and a path that the round trip reports
// still names the channel's place in the desired list.
func TestUnsupportedGen2Settings_NamesARoundTripRefusalAtItsOwnIndex(t *testing.T) {
	relays := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0}, {ID: 1, Name: StringPtr("good")}, {ID: -1, Name: StringPtr("bad")},
	}}}
	inputs := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{
		{ID: 0}, {ID: 1, Name: StringPtr("good")}, {ID: -1, Name: StringPtr("bad")},
	}}}

	relayPaths, err := unsupportedGen2Settings(NewGen2Converter(nil), relays)
	require.NoError(t, err)
	inputPaths, err := unsupportedGen2Settings(NewGen2Converter(nil), inputs)
	require.NoError(t, err)

	assert.Contains(t, relayPaths, "relay.relays.2")
	assert.NotContains(t, relayPaths, "relay.relays.1", "the valid entry is not refused")
	assert.Contains(t, inputPaths, "input.inputs.2")
	assert.NotContains(t, inputPaths, "input.inputs.1", "the valid entry is not refused")
}

func TestVerifyConfig_Gen2PairsChannelsByID(t *testing.T) {
	dev := fakeGen2DeviceWithChannels()
	dev.doc["switch:1"].(map[string]interface{})["name"] = "b"
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, Name: StringPtr("b")}}}}

	result, err := NewConfigVerifier(NewGen2Converter(nil), nil).VerifyConfig(context.Background(), dev, desired, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.Match, result.Differences)
}

// A template or an override is checked when it is saved. The Gen2 words that apply writes must pass
// that check, or they never reach the converter.
func TestValidateConfigBody_AcceptsTheGen2SwitchAndInputWords(t *testing.T) {
	for _, state := range []string{"on", "off", "last", "switch", "restore_last", "match_input"} {
		body := map[string]interface{}{"relay": map[string]interface{}{"relays": []interface{}{
			map[string]interface{}{"id": 0.0, "default_state": state},
		}}}
		assert.Empty(t, ValidateConfigBody("SNSW-001X16EU", body), "initial state %s", state)
	}
	for _, mode := range []string{"momentary", "follow", "flip", "detached", "cycle", "activate"} {
		body := map[string]interface{}{"input": map[string]interface{}{"inputs": []interface{}{
			map[string]interface{}{"id": 0.0, "mode": mode},
		}}}
		assert.Empty(t, ValidateConfigBody("SNSW-001X16EU", body), "mode %s", mode)
	}
}

// The Gen1 words toggle and edge are read as follow and flip, so a Gen2 model or a template must
// accept them when it is saved. A supported Gen1 model keeps its own stored words.
func TestValidateConfigBody_AcceptsTheGen1InputAliasesForGen2AndTemplates(t *testing.T) {
	for _, mode := range []string{"toggle", "edge"} {
		body := map[string]interface{}{"input": map[string]interface{}{"inputs": []interface{}{
			map[string]interface{}{"id": 0.0, "mode": mode},
		}}}
		assert.Empty(t, ValidateConfigBody("SNSW-001X16EU", body), "Gen2 model, mode %s", mode)
		assert.Empty(t, ValidateConfigBody("", body), "template, mode %s", mode)
		assert.Len(t, ValidateConfigBody("SHSW-25", body), 1, "Gen1 model, mode %s", mode)
	}
}

// A desired list may skip a channel id or hold the channels in any order. The refusal check pairs
// channels by id, so a channel that sets one field is not held against another channel's fields.
func TestApplyDesiredConfig_Gen2AcceptsSparseAndReorderedChannels(t *testing.T) {
	desired := &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{
			{ID: 1, Name: StringPtr("pump")},
			{ID: 0, DefaultState: StringPtr("off")},
		}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("detached")}}},
	}
	svc, _ := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Len(t, dev.calls, 2, "switch 0 and switch 1, one call each")
}

func TestValidateConfigBody_StillRefusesAWordNoDeviceHas(t *testing.T) {
	state := map[string]interface{}{"relay": map[string]interface{}{"relays": []interface{}{
		map[string]interface{}{"id": 0.0, "default_state": "restart"},
	}}}
	mode := map[string]interface{}{"input": map[string]interface{}{"inputs": []interface{}{
		map[string]interface{}{"id": 0.0, "mode": "bogus"},
	}}}

	assert.Len(t, ValidateConfigBody("SNSW-001X16EU", state), 1)
	assert.Len(t, ValidateConfigBody("SNSW-001X16EU", mode), 1)
}

func TestConfigComparator_ComparesByPositionWhereAnIDRepeats(t *testing.T) {
	desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, Name: StringPtr("a")}, {ID: 0, Name: StringPtr("b")},
	}}}
	actual := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, Name: StringPtr("a")}, {ID: 1, Name: StringPtr("b")},
	}}}

	result := NewChannelIDConfigComparator().Compare(desired, actual)

	assert.False(t, result.Match, "the second desired element is compared with the second actual one")
	require.NotEmpty(t, result.Differences)
	assert.Equal(t, "relay.relays.1.id", result.Differences[0].Path)
}

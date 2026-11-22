package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGen2DeviceReportingReverseAndInputID is the two-channel device with the two keys a model reports
// only when it has them: reverse (a model that measures returned energy) and input_id (a model with a
// selectable input). Both switches report them, with input_id equal to their own number.
func fakeGen2DeviceReportingReverseAndInputID() *fakeGen2Device {
	d := fakeGen2DeviceWithChannels()
	for _, key := range []string{"switch:0", "switch:1"} {
		section := d.doc[key].(map[string]interface{})
		section["reverse"] = false
		section["input_id"] = section["id"]
	}
	return d
}

// reverse is written with Switch.SetConfig and read back. The page says the option requires a restart,
// so a changed value owes one whether or not the answer carries restart_required; an unchanged value
// owes none.
func TestApplyDesiredConfig_Gen2WritesReverseAndOwesTheRestart(t *testing.T) {
	cases := map[string]struct {
		live    bool
		desired bool
		answer  bool
		owed    bool
	}{
		"changed, answer says no":   {false, true, false, true},
		"changed, answer says yes":  {false, true, true, true},
		"changed back":              {true, false, false, true},
		"unchanged, answer says no": {true, true, false, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newGen2Service(t, relayOf(SingleRelayConfig{ID: 0, Reverse: BoolPtr(tc.desired)}))
			dev := fakeGen2DeviceReportingReverseAndInputID()
			dev.doc["switch:0"].(map[string]interface{})["reverse"] = tc.live
			dev.restart = tc.answer

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied, "reverse is read back as written")
			require.Len(t, dev.calls, 1)
			assert.Equal(t, "Switch.SetConfig", dev.calls[0].method)
			assert.Equal(t, map[string]interface{}{"reverse": tc.desired}, dev.calls[0].config)
			assert.Equal(t, tc.owed, result.ApplyResult.RequiresReboot)
			if tc.owed {
				assert.Equal(t, 1, dev.reboots)
				if !tc.answer {
					assert.Contains(t, result.ApplyResult.Warnings, "Device reboot required for some settings to take effect (a changed reverse needs a restart)")
				}
				return
			}
			assert.Equal(t, 0, dev.reboots)
			assert.False(t, repo.devices[1].PendingReboot)
		})
	}
}

// The restart for a changed reverse is known before the request, so it is recorded before it.
func TestApplyDesiredConfig_Gen2RecordsTheReverseRestartBeforeTheWrite(t *testing.T) {
	svc, repo := newGen2Service(t, relayOf(SingleRelayConfig{ID: 0, Reverse: BoolPtr(true)}))
	dev := fakeGen2DeviceReportingReverseAndInputID()
	dev.refuse = errors.New("no answer")

	result, _ := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NotNil(t, result)
	assert.True(t, result.ApplyResult.RequiresReboot, "owed although the write got no answer")
	assert.True(t, repo.devices[1].PendingReboot)
}

// input_id is written where the live switch reports it, and the page's valid values are 0 and 1.
func TestApplyDesiredConfig_Gen2WritesInputIDWhereTheDeviceReportsIt(t *testing.T) {
	for _, id := range []int{0, 1} {
		svc, _ := newGen2Service(t, relayOf(SingleRelayConfig{ID: 0, InputID: IntPtr(id)}))
		dev := fakeGen2DeviceReportingReverseAndInputID()

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

		require.NoError(t, err)
		assert.True(t, result.ConfigApplied, "input_id is read back as written")
		require.Len(t, dev.calls, 1)
		assert.Equal(t, "Switch.SetConfig", dev.calls[0].method)
		assert.Equal(t, map[string]interface{}{"input_id": float64(id)}, dev.calls[0].config)
		assert.False(t, result.ApplyResult.RequiresReboot, "the page names no restart for input_id")
	}
}

// A key the live section does not carry, and an input_id the page does not list, are refused before
// the first write. There is no model table: the device says whether it has the key.
func TestApplyDesiredConfig_Gen2RefusesReverseAndInputIDBeforeAnyWrite(t *testing.T) {
	missing := func(key string) func(*fakeGen2Device) {
		return func(d *fakeGen2Device) { delete(d.doc["switch:0"].(map[string]interface{}), key) }
	}
	cases := map[string]struct {
		relay   SingleRelayConfig
		prepare func(*fakeGen2Device)
		want    string
	}{
		"reverse not reported":  {SingleRelayConfig{ID: 0, Reverse: BoolPtr(true)}, missing("reverse"), "relay.relays.0.reverse (reverse)"},
		"input_id not reported": {SingleRelayConfig{ID: 0, InputID: IntPtr(1)}, missing("input_id"), "relay.relays.0.input_id (input_id)"},
		"input_id 2":            {SingleRelayConfig{ID: 0, InputID: IntPtr(2)}, nil, "relay.relays.0.input_id"},
		"input_id -1":           {SingleRelayConfig{ID: 0, InputID: IntPtr(-1)}, nil, "relay.relays.0.input_id"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, relayOf(tc.relay))
			dev := fakeGen2DeviceReportingReverseAndInputID()
			if tc.prepare != nil {
				tc.prepare(dev)
			}

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.want)
			assert.Empty(t, dev.calls, "nothing is written")
		})
	}
}

// An input_id that changes in the same apply as an input mode would put the mode on the wrong input: the
// mode is written to the switch of the input, and the check of refuseRemappedInputs reads the live
// input_id. A switch whose live input_id is null or absent runs its own input, as refuseRemappedInputs
// reads it, so a new input_id there changes it too. Where the new input_id moves the mode to another
// input, the round trip before the device is read already refuses it, naming the mode; the live check
// answers the rest, with both paths.
func TestApplyDesiredConfig_Gen2RefusesAChangedInputIDTogetherWithAnInputMode(t *testing.T) {
	cases := map[string]struct {
		live    interface{}
		desired int
		want    []string
	}{
		"live is another input, desired is its own": {1.0, 0, []string{"relay.relays.0.input_id", "input.inputs.0.mode", "one at a time"}},
		"live is null, desired is another input":    {nil, 1, []string{"input.inputs.0.mode"}},
		"live is its own, desired is another input": {0.0, 1, []string{"input.inputs.0.mode"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, &DeviceConfiguration{
				Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, InputID: IntPtr(tc.desired)}}},
				Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("flip")}}},
			})
			dev := fakeGen2DeviceReportingReverseAndInputID()
			dev.doc["switch:0"].(map[string]interface{})["input_id"] = tc.live

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			for _, want := range tc.want {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, dev.calls, "nothing is written")
		})
	}
}

// An imported template carries the input_id the device already runs beside the input mode. Applying it
// with one other edit is written, both keys: the input_id does not change, so the mode lands on the
// input it names.
func TestApplyDesiredConfig_Gen2WritesAnUnchangedInputIDBesideAnInputMode(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, InputID: IntPtr(1), Reverse: BoolPtr(true)}}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 1, Mode: StringPtr("flip")}}},
	})
	dev := fakeGen2DeviceReportingReverseAndInputID()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	section := dev.doc["switch:1"].(map[string]interface{})
	assert.Equal(t, "flip", section["in_mode"])
	assert.Equal(t, 1.0, section["input_id"])
	assert.Equal(t, true, section["reverse"])
}

// A switch the device does not report has no input to compare: splitting the apply cannot help, so the
// refusal is the missing key's, not the one-at-a-time one.
func TestApplyDesiredConfig_Gen2AnAbsentSwitchIsRefusedForItsMissingKey(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, InputID: IntPtr(0)}}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("flip")}}},
	})
	dev := fakeGen2DeviceReportingReverseAndInputID()
	delete(dev.doc, "switch:1")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "relay.relays.0.input_id (input_id)")
	assert.NotContains(t, err.Error(), "one at a time")
	assert.Empty(t, dev.calls, "nothing is written")
}

// reverse beside an input mode is unchanged: the mode goes through the live input_id check as before.
func TestApplyDesiredConfig_Gen2ReverseBesideAnInputModeIsWritten(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 1, Reverse: BoolPtr(true)}}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 1, Mode: StringPtr("flip")}}},
	})
	dev := fakeGen2DeviceReportingReverseAndInputID()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, "flip", dev.doc["switch:1"].(map[string]interface{})["in_mode"])
	assert.Equal(t, true, dev.doc["switch:1"].(map[string]interface{})["reverse"])
}

// The mode of input N is written to switch N. A device that reports no such switch (a light device
// holds light:0 and input:0) would answer the write with a failed row, so it is refused before any write.
func TestApplyDesiredConfig_Gen2RefusesAnInputModeWhereTheDeviceReportsNoSwitch(t *testing.T) {
	desired := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("flip")}}}}

	t.Run("no switch: refused", func(t *testing.T) {
		svc, _ := newGen2Service(t, desired)
		dev := newFakeGen2Device()
		dev.strict = true
		dev.doc["light:0"] = map[string]interface{}{"id": 0.0, "name": nil}
		dev.doc["input:0"] = map[string]interface{}{"id": 0.0, "name": nil, "type": "switch", "enable": true, "invert": false}

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNDM-0013US")

		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		assert.Contains(t, err.Error(), "input.inputs.0.mode")
		assert.Contains(t, err.Error(), "no switch")
		assert.Empty(t, dev.calls, "nothing is written")
	})
	t.Run("switch: written as before", func(t *testing.T) {
		svc, _ := newGen2Service(t, desired)
		dev := fakeGen2DeviceWithChannels()
		dev.doc["input:0"].(map[string]interface{})["type"] = "switch"

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

		require.NoError(t, err)
		assert.True(t, result.ConfigApplied)
		assert.Equal(t, "flip", dev.doc["switch:0"].(map[string]interface{})["in_mode"])
	})
}

// A null, a missing key and a fraction have no stored value.
func TestGen2Converter_ReadsReverseAndInputID(t *testing.T) {
	read := func(section map[string]interface{}) SingleRelayConfig {
		raw, err := json.Marshal(map[string]interface{}{"switch:0": section})
		require.NoError(t, err)
		got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")
		require.NoError(t, err)
		require.NotNil(t, got.Relay)
		return got.Relay.Relays[0]
	}

	set := read(map[string]interface{}{"id": 0, "reverse": true, "input_id": 1.0})
	require.NotNil(t, set.Reverse)
	assert.True(t, *set.Reverse)
	require.NotNil(t, set.InputID)
	assert.Equal(t, 1, *set.InputID)

	unset := read(map[string]interface{}{"id": 0, "reverse": nil, "input_id": 0.5})
	assert.Nil(t, unset.Reverse)
	assert.Nil(t, unset.InputID)
}

// A Gen1 model has neither key, so the Gen1 path refuses both before it writes.
func TestGen1RefusesReverseAndInputID(t *testing.T) {
	for name, relay := range map[string]SingleRelayConfig{
		"reverse":  {ID: 0, Reverse: BoolPtr(true)},
		"input_id": {ID: 0, InputID: IntPtr(1)},
	} {
		refused, err := unsupportedApplySettings(NewGen1Converter(nil), relayOf(relay), "SHSW-1", true)
		require.NoError(t, err, name)
		assert.Contains(t, refused, "relay.relays.0."+name)
	}
}

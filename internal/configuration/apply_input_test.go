package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inputTestClient adds the Shelly i3 input surface (POST and GET /settings/input/<n>) to the apply
// test client. The relay surface, which the relay-input models use, is the embedded client's.
type inputTestClient struct {
	*applyTestClient
	inputWrites []relayWrite // channel and form of each write
	inputLive   map[int]map[string]interface{}
	inputSetErr error
}

func newInputTestClient() *inputTestClient {
	return &inputTestClient{applyTestClient: newApplyTestClient()}
}

func (c *inputTestClient) SetInputSettings(_ context.Context, input int, settings map[string]interface{}) error {
	if c.inputSetErr != nil {
		return c.inputSetErr
	}
	c.inputWrites = append(c.inputWrites, relayWrite{channel: input, form: settings})
	return nil
}

func (c *inputTestClient) GetInputSettings(_ context.Context, input int) (map[string]interface{}, error) {
	if live, ok := c.inputLive[input]; ok {
		return live, nil
	}
	echo := map[string]interface{}{}
	for _, w := range c.inputWrites {
		if w.channel != input {
			continue
		}
		for k, v := range w.form {
			// A device returns JSON numbers as float64.
			if n, ok := v.(int); ok {
				v = float64(n)
			}
			echo[k] = v
		}
	}
	return echo, nil
}

func inputsDesired(inputs ...SingleInputConfig) *DeviceConfiguration {
	return &DeviceConfiguration{Input: &InputConfig{Inputs: inputs}}
}

func TestApplyConfig_I3InputsAreWrittenPerChannelWithGen1Names(t *testing.T) {
	client := newInputTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, inputsDesired(
		SingleInputConfig{ID: 0, Mode: StringPtr("follow"), Inverted: BoolPtr(true)},
		SingleInputConfig{ID: 1, Mode: StringPtr("momentary"), Inverted: BoolPtr(false)},
	), "SHIX3-1")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	// The stored mode "follow" is the Gen1 button type "toggle"; the reverse flag goes out as 0 or 1.
	require.Len(t, client.inputWrites, 2)
	assert.Equal(t, relayWrite{channel: 0, form: map[string]interface{}{"btn_type": "toggle", "btn_reverse": float64(1)}}, client.inputWrites[0])
	assert.Equal(t, relayWrite{channel: 1, form: map[string]interface{}{"btn_type": "momentary", "btn_reverse": float64(0)}}, client.inputWrites[1])
	// An i3 has no relay, and the input is never sent as a nested document on POST /settings.
	assert.Empty(t, client.relayWrites)
	assert.Empty(t, client.configs)
	assert.Equal(t, 4, result.SettingsCount)
	assert.Equal(t, 4, result.AppliedCount)
}

func TestApplyConfig_RelayModelInputsGoToTheRelayChannel(t *testing.T) {
	client := newInputTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, inputsDesired(
		SingleInputConfig{ID: 0, Mode: StringPtr("flip")},
		SingleInputConfig{ID: 1, Inverted: BoolPtr(true)},
	), "SHSW-25")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	// A Shelly 1, 1PM or 2.5 keeps the button type and the reverse flag in its relay channel.
	require.Len(t, client.relayWrites, 2)
	assert.Equal(t, relayWrite{channel: 0, form: map[string]interface{}{"btn_type": "edge"}}, client.relayWrites[0])
	assert.Equal(t, relayWrite{channel: 1, form: map[string]interface{}{"btn_reverse": float64(1)}}, client.relayWrites[1])
	assert.Empty(t, client.inputWrites, "a relay model has no /settings/input route")
	assert.Equal(t, 2, result.AppliedCount)
}

func TestApplyConfig_InputKeepsRelayKeysOutOfTheButtonWrite(t *testing.T) {
	client := newInputTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, AutoOff: IntPtr(30)}}},
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("detached")}}},
	}, "SHSW-1")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.relayWrites, 2)
	assert.Equal(t, map[string]interface{}{"auto_off": float64(30)}, client.relayWrites[0].form)
	assert.Equal(t, map[string]interface{}{"btn_type": "detached"}, client.relayWrites[1].form)
}

func TestApplyConfig_InputReadBackMismatchFailsThePath(t *testing.T) {
	client := newInputTestClient()
	client.inputLive = map[int]map[string]interface{}{
		0: {"btn_type": "momentary", "btn_reverse": float64(0)}, // the device kept the old type
	}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, inputsDesired(
		SingleInputConfig{ID: 0, Mode: StringPtr("follow"), Inverted: BoolPtr(false)},
	), "SHIX3-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "inputs[0].btn_type", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, "momentary")
	assert.Equal(t, 1, result.AppliedCount, "the reverse flag was kept")
	assert.Equal(t, 1, result.FailedCount)
}

func TestApplyConfig_InputWriteFailureNamesEveryKeyOfTheChannel(t *testing.T) {
	client := newInputTestClient()
	client.inputSetErr = errors.New("503 busy")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, inputsDesired(
		SingleInputConfig{ID: 0, Mode: StringPtr("follow"), Inverted: BoolPtr(true)},
	), "SHIX3-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	var paths []string
	for _, f := range result.Failures {
		paths = append(paths, f.Path)
		assert.Equal(t, "503 busy", f.Error)
	}
	sort.Strings(paths)
	assert.Equal(t, []string{"inputs[0].btn_reverse", "inputs[0].btn_type"}, paths)
	assert.Equal(t, 2, result.FailedCount)
}

func TestApplyConfig_InputReadBackFailureIsReported(t *testing.T) {
	client := newApplyTestClient()
	client.relayGetErr = errors.New("timeout")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, inputsDesired(
		SingleInputConfig{ID: 0, Mode: StringPtr("follow")},
	), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "inputs[0].btn_type", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, "read-back failed")
}

func TestApplyConfig_ClientWithoutInputRouteFailsInsteadOfFallingBack(t *testing.T) {
	// A plain client has the relay surface but no /settings/input route, so an i3 input cannot be
	// written. Apply reports that and never sends a nested "inputs" document on POST /settings.
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, inputsDesired(
		SingleInputConfig{ID: 0, Mode: StringPtr("follow")},
	), "SHIX3-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "inputs", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, errNoInputClient.Error())
	assert.Empty(t, client.configs)
}

func unsupportedFor(t *testing.T, deviceType string, desired *DeviceConfiguration) []string {
	t.Helper()
	unsupported, err := unsupportedApplySettings(NewGen1Converter(nil), desired, deviceType, true)
	require.NoError(t, err)
	return unsupported
}

func TestUnsupportedApplySettings_InputRules(t *testing.T) {
	type tc struct {
		name       string
		deviceType string
		inputs     []SingleInputConfig
		want       []string
	}
	cases := []tc{
		{"mode and reverse on a relay model", "SHSW-1",
			[]SingleInputConfig{{ID: 0, Mode: StringPtr("flip"), Inverted: BoolPtr(true)}}, nil},
		{"all four modes on a Shelly 2.5", "SHSW-25",
			[]SingleInputConfig{
				{ID: 0, Mode: StringPtr("momentary")}, {ID: 1, Mode: StringPtr("detached")}}, nil},
		{"follow on an i3", "SHIX3-1",
			[]SingleInputConfig{{ID: 0, Mode: StringPtr("follow")}}, nil},
		{"flip is not a button type of an i3", "SHIX3-1",
			[]SingleInputConfig{{ID: 0, Mode: StringPtr("flip")}}, []string{"input.inputs[0].mode"}},
		{"detached is not a button type of an i3", "SHIX3-1",
			[]SingleInputConfig{{ID: 0, Mode: StringPtr("detached")}}, []string{"input.inputs[0].mode"}},
		{"a mode the device has no name for", "SHSW-1",
			[]SingleInputConfig{{ID: 0, Mode: StringPtr("bogus")}}, []string{"input.inputs[0].mode"}},
		{"a name is not written yet", "SHIX3-1",
			[]SingleInputConfig{{ID: 0, Name: StringPtr("Door")}}, []string{"input.inputs.0.name"}},
		{"an input type has no Gen1 setting", "SHSW-1",
			[]SingleInputConfig{{ID: 0, Type: StringPtr("button")}}, []string{"input.inputs.0.type"}},
		{"push actions are not written", "SHIX3-1",
			[]SingleInputConfig{{ID: 0, SinglePushAction: StringPtr("scene_1"), LongPushAction: StringPtr("off")}},
			[]string{"input.inputs.0.long_push_action", "input.inputs.0.single_push_action"}},
		{"an id that is not its position", "SHIX3-1",
			[]SingleInputConfig{{ID: 1, Mode: StringPtr("follow")}}, []string{"input.inputs[0].id"}},
		{"a plug has no inputs", "SHPLG-S",
			[]SingleInputConfig{{ID: 0, Mode: StringPtr("follow")}}, []string{"input"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, unsupportedFor(t, c.deviceType, inputsDesired(c.inputs...)))
		})
	}
}

func TestUnsupportedApplySettings_InputGlobalFieldsStayRefused(t *testing.T) {
	// The section-wide fields have no Gen1 setting: only the per-input list is written.
	desired := &DeviceConfiguration{Input: &InputConfig{DebounceTime: IntPtr(50)}}
	assert.Equal(t, []string{"input"}, unsupportedFor(t, "SHSW-1", desired))
}

func TestGen1Converter_InputsUseTheGen1ButtonKeys(t *testing.T) {
	converter := NewGen1Converter(nil)

	t.Run("an i3 reports its inputs array", func(t *testing.T) {
		config, err := converter.FromAPIConfig(json.RawMessage(
			`{"inputs":[{"name":"","btn_type":"toggle","btn_reverse":1},{"name":null,"btn_type":"momentary","btn_reverse":0}]}`), "SHIX3-1")
		require.NoError(t, err)
		require.NotNil(t, config.Input)
		require.Len(t, config.Input.Inputs, 2)
		assert.Equal(t, "follow", *config.Input.Inputs[0].Mode)
		assert.True(t, *config.Input.Inputs[0].Inverted)
		assert.Equal(t, "momentary", *config.Input.Inputs[1].Mode)
		assert.False(t, *config.Input.Inputs[1].Inverted)
	})

	t.Run("a relay model reports them in its relays", func(t *testing.T) {
		config, err := converter.FromAPIConfig(json.RawMessage(
			`{"relays":[{"btn_type":"edge","btn_reverse":0},{"btn_type":"detached","btn_reverse":1}]}`), "SHSW-25")
		require.NoError(t, err)
		require.NotNil(t, config.Input)
		require.Len(t, config.Input.Inputs, 2)
		assert.Equal(t, 0, config.Input.Inputs[0].ID)
		assert.Equal(t, "flip", *config.Input.Inputs[0].Mode)
		assert.False(t, *config.Input.Inputs[0].Inverted)
		assert.Equal(t, 1, config.Input.Inputs[1].ID)
		assert.Equal(t, "detached", *config.Input.Inputs[1].Mode)
		assert.True(t, *config.Input.Inputs[1].Inverted)
	})

	t.Run("a plug has no inputs even when a relay reports a button type", func(t *testing.T) {
		config, err := converter.FromAPIConfig(json.RawMessage(`{"relays":[{"btn_type":"toggle"}]}`), "SHPLG-S")
		require.NoError(t, err)
		assert.Nil(t, config.Input)
	})

	t.Run("a button type with no stored name is kept as the device spells it", func(t *testing.T) {
		config, err := converter.FromAPIConfig(json.RawMessage(`{"relays":[{"btn_type":"momentary_on_release"}]}`), "SHSW-1")
		require.NoError(t, err)
		require.NotNil(t, config.Input)
		assert.Equal(t, "momentary_on_release", *config.Input.Inputs[0].Mode)
	})

	t.Run("the export carries btn_type and a numeric btn_reverse, and no legacy keys", func(t *testing.T) {
		apiJSON, err := converter.ToAPIConfig(inputsDesired(
			SingleInputConfig{ID: 0, Mode: StringPtr("follow"), Inverted: BoolPtr(true),
				Type: StringPtr("button"), SinglePushAction: StringPtr("toggle")}), "SHIX3-1")
		require.NoError(t, err)
		var gen1 map[string]interface{}
		require.NoError(t, json.Unmarshal(apiJSON, &gen1))
		inputs := gen1["inputs"].([]interface{})
		assert.Equal(t, map[string]interface{}{"btn_type": "toggle", "btn_reverse": float64(1)}, inputs[0])
	})
}

func TestCountSettings_CountsTheInputKeysApplyWrites(t *testing.T) {
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	apiConfig := map[string]interface{}{
		"inputs": []interface{}{
			map[string]interface{}{"btn_type": "toggle", "btn_reverse": 1, "name": "ignored"},
			map[string]interface{}{"btn_reverse": 0},
		},
	}
	assert.Equal(t, 3, applier.countSettings(apiConfig))
}

// fakeInputDevice is a Gen1 device that keeps inputs between a write and GET /settings: an i3
// stores them in its "inputs" array, and a relay model in its "relays" array.
type fakeInputDevice struct {
	*fakeGen1Device
	relayModel bool
}

func newFakeInputDevice(relayModel bool) *fakeInputDevice {
	d := &fakeInputDevice{fakeGen1Device: newFakeGen1Device(), relayModel: relayModel}
	if relayModel {
		d.settings["relays"] = []interface{}{map[string]interface{}{"btn_type": "momentary", "btn_reverse": float64(0)}}
	} else {
		d.settings["inputs"] = []interface{}{
			map[string]interface{}{"name": "", "btn_type": "momentary", "btn_reverse": float64(0)},
			map[string]interface{}{"name": "", "btn_type": "momentary", "btn_reverse": float64(0)},
		}
	}
	return d
}

func (d *fakeInputDevice) store(key string, channel int, form map[string]interface{}) {
	if d.ignoreWrites {
		return
	}
	list := d.settings[key].([]interface{})
	item := list[channel].(map[string]interface{})
	for k, v := range form {
		if n, ok := v.(int); ok {
			v = float64(n)
		}
		item[k] = v
	}
}

func (d *fakeInputDevice) SetInputSettings(_ context.Context, input int, form map[string]interface{}) error {
	d.store("inputs", input, form)
	return nil
}

func (d *fakeInputDevice) GetInputSettings(_ context.Context, input int) (map[string]interface{}, error) {
	return d.settings["inputs"].([]interface{})[input].(map[string]interface{}), nil
}

func (d *fakeInputDevice) SetRelaySettings(_ context.Context, channel int, form map[string]interface{}) error {
	d.store("relays", channel, form)
	return nil
}

func (d *fakeInputDevice) GetRelaySettings(_ context.Context, channel int) (map[string]interface{}, error) {
	return d.settings["relays"].([]interface{})[channel].(map[string]interface{}), nil
}

func TestApplyDesiredConfig_AppliesI3InputsAndVerifiesThem(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "i3-hall")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("i3-hall")}},
		Input: &InputConfig{Inputs: []SingleInputConfig{
			{ID: 0, Mode: StringPtr("follow"), Inverted: BoolPtr(true)},
			{ID: 1, Mode: StringPtr("momentary")},
		}},
	})
	dev := newFakeInputDevice(false)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHIX3-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "differences: %+v", result.VerifyResult)
	inputs := dev.settings["inputs"].([]interface{})
	assert.Equal(t, "toggle", inputs[0].(map[string]interface{})["btn_type"])
	assert.Equal(t, float64(1), inputs[0].(map[string]interface{})["btn_reverse"])
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_AppliesRelayModelInputsAndVerifiesThem(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "sw-1")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("sw-1")}},
		Input:  &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("flip"), Inverted: BoolPtr(true)}}},
	})
	dev := newFakeInputDevice(true)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHSW-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "differences: %+v", result.VerifyResult)
	relay := dev.settings["relays"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "edge", relay["btn_type"])
	assert.Equal(t, float64(1), relay["btn_reverse"])
}

func TestApplyDesiredConfig_InputStaysPendingWhenDeviceIgnoresWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "i3-hall")
	setDesired(t, repo, &DeviceConfiguration{
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Mode: StringPtr("follow")}}},
	})
	dev := newFakeInputDevice(false)
	dev.ignoreWrites = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHIX3-1")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_RefusesAnInputNameBeforeTouchingTheDevice(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "i3-hall")
	setDesired(t, repo, &DeviceConfiguration{
		Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Name: StringPtr("Door")}}},
	})
	dev := newFakeInputDevice(false)

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHIX3-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "input.inputs.0.name")
	assert.Zero(t, dev.setCalls)
}

func TestDeviceSchemaInputModesAreAllApplicable(t *testing.T) {
	// The schema must offer only the modes apply accepts on that model: a schema-valid value that
	// apply refuses sends the user to a 422. TestDeviceSchemaApplicableMatchesApply samples the
	// first enum value only, so this walks every one.
	for _, model := range []string{"SHSW-1", "SHSW-PM", "SHSW-25", "SHIX3-1"} {
		leaves := map[string]map[string]interface{}{}
		schemaLeaves("", GetDeviceConfigurationSchema(model), leaves)
		modes, ok := leaves["input.inputs[].mode"]["enum"].([]string)
		require.True(t, ok, "%s: no mode enum", model)
		require.NotEmpty(t, modes)
		for _, mode := range modes {
			assert.Empty(t, unsupportedFor(t, model, inputsDesired(SingleInputConfig{ID: 0, Mode: StringPtr(mode)})),
				"%s: the schema offers mode %q but apply refuses it", model, mode)
		}
	}
}

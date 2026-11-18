package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A sparse override layer names the channel it changes by id, so the merge must pair channels by id
// and not by list position (ginsys/shelly-manager#537). The pointer fields are compared as values,
// so a wrong pairing fails an assertion and never dereferences nil.

func relayLayer(name string, relays ...SingleRelayConfig) ConfigLayer {
	return ConfigLayer{Name: name, Config: &DeviceConfiguration{Relay: &RelayConfig{Relays: relays}}}
}

func inputLayer(name string, inputs ...SingleInputConfig) ConfigLayer {
	return ConfigLayer{Name: name, Config: &DeviceConfiguration{Input: &InputConfig{Inputs: inputs}}}
}

func mergeRelays(t *testing.T, layers ...ConfigLayer) (*MergeResult, []SingleRelayConfig) {
	t.Helper()
	result, err := MergeConfigurations(layers)
	require.NoError(t, err)
	require.NotNil(t, result.Config.Relay)
	return result, result.Config.Relay.Relays
}

func TestMergeConfigurations_SparseOverrideKeepsEachSettingOnItsChannel(t *testing.T) {
	result, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 1, AutoOff: IntPtr(60)}),
		relayLayer("override", SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
	)

	require.Len(t, relays, 2)
	assert.Equal(t, 0, relays[0].ID)
	assert.Equal(t, IntPtr(30), relays[0].AutoOn)
	assert.Nil(t, relays[0].Name)
	assert.Nil(t, relays[0].AutoOff)

	assert.Equal(t, 1, relays[1].ID)
	assert.Equal(t, IntPtr(60), relays[1].AutoOff)
	assert.Equal(t, StringPtr("B"), relays[1].Name)
	assert.Nil(t, relays[1].AutoOn)

	assert.Equal(t, "template", result.Sources["relay.relays.0.auto_on"])
	assert.Equal(t, "template", result.Sources["relay.relays.1.auto_off"])
	assert.Equal(t, "override", result.Sources["relay.relays.1.name"])
}

func TestMergeConfigurations_SparseInputOverrideKeepsEachSettingOnItsChannel(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{
		inputLayer("template", SingleInputConfig{ID: 0, Type: StringPtr("switch")}, SingleInputConfig{ID: 1, Type: StringPtr("button")}),
		inputLayer("override", SingleInputConfig{ID: 1, Name: StringPtr("Door")}),
	})
	require.NoError(t, err)

	inputs := result.Config.Input.Inputs
	require.Len(t, inputs, 2)
	assert.Equal(t, 0, inputs[0].ID)
	assert.Equal(t, StringPtr("switch"), inputs[0].Type)
	assert.Nil(t, inputs[0].Name)
	assert.Equal(t, 1, inputs[1].ID)
	assert.Equal(t, StringPtr("button"), inputs[1].Type)
	assert.Equal(t, StringPtr("Door"), inputs[1].Name)
	assert.Equal(t, "override", result.Sources["input.inputs.1.name"])
}

// With no repeated id nothing refuses the old pairing: ids 0 and 2 plus an override for id 1 put
// channel 0's settings under id 1 and left no sign of it. The channel the template lacks goes in
// its id order, so the list still reads id 0, 1, 2.
func TestMergeConfigurations_OverrideForAnIdTheTemplateLacksIsInsertedInIdOrder(t *testing.T) {
	_, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 2, AutoOff: IntPtr(60)}),
		relayLayer("override", SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
	)

	require.Len(t, relays, 3)
	assert.Equal(t, 0, relays[0].ID)
	assert.Equal(t, IntPtr(30), relays[0].AutoOn)
	assert.Nil(t, relays[0].Name)
	assert.Equal(t, 1, relays[1].ID)
	assert.Equal(t, StringPtr("B"), relays[1].Name)
	assert.Nil(t, relays[1].AutoOn)
	assert.Equal(t, 2, relays[2].ID)
	assert.Equal(t, IntPtr(60), relays[2].AutoOff)
	assert.Nil(t, relays[2].Name)
}

// A lower layer that holds only channel 1, under a layer with channels 0 and 1, must give the list
// 0, 1: the Gen1 apply requires each channel's id to equal its position. Channel 0 takes nothing from
// channel 1's lower-layer settings.
func TestMergeConfigurations_ChannelFromALowerLayerKeepsIdOrder(t *testing.T) {
	result, relays := mergeRelays(t,
		relayLayer("lower", SingleRelayConfig{ID: 1, AutoOn: IntPtr(30)}),
		relayLayer("higher", SingleRelayConfig{ID: 0, Name: StringPtr("A")}, SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
	)

	require.Len(t, relays, 2)
	assert.Equal(t, 0, relays[0].ID)
	assert.Equal(t, StringPtr("A"), relays[0].Name)
	assert.Nil(t, relays[0].AutoOn)
	assert.Equal(t, 1, relays[1].ID)
	assert.Equal(t, StringPtr("B"), relays[1].Name)
	assert.Equal(t, IntPtr(30), relays[1].AutoOn)

	assert.Equal(t, "higher", result.Sources["relay.relays.0.name"])
	assert.Equal(t, "higher", result.Sources["relay.relays.1.name"])
	assert.Equal(t, "lower", result.Sources["relay.relays.1.auto_on"])
}

// A channel put in front of earlier ones moves their sources with them, including the sources an
// earlier layer wrote, and the keys the later layer writes stay where its channels are.
func TestMergeConfigurations_InsertedChannelMovesTheSourcesOfEarlierLayers(t *testing.T) {
	result, relays := mergeRelays(t,
		relayLayer("first", SingleRelayConfig{ID: 0, Name: StringPtr("A0")}, SingleRelayConfig{ID: 2, Name: StringPtr("C2")}),
		relayLayer("second", SingleRelayConfig{ID: 0, AutoOff: IntPtr(9)}),
		relayLayer("third", SingleRelayConfig{ID: 1, Name: StringPtr("B1")}),
	)

	require.Len(t, relays, 3)
	assert.Equal(t, []int{0, 1, 2}, []int{relays[0].ID, relays[1].ID, relays[2].ID})
	assert.Equal(t, StringPtr("C2"), relays[2].Name)

	assert.Equal(t, "first", result.Sources["relay.relays.0.name"])
	assert.Equal(t, "second", result.Sources["relay.relays.0.auto_off"])
	assert.Equal(t, "third", result.Sources["relay.relays.1.name"])
	assert.Equal(t, "first", result.Sources["relay.relays.2.name"])
	assert.NotContains(t, result.Sources, "relay.relays.3.name")
}

func TestMergeConfigurations_InsertedChannelsKeepIdOrderWhateverTheirOrder(t *testing.T) {
	_, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{ID: 0}, SingleRelayConfig{ID: 4}),
		relayLayer("override", SingleRelayConfig{ID: 3, Name: StringPtr("D")}, SingleRelayConfig{ID: 1, Name: StringPtr("B")}, SingleRelayConfig{ID: 5, Name: StringPtr("F")}),
	)

	require.Len(t, relays, 5)
	assert.Equal(t, []int{0, 1, 3, 4, 5}, []int{relays[0].ID, relays[1].ID, relays[2].ID, relays[3].ID, relays[4].ID})
	assert.Equal(t, StringPtr("B"), relays[1].Name)
	assert.Equal(t, StringPtr("D"), relays[2].Name)
	assert.Equal(t, StringPtr("F"), relays[4].Name)
}

// Guards, which pass with or without the insert rule: an id above every existing one is appended,
// and an element that is already in the list is never moved. A first layer is copied as it is.
func TestMergeConfigurations_InsertRuleNeverMovesAnExistingChannel(t *testing.T) {
	t.Run("a higher id is appended", func(t *testing.T) {
		_, relays := mergeRelays(t,
			relayLayer("template", SingleRelayConfig{ID: 0}, SingleRelayConfig{ID: 1}),
			relayLayer("override", SingleRelayConfig{ID: 2, Name: StringPtr("C")}),
		)
		require.Len(t, relays, 3)
		assert.Equal(t, []int{0, 1, 2}, []int{relays[0].ID, relays[1].ID, relays[2].ID})
		assert.Equal(t, StringPtr("C"), relays[2].Name)
	})

	t.Run("a stored list out of order stays out of order", func(t *testing.T) {
		_, relays := mergeRelays(t,
			relayLayer("template", SingleRelayConfig{ID: 1, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 0, AutoOff: IntPtr(60)}),
			relayLayer("override", SingleRelayConfig{ID: 0, Name: StringPtr("A")}),
		)
		require.Len(t, relays, 2)
		assert.Equal(t, 1, relays[0].ID)
		assert.Equal(t, IntPtr(30), relays[0].AutoOn)
		assert.Equal(t, 0, relays[1].ID)
		assert.Equal(t, StringPtr("A"), relays[1].Name)
		assert.Equal(t, IntPtr(60), relays[1].AutoOff)
	})

	t.Run("a first layer is copied in its own order", func(t *testing.T) {
		_, relays := mergeRelays(t, relayLayer("only", SingleRelayConfig{ID: 1}, SingleRelayConfig{ID: 0}))
		require.Len(t, relays, 2)
		assert.Equal(t, []int{1, 0}, []int{relays[0].ID, relays[1].ID})
	})
}

func TestMergeConfigurations_OverrideOrderDoesNotChangeThePairing(t *testing.T) {
	_, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 1, AutoOff: IntPtr(60)}),
		relayLayer("override", SingleRelayConfig{ID: 1, Name: StringPtr("B")}, SingleRelayConfig{ID: 0, Name: StringPtr("A")}),
	)

	require.Len(t, relays, 2)
	assert.Equal(t, StringPtr("A"), relays[0].Name)
	assert.Equal(t, IntPtr(30), relays[0].AutoOn)
	assert.Equal(t, StringPtr("B"), relays[1].Name)
	assert.Equal(t, IntPtr(60), relays[1].AutoOff)
}

// A template written without ids decodes every id as 0. For a list of more than one entry that
// means the list index, as the Gen1 apply reads it, so a sparse override still finds its channel.
func TestMergeConfigurations_IdlessTemplateTakesItsIdsFromPosition(t *testing.T) {
	result, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{AutoOn: IntPtr(30)}, SingleRelayConfig{AutoOff: IntPtr(60)}),
		relayLayer("override", SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
	)

	require.Len(t, relays, 2)
	assert.Equal(t, 0, relays[0].ID)
	assert.Equal(t, IntPtr(30), relays[0].AutoOn)
	assert.Nil(t, relays[0].Name)
	assert.Equal(t, 1, relays[1].ID)
	assert.Equal(t, IntPtr(60), relays[1].AutoOff)
	assert.Equal(t, StringPtr("B"), relays[1].Name)
	assert.Equal(t, "override", result.Sources["relay.relays.1.name"])
}

func TestMergeConfigurations_TwoIdlessListsPairByPositionAndAreNumbered(t *testing.T) {
	_, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{AutoOn: IntPtr(30)}, SingleRelayConfig{AutoOff: IntPtr(60)}),
		relayLayer("override", SingleRelayConfig{Name: StringPtr("A")}, SingleRelayConfig{Name: StringPtr("B")}),
	)

	require.Len(t, relays, 2)
	assert.Equal(t, 0, relays[0].ID)
	assert.Equal(t, StringPtr("A"), relays[0].Name)
	assert.Equal(t, IntPtr(30), relays[0].AutoOn)
	assert.Equal(t, 1, relays[1].ID)
	assert.Equal(t, StringPtr("B"), relays[1].Name)
	assert.Equal(t, IntPtr(60), relays[1].AutoOff)
}

// A one-entry list keeps the id it states: id 0 is channel 0.
func TestMergeConfigurations_SingleEntryOverrideKeepsItsStatedId(t *testing.T) {
	_, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}),
		relayLayer("override", SingleRelayConfig{Name: StringPtr("A")}),
	)

	require.Len(t, relays, 1)
	assert.Equal(t, 0, relays[0].ID)
	assert.Equal(t, StringPtr("A"), relays[0].Name)
	assert.Equal(t, IntPtr(30), relays[0].AutoOn)
}

// A list that repeats an id (not the all-zero pattern) cannot be paired by id, so it keeps the
// position merge it had.
func TestMergeConfigurations_RepeatedIdFallsBackToPosition(t *testing.T) {
	_, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 0, AutoOff: IntPtr(60)}, SingleRelayConfig{ID: 1}),
		relayLayer("override", SingleRelayConfig{ID: 0, Name: StringPtr("A")}),
	)

	require.Len(t, relays, 3)
	assert.Equal(t, StringPtr("A"), relays[0].Name)
	assert.Equal(t, IntPtr(30), relays[0].AutoOn)
	assert.Nil(t, relays[1].Name)
	assert.Equal(t, IntPtr(60), relays[1].AutoOff)
}

// The result never shares a channel, appended (id 2) or merged (id 1), with the layer it came from.
func TestMergeConfigurations_MergedChannelsDoNotAliasTheOverride(t *testing.T) {
	override := relayLayer("override",
		SingleRelayConfig{ID: 1, AutoOn: IntPtr(7), ScheduleRules: StringsPtr("0800-0123456-on")},
		SingleRelayConfig{ID: 2, AutoOn: IntPtr(5), ScheduleRules: StringsPtr("0900-0123456-on")},
	)
	_, relays := mergeRelays(t,
		relayLayer("template", SingleRelayConfig{ID: 0}, SingleRelayConfig{ID: 1}),
		override,
	)

	require.Len(t, relays, 3)
	for _, at := range []struct {
		index, id, autoOn int
		rule              string
	}{{1, 1, 7, "0800-0123456-on"}, {2, 2, 5, "0900-0123456-on"}} {
		require.Equal(t, at.id, relays[at.index].ID)
		require.NotNil(t, relays[at.index].AutoOn)
		require.NotNil(t, relays[at.index].ScheduleRules)
		*relays[at.index].AutoOn = 99
		(*relays[at.index].ScheduleRules)[0] = "changed"
		*relays[at.index].ScheduleRules = append(*relays[at.index].ScheduleRules, "more")

		source := override.Config.Relay.Relays[at.id-1]
		assert.Equal(t, IntPtr(at.autoOn), source.AutoOn)
		assert.Equal(t, StringsPtr(at.rule), source.ScheduleRules)
	}
}

// Merging the result again with the same override changes nothing, with ids 1 and 3 after the
// template's 0 and 1, and with ids that go in front of an existing one.
func TestMergeConfigurations_ChannelMergeIsIdempotent(t *testing.T) {
	template := relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 1, AutoOff: IntPtr(60)})
	override := relayLayer("override", SingleRelayConfig{ID: 1, Name: StringPtr("B")}, SingleRelayConfig{ID: 3, Name: StringPtr("D")})
	assertIdempotent(t, template, override)

	assertIdempotent(t,
		relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 2, AutoOff: IntPtr(60)}),
		relayLayer("override", SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
	)
}

func assertIdempotent(t *testing.T, template, override ConfigLayer) {
	t.Helper()

	first, err := MergeConfigurations([]ConfigLayer{template, override})
	require.NoError(t, err)
	second, err := MergeConfigurations([]ConfigLayer{{Name: "merged", Config: first.Config}, override})
	require.NoError(t, err)

	assert.Equal(t, first.Config, second.Config)
}

// The case ginsys/shelly-manager#537 asks for: a sparse override over a two-channel template gives
// the Gen2 apply a list in which every setting sits under its own channel and nothing is refused.
func TestMergeConfigurations_SparseOverrideReachesTheGen2ApplyOnItsOwnChannel(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{
		relayLayer("template", SingleRelayConfig{ID: 0, AutoOn: IntPtr(30)}, SingleRelayConfig{ID: 1, AutoOff: IntPtr(60)}),
		relayLayer("override", SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
	})
	require.NoError(t, err)

	paths, checked, _ := unmappedChannelSettings(result.Config)
	assert.Empty(t, paths)

	doc := map[string]interface{}{}
	gen2ChannelsToDoc(checked, doc)
	assert.Equal(t, map[string]interface{}{"auto_on": true, "auto_on_delay": 30}, doc["switch:0"])
	assert.Equal(t, map[string]interface{}{"name": "B", "auto_off": true, "auto_off_delay": 60}, doc["switch:1"])
}

package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The refusal of a negative channel id names the entry in the desired list that apply checks, which
// is the merged list. The merge pairs channels by id and adds a missing one in id order
// (ginsys/shelly-manager#537), so the index is the one in the merged list and not the one in the
// override body.

func TestUnsupportedGen2Settings_NamesANegativeIDAtItsIndexInTheMergedList(t *testing.T) {
	merged, err := MergeConfigurations([]ConfigLayer{
		relayLayer("template", SingleRelayConfig{ID: 0, Name: StringPtr("a")}, SingleRelayConfig{ID: 1, Name: StringPtr("b")}),
		relayLayer("override", SingleRelayConfig{ID: -1, Name: StringPtr("bad")}),
	})
	require.NoError(t, err)
	require.Len(t, merged.Config.Relay.Relays, 3)
	assert.Equal(t, -1, merged.Config.Relay.Relays[0].ID, "the entry the override added sits in id order")

	unsupported, err := unsupportedGen2Settings(NewGen2Converter(nil), merged.Config)

	require.NoError(t, err)
	assert.Equal(t, []string{"relay.relays.0.id"}, unsupported)
}

func TestUnsupportedGen2Settings_NamesANegativeIDThatBothLayersCarryOnce(t *testing.T) {
	merged, err := MergeConfigurations([]ConfigLayer{
		inputLayer("template", SingleInputConfig{ID: 0, Name: StringPtr("a")}, SingleInputConfig{ID: -1, Name: StringPtr("t")}),
		inputLayer("override", SingleInputConfig{ID: -1, Name: StringPtr("o")}),
	})
	require.NoError(t, err)
	require.Len(t, merged.Config.Input.Inputs, 2, "the override pairs with the template's channel -1")

	unsupported, err := unsupportedGen2Settings(NewGen2Converter(nil), merged.Config)

	require.NoError(t, err)
	assert.Equal(t, []string{"input.inputs.1.id"}, unsupported)
}

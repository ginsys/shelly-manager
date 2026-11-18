package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A PATCH body names a channel by id and lists a null as the path of a field inside that list
// element. The null removes the field of the channel with that id, wherever the merge put it
// (ginsys/shelly-manager#537).

func patchOverrides(t *testing.T, stored *DeviceConfiguration, body string) *DeviceConfiguration {
	t.Helper()
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)
	require.NoError(t, svc.SetDeviceOverrides(1, stored))

	var patch DeviceConfiguration
	require.NoError(t, json.Unmarshal([]byte(body), &patch))
	removed, err := NullPaths([]byte(body))
	require.NoError(t, err)
	require.NoError(t, svc.PatchDeviceOverrides(1, &patch, removed))

	got, err := svc.GetDeviceOverrides(1)
	require.NoError(t, err)
	return got
}

func storedRelays(relays ...SingleRelayConfig) *DeviceConfiguration {
	return &DeviceConfiguration{Relay: &RelayConfig{Relays: relays}}
}

func TestPatchDeviceOverrides_NullOnAnAppendedChannelLeavesTheFirstChannel(t *testing.T) {
	got := patchOverrides(t, storedRelays(SingleRelayConfig{ID: 0, Name: StringPtr("A")}),
		`{"relay":{"relays":[{"id":1,"name":null}]}}`)

	require.NotNil(t, got.Relay)
	require.Len(t, got.Relay.Relays, 2)
	assert.Equal(t, 0, got.Relay.Relays[0].ID)
	assert.Equal(t, StringPtr("A"), got.Relay.Relays[0].Name)
	assert.Equal(t, 1, got.Relay.Relays[1].ID)
	assert.Nil(t, got.Relay.Relays[1].Name)
}

func TestPatchDeviceOverrides_NullRemovesTheNameOfTheChannelItNames(t *testing.T) {
	got := patchOverrides(t,
		storedRelays(SingleRelayConfig{ID: 0, Name: StringPtr("A")}, SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
		`{"relay":{"relays":[{"id":1,"name":null}]}}`)

	require.Len(t, got.Relay.Relays, 2)
	assert.Equal(t, StringPtr("A"), got.Relay.Relays[0].Name)
	assert.Nil(t, got.Relay.Relays[1].Name)
}

func TestPatchDeviceOverrides_NullFollowsTheIdWhenThePatchIsInAnotherOrder(t *testing.T) {
	got := patchOverrides(t,
		storedRelays(SingleRelayConfig{ID: 0, Name: StringPtr("A")}, SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
		`{"relay":{"relays":[{"id":1,"name":"C"},{"id":0,"name":null}]}}`)

	require.Len(t, got.Relay.Relays, 2)
	assert.Nil(t, got.Relay.Relays[0].Name)
	assert.Equal(t, StringPtr("C"), got.Relay.Relays[1].Name)
}

func TestPatchDeviceOverrides_NullOnAnInputFollowsItsId(t *testing.T) {
	stored := &DeviceConfiguration{Input: &InputConfig{Inputs: []SingleInputConfig{{ID: 0, Name: StringPtr("A")}}}}
	got := patchOverrides(t, stored, `{"input":{"inputs":[{"id":1,"name":null}]}}`)

	require.NotNil(t, got.Input)
	require.Len(t, got.Input.Inputs, 2)
	assert.Equal(t, StringPtr("A"), got.Input.Inputs[0].Name)
	assert.Nil(t, got.Input.Inputs[1].Name)
}

// A stored list that repeats an id (here 1 twice; ids 0 and 0 would read as 0 and 1) is merged by
// position, so a null keeps the position it had.
func TestPatchDeviceOverrides_NullKeepsItsPositionWhenTheMergeFallsBackToPosition(t *testing.T) {
	got := patchOverrides(t,
		storedRelays(SingleRelayConfig{ID: 1, Name: StringPtr("A")}, SingleRelayConfig{ID: 1, Name: StringPtr("B")}),
		`{"relay":{"relays":[{"id":1,"name":null}]}}`)

	require.Len(t, got.Relay.Relays, 2)
	assert.Nil(t, got.Relay.Relays[0].Name)
	assert.Equal(t, StringPtr("B"), got.Relay.Relays[1].Name)
}

func TestPatchDeviceOverrides_NullOutsideAChannelListIsUnchanged(t *testing.T) {
	stored := &DeviceConfiguration{MQTT: &MQTTConfiguration{Server: StringPtr("broker"), Port: IntPtr(1883)}}
	got := patchOverrides(t, stored, `{"mqtt":{"server":null}}`)

	require.NotNil(t, got.MQTT)
	assert.Nil(t, got.MQTT.Server)
	assert.Equal(t, IntPtr(1883), got.MQTT.Port)
}

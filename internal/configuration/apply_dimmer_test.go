package configuration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dimmerDevice is a Gen1 Dimmer as the apply sees it: a fakeGen1Device that also keeps the
// per-channel light surface, POST and GET on /settings/light/<n>. A write is stored in the
// "lights" array of GET /settings, which is where a Dimmer reports its channels, and GET
// /settings/light/<n> answers from it unless lightLive states what the device says.
type dimmerDevice struct {
	*fakeGen1Device
	lightWrites []relayWrite
	lightLive   map[int]map[string]interface{}
}

func newDimmerDevice() *dimmerDevice {
	return &dimmerDevice{fakeGen1Device: newFakeGen1Device()}
}

func (d *dimmerDevice) lightDoc(channel int) map[string]interface{} {
	lights, _ := d.settings["lights"].([]interface{})
	for len(lights) <= channel {
		lights = append(lights, map[string]interface{}{})
	}
	d.settings["lights"] = lights
	doc, _ := lights[channel].(map[string]interface{})
	return doc
}

func (d *dimmerDevice) SetLightSettings(_ context.Context, channel int, form map[string]interface{}) error {
	d.lightWrites = append(d.lightWrites, relayWrite{channel: channel, form: form})
	doc := d.lightDoc(channel)
	for k, v := range form {
		// A device returns JSON numbers as float64.
		if n, ok := v.(int); ok {
			v = float64(n)
		}
		doc[k] = v
	}
	return nil
}

func (d *dimmerDevice) GetLightSettings(_ context.Context, channel int) (map[string]interface{}, error) {
	if live, ok := d.lightLive[channel]; ok {
		return live, nil
	}
	return d.lightDoc(channel), nil
}

// A Dimmer has no relay: its channel is /settings/light/<n>, and the reference lists no
// /settings/relay path for it. A desired relay is refused before any write, never sent to a path the
// device does not have.
func TestApplyDimmer_RelayIsRefusedBeforeAnyWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, &DeviceConfiguration{
		Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, AutoOff: IntPtr(60)}}},
	})
	dev := newDimmerDevice()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "relay")
	assert.Equal(t, 0, dev.setCalls, "nothing may be written to /settings")
	assert.Empty(t, dev.relayWrites, "nothing may reach /settings/relay/<n>")
	assert.Empty(t, dev.lightWrites, "nothing may reach /settings/light/<n>")
}

// hasRelay is the model check behind it. The six models that are supported today keep their answer,
// so their relay and roller results do not move; the Dimmer and the i3 have no relay.
func TestGen1Converter_RelayGateByModel(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, model := range []string{"SHPLG-S", "SHPLG-1", "SHSW-1", "SHSW-PM", "SHSW-25"} {
		assert.True(t, converter.hasRelay(model), model)
	}
	for _, model := range []string{"SHIX3-1", "SHDM-1", "SHDM-2"} {
		assert.False(t, converter.hasRelay(model), model)
	}
}

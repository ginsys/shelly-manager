package configuration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Gen1 reference for the Shelly 2.5 says of POST /settings max_power: "Overpower threshold in
// Watts for roller mode only. For relay mode the device keeps separate, per-channel
// configurations", and of POST /settings/relay/<n> max_power: "Set per-channel overpower threshold
// (used in relay mode)".

func maxPowerApplyClient(mode string) *applyTestClient {
	client := newApplyTestClient()
	client.currentRaw = json.RawMessage(`{"mode":"` + mode + `","max_power":4600,"relays":[{"max_power":4600},{"max_power":4600}]}`)
	return client
}

func applyMaxPower(t *testing.T, client *applyTestClient, deviceType string, desired *DeviceConfiguration) *ApplyResult {
	t.Helper()
	result, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), client, desired, deviceType)
	require.NoError(t, err)
	return result
}

func TestApplyConfig_MaxPowerOfA25InRelayModeGoesToEachChannel(t *testing.T) {
	client := maxPowerApplyClient("relay")

	result := applyMaxPower(t, client, "SHSW-25", &DeviceConfiguration{PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(3000)}})

	assert.True(t, result.Success, "failures: %+v", result.Failures)
	for _, cfg := range client.configs {
		assert.NotContains(t, cfg, "max_power", "POST /settings max_power is for roller mode only")
	}
	require.Len(t, client.relayWrites, 2)
	for channel, write := range client.relayWrites {
		assert.Equal(t, channel, write.channel)
		assert.Equal(t, map[string]interface{}{"max_power": float64(3000)}, write.form)
	}
	assert.Equal(t, 2, result.SettingsCount)
	assert.Equal(t, 2, result.AppliedCount)
}

// The control: in roller mode the same desired value goes to POST /settings, and no channel is written.
func TestApplyConfig_MaxPowerOfA25InRollerModeGoesToSettings(t *testing.T) {
	client := maxPowerApplyClient("roller")

	result := applyMaxPower(t, client, "SHSW-25", &DeviceConfiguration{PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(3000)}})

	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.configs, 1)
	assert.Equal(t, float64(3000), client.configs[0]["max_power"])
	assert.Empty(t, client.relayWrites)
}

// A channel's own value wins over the model-wide one, and the other channel gets the model-wide one.
func TestApplyConfig_MaxPowerOfAChannelBeatsTheModelWideValue(t *testing.T) {
	client := maxPowerApplyClient("relay")

	result := applyMaxPower(t, client, "SHSW-25", &DeviceConfiguration{
		PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(3000)},
		Relay: &RelayConfig{Relays: []SingleRelayConfig{
			{ID: 0, MaxPower: IntPtr(1200)},
			{ID: 1},
		}},
	})

	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.relayWrites, 2)
	assert.Equal(t, map[string]interface{}{"max_power": float64(1200)}, client.relayWrites[0].form)
	assert.Equal(t, map[string]interface{}{"max_power": float64(3000)}, client.relayWrites[1].form)
}

// Other models keep the model-wide value on POST /settings, as before.
func TestApplyConfig_MaxPowerOfA1PMStaysOnSettings(t *testing.T) {
	client := newApplyTestClient()
	client.currentRaw = json.RawMessage(`{"max_power":3500,"relays":[{"max_power":3500}]}`)

	result := applyMaxPower(t, client, "SHSW-PM", &DeviceConfiguration{PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(2000)}})

	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.configs, 1)
	assert.Equal(t, float64(2000), client.configs[0]["max_power"])
	assert.Empty(t, client.relayWrites)
}

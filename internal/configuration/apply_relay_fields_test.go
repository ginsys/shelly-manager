package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyConfig_RelayWritesTheScheduleApplianceTypeAndMaxPower(t *testing.T) {
	client := newApplyTestClient()
	client.currentRaw = json.RawMessage(`{"relays":[{}]}`)

	result := applyMaxPower(t, client, "SHSW-PM", &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{
		ID: 0, ApplianceType: StringPtr("heater"), Schedule: BoolPtr(true),
		ScheduleRules: StringsPtr("0700-0123456-on", "2200-0123456-off"), MaxPower: IntPtr(1500),
	}}}})

	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.relayWrites, 1)
	assert.Equal(t, map[string]interface{}{
		"appliance_type": "heater",
		"schedule":       true,
		"schedule_rules": []interface{}{"0700-0123456-on", "2200-0123456-off"},
		"max_power":      float64(1500),
	}, client.relayWrites[0].form)
	assert.Equal(t, 4, result.AppliedCount)
}

// A device answers 200 and keeps the old rules: the read-back catches it.
func TestApplyConfig_RelayScheduleRulesReadBackMismatchFails(t *testing.T) {
	client := newApplyTestClient()
	client.currentRaw = json.RawMessage(`{"relays":[{}]}`)
	client.relayLive = map[int]map[string]interface{}{
		0: {"schedule_rules": []interface{}{"0700-0123456-on"}},
	}

	result := applyMaxPower(t, client, "SHSW-PM", &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{
		ID: 0, ScheduleRules: StringsPtr("0700-0123456-on", "2200-0123456-off"),
	}}}})

	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "relays[0].schedule_rules", result.Failures[0].Path)
}

func TestApplyConfig_RelayScheduleRulesReadBackMatches(t *testing.T) {
	client := newApplyTestClient()
	client.currentRaw = json.RawMessage(`{"relays":[{}]}`)
	client.relayLive = map[int]map[string]interface{}{
		0: {"schedule_rules": []interface{}{"0700-0123456-on", "2200-0123456-off"}},
	}

	result := applyMaxPower(t, client, "SHSW-PM", &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{
		ID: 0, ScheduleRules: StringsPtr("0700-0123456-on", "2200-0123456-off"),
	}}}})

	assert.True(t, result.Success, "failures: %+v", result.Failures)
}

func TestUnsupportedApplySettings_RelayValues(t *testing.T) {
	converter := NewGen1Converter(nil)
	tooMany := make([]string, gen1MaxScheduleRules+1)
	for i := range tooMany {
		tooMany[i] = "0000-0123456-on"
	}
	cases := map[string]struct {
		relay SingleRelayConfig
		want  string
	}{
		"negative channel max power": {SingleRelayConfig{MaxPower: IntPtr(-1)}, "relay.relays[0].max_power"},
		"empty rule":                 {SingleRelayConfig{ScheduleRules: StringsPtr("")}, "relay.relays[0].schedule_rules"},
		"rule with a comma":          {SingleRelayConfig{ScheduleRules: StringsPtr("0000-0123456-on,1000-0123456-off")}, "relay.relays[0].schedule_rules"},
		"too many rules":             {SingleRelayConfig{ScheduleRules: &tooMany}, "relay.relays[0].schedule_rules"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			desired := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{tc.relay}}}
			unsupported, err := unsupportedApplySettings(converter, desired, "SHSW-25", false)
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, unsupported)
		})
	}

	negative := &DeviceConfiguration{PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(-5)}}
	unsupported, err := unsupportedApplySettings(converter, negative, "SHSW-PM", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"power_metering.max_power"}, unsupported)
}

// The read-back of a Shelly 2.5 in relay mode reports the model-wide max power as the value every
// channel carries, so verify can compare what apply routed onto the channels.
func TestGen1Converter_MaxPowerOfA25InRelayModeIsTheChannelsCommonValue(t *testing.T) {
	converter := NewGen1Converter(nil)
	read := func(raw string) *DeviceConfiguration {
		config, err := converter.FromAPIConfig(json.RawMessage(raw), "SHSW-25")
		require.NoError(t, err)
		return config
	}

	same := read(`{"mode":"relay","max_power":4600,"relays":[{"max_power":3000},{"max_power":3000}]}`)
	require.NotNil(t, same.PowerMetering)
	assert.Equal(t, 3000, *same.PowerMetering.MaxPower)
	require.Len(t, same.Relay.Relays, 2)
	assert.Equal(t, 3000, *same.Relay.Relays[1].MaxPower)

	assert.Nil(t, read(`{"mode":"relay","max_power":4600,"relays":[{"max_power":3000},{"max_power":1000}]}`).PowerMetering)

	roller := read(`{"mode":"roller","max_power":4600,"relays":[{"max_power":3000},{"max_power":3000}]}`)
	require.NotNil(t, roller.PowerMetering)
	assert.Equal(t, 4600, *roller.PowerMetering.MaxPower)
}

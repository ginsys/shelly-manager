package configuration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func verifyRelay(t *testing.T, deviceType, raw string, desired *DeviceConfiguration) *VerifyResult {
	t.Helper()
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(raw)
	result, err := NewConfigVerifier(NewGen1Converter(nil), nil).VerifyConfig(context.Background(), client, desired, deviceType)
	require.NoError(t, err)
	return result
}

// Apply writes the model-wide max power to every channel that states none of its own, so a channel
// that differs by design is no drift, and a channel that does not carry the value is.
func TestVerifyConfig_A25RelayModeMaxPowerIsComparedPerChannel(t *testing.T) {
	desired := &DeviceConfiguration{
		PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(3000)},
		Relay:         &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, MaxPower: IntPtr(1200)}}},
	}

	applied := verifyRelay(t, "SHSW-25", `{"mode":"relay","relays":[{"max_power":1200},{"max_power":3000}]}`, desired)
	assert.True(t, applied.Match, "differences: %+v", applied.Differences)

	missed := verifyRelay(t, "SHSW-25", `{"mode":"relay","relays":[{"max_power":1200},{"max_power":1000}]}`, desired)
	require.False(t, missed.Match)
	require.Len(t, missed.Differences, 1)
	assert.Equal(t, "relay.relays.1.max_power", missed.Differences[0].Path)

	// The caller's configuration is not changed.
	assert.Equal(t, 3000, *desired.PowerMetering.MaxPower)
	assert.Len(t, desired.Relay.Relays, 1)
}

func TestVerifyConfig_A25RollerModeKeepsTheModelWideMaxPower(t *testing.T) {
	desired := &DeviceConfiguration{PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(3000)}}

	assert.True(t, verifyRelay(t, "SHSW-25", `{"mode":"roller","max_power":3000,"relays":[{"max_power":100},{"max_power":200}]}`, desired).Match)
	assert.False(t, verifyRelay(t, "SHSW-25", `{"mode":"roller","max_power":2000,"relays":[{"max_power":3000},{"max_power":3000}]}`, desired).Match)
}

// The schedule is the whole list: a rule left on the device is drift, an empty desired list asks for
// none, and an unstated list is left alone.
func TestVerifyConfig_ScheduleRulesMustMatchWhole(t *testing.T) {
	const raw = `{"relays":[{"schedule_rules":["0700-0123456-on","2200-0123456-off"]}]}`
	rules := func(list *[]string) *DeviceConfiguration {
		return &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, ScheduleRules: list}}}}
	}

	assert.True(t, verifyRelay(t, "SHSW-PM", raw, rules(StringsPtr("0700-0123456-on", "2200-0123456-off"))).Match)
	assert.True(t, verifyRelay(t, "SHSW-PM", raw, rules(nil)).Match, "an unstated list is not compared")

	extra := verifyRelay(t, "SHSW-PM", raw, rules(StringsPtr("0700-0123456-on")))
	require.False(t, extra.Match)
	assert.Equal(t, "relay.relays.0.schedule_rules.1", extra.Differences[0].Path)

	assert.False(t, verifyRelay(t, "SHSW-PM", raw, rules(StringsPtr())).Match, "an empty list asks for no rules")
	assert.True(t, verifyRelay(t, "SHSW-PM", `{"relays":[{"schedule_rules":[]}]}`, rules(StringsPtr())).Match)
	assert.False(t, verifyRelay(t, "SHSW-PM", `{"relays":[{"schedule_rules":["0700-0123456-on"]}]}`,
		rules(StringsPtr("0700-0123456-on", "2200-0123456-off"))).Match, "a missing rule is drift")
}

// An empty rule list is a request to clear the rules, so it survives a save and a reload.
func TestSingleRelayConfig_EmptyScheduleRulesSurviveSerialization(t *testing.T) {
	for name, list := range map[string]*[]string{"empty": StringsPtr(), "set": StringsPtr("0000-0123456-on"), "unset": nil} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(SingleRelayConfig{ID: 0, ScheduleRules: list})
			require.NoError(t, err)
			var back SingleRelayConfig
			require.NoError(t, json.Unmarshal(data, &back))
			assert.Equal(t, list, back.ScheduleRules)
		})
	}
}

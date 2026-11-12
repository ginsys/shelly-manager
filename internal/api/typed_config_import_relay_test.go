package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An imported Gen1 relay entry keeps its appliance type, schedule rules and max power per channel,
// and the appliance type is never taken for the relay name.
func TestConvertRelayConfig_ImportKeepsApplianceTypeScheduleRulesAndMaxPower(t *testing.T) {
	h := &Handler{}

	t.Run("relays array", func(t *testing.T) {
		relay, _ := h.convertRelayConfig(map[string]interface{}{
			"relays": []interface{}{
				map[string]interface{}{
					"appliance_type": "Heater",
					"schedule_rules": []interface{}{"0700-0123456-on", "2200-0123456-off"},
					"max_power":      float64(1200),
				},
				map[string]interface{}{
					"name":           "Pump",
					"appliance_type": "Pump",
					"schedule_rules": []interface{}{},
					"max_power":      float64(0),
				},
			},
		}, "SHSW-25")

		require.Len(t, relay.Relays, 2)
		first, second := relay.Relays[0], relay.Relays[1]

		assert.Nil(t, first.Name, "the appliance type is not a name")
		require.NotNil(t, first.ApplianceType)
		assert.Equal(t, "Heater", *first.ApplianceType)
		require.NotNil(t, first.ScheduleRules)
		assert.Equal(t, []string{"0700-0123456-on", "2200-0123456-off"}, *first.ScheduleRules)
		require.NotNil(t, first.MaxPower)
		assert.Equal(t, 1200, *first.MaxPower)

		require.NotNil(t, second.Name)
		assert.Equal(t, "Pump", *second.Name)
		require.NotNil(t, second.ApplianceType)
		assert.Equal(t, "Pump", *second.ApplianceType)
		require.NotNil(t, second.ScheduleRules, "an empty list is kept, it means no rules")
		assert.Empty(t, *second.ScheduleRules)
		require.NotNil(t, second.MaxPower)
		assert.Equal(t, 0, *second.MaxPower)
	})

	t.Run("an entry that states none of the three leaves them unset", func(t *testing.T) {
		relay, _ := h.convertRelayConfig(map[string]interface{}{
			"relays": []interface{}{map[string]interface{}{"name": "Lamp"}},
		}, "SHSW-1")

		require.Len(t, relay.Relays, 1)
		assert.Nil(t, relay.Relays[0].ApplianceType)
		assert.Nil(t, relay.Relays[0].ScheduleRules)
		assert.Nil(t, relay.Relays[0].MaxPower)
	})

	t.Run("a fractional max_power reads as no value, not its truncation", func(t *testing.T) {
		relay, _ := h.convertRelayConfig(map[string]interface{}{
			"relays": []interface{}{map[string]interface{}{"max_power": float64(7.5)}},
		}, "SHSW-25")

		require.Len(t, relay.Relays, 1)
		assert.Nil(t, relay.Relays[0].MaxPower)
	})

	t.Run("relay_0 and relay_1", func(t *testing.T) {
		relay, _ := h.convertRelayConfig(map[string]interface{}{
			"relay_0": map[string]interface{}{
				"appliance_type": "Heater",
				"schedule_rules": []interface{}{"0700-0123456-on"},
				"max_power":      float64(900),
			},
		}, "SHSW-25")

		require.Len(t, relay.Relays, 1)
		channel := relay.Relays[0]
		require.NotNil(t, channel.ApplianceType)
		assert.Equal(t, "Heater", *channel.ApplianceType)
		require.NotNil(t, channel.ScheduleRules)
		assert.Equal(t, []string{"0700-0123456-on"}, *channel.ScheduleRules)
		require.NotNil(t, channel.MaxPower)
		assert.Equal(t, 900, *channel.MaxPower)
	})
}

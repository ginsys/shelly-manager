package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A template with no model holds the widest bound, so a Bulb or a group of Bulbs can carry effect 5 and
// 6; a model's own schema narrows it: the RGBW2 stops at 4, the Bulb at 6.
func TestValidateConfigBodyEffectBoundFollowsTheModel(t *testing.T) {
	body := func(effect int) map[string]interface{} {
		return map[string]interface{}{"light": map[string]interface{}{"lights": []interface{}{
			map[string]interface{}{"id": float64(0), "effect": float64(effect)},
		}}}
	}
	cases := []struct {
		model  string
		effect int
		valid  bool
	}{
		{"", 6, true}, {"", 7, false},
		{"SHBLB-1", 6, true}, {"SHBLB-1", 7, false},
		{"SHRGBW2", 4, true}, {"SHRGBW2", 5, false},
	}
	for _, tc := range cases {
		errs := ValidateConfigBody(tc.model, body(tc.effect))
		assert.Equal(t, tc.valid, len(errs) == 0, "model %q effect %d: %v", tc.model, tc.effect, errs)
	}
}

// reset is an action, never a setting: the key list apply sends holds neither it nor a runtime key.
func TestApplyBulbLight_KeyListHoldsOnlyTheSixSettings(t *testing.T) {
	assert.ElementsMatch(t, []string{"default_state", "auto_on", "auto_off", "effect", "schedule", "schedule_rules"}, gen1BulbLightKeys)
	assert.NotContains(t, gen1BulbLightKeys, "reset")
}

// The schema of the Bulb: the six page keys, effect bounded to 0 to 6, the three default states, no
// key of the RGBW2 or the Dimmer, and the mode with its two words. The Bulb is not a color/white
// model, so the Dimmer's narrowing must not take it (it would drop effect).
func TestDeviceSchemaBulbHoldsItsOwnLightKeys(t *testing.T) {
	leaves := map[string]map[string]interface{}{}
	schemaLeaves("", GetDeviceConfigurationSchema("SHBLB-1"), leaves)

	for _, key := range gen1BulbLightKeys {
		leaf, ok := leaves["light.lights[]."+key]
		require.True(t, ok, "no schema leaf %s", key)
		assert.Equal(t, true, leaf[SchemaKeyApplicable], key)
	}
	assert.EqualValues(t, gen1BulbMaxEffect, leaves["light.lights[].effect"]["maximum"])
	assert.Equal(t, []string{"on", "off", "last"}, leaves["light.lights[].default_state"]["enum"])
	for _, key := range []string{"name", "transition", "btn_reverse", "btn_type", "btn_debounce", "swap_inputs"} {
		assert.NotContains(t, leaves, "light.lights[]."+key)
	}
	mode, ok := leaves["system.device.mode"]
	require.True(t, ok)
	assert.Equal(t, []string{"white", "color"}, mode["enum"])
	assert.Equal(t, true, mode[SchemaKeyApplicable])
	for path := range leaves {
		assert.NotContains(t, path, "dimming.", "the Bulb has no Dimmer group")
	}
}

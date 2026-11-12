package configuration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The roller keys beyond the travel times, as the Gen1 reference lists them for the Shelly 2.5
// (POST /settings/roller/<n>). The device calls the button type btn_type when written and
// button_type when read, and keeps btn_reverse as a number.
func rollerRemainderConfig() *DeviceConfiguration {
	return &DeviceConfiguration{Roller: &RollerConfig{
		DefaultState:           StringPtr("open"),
		ButtonType:             StringPtr("toggle"),
		ButtonReverse:          BoolPtr(true),
		Schedule:               BoolPtr(true),
		ScheduleRules:          StringsPtr("0700-0123456-on"),
		ObstacleMode:           StringPtr("while_closing"),
		ObstacleAction:         StringPtr("reverse"),
		ObstaclePower:          IntPtr(250),
		ObstacleDelay:          IntPtr(2),
		SafetyMode:             StringPtr("while_moving"),
		SafetyAction:           StringPtr("pause"),
		SafetyAllowedOnTrigger: StringPtr("open"),
		OffPower:               IntPtr(3),
		EndsDelay:              IntPtr(500),
	}}
}

// The shape of the red-first route test: every remaining key reaches POST /settings/roller/0 under
// the device's own name, and is read back.
func TestApplyConfig_RollerWritesTheRemainingKeys(t *testing.T) {
	client := newRollerApplyClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, rollerRemainderConfig(), "SHSW-25")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.rollerWrites, 1)
	assert.Equal(t, 0, client.rollerWrites[0].channel)
	assert.Equal(t, map[string]interface{}{
		"default_state":             "open",
		"btn_type":                  "toggle",
		"btn_reverse":               float64(1),
		"schedule":                  true,
		"schedule_rules":            []interface{}{"0700-0123456-on"},
		"obstacle_mode":             "while_closing",
		"obstacle_action":           "reverse",
		"obstacle_power":            float64(250),
		"obstacle_delay":            float64(2),
		"safety_mode":               "while_moving",
		"safety_action":             "pause",
		"safety_allowed_on_trigger": "open",
		"off_power":                 float64(3),
		"ends_delay":                float64(500),
	}, client.rollerWrites[0].form)
	assert.Equal(t, 14, result.AppliedCount)
}

// The button type goes out as btn_type and comes back as button_type, so the read-back looks for
// the second name. A device that keeps the old type fails under the written name.
func TestApplyConfig_RollerButtonTypeIsReadBackAsButtonType(t *testing.T) {
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	config := &DeviceConfiguration{Roller: &RollerConfig{ButtonType: StringPtr("toggle")}}

	kept := newRollerApplyClient()
	kept.rollerLive = map[int]map[string]interface{}{0: {"button_type": "toggle"}}
	result, err := applier.ApplyConfig(context.Background(), kept, config, "SHSW-25")
	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	assert.Equal(t, map[string]interface{}{"btn_type": "toggle"}, kept.rollerWrites[0].form)

	dropped := newRollerApplyClient()
	dropped.rollerLive = map[int]map[string]interface{}{0: {"button_type": "momentary"}}
	result, err = applier.ApplyConfig(context.Background(), dropped, config, "SHSW-25")
	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "rollers[0].btn_type", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, "momentary")
}

// A value the device would not keep is refused before the device is touched: an enum outside the
// reference's list, a negative number, and an ends delay off the 0 to 10000 step of 100.
func TestUnsupportedApplySettings_RollerRemainderValues(t *testing.T) {
	converter := NewGen1Converter(nil)
	refused := func(t *testing.T, roller *RollerConfig) []string {
		t.Helper()
		roller.MaxOpenTime = IntPtr(20)
		unsupported, err := unsupportedApplySettings(converter, &DeviceConfiguration{Roller: roller}, "SHSW-25", true)
		require.NoError(t, err)
		return unsupported
	}

	t.Run("every remaining key is accepted with a listed value", func(t *testing.T) {
		assert.Empty(t, refused(t, rollerRemainderConfig().Roller))
	})

	t.Run("an enum outside the reference is refused", func(t *testing.T) {
		assert.Equal(t, []string{"roller.default_state"}, refused(t, &RollerConfig{DefaultState: StringPtr("half")}))
		assert.Equal(t, []string{"roller.button_type"}, refused(t, &RollerConfig{ButtonType: StringPtr("rocker")}))
		assert.Equal(t, []string{"roller.obstacle_mode"}, refused(t, &RollerConfig{ObstacleMode: StringPtr("always")}))
		assert.Equal(t, []string{"roller.obstacle_action"}, refused(t, &RollerConfig{ObstacleAction: StringPtr("pause")}))
		assert.Equal(t, []string{"roller.safety_mode"}, refused(t, &RollerConfig{SafetyMode: StringPtr("always")}))
		assert.Equal(t, []string{"roller.safety_action"}, refused(t, &RollerConfig{SafetyAction: StringPtr("open")}))
		assert.Equal(t, []string{"roller.safety_allowed_on_trigger"}, refused(t, &RollerConfig{SafetyAllowedOnTrigger: StringPtr("some")}))
	})

	t.Run("a negative number is refused", func(t *testing.T) {
		assert.Equal(t, []string{"roller.obstacle_power"}, refused(t, &RollerConfig{ObstaclePower: IntPtr(-1)}))
		assert.Equal(t, []string{"roller.obstacle_delay"}, refused(t, &RollerConfig{ObstacleDelay: IntPtr(-1)}))
		assert.Equal(t, []string{"roller.off_power"}, refused(t, &RollerConfig{OffPower: IntPtr(-1)}))
	})

	t.Run("an ends delay is 0 to 10000 in steps of 100", func(t *testing.T) {
		for _, bad := range []int{-100, 150, 10100} {
			assert.Equal(t, []string{"roller.ends_delay"}, refused(t, &RollerConfig{EndsDelay: IntPtr(bad)}), "%d ms", bad)
		}
		for _, good := range []int{0, 100, 10000} {
			assert.Empty(t, refused(t, &RollerConfig{EndsDelay: IntPtr(good)}), "%d ms", good)
		}
	})

	t.Run("a schedule rule the device could not keep is refused", func(t *testing.T) {
		assert.Equal(t, []string{"roller.schedule_rules"}, refused(t, &RollerConfig{ScheduleRules: StringsPtr("")}))
		assert.Equal(t, []string{"roller.schedule_rules"}, refused(t, &RollerConfig{ScheduleRules: StringsPtr("0700-0123456-on,2200-0123456-off")}))
		tooMany := make([]string, gen1MaxScheduleRules+1)
		for i := range tooMany {
			tooMany[i] = "0700-0123456-on"
		}
		assert.Equal(t, []string{"roller.schedule_rules"}, refused(t, &RollerConfig{ScheduleRules: &tooMany}))
	})
}

// Positioning is not part of this change: its calibration is a separate action, so a stored
// positioning flag and the other fields with no applicable key stay refused.
func TestUnsupportedApplySettings_RollerPositioningStaysRefused(t *testing.T) {
	config := &DeviceConfiguration{Roller: &RollerConfig{
		MaxOpenTime: IntPtr(20), PositioningEnabled: BoolPtr(true), SafetySwitch: BoolPtr(true),
	}}

	unsupported, err := unsupportedApplySettings(NewGen1Converter(nil), config, "SHSW-25", true)

	require.NoError(t, err)
	assert.Equal(t, []string{"roller.positioning_enabled", "roller.safety_switch"}, unsupported)
}

// A model without roller mode refuses the section, whichever key it names.
func TestUnsupportedApplySettings_RollerRemainderNeedsRollerMode(t *testing.T) {
	for _, model := range []string{"SHSW-1", "SHSW-PM", "SHPLG-S"} {
		unsupported, err := unsupportedApplySettings(NewGen1Converter(nil), rollerRemainderConfig(), model, true)
		require.NoError(t, err)
		assert.Equal(t, []string{"roller"}, unsupported, model)
	}
}

// An unstated rule list is left alone, a stated one is written whole, and an empty one clears the
// rules. Verify compares the whole list, so a rule left on the device is drift.
func TestRollerScheduleRules_LeaveClearAndCompareWhole(t *testing.T) {
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	apply := func(rules *[]string) map[string]interface{} {
		client := newRollerApplyClient()
		result, err := applier.ApplyConfig(context.Background(), client,
			&DeviceConfiguration{Roller: &RollerConfig{MaxOpenTime: IntPtr(20), ScheduleRules: rules}}, "SHSW-25")
		require.NoError(t, err)
		assert.True(t, result.Success, "failures: %+v", result.Failures)
		require.Len(t, client.rollerWrites, 1)
		return client.rollerWrites[0].form
	}

	assert.NotContains(t, apply(nil), "schedule_rules", "an unstated list is not written")
	assert.Equal(t, []interface{}{}, apply(StringsPtr())["schedule_rules"], "an empty list clears the rules")
	assert.Equal(t, []interface{}{"0700-0123456-on", "2200-0123456-off"},
		apply(StringsPtr("0700-0123456-on", "2200-0123456-off"))["schedule_rules"])

	const live = `{"mode":"roller","rollers":[{"maxtime_open":20,"schedule_rules":["0700-0123456-on","2200-0123456-off"]}]}`
	desired := func(rules *[]string) *DeviceConfiguration {
		return &DeviceConfiguration{Roller: &RollerConfig{MaxOpenTime: IntPtr(20), ScheduleRules: rules}}
	}
	assert.True(t, verifyRelay(t, "SHSW-25", live, desired(StringsPtr("0700-0123456-on", "2200-0123456-off"))).Match)
	assert.True(t, verifyRelay(t, "SHSW-25", live, desired(nil)).Match, "an unstated list is not compared")
	assert.False(t, verifyRelay(t, "SHSW-25", live, desired(StringsPtr("0700-0123456-on"))).Match, "a rule left on the device is drift")
	assert.False(t, verifyRelay(t, "SHSW-25", live, desired(StringsPtr())).Match, "an empty list asks for no rules")
}

// A saved empty rule list survives a save and a reload, and the layer merge.
func TestRollerConfig_EmptyScheduleRulesSurviveSerialization(t *testing.T) {
	for name, list := range map[string]*[]string{"empty": StringsPtr(), "set": StringsPtr("0000-0123456-on"), "unset": nil} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(RollerConfig{ScheduleRules: list})
			require.NoError(t, err)
			var back RollerConfig
			require.NoError(t, json.Unmarshal(data, &back))
			assert.Equal(t, list, back.ScheduleRules)
		})
	}
}

// The converter maps every remaining key in both directions, with the device's own names and the
// 0 or 1 of btn_reverse.
func TestGen1Converter_RollerRemainderRoundTrip(t *testing.T) {
	const gen1 = `{"rollers":[{
		"maxtime_open":20,"maxtime_close":22,
		"default_state":"open","button_type":"toggle","btn_reverse":1,
		"schedule":true,"schedule_rules":["0700-0123456-on"],
		"obstacle_mode":"while_closing","obstacle_action":"reverse","obstacle_power":250,"obstacle_delay":2,
		"safety_mode":"while_moving","safety_action":"pause","safety_allowed_on_trigger":"open",
		"off_power":3,"ends_delay":500}]}`
	converter := NewGen1Converter(nil)

	config, err := converter.FromAPIConfig(json.RawMessage(gen1), "SHSW-25")
	require.NoError(t, err)
	want := rollerRemainderConfig().Roller
	want.MaxOpenTime = IntPtr(20)
	want.MaxCloseTime = IntPtr(22)
	assert.Equal(t, want, config.Roller)

	raw, err := converter.ToAPIConfig(&DeviceConfiguration{Roller: want}, "SHSW-25")
	require.NoError(t, err)
	var back map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &back))
	rollers, ok := back["rollers"].([]interface{})
	require.True(t, ok)
	require.Len(t, rollers, 1)
	roller := rollers[0].(map[string]interface{})
	assert.Equal(t, "toggle", roller["btn_type"])
	assert.Equal(t, float64(1), roller["btn_reverse"])
	assert.Equal(t, float64(500), roller["ends_delay"])
	assert.Equal(t, []interface{}{"0700-0123456-on"}, roller["schedule_rules"])
}

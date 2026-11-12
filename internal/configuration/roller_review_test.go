package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A layer's roller schedule_rules replaces the lower layer's list whole, an empty list (clear the
// rules) included, and an unstated list leaves the lower one alone.
func TestMergeConfigurations_RollerScheduleRulesReplaceWhole(t *testing.T) {
	roller := func(rules *[]string) *DeviceConfiguration {
		return &DeviceConfiguration{Roller: &RollerConfig{MaxOpenTime: IntPtr(20), ScheduleRules: rules}}
	}
	global := ConfigLayer{Name: "global", Config: roller(StringsPtr("0700-0123456-on", "2200-0123456-off"))}

	cleared, err := MergeConfigurations([]ConfigLayer{global, {Name: "device", Config: roller(StringsPtr())}})
	require.NoError(t, err)
	require.NotNil(t, cleared.Config.Roller.ScheduleRules)
	assert.Empty(t, *cleared.Config.Roller.ScheduleRules)
	assert.Equal(t, "device", cleared.Sources["roller.schedule_rules"])

	replaced, err := MergeConfigurations([]ConfigLayer{global, {Name: "device", Config: roller(StringsPtr("0900-0123456-on"))}})
	require.NoError(t, err)
	require.NotNil(t, replaced.Config.Roller.ScheduleRules)
	assert.Equal(t, []string{"0900-0123456-on"}, *replaced.Config.Roller.ScheduleRules)

	kept, err := MergeConfigurations([]ConfigLayer{global, {Name: "device", Config: roller(nil)}})
	require.NoError(t, err)
	require.NotNil(t, kept.Config.Roller.ScheduleRules)
	assert.Equal(t, []string{"0700-0123456-on", "2200-0123456-off"}, *kept.Config.Roller.ScheduleRules)
	assert.Equal(t, "global", kept.Sources["roller.schedule_rules"])
}

// The device reports a whole number for these keys. A fraction is no whole number, so it is not read
// as its truncation: a desired 2 must not verify against a live 2.5.
func TestVerifyConfig_RollerNumberWithAFractionIsDrift(t *testing.T) {
	for _, key := range []string{"off_power", "obstacle_power", "obstacle_delay"} {
		t.Run(key, func(t *testing.T) {
			desired := &DeviceConfiguration{Roller: &RollerConfig{}}
			switch key {
			case "off_power":
				desired.Roller.OffPower = IntPtr(2)
			case "obstacle_power":
				desired.Roller.ObstaclePower = IntPtr(2)
			default:
				desired.Roller.ObstacleDelay = IntPtr(2)
			}

			whole := verifyRelay(t, "SHSW-25", `{"mode":"roller","rollers":[{"`+key+`":2}]}`, desired)
			assert.True(t, whole.Match, "differences: %+v", whole.Differences)

			fraction := verifyRelay(t, "SHSW-25", `{"mode":"roller","rollers":[{"`+key+`":2.5}]}`, desired)
			assert.False(t, fraction.Match, "a live 2.5 must not match a desired 2")
		})
	}
}

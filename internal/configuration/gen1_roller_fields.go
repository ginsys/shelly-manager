package configuration

import (
	"math"
	"slices"
	"strings"
)

// The Shelly 2.5 roller keys beyond the travel times, direction and inputs (POST and GET
// /settings/roller/<n>). The names and the values below are read from the Gen1 reference and are
// unconfirmed on hardware. The stored form keeps the device's own spelling for all of them; only the
// button type and its reverse flag are named in full (button_type, button_reverse), where the device
// says btn_type and btn_reverse.
var (
	gen1RollerStates         = []string{"stop", "open", "close"}
	gen1RollerButtonTypes    = []string{"momentary", "toggle", "detached", "action"}
	gen1RollerDetectModes    = []string{"disabled", "while_opening", "while_closing", "while_moving"}
	gen1RollerObstacleAction = []string{"stop", "reverse"}
	gen1RollerSafetyActions  = []string{"stop", "pause", "reverse"}
	gen1RollerSafetyAllowed  = []string{"none", "open", "close", "all"}
)

// gen1RollerEndsDelayStep and gen1RollerEndsDelayMax bound ends_delay: 0 to 10000 milliseconds in
// steps of 100.
const (
	gen1RollerEndsDelayStep = 100
	gen1RollerEndsDelayMax  = 10000
)

// rollerEnum returns raw when it is a string in allowed, and nil otherwise, so a value the reference
// does not list is never invented on import.
func rollerEnum(raw interface{}, allowed []string) *string {
	value, ok := raw.(string)
	if !ok || !slices.Contains(allowed, value) {
		return nil
	}
	return &value
}

// rollerNumber returns a JSON number as a whole number, and nil for anything else. A fraction is left
// unread and never truncated, so a live 2.5 shows as a difference from a desired 2.
func rollerNumber(raw interface{}) *int {
	n, ok := raw.(float64)
	if !ok || n != math.Trunc(n) {
		return nil
	}
	whole := int(n)
	return &whole
}

// convertRollerRemainder reads the keys beyond the five the converter has always mapped, and says
// whether it found any.
func convertRollerRemainder(rollerMap map[string]interface{}, roller *RollerConfig) bool {
	found := false
	keep := func(set bool) {
		found = found || set
	}

	if roller.DefaultState = rollerEnum(rollerMap["default_state"], gen1RollerStates); roller.DefaultState != nil {
		keep(true)
	}
	// The device reports the button type as button_type. A document that already holds it as btn_type
	// reads the same.
	buttonType := rollerMap["button_type"]
	if buttonType == nil {
		buttonType = rollerMap["btn_type"]
	}
	if roller.ButtonType = rollerEnum(buttonType, gen1RollerButtonTypes); roller.ButtonType != nil {
		keep(true)
	}
	if reverse, ok := reverseFlag(rollerMap["btn_reverse"]); ok {
		roller.ButtonReverse = BoolPtr(reverse)
		keep(true)
	}
	if schedule, ok := rollerMap["schedule"].(bool); ok {
		roller.Schedule = BoolPtr(schedule)
		keep(true)
	}
	if rules, ok := rollerMap["schedule_rules"].([]interface{}); ok {
		list := make([]string, 0, len(rules))
		for _, rule := range rules {
			if text, isText := rule.(string); isText {
				list = append(list, text)
			}
		}
		roller.ScheduleRules = &list
		keep(true)
	}
	if roller.ObstacleMode = rollerEnum(rollerMap["obstacle_mode"], gen1RollerDetectModes); roller.ObstacleMode != nil {
		keep(true)
	}
	if roller.ObstacleAction = rollerEnum(rollerMap["obstacle_action"], gen1RollerObstacleAction); roller.ObstacleAction != nil {
		keep(true)
	}
	if roller.ObstaclePower = rollerNumber(rollerMap["obstacle_power"]); roller.ObstaclePower != nil {
		keep(true)
	}
	if roller.ObstacleDelay = rollerNumber(rollerMap["obstacle_delay"]); roller.ObstacleDelay != nil {
		keep(true)
	}
	if roller.SafetyMode = rollerEnum(rollerMap["safety_mode"], gen1RollerDetectModes); roller.SafetyMode != nil {
		keep(true)
	}
	if roller.SafetyAction = rollerEnum(rollerMap["safety_action"], gen1RollerSafetyActions); roller.SafetyAction != nil {
		keep(true)
	}
	if roller.SafetyAllowedOnTrigger = rollerEnum(rollerMap["safety_allowed_on_trigger"], gen1RollerSafetyAllowed); roller.SafetyAllowedOnTrigger != nil {
		keep(true)
	}
	if roller.OffPower = rollerNumber(rollerMap["off_power"]); roller.OffPower != nil {
		keep(true)
	}
	if roller.EndsDelay = rollerNumber(rollerMap["ends_delay"]); roller.EndsDelay != nil {
		keep(true)
	}
	return found
}

// exportRollerRemainder writes the same keys under the names POST /settings/roller/<n> takes. A
// value outside the reference's list is left out, so the refusal check sees it as lost in the round
// trip and apply never sends it.
func exportRollerRemainder(roller *RollerConfig, rollerMap map[string]interface{}) {
	setEnum := func(key string, value *string, allowed []string) {
		if value != nil && slices.Contains(allowed, *value) {
			rollerMap[key] = *value
		}
	}
	setNumber := func(key string, value *int) {
		if value != nil {
			rollerMap[key] = *value
		}
	}

	setEnum("default_state", roller.DefaultState, gen1RollerStates)
	setEnum("btn_type", roller.ButtonType, gen1RollerButtonTypes)
	if roller.ButtonReverse != nil {
		reverse := 0
		if *roller.ButtonReverse {
			reverse = 1
		}
		rollerMap["btn_reverse"] = reverse
	}
	if roller.Schedule != nil {
		rollerMap["schedule"] = *roller.Schedule
	}
	if roller.ScheduleRules != nil {
		rollerMap["schedule_rules"] = append([]string{}, *roller.ScheduleRules...)
	}
	setEnum("obstacle_mode", roller.ObstacleMode, gen1RollerDetectModes)
	setEnum("obstacle_action", roller.ObstacleAction, gen1RollerObstacleAction)
	setNumber("obstacle_power", roller.ObstaclePower)
	setNumber("obstacle_delay", roller.ObstacleDelay)
	setEnum("safety_mode", roller.SafetyMode, gen1RollerDetectModes)
	setEnum("safety_action", roller.SafetyAction, gen1RollerSafetyActions)
	setEnum("safety_allowed_on_trigger", roller.SafetyAllowedOnTrigger, gen1RollerSafetyAllowed)
	setNumber("off_power", roller.OffPower)
	setNumber("ends_delay", roller.EndsDelay)
}

// unsupportedRollerValues returns the paths of a desired roller whose value no device keeps: a
// negative power or delay, an ends delay outside 0 to 10000 or off the step of 100, and a schedule
// rule that is empty, holds a comma (rules are sent comma-separated) or exceeds the rule bound. An
// enum outside the reference is not named here: the converter drops it, and the round trip names it.
func unsupportedRollerValues(roller *RollerConfig) []string {
	var unsupported []string
	negative := func(name string, value *int) {
		if value != nil && *value < 0 {
			unsupported = append(unsupported, "roller."+name)
		}
	}
	negative("obstacle_power", roller.ObstaclePower)
	negative("obstacle_delay", roller.ObstacleDelay)
	negative("off_power", roller.OffPower)

	if delay := roller.EndsDelay; delay != nil && (*delay < 0 || *delay > gen1RollerEndsDelayMax || *delay%gen1RollerEndsDelayStep != 0) {
		unsupported = append(unsupported, "roller.ends_delay")
	}
	if roller.ScheduleRules != nil {
		bad := len(*roller.ScheduleRules) > gen1MaxScheduleRules
		for _, rule := range *roller.ScheduleRules {
			if rule == "" || strings.Contains(rule, ",") {
				bad = true
			}
		}
		if bad {
			unsupported = append(unsupported, "roller.schedule_rules")
		}
	}
	return unsupported
}

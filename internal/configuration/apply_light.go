package configuration

import (
	"fmt"
	"slices"
	"strings"
)

// The Gen1 Dimmer settings page, as relayed (not read from a device): default_state is on, off, last
// or switch; btn_type is one of the words below; btn_debounce is 60 to 200 ms. The page gives
// auto_on and auto_off no range and schedule_rules no maximum, so the relay's bound of
// gen1MaxScheduleRules is used for the rules and only a negative timer is refused. The page's
// parameter table spells one button word "toglle" where its attribute table and example say "toggle":
// "toggle" is the word sent. Which word the device reads back is unconfirmed until a real device
// reads one back.
const (
	gen1LightMinDebounce = 60
	gen1LightMaxDebounce = 200
)

// gen1LightButtonTypes is the one list the check and the schema enum both read. The default states
// are gen1InitialStates, the same four words a relay takes.
var gen1LightButtonTypes = []string{"one_button", "dual_button", "toggle", "edge", "detached", "action"}

// The RGBW2 page's ranges and words, as relayed: effect 0 to 4, transition 0 to 5000 ms.
const (
	gen1RGBW2MaxEffect     = 4
	gen1RGBW2MaxTransition = 5000
	gen1RGBW2WhiteChannels = 4
)

var (
	gen1RGBW2ButtonTypes = []string{"momentary", "toggle", "edge", "detached", "action"}
	gen1RGBW2States      = []string{"on", "off", "last"}
)

// unsupportedLightValues returns the paths of one Dimmer light channel whose value no device keeps: a
// word outside the page's lists, a debounce outside 60 to 200 ms, a negative timer and a schedule that
// unsupportedScheduleRules refuses.
func unsupportedLightValues(channel int, light SingleLightConfig) []string {
	path := fmt.Sprintf("light.lights[%d].", channel)
	var unsupported []string
	if light.ID != channel {
		unsupported = append(unsupported, path+"id")
	}
	if light.DefaultState != nil && !gen1InitialStates[*light.DefaultState] {
		unsupported = append(unsupported, path+"default_state")
	}
	if light.ButtonType != nil && !slices.Contains(gen1LightButtonTypes, *light.ButtonType) {
		unsupported = append(unsupported, path+"btn_type")
	}
	if d := light.ButtonDebounce; d != nil && (*d < gen1LightMinDebounce || *d > gen1LightMaxDebounce) {
		unsupported = append(unsupported, path+"btn_debounce")
	}
	if light.AutoOn != nil && *light.AutoOn < 0 {
		unsupported = append(unsupported, path+"auto_on")
	}
	if light.AutoOff != nil && *light.AutoOff < 0 {
		unsupported = append(unsupported, path+"auto_off")
	}
	if unsupportedScheduleRules(light.ScheduleRules) {
		unsupported = append(unsupported, path+"schedule_rules")
	}
	return unsupported
}

// unsupportedRGBW2LightValues is the model-only rules for an RGBW2 channel; mode-dependent ones are
// checkLightModeBeforeWrite's.
func unsupportedRGBW2LightValues(channel int, light SingleLightConfig) []string {
	path := fmt.Sprintf("light.lights[%d].", channel)
	var unsupported []string
	if light.ID != channel {
		unsupported = append(unsupported, path+"id")
	}
	if channel >= gen1RGBW2WhiteChannels {
		unsupported = append(unsupported, strings.TrimSuffix(path, "."))
	}
	if s := light.DefaultState; s != nil && !slices.Contains(gen1RGBW2States, *s) {
		unsupported = append(unsupported, path+"default_state")
	}
	if light.ButtonType != nil && !slices.Contains(gen1RGBW2ButtonTypes, *light.ButtonType) {
		unsupported = append(unsupported, path+"btn_type")
	}
	if e := light.Effect; e != nil && (*e < 0 || *e > gen1RGBW2MaxEffect) {
		unsupported = append(unsupported, path+"effect")
	}
	if t := light.Transition; t != nil && (*t < 0 || *t > gen1RGBW2MaxTransition) {
		unsupported = append(unsupported, path+"transition")
	}
	if light.ButtonDebounce != nil {
		unsupported = append(unsupported, path+"btn_debounce")
	}
	if light.SwapInputs != nil {
		unsupported = append(unsupported, path+"swap_inputs")
	}
	if light.AutoOn != nil && *light.AutoOn < 0 {
		unsupported = append(unsupported, path+"auto_on")
	}
	if light.AutoOff != nil && *light.AutoOff < 0 {
		unsupported = append(unsupported, path+"auto_off")
	}
	if unsupportedScheduleRules(light.ScheduleRules) {
		unsupported = append(unsupported, path+"schedule_rules")
	}
	return unsupported
}

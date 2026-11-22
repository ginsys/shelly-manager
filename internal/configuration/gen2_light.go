package configuration

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Gen2 light:N keys, words and ranges come from the Shelly Light page and are unconfirmed on hardware.

// gen2LightInitialStates: Gen1 "last" is restore_last, and Gen1 "switch" has no Gen2 light word.
var gen2LightInitialStates = map[string]string{
	"on": "on", "off": "off", "restore_last": "restore_last", "toggle": "toggle", "last": "restore_last",
}

func gen2LightInitialStateWord(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		if mapped, ok := gen2LightInitialStates[s]; ok {
			return mapped
		}
	}
	return v
}

// gen2LightFields are the stored light fields with a Light.SetConfig key; any other one set is refused.
var gen2LightFields = map[string]bool{
	"id": true, "name": true, "default_state": true, "auto_on": true, "auto_off": true,
	"transition_duration": true, "min_brightness_on_toggle": true, "button_fade_rate": true,
	"night_mode": true, "night_mode_brightness": true, "night_mode_start": true, "night_mode_end": true,
	"warmup_enabled": true, "warmup_brightness": true, "warmup_time": true,
	"max_power": true, "voltage_limit": true, "undervoltage_limit": true, "current_limit": true, "in_mode": true,
}

// gen2LightInModes are the in_mode words of the Light page; dim and dual_dim are "if applicable".
var gen2LightInModes = []string{"follow", "flip", "activate", "detached", "dim", "dual_dim"}

// gen2ClockTime takes only the two-digit HH:MM form, so a read-back compares equal.
var gen2ClockTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func putIfSet[T any](body map[string]interface{}, key string, value *T) {
	if value != nil {
		body[key] = *value
	}
}

// gen2LightToDoc writes only the keys set; active_between needs both ends.
func gen2LightToDoc(config *DeviceConfiguration, doc map[string]interface{}) {
	if config.Light == nil {
		return
	}
	for _, light := range config.Light.Lights {
		body := map[string]interface{}{}
		putIfSet(body, "name", light.Name)
		if light.DefaultState != nil {
			if state, ok := gen2LightInitialStates[*light.DefaultState]; ok {
				body["initial_state"] = state
			}
		}
		gen2TimerToDoc(body, "auto_on", "auto_on_delay", light.AutoOn)
		gen2TimerToDoc(body, "auto_off", "auto_off_delay", light.AutoOff)
		putIfSet(body, "transition_duration", light.TransitionDuration)
		putIfSet(body, "min_brightness_on_toggle", light.MinBrightnessOnToggle)
		putIfSet(body, "button_fade_rate", light.ButtonFadeRate)
		putIfSet(body, "power_limit", light.MaxPower)
		putIfSet(body, "voltage_limit", light.VoltageLimit)
		putIfSet(body, "undervoltage_limit", light.UndervoltageLimit)
		putIfSet(body, "current_limit", light.CurrentLimit)
		putIfSet(body, "in_mode", light.InMode)

		night := map[string]interface{}{}
		putIfSet(night, "enable", light.NightMode)
		putIfSet(night, "brightness", light.NightModeBrightness)
		if light.NightModeStart != nil && light.NightModeEnd != nil {
			night["active_between"] = []string{*light.NightModeStart, *light.NightModeEnd}
		}
		warmup := map[string]interface{}{}
		putIfSet(warmup, "enable", light.WarmupEnabled)
		putIfSet(warmup, "brightness", light.WarmupBrightness)
		putIfSet(warmup, "time_ms", light.WarmupTime)
		for key, nested := range map[string]map[string]interface{}{"night_mode": night, "warmup": warmup} {
			if len(nested) > 0 {
				body[key] = nested
			}
		}
		if len(body) > 0 && light.ID >= 0 {
			doc["light:"+strconv.Itoa(light.ID)] = body
		}
	}
}

// gen2LightsFromDoc leaves a null or a fractional whole-number field unset and keeps an initial_state word as it is, so an unknown one reads as a difference.
func gen2LightsFromDoc(doc map[string]interface{}) *LightConfig {
	var lights []SingleLightConfig
	for _, id := range gen2ChannelIDs(doc, "light") {
		raw, ok := doc["light:"+strconv.Itoa(id)].(map[string]interface{})
		if !ok {
			continue
		}
		light := SingleLightConfig{ID: id}
		if name, ok := raw["name"].(string); ok {
			light.Name = StringPtr(name)
		}
		if state, ok := raw["initial_state"].(string); ok {
			light.DefaultState = StringPtr(state)
		}
		light.AutoOn = gen2TimerFromDoc(raw, "auto_on", "auto_on_delay", math.MaxInt32)
		light.AutoOff = gen2TimerFromDoc(raw, "auto_off", "auto_off_delay", math.MaxInt32)
		if seconds, ok := raw["transition_duration"].(float64); ok {
			light.TransitionDuration = &seconds
		}
		light.MinBrightnessOnToggle = gen2WholeFromDoc(raw, "min_brightness_on_toggle")
		light.ButtonFadeRate = gen2WholeFromDoc(raw, "button_fade_rate")
		light.MaxPower = gen2WholeFromDoc(raw, "power_limit")
		light.VoltageLimit = gen2WholeFromDoc(raw, "voltage_limit")
		light.UndervoltageLimit = gen2WholeFromDoc(raw, "undervoltage_limit")
		if amps, ok := raw["current_limit"].(float64); ok {
			light.CurrentLimit = &amps
		}
		if mode, ok := raw["in_mode"].(string); ok {
			light.InMode = StringPtr(mode)
		}
		if night, ok := raw["night_mode"].(map[string]interface{}); ok {
			if enable, ok := night["enable"].(bool); ok {
				light.NightMode = BoolPtr(enable)
			}
			light.NightModeBrightness = gen2WholeFromDoc(night, "brightness")
			if span, ok := night["active_between"].([]interface{}); ok && len(span) == 2 {
				start, startOK := span[0].(string)
				end, endOK := span[1].(string)
				if startOK && endOK {
					light.NightModeStart, light.NightModeEnd = StringPtr(start), StringPtr(end)
				}
			}
		}
		if warmup, ok := raw["warmup"].(map[string]interface{}); ok {
			if enable, ok := warmup["enable"].(bool); ok {
				light.WarmupEnabled = BoolPtr(enable)
			}
			light.WarmupBrightness = gen2WholeFromDoc(warmup, "brightness")
			light.WarmupTime = gen2WholeFromDoc(warmup, "time_ms")
		}
		lights = append(lights, light)
	}
	if len(lights) == 0 {
		return nil
	}
	return &LightConfig{Lights: lights}
}

// unmappedLightSettings returns the refused paths, the lights left to compare and their original indexes.
func unmappedLightSettings(desired *LightConfig) ([]string, *LightConfig, []int) {
	if desired == nil {
		return nil, nil, nil
	}
	var paths []string
	var kept []int
	rest := &LightConfig{}
	seen := map[int]bool{}
	for i, source := range desired.Lights {
		light := source
		prefix := "light.lights." + strconv.Itoa(i)
		paths = append(paths, unkeyedFields(&light, prefix, gen2LightFields)...)
		paths = append(paths, gen2LightBounds(&light, prefix)...)
		if light.ID < 0 || seen[light.ID] {
			paths = append(paths, prefix+".id")
			continue
		}
		seen[light.ID] = true
		if light != (SingleLightConfig{ID: light.ID}) {
			rest.Lights, kept = append(rest.Lights, light), append(kept, i)
		}
	}
	if len(rest.Lights) == 0 {
		rest = nil
	}
	return paths, rest, kept
}

// gen2LightBounds names, and clears, each value outside the page's ranges; the page gives some no range, so those get plausibility bounds.
func gen2LightBounds(light *SingleLightConfig, prefix string) []string {
	var paths []string
	refuse := func(name string, bad bool, clear func()) {
		if bad {
			paths = append(paths, prefix+"."+name)
			clear()
		}
	}
	outside := func(value *int, low, high int) bool { return value != nil && (*value < low || *value > high) }
	_, known := gen2LightInitialStates[stringOrEmpty(light.DefaultState)]
	refuse("default_state", light.DefaultState != nil && !known, func() { light.DefaultState = nil })
	refuse("auto_on", outside(light.AutoOn, 0, math.MaxInt32), func() { light.AutoOn = nil })
	refuse("auto_off", outside(light.AutoOff, 0, math.MaxInt32), func() { light.AutoOff = nil })
	refuse("transition_duration", light.TransitionDuration != nil && *light.TransitionDuration < 0, func() { light.TransitionDuration = nil })
	refuse("min_brightness_on_toggle", outside(light.MinBrightnessOnToggle, 0, 100), func() { light.MinBrightnessOnToggle = nil })
	refuse("button_fade_rate", outside(light.ButtonFadeRate, 1, 5), func() { light.ButtonFadeRate = nil })
	refuse("night_mode_brightness", outside(light.NightModeBrightness, 0, 100), func() { light.NightModeBrightness = nil })
	refuse("warmup_brightness", outside(light.WarmupBrightness, 10, 100), func() { light.WarmupBrightness = nil })
	refuse("warmup_time", outside(light.WarmupTime, 20, 1000), func() { light.WarmupTime = nil })
	// The page gives the limits no range: only a negative one is refused, and no pair is compared.
	refuse("max_power", outside(light.MaxPower, 0, math.MaxInt32), func() { light.MaxPower = nil })
	refuse("voltage_limit", outside(light.VoltageLimit, 0, math.MaxInt32), func() { light.VoltageLimit = nil })
	refuse("undervoltage_limit", outside(light.UndervoltageLimit, 0, math.MaxInt32), func() { light.UndervoltageLimit = nil })
	refuse("current_limit", light.CurrentLimit != nil && *light.CurrentLimit < 0, func() { light.CurrentLimit = nil })
	refuse("in_mode", light.InMode != nil && !slices.Contains(gen2LightInModes, *light.InMode), func() { light.InMode = nil })
	// A span needs both ends as HH:MM; a bad one refuses the ends that were supplied.
	start, end := light.NightModeStart, light.NightModeEnd
	badClock := func(clock *string) bool { return clock != nil && !gen2ClockTime.MatchString(*clock) }
	if (start == nil) != (end == nil) || badClock(start) || badClock(end) {
		refuse("night_mode_start", start != nil, func() { light.NightModeStart = nil })
		refuse("night_mode_end", end != nil, func() { light.NightModeEnd = nil })
	}
	return paths
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// refuseUnreportedLights refuses a light or key the live device does not report; null counts as reported.
func refuseUnreportedLights(doc, apiConfig map[string]interface{}, desired *LightConfig) error {
	if desired == nil {
		return nil
	}
	var missing []string
	for _, light := range desired.Lights {
		key := "light:" + strconv.Itoa(light.ID)
		live, ok := doc[key].(map[string]interface{})
		if !ok {
			return &refusedBeforeWriteError{fmt.Errorf("%w: light.lights: the device reports no %s, so it has no such light, and apply does not change the profile", ErrUnsupportedSettings, key)}
		}
		body, _ := apiConfig[key].(map[string]interface{})
		for _, leaf := range gen2Leaves("", body) {
			if !gen2KeyReported(live, strings.Split(strings.TrimPrefix(leaf.path, "."), ".")) {
				missing = append(missing, key+leaf.path)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &refusedBeforeWriteError{fmt.Errorf("%w: %s: the device's firmware does not report the key, so nothing was written", ErrUnsupportedSettings, strings.Join(missing, ", "))}
}

func gen2KeyReported(object map[string]interface{}, path []string) bool {
	value, held := object[path[0]]
	if len(path) == 1 || !held {
		return held
	}
	nested, isObject := value.(map[string]interface{})
	return isObject && gen2KeyReported(nested, path[1:])
}

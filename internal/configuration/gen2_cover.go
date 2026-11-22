package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// The Gen2 cover: a device in cover profile (Plus 2PM, Pro 2PM, Pro Dual Cover) holds its roller
// as the component cover:0, written with Cover.SetConfig. The stored RollerConfig is one roller, as
// on Gen1, so it is cover:0 and a second cover (cover:1) is neither read nor written. The stored
// Gen1 words are mapped to the Cover keys and no stored field is added, so one field has one
// meaning. The key names, values and ranges are read from the Shelly Cover reference and are
// unconfirmed on hardware.

// gen2CoverKey is the config document key of the cover that the stored roller maps to.
const gen2CoverKey = "cover:0"

// The Cover page gives the travel times and the obstruction holdoff as 0.1 to 300 seconds. The
// stored values are whole seconds, so the bounds are 1 to 300.
const (
	gen2CoverSecondsMin = 1
	gen2CoverSecondsMax = 300
)

// gen2CoverDirections maps a stored detection mode to the direction of obstruction_detection and
// safety_switch. A mode that is not here ("disabled") is enable false with no direction.
var gen2CoverDirections = map[string]string{
	"while_opening": "open", "while_closing": "close", "while_moving": "both",
}

// gen2RollerFields are the stored roller fields that have a Cover key. Every other field that a
// desired roller sets has none and is refused.
var gen2RollerFields = map[string]bool{
	"max_open_time": true, "max_close_time": true, "swap_inputs": true, "motor_direction": true,
	"input_mode": true, "obstacle_mode": true, "obstacle_action": true, "obstacle_power": true,
	"obstacle_delay": true, "safety_mode": true, "safety_action": true,
}

// gen2DetectionToDoc returns the enable and direction keys of a detection mode, and nothing for a
// value that is not a mode.
func gen2DetectionToDoc(mode *string, body map[string]interface{}) {
	if mode == nil {
		return
	}
	if *mode == "disabled" {
		body["enable"] = false
	} else if direction, ok := gen2CoverDirections[*mode]; ok {
		body["enable"], body["direction"] = true, direction
	}
}

// gen2DetectionFromDoc reads the stored detection mode of an obstruction_detection or safety_switch
// object. enable false is disabled whatever direction is reported, and a direction the page does not
// list names no mode.
func gen2DetectionFromDoc(raw map[string]interface{}) *string {
	enabled, ok := raw["enable"].(bool)
	if !ok {
		return nil
	}
	if !enabled {
		return StringPtr("disabled")
	}
	direction, _ := raw["direction"].(string)
	for mode, word := range gen2CoverDirections {
		if word == direction {
			return StringPtr(mode)
		}
	}
	return nil
}

// gen2CoverToDoc adds the cover:0 section with only the keys the stored roller sets, and in the
// nested objects only the sub-keys it sets. A word outside the stored sets is left out here:
// unmappedRollerSettings names it before anything is written, so it is never dropped unseen.
func gen2CoverToDoc(config *DeviceConfiguration, doc map[string]interface{}) {
	roller := config.Roller
	if roller == nil {
		return
	}
	body := map[string]interface{}{}
	if roller.MaxOpenTime != nil {
		body["maxtime_open"] = *roller.MaxOpenTime
	}
	if roller.MaxCloseTime != nil {
		body["maxtime_close"] = *roller.MaxCloseTime
	}
	if roller.SwapInputs != nil {
		body["swap_inputs"] = *roller.SwapInputs
	}
	switch {
	case roller.MotorDirection == nil:
	case *roller.MotorDirection == rollerMotorNormal:
		body["invert_directions"] = false
	case *roller.MotorDirection == rollerMotorReversed:
		body["invert_directions"] = true
	}
	if roller.InputMode != nil && (*roller.InputMode == rollerInputSingle || *roller.InputMode == rollerInputDual) {
		body["in_mode"] = *roller.InputMode
	}

	obstruction := map[string]interface{}{}
	gen2DetectionToDoc(roller.ObstacleMode, obstruction)
	if roller.ObstacleAction != nil && slices.Contains(gen1RollerObstacleAction, *roller.ObstacleAction) {
		obstruction["action"] = *roller.ObstacleAction
	}
	if roller.ObstaclePower != nil {
		obstruction["power_thr"] = *roller.ObstaclePower
	}
	if roller.ObstacleDelay != nil {
		obstruction["holdoff"] = *roller.ObstacleDelay
	}
	if len(obstruction) > 0 {
		body["obstruction_detection"] = obstruction
	}

	safety := map[string]interface{}{}
	gen2DetectionToDoc(roller.SafetyMode, safety)
	if roller.SafetyAction != nil && slices.Contains(gen1RollerSafetyActions, *roller.SafetyAction) {
		safety["action"] = *roller.SafetyAction
	}
	if len(safety) > 0 {
		body["safety_switch"] = safety
	}
	if len(body) > 0 {
		doc[gen2CoverKey] = body
	}
}

// gen2CoverFromDoc reads cover:0 as the stored roller, or nil when the document has none or nothing
// in it is read. A fraction, a word with no stored word (in_mode detached) and an unknown
// direction are left unset, so a desired value shows as a difference and is never matched by a guess.
func gen2CoverFromDoc(doc map[string]interface{}) *RollerConfig {
	raw, ok := doc[gen2CoverKey].(map[string]interface{})
	if !ok {
		return nil
	}
	roller := &RollerConfig{
		MaxOpenTime:  rollerNumber(raw["maxtime_open"]),
		MaxCloseTime: rollerNumber(raw["maxtime_close"]),
	}
	if swap, ok := raw["swap_inputs"].(bool); ok {
		roller.SwapInputs = BoolPtr(swap)
	}
	if invert, ok := raw["invert_directions"].(bool); ok {
		roller.MotorDirection = StringPtr(rollerMotorNormal)
		if invert {
			roller.MotorDirection = StringPtr(rollerMotorReversed)
		}
	}
	roller.InputMode = rollerEnum(raw["in_mode"], []string{rollerInputSingle, rollerInputDual})
	if obstruction, ok := raw["obstruction_detection"].(map[string]interface{}); ok {
		roller.ObstacleMode = gen2DetectionFromDoc(obstruction)
		roller.ObstacleAction = rollerEnum(obstruction["action"], gen1RollerObstacleAction)
		roller.ObstaclePower = rollerNumber(obstruction["power_thr"])
		roller.ObstacleDelay = rollerNumber(obstruction["holdoff"])
	}
	if safety, ok := raw["safety_switch"].(map[string]interface{}); ok {
		roller.SafetyMode = gen2DetectionFromDoc(safety)
		roller.SafetyAction = rollerEnum(safety["action"], gen1RollerSafetyActions)
	}
	if *roller == (RollerConfig{}) {
		return nil
	}
	return roller
}

// unmappedRollerSettings returns the paths of the roller settings that Cover.SetConfig has no key
// for, or whose value no cover keeps, and the rest of the roller, or nil when nothing else is set.
// A word outside the stored sets has no Cover word, so it is named here by its field. A travel time
// or an obstruction holdoff outside 1 to 300 whole seconds is below or above the page's range, and a
// negative obstruction power is a value no device keeps.
func unmappedRollerSettings(roller *RollerConfig) ([]string, *RollerConfig) {
	if roller == nil {
		return nil, nil
	}
	rest := *roller
	paths := unkeyedFields(&rest, "roller", gen2RollerFields)
	paths = append(paths, unsupportedRollerValues(&rest)...)
	if rest.ObstaclePower != nil && *rest.ObstaclePower < 0 {
		rest.ObstaclePower = nil
	}
	for name, word := range map[string]struct {
		value   **string
		allowed []string
	}{
		"motor_direction": {&rest.MotorDirection, []string{rollerMotorNormal, rollerMotorReversed}},
		"input_mode":      {&rest.InputMode, []string{rollerInputSingle, rollerInputDual}},
		"obstacle_mode":   {&rest.ObstacleMode, gen1RollerDetectModes},
		"obstacle_action": {&rest.ObstacleAction, gen1RollerObstacleAction},
		"safety_mode":     {&rest.SafetyMode, gen1RollerDetectModes},
		"safety_action":   {&rest.SafetyAction, gen1RollerSafetyActions},
	} {
		if *word.value != nil && !slices.Contains(word.allowed, **word.value) {
			paths, *word.value = append(paths, "roller."+name), nil
		}
	}
	for name, value := range map[string]**int{
		"max_open_time": &rest.MaxOpenTime, "max_close_time": &rest.MaxCloseTime, "obstacle_delay": &rest.ObstacleDelay,
	} {
		if *value != nil && (**value < gen2CoverSecondsMin || **value > gen2CoverSecondsMax) {
			paths, *value = append(paths, "roller."+name), nil
		}
	}
	if rest == (RollerConfig{}) {
		return paths, nil
	}
	return paths, &rest
}

// refuseUncoverableRoller stops an apply, before the first write, whose cover write the live device
// cannot take. It reads the device once, and only when the configuration writes cover:0. A device
// with no cover:0 is in switch profile, and the apply never changes the profile (that deletes the
// webhooks and schedules). A key that the page lists as present only with the inputs it needs is
// refused when the device does not report it, and a safety action of reverse is refused unless the
// device already reports allowed_move reverse, which the page requires and the stored roller has
// no word for. The second result says whether the write changes invert_directions, which the page
// says needs a reboot: a live value that is not read counts as a change.
func refuseUncoverableRoller(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}) (reverses bool, err error) {
	body, ok := apiConfig[gen2CoverKey].(map[string]interface{})
	if !ok || len(body) == 0 {
		return false, nil
	}
	live, err := client.GetConfig(ctx)
	if err != nil {
		return false, &refusedBeforeWriteError{fmt.Errorf("could not read the device before writing the cover: %w", err)}
	}
	var doc map[string]interface{}
	if err = json.Unmarshal(live.Raw, &doc); err != nil {
		return false, &refusedBeforeWriteError{fmt.Errorf("could not read the device before writing the cover: %w", err)}
	}
	cover, ok := doc[gen2CoverKey].(map[string]interface{})
	if !ok {
		return false, &refusedBeforeWriteError{fmt.Errorf("%w: roller: the device reports no %s, so it is not in cover profile, and apply does not change the profile", ErrUnsupportedSettings, gen2CoverKey)}
	}

	var paths []string
	if _, set := body["swap_inputs"]; set {
		if _, held := cover["swap_inputs"]; !held {
			paths = append(paths, "roller.swap_inputs")
		}
	}
	if _, set := body["in_mode"]; set {
		if _, held := cover["in_mode"]; !held {
			paths = append(paths, "roller.input_mode")
		}
	}
	if safety, set := body["safety_switch"].(map[string]interface{}); set {
		liveSafety, held := cover["safety_switch"].(map[string]interface{})
		for key, path := range map[string]string{"enable": "roller.safety_mode", "action": "roller.safety_action"} {
			if _, wanted := safety[key]; !wanted {
				continue
			}
			if !held || (key == "action" && safety[key] == "reverse" && liveSafety["allowed_move"] != "reverse") {
				paths = append(paths, path)
			}
		}
		// The page says the safety switch works only when in_mode is single, so enabling it under
		// any other effective mode would be confirmed by the read back and do nothing.
		if enable, _ := safety["enable"].(bool); enable && held {
			mode, ok := body["in_mode"].(string)
			if !ok {
				mode, _ = cover["in_mode"].(string)
			}
			if mode != rollerInputSingle {
				paths = append(paths, "roller.safety_mode")
			}
		}
	}
	if len(paths) > 0 {
		slices.Sort(paths)
		return false, &refusedBeforeWriteError{fmt.Errorf("%w: %s: the device does not report what these need (inputs for swap_inputs, in_mode and safety_switch, in_mode single for an enabled safety switch, allowed_move reverse for a safety action of reverse)", ErrUnsupportedSettings, strings.Join(paths, ", "))}
	}

	if want, set := body["invert_directions"].(bool); set {
		current, known := cover["invert_directions"].(bool)
		reverses = !known || current != want
	}
	return reverses, nil
}

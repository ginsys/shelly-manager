package configuration

import (
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// The Gen2 switch and input channels. A Gen2 document holds one section per channel, switch:0,
// input:0 and so on. The stored model keeps Gen1 words where a Gen1 template uses them, so a word
// that has the same meaning on a Gen2 device is written as the Gen2 word, and a Gen2 word is read
// back as it is. The compare rules treat each pair as one value.

// gen2InitialStates maps a stored initial state to the Gen2 initial_state. Gen1 "last" restores the
// last state and Gen1 "switch" follows the input, which are restore_last and match_input.
var gen2InitialStates = map[string]string{
	"on": "on", "off": "off", "restore_last": "restore_last", "match_input": "match_input",
	"last": "restore_last", "switch": "match_input",
}

// gen2InModes maps a stored input mode to the Gen2 in_mode of the switch it drives. Gen1 "toggle"
// and "edge" are follow and flip. Gen1 "action" has no Gen2 word and is not here.
var gen2InModes = map[string]string{
	"momentary": "momentary", "follow": "follow", "flip": "flip", "detached": "detached",
	"cycle": "cycle", "activate": "activate",
	"toggle": "follow", "edge": "flip",
}

// gen2InputTypes are the input types that Input.SetConfig is written for. analog and count have
// settings of their own that have no stored field.
var gen2InputTypes = map[string]bool{"switch": true, "button": true}

// gen2InitialStateWord and gen2InModeWord give the compare rules one word per meaning.
func gen2InitialStateWord(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		if mapped, ok := gen2InitialStates[s]; ok {
			return mapped
		}
	}
	return v
}

func gen2InModeWord(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		if mapped, ok := gen2InModes[s]; ok {
			return mapped
		}
	}
	return v
}

// gen2ChannelID returns the id of a "switch:2" or "input:2" key of the given kind. Only the canonical
// spelling counts: "switch:02" and "switch:+2" are not channels, so no two keys name one channel and
// the section a reader looks up by "kind:id" is the section that was found.
func gen2ChannelID(key, kind string) (int, bool) {
	rest, ok := strings.CutPrefix(key, kind+":")
	if !ok {
		return 0, false
	}
	id, err := strconv.Atoi(rest)
	if err != nil || id < 0 || strconv.Itoa(id) != rest {
		return 0, false
	}
	return id, true
}

// gen2KeyApplies reports whether apply writes a top-level key of a Gen2 config document: a fixed key
// or a channel key, and only when its value is an object, as every section is.
func gen2KeyApplies(key string, value interface{}) bool {
	if _, isObject := value.(map[string]interface{}); !isObject {
		return false
	}
	if gen2ApplyKeys[key] {
		return true
	}
	for _, kind := range []string{"switch", "input", "light"} {
		if _, ok := gen2ChannelID(key, kind); ok {
			return true
		}
	}
	return false
}

// gen2ChannelIDs returns the ids of one kind of channel in a document, ascending. An id is listed
// once, because only the canonical key of a channel is read.
func gen2ChannelIDs(doc map[string]interface{}, kind string) []int {
	var ids []int
	for key := range doc {
		if id, ok := gen2ChannelID(key, kind); ok {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

// gen2ChannelsFromDoc reads the switch and input sections. The in_mode of switch:N is the mode of
// the input that switch:N takes (input N, or the one input_id names): the stored model keeps the
// mode with the input. An auto timer is its seconds, or 0 when
// the device has it off, whatever delay it keeps. A null value is an unset setting.
func gen2ChannelsFromDoc(doc map[string]interface{}) (*RelayConfig, *InputConfig) {
	inputs := map[int]*SingleInputConfig{}
	input := func(id int) *SingleInputConfig {
		if inputs[id] == nil {
			inputs[id] = &SingleInputConfig{ID: id}
		}
		return inputs[id]
	}

	var relays []SingleRelayConfig
	for _, id := range gen2ChannelIDs(doc, "switch") {
		raw, ok := doc["switch:"+strconv.Itoa(id)].(map[string]interface{})
		if !ok {
			continue
		}
		relay := SingleRelayConfig{ID: id}
		if name, ok := raw["name"].(string); ok {
			relay.Name = StringPtr(name)
		}
		if state, ok := raw["initial_state"].(string); ok {
			relay.DefaultState = StringPtr(state)
		}
		relay.AutoOn = gen2TimerFromDoc(raw, "auto_on", "auto_on_delay", math.MaxInt32)
		relay.AutoOff = gen2TimerFromDoc(raw, "auto_off", "auto_off_delay", math.MaxInt32)
		relay.PowerRestoreDelay = gen2TimerFromDoc(raw, "power_restore_delay_enable", "power_restore_delay", gen2MaxRestoreDelay)
		relay.MaxPower = gen2WholeFromDoc(raw, "power_limit")
		relay.VoltageLimit = gen2WholeFromDoc(raw, "voltage_limit")
		relay.UndervoltageLimit = gen2WholeFromDoc(raw, "undervoltage_limit")
		if amps, ok := raw["current_limit"].(float64); ok {
			relay.CurrentLimit = &amps
		}
		if locked, ok := raw["in_locked"].(bool); ok {
			relay.InLocked = BoolPtr(locked)
		}
		if recover, ok := raw["autorecover_voltage_errors"].(bool); ok {
			relay.AutorecoverVoltageErrors = BoolPtr(recover)
		}
		if mode, ok := raw["in_mode"].(string); ok {
			// A model with a selectable input (input_id) runs the mode on the input it names.
			target := id
			if selected, ok := raw["input_id"].(float64); ok && selected >= 0 && selected == math.Trunc(selected) {
				target = int(selected)
			}
			// The mode belongs to an input the device reports: in_mode alone does not make one.
			if _, held := doc["input:"+strconv.Itoa(target)].(map[string]interface{}); held {
				input(target).Mode = StringPtr(mode)
			}
		}
		relays = append(relays, relay)
	}

	for _, id := range gen2ChannelIDs(doc, "input") {
		raw, ok := doc["input:"+strconv.Itoa(id)].(map[string]interface{})
		if !ok {
			continue
		}
		in := input(id)
		if name, ok := raw["name"].(string); ok {
			in.Name = StringPtr(name)
		}
		if kind, ok := raw["type"].(string); ok {
			in.Type = StringPtr(kind)
		}
		if invert, ok := raw["invert"].(bool); ok {
			in.Inverted = BoolPtr(invert)
		}
		if enable, ok := raw["enable"].(bool); ok {
			in.Enable = BoolPtr(enable)
		}
		if reset, ok := raw["factory_reset"].(bool); ok {
			in.FactoryReset = BoolPtr(reset)
		}
	}

	var relayConfig *RelayConfig
	if len(relays) > 0 {
		relayConfig = &RelayConfig{Relays: relays}
	}
	var inputConfig *InputConfig
	if len(inputs) > 0 {
		ids := make([]int, 0, len(inputs))
		for id := range inputs {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		inputConfig = &InputConfig{}
		for _, id := range ids {
			inputConfig.Inputs = append(inputConfig.Inputs, *inputs[id])
		}
	}
	return relayConfig, inputConfig
}

// gen2MaxRestoreDelay is the highest power_restore_delay in seconds that the Switch page gives (relayed,
// not confirmed on hardware).
const gen2MaxRestoreDelay = 600

// gen2TimerFromDoc reads a timer (auto_on, auto_off, power_restore_delay) with its delay as one number
// of seconds. An enabled timer whose delay is not a whole number from 1 to max has no stored value: it
// is read as unset, so a desired value reports a difference. It is never turned into 0, which means off.
func gen2TimerFromDoc(raw map[string]interface{}, enableKey, delayKey string, max float64) *int {
	enabled, ok := raw[enableKey].(bool)
	if !ok {
		return nil
	}
	if !enabled {
		return IntPtr(0)
	}
	delay, ok := raw[delayKey].(float64)
	if !ok || delay < 1 || delay != math.Trunc(delay) || delay > max {
		return nil
	}
	return IntPtr(int(delay))
}

// gen2WholeFromDoc reads a limit that the stored model keeps as a whole number. A null, a missing key
// and a value with a fraction are unset, so a desired number reports a difference.
func gen2WholeFromDoc(raw map[string]interface{}, key string) *int {
	v, ok := raw[key].(float64)
	if !ok || v < 0 || v != math.Trunc(v) || v > math.MaxInt32 {
		return nil
	}
	return IntPtr(int(v))
}

// gen2ChannelsToDoc adds the input:N and switch:N sections that the configuration sets, with only
// the keys it sets. A value with no Gen2 word is left out here, and unmappedChannelSettings names
// it before anything is written.
func gen2ChannelsToDoc(config *DeviceConfiguration, doc map[string]interface{}) {
	switches := map[int]map[string]interface{}{}
	switchBody := func(id int) map[string]interface{} {
		if switches[id] == nil {
			switches[id] = map[string]interface{}{}
		}
		return switches[id]
	}

	if config.Input != nil {
		for _, in := range config.Input.Inputs {
			body := map[string]interface{}{}
			if in.Name != nil {
				body["name"] = *in.Name
			}
			if in.Type != nil && gen2InputTypes[*in.Type] {
				body["type"] = *in.Type
			}
			if in.Inverted != nil {
				body["invert"] = *in.Inverted
			}
			if in.Enable != nil {
				body["enable"] = *in.Enable
			}
			if in.FactoryReset != nil {
				body["factory_reset"] = *in.FactoryReset
			}
			if len(body) > 0 {
				doc["input:"+strconv.Itoa(in.ID)] = body
			}
			if in.Mode != nil {
				if mode, ok := gen2InModes[*in.Mode]; ok {
					switchBody(in.ID)["in_mode"] = mode
				}
			}
		}
	}

	if config.Relay != nil {
		for _, relay := range config.Relay.Relays {
			body := switchBody(relay.ID)
			if relay.Name != nil {
				body["name"] = *relay.Name
			}
			if relay.DefaultState != nil {
				if state, ok := gen2InitialStates[*relay.DefaultState]; ok {
					body["initial_state"] = state
				}
			}
			gen2TimerToDoc(body, "auto_on", "auto_on_delay", relay.AutoOn)
			gen2TimerToDoc(body, "auto_off", "auto_off_delay", relay.AutoOff)
			gen2TimerToDoc(body, "power_restore_delay_enable", "power_restore_delay", relay.PowerRestoreDelay)
			for key, value := range map[string]interface{}{
				"power_limit": relay.MaxPower, "voltage_limit": relay.VoltageLimit,
				"undervoltage_limit": relay.UndervoltageLimit, "current_limit": relay.CurrentLimit,
				"in_locked": relay.InLocked, "autorecover_voltage_errors": relay.AutorecoverVoltageErrors,
			} {
				if v := reflect.ValueOf(value); !v.IsNil() {
					body[key] = v.Elem().Interface()
				}
			}
		}
	}

	for id, body := range switches {
		if len(body) > 0 {
			doc["switch:"+strconv.Itoa(id)] = body
		}
	}
}

// gen2TimerToDoc writes seconds above 0 as the timer on with that delay, and 0 as the timer off
// with no delay. A negative number is not written.
func gen2TimerToDoc(body map[string]interface{}, enableKey, delayKey string, seconds *int) {
	if seconds == nil || *seconds < 0 {
		return
	}
	body[enableKey] = *seconds > 0
	if *seconds > 0 {
		body[delayKey] = *seconds
	}
}

// gen2ChannelFields are the fields of a switch or input channel that have a Gen2 key. Every other
// field that a desired channel sets has no key to write.
var gen2ChannelFields = map[string]bool{
	"id": true, "name": true, "default_state": true, "auto_on": true, "auto_off": true,
	"type": true, "mode": true, "inverted": true, "enable": true, "factory_reset": true,
	"max_power": true, "voltage_limit": true, "undervoltage_limit": true, "current_limit": true,
	"in_locked": true, "autorecover_voltage_errors": true, "power_restore_delay": true,
}

// unmappedLimits names, and clears in r, each limit that no Gen2 value can carry: a negative one, a
// restore delay outside 0 to 600, and an undervoltage limit above the voltage limit when both are
// desired. A voltage limit below the undervoltage limit the device holds is the device's to refuse.
func unmappedLimits(r *SingleRelayConfig, prefix string) []string {
	var paths []string
	drop := func(name string, refused bool, clear func()) {
		if refused {
			paths = append(paths, prefix+"."+name)
			clear()
		}
	}
	drop("max_power", r.MaxPower != nil && *r.MaxPower < 0, func() { r.MaxPower = nil })
	drop("voltage_limit", r.VoltageLimit != nil && *r.VoltageLimit < 0, func() { r.VoltageLimit = nil })
	drop("undervoltage_limit", r.UndervoltageLimit != nil && *r.UndervoltageLimit < 0, func() { r.UndervoltageLimit = nil })
	drop("current_limit", r.CurrentLimit != nil && *r.CurrentLimit < 0, func() { r.CurrentLimit = nil })
	drop("power_restore_delay", r.PowerRestoreDelay != nil && (*r.PowerRestoreDelay < 0 || *r.PowerRestoreDelay > gen2MaxRestoreDelay),
		func() { r.PowerRestoreDelay = nil })
	if r.UndervoltageLimit != nil && r.VoltageLimit != nil && *r.UndervoltageLimit > *r.VoltageLimit {
		paths = append(paths, prefix+".undervoltage_limit", prefix+".voltage_limit")
		r.UndervoltageLimit, r.VoltageLimit = nil, nil
	}
	return paths
}

// unreportedChannelKeys names the desired switch and input settings whose Gen2 key the live section
// does not carry, each as the desired path with the device key beside it. A device that lacks a key
// takes the write of it as a failed row at best, so the apply refuses before it writes anything. A key
// that is present with null is reported (the device default), so it is not named. A restore delay
// above 0 needs both its keys, and 0 needs the enable key.
func unreportedChannelKeys(doc map[string]interface{}, config *DeviceConfiguration) []string {
	var missing []string
	type setting struct {
		set        bool
		field, key string
	}
	check := func(section, prefix string, settings []setting) {
		live, _ := doc[section].(map[string]interface{})
		for _, s := range settings {
			if _, reported := live[s.key]; s.set && !reported {
				missing = append(missing, prefix+s.field+" ("+s.key+")")
			}
		}
	}
	if config.Relay != nil {
		for i, r := range config.Relay.Relays {
			delay := r.PowerRestoreDelay
			check("switch:"+strconv.Itoa(r.ID), "relay.relays."+strconv.Itoa(i)+".", []setting{
				{r.MaxPower != nil, "max_power", "power_limit"},
				{r.VoltageLimit != nil, "voltage_limit", "voltage_limit"},
				{r.UndervoltageLimit != nil, "undervoltage_limit", "undervoltage_limit"},
				{r.CurrentLimit != nil, "current_limit", "current_limit"},
				{r.InLocked != nil, "in_locked", "in_locked"},
				{r.AutorecoverVoltageErrors != nil, "autorecover_voltage_errors", "autorecover_voltage_errors"},
				{delay != nil, "power_restore_delay", "power_restore_delay_enable"},
				{delay != nil && *delay > 0, "power_restore_delay", "power_restore_delay"},
			})
		}
	}
	if config.Input != nil {
		for i, in := range config.Input.Inputs {
			check("input:"+strconv.Itoa(in.ID), "input.inputs."+strconv.Itoa(i)+".", []setting{
				{in.Enable != nil, "enable", "enable"},
				{in.FactoryReset != nil, "factory_reset", "factory_reset"},
			})
		}
	}
	return missing
}

// unkeyedFields names, and clears in v, every field of the struct that v points to which is set and
// that no Gen2 key carries. keep lists the JSON names that stay.
func unkeyedFields(v interface{}, prefix string, keep map[string]bool) []string {
	var paths []string
	elem := reflect.ValueOf(v).Elem()
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Type().Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if keep[name] || elem.Field(i).Kind() != reflect.Pointer || elem.Field(i).IsNil() {
			continue
		}
		paths = append(paths, prefix+"."+name)
		elem.Field(i).Set(reflect.Zero(field.Type))
	}
	return paths
}

// unmappedChannelSettings returns the paths of the switch and input settings that no Gen2 key or
// word carries, and a copy of the configuration without them. Each path is named on its own, so a
// channel that sets nothing else still gets its own paths. A channel left with no setting is not in
// the copy. The caller's configuration is never changed. The third value maps the channel list of the
// copy ("relay.relays", "input.inputs") to the original index of each channel it keeps.
func unmappedChannelSettings(desired *DeviceConfiguration) ([]string, *DeviceConfiguration, map[string][]int) {
	var paths []string
	checked := *desired
	kept := map[string][]int{}

	if desired.Relay != nil {
		relay := *desired.Relay
		paths = append(paths, unkeyedFields(&relay, "relay", map[string]bool{"relays": true})...)
		relay.Relays = nil
		seenRelays := map[int]bool{}
		for i, source := range desired.Relay.Relays {
			r := source
			prefix := "relay.relays." + strconv.Itoa(i)
			paths = append(paths, unkeyedFields(&r, prefix, gen2ChannelFields)...)
			if r.DefaultState != nil {
				if _, ok := gen2InitialStates[*r.DefaultState]; !ok {
					paths, r.DefaultState = append(paths, prefix+".default_state"), nil
				}
			}
			// The reader takes a delay up to MaxInt32, so a larger one cannot round trip.
			if r.AutoOn != nil && (*r.AutoOn < 0 || *r.AutoOn > math.MaxInt32) {
				paths, r.AutoOn = append(paths, prefix+".auto_on"), nil
			}
			if r.AutoOff != nil && (*r.AutoOff < 0 || *r.AutoOff > math.MaxInt32) {
				paths, r.AutoOff = append(paths, prefix+".auto_off"), nil
			}
			paths = append(paths, unmappedLimits(&r, prefix)...)
			// A negative id can never be written, so it is named here, at its place in the desired list,
			// and the converter never sees it.
			if r.ID < 0 {
				paths = append(paths, prefix+".id")
				continue
			}
			// A second channel with the same id cannot say which channel its settings belong to. An entry
			// that sets nothing holds its id too: verify compares the desired list as it stands.
			if seenRelays[r.ID] {
				paths = append(paths, prefix+".id")
				continue
			}
			seenRelays[r.ID] = true
			// What is left is a setting that has a key: the unkeyed ones were cleared above.
			if r != (SingleRelayConfig{ID: r.ID}) {
				relay.Relays = append(relay.Relays, r)
				kept["relay.relays"] = append(kept["relay.relays"], i)
			}
		}
		checked.Relay = nil
		if len(relay.Relays) > 0 {
			checked.Relay = &relay
		}
	}

	if desired.Input != nil {
		input := *desired.Input
		paths = append(paths, unkeyedFields(&input, "input", map[string]bool{"inputs": true})...)
		input.Inputs = nil
		seenInputs := map[int]bool{}
		for i, source := range desired.Input.Inputs {
			in := source
			prefix := "input.inputs." + strconv.Itoa(i)
			paths = append(paths, unkeyedFields(&in, prefix, gen2ChannelFields)...)
			if in.Mode != nil {
				if _, ok := gen2InModes[*in.Mode]; !ok {
					paths, in.Mode = append(paths, prefix+".mode"), nil
				}
			}
			if in.Type != nil && !gen2InputTypes[*in.Type] {
				paths, in.Type = append(paths, prefix+".type"), nil
			}
			if in.ID < 0 {
				paths = append(paths, prefix+".id")
				continue
			}
			if seenInputs[in.ID] {
				paths = append(paths, prefix+".id")
				continue
			}
			seenInputs[in.ID] = true
			if in != (SingleInputConfig{ID: in.ID}) {
				input.Inputs = append(input.Inputs, in)
				kept["input.inputs"] = append(kept["input.inputs"], i)
			}
		}
		checked.Input = nil
		if len(input.Inputs) > 0 {
			checked.Input = &input
		}
	}
	return paths, &checked, kept
}

// originalChannelPath turns a path whose channel index counts the kept channels into the path of
// the same channel in the desired list. Any other path is returned as it is.
func originalChannelPath(path string, kept map[string][]int) string {
	for list, original := range kept {
		rest, ok := strings.CutPrefix(path, list+".")
		if !ok {
			continue
		}
		index, tail, _ := strings.Cut(rest, ".")
		n, err := strconv.Atoi(index)
		if err != nil || n < 0 || n >= len(original) {
			return path
		}
		path = list + "." + strconv.Itoa(original[n])
		if tail != "" {
			path += "." + tail
		}
		return path
	}
	return path
}

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

// gen2ChannelID returns the id of a "switch:2" or "input:2" key of the given kind.
func gen2ChannelID(key, kind string) (int, bool) {
	rest, ok := strings.CutPrefix(key, kind+":")
	if !ok {
		return 0, false
	}
	id, err := strconv.Atoi(rest)
	return id, err == nil && id >= 0
}

// gen2KeyApplies reports whether apply writes a top-level key of a Gen2 config document.
func gen2KeyApplies(key string) bool {
	if gen2ApplyKeys[key] {
		return true
	}
	if _, ok := gen2ChannelID(key, "switch"); ok {
		return true
	}
	_, ok := gen2ChannelID(key, "input")
	return ok
}

// gen2ChannelIDs returns the ids of one kind of channel in a document, ascending.
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
		relay.AutoOn = gen2AutoTimerFromDoc(raw, "auto_on")
		relay.AutoOff = gen2AutoTimerFromDoc(raw, "auto_off")
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

// gen2AutoTimerFromDoc reads auto_on or auto_off with its delay as one number of seconds. An
// enabled timer whose delay is not a whole number above 0 has no stored value: it is read as unset,
// so a desired value reports a difference. It is never turned into 0, which means off.
func gen2AutoTimerFromDoc(raw map[string]interface{}, key string) *int {
	enabled, ok := raw[key].(bool)
	if !ok {
		return nil
	}
	if !enabled {
		return IntPtr(0)
	}
	delay, ok := raw[key+"_delay"].(float64)
	if !ok || delay < 1 || delay != math.Trunc(delay) || delay > math.MaxInt32 {
		return nil
	}
	return IntPtr(int(delay))
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
			gen2AutoTimerToDoc(body, "auto_on", relay.AutoOn)
			gen2AutoTimerToDoc(body, "auto_off", relay.AutoOff)
		}
	}

	for id, body := range switches {
		if len(body) > 0 {
			doc["switch:"+strconv.Itoa(id)] = body
		}
	}
}

// gen2AutoTimerToDoc writes seconds above 0 as the timer on with that delay, and 0 as the timer off
// with no delay. A negative number is not written.
func gen2AutoTimerToDoc(body map[string]interface{}, key string, seconds *int) {
	if seconds == nil || *seconds < 0 {
		return
	}
	body[key] = *seconds > 0
	if *seconds > 0 {
		body[key+"_delay"] = *seconds
	}
}

// gen2ChannelFields are the fields of a switch or input channel that have a Gen2 key. Every other
// field that a desired channel sets has no key to write.
var gen2ChannelFields = map[string]bool{
	"id": true, "name": true, "default_state": true, "auto_on": true, "auto_off": true,
	"type": true, "mode": true, "inverted": true,
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
// the copy. The caller's configuration is never changed.
func unmappedChannelSettings(desired *DeviceConfiguration) ([]string, *DeviceConfiguration) {
	var paths []string
	checked := *desired

	if desired.Relay != nil {
		relay := *desired.Relay
		paths = append(paths, unkeyedFields(&relay, "relay", map[string]bool{"relays": true})...)
		relay.Relays = nil
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
			if r.Name != nil || r.DefaultState != nil || r.AutoOn != nil || r.AutoOff != nil {
				relay.Relays = append(relay.Relays, r)
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
			if in.Name != nil || in.Type != nil || in.Mode != nil || in.Inverted != nil {
				input.Inputs = append(input.Inputs, in)
			}
		}
		checked.Input = nil
		if len(input.Inputs) > 0 {
			checked.Input = &input
		}
	}
	return paths, &checked
}

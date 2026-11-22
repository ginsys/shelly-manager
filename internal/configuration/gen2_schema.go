package configuration

import (
	"reflect"
	"strings"
)

// gen2SectionPaths are the section-level fields that the Gen2 apply writes and no field map names.
// TestGen2DeviceSchemaApplicableMatchesApply fails when this list and unsupportedGen2Settings disagree.
// wifi.enable is not here: apply refuses false, and a station with an ssid is enabled already.
var gen2SectionPaths = []string{
	"system.device.name", "system.device.eco_mode", "system.device.discoverable", "system.sntp.server",
	"location.tz", "location.lat", "location.lng",
	"mqtt.enable", "mqtt.server", "mqtt.port", "mqtt.user", "mqtt.pass", "mqtt.id", "mqtt.topic_prefix",
	"mqtt.rpc_ntf", "mqtt.status_ntf",
	"cloud.enable", "cloud.server",
	"auth.enable", "auth.user", "auth.pass",
	"wifi.ssid", "wifi.password", "wifi.ipv4mode", "wifi.roam_threshold",
	"wifi.static_ip.ip", "wifi.static_ip.netmask", "wifi.static_ip.gw", "wifi.static_ip.nameserver",
	"network.eth.enable", "network.eth.ipv4mode",
	"network.eth.ip.ip", "network.eth.ip.netmask", "network.eth.ip.gw", "network.eth.ip.nameserver",
}

// gen2ApplicablePaths are the DeviceConfiguration field paths that a Gen2 apply writes. The channel,
// light and roller fields come from the maps the Gen2 check reads (gen2ChannelFields, gen2LightFields,
// gen2RollerFields), matched to the json tags of the stored structs, so a key a later slice adds to
// one of them is marked with no edit here.
var gen2ApplicablePaths = buildGen2ApplicablePaths()

func buildGen2ApplicablePaths() map[string]bool {
	paths := map[string]bool{}
	for _, path := range gen2SectionPaths {
		paths[path] = true
	}
	for _, source := range []struct {
		stored interface{}
		prefix string
		keyed  map[string]bool
	}{
		{SingleRelayConfig{}, "relay.relays[].", gen2ChannelFields},
		{SingleInputConfig{}, "input.inputs[].", gen2ChannelFields},
		{SingleLightConfig{}, "light.lights[].", gen2LightFields},
		{RollerConfig{}, "roller.", gen2RollerFields},
	} {
		stored := reflect.TypeOf(source.stored)
		for i := 0; i < stored.NumField(); i++ {
			name := strings.Split(stored.Field(i).Tag.Get("json"), ",")[0]
			if source.keyed[name] && name != "id" { // an item's id is its position, not a setting
				paths[source.prefix+name] = true
			}
		}
	}
	return paths
}

// gen2DeviceSchema is the schema of a Gen2 or later device: the sections its apply can write, each
// leaf marked from gen2ApplicablePaths. It is generation-wide: a device that lacks a component
// (no Ethernet, no cover profile) still shows its fields applicable, and apply refuses them from the
// live document before any write.
func gen2DeviceSchema(deviceType string) map[string]interface{} {
	sections := map[string]interface{}{
		"wifi":     getWiFiSchema(),
		"mqtt":     getMQTTSchema(),
		"auth":     getAuthSchema(),
		"system":   getSystemSchema(),
		"cloud":    getCloudSchema(),
		"location": getLocationSchema(),
		"network":  getDeviceNetworkSchema(),
		"relay":    getRelaySchema(),
		"input":    getInputSchema(),
		"roller":   getRollerSchema(),
		"light":    getLightSchema(),
	}
	// A word the Gen2 check refuses leaves the enum: the stored words are the union of both generations.
	onlyWords(sections["input"], []string{"inputs", "type"}, func(word string) bool { return gen2InputTypes[word] })
	onlyWords(sections["light"], []string{"lights", "default_state"}, func(word string) bool { _, ok := gen2LightInitialStates[word]; return ok })
	// A Gen2 device has one user, and the cover takes whole seconds from 1 to 300 (gen2_cover.go).
	if user := schemaNodeAt(sections["auth"], "user"); user != nil {
		user["enum"] = []string{gen2LoginUser}
	}
	for _, name := range []string{"max_open_time", "max_close_time", "obstacle_delay"} {
		if seconds := schemaNodeAt(sections["roller"], name); seconds != nil {
			seconds["minimum"], seconds["maximum"] = gen2CoverSecondsMin, gen2CoverSecondsMax
		}
	}
	schema := map[string]interface{}{
		"$schema":               "http://json-schema.org/draft-07/schema#",
		"type":                  "object",
		"title":                 "Shelly Device Configuration",
		"properties":            sections,
		SchemaKeyDeviceType:     deviceType,
		SchemaKeyApplySupported: true,
	}
	markApplicable(schema, "", func(path string) bool { return gen2ApplicablePaths[path] }, nil)
	return schema
}

// schemaNodeAt returns the node at a path of property names below a section, entering an array's
// items where a property is an array, or nil.
func schemaNodeAt(section interface{}, path ...string) map[string]interface{} {
	node, _ := section.(map[string]interface{})
	for _, name := range path {
		props := schemaProperties(node)
		if props == nil {
			if items, ok := node["items"].(map[string]interface{}); ok {
				props = schemaProperties(items)
			}
		}
		node, _ = props[name].(map[string]interface{})
	}
	return node
}

// onlyWords keeps the words of the enum at path that allowed accepts.
func onlyWords(section interface{}, path []string, allowed func(string) bool) {
	node := schemaNodeAt(section, path...)
	if node == nil {
		return
	}
	words, _ := node["enum"].([]string)
	var kept []string
	for _, word := range words {
		if allowed(word) {
			kept = append(kept, word)
		}
	}
	node["enum"] = kept
}

package configuration

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Gen2Converter converts between the stored configuration and the document that a Gen2 or later
// device returns from Shelly.GetConfig, which is keyed by component (sys, wifi, mqtt, switch:0 and
// so on). It reads and writes the sys, mqtt, cloud, switch and input sections and the Wi-Fi station
// so far: a setting of any other section has no mapping, so the round-trip check in
// unsupportedGen2Settings refuses it before the device is touched. Each later section is added
// together with the code that writes it.
type Gen2Converter struct {
	logger *logging.Logger
}

// NewGen2Converter creates a Gen2 converter.
func NewGen2Converter(logger *logging.Logger) *Gen2Converter {
	if logger == nil {
		logger = logging.GetDefault()
	}
	return &Gen2Converter{logger: logger}
}

// Generation returns 2: Gen3 and later devices speak the same RPC.
func (c *Gen2Converter) Generation() int { return 2 }

// SupportedDeviceTypes returns no list: the Gen2 path does not match on a model name.
func (c *Gen2Converter) SupportedDeviceTypes() []string { return nil }

// FromAPIConfig reads the sys section of a Shelly.GetConfig document. A null value, such as a
// device with no name, is an unset setting.
func (c *Gen2Converter) FromAPIConfig(apiJSON json.RawMessage, _ string) (*DeviceConfiguration, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(apiJSON, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse Gen2 config: %w", err)
	}
	config := &DeviceConfiguration{}
	sys, _ := doc["sys"].(map[string]interface{})

	device := &TypedDeviceConfig{}
	if raw, ok := sys["device"].(map[string]interface{}); ok {
		if name, ok := raw["name"].(string); ok {
			device.Name = StringPtr(name)
		}
		if eco, ok := raw["eco_mode"].(bool); ok {
			device.EcoMode = BoolPtr(eco)
		}
		if discoverable, ok := raw["discoverable"].(bool); ok {
			device.Discoverable = BoolPtr(discoverable)
		}
	}
	if device.Name != nil || device.EcoMode != nil || device.Discoverable != nil {
		config.System = &SystemConfiguration{Device: device}
	}

	// An empty time server means none, and cannot be told from an unset one in SNTPConfig.
	if raw, ok := sys["sntp"].(map[string]interface{}); ok {
		if server, ok := raw["server"].(string); ok && server != "" {
			if config.System == nil {
				config.System = &SystemConfiguration{}
			}
			config.System.SNTP = &SNTPConfig{Server: server}
		}
	}

	location := &LocationConfiguration{}
	if raw, ok := sys["location"].(map[string]interface{}); ok {
		if tz, ok := raw["tz"].(string); ok {
			location.Timezone = StringPtr(tz)
		}
		if lat, ok := raw["lat"].(float64); ok {
			location.Latitude = Float64Ptr(lat)
		}
		if lon, ok := raw["lon"].(float64); ok {
			location.Longitude = Float64Ptr(lon)
		}
	}
	if location.Timezone != nil || location.Latitude != nil || location.Longitude != nil {
		config.Location = location
	}

	config.MQTT = gen2MQTTFromDoc(doc["mqtt"])
	if raw, ok := doc["cloud"].(map[string]interface{}); ok {
		cloud := &CloudConfiguration{}
		if enable, ok := raw["enable"].(bool); ok {
			cloud.Enable = BoolPtr(enable)
		}
		if server, ok := raw["server"].(string); ok && server != "" {
			cloud.Server = StringPtr(server)
		}
		if cloud.Enable != nil || cloud.Server != nil {
			config.Cloud = cloud
		}
	}
	config.Relay, config.Input = gen2ChannelsFromDoc(doc)
	config.WiFi = gen2WiFiFromDoc(doc["wifi"])
	return config, nil
}

// UnreadSettings names the settings of a Shelly.GetConfig document that FromAPIConfig does not read, and
// returns them with their values. A setting is read when removing it, or changing it to another value of
// its kind, changes what FromAPIConfig returns: a read setting at its zero value (an eco_mode of false)
// reads the same when it is removed, so removal alone would call it unread. A top-level key that nothing
// in it is read from is named as the key, any other unread setting by its dotted path. The rest is the
// document without its read settings and without empty objects. The paths hold no values.
func (c *Gen2Converter) UnreadSettings(apiJSON json.RawMessage) ([]string, map[string]interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(apiJSON, &doc); err != nil {
		return nil, nil, fmt.Errorf("failed to parse Gen2 config: %w", err)
	}
	base, err := c.FromAPIConfig(apiJSON, "")
	if err != nil {
		return nil, nil, err
	}
	probe := &readProbe{converter: c, doc: doc, base: base}

	rest := map[string]interface{}{}
	var paths []string
	for _, key := range sortedKeys(doc) {
		// FromAPIConfig reads only the keys apply writes, so any other key is unread whole, without a probe.
		if !gen2KeyApplies(key) {
			rest[key] = doc[key]
			paths = append(paths, key)
			continue
		}
		r := probe.walk(doc, key, key)
		if r.unread {
			rest[key] = r.rest
		}
		if r.unread && r.reads == 0 {
			paths = append(paths, key)
		} else {
			paths = append(paths, r.paths...)
		}
	}
	sort.Strings(paths)
	return paths, rest, nil
}

// readProbe tries each setting of doc against the result for the whole document.
type readProbe struct {
	converter *Gen2Converter
	doc       map[string]interface{}
	base      *DeviceConfiguration
}

type probeResult struct {
	rest   interface{} // the unread part of the value
	unread bool        // whether there is one
	paths  []string
	reads  int
}

func (p *readProbe) walk(parent map[string]interface{}, key, path string) probeResult {
	child, isObject := parent[key].(map[string]interface{})
	if !isObject {
		if p.isRead(parent, key) {
			return probeResult{reads: 1}
		}
		return probeResult{rest: parent[key], unread: true, paths: []string{path}}
	}
	var out probeResult
	kept := map[string]interface{}{}
	for _, k := range sortedKeys(child) {
		r := p.walk(child, k, path+"."+k)
		out.paths = append(out.paths, r.paths...)
		out.reads += r.reads
		if r.unread {
			kept[k] = r.rest
		}
	}
	if len(kept) > 0 {
		out.rest, out.unread = kept, true
	}
	return out
}

// isRead reports whether the setting changes the result when it is removed or changed. The setting is
// put back before it returns.
func (p *readProbe) isRead(parent map[string]interface{}, key string) bool {
	// input_id only chooses which input receives the in_mode of a switch. It has no typed home, so it
	// stays in the raw part although changing it changes the result.
	if key == "input_id" {
		return false
	}
	// An enabled auto timer whose delay is not a whole number above 0 has no typed value, so the flag
	// is not read: it stays in the raw part beside its delay.
	if key == "auto_on" || key == "auto_off" {
		if enabled, ok := parent[key].(bool); ok && enabled && gen2AutoTimerFromDoc(parent, key) == nil {
			return false
		}
	}
	original := parent[key]
	defer func() { parent[key] = original }()
	delete(parent, key)
	if p.changed() {
		return true
	}
	var variants []interface{}
	switch v := original.(type) {
	case bool:
		variants = []interface{}{!v}
	case float64:
		variants = []interface{}{v + 1}
	case string:
		variants = []interface{}{v + "x"}
	case nil:
		variants = []interface{}{"x", true, float64(1)}
	}
	for _, v := range variants {
		parent[key] = v
		if p.changed() {
			return true
		}
	}
	return false
}

// changed reports whether the document now reads differently. A document that cannot be read counts as
// unchanged, so the setting stays in the raw part and is never dropped.
func (p *readProbe) changed() bool {
	data, err := json.Marshal(p.doc)
	if err != nil {
		return false
	}
	got, err := p.converter.FromAPIConfig(data, "")
	return err == nil && !reflect.DeepEqual(p.base, got)
}

// redactedSecret is the text the API shows in place of a secret that is set.
const redactedSecret = "********"

// gen2MQTTFromDoc reads the mqtt section. The device returns no password. A null value, such as a
// broker that is not set, is an unset setting. The server is one "host:port" string that is split
// into server and port.
func gen2MQTTFromDoc(section interface{}) *MQTTConfiguration {
	raw, ok := section.(map[string]interface{})
	if !ok {
		return nil
	}
	mqtt := &MQTTConfiguration{}
	if enable, ok := raw["enable"].(bool); ok {
		mqtt.Enable = BoolPtr(enable)
	}
	if server, ok := raw["server"].(string); ok && server != "" {
		host, port, err := net.SplitHostPort(server)
		portNumber, convErr := strconv.Atoi(port)
		if err != nil || convErr != nil {
			mqtt.Server = StringPtr(server)
		} else {
			mqtt.Server = StringPtr(host)
			mqtt.Port = IntPtr(portNumber)
		}
	}
	if user, ok := raw["user"].(string); ok {
		mqtt.User = StringPtr(user)
	}
	if id, ok := raw["client_id"].(string); ok {
		mqtt.ClientID = StringPtr(id)
	}
	if prefix, ok := raw["topic_prefix"].(string); ok {
		mqtt.TopicPrefix = StringPtr(prefix)
	}
	if rpc, ok := raw["rpc_ntf"].(bool); ok {
		mqtt.RPCNotifications = BoolPtr(rpc)
	}
	if status, ok := raw["status_ntf"].(bool); ok {
		mqtt.StatusNotification = BoolPtr(status)
	}
	return mqtt
}

// gen2MQTTToDoc returns the keys of MQTT.SetConfig that the configuration sets. The password is
// sent only with a user, and never when it is empty or is the mask the API shows: neither is a
// password. A setting with no key here, such as keep_alive, is never sent.
func gen2MQTTToDoc(m *MQTTConfiguration) map[string]interface{} {
	mqtt := map[string]interface{}{}
	if m.Enable != nil {
		mqtt["enable"] = *m.Enable
	}
	if m.Server != nil {
		server := *m.Server
		if m.Port != nil {
			server = net.JoinHostPort(server, strconv.Itoa(*m.Port))
		}
		mqtt["server"] = server
	}
	if m.User != nil {
		mqtt["user"] = *m.User
	}
	if gen2PasswordIsSet(m) {
		mqtt["pass"] = *m.Password
	}
	if m.ClientID != nil {
		mqtt["client_id"] = *m.ClientID
	}
	if m.TopicPrefix != nil {
		mqtt["topic_prefix"] = *m.TopicPrefix
	}
	if m.RPCNotifications != nil {
		mqtt["rpc_ntf"] = *m.RPCNotifications
	}
	if m.StatusNotification != nil {
		mqtt["status_ntf"] = *m.StatusNotification
	}
	return mqtt
}

// gen2PasswordIsSet reports whether the configuration holds a password to write.
func gen2PasswordIsSet(m *MQTTConfiguration) bool {
	return m.Password != nil && *m.Password != "" && *m.Password != redactedSecret
}

// ToAPIConfig returns the sys section as the config object of Sys.SetConfig, with only the keys
// the configuration sets: the documentation does not say that SetConfig merges, so a key left out
// is never sent.
func (c *Gen2Converter) ToAPIConfig(config *DeviceConfiguration, _ string) (json.RawMessage, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	sys := map[string]interface{}{}

	device := map[string]interface{}{}
	if d := configDevice(config); d != nil {
		if d.Name != nil {
			device["name"] = *d.Name
		}
		if d.EcoMode != nil {
			device["eco_mode"] = *d.EcoMode
		}
		if d.Discoverable != nil {
			device["discoverable"] = *d.Discoverable
		}
	}
	if len(device) > 0 {
		sys["device"] = device
	}

	location := map[string]interface{}{}
	if l := config.Location; l != nil {
		if l.Timezone != nil {
			location["tz"] = *l.Timezone
		}
		if l.Latitude != nil {
			location["lat"] = *l.Latitude
		}
		if l.Longitude != nil {
			location["lon"] = *l.Longitude
		}
	}
	if len(location) > 0 {
		sys["location"] = location
	}

	if config.System != nil && config.System.SNTP != nil && config.System.SNTP.Server != "" {
		sys["sntp"] = map[string]interface{}{"server": config.System.SNTP.Server}
	}

	doc := map[string]interface{}{}
	if len(sys) > 0 {
		doc["sys"] = sys
	}
	if config.MQTT != nil {
		if mqtt := gen2MQTTToDoc(config.MQTT); len(mqtt) > 0 {
			doc["mqtt"] = mqtt
		}
	}
	if c := config.Cloud; c != nil {
		cloud := map[string]interface{}{}
		if c.Enable != nil {
			cloud["enable"] = *c.Enable
		}
		if c.Server != nil {
			cloud["server"] = *c.Server
		}
		if len(cloud) > 0 {
			doc["cloud"] = cloud
		}
	}
	gen2ChannelsToDoc(config, doc)
	if config.WiFi != nil {
		if wifi := gen2WiFiToDoc(config.WiFi); len(wifi) > 0 {
			doc["wifi"] = wifi
		}
	}
	return json.Marshal(doc)
}

func configDevice(config *DeviceConfiguration) *TypedDeviceConfig {
	if config.System == nil {
		return nil
	}
	return config.System.Device
}

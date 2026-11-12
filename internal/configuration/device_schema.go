package configuration

// Schema extension keywords added by GetDeviceConfigurationSchema.
const (
	// SchemaKeyApplicable marks a field that apply can write to the device today.
	SchemaKeyApplicable = "x-applicable"
	// SchemaKeyDeviceType names the device model a schema was built for.
	SchemaKeyDeviceType = "x-device-type"
	// SchemaKeyApplySupported is true when apply supports the model at all.
	SchemaKeyApplySupported = "x-apply-supported"
)

// GetDeviceConfigurationSchema returns the schema of the configuration that is stored for a
// device of one model: overrides, templates and the desired configuration all use this shape
// (DeviceConfiguration).
//
// Unlike GetConfigurationSchema, it holds only fields that DeviceConfiguration stores, so a form
// built on it never writes a key that is silently dropped (TestDeviceSchemaHoldsOnlyStoredFields).
// For a supported Gen1 model it holds only the sections that model has, applicable or not (the
// network section, and roller for a model with roller mode, are stored but never applied). Every field carries
// SchemaKeyApplicable: true when apply can write that field to this model today. For a model
// apply does not support, every section of DeviceConfiguration is present and no field is
// applicable.
func GetDeviceConfigurationSchema(deviceType string) map[string]interface{} {
	converter := NewGen1Converter(nil)
	supported := false
	for _, t := range converter.SupportedDeviceTypes() {
		if t == deviceType {
			supported = true
			break
		}
	}

	sections := map[string]interface{}{
		"wifi":     getWiFiSchema(),
		"mqtt":     getMQTTSchema(),
		"auth":     getAuthSchema(),
		"system":   getSystemSchema(),
		"cloud":    getCloudSchema(),
		"location": getLocationSchema(),
		"coiot":    getDeviceCoIoTSchema(),
		"network":  getDeviceNetworkSchema(),
	}
	if !supported || converter.hasRelay(deviceType) {
		sections["relay"] = getRelaySchema()
	}
	if !supported || converter.hasPowerMetering(deviceType) {
		sections["power_metering"] = getPowerMeteringSchema()
	}
	if !supported || converter.hasLED(deviceType) {
		sections["led"] = getLEDSchema()
	}
	if !supported || hasInputSettings(deviceType) {
		sections["input"] = getInputSchema()
	}
	if !supported || HasRollerMode(deviceType) {
		sections["roller"] = getRollerSchema()
	}
	if !supported {
		// No supported Gen1 model is a dimmer.
		sections["dimming"] = getDimmingSchema()
	}

	schema := map[string]interface{}{
		"$schema":               "http://json-schema.org/draft-07/schema#",
		"type":                  "object",
		"title":                 "Shelly Device Configuration",
		"properties":            sections,
		SchemaKeyDeviceType:     deviceType,
		SchemaKeyApplySupported: supported,
	}
	markApplicable(schema, "", supported)
	return schema
}

// GetTemplateConfigurationSchema returns the schema of a global or group template: every
// section DeviceConfiguration stores, because such a template can reach any model. A field is
// marked applicable when apply can write it on at least one supported model. A device_type
// template uses GetDeviceConfigurationSchema of its model instead.
func GetTemplateConfigurationSchema() map[string]interface{} {
	schema := GetDeviceConfigurationSchema("")
	schema[SchemaKeyApplySupported] = true
	markApplicable(schema, "", true)
	return schema
}

// hasInputSettings reports whether a Gen1 model has configurable switch inputs.
func hasInputSettings(deviceType string) bool {
	switch deviceType {
	case "SHSW-1", "SHSW-PM", "SHSW-25", "SHIX3-1":
		return true
	default:
		return false
	}
}

// HasRollerMode reports whether a Gen1 model can run as a roller (cover) controller. The device
// schema and the capabilities route both read it, so one place decides.
func HasRollerMode(deviceType string) bool {
	return deviceType == "SHSW-25"
}

// getDeviceNetworkSchema describes NetworkConfiguration, the network section DeviceConfiguration
// stores: Ethernet, and the typed Wi-Fi station and access point. Apply never writes it.
// Each call builds fresh maps, because markApplicable writes into every leaf.
func getDeviceNetworkSchema() map[string]interface{} {
	str := func(title string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "title": title}
	}
	secret := func(title string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "title": title, "format": "password"}
	}
	flag := func(title string) map[string]interface{} {
		return map[string]interface{}{"type": "boolean", "title": title}
	}
	obj := func(title string, props map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"type": "object", "title": title, "properties": props}
	}
	ipv4mode := func() map[string]interface{} {
		return map[string]interface{}{"type": "string", "title": "IP Mode", "enum": []string{"dhcp", "static"}}
	}
	staticIP := func() map[string]interface{} {
		return obj("Static IP Settings", map[string]interface{}{
			"ip": str("IP Address"), "netmask": str("Netmask"), "gw": str("Gateway"), "nameserver": str("DNS Server"),
		})
	}
	return map[string]interface{}{
		"type":        "object",
		"title":       "Network Settings",
		"description": "Ethernet and Wi-Fi station and access point settings",
		"properties": map[string]interface{}{
			"eth": obj("Ethernet", map[string]interface{}{
				"enable": flag("Enable Ethernet"), "ipv4mode": ipv4mode(), "ip": staticIP(),
			}),
			"wifi": obj("Wi-Fi", map[string]interface{}{
				"sta": obj("Station", map[string]interface{}{
					"enable": flag("Enable"), "ssid": str("Network Name (SSID)"), "pass": secret("Password"),
					"is_open": flag("Open Network"), "ipv4mode": ipv4mode(), "ip": staticIP(),
					"nameserver": str("DNS Server"),
				}),
				"ap": obj("Access Point", map[string]interface{}{
					"enable": flag("Enable"), "ssid": str("Network Name (SSID)"), "pass": secret("Password"),
					"is_open":     flag("Open Network"),
					"max_sta":     map[string]interface{}{"type": "integer", "title": "Max Clients"},
					"range_start": str("DHCP Range Start"), "range_end": str("DHCP Range End"),
				}),
			}),
		},
	}
}

// getDeviceCoIoTSchema describes CoIoTConfiguration, the CoIoT section DeviceConfiguration
// stores. getCoIoTSchema describes the TypedConfiguration section, whose keys differ.
func getDeviceCoIoTSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "CoIoT Settings",
		"description": "Configure CoIoT protocol (local multicast)",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable CoIoT",
				"description": "Enable CoIoT protocol for local discovery",
			},
			"peer": map[string]interface{}{
				"type":        "string",
				"title":       "Peer",
				"description": "CoIoT unicast peer (host:port); empty for multicast",
			},
			"update_period": map[string]interface{}{
				"type":        "integer",
				"title":       "Update Period",
				"description": "Status update interval in seconds",
				"minimum":     gen1CoIoTMinPeriod,
				"maximum":     gen1CoIoTMaxPeriod,
			},
		},
	}
}

// markApplicable sets SchemaKeyApplicable on every leaf field below node. path is the dotted
// path of node; array items add "[]".
func markApplicable(node map[string]interface{}, path string, supported bool) {
	if props, ok := node["properties"].(map[string]interface{}); ok {
		for name, child := range props {
			childPath := name
			if path != "" {
				childPath = path + "." + name
			}
			if childMap, ok := child.(map[string]interface{}); ok {
				markApplicable(childMap, childPath, supported)
			}
		}
		return
	}
	if items, ok := node["items"].(map[string]interface{}); ok {
		if _, isObject := items["properties"]; isObject {
			markApplicable(items, path+"[]", supported)
			return
		}
	}
	node[SchemaKeyApplicable] = supported && gen1ApplicablePaths[path]
}

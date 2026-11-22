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
// network section is stored but never applied, and roller for a model with roller mode is applied
// for five of its fields). Every field carries
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
	withoutModeOfModelsWithoutIt(sections, deviceType, supported)
	withoutRecoveryOfModelsWithoutIt(sections, deviceType, supported)
	if !supported || converter.hasRelay(deviceType) {
		sections["relay"] = getRelayStates(getRelaySchema(), supported)
	}
	if !supported || converter.hasPowerMetering(deviceType) {
		sections["power_metering"] = getPowerMeteringSchema()
	}
	if !supported || converter.hasStatusLED(deviceType) {
		sections["led"] = withoutPowerLEDOfModelsWithoutIt(getLEDSchema(), converter.hasLED(deviceType), supported)
	}
	if !supported || hasInputSettings(deviceType) {
		sections["input"] = getInputModes(getInputSchema(), deviceType)
	}
	if !supported || HasRollerMode(deviceType) {
		sections["roller"] = getRollerSchema()
	}
	if _, hasActions := gen1ActionTable(deviceType); !supported || hasActions {
		sections["actions"] = getActionsSchema(deviceType)
	}
	if !supported || converter.hasLight(deviceType) {
		sections["light"] = lightSchemaOf(getLightSchema(), deviceType, supported)
	}
	if !supported || converter.hasDimmerSettings(deviceType) {
		// Five fields of the Dimmer's device-level group are applied (gen1ApplicablePaths); the rest of
		// it is stored and not written, so those fields are not applicable.
		sections["dimming"] = withoutEdgeOfDimmerWithoutDetection(getDimmingSchema(), deviceType)
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

// getInputModes narrows the input mode enum of a supported model to the modes apply accepts for
// it: a Shelly i3 has toggle and momentary inputs only, so it keeps momentary and follow. Another
// model, and a template with no model, keep every mode.
func getInputModes(schema map[string]interface{}, deviceType string) map[string]interface{} {
	if !hasInputSettings(deviceType) {
		return schema
	}
	props, _ := schema["properties"].(map[string]interface{})
	inputs, _ := props["inputs"].(map[string]interface{})
	items, _ := inputs["items"].(map[string]interface{})
	itemProps, _ := items["properties"].(map[string]interface{})
	mode, _ := itemProps["mode"].(map[string]interface{})
	if mode == nil {
		return schema
	}
	var allowed []string
	for _, name := range mode["enum"].([]string) {
		if _, known := gen1ButtonTypes[name]; !known {
			continue // a Gen2 word (cycle, activate): a Gen1 model has no such button type
		}
		if inputsLiveInRelays(deviceType) || gen1ButtonTypesOfI3[name] {
			allowed = append(allowed, name)
		}
	}
	mode["enum"] = allowed
	return schema
}

// lightSchemaOf narrows the light schema of a supported model to its own keys and words. A template
// with no model keeps every key.
func lightSchemaOf(schema map[string]interface{}, deviceType string, supported bool) map[string]interface{} {
	if !supported {
		return schema
	}
	lights, _ := schemaProperties(schema)["lights"].(map[string]interface{})
	items := schemaProperties(lights["items"])
	if items == nil {
		return schema
	}
	if deviceType != "SHRGBW2" {
		for _, key := range []string{"name", "effect", "transition", "btn_reverse"} {
			delete(items, key)
		}
		return schema
	}
	delete(items, "btn_debounce")
	delete(items, "swap_inputs")
	if state, ok := items["default_state"].(map[string]interface{}); ok {
		state["enum"] = gen1RGBW2States
	}
	if button, ok := items["btn_type"].(map[string]interface{}); ok {
		button["enum"] = gen1RGBW2ButtonTypes
	}
	return schema
}

// gen1InitialStates are the stored initial states a Gen1 model accepts. restore_last and
// match_input are the Gen2 words for last and switch.
var gen1InitialStates = map[string]bool{"off": true, "on": true, "last": true, "switch": true}

// getRelayStates narrows the per-channel initial state enum of a supported Gen1 model to the
// Gen1 words. Another model, and a template with no model, keep every word.
func getRelayStates(schema map[string]interface{}, supported bool) map[string]interface{} {
	if !supported {
		return schema
	}
	props, _ := schema["properties"].(map[string]interface{})
	relays, _ := props["relays"].(map[string]interface{})
	items, _ := relays["items"].(map[string]interface{})
	itemProps, _ := items["properties"].(map[string]interface{})
	state, _ := itemProps["default_state"].(map[string]interface{})
	if state == nil {
		return schema
	}
	var allowed []string
	for _, name := range state["enum"].([]string) {
		if gen1InitialStates[name] {
			allowed = append(allowed, name)
		}
	}
	state["enum"] = allowed
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

// gen1DeviceModes returns the values of the device-level mode (system.device.mode) a Gen1 model
// accepts on POST /settings, and nil for a model that has no such switch. It is the one place that
// decides both whether a model has the setting and which words it takes.
func gen1DeviceModes(deviceType string) []string {
	switch deviceType {
	case "SHSW-25":
		return []string{"relay", "roller"}
	case "SHRGBW2":
		return []string{"color", "white"}
	}
	return nil
}

// withoutModeOfModelsWithoutIt drops the system.device.mode leaf from the schema of a supported
// model that has no mode switch, so the leaf is never marked applicable where apply refuses it.
// A model that has one keeps the leaf with its own words only (relay and roller, or color and white).
// An unsupported model keeps it (marked not applicable), and so does a template with no model.
func withoutModeOfModelsWithoutIt(sections map[string]interface{}, deviceType string, supported bool) {
	if !supported {
		return
	}
	system, _ := sections["system"].(map[string]interface{})
	device := schemaProperties(schemaProperties(system)["device"])
	if device == nil {
		return
	}
	modes := gen1DeviceModes(deviceType)
	if len(modes) == 0 {
		delete(device, "mode")
		return
	}
	if mode, ok := device["mode"].(map[string]interface{}); ok {
		mode["enum"] = modes
	}
}

// gen1HasWiFiRecovery reports whether a Gen1 model has the Wi-Fi recovery reboot flag
// (wifirecovery_reboot_enabled). The reference applies it to the Shelly 1/1PM, 2, 2.5 and
// Plug/PlugS, and not to the i3. It is the one place that decides it: the converter reads the flag
// only for such a model (so apply refuses it on another one through the round trip), and the schema
// drops the leaf for the others.
func gen1HasWiFiRecovery(deviceType string) bool {
	switch deviceType {
	case "SHSW-1", "SHSW-PM", "SHSW-25", "SHPLG-S", "SHPLG-1":
		return true
	}
	return false
}

// withoutRecoveryOfModelsWithoutIt drops the wifi.recovery_reboot leaf from the schema of a
// supported model without the flag, so it is never marked applicable where apply refuses it. An
// unsupported model keeps it (marked not applicable), and so does a template with no model.
func withoutRecoveryOfModelsWithoutIt(sections map[string]interface{}, deviceType string, supported bool) {
	if !supported || gen1HasWiFiRecovery(deviceType) {
		return
	}
	delete(schemaProperties(sections["wifi"]), "recovery_reboot")
}

// withoutPowerLEDOfModelsWithoutIt drops the led.power_indication leaf (led_power_disable) from the
// schema of a supported model that has only the status LED, so it is never marked applicable where
// apply refuses it. An unsupported model keeps it, and so does a template with no model.
func withoutPowerLEDOfModelsWithoutIt(led map[string]interface{}, hasPowerLED, supported bool) map[string]interface{} {
	if supported && !hasPowerLED {
		delete(schemaProperties(led), "power_indication")
	}
	return led
}

// withoutEdgeOfDimmerWithoutDetection drops the dimming.leading_edge leaf from the schema of the
// Dimmer 1, which never reports pulse_mode_detected, so apply always refuses the field there and it
// is never marked applicable. The Dimmer 2, an unsupported model and a template with no model keep it.
func withoutEdgeOfDimmerWithoutDetection(dimming map[string]interface{}, deviceType string) map[string]interface{} {
	if deviceType == "SHDM-1" {
		delete(schemaProperties(dimming), "leading_edge")
	}
	return dimming
}

// schemaProperties returns the properties map of a schema node, or nil.
func schemaProperties(node interface{}) map[string]interface{} {
	object, _ := node.(map[string]interface{})
	props, _ := object["properties"].(map[string]interface{})
	return props
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
	addr := func(title string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "title": title, "format": "ipv4"}
	}
	staticIP := func() map[string]interface{} {
		return obj("Static IP Settings", map[string]interface{}{
			"ip": addr("IP Address"), "netmask": addr("Netmask"), "gw": addr("Gateway"), "nameserver": addr("DNS Server"),
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
					"nameserver": addr("DNS Server"),
				}),
				"ap": obj("Access Point", map[string]interface{}{
					"enable": flag("Enable"), "ssid": str("Network Name (SSID)"), "pass": secret("Password"),
					"is_open":     flag("Open Network"),
					"max_sta":     map[string]interface{}{"type": "integer", "title": "Max Clients"},
					"range_start": addr("DHCP Range Start"), "range_end": addr("DHCP Range End"),
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
				"format":      "hostport",
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
	applicable := supported && gen1ApplicablePaths[path]
	node[SchemaKeyApplicable] = applicable
	// A default is a promise that the field does something, so a field apply never writes shows
	// none, and an applied field shows only the one the Gen1 reference states (gen1_defaults.go).
	delete(node, "default")
	if value, ok := gen1FieldDefaults[path]; ok && applicable {
		node["default"] = value
	}
}

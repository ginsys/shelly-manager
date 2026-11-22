package configuration

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Gen1Converter handles Gen1 device API conversion
type Gen1Converter struct {
	logger *logging.Logger
}

// NewGen1Converter creates a new Gen1 converter
func NewGen1Converter(logger *logging.Logger) *Gen1Converter {
	if logger == nil {
		logger = logging.GetDefault()
	}
	return &Gen1Converter{
		logger: logger,
	}
}

// FromAPIConfig converts Gen1 API JSON to DeviceConfiguration
func (c *Gen1Converter) FromAPIConfig(apiJSON json.RawMessage, deviceType string) (*DeviceConfiguration, error) {
	// Parse Gen1 API JSON into intermediate structure
	var gen1 map[string]interface{}
	if err := json.Unmarshal(apiJSON, &gen1); err != nil {
		return nil, fmt.Errorf("failed to parse Gen1 API JSON: %w", err)
	}

	config := &DeviceConfiguration{}

	// Convert WiFi settings
	if err := c.convertWiFi(gen1, config, deviceType); err != nil {
		return nil, fmt.Errorf("failed to convert WiFi: %w", err)
	}

	// Convert MQTT settings
	if err := c.convertMQTT(gen1, config); err != nil {
		return nil, fmt.Errorf("failed to convert MQTT: %w", err)
	}

	// Convert Auth settings
	if err := c.convertAuth(gen1, config); err != nil {
		return nil, fmt.Errorf("failed to convert Auth: %w", err)
	}

	// Convert Cloud settings
	if err := c.convertCloud(gen1, config); err != nil {
		return nil, fmt.Errorf("failed to convert Cloud: %w", err)
	}

	// Convert CoIoT settings
	if err := c.convertCoIoT(gen1, config); err != nil {
		return nil, fmt.Errorf("failed to convert CoIoT: %w", err)
	}

	// Convert System settings
	if err := c.convertSystem(gen1, config, deviceType); err != nil {
		return nil, fmt.Errorf("failed to convert System: %w", err)
	}

	// Convert Location settings
	if err := c.convertLocation(gen1, config); err != nil {
		return nil, fmt.Errorf("failed to convert Location: %w", err)
	}

	// Convert Relay settings (device-specific)
	if err := c.convertRelay(gen1, config, deviceType); err != nil {
		return nil, fmt.Errorf("failed to convert Relay: %w", err)
	}

	// Convert Light settings (a Dimmer's channels)
	c.convertLight(gen1, config, deviceType)

	// Convert Dimming settings (a Dimmer's device-level parameters, and the night mode of a Duo or Vintage)
	c.convertDimming(gen1, config, deviceType)

	// Convert PowerMetering settings (device-specific)
	if err := c.convertPowerMetering(gen1, config, deviceType); err != nil {
		return nil, fmt.Errorf("failed to convert PowerMetering: %w", err)
	}

	// Convert LED settings (device-specific)
	if err := c.convertLED(gen1, config, deviceType); err != nil {
		return nil, fmt.Errorf("failed to convert LED: %w", err)
	}

	// Convert Input settings
	if err := c.convertInput(gen1, config, deviceType); err != nil {
		return nil, fmt.Errorf("failed to convert Input: %w", err)
	}

	// Convert Roller settings (roller-mode models only)
	c.convertRoller(gen1, config, deviceType)

	return config, nil
}

// ToAPIConfig converts DeviceConfiguration to Gen1 API JSON
func (c *Gen1Converter) ToAPIConfig(config *DeviceConfiguration, deviceType string) (json.RawMessage, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	gen1 := make(map[string]interface{})

	// Convert WiFi settings
	c.exportWiFi(config, gen1)

	// Convert MQTT settings
	c.exportMQTT(config, gen1)

	// Convert Auth settings
	c.exportAuth(config, gen1)

	// Convert Cloud settings
	c.exportCloud(config, gen1)

	// Convert CoIoT settings
	c.exportCoIoT(config, gen1)

	// Convert System settings
	c.exportSystem(config, gen1)

	// Convert Location settings
	c.exportLocation(config, gen1)

	// Convert Relay settings
	c.exportRelay(config, gen1, deviceType)

	// Convert Light settings
	c.exportLight(config, gen1, deviceType)

	// Convert Dimming settings
	c.exportDimming(config, gen1, deviceType)

	// Convert PowerMetering settings
	c.exportPowerMetering(config, gen1, deviceType)

	// Convert LED settings
	c.exportLED(config, gen1, deviceType)

	// Convert Input settings
	c.exportInput(config, gen1, deviceType)

	// Convert Roller settings
	c.exportRoller(config, gen1, deviceType)

	// Marshal to JSON
	return json.Marshal(gen1)
}

// SupportedDeviceTypes returns list of supported Gen1 device types
func (c *Gen1Converter) SupportedDeviceTypes() []string {
	return []string{
		"SHPLG-S",  // Smart Plug with metering
		"SHPLG-1",  // Smart Plug
		"SHSW-1",   // Shelly 1
		"SHSW-PM",  // Shelly 1PM
		"SHSW-25",  // Shelly 2.5
		"SHIX3-1",  // Shelly i3
		"SHDM-1",   // Shelly Dimmer
		"SHDM-2",   // Shelly Dimmer 2 (read as the same channel as SHDM-1; unverified)
		"SHRGBW2",  // Shelly RGBW2 (color mode: one channel, white mode: four)
		"SHBLB-1",  // Shelly Bulb (one light channel; mode white or color)
		"SHBDUO-1", // Shelly Duo (one white channel with a name and a night mode)
		"SHVIN-1",  // Shelly Vintage (as the Duo)
	}
}

// Generation returns 1 for Gen1 devices
func (c *Gen1Converter) Generation() int {
	return 1
}

// Helper functions for FromAPIConfig

func (c *Gen1Converter) convertWiFi(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) error {
	wifiSTA, ok := gen1["wifi_sta"].(map[string]interface{})
	if !ok {
		return nil // WiFi settings not present
	}

	wifi := &WiFiConfiguration{}

	if enabled, ok := wifiSTA["enabled"].(bool); ok {
		wifi.Enable = BoolPtr(enabled)
	}
	if ssid, ok := wifiSTA["ssid"].(string); ok {
		wifi.SSID = StringPtr(ssid)
	}
	if key, ok := wifiSTA["key"].(string); ok {
		wifi.Password = StringPtr(key)
	}
	if ipv4Method, ok := wifiSTA["ipv4_method"].(string); ok {
		wifi.IPv4Mode = StringPtr(ipv4Method)
	}

	// Static IP configuration: present as soon as the device reports any of its four addresses, so
	// a desired gateway alone survives the round trip that decides what apply can write.
	staticIP := &StaticIPConfig{}
	if ip, ok := wifiSTA["ip"].(string); ok && ip != "" {
		staticIP.IP = StringPtr(ip)
	}
	// The device reports the netmask as "mask"; "netmask" is the older invented key and is still
	// read, but never wins over the real one.
	if netmask, ok := wifiSTA["mask"].(string); ok {
		staticIP.Netmask = StringPtr(netmask)
	} else if netmask, ok := wifiSTA["netmask"].(string); ok {
		staticIP.Netmask = StringPtr(netmask)
	}
	if gw, ok := wifiSTA["gw"].(string); ok {
		staticIP.Gateway = StringPtr(gw)
	}
	if dns, ok := wifiSTA["dns"].(string); ok {
		staticIP.Nameserver = StringPtr(dns)
	}
	if staticIP.IP != nil || staticIP.Netmask != nil || staticIP.Gateway != nil || staticIP.Nameserver != nil {
		wifi.StaticIP = staticIP
	}

	// Access Point configuration
	if wifiAP, ok := gen1["wifi_ap"].(map[string]interface{}); ok {
		ap := &AccessPointConfig{}
		if enabled, ok := wifiAP["enabled"].(bool); ok {
			ap.Enable = BoolPtr(enabled)
		}
		if ssid, ok := wifiAP["ssid"].(string); ok {
			ap.SSID = StringPtr(ssid)
		}
		if key, ok := wifiAP["key"].(string); ok {
			ap.Password = StringPtr(key)
		}
		wifi.AccessPoint = ap
	}

	// Roaming is the ap_roaming object of GET /settings. The recovery flag is flat, and only a
	// model that has it keeps it: a desired flag on another model fails the round trip.
	if roaming, ok := gen1["ap_roaming"].(map[string]interface{}); ok {
		if enabled, ok := roaming["enabled"].(bool); ok {
			wifi.RoamEnable = BoolPtr(enabled)
		}
		if threshold, ok := roaming["threshold"].(float64); ok && threshold == math.Trunc(threshold) {
			wifi.RoamThreshold = IntPtr(int(threshold))
		}
	}
	if recovery, ok := gen1["wifirecovery_reboot_enabled"].(bool); ok && gen1HasWiFiRecovery(deviceType) {
		wifi.RecoveryReboot = BoolPtr(recovery)
	}

	config.WiFi = wifi
	return nil
}

func (c *Gen1Converter) convertMQTT(gen1 map[string]interface{}, config *DeviceConfiguration) error {
	mqttData, ok := gen1["mqtt"].(map[string]interface{})
	if !ok {
		return nil
	}

	mqtt := &MQTTConfiguration{}

	if enable, ok := mqttData["enable"].(bool); ok {
		mqtt.Enable = BoolPtr(enable)
	}

	// Split server "host:port" into separate fields
	if server, ok := mqttData["server"].(string); ok && server != "" {
		parts := strings.Split(server, ":")
		if len(parts) >= 1 {
			mqtt.Server = StringPtr(parts[0])
		}
		if len(parts) == 2 {
			var port int
			if _, err := fmt.Sscanf(parts[1], "%d", &port); err == nil {
				mqtt.Port = IntPtr(port)
			}
		}
	}

	if user, ok := mqttData["user"].(string); ok {
		mqtt.User = StringPtr(user)
	}
	if pass, ok := mqttData["pass"].(string); ok {
		mqtt.Password = StringPtr(pass)
	}
	if id, ok := mqttData["id"].(string); ok {
		mqtt.ClientID = StringPtr(id)
	}
	if cleanSession, ok := mqttData["clean_session"].(bool); ok {
		mqtt.CleanSession = BoolPtr(cleanSession)
	}
	if keepAlive, ok := shelly.WholeNumber(mqttData["keep_alive"]); ok {
		mqtt.KeepAlive = IntPtr(keepAlive)
	}
	if updatePeriod, ok := shelly.WholeNumber(mqttData["update_period"]); ok {
		mqtt.UpdatePeriod = IntPtr(updatePeriod)
	}

	config.MQTT = mqtt
	return nil
}

func (c *Gen1Converter) convertAuth(gen1 map[string]interface{}, config *DeviceConfiguration) error {
	loginData, ok := gen1["login"].(map[string]interface{})
	if !ok {
		return nil
	}

	auth := &AuthConfiguration{}

	if enabled, ok := loginData["enabled"].(bool); ok {
		auth.Enable = BoolPtr(enabled)
	}
	if username, ok := loginData["username"].(string); ok {
		auth.Username = StringPtr(username)
	}
	if password, ok := loginData["password"].(string); ok {
		auth.Password = StringPtr(password)
	}

	config.Auth = auth
	return nil
}

func (c *Gen1Converter) convertCloud(gen1 map[string]interface{}, config *DeviceConfiguration) error {
	cloudData, ok := gen1["cloud"].(map[string]interface{})
	if !ok {
		return nil
	}

	cloud := &CloudConfiguration{}

	if enabled, ok := cloudData["enabled"].(bool); ok {
		cloud.Enable = BoolPtr(enabled)
	}
	if server, ok := cloudData["server"].(string); ok {
		cloud.Server = StringPtr(server)
	}

	config.Cloud = cloud
	return nil
}

func (c *Gen1Converter) convertCoIoT(gen1 map[string]interface{}, config *DeviceConfiguration) error {
	coiotData, ok := gen1["coiot"].(map[string]interface{})
	if !ok {
		return nil
	}

	coiot := &CoIoTConfiguration{}

	if enabled, ok := coiotData["enabled"].(bool); ok {
		coiot.Enable = BoolPtr(enabled)
	}
	if updatePeriod, ok := shelly.WholeNumber(coiotData["update_period"]); ok {
		coiot.UpdatePeriod = IntPtr(updatePeriod)
	}
	if peer, ok := coiotData["peer"].(string); ok {
		coiot.Peer = StringPtr(peer)
	}

	config.CoIoT = coiot
	return nil
}

func (c *Gen1Converter) convertSystem(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) error {
	// System settings are scattered in Gen1 API (name, eco_mode_enabled, discoverable)
	system := &SystemConfiguration{}
	device := &TypedDeviceConfig{}

	if name, ok := gen1["name"].(string); ok {
		device.Name = StringPtr(name)
	}
	if ecoMode, ok := gen1["eco_mode_enabled"].(bool); ok {
		device.EcoMode = BoolPtr(ecoMode)
	}
	if discoverable, ok := gen1["discoverable"].(bool); ok {
		device.Discoverable = BoolPtr(discoverable)
	}

	// Only a model that has the switch reports a mode worth keeping (gen1DeviceModes lists them).
	if mode, ok := gen1["mode"].(string); ok && slices.Contains(gen1DeviceModes(deviceType), mode) {
		device.Mode = StringPtr(mode)
	}

	if device.Name != nil || device.EcoMode != nil || device.Discoverable != nil || device.Mode != nil {
		system.Device = device
		config.System = system
	}

	// The device reports its time server as sntp.server. An empty server means no time server
	// and cannot be told from an unset one in SNTPConfig, so it is left out.
	if sntp, ok := gen1["sntp"].(map[string]interface{}); ok {
		if server, ok := sntp["server"].(string); ok && server != "" {
			system.SNTP = &SNTPConfig{Server: server}
			config.System = system
		}
	}

	return nil
}

func (c *Gen1Converter) convertLocation(gen1 map[string]interface{}, config *DeviceConfiguration) error {
	location := &LocationConfiguration{}
	hasData := false

	if timezone, ok := gen1["timezone"].(string); ok {
		location.Timezone = StringPtr(timezone)
		hasData = true
	}
	if lat, ok := gen1["lat"].(float64); ok {
		location.Latitude = &lat
		hasData = true
	}
	if lng, ok := gen1["lng"].(float64); ok {
		location.Longitude = &lng
		hasData = true
	}

	if hasData {
		config.Location = location
	}

	return nil
}

func (c *Gen1Converter) convertRelay(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) error {
	relaysData, ok := gen1["relays"].([]interface{})
	if !ok || len(relaysData) == 0 {
		return nil
	}

	relay := &RelayConfig{
		Relays: make([]SingleRelayConfig, 0, len(relaysData)),
	}

	for i, relayItem := range relaysData {
		relayMap, ok := relayItem.(map[string]interface{})
		if !ok {
			continue
		}

		singleRelay := SingleRelayConfig{
			ID: i,
		}

		if name, ok := relayMap["name"].(string); ok {
			singleRelay.Name = StringPtr(name)
		}
		if defaultState, ok := relayMap["default_state"].(string); ok {
			singleRelay.DefaultState = StringPtr(defaultState)
		}
		if autoOn, ok := shelly.WholeNumber(relayMap["auto_on"]); ok {
			singleRelay.AutoOn = IntPtr(autoOn)
		}
		if autoOff, ok := shelly.WholeNumber(relayMap["auto_off"]); ok {
			singleRelay.AutoOff = IntPtr(autoOff)
		}
		if applianceType, ok := relayMap["appliance_type"].(string); ok {
			singleRelay.ApplianceType = StringPtr(applianceType)
		}
		if schedule, ok := relayMap["schedule"].(bool); ok {
			singleRelay.Schedule = BoolPtr(schedule)
		}
		if rules, ok := scheduleRulesOf(relayMap["schedule_rules"]); ok {
			singleRelay.ScheduleRules = rules
		}
		if maxPower, ok := shelly.WholeNumber(relayMap["max_power"]); ok {
			singleRelay.MaxPower = IntPtr(maxPower)
		}

		relay.Relays = append(relay.Relays, singleRelay)
	}

	if len(relay.Relays) > 0 {
		config.Relay = relay
	}

	return nil
}

// scheduleRulesOf reads a channel's schedule_rules: the list of rule strings, empty when the device
// keeps none. It is false when the value is no list.
func scheduleRulesOf(raw interface{}) (*[]string, bool) {
	rules, ok := raw.([]interface{})
	if !ok {
		return nil, false
	}
	list := []string{}
	for _, rule := range rules {
		if text, isString := rule.(string); isString {
			list = append(list, text)
		}
	}
	return &list, true
}

// convertLight reads the lights array of a model whose channels are lights. The name is read where
// hasLightName says the page lists it as a parameter.
func (c *Gen1Converter) convertLight(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) {
	if !c.hasLight(deviceType) {
		return
	}
	lights := relayMaps(gen1["lights"])
	if len(lights) == 0 {
		return
	}

	light := &LightConfig{Lights: make([]SingleLightConfig, 0, len(lights))}
	for i, lightMap := range lights {
		single := SingleLightConfig{ID: i}
		if hasLightEffect(deviceType) {
			if effect, ok := shelly.WholeNumber(lightMap["effect"]); ok {
				single.Effect = IntPtr(effect)
			}
		}
		if hasLightName(deviceType) {
			if name, ok := lightMap["name"].(string); ok {
				single.Name = StringPtr(name)
			}
		}
		if hasColorWhiteChannels(deviceType) {
			if transition, ok := shelly.WholeNumber(lightMap["transition"]); ok {
				single.Transition = IntPtr(transition)
			}
			if reverse, ok := reverseFlag(lightMap["btn_reverse"]); ok {
				single.ButtonReverse = BoolPtr(reverse)
			}
		}
		if defaultState, ok := lightMap["default_state"].(string); ok {
			single.DefaultState = StringPtr(defaultState)
		}
		if autoOn, ok := shelly.WholeNumber(lightMap["auto_on"]); ok {
			single.AutoOn = IntPtr(autoOn)
		}
		if autoOff, ok := shelly.WholeNumber(lightMap["auto_off"]); ok {
			single.AutoOff = IntPtr(autoOff)
		}
		if schedule, ok := lightMap["schedule"].(bool); ok {
			single.Schedule = BoolPtr(schedule)
		}
		if rules, ok := scheduleRulesOf(lightMap["schedule_rules"]); ok {
			single.ScheduleRules = rules
		}
		if buttonType, ok := lightMap["btn_type"].(string); ok {
			single.ButtonType = StringPtr(buttonType)
		}
		if debounce, ok := shelly.WholeNumber(lightMap["btn_debounce"]); ok {
			single.ButtonDebounce = IntPtr(debounce)
		}
		// The page's example shows swap_inputs as 0; a device may report a number or a boolean.
		if swap, ok := reverseFlag(lightMap["swap_inputs"]); ok {
			single.SwapInputs = BoolPtr(swap)
		}
		light.Lights = append(light.Lights, single)
	}
	config.Light = light
}

// convertDimming reads the night mode of a Dimmer, a Duo or a Vintage, and a Dimmer's other
// device-level settings, the flat parameters and the warm-up of GET /settings.
// pulse_mode 1 and 2 are the stored edge; any other value (the page's example shows 0 before the mode
// is detected) is no value at all.
func (c *Gen1Converter) convertDimming(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) {
	if !c.hasNightMode(deviceType) {
		return
	}
	dimming := &DimmingConfig{}
	found := false
	if n, ok := shelly.WholeNumber(gen1["transition"]); ok && c.hasTransition(deviceType) {
		dimming.TransitionTime = IntPtr(n)
		found = true
	}
	if c.hasDimmerSettings(deviceType) {
		for key, dst := range map[string]**int{
			"min_brightness": &dimming.MinBrightness, "fade_rate": &dimming.FadeRate, "zcross_debounce": &dimming.ZcrossDebounce,
		} {
			// The page's example shows fade_rate 0 beside the range 1..5: a value outside it is no value.
			if n, ok := shelly.WholeNumber(gen1[key]); ok && (key != "fade_rate" || (n >= gen1DimmerMinFadeRate && n <= gen1DimmerMaxFadeRate)) {
				*dst = IntPtr(n)
				found = true
			}
		}
		if mode, ok := shelly.WholeNumber(gen1["pulse_mode"]); ok && (mode == gen1DimmerLeadingEdge || mode == gen1DimmerTrailingEdge) {
			dimming.LeadingEdge = BoolPtr(mode == gen1DimmerLeadingEdge)
			found = true
		}
		warm, _ := gen1["warm_up"].(map[string]interface{})
		found = readDimmerObject(warm, map[string]interface{}{
			"enabled": &dimming.WarmupEnabled, "brightness": &dimming.WarmupBrightness, "time": &dimming.WarmupTime,
		}) || found
	}
	// The top-level object night_mode; the copy of it inside a light is not read. A value is stored
	// as the device reports it, in range or not.
	night, _ := gen1["night_mode"].(map[string]interface{})
	found = readDimmerObject(night, map[string]interface{}{
		"enabled": &dimming.NightModeEnabled, "start_time": &dimming.NightModeStart,
		"end_time": &dimming.NightModeEnd, "brightness": &dimming.NightModeBrightness,
	}) || found
	if found {
		config.Dimming = dimming
	}
}

// readDimmerObject copies each key of a /settings object that has the stored field's type into it,
// and reports whether any did. A flag may be 0/1: the page types warm-up enabled as bool and shows 0.
func readDimmerObject(object map[string]interface{}, fields map[string]interface{}) bool {
	read := false
	for key, dst := range fields {
		switch field := dst.(type) {
		case **bool:
			if flag, ok := reverseFlag(object[key]); ok {
				*field, read = BoolPtr(flag), true
			}
		case **int:
			if n, ok := shelly.WholeNumber(object[key]); ok {
				*field, read = IntPtr(n), true
			}
		case **string:
			if s, ok := object[key].(string); ok {
				*field, read = StringPtr(s), true
			}
		}
	}
	return read
}

func (c *Gen1Converter) convertPowerMetering(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) error {
	if !c.hasPowerMetering(deviceType) {
		return nil
	}

	if maxPower, ok := shelly.WholeNumber(gen1["max_power"]); ok {
		config.PowerMetering = &PowerMeteringConfig{
			MaxPower: IntPtr(maxPower),
		}
	}

	// A Shelly 2.5 in relay mode keeps no model-wide threshold: POST /settings max_power is "for
	// roller mode only". The model-wide value is the one every channel carries, if they agree.
	if gen1MaxPowerIsPerChannel(deviceType, gen1) {
		config.PowerMetering = nil
		if maxPower, agreed := commonChannelMaxPower(gen1); agreed {
			config.PowerMetering = &PowerMeteringConfig{MaxPower: IntPtr(maxPower)}
		}
	}

	return nil
}

// gen1MaxPowerIsPerChannel reports whether a device keeps its overpower threshold per channel: a
// Shelly 2.5 whose reported mode is relay.
func gen1MaxPowerIsPerChannel(deviceType string, gen1 map[string]interface{}) bool {
	mode, _ := gen1["mode"].(string)
	return deviceType == "SHSW-25" && mode == "relay"
}

// commonChannelMaxPower is the max_power that every relay channel reports, and false when a channel
// reports none or the channels differ.
func commonChannelMaxPower(gen1 map[string]interface{}) (int, bool) {
	relays := relayMaps(gen1["relays"])
	if len(relays) == 0 {
		return 0, false
	}
	common := 0
	for i, relay := range relays {
		value, ok := shelly.WholeNumber(relay["max_power"])
		if !ok || (i > 0 && value != common) {
			return 0, false
		}
		common = value
	}
	return common, true
}

func (c *Gen1Converter) convertLED(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) error {
	if !c.hasStatusLED(deviceType) {
		return nil
	}

	led := &LEDConfig{}
	hasData := false

	if ledPowerDisable, ok := gen1["led_power_disable"].(bool); ok && c.hasLED(deviceType) {
		led.PowerIndication = BoolPtr(!ledPowerDisable)
		hasData = true
	}
	if ledStatusDisable, ok := gen1["led_status_disable"].(bool); ok {
		led.NetworkIndication = BoolPtr(!ledStatusDisable)
		hasData = true
	}

	if hasData {
		config.LED = led
	}

	return nil
}

// Helper functions for ToAPIConfig

func (c *Gen1Converter) exportWiFi(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.WiFi == nil {
		return
	}

	wifiSTA := make(map[string]interface{})

	if config.WiFi.Enable != nil {
		wifiSTA["enabled"] = *config.WiFi.Enable
	}
	if config.WiFi.SSID != nil {
		wifiSTA["ssid"] = *config.WiFi.SSID
	}
	if config.WiFi.Password != nil {
		wifiSTA["key"] = *config.WiFi.Password
	}
	if config.WiFi.IPv4Mode != nil {
		wifiSTA["ipv4_method"] = *config.WiFi.IPv4Mode
	}

	if config.WiFi.StaticIP != nil {
		if config.WiFi.StaticIP.IP != nil {
			wifiSTA["ip"] = *config.WiFi.StaticIP.IP
		}
		if config.WiFi.StaticIP.Netmask != nil {
			wifiSTA["mask"] = *config.WiFi.StaticIP.Netmask
		}
		if config.WiFi.StaticIP.Gateway != nil {
			wifiSTA["gw"] = *config.WiFi.StaticIP.Gateway
		}
		if config.WiFi.StaticIP.Nameserver != nil {
			wifiSTA["dns"] = *config.WiFi.StaticIP.Nameserver
		}
	}

	gen1["wifi_sta"] = wifiSTA

	// Roaming and the recovery flag, in the shape GET /settings reports them.
	if config.WiFi.RoamEnable != nil || config.WiFi.RoamThreshold != nil {
		roaming := make(map[string]interface{})
		if config.WiFi.RoamEnable != nil {
			roaming["enabled"] = *config.WiFi.RoamEnable
		}
		if config.WiFi.RoamThreshold != nil {
			roaming["threshold"] = *config.WiFi.RoamThreshold
		}
		gen1["ap_roaming"] = roaming
	}
	if config.WiFi.RecoveryReboot != nil {
		gen1["wifirecovery_reboot_enabled"] = *config.WiFi.RecoveryReboot
	}

	// Access Point
	if config.WiFi.AccessPoint != nil {
		wifiAP := make(map[string]interface{})
		if config.WiFi.AccessPoint.Enable != nil {
			wifiAP["enabled"] = *config.WiFi.AccessPoint.Enable
		}
		if config.WiFi.AccessPoint.SSID != nil {
			wifiAP["ssid"] = *config.WiFi.AccessPoint.SSID
		}
		if config.WiFi.AccessPoint.Password != nil {
			wifiAP["key"] = *config.WiFi.AccessPoint.Password
		}
		gen1["wifi_ap"] = wifiAP
	}
}

func (c *Gen1Converter) exportMQTT(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.MQTT == nil {
		return
	}

	mqtt := make(map[string]interface{})

	if config.MQTT.Enable != nil {
		mqtt["enable"] = *config.MQTT.Enable
	}

	// Combine server + port into "host:port"
	if config.MQTT.Server != nil {
		server := *config.MQTT.Server
		if config.MQTT.Port != nil {
			server = fmt.Sprintf("%s:%d", server, *config.MQTT.Port)
		}
		mqtt["server"] = server
	}

	if config.MQTT.User != nil {
		mqtt["user"] = *config.MQTT.User
	}
	if config.MQTT.Password != nil {
		mqtt["pass"] = *config.MQTT.Password
	}
	if config.MQTT.ClientID != nil {
		mqtt["id"] = *config.MQTT.ClientID
	}
	if config.MQTT.CleanSession != nil {
		mqtt["clean_session"] = *config.MQTT.CleanSession
	}
	if config.MQTT.KeepAlive != nil {
		mqtt["keep_alive"] = *config.MQTT.KeepAlive
	}
	if config.MQTT.UpdatePeriod != nil {
		mqtt["update_period"] = *config.MQTT.UpdatePeriod
	}

	gen1["mqtt"] = mqtt
}

func (c *Gen1Converter) exportAuth(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.Auth == nil {
		return
	}

	login := make(map[string]interface{})

	if config.Auth.Enable != nil {
		login["enabled"] = *config.Auth.Enable
	}
	if config.Auth.Username != nil {
		login["username"] = *config.Auth.Username
	}
	if config.Auth.Password != nil {
		login["password"] = *config.Auth.Password
	}

	gen1["login"] = login
}

func (c *Gen1Converter) exportCloud(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.Cloud == nil {
		return
	}

	cloud := make(map[string]interface{})

	if config.Cloud.Enable != nil {
		cloud["enabled"] = *config.Cloud.Enable
	}
	if config.Cloud.Server != nil {
		cloud["server"] = *config.Cloud.Server
	}

	gen1["cloud"] = cloud
}

func (c *Gen1Converter) exportCoIoT(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.CoIoT == nil {
		return
	}

	coiot := make(map[string]interface{})

	if config.CoIoT.Enable != nil {
		coiot["enabled"] = *config.CoIoT.Enable
	}
	if config.CoIoT.UpdatePeriod != nil {
		coiot["update_period"] = *config.CoIoT.UpdatePeriod
	}
	if config.CoIoT.Peer != nil {
		coiot["peer"] = *config.CoIoT.Peer
	}

	gen1["coiot"] = coiot
}

func (c *Gen1Converter) exportSystem(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.System == nil {
		return
	}

	if device := config.System.Device; device != nil {
		if device.Name != nil {
			gen1["name"] = *device.Name
		}
		if device.EcoMode != nil {
			gen1["eco_mode_enabled"] = *device.EcoMode
		}
		if device.Discoverable != nil {
			gen1["discoverable"] = *device.Discoverable
		}
		if device.Mode != nil {
			gen1["mode"] = *device.Mode
		}
	}
	if sntp := config.System.SNTP; sntp != nil && sntp.Server != "" {
		gen1["sntp"] = map[string]interface{}{"server": sntp.Server}
	}
}

func (c *Gen1Converter) exportLocation(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.Location == nil {
		return
	}

	if config.Location.Timezone != nil {
		gen1["timezone"] = *config.Location.Timezone
	}
	if config.Location.Latitude != nil {
		gen1["lat"] = *config.Location.Latitude
	}
	if config.Location.Longitude != nil {
		gen1["lng"] = *config.Location.Longitude
	}
}

func (c *Gen1Converter) exportRelay(config *DeviceConfiguration, gen1 map[string]interface{}, deviceType string) {
	if !c.hasRelay(deviceType) {
		return
	}

	if config.Relay == nil || len(config.Relay.Relays) == 0 {
		return
	}

	relays := make([]map[string]interface{}, len(config.Relay.Relays))

	for i, relay := range config.Relay.Relays {
		relayMap := make(map[string]interface{})

		if relay.Name != nil {
			relayMap["name"] = *relay.Name
		}
		if relay.DefaultState != nil {
			relayMap["default_state"] = *relay.DefaultState
		}
		if relay.AutoOn != nil {
			relayMap["auto_on"] = *relay.AutoOn
		}
		if relay.AutoOff != nil {
			relayMap["auto_off"] = *relay.AutoOff
		}
		if relay.ApplianceType != nil {
			relayMap["appliance_type"] = *relay.ApplianceType
		}
		if relay.Schedule != nil {
			relayMap["schedule"] = *relay.Schedule
		}
		if relay.ScheduleRules != nil {
			relayMap["schedule_rules"] = *relay.ScheduleRules
		}
		if relay.MaxPower != nil {
			relayMap["max_power"] = *relay.MaxPower
		}

		relays[i] = relayMap
	}

	gen1["relays"] = relays
}

func (c *Gen1Converter) exportLight(config *DeviceConfiguration, gen1 map[string]interface{}, deviceType string) {
	if !c.hasLight(deviceType) || config.Light == nil || len(config.Light.Lights) == 0 {
		return
	}

	// name, effect, transition and btn_reverse are exported for any light model and read back for the
	// RGBW2 only: that asymmetry is what refuses them on a Dimmer through the round trip.
	lights := make([]map[string]interface{}, len(config.Light.Lights))
	for i, light := range config.Light.Lights {
		lightMap := make(map[string]interface{})
		if light.Name != nil {
			lightMap["name"] = *light.Name
		}
		if light.Effect != nil {
			lightMap["effect"] = *light.Effect
		}
		if light.Transition != nil {
			lightMap["transition"] = *light.Transition
		}
		if light.ButtonReverse != nil {
			lightMap["btn_reverse"] = *light.ButtonReverse
		}
		if light.DefaultState != nil {
			lightMap["default_state"] = *light.DefaultState
		}
		if light.AutoOn != nil {
			lightMap["auto_on"] = *light.AutoOn
		}
		if light.AutoOff != nil {
			lightMap["auto_off"] = *light.AutoOff
		}
		if light.Schedule != nil {
			lightMap["schedule"] = *light.Schedule
		}
		if light.ScheduleRules != nil {
			lightMap["schedule_rules"] = *light.ScheduleRules
		}
		if light.ButtonType != nil {
			lightMap["btn_type"] = *light.ButtonType
		}
		if light.ButtonDebounce != nil {
			lightMap["btn_debounce"] = *light.ButtonDebounce
		}
		if light.SwapInputs != nil {
			swap := 0
			if *light.SwapInputs {
				swap = 1
			}
			lightMap["swap_inputs"] = swap
		}
		lights[i] = lightMap
	}
	gen1["lights"] = lights
}

// exportDimming writes the night mode of a Dimmer, a Duo or a Vintage, and a Dimmer's other
// device-level settings as flat parameters and the warm-up. A field with no Gen1 parameter is left
// out, so the round trip reports it.
func (c *Gen1Converter) exportDimming(config *DeviceConfiguration, gen1 map[string]interface{}, deviceType string) {
	if !c.hasNightMode(deviceType) || config.Dimming == nil {
		return
	}
	d := config.Dimming
	if night := dimmerObject(map[string]interface{}{
		"enabled": d.NightModeEnabled, "start_time": d.NightModeStart, "end_time": d.NightModeEnd,
		"brightness": d.NightModeBrightness,
	}); len(night) > 0 {
		gen1["night_mode"] = night
	}
	if c.hasTransition(deviceType) && d.TransitionTime != nil {
		gen1["transition"] = *d.TransitionTime
	}
	if !c.hasDimmerSettings(deviceType) {
		return
	}
	for key, value := range map[string]*int{
		"min_brightness": d.MinBrightness, "fade_rate": d.FadeRate, "zcross_debounce": d.ZcrossDebounce,
	} {
		if value != nil {
			gen1[key] = *value
		}
	}
	if d.LeadingEdge != nil {
		gen1["pulse_mode"] = gen1DimmerTrailingEdge
		if *d.LeadingEdge {
			gen1["pulse_mode"] = gen1DimmerLeadingEdge
		}
	}
	if warm := dimmerObject(map[string]interface{}{
		"enabled": d.WarmupEnabled, "brightness": d.WarmupBrightness, "time": d.WarmupTime,
	}); len(warm) > 0 {
		gen1["warm_up"] = warm
	}
}

// dimmerObject keeps the fields that hold a value, dereferenced.
func dimmerObject(fields map[string]interface{}) map[string]interface{} {
	object := map[string]interface{}{}
	for key, field := range fields {
		switch v := field.(type) {
		case *bool:
			if v != nil {
				object[key] = *v
			}
		case *int:
			if v != nil {
				object[key] = *v
			}
		case *string:
			if v != nil {
				object[key] = *v
			}
		}
	}
	return object
}

func (c *Gen1Converter) exportPowerMetering(config *DeviceConfiguration, gen1 map[string]interface{}, deviceType string) {
	if !c.hasPowerMetering(deviceType) {
		return
	}

	if config.PowerMetering == nil {
		return
	}

	if config.PowerMetering.MaxPower != nil {
		gen1["max_power"] = *config.PowerMetering.MaxPower
	}
}

func (c *Gen1Converter) exportLED(config *DeviceConfiguration, gen1 map[string]interface{}, deviceType string) {
	if !c.hasStatusLED(deviceType) {
		return
	}

	if config.LED == nil {
		return
	}

	// Inverted: enabled=false means disable=true. A Dimmer has no power LED, so a desired one is not
	// exported and the round trip refuses it.
	if config.LED.PowerIndication != nil && c.hasLED(deviceType) {
		gen1["led_power_disable"] = !*config.LED.PowerIndication
	}
	if config.LED.NetworkIndication != nil {
		gen1["led_status_disable"] = !*config.LED.NetworkIndication
	}
}

// Input conversion methods

// gen1ButtonTypes maps a stored input mode to the btn_type of a Gen1 device. The mapping is read
// from the reference and is not yet checked on hardware: Gen1 documents no names for "follow" and
// "flip", so toggle and edge are the closest button types.
var gen1ButtonTypes = map[string]string{
	"momentary": "momentary",
	"follow":    "toggle",
	"flip":      "edge",
	"detached":  "detached",
}

// gen1ButtonTypesOfI3 are the stored modes a Shelly i3 accepts: its inputs are toggle or
// momentary only.
var gen1ButtonTypesOfI3 = map[string]bool{"momentary": true, "follow": true}

// inputModeOfButtonType is the stored mode of a device's btn_type. A button type with no stored
// name (momentary_on_release, action, ...) is kept as the device spells it.
func inputModeOfButtonType(buttonType string) string {
	for mode, name := range gen1ButtonTypes {
		if name == buttonType {
			return mode
		}
	}
	return buttonType
}

// buttonTypeOfInputMode is the btn_type to send for a stored mode. A mode with no mapping is sent
// as it is: unsupportedApplySettings refuses it before it reaches a device.
func buttonTypeOfInputMode(mode string) string {
	if name, ok := gen1ButtonTypes[mode]; ok {
		return name
	}
	return mode
}

// inputsLiveInRelays reports whether a model keeps its button type and reverse flag in the relay
// channels (Shelly 1, 1PM, 2.5, written with POST /settings/relay/<n>). The Shelly i3 has no relay:
// its inputs have their own array and route.
func inputsLiveInRelays(deviceType string) bool {
	switch deviceType {
	case "SHSW-1", "SHSW-PM", "SHSW-25":
		return true
	default:
		return false
	}
}

// reverseFlag reads btn_reverse, which a device reports as 0 or 1 (the reference also shows a bool).
func reverseFlag(raw interface{}) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case float64:
		return v != 0, true
	}
	return false, false
}

// relayInputs lists the inputs a relay model reports in its relays, one per relay with a button
// setting, and nil when none has one.
func relayInputs(gen1 map[string]interface{}) []interface{} {
	relays := relayMaps(gen1["relays"])
	inputs := make([]interface{}, len(relays))
	found := false
	for i, relay := range relays {
		input := map[string]interface{}{}
		for _, key := range gen1InputKeys {
			if value, ok := relay[key]; ok {
				input[key] = value
				found = true
			}
		}
		inputs[i] = input
	}
	if !found {
		return nil
	}
	return inputs
}

func (c *Gen1Converter) convertInput(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) error {
	inputsData, ok := gen1["inputs"].([]interface{})
	// The relays of a 2.5 report a button type in either mode, but in roller mode the inputs are the
	// roller's: reading none keeps an import from being refused when it is applied again.
	rollerMode := HasRollerMode(deviceType) && gen1["mode"] == "roller"
	if (!ok || len(inputsData) == 0) && inputsLiveInRelays(deviceType) && !rollerMode {
		inputsData = relayInputs(gen1)
	}
	if len(inputsData) == 0 {
		return nil
	}

	input := &InputConfig{
		Inputs: make([]SingleInputConfig, 0, len(inputsData)),
	}

	for i, inputItem := range inputsData {
		inputMap, ok := inputItem.(map[string]interface{})
		if !ok {
			continue
		}

		singleInput := SingleInputConfig{
			ID: i,
		}

		if name, ok := inputMap["name"].(string); ok {
			singleInput.Name = StringPtr(name)
		}
		if inputType, ok := inputMap["type"].(string); ok {
			singleInput.Type = StringPtr(inputType)
		}
		if mode, ok := inputMap["mode"].(string); ok {
			singleInput.Mode = StringPtr(mode)
		}
		if inverted, ok := inputMap["inverted"].(bool); ok {
			singleInput.Inverted = BoolPtr(inverted)
		}
		// The keys a real device reports win over the older, invented ones above.
		if buttonType, ok := inputMap["btn_type"].(string); ok {
			singleInput.Mode = StringPtr(inputModeOfButtonType(buttonType))
		}
		if reverse, ok := reverseFlag(inputMap["btn_reverse"]); ok {
			singleInput.Inverted = BoolPtr(reverse)
		}
		if action, ok := inputMap["single_push_action"].(string); ok {
			singleInput.SinglePushAction = StringPtr(action)
		}
		if action, ok := inputMap["long_push_action"].(string); ok {
			singleInput.LongPushAction = StringPtr(action)
		}

		input.Inputs = append(input.Inputs, singleInput)
	}

	if len(input.Inputs) > 0 {
		config.Input = input
	}

	return nil
}

func (c *Gen1Converter) exportInput(config *DeviceConfiguration, gen1 map[string]interface{}, deviceType string) {
	if config.Input == nil || len(config.Input.Inputs) == 0 {
		return
	}

	inputs := make([]map[string]interface{}, len(config.Input.Inputs))

	for i, input := range config.Input.Inputs {
		// Only the keys a Gen1 device has: the button type and the reverse flag. The stored name,
		// type and push actions have no Gen1 setting here, so they do not survive the export and
		// unsupportedApplySettings reports them.
		inputMap := make(map[string]interface{})

		if input.Mode != nil {
			inputMap["btn_type"] = buttonTypeOfInputMode(*input.Mode)
		}
		if input.Inverted != nil {
			reverse := 0
			if *input.Inverted {
				reverse = 1
			}
			inputMap["btn_reverse"] = reverse
		}

		inputs[i] = inputMap
	}

	gen1["inputs"] = inputs
}

// Roller conversion. The stored form names a value one way and Gen1 another: motor_direction is
// "normal" or "reversed" where Gen1 has the boolean swap, and input_mode is "single" or "dual"
// where Gen1 has "onebutton" or "openclose". The times are seconds on both sides.

const (
	rollerMotorNormal   = "normal"
	rollerMotorReversed = "reversed"
	rollerInputSingle   = "single"
	rollerInputDual     = "dual"
	gen1RollerOneButton = "onebutton"
	gen1RollerOpenClose = "openclose"
)

func (c *Gen1Converter) convertRoller(gen1 map[string]interface{}, config *DeviceConfiguration, deviceType string) {
	if !HasRollerMode(deviceType) {
		return
	}

	rollersData, ok := gen1["rollers"].([]interface{})
	if !ok || len(rollersData) == 0 {
		return
	}
	rollerMap, ok := rollersData[0].(map[string]interface{})
	if !ok {
		return
	}

	roller := &RollerConfig{}
	hasData := false

	if roller.MaxOpenTime = rollerNumber(rollerMap["maxtime_open"]); roller.MaxOpenTime != nil {
		hasData = true
	}
	if roller.MaxCloseTime = rollerNumber(rollerMap["maxtime_close"]); roller.MaxCloseTime != nil {
		hasData = true
	}
	if swap, ok := rollerMap["swap"].(bool); ok {
		direction := rollerMotorNormal
		if swap {
			direction = rollerMotorReversed
		}
		roller.MotorDirection = StringPtr(direction)
		hasData = true
	}
	if swapInputs, ok := rollerMap["swap_inputs"].(bool); ok {
		roller.SwapInputs = BoolPtr(swapInputs)
		hasData = true
	}
	if mode, ok := rollerMap["input_mode"].(string); ok {
		switch mode {
		case gen1RollerOneButton:
			roller.InputMode = StringPtr(rollerInputSingle)
			hasData = true
		case gen1RollerOpenClose:
			roller.InputMode = StringPtr(rollerInputDual)
			hasData = true
		}
	}

	if convertRollerRemainder(rollerMap, roller) {
		hasData = true
	}

	if hasData {
		config.Roller = roller
	}
}

// exportRoller writes the one roller a Gen1 model with roller mode has. Only the fields with a
// Gen1 counterpart are written, so a stored field with none stays in the refusal check's diff.
func (c *Gen1Converter) exportRoller(config *DeviceConfiguration, gen1 map[string]interface{}, deviceType string) {
	if !HasRollerMode(deviceType) || config.Roller == nil {
		return
	}

	rollerMap := make(map[string]interface{})

	if config.Roller.MaxOpenTime != nil {
		rollerMap["maxtime_open"] = *config.Roller.MaxOpenTime
	}
	if config.Roller.MaxCloseTime != nil {
		rollerMap["maxtime_close"] = *config.Roller.MaxCloseTime
	}
	if config.Roller.MotorDirection != nil {
		switch *config.Roller.MotorDirection {
		case rollerMotorNormal:
			rollerMap["swap"] = false
		case rollerMotorReversed:
			rollerMap["swap"] = true
		}
	}
	if config.Roller.SwapInputs != nil {
		rollerMap["swap_inputs"] = *config.Roller.SwapInputs
	}
	if config.Roller.InputMode != nil {
		switch *config.Roller.InputMode {
		case rollerInputSingle:
			rollerMap["input_mode"] = gen1RollerOneButton
		case rollerInputDual:
			rollerMap["input_mode"] = gen1RollerOpenClose
		}
	}

	exportRollerRemainder(config.Roller, rollerMap)

	if len(rollerMap) == 0 {
		return
	}
	gen1["rollers"] = []map[string]interface{}{rollerMap}
}

// Device capability helper methods

// hasLED reports whether a model has the power LED, the key led_power_disable. Only plugs do.
func (c *Gen1Converter) hasLED(deviceType string) bool {
	switch deviceType {
	case "SHPLG-S", "SHPLG-1":
		return true
	default:
		return false
	}
}

// hasStatusLED reports whether a model has the status LED, the key led_status_disable: the plugs
// and the Dimmers (the Dimmer page lists it, and lists no led_power_disable). It is a list of its
// own, never derived from hasLight, because another light model may not have it.
func (c *Gen1Converter) hasStatusLED(deviceType string) bool {
	switch deviceType {
	case "SHPLG-S", "SHPLG-1", "SHDM-1", "SHDM-2":
		return true
	default:
		return false
	}
}

func (c *Gen1Converter) hasPowerMetering(deviceType string) bool {
	switch deviceType {
	case "SHPLG-S", "SHSW-PM", "SHSW-25":
		return true
	default:
		return false
	}
}

func (c *Gen1Converter) hasRelay(deviceType string) bool {
	switch deviceType {
	case "SHIX3-1":
		// Input-only device, no relays
		return false
	case "SHDM-1", "SHDM-2", "SHRGBW2", "SHBLB-1", "SHBDUO-1", "SHVIN-1":
		// A light model's channel is /settings/light/<n> (or color, white), and its settings carry a
		// lights array and no relays array. A relay sent to /settings/relay/<n> goes to a path the
		// device does not have.
		return false
	default:
		return true
	}
}

// hasColorWhiteChannels reports whether a model's lights are one color channel or four white ones, by
// its device mode (the RGBW2).
func hasColorWhiteChannels(deviceType string) bool {
	return deviceType == "SHRGBW2"
}

// hasDimmerSettings reports whether a model has a Dimmer's device-level settings (the dimming
// section: flat parameters of POST /settings, and the warm-up). It is a list of its own, never derived
// from hasLight, because another light model (the RGBW2) has light channels and none of these
// parameters. The Duo and Vintage have the night mode of that section only: see hasNightMode.
func (c *Gen1Converter) hasDimmerSettings(deviceType string) bool {
	switch deviceType {
	case "SHDM-1", "SHDM-2":
		return true
	default:
		return false
	}
}

// hasTransition reports whether a model has the on/off transition parameter of POST /settings: the
// Dimmers, the Duo and the Vintage. The Duo and the Vintage have none of a Dimmer's other device-level
// settings, so this is a list of its own and hasDimmerSettings stays the Dimmers'.
func (c *Gen1Converter) hasTransition(deviceType string) bool {
	return c.hasDimmerSettings(deviceType) || isGen1WhiteBulb(deviceType)
}

// hasLightEffect reports whether a model's light has the effect parameter: the RGBW2 (color mode) and
// the Bulb. A Dimmer's page has none.
func hasLightEffect(deviceType string) bool {
	return hasColorWhiteChannels(deviceType) || isGen1Bulb(deviceType)
}

// isGen1Bulb reports whether a model is the Shelly Bulb: one light channel, effect 0 to 6, the mode
// white or color.
func isGen1Bulb(deviceType string) bool {
	return deviceType == "SHBLB-1"
}

// isGen1WhiteBulb reports whether a model is the Shelly Duo or Vintage: one white channel with a name
// and a night mode, no mode and no effect.
func isGen1WhiteBulb(deviceType string) bool {
	return deviceType == "SHBDUO-1" || deviceType == "SHVIN-1"
}

// hasLightName reports whether the page lists name as a parameter of the model's light channel: the
// RGBW2, the Duo and the Vintage. A Dimmer's and a Bulb's page lists it as an attribute only.
func hasLightName(deviceType string) bool {
	return hasColorWhiteChannels(deviceType) || isGen1WhiteBulb(deviceType)
}

// hasNightMode reports whether a model has POST /settings/night_mode: the Dimmers, the Duo and the
// Vintage. The Duo and the Vintage have none of a Dimmer's other device-level settings, so this is a
// list of its own and hasDimmerSettings stays the Dimmers'.
func (c *Gen1Converter) hasNightMode(deviceType string) bool {
	return c.hasDimmerSettings(deviceType) || isGen1WhiteBulb(deviceType)
}

// hasLight reports whether a model's channels are lights: /settings/light/<n> for a Dimmer, a Bulb, a
// Duo or a Vintage, /settings/color/0 or /settings/white/<n> for an RGBW2.
func (c *Gen1Converter) hasLight(deviceType string) bool {
	switch deviceType {
	case "SHDM-1", "SHDM-2", "SHRGBW2", "SHBLB-1", "SHBDUO-1", "SHVIN-1":
		return true
	default:
		return false
	}
}

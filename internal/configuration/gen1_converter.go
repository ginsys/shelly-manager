package configuration

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ginsys/shelly-manager/internal/logging"
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
	if err := c.convertWiFi(gen1, config); err != nil {
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
	if err := c.convertSystem(gen1, config); err != nil {
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

	// Convert PowerMetering settings
	if err := c.convertPowerMetering(gen1, config); err != nil {
		return nil, fmt.Errorf("failed to convert PowerMetering: %w", err)
	}

	// Convert LED settings
	if err := c.convertLED(gen1, config); err != nil {
		return nil, fmt.Errorf("failed to convert LED: %w", err)
	}

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

	// Convert PowerMetering settings
	c.exportPowerMetering(config, gen1)

	// Convert LED settings
	c.exportLED(config, gen1)

	// Marshal to JSON
	return json.Marshal(gen1)
}

// SupportedDeviceTypes returns list of supported Gen1 device types
func (c *Gen1Converter) SupportedDeviceTypes() []string {
	return []string{
		"SHPLG-S", // Smart Plug with metering
		"SHPLG-1", // Smart Plug
		"SHSW-1",  // Shelly 1
		"SHSW-PM", // Shelly 1PM
		"SHSW-25", // Shelly 2.5
		"SHIX3-1", // Shelly i3
	}
}

// Generation returns 1 for Gen1 devices
func (c *Gen1Converter) Generation() int {
	return 1
}

// Helper functions for FromAPIConfig

func (c *Gen1Converter) convertWiFi(gen1 map[string]interface{}, config *DeviceConfiguration) error {
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

	// Static IP configuration
	if ip, hasIP := wifiSTA["ip"].(string); hasIP && ip != "" {
		staticIP := &StaticIPConfig{
			IP: StringPtr(ip),
		}
		if netmask, ok := wifiSTA["netmask"].(string); ok {
			staticIP.Netmask = StringPtr(netmask)
		}
		if gw, ok := wifiSTA["gw"].(string); ok {
			staticIP.Gateway = StringPtr(gw)
		}
		if dns, ok := wifiSTA["dns"].(string); ok {
			staticIP.Nameserver = StringPtr(dns)
		}
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
	if keepAlive, ok := mqttData["keep_alive"].(float64); ok {
		mqtt.KeepAlive = IntPtr(int(keepAlive))
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
	if updatePeriod, ok := coiotData["update_period"].(float64); ok {
		coiot.UpdatePeriod = IntPtr(int(updatePeriod))
	}
	if peer, ok := coiotData["peer"].(string); ok {
		coiot.Peer = StringPtr(peer)
	}

	config.CoIoT = coiot
	return nil
}

func (c *Gen1Converter) convertSystem(gen1 map[string]interface{}, config *DeviceConfiguration) error {
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

	if device.Name != nil || device.EcoMode != nil || device.Discoverable != nil {
		system.Device = device
		config.System = system
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
		if autoOn, ok := relayMap["auto_on"].(float64); ok {
			singleRelay.AutoOn = IntPtr(int(autoOn))
		}
		if autoOff, ok := relayMap["auto_off"].(float64); ok {
			singleRelay.AutoOff = IntPtr(int(autoOff))
		}

		relay.Relays = append(relay.Relays, singleRelay)
	}

	if len(relay.Relays) > 0 {
		config.Relay = relay
	}

	return nil
}

func (c *Gen1Converter) convertPowerMetering(gen1 map[string]interface{}, config *DeviceConfiguration) error {
	if maxPower, ok := gen1["max_power"].(float64); ok {
		config.PowerMetering = &PowerMeteringConfig{
			MaxPower: IntPtr(int(maxPower)),
		}
	}

	return nil
}

func (c *Gen1Converter) convertLED(gen1 map[string]interface{}, config *DeviceConfiguration) error {
	led := &LEDConfig{}
	hasData := false

	// Gen1 uses led_power_disable and led_status_disable
	if ledPowerDisable, ok := gen1["led_power_disable"].(bool); ok {
		// Inverted: disable=true means enabled=false
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
			wifiSTA["netmask"] = *config.WiFi.StaticIP.Netmask
		}
		if config.WiFi.StaticIP.Gateway != nil {
			wifiSTA["gw"] = *config.WiFi.StaticIP.Gateway
		}
		if config.WiFi.StaticIP.Nameserver != nil {
			wifiSTA["dns"] = *config.WiFi.StaticIP.Nameserver
		}
	}

	gen1["wifi_sta"] = wifiSTA

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
	if config.System == nil || config.System.Device == nil {
		return
	}

	if config.System.Device.Name != nil {
		gen1["name"] = *config.System.Device.Name
	}
	if config.System.Device.EcoMode != nil {
		gen1["eco_mode_enabled"] = *config.System.Device.EcoMode
	}
	if config.System.Device.Discoverable != nil {
		gen1["discoverable"] = *config.System.Device.Discoverable
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

		relays[i] = relayMap
	}

	gen1["relays"] = relays
}

func (c *Gen1Converter) exportPowerMetering(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.PowerMetering == nil {
		return
	}

	if config.PowerMetering.MaxPower != nil {
		gen1["max_power"] = *config.PowerMetering.MaxPower
	}
}

func (c *Gen1Converter) exportLED(config *DeviceConfiguration, gen1 map[string]interface{}) {
	if config.LED == nil {
		return
	}

	// Inverted: enabled=false means disable=true
	if config.LED.PowerIndication != nil {
		gen1["led_power_disable"] = !*config.LED.PowerIndication
	}
	if config.LED.NetworkIndication != nil {
		gen1["led_status_disable"] = !*config.LED.NetworkIndication
	}
}

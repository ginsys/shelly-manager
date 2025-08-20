package api

import (
	"encoding/json"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// NormalizedConfig represents a configuration in a standardized, comparable format
type NormalizedConfig struct {
	WiFi     NormalizedWiFi     `json:"wifi"`
	MQTT     NormalizedMQTT     `json:"mqtt"`
	Auth     NormalizedAuth     `json:"auth"`
	Device   NormalizedDevice   `json:"device"`
	Cloud    NormalizedCloud    `json:"cloud"`
	CoIoT    NormalizedCoIoT    `json:"coiot"`
	SNTP     NormalizedSNTP     `json:"sntp"`
	Location NormalizedLocation `json:"location"`
	Relay    *NormalizedRelay   `json:"relay,omitempty"`
	Raw      NormalizedRaw      `json:"raw"`
	Time     NormalizedTime     `json:"time"`
}

type NormalizedWiFi struct {
	Enabled bool             `json:"enabled"`
	SSID    string           `json:"ssid"`
	DHCP    bool             `json:"dhcp"`
	AP      NormalizedWiFiAP `json:"ap"`
}

type NormalizedWiFiAP struct {
	Enabled bool   `json:"enabled"`
	SSID    string `json:"ssid"`
}

type NormalizedMQTT struct {
	Enabled   bool   `json:"enabled"`
	Server    string `json:"server"`
	User      string `json:"user"`
	KeepAlive int    `json:"keep_alive"`
}

type NormalizedAuth struct {
	Enabled bool   `json:"enabled"`
	User    string `json:"user"`
}

type NormalizedDevice struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	MAC      string `json:"mac"`
}

type NormalizedCloud struct {
	Enabled bool `json:"enabled"`
}

type NormalizedCoIoT struct {
	Enabled bool `json:"enabled"`
}

type NormalizedSNTP struct {
	Server string `json:"server"`
}

type NormalizedLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type NormalizedRelay struct {
	State        string `json:"state"`
	AutoOff      int    `json:"auto_off"`
	AutoOn       int    `json:"auto_on"`
	DefaultState string `json:"default_state"`
}

type NormalizedRaw struct {
	EcoModeEnabled   *bool `json:"eco_mode_enabled,omitempty"`
	LEDPowerDisable  *bool `json:"led_power_disable,omitempty"`
	LEDStatusDisable *bool `json:"led_status_disable,omitempty"`
	MaxPower         *int  `json:"max_power,omitempty"`
	Discoverable     *bool `json:"discoverable,omitempty"`
	DebugEnable      *bool `json:"debug_enable,omitempty"`
	AllowCrossOrigin *bool `json:"allow_cross_origin,omitempty"`

	// Additional raw fields stored as-is for complete comparison
	Additional map[string]interface{} `json:",inline"`
}

type NormalizedTime struct {
	Time         string `json:"time,omitempty"`
	UnixTime     *int64 `json:"unixtime,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	TZDst        *bool  `json:"tz_dst,omitempty"`
	TZDstAuto    *bool  `json:"tz_dst_auto,omitempty"`
	TZUtcOffset  *int   `json:"tz_utc_offset,omitempty"`
	TZAutoDetect *bool  `json:"tz_autodetect,omitempty"`
}

// ConfigNormalizer handles conversion of various config formats to normalized format
type ConfigNormalizer struct{}

// NewConfigNormalizer creates a new config normalizer
func NewConfigNormalizer() *ConfigNormalizer {
	return &ConfigNormalizer{}
}

// NormalizeRawConfig normalizes a raw device configuration (from device response)
func (n *ConfigNormalizer) NormalizeRawConfig(rawConfig map[string]interface{}) *NormalizedConfig {
	normalized := &NormalizedConfig{}

	// Helper function to safely get nested values
	getNestedValue := func(obj map[string]interface{}, path string) interface{} {
		keys := []string{}
		currentPath := ""
		for _, char := range path {
			if char == '.' {
				if currentPath != "" {
					keys = append(keys, currentPath)
					currentPath = ""
				}
			} else {
				currentPath += string(char)
			}
		}
		if currentPath != "" {
			keys = append(keys, currentPath)
		}

		current := obj
		for _, key := range keys {
			if current == nil {
				return nil
			}
			if val, ok := current[key]; ok {
				if nextMap, isMap := val.(map[string]interface{}); isMap {
					current = nextMap
				} else {
					return val
				}
			} else {
				return nil
			}
		}
		return current
	}

	// WiFi Configuration
	if wifiSta, ok := getNestedValue(rawConfig, "wifi_sta").(map[string]interface{}); ok {
		if enabled, ok := wifiSta["enabled"].(bool); ok {
			normalized.WiFi.Enabled = enabled
		}
		if ssid, ok := wifiSta["ssid"].(string); ok {
			normalized.WiFi.SSID = ssid
		}
		if method, ok := wifiSta["ipv4_method"].(string); ok {
			normalized.WiFi.DHCP = method == "dhcp"
		}
	}

	// WiFi AP Configuration
	if wifiAP, ok := getNestedValue(rawConfig, "wifi_ap").(map[string]interface{}); ok {
		if enabled, ok := wifiAP["enabled"].(bool); ok {
			normalized.WiFi.AP.Enabled = enabled
		}
		if ssid, ok := wifiAP["ssid"].(string); ok {
			normalized.WiFi.AP.SSID = ssid
		}
	}

	// MQTT Configuration
	if mqtt, ok := getNestedValue(rawConfig, "mqtt").(map[string]interface{}); ok {
		if enabled, ok := mqtt["enable"].(bool); ok {
			normalized.MQTT.Enabled = enabled
		}
		if server, ok := mqtt["server"].(string); ok {
			normalized.MQTT.Server = server
		}
		if user, ok := mqtt["user"].(string); ok {
			normalized.MQTT.User = user
		}
		if keepAlive, ok := mqtt["keep_alive"].(float64); ok {
			normalized.MQTT.KeepAlive = int(keepAlive)
		}
	}

	// Authentication
	if login, ok := getNestedValue(rawConfig, "login").(map[string]interface{}); ok {
		if enabled, ok := login["enabled"].(bool); ok {
			normalized.Auth.Enabled = enabled
		}
		if username, ok := login["username"].(string); ok {
			normalized.Auth.User = username
		}
	}

	// Device Information
	if name, ok := rawConfig["name"].(string); ok {
		normalized.Device.Name = name
	}
	if device, ok := getNestedValue(rawConfig, "device").(map[string]interface{}); ok {
		if hostname, ok := device["hostname"].(string); ok {
			normalized.Device.Hostname = hostname
		}
		if mac, ok := device["mac"].(string); ok {
			normalized.Device.MAC = mac
		}
	}

	// Cloud
	if cloud, ok := getNestedValue(rawConfig, "cloud").(map[string]interface{}); ok {
		if enabled, ok := cloud["enabled"].(bool); ok {
			normalized.Cloud.Enabled = enabled
		}
	}

	// CoIoT
	if coiot, ok := getNestedValue(rawConfig, "coiot").(map[string]interface{}); ok {
		if enabled, ok := coiot["enabled"].(bool); ok {
			normalized.CoIoT.Enabled = enabled
		}
	}

	// SNTP
	if sntp, ok := getNestedValue(rawConfig, "sntp").(map[string]interface{}); ok {
		if server, ok := sntp["server"].(string); ok {
			normalized.SNTP.Server = server
		}
	}

	// Location
	if lat, ok := rawConfig["lat"].(float64); ok {
		normalized.Location.Lat = lat
	}
	if lng, ok := rawConfig["lng"].(float64); ok {
		normalized.Location.Lng = lng
	}

	// Timezone
	if timezone, ok := rawConfig["timezone"].(string); ok {
		normalized.Time.Timezone = timezone
	}

	// Relay Configuration (for switches/plugs)
	if relays, ok := rawConfig["relays"].([]interface{}); ok && len(relays) > 0 {
		if relay, ok := relays[0].(map[string]interface{}); ok {
			normalized.Relay = &NormalizedRelay{}
			if ison, ok := relay["ison"].(bool); ok {
				if ison {
					normalized.Relay.State = "on"
				} else {
					normalized.Relay.State = "off"
				}
			}
			if autoOff, ok := relay["auto_off"].(float64); ok {
				normalized.Relay.AutoOff = int(autoOff)
			}
			if autoOn, ok := relay["auto_on"].(float64); ok {
				normalized.Relay.AutoOn = int(autoOn)
			}
			if defaultState, ok := relay["default_state"].(string); ok {
				normalized.Relay.DefaultState = defaultState
			}
		}
	}

	// Raw fields - initialize additional fields map
	normalized.Raw.Additional = make(map[string]interface{})

	// Track which fields we handle explicitly
	rawHandledFields := map[string]bool{
		"eco_mode_enabled":   false,
		"led_power_disable":  false,
		"led_status_disable": false,
		"max_power":          false,
		"discoverable":       false,
		"debug_enable":       false,
		"allow_cross_origin": false,
		// Don't include fields that go to other sections
		"lat": true, "lng": true, "timezone": true,
		"time": true, "unixtime": true, "tz_dst": true, "tz_dst_auto": true, "tz_utc_offset": true, "tzautodetect": true,
		// Skip structured fields that have their own sections
		"wifi_sta": true, "wifi_ap": true, "mqtt": true, "login": true, "device": true, "cloud": true, "coiot": true, "sntp": true, "relays": true,
	}

	if ecoMode, ok := rawConfig["eco_mode_enabled"].(bool); ok {
		normalized.Raw.EcoModeEnabled = &ecoMode
		rawHandledFields["eco_mode_enabled"] = true
	}
	if ledPower, ok := rawConfig["led_power_disable"].(bool); ok {
		normalized.Raw.LEDPowerDisable = &ledPower
		rawHandledFields["led_power_disable"] = true
	}
	if ledStatus, ok := rawConfig["led_status_disable"].(bool); ok {
		normalized.Raw.LEDStatusDisable = &ledStatus
		rawHandledFields["led_status_disable"] = true
	}
	if maxPower, ok := rawConfig["max_power"].(float64); ok {
		maxPowerInt := int(maxPower)
		normalized.Raw.MaxPower = &maxPowerInt
		rawHandledFields["max_power"] = true
	}
	if discoverable, ok := rawConfig["discoverable"].(bool); ok {
		normalized.Raw.Discoverable = &discoverable
		rawHandledFields["discoverable"] = true
	}
	if debugEnable, ok := rawConfig["debug_enable"].(bool); ok {
		normalized.Raw.DebugEnable = &debugEnable
		rawHandledFields["debug_enable"] = true
	}
	if allowCrossOrigin, ok := rawConfig["allow_cross_origin"].(bool); ok {
		normalized.Raw.AllowCrossOrigin = &allowCrossOrigin
		rawHandledFields["allow_cross_origin"] = true
	}

	// Add remaining fields to Additional map
	for key, value := range rawConfig {
		if !rawHandledFields[key] {
			normalized.Raw.Additional[key] = value
		}
	}

	// Time fields
	if time, ok := rawConfig["time"].(string); ok {
		normalized.Time.Time = time
	}
	if unixtime, ok := rawConfig["unixtime"].(float64); ok {
		unixtimeInt := int64(unixtime)
		normalized.Time.UnixTime = &unixtimeInt
	}
	if tzDst, ok := rawConfig["tz_dst"].(bool); ok {
		normalized.Time.TZDst = &tzDst
	}
	if tzDstAuto, ok := rawConfig["tz_dst_auto"].(bool); ok {
		normalized.Time.TZDstAuto = &tzDstAuto
	}
	if tzUtcOffset, ok := rawConfig["tz_utc_offset"].(float64); ok {
		tzUtcOffsetInt := int(tzUtcOffset)
		normalized.Time.TZUtcOffset = &tzUtcOffsetInt
	}
	if tzAutoDetect, ok := rawConfig["tzautodetect"].(bool); ok {
		normalized.Time.TZAutoDetect = &tzAutoDetect
	}

	return normalized
}

// NormalizeTypedConfig normalizes a typed configuration (from database)
func (n *ConfigNormalizer) NormalizeTypedConfig(typedConfig *configuration.TypedConfiguration) *NormalizedConfig {
	normalized := &NormalizedConfig{}

	if typedConfig.WiFi != nil {
		normalized.WiFi.Enabled = typedConfig.WiFi.Enable
		normalized.WiFi.SSID = typedConfig.WiFi.SSID
		normalized.WiFi.DHCP = typedConfig.WiFi.IPv4Mode == "dhcp"

		if typedConfig.WiFi.AccessPoint != nil {
			normalized.WiFi.AP.Enabled = typedConfig.WiFi.AccessPoint.Enable
			normalized.WiFi.AP.SSID = typedConfig.WiFi.AccessPoint.SSID
		}
	}

	if typedConfig.MQTT != nil {
		normalized.MQTT.Enabled = typedConfig.MQTT.Enable
		normalized.MQTT.Server = typedConfig.MQTT.Server
		normalized.MQTT.User = typedConfig.MQTT.User
		normalized.MQTT.KeepAlive = typedConfig.MQTT.KeepAlive
	}

	if typedConfig.Auth != nil {
		normalized.Auth.Enabled = typedConfig.Auth.Enable
		normalized.Auth.User = typedConfig.Auth.Username
	}

	if typedConfig.System != nil && typedConfig.System.Device != nil {
		normalized.Device.Name = typedConfig.System.Device.Name
		normalized.Device.Hostname = typedConfig.System.Device.Hostname
		normalized.Device.MAC = typedConfig.System.Device.MAC
		normalized.Time.Timezone = typedConfig.System.Device.Timezone

		// Add discoverable from system device settings
		if typedConfig.System.Device.Discoverable {
			discoverable := typedConfig.System.Device.Discoverable
			normalized.Raw.Discoverable = &discoverable
		}

		if typedConfig.System.Location != nil {
			normalized.Location.Lat = typedConfig.System.Location.Latitude
			normalized.Location.Lng = typedConfig.System.Location.Longitude
		}

		if typedConfig.System.SNTP != nil {
			normalized.SNTP.Server = typedConfig.System.SNTP.Server
		}
	}

	if typedConfig.Cloud != nil {
		normalized.Cloud.Enabled = typedConfig.Cloud.Enable
	}

	if typedConfig.CoIoT != nil {
		normalized.CoIoT.Enabled = typedConfig.CoIoT.Enabled
	}

	// Convert Relay configuration
	if typedConfig.Relay != nil {
		normalized.Relay = &NormalizedRelay{}

		if typedConfig.Relay.DefaultState != "" {
			normalized.Relay.DefaultState = typedConfig.Relay.DefaultState
		}
		if typedConfig.Relay.AutoOn != nil {
			normalized.Relay.AutoOn = *typedConfig.Relay.AutoOn
		}
		if typedConfig.Relay.AutoOff != nil {
			normalized.Relay.AutoOff = *typedConfig.Relay.AutoOff
		}

		// Use first relay for state (if relays exist)
		if len(typedConfig.Relay.Relays) > 0 {
			firstRelay := typedConfig.Relay.Relays[0]
			if firstRelay.DefaultState != "" {
				normalized.Relay.DefaultState = firstRelay.DefaultState
			}
			if firstRelay.AutoOn != nil {
				normalized.Relay.AutoOn = *firstRelay.AutoOn
			}
			if firstRelay.AutoOff != nil {
				normalized.Relay.AutoOff = *firstRelay.AutoOff
			}
		}

		// Default state from actual relay state is handled separately
		normalized.Relay.State = "off" // Default, actual state comes from device status
	}

	// Extract raw fields if available
	if typedConfig.Raw != nil {
		var rawData map[string]interface{}
		if err := json.Unmarshal(typedConfig.Raw, &rawData); err == nil {
			// Initialize additional fields map
			normalized.Raw.Additional = make(map[string]interface{})

			// Track which fields we handle explicitly
			handledFields := map[string]bool{
				"eco_mode_enabled":   false,
				"led_power_disable":  false,
				"led_status_disable": false,
				"max_power":          false,
				"discoverable":       false,
				"debug_enable":       false,
				"allow_cross_origin": false,
				"time":               false,
				"unixtime":           false,
				"tz_dst":             false,
				"tz_dst_auto":        false,
				"tz_utc_offset":      false,
				"tzautodetect":       false,
			}

			// Handle specific fields with proper typing
			if ecoMode, ok := rawData["eco_mode_enabled"].(bool); ok {
				normalized.Raw.EcoModeEnabled = &ecoMode
				handledFields["eco_mode_enabled"] = true
			}
			if ledPower, ok := rawData["led_power_disable"].(bool); ok {
				normalized.Raw.LEDPowerDisable = &ledPower
				handledFields["led_power_disable"] = true
			}
			if ledStatus, ok := rawData["led_status_disable"].(bool); ok {
				normalized.Raw.LEDStatusDisable = &ledStatus
				handledFields["led_status_disable"] = true
			}
			if maxPower, ok := rawData["max_power"].(float64); ok {
				maxPowerInt := int(maxPower)
				normalized.Raw.MaxPower = &maxPowerInt
				handledFields["max_power"] = true
			}
			if discoverable, ok := rawData["discoverable"].(bool); ok {
				normalized.Raw.Discoverable = &discoverable
				handledFields["discoverable"] = true
			}
			if debugEnable, ok := rawData["debug_enable"].(bool); ok {
				normalized.Raw.DebugEnable = &debugEnable
				handledFields["debug_enable"] = true
			}
			if allowCrossOrigin, ok := rawData["allow_cross_origin"].(bool); ok {
				normalized.Raw.AllowCrossOrigin = &allowCrossOrigin
				handledFields["allow_cross_origin"] = true
			}
			if time, ok := rawData["time"].(string); ok {
				normalized.Time.Time = time
				handledFields["time"] = true
			}
			if unixtime, ok := rawData["unixtime"].(float64); ok {
				unixtimeInt := int64(unixtime)
				normalized.Time.UnixTime = &unixtimeInt
				handledFields["unixtime"] = true
			}
			if tzDst, ok := rawData["tz_dst"].(bool); ok {
				normalized.Time.TZDst = &tzDst
				handledFields["tz_dst"] = true
			}
			if tzDstAuto, ok := rawData["tz_dst_auto"].(bool); ok {
				normalized.Time.TZDstAuto = &tzDstAuto
				handledFields["tz_dst_auto"] = true
			}
			if tzUtcOffset, ok := rawData["tz_utc_offset"].(float64); ok {
				tzUtcOffsetInt := int(tzUtcOffset)
				normalized.Time.TZUtcOffset = &tzUtcOffsetInt
				handledFields["tz_utc_offset"] = true
			}
			if tzAutoDetect, ok := rawData["tzautodetect"].(bool); ok {
				normalized.Time.TZAutoDetect = &tzAutoDetect
				handledFields["tzautodetect"] = true
			}

			// Add all remaining fields to Additional map
			for key, value := range rawData {
				if !handledFields[key] {
					normalized.Raw.Additional[key] = value
				}
			}
		}
	}

	return normalized
}

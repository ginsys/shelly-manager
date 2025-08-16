package configuration

import (
	"encoding/json"
	"fmt"
)

// GetTemplateExamples returns common configuration templates with variable substitution
func GetTemplateExamples() map[string]string {
	examples := map[string]string{
		"basic_wifi": `{
			"wifi": {
				"enabled": true,
				"ssid": "{{.Network.SSID}}",
				"password": "{{.Network.Password | default ""}}"
			},
			"device": {
				"name": "{{.Device.Model}}-{{.Device.MAC | macLast4}}",
				"hostname": "{{.Device.Name | hostName}}"
			}
		}`,

		"gen2_comprehensive": `{
			"wifi_sta": {
				"enable": true,
				"ssid": "{{.Network.SSID}}",
				"pass": "{{.Custom.wifi_password | default ""}}",
				"ipv4mode": "{{.Network.IPMode | default "dhcp"}}",
				"static_ip": {{if eq (.Network.IPMode | default "dhcp") "static"}}{
					"ip": "{{.Network.StaticIP}}",
					"netmask": "{{.Network.Netmask}}",
					"gw": "{{.Network.Gateway}}",
					"nameserver": "{{.Network.DNS}}"
				}{{else}}null{{end}}
			},
			"sys": {
				"device": {
					"name": "{{.Custom.device_name | default (.Device.Model | deviceUnique .Device.MAC)}}",
					"discoverable": {{.Custom.discoverable | default true}},
					"eco_mode": {{.Custom.eco_mode | default false}}
				},
				"location": {
					"tz": "{{.Location.Timezone}}",
					"lat": {{.Location.Latitude}},
					"lng": {{.Location.Longitude}}
				},
				"debug": {
					"level": {{.Custom.debug_level | default 2}},
					"file_level": {{.Custom.file_debug_level | default 2}}
				},
				"sntp": {
					"server": "{{.Location.NTPServer}}"
				}
			},
			"mqtt": {{if .Custom.enable_mqtt}}{
				"enable": true,
				"server": "{{.Custom.mqtt_server}}:{{.Custom.mqtt_port | default 1883}}",
				"user": "{{.Auth.Username}}",
				"pass": "{{.Auth.Password}}",
				"topic_prefix": "{{.Custom.mqtt_prefix | default "shelly"}}/{{.Device.MAC | macNone}}"
			}{{else}}null{{end}},
			"cloud": {
				"enable": {{.Custom.enable_cloud | default false}},
				"server": "{{.Custom.cloud_server | default "shelly-cloud.allterco.com"}}"
			}
		}`,

		"gen1_basic": `{
			"wifi_ap": {
				"enabled": false
			},
			"wifi_sta": {
				"enabled": true,
				"ssid": "{{.Network.SSID}}",
				"key": "{{.Custom.wifi_password | default ""}}",
				"ipv4_method": "{{.Network.IPMode | default "dhcp"}}",
				"ip": "{{.Network.StaticIP | default ""}}",
				"gw": "{{.Network.Gateway | default ""}}",
				"mask": "{{.Network.Netmask | default ""}}",
				"dns": "{{.Network.DNS | default ""}}"
			},
			"login": {
				"enabled": {{if .Custom.enable_auth}}true{{else}}false{{end}},
				"username": "{{.Auth.Username | default "admin"}}",
				"password": "{{.Auth.Password | default ""}}"
			},
			"name": "{{.Custom.device_name | default (.Device.Model | deviceShortName .Device.MAC)}}",
			"timezone": "{{.Location.Timezone}}",
			"lat": {{.Location.Latitude | default 0}},
			"lng": {{.Location.Longitude | default 0}},
			"tzautodetect": {{.Custom.auto_timezone | default true}},
			"tz_utc_offset": {{.Custom.utc_offset | default 0}},
			"tz_dst": {{.Custom.enable_dst | default true}},
			"tz_dst_auto": {{.Custom.auto_dst | default true}},
			"time": "{{now | formatTime "15:04"}}",
			"sntp_server": "{{.Location.NTPServer}}"
		}`,

		"authentication_required": `{
			"auth": {
				"enable": true,
				"user": "{{.Auth.Username | required}}",
				"pass": "{{.Auth.Password | required}}",
				"realm": "{{.Auth.Realm | default "Shelly Device"}}"
			},
			"wifi_sta": {
				"enable": true,
				"ssid": "{{.Network.SSID | required}}",
				"pass": "{{.Custom.wifi_password | required}}"
			},
			"sys": {
				"device": {
					"name": "{{.Custom.device_name | default (.Device.Model | deviceUnique .Device.MAC)}}",
					"discoverable": false
				}
			}
		}`,

		"mqtt_integration": `{
			"mqtt": {
				"enable": true,
				"server": "{{.Custom.mqtt_server | required}}:{{.Custom.mqtt_port | default 1883}}",
				"id": "{{.Device.MAC | macNone}}-{{timestamp}}",
				"user": "{{.Custom.mqtt_username | default ""}}",
				"pass": "{{.Custom.mqtt_password | default ""}}",
				"clean_session": {{.Custom.mqtt_clean_session | default true}},
				"keep_alive": {{.Custom.mqtt_keepalive | default 60}},
				"max_qos": {{.Custom.mqtt_qos | default 1}},
				"retain": {{.Custom.mqtt_retain | default false}},
				"update_period": {{.Custom.mqtt_update_period | default 30}},
				"topic_prefix": "{{.Custom.mqtt_topic_prefix | default "homeassistant"}}/{{.Device.Model | lower}}/{{.Device.MAC | macNone}}"
			},
			"wifi_sta": {
				"enable": true,
				"ssid": "{{.Network.SSID}}",
				"pass": "{{.Custom.wifi_password}}"
			},
			"sys": {
				"device": {
					"name": "{{.Custom.device_name | default (.Device.Model | deviceUnique .Device.MAC)}}"
				}
			}
		}`,

		"static_ip_config": `{
			"wifi_sta": {
				"enable": true,
				"ssid": "{{.Network.SSID}}",
				"pass": "{{.Custom.wifi_password}}",
				"ipv4mode": "static",
				"static_ip": {
					"ip": "{{.Custom.static_ip | required}}",
					"netmask": "{{.Custom.netmask | default "255.255.255.0"}}",
					"gw": "{{.Custom.gateway | required}}",
					"nameserver": "{{.Custom.dns | default "8.8.8.8"}}"
				}
			},
			"sys": {
				"device": {
					"name": "{{.Custom.device_name | default (.Device.Model | deviceUnique .Device.MAC)}}"
				}
			}
		}`,

		"relay_automation": `{
			"switch:0": {
				"name": "{{.Custom.relay_name | default "Main Relay"}}",
				"in_mode": "{{.Custom.input_mode | default "follow"}}",
				"initial_state": "{{.Custom.initial_state | default "off"}}",
				"auto_on": {{.Custom.auto_on_delay | default 0}},
				"auto_off": {{.Custom.auto_off_delay | default 0}}
			},
			"input:0": {
				"name": "{{.Custom.input_name | default "Main Input"}}",
				"type": "{{.Custom.input_type | default "switch"}}",
				"invert": {{.Custom.invert_input | default false}}
			},
			"sys": {
				"device": {
					"name": "{{.Custom.device_name | default (.Device.Model | deviceShortName .Device.MAC)}}"
				}
			},
			"wifi_sta": {
				"enable": true,
				"ssid": "{{.Network.SSID}}",
				"pass": "{{.Custom.wifi_password}}"
			}
		}`,

		"cover_configuration": `{
			"cover:0": {
				"name": "{{.Custom.cover_name | default "Main Cover"}}",
				"motor": {
					"idle_power_thr": {{.Custom.idle_power_threshold | default 2}},
					"direction": "{{.Custom.motor_direction | default "normal"}}"
				},
				"maxtime_open": {{.Custom.max_open_time | default 60}},
				"maxtime_close": {{.Custom.max_close_time | default 60}},
				"swap_inputs": {{.Custom.swap_inputs | default false}},
				"inching": {
					"on_time": {{.Custom.inching_time | default 1}}
				},
				"obstruction_detection": {
					"enable": {{.Custom.enable_obstruction | default true}},
					"direction": "{{.Custom.obstruction_direction | default "both"}}"
				}
			},
			"sys": {
				"device": {
					"name": "{{.Custom.device_name | default (.Device.Model | deviceShortName .Device.MAC)}}"
				}
			},
			"wifi_sta": {
				"enable": true,
				"ssid": "{{.Network.SSID}}",
				"pass": "{{.Custom.wifi_password}}"
			}
		}`,
	}

	return examples
}

// GetTemplateDocumentation returns documentation for available template variables and functions
func GetTemplateDocumentation() map[string]interface{} {
	return map[string]interface{}{
		"variables": map[string]interface{}{
			"Device": map[string]string{
				"ID":         "Device database ID (uint)",
				"MAC":        "Device MAC address (string)",
				"IP":         "Device IP address (string)",
				"Name":       "Device name (string)",
				"Model":      "Device model (string)",
				"Generation": "Device generation (int)",
				"Firmware":   "Device firmware version (string)",
			},
			"Network": map[string]string{
				"SSID":    "WiFi network SSID (string)",
				"Gateway": "Network gateway IP (string)",
				"Subnet":  "Network subnet (string)",
				"DNS":     "DNS server IP (string)",
			},
			"System": map[string]string{
				"Timestamp":   "Current timestamp (time.Time)",
				"ConfigHash":  "Configuration hash (string)",
				"Environment": "Deployment environment (string)",
				"Version":     "System version (string)",
			},
			"Custom": "User-defined variables (map[string]interface{})",
			"Auth": map[string]string{
				"Username": "Authentication username (string)",
				"Password": "Authentication password (string)",
				"Realm":    "Authentication realm (string)",
			},
			"Location": map[string]string{
				"Timezone":  "Device timezone (string)",
				"Latitude":  "Device latitude (float64)",
				"Longitude": "Device longitude (float64)",
				"NTPServer": "NTP server address (string)",
			},
		},
		"functions": map[string]interface{}{
			"string_functions": []string{
				"upper", "lower", "title", "trim", "replace", "contains", "hasPrefix", "hasSuffix",
			},
			"mac_functions": []string{
				"macColon - format MAC with colons (AA:BB:CC:DD:EE:FF)",
				"macDash - format MAC with dashes (AA-BB-CC-DD-EE-FF)",
				"macNone - format MAC without separators (AABBCCDDEEFF)",
				"macLast4 - get last 4 characters of MAC (EEFF)",
				"macLast6 - get last 6 characters of MAC (DDEEFF)",
			},
			"ip_functions": []string{
				"ipOctets - split IP into array of octets",
				"ipLast - get last octet of IP address",
			},
			"device_functions": []string{
				"deviceShortName model mac - generate short device name",
				"deviceUnique model mac - generate unique device name",
				"hostName name - convert to valid hostname",
				"networkName ssid - sanitize network name",
			},
			"time_functions": []string{
				"now - current time",
				"formatTime format time - format time with Go layout",
				"timestamp - Unix timestamp",
			},
			"validation_functions": []string{
				"required value - ensure value is not empty",
				"default value defaultVal - use default if value is empty",
			},
			"utility_functions": []string{
				"toJson value - convert to JSON string",
				"fromJson jsonStr - parse JSON string",
				"add a b - addition",
				"sub a b - subtraction",
				"mul a b - multiplication",
				"div a b - division",
			},
		},
		"examples": map[string]string{
			"device_naming":    `"{{.Device.Model}}-{{.Device.MAC | macLast4}}"`,
			"conditional_auth": `{{if .Custom.enable_auth}}"enable": true{{else}}"enable": false{{end}}`,
			"required_field":   `"ssid": "{{.Network.SSID | required}}"`,
			"default_value":    `"port": {{.Custom.port | default 1883}}`,
			"mac_formatting":   `"id": "{{.Device.MAC | macNone | lower}}"`,
			"time_formatting":  `"timestamp": "{{now | formatTime "2006-01-02T15:04:05Z07:00"}}"`,
			"hostname_safe":    `"hostname": "{{.Device.Name | hostName}}"`,
			"mqtt_topic":       `"topic": "devices/{{.Device.Model | lower}}/{{.Device.MAC | macNone}}/status"`,
		},
	}
}

// ValidateTemplateExample validates a template example
func (s *Service) ValidateTemplateExample(templateName string) error {
	examples := GetTemplateExamples()
	templateStr, exists := examples[templateName]
	if !exists {
		return fmt.Errorf("template example '%s' not found", templateName)
	}

	return s.templateEngine.ValidateTemplate(templateStr)
}

// RenderTemplateExample renders a template example with sample data
func (s *Service) RenderTemplateExample(templateName string) (json.RawMessage, error) {
	examples := GetTemplateExamples()
	templateStr, exists := examples[templateName]
	if !exists {
		return nil, fmt.Errorf("template example '%s' not found", templateName)
	}

	// Create sample context
	sampleDevice := &Device{
		ID:       1,
		MAC:      "AA:BB:CC:DD:EE:FF",
		IP:       "192.168.1.100",
		Name:     "Test Device",
		Type:     "SHSW-25",
		Settings: `{"model":"SHSW-25","gen":1,"fw_id":"20230913-114336/v1.14.0-gcb84623"}`,
	}

	sampleVariables := map[string]interface{}{
		"network": map[string]interface{}{
			"ssid":    "TestNetwork",
			"gateway": "192.168.1.1",
			"dns":     "8.8.8.8",
		},
		"custom": map[string]interface{}{
			"wifi_password": "test123",
			"device_name":   "Living Room Switch",
			"enable_auth":   true,
			"enable_mqtt":   true,
			"mqtt_server":   "mqtt.example.com",
			"mqtt_port":     1883,
			"debug_level":   2,
		},
		"auth": map[string]interface{}{
			"username": "admin",
			"password": "secure123",
		},
		"location": map[string]interface{}{
			"timezone":  "Europe/London",
			"latitude":  51.5074,
			"longitude": -0.1278,
		},
	}

	context := s.templateEngine.CreateTemplateContext(sampleDevice, sampleVariables)

	// Populate context with sample variables
	if networkData, ok := sampleVariables["network"].(map[string]interface{}); ok {
		if ssid, ok := networkData["ssid"].(string); ok {
			context.Network.SSID = ssid
		}
		if gateway, ok := networkData["gateway"].(string); ok {
			context.Network.Gateway = gateway
		}
		if dns, ok := networkData["dns"].(string); ok {
			context.Network.DNS = dns
		}
	}

	if customData, ok := sampleVariables["custom"].(map[string]interface{}); ok {
		for key, value := range customData {
			context.Custom[key] = value
		}
	}

	if authData, ok := sampleVariables["auth"].(map[string]interface{}); ok {
		if username, ok := authData["username"].(string); ok {
			context.Auth.Username = username
		}
		if password, ok := authData["password"].(string); ok {
			context.Auth.Password = password
		}
	}

	if locationData, ok := sampleVariables["location"].(map[string]interface{}); ok {
		if timezone, ok := locationData["timezone"].(string); ok {
			context.Location.Timezone = timezone
		}
		if lat, ok := locationData["latitude"].(float64); ok {
			context.Location.Latitude = lat
		}
		if lng, ok := locationData["longitude"].(float64); ok {
			context.Location.Longitude = lng
		}
	}

	return s.templateEngine.SubstituteVariables(json.RawMessage(templateStr), context)
}

package shelly

import (
	"encoding/json"
	"testing"
	"time"
)

// Helper functions are now in testhelpers_test.go

func TestDeviceInfo_JSONSerialization(t *testing.T) {
	originalTime := time.Now()
	info := &DeviceInfo{
		ID:         "shellyplusht-08b61fcb7f3c",
		MAC:        "08B61FCB7F3C",
		Model:      "SNSN-0013A",
		Generation: 2,
		FirmwareID: "20231031-165617/1.0.3-geb51a17",
		Version:    "1.0.3",
		App:        "PlusHT",
		AuthEn:     false,
		AuthDomain: "shellyplusht-08b61fcb7f3c",
		Type:       "SHSW-1",
		FW:         "20231219-134356",
		Auth:       true,
		IP:         "192.168.1.100",
		Discovered: originalTime,
	}
	
	// Test JSON marshaling
	data, err := json.Marshal(info)
	assertNoError(t, err)
	
	// Test JSON unmarshaling
	var restored DeviceInfo
	err = json.Unmarshal(data, &restored)
	assertNoError(t, err)
	
	// Check that all JSON-tagged fields were preserved
	assertEqual(t, info.ID, restored.ID)
	assertEqual(t, info.MAC, restored.MAC)
	assertEqual(t, info.Model, restored.Model)
	assertEqual(t, info.Generation, restored.Generation)
	assertEqual(t, info.FirmwareID, restored.FirmwareID)
	assertEqual(t, info.Version, restored.Version)
	assertEqual(t, info.App, restored.App)
	assertEqual(t, info.AuthEn, restored.AuthEn)
	assertEqual(t, info.AuthDomain, restored.AuthDomain)
	assertEqual(t, info.Type, restored.Type)
	assertEqual(t, info.FW, restored.FW)
	assertEqual(t, info.Auth, restored.Auth)
	
	// Check that non-JSON fields are their zero values
	assertEqual(t, "", restored.IP)
	assertEqual(t, time.Time{}, restored.Discovered)
}

func TestDeviceStatus_Components(t *testing.T) {
	status := &DeviceStatus{
		Temperature:     45.2,
		Overtemperature: false,
		WiFiStatus: &WiFiStatus{
			Connected: true,
			SSID:      "TestNetwork",
			IP:        "192.168.1.100",
			RSSI:      -45,
		},
		Cloud: &CloudStatus{
			Enabled:   false,
			Connected: false,
		},
		MQTT: &MQTTStatus{
			Connected: true,
		},
		Time:      "15:04",
		Unixtime:  1234567890,
		HasUpdate: false,
		RAMTotal:  50592,
		RAMFree:   39052,
		FSSize:    233681,
		FSFree:    162648,
		Uptime:    3600,
		Switches: []SwitchStatus{
			{
				ID:          0,
				Output:      true,
				APower:      25.5,
				Voltage:     230.0,
				Current:     0.11,
				Temperature: 45.2,
				Source:      "input",
			},
		},
		Lights: []LightStatus{
			{
				ID:         0,
				Output:     true,
				Brightness: 75,
			},
		},
		Inputs: []InputStatus{
			{
				ID:    0,
				State: true,
			},
		},
		Rollers: []RollerStatus{
			{
				ID:              0,
				State:           "open",
				CurrentPosition: 100,
			},
		},
		Meters: []MeterStatus{
			{
				ID:      0,
				Power:   25.5,
				IsValid: true,
			},
		},
		Raw: map[string]interface{}{
			"custom_field": "custom_value",
		},
	}
	
	// Test that all fields are accessible
	assertEqual(t, 45.2, status.Temperature)
	assertEqual(t, false, status.Overtemperature)
	assertNotNil(t, status.WiFiStatus)
	assertEqual(t, true, status.WiFiStatus.Connected)
	assertEqual(t, 1, len(status.Switches))
	assertEqual(t, 0, status.Switches[0].ID)
	assertEqual(t, 1, len(status.Lights))
	assertEqual(t, 1, len(status.Inputs))
	assertEqual(t, 1, len(status.Rollers))
	assertEqual(t, 1, len(status.Meters))
	assertEqual(t, "custom_value", status.Raw["custom_field"])
}

func TestDeviceConfig_Initialization(t *testing.T) {
	config := &DeviceConfig{
		Name:     "Test Device",
		Timezone: "Europe/Sofia",
		Lat:      42.6977,
		Lng:      23.3219,
		WiFi: &WiFiConfig{
			Enable:    true,
			SSID:      "TestNetwork",
			Password:  "password123",
			IPV4Mode:  "dhcp",
			IP:        "",
			Netmask:   "",
			Gateway:   "",
			DNS:       "",
		},
		Ethernet: &EthernetConfig{
			Enable:  true,
			IPV4Mode: "dhcp",
		},
		Auth: &AuthConfig{
			Enable:   true,
			Username: "admin",
			Password: "secret",
		},
		Cloud: &CloudConfig{
			Enable: false,
			Server: "shelly-103-eu.shelly.cloud:6022/jrpc",
		},
		MQTT: &MQTTConfig{
			Enable:   true,
			Server:   "mqtt.example.com:1883",
			User:     "mqtt_user",
			Password: "mqtt_pass",
			ID:       "shelly_device",
		},
		Switches: []SwitchConfig{
			{
				ID:           0,
				Name:         "Switch 0",
				InMode:       "momentary",
				InitialState: "restore_last",
				AutoOn:       0,
				AutoOff:      0,
			},
		},
		Lights: []LightConfig{
			{
				ID:                0,
				Name:              "Light 0",
				InitialState:      "on",
				DefaultBrightness: 75,
			},
		},
		Inputs: []InputConfig{
			{
				ID:     0,
				Name:   "Input 0",
				Type:   "switch",
				Invert: false,
			},
		},
		Rollers: []RollerConfig{
			{
				ID:           0,
				Name:         "Roller 0",
				DefaultState: "open",
				MaxTime:      60,
			},
		},
		Debug: false,
		WebUI: true,
		Raw:   json.RawMessage(`{"custom_setting": "value"}`),
	}
	
	// Test field access
	assertEqual(t, "Test Device", config.Name)
	assertEqual(t, "Europe/Sofia", config.Timezone)
	assertEqual(t, 42.6977, config.Lat)
	assertEqual(t, 23.3219, config.Lng)
	
	assertNotNil(t, config.WiFi)
	assertTrue(t, config.WiFi.Enable)
	assertEqual(t, "TestNetwork", config.WiFi.SSID)
	
	assertNotNil(t, config.Ethernet)
	assertTrue(t, config.Ethernet.Enable)
	
	assertNotNil(t, config.Auth)
	assertTrue(t, config.Auth.Enable)
	assertEqual(t, "admin", config.Auth.Username)
	
	assertNotNil(t, config.Cloud)
	assertEqual(t, false, config.Cloud.Enable)
	
	assertNotNil(t, config.MQTT)
	assertTrue(t, config.MQTT.Enable)
	assertEqual(t, "mqtt.example.com:1883", config.MQTT.Server)
	
	assertEqual(t, 1, len(config.Switches))
	assertEqual(t, "Switch 0", config.Switches[0].Name)
	
	assertEqual(t, 1, len(config.Lights))
	assertEqual(t, "Light 0", config.Lights[0].Name)
	
	assertEqual(t, 1, len(config.Inputs))
	assertEqual(t, "Input 0", config.Inputs[0].Name)
	
	assertEqual(t, 1, len(config.Rollers))
	assertEqual(t, "Roller 0", config.Rollers[0].Name)
	
	assertEqual(t, false, config.Debug)
	assertTrue(t, config.WebUI)
	assertNotNil(t, config.Raw)
}

func TestWiFiStatus_Fields(t *testing.T) {
	status := &WiFiStatus{
		Connected: true,
		SSID:      "TestNetwork",
		IP:        "192.168.1.100",
		RSSI:      -45,
	}
	
	assertTrue(t, status.Connected)
	assertEqual(t, "TestNetwork", status.SSID)
	assertEqual(t, "192.168.1.100", status.IP)
	assertEqual(t, -45, status.RSSI)
}

func TestWiFiStatus_EmptyValues(t *testing.T) {
	status := &WiFiStatus{}
	
	assertEqual(t, false, status.Connected)
	assertEqual(t, "", status.SSID)
	assertEqual(t, "", status.IP)
	assertEqual(t, 0, status.RSSI)
}

func TestDeviceInfo_Gen1Fields(t *testing.T) {
	info := &DeviceInfo{
		Type: "SHSW-1",
		FW:   "20231219-134356",
		Auth: true,
	}
	
	assertEqual(t, "SHSW-1", info.Type)
	assertEqual(t, "20231219-134356", info.FW)
	assertTrue(t, info.Auth)
}

func TestDeviceInfo_Gen2Fields(t *testing.T) {
	info := &DeviceInfo{
		ID:         "shellyplusht-08b61fcb7f3c",
		MAC:        "08B61FCB7F3C",
		Model:      "SNSN-0013A",
		Generation: 2,
		FirmwareID: "20231031-165617/1.0.3-geb51a17",
		Version:    "1.0.3",
		App:        "PlusHT",
		AuthEn:     false,
		AuthDomain: "shellyplusht-08b61fcb7f3c",
	}
	
	assertEqual(t, "shellyplusht-08b61fcb7f3c", info.ID)
	assertEqual(t, "08B61FCB7F3C", info.MAC)
	assertEqual(t, "SNSN-0013A", info.Model)
	assertEqual(t, 2, info.Generation)
	assertEqual(t, "20231031-165617/1.0.3-geb51a17", info.FirmwareID)
	assertEqual(t, "1.0.3", info.Version)
	assertEqual(t, "PlusHT", info.App)
	assertEqual(t, false, info.AuthEn)
	assertEqual(t, "shellyplusht-08b61fcb7f3c", info.AuthDomain)
}

func TestDeviceInfo_MetadataFields(t *testing.T) {
	discovered := time.Now()
	info := &DeviceInfo{
		IP:         "192.168.1.100",
		Discovered: discovered,
	}
	
	assertEqual(t, "192.168.1.100", info.IP)
	assertEqual(t, discovered, info.Discovered)
}

func TestDeviceStatus_OptionalFields(t *testing.T) {
	status := &DeviceStatus{}
	
	// Test that optional fields can be nil
	assertEqual(t, (*WiFiStatus)(nil), status.WiFiStatus)
	assertEqual(t, (*CloudStatus)(nil), status.Cloud)
	assertEqual(t, (*MQTTStatus)(nil), status.MQTT)
	
	// Test that slices are empty by default
	assertEqual(t, 0, len(status.Switches))
	assertEqual(t, 0, len(status.Lights))
	assertEqual(t, 0, len(status.Inputs))
	assertEqual(t, 0, len(status.Rollers))
	assertEqual(t, 0, len(status.Meters))
}

func TestDeviceConfig_OptionalFields(t *testing.T) {
	config := &DeviceConfig{}
	
	// Test that optional config sections can be nil
	assertEqual(t, (*WiFiConfig)(nil), config.WiFi)
	assertEqual(t, (*EthernetConfig)(nil), config.Ethernet)
	assertEqual(t, (*AuthConfig)(nil), config.Auth)
	assertEqual(t, (*CloudConfig)(nil), config.Cloud)
	assertEqual(t, (*MQTTConfig)(nil), config.MQTT)
	
	// Test that component config slices are empty by default
	assertEqual(t, 0, len(config.Switches))
	assertEqual(t, 0, len(config.Lights))
	assertEqual(t, 0, len(config.Inputs))
	assertEqual(t, 0, len(config.Rollers))
}

func TestDeviceConfig_RawJSON(t *testing.T) {
	rawData := json.RawMessage(`{"custom_field": "custom_value", "number": 42}`)
	config := &DeviceConfig{
		Raw: rawData,
	}
	
	assertNotNil(t, config.Raw)
	
	// Parse the raw JSON to verify it's valid
	var parsed map[string]interface{}
	err := json.Unmarshal(config.Raw, &parsed)
	assertNoError(t, err)
	assertEqual(t, "custom_value", parsed["custom_field"])
	assertEqual(t, 42.0, parsed["number"].(float64)) // JSON numbers are float64
}

func TestDeviceStatus_RawData(t *testing.T) {
	rawData := map[string]interface{}{
		"custom_component": map[string]interface{}{
			"value": float64(42),
			"state": "active",
		},
		"extra_field": "extra_value",
	}
	
	status := &DeviceStatus{
		Raw: rawData,
	}
	
	assertNotNil(t, status.Raw)
	assertEqual(t, "extra_value", status.Raw["extra_field"])
	
	customComponent, ok := status.Raw["custom_component"].(map[string]interface{})
	assertTrue(t, ok)
	assertEqual(t, 42.0, customComponent["value"])
	assertEqual(t, "active", customComponent["state"])
}
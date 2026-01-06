package configuration

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestGen1Converter_FromAPIConfig_SHPLGS(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	// Load test fixture
	fixture, err := os.ReadFile("testdata/shplg_s_settings.json")
	if err != nil {
		t.Fatalf("failed to load test fixture: %v", err)
	}

	// Convert from Gen1 API to DeviceConfiguration
	config, err := converter.FromAPIConfig(fixture, "SHPLG-S")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	// Verify WiFi settings
	if config.WiFi == nil {
		t.Fatal("WiFi config is nil")
	}
	if config.WiFi.Enable == nil || !*config.WiFi.Enable {
		t.Error("WiFi should be enabled")
	}
	if config.WiFi.SSID == nil || *config.WiFi.SSID != "MyHomeNetwork" {
		t.Errorf("WiFi SSID = %v, want MyHomeNetwork", StringVal(config.WiFi.SSID, ""))
	}
	if config.WiFi.Password == nil || *config.WiFi.Password != "mypassword123" {
		t.Errorf("WiFi Password = %v, want mypassword123", StringVal(config.WiFi.Password, ""))
	}
	if config.WiFi.IPv4Mode == nil || *config.WiFi.IPv4Mode != "dhcp" {
		t.Errorf("WiFi IPv4Mode = %v, want dhcp", StringVal(config.WiFi.IPv4Mode, ""))
	}

	// Verify WiFi AP
	if config.WiFi.AccessPoint == nil {
		t.Error("WiFi AccessPoint is nil")
	} else {
		if config.WiFi.AccessPoint.Enable == nil || *config.WiFi.AccessPoint.Enable {
			t.Error("WiFi AP should be disabled")
		}
		if config.WiFi.AccessPoint.SSID == nil || *config.WiFi.AccessPoint.SSID != "shellyplug-s-AABBCCDDEEFF" {
			t.Errorf("WiFi AP SSID = %v, want shellyplug-s-AABBCCDDEEFF", StringVal(config.WiFi.AccessPoint.SSID, ""))
		}
	}

	// Verify MQTT settings
	if config.MQTT == nil {
		t.Fatal("MQTT config is nil")
	}
	if config.MQTT.Enable == nil || !*config.MQTT.Enable {
		t.Error("MQTT should be enabled")
	}
	if config.MQTT.Server == nil || *config.MQTT.Server != "192.168.1.100" {
		t.Errorf("MQTT Server = %v, want 192.168.1.100", StringVal(config.MQTT.Server, ""))
	}
	if config.MQTT.Port == nil || *config.MQTT.Port != 1883 {
		t.Errorf("MQTT Port = %v, want 1883", IntVal(config.MQTT.Port, 0))
	}
	if config.MQTT.User == nil || *config.MQTT.User != "mqtt_user" {
		t.Errorf("MQTT User = %v, want mqtt_user", StringVal(config.MQTT.User, ""))
	}
	if config.MQTT.Password == nil || *config.MQTT.Password != "mqtt_pass" {
		t.Errorf("MQTT Password = %v, want mqtt_pass", StringVal(config.MQTT.Password, ""))
	}
	if config.MQTT.ClientID == nil || *config.MQTT.ClientID != "shellyplug-s-AABBCCDDEEFF" {
		t.Errorf("MQTT ClientID = %v, want shellyplug-s-AABBCCDDEEFF", StringVal(config.MQTT.ClientID, ""))
	}
	if config.MQTT.CleanSession == nil || !*config.MQTT.CleanSession {
		t.Error("MQTT CleanSession should be true")
	}
	if config.MQTT.KeepAlive == nil || *config.MQTT.KeepAlive != 60 {
		t.Errorf("MQTT KeepAlive = %v, want 60", IntVal(config.MQTT.KeepAlive, 0))
	}

	// Verify Auth settings
	if config.Auth == nil {
		t.Fatal("Auth config is nil")
	}
	if config.Auth.Enable == nil || !*config.Auth.Enable {
		t.Error("Auth should be enabled")
	}
	if config.Auth.Username == nil || *config.Auth.Username != "admin" {
		t.Errorf("Auth Username = %v, want admin", StringVal(config.Auth.Username, ""))
	}
	if config.Auth.Password == nil || *config.Auth.Password != "adminpass" {
		t.Errorf("Auth Password = %v, want adminpass", StringVal(config.Auth.Password, ""))
	}

	// Verify Cloud settings
	if config.Cloud == nil {
		t.Fatal("Cloud config is nil")
	}
	if config.Cloud.Enable == nil || *config.Cloud.Enable {
		t.Error("Cloud should be disabled")
	}
	if config.Cloud.Server == nil || *config.Cloud.Server != "shelly-cloud.allterco.com:6012/jrpc" {
		t.Errorf("Cloud Server = %v, want shelly-cloud.allterco.com:6012/jrpc", StringVal(config.Cloud.Server, ""))
	}

	// Verify CoIoT settings
	if config.CoIoT == nil {
		t.Fatal("CoIoT config is nil")
	}
	if config.CoIoT.Enable == nil || !*config.CoIoT.Enable {
		t.Error("CoIoT should be enabled")
	}
	if config.CoIoT.UpdatePeriod == nil || *config.CoIoT.UpdatePeriod != 15 {
		t.Errorf("CoIoT UpdatePeriod = %v, want 15", IntVal(config.CoIoT.UpdatePeriod, 0))
	}

	// Verify System settings
	if config.System == nil || config.System.Device == nil {
		t.Fatal("System.Device config is nil")
	}
	if config.System.Device.Name == nil || *config.System.Device.Name != "Kitchen Plug" {
		t.Errorf("System.Device.Name = %v, want Kitchen Plug", StringVal(config.System.Device.Name, ""))
	}
	if config.System.Device.EcoMode == nil || *config.System.Device.EcoMode {
		t.Error("System.Device.EcoMode should be false")
	}
	if config.System.Device.Discoverable == nil || !*config.System.Device.Discoverable {
		t.Error("System.Device.Discoverable should be true")
	}

	// Verify Location settings
	if config.Location == nil {
		t.Fatal("Location config is nil")
	}
	if config.Location.Timezone == nil || *config.Location.Timezone != "America/New_York" {
		t.Errorf("Location.Timezone = %v, want America/New_York", StringVal(config.Location.Timezone, ""))
	}
	if config.Location.Latitude == nil || *config.Location.Latitude != 40.7128 {
		t.Errorf("Location.Latitude = %v, want 40.7128", config.Location.Latitude)
	}
	if config.Location.Longitude == nil || *config.Location.Longitude != -74.0060 {
		t.Errorf("Location.Longitude = %v, want -74.0060", config.Location.Longitude)
	}

	// Verify Relay settings
	if config.Relay == nil {
		t.Fatal("Relay config is nil")
	}
	if len(config.Relay.Relays) != 1 {
		t.Fatalf("Expected 1 relay, got %d", len(config.Relay.Relays))
	}
	relay := config.Relay.Relays[0]
	if relay.Name == nil || *relay.Name != "Kitchen Outlet" {
		t.Errorf("Relay.Name = %v, want Kitchen Outlet", StringVal(relay.Name, ""))
	}
	if relay.DefaultState == nil || *relay.DefaultState != "off" {
		t.Errorf("Relay.DefaultState = %v, want off", StringVal(relay.DefaultState, ""))
	}
	if relay.AutoOn == nil || *relay.AutoOn != 0 {
		t.Errorf("Relay.AutoOn = %v, want 0", IntVal(relay.AutoOn, -1))
	}
	if relay.AutoOff == nil || *relay.AutoOff != 0 {
		t.Errorf("Relay.AutoOff = %v, want 0", IntVal(relay.AutoOff, -1))
	}

	// Verify PowerMetering settings
	if config.PowerMetering == nil {
		t.Fatal("PowerMetering config is nil")
	}
	if config.PowerMetering.MaxPower == nil || *config.PowerMetering.MaxPower != 2500 {
		t.Errorf("PowerMetering.MaxPower = %v, want 2500", IntVal(config.PowerMetering.MaxPower, 0))
	}

	// Verify LED settings
	if config.LED == nil {
		t.Fatal("LED config is nil")
	}
	if config.LED.PowerIndication == nil || !*config.LED.PowerIndication {
		t.Error("LED.PowerIndication should be true (led_power_disable=false)")
	}
	if config.LED.NetworkIndication == nil || !*config.LED.NetworkIndication {
		t.Error("LED.NetworkIndication should be true (led_status_disable=false)")
	}
}

func TestGen1Converter_ToAPIConfig_SHPLGS(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	// Create DeviceConfiguration
	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable:   BoolPtr(true),
			SSID:     StringPtr("MyNetwork"),
			Password: StringPtr("mypass"),
			IPv4Mode: StringPtr("dhcp"),
		},
		MQTT: &MQTTConfiguration{
			Enable:       BoolPtr(true),
			Server:       StringPtr("192.168.1.100"),
			Port:         IntPtr(1883),
			User:         StringPtr("mqtt_user"),
			Password:     StringPtr("mqtt_pass"),
			ClientID:     StringPtr("device-123"),
			CleanSession: BoolPtr(true),
			KeepAlive:    IntPtr(60),
		},
		Auth: &AuthConfiguration{
			Enable:   BoolPtr(true),
			Username: StringPtr("admin"),
			Password: StringPtr("secret"),
		},
		Cloud: &CloudConfiguration{
			Enable: BoolPtr(false),
			Server: StringPtr("cloud.example.com"),
		},
		CoIoT: &CoIoTConfiguration{
			Enable:       BoolPtr(true),
			UpdatePeriod: IntPtr(15),
		},
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{
				Name:         StringPtr("Test Device"),
				EcoMode:      BoolPtr(false),
				Discoverable: BoolPtr(true),
			},
		},
		Location: &LocationConfiguration{
			Timezone:  StringPtr("UTC"),
			Latitude:  Float64Ptr(51.5074),
			Longitude: Float64Ptr(-0.1278),
		},
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{
					ID:           0,
					Name:         StringPtr("Main Relay"),
					DefaultState: StringPtr("off"),
					AutoOn:       IntPtr(0),
					AutoOff:      IntPtr(300),
				},
			},
		},
		PowerMetering: &PowerMeteringConfig{
			MaxPower: IntPtr(2000),
		},
		LED: &LEDConfig{
			PowerIndication:   BoolPtr(true),
			NetworkIndication: BoolPtr(false),
		},
	}

	// Convert to Gen1 API
	apiJSON, err := converter.ToAPIConfig(config, "SHPLG-S")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	// Parse result
	var gen1 map[string]interface{}
	if err := json.Unmarshal(apiJSON, &gen1); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}

	// Verify WiFi
	wifiSTA, ok := gen1["wifi_sta"].(map[string]interface{})
	if !ok {
		t.Fatal("wifi_sta not found")
	}
	if wifiSTA["enabled"] != true {
		t.Error("wifi_sta.enabled should be true")
	}
	if wifiSTA["ssid"] != "MyNetwork" {
		t.Errorf("wifi_sta.ssid = %v, want MyNetwork", wifiSTA["ssid"])
	}
	if wifiSTA["key"] != "mypass" {
		t.Errorf("wifi_sta.key = %v, want mypass", wifiSTA["key"])
	}

	// Verify MQTT server is combined
	mqttData, ok := gen1["mqtt"].(map[string]interface{})
	if !ok {
		t.Fatal("mqtt not found")
	}
	if mqttData["server"] != "192.168.1.100:1883" {
		t.Errorf("mqtt.server = %v, want 192.168.1.100:1883", mqttData["server"])
	}

	// Verify Auth
	loginData, ok := gen1["login"].(map[string]interface{})
	if !ok {
		t.Fatal("login not found")
	}
	if loginData["enabled"] != true {
		t.Error("login.enabled should be true")
	}
	if loginData["username"] != "admin" {
		t.Errorf("login.username = %v, want admin", loginData["username"])
	}

	// Verify System
	if gen1["name"] != "Test Device" {
		t.Errorf("name = %v, want Test Device", gen1["name"])
	}
	if gen1["eco_mode_enabled"] != false {
		t.Error("eco_mode_enabled should be false")
	}

	// Verify Location
	if gen1["timezone"] != "UTC" {
		t.Errorf("timezone = %v, want UTC", gen1["timezone"])
	}
	if gen1["lat"] != 51.5074 {
		t.Errorf("lat = %v, want 51.5074", gen1["lat"])
	}

	// Verify Relay
	relaysData, ok := gen1["relays"].([]interface{})
	if !ok || len(relaysData) == 0 {
		t.Fatal("relays not found or empty")
	}
	relay0, ok := relaysData[0].(map[string]interface{})
	if !ok {
		t.Fatal("relay[0] not a map")
	}
	if relay0["name"] != "Main Relay" {
		t.Errorf("relay[0].name = %v, want Main Relay", relay0["name"])
	}
	if relay0["auto_off"] != float64(300) {
		t.Errorf("relay[0].auto_off = %v, want 300", relay0["auto_off"])
	}

	// Verify PowerMetering
	if gen1["max_power"] != float64(2000) {
		t.Errorf("max_power = %v, want 2000", gen1["max_power"])
	}

	// Verify LED (inverted)
	if gen1["led_power_disable"] != false {
		t.Error("led_power_disable should be false (PowerIndication=true)")
	}
	if gen1["led_status_disable"] != true {
		t.Error("led_status_disable should be true (NetworkIndication=false)")
	}

	// Verify read-only fields are NOT present
	if _, exists := gen1["device"]; exists {
		t.Error("read-only field 'device' should not be in output")
	}
	if _, exists := gen1["hwinfo"]; exists {
		t.Error("read-only field 'hwinfo' should not be in output")
	}
	if _, exists := gen1["fw"]; exists {
		t.Error("read-only field 'fw' should not be in output")
	}
	if _, exists := gen1["time"]; exists {
		t.Error("read-only field 'time' should not be in output")
	}
}

func TestGen1Converter_RoundTrip_SHPLGS(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	// Load test fixture
	originalJSON, err := os.ReadFile("testdata/shplg_s_settings.json")
	if err != nil {
		t.Fatalf("failed to load test fixture: %v", err)
	}

	// Convert from Gen1 API to DeviceConfiguration
	config, err := converter.FromAPIConfig(originalJSON, "SHPLG-S")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	// Convert back to Gen1 API
	resultJSON, err := converter.ToAPIConfig(config, "SHPLG-S")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	// Parse both JSONs
	var original, result map[string]interface{}
	if err := json.Unmarshal(originalJSON, &original); err != nil {
		t.Fatalf("failed to parse original JSON: %v", err)
	}
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}

	// Compare key fields (read-only fields should be excluded from result)
	checkField := func(path string, originalVal, resultVal interface{}) {
		if originalVal != resultVal {
			t.Errorf("%s: original=%v, result=%v", path, originalVal, resultVal)
		}
	}

	// WiFi
	origWiFi := original["wifi_sta"].(map[string]interface{})
	resWiFi := result["wifi_sta"].(map[string]interface{})
	checkField("wifi_sta.enabled", origWiFi["enabled"], resWiFi["enabled"])
	checkField("wifi_sta.ssid", origWiFi["ssid"], resWiFi["ssid"])
	checkField("wifi_sta.key", origWiFi["key"], resWiFi["key"])

	// MQTT
	origMQTT := original["mqtt"].(map[string]interface{})
	resMQTT := result["mqtt"].(map[string]interface{})
	checkField("mqtt.enable", origMQTT["enable"], resMQTT["enable"])
	checkField("mqtt.server", origMQTT["server"], resMQTT["server"])
	checkField("mqtt.user", origMQTT["user"], resMQTT["user"])

	// System
	checkField("name", original["name"], result["name"])
	checkField("timezone", original["timezone"], result["timezone"])

	// Verify read-only fields are excluded
	if _, exists := result["device"]; exists {
		t.Error("read-only field 'device' present in round-trip result")
	}
	if _, exists := result["fw"]; exists {
		t.Error("read-only field 'fw' present in round-trip result")
	}
	if _, exists := result["hwinfo"]; exists {
		t.Error("read-only field 'hwinfo' present in round-trip result")
	}
	if _, exists := result["time"]; exists {
		t.Error("read-only field 'time' present in round-trip result")
	}
	if _, exists := result["unixtime"]; exists {
		t.Error("read-only field 'unixtime' present in round-trip result")
	}
}

func TestGen1Converter_SupportedDeviceTypes(t *testing.T) {
	converter := NewGen1Converter(nil)
	types := converter.SupportedDeviceTypes()

	expectedTypes := []string{"SHPLG-S", "SHPLG-1", "SHSW-1", "SHSW-PM", "SHSW-25", "SHIX3-1"}
	if len(types) != len(expectedTypes) {
		t.Errorf("Expected %d device types, got %d", len(expectedTypes), len(types))
	}

	for _, expected := range expectedTypes {
		found := false
		for _, actual := range types {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected device type %s not found", expected)
		}
	}
}

func TestGen1Converter_Generation(t *testing.T) {
	converter := NewGen1Converter(nil)
	if converter.Generation() != 1 {
		t.Errorf("Generation() = %d, want 1", converter.Generation())
	}
}

func TestGen1Converter_FromAPIConfig_SHSW1(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	fixture, err := os.ReadFile("testdata/shsw_1_settings.json")
	if err != nil {
		t.Fatalf("failed to load test fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(fixture, "SHSW-1")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	if config.System == nil || config.System.Device == nil {
		t.Fatal("System.Device is nil")
	}
	if StringVal(config.System.Device.Name, "") != "Garage Light" {
		t.Errorf("Name = %v, want Garage Light", StringVal(config.System.Device.Name, ""))
	}

	if config.Relay == nil || len(config.Relay.Relays) != 1 {
		t.Fatalf("Expected 1 relay, got %v", config.Relay)
	}

	if config.Input == nil || len(config.Input.Inputs) != 1 {
		t.Fatalf("Expected 1 input, got %v", config.Input)
	}

	if config.PowerMetering != nil {
		t.Error("SHSW-1 should NOT have PowerMetering")
	}
	if config.LED != nil {
		t.Error("SHSW-1 should NOT have LED config")
	}
}

func TestGen1Converter_ToAPIConfig_SHSW1(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	config := &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{Name: StringPtr("Test SHSW-1")},
		},
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{{ID: 0, Name: StringPtr("Test Relay")}},
		},
		Input: &InputConfig{
			Inputs: []SingleInputConfig{{ID: 0, Name: StringPtr("Test Input")}},
		},
		PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(1000)},
		LED:           &LEDConfig{PowerIndication: BoolPtr(true)},
	}

	apiJSON, err := converter.ToAPIConfig(config, "SHSW-1")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var gen1 map[string]interface{}
	if err := json.Unmarshal(apiJSON, &gen1); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if _, exists := gen1["max_power"]; exists {
		t.Error("SHSW-1 should NOT have max_power")
	}
	if _, exists := gen1["led_power_disable"]; exists {
		t.Error("SHSW-1 should NOT have LED settings")
	}
}

func TestGen1Converter_RoundTrip_SHSW1(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	originalJSON, err := os.ReadFile("testdata/shsw_1_settings.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(originalJSON, "SHSW-1")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	resultJSON, err := converter.ToAPIConfig(config, "SHSW-1")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if result["name"] != "Garage Light" {
		t.Errorf("name = %v, want Garage Light", result["name"])
	}
}

func TestGen1Converter_FromAPIConfig_SHSWPM(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	fixture, err := os.ReadFile("testdata/shsw_pm_settings.json")
	if err != nil {
		t.Fatalf("failed to load test fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(fixture, "SHSW-PM")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	if config.PowerMetering == nil {
		t.Fatal("SHSW-PM should have PowerMetering")
	}
	if IntVal(config.PowerMetering.MaxPower, 0) != 3500 {
		t.Errorf("MaxPower = %v, want 3500", IntVal(config.PowerMetering.MaxPower, 0))
	}

	if config.LED != nil {
		t.Error("SHSW-PM should NOT have LED config")
	}
}

func TestGen1Converter_ToAPIConfig_SHSWPM(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	config := &DeviceConfiguration{
		PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(2000)},
		LED:           &LEDConfig{PowerIndication: BoolPtr(true)},
	}

	apiJSON, err := converter.ToAPIConfig(config, "SHSW-PM")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var gen1 map[string]interface{}
	if err := json.Unmarshal(apiJSON, &gen1); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if gen1["max_power"] != float64(2000) {
		t.Errorf("max_power = %v, want 2000", gen1["max_power"])
	}
	if _, exists := gen1["led_power_disable"]; exists {
		t.Error("SHSW-PM should NOT have LED settings")
	}
}

func TestGen1Converter_RoundTrip_SHSWPM(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	originalJSON, err := os.ReadFile("testdata/shsw_pm_settings.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(originalJSON, "SHSW-PM")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	resultJSON, err := converter.ToAPIConfig(config, "SHSW-PM")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if result["max_power"] != float64(3500) {
		t.Errorf("max_power = %v, want 3500", result["max_power"])
	}
}

func TestGen1Converter_FromAPIConfig_SHSW25(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	fixture, err := os.ReadFile("testdata/shsw_25_settings.json")
	if err != nil {
		t.Fatalf("failed to load test fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(fixture, "SHSW-25")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	if config.Relay == nil || len(config.Relay.Relays) != 2 {
		t.Fatalf("Expected 2 relays, got %d", len(config.Relay.Relays))
	}

	if config.Input == nil || len(config.Input.Inputs) != 2 {
		t.Fatalf("Expected 2 inputs, got %d", len(config.Input.Inputs))
	}

	if config.PowerMetering == nil {
		t.Fatal("SHSW-25 should have PowerMetering")
	}

	if config.LED != nil {
		t.Error("SHSW-25 should NOT have LED config")
	}
}

func TestGen1Converter_ToAPIConfig_SHSW25(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	config := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("Relay 1")},
				{ID: 1, Name: StringPtr("Relay 2")},
			},
		},
		Input: &InputConfig{
			Inputs: []SingleInputConfig{
				{ID: 0, Name: StringPtr("Input 1")},
				{ID: 1, Name: StringPtr("Input 2")},
			},
		},
		PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(4000)},
	}

	apiJSON, err := converter.ToAPIConfig(config, "SHSW-25")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var gen1 map[string]interface{}
	if err := json.Unmarshal(apiJSON, &gen1); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	relays := gen1["relays"].([]interface{})
	if len(relays) != 2 {
		t.Errorf("Expected 2 relays, got %d", len(relays))
	}

	inputs := gen1["inputs"].([]interface{})
	if len(inputs) != 2 {
		t.Errorf("Expected 2 inputs, got %d", len(inputs))
	}
}

func TestGen1Converter_RoundTrip_SHSW25(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	originalJSON, err := os.ReadFile("testdata/shsw_25_settings.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(originalJSON, "SHSW-25")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	resultJSON, err := converter.ToAPIConfig(config, "SHSW-25")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	relays := result["relays"].([]interface{})
	if len(relays) != 2 {
		t.Errorf("Expected 2 relays in round-trip")
	}
}

func TestGen1Converter_FromAPIConfig_SHIX31(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	fixture, err := os.ReadFile("testdata/shix3_1_settings.json")
	if err != nil {
		t.Fatalf("failed to load test fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(fixture, "SHIX3-1")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	if config.Input == nil || len(config.Input.Inputs) != 3 {
		t.Fatalf("Expected 3 inputs, got %v", config.Input)
	}

	if config.Relay != nil && len(config.Relay.Relays) > 0 {
		t.Error("SHIX3-1 should NOT have relays")
	}
	if config.PowerMetering != nil {
		t.Error("SHIX3-1 should NOT have PowerMetering")
	}
	if config.LED != nil {
		t.Error("SHIX3-1 should NOT have LED")
	}
}

func TestGen1Converter_ToAPIConfig_SHIX31(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	config := &DeviceConfiguration{
		Input: &InputConfig{
			Inputs: []SingleInputConfig{
				{ID: 0, Name: StringPtr("Button 1")},
				{ID: 1, Name: StringPtr("Button 2")},
				{ID: 2, Name: StringPtr("Button 3")},
			},
		},
		Relay:         &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, Name: StringPtr("Fake")}}},
		PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(1000)},
		LED:           &LEDConfig{PowerIndication: BoolPtr(true)},
	}

	apiJSON, err := converter.ToAPIConfig(config, "SHIX3-1")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var gen1 map[string]interface{}
	if err := json.Unmarshal(apiJSON, &gen1); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	inputs := gen1["inputs"].([]interface{})
	if len(inputs) != 3 {
		t.Errorf("Expected 3 inputs, got %d", len(inputs))
	}

	if _, exists := gen1["relays"]; exists {
		t.Error("SHIX3-1 should NOT have relays in output")
	}
	if _, exists := gen1["max_power"]; exists {
		t.Error("SHIX3-1 should NOT have max_power")
	}
}

func TestGen1Converter_RoundTrip_SHIX31(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	converter := NewGen1Converter(logger)

	originalJSON, err := os.ReadFile("testdata/shix3_1_settings.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	config, err := converter.FromAPIConfig(originalJSON, "SHIX3-1")
	if err != nil {
		t.Fatalf("FromAPIConfig failed: %v", err)
	}

	resultJSON, err := converter.ToAPIConfig(config, "SHIX3-1")
	if err != nil {
		t.Fatalf("ToAPIConfig failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	inputs := result["inputs"].([]interface{})
	if len(inputs) != 3 {
		t.Errorf("Expected 3 inputs in round-trip, got %d", len(inputs))
	}

	if _, exists := result["relays"]; exists {
		t.Error("relays should be excluded for SHIX3-1")
	}
}

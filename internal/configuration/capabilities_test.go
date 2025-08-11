package configuration

import (
	"encoding/json"
	"testing"
	"time"
)

// TestBaseDeviceConfig tests the base device configuration
func TestBaseDeviceConfig(t *testing.T) {
	config := BaseDeviceConfig{
		DeviceID:   "shelly1-123456",
		DeviceType: "SHSW-1",
		Name:       "Test Switch",
		Generation: 1,
		WiFi: WiFiConfig{
			SSID:     "TestNetwork",
			Password: "secret123",
			IP:       "192.168.1.100",
			Netmask:  "255.255.255.0",
			Gateway:  "192.168.1.1",
			DNS:      "8.8.8.8",
			DHCP:     false,
		},
		Auth: AuthConfig{
			Enabled:  true,
			Username: "admin",
			Password: "password",
		},
		Cloud: CloudConfig{
			Enabled: true,
			Server:  "cloud.shelly.com",
		},
		MQTT: MQTTConfig{
			Enabled:        true,
			Server:         "mqtt.local",
			Port:           1883,
			User:           "mqtt_user",
			Password:       "mqtt_pass",
			ClientID:       "shelly1-123456",
			TopicPrefix:    "shellies/shelly1-123456",
			CleanSession:   true,
			RetainMessages: false,
			QoS:            1,
			KeepAlive:      60,
		},
		Timezone: "Europe/Berlin",
		Location: &Location{
			Latitude:  52.520008,
			Longitude: 13.404954,
		},
		LastModified: time.Now(),
		Version:      1,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal BaseDeviceConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded BaseDeviceConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal BaseDeviceConfig: %v", err)
	}

	// Verify fields
	if decoded.DeviceID != config.DeviceID {
		t.Errorf("DeviceID mismatch: got %s, want %s", decoded.DeviceID, config.DeviceID)
	}
	if decoded.WiFi.SSID != config.WiFi.SSID {
		t.Errorf("WiFi SSID mismatch: got %s, want %s", decoded.WiFi.SSID, config.WiFi.SSID)
	}
	if decoded.Auth.Username != config.Auth.Username {
		t.Errorf("Auth Username mismatch: got %s, want %s", decoded.Auth.Username, config.Auth.Username)
	}
	if decoded.MQTT.Server != config.MQTT.Server {
		t.Errorf("MQTT Server mismatch: got %s, want %s", decoded.MQTT.Server, config.MQTT.Server)
	}
	if decoded.Location == nil {
		t.Error("Location should not be nil")
	} else if decoded.Location.Latitude != config.Location.Latitude {
		t.Errorf("Location Latitude mismatch: got %f, want %f", decoded.Location.Latitude, config.Location.Latitude)
	}
}

// TestWiFiConfig tests WiFi configuration
func TestWiFiConfig(t *testing.T) {
	tests := []struct {
		name   string
		config WiFiConfig
	}{
		{
			name: "DHCP enabled",
			config: WiFiConfig{
				SSID:     "TestNetwork",
				Password: "secret",
				DHCP:     true,
			},
		},
		{
			name: "Static IP",
			config: WiFiConfig{
				SSID:     "TestNetwork",
				Password: "secret",
				IP:       "192.168.1.100",
				Netmask:  "255.255.255.0",
				Gateway:  "192.168.1.1",
				DNS:      "8.8.8.8",
				DHCP:     false,
			},
		},
		{
			name: "Open network",
			config: WiFiConfig{
				SSID: "OpenNetwork",
				DHCP: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatalf("Failed to marshal WiFiConfig: %v", err)
			}

			var decoded WiFiConfig
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal WiFiConfig: %v", err)
			}

			if decoded.SSID != tt.config.SSID {
				t.Errorf("SSID mismatch: got %s, want %s", decoded.SSID, tt.config.SSID)
			}
			if decoded.DHCP != tt.config.DHCP {
				t.Errorf("DHCP mismatch: got %v, want %v", decoded.DHCP, tt.config.DHCP)
			}
			if !tt.config.DHCP && decoded.IP != tt.config.IP {
				t.Errorf("IP mismatch: got %s, want %s", decoded.IP, tt.config.IP)
			}
		})
	}
}

// TestAuthConfig tests authentication configuration
func TestAuthConfig(t *testing.T) {
	tests := []struct {
		name   string
		config AuthConfig
	}{
		{
			name: "Basic auth",
			config: AuthConfig{
				Enabled:  true,
				Username: "admin",
				Password: "secret",
			},
		},
		{
			name: "Digest auth with realm",
			config: AuthConfig{
				Enabled:  true,
				Username: "admin",
				Password: "secret",
				Realm:    "shelly",
			},
		},
		{
			name: "Disabled auth",
			config: AuthConfig{
				Enabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatalf("Failed to marshal AuthConfig: %v", err)
			}

			var decoded AuthConfig
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal AuthConfig: %v", err)
			}

			if decoded.Enabled != tt.config.Enabled {
				t.Errorf("Enabled mismatch: got %v, want %v", decoded.Enabled, tt.config.Enabled)
			}
			if decoded.Username != tt.config.Username {
				t.Errorf("Username mismatch: got %s, want %s", decoded.Username, tt.config.Username)
			}
			if decoded.Realm != tt.config.Realm {
				t.Errorf("Realm mismatch: got %s, want %s", decoded.Realm, tt.config.Realm)
			}
		})
	}
}

// TestMQTTConfig tests MQTT configuration
func TestMQTTConfig(t *testing.T) {
	config := MQTTConfig{
		Enabled:        true,
		Server:         "mqtt.broker.com",
		Port:           1883,
		User:           "mqtt_user",
		Password:       "mqtt_pass",
		ClientID:       "shelly-123",
		TopicPrefix:    "shellies/device",
		CleanSession:   true,
		RetainMessages: false,
		QoS:            1,
		KeepAlive:      60,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal MQTTConfig: %v", err)
	}

	// Test JSON unmarshaling
	var decoded MQTTConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal MQTTConfig: %v", err)
	}

	// Verify all fields
	if decoded.Enabled != config.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", decoded.Enabled, config.Enabled)
	}
	if decoded.Server != config.Server {
		t.Errorf("Server mismatch: got %s, want %s", decoded.Server, config.Server)
	}
	if decoded.Port != config.Port {
		t.Errorf("Port mismatch: got %d, want %d", decoded.Port, config.Port)
	}
	if decoded.User != config.User {
		t.Errorf("User mismatch: got %s, want %s", decoded.User, config.User)
	}
	if decoded.ClientID != config.ClientID {
		t.Errorf("ClientID mismatch: got %s, want %s", decoded.ClientID, config.ClientID)
	}
	if decoded.TopicPrefix != config.TopicPrefix {
		t.Errorf("TopicPrefix mismatch: got %s, want %s", decoded.TopicPrefix, config.TopicPrefix)
	}
	if decoded.CleanSession != config.CleanSession {
		t.Errorf("CleanSession mismatch: got %v, want %v", decoded.CleanSession, config.CleanSession)
	}
	if decoded.RetainMessages != config.RetainMessages {
		t.Errorf("RetainMessages mismatch: got %v, want %v", decoded.RetainMessages, config.RetainMessages)
	}
	if decoded.QoS != config.QoS {
		t.Errorf("QoS mismatch: got %d, want %d", decoded.QoS, config.QoS)
	}
	if decoded.KeepAlive != config.KeepAlive {
		t.Errorf("KeepAlive mismatch: got %d, want %d", decoded.KeepAlive, config.KeepAlive)
	}
}

// TestCloudConfig tests cloud configuration
func TestCloudConfig(t *testing.T) {
	tests := []struct {
		name   string
		config CloudConfig
	}{
		{
			name: "Cloud enabled",
			config: CloudConfig{
				Enabled: true,
				Server:  "cloud.shelly.com",
			},
		},
		{
			name: "Cloud disabled",
			config: CloudConfig{
				Enabled: false,
			},
		},
		{
			name: "Custom cloud server",
			config: CloudConfig{
				Enabled: true,
				Server:  "custom.cloud.server",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatalf("Failed to marshal CloudConfig: %v", err)
			}

			var decoded CloudConfig
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal CloudConfig: %v", err)
			}

			if decoded.Enabled != tt.config.Enabled {
				t.Errorf("Enabled mismatch: got %v, want %v", decoded.Enabled, tt.config.Enabled)
			}
			if decoded.Server != tt.config.Server {
				t.Errorf("Server mismatch: got %s, want %s", decoded.Server, tt.config.Server)
			}
		})
	}
}

// TestLocation tests location configuration
func TestLocation(t *testing.T) {
	tests := []struct {
		name     string
		location Location
	}{
		{
			name: "Berlin",
			location: Location{
				Latitude:  52.520008,
				Longitude: 13.404954,
			},
		},
		{
			name: "New York",
			location: Location{
				Latitude:  40.712776,
				Longitude: -74.005974,
			},
		},
		{
			name: "Sydney",
			location: Location{
				Latitude:  -33.868820,
				Longitude: 151.209290,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.location)
			if err != nil {
				t.Fatalf("Failed to marshal Location: %v", err)
			}

			var decoded Location
			err = json.Unmarshal(data, &decoded)
			if err != nil {
				t.Fatalf("Failed to unmarshal Location: %v", err)
			}

			if decoded.Latitude != tt.location.Latitude {
				t.Errorf("Latitude mismatch: got %f, want %f", decoded.Latitude, tt.location.Latitude)
			}
			if decoded.Longitude != tt.location.Longitude {
				t.Errorf("Longitude mismatch: got %f, want %f", decoded.Longitude, tt.location.Longitude)
			}
		})
	}
}

// Mock device that implements capabilities
type mockDevice struct {
	BaseDeviceConfig
	relay *RelayConfig
	power *PowerMeteringConfig
}

func (m *mockDevice) GetDeviceType() string { return m.DeviceType }
func (m *mockDevice) GetGeneration() int { return m.Generation }
func (m *mockDevice) GetModel() string { return m.DeviceType }
func (m *mockDevice) HasRelayCapability() bool { return m.relay != nil }
func (m *mockDevice) HasPowerMeteringCapability() bool { return m.power != nil }
func (m *mockDevice) HasDimmingCapability() bool { return false }
func (m *mockDevice) HasRollerCapability() bool { return false }
func (m *mockDevice) HasInputCapability() bool { return false }
func (m *mockDevice) HasLEDCapability() bool { return false }
func (m *mockDevice) HasColorControlCapability() bool { return false }
func (m *mockDevice) HasTemperatureProtectionCapability() bool { return false }
func (m *mockDevice) HasScheduleCapability() bool { return false }
func (m *mockDevice) HasCoIoTCapability() bool { return false }
func (m *mockDevice) HasEnergyMeterCapability() bool { return false }
func (m *mockDevice) HasMotionCapability() bool { return false }
func (m *mockDevice) HasSensorCapability() bool { return false }
func (m *mockDevice) GetRelayCount() int { 
	if m.relay != nil {
		return len(m.relay.Relays)
	}
	return 0
}
func (m *mockDevice) GetInputCount() int { return 0 }
func (m *mockDevice) GetRollerCount() int { return 0 }
func (m *mockDevice) GetRelayConfig() *RelayConfig { return m.relay }
func (m *mockDevice) SetRelayConfig(c *RelayConfig) { m.relay = c }
func (m *mockDevice) GetPowerMeteringConfig() *PowerMeteringConfig { return m.power }
func (m *mockDevice) SetPowerMeteringConfig(c *PowerMeteringConfig) { m.power = c }

// TestDeviceCapabilities tests the DeviceCapabilities interface
func TestDeviceCapabilities(t *testing.T) {
	autoOn := 300
	maxPower := 2300
	
	device := &mockDevice{
		BaseDeviceConfig: BaseDeviceConfig{
			DeviceID:   "shelly1pm-123456",
			DeviceType: "SHSW-PM",
			Name:       "Test Switch PM",
			Generation: 1,
		},
		relay: &RelayConfig{
			DefaultState: "last",
			ButtonType:   "toggle",
			AutoOn:       &autoOn,
			AutoOff:      nil,
			HasTimer:     true,
			Relays: []SingleRelayConfig{
				{
					ID:           0,
					Name:         "Relay 0",
					DefaultState: "off",
					Schedule:     true,
				},
			},
		},
		power: &PowerMeteringConfig{
			MaxPower:         &maxPower,
			ProtectionAction: "off",
			PowerCorrection:  1.0,
			ReportingPeriod:  60,
		},
	}

	// Test capability queries
	if device.GetDeviceType() != "SHSW-PM" {
		t.Errorf("GetDeviceType() = %s, want SHSW-PM", device.GetDeviceType())
	}
	if device.GetGeneration() != 1 {
		t.Errorf("GetGeneration() = %d, want 1", device.GetGeneration())
	}
	if !device.HasRelayCapability() {
		t.Error("Device should have relay capability")
	}
	if !device.HasPowerMeteringCapability() {
		t.Error("Device should have power metering capability")
	}
	if device.HasDimmingCapability() {
		t.Error("Device should not have dimming capability")
	}
	if device.GetRelayCount() != 1 {
		t.Errorf("GetRelayCount() = %d, want 1", device.GetRelayCount())
	}

	// Test HasRelay interface
	var hasRelay HasRelay = device
	relayConfig := hasRelay.GetRelayConfig()
	if relayConfig == nil {
		t.Fatal("GetRelayConfig() returned nil")
	}
	if relayConfig.DefaultState != "last" {
		t.Errorf("RelayConfig.DefaultState = %s, want last", relayConfig.DefaultState)
	}

	// Test HasPowerMetering interface
	var hasPower HasPowerMetering = device
	powerConfig := hasPower.GetPowerMeteringConfig()
	if powerConfig == nil {
		t.Fatal("GetPowerMeteringConfig() returned nil")
	}
	if *powerConfig.MaxPower != 2300 {
		t.Errorf("PowerConfig.MaxPower = %d, want 2300", *powerConfig.MaxPower)
	}

	// Test setting configs
	newAutoOn := 600
	hasRelay.SetRelayConfig(&RelayConfig{
		DefaultState: "on",
		AutoOn:       &newAutoOn,
	})
	updatedRelay := hasRelay.GetRelayConfig()
	if updatedRelay.DefaultState != "on" {
		t.Errorf("Updated RelayConfig.DefaultState = %s, want on", updatedRelay.DefaultState)
	}
	if *updatedRelay.AutoOn != 600 {
		t.Errorf("Updated RelayConfig.AutoOn = %d, want 600", *updatedRelay.AutoOn)
	}
}
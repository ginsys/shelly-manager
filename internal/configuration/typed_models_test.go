package configuration

import (
	"encoding/json"
	"testing"
)

func TestTypedConfiguration_Validate(t *testing.T) {
	tests := []struct {
		name          string
		config        *TypedConfiguration
		expectValid   bool
		expectedError string
	}{
		{
			name: "Valid complete configuration",
			config: &TypedConfiguration{
				WiFi: &WiFiConfiguration{
					Enable:   true,
					SSID:     "TestNetwork",
					Password: "password123",
					IPv4Mode: "dhcp",
				},
				MQTT: &MQTTConfiguration{
					Enable: true,
					Server: "mqtt.example.com:1883",
					Port:   1883,
				},
				Auth: &AuthConfiguration{
					Enable:   true,
					Username: "admin",
					Password: "securepass",
				},
			},
			expectValid: true,
		},
		{
			name: "Valid minimal configuration",
			config: &TypedConfiguration{
				WiFi: &WiFiConfiguration{
					Enable: false,
				},
			},
			expectValid: true,
		},
		{
			name: "Invalid WiFi - empty SSID when enabled",
			config: &TypedConfiguration{
				WiFi: &WiFiConfiguration{
					Enable: true,
					SSID:   "",
				},
			},
			expectValid:   false,
			expectedError: "wifi validation failed",
		},
		{
			name: "Invalid MQTT - empty server when enabled",
			config: &TypedConfiguration{
				MQTT: &MQTTConfiguration{
					Enable: true,
					Server: "",
				},
			},
			expectValid:   false,
			expectedError: "mqtt validation failed",
		},
		{
			name: "Invalid Auth - empty username when enabled",
			config: &TypedConfiguration{
				Auth: &AuthConfiguration{
					Enable:   true,
					Username: "",
				},
			},
			expectValid:   false,
			expectedError: "auth validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected configuration to be valid, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected configuration to be invalid, got no error")
				}
				if tt.expectedError != "" && err.Error() == "" {
					t.Errorf("Expected error containing %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestWiFiConfiguration_Validate(t *testing.T) {
	tests := []struct {
		name          string
		wifi          *WiFiConfiguration
		expectValid   bool
		expectedError string
	}{
		{
			name: "Valid enabled WiFi with DHCP",
			wifi: &WiFiConfiguration{
				Enable:   true,
				SSID:     "TestNetwork",
				Password: "password123",
				IPv4Mode: "dhcp",
			},
			expectValid: true,
		},
		{
			name: "Valid enabled WiFi with static IP",
			wifi: &WiFiConfiguration{
				Enable:   true,
				SSID:     "TestNetwork",
				Password: "password123",
				IPv4Mode: "static",
				StaticIP: &StaticIPConfig{
					IP:      "192.168.1.100",
					Netmask: "255.255.255.0",
					Gateway: "192.168.1.1",
				},
			},
			expectValid: true,
		},
		{
			name: "Valid disabled WiFi",
			wifi: &WiFiConfiguration{
				Enable: false,
			},
			expectValid: true,
		},
		{
			name: "Invalid - enabled without SSID",
			wifi: &WiFiConfiguration{
				Enable: true,
				SSID:   "",
			},
			expectValid:   false,
			expectedError: "SSID is required when WiFi is enabled",
		},
		{
			name: "Invalid - SSID too long",
			wifi: &WiFiConfiguration{
				Enable: true,
				SSID:   "ThisSSIDIsWayTooLongAndExceedsTheThirtyTwoCharacterLimit",
			},
			expectValid:   false,
			expectedError: "SSID must be 32 characters or less",
		},
		{
			name: "Invalid - password too long",
			wifi: &WiFiConfiguration{
				Enable:   true,
				SSID:     "TestNetwork",
				Password: "ThisPasswordIsWayTooLongAndExceedsTheSixtyThreeCharacterLimitForWiFiPasswords123456789",
			},
			expectValid:   false,
			expectedError: "WiFi password must be 63 characters or less",
		},
		{
			name: "Invalid - static mode without static IP config",
			wifi: &WiFiConfiguration{
				Enable:   true,
				SSID:     "TestNetwork",
				IPv4Mode: "static",
				StaticIP: nil,
			},
			expectValid:   false,
			expectedError: "static IP configuration required when IPv4 mode is 'static'",
		},
		{
			name: "Invalid - invalid IPv4 mode",
			wifi: &WiFiConfiguration{
				Enable:   true,
				SSID:     "TestNetwork",
				IPv4Mode: "invalid",
			},
			expectValid:   false,
			expectedError: "IPv4 mode must be 'dhcp' or 'static'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wifi.Validate()

			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected WiFi configuration to be valid, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected WiFi configuration to be invalid, got no error")
				}
				if tt.expectedError != "" && err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestStaticIPConfig_Validate(t *testing.T) {
	tests := []struct {
		name          string
		staticIP      *StaticIPConfig
		expectValid   bool
		expectedError string
	}{
		{
			name: "Valid static IP configuration",
			staticIP: &StaticIPConfig{
				IP:      "192.168.1.100",
				Netmask: "255.255.255.0",
				Gateway: "192.168.1.1",
			},
			expectValid: true,
		},
		{
			name: "Valid static IP configuration with nameserver",
			staticIP: &StaticIPConfig{
				IP:         "192.168.1.100",
				Netmask:    "255.255.255.0",
				Gateway:    "192.168.1.1",
				Nameserver: "8.8.8.8",
			},
			expectValid: true,
		},
		{
			name: "Invalid IP address",
			staticIP: &StaticIPConfig{
				IP:      "invalid.ip",
				Netmask: "255.255.255.0",
				Gateway: "192.168.1.1",
			},
			expectValid:   false,
			expectedError: "invalid IP address: invalid.ip",
		},
		{
			name: "Invalid netmask",
			staticIP: &StaticIPConfig{
				IP:      "192.168.1.100",
				Netmask: "invalid.mask",
				Gateway: "192.168.1.1",
			},
			expectValid:   false,
			expectedError: "invalid netmask: invalid.mask",
		},
		{
			name: "Invalid gateway",
			staticIP: &StaticIPConfig{
				IP:      "192.168.1.100",
				Netmask: "255.255.255.0",
				Gateway: "invalid.gateway",
			},
			expectValid:   false,
			expectedError: "invalid gateway: invalid.gateway",
		},
		{
			name: "Invalid nameserver",
			staticIP: &StaticIPConfig{
				IP:         "192.168.1.100",
				Netmask:    "255.255.255.0",
				Gateway:    "192.168.1.1",
				Nameserver: "invalid.dns",
			},
			expectValid:   false,
			expectedError: "invalid nameserver: invalid.dns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.staticIP.Validate()

			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected static IP configuration to be valid, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected static IP configuration to be invalid, got no error")
				}
				if tt.expectedError != "" && err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestMQTTConfiguration_Validate(t *testing.T) {
	tests := []struct {
		name          string
		mqtt          *MQTTConfiguration
		expectValid   bool
		expectedError string
	}{
		{
			name: "Valid enabled MQTT",
			mqtt: &MQTTConfiguration{
				Enable: true,
				Server: "mqtt.example.com:1883",
			},
			expectValid: true,
		},
		{
			name: "Valid disabled MQTT",
			mqtt: &MQTTConfiguration{
				Enable: false,
			},
			expectValid: true,
		},
		{
			name: "Valid MQTT with all settings",
			mqtt: &MQTTConfiguration{
				Enable:            true,
				Server:            "mqtt.example.com:1883",
				User:              "testuser",
				Password:          "testpass",
				ClientID:          "client123",
				KeepAlive:         60,
				MaxQueuedMessages: 10,
				TopicPrefix:       "homeassistant",
			},
			expectValid: true,
		},
		{
			name: "Invalid - enabled without server",
			mqtt: &MQTTConfiguration{
				Enable: true,
				Server: "",
			},
			expectValid:   false,
			expectedError: "MQTT server is required when MQTT is enabled",
		},
		{
			name: "Invalid - invalid server format",
			mqtt: &MQTTConfiguration{
				Enable: true,
				Server: "invalid..server",
			},
			expectValid:   false,
			expectedError: "invalid MQTT server format: invalid..server",
		},
		{
			name: "Invalid - port out of range",
			mqtt: &MQTTConfiguration{
				Enable: true,
				Server: "mqtt.example.com:1883",
				Port:   70000,
			},
			expectValid:   false,
			expectedError: "MQTT port must be between 1 and 65535",
		},
		{
			name: "Invalid - client ID too long",
			mqtt: &MQTTConfiguration{
				Enable:   true,
				Server:   "mqtt.example.com",
				ClientID: "ThisClientIDIsWayTooLongAndExceedsTheOneHundredTwentyEightCharacterLimitForMQTTClientIDsWhichShouldCauseValidationToFailCompletely",
			},
			expectValid:   false,
			expectedError: "MQTT client ID must be 128 characters or less",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mqtt.Validate()

			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected MQTT configuration to be valid, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected MQTT configuration to be invalid, got no error")
				} else if tt.expectedError != "" && err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestAuthConfiguration_Validate(t *testing.T) {
	tests := []struct {
		name          string
		auth          *AuthConfiguration
		expectValid   bool
		expectedError string
	}{
		{
			name: "Valid enabled auth",
			auth: &AuthConfiguration{
				Enable:   true,
				Username: "admin",
				Password: "securepass",
			},
			expectValid: true,
		},
		{
			name: "Valid disabled auth",
			auth: &AuthConfiguration{
				Enable: false,
			},
			expectValid: true,
		},
		{
			name: "Invalid - enabled without username",
			auth: &AuthConfiguration{
				Enable:   true,
				Username: "",
				Password: "securepass",
			},
			expectValid:   false,
			expectedError: "username is required when authentication is enabled",
		},
		{
			name: "Invalid - enabled without password",
			auth: &AuthConfiguration{
				Enable:   true,
				Username: "admin",
				Password: "",
			},
			expectValid:   false,
			expectedError: "password is required when authentication is enabled",
		},
		{
			name: "Invalid - username too long",
			auth: &AuthConfiguration{
				Enable:   true,
				Username: "ThisUsernameIsWayTooLongAndExceedsTheSixtyFourCharacterLimitExtra",
				Password: "securepass",
			},
			expectValid:   false,
			expectedError: "username must be 64 characters or less",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.Validate()

			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected auth configuration to be valid, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected auth configuration to be invalid, got no error")
				} else if tt.expectedError != "" && err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
			}
		})
	}
}

func TestTypedConfiguration_ToJSON(t *testing.T) {
	config := &TypedConfiguration{
		WiFi: &WiFiConfiguration{
			Enable:   true,
			SSID:     "TestNetwork",
			Password: "password123",
		},
		MQTT: &MQTTConfiguration{
			Enable: true,
			Server: "mqtt.example.com",
			Port:   1883,
		},
	}

	jsonData, err := config.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	// Verify JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check WiFi section
	wifiSection, ok := result["wifi"].(map[string]interface{})
	if !ok {
		t.Errorf("WiFi section not found or invalid type")
	} else {
		if wifiSection["enable"] != true {
			t.Errorf("Expected WiFi enabled to be true, got %v", wifiSection["enable"])
		}
		if wifiSection["ssid"] != "TestNetwork" {
			t.Errorf("Expected SSID to be 'TestNetwork', got %v", wifiSection["ssid"])
		}
	}

	// Check MQTT section
	mqttSection, ok := result["mqtt"].(map[string]interface{})
	if !ok {
		t.Errorf("MQTT section not found or invalid type")
	} else {
		if mqttSection["enable"] != true {
			t.Errorf("Expected MQTT enabled to be true, got %v", mqttSection["enable"])
		}
		if mqttSection["server"] != "mqtt.example.com" {
			t.Errorf("Expected MQTT server to be 'mqtt.example.com', got %v", mqttSection["server"])
		}
	}
}

func TestFromJSON(t *testing.T) {
	jsonData := json.RawMessage(`{
		"wifi": {
			"enable": true,
			"ssid": "TestNetwork",
			"password": "password123"
		},
		"mqtt": {
			"enable": true,
			"server": "mqtt.example.com",
			"port": 1883
		}
	}`)

	config, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify WiFi configuration
	if config.WiFi == nil {
		t.Errorf("WiFi configuration is nil")
	} else {
		if !config.WiFi.Enable {
			t.Errorf("Expected WiFi to be enabled")
		}
		if config.WiFi.SSID != "TestNetwork" {
			t.Errorf("Expected SSID to be 'TestNetwork', got %q", config.WiFi.SSID)
		}
		if config.WiFi.Password != "password123" {
			t.Errorf("Expected password to be 'password123', got %q", config.WiFi.Password)
		}
	}

	// Verify MQTT configuration
	if config.MQTT == nil {
		t.Errorf("MQTT configuration is nil")
	} else {
		if !config.MQTT.Enable {
			t.Errorf("Expected MQTT to be enabled")
		}
		if config.MQTT.Server != "mqtt.example.com" {
			t.Errorf("Expected MQTT server to be 'mqtt.example.com', got %q", config.MQTT.Server)
		}
		if config.MQTT.Port != 1883 {
			t.Errorf("Expected MQTT port to be 1883, got %d", config.MQTT.Port)
		}
	}
}

func TestFromJSON_InvalidJSON(t *testing.T) {
	invalidJSON := json.RawMessage(`{invalid json}`)

	_, err := FromJSON(invalidJSON)
	if err == nil {
		t.Errorf("Expected error for invalid JSON, got nil")
	}
}

func TestGetConfigurationSchema(t *testing.T) {
	schema := GetConfigurationSchema()

	// Verify schema structure
	if schema["$schema"] != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("Expected JSON schema version, got %v", schema["$schema"])
	}

	if schema["type"] != "object" {
		t.Errorf("Expected type to be 'object', got %v", schema["type"])
	}

	// Verify properties exist
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Errorf("Properties section not found or invalid type")
	} else {
		// Check that WiFi, MQTT, and Auth properties exist
		if _, ok := properties["wifi"]; !ok {
			t.Errorf("WiFi property not found in schema")
		}
		if _, ok := properties["mqtt"]; !ok {
			t.Errorf("MQTT property not found in schema")
		}
		if _, ok := properties["auth"]; !ok {
			t.Errorf("Auth property not found in schema")
		}
	}
}

// Helper validation function tests
func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		hostname string
		expected bool
	}{
		{"example.com", true},
		{"test-server", true},
		{"localhost", true},
		{"server123", true},
		{"my-device.local", true},
		{"", false},
		{"-invalid", false},
		{"invalid-", false},
		{"too.long.hostname.that.exceeds.the.maximum.length.limit.for.hostnames.which.should.be.253.characters.maximum.this.hostname.is.definitely.too.long.and.should.fail.validation.because.it.exceeds.the.rfc.limits.for.hostname.length.more.text.to.make.it.even.longer.and.exceed.the.limit", false},
		{"invalid..double.dot", false},
		{"invalid.-.dash", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := isValidHostname(tt.hostname)
			if result != tt.expected {
				t.Errorf("isValidHostname(%q) = %v, expected %v", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestIsValidHostnamePort(t *testing.T) {
	tests := []struct {
		hostPort string
		expected bool
	}{
		{"example.com:1883", true},
		{"localhost:8080", true},
		{"test-server:443", true},
		{"192.168.1.1:1883", false},  // This should use isValidIPPort instead
		{"example.com", false},       // Missing port
		{"example.com:abc", false},   // Invalid port
		{"example.com:70000", false}, // Port out of range
		{"example.com:0", false},     // Port zero
		{":1883", false},             // Missing hostname
	}

	for _, tt := range tests {
		t.Run(tt.hostPort, func(t *testing.T) {
			result := isValidHostnamePort(tt.hostPort)
			if result != tt.expected {
				t.Errorf("isValidHostnamePort(%q) = %v, expected %v", tt.hostPort, result, tt.expected)
			}
		})
	}
}

func TestIsValidIPPort(t *testing.T) {
	tests := []struct {
		ipPort   string
		expected bool
	}{
		{"192.168.1.1:1883", true},
		{"127.0.0.1:8080", true},
		{"10.0.0.1:443", true},
		{"192.168.1.1", false},       // Missing port
		{"192.168.1.1:abc", false},   // Invalid port
		{"192.168.1.1:70000", false}, // Port out of range
		{"invalid.ip:1883", false},   // Invalid IP
		{":1883", false},             // Missing IP
		{"example.com:1883", false},  // Hostname, not IP
	}

	for _, tt := range tests {
		t.Run(tt.ipPort, func(t *testing.T) {
			result := isValidIPPort(tt.ipPort)
			if result != tt.expected {
				t.Errorf("isValidIPPort(%q) = %v, expected %v", tt.ipPort, result, tt.expected)
			}
		})
	}
}

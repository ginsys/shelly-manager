package configuration

import (
	"encoding/json"
	"testing"
)

func TestConfigurationValidator_ValidateConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		validationLevel   ValidationLevel
		deviceModel       string
		generation        int
		capabilities      []string
		config            string
		expectValid       bool
		expectErrors      int
		expectWarnings    int
		expectedErrorCode string
	}{
		{
			name:            "Valid basic WiFi configuration",
			validationLevel: ValidationLevelBasic,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "mqtt"},
			config: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork",
					"password": "password123"
				}
			}`,
			expectValid: true,
		},
		{
			name:            "Valid complete configuration",
			validationLevel: ValidationLevelBasic,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "mqtt", "auth"},
			config: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork",
					"password": "password123",
					"ipv4mode": "dhcp"
				},
				"mqtt": {
					"enable": true,
					"server": "mqtt.example.com",
					"port": 1883,
					"user": "testuser",
					"pass": "testpass"
				},
				"auth": {
					"enable": true,
					"user": "admin",
					"pass": "securepassword"
				}
			}`,
			expectValid: true,
		},
		{
			name:            "Invalid WiFi - missing SSID",
			validationLevel: ValidationLevelBasic,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi"},
			config: `{
				"wifi": {
					"enable": true,
					"ssid": ""
				}
			}`,
			expectValid:       false,
			expectErrors:      1,
			expectedErrorCode: "TYPED_VALIDATION_FAILED",
		},
		{
			name:            "Invalid MQTT - missing server",
			validationLevel: ValidationLevelBasic,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "mqtt"},
			config: `{
				"mqtt": {
					"enable": true,
					"server": ""
				}
			}`,
			expectValid:       false,
			expectErrors:      1,
			expectedErrorCode: "TYPED_VALIDATION_FAILED",
		},
		{
			name:            "Warnings for weak password",
			validationLevel: ValidationLevelBasic,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "auth"},
			config: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork",
					"password": "123"
				},
				"auth": {
					"enable": true,
					"user": "admin",
					"pass": "admin"
				}
			}`,
			expectValid:    true,
			expectWarnings: 2, // Short WiFi password + default auth password
		},
		{
			name:            "Strict validation rejects weak passwords",
			validationLevel: ValidationLevelStrict,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "auth"},
			config: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork",
					"password": "123"
				},
				"auth": {
					"enable": true,
					"user": "admin",
					"pass": "admin"
				}
			}`,
			expectValid:       false,
			expectErrors:      2, // Both passwords rejected in strict mode
			expectedErrorCode: "WEAK_WIFI_PASSWORD",
		},
		{
			name:              "Invalid JSON",
			validationLevel:   ValidationLevelBasic,
			deviceModel:       "SHSW-1",
			generation:        2,
			capabilities:      []string{"wifi"},
			config:            `{invalid json}`,
			expectValid:       false,
			expectErrors:      1,
			expectedErrorCode: "INVALID_JSON",
		},
		{
			name:            "Production warnings for insecure settings",
			validationLevel: ValidationLevelProduction,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "mqtt", "auth", "cloud"},
			config: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork",
					"password": "password123"
				},
				"mqtt": {
					"enable": true,
					"server": "localhost:1883"
				},
				"cloud": {
					"enable": true,
					"server": "https://cloud.example.com"
				}
			}`,
			expectValid:    true,
			expectWarnings: 3, // Auth disabled, localhost MQTT, cloud enabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigurationValidator(
				tt.validationLevel,
				tt.deviceModel,
				tt.generation,
				tt.capabilities,
			)

			result := validator.ValidateConfiguration(json.RawMessage(tt.config))

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, result.Valid)
			}

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			if tt.expectedErrorCode != "" && len(result.Errors) > 0 {
				found := false
				for _, err := range result.Errors {
					if err.Code == tt.expectedErrorCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error code %q not found in errors: %v", tt.expectedErrorCode, result.Errors)
				}
			}
		})
	}
}

func TestConfigurationValidator_ValidateWiFi(t *testing.T) {
	validator := NewConfigurationValidator(
		ValidationLevelBasic,
		"SHSW-1",
		2,
		[]string{"wifi"},
	)

	tests := []struct {
		name           string
		wifi           *WiFiConfiguration
		expectWarnings int
		expectedCodes  []string
	}{
		{
			name: "Short SSID warning",
			wifi: &WiFiConfiguration{
				Enable: true,
				SSID:   "ab",
			},
			expectWarnings: 1,
			expectedCodes:  []string{"SHORT_SSID"},
		},
		{
			name: "Special characters in SSID",
			wifi: &WiFiConfiguration{
				Enable: true,
				SSID:   "Test\"Network",
			},
			expectWarnings: 1,
			expectedCodes:  []string{"SPECIAL_CHARS_SSID"},
		},
		{
			name: "Short password warning",
			wifi: &WiFiConfiguration{
				Enable:   true,
				SSID:     "TestNetwork",
				Password: "123",
			},
			expectWarnings: 1,
			expectedCodes:  []string{"WEAK_WIFI_PASSWORD"},
		},
		{
			name: "WiFi repeater mode info",
			wifi: &WiFiConfiguration{
				Enable: true,
				SSID:   "TestNetwork",
				AccessPoint: &AccessPointConfig{
					Enable: true,
					SSID:   "TestAP",
				},
			},
			expectWarnings: 0, // This should be an info message, not warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{},
				Info:     []ValidationInfo{},
			}

			validator.validateWiFi(tt.wifi, result)

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			for _, expectedCode := range tt.expectedCodes {
				found := false
				for _, warning := range result.Warnings {
					if warning.Code == expectedCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning code %q not found in warnings: %v", expectedCode, result.Warnings)
				}
			}
		})
	}
}

func TestConfigurationValidator_ValidateMQTT(t *testing.T) {
	tests := []struct {
		name            string
		validationLevel ValidationLevel
		mqtt            *MQTTConfiguration
		expectWarnings  int
		expectedCodes   []string
	}{
		{
			name:            "Localhost server warning in production",
			validationLevel: ValidationLevelProduction,
			mqtt: &MQTTConfiguration{
				Enable: true,
				Server: "localhost:1883",
			},
			expectWarnings: 1,
			expectedCodes:  []string{"LOCALHOST_MQTT_SERVER"},
		},
		{
			name:            "Default credentials warning",
			validationLevel: ValidationLevelBasic,
			mqtt: &MQTTConfiguration{
				Enable:   true,
				Server:   "mqtt.example.com",
				User:     "admin",
				Password: "admin",
			},
			expectWarnings: 1,
			expectedCodes:  []string{"DEFAULT_MQTT_CREDENTIALS"},
		},
		{
			name:            "Short keep alive warning",
			validationLevel: ValidationLevelBasic,
			mqtt: &MQTTConfiguration{
				Enable:    true,
				Server:    "mqtt.example.com",
				KeepAlive: 15,
			},
			expectWarnings: 1,
			expectedCodes:  []string{"SHORT_KEEPALIVE"},
		},
		{
			name:            "Topic prefix with wildcards error",
			validationLevel: ValidationLevelBasic,
			mqtt: &MQTTConfiguration{
				Enable:      true,
				Server:      "mqtt.example.com",
				TopicPrefix: "home/+/sensors",
			},
			expectWarnings: 0, // This should be an error, not warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigurationValidator(
				tt.validationLevel,
				"SHSW-1",
				2,
				[]string{"mqtt"},
			)

			result := &ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{},
				Info:     []ValidationInfo{},
			}

			validator.validateMQTT(tt.mqtt, result)

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			for _, expectedCode := range tt.expectedCodes {
				found := false
				for _, warning := range result.Warnings {
					if warning.Code == expectedCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning code %q not found in warnings: %v", expectedCode, result.Warnings)
				}
			}
		})
	}
}

func TestConfigurationValidator_ValidateAuth(t *testing.T) {
	tests := []struct {
		name            string
		validationLevel ValidationLevel
		auth            *AuthConfiguration
		expectWarnings  int
		expectedCodes   []string
	}{
		{
			name:            "Auth disabled warning in production",
			validationLevel: ValidationLevelProduction,
			auth:            nil, // No auth configuration = disabled
			expectWarnings:  1,
			expectedCodes:   []string{"AUTH_DISABLED"},
		},
		{
			name:            "Common username warning",
			validationLevel: ValidationLevelBasic,
			auth: &AuthConfiguration{
				Enable:   true,
				Username: "admin",
				Password: "securepassword",
			},
			expectWarnings: 1,
			expectedCodes:  []string{"COMMON_USERNAME"},
		},
		{
			name:            "Default password warning",
			validationLevel: ValidationLevelBasic,
			auth: &AuthConfiguration{
				Enable:   true,
				Username: "user",
				Password: "admin",
			},
			expectWarnings: 1,
			expectedCodes:  []string{"DEFAULT_PASSWORD"},
		},
		{
			name:            "Default password error in strict mode",
			validationLevel: ValidationLevelStrict,
			auth: &AuthConfiguration{
				Enable:   true,
				Username: "user",
				Password: "123456",
			},
			expectWarnings: 0, // Should be error in strict mode
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigurationValidator(
				tt.validationLevel,
				"SHSW-1",
				2,
				[]string{"auth"},
			)

			result := &ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{},
				Info:     []ValidationInfo{},
			}

			validator.validateAuth(tt.auth, result)

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			for _, expectedCode := range tt.expectedCodes {
				found := false
				for _, warning := range result.Warnings {
					if warning.Code == expectedCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning code %q not found in warnings: %v", expectedCode, result.Warnings)
				}
			}
		})
	}
}

func TestConfigurationValidator_ValidatePasswordStrength(t *testing.T) {
	validator := NewConfigurationValidator(
		ValidationLevelBasic,
		"generic",
		2,
		[]string{},
	)

	tests := []struct {
		name             string
		password         string
		expectedWarnings int
		expectedContains []string
	}{
		{
			name:             "Short password",
			password:         "123",
			expectedWarnings: 1,
			expectedContains: []string{"shorter than 8 characters"},
		},
		{
			name:             "No uppercase",
			password:         "password123",
			expectedWarnings: 1,
			expectedContains: []string{"at least 3 of"},
		},
		{
			name:             "Repeated characters",
			password:         "passsword123",
			expectedWarnings: 1,
			expectedContains: []string{"repeated characters"},
		},
		{
			name:             "Sequential characters",
			password:         "password123",
			expectedWarnings: 2,
			expectedContains: []string{"at least 3 of", "sequential characters"},
		},
		{
			name:             "Strong password",
			password:         "SecureP@ssw0rd!",
			expectedWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := validator.validatePasswordStrength(tt.password)

			if len(warnings) != tt.expectedWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectedWarnings, len(warnings), warnings)
			}

			for _, expectedText := range tt.expectedContains {
				found := false
				for _, warning := range warnings {
					if containsText(warning, expectedText) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q not found in warnings: %v", expectedText, warnings)
				}
			}
		})
	}
}

func TestConfigurationValidator_ValidateIPConfiguration(t *testing.T) {
	validator := NewConfigurationValidator(
		ValidationLevelBasic,
		"SHSW-1",
		2,
		[]string{"wifi"},
	)

	tests := []struct {
		name           string
		ipConfig       *StaticIPConfig
		expectErrors   int
		expectWarnings int
		expectedCodes  []string
	}{
		{
			name: "Valid IP configuration",
			ipConfig: &StaticIPConfig{
				IP:      "192.168.1.100",
				Netmask: "255.255.255.0",
				Gateway: "192.168.1.1",
			},
			expectErrors:   0,
			expectWarnings: 0,
		},
		{
			name: "Invalid IP address",
			ipConfig: &StaticIPConfig{
				IP:      "invalid.ip",
				Netmask: "255.255.255.0",
				Gateway: "192.168.1.1",
			},
			expectErrors:  1,
			expectedCodes: []string{"INVALID_IP"},
		},
		{
			name: "IP and gateway in different subnets",
			ipConfig: &StaticIPConfig{
				IP:      "192.168.1.100",
				Netmask: "255.255.255.0",
				Gateway: "10.0.0.1",
			},
			expectErrors:   0,
			expectWarnings: 1,
			expectedCodes:  []string{"IP_GATEWAY_SUBNET_MISMATCH"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{},
				Info:     []ValidationInfo{},
			}

			validator.validateIPConfiguration(tt.ipConfig, "test", result)

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			for _, expectedCode := range tt.expectedCodes {
				found := false
				// Check both errors and warnings
				for _, err := range result.Errors {
					if err.Code == expectedCode {
						found = true
						break
					}
				}
				for _, warning := range result.Warnings {
					if warning.Code == expectedCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected code %q not found in errors or warnings", expectedCode)
				}
			}

			// If there were errors, result should be invalid
			if len(result.Errors) > 0 && result.Valid {
				result.Valid = false
			}
		})
	}
}

func TestConfigurationValidator_DeviceCompatibility(t *testing.T) {
	tests := []struct {
		name          string
		deviceModel   string
		generation    int
		config        *TypedConfiguration
		expectInfo    int
		expectedCodes []string
	}{
		{
			name:        "BLE on Gen1 device warning",
			deviceModel: "SHSW-1",
			generation:  1,
			config: &TypedConfiguration{
				System: &SystemConfiguration{
					Device: &TypedDeviceConfig{
						BleConfig: &BLEConfig{
							Enable: true,
						},
					},
				},
			},
			expectInfo:    1,
			expectedCodes: []string{"BLE_NOT_SUPPORTED_GEN1"},
		},
		{
			name:        "Eco mode on switch device",
			deviceModel: "SHSW-1",
			generation:  2,
			config: &TypedConfiguration{
				System: &SystemConfiguration{
					Device: &TypedDeviceConfig{
						EcoMode: true,
					},
				},
			},
			expectInfo:    1,
			expectedCodes: []string{"ECO_MODE_SWITCH"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigurationValidator(
				ValidationLevelBasic,
				tt.deviceModel,
				tt.generation,
				[]string{"wifi"},
			)

			result := &ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{},
				Info:     []ValidationInfo{},
			}

			validator.validateDeviceCompatibility(tt.config, result)

			if len(result.Info) != tt.expectInfo {
				t.Errorf("Expected %d info messages, got %d: %v", tt.expectInfo, len(result.Info), result.Info)
			}

			for _, expectedCode := range tt.expectedCodes {
				found := false
				// Check info messages and warnings
				for _, info := range result.Info {
					if info.Code == expectedCode {
						found = true
						break
					}
				}
				for _, warning := range result.Warnings {
					if warning.Code == expectedCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected code %q not found in info or warnings", expectedCode)
				}
			}
		})
	}
}

func TestValidationResult_GetValidationSummary(t *testing.T) {
	tests := []struct {
		name     string
		result   *ValidationResult
		expected string
	}{
		{
			name: "Valid with no issues",
			result: &ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{},
				Info:     []ValidationInfo{},
			},
			expected: "Configuration is valid with no issues",
		},
		{
			name: "Valid with warnings",
			result: &ValidationResult{
				Valid:    true,
				Errors:   []ValidationError{},
				Warnings: []ValidationWarning{{Field: "test", Message: "test warning", Code: "TEST"}},
				Info:     []ValidationInfo{{Field: "test", Message: "test info", Code: "TEST"}},
			},
			expected: "Configuration is valid with 1 warnings and 1 info messages",
		},
		{
			name: "Invalid with errors",
			result: &ValidationResult{
				Valid:    false,
				Errors:   []ValidationError{{Field: "test", Message: "test error", Code: "TEST"}},
				Warnings: []ValidationWarning{{Field: "test", Message: "test warning", Code: "TEST"}},
			},
			expected: "Configuration is invalid with 1 errors, 1 warnings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := tt.result.GetValidationSummary()
			if summary != tt.expected {
				t.Errorf("Expected summary %q, got %q", tt.expected, summary)
			}
		})
	}
}

// Helper functions

func containsText(text, substring string) bool {
	return len(text) >= len(substring) &&
		(text == substring ||
			findSubstring(text, substring))
}

func findSubstring(text, substring string) bool {
	for i := 0; i <= len(text)-len(substring); i++ {
		if text[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}

package configuration

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Auto-migrate all tables
	err = db.AutoMigrate(
		&ConfigTemplate{},
		&DeviceConfig{},
		&ConfigHistory{},
		&DriftDetectionSchedule{},
		&DriftReport{},
		&DriftTrend{},
		&Device{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

// createTestService creates a configuration service for testing
func createTestService(t *testing.T) *Service {
	db := setupTestDB(t)
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text"})
	return NewService(db, logger)
}

func TestConfigurationService_TypedConfigurationWorkflow(t *testing.T) {
	service := createTestService(t)

	// Create test device
	device := Device{
		ID:   1,
		MAC:  "AABBCCDDEEFF",
		IP:   "192.168.1.100",
		Type: "SHSW-1",
		Name: "Test Switch",
	}

	if err := service.db.Create(&device).Error; err != nil {
		t.Fatalf("Failed to create test device: %v", err)
	}

	// Test 1: Create typed configuration
	typedConfig := &TypedConfiguration{
		WiFi: &WiFiConfiguration{
			Enable:   true,
			SSID:     "TestNetwork",
			Password: "password123",
			IPv4Mode: "dhcp",
		},
		MQTT: &MQTTConfiguration{
			Enable: true,
			Server: "mqtt.example.com",
			Port:   1883,
			User:   "testuser",
		},
		Auth: &AuthConfiguration{
			Enable:   true,
			Username: "admin",
			Password: "securepassword",
		},
	}

	// Test updating typed configuration
	err := service.UpdateTypedDeviceConfig(device.ID, typedConfig)
	if err != nil {
		t.Fatalf("Failed to update typed configuration: %v", err)
	}

	// Test 2: Retrieve typed configuration
	retrievedConfig, err := service.GetTypedDeviceConfig(device.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve typed configuration: %v", err)
	}

	// Verify retrieved configuration
	if retrievedConfig.WiFi == nil {
		t.Errorf("WiFi configuration is nil")
	} else {
		if retrievedConfig.WiFi.SSID != "TestNetwork" {
			t.Errorf("Expected SSID 'TestNetwork', got %q", retrievedConfig.WiFi.SSID)
		}
		if !retrievedConfig.WiFi.Enable {
			t.Errorf("Expected WiFi to be enabled")
		}
	}

	if retrievedConfig.MQTT == nil {
		t.Errorf("MQTT configuration is nil")
	} else {
		if retrievedConfig.MQTT.Server != "mqtt.example.com" {
			t.Errorf("Expected MQTT server 'mqtt.example.com', got %q", retrievedConfig.MQTT.Server)
		}
		if retrievedConfig.MQTT.Port != 1883 {
			t.Errorf("Expected MQTT port 1883, got %d", retrievedConfig.MQTT.Port)
		}
	}

	// Test 3: Validate configuration
	validationResult := service.ValidateTypedConfiguration(
		retrievedConfig,
		ValidationLevelBasic,
		"SHSW-1",
		2,
		[]string{"wifi", "mqtt", "auth"},
	)

	if !validationResult.Valid {
		t.Errorf("Configuration should be valid, got errors: %v", validationResult.Errors)
	}

	// Test 4: Convert to raw and back
	rawJSON, err := service.ConvertTypedToRaw(retrievedConfig)
	if err != nil {
		t.Fatalf("Failed to convert typed to raw: %v", err)
	}

	convertedBack, warnings, err := service.ConvertRawToTyped(rawJSON)
	if err != nil {
		t.Fatalf("Failed to convert raw to typed: %v", err)
	}

	if len(warnings) > 0 {
		t.Logf("Conversion warnings: %v", warnings)
	}

	// Verify round-trip conversion
	if convertedBack.WiFi.SSID != typedConfig.WiFi.SSID {
		t.Errorf("Round-trip conversion failed for WiFi SSID")
	}
	if convertedBack.MQTT.Server != typedConfig.MQTT.Server {
		t.Errorf("Round-trip conversion failed for MQTT server")
	}
}

func TestConfigurationService_TemplateEngineIntegration(t *testing.T) {
	service := createTestService(t)

	// Create test device for template processing
	device := &Device{
		ID:   1,
		MAC:  "AABBCCDDEEFF",
		IP:   "192.168.1.100",
		Type: "SHSW-1",
		Name: "Living Room Switch",
	}

	// Use the device for template processing
	_ = device

	// Test template substitution
	templateConfig := json.RawMessage(`{
		"wifi": {
			"enable": true,
			"ssid": "{{.Network.SSID}}",
			"pass": "{{.Custom.wifi_password}}"
		},
		"sys": {
			"device": {
				"name": "{{deviceShortName .Device.Model .Device.MAC}}",
				"hostname": "{{.Device.Name | hostName}}"
			}
		},
		"mqtt": {
			"enable": {{.Custom.enable_mqtt | default false}},
			"server": "{{.Custom.mqtt_server | default \"localhost\"}}",
			"id": "{{.Device.MAC | macNone}}"
		}
	}`)

	variables := map[string]interface{}{
		"network": map[string]interface{}{
			"ssid": "HomeNetwork",
		},
		"custom": map[string]interface{}{
			"wifi_password": "supersecret",
			"enable_mqtt":   true,
			"mqtt_server":   "mqtt.home.local",
		},
	}

	result := service.SubstituteVariables(templateConfig, variables)

	// Parse result
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Failed to parse template result: %v", err)
	}

	// Verify substitutions
	wifi, ok := parsed["wifi"].(map[string]interface{})
	if !ok {
		t.Errorf("WiFi section not found")
	} else {
		if wifi["ssid"] != "HomeNetwork" {
			t.Errorf("Expected SSID 'HomeNetwork', got %v", wifi["ssid"])
		}
		if wifi["pass"] != "supersecret" {
			t.Errorf("Expected password 'supersecret', got %v", wifi["pass"])
		}
	}

	mqtt, ok := parsed["mqtt"].(map[string]interface{})
	if !ok {
		t.Errorf("MQTT section not found")
	} else {
		if mqtt["enable"] != true {
			t.Errorf("Expected MQTT enabled, got %v", mqtt["enable"])
		}
		if mqtt["server"] != "mqtt.home.local" {
			t.Errorf("Expected MQTT server 'mqtt.home.local', got %v", mqtt["server"])
		}
		if mqtt["id"] != "AABBCCDDEEFF" {
			t.Errorf("Expected MQTT ID 'AABBCCDDEEFF', got %v", mqtt["id"])
		}
	}
}

func TestConfigurationService_ValidationWorkflow(t *testing.T) {
	service := createTestService(t)

	tests := []struct {
		name            string
		config          *TypedConfiguration
		validationLevel ValidationLevel
		deviceModel     string
		generation      int
		capabilities    []string
		expectValid     bool
		expectWarnings  int
		expectErrors    int
	}{
		{
			name: "Basic valid configuration",
			config: &TypedConfiguration{
				WiFi: &WiFiConfiguration{
					Enable:   true,
					SSID:     "TestNetwork",
					Password: "password123",
				},
			},
			validationLevel: ValidationLevelBasic,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi"},
			expectValid:     true,
		},
		{
			name: "Configuration with warnings",
			config: &TypedConfiguration{
				WiFi: &WiFiConfiguration{
					Enable:   true,
					SSID:     "ab",  // Too short
					Password: "123", // Too short
				},
				Auth: &AuthConfiguration{
					Enable:   true,
					Username: "admin", // Common username
					Password: "admin", // Default password
				},
			},
			validationLevel: ValidationLevelBasic,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "auth"},
			expectValid:     true,
			expectWarnings:  4, // Short SSID, weak WiFi password, common username, default auth password
		},
		{
			name: "Strict validation rejects weak passwords",
			config: &TypedConfiguration{
				WiFi: &WiFiConfiguration{
					Enable:   true,
					SSID:     "TestNetwork",
					Password: "123",
				},
				Auth: &AuthConfiguration{
					Enable:   true,
					Username: "admin",
					Password: "admin",
				},
			},
			validationLevel: ValidationLevelStrict,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "auth"},
			expectValid:     false,
			expectErrors:    2, // Both passwords rejected in strict mode
		},
		{
			name: "Production validation warnings",
			config: &TypedConfiguration{
				WiFi: &WiFiConfiguration{
					Enable:   true,
					SSID:     "TestNetwork",
					Password: "password123",
				},
				MQTT: &MQTTConfiguration{
					Enable: true,
					Server: "localhost:1883",
				},
				Cloud: &CloudConfiguration{
					Enable: true,
					Server: "https://cloud.example.com",
				},
				// No auth configuration = disabled
			},
			validationLevel: ValidationLevelProduction,
			deviceModel:     "SHSW-1",
			generation:      2,
			capabilities:    []string{"wifi", "mqtt", "cloud"},
			expectValid:     true,
			expectWarnings:  3, // Auth disabled, localhost MQTT, cloud enabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.ValidateTypedConfiguration(
				tt.config,
				tt.validationLevel,
				tt.deviceModel,
				tt.generation,
				tt.capabilities,
			)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v. Errors: %v",
					tt.expectValid, result.Valid, result.Errors)
			}

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v",
					tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v",
					tt.expectErrors, len(result.Errors), result.Errors)
			}
		})
	}
}

func TestConfigurationService_BatchValidation(t *testing.T) {
	service := createTestService(t)

	configs := []*TypedConfiguration{
		{
			WiFi: &WiFiConfiguration{
				Enable:   true,
				SSID:     "ValidNetwork",
				Password: "password123",
			},
		},
		{
			WiFi: &WiFiConfiguration{
				Enable: true,
				SSID:   "", // Invalid - empty SSID
			},
		},
		{
			MQTT: &MQTTConfiguration{
				Enable: true,
				Server: "mqtt.example.com",
				Port:   1883,
			},
		},
	}

	results := service.BatchValidateConfigurations(configs, ValidationLevelBasic)

	if len(results) != 3 {
		t.Fatalf("Expected 3 validation results, got %d", len(results))
	}

	// First config should be valid
	if !results[0].Valid {
		t.Errorf("Expected first config to be valid, got errors: %v", results[0].Errors)
	}

	// Second config should be invalid (empty SSID)
	if results[1].Valid {
		t.Errorf("Expected second config to be invalid")
	}

	// Third config should be valid
	if !results[2].Valid {
		t.Errorf("Expected third config to be valid, got errors: %v", results[2].Errors)
	}
}

func TestConfigurationService_RawToTypedConversion(t *testing.T) {
	service := createTestService(t)

	tests := []struct {
		name               string
		rawConfig          string
		expectSuccess      bool
		expectWarnings     int
		expectedWiFiSSID   string
		expectedMQTTServer string
	}{
		{
			name: "Complete configuration conversion",
			rawConfig: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork",
					"pass": "password123",
					"ipv4mode": "dhcp"
				},
				"mqtt": {
					"enable": true,
					"server": "mqtt.example.com",
					"port": 1883,
					"user": "testuser"
				},
				"auth": {
					"enable": true,
					"user": "admin",
					"pass": "securepass"
				},
				"sys": {
					"device": {
						"name": "Test Device",
						"hostname": "test-device"
					}
				},
				"cloud": {
					"enable": false
				}
			}`,
			expectSuccess:      true,
			expectedWiFiSSID:   "TestNetwork",
			expectedMQTTServer: "mqtt.example.com",
		},
		{
			name: "Partial configuration with unknown sections",
			rawConfig: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork"
				},
				"unknown_section": {
					"some_setting": "value"
				},
				"another_unknown": {
					"test": true
				}
			}`,
			expectSuccess:    true,
			expectWarnings:   1, // Unknown sections warning
			expectedWiFiSSID: "TestNetwork",
		},
		{
			name: "Invalid JSON",
			rawConfig: `{
				"wifi": {
					"enable": true,
					"ssid": "TestNetwork"
				}
				// Missing comma and invalid comment
				"mqtt": {
					"enable": false
				}
			}`,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typedConfig, warnings, err := service.ConvertRawToTyped(json.RawMessage(tt.rawConfig))

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected conversion to succeed, got error: %v", err)
					return
				}

				if len(warnings) != tt.expectWarnings {
					t.Errorf("Expected %d warnings, got %d: %v",
						tt.expectWarnings, len(warnings), warnings)
				}

				// Verify converted configuration
				if tt.expectedWiFiSSID != "" {
					if typedConfig.WiFi == nil {
						t.Errorf("WiFi configuration is nil")
					} else if typedConfig.WiFi.SSID != tt.expectedWiFiSSID {
						t.Errorf("Expected WiFi SSID %q, got %q",
							tt.expectedWiFiSSID, typedConfig.WiFi.SSID)
					}
				}

				if tt.expectedMQTTServer != "" {
					if typedConfig.MQTT == nil {
						t.Errorf("MQTT configuration is nil")
					} else if typedConfig.MQTT.Server != tt.expectedMQTTServer {
						t.Errorf("Expected MQTT server %q, got %q",
							tt.expectedMQTTServer, typedConfig.MQTT.Server)
					}
				}
			} else {
				if err == nil {
					t.Errorf("Expected conversion to fail, got success")
				}
			}
		})
	}
}

func TestConfigurationService_ConfigurationHistory(t *testing.T) {
	service := createTestService(t)

	// Create test device
	device := Device{
		ID:   1,
		MAC:  "AABBCCDDEEFF",
		IP:   "192.168.1.100",
		Type: "SHSW-1",
		Name: "Test Switch",
	}

	if err := service.db.Create(&device).Error; err != nil {
		t.Fatalf("Failed to create test device: %v", err)
	}

	// Create initial configuration
	config1 := &TypedConfiguration{
		WiFi: &WiFiConfiguration{
			Enable:   true,
			SSID:     "Network1",
			Password: "password1",
		},
	}

	err := service.UpdateTypedDeviceConfig(device.ID, config1)
	if err != nil {
		t.Fatalf("Failed to create initial configuration: %v", err)
	}

	// Update configuration
	config2 := &TypedConfiguration{
		WiFi: &WiFiConfiguration{
			Enable:   true,
			SSID:     "Network2",
			Password: "password2",
		},
		MQTT: &MQTTConfiguration{
			Enable: true,
			Server: "mqtt.example.com",
		},
	}

	err = service.UpdateTypedDeviceConfig(device.ID, config2)
	if err != nil {
		t.Fatalf("Failed to update configuration: %v", err)
	}

	// Verify history was recorded
	var historyCount int64
	service.db.Model(&ConfigHistory{}).Where("device_id = ?", device.ID).Count(&historyCount)

	if historyCount != 2 {
		t.Errorf("Expected 2 history entries, got %d", historyCount)
	}

	// Verify latest configuration
	currentConfig, err := service.GetTypedDeviceConfig(device.ID)
	if err != nil {
		t.Fatalf("Failed to get current configuration: %v", err)
	}

	if currentConfig.WiFi.SSID != "Network2" {
		t.Errorf("Expected current SSID 'Network2', got %q", currentConfig.WiFi.SSID)
	}

	if currentConfig.MQTT == nil {
		t.Errorf("Expected MQTT configuration to be present")
	}
}

func TestConfigurationService_ErrorHandling(t *testing.T) {
	service := createTestService(t)

	// Test retrieving configuration for non-existent device
	_, err := service.GetTypedDeviceConfig(999)
	if err == nil {
		t.Errorf("Expected error for non-existent device, got nil")
	}

	// Test updating configuration with invalid data
	invalidConfig := &TypedConfiguration{
		WiFi: &WiFiConfiguration{
			Enable: true,
			SSID:   "", // Invalid - empty SSID when enabled
		},
	}

	err = service.UpdateTypedDeviceConfig(1, invalidConfig)
	if err == nil {
		t.Errorf("Expected error for invalid configuration, got nil")
	}

	// Test conversion of invalid JSON
	invalidJSON := json.RawMessage(`{invalid json}`)
	_, _, err = service.ConvertRawToTyped(invalidJSON)
	if err == nil {
		t.Errorf("Expected error for invalid JSON, got nil")
	}
}

func TestConfigurationService_PerformanceStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance stress test in short mode")
	}

	service := createTestService(t)

	// Create multiple devices
	deviceCount := 100
	for i := 1; i <= deviceCount; i++ {
		device := Device{
			ID:   uint(i),
			MAC:  "AABBCCDDEEFF",
			IP:   "192.168.1.100",
			Type: "SHSW-1",
			Name: "Test Switch",
		}

		if err := service.db.Create(&device).Error; err != nil {
			t.Fatalf("Failed to create test device %d: %v", i, err)
		}
	}

	// Measure time for batch operations
	start := time.Now()

	// Create configurations for all devices
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

	for i := 1; i <= deviceCount; i++ {
		err := service.UpdateTypedDeviceConfig(uint(i), config)
		if err != nil {
			t.Errorf("Failed to update configuration for device %d: %v", i, err)
		}
	}

	updateDuration := time.Since(start)
	t.Logf("Time to update %d configurations: %v", deviceCount, updateDuration)

	// Measure time for batch validation
	configs := make([]*TypedConfiguration, deviceCount)
	for i := 0; i < deviceCount; i++ {
		configs[i] = config
	}

	start = time.Now()
	results := service.BatchValidateConfigurations(configs, ValidationLevelBasic)
	validationDuration := time.Since(start)

	t.Logf("Time to validate %d configurations: %v", deviceCount, validationDuration)

	// Verify all validations succeeded
	for i, result := range results {
		if !result.Valid {
			t.Errorf("Validation failed for config %d: %v", i, result.Errors)
		}
	}

	// Performance benchmarks (these are rough guidelines)
	avgUpdateTime := updateDuration / time.Duration(deviceCount)
	avgValidationTime := validationDuration / time.Duration(deviceCount)

	t.Logf("Average update time per configuration: %v", avgUpdateTime)
	t.Logf("Average validation time per configuration: %v", avgValidationTime)

	// These thresholds can be adjusted based on requirements
	if avgUpdateTime > 10*time.Millisecond {
		t.Logf("Warning: Average update time (%v) exceeds 10ms threshold", avgUpdateTime)
	}

	if avgValidationTime > 5*time.Millisecond {
		t.Logf("Warning: Average validation time (%v) exceeds 5ms threshold", avgValidationTime)
	}
}

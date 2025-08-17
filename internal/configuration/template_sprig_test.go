package configuration

import (
	"encoding/json"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestTemplateEngineSprigFunctions(t *testing.T) {
	// Create logger
	logger, err := logging.New(logging.Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create template engine
	engine := NewTemplateEngine(logger)

	// Create test context
	context := &TemplateContext{
		Device: struct {
			ID         uint   `json:"id"`
			MAC        string `json:"mac"`
			IP         string `json:"ip"`
			Name       string `json:"name"`
			Model      string `json:"model"`
			Generation int    `json:"generation"`
			Firmware   string `json:"firmware"`
		}{
			ID:         1,
			MAC:        "AA:BB:CC:DD:EE:FF",
			IP:         "192.168.1.100",
			Name:       "Test Device",
			Model:      "SHSW-25",
			Generation: 1,
			Firmware:   "v1.14.0",
		},
		Network: struct {
			SSID    string `json:"ssid"`
			Gateway string `json:"gateway"`
			Subnet  string `json:"subnet"`
			DNS     string `json:"dns"`
		}{
			SSID:    "MyNetwork",
			Gateway: "192.168.1.1",
		},
		Custom: map[string]interface{}{
			"app_name":    "shelly-manager",
			"environment": "production",
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "String manipulation with Sprig",
			template: `"{{.Device.Model | lower}}"`,
			expected: `"shsw-25"`,
		},
		{
			name:     "String transformation",
			template: `"{{.Network.SSID | upper | repeat 2}}"`,
			expected: `"MYNETWORKMYNETWORK"`,
		},
		{
			name:     "Conditional logic",
			template: `"{{if eq .Device.Generation 1}}Gen1{{else}}Gen2+{{end}}"`,
			expected: `"Gen1"`,
		},
		{
			name:     "Default values",
			template: `"{{.Network.DNS | default "8.8.8.8"}}"`,
			expected: `"8.8.8.8"`,
		},
		{
			name:     "String contains check",
			template: `{{contains "25" .Device.Model}}`,
			expected: `true`,
		},
		{
			name:     "Environment variable with default",
			template: `"{{envOr "NONEXISTENT_VAR" "default_value"}}"`,
			expected: `"default_value"`,
		},
		{
			name:     "Custom function validation",
			template: `{{empty .Network.DNS}}`,
			expected: `true`,
		},
		{
			name:     "MAC address formatting",
			template: `"{{.Device.MAC | macLast4}}"`,
			expected: `"EEFF"`,
		},
		{
			name:     "Complex template with multiple functions",
			template: `"{{printf "%s-%s" (.Device.Model | lower) (.Device.MAC | macLast4)}}"`,
			expected: `"shsw-25-EEFF"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := engine.SubstituteVariables(json.RawMessage(test.template), context)
			if err != nil {
				t.Errorf("Failed to substitute template '%s': %v", test.template, err)
				return
			}

			if string(result) != test.expected {
				t.Errorf("Template '%s': expected '%s', got '%s'", test.template, test.expected, string(result))
			}
		})
	}
}

func TestTemplateEngineBaseTemplateInheritance(t *testing.T) {
	// Create logger
	logger, err := logging.New(logging.Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create template engine
	engine := NewTemplateEngine(logger)

	// Create test device context
	context := &TemplateContext{
		Device: struct {
			ID         uint   `json:"id"`
			MAC        string `json:"mac"`
			IP         string `json:"ip"`
			Name       string `json:"name"`
			Model      string `json:"model"`
			Generation int    `json:"generation"`
			Firmware   string `json:"firmware"`
		}{
			ID:         1,
			MAC:        "AA:BB:CC:DD:EE:FF",
			IP:         "192.168.1.100",
			Name:       "Test Device",
			Model:      "SHSW-25",
			Generation: 1,
			Firmware:   "v1.14.0",
		},
		Network: struct {
			SSID    string `json:"ssid"`
			Gateway string `json:"gateway"`
			Subnet  string `json:"subnet"`
			DNS     string `json:"dns"`
		}{
			SSID: "TestNetwork",
		},
		Auth: struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Realm    string `json:"realm"`
		}{
			Username: "admin",
			Password: "secret",
		},
	}

	t.Run("Gen1 base template", func(t *testing.T) {
		// Test applying Gen1 base template with empty custom config
		result, err := engine.ApplyBaseTemplate(1, json.RawMessage("{}"), context)
		if err != nil {
			t.Fatalf("Failed to apply base template: %v", err)
		}

		// Parse result to verify structure
		var config map[string]interface{}
		if err := json.Unmarshal(result, &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		// Verify expected Gen1 structure
		if device, ok := config["device"].(map[string]interface{}); ok {
			if name, ok := device["name"].(string); !ok || name == "" {
				t.Errorf("Expected device name to be populated, got: %v", device["name"])
			}
		} else {
			t.Error("Expected device section in config")
		}

		if login, ok := config["login"].(map[string]interface{}); ok {
			if _, ok := login["enabled"].(bool); !ok {
				t.Errorf("Expected login enabled to be a boolean, got: %v", login["enabled"])
			}
			// Login is disabled by default in simplified template, which is correct
		} else {
			t.Error("Expected login section in config")
		}
	})

	t.Run("Gen2 base template", func(t *testing.T) {
		// Test applying Gen2 base template
		context.Device.Generation = 2
		result, err := engine.ApplyBaseTemplate(2, json.RawMessage("{}"), context)
		if err != nil {
			t.Fatalf("Failed to apply base template: %v", err)
		}

		// Parse result to verify structure
		var config map[string]interface{}
		if err := json.Unmarshal(result, &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		// Verify expected Gen2 structure
		if sys, ok := config["sys"].(map[string]interface{}); ok {
			if device, ok := sys["device"].(map[string]interface{}); ok {
				if name, ok := device["name"].(string); !ok || name == "" {
					t.Errorf("Expected sys.device.name to be populated, got: %v", device["name"])
				}
			} else {
				t.Error("Expected sys.device section in config")
			}
		} else {
			t.Error("Expected sys section in config")
		}

		if wifi, ok := config["wifi"].(map[string]interface{}); ok {
			if sta, ok := wifi["sta"].(map[string]interface{}); ok {
				if ssid, ok := sta["ssid"].(string); !ok || ssid != "TestNetwork" {
					t.Errorf("Expected wifi.sta.ssid to be 'TestNetwork', got: %v", sta["ssid"])
				}
			} else {
				t.Error("Expected wifi.sta section in config")
			}
		} else {
			t.Error("Expected wifi section in config")
		}
	})

	t.Run("Custom config override", func(t *testing.T) {
		// Test merging custom config with base template
		customConfig := `{
			"device": {
				"name": "Custom Device Name",
				"custom_field": "custom_value"
			},
			"mqtt": {
				"enable": true,
				"custom_mqtt_field": "mqtt_value"
			}
		}`

		result, err := engine.ApplyBaseTemplate(1, json.RawMessage(customConfig), context)
		if err != nil {
			t.Fatalf("Failed to apply base template with custom config: %v", err)
		}

		// Parse result to verify merge
		var config map[string]interface{}
		if err := json.Unmarshal(result, &config); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		// Verify custom device name override
		if device, ok := config["device"].(map[string]interface{}); ok {
			if name, ok := device["name"].(string); !ok || name != "Custom Device Name" {
				t.Errorf("Expected device name to be 'Custom Device Name', got: %v", device["name"])
			}
			if customField, ok := device["custom_field"].(string); !ok || customField != "custom_value" {
				t.Errorf("Expected custom_field to be preserved, got: %v", device["custom_field"])
			}
		} else {
			t.Error("Expected device section in config")
		}

		// Verify MQTT merge
		if mqtt, ok := config["mqtt"].(map[string]interface{}); ok {
			if enable, ok := mqtt["enable"].(bool); !ok || !enable {
				t.Errorf("Expected mqtt.enable to be true, got: %v", mqtt["enable"])
			}
			if customField, ok := mqtt["custom_mqtt_field"].(string); !ok || customField != "mqtt_value" {
				t.Errorf("Expected custom_mqtt_field to be preserved, got: %v", mqtt["custom_mqtt_field"])
			}
			// Verify base template fields are preserved
			if server, ok := mqtt["server"].(string); !ok || server == "" {
				t.Errorf("Expected mqtt.server from base template to be preserved, got: %v", mqtt["server"])
			}
		} else {
			t.Error("Expected mqtt section in config")
		}
	})
}

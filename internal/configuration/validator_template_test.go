package configuration

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestConfigurationValidator_ValidateTemplates(t *testing.T) {
	validator := NewConfigurationValidator(ValidationLevelStrict, "SHSW-25", 1, []string{"wifi", "mqtt"})

	tests := []struct {
		name            string
		config          string
		expectValid     bool
		expectErrors    int
		expectWarnings  int
		expectInfo      int
		checkErrorCodes []string
	}{
		{
			name: "Valid template with safe functions",
			config: `{
				"device": {
					"name": "{{.Device.Name}}",
					"hostname": "{{.Device.Name | hostName}}"
				},
				"mqtt": {
					"id": "{{.Device.MAC | macNone}}",
					"topic_prefix": "{{printf \"shelly_%s\" (.Device.MAC | macLast4)}}"
				}
			}`,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 0,
			expectInfo:     0,
		},
		{
			name: "Template with Sprig functions",
			config: `{
				"device": {
					"name": "{{.Device.Model | upper}}-{{.Device.MAC | macLast4}}",
					"enabled": {{if eq .Device.Generation 1}}true{{else}}false{{end}}
				},
				"mqtt": {
					"server": "{{.Custom.mqtt_server | default \"mqtt.local:1883\"}}"
				}
			}`,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 1, // Custom variable reference
			expectInfo:     0,
		},
		{
			name: "Template with environment variables",
			config: `{
				"wifi": {
					"ssid": "{{.Network.SSID}}",
					"password": "{{envOr \"WIFI_PASSWORD\" \"defaultpass\"}}"
				},
				"mqtt": {
					"server": "{{env \"MQTT_SERVER\"}}"
				}
			}`,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 0,
			expectInfo:     2, // Environment variable references
		},
		{
			name: "Template with dangerous functions",
			config: `{
				"device": {
					"name": "{{exec \"whoami\"}}",
					"config": "{{readFile \"/etc/passwd\"}}"
				}
			}`,
			expectValid:     false,
			expectErrors:    2,
			expectWarnings:  0,
			expectInfo:      0,
			checkErrorCodes: []string{"DANGEROUS_TEMPLATE_FUNCTION"},
		},
		{
			name: "Invalid template syntax",
			config: `{
				"device": {
					"name": "{{.Device.Name",
					"id": "{{.Device.MAC | invalidFunction}}"
				}
			}`,
			expectValid:     false,
			expectErrors:    1,
			expectWarnings:  0,
			expectInfo:      0,
			checkErrorCodes: []string{"TEMPLATE_SYNTAX_ERROR"},
		},
		{
			name: "Template with custom variables",
			config: `{
				"wifi": {
					"ssid": "{{.Custom.network_name}}",
					"password": "{{.Custom.wifi_password}}"
				},
				"device": {
					"location": "{{.Custom.deployment_location}}"
				}
			}`,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 3, // Three custom variable references
			expectInfo:     0,
		},
		{
			name: "No templates",
			config: `{
				"device": {
					"name": "Static Device Name",
					"hostname": "static-hostname"
				},
				"mqtt": {
					"enable": false
				}
			}`,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 0,
			expectInfo:     0,
		},
		{
			name: "Complex valid template",
			config: `{
				"device": {
					"name": "{{printf \"%s-%s\" (.Device.Model | lower) (.Device.MAC | macLast4)}}",
					"hostname": "{{.Device.Name | hostName}}"
				},
				"wifi": {
					"ssid": "{{.Network.SSID}}",
					"enabled": {{.Network.SSID | empty | not}}
				},
				"mqtt": {
					"enable": {{if .Custom.enable_mqtt}}{{.Custom.enable_mqtt}}{{else}}false{{end}},
					"server": "{{if .Custom.mqtt_server}}{{.Custom.mqtt_server}}{{else}}mqtt.local:1883{{end}}",
					"topic_prefix": "shelly/{{.Device.MAC | macNone}}"
				}
			}`,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 2, // Two custom variable references
			expectInfo:     0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := validator.ValidateConfiguration(json.RawMessage(test.config))

			// Check overall validity
			if result.Valid != test.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", test.expectValid, result.Valid)
			}

			// Check error count
			if len(result.Errors) != test.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", test.expectErrors, len(result.Errors), result.Errors)
			}

			// Check warning count
			if len(result.Warnings) != test.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", test.expectWarnings, len(result.Warnings), result.Warnings)
			}

			// Check info count
			if len(result.Info) != test.expectInfo {
				t.Errorf("Expected %d info messages, got %d: %v", test.expectInfo, len(result.Info), result.Info)
			}

			// Check specific error codes
			if len(test.checkErrorCodes) > 0 {
				errorCodes := make(map[string]bool)
				for _, err := range result.Errors {
					errorCodes[err.Code] = true
				}

				for _, expectedCode := range test.checkErrorCodes {
					if !errorCodes[expectedCode] {
						t.Errorf("Expected error code '%s' not found in results", expectedCode)
					}
				}
			}
		})
	}
}

func TestConfigurationValidator_TemplateSecurityValidation(t *testing.T) {
	validator := NewConfigurationValidator(ValidationLevelProduction, "SHSW-25", 1, []string{"wifi", "mqtt"})

	dangerousFunctions := []string{
		"exec", "shell", "command", "readFile", "writeFile", "httpGet", "httpPost",
	}

	for _, dangerousFunc := range dangerousFunctions {
		t.Run("Block dangerous function: "+dangerousFunc, func(t *testing.T) {
			config := fmt.Sprintf(`{
				"device": {
					"name": "{{%s \"dangerous_command\"}}"
				}
			}`, dangerousFunc)

			result := validator.ValidateConfiguration(json.RawMessage(config))

			if result.Valid {
				t.Error("Expected configuration with dangerous function to be invalid")
			}

			found := false
			for _, err := range result.Errors {
				if err.Code == "DANGEROUS_TEMPLATE_FUNCTION" {
					found = true
					break
				}
			}

			if !found {
				t.Error("Expected DANGEROUS_TEMPLATE_FUNCTION error code")
			}
		})
	}
}

func TestConfigurationValidator_TemplateVariableDetection(t *testing.T) {
	validator := NewConfigurationValidator(ValidationLevelBasic, "SHSW-25", 1, []string{})

	tests := []struct {
		name           string
		config         string
		expectWarnings []string
		expectInfo     []string
	}{
		{
			name: "Custom variable detection",
			config: `{
				"wifi": {
					"ssid": "{{.Custom.network_name}}",
					"password": "{{.Custom.wifi_password}}"
				}
			}`,
			expectWarnings: []string{"CUSTOM_VARIABLE_REFERENCE"},
		},
		{
			name: "Environment variable detection",
			config: `{
				"mqtt": {
					"server": "{{env \"MQTT_SERVER\"}}",
					"password": "{{envOr \"MQTT_PASS\" \"default\"}}"
				}
			}`,
			expectInfo: []string{"ENV_VARIABLE_REFERENCE"},
		},
		{
			name: "Mixed variable types",
			config: `{
				"device": {
					"name": "{{.Custom.device_name}}",
					"location": "{{envOr \"DEVICE_LOCATION\" \"unknown\"}}"
				}
			}`,
			expectWarnings: []string{"CUSTOM_VARIABLE_REFERENCE"},
			expectInfo:     []string{"ENV_VARIABLE_REFERENCE"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := validator.ValidateConfiguration(json.RawMessage(test.config))

			// Check warning codes
			for _, expectedCode := range test.expectWarnings {
				found := false
				for _, warning := range result.Warnings {
					if warning.Code == expectedCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning code '%s' not found", expectedCode)
				}
			}

			// Check info codes
			for _, expectedCode := range test.expectInfo {
				found := false
				for _, info := range result.Info {
					if info.Code == expectedCode {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected info code '%s' not found", expectedCode)
				}
			}
		})
	}
}

func TestConfigurationValidator_Integration(t *testing.T) {
	// Test that template validation integrates well with other validation
	validator := NewConfigurationValidator(ValidationLevelStrict, "SHSW-25", 1, []string{"wifi", "mqtt"})

	config := `{
		"device": {
			"name": "{{.Device.Name}}"
		},
		"wifi": {
			"ssid": "{{.Network.SSID}}",
			"password": "weak",
			"enable": true
		},
		"mqtt": {
			"enable": true,
			"server": "{{.Custom.mqtt_server}}"
		},
		"auth": {
			"enable": false
		}
	}`

	result := validator.ValidateConfiguration(json.RawMessage(config))

	// Should have multiple validation issues from different validators
	if len(result.Warnings) == 0 {
		t.Error("Expected warnings from multiple validators")
	}

	// Check that we have both template warnings and other validation warnings
	hasTemplateWarning := false
	hasOtherWarning := false

	for _, warning := range result.Warnings {
		if warning.Code == "CUSTOM_VARIABLE_REFERENCE" {
			hasTemplateWarning = true
		}
		if warning.Code == "WEAK_WIFI_PASSWORD" || warning.Code == "AUTH_DISABLED" {
			hasOtherWarning = true
		}
	}

	if !hasTemplateWarning {
		t.Error("Expected template validation warning")
	}

	if !hasOtherWarning {
		t.Error("Expected other validation warnings")
	}
}

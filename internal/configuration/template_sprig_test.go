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

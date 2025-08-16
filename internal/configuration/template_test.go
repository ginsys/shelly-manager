package configuration

import (
	"encoding/json"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestTemplateEngineBasicSubstitution(t *testing.T) {
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

	// Create test device
	device := &Device{
		ID:       1,
		MAC:      "AA:BB:CC:DD:EE:FF",
		IP:       "192.168.1.100",
		Name:     "Test Device",
		Type:     "SHSW-25",
		Settings: `{"model":"SHSW-25","gen":1,"fw_id":"20230913-114336/v1.14.0-gcb84623"}`,
	}

	// Create template context
	context := engine.CreateTemplateContext(device, map[string]interface{}{
		"network": map[string]interface{}{
			"ssid": "TestNetwork",
		},
		"custom": map[string]interface{}{
			"device_name": "Living Room Switch",
		},
	})

	// Populate context
	context.Network.SSID = "TestNetwork"
	context.Custom["device_name"] = "Living Room Switch"

	// Test template JSON - testing basic substitution
	templateJSON := `{
		"wifi": {
			"ssid": "{{.Network.SSID}}",
			"enabled": true
		},
		"device": {
			"name": "Living Room Switch",
			"id": "{{.Device.MAC | macLast4}}"
		}
	}`

	// Substitute variables
	result, err := engine.SubstituteVariables(json.RawMessage(templateJSON), context)
	if err != nil {
		t.Fatalf("Template substitution failed: %v", err)
	}

	// Parse result to verify
	var resultConfig map[string]interface{}
	if err := json.Unmarshal(result, &resultConfig); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Verify substitutions
	wifi := resultConfig["wifi"].(map[string]interface{})
	if wifi["ssid"] != "TestNetwork" {
		t.Errorf("Expected SSID 'TestNetwork', got '%v'", wifi["ssid"])
	}

	device_config := resultConfig["device"].(map[string]interface{})
	if device_config["name"] != "Living Room Switch" {
		t.Errorf("Expected device name 'Living Room Switch', got '%v'", device_config["name"])
	}

	if device_config["id"] != "EEFF" {
		t.Errorf("Expected device ID 'EEFF', got '%v'", device_config["id"])
	}

	t.Logf("Template substitution successful: %s", string(result))
}

func TestTemplateEngineFunctions(t *testing.T) {
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

	// Test MAC formatting functions
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
			MAC:   "AA:BB:CC:DD:EE:FF",
			Model: "SHSW-25",
		},
	}

	tests := []struct {
		template string
		expected string
	}{
		{`"{{.Device.MAC | macLast4}}"`, `"EEFF"`},
		{`"{{.Device.MAC | macLast6}}"`, `"DDEEFF"`},
		{`"{{.Device.MAC | macNone}}"`, `"AABBCCDDEEFF"`},
		{`"{{.Device.MAC | macDash}}"`, `"AA-BB-CC-DD-EE-FF"`},
		{`"{{deviceShortName .Device.Model .Device.MAC}}"`, `"SHSW-EEFF"`},
	}

	for _, test := range tests {
		result, err := engine.SubstituteVariables(json.RawMessage(test.template), context)
		if err != nil {
			t.Errorf("Failed to substitute template '%s': %v", test.template, err)
			continue
		}

		if string(result) != test.expected {
			t.Errorf("Template '%s': expected '%s', got '%s'", test.template, test.expected, string(result))
		}
	}
}

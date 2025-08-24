package sync

import (
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestAdvancedTemplateEngine_NewAdvancedTemplateEngine(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	if engine == nil {
		t.Fatal("Engine should not be nil")
	}

	if engine.logger != logger {
		t.Error("Logger should be set correctly")
	}

	if len(engine.funcs) == 0 {
		t.Error("Functions should be loaded")
	}
}

func TestAdvancedTemplateEngine_RenderTemplate(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	testCases := []struct {
		name     string
		template string
		data     interface{}
		expected string
	}{
		{
			name:     "Simple substitution",
			template: "Device: {{.Name}}",
			data:     map[string]interface{}{"Name": "TestDevice"},
			expected: "Device: TestDevice",
		},
		{
			name:     "MAC formatting with custom function",
			template: "MAC: {{macOUI .MAC}}",
			data:     map[string]interface{}{"MAC": "aa:bb:cc:dd:ee:ff"},
			expected: "MAC: AABBCC",
		},
		{
			name:     "Conditional logic",
			template: "Status: {{if .Enabled}}Active{{else}}Inactive{{end}}",
			data:     map[string]interface{}{"Enabled": true},
			expected: "Status: Active",
		},
		{
			name:     "String manipulation",
			template: "Name: {{.Name | upper | truncate 8}}",
			data:     map[string]interface{}{"Name": "very long device name"},
			expected: "Name: VERY LON",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.RenderTemplate(tc.template, tc.data)
			if err != nil {
				t.Errorf("Template rendering failed: %v", err)
			}

			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestAdvancedTemplateEngine_NetworkFunctions(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	testCases := []struct {
		name     string
		template string
		data     interface{}
		validate func(string) bool
	}{
		{
			name:     "Network address calculation",
			template: "Network: {{networkAddress .IP .Mask}}",
			data:     map[string]interface{}{"IP": "192.168.1.100", "Mask": "255.255.255.0"},
			validate: func(result string) bool {
				return result == "Network: 192.168.1.0"
			},
		},
		{
			name:     "IP in network check",
			template: "InNetwork: {{isInNetwork .IP .CIDR}}",
			data:     map[string]interface{}{"IP": "192.168.1.100", "CIDR": "192.168.1.0/24"},
			validate: func(result string) bool {
				return result == "InNetwork: true"
			},
		},
		{
			name:     "MAC OUI extraction",
			template: "OUI: {{macOUI .MAC}}",
			data:     map[string]interface{}{"MAC": "8c:aa:b5:12:34:56"},
			validate: func(result string) bool {
				return result == "OUI: 8CAAB5"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.RenderTemplate(tc.template, tc.data)
			if err != nil {
				t.Errorf("Template rendering failed: %v", err)
			}

			if !tc.validate(result) {
				t.Errorf("Validation failed for result: '%s'", result)
			}
		})
	}
}

func TestAdvancedTemplateEngine_DeviceFunctions(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	testCases := []struct {
		name     string
		template string
		data     interface{}
		expected string
	}{
		{
			name:     "Device type determination",
			template: "Type: {{deviceType .Model .Name}}",
			data:     map[string]interface{}{"Model": "ShellyPlus1", "Name": "Kitchen Switch"},
			expected: "Type: switch",
		},
		{
			name:     "Device capability check",
			template: "HasPowerMonitoring: {{checkDeviceCapability .Type \"power_monitoring\"}}",
			data:     map[string]interface{}{"Type": "switch"},
			expected: "HasPowerMonitoring: true",
		},
		{
			name:     "MAC manufacturer",
			template: "Manufacturer: {{macManufacturer .MAC}}",
			data:     map[string]interface{}{"MAC": "8c:aa:b5:12:34:56"},
			expected: "Manufacturer: Allterco Robotics Ltd (Shelly)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.RenderTemplate(tc.template, tc.data)
			if err != nil {
				t.Errorf("Template rendering failed: %v", err)
			}

			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestAdvancedTemplateEngine_TemplateConfig(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	config := TemplateConfig{
		Name:        "test_config",
		Description: "Test configuration",
		Version:     "1.0.0",
		Templates: map[string]string{
			"device_info":  "Device: {{.Name}} ({{.Type}})",
			"network_info": "IP: {{.IP}} in {{.Network}}",
		},
		Variables: map[string]interface{}{
			"Network": "192.168.1.0/24",
			"Type":    "switch",
		},
	}

	data := map[string]interface{}{
		"Name": "TestDevice",
		"IP":   "192.168.1.100",
	}

	result, err := engine.RenderTemplateConfig(config, "device_info", data)
	if err != nil {
		t.Errorf("Template config rendering failed: %v", err)
	}

	expected := "Device: TestDevice (switch)"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test template with variables
	result, err = engine.RenderTemplateConfig(config, "network_info", data)
	if err != nil {
		t.Errorf("Template config rendering failed: %v", err)
	}

	expected = "IP: 192.168.1.100 in 192.168.1.0/24"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestAdvancedTemplateEngine_ConditionalFunctions(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	testCases := []struct {
		name     string
		template string
		data     interface{}
		expected string
	}{
		{
			name:     "ifThen function",
			template: "{{ifThen .Enabled \"Active\"}}",
			data:     map[string]interface{}{"Enabled": true},
			expected: "Active",
		},
		{
			name:     "ifThen function false",
			template: "{{ifThen .Enabled \"Active\"}}",
			data:     map[string]interface{}{"Enabled": false},
			expected: "",
		},
		{
			name:     "ifThenElse function",
			template: "{{ifThenElse .Enabled \"Active\" \"Inactive\"}}",
			data:     map[string]interface{}{"Enabled": false},
			expected: "Inactive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.RenderTemplate(tc.template, tc.data)
			if err != nil {
				t.Errorf("Template rendering failed: %v", err)
			}

			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestAdvancedTemplateEngine_ExternalAPI(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	// Register a mock external API
	engine.RegisterExternalAPI("mockAPI", func(params map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status": "success",
			"data":   "mock data",
		}, nil
	})

	// Test external API call
	template := `{{$result := apiCall "mockAPI" .}}Status: {{$result.status}}`
	data := map[string]interface{}{}

	result, err := engine.RenderTemplate(template, data)
	if err != nil {
		t.Errorf("Template rendering failed: %v", err)
	}

	expected := "Status: success"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestAdvancedTemplateEngine_StringFunctions(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	testCases := []struct {
		name     string
		template string
		data     interface{}
		expected string
	}{
		{
			name:     "Sanitize string",
			template: "{{sanitize .Text}}",
			data:     map[string]interface{}{"Text": "Hello@World#Test!"},
			expected: "HelloWorldTest",
		},
		{
			name:     "Truncate string",
			template: "{{truncate .Text 5}}",
			data:     map[string]interface{}{"Text": "Hello World"},
			expected: "Hello",
		},
		{
			name:     "Pad left",
			template: "{{padLeft .Text 10 \"0\"}}",
			data:     map[string]interface{}{"Text": "123"},
			expected: "0000000123",
		},
		{
			name:     "Regex replace",
			template: "{{regexReplace \"[0-9]\" \"X\" .Text}}",
			data:     map[string]interface{}{"Text": "Test123"},
			expected: "TestXXX",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.RenderTemplate(tc.template, tc.data)
			if err != nil {
				t.Errorf("Template rendering failed: %v", err)
			}

			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestAdvancedTemplateEngine_TimeFunctions(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	template := `Timestamp: {{timestamp}}`
	data := map[string]interface{}{}

	result, err := engine.RenderTemplate(template, data)
	if err != nil {
		t.Errorf("Template rendering failed: %v", err)
	}

	// Just check that we get a timestamp format
	if !strings.Contains(result, "Timestamp: ") {
		t.Errorf("Expected timestamp output, got: '%s'", result)
	}

	// Test time formatting
	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)
	template2 := `{{formatTime .Time "2006-01-02 15:04:05"}}`
	data2 := map[string]interface{}{"Time": testTime}

	result2, err := engine.RenderTemplate(template2, data2)
	if err != nil {
		t.Errorf("Template rendering failed: %v", err)
	}

	expected := "2023-12-25 15:04:05"
	if result2 != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result2)
	}
}

// Benchmark tests
func BenchmarkAdvancedTemplateEngine_RenderSimpleTemplate(b *testing.B) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	template := "Device: {{.Name}} - IP: {{.IP}} - MAC: {{.MAC}}"
	data := map[string]interface{}{
		"Name": "TestDevice",
		"IP":   "192.168.1.100",
		"MAC":  "aa:bb:cc:dd:ee:ff",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.RenderTemplate(template, data)
		if err != nil {
			b.Errorf("Template rendering failed: %v", err)
		}
	}
}

func BenchmarkAdvancedTemplateEngine_RenderComplexTemplate(b *testing.B) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	engine := NewAdvancedTemplateEngine(logger)

	template := `
{{range .Devices}}
Device: {{.Name | upper}}
MAC: {{.MAC | macOUI}} ({{macManufacturer .MAC}})
Type: {{deviceType .Model .Name}}
Network: {{if isInNetwork .IP "192.168.1.0/24"}}Local{{else}}Remote{{end}}
{{end}}
`
	data := map[string]interface{}{
		"Devices": []map[string]interface{}{
			{
				"Name":  "Kitchen Light",
				"MAC":   "8c:aa:b5:12:34:56",
				"Model": "ShellyPlus1",
				"IP":    "192.168.1.100",
			},
			{
				"Name":  "Living Room Outlet",
				"MAC":   "c4:5b:be:78:90:12",
				"Model": "ShellyPlug",
				"IP":    "192.168.1.101",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.RenderTemplate(template, data)
		if err != nil {
			b.Errorf("Template rendering failed: %v", err)
		}
	}
}

package opnsense

import (
	"context"
	"fmt"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

func TestOPNSenseExporter_Info(t *testing.T) {
	exporter := NewOPNSenseExporter()
	info := exporter.Info()

	if info.Name != "opnsense" {
		t.Errorf("Expected name 'opnsense', got %s", info.Name)
	}

	if info.Category != sync.CategoryNetworking {
		t.Errorf("Expected category %s, got %s", sync.CategoryNetworking, info.Category)
	}

	expectedFormats := []string{"dhcp_reservations", "firewall_aliases", "bidirectional_sync", "xml_config"}
	if len(info.SupportedFormats) != len(expectedFormats) {
		t.Errorf("Expected %d supported formats, got %d", len(expectedFormats), len(info.SupportedFormats))
	}

	for i, format := range expectedFormats {
		if info.SupportedFormats[i] != format {
			t.Errorf("Expected format %s at index %d, got %s", format, i, info.SupportedFormats[i])
		}
	}
}

func TestOPNSenseExporter_ConfigSchema(t *testing.T) {
	exporter := NewOPNSenseExporter()
	schema := exporter.ConfigSchema()

	// Test required fields
	expectedRequired := []string{"host", "api_key", "api_secret"}
	if len(schema.Required) != len(expectedRequired) {
		t.Errorf("Expected %d required fields, got %d", len(expectedRequired), len(schema.Required))
	}

	for i, field := range expectedRequired {
		if schema.Required[i] != field {
			t.Errorf("Expected required field %s at index %d, got %s", field, i, schema.Required[i])
		}
	}

	// Test that sensitive fields are marked
	if !schema.Properties["api_key"].Sensitive {
		t.Error("api_key should be marked as sensitive")
	}

	if !schema.Properties["api_secret"].Sensitive {
		t.Error("api_secret should be marked as sensitive")
	}

	// Test default values
	if schema.Properties["port"].Default != 443 {
		t.Errorf("Expected default port 443, got %v", schema.Properties["port"].Default)
	}

	if schema.Properties["use_https"].Default != true {
		t.Errorf("Expected default use_https true, got %v", schema.Properties["use_https"].Default)
	}
}

func TestOPNSenseExporter_ValidateConfig(t *testing.T) {
	exporter := NewOPNSenseExporter()

	// Test valid configuration
	validConfig := map[string]interface{}{
		"host":       "192.168.1.1",
		"api_key":    "test_key",
		"api_secret": "test_secret",
		"port":       float64(443),
		"timeout":    float64(30),
	}

	err := exporter.ValidateConfig(validConfig)
	if err != nil {
		t.Errorf("Valid configuration should not return error, got: %v", err)
	}

	// Test missing required field
	invalidConfig := map[string]interface{}{
		"host":    "192.168.1.1",
		"api_key": "test_key",
		// api_secret missing
	}

	err = exporter.ValidateConfig(invalidConfig)
	if err == nil {
		t.Error("Missing api_secret should return validation error")
	}

	// Test invalid sync_mode
	invalidSyncMode := map[string]interface{}{
		"host":       "192.168.1.1",
		"api_key":    "test_key",
		"api_secret": "test_secret",
		"sync_mode":  "invalid_mode",
	}

	err = exporter.ValidateConfig(invalidSyncMode)
	if err == nil {
		t.Error("Invalid sync_mode should return validation error")
	}

	// Test invalid port range
	invalidPort := map[string]interface{}{
		"host":       "192.168.1.1",
		"api_key":    "test_key",
		"api_secret": "test_secret",
		"port":       float64(99999),
	}

	err = exporter.ValidateConfig(invalidPort)
	if err == nil {
		t.Error("Invalid port should return validation error")
	}
}

func TestOPNSenseExporter_Preview(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	exporter := NewOPNSenseExporter()
	if err := exporter.Initialize(logger); err != nil {
		t.Logf("Failed to initialize exporter: %v", err)
	}

	// Create test data
	testData := &sync.ExportData{
		Devices: []sync.DeviceData{
			{
				ID:   1,
				MAC:  "aa:bb:cc:dd:ee:ff",
				IP:   "192.168.1.100",
				Type: "ShellyPlus1",
				Name: "TestShelly1",
			},
			{
				ID:   2,
				MAC:  "11:22:33:44:55:66",
				IP:   "192.168.1.101",
				Type: "ShellyPlus2PM",
				Name: "TestShelly2",
			},
		},
		Metadata: sync.ExportMetadata{
			TotalDevices: 2,
		},
	}

	config := sync.ExportConfig{
		Format: "dhcp_reservations",
		Config: map[string]interface{}{
			"host":       "192.168.1.1",
			"api_key":    "test_key",
			"api_secret": "test_secret",
		},
	}

	ctx := context.Background()
	preview, err := exporter.Preview(ctx, testData, config)
	if err != nil {
		t.Errorf("Preview should not return error, got: %v", err)
	}

	if !preview.Success {
		t.Error("Preview should be successful")
	}

	if preview.RecordCount != 2 {
		t.Errorf("Expected record count 2, got %d", preview.RecordCount)
	}

	if len(preview.SampleData) == 0 {
		t.Error("Preview should contain sample data")
	}
}

func TestOPNSenseExporter_Capabilities(t *testing.T) {
	exporter := NewOPNSenseExporter()
	capabilities := exporter.Capabilities()

	if !capabilities.SupportsIncremental {
		t.Error("OPNSense exporter should support incremental exports")
	}

	if !capabilities.SupportsScheduling {
		t.Error("OPNSense exporter should support scheduling")
	}

	if !capabilities.RequiresAuthentication {
		t.Error("OPNSense exporter should require authentication")
	}

	if capabilities.ConcurrencyLevel != 1 {
		t.Errorf("Expected concurrency level 1, got %d", capabilities.ConcurrencyLevel)
	}

	expectedOutputs := []string{"api", "file"}
	if len(capabilities.SupportedOutputs) != len(expectedOutputs) {
		t.Errorf("Expected %d supported outputs, got %d", len(expectedOutputs), len(capabilities.SupportedOutputs))
	}
}

func TestOPNSenseExporter_GenerateHostname(t *testing.T) {
	exporter := NewOPNSenseExporter()

	device := sync.DeviceData{
		MAC:  "aa:bb:cc:dd:ee:ff",
		Type: "ShellyPlus1PM",
		Name: "Kitchen Light",
	}

	tests := []struct {
		template string
		expected string
	}{
		{
			template: "shelly-{{.Type}}-{{.MAC | last4}}",
			expected: "shelly-shellyplus1pm-eeff",
		},
		{
			template: "{{.Name}}-device",
			expected: "kitchen-light-device",
		},
		{
			template: "shelly-{{.MAC | last4}}",
			expected: "shelly-eeff",
		},
	}

	for _, test := range tests {
		result := exporter.generateHostname(device, test.template)
		if result != test.expected {
			t.Errorf("Template '%s': expected '%s', got '%s'", test.template, test.expected, result)
		}
	}
}

func TestOPNSenseExporter_SanitizeHostname(t *testing.T) {
	exporter := NewOPNSenseExporter()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Valid-Hostname123",
			expected: "valid-hostname123",
		},
		{
			input:    "Invalid@Hostname#With$Special%Characters",
			expected: "invalid-hostname-with-special-characters",
		},
		{
			input:    "---leading-and-trailing-hyphens---",
			expected: "leading-and-trailing-hyphens",
		},
		{
			input:    "",
			expected: "shelly-device",
		},
		{
			input:    "Very-Long-Hostname-That-Exceeds-The-Maximum-Length-Of-Sixty-Three-Characters-Total",
			expected: "very-long-hostname-that-exceeds-the-maximum-length-of-sixty-thr",
		},
	}

	for _, test := range tests {
		result := exporter.sanitizeHostname(test.input)
		if result != test.expected {
			t.Errorf("Input '%s': expected '%s', got '%s'", test.input, test.expected, result)
		}

		// Ensure result is not longer than 63 characters
		if len(result) > 63 {
			t.Errorf("Sanitized hostname '%s' exceeds 63 characters", result)
		}
	}
}

func TestOPNSenseExporter_ConvertToDeviceMappings(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	exporter := NewOPNSenseExporter()
	if err := exporter.Initialize(logger); err != nil {
		t.Logf("Failed to initialize exporter: %v", err)
	}

	testData := &sync.ExportData{
		Devices: []sync.DeviceData{
			{
				MAC:  "aa:bb:cc:dd:ee:ff",
				IP:   "192.168.1.100",
				Type: "ShellyPlus1",
				Name: "TestShelly1",
			},
			{
				MAC:  "11:22:33:44:55:66",
				IP:   "192.168.1.101",
				Type: "ShellyPlus2PM",
				Name: "TestShelly2",
			},
		},
		DiscoveredDevices: []sync.DiscoveredDeviceData{
			{
				MAC:   "77:88:99:aa:bb:cc",
				IP:    "192.168.1.102",
				Model: "ShellyPlus1",
			},
		},
	}

	config := map[string]interface{}{
		"hostname_template":  "shelly-{{.Type}}-{{.MAC | last4}}",
		"dhcp_interface":     "lan",
		"include_discovered": true,
	}

	mappings := exporter.convertToDeviceMappings(testData, config)

	// Should include 2 configured devices + 1 discovered device
	expectedCount := 3
	if len(mappings) != expectedCount {
		t.Errorf("Expected %d mappings, got %d", expectedCount, len(mappings))
	}

	// Check first device
	firstMapping := mappings[0]
	if firstMapping.ShellyMAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Expected MAC 'aa:bb:cc:dd:ee:ff', got %s", firstMapping.ShellyMAC)
	}

	if firstMapping.ShellyIP != "192.168.1.100" {
		t.Errorf("Expected IP '192.168.1.100', got %s", firstMapping.ShellyIP)
	}

	if firstMapping.Interface != "lan" {
		t.Errorf("Expected interface 'lan', got %s", firstMapping.Interface)
	}

	// Check that hostname was generated
	if firstMapping.OPNSenseHostname == "" {
		t.Error("Hostname should not be empty")
	}

	// Check discovered device (should be last)
	discoveredMapping := mappings[2]
	if discoveredMapping.SyncStatus != "discovered" {
		t.Errorf("Expected sync status 'discovered', got %s", discoveredMapping.SyncStatus)
	}
}

func TestOPNSenseExporter_Initialize(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	exporter := NewOPNSenseExporter()

	err := exporter.Initialize(logger)
	if err != nil {
		t.Errorf("Initialize should not return error, got: %v", err)
	}

	if exporter.logger != logger {
		t.Error("Logger should be set after initialization")
	}
}

func TestOPNSenseExporter_Cleanup(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	exporter := NewOPNSenseExporter()
	if err := exporter.Initialize(logger); err != nil {
		t.Logf("Failed to initialize exporter: %v", err)
	}

	err := exporter.Cleanup()
	if err != nil {
		t.Errorf("Cleanup should not return error, got: %v", err)
	}
}

// Benchmark tests
func BenchmarkOPNSenseExporter_ConvertToDeviceMappings(b *testing.B) {
	logger, _ := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	exporter := NewOPNSenseExporter()
	if err := exporter.Initialize(logger); err != nil {
		b.Logf("Failed to initialize exporter: %v", err)
	}

	// Create test data with many devices
	testData := &sync.ExportData{
		Devices: make([]sync.DeviceData, 1000),
	}

	for i := range testData.Devices {
		testData.Devices[i] = sync.DeviceData{
			MAC:  fmt.Sprintf("aa:bb:cc:dd:ee:%02x", i%256),
			IP:   fmt.Sprintf("192.168.1.%d", (i%254)+1),
			Type: "ShellyPlus1",
			Name: fmt.Sprintf("TestShelly%d", i),
		}
	}

	config := map[string]interface{}{
		"hostname_template": "shelly-{{.Type}}-{{.MAC | last4}}",
		"dhcp_interface":    "lan",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mappings := exporter.convertToDeviceMappings(testData, config)
		_ = mappings // Prevent optimization
	}
}

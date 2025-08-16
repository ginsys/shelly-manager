package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	configContent := `server:
  port: 9090
  host: 0.0.0.0
  log_level: "debug"

logging:
  level: "info"
  format: "json"
  output: "stdout"

database:
  path: "/tmp/test.db"

discovery:
  enabled: true
  networks:
    - 192.168.1.0/24
    - 10.0.0.0/8
  interval: 600
  timeout: 10
  enable_mdns: false
  enable_ssdp: true
  concurrent_scans: 50

provisioning:
  auth_enabled: true
  auth_user: "testuser"
  auth_password: "testpass"
  cloud_enabled: true
  mqtt_enabled: true
  mqtt_server: "mqtt.example.com"
  device_name_pattern: "device_{mac}"
  auto_provision: true
  provision_interval: 300

dhcp:
  network: 10.0.0.0/24
  start_ip: 10.0.0.100
  end_ip: 10.0.0.200
  auto_reserve: true

opnsense:
  enabled: true
  host: 192.168.1.1
  port: 8443
  api_key: "testkey"
  api_secret: "testsecret"
  auto_apply: true

main_app:
  url: "http://example.com:8080"
  api_key: "appkey"
  enabled: false
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify server config
	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected host 0.0.0.0, got %s", config.Server.Host)
	}
	if config.Server.LogLevel != "debug" {
		t.Errorf("Expected log level debug, got %s", config.Server.LogLevel)
	}

	// Verify logging config
	if config.Logging.Level != "info" {
		t.Errorf("Expected logging level info, got %s", config.Logging.Level)
	}
	if config.Logging.Format != "json" {
		t.Errorf("Expected logging format json, got %s", config.Logging.Format)
	}
	if config.Logging.Output != "stdout" {
		t.Errorf("Expected logging output stdout, got %s", config.Logging.Output)
	}

	// Verify database config
	if config.Database.Path != "/tmp/test.db" {
		t.Errorf("Expected database path /tmp/test.db, got %s", config.Database.Path)
	}

	// Verify discovery config
	if !config.Discovery.Enabled {
		t.Error("Expected discovery to be enabled")
	}
	expectedNetworks := []string{"192.168.1.0/24", "10.0.0.0/8"}
	if len(config.Discovery.Networks) != len(expectedNetworks) {
		t.Errorf("Expected %d networks, got %d", len(expectedNetworks), len(config.Discovery.Networks))
	}
	for i, network := range expectedNetworks {
		if config.Discovery.Networks[i] != network {
			t.Errorf("Expected network %s, got %s", network, config.Discovery.Networks[i])
		}
	}
	if config.Discovery.Interval != 600 {
		t.Errorf("Expected discovery interval 600, got %d", config.Discovery.Interval)
	}
	if config.Discovery.Timeout != 10 {
		t.Errorf("Expected discovery timeout 10, got %d", config.Discovery.Timeout)
	}
	if config.Discovery.EnableMDNS {
		t.Error("Expected mDNS to be disabled")
	}
	if !config.Discovery.EnableSSDP {
		t.Error("Expected SSDP to be enabled")
	}
	if config.Discovery.ConcurrentScans != 50 {
		t.Errorf("Expected concurrent scans 50, got %d", config.Discovery.ConcurrentScans)
	}

	// Verify provisioning config
	if !config.Provisioning.AuthEnabled {
		t.Error("Expected provisioning auth to be enabled")
	}
	if config.Provisioning.AuthUser != "testuser" {
		t.Errorf("Expected auth user testuser, got %s", config.Provisioning.AuthUser)
	}
	if config.Provisioning.AuthPassword != "testpass" {
		t.Errorf("Expected auth password testpass, got %s", config.Provisioning.AuthPassword)
	}
	if !config.Provisioning.CloudEnabled {
		t.Error("Expected cloud to be enabled")
	}
	if !config.Provisioning.MQTTEnabled {
		t.Error("Expected MQTT to be enabled")
	}
	if config.Provisioning.MQTTServer != "mqtt.example.com" {
		t.Errorf("Expected MQTT server mqtt.example.com, got %s", config.Provisioning.MQTTServer)
	}
	if config.Provisioning.DeviceNamePattern != "device_{mac}" {
		t.Errorf("Expected device name pattern device_{mac}, got %s", config.Provisioning.DeviceNamePattern)
	}
	if !config.Provisioning.AutoProvision {
		t.Error("Expected auto provision to be enabled")
	}
	if config.Provisioning.ProvisionInterval != 300 {
		t.Errorf("Expected provision interval 300, got %d", config.Provisioning.ProvisionInterval)
	}

	// Verify DHCP config
	if config.DHCP.Network != "10.0.0.0/24" {
		t.Errorf("Expected DHCP network 10.0.0.0/24, got %s", config.DHCP.Network)
	}
	if config.DHCP.StartIP != "10.0.0.100" {
		t.Errorf("Expected DHCP start IP 10.0.0.100, got %s", config.DHCP.StartIP)
	}
	if config.DHCP.EndIP != "10.0.0.200" {
		t.Errorf("Expected DHCP end IP 10.0.0.200, got %s", config.DHCP.EndIP)
	}
	if !config.DHCP.AutoReserve {
		t.Error("Expected DHCP auto reserve to be enabled")
	}

	// Verify OPNSense config
	if !config.OPNSense.Enabled {
		t.Error("Expected OPNSense to be enabled")
	}
	if config.OPNSense.Host != "192.168.1.1" {
		t.Errorf("Expected OPNSense host 192.168.1.1, got %s", config.OPNSense.Host)
	}
	if config.OPNSense.Port != 8443 {
		t.Errorf("Expected OPNSense port 8443, got %d", config.OPNSense.Port)
	}
	if config.OPNSense.APIKey != "testkey" {
		t.Errorf("Expected OPNSense API key testkey, got %s", config.OPNSense.APIKey)
	}
	if config.OPNSense.APISecret != "testsecret" {
		t.Errorf("Expected OPNSense API secret testsecret, got %s", config.OPNSense.APISecret)
	}
	if !config.OPNSense.AutoApply {
		t.Error("Expected OPNSense auto apply to be enabled")
	}

	// Verify main app config
	if config.MainApp.URL != "http://example.com:8080" {
		t.Errorf("Expected main app URL http://example.com:8080, got %s", config.MainApp.URL)
	}
	if config.MainApp.APIKey != "appkey" {
		t.Errorf("Expected main app API key appkey, got %s", config.MainApp.APIKey)
	}
	if config.MainApp.Enabled {
		t.Error("Expected main app to be disabled")
	}
}

func TestLoad_DefaultConfig(t *testing.T) {
	// Create minimal config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal-config.yaml")

	configContent := `# Minimal config file
server:
  port: 8080
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify defaults are applied
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}
	if config.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host 0.0.0.0, got %s", config.Server.Host)
	}
	if config.Server.LogLevel != "info" {
		t.Errorf("Expected default log level info, got %s", config.Server.LogLevel)
	}

	// Verify logging defaults
	if config.Logging.Level != "info" {
		t.Errorf("Expected default logging level info, got %s", config.Logging.Level)
	}
	if config.Logging.Format != "text" {
		t.Errorf("Expected default logging format text, got %s", config.Logging.Format)
	}
	if config.Logging.Output != "stdout" {
		t.Errorf("Expected default logging output stdout, got %s", config.Logging.Output)
	}

	// Verify database defaults
	if config.Database.Path != "data/shelly.db" {
		t.Errorf("Expected default database path data/shelly.db, got %s", config.Database.Path)
	}

	// Verify discovery defaults
	if !config.Discovery.Enabled {
		t.Error("Expected discovery to be enabled by default")
	}
	expectedDefaultNetworks := []string{"192.168.1.0/24"}
	if len(config.Discovery.Networks) != len(expectedDefaultNetworks) {
		t.Errorf("Expected %d default networks, got %d", len(expectedDefaultNetworks), len(config.Discovery.Networks))
	}
	if config.Discovery.Networks[0] != expectedDefaultNetworks[0] {
		t.Errorf("Expected default network %s, got %s", expectedDefaultNetworks[0], config.Discovery.Networks[0])
	}
	if config.Discovery.Interval != 300 {
		t.Errorf("Expected default discovery interval 300, got %d", config.Discovery.Interval)
	}
	if config.Discovery.Timeout != 5 {
		t.Errorf("Expected default discovery timeout 5, got %d", config.Discovery.Timeout)
	}
	if !config.Discovery.EnableMDNS {
		t.Error("Expected mDNS to be enabled by default")
	}
	if !config.Discovery.EnableSSDP {
		t.Error("Expected SSDP to be enabled by default")
	}
	if config.Discovery.ConcurrentScans != 20 {
		t.Errorf("Expected default concurrent scans 20, got %d", config.Discovery.ConcurrentScans)
	}
}

func TestLoad_EmptyConfigPath(t *testing.T) {
	// Test loading with empty config path (should use default search paths)
	// This will likely fail since there's no default config file, but we're testing the behavior
	config, err := Load("")

	// The behavior depends on whether a default config file exists
	// In test environments, this usually fails, but it's valid behavior
	if err != nil {
		// Error should mention config file
		if !contains(err.Error(), "config") {
			t.Errorf("Expected error message to mention config file, got: %s", err.Error())
		}
	} else {
		// If it succeeds, it found a default config file somewhere
		t.Log("Load with empty path succeeded (found default config)")

		// Verify the loaded config has reasonable defaults to ensure it parsed correctly
		if config == nil {
			t.Error("Config should not be nil when load succeeds")
		} else {
			// Basic sanity checks to ensure the config parsed correctly
			if config.Server.Port <= 0 || config.Server.Port > 65535 {
				t.Errorf("Invalid server port: %d", config.Server.Port)
			}
			if config.Database.Path == "" {
				t.Error("Database path should not be empty")
			}
		}
	}
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	// Create invalid YAML file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid-config.yaml")

	invalidContent := `server:
  port: 8080
  invalid_yaml: [
    missing_closing_bracket
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	// Load config
	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error when loading invalid config file")
	}

	// Error should mention config parsing failure
	if err != nil && !contains(err.Error(), "config") {
		t.Errorf("Expected error message to mention config, got: %s", err.Error())
	}
}

func TestLoad_NonexistentConfigFile(t *testing.T) {
	// Test loading nonexistent config file
	_, err := Load("/path/that/does/not/exist/config.yaml")
	if err == nil {
		t.Error("Expected error when loading nonexistent config file")
	}

	// Error should mention file not found
	if err != nil && !contains(err.Error(), "config") {
		t.Errorf("Expected error message to mention config file, got: %s", err.Error())
	}
}

func TestLoad_EmptyConfigFile(t *testing.T) {
	// Create empty config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty-config.yaml")

	err := os.WriteFile(configPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write empty config file: %v", err)
	}

	// Load config - should work with defaults
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load empty config: %v", err)
	}

	// Verify defaults are applied
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}
	if config.Database.Path != "data/shelly.db" {
		t.Errorf("Expected default database path, got %s", config.Database.Path)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	// Create config with only some sections
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "partial-config.yaml")

	configContent := `database:
  path: "/custom/path.db"

discovery:
  networks:
    - 172.16.0.0/12
    - 10.10.10.0/24
  timeout: 15
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write partial config file: %v", err)
	}

	// Load config
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load partial config: %v", err)
	}

	// Verify specified values
	if config.Database.Path != "/custom/path.db" {
		t.Errorf("Expected custom database path /custom/path.db, got %s", config.Database.Path)
	}

	expectedNetworks := []string{"172.16.0.0/12", "10.10.10.0/24"}
	if len(config.Discovery.Networks) != len(expectedNetworks) {
		t.Errorf("Expected %d networks, got %d", len(expectedNetworks), len(config.Discovery.Networks))
	}
	for i, network := range expectedNetworks {
		if config.Discovery.Networks[i] != network {
			t.Errorf("Expected network %s, got %s", network, config.Discovery.Networks[i])
		}
	}

	if config.Discovery.Timeout != 15 {
		t.Errorf("Expected custom discovery timeout 15, got %d", config.Discovery.Timeout)
	}

	// Verify defaults for unspecified values
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}
	if config.Discovery.Interval != 300 {
		t.Errorf("Expected default discovery interval 300, got %d", config.Discovery.Interval)
	}
	if !config.Discovery.Enabled {
		t.Error("Expected discovery to be enabled by default")
	}
}

func TestSetDefaults(t *testing.T) {
	// Test that setDefaults function sets expected values
	// This is implicitly tested in other tests, but we can verify specific defaults

	// Create minimal config to trigger defaults
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-defaults.yaml")

	err := os.WriteFile(configPath, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to write minimal config file: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test all default values
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
	}{
		{"server.port", config.Server.Port, 8080},
		{"server.host", config.Server.Host, "0.0.0.0"},
		{"server.log_level", config.Server.LogLevel, "info"},
		{"logging.level", config.Logging.Level, "info"},
		{"logging.format", config.Logging.Format, "text"},
		{"logging.output", config.Logging.Output, "stdout"},
		{"database.path", config.Database.Path, "data/shelly.db"},
		{"discovery.enabled", config.Discovery.Enabled, true},
		{"discovery.interval", config.Discovery.Interval, 300},
		{"discovery.timeout", config.Discovery.Timeout, 5},
		{"discovery.enable_mdns", config.Discovery.EnableMDNS, true},
		{"discovery.enable_ssdp", config.Discovery.EnableSSDP, true},
		{"discovery.concurrent_scans", config.Discovery.ConcurrentScans, 20},
		{"provisioning.auth_enabled", config.Provisioning.AuthEnabled, false},
		{"provisioning.auth_user", config.Provisioning.AuthUser, "admin"},
		{"provisioning.cloud_enabled", config.Provisioning.CloudEnabled, false},
		{"provisioning.mqtt_enabled", config.Provisioning.MQTTEnabled, false},
		{"provisioning.device_name_pattern", config.Provisioning.DeviceNamePattern, "shelly_{type}_{mac}"},
		{"provisioning.auto_provision", config.Provisioning.AutoProvision, false},
		{"provisioning.provision_interval", config.Provisioning.ProvisionInterval, 600},
		{"dhcp.network", config.DHCP.Network, "192.168.1.0/24"},
		{"dhcp.start_ip", config.DHCP.StartIP, "192.168.1.100"},
		{"dhcp.end_ip", config.DHCP.EndIP, "192.168.1.199"},
		{"dhcp.auto_reserve", config.DHCP.AutoReserve, false},
		{"opnsense.enabled", config.OPNSense.Enabled, false},
		{"opnsense.port", config.OPNSense.Port, 443},
		{"opnsense.auto_apply", config.OPNSense.AutoApply, false},
		{"main_app.url", config.MainApp.URL, "http://localhost:8080"},
		{"main_app.enabled", config.MainApp.Enabled, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.actual != test.expected {
				t.Errorf("Expected %s = %v, got %v", test.name, test.expected, test.actual)
			}
		})
	}

	// Test default networks array
	expectedDefaultNetworks := []string{"192.168.1.0/24"}
	if len(config.Discovery.Networks) != len(expectedDefaultNetworks) {
		t.Errorf("Expected %d default networks, got %d", len(expectedDefaultNetworks), len(config.Discovery.Networks))
	}
	for i, network := range expectedDefaultNetworks {
		if config.Discovery.Networks[i] != network {
			t.Errorf("Expected default network %s, got %s", network, config.Discovery.Networks[i])
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

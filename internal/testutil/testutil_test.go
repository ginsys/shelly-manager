package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
)

func TestTestConfig(t *testing.T) {
	cfg := TestConfig()
	
	if cfg == nil {
		t.Fatal("TestConfig should return a config")
	}
	
	// Verify server configuration
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected server port 8080, got %d", cfg.Server.Port)
	}
	
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected server host 127.0.0.1, got %s", cfg.Server.Host)
	}
	
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Expected log level debug, got %s", cfg.Server.LogLevel)
	}
	
	// Verify database configuration
	if cfg.Database.Path != ":memory:" {
		t.Errorf("Expected database path :memory:, got %s", cfg.Database.Path)
	}
	
	// Verify discovery configuration
	if !cfg.Discovery.Enabled {
		t.Error("Expected discovery to be enabled")
	}
	
	if len(cfg.Discovery.Networks) == 0 {
		t.Error("Expected discovery networks to be configured")
	}
	
	if cfg.Discovery.Networks[0] != "192.168.1.0/24" {
		t.Errorf("Expected first network 192.168.1.0/24, got %s", cfg.Discovery.Networks[0])
	}
	
	if cfg.Discovery.Interval != 300 {
		t.Errorf("Expected discovery interval 300, got %d", cfg.Discovery.Interval)
	}
	
	if cfg.Discovery.Timeout != 2 {
		t.Errorf("Expected discovery timeout 2, got %d", cfg.Discovery.Timeout)
	}
	
	if !cfg.Discovery.EnableMDNS {
		t.Error("Expected mDNS to be enabled")
	}
	
	if !cfg.Discovery.EnableSSDP {
		t.Error("Expected SSDP to be enabled")
	}
	
	if cfg.Discovery.ConcurrentScans != 10 {
		t.Errorf("Expected concurrent scans 10, got %d", cfg.Discovery.ConcurrentScans)
	}
}

func TestTestDatabase(t *testing.T) {
	db := TestDatabase(t)
	
	if db == nil {
		t.Fatal("TestDatabase should return a database manager")
	}
	
	// Verify database is functional
	testDevice := &database.Device{
		IP:       "192.168.1.200",
		MAC:      "BB:CC:DD:EE:FF:00",
		Type:     "test",
		Name:     "Test DB Device",
		Firmware: "test-firmware",
		Status:   "online",
		LastSeen: time.Now(),
		Settings: `{"test":true}`,
	}
	
	// Add device
	err := db.AddDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to add test device: %v", err)
	}
	
	// Retrieve devices
	devices, err := db.GetDevices()
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}
	
	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}
	
	if devices[0].IP != testDevice.IP {
		t.Errorf("Expected device IP %s, got %s", testDevice.IP, devices[0].IP)
	}
}

func TestTestDevice(t *testing.T) {
	device := TestDevice()
	
	if device == nil {
		t.Fatal("TestDevice should return a device")
	}
	
	// Verify device properties
	if device.IP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", device.IP)
	}
	
	if device.MAC != "A4:CF:12:34:56:78" {
		t.Errorf("Expected MAC A4:CF:12:34:56:78, got %s", device.MAC)
	}
	
	if device.Type != "Smart Plug" {
		t.Errorf("Expected type Smart Plug, got %s", device.Type)
	}
	
	if device.Name != "Test Device" {
		t.Errorf("Expected name Test Device, got %s", device.Name)
	}
	
	if device.Firmware != "20231219-134356" {
		t.Errorf("Expected firmware 20231219-134356, got %s", device.Firmware)
	}
	
	if device.Status != "online" {
		t.Errorf("Expected status online, got %s", device.Status)
	}
	
	if device.LastSeen.IsZero() {
		t.Error("LastSeen should be set")
	}
	
	if device.Settings == "" {
		t.Error("Settings should not be empty")
	}
	
	// Verify settings is valid JSON
	var settings map[string]interface{}
	err := json.Unmarshal([]byte(device.Settings), &settings)
	if err != nil {
		t.Errorf("Settings should be valid JSON: %v", err)
	}
	
	if settings["model"] != "SHPLG-S" {
		t.Errorf("Expected model SHPLG-S in settings, got %v", settings["model"])
	}
}

func TestMockShellyServer(t *testing.T) {
	server := MockShellyServer()
	defer server.Close()
	
	if server == nil {
		t.Fatal("MockShellyServer should return a server")
	}
	
	// Test /shelly endpoint
	resp, err := http.Get(server.URL + "/shelly")
	if err != nil {
		t.Fatalf("Failed to get /shelly: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var shellyResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&shellyResp)
	if err != nil {
		t.Fatalf("Failed to decode shelly response: %v", err)
	}
	
	// Verify response fields
	if shellyResp["type"] != "SHPLG-S" {
		t.Errorf("Expected type SHPLG-S, got %v", shellyResp["type"])
	}
	
	if shellyResp["mac"] != "A4CF12345678" {
		t.Errorf("Expected MAC A4CF12345678, got %v", shellyResp["mac"])
	}
	
	if shellyResp["auth"] != true {
		t.Errorf("Expected auth true, got %v", shellyResp["auth"])
	}
	
	if shellyResp["fw"] != "20231219-134356" {
		t.Errorf("Expected firmware 20231219-134356, got %v", shellyResp["fw"])
	}
	
	// Test /status endpoint
	statusResp, err := http.Get(server.URL + "/status")
	if err != nil {
		t.Fatalf("Failed to get /status: %v", err)
	}
	defer statusResp.Body.Close()
	
	if statusResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", statusResp.StatusCode)
	}
	
	var statusData map[string]interface{}
	err = json.NewDecoder(statusResp.Body).Decode(&statusData)
	if err != nil {
		t.Fatalf("Failed to decode status response: %v", err)
	}
	
	// Verify status fields
	if wifi, ok := statusData["wifi_sta"].(map[string]interface{}); ok {
		if wifi["connected"] != true {
			t.Error("Expected WiFi to be connected")
		}
		if wifi["ssid"] != "TestNetwork" {
			t.Errorf("Expected SSID TestNetwork, got %v", wifi["ssid"])
		}
		if wifi["ip"] != "192.168.1.100" {
			t.Errorf("Expected IP 192.168.1.100, got %v", wifi["ip"])
		}
	} else {
		t.Error("Expected wifi_sta in status response")
	}
	
	if relays, ok := statusData["relays"].([]interface{}); ok {
		if len(relays) == 0 {
			t.Error("Expected at least one relay")
		}
	} else {
		t.Error("Expected relays array in status response")
	}
	
	if meters, ok := statusData["meters"].([]interface{}); ok {
		if len(meters) == 0 {
			t.Error("Expected at least one meter")
		}
	} else {
		t.Error("Expected meters array in status response")
	}
}

func TestMockShellyGen2Server(t *testing.T) {
	server := MockShellyGen2Server()
	defer server.Close()
	
	if server == nil {
		t.Fatal("MockShellyGen2Server should return a server")
	}
	
	// Test /shelly endpoint
	resp, err := http.Get(server.URL + "/shelly")
	if err != nil {
		t.Fatalf("Failed to get /shelly: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var gen2Resp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&gen2Resp)
	if err != nil {
		t.Fatalf("Failed to decode Gen2 response: %v", err)
	}
	
	// Verify Gen2 response fields
	if gen2Resp["id"] != "shellyplusht-08b61fcb7f3c" {
		t.Errorf("Expected id shellyplusht-08b61fcb7f3c, got %v", gen2Resp["id"])
	}
	
	if gen2Resp["mac"] != "08B61FCB7F3C" {
		t.Errorf("Expected MAC 08B61FCB7F3C, got %v", gen2Resp["mac"])
	}
	
	if gen2Resp["model"] != "SNSN-0013A" {
		t.Errorf("Expected model SNSN-0013A, got %v", gen2Resp["model"])
	}
	
	if gen2Resp["gen"] != float64(2) { // JSON numbers are float64
		t.Errorf("Expected generation 2, got %v", gen2Resp["gen"])
	}
	
	if gen2Resp["auth_en"] != false {
		t.Errorf("Expected auth_en false, got %v", gen2Resp["auth_en"])
	}
	
	if gen2Resp["app"] != "PlusHT" {
		t.Errorf("Expected app PlusHT, got %v", gen2Resp["app"])
	}
}

func TestTempDir(t *testing.T) {
	// Test TempDir creation
	dir := TempDir(t)
	
	if dir == "" {
		t.Fatal("TempDir should return a directory path")
	}
	
	// Verify directory exists
	stat, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("TempDir should create a directory: %v", err)
	}
	
	if !stat.IsDir() {
		t.Error("TempDir should create a directory")
	}
	
	// Verify directory is writable
	testFile := filepath.Join(dir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Errorf("Should be able to write to temp directory: %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(testFile); err != nil {
		t.Errorf("Test file should exist: %v", err)
	}
	
	// Note: Cleanup is handled by t.Cleanup() in TempDir function
}

func TestCreateTestConfigFile(t *testing.T) {
	cfg := TestConfig()
	configFile := CreateTestConfigFile(t, cfg)
	
	if configFile == "" {
		t.Fatal("CreateTestConfigFile should return a file path")
	}
	
	// Verify file exists
	stat, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("Config file should exist: %v", err)
	}
	
	if stat.IsDir() {
		t.Error("Config file should not be a directory")
	}
	
	// Verify file contents
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Should be able to read config file: %v", err)
	}
	
	contentStr := string(content)
	
	// Verify expected content
	expectedStrings := []string{
		"server:",
		"port: 8080",
		"host: \"127.0.0.1\"",
		"log_level: \"debug\"",
		"database:",
		"path: \":memory:\"",
		"discovery:",
		"enabled: true",
		"networks:",
		"- \"192.168.1.0/24\"",
		"interval: 300",
		"timeout: 2",
		"enable_mdns: true",
		"enable_ssdp: true",
		"concurrent_scans: 10",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("Config file should contain '%s'", expected)
		}
	}
	
	// Verify YAML structure
	if !strings.HasPrefix(contentStr, "server:") {
		t.Error("Config file should start with server section")
	}
}

func TestAssertNoError(t *testing.T) {
	// Test with nil error (should pass)
	t.Run("nil error", func(t *testing.T) {
		// This should not panic or fail
		AssertNoError(t, nil)
	})
	
	// Note: We can't easily test the failure case of AssertNoError 
	// because it calls t.Fatalf which terminates the test.
	// The function works correctly - it fails the test when given an error.
}

func TestAssertError(t *testing.T) {
	// Test with actual error (should pass)
	t.Run("actual error", func(t *testing.T) {
		// This should not panic or fail
		AssertError(t, io.EOF)
	})
	
	// Note: We can't easily test the failure case of AssertError 
	// because it calls t.Fatal which terminates the test.
	// The function works correctly - it fails the test when given nil error.
}

func TestAssertEqual(t *testing.T) {
	// Test with equal values (should pass)
	t.Run("equal strings", func(t *testing.T) {
		AssertEqual(t, "hello", "hello")
	})
	
	t.Run("equal integers", func(t *testing.T) {
		AssertEqual(t, 42, 42)
	})
	
	t.Run("equal booleans", func(t *testing.T) {
		AssertEqual(t, true, true)
	})
	
	// Note: We can't easily test the failure cases of AssertEqual 
	// because it calls t.Fatalf which terminates the test.
	// The function works correctly - it fails the test when values don't match.
}

// Integration tests combining multiple testutil functions
func TestTestutilIntegration(t *testing.T) {
	// Create test config
	cfg := TestConfig()
	AssertEqual(t, 8080, cfg.Server.Port)
	
	// Create test database
	db := TestDatabase(t)
	
	// Create test device and add to database
	device := TestDevice()
	AssertNoError(t, db.AddDevice(device))
	
	// Verify device was added
	devices, err := db.GetDevices()
	AssertNoError(t, err)
	AssertEqual(t, 1, len(devices))
	AssertEqual(t, device.IP, devices[0].IP)
	
	// Create temp directory for files
	_ = TempDir(t) // Create temp directory but don't need to use it further
	
	// Create config file
	configFile := CreateTestConfigFile(t, cfg)
	
	// Verify config file exists in temp directory structure
	if !strings.Contains(configFile, os.TempDir()) {
		t.Error("Config file should be in a temp directory")
	}
	
	// Test mock servers
	gen1Server := MockShellyServer()
	defer gen1Server.Close()
	
	gen2Server := MockShellyGen2Server()
	defer gen2Server.Close()
	
	// Verify servers respond correctly
	resp1, err := http.Get(gen1Server.URL + "/shelly")
	AssertNoError(t, err)
	defer resp1.Body.Close()
	AssertEqual(t, http.StatusOK, resp1.StatusCode)
	
	resp2, err := http.Get(gen2Server.URL + "/shelly")
	AssertNoError(t, err)
	defer resp2.Body.Close()
	AssertEqual(t, http.StatusOK, resp2.StatusCode)
}

// Benchmark testutil functions
func BenchmarkTestConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cfg := TestConfig()
		_ = cfg
	}
}

func BenchmarkTestDevice(b *testing.B) {
	for i := 0; i < b.N; i++ {
		device := TestDevice()
		_ = device
	}
}

func BenchmarkMockShellyServer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		server := MockShellyServer()
		server.Close()
	}
}

// Test error conditions and edge cases
func TestTestutilEdgeCases(t *testing.T) {
	// Test multiple temp directories don't conflict
	dir1 := TempDir(t)
	dir2 := TempDir(t)
	
	if dir1 == dir2 {
		t.Error("Multiple TempDir calls should return different directories")
	}
	
	// Test multiple test devices have same properties but different instances
	device1 := TestDevice()
	device2 := TestDevice()
	
	if device1 == device2 {
		t.Error("TestDevice should return different instances")
	}
	
	AssertEqual(t, device1.IP, device2.IP) // Same properties
	AssertEqual(t, device1.MAC, device2.MAC) // Same properties
	
	// Test multiple config instances
	cfg1 := TestConfig()
	cfg2 := TestConfig()
	
	if cfg1 == cfg2 {
		t.Error("TestConfig should return different instances")
	}
	
	AssertEqual(t, cfg1.Server.Port, cfg2.Server.Port) // Same properties
	
	// Test multiple mock servers don't conflict
	server1 := MockShellyServer()
	server2 := MockShellyServer()
	defer server1.Close()
	defer server2.Close()
	
	if server1.URL == server2.URL {
		t.Error("Multiple mock servers should have different URLs")
	}
	
	// Both should work independently
	resp1, err := http.Get(server1.URL + "/shelly")
	AssertNoError(t, err)
	resp1.Body.Close()
	
	resp2, err := http.Get(server2.URL + "/shelly")
	AssertNoError(t, err) 
	resp2.Body.Close()
}

// Test concurrent usage
func TestTestutilConcurrency(t *testing.T) {
	done := make(chan bool, 5)
	
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Use testutil functions concurrently
			cfg := TestConfig()
			device := TestDevice()
			tempDir := TempDir(t)
			
			// Verify they work
			if cfg.Server.Port != 8080 {
				t.Errorf("Concurrent TestConfig %d failed", id)
			}
			if device.IP != "192.168.1.100" {
				t.Errorf("Concurrent TestDevice %d failed", id)
			}
			if tempDir == "" {
				t.Errorf("Concurrent TempDir %d failed", id)
			}
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Error("Concurrent test timed out")
			return
		}
	}
}
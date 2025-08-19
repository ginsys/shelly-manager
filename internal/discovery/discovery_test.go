package discovery

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/testutil"
)

func TestNewScanner(t *testing.T) {
	tests := []struct {
		name            string
		timeout         time.Duration
		concurrentScans int
		expectedTimeout time.Duration
		expectedConc    int
	}{
		{
			name:            "default values",
			timeout:         0,
			concurrentScans: 0,
			expectedTimeout: 1 * time.Second,
			expectedConc:    10,
		},
		{
			name:            "custom values",
			timeout:         5 * time.Second,
			concurrentScans: 20,
			expectedTimeout: 5 * time.Second,
			expectedConc:    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewScanner(tt.timeout, tt.concurrentScans)

			if scanner.timeout != tt.expectedTimeout {
				t.Errorf("Expected timeout %v, got %v", tt.expectedTimeout, scanner.timeout)
			}

			if scanner.concurrentScans != tt.expectedConc {
				t.Errorf("Expected concurrent scans %d, got %d", tt.expectedConc, scanner.concurrentScans)
			}

			if scanner.httpClient.Timeout != tt.expectedTimeout {
				t.Errorf("Expected HTTP client timeout %v, got %v", tt.expectedTimeout, scanner.httpClient.Timeout)
			}
		})
	}
}

func TestScanHost_Gen1Device(t *testing.T) {
	// Create mock server for Gen1 device
	server := testutil.MockShellyServer()
	defer server.Close()

	// Parse server URL to get host:port
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	scanner := NewScanner(100*time.Millisecond, 1) // Short timeout for TEST-NET scanning
	ctx := context.Background()

	device, err := scanner.ScanHost(ctx, serverURL.Host)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if device == nil {
		t.Fatal("Expected device to be found")
	}

	// Verify Gen1 device fields
	if device.Type != "SHPLG-S" {
		t.Errorf("Expected type SHPLG-S, got %s", device.Type)
	}

	if device.MAC != "A4CF12345678" {
		t.Errorf("Expected MAC A4CF12345678, got %s", device.MAC)
	}

	if device.Generation != 1 {
		t.Errorf("Expected generation 1, got %d", device.Generation)
	}

	if !device.AuthEn {
		t.Error("Expected auth to be enabled")
	}

	if !strings.Contains(device.ID, "shelly") {
		t.Errorf("Expected ID to contain 'shelly', got %s", device.ID)
	}
}

func TestScanHost_Gen2Device(t *testing.T) {
	// Create mock server for Gen2 device
	server := testutil.MockShellyGen2Server()
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	scanner := NewScanner(100*time.Millisecond, 1) // Short timeout for TEST-NET scanning
	ctx := context.Background()

	device, err := scanner.ScanHost(ctx, serverURL.Host)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if device == nil {
		t.Fatal("Expected device to be found")
	}

	// Verify Gen2 device fields
	if device.ID != "shellyplusht-08b61fcb7f3c" {
		t.Errorf("Expected ID shellyplusht-08b61fcb7f3c, got %s", device.ID)
	}

	if device.MAC != "08B61FCB7F3C" {
		t.Errorf("Expected MAC 08B61FCB7F3C, got %s", device.MAC)
	}

	if device.Generation != 2 {
		t.Errorf("Expected generation 2, got %d", device.Generation)
	}

	if device.Model != "SNSN-0013A" {
		t.Errorf("Expected model SNSN-0013A, got %s", device.Model)
	}
}

func TestScanHost_NoDevice(t *testing.T) {
	// Use improved network test strategy
	config := testutil.DefaultNetworkConfig()
	testutil.SkipNetworkTestIfNeeded(t, config)

	scanner := NewScanner(config.QuickTimeout, 1) // Very short timeout
	ctx, cancel := testutil.CreateNetworkTestContext(config)
	defer cancel()

	// Try to scan a non-routable address (TEST-NET-1 range)
	testAddr := testutil.TestNetworkAddress()
	testutil.AssertTestNetAddress(t, testAddr) // Verify we're using TEST-NET
	device, err := scanner.ScanHost(ctx, testAddr)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if device != nil {
		t.Error("Expected no device to be found")
	}
}

func TestScanHost_Timeout(t *testing.T) {
	// Use improved network test strategy
	config := testutil.DefaultNetworkConfig()
	testutil.SkipNetworkTestIfNeeded(t, config)

	scanner := NewScanner(10*time.Millisecond, 1) // Very short timeout
	ctx := context.Background()

	// Try to scan a non-routable address (TEST-NET-2 range)
	testAddr := "198.51.100.1"
	testutil.AssertTestNetAddress(t, testAddr)     // Verify we're using TEST-NET
	device, err := scanner.ScanHost(ctx, testAddr) // Will timeout
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if device != nil {
		t.Error("Expected no device to be found due to timeout")
	}
}

func TestGetDeviceType(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		// Gen1 exact matches
		{"SHPLG-S", "Smart Plug"},
		{"SHSW-1", "Relay Switch"},
		{"SHSW-PM", "Power Meter Switch"},
		{"SHSW-25", "Dual Relay/Roller"},
		{"SHIX3-1", "3-Input Controller"},
		{"SHDM-1", "Dimmer"},
		{"SHRGBW2", "RGBW Controller"},
		{"SHEM", "Energy Meter"},
		{"SHUNI-1", "Universal Module"},
		{"SHHT-1", "Humidity/Temperature"},

		// Gen2+ patterns
		{"SNSN-0013A", "Plus Sensor"},
		{"SPSW-004PE16EU", "Plus Switch"},

		// Pattern matching
		{"SomePlug", "Smart Plug"},
		{"DeviceWithPM", "Power Meter Device"},
		{"TestDimmer", "Dimmer"},
		{"RGBWLight", "RGBW Light"},
		{"BulbDevice", "Smart Bulb"},
		{"MotionSensor", "Motion Sensor"},
		{"HTSensor", "Humidity/Temperature"},
		{"FloodDetector", "Flood Sensor"},
		{"DoorSensor", "Door/Window Sensor"},
		{"WindowSensor", "Door/Window Sensor"},
		{"SmokeAlarm", "Smoke Detector"},
		{"GasDetector", "Gas Detector"},
		{"EMeter", "Energy Meter"},
		{"RollerShutter", "Roller Shutter"},
		{"ShutterControl", "Roller Shutter"},
		{"ValveController", "Valve Controller"},
		{"I3Controller", "3-Input Controller"},
		{"ButtonDevice", "Button Controller"},
		{"UniController", "Universal Module"},
		{"PlusDevice", "Plus Device"},
		{"ProDevice", "Pro Device"},

		// Unknown device
		{"UnknownDevice", "Shelly Device"},
		{"", "Shelly Device"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := GetDeviceType(tt.model)
			if result != tt.expected {
				t.Errorf("GetDeviceType(%s) = %s, expected %s", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGetDeviceType_CaseInsensitive(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"shplg-s", "Smart Plug"},
		{"SHPLG-S", "Smart Plug"},
		{"ShPlG-s", "Smart Plug"},
		{"shsw-1", "Relay Switch"},
		{"SHSW-1", "Relay Switch"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := GetDeviceType(tt.model)
			if result != tt.expected {
				t.Errorf("GetDeviceType(%s) = %s, expected %s", tt.model, result, tt.expected)
			}
		})
	}
}

func TestScanNetwork_InvalidCIDR(t *testing.T) {
	scanner := NewScanner(1*time.Second, 1)
	ctx := context.Background()

	_, err := scanner.ScanNetwork(ctx, "invalid-cidr")
	if err == nil {
		t.Error("Expected error for invalid CIDR")
	}

	if !strings.Contains(err.Error(), "invalid CIDR") {
		t.Errorf("Expected 'invalid CIDR' in error, got: %v", err)
	}
}

func TestScanNetwork_SmallRange(t *testing.T) {
	// Use improved network test strategy
	config := testutil.DefaultNetworkConfig()
	testutil.SkipNetworkTestIfNeeded(t, config)

	// Create mock server
	server := testutil.MockShellyServer()
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Extract just the port from the server
	parts := strings.Split(serverURL.Host, ":")
	if len(parts) != 2 {
		t.Fatalf("Expected host:port format, got: %s", serverURL.Host)
	}
	port := parts[1]

	// Create a CIDR that includes localhost with the server port
	// This is a bit hacky but allows us to test the scanning logic
	scanner := NewScanner(100*time.Millisecond, 2) // Short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Test with a small range in TEST-NET-3 that won't find anything
	// (since our mock server is on localhost, not in a real network range)
	testCIDR := "203.0.113.0/30"                    // Only 4 IPs in TEST-NET-3
	testutil.AssertTestNetAddress(t, "203.0.113.1") // Verify CIDR is in TEST-NET
	devices, err := scanner.ScanNetwork(ctx, testCIDR)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should find no devices in this range
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(devices))
	}

	// Verify the function completes without hanging
	t.Log("Network scan completed successfully")

	// Suppress unused variable warning
	_ = port
}

func TestGetDeviceStatus_Gen1(t *testing.T) {
	server := testutil.MockShellyServer()
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	scanner := NewScanner(100*time.Millisecond, 1) // Short timeout for TEST-NET scanning
	ctx := context.Background()

	// Test Gen1 device status
	status, err := scanner.GetDeviceStatus(ctx, serverURL.Host, 1)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify some expected fields in the status
	if wifiSta, ok := status["wifi_sta"].(map[string]interface{}); ok {
		if connected, ok := wifiSta["connected"].(bool); !ok || !connected {
			t.Error("Expected wifi_sta.connected to be true")
		}
	} else {
		t.Error("Expected wifi_sta in status response")
	}

	if relays, ok := status["relays"].([]interface{}); ok {
		if len(relays) == 0 {
			t.Error("Expected at least one relay in status")
		}
	} else {
		t.Error("Expected relays in status response")
	}
}

func TestInc(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "simple increment",
			input:    []byte{192, 168, 1, 100},
			expected: []byte{192, 168, 1, 101},
		},
		{
			name:     "overflow single byte",
			input:    []byte{192, 168, 1, 255},
			expected: []byte{192, 168, 2, 0},
		},
		{
			name:     "overflow multiple bytes",
			input:    []byte{192, 168, 255, 255},
			expected: []byte{192, 169, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy to avoid modifying the test data
			ip := make([]byte, len(tt.input))
			copy(ip, tt.input)

			inc(ip)

			for i, expected := range tt.expected {
				if ip[i] != expected {
					t.Errorf("Expected byte %d to be %d, got %d", i, expected, ip[i])
				}
			}
		})
	}
}

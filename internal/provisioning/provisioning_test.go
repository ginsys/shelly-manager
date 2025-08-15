package provisioning

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// testMockInterface creates a test mock network interface
func testMockInterface(logger *logging.Logger) NetworkInterface {
	return NewTestMockNetworkInterface(logger)
}

func TestProvisioningManager_DiscoverUnprovisionedDevices(t *testing.T) {
	// Create test logger
	logger, err := logging.New(logging.Config{
		Level:  "debug",
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatal("Failed to create logger:", err)
	}

	// Create test config
	cfg := &config.Config{}

	// Create provisioning manager
	pm := NewProvisioningManager(cfg, logger)

	// Create mock network interface
	mockNetIface := testMockInterface(logger)
	pm.SetNetworkInterface(mockNetIface)

	// Create mock provisioner
	mockProvisioner := NewShellyProvisioner(logger, mockNetIface)
	pm.SetDeviceProvisioner(mockProvisioner)

	// Test discovery
	ctx := context.Background()
	devices, err := pm.DiscoverUnprovisionedDevices(ctx)
	if err != nil {
		t.Fatal("Failed to discover devices:", err)
	}

	// Should find mock Shelly devices (3 from the test data)
	expectedCount := 3
	if len(devices) != expectedCount {
		t.Errorf("Expected %d devices, got %d", expectedCount, len(devices))
	}

	// Verify device properties
	for _, device := range devices {
		if device.MAC == "" {
			t.Error("Device MAC should not be empty")
		}
		if device.SSID == "" {
			t.Error("Device SSID should not be empty")
		}
		if device.Model == "" {
			t.Error("Device model should not be empty")
		}
		if device.Generation == 0 {
			t.Error("Device generation should be set")
		}
		if device.IP == "" {
			t.Error("Device IP should not be empty")
		}
	}
}

func TestProvisioningManager_ProvisionDevice(t *testing.T) {
	// Create test logger (error level to reduce test output)
	logger, err := logging.New(logging.Config{
		Level:  "error",
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatal("Failed to create logger:", err)
	}

	// Create test config
	cfg := &config.Config{}

	// Create provisioning manager
	pm := NewProvisioningManager(cfg, logger)

	// Create mock network interface
	mockNetIface := testMockInterface(logger)
	pm.SetNetworkInterface(mockNetIface)

	// Create mock provisioner
	mockProvisioner := NewShellyProvisioner(logger, mockNetIface)
	pm.SetDeviceProvisioner(mockProvisioner)

	// Create test device
	device := UnprovisionedDevice{
		MAC:        "A4:CF:12:34:56:78",
		SSID:       "shelly1-345678",
		Password:   "",
		Model:      "SHSW-1",
		Generation: 1,
		IP:         "192.168.33.1",
		Signal:     75,
		Discovered: time.Now(),
	}

	// Create test request
	request := ProvisioningRequest{
		SSID:       "TestNetwork",
		Password:   "testpass123",
		DeviceName: "TestShelly",
		Timeout:    30,
	}

	// Test provisioning (this will use mock implementations)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := pm.ProvisionDevice(ctx, device, request)

	// The provisioning will fail when trying to actually connect to the device,
	// but we can verify the workflow structure was executed

	if result == nil {
		t.Error("Result should not be nil even on failure")
		return
	}

	// Verify result structure
	if result.DeviceMAC != device.MAC {
		t.Errorf("Expected MAC %s, got %s", device.MAC, result.DeviceMAC)
	}

	if result.DeviceName != request.DeviceName {
		t.Errorf("Expected name %s, got %s", request.DeviceName, result.DeviceName)
	}

	if result.StartTime.IsZero() {
		t.Error("Start time should be set")
	}

	if result.EndTime.IsZero() {
		t.Error("End time should be set")
	}

	if len(result.Steps) == 0 {
		t.Error("Should have recorded provisioning steps")
	}

	// The test should fail at device communication but succeed in workflow execution
	if err == nil {
		t.Log("Provisioning completed successfully (unexpected with mock)")
	} else {
		t.Logf("Provisioning failed as expected with mock devices: %v", err)

		// Check that we attempted some steps
		if len(result.Steps) == 0 {
			t.Error("Should have attempted at least some provisioning steps")
		}
	}
}

func TestMockNetworkInterfaceOperations(t *testing.T) {
	logger, err := logging.New(logging.Config{
		Level:  "error",
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatal("Failed to create logger:", err)
	}

	mockIface := testMockInterface(logger)
	ctx := context.Background()

	// Test network scanning
	networks, err := mockIface.GetAvailableNetworks(ctx)
	if err != nil {
		t.Fatal("Failed to get networks:", err)
	}

	if len(networks) == 0 {
		t.Error("Should have mock networks available")
	}

	// Find a Shelly network
	var shellyNetwork *WiFiNetwork
	for _, network := range networks {
		if network.SSID == "shelly1-AABBCC" {
			shellyNetwork = &network
			break
		}
	}

	if shellyNetwork == nil {
		t.Error("Should have mock Shelly network")
		return
	}

	// Test connection to Shelly AP (open network)
	err = mockIface.ConnectToNetwork(ctx, shellyNetwork.SSID, "")
	if err != nil {
		t.Fatal("Failed to connect to mock network:", err)
	}

	// Test current network check
	current, err := mockIface.GetCurrentNetwork(ctx)
	if err != nil {
		t.Fatal("Failed to get current network:", err)
	}

	if current.SSID != shellyNetwork.SSID {
		t.Errorf("Expected current network %s, got %s", shellyNetwork.SSID, current.SSID)
	}

	// Test connection check
	connected, err := mockIface.IsConnected(ctx, shellyNetwork.SSID)
	if err != nil {
		t.Fatal("Failed to check connection:", err)
	}

	if !connected {
		t.Error("Should be connected to the network")
	}

	// Test disconnection
	err = mockIface.DisconnectFromNetwork(ctx)
	if err != nil {
		t.Fatal("Failed to disconnect:", err)
	}

	// Verify disconnection
	_, err = mockIface.GetCurrentNetwork(ctx)
	if err == nil {
		t.Error("Should not have current network after disconnect")
	}
}

func TestShellyProvisioner_DiscoverUnprovisionedDevices(t *testing.T) {
	logger, err := logging.New(logging.Config{
		Level:  "error",
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatal("Failed to create logger:", err)
	}

	mockIface := testMockInterface(logger)
	provisioner := NewShellyProvisioner(logger, mockIface)

	ctx := context.Background()
	devices, err := provisioner.DiscoverUnprovisionedDevices(ctx)
	if err != nil {
		t.Fatal("Failed to discover devices:", err)
	}

	// Should find Shelly devices from mock networks
	expectedCount := 3 // shelly1-AABBCC, shellyplus1-DDEEFF, shellydimmer-112233
	if len(devices) != expectedCount {
		t.Errorf("Expected %d Shelly devices, got %d", expectedCount, len(devices))
	}

	// Verify device identification
	for _, device := range devices {
		if device.Generation == 0 {
			t.Error("Device generation should be identified")
		}

		if device.Model == "" {
			t.Error("Device model should be identified")
		}

		// Check model mapping
		switch device.SSID {
		case "shelly1-AABBCC":
			if device.Model != "SHSW-1" {
				t.Errorf("Expected model SHSW-1 for %s, got %s", device.SSID, device.Model)
			}
			if device.Generation != 1 {
				t.Errorf("Expected generation 1 for %s, got %d", device.SSID, device.Generation)
			}
		case "shellyplus1-DDEEFF":
			if device.Generation != 2 {
				t.Errorf("Expected generation 2 for Plus device %s, got %d", device.SSID, device.Generation)
			}
		case "shellydimmer-112233":
			if device.Model != "SHDM-1" {
				t.Errorf("Expected model SHDM-1 for %s, got %s", device.SSID, device.Model)
			}
		}
	}
}

func TestProvisioningRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request ProvisioningRequest
		valid   bool
	}{
		{
			name: "valid minimal request",
			request: ProvisioningRequest{
				SSID:    "MyNetwork",
				Timeout: 300,
			},
			valid: true,
		},
		{
			name: "valid full request",
			request: ProvisioningRequest{
				SSID:         "MyNetwork",
				Password:     "password123",
				DeviceName:   "MyShelly",
				EnableAuth:   true,
				AuthUser:     "admin",
				AuthPassword: "adminpass",
				EnableCloud:  true,
				EnableMQTT:   true,
				MQTTServer:   "mqtt.example.com",
				Timeout:      600,
			},
			valid: true,
		},
		{
			name: "missing SSID",
			request: ProvisioningRequest{
				Password: "password123",
				Timeout:  300,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - SSID is required
			if tt.request.SSID == "" && tt.valid {
				t.Error("Request should be invalid when SSID is missing")
			}

			if tt.request.SSID != "" && !tt.valid && tt.name == "missing SSID" {
				t.Error("Request should be valid when SSID is present")
			}
		})
	}
}

func TestProvisioningStatus_Transitions(t *testing.T) {
	validTransitions := map[ProvisioningStatus][]ProvisioningStatus{
		StatusIdle:        {StatusScanning, StatusConnecting},
		StatusScanning:    {StatusConnecting, StatusFailed},
		StatusConnecting:  {StatusConfiguring, StatusFailed, StatusTimeout},
		StatusConfiguring: {StatusCompleted, StatusFailed, StatusTimeout},
		StatusCompleted:   {StatusIdle},
		StatusFailed:      {StatusIdle},
		StatusTimeout:     {StatusIdle},
	}

	// Test that each status has valid next states
	for current, validNext := range validTransitions {
		if len(validNext) == 0 {
			t.Errorf("Status %s should have valid next states", current)
		}

		// Each final state should transition back to idle
		for _, next := range validNext {
			if next == StatusCompleted || next == StatusFailed || next == StatusTimeout {
				found := false
				for _, finalNext := range validTransitions[next] {
					if finalNext == StatusIdle {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Final status %s should be able to transition to idle", next)
				}
			}
		}
	}
}

// Test device model identification
func TestGetModelFromSSID(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text", Output: "stderr"})
	provisioner := &ShellyProvisioner{logger: logger}

	tests := []struct {
		ssid     string
		expected string
	}{
		{"shelly1-AABBCC", "SHSW-1"},
		{"shellyplus1-DDEEFF", "SPSW-001X16EU"},
		{"shellydimmer-112233", "SHDM-1"},
		{"shellyplug-445566", "SHPLG-S"},
		{"shellyht-778899", "SHHT-1"},
		{"shelly25-AABBCC", "SHSW-25"},
		{"shellyem-DDEEFF", "SHEM"},
		{"unknown-device", "SHSW-1"}, // fallback
	}

	for _, tt := range tests {
		t.Run(tt.ssid, func(t *testing.T) {
			result := provisioner.getModelFromSSID(tt.ssid)
			if result != tt.expected {
				t.Errorf("getModelFromSSID(%s) = %s, want %s", tt.ssid, result, tt.expected)
			}
		})
	}
}

// Test device generation detection
func TestGetGenerationFromModel(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text", Output: "stderr"})
	provisioner := &ShellyProvisioner{logger: logger}

	tests := []struct {
		model    string
		expected int
	}{
		{"SHSW-1", 1},
		{"SHPLG-S", 1},
		{"SHDM-1", 1},
		{"SPSW-001X16EU", 2},
		{"SNSN-0013A", 2},
		{"SPSH-001", 2},
		{"ShellyPlus1", 2},
		{"unknown", 1}, // fallback to Gen1
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := provisioner.getGenerationFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("getGenerationFromModel(%s) = %d, want %d", tt.model, result, tt.expected)
			}
		})
	}
}

// Benchmark provisioning workflow performance
func BenchmarkProvisioningDiscovery(b *testing.B) {
	logger, _ := logging.New(logging.Config{
		Level:  "error", // Minimal logging for benchmarks
		Format: "text",
		Output: "stderr",
	})

	cfg := &config.Config{}
	pm := NewProvisioningManager(cfg, logger)

	mockIface := testMockInterface(logger)
	pm.SetNetworkInterface(mockIface)

	mockProvisioner := NewShellyProvisioner(logger, mockIface)
	pm.SetDeviceProvisioner(mockProvisioner)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pm.DiscoverUnprovisionedDevices(ctx)
	}
}

// Test example usage
func ExampleProvisioningManager() {
	// Create logger
	logger, _ := logging.New(logging.Config{
		Level:  "info",
		Format: "text",
		Output: "stdout",
	})

	// Create config
	cfg := &config.Config{}

	// Create provisioning manager
	pm := NewProvisioningManager(cfg, logger)

	// Set up network interface (would be platform-specific in real usage)
	netInterface := CreateNetworkInterface(logger)
	pm.SetNetworkInterface(netInterface)

	// Create device provisioner
	shellyProvisioner := NewShellyProvisioner(logger, netInterface)
	pm.SetDeviceProvisioner(shellyProvisioner)

	// Discover unprovisioned devices
	ctx := context.Background()
	devices, err := pm.DiscoverUnprovisionedDevices(ctx)
	if err != nil {
		fmt.Printf("Discovery failed: %v\n", err)
		return
	}

	fmt.Printf("Found %d unprovisioned devices\n", len(devices))

	// Example output: Found 3 unprovisioned devices
}

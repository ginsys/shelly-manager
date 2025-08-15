package provisioning

import (
	"context"
	"fmt"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestMockNetworkInterface provides a test mock for any platform
type TestMockNetworkInterface struct {
	logger            *logging.Logger
	currentNetwork    *WiFiNetwork
	availableNetworks []WiFiNetwork
}

// NewTestMockNetworkInterface creates a new test mock network interface
func NewTestMockNetworkInterface(logger *logging.Logger) *TestMockNetworkInterface {
	mockNetworks := []WiFiNetwork{
		{
			SSID:      "shelly1-AABBCC",
			Security:  "",
			Signal:    75,
			Channel:   6,
			Frequency: 2437,
		},
		{
			SSID:      "shellyplus1-DDEEFF",
			Security:  "",
			Signal:    65,
			Channel:   11,
			Frequency: 2462,
		},
		{
			SSID:      "shellydimmer-112233",
			Security:  "",
			Signal:    55,
			Channel:   1,
			Frequency: 2412,
		},
		{
			SSID:      "MyHomeWiFi",
			Security:  "WPA2",
			Signal:    90,
			Channel:   6,
			Frequency: 2437,
		},
	}

	return &TestMockNetworkInterface{
		logger:            logger,
		availableNetworks: mockNetworks,
	}
}

// GetAvailableNetworks returns mock WiFi networks
func (ni *TestMockNetworkInterface) GetAvailableNetworks(ctx context.Context) ([]WiFiNetwork, error) {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "test_mock",
	}).Debug("Scanning for available WiFi networks (test mock)")

	// Simulate scan delay
	time.Sleep(100 * time.Millisecond)

	return ni.availableNetworks, nil
}

// ConnectToNetwork simulates connecting to a WiFi network
func (ni *TestMockNetworkInterface) ConnectToNetwork(ctx context.Context, ssid, password string) error {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "test_mock",
		"ssid":      ssid,
	}).Debug("Connecting to WiFi network (test mock)")

	// Find the network in our available list
	var targetNetwork *WiFiNetwork
	for _, network := range ni.availableNetworks {
		if network.SSID == ssid {
			targetNetwork = &network
			break
		}
	}

	if targetNetwork == nil {
		return fmt.Errorf("network %s not found", ssid)
	}

	// Simulate connection delay
	time.Sleep(100 * time.Millisecond)

	// Check if password is required
	if targetNetwork.Security != "" && password == "" {
		return fmt.Errorf("password required for secured network %s", ssid)
	}

	// Set as current network
	ni.currentNetwork = targetNetwork

	return nil
}

// DisconnectFromNetwork simulates disconnecting from WiFi
func (ni *TestMockNetworkInterface) DisconnectFromNetwork(ctx context.Context) error {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "test_mock",
	}).Debug("Disconnecting from current WiFi network (test mock)")

	ni.currentNetwork = nil
	return nil
}

// GetCurrentNetwork returns the currently connected network
func (ni *TestMockNetworkInterface) GetCurrentNetwork(ctx context.Context) (*WiFiNetwork, error) {
	if ni.currentNetwork == nil {
		return nil, fmt.Errorf("no active WiFi connection")
	}

	return ni.currentNetwork, nil
}

// IsConnected checks if connected to a specific network
func (ni *TestMockNetworkInterface) IsConnected(ctx context.Context, ssid string) (bool, error) {
	if ni.currentNetwork == nil {
		return false, nil
	}

	return ni.currentNetwork.SSID == ssid, nil
}

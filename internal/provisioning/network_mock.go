//go:build !linux

package provisioning

import (
	"context"
	"fmt"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// MockNetworkInterface provides a mock implementation for non-Linux platforms
// This is useful for development and testing on macOS/Windows
type MockNetworkInterface struct {
	logger            *logging.Logger
	currentNetwork    *WiFiNetwork
	availableNetworks []WiFiNetwork
}

// NewMockNetworkInterface creates a new mock network interface
func NewMockNetworkInterface(logger *logging.Logger) *MockNetworkInterface {
	// Pre-populate with some mock Shelly devices in AP mode
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
		{
			SSID:      "GuestNetwork",
			Security:  "",
			Signal:    80,
			Channel:   11,
			Frequency: 2462,
		},
	}

	return &MockNetworkInterface{
		logger:            logger,
		availableNetworks: mockNetworks,
	}
}

// GetAvailableNetworks returns mock WiFi networks
func (ni *MockNetworkInterface) GetAvailableNetworks(ctx context.Context) ([]WiFiNetwork, error) {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "mock",
	}).Debug("Scanning for available WiFi networks (mock)")

	// Simulate scan delay
	time.Sleep(1 * time.Second)

	ni.logger.WithFields(map[string]any{
		"component":      "network_interface",
		"networks_found": len(ni.availableNetworks),
	}).Debug("WiFi network scan completed (mock)")

	return ni.availableNetworks, nil
}

// ConnectToNetwork simulates connecting to a WiFi network
func (ni *MockNetworkInterface) ConnectToNetwork(ctx context.Context, ssid, password string) error {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "mock",
		"ssid":      ssid,
	}).Info("Connecting to WiFi network (mock)")

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
	time.Sleep(3 * time.Second)

	// Check if password is required
	if targetNetwork.Security != "" && password == "" {
		return fmt.Errorf("password required for secured network %s", ssid)
	}

	// Simulate occasional connection failures for realism
	if targetNetwork.Signal < 30 {
		return fmt.Errorf("connection failed: weak signal")
	}

	// Set as current network
	ni.currentNetwork = targetNetwork

	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"ssid":      ssid,
	}).Info("Successfully connected to WiFi network (mock)")

	return nil
}

// DisconnectFromNetwork simulates disconnecting from WiFi
func (ni *MockNetworkInterface) DisconnectFromNetwork(ctx context.Context) error {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "mock",
	}).Info("Disconnecting from current WiFi network (mock)")

	if ni.currentNetwork != nil {
		ni.logger.WithFields(map[string]any{
			"component": "network_interface",
			"ssid":      ni.currentNetwork.SSID,
		}).Debug("Disconnected from network (mock)")
	}

	ni.currentNetwork = nil
	return nil
}

// GetCurrentNetwork returns the currently connected network
func (ni *MockNetworkInterface) GetCurrentNetwork(ctx context.Context) (*WiFiNetwork, error) {
	if ni.currentNetwork == nil {
		return nil, fmt.Errorf("no active WiFi connection")
	}

	return ni.currentNetwork, nil
}

// IsConnected checks if connected to a specific network
func (ni *MockNetworkInterface) IsConnected(ctx context.Context, ssid string) (bool, error) {
	if ni.currentNetwork == nil {
		return false, nil
	}

	return ni.currentNetwork.SSID == ssid, nil
}

// AddMockNetwork adds a mock network for testing
func (ni *MockNetworkInterface) AddMockNetwork(network WiFiNetwork) {
	ni.availableNetworks = append(ni.availableNetworks, network)
}

// RemoveMockNetwork removes a mock network
func (ni *MockNetworkInterface) RemoveMockNetwork(ssid string) {
	for i, network := range ni.availableNetworks {
		if network.SSID == ssid {
			ni.availableNetworks = append(ni.availableNetworks[:i], ni.availableNetworks[i+1:]...)
			break
		}
	}
}

// SetMockSignalStrength updates the signal strength of a mock network
func (ni *MockNetworkInterface) SetMockSignalStrength(ssid string, signal int) {
	for i, network := range ni.availableNetworks {
		if network.SSID == ssid {
			ni.availableNetworks[i].Signal = signal
			break
		}
	}
}

// CreateNetworkInterface creates a mock network interface for non-Linux platforms
func CreateNetworkInterface(logger *logging.Logger) NetworkInterface {
	logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "mock",
	}).Warn("Using mock network interface - real WiFi provisioning not available on this platform")

	return NewMockNetworkInterface(logger)
}

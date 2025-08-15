//go:build linux

package provisioning

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// LinuxNetworkInterface implements NetworkInterface for Linux systems using NetworkManager
type LinuxNetworkInterface struct {
	logger *logging.Logger
}

// NewLinuxNetworkInterface creates a new Linux network interface manager
func NewLinuxNetworkInterface(logger *logging.Logger) *LinuxNetworkInterface {
	return &LinuxNetworkInterface{
		logger: logger,
	}
}

// GetAvailableNetworks scans for available WiFi networks using nmcli
func (ni *LinuxNetworkInterface) GetAvailableNetworks(ctx context.Context) ([]WiFiNetwork, error) {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "linux",
	}).Debug("Scanning for available WiFi networks")

	// Request a fresh scan
	scanCmd := exec.CommandContext(ctx, "nmcli", "device", "wifi", "rescan")
	if err := scanCmd.Run(); err != nil {
		ni.logger.WithFields(map[string]any{
			"component": "network_interface",
			"error":     err.Error(),
		}).Warn("Failed to trigger WiFi rescan")
	}

	// Wait a moment for scan to complete
	time.Sleep(2 * time.Second)

	// Get the list of networks
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "SSID,SECURITY,SIGNAL,CHAN,FREQ", "device", "wifi", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list WiFi networks: %w", err)
	}

	return ni.parseNetworkList(string(output))
}

// parseNetworkList parses nmcli output into WiFiNetwork structs
func (ni *LinuxNetworkInterface) parseNetworkList(output string) ([]WiFiNetwork, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	networks := make([]WiFiNetwork, 0, len(lines))
	seen := make(map[string]bool)

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) < 5 {
			continue
		}

		ssid := strings.TrimSpace(parts[0])
		if ssid == "" || seen[ssid] {
			continue // Skip empty SSIDs or duplicates
		}
		seen[ssid] = true

		security := strings.TrimSpace(parts[1])
		signalStr := strings.TrimSpace(parts[2])
		channelStr := strings.TrimSpace(parts[3])
		freqStr := strings.TrimSpace(parts[4])

		signal, _ := strconv.Atoi(signalStr)
		channel, _ := strconv.Atoi(channelStr)
		frequency, _ := strconv.Atoi(freqStr)

		network := WiFiNetwork{
			SSID:      ssid,
			Security:  security,
			Signal:    signal,
			Channel:   channel,
			Frequency: frequency,
		}

		networks = append(networks, network)
	}

	ni.logger.WithFields(map[string]any{
		"component":      "network_interface",
		"networks_found": len(networks),
	}).Debug("WiFi network scan completed")

	return networks, nil
}

// ConnectToNetwork connects to a WiFi network using nmcli
func (ni *LinuxNetworkInterface) ConnectToNetwork(ctx context.Context, ssid, password string) error {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "linux",
		"ssid":      ssid,
	}).Info("Connecting to WiFi network")

	// Check if we already have a connection profile for this network
	profileExists := ni.checkConnectionProfile(ctx, ssid)

	var cmd *exec.Cmd
	if password == "" {
		// Open network
		if profileExists {
			cmd = exec.CommandContext(ctx, "nmcli", "connection", "up", ssid)
		} else {
			cmd = exec.CommandContext(ctx, "nmcli", "device", "wifi", "connect", ssid)
		}
	} else {
		// Secured network
		if profileExists {
			cmd = exec.CommandContext(ctx, "nmcli", "connection", "up", ssid)
		} else {
			cmd = exec.CommandContext(ctx, "nmcli", "device", "wifi", "connect", ssid, "password", password)
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		ni.logger.WithFields(map[string]any{
			"component": "network_interface",
			"ssid":      ssid,
			"error":     err.Error(),
			"output":    string(output),
		}).Error("Failed to connect to WiFi network")
		return fmt.Errorf("failed to connect to network %s: %w", ssid, err)
	}

	// Wait for connection to establish
	if err := ni.waitForConnection(ctx, ssid, 30*time.Second); err != nil {
		return fmt.Errorf("connection to %s timed out: %w", ssid, err)
	}

	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"ssid":      ssid,
	}).Info("Successfully connected to WiFi network")

	return nil
}

// checkConnectionProfile checks if a connection profile exists for the SSID
func (ni *LinuxNetworkInterface) checkConnectionProfile(ctx context.Context, ssid string) bool {
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "NAME", "connection", "show")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	profiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, profile := range profiles {
		if strings.TrimSpace(profile) == ssid {
			return true
		}
	}

	return false
}

// waitForConnection waits for a connection to be established
func (ni *LinuxNetworkInterface) waitForConnection(ctx context.Context, ssid string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if connected, _ := ni.IsConnected(ctx, ssid); connected {
				return nil
			}
		}
	}
}

// DisconnectFromNetwork disconnects from the current WiFi network
func (ni *LinuxNetworkInterface) DisconnectFromNetwork(ctx context.Context) error {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "linux",
	}).Info("Disconnecting from current WiFi network")

	// Get the active WiFi connection
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "NAME,TYPE,STATE", "connection", "show", "--active")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get active connections: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[1] == "802-11-wireless" && parts[2] == "activated" {
			connectionName := parts[0]

			ni.logger.WithFields(map[string]any{
				"component":  "network_interface",
				"connection": connectionName,
			}).Debug("Disconnecting from WiFi connection")

			downCmd := exec.CommandContext(ctx, "nmcli", "connection", "down", connectionName)
			if err := downCmd.Run(); err != nil {
				ni.logger.WithFields(map[string]any{
					"component":  "network_interface",
					"connection": connectionName,
					"error":      err.Error(),
				}).Warn("Failed to disconnect from WiFi connection")
			}
		}
	}

	return nil
}

// GetCurrentNetwork returns the currently connected network info
func (ni *LinuxNetworkInterface) GetCurrentNetwork(ctx context.Context) (*WiFiNetwork, error) {
	// Get the active WiFi connection
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "NAME,TYPE,STATE", "connection", "show", "--active")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get active connections: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[1] == "802-11-wireless" && parts[2] == "activated" {
			ssid := parts[0]

			// Get detailed info about this connection
			detailCmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "802-11-wireless.ssid,802-11-wireless.security", "connection", "show", ssid)
			detailOutput, err := detailCmd.Output()
			if err != nil {
				continue
			}

			network := &WiFiNetwork{
				SSID: ssid,
			}

			// Parse connection details
			detailLines := strings.Split(strings.TrimSpace(string(detailOutput)), "\n")
			for _, detailLine := range detailLines {
				detailParts := strings.Split(detailLine, ":")
				if len(detailParts) >= 2 {
					if strings.Contains(detailParts[0], "ssid") {
						network.SSID = strings.TrimSpace(detailParts[1])
					} else if strings.Contains(detailParts[0], "security") {
						network.Security = strings.TrimSpace(detailParts[1])
					}
				}
			}

			return network, nil
		}
	}

	return nil, fmt.Errorf("no active WiFi connection found")
}

// IsConnected checks if connected to a specific network
func (ni *LinuxNetworkInterface) IsConnected(ctx context.Context, ssid string) (bool, error) {
	current, err := ni.GetCurrentNetwork(ctx)
	if err != nil {
		return false, nil // Not connected to any network
	}

	return current.SSID == ssid, nil
}

// CreateNetworkInterface creates a platform-specific network interface
func CreateNetworkInterface(logger *logging.Logger) NetworkInterface {
	return NewLinuxNetworkInterface(logger)
}

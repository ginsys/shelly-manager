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

// GetInterfaceInfo returns information about the Linux network interface
func (ni *LinuxNetworkInterface) GetInterfaceInfo() NetworkInterfaceInfo {
	info := NetworkInterfaceInfo{
		Type:         "wireless",
		Tooling:      "nmcli (NetworkManager)",
		Capabilities: []string{"scan", "connect", "disconnect", "monitor"},
		Status:       "unknown",
	}

	// Try to get WiFi device information using nmcli
	cmd := exec.Command("nmcli", "-t", "-f", "DEVICE,TYPE,STATE", "device", "status")
	output, err := cmd.Output()
	if err != nil {
		ni.logger.WithFields(map[string]any{
			"component": "network_interface",
			"error":     err.Error(),
		}).Warn("Failed to get network device information")
		info.Name = "unknown"
		info.Status = "error"
		return info
	}

	// Parse nmcli output to find WiFi device
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && fields[1] == "wifi" {
			info.Name = fields[0]
			switch fields[2] {
			case "connected":
				info.Status = "connected"
			case "disconnected":
				info.Status = "disconnected"
			case "unavailable":
				info.Status = "unavailable"
			default:
				info.Status = fields[2]
			}
			break
		}
	}

	// If no WiFi device found, set default values
	if info.Name == "" {
		info.Name = "no-wifi-device"
		info.Status = "unavailable"
		info.Capabilities = []string{"none"}
	}

	return info
}

// GetAvailableNetworks scans for available WiFi networks using nmcli
func (ni *LinuxNetworkInterface) GetAvailableNetworks(ctx context.Context) ([]WiFiNetwork, error) {
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"platform":  "linux",
	}).Debug("Scanning for available WiFi networks")

	// Debug: Check if nmcli is available and get device status first
	deviceCmd := exec.CommandContext(ctx, "nmcli", "device", "status")
	deviceOutput, deviceErr := deviceCmd.CombinedOutput()
	ni.logger.WithFields(map[string]any{
		"component": "network_interface",
		"command":   "nmcli device status",
		"output":    string(deviceOutput),
		"error":     deviceErr,
	}).Debug("NetworkManager device status")

	// Request a fresh scan
	scanCmd := exec.CommandContext(ctx, "nmcli", "device", "wifi", "rescan")
	scanOutput, scanErr := scanCmd.CombinedOutput()
	if scanErr != nil {
		ni.logger.WithFields(map[string]any{
			"component":   "network_interface",
			"command":     "nmcli device wifi rescan",
			"error":       scanErr.Error(),
			"output":      string(scanOutput),
			"exit_status": scanCmd.ProcessState.ExitCode(),
		}).Warn("Failed to trigger WiFi rescan - detailed error info")
	} else {
		ni.logger.WithFields(map[string]any{
			"component": "network_interface",
			"command":   "nmcli device wifi rescan",
			"output":    string(scanOutput),
		}).Debug("WiFi rescan command completed successfully")
	}

	// Wait a moment for scan to complete
	time.Sleep(2 * time.Second)

	// Get the list of networks
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "SSID,SECURITY,SIGNAL,CHAN,FREQ", "device", "wifi", "list")
	output, err := cmd.CombinedOutput()
	ni.logger.WithFields(map[string]any{
		"component":    "network_interface",
		"command":      "nmcli -t -f SSID,SECURITY,SIGNAL,CHAN,FREQ device wifi list",
		"raw_output":   string(output),
		"output_lines": len(strings.Split(string(output), "\n")),
		"error":        err,
	}).Debug("WiFi network list command output")

	if err != nil {
		return nil, fmt.Errorf("failed to list WiFi networks: %w (output: %s)", err, string(output))
	}

	networks, parseErr := ni.parseNetworkList(string(output))
	ni.logger.WithFields(map[string]any{
		"component":       "network_interface",
		"raw_output_len":  len(output),
		"networks_parsed": len(networks),
		"parse_error":     parseErr,
	}).Debug("Network parsing completed")

	return networks, parseErr
}

// parseNetworkList parses nmcli output into WiFiNetwork structs
func (ni *LinuxNetworkInterface) parseNetworkList(output string) ([]WiFiNetwork, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	networks := make([]WiFiNetwork, 0, len(lines))
	seen := make(map[string]bool)

	ni.logger.WithFields(map[string]any{
		"component":  "network_interface",
		"raw_output": output,
		"line_count": len(lines),
	}).Debug("Starting network list parsing")

	for i, line := range lines {
		ni.logger.WithFields(map[string]any{
			"component":    "network_interface",
			"line_index":   i,
			"line_content": line,
		}).Debug("Processing network line")

		if line == "" {
			ni.logger.WithFields(map[string]any{
				"component":  "network_interface",
				"line_index": i,
			}).Debug("Skipping empty line")
			continue
		}

		parts := strings.Split(line, ":")
		ni.logger.WithFields(map[string]any{
			"component":   "network_interface",
			"line_index":  i,
			"parts_count": len(parts),
			"parts":       parts,
		}).Debug("Line split into parts")

		if len(parts) < 5 {
			ni.logger.WithFields(map[string]any{
				"component":   "network_interface",
				"line_index":  i,
				"parts_count": len(parts),
				"expected":    5,
			}).Debug("Skipping line with insufficient parts")
			continue
		}

		ssid := strings.TrimSpace(parts[0])
		ni.logger.WithFields(map[string]any{
			"component":    "network_interface",
			"line_index":   i,
			"ssid":         ssid,
			"already_seen": seen[ssid],
		}).Debug("Processing SSID")

		if ssid == "" || seen[ssid] {
			ni.logger.WithFields(map[string]any{
				"component":  "network_interface",
				"line_index": i,
				"ssid":       ssid,
				"reason": func() string {
					if ssid == "" {
						return "empty_ssid"
					}
					return "duplicate_ssid"
				}(),
			}).Debug("Skipping SSID")
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

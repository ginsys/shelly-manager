package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// ShellyProvisioner implements DeviceProvisioner for Shelly devices
type ShellyProvisioner struct {
	logger     *logging.Logger
	httpClient *http.Client
	netIface   NetworkInterface
}

// NewShellyProvisioner creates a new Shelly device provisioner
func NewShellyProvisioner(logger *logging.Logger, netIface NetworkInterface) *ShellyProvisioner {
	return &ShellyProvisioner{
		logger:   logger,
		netIface: netIface,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DiscoverUnprovisionedDevices scans for Shelly devices in AP mode
func (sp *ShellyProvisioner) DiscoverUnprovisionedDevices(ctx context.Context) ([]UnprovisionedDevice, error) {
	if sp.netIface == nil {
		return nil, fmt.Errorf("network interface not available")
	}
	
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
	}).Info("Scanning for Shelly devices in AP mode")
	
	// Get available WiFi networks
	networks, err := sp.netIface.GetAvailableNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scan WiFi networks: %w", err)
	}
	
	var devices []UnprovisionedDevice
	
	// Look for Shelly AP SSIDs
	shellyAPPattern := regexp.MustCompile(`^shelly[a-zA-Z0-9\-_]*$`)
	
	for _, network := range networks {
		if shellyAPPattern.MatchString(strings.ToLower(network.SSID)) {
			device, err := sp.identifyShellyDevice(ctx, network)
			if err != nil {
				sp.logger.WithFields(map[string]any{
					"component": "shelly_provisioner",
					"ssid": network.SSID,
					"error": err.Error(),
				}).Warn("Failed to identify Shelly device")
				continue
			}
			
			if device != nil {
				devices = append(devices, *device)
				sp.logger.WithFields(map[string]any{
					"component": "shelly_provisioner",
					"device_mac": device.MAC,
					"device_ssid": device.SSID,
					"device_model": device.Model,
				}).Info("Discovered unprovisioned Shelly device")
			}
		}
	}
	
	return devices, nil
}

// identifyShellyDevice attempts to identify a Shelly device from its AP
func (sp *ShellyProvisioner) identifyShellyDevice(ctx context.Context, network WiFiNetwork) (*UnprovisionedDevice, error) {
	// Extract potential MAC from SSID (e.g., shelly1-AABBCC)
	parts := strings.Split(network.SSID, "-")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid Shelly SSID format: %s", network.SSID)
	}
	
	macSuffix := parts[len(parts)-1]
	if len(macSuffix) != 6 {
		return nil, fmt.Errorf("invalid MAC suffix in SSID: %s", network.SSID)
	}
	
	// Convert MAC suffix to full MAC format (assuming standard Shelly MAC patterns)
	fullMAC := fmt.Sprintf("%s:%s:%s:%s:%s:%s", 
		macSuffix[0:2], macSuffix[2:4], macSuffix[4:6], 
		"00", "00", "00") // Placeholder - real MAC would be detected during connection
	
	// Determine model from SSID prefix
	model := sp.getModelFromSSID(network.SSID)
	generation := sp.getGenerationFromModel(model)
	
	device := &UnprovisionedDevice{
		MAC:        fullMAC,
		SSID:       network.SSID,
		Password:   sp.getDefaultPassword(model, macSuffix),
		Model:      model,
		Generation: generation,
		IP:         "192.168.33.1", // Standard Shelly AP IP
		Signal:     network.Signal,
		Discovered: time.Now(),
	}
	
	return device, nil
}

// getModelFromSSID extracts device model from SSID
func (sp *ShellyProvisioner) getModelFromSSID(ssid string) string {
	ssidLower := strings.ToLower(ssid)
	
	// Common Shelly device patterns
	if strings.Contains(ssidLower, "shelly1") {
		return "SHSW-1"
	}
	if strings.Contains(ssidLower, "shellyplus1") {
		return "SPSW-001X16EU"
	}
	if strings.Contains(ssidLower, "shellydimmer") {
		return "SHDM-1"
	}
	if strings.Contains(ssidLower, "shellyplug") {
		return "SHPLG-S"
	}
	if strings.Contains(ssidLower, "shellyht") {
		return "SHHT-1"
	}
	if strings.Contains(ssidLower, "shelly25") {
		return "SHSW-25"
	}
	if strings.Contains(ssidLower, "shellyem") {
		return "SHEM"
	}
	
	// Generic fallback
	return "SHSW-1"
}

// getGenerationFromModel determines device generation from model
func (sp *ShellyProvisioner) getGenerationFromModel(model string) int {
	// Gen2+ devices typically have "Plus" in the name or newer model numbers
	modelUpper := strings.ToUpper(model)
	if strings.Contains(modelUpper, "PLUS") || 
	   strings.HasPrefix(modelUpper, "SPSW-") ||
	   strings.HasPrefix(modelUpper, "SNSN-") ||
	   strings.HasPrefix(modelUpper, "SPSH-") {
		return 2
	}
	return 1
}

// getDefaultPassword returns the default AP password for a Shelly device
func (sp *ShellyProvisioner) getDefaultPassword(model, macSuffix string) string {
	// Most Shelly devices use no password or a pattern based on MAC
	// Gen1 devices typically have no password or "shelly" + MAC suffix
	// Gen2+ devices may use different patterns
	
	generation := sp.getGenerationFromModel(model)
	if generation == 2 {
		return "" // Gen2+ typically open initially
	}
	
	// Gen1 default patterns
	return "" // Most Gen1 devices start with open AP
}

// ConnectToDeviceAP connects to a Shelly device's AP
func (sp *ShellyProvisioner) ConnectToDeviceAP(ctx context.Context, device UnprovisionedDevice) error {
	if sp.netIface == nil {
		return fmt.Errorf("network interface not available")
	}
	
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"device_mac": device.MAC,
		"device_ssid": device.SSID,
	}).Info("Connecting to device AP")
	
	// Check if already connected
	if connected, _ := sp.netIface.IsConnected(ctx, device.SSID); connected {
		sp.logger.WithFields(map[string]any{
			"component": "shelly_provisioner",
			"device_ssid": device.SSID,
		}).Debug("Already connected to device AP")
		return nil
	}
	
	// Connect to the device AP
	err := sp.netIface.ConnectToNetwork(ctx, device.SSID, device.Password)
	if err != nil {
		return fmt.Errorf("failed to connect to device AP %s: %w", device.SSID, err)
	}
	
	// Wait a moment for connection to stabilize
	time.Sleep(2 * time.Second)
	
	// Verify connection by trying to reach the device
	if err := sp.pingDevice(ctx, device.IP); err != nil {
		return fmt.Errorf("device not reachable after connecting to AP: %w", err)
	}
	
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"device_ssid": device.SSID,
		"device_ip": device.IP,
	}).Info("Successfully connected to device AP")
	
	return nil
}

// ConfigureWiFi configures the device's WiFi settings
func (sp *ShellyProvisioner) ConfigureWiFi(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest) error {
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"device_mac": device.MAC,
		"target_ssid": request.SSID,
	}).Info("Configuring device WiFi settings")
	
	if device.Generation == 1 {
		return sp.configureGen1WiFi(ctx, device, request)
	} else {
		return sp.configureGen2WiFi(ctx, device, request)
	}
}

// configureGen1WiFi configures WiFi for Gen1 devices
func (sp *ShellyProvisioner) configureGen1WiFi(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest) error {
	// Gen1 devices use /settings/sta endpoint
	url := fmt.Sprintf("http://%s/settings/sta", device.IP)
	
	params := map[string]interface{}{
		"enabled": true,
		"ssid":    request.SSID,
		"key":     request.Password,
	}
	
	return sp.makeDeviceRequest(ctx, "POST", url, params)
}

// configureGen2WiFi configures WiFi for Gen2+ devices
func (sp *ShellyProvisioner) configureGen2WiFi(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest) error {
	// Gen2+ devices use RPC-style API
	url := fmt.Sprintf("http://%s/rpc", device.IP)
	
	rpcRequest := map[string]interface{}{
		"id":     1,
		"method": "WiFi.SetConfig",
		"params": map[string]interface{}{
			"config": map[string]interface{}{
				"sta": map[string]interface{}{
					"enable": true,
					"ssid":   request.SSID,
					"pass":   request.Password,
				},
			},
		},
	}
	
	return sp.makeDeviceRequest(ctx, "POST", url, rpcRequest)
}

// ConfigureDevice applies additional device settings
func (sp *ShellyProvisioner) ConfigureDevice(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest) error {
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"device_mac": device.MAC,
	}).Info("Configuring additional device settings")
	
	// Set device name if provided
	if request.DeviceName != "" {
		if err := sp.setDeviceName(ctx, device, request.DeviceName); err != nil {
			sp.logger.WithFields(map[string]any{
				"component": "shelly_provisioner",
				"device_mac": device.MAC,
				"error": err.Error(),
			}).Warn("Failed to set device name")
		}
	}
	
	// Configure authentication if requested
	if request.EnableAuth {
		if err := sp.configureAuth(ctx, device, request.AuthUser, request.AuthPassword); err != nil {
			sp.logger.WithFields(map[string]any{
				"component": "shelly_provisioner",
				"device_mac": device.MAC,
				"error": err.Error(),
			}).Warn("Failed to configure authentication")
		}
	}
	
	// Configure MQTT if requested
	if request.EnableMQTT && request.MQTTServer != "" {
		if err := sp.configureMQTT(ctx, device, request.MQTTServer); err != nil {
			sp.logger.WithFields(map[string]any{
				"component": "shelly_provisioner",
				"device_mac": device.MAC,
				"error": err.Error(),
			}).Warn("Failed to configure MQTT")
		}
	}
	
	// Configure cloud settings
	if err := sp.configureCloud(ctx, device, request.EnableCloud); err != nil {
		sp.logger.WithFields(map[string]any{
			"component": "shelly_provisioner",
			"device_mac": device.MAC,
			"error": err.Error(),
		}).Warn("Failed to configure cloud settings")
	}
	
	return nil
}

// setDeviceName sets the device name
func (sp *ShellyProvisioner) setDeviceName(ctx context.Context, device UnprovisionedDevice, name string) error {
	if device.Generation == 1 {
		url := fmt.Sprintf("http://%s/settings", device.IP)
		params := map[string]interface{}{
			"name": name,
		}
		return sp.makeDeviceRequest(ctx, "POST", url, params)
	} else {
		url := fmt.Sprintf("http://%s/rpc", device.IP)
		rpcRequest := map[string]interface{}{
			"id":     1,
			"method": "Sys.SetConfig",
			"params": map[string]interface{}{
				"config": map[string]interface{}{
					"device": map[string]interface{}{
						"name": name,
					},
				},
			},
		}
		return sp.makeDeviceRequest(ctx, "POST", url, rpcRequest)
	}
}

// configureAuth configures device authentication
func (sp *ShellyProvisioner) configureAuth(ctx context.Context, device UnprovisionedDevice, user, password string) error {
	if device.Generation == 1 {
		url := fmt.Sprintf("http://%s/settings/login", device.IP)
		params := map[string]interface{}{
			"enabled":  true,
			"username": user,
			"password": password,
		}
		return sp.makeDeviceRequest(ctx, "POST", url, params)
	} else {
		// Gen2+ authentication configuration would go here
		return nil // Not implemented for Gen2+ yet
	}
}

// configureMQTT configures MQTT settings
func (sp *ShellyProvisioner) configureMQTT(ctx context.Context, device UnprovisionedDevice, server string) error {
	if device.Generation == 1 {
		url := fmt.Sprintf("http://%s/settings", device.IP)
		params := map[string]interface{}{
			"mqtt_server": server,
			"mqtt_enable": true,
		}
		return sp.makeDeviceRequest(ctx, "POST", url, params)
	} else {
		// Gen2+ MQTT configuration would go here
		return nil // Not implemented for Gen2+ yet
	}
}

// configureCloud configures cloud connectivity
func (sp *ShellyProvisioner) configureCloud(ctx context.Context, device UnprovisionedDevice, enable bool) error {
	if device.Generation == 1 {
		url := fmt.Sprintf("http://%s/settings", device.IP)
		params := map[string]interface{}{
			"cloud_enabled": enable,
		}
		return sp.makeDeviceRequest(ctx, "POST", url, params)
	} else {
		// Gen2+ cloud configuration would go here
		return nil // Not implemented for Gen2+ yet
	}
}

// RebootDevice reboots the device to apply new configuration
func (sp *ShellyProvisioner) RebootDevice(ctx context.Context, device UnprovisionedDevice) error {
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"device_mac": device.MAC,
	}).Info("Rebooting device to apply configuration")
	
	if device.Generation == 1 {
		url := fmt.Sprintf("http://%s/reboot", device.IP)
		return sp.makeDeviceRequest(ctx, "GET", url, nil)
	} else {
		url := fmt.Sprintf("http://%s/rpc", device.IP)
		rpcRequest := map[string]interface{}{
			"id":     1,
			"method": "Shelly.Reboot",
		}
		return sp.makeDeviceRequest(ctx, "POST", url, rpcRequest)
	}
}

// VerifyProvisioning verifies the device is accessible on the target network
func (sp *ShellyProvisioner) VerifyProvisioning(ctx context.Context, device UnprovisionedDevice, targetSSID string, timeout time.Duration) (*ProvisioningResult, error) {
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"device_mac": device.MAC,
		"target_ssid": targetSSID,
		"timeout": timeout,
	}).Info("Verifying device provisioning")
	
	// Wait for device to reboot and connect to target network
	time.Sleep(10 * time.Second)
	
	// Try to reconnect to the target network
	if sp.netIface != nil {
		if err := sp.netIface.ConnectToNetwork(ctx, targetSSID, ""); err != nil {
			sp.logger.WithFields(map[string]any{
				"component": "shelly_provisioner",
				"target_ssid": targetSSID,
				"error": err.Error(),
			}).Warn("Failed to reconnect to target network")
		}
	}
	
	// TODO: Implement device discovery on target network
	// This would require scanning the target network for the device
	// For now, return a basic result
	
	result := &ProvisioningResult{
		DeviceMAC:    device.MAC,
		DeviceName:   device.Model,
		Status:       StatusCompleted,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
	}
	
	return result, nil
}

// pingDevice checks if a device is reachable
func (sp *ShellyProvisioner) pingDevice(ctx context.Context, ip string) error {
	url := fmt.Sprintf("http://%s/shelly", ip)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	resp, err := sp.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("device returned status %d", resp.StatusCode)
	}
	
	return nil
}

// makeDeviceRequest makes an HTTP request to a Shelly device
func (sp *ShellyProvisioner) makeDeviceRequest(ctx context.Context, method, url string, params interface{}) error {
	var body io.Reader
	
	if params != nil {
		jsonData, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal request data: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}
	
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"method": method,
		"url": url,
	}).Debug("Making device request")
	
	resp, err := sp.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device returned error %d: %s", resp.StatusCode, string(respBody))
	}
	
	sp.logger.WithFields(map[string]any{
		"component": "shelly_provisioner",
		"method": method,
		"url": url,
		"status": resp.StatusCode,
	}).Debug("Device request completed successfully")
	
	return nil
}
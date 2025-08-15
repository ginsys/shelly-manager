package provisioning

import (
	"context"
	"fmt"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// ProvisioningStatus represents the current status of a provisioning operation
type ProvisioningStatus string

const (
	StatusIdle        ProvisioningStatus = "idle"
	StatusScanning    ProvisioningStatus = "scanning"
	StatusConnecting  ProvisioningStatus = "connecting"
	StatusConfiguring ProvisioningStatus = "configuring"
	StatusCompleted   ProvisioningStatus = "completed"
	StatusFailed      ProvisioningStatus = "failed"
	StatusTimeout     ProvisioningStatus = "timeout"
)

// ProvisioningRequest contains WiFi credentials and device configuration
type ProvisioningRequest struct {
	SSID         string `json:"ssid" validate:"required"`
	Password     string `json:"password"`
	DeviceName   string `json:"device_name"`
	EnableAuth   bool   `json:"enable_auth"`
	AuthUser     string `json:"auth_user"`
	AuthPassword string `json:"auth_password"`
	EnableCloud  bool   `json:"enable_cloud"`
	EnableMQTT   bool   `json:"enable_mqtt"`
	MQTTServer   string `json:"mqtt_server"`
	Timeout      int    `json:"timeout"` // seconds
}

// ProvisioningResult contains the outcome of a provisioning operation
type ProvisioningResult struct {
	DeviceMAC  string             `json:"device_mac"`
	DeviceIP   string             `json:"device_ip"`
	DeviceName string             `json:"device_name"`
	Status     ProvisioningStatus `json:"status"`
	Error      string             `json:"error,omitempty"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
	Duration   time.Duration      `json:"duration"`
	Steps      []ProvisioningStep `json:"steps"`
}

// ProvisioningStep represents a single step in the provisioning process
type ProvisioningStep struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"` // success, failed, in_progress
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Description string        `json:"description"`
}

// UnprovisionedDevice represents a Shelly device in AP mode waiting for configuration
type UnprovisionedDevice struct {
	MAC        string    `json:"mac"`
	SSID       string    `json:"ssid"`       // AP SSID (e.g., shelly1-AABBCC)
	Password   string    `json:"password"`   // Default AP password
	Model      string    `json:"model"`      // Device model
	Generation int       `json:"generation"` // Gen1 or Gen2+
	IP         string    `json:"ip"`         // IP in AP mode (usually 192.168.33.1)
	Signal     int       `json:"signal"`     // WiFi signal strength
	Discovered time.Time `json:"discovered"`
}

// WiFiNetwork represents an available WiFi network
type WiFiNetwork struct {
	SSID      string `json:"ssid"`
	Security  string `json:"security"` // WPA2, WPA3, Open, etc.
	Signal    int    `json:"signal"`   // Signal strength (0-100)
	Channel   int    `json:"channel"`
	Frequency int    `json:"frequency"` // MHz
}

// NetworkInterface represents a system network interface abstraction
type NetworkInterface interface {
	// GetAvailableNetworks scans for available WiFi networks
	GetAvailableNetworks(ctx context.Context) ([]WiFiNetwork, error)

	// ConnectToNetwork connects to a WiFi network with credentials
	ConnectToNetwork(ctx context.Context, ssid, password string) error

	// DisconnectFromNetwork disconnects from current WiFi network
	DisconnectFromNetwork(ctx context.Context) error

	// GetCurrentNetwork returns the currently connected network info
	GetCurrentNetwork(ctx context.Context) (*WiFiNetwork, error)

	// IsConnected checks if connected to a specific network
	IsConnected(ctx context.Context, ssid string) (bool, error)
}

// DeviceProvisioner handles communication with Shelly devices during provisioning
type DeviceProvisioner interface {
	// DiscoverUnprovisionedDevices scans for Shelly devices in AP mode
	DiscoverUnprovisionedDevices(ctx context.Context) ([]UnprovisionedDevice, error)

	// ConnectToDeviceAP connects to a Shelly device's AP
	ConnectToDeviceAP(ctx context.Context, device UnprovisionedDevice) error

	// ConfigureWiFi configures the device's WiFi settings
	ConfigureWiFi(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest) error

	// ConfigureDevice applies additional device settings (auth, MQTT, etc.)
	ConfigureDevice(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest) error

	// RebootDevice reboots the device to apply new configuration
	RebootDevice(ctx context.Context, device UnprovisionedDevice) error

	// VerifyProvisioning verifies the device is accessible on the target network
	VerifyProvisioning(ctx context.Context, device UnprovisionedDevice, targetSSID string, timeout time.Duration) (*ProvisioningResult, error)
}

// ProvisioningManager orchestrates the complete provisioning workflow
type ProvisioningManager struct {
	config      *config.Config
	logger      *logging.Logger
	netIface    NetworkInterface
	provisioner DeviceProvisioner

	// Current operation state
	currentStatus  ProvisioningStatus
	currentDevice  *UnprovisionedDevice
	currentRequest *ProvisioningRequest
	currentResult  *ProvisioningResult

	// Callbacks for status updates
	statusCallback func(status ProvisioningStatus, result *ProvisioningResult)
}

// NewProvisioningManager creates a new provisioning manager
func NewProvisioningManager(cfg *config.Config, logger *logging.Logger) *ProvisioningManager {
	return &ProvisioningManager{
		config:        cfg,
		logger:        logger,
		currentStatus: StatusIdle,
	}
}

// SetNetworkInterface sets the platform-specific network interface implementation
func (pm *ProvisioningManager) SetNetworkInterface(iface NetworkInterface) {
	pm.netIface = iface
}

// SetDeviceProvisioner sets the device provisioner implementation
func (pm *ProvisioningManager) SetDeviceProvisioner(provisioner DeviceProvisioner) {
	pm.provisioner = provisioner
}

// SetStatusCallback sets a callback for provisioning status updates
func (pm *ProvisioningManager) SetStatusCallback(callback func(status ProvisioningStatus, result *ProvisioningResult)) {
	pm.statusCallback = callback
}

// GetStatus returns the current provisioning status
func (pm *ProvisioningManager) GetStatus() ProvisioningStatus {
	return pm.currentStatus
}

// GetCurrentResult returns the current provisioning result
func (pm *ProvisioningManager) GetCurrentResult() *ProvisioningResult {
	return pm.currentResult
}

// DiscoverUnprovisionedDevices discovers Shelly devices in AP mode
func (pm *ProvisioningManager) DiscoverUnprovisionedDevices(ctx context.Context) ([]UnprovisionedDevice, error) {
	if pm.provisioner == nil {
		return nil, fmt.Errorf("device provisioner not set")
	}

	pm.logger.WithFields(map[string]any{
		"component": "provisioning",
	}).Info("Starting discovery of unprovisioned devices")

	devices, err := pm.provisioner.DiscoverUnprovisionedDevices(ctx)
	if err != nil {
		pm.logger.WithFields(map[string]any{
			"component": "provisioning",
			"error":     err.Error(),
		}).Error("Failed to discover unprovisioned devices")
		return nil, err
	}

	pm.logger.WithFields(map[string]any{
		"component":     "provisioning",
		"devices_found": len(devices),
	}).Info("Discovery of unprovisioned devices completed")

	return devices, nil
}

// ProvisionDevice provisions a single Shelly device
func (pm *ProvisioningManager) ProvisionDevice(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest) (*ProvisioningResult, error) {
	if pm.netIface == nil {
		return nil, fmt.Errorf("network interface not set")
	}
	if pm.provisioner == nil {
		return nil, fmt.Errorf("device provisioner not set")
	}

	// Initialize result tracking
	result := &ProvisioningResult{
		DeviceMAC:  device.MAC,
		DeviceName: request.DeviceName,
		StartTime:  time.Now(),
		Status:     StatusIdle,
		Steps:      make([]ProvisioningStep, 0),
	}

	pm.currentDevice = &device
	pm.currentRequest = &request
	pm.currentResult = result

	// Set timeout
	timeout := time.Duration(request.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute // Default timeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pm.logger.WithFields(map[string]any{
		"component":   "provisioning",
		"device_mac":  device.MAC,
		"device_ssid": device.SSID,
		"target_ssid": request.SSID,
		"timeout":     timeout,
	}).Info("Starting device provisioning")

	// Execute provisioning steps
	err := pm.executeProvisioningWorkflow(ctx, device, request, result)

	// Finalize result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		pm.logger.WithFields(map[string]any{
			"component":  "provisioning",
			"device_mac": device.MAC,
			"error":      err.Error(),
			"duration":   result.Duration,
		}).Error("Device provisioning failed")
	} else {
		result.Status = StatusCompleted
		pm.logger.WithFields(map[string]any{
			"component":  "provisioning",
			"device_mac": device.MAC,
			"duration":   result.Duration,
		}).Info("Device provisioning completed successfully")
	}

	// Update status
	pm.currentStatus = result.Status
	if pm.statusCallback != nil {
		pm.statusCallback(result.Status, result)
	}

	return result, err
}

// executeProvisioningWorkflow executes the complete provisioning workflow
func (pm *ProvisioningManager) executeProvisioningWorkflow(ctx context.Context, device UnprovisionedDevice, request ProvisioningRequest, result *ProvisioningResult) error {
	steps := []struct {
		name        string
		description string
		execute     func() error
	}{
		{
			name:        "connect_to_device_ap",
			description: fmt.Sprintf("Connect to device AP: %s", device.SSID),
			execute: func() error {
				pm.updateStatus(StatusConnecting, result)
				return pm.provisioner.ConnectToDeviceAP(ctx, device)
			},
		},
		{
			name:        "configure_wifi",
			description: fmt.Sprintf("Configure WiFi: %s", request.SSID),
			execute: func() error {
				pm.updateStatus(StatusConfiguring, result)
				return pm.provisioner.ConfigureWiFi(ctx, device, request)
			},
		},
		{
			name:        "configure_device",
			description: "Configure device settings",
			execute: func() error {
				return pm.provisioner.ConfigureDevice(ctx, device, request)
			},
		},
		{
			name:        "reboot_device",
			description: "Reboot device to apply configuration",
			execute: func() error {
				return pm.provisioner.RebootDevice(ctx, device)
			},
		},
		{
			name:        "verify_provisioning",
			description: "Verify device is accessible on target network",
			execute: func() error {
				verifyResult, err := pm.provisioner.VerifyProvisioning(ctx, device, request.SSID, 2*time.Minute)
				if err == nil && verifyResult != nil {
					result.DeviceIP = verifyResult.DeviceIP
				}
				return err
			},
		},
	}

	// Execute each step
	for _, step := range steps {
		stepResult := ProvisioningStep{
			Name:        step.name,
			Description: step.description,
			StartTime:   time.Now(),
			Status:      "in_progress",
		}

		pm.logger.WithFields(map[string]any{
			"component":  "provisioning",
			"device_mac": device.MAC,
			"step":       step.name,
		}).Debug("Executing provisioning step")

		err := step.execute()

		stepResult.EndTime = time.Now()
		stepResult.Duration = stepResult.EndTime.Sub(stepResult.StartTime)

		if err != nil {
			stepResult.Status = "failed"
			stepResult.Error = err.Error()
			result.Steps = append(result.Steps, stepResult)

			pm.logger.WithFields(map[string]any{
				"component":  "provisioning",
				"device_mac": device.MAC,
				"step":       step.name,
				"error":      err.Error(),
				"duration":   stepResult.Duration,
			}).Error("Provisioning step failed")

			return fmt.Errorf("step %s failed: %w", step.name, err)
		}

		stepResult.Status = "success"
		result.Steps = append(result.Steps, stepResult)

		pm.logger.WithFields(map[string]any{
			"component":  "provisioning",
			"device_mac": device.MAC,
			"step":       step.name,
			"duration":   stepResult.Duration,
		}).Debug("Provisioning step completed successfully")
	}

	return nil
}

// updateStatus updates the current status and notifies callback
func (pm *ProvisioningManager) updateStatus(status ProvisioningStatus, result *ProvisioningResult) {
	pm.currentStatus = status
	result.Status = status

	if pm.statusCallback != nil {
		pm.statusCallback(status, result)
	}
}

// Stop stops any ongoing provisioning operation
func (pm *ProvisioningManager) Stop() {
	pm.logger.WithFields(map[string]any{
		"component": "provisioning",
	}).Info("Stopping provisioning manager")

	pm.currentStatus = StatusIdle
	pm.currentDevice = nil
	pm.currentRequest = nil
	pm.currentResult = nil
}

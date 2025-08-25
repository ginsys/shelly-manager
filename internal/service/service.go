package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/discovery"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
)

// ShellyService handles the core business logic
type ShellyService struct {
	DB        database.DatabaseInterface
	Config    *config.Config
	ConfigSvc *configuration.Service
	logger    *logging.Logger
	ctx       context.Context
	cancel    context.CancelFunc

	// Client cache for device connections
	clientMu sync.RWMutex
	clients  map[string]shelly.Client
}

// NewService creates a new Shelly service
func NewService(db database.DatabaseInterface, cfg *config.Config) *ShellyService {
	return NewServiceWithLogger(db, cfg, logging.GetDefault())
}

// NewServiceWithLogger creates a new Shelly service with custom logger
func NewServiceWithLogger(db database.DatabaseInterface, cfg *config.Config, logger *logging.Logger) *ShellyService {
	ctx, cancel := context.WithCancel(context.Background())

	// Create configuration service
	configSvc := configuration.NewService(db.GetDB(), logger)

	return &ShellyService{
		DB:        db,
		Config:    cfg,
		ConfigSvc: configSvc,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		clients:   make(map[string]shelly.Client),
	}
}

// DiscoverDevices performs device discovery using HTTP and mDNS
func (s *ShellyService) DiscoverDevices(network string) ([]database.Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.logger.WithFields(map[string]any{
		"network":   network,
		"component": "service",
	}).Info("Starting device discovery")

	// Determine networks to scan
	var networks []string
	if network != "" && network != "auto" {
		networks = []string{network}
	} else if len(s.Config.Discovery.Networks) > 0 {
		networks = s.Config.Discovery.Networks
	}

	s.logger.WithFields(map[string]any{
		"networks":  networks,
		"timeout":   s.Config.Discovery.Timeout,
		"component": "service",
	}).Debug("Discovery configuration")

	// Use timeout from config or default
	timeout := time.Duration(s.Config.Discovery.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	// Perform combined discovery (HTTP + mDNS)
	shellyDevices, err := discovery.CombinedDiscovery(ctx, networks, timeout)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Upsert discovered devices to preserve existing data
	var devices []database.Device
	for _, sd := range shellyDevices {
		// Skip devices without MAC address (can't use as unique identifier)
		if sd.MAC == "" {
			s.logger.WithFields(map[string]any{
				"device_ip": sd.IP,
				"device_id": sd.ID,
				"component": "service",
			}).Warn("Skipping device with empty MAC address")
			continue
		}

		// Prepare discovery update data
		update := database.DiscoveryUpdate{
			IP:       sd.IP,
			Type:     discovery.GetDeviceType(sd.Model),
			Firmware: sd.Version,
			Status:   "online",
			LastSeen: sd.Discovered,
		}

		// Use UpsertDeviceFromDiscovery to preserve existing data
		device, err := s.DB.UpsertDeviceFromDiscovery(sd.MAC, update, sd.ID)
		if err != nil {
			s.logger.WithFields(map[string]any{
				"mac":       sd.MAC,
				"ip":        sd.IP,
				"error":     err.Error(),
				"component": "service",
			}).Error("Failed to upsert device from discovery")
			continue
		}

		// Update device settings with latest discovery info (preserve existing settings)
		var existingSettings map[string]interface{}
		if err := json.Unmarshal([]byte(device.Settings), &existingSettings); err != nil {
			// If parsing fails, create new settings
			existingSettings = make(map[string]interface{})
		}

		// Update discovery-related settings only
		existingSettings["model"] = sd.Model
		existingSettings["gen"] = sd.Generation
		existingSettings["auth_enabled"] = sd.AuthEn

		// Preserve existing auth credentials if they exist
		if _, hasUser := existingSettings["auth_user"]; !hasUser {
			existingSettings["auth_user"] = ""
		}
		if _, hasPass := existingSettings["auth_pass"]; !hasPass {
			existingSettings["auth_pass"] = ""
		}

		updatedSettings, _ := json.Marshal(existingSettings)
		device.Settings = string(updatedSettings)

		// Save updated settings
		if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
	}; err != nil {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"error":     err.Error(),
				"component": "service",
			}).Error("Failed to update device settings")
		}

		devices = append(devices, *device)
	}

	s.logger.WithFields(map[string]any{
		"devices_found": len(devices),
		"component":     "service",
	}).Info("Discovery complete")

	log.Printf("Discovery complete. Found %d devices", len(devices))
	return devices, nil
}

// Stop gracefully stops the service
func (s *ShellyService) Stop() {
	s.logger.WithFields(map[string]any{
		"component": "service",
	}).Info("Stopping Shelly service")
	s.cancel()
}

// ClearClientCache clears the cached client for a specific device or all devices
func (s *ShellyService) ClearClientCache(deviceIP string) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if deviceIP == "" {
		// Clear all cached clients
		s.clients = make(map[string]shelly.Client)
		s.logger.WithFields(map[string]any{
			"component": "service",
		}).Info("Cleared all cached clients")
	} else {
		// Clear specific client
		delete(s.clients, deviceIP)
		s.logger.WithFields(map[string]any{
			"device_ip": deviceIP,
			"component": "service",
		}).Info("Cleared cached client for device")
	}
}

// getClient returns a cached client or creates a new one for the device
func (s *ShellyService) getClient(device *database.Device) (shelly.Client, error) {
	return s.getClientWithRetry(device, true)
}

// getClientWithAuthRetry gets a client and retries with config credentials if auth fails
func (s *ShellyService) getClientWithAuthRetry(device *database.Device) (shelly.Client, error) {
	// First, try to get a client normally
	client, err := s.getClientWithRetry(device, false) // Don't allow retry to prevent loops
	if err != nil {
		return nil, err
	}

	// Quick test to see if auth works - use a simple endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Try GetInfo first as it's faster and still returns auth errors
	s.logger.WithFields(map[string]any{
		"device_id":      device.ID,
		"device_ip":      device.IP,
		"testing_client": true,
		"component":      "service",
	}).Debug("Testing client authentication with GetInfo")

	_, testErr := client.GetInfo(ctx)

	if testErr != nil {
		s.logger.WithFields(map[string]any{
			"device_id":     device.ID,
			"device_ip":     device.IP,
			"error":         testErr.Error(),
			"is_auth_error": shelly.IsAuthError(testErr),
			"component":     "service",
		}).Warn("Client test failed")
	} else {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"device_ip": device.IP,
			"component": "service",
		}).Debug("Client test successful")
	}

	// If no error or non-auth error, return the client
	if testErr == nil || !shelly.IsAuthError(testErr) {
		return client, nil
	}

	// Auth failed, try to recover
	s.logger.WithFields(map[string]any{
		"device_id": device.ID,
		"device_ip": device.IP,
		"component": "service",
	}).Debug("Auth failed, attempting recovery")

	// Parse settings
	var settings struct {
		Model       string `json:"model"`
		Gen         int    `json:"gen"`
		AuthEnabled bool   `json:"auth_enabled"`
		AuthUser    string `json:"auth_user,omitempty"`
		AuthPass    string `json:"auth_pass,omitempty"`
	}

	if err := json.Unmarshal([]byte(device.Settings), &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	// Only retry if we had saved credentials (to avoid infinite loop)
	if settings.AuthUser != "" || settings.AuthPass != "" {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"device_ip": device.IP,
			"component": "service",
		}).Info("Clearing failed credentials and retrying with config")

		// Clear bad credentials
		settings.AuthUser = ""
		settings.AuthPass = ""
		updatedSettings, _ := json.Marshal(settings)
		device.Settings = string(updatedSettings)
		if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
	}; err != nil {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"error":     err.Error(),
			}).Error("Failed to update device with new settings")
		}

		// Clear from cache
		s.ClearClientCache(device.IP)

		// Retry with config credentials - use getClientWithRetry with false to prevent infinite recursion
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"device_ip": device.IP,
			"component": "service",
		}).Info("Retrying with config credentials after clearing bad saved credentials")

		retryClient, retryErr := s.getClientWithRetry(device, false)
		if retryErr != nil {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"error":     retryErr.Error(),
				"component": "service",
			}).Error("Retry with config credentials failed")
		} else {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"component": "service",
			}).Info("Retry with config credentials successful")
		}
		return retryClient, retryErr
	}

	// No saved credentials but auth required - config should have been tried already
	s.logger.WithFields(map[string]any{
		"device_id": device.ID,
		"device_ip": device.IP,
		"component": "service",
	}).Warn("Device requires auth but no working credentials available")

	return client, testErr // Return the client anyway, let the caller handle the auth error
}

// getClientWithRetry returns a cached client or creates a new one with retry logic
func (s *ShellyService) getClientWithRetry(device *database.Device, allowRetry bool) (shelly.Client, error) {
	s.clientMu.RLock()
	client, exists := s.clients[device.IP]
	s.clientMu.RUnlock()

	if exists {
		// Test if existing client still works
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.TestConnection(ctx); err == nil {
			return client, nil
		}
		// Client no longer works, recreate it
		s.clientMu.Lock()
		delete(s.clients, device.IP)
		s.clientMu.Unlock()
	}

	// Parse device settings to get generation and auth info
	var settings struct {
		Model       string `json:"model"`
		Gen         int    `json:"gen"`
		AuthEnabled bool   `json:"auth_enabled"`
		AuthUser    string `json:"auth_user,omitempty"`
		AuthPass    string `json:"auth_pass,omitempty"`
	}

	// Handle empty or invalid settings gracefully
	if device.Settings == "" {
		// Use default settings for devices without configuration
		settings = struct {
			Model       string `json:"model"`
			Gen         int    `json:"gen"`
			AuthEnabled bool   `json:"auth_enabled"`
			AuthUser    string `json:"auth_user,omitempty"`
			AuthPass    string `json:"auth_pass,omitempty"`
		}{
			Model:       "Unknown",
			Gen:         1, // Default to Gen1 for unknown devices
			AuthEnabled: false,
		}
	} else {
		if err := json.Unmarshal([]byte(device.Settings), &settings); err != nil {
			return nil, fmt.Errorf("failed to parse device settings: %w", err)
		}
	}

	// Create new client based on generation
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	// Check again in case another goroutine created it
	if client, exists = s.clients[device.IP]; exists {
		return client, nil
	}

	// Determine auth credentials to use
	var authUser, authPass string
	var saveCredentials bool

	if settings.AuthEnabled {
		// First try device-specific credentials if available
		if settings.AuthUser != "" && settings.AuthPass != "" {
			authUser = settings.AuthUser
			authPass = settings.AuthPass
			s.logger.WithFields(map[string]any{
				"device_id":       device.ID,
				"device_ip":       device.IP,
				"has_saved_creds": true,
				"component":       "service",
			}).Debug("Using saved device credentials")
		} else if s.Config.Provisioning.AuthEnabled {
			// Fall back to global config credentials
			authUser = s.Config.Provisioning.AuthUser
			authPass = s.Config.Provisioning.AuthPassword
			saveCredentials = true // Mark to save if they work
			s.logger.WithFields(map[string]any{
				"device_id":    device.ID,
				"device_ip":    device.IP,
				"using_config": true,
				"config_user":  authUser,
				"has_password": authPass != "",
				"component":    "service",
			}).Debug("Using config credentials")
		} else {
			s.logger.WithFields(map[string]any{
				"device_id":           device.ID,
				"device_ip":           device.IP,
				"auth_enabled":        settings.AuthEnabled,
				"config_auth_enabled": s.Config.Provisioning.AuthEnabled,
				"component":           "service",
			}).Warn("Device requires auth but no credentials available")
		}
	}

	// Create appropriate client based on generation
	switch settings.Gen {
	case 1:
		// Gen1 device
		var opts []gen1.ClientOption
		if authUser != "" && authPass != "" {
			opts = append(opts, gen1.WithAuth(authUser, authPass))
		}
		client = gen1.NewClient(device.IP, opts...)

	case 2, 3:
		// Gen2+ device
		var opts []gen2.ClientOption
		if authUser != "" && authPass != "" {
			opts = append(opts, gen2.WithAuth(authUser, authPass))
		}
		client = gen2.NewClient(device.IP, opts...)

	default:
		return nil, fmt.Errorf("unsupported device generation: %d", settings.Gen)
	}

	// Test the connection to verify credentials work
	if saveCredentials && authUser != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Try to get status to really verify credentials work
		_, err := client.GetStatus(ctx)
		if err == nil {
			// Credentials work, save them to device settings
			settings.AuthUser = authUser
			settings.AuthPass = authPass

			updatedSettings, _ := json.Marshal(settings)
			device.Settings = string(updatedSettings)

			// Update database with working credentials
			if updateErr := s.DB.UpdateDevice(device); updateErr != nil {
				s.logger.WithFields(map[string]any{
					"device_id": device.ID,
					"device_ip": device.IP,
					"error":     updateErr.Error(),
					"component": "service",
				}).Warn("Failed to save device credentials")
			} else {
				s.logger.WithFields(map[string]any{
					"device_id": device.ID,
					"device_ip": device.IP,
					"component": "service",
				}).Info("Saved working credentials to device")
			}
		} else if shelly.IsAuthError(err) {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"component": "service",
			}).Debug("Config credentials did not work for device")
		}
	}

	// Cache the client
	s.clients[device.IP] = client

	// Test the client works
	testCtx, testCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer testCancel()

	if err := client.TestConnection(testCtx); err != nil && shelly.IsAuthError(err) && allowRetry {
		// If auth failed with saved credentials, clear them and try with config credentials
		if settings.AuthUser != "" && settings.AuthPass != "" {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"component": "service",
			}).Info("Saved credentials failed, retrying with config credentials")

			// Clear the bad saved credentials
			settings.AuthUser = ""
			settings.AuthPass = ""
			updatedSettings, _ := json.Marshal(settings)
			device.Settings = string(updatedSettings)
			if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
	}; err != nil {
				s.logger.WithFields(map[string]any{
					"device_id": device.ID,
					"error":     err.Error(),
				}).Error("Failed to update device after clearing credentials")
			}

			// Clear from cache
			s.clientMu.Lock()
			delete(s.clients, device.IP)
			s.clientMu.Unlock()

			// Retry with config credentials
			return s.getClientWithRetry(device, false)
		}
	}

	return client, nil
}

// ControlDevice sends a control command to a device
func (s *ShellyService) ControlDevice(deviceID uint, action string, params map[string]interface{}) error {
	// Get device from database
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Get or create client
	client, err := s.getClient(device)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// Execute action with auth retry
	var actionErr error
	switch action {
	case "on":
		channel := 0
		if ch, ok := params["channel"].(float64); ok {
			channel = int(ch)
		}
		err = client.SetSwitch(ctx, channel, true)

	case "off":
		channel := 0
		if ch, ok := params["channel"].(float64); ok {
			channel = int(ch)
		}
		err = client.SetSwitch(ctx, channel, false)

	case "toggle":
		channel := 0
		if ch, ok := params["channel"].(float64); ok {
			channel = int(ch)
		}
		// Get current status
		status, statusErr := client.GetStatus(ctx)
		if statusErr != nil {
			return fmt.Errorf("failed to get status: %w", statusErr)
		}
		if len(status.Switches) > channel {
			newState := !status.Switches[channel].Output
			err = client.SetSwitch(ctx, channel, newState)
		} else {
			err = fmt.Errorf("channel %d not found", channel)
		}

	case "reboot":
		err = client.Reboot(ctx)

	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	actionErr = err

	// If auth failed, retry with cleared credentials
	if actionErr != nil && shelly.IsAuthError(actionErr) {
		s.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"device_ip": device.IP,
			"action":    action,
			"component": "service",
		}).Info("Auth failed, clearing credentials and retrying")

		// Parse and clear credentials
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(device.Settings), &settings); err == nil {
			if settings["auth_user"] != nil || settings["auth_pass"] != nil {
				// Clear bad credentials
				delete(settings, "auth_user")
				delete(settings, "auth_pass")
				updatedSettings, _ := json.Marshal(settings)
				device.Settings = string(updatedSettings)
				if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
	}

				// Clear from cache
				s.ClearClientCache(device.IP)

				// Get new client and retry
				client, err = s.getClient(device)
				if err == nil {
					// Retry the action
					switch action {
					case "on":
						channel := 0
						if ch, ok := params["channel"].(float64); ok {
							channel = int(ch)
						}
						actionErr = client.SetSwitch(ctx, channel, true)

					case "off":
						channel := 0
						if ch, ok := params["channel"].(float64); ok {
							channel = int(ch)
						}
						actionErr = client.SetSwitch(ctx, channel, false)

					case "toggle":
						channel := 0
						if ch, ok := params["channel"].(float64); ok {
							channel = int(ch)
						}
						status, err := client.GetStatus(ctx)
						if err == nil && len(status.Switches) > channel {
							newState := !status.Switches[channel].Output
							actionErr = client.SetSwitch(ctx, channel, newState)
						} else {
							actionErr = err
						}

					case "reboot":
						actionErr = client.Reboot(ctx)
					}

					if actionErr == nil {
						s.logger.WithFields(map[string]any{
							"device_id": deviceID,
							"device_ip": device.IP,
							"action":    action,
							"component": "service",
						}).Info("Action succeeded after auth retry")
					}
				}
			}
		}
	}

	if actionErr != nil {
		return fmt.Errorf("action failed: %w", actionErr)
	}

	// Update device status in database
	device.Status = "online"
	device.LastSeen = time.Now()
	if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
	}

	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"device_ip": device.IP,
		"action":    action,
		"component": "service",
	}).Info("Device control executed")

	return nil
}

// GetDeviceStatus retrieves the current status of a device
func (s *ShellyService) GetDeviceStatus(deviceID uint) (map[string]interface{}, error) {
	// Get device from database
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Get or create client
	client, err := s.getClient(device)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	// Get status from device
	status, err := client.GetStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Convert to map for JSON response
	result := map[string]interface{}{
		"device_id":   deviceID,
		"ip":          device.IP,
		"temperature": status.Temperature,
		"uptime":      status.Uptime,
		"wifi":        status.WiFiStatus,
		"switches":    status.Switches,
		"meters":      status.Meters,
	}

	// Update device in database
	device.Status = "online"
	device.LastSeen = time.Now()
	if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
	}

	return result, nil
}

// GetDeviceEnergy retrieves energy consumption data
func (s *ShellyService) GetDeviceEnergy(deviceID uint, channel int) (*shelly.EnergyData, error) {
	// Get device from database
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Get or create client
	client, err := s.getClient(device)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	// Get energy data
	energy, err := client.GetEnergyData(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("failed to get energy data: %w", err)
	}

	return energy, nil
}

// ImportDeviceConfig imports configuration from a physical device
func (s *ShellyService) ImportDeviceConfig(deviceID uint) (*configuration.DeviceConfig, error) {
	// Get device from database
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Get or create client with auth retry
	s.logger.WithFields(map[string]any{
		"device_id": device.ID,
		"device_ip": device.IP,
		"component": "service",
	}).Debug("ImportDeviceConfig: Getting client with auth retry")

	client, err := s.getClientWithAuthRetry(device)
	if err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"device_ip": device.IP,
			"error":     err.Error(),
			"component": "service",
		}).Error("ImportDeviceConfig: Failed to get client")
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"device_id": device.ID,
		"device_ip": device.IP,
		"component": "service",
	}).Debug("ImportDeviceConfig: Got client, proceeding to import")

	// Import configuration
	return s.ConfigSvc.ImportFromDevice(deviceID, client)
}

// GetDeviceConfig gets the stored configuration for a device
func (s *ShellyService) GetDeviceConfig(deviceID uint) (*configuration.DeviceConfig, error) {
	return s.ConfigSvc.GetDeviceConfig(deviceID)
}

// UpdateDeviceConfig updates the stored configuration for a device
func (s *ShellyService) UpdateDeviceConfig(deviceID uint, configUpdate map[string]interface{}) error {
	return s.ConfigSvc.UpdateDeviceConfig(deviceID, configUpdate)
}

// GetImportStatus gets the import status for a device
func (s *ShellyService) GetImportStatus(deviceID uint) (*configuration.ImportStatus, error) {
	return s.ConfigSvc.GetImportStatus(deviceID)
}

// UpdateRelayConfig updates relay-specific configuration
func (s *ShellyService) UpdateRelayConfig(deviceID uint, config *configuration.RelayConfig) error {
	return s.ConfigSvc.UpdateCapabilityConfig(deviceID, "relay", config)
}

// UpdateDimmingConfig updates dimming-specific configuration
func (s *ShellyService) UpdateDimmingConfig(deviceID uint, config *configuration.DimmingConfig) error {
	return s.ConfigSvc.UpdateCapabilityConfig(deviceID, "dimming", config)
}

// UpdateRollerConfig updates roller-specific configuration
func (s *ShellyService) UpdateRollerConfig(deviceID uint, config *configuration.RollerConfig) error {
	return s.ConfigSvc.UpdateCapabilityConfig(deviceID, "roller", config)
}

// UpdatePowerMeteringConfig updates power metering configuration
func (s *ShellyService) UpdatePowerMeteringConfig(deviceID uint, config *configuration.PowerMeteringConfig) error {
	return s.ConfigSvc.UpdateCapabilityConfig(deviceID, "power_metering", config)
}

// UpdateDeviceAuth updates device authentication credentials
func (s *ShellyService) UpdateDeviceAuth(deviceID uint, username, password string) error {
	// Get device
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Update device settings with auth info
	settings := make(map[string]interface{})
	if device.Settings != "" {
		if err := json.Unmarshal([]byte(device.Settings), &settings); err != nil {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"error":     err,
				"component": "service",
			}).Warn("Failed to unmarshal device settings, using empty settings")
		}
	}

	settings["auth"] = map[string]string{
		"username": username,
		"password": password,
	}

	settingsJSON, _ := json.Marshal(settings)
	device.Settings = string(settingsJSON)

	if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
		return err
	}
	return nil
}

// ExportDeviceConfig exports configuration to a physical device
func (s *ShellyService) ExportDeviceConfig(deviceID uint) error {
	// Get device from database
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Get or create client with auth retry
	client, err := s.getClientWithAuthRetry(device)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Export configuration
	return s.ConfigSvc.ExportToDevice(deviceID, client)
}

// DetectConfigDrift checks for configuration drift on a device
func (s *ShellyService) DetectConfigDrift(deviceID uint) (*configuration.ConfigDrift, error) {
	// Get device from database
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Get or create client with auth retry
	client, err := s.getClientWithAuthRetry(device)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Detect drift
	return s.ConfigSvc.DetectDrift(deviceID, client)
}

// BulkDetectConfigDrift checks for configuration drift across all devices
func (s *ShellyService) BulkDetectConfigDrift() (*configuration.BulkDriftResult, error) {
	// Get all devices
	devices, err := s.DB.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	if len(devices) == 0 {
		return &configuration.BulkDriftResult{
			Total:       0,
			InSync:      0,
			Drifted:     0,
			Errors:      0,
			Results:     []configuration.DriftResult{},
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
			Duration:    0,
		}, nil
	}

	// Extract device IDs
	deviceIDs := make([]uint, len(devices))
	for i, device := range devices {
		deviceIDs[i] = device.ID
	}

	// Create client getter function that uses our auth retry logic
	clientGetter := func(deviceID uint) (shelly.Client, error) {
		device, err := s.DB.GetDevice(deviceID)
		if err != nil {
			return nil, fmt.Errorf("device not found: %w", err)
		}

		return s.getClientWithAuthRetry(device)
	}

	// Perform bulk drift detection
	return s.ConfigSvc.BulkDetectDrift(deviceIDs, clientGetter)
}

// ApplyConfigTemplate applies a configuration template to a device
func (s *ShellyService) ApplyConfigTemplate(deviceID uint, templateID uint, variables map[string]interface{}) error {
	return s.ConfigSvc.ApplyTemplate(deviceID, templateID, variables)
}

// Drift Schedule Management Methods

// GetDriftSchedules returns all drift detection schedules
func (s *ShellyService) GetDriftSchedules() ([]configuration.DriftDetectionSchedule, error) {
	var schedules []configuration.DriftDetectionSchedule
	if err := s.DB.GetDB().Find(&schedules).Error; err != nil {
		return nil, fmt.Errorf("failed to get drift schedules: %w", err)
	}
	return schedules, nil
}

// CreateDriftSchedule creates a new drift detection schedule
func (s *ShellyService) CreateDriftSchedule(schedule configuration.DriftDetectionSchedule) (*configuration.DriftDetectionSchedule, error) {
	if err := s.DB.GetDB().Create(&schedule).Error; err != nil {
		return nil, fmt.Errorf("failed to create drift schedule: %w", err)
	}
	return &schedule, nil
}

// GetDriftSchedule returns a specific drift detection schedule
func (s *ShellyService) GetDriftSchedule(scheduleID uint) (*configuration.DriftDetectionSchedule, error) {
	var schedule configuration.DriftDetectionSchedule
	if err := s.DB.GetDB().First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("drift schedule not found: %w", err)
	}
	return &schedule, nil
}

// UpdateDriftSchedule updates an existing drift detection schedule
func (s *ShellyService) UpdateDriftSchedule(scheduleID uint, updates configuration.DriftDetectionSchedule) (*configuration.DriftDetectionSchedule, error) {
	var schedule configuration.DriftDetectionSchedule
	if err := s.DB.GetDB().First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("drift schedule not found: %w", err)
	}

	if err := s.DB.GetDB().Model(&schedule).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update drift schedule: %w", err)
	}

	// Reload the updated schedule
	if err := s.DB.GetDB().First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload updated schedule: %w", err)
	}

	return &schedule, nil
}

// DeleteDriftSchedule removes a drift detection schedule
func (s *ShellyService) DeleteDriftSchedule(scheduleID uint) error {
	if err := s.DB.GetDB().Delete(&configuration.DriftDetectionSchedule{}, scheduleID).Error; err != nil {
		return fmt.Errorf("failed to delete drift schedule: %w", err)
	}
	return nil
}

// ToggleDriftSchedule toggles the enabled status of a drift detection schedule
func (s *ShellyService) ToggleDriftSchedule(scheduleID uint) (*configuration.DriftDetectionSchedule, error) {
	var schedule configuration.DriftDetectionSchedule
	if err := s.DB.GetDB().First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("drift schedule not found: %w", err)
	}

	// Toggle the enabled status
	schedule.Enabled = !schedule.Enabled

	if err := s.DB.GetDB().Save(&schedule).Error; err != nil {
		return nil, fmt.Errorf("failed to toggle drift schedule: %w", err)
	}

	return &schedule, nil
}

// GetDriftScheduleRuns returns the execution history for a schedule
func (s *ShellyService) GetDriftScheduleRuns(scheduleID uint, limit int) ([]configuration.DriftDetectionRun, error) {
	var runs []configuration.DriftDetectionRun
	query := s.DB.GetDB().Where("schedule_id = ?", scheduleID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("failed to get drift schedule runs: %w", err)
	}

	return runs, nil
}

// Comprehensive Drift Reporting Methods

// GetDriftReports returns drift reports with optional filtering
func (s *ShellyService) GetDriftReports(reportType string, deviceID *uint, limit int) ([]configuration.DriftReport, error) {
	return s.ConfigSvc.GetDriftReports(reportType, deviceID, limit)
}

// GenerateDeviceDriftReport generates a comprehensive drift report for a single device
func (s *ShellyService) GenerateDeviceDriftReport(deviceID uint) (*configuration.DriftReport, error) {
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	client, err := s.getClientWithAuthRetry(device)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return s.ConfigSvc.GenerateDeviceDriftReport(deviceID, client)
}

// GetDriftTrends returns drift trends with optional filtering
func (s *ShellyService) GetDriftTrends(deviceID *uint, resolved *bool, limit int) ([]configuration.DriftTrend, error) {
	return s.ConfigSvc.GetDriftTrends(deviceID, resolved, limit)
}

// MarkTrendResolved marks a drift trend as resolved
func (s *ShellyService) MarkTrendResolved(trendID uint) error {
	return s.ConfigSvc.MarkTrendResolved(trendID)
}

// EnhanceBulkDriftResult adds comprehensive reporting to bulk drift results
func (s *ShellyService) EnhanceBulkDriftResult(result *configuration.BulkDriftResult, scheduleID *uint) (*configuration.DriftReport, error) {
	return s.ConfigSvc.EnhanceBulkDriftResult(result, scheduleID)
}

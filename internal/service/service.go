package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/discovery"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
)

// ShellyService handles the core business logic
type ShellyService struct {
	DB     *database.Manager
	Config *config.Config
	logger *logging.Logger
	ctx    context.Context
	cancel context.CancelFunc
	
	// Client cache for device connections
	clientMu sync.RWMutex
	clients  map[string]shelly.Client
}

// NewService creates a new Shelly service
func NewService(db *database.Manager, cfg *config.Config) *ShellyService {
	return NewServiceWithLogger(db, cfg, logging.GetDefault())
}

// NewServiceWithLogger creates a new Shelly service with custom logger
func NewServiceWithLogger(db *database.Manager, cfg *config.Config, logger *logging.Logger) *ShellyService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ShellyService{
		DB:      db,
		Config:  cfg,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		clients: make(map[string]shelly.Client),
	}
}

// DiscoverDevices performs device discovery using HTTP and mDNS
func (s *ShellyService) DiscoverDevices(network string) ([]database.Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.logger.WithFields(map[string]any{
		"network": network,
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
		"networks": networks,
		"timeout": s.Config.Discovery.Timeout,
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
	
	// Convert discovered Shelly devices to our Device model
	var devices []database.Device
	for _, sd := range shellyDevices {
		device := database.Device{
			IP:       sd.IP,
			MAC:      sd.MAC,
			Type:     discovery.GetDeviceType(sd.Model),
			Name:     sd.ID, // Use ID as initial name, can be updated later
			Firmware: sd.Version,
			Status:   "online",
			LastSeen: sd.Discovered,
			Settings: fmt.Sprintf(`{"model":"%s","gen":%d,"auth_enabled":%v}`, 
				sd.Model, sd.Generation, sd.AuthEn),
		}
		devices = append(devices, device)
	}
	
	s.logger.WithFields(map[string]any{
		"devices_found": len(devices),
		"component": "service",
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
	client, err := s.getClient(device)
	if err != nil {
		return nil, err
	}
	
	// Quick test to see if auth works
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	_, testErr := client.GetInfo(ctx)
	if testErr != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"device_ip": device.IP,
			"error": testErr.Error(),
			"is_auth_error": shelly.IsAuthError(testErr),
			"component": "service",
		}).Debug("Client test result")
		
		if shelly.IsAuthError(testErr) {
			// Auth failed, clear saved credentials and retry
			var settings struct {
				Model       string `json:"model"`
				Gen         int    `json:"gen"`
				AuthEnabled bool   `json:"auth_enabled"`
				AuthUser    string `json:"auth_user,omitempty"`
				AuthPass    string `json:"auth_pass,omitempty"`
			}
			
			if err := json.Unmarshal([]byte(device.Settings), &settings); err == nil {
				if settings.AuthUser != "" || settings.AuthPass != "" {
					s.logger.WithFields(map[string]any{
						"device_id": device.ID,
						"device_ip": device.IP,
						"had_user": settings.AuthUser != "",
						"had_pass": settings.AuthPass != "",
						"component": "service",
					}).Info("Clearing failed credentials and retrying with config")
				
				// Clear bad credentials
				settings.AuthUser = ""
				settings.AuthPass = ""
				updatedSettings, _ := json.Marshal(settings)
				device.Settings = string(updatedSettings)
				s.DB.UpdateDevice(device)
				
				// Clear from cache
				s.ClearClientCache(device.IP)
				
					// Retry
					return s.getClient(device)
				}
			}
		}
	}
	
	return client, nil
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
	
	if err := json.Unmarshal([]byte(device.Settings), &settings); err != nil {
		return nil, fmt.Errorf("failed to parse device settings: %w", err)
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
				"device_id": device.ID,
				"device_ip": device.IP,
				"has_saved_creds": true,
				"component": "service",
			}).Debug("Using saved device credentials")
		} else if s.Config.Provisioning.AuthEnabled {
			// Fall back to global config credentials
			authUser = s.Config.Provisioning.AuthUser
			authPass = s.Config.Provisioning.AuthPassword
			saveCredentials = true // Mark to save if they work
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"using_config": true,
				"component": "service",
			}).Debug("Using config credentials")
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
			if err := s.DB.UpdateDevice(device); err != nil {
				s.logger.WithFields(map[string]any{
					"device_id": device.ID,
					"device_ip": device.IP,
					"error":     err.Error(),
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
			s.DB.UpdateDevice(device)
			
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
		status, err := client.GetStatus(ctx)
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}
		if len(status.Switches) > channel {
			newState := !status.Switches[channel].Output
			err = client.SetSwitch(ctx, channel, newState)
		} else {
			return fmt.Errorf("channel %d not found", channel)
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
			"action": action,
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
				s.DB.UpdateDevice(device)
				
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
							"action": action,
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
	s.DB.UpdateDevice(device)
	
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
		"device_id":    deviceID,
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
	s.DB.UpdateDevice(device)
	
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
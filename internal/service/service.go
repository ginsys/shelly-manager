package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/discovery"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
)

// ErrDeviceOffline is returned when a device is known to be offline and communication is skipped.
var ErrDeviceOffline = errors.New("device is offline")

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

	// clientOwner and clientAddr record which device each cached client was built for, so an
	// entry left at a device's old address is found and dropped when the device moves, and is
	// never handed to another device that takes the address. A client cached without an owner
	// (a device with ID 0) is matched by address alone. Both maps are guarded by clientMu.
	clientOwner map[string]uint
	clientAddr  map[uint]string

	// discover runs the network discovery. Nil means discovery.CombinedDiscovery.
	discover DiscoverFunc

	// beforeClientBuild is a test hook. getClientWithRetry calls it after a cache miss and before
	// it takes the cache lock to build a client. It is nil in production.
	beforeClientBuild func()

	// beforeClientPublish is a test hook. getClientWithRetry calls it while it holds the cache
	// lock, after it reloaded the device and built the client, and before it caches the client.
	// It is nil in production.
	beforeClientPublish func()
}

// DiscoverFunc runs one combined HTTP and mDNS discovery.
type DiscoverFunc func(ctx context.Context, networks []string, timeout time.Duration) ([]discovery.ShellyDevice, error)

// SetDiscoverFunc replaces the discovery that DiscoverDevices runs. A test uses it so that
// it never scans a real network. Call it before the first DiscoverDevices.
func (s *ShellyService) SetDiscoverFunc(fn DiscoverFunc) { s.discover = fn }

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

		clientOwner: make(map[string]uint),
		clientAddr:  make(map[uint]string),
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
	discover := s.discover
	if discover == nil {
		discover = discovery.CombinedDiscovery
	}
	shellyDevices, err := discover(ctx, networks, timeout)
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
		s.clientOwner = make(map[string]uint)
		s.clientAddr = make(map[uint]string)
		s.logger.WithFields(map[string]any{
			"component": "service",
		}).Info("Cleared all cached clients")
	} else {
		// Clear specific client
		s.dropClientLocked(deviceIP)
		s.logger.WithFields(map[string]any{
			"device_ip": deviceIP,
			"component": "service",
		}).Info("Cleared cached client for device")
	}
}

// dropClientLocked removes the cached client at ip and its owner record. The caller holds
// clientMu for writing.
func (s *ShellyService) dropClientLocked(ip string) {
	delete(s.clients, ip)
	if id, ok := s.clientOwner[ip]; ok {
		delete(s.clientOwner, ip)
		if s.clientAddr[id] == ip {
			delete(s.clientAddr, id)
		}
	}
}

// cachedClientLocked returns the cached client for the device at its current address. It first
// drops what the cache holds under a stale address: the entry at the address the device used
// before it moved, and an entry at this address that was built for another device. The caller
// holds clientMu for writing.
func (s *ShellyService) cachedClientLocked(device *database.Device) (shelly.Client, bool) {
	if prev, ok := s.clientAddr[device.ID]; ok && prev != device.IP {
		s.dropClientLocked(prev)
	}
	if owner, ok := s.clientOwner[device.IP]; ok && owner != device.ID {
		s.dropClientLocked(device.IP)
	}
	client, ok := s.clients[device.IP]
	return client, ok
}

// storeClientLocked caches the client for the device at its current address and records the
// device as its owner. The caller holds clientMu for writing.
func (s *ShellyService) storeClientLocked(device *database.Device, client shelly.Client) {
	s.dropClientLocked(device.IP)
	s.clients[device.IP] = client
	if device.ID == 0 {
		return
	}
	s.clientOwner[device.IP] = device.ID
	s.clientAddr[device.ID] = device.IP
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

	// Parse settings. A login saved only in the legacy settings.auth pair counts as a saved
	// login: the client builders use it, so a rejected one is cleared the same way.
	settings, parseErr := parseClientSettings(device.Settings)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", parseErr)
	}

	// Only retry if we had saved credentials (to avoid infinite loop)
	if settings.AuthUser != "" || settings.AuthPass != "" {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"device_ip": device.IP,
			"component": "service",
		}).Info("Clearing failed credentials and retrying with config")

		// Clear the login the device rejected, unless a newer one was saved meanwhile. The
		// retry below reads the stored record either way.
		s.clearRejectedLogin(device, settings.AuthUser, settings.AuthPass)

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

// clientSettings is what the client builders read from the settings of a device.
type clientSettings struct {
	Model       string `json:"model"`
	Gen         int    `json:"gen"`
	AuthEnabled bool   `json:"auth_enabled"`
	AuthUser    string `json:"auth_user,omitempty"`
	AuthPass    string `json:"auth_pass,omitempty"`
}

// parseClientSettings reads the generation and the login from the settings of a device.
func parseClientSettings(raw string) (clientSettings, error) {
	// Handle empty settings gracefully: a device without configuration is a Gen1 device.
	if raw == "" {
		return clientSettings{Model: "Unknown", Gen: 1}, nil
	}
	var settings clientSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return settings, fmt.Errorf("failed to parse device settings: %w", err)
	}
	// Credentials saved before the flat keys existed are still honoured.
	if settings.AuthUser == "" && settings.AuthPass == "" {
		if user, pass, ok := configuration.LegacyDeviceAuth(raw); ok {
			settings.AuthEnabled = true
			settings.AuthUser, settings.AuthPass = user, pass
		}
	}
	return settings, nil
}

// getClientWithRetry returns a cached client or creates a new one with retry logic
func (s *ShellyService) getClientWithRetry(device *database.Device, allowRetry bool) (shelly.Client, error) {
	s.clientMu.Lock()
	client, exists := s.cachedClientLocked(device)
	s.clientMu.Unlock()

	if exists {
		// Test if existing client still works
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.TestConnection(ctx); err == nil {
			return client, nil
		}
		// Client no longer works, recreate it
		s.evictClient(device.IP, client)
	}

	if s.beforeClientBuild != nil {
		s.beforeClientBuild()
	}

	// Create new client based on generation
	// The cache lock is released once the client is cached: the connection test and the retry
	// below run without it, because the retry takes the lock again.
	s.clientMu.Lock()

	// Check again in case another goroutine created it
	if client, exists = s.cachedClientLocked(device); exists {
		s.clientMu.Unlock()
		return client, nil
	}

	// The device passed in may predate a credential update. UpdateDeviceAuth writes the
	// database and then clears the cache under this lock, so a reload here sees the new login,
	// or the clear runs after the publish below and removes the client built from the old one.
	// A failed reload is an error: the old settings could be the ones the update replaced. A
	// device that the database does not hold has no update to miss, because UpdateDeviceAuth
	// refuses it, so its own settings stand.
	if device.ID != 0 {
		fresh, loadErr := s.DB.GetDevice(device.ID)
		switch {
		case loadErr == nil:
			device.Settings = fresh.Settings
		case !errors.Is(loadErr, gorm.ErrRecordNotFound):
			s.clientMu.Unlock()
			return nil, fmt.Errorf("failed to reload device %d: %w", device.ID, loadErr)
		}
	}

	// Parse device settings to get generation and auth info
	settings, parseErr := parseClientSettings(device.Settings)
	if parseErr != nil {
		s.clientMu.Unlock()
		return nil, parseErr
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
		s.clientMu.Unlock()
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

	if s.beforeClientPublish != nil {
		s.beforeClientPublish()
	}

	// Cache the client
	s.storeClientLocked(device, client)
	s.clientMu.Unlock()

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

			// Clear the saved login the device rejected, unless a newer one replaced it.
			s.clearRejectedLogin(device, settings.AuthUser, settings.AuthPass)
			s.evictClient(device.IP, client)

			// Retry with config credentials
			return s.getClientWithRetry(device, false)
		}
	}

	return client, nil
}

// evictClient removes the cached client for ip only if it is still the given client. The
// connection test runs without the cache lock, so another request can already have cached a
// working replacement; a late failure of the old client must not evict it.
func (s *ShellyService) evictClient(ip string, client shelly.Client) {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	if cached, ok := s.clients[ip]; ok && cached == client {
		s.dropClientLocked(ip)
	}
}

// clearRejectedLogin removes the saved login from the device record, but only while the record
// still holds the login that the device rejected. It reloads the record under clientMu, the lock
// that UpdateDeviceAuth holds for its write, so a login saved after the rejected request is
// never erased, and it writes the reloaded record, never the caller's older snapshot. It
// returns true when the caller should retry: the login was cleared, or a different login is
// stored now. The caller must not hold clientMu.
func (s *ShellyService) clearRejectedLogin(device *database.Device, rejectedUser, rejectedPass string) bool {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	record := device
	if device.ID != 0 {
		fresh, err := s.DB.GetDevice(device.ID)
		if err != nil {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"error":     err.Error(),
				"component": "service",
			}).Error("Failed to reload device before clearing credentials")
			return false
		}
		record = fresh
	}

	stored, err := parseClientSettings(record.Settings)
	if err != nil {
		return false
	}
	if stored.AuthUser != rejectedUser || stored.AuthPass != rejectedPass {
		// The stored login is not the rejected one. Keep it. The cached client may carry the
		// rejected login (a cache hit never refreshes the caller's snapshot), so drop it and
		// let the caller retry from the stored record.
		device.Settings = record.Settings
		s.dropClientLocked(device.IP)
		return true
	}

	var settings map[string]interface{}
	if err = json.Unmarshal([]byte(record.Settings), &settings); err != nil {
		return false
	}
	// The nested pair counts too: the client builders fall back to it when the flat keys are
	// gone, so leaving it would retry with the login the device just rejected. A boolean "auth"
	// from discovery is not a login and stays.
	_, nestedPair := settings["auth"].(map[string]interface{})
	if settings["auth_user"] == nil && settings["auth_pass"] == nil && !nestedPair {
		return false
	}
	delete(settings, "auth_user")
	delete(settings, "auth_pass")
	if nestedPair {
		delete(settings, "auth")
	}
	// The client builders read a legacy pair as auth enabled. Keep that after the pair is gone,
	// so the retry falls back to the provisioning login and does not go unauthenticated.
	if stored.AuthEnabled {
		settings["auth_enabled"] = true
	}
	updated, err := json.Marshal(settings)
	if err != nil {
		return false
	}
	record.Settings = string(updated)
	if err := s.DB.UpdateDevice(record); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
			"component": "service",
		}).Error("Failed to update device after clearing credentials")
		return false
	}
	device.Settings = record.Settings
	s.dropClientLocked(device.IP)
	return true
}

// recordDeviceOnline marks the device online. It writes the reloaded record under clientMu, so
// the write cannot put an older login back over one that UpdateDeviceAuth saved meanwhile.
func (s *ShellyService) recordDeviceOnline(device *database.Device) error {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	record := device
	if device.ID != 0 {
		fresh, err := s.DB.GetDevice(device.ID)
		if err != nil {
			return err
		}
		record = fresh
	}
	record.Status = "online"
	record.LastSeen = time.Now()
	if err := s.DB.UpdateDevice(record); err != nil {
		return err
	}
	device.Status, device.LastSeen, device.Settings = record.Status, record.LastSeen, record.Settings
	return nil
}

// ControlDevice sends a control command to a device
func (s *ShellyService) ControlDevice(deviceID uint, action string, params map[string]interface{}) error {
	// Get device from database
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Pre-check: fail fast if device is offline (bypass with force flag)
	force, _ := params["force"].(bool)
	if !force && device.Status == "offline" {
		return ErrDeviceOffline
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

		// Clear the login the device rejected, unless a newer one was saved meanwhile.
		rejected, parseErr := parseClientSettings(device.Settings)
		if parseErr == nil {
			if s.clearRejectedLogin(device, rejected.AuthUser, rejected.AuthPass) {
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
	if err := s.recordDeviceOnline(device); err != nil {
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

	if device.Status == "offline" {
		// Allow a short revalidation probe if recently marked offline (within 5 minutes)
		if time.Since(device.LastSeen) <= 5*time.Minute {
			client, clientErr := s.getClient(device)
			if clientErr == nil {
				probeCtx, probeCancel := context.WithTimeout(s.ctx, 3*time.Second)
				defer probeCancel()
				if status, probeErr := client.GetStatus(probeCtx); probeErr == nil {
					device.Status = "online"
					device.LastSeen = time.Now()
					_ = s.DB.UpdateDevice(device)
					return map[string]interface{}{
						"device_id":   deviceID,
						"ip":          device.IP,
						"temperature": status.Temperature,
						"uptime":      status.Uptime,
						"wifi":        status.WiFiStatus,
						"switches":    status.Switches,
						"meters":      status.Meters,
					}, nil
				}
			}
		}
		return nil, ErrDeviceOffline
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

	if device.Status == "offline" {
		return nil, ErrDeviceOffline
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

// ReadDeviceConfig reads the device's live configuration and stores nothing:
// the stored configuration, its sync status and its history stay as they were,
// so drift detection keeps its baseline. Use ImportDeviceConfig to accept the
// device's state as the new stored configuration.
func (s *ShellyService) ReadDeviceConfig(deviceID uint) (*configuration.DeviceConfig, error) {
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	client, err := s.getClientWithAuthRetry(device)
	if err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"device_ip": device.IP,
			"error":     err.Error(),
			"component": "service",
		}).Error("ReadDeviceConfig: Failed to get client")
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return s.ConfigSvc.ReadFromDevice(deviceID, client)
}

// ApplyDesiredConfig pushes the device's desired configuration to the physical
// device and verifies it by reading it back.
func (s *ShellyService) ApplyDesiredConfig(ctx context.Context, deviceID uint) (*configuration.ApplyVerifyResult, error) {
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	client, err := s.getClientWithAuthRetry(device)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return s.ConfigSvc.ConfigurationSvc.ApplyDesiredConfig(ctx, deviceID, client, DeviceModel(device))
}

// DeviceModel returns the model identifier the configuration converters match on (for example
// "SHPLG-S"). Discovery stores it in the device settings; device.Type holds a display label such
// as "Smart Plug", so it is only a fallback for records that have no stored model.
func DeviceModel(device *database.Device) string {
	var settings struct {
		Model string `json:"model"`
	}
	if device.Settings != "" {
		if err := json.Unmarshal([]byte(device.Settings), &settings); err == nil && settings.Model != "" {
			return settings.Model
		}
	}
	return device.Type
}

// VerifyDesiredConfig compares the device's desired configuration with the
// live configuration on the physical device. It changes nothing.
func (s *ShellyService) VerifyDesiredConfig(ctx context.Context, deviceID uint) (*configuration.VerifyResult, error) {
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	client, err := s.getClientWithAuthRetry(device)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return s.ConfigSvc.ConfigurationSvc.VerifyDesiredConfig(ctx, deviceID, client, DeviceModel(device))
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
	// The read, the write and the cache drop happen under one hold of clientMu. A 401 path that
	// compares the stored login with the one the device rejected (clearRejectedLogin) then sees
	// either the old record or the finished update, never the middle.
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	// Get device
	device, err := s.DB.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Update device settings with auth info
	settings := make(map[string]interface{})
	if device.Settings != "" {
		if unmarshalErr := json.Unmarshal([]byte(device.Settings), &settings); unmarshalErr != nil {
			s.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"error":     unmarshalErr,
				"component": "service",
			}).Warn("Failed to unmarshal device settings, using empty settings")
		}
	}

	// Store under the keys the client builders read (auth_enabled, auth_user, auth_pass), and keep
	// the nested "auth" pair that older versions wrote in step with them. An SMA archive in the
	// older shape carries only "auth", redacted, and a forced restore takes its value from the
	// stored "auth"; the builders read that pair when the flat keys are absent
	// (configuration.LegacyDeviceAuth). Without it, such a restore drops the login.
	// An empty user disables auth: both client builders need a user and a password.
	if username == "" {
		settings["auth_enabled"] = false
		delete(settings, "auth_user")
		delete(settings, "auth_pass")
		delete(settings, "auth")
	} else {
		settings["auth_enabled"] = true
		settings["auth_user"] = username
		settings["auth_pass"] = password
		settings["auth"] = map[string]string{"username": username, "password": password}
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to encode device settings: %w", err)
	}
	device.Settings = string(settingsJSON)

	if err := s.DB.UpdateDevice(device); err != nil {
		s.logger.WithFields(map[string]any{
			"device_id": device.ID,
			"error":     err.Error(),
		}).Error("Failed to update device")
		return err
	}

	// A cached client still carries the previous credentials. ClearClientCache takes clientMu,
	// which this function already holds, so the entry goes inline.
	s.dropClientLocked(device.IP)
	s.logger.WithFields(map[string]any{
		"device_ip": device.IP,
		"component": "service",
	}).Info("Cleared cached client for device")
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

	if device.Status == "offline" {
		return nil, ErrDeviceOffline
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

// GetDriftSchedule returns a specific drift detection schedule
func (s *ShellyService) GetDriftSchedule(scheduleID uint) (*configuration.DriftDetectionSchedule, error) {
	var schedule configuration.DriftDetectionSchedule
	if err := s.DB.GetDB().First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("drift schedule not found: %w", err)
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

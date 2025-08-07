package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/discovery"
)

// ShellyService handles the core business logic
type ShellyService struct {
	DB     *database.Manager
	Config *config.Config
	ctx    context.Context
	cancel context.CancelFunc
}

// NewService creates a new Shelly service
func NewService(db *database.Manager, cfg *config.Config) *ShellyService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ShellyService{
		DB:     db,
		Config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// DiscoverDevices performs device discovery using HTTP and mDNS
func (s *ShellyService) DiscoverDevices(network string) ([]database.Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("Starting device discovery on network: %s", network)
	
	// Determine networks to scan
	var networks []string
	if network != "" && network != "auto" {
		networks = []string{network}
	} else if len(s.Config.Discovery.Networks) > 0 {
		networks = s.Config.Discovery.Networks
	}
	
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
	
	log.Printf("Discovery complete. Found %d devices", len(devices))
	return devices, nil
}

// Stop gracefully stops the service
func (s *ShellyService) Stop() {
	s.cancel()
}
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// ShellyDevice represents a discovered Shelly device
type ShellyDevice struct {
	// Gen2+ fields
	ID         string `json:"id"`
	Model      string `json:"model"`
	Generation int    `json:"gen"`
	FirmwareID string `json:"fw_id"`
	Version    string `json:"ver"`
	App        string `json:"app"`
	AuthDomain string `json:"auth_domain"`

	// Gen1 fields
	Type string `json:"type"`
	FW   string `json:"fw"`
	Auth bool   `json:"auth"`

	// Common fields
	MAC    string `json:"mac"`
	AuthEn bool   `json:"auth_en"`

	// Internal fields
	IP         string    `json:"-"`
	Discovered time.Time `json:"-"`
}

// Scanner handles device discovery operations
type Scanner struct {
	timeout         time.Duration
	concurrentScans int
	httpClient      *http.Client
	logger          *logging.Logger
}

// NewScanner creates a new discovery scanner
func NewScanner(timeout time.Duration, concurrentScans int) *Scanner {
	return NewScannerWithLogger(timeout, concurrentScans, logging.GetDefault())
}

// NewScannerWithLogger creates a new discovery scanner with custom logger
func NewScannerWithLogger(timeout time.Duration, concurrentScans int, logger *logging.Logger) *Scanner {
	if concurrentScans <= 0 {
		concurrentScans = 10
	}
	if timeout <= 0 {
		timeout = 1 * time.Second // Reduced timeout for faster scanning
	}

	return &Scanner{
		timeout:         timeout,
		concurrentScans: concurrentScans,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// ScanNetwork performs HTTP-based discovery on the specified network range
func (s *Scanner) ScanNetwork(ctx context.Context, cidr string) ([]ShellyDevice, error) {
	start := time.Now()
	s.logger.WithFields(map[string]any{
		"network":   cidr,
		"component": "discovery",
	}).Info("Starting network scan")

	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		s.logger.LogDiscoveryOperation("network_scan", cidr, 0, time.Since(start).Milliseconds(), err)
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	var devices []ShellyDevice
	var mu sync.Mutex
	var wg sync.WaitGroup
	var scanned, found int32

	// Create a channel for IPs to scan
	ipChan := make(chan string, 100)

	// Start worker goroutines
	for i := 0; i < s.concurrentScans; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for ip := range ipChan {
				select {
				case <-ctx.Done():
					return
				default:
					if device := s.checkDevice(ctx, ip); device != nil {
						mu.Lock()
						devices = append(devices, *device)
						mu.Unlock()
						atomic.AddInt32(&found, 1)
						fmt.Printf("Found Shelly device at %s: %s\n", device.IP, device.Model)
					}
					currentScanned := atomic.AddInt32(&scanned, 1)
					if currentScanned%50 == 0 {
						currentFound := atomic.LoadInt32(&found)
						fmt.Printf("Scanned %d IPs, found %d devices...\n", currentScanned, currentFound)
					}
				}
			}
		}(i)
	}

	// Generate IPs to scan
	go func() {
		defer close(ipChan)
		for currentIP := ip.Mask(ipnet.Mask); ipnet.Contains(currentIP); inc(currentIP) {
			select {
			case <-ctx.Done():
				return
			case ipChan <- currentIP.String():
			}
		}
	}()

	wg.Wait()
	duration := time.Since(start).Milliseconds()
	finalFound := atomic.LoadInt32(&found)
	finalScanned := atomic.LoadInt32(&scanned)
	s.logger.LogDiscoveryOperation("network_scan", cidr, int(finalFound), duration, nil)
	fmt.Printf("Scan complete: checked %d IPs, found %d devices\n", finalScanned, finalFound)
	return devices, nil
}

// ScanHost checks a specific host for Shelly device
func (s *Scanner) ScanHost(ctx context.Context, host string) (*ShellyDevice, error) {
	start := time.Now()
	s.logger.WithFields(map[string]any{
		"host":      host,
		"component": "discovery",
	}).Debug("Scanning host")

	device := s.checkDevice(ctx, host)
	duration := time.Since(start).Milliseconds()
	devicesFound := 0
	if device != nil {
		devicesFound = 1
	}
	s.logger.LogDiscoveryOperation("host_scan", host, devicesFound, duration, nil)
	return device, nil
}

// checkDevice attempts to identify a Shelly device at the given IP
func (s *Scanner) checkDevice(ctx context.Context, ip string) *ShellyDevice {
	url := fmt.Sprintf("http://%s/shelly", ip)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.logger.WithFields(map[string]any{
			"ip":        ip,
			"error":     err.Error(),
			"component": "discovery",
		}).Debug("Failed to create request")
		return nil
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Only log errors that aren't connection timeouts/refusals (too noisy)
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "refused") {
			s.logger.WithFields(map[string]any{
				"ip":        ip,
				"error":     err.Error(),
				"component": "discovery",
			}).Debug("HTTP request failed")
		}
		return nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			s.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"component": "discovery",
			}).Debug("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var device ShellyDevice
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return nil
	}

	// Validate it's a Shelly device (Gen1 has Type, Gen2+ has ID)
	if device.Type == "" && device.ID == "" {
		return nil
	}

	// Normalize fields for Gen1 devices
	if device.Type != "" && device.ID == "" {
		// Gen1 device
		device.ID = fmt.Sprintf("shelly%s-%s", strings.ToLower(device.Type), device.MAC)
		device.Model = device.Type
		device.Generation = 1
		device.Version = device.FW
		device.AuthEn = device.Auth
	}

	// Ensure we have a model field
	if device.Model == "" && device.Type != "" {
		device.Model = device.Type
	}

	device.IP = ip
	device.Discovered = time.Now()

	s.logger.LogDeviceOperation("discovered", ip, device.MAC, nil)

	return &device
}

// GetDeviceStatus retrieves the current status of a Shelly device
func (s *Scanner) GetDeviceStatus(ctx context.Context, ip string, gen int) (map[string]interface{}, error) {
	start := time.Now()
	s.logger.WithFields(map[string]any{
		"device_ip":  ip,
		"generation": gen,
		"component":  "discovery",
	}).Debug("Getting device status")

	var url string
	if gen == 1 {
		url = fmt.Sprintf("http://%s/status", ip)
	} else {
		// Gen2+ uses RPC-style endpoints
		url = fmt.Sprintf("http://%s/rpc/Shelly.GetStatus", ip)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.logger.LogDeviceOperation("get_status", ip, "", err)
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.LogDeviceOperation("get_status", ip, "", err)
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			s.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"component": "discovery",
			}).Debug("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("status code: %d", resp.StatusCode)
		s.logger.LogDeviceOperation("get_status", ip, "", err)
		return nil, err
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		s.logger.LogDeviceOperation("get_status", ip, "", err)
		return nil, err
	}

	duration := time.Since(start).Milliseconds()
	s.logger.WithFields(map[string]any{
		"device_ip": ip,
		"duration":  duration,
		"component": "discovery",
	}).Debug("Device status retrieved successfully")

	return status, nil
}

// inc increments an IP address
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// GetDeviceType returns a human-readable device type based on the model
func GetDeviceType(model string) string {
	model = strings.ToUpper(model)

	// Exact model matches for Gen1 devices
	switch model {
	case "SHPLG-S", "SHPLG-1":
		return "Smart Plug"
	case "SHSW-1":
		return "Relay Switch"
	case "SHSW-PM":
		return "Power Meter Switch"
	case "SHSW-25":
		return "Dual Relay/Roller"
	case "SHIX3-1":
		return "3-Input Controller"
	case "SHBTN-1", "SHBTN-2":
		return "Button Controller"
	case "SHDM-1", "SHDM-2":
		return "Dimmer"
	case "SHRGBW2":
		return "RGBW Controller"
	case "SHBLB-1":
		return "Smart Bulb"
	case "SHEM", "SHEM-3":
		return "Energy Meter"
	case "SHUNI-1":
		return "Universal Module"
	case "SHHT-1":
		return "Humidity/Temperature"
	case "SHMOS-01":
		return "Motion Sensor"
	case "SHDW-1", "SHDW-2":
		return "Door/Window Sensor"
	case "SHWT-1":
		return "Flood Sensor"
	case "SHGS-1":
		return "Gas Sensor"
	case "SHSM-01":
		return "Smoke Detector"
	case "SHTRV-01":
		return "TRV Controller"
	}

	// Check for Gen2+ models (contain pattern like SNSN-, SPSW-, etc.)
	if strings.HasPrefix(model, "SNSN-") {
		return "Plus Sensor"
	}
	if strings.HasPrefix(model, "SPSW-") {
		return "Plus Switch"
	}
	if strings.HasPrefix(model, "SPSH-") {
		return "Plus Smart Home"
	}

	// Fallback to pattern matching (order matters - more specific patterns first)
	lowerModel := strings.ToLower(model)
	switch {
	case strings.Contains(lowerModel, "plug"):
		return "Smart Plug"
	case strings.Contains(lowerModel, "valve"):
		return "Valve Controller"
	case strings.Contains(lowerModel, "i3"):
		return "3-Input Controller"
	case strings.Contains(lowerModel, "uni"):
		return "Universal Module"
	case strings.Contains(lowerModel, "roller") || strings.Contains(lowerModel, "shutter"):
		return "Roller Shutter"
	case strings.Contains(lowerModel, "pm"):
		return "Power Meter Device"
	case strings.Contains(lowerModel, "dimmer"):
		return "Dimmer"
	case strings.Contains(lowerModel, "rgbw"):
		return "RGBW Light"
	case strings.Contains(lowerModel, "bulb"):
		return "Smart Bulb"
	case strings.Contains(lowerModel, "motion"):
		return "Motion Sensor"
	case strings.Contains(lowerModel, "ht"):
		return "Humidity/Temperature"
	case strings.Contains(lowerModel, "flood"):
		return "Flood Sensor"
	case strings.Contains(lowerModel, "door") || strings.Contains(lowerModel, "window"):
		return "Door/Window Sensor"
	case strings.Contains(lowerModel, "smoke"):
		return "Smoke Detector"
	case strings.Contains(lowerModel, "gas"):
		return "Gas Detector"
	case strings.Contains(lowerModel, "em"):
		return "Energy Meter"
	case strings.Contains(lowerModel, "button"):
		return "Button Controller"
	case strings.Contains(lowerModel, "plus"):
		return "Plus Device"
	case strings.Contains(lowerModel, "pro"):
		return "Pro Device"
	default:
		return "Shelly Device"
	}
}

package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ShellyDevice represents a discovered Shelly device
type ShellyDevice struct {
	ID         string `json:"id"`
	MAC        string `json:"mac"`
	Model      string `json:"model"`
	Generation int    `json:"gen"`
	FirmwareID string `json:"fw_id"`
	Version    string `json:"ver"`
	App        string `json:"app"`
	AuthEn     bool   `json:"auth_en"`
	AuthDomain string `json:"auth_domain"`
	IP         string `json:"-"`
	Discovered time.Time `json:"-"`
}

// Scanner handles device discovery operations
type Scanner struct {
	timeout         time.Duration
	concurrentScans int
	httpClient      *http.Client
}

// NewScanner creates a new discovery scanner
func NewScanner(timeout time.Duration, concurrentScans int) *Scanner {
	if concurrentScans <= 0 {
		concurrentScans = 10
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	return &Scanner{
		timeout:         timeout,
		concurrentScans: concurrentScans,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ScanNetwork performs HTTP-based discovery on the specified network range
func (s *Scanner) ScanNetwork(ctx context.Context, cidr string) ([]ShellyDevice, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	var devices []ShellyDevice
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create a channel for IPs to scan
	ipChan := make(chan string, 100)
	
	// Start worker goroutines
	for i := 0; i < s.concurrentScans; i++ {
		wg.Add(1)
		go func() {
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
					}
				}
			}
		}()
	}

	// Generate IPs to scan
	go func() {
		defer close(ipChan)
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			select {
			case <-ctx.Done():
				return
			case ipChan <- ip.String():
			}
		}
	}()

	wg.Wait()
	return devices, nil
}

// ScanHost checks a specific host for Shelly device
func (s *Scanner) ScanHost(ctx context.Context, host string) (*ShellyDevice, error) {
	return s.checkDevice(ctx, host), nil
}

// checkDevice attempts to identify a Shelly device at the given IP
func (s *Scanner) checkDevice(ctx context.Context, ip string) *ShellyDevice {
	url := fmt.Sprintf("http://%s/shelly", ip)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var device ShellyDevice
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return nil
	}

	// Validate it's a Shelly device
	if device.ID == "" || !strings.Contains(strings.ToLower(device.ID), "shelly") {
		return nil
	}

	device.IP = ip
	device.Discovered = time.Now()
	
	return &device
}

// GetDeviceStatus retrieves the current status of a Shelly device
func (s *Scanner) GetDeviceStatus(ctx context.Context, ip string, gen int) (map[string]interface{}, error) {
	var url string
	if gen == 1 {
		url = fmt.Sprintf("http://%s/status", ip)
	} else {
		// Gen2+ uses RPC-style endpoints
		url = fmt.Sprintf("http://%s/rpc/Shelly.GetStatus", ip)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

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
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "plug"):
		return "Smart Plug"
	case strings.Contains(model, "1pm") || strings.Contains(model, "2pm") || strings.Contains(model, "4pm"):
		return "Power Meter Switch"
	case strings.Contains(model, "dimmer"):
		return "Dimmer"
	case strings.Contains(model, "rgbw"):
		return "RGBW Light"
	case strings.Contains(model, "bulb"):
		return "Smart Bulb"
	case strings.Contains(model, "motion"):
		return "Motion Sensor"
	case strings.Contains(model, "ht") || strings.Contains(model, "humidity"):
		return "Temperature/Humidity Sensor"
	case strings.Contains(model, "flood"):
		return "Flood Sensor"
	case strings.Contains(model, "door") || strings.Contains(model, "window"):
		return "Door/Window Sensor"
	case strings.Contains(model, "smoke"):
		return "Smoke Detector"
	case strings.Contains(model, "gas"):
		return "Gas Detector"
	case strings.Contains(model, "em"):
		return "Energy Meter"
	case strings.Contains(model, "3em"):
		return "3-Phase Energy Meter"
	case strings.Contains(model, "roller") || strings.Contains(model, "shutter"):
		return "Roller Shutter"
	case strings.Contains(model, "valve"):
		return "Valve Controller"
	case strings.Contains(model, "i3"):
		return "Input Controller"
	case strings.Contains(model, "button"):
		return "Button Controller"
	case strings.Contains(model, "uni"):
		return "Universal Controller"
	default:
		if strings.HasPrefix(model, "shelly1") {
			return "Relay Switch"
		}
		if strings.HasPrefix(model, "shelly2") {
			return "Dual Relay Switch"
		}
		if strings.HasPrefix(model, "shellypro") {
			return "Pro Series Device"
		}
		return "Shelly Device"
	}
}
package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

// mdnsQuery sends one mDNS query. A test replaces it, so that it never sends on the network.
var mdnsQuery = mdns.Query

// MDNSScanner handles mDNS-based device discovery
type MDNSScanner struct {
	timeout time.Duration
	scanner *Scanner
}

// NewMDNSScanner creates a new mDNS scanner
func NewMDNSScanner(timeout time.Duration) *MDNSScanner {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &MDNSScanner{
		timeout: timeout,
		scanner: NewScanner(2*time.Second, 5),
	}
}

// DiscoverDevices discovers Shelly devices using mDNS
func (m *MDNSScanner) DiscoverDevices(ctx context.Context) ([]ShellyDevice, error) {
	var devices []ShellyDevice
	entriesCh := make(chan *mdns.ServiceEntry, 10)

	// Start the discovery
	go func() {
		defer close(entriesCh)

		// Look for Shelly-specific service
		params := mdns.DefaultParams("_shelly._tcp")
		params.Entries = entriesCh
		params.Timeout = m.timeout
		params.DisableIPv6 = true

		if err := mdnsQuery(params); err != nil {
			// Try generic HTTP service as fallback
			params.Service = "_http._tcp"
			if fallbackErr := mdnsQuery(params); fallbackErr != nil {
				// Log the error but continue - mDNS discovery is best effort
				m.scanner.logger.WithFields(map[string]any{
					"component": "discovery",
					"service":   params.Service,
					"error":     fallbackErr,
				}).Debug("mDNS fallback query failed")
			}
		}
	}()

	// Collect the entry pointers and read none of their fields yet. At the first record of a
	// service the client sends the entry, and at each later record it writes more fields into
	// that same entry, so a read here races with the client. The goroutine closes the channel
	// only after mdns.Query has returned, and the client writes nothing after that.
	var entries []*mdns.ServiceEntry
	for entry := range entriesCh {
		entries = append(entries, entry)
	}

	// Process discovered services
	seen := make(map[string]bool)
	for _, entry := range entries {
		// Check if it's a Shelly device
		if !m.isShellyDevice(entry) {
			continue
		}

		// Get the best IP address
		ip := m.getBestIP(entry)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true

		// Verify it's actually a Shelly device by querying the API
		device := m.scanner.checkDevice(ctx, ip)
		if device != nil {
			devices = append(devices, *device)
		}
	}

	return devices, nil
}

// isShellyDevice checks if an mDNS entry is likely a Shelly device
func (m *MDNSScanner) isShellyDevice(entry *mdns.ServiceEntry) bool {
	// Check service name
	if strings.Contains(strings.ToLower(entry.Name), "shelly") {
		return true
	}

	// Check hostname
	if strings.Contains(strings.ToLower(entry.Host), "shelly") {
		return true
	}

	// Check TXT records for Shelly-specific info
	for _, txt := range entry.InfoFields {
		if strings.Contains(strings.ToLower(txt), "shelly") ||
			strings.Contains(txt, "gen=") {
			return true
		}
	}

	return false
}

// getBestIP returns the best IP address from an mDNS entry
func (m *MDNSScanner) getBestIP(entry *mdns.ServiceEntry) string {
	// Prefer IPv4 addresses
	if entry.AddrV4 != nil {
		return entry.AddrV4.String()
	}

	// Fall back to IPv6 if available
	if entry.AddrV6 != nil {
		// Check if it's a link-local address and skip if so
		if !entry.AddrV6.IsLinkLocalUnicast() {
			return entry.AddrV6.String()
		}
	}

	// Try to resolve hostname if no direct IP
	if entry.Host != "" {
		if ips, err := net.LookupIP(entry.Host); err == nil {
			for _, ip := range ips {
				if ip4 := ip.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}

	return ""
}

// CombinedDiscovery performs both HTTP scanning and mDNS discovery
func CombinedDiscovery(ctx context.Context, networks []string, timeout time.Duration) ([]ShellyDevice, error) {
	var allDevices []ShellyDevice
	seen := make(map[string]bool)

	// HTTP scanning for specified networks
	if len(networks) > 0 {
		scanner := NewScanner(timeout, 50) // Increased concurrency for faster scanning
		for _, network := range networks {
			devices, err := scanner.ScanNetwork(ctx, network)
			if err != nil {
				fmt.Printf("Error scanning network %s: %v\n", network, err)
				continue
			}

			for _, device := range devices {
				if !seen[device.MAC] {
					allDevices = append(allDevices, device)
					seen[device.MAC] = true
				}
			}
		}
	}

	// mDNS discovery
	mdnsScanner := NewMDNSScanner(timeout)
	mdnsDevices, err := mdnsScanner.DiscoverDevices(ctx)
	if err != nil {
		fmt.Printf("mDNS discovery error: %v\n", err)
	} else {
		for _, device := range mdnsDevices {
			if !seen[device.MAC] {
				allDevices = append(allDevices, device)
				seen[device.MAC] = true
			}
		}
	}

	return allDevices, nil
}

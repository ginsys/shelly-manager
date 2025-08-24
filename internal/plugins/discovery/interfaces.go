package discovery

import (
	"context"
	"net"
	"time"

	"github.com/ginsys/shelly-manager/internal/plugins"
)

// DiscoveryPlugin extends the base Plugin interface for device discovery operations
type DiscoveryPlugin interface {
	plugins.Plugin

	// Discover discovers devices on the network
	Discover(ctx context.Context, config DiscoveryConfig) (*DiscoveryResult, error)

	// Scan performs a targeted scan of specific addresses/ranges
	Scan(ctx context.Context, targets []string, config DiscoveryConfig) (*DiscoveryResult, error)

	// Monitor starts continuous monitoring for new devices
	Monitor(ctx context.Context, config DiscoveryConfig, callback func(Device)) error

	// StopMonitoring stops continuous monitoring
	StopMonitoring() error

	// Validate validates if a device is of the expected type
	Validate(ctx context.Context, device Device) (*ValidationResult, error)

	// GetSupportedProtocols returns the discovery protocols this plugin supports
	GetSupportedProtocols() []DiscoveryProtocol

	// Discovery-specific capabilities
	Capabilities() DiscoveryCapabilities
}

// DiscoveryProtocol defines the type of discovery protocol
type DiscoveryProtocol string

const (
	ProtocolMDNS      DiscoveryProtocol = "mdns"
	ProtocolSSDP      DiscoveryProtocol = "ssdp"
	ProtocolUPnP      DiscoveryProtocol = "upnp"
	ProtocolDHCP      DiscoveryProtocol = "dhcp"
	ProtocolARP       DiscoveryProtocol = "arp"
	ProtocolNMAP      DiscoveryProtocol = "nmap"
	ProtocolSNMP      DiscoveryProtocol = "snmp"
	ProtocolBluetooth DiscoveryProtocol = "bluetooth"
	ProtocolZigbee    DiscoveryProtocol = "zigbee"
	ProtocolZwave     DiscoveryProtocol = "zwave"
	ProtocolMatter    DiscoveryProtocol = "matter"
	ProtocolHomeKit   DiscoveryProtocol = "homekit"
	ProtocolHTTP      DiscoveryProtocol = "http"
	ProtocolTelnet    DiscoveryProtocol = "telnet"
	ProtocolSSH       DiscoveryProtocol = "ssh"
	ProtocolCustom    DiscoveryProtocol = "custom"
)

// DiscoveryConfig holds configuration for discovery operations
type DiscoveryConfig struct {
	Protocols         []DiscoveryProtocol    `json:"protocols"`
	NetworkInterfaces []string               `json:"network_interfaces,omitempty"`
	IPRanges          []string               `json:"ip_ranges,omitempty"`
	Ports             []int                  `json:"ports,omitempty"`
	Timeout           time.Duration          `json:"timeout"`
	MaxConcurrency    int                    `json:"max_concurrency"`
	Filters           DiscoveryFilters       `json:"filters"`
	Options           map[string]interface{} `json:"options,omitempty"`
	RetryCount        int                    `json:"retry_count"`
	RetryDelay        time.Duration          `json:"retry_delay"`
}

// DiscoveryFilters defines filters for discovery operations
type DiscoveryFilters struct {
	DeviceTypes     []string `json:"device_types,omitempty"`
	Manufacturers   []string `json:"manufacturers,omitempty"`
	ModelNumbers    []string `json:"model_numbers,omitempty"`
	ServiceTypes    []string `json:"service_types,omitempty"`
	MinRSSI         *int     `json:"min_rssi,omitempty"`
	RequiresAuth    *bool    `json:"requires_auth,omitempty"`
	IsOnline        *bool    `json:"is_online,omitempty"`
	HasWebInterface *bool    `json:"has_web_interface,omitempty"`
	ExcludeKnown    bool     `json:"exclude_known"`
	IncludeInactive bool     `json:"include_inactive"`
}

// Device represents a discovered device
type Device struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name,omitempty"`
	Type          string                 `json:"type"`
	Protocol      DiscoveryProtocol      `json:"protocol"`
	IPAddress     net.IP                 `json:"ip_address,omitempty"`
	MACAddress    string                 `json:"mac_address,omitempty"`
	Port          int                    `json:"port,omitempty"`
	Manufacturer  string                 `json:"manufacturer,omitempty"`
	Model         string                 `json:"model,omitempty"`
	Version       string                 `json:"version,omitempty"`
	SerialNumber  string                 `json:"serial_number,omitempty"`
	Services      []DeviceService        `json:"services,omitempty"`
	Capabilities  []string               `json:"capabilities,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	LastSeen      time.Time              `json:"last_seen"`
	FirstSeen     time.Time              `json:"first_seen"`
	IsOnline      bool                   `json:"is_online"`
	RSSI          *int                   `json:"rssi,omitempty"`          // For wireless devices
	BatteryLevel  *int                   `json:"battery_level,omitempty"` // 0-100 percentage
	Hostname      string                 `json:"hostname,omitempty"`
	WebInterface  string                 `json:"web_interface,omitempty"` // URL to web interface
	RequiresAuth  bool                   `json:"requires_auth"`
	SecurityLevel string                 `json:"security_level,omitempty"` // none, wep, wpa, wpa2, etc.
	Location      *DeviceLocation        `json:"location,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	DiscoveredBy  string                 `json:"discovered_by"` // Plugin name
}

// DeviceService represents a service offered by a device
type DeviceService struct {
	Type        string            `json:"type"`
	Port        int               `json:"port"`
	Protocol    string            `json:"protocol"` // tcp, udp, etc.
	Name        string            `json:"name,omitempty"`
	Version     string            `json:"version,omitempty"`
	Path        string            `json:"path,omitempty"`        // URL path for HTTP services
	TxtRecords  map[string]string `json:"txt_records,omitempty"` // For mDNS services
	Description string            `json:"description,omitempty"`
	IsSecure    bool              `json:"is_secure"`
}

// DeviceLocation represents the physical location of a device
type DeviceLocation struct {
	Room      string   `json:"room,omitempty"`
	Floor     string   `json:"floor,omitempty"`
	Building  string   `json:"building,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Address   string   `json:"address,omitempty"`
}

// DiscoveryResult contains the result of a discovery operation
type DiscoveryResult struct {
	Success         bool                   `json:"success"`
	DevicesFound    []Device               `json:"devices_found"`
	NewDevices      []Device               `json:"new_devices"`     // Devices not seen before
	UpdatedDevices  []Device               `json:"updated_devices"` // Devices with changes
	OfflineDevices  []Device               `json:"offline_devices"` // Devices that went offline
	TotalScanned    int                    `json:"total_scanned"`   // Total addresses/ranges scanned
	Duration        time.Duration          `json:"duration"`
	Errors          []string               `json:"errors,omitempty"`
	Warnings        []string               `json:"warnings,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	NetworkInfo     *NetworkInfo           `json:"network_info,omitempty"`
	CompletedAt     time.Time              `json:"completed_at"`
	ProtocolResults map[string]int         `json:"protocol_results"` // protocol -> device count
}

// NetworkInfo contains information about the network being scanned
type NetworkInfo struct {
	Interface     string    `json:"interface"`
	IPRange       string    `json:"ip_range"`
	Gateway       string    `json:"gateway,omitempty"`
	DNSServers    []string  `json:"dns_servers,omitempty"`
	NetworkName   string    `json:"network_name,omitempty"`   // WiFi SSID or network name
	NetworkType   string    `json:"network_type,omitempty"`   // wifi, ethernet, etc.
	SignalQuality *int      `json:"signal_quality,omitempty"` // 0-100 for WiFi
	LastScanAt    time.Time `json:"last_scan_at"`
}

// ValidationResult contains the result of device validation
type ValidationResult struct {
	IsValid      bool                   `json:"is_valid"`
	DeviceType   string                 `json:"device_type,omitempty"`
	Manufacturer string                 `json:"manufacturer,omitempty"`
	Model        string                 `json:"model,omitempty"`
	Version      string                 `json:"version,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Services     []DeviceService        `json:"services,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Errors       []string               `json:"errors,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	ValidatedAt  time.Time              `json:"validated_at"`
}

// DiscoveryCapabilities describes what a discovery plugin can do
type DiscoveryCapabilities struct {
	SupportedProtocols    []DiscoveryProtocol `json:"supported_protocols"`
	SupportsMonitoring    bool                `json:"supports_monitoring"`
	SupportsValidation    bool                `json:"supports_validation"`
	SupportsFiltering     bool                `json:"supports_filtering"`
	MaxConcurrentScans    int                 `json:"max_concurrent_scans"`
	RequiresRootAccess    bool                `json:"requires_root_access"`
	RequiresNetworkAccess bool                `json:"requires_network_access"`
	SupportedPlatforms    []string            `json:"supported_platforms"` // linux, windows, darwin
	MinScanInterval       time.Duration       `json:"min_scan_interval"`
	MaxDevicesPerScan     int                 `json:"max_devices_per_scan"`
	SupportsDeviceControl bool                `json:"supports_device_control"` // Can control discovered devices
	SupportsConfiguration bool                `json:"supports_configuration"`  // Can configure discovered devices
	SupportsBatteryInfo   bool                `json:"supports_battery_info"`   // Can read battery levels
	SupportsLocationInfo  bool                `json:"supports_location_info"`  // Can determine device location
}

// MonitoringCallback is the function signature for monitoring callbacks
type MonitoringCallback func(device Device, event MonitoringEvent)

// MonitoringEvent represents an event during device monitoring
type MonitoringEvent struct {
	Type      MonitoringEventType    `json:"type"`
	Device    Device                 `json:"device"`
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MonitoringEventType defines the type of monitoring event
type MonitoringEventType string

const (
	EventDeviceFound    MonitoringEventType = "device_found"
	EventDeviceUpdated  MonitoringEventType = "device_updated"
	EventDeviceOffline  MonitoringEventType = "device_offline"
	EventDeviceOnline   MonitoringEventType = "device_online"
	EventDeviceRemoved  MonitoringEventType = "device_removed"
	EventNetworkChanged MonitoringEventType = "network_changed"
	EventScanComplete   MonitoringEventType = "scan_complete"
	EventError          MonitoringEventType = "error"
)

// Example discovery plugins that could be implemented:

// MDNSDiscoveryPlugin would implement DiscoveryPlugin for mDNS/Bonjour discovery
// SSDPDiscoveryPlugin would implement DiscoveryPlugin for UPnP/SSDP discovery
// NetworkScanPlugin would implement DiscoveryPlugin for network scanning (nmap-style)
// ShellyDiscoveryPlugin would implement DiscoveryPlugin specifically for Shelly devices
// BluetoothDiscoveryPlugin would implement DiscoveryPlugin for Bluetooth devices
// ZigbeeDiscoveryPlugin would implement DiscoveryPlugin for Zigbee devices
// MatterDiscoveryPlugin would implement DiscoveryPlugin for Matter/Thread devices

// Future features to consider:
// - Device fingerprinting and identification
// - Automatic device categorization using AI/ML
// - Integration with device databases (MAC OUI lookups, etc.)
// - Network topology mapping and visualization
// - Device relationship detection (gateway, hub, endpoint)
// - Security scanning and vulnerability detection
// - Device configuration backup and restore
// - Firmware update detection and management
// - Device performance monitoring and alerting
// - Integration with IPAM (IP Address Management) systems

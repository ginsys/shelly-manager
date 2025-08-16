package configuration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// TypedConfiguration represents a strongly-typed device configuration
type TypedConfiguration struct {
	WiFi     *WiFiConfiguration     `json:"wifi,omitempty"`
	MQTT     *MQTTConfiguration     `json:"mqtt,omitempty"`
	Auth     *AuthConfiguration     `json:"auth,omitempty"`
	System   *SystemConfiguration   `json:"system,omitempty"`
	Network  *NetworkConfiguration  `json:"network,omitempty"`
	Cloud    *CloudConfiguration    `json:"cloud,omitempty"`
	Location *LocationConfiguration `json:"location,omitempty"`
	Raw      json.RawMessage        `json:"raw,omitempty"` // For unsupported settings
}

// WiFiConfiguration represents WiFi settings
type WiFiConfiguration struct {
	Enable        bool               `json:"enable" validate:"required"`
	SSID          string             `json:"ssid" validate:"required,min=1,max=32"`
	Password      string             `json:"password,omitempty" validate:"max=63"`
	IPv4Mode      string             `json:"ipv4mode,omitempty" validate:"oneof=dhcp static"`
	StaticIP      *StaticIPConfig    `json:"static_ip,omitempty"`
	AccessPoint   *AccessPointConfig `json:"ap,omitempty"`
	RoamThreshold int                `json:"roam_threshold,omitempty" validate:"min=-100,max=-50"`
}

// StaticIPConfig represents static IP configuration
type StaticIPConfig struct {
	IP         string `json:"ip" validate:"required,ip"`
	Netmask    string `json:"netmask" validate:"required,ip"`
	Gateway    string `json:"gw" validate:"required,ip"`
	Nameserver string `json:"nameserver,omitempty" validate:"omitempty,ip"`
}

// AccessPointConfig represents access point configuration
type AccessPointConfig struct {
	Enable   bool   `json:"enable"`
	SSID     string `json:"ssid,omitempty" validate:"max=32"`
	Password string `json:"pass,omitempty" validate:"min=8,max=63"`
	Key      string `json:"key,omitempty" validate:"oneof=open wpa_psk wpa2_psk"`
}

// MQTTConfiguration represents MQTT settings
type MQTTConfiguration struct {
	Enable             bool   `json:"enable"`
	Server             string `json:"server,omitempty" validate:"omitempty,hostname_port|ip"`
	Port               int    `json:"port,omitempty" validate:"omitempty,min=1,max=65535"`
	User               string `json:"user,omitempty"`
	Password           string `json:"pass,omitempty"`
	ClientID           string `json:"id,omitempty" validate:"max=128"`
	CleanSession       bool   `json:"clean_session"`
	KeepAlive          int    `json:"keep_alive,omitempty" validate:"omitempty,min=1,max=3600"`
	MaxQueuedMessages  int    `json:"max_qos_msgs,omitempty" validate:"omitempty,min=1,max=100"`
	RetainPeriod       int    `json:"retain,omitempty" validate:"omitempty,min=0"`
	UpdatePeriod       int    `json:"update_period,omitempty" validate:"omitempty,min=1,max=3600"`
	TopicPrefix        string `json:"topic_prefix,omitempty" validate:"max=256"`
	RPCNotifications   bool   `json:"rpc_ntf"`
	StatusNotification bool   `json:"status_ntf"`
	UseClientCert      bool   `json:"ssl_ca,omitempty"`
	EnableSNI          bool   `json:"enable_sni,omitempty"`
}

// AuthConfiguration represents authentication settings
type AuthConfiguration struct {
	Enable   bool   `json:"enable"`
	Username string `json:"user,omitempty" validate:"omitempty,min=1,max=64"`
	Password string `json:"pass,omitempty" validate:"omitempty,min=1"`
	Realm    string `json:"realm,omitempty" validate:"max=64"`
}

// SystemConfiguration represents system settings
type SystemConfiguration struct {
	Device       *TypedDeviceConfig `json:"device,omitempty"`
	Location     *LocationConfig    `json:"location,omitempty"`
	Debug        *DebugConfig       `json:"debug,omitempty"`
	UIData       *UIConfig          `json:"ui_data,omitempty"`
	RPC          *RPCConfig         `json:"rpc_udp,omitempty"`
	SNTP         *SNTPConfig        `json:"sntp,omitempty"`
	Sleep        *SleepConfig       `json:"sleep_mode,omitempty"`
	WakeupPeriod int                `json:"wakeup_period,omitempty" validate:"omitempty,min=1"`
}

// TypedDeviceConfig represents device-specific settings
type TypedDeviceConfig struct {
	Name         string     `json:"name,omitempty" validate:"max=64"`
	MAC          string     `json:"mac,omitempty"`
	Hostname     string     `json:"hostname,omitempty" validate:"max=63,hostname"`
	EcoMode      bool       `json:"eco_mode,omitempty"`
	Profile      string     `json:"profile,omitempty"`
	Discoverable bool       `json:"discoverable,omitempty"`
	AddonType    string     `json:"addon_type,omitempty"`
	FWAutoUpdate bool       `json:"fw_auto_update,omitempty"`
	Timezone     string     `json:"tz,omitempty" validate:"max=64"`
	LatLon       []float64  `json:"lat_lon,omitempty" validate:"len=0|len=2"`
	BleConfig    *BLEConfig `json:"ble,omitempty"`
}

// LocationConfig represents location settings
type LocationConfig struct {
	Timezone  string  `json:"tz,omitempty" validate:"max=64"`
	Latitude  float64 `json:"lat,omitempty" validate:"min=-90,max=90"`
	Longitude float64 `json:"lng,omitempty" validate:"min=-180,max=180"`
}

// NetworkConfiguration represents network settings
type NetworkConfiguration struct {
	Ethernet *EthernetConfig  `json:"eth,omitempty"`
	WiFi     *TypedWiFiConfig `json:"wifi,omitempty"`
}

// EthernetConfig represents ethernet settings
type EthernetConfig struct {
	Enable   bool            `json:"enable"`
	IPv4Mode string          `json:"ipv4mode,omitempty" validate:"oneof=dhcp static"`
	StaticIP *StaticIPConfig `json:"ip,omitempty"`
}

// TypedWiFiConfig represents WiFi network settings (different from WiFiConfiguration)
type TypedWiFiConfig struct {
	STA *WiFiSTAConfig `json:"sta,omitempty"`
	AP  *WiFiAPConfig  `json:"ap,omitempty"`
}

// WiFiSTAConfig represents WiFi station settings
type WiFiSTAConfig struct {
	Enable     bool            `json:"enable"`
	SSID       string          `json:"ssid,omitempty" validate:"max=32"`
	Password   string          `json:"pass,omitempty" validate:"max=63"`
	IsOpen     bool            `json:"is_open,omitempty"`
	IPv4Mode   string          `json:"ipv4mode,omitempty" validate:"oneof=dhcp static"`
	StaticIP   *StaticIPConfig `json:"ip,omitempty"`
	Nameserver string          `json:"nameserver,omitempty" validate:"omitempty,ip"`
}

// WiFiAPConfig represents WiFi access point settings
type WiFiAPConfig struct {
	Enable     bool   `json:"enable"`
	SSID       string `json:"ssid,omitempty" validate:"max=32"`
	Password   string `json:"pass,omitempty" validate:"min=8,max=63"`
	IsOpen     bool   `json:"is_open,omitempty"`
	MaxClients int    `json:"max_sta,omitempty" validate:"min=1,max=10"`
	RangeStart string `json:"range_start,omitempty" validate:"omitempty,ip"`
	RangeEnd   string `json:"range_end,omitempty" validate:"omitempty,ip"`
}

// CloudConfiguration represents cloud settings
type CloudConfiguration struct {
	Enable bool   `json:"enable"`
	Server string `json:"server,omitempty" validate:"omitempty,url"`
}

// LocationConfiguration represents location and time settings
type LocationConfiguration struct {
	Timezone  string  `json:"tz,omitempty" validate:"max=64"`
	Latitude  float64 `json:"lat,omitempty" validate:"min=-90,max=90"`
	Longitude float64 `json:"lng,omitempty" validate:"min=-180,max=180"`
}

// Additional configuration structures for completeness

// DebugConfig represents debug settings
type DebugConfig struct {
	Level      int  `json:"level,omitempty" validate:"min=0,max=4"`
	FileLevel  int  `json:"file_level,omitempty" validate:"min=0,max=4"`
	MQTTOutput bool `json:"mqtt,omitempty"`
	WSOutput   bool `json:"websocket,omitempty"`
	UDPOutput  bool `json:"udp,omitempty"`
}

// UIConfig represents UI settings
type UIConfig struct {
	SleepMode bool `json:"sleep_mode,omitempty"`
}

// RPCConfig represents RPC settings
type RPCConfig struct {
	DstAddr    string `json:"dst_addr,omitempty" validate:"omitempty,ip"`
	ListenPort int    `json:"listen_port,omitempty" validate:"omitempty,min=1,max=65535"`
}

// SNTPConfig represents SNTP settings
type SNTPConfig struct {
	Server string `json:"server,omitempty" validate:"omitempty,hostname"`
}

// SleepConfig represents sleep mode settings
type SleepConfig struct {
	Enable bool   `json:"enable"`
	Period int    `json:"period,omitempty" validate:"omitempty,min=1"`
	Unit   string `json:"unit,omitempty" validate:"omitempty,oneof=s m h"`
}

// BLEConfig represents Bluetooth LE settings
type BLEConfig struct {
	Enable bool       `json:"enable"`
	RPC    *RPCConfig `json:"rpc,omitempty"`
}

// Validation methods

// Validate validates the entire typed configuration
func (tc *TypedConfiguration) Validate() error {
	if tc.WiFi != nil {
		if err := tc.WiFi.Validate(); err != nil {
			return fmt.Errorf("wifi validation failed: %w", err)
		}
	}

	if tc.MQTT != nil {
		if err := tc.MQTT.Validate(); err != nil {
			return fmt.Errorf("mqtt validation failed: %w", err)
		}
	}

	if tc.Auth != nil {
		if err := tc.Auth.Validate(); err != nil {
			return fmt.Errorf("auth validation failed: %w", err)
		}
	}

	if tc.System != nil {
		if err := tc.System.Validate(); err != nil {
			return fmt.Errorf("system validation failed: %w", err)
		}
	}

	if tc.Network != nil {
		if err := tc.Network.Validate(); err != nil {
			return fmt.Errorf("network validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates WiFi configuration
func (w *WiFiConfiguration) Validate() error {
	if w.Enable && w.SSID == "" {
		return fmt.Errorf("SSID is required when WiFi is enabled")
	}

	if len(w.SSID) > 32 {
		return fmt.Errorf("SSID must be 32 characters or less")
	}

	if len(w.Password) > 63 {
		return fmt.Errorf("WiFi password must be 63 characters or less")
	}

	if w.IPv4Mode != "" && w.IPv4Mode != "dhcp" && w.IPv4Mode != "static" {
		return fmt.Errorf("IPv4 mode must be 'dhcp' or 'static'")
	}

	if w.IPv4Mode == "static" && w.StaticIP == nil {
		return fmt.Errorf("static IP configuration required when IPv4 mode is 'static'")
	}

	if w.StaticIP != nil {
		if err := w.StaticIP.Validate(); err != nil {
			return fmt.Errorf("static IP validation failed: %w", err)
		}
	}

	if w.RoamThreshold != 0 && (w.RoamThreshold < -100 || w.RoamThreshold > -50) {
		return fmt.Errorf("roam threshold must be between -100 and -50 dBm")
	}

	return nil
}

// Validate validates static IP configuration
func (s *StaticIPConfig) Validate() error {
	if net.ParseIP(s.IP) == nil {
		return fmt.Errorf("invalid IP address: %s", s.IP)
	}

	if net.ParseIP(s.Netmask) == nil {
		return fmt.Errorf("invalid netmask: %s", s.Netmask)
	}

	if net.ParseIP(s.Gateway) == nil {
		return fmt.Errorf("invalid gateway: %s", s.Gateway)
	}

	if s.Nameserver != "" && net.ParseIP(s.Nameserver) == nil {
		return fmt.Errorf("invalid nameserver: %s", s.Nameserver)
	}

	return nil
}

// Validate validates MQTT configuration
func (m *MQTTConfiguration) Validate() error {
	if !m.Enable {
		return nil // Skip validation if MQTT is disabled
	}

	if m.Server == "" {
		return fmt.Errorf("MQTT server is required when MQTT is enabled")
	}

	// Validate server format (hostname:port, IP:port, or just hostname/IP)
	if !isValidHostnamePort(m.Server) && !isValidIPPort(m.Server) && !isValidHostname(m.Server) && net.ParseIP(m.Server) == nil {
		return fmt.Errorf("invalid MQTT server format: %s", m.Server)
	}

	if m.Port != 0 && (m.Port < 1 || m.Port > 65535) {
		return fmt.Errorf("MQTT port must be between 1 and 65535")
	}

	if len(m.ClientID) > 128 {
		return fmt.Errorf("MQTT client ID must be 128 characters or less")
	}

	if m.KeepAlive != 0 && (m.KeepAlive < 1 || m.KeepAlive > 3600) {
		return fmt.Errorf("MQTT keep alive must be between 1 and 3600 seconds")
	}

	if m.MaxQueuedMessages != 0 && (m.MaxQueuedMessages < 1 || m.MaxQueuedMessages > 100) {
		return fmt.Errorf("MQTT max queued messages must be between 1 and 100")
	}

	if len(m.TopicPrefix) > 256 {
		return fmt.Errorf("MQTT topic prefix must be 256 characters or less")
	}

	return nil
}

// Validate validates authentication configuration
func (a *AuthConfiguration) Validate() error {
	if !a.Enable {
		return nil // Skip validation if auth is disabled
	}

	if a.Username == "" {
		return fmt.Errorf("username is required when authentication is enabled")
	}

	if len(a.Username) > 64 {
		return fmt.Errorf("username must be 64 characters or less")
	}

	if a.Password == "" {
		return fmt.Errorf("password is required when authentication is enabled")
	}

	if len(a.Realm) > 64 {
		return fmt.Errorf("realm must be 64 characters or less")
	}

	return nil
}

// Validate validates system configuration
func (s *SystemConfiguration) Validate() error {
	if s.Device != nil {
		if err := s.Device.Validate(); err != nil {
			return fmt.Errorf("device config validation failed: %w", err)
		}
	}

	if s.Location != nil {
		if err := s.Location.Validate(); err != nil {
			return fmt.Errorf("location config validation failed: %w", err)
		}
	}

	if s.WakeupPeriod != 0 && s.WakeupPeriod < 1 {
		return fmt.Errorf("wakeup period must be positive")
	}

	return nil
}

// Validate validates device configuration
func (d *TypedDeviceConfig) Validate() error {
	if len(d.Name) > 64 {
		return fmt.Errorf("device name must be 64 characters or less")
	}

	if d.Hostname != "" && !isValidHostname(d.Hostname) {
		return fmt.Errorf("invalid hostname: %s", d.Hostname)
	}

	if len(d.Hostname) > 63 {
		return fmt.Errorf("hostname must be 63 characters or less")
	}

	if len(d.Timezone) > 64 {
		return fmt.Errorf("timezone must be 64 characters or less")
	}

	if len(d.LatLon) != 0 && len(d.LatLon) != 2 {
		return fmt.Errorf("lat_lon must be empty or contain exactly 2 values")
	}

	if len(d.LatLon) == 2 {
		lat, lng := d.LatLon[0], d.LatLon[1]
		if lat < -90 || lat > 90 {
			return fmt.Errorf("latitude must be between -90 and 90")
		}
		if lng < -180 || lng > 180 {
			return fmt.Errorf("longitude must be between -180 and 180")
		}
	}

	return nil
}

// Validate validates location configuration
func (l *LocationConfig) Validate() error {
	if len(l.Timezone) > 64 {
		return fmt.Errorf("timezone must be 64 characters or less")
	}

	if l.Latitude < -90 || l.Latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}

	if l.Longitude < -180 || l.Longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}

	return nil
}

// Validate validates network configuration
func (n *NetworkConfiguration) Validate() error {
	if n.WiFi != nil && n.WiFi.STA != nil {
		if err := n.WiFi.STA.Validate(); err != nil {
			return fmt.Errorf("WiFi STA validation failed: %w", err)
		}
	}

	if n.WiFi != nil && n.WiFi.AP != nil {
		if err := n.WiFi.AP.Validate(); err != nil {
			return fmt.Errorf("WiFi AP validation failed: %w", err)
		}
	}

	if n.Ethernet != nil {
		if err := n.Ethernet.Validate(); err != nil {
			return fmt.Errorf("Ethernet validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates WiFi STA configuration
func (w *WiFiSTAConfig) Validate() error {
	if w.Enable && w.SSID == "" {
		return fmt.Errorf("SSID is required when WiFi STA is enabled")
	}

	if len(w.SSID) > 32 {
		return fmt.Errorf("SSID must be 32 characters or less")
	}

	if len(w.Password) > 63 {
		return fmt.Errorf("WiFi password must be 63 characters or less")
	}

	if w.IPv4Mode != "" && w.IPv4Mode != "dhcp" && w.IPv4Mode != "static" {
		return fmt.Errorf("IPv4 mode must be 'dhcp' or 'static'")
	}

	if w.IPv4Mode == "static" && w.StaticIP == nil {
		return fmt.Errorf("static IP configuration required when IPv4 mode is 'static'")
	}

	if w.StaticIP != nil {
		if err := w.StaticIP.Validate(); err != nil {
			return fmt.Errorf("static IP validation failed: %w", err)
		}
	}

	if w.Nameserver != "" && net.ParseIP(w.Nameserver) == nil {
		return fmt.Errorf("invalid nameserver: %s", w.Nameserver)
	}

	return nil
}

// Validate validates WiFi AP configuration
func (w *WiFiAPConfig) Validate() error {
	if len(w.SSID) > 32 {
		return fmt.Errorf("AP SSID must be 32 characters or less")
	}

	if !w.IsOpen && len(w.Password) < 8 {
		return fmt.Errorf("AP password must be at least 8 characters when not open")
	}

	if len(w.Password) > 63 {
		return fmt.Errorf("AP password must be 63 characters or less")
	}

	if w.MaxClients != 0 && (w.MaxClients < 1 || w.MaxClients > 10) {
		return fmt.Errorf("max clients must be between 1 and 10")
	}

	if w.RangeStart != "" && net.ParseIP(w.RangeStart) == nil {
		return fmt.Errorf("invalid range start IP: %s", w.RangeStart)
	}

	if w.RangeEnd != "" && net.ParseIP(w.RangeEnd) == nil {
		return fmt.Errorf("invalid range end IP: %s", w.RangeEnd)
	}

	return nil
}

// Validate validates Ethernet configuration
func (e *EthernetConfig) Validate() error {
	if e.IPv4Mode != "" && e.IPv4Mode != "dhcp" && e.IPv4Mode != "static" {
		return fmt.Errorf("IPv4 mode must be 'dhcp' or 'static'")
	}

	if e.IPv4Mode == "static" && e.StaticIP == nil {
		return fmt.Errorf("static IP configuration required when IPv4 mode is 'static'")
	}

	if e.StaticIP != nil {
		if err := e.StaticIP.Validate(); err != nil {
			return fmt.Errorf("static IP validation failed: %w", err)
		}
	}

	return nil
}

// Conversion methods

// ToJSON converts typed configuration to JSON
func (tc *TypedConfiguration) ToJSON() (json.RawMessage, error) {
	data, err := json.Marshal(tc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal typed configuration: %w", err)
	}
	return json.RawMessage(data), nil
}

// FromJSON creates typed configuration from JSON
func FromJSON(data json.RawMessage) (*TypedConfiguration, error) {
	var tc TypedConfiguration
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal typed configuration: %w", err)
	}
	return &tc, nil
}

// Helper functions

// isValidHostnamePort validates hostname:port format
func isValidHostnamePort(hostPort string) bool {
	parts := strings.Split(hostPort, ":")
	if len(parts) != 2 {
		return false
	}

	// Validate hostname
	if !isValidHostname(parts[0]) {
		return false
	}

	// Validate port
	port, err := strconv.Atoi(parts[1])
	if err != nil || port < 1 || port > 65535 {
		return false
	}

	return true
}

// isValidIPPort validates IP:port format
func isValidIPPort(ipPort string) bool {
	parts := strings.Split(ipPort, ":")
	if len(parts) != 2 {
		return false
	}

	// Validate IP
	if net.ParseIP(parts[0]) == nil {
		return false
	}

	// Validate port
	port, err := strconv.Atoi(parts[1])
	if err != nil || port < 1 || port > 65535 {
		return false
	}

	return true
}

// isValidHostname validates hostname format
func isValidHostname(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 253 {
		return false
	}

	// Reject IP addresses - hostnames should not be IPs
	if net.ParseIP(hostname) != nil {
		return false
	}

	// Check for valid hostname format
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return hostnameRegex.MatchString(hostname)
}

// GetConfigurationSchema returns a JSON schema for validation
func GetConfigurationSchema() map[string]interface{} {
	return map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"title":   "Shelly Device Configuration",
		"properties": map[string]interface{}{
			"wifi": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"enable":   map[string]interface{}{"type": "boolean"},
					"ssid":     map[string]interface{}{"type": "string", "maxLength": 32},
					"password": map[string]interface{}{"type": "string", "maxLength": 63},
					"ipv4mode": map[string]interface{}{"type": "string", "enum": []string{"dhcp", "static"}},
				},
				"required": []string{"enable"},
				"if": map[string]interface{}{
					"properties": map[string]interface{}{"enable": map[string]interface{}{"const": true}},
				},
				"then": map[string]interface{}{
					"required": []string{"ssid"},
				},
			},
			"mqtt": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"enable": map[string]interface{}{"type": "boolean"},
					"server": map[string]interface{}{"type": "string"},
					"port":   map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 65535},
				},
			},
			"auth": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"enable": map[string]interface{}{"type": "boolean"},
					"user":   map[string]interface{}{"type": "string", "maxLength": 64},
					"pass":   map[string]interface{}{"type": "string"},
					"realm":  map[string]interface{}{"type": "string", "maxLength": 64},
				},
			},
		},
	}
}

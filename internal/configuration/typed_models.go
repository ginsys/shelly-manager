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

	// Device capability-specific configurations
	Relay          *RelayConfig          `json:"relay,omitempty"`
	PowerMetering  *PowerMeteringConfig  `json:"power_metering,omitempty"`
	Dimming        *DimmingConfig        `json:"dimming,omitempty"`
	Roller         *RollerConfig         `json:"roller,omitempty"`
	Input          *InputConfig          `json:"input,omitempty"`
	LED            *LEDConfig            `json:"led,omitempty"`
	Color          *ColorConfig          `json:"color,omitempty"`
	TempProtection *TempProtectionConfig `json:"temp_protection,omitempty"`
	Schedule       *ScheduleConfig       `json:"schedule,omitempty"`
	CoIoT          *CoIoTConfig          `json:"coiot,omitempty"`
	EnergyMeter    *EnergyMeterConfig    `json:"energy_meter,omitempty"`
	Motion         *MotionConfig         `json:"motion,omitempty"`
	Sensor         *SensorConfig         `json:"sensor,omitempty"`

	Raw json.RawMessage `json:"raw,omitempty"` // For unsupported settings
}

// WiFiConfiguration represents WiFi settings
type WiFiConfiguration struct {
	Enable        *bool              `json:"enable,omitempty"`
	SSID          *string            `json:"ssid,omitempty"`
	Password      *string            `json:"password,omitempty"`
	IPv4Mode      *string            `json:"ipv4mode,omitempty"`
	StaticIP      *StaticIPConfig    `json:"static_ip,omitempty"`
	AccessPoint   *AccessPointConfig `json:"ap,omitempty"`
	RoamThreshold *int               `json:"roam_threshold,omitempty"`
}

// StaticIPConfig represents static IP configuration
type StaticIPConfig struct {
	IP         *string `json:"ip,omitempty"`
	Netmask    *string `json:"netmask,omitempty"`
	Gateway    *string `json:"gw,omitempty"`
	Nameserver *string `json:"nameserver,omitempty"`
}

// AccessPointConfig represents access point configuration
type AccessPointConfig struct {
	Enable   *bool   `json:"enable,omitempty"`
	SSID     *string `json:"ssid,omitempty"`
	Password *string `json:"pass,omitempty"`
	Key      *string `json:"key,omitempty"`
}

// MQTTConfiguration represents MQTT settings
type MQTTConfiguration struct {
	Enable             *bool   `json:"enable,omitempty"`
	Server             *string `json:"server,omitempty"`
	Port               *int    `json:"port,omitempty"`
	User               *string `json:"user,omitempty"`
	Password           *string `json:"pass,omitempty"`
	ClientID           *string `json:"id,omitempty"`
	CleanSession       *bool   `json:"clean_session,omitempty"`
	KeepAlive          *int    `json:"keep_alive,omitempty"`
	MaxQueuedMessages  *int    `json:"max_qos_msgs,omitempty"`
	RetainPeriod       *int    `json:"retain,omitempty"`
	UpdatePeriod       *int    `json:"update_period,omitempty"`
	TopicPrefix        *string `json:"topic_prefix,omitempty"`
	RPCNotifications   *bool   `json:"rpc_ntf,omitempty"`
	StatusNotification *bool   `json:"status_ntf,omitempty"`
	UseClientCert      *bool   `json:"ssl_ca,omitempty"`
	EnableSNI          *bool   `json:"enable_sni,omitempty"`
}

// AuthConfiguration represents authentication settings
type AuthConfiguration struct {
	Enable   *bool   `json:"enable,omitempty"`
	Username *string `json:"user,omitempty"`
	Password *string `json:"pass,omitempty"`
	Realm    *string `json:"realm,omitempty"`
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
	WakeupPeriod *int               `json:"wakeup_period,omitempty"`
}

// TypedDeviceConfig represents device-specific settings
type TypedDeviceConfig struct {
	Name         *string    `json:"name,omitempty"`
	MAC          *string    `json:"mac,omitempty"`
	Hostname     *string    `json:"hostname,omitempty"`
	EcoMode      *bool      `json:"eco_mode,omitempty"`
	Profile      *string    `json:"profile,omitempty"`
	Discoverable *bool      `json:"discoverable,omitempty"`
	AddonType    *string    `json:"addon_type,omitempty"`
	FWAutoUpdate *bool      `json:"fw_auto_update,omitempty"`
	Timezone     *string    `json:"tz,omitempty"`
	LatLon       []float64  `json:"lat_lon,omitempty"`
	BleConfig    *BLEConfig `json:"ble,omitempty"`
}

// LocationConfig represents location settings
type LocationConfig struct {
	Timezone  *string  `json:"tz,omitempty"`
	Latitude  *float64 `json:"lat,omitempty"`
	Longitude *float64 `json:"lng,omitempty"`
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
	Enable *bool   `json:"enable,omitempty"`
	Server *string `json:"server,omitempty"`
}

// LocationConfiguration represents location and time settings
type LocationConfiguration struct {
	Timezone  *string  `json:"tz,omitempty"`
	Latitude  *float64 `json:"lat,omitempty"`
	Longitude *float64 `json:"lng,omitempty"`
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
	if w == nil {
		return nil
	}

	if w.Enable != nil && *w.Enable {
		if w.SSID == nil || *w.SSID == "" {
			return fmt.Errorf("SSID is required when WiFi is enabled")
		}
	}

	if w.SSID != nil && len(*w.SSID) > 32 {
		return fmt.Errorf("SSID must be 32 characters or less")
	}

	if w.Password != nil && len(*w.Password) > 63 {
		return fmt.Errorf("WiFi password must be 63 characters or less")
	}

	if w.IPv4Mode != nil && *w.IPv4Mode != "" && *w.IPv4Mode != "dhcp" && *w.IPv4Mode != "static" {
		return fmt.Errorf("IPv4 mode must be 'dhcp' or 'static'")
	}

	if w.IPv4Mode != nil && *w.IPv4Mode == "static" && w.StaticIP == nil {
		return fmt.Errorf("static IP configuration required when IPv4 mode is 'static'")
	}

	if w.StaticIP != nil {
		if err := w.StaticIP.Validate(); err != nil {
			return fmt.Errorf("static IP validation failed: %w", err)
		}
	}

	if w.RoamThreshold != nil && (*w.RoamThreshold < -100 || *w.RoamThreshold > -50) {
		return fmt.Errorf("roam threshold must be between -100 and -50 dBm")
	}

	return nil
}

// Validate validates static IP configuration
func (s *StaticIPConfig) Validate() error {
	if s == nil {
		return nil
	}

	if s.IP != nil && net.ParseIP(*s.IP) == nil {
		return fmt.Errorf("invalid IP address: %s", *s.IP)
	}

	if s.Netmask != nil && net.ParseIP(*s.Netmask) == nil {
		return fmt.Errorf("invalid netmask: %s", *s.Netmask)
	}

	if s.Gateway != nil && net.ParseIP(*s.Gateway) == nil {
		return fmt.Errorf("invalid gateway: %s", *s.Gateway)
	}

	if s.Nameserver != nil && *s.Nameserver != "" && net.ParseIP(*s.Nameserver) == nil {
		return fmt.Errorf("invalid nameserver: %s", *s.Nameserver)
	}

	return nil
}

// Validate validates MQTT configuration
func (m *MQTTConfiguration) Validate() error {
	if m == nil {
		return nil
	}

	if m.Enable == nil || !*m.Enable {
		return nil
	}

	if m.Server == nil || *m.Server == "" {
		return fmt.Errorf("MQTT server is required when MQTT is enabled")
	}

	if m.Server != nil && !isValidHostnamePort(*m.Server) && !isValidIPPort(*m.Server) && !isValidHostname(*m.Server) && net.ParseIP(*m.Server) == nil {
		return fmt.Errorf("invalid MQTT server format: %s", *m.Server)
	}

	if m.Port != nil && (*m.Port < 1 || *m.Port > 65535) {
		return fmt.Errorf("MQTT port must be between 1 and 65535")
	}

	if m.ClientID != nil && len(*m.ClientID) > 128 {
		return fmt.Errorf("MQTT client ID must be 128 characters or less")
	}

	if m.KeepAlive != nil && (*m.KeepAlive < 1 || *m.KeepAlive > 3600) {
		return fmt.Errorf("MQTT keep alive must be between 1 and 3600 seconds")
	}

	if m.MaxQueuedMessages != nil && (*m.MaxQueuedMessages < 1 || *m.MaxQueuedMessages > 100) {
		return fmt.Errorf("MQTT max queued messages must be between 1 and 100")
	}

	if m.TopicPrefix != nil && len(*m.TopicPrefix) > 256 {
		return fmt.Errorf("MQTT topic prefix must be 256 characters or less")
	}

	return nil
}

// Validate validates authentication configuration
func (a *AuthConfiguration) Validate() error {
	if a == nil {
		return nil
	}

	if a.Enable == nil || !*a.Enable {
		return nil
	}

	if a.Username == nil || *a.Username == "" {
		return fmt.Errorf("username is required when authentication is enabled")
	}

	if a.Username != nil && len(*a.Username) > 64 {
		return fmt.Errorf("username must be 64 characters or less")
	}

	if a.Password == nil || *a.Password == "" {
		return fmt.Errorf("password is required when authentication is enabled")
	}

	if a.Realm != nil && len(*a.Realm) > 64 {
		return fmt.Errorf("realm must be 64 characters or less")
	}

	return nil
}

// Validate validates system configuration
func (s *SystemConfiguration) Validate() error {
	if s == nil {
		return nil
	}

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

	if s.WakeupPeriod != nil && *s.WakeupPeriod < 1 {
		return fmt.Errorf("wakeup period must be positive")
	}

	return nil
}

// Validate validates device configuration
func (d *TypedDeviceConfig) Validate() error {
	if d == nil {
		return nil
	}

	if d.Name != nil && len(*d.Name) > 64 {
		return fmt.Errorf("device name must be 64 characters or less")
	}

	if d.Hostname != nil && *d.Hostname != "" && !isValidHostname(*d.Hostname) {
		return fmt.Errorf("invalid hostname: %s", *d.Hostname)
	}

	if d.Hostname != nil && len(*d.Hostname) > 63 {
		return fmt.Errorf("hostname must be 63 characters or less")
	}

	if d.Timezone != nil && len(*d.Timezone) > 64 {
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
	if l == nil {
		return nil
	}

	if l.Timezone != nil && len(*l.Timezone) > 64 {
		return fmt.Errorf("timezone must be 64 characters or less")
	}

	if l.Latitude != nil && (*l.Latitude < -90 || *l.Latitude > 90) {
		return fmt.Errorf("latitude must be between -90 and 90")
	}

	if l.Longitude != nil && (*l.Longitude < -180 || *l.Longitude > 180) {
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
			return fmt.Errorf("ethernet validation failed: %w", err)
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
			"wifi":            getWiFiSchema(),
			"mqtt":            getMQTTSchema(),
			"auth":            getAuthSchema(),
			"system":          getSystemSchema(),
			"cloud":           getCloudSchema(),
			"location":        getLocationSchema(),
			"relay":           getRelaySchema(),
			"led":             getLEDSchema(),
			"power_metering":  getPowerMeteringSchema(),
			"input":           getInputSchema(),
			"coiot":           getCoIoTSchema(),
			"dimming":         getDimmingSchema(),
			"roller":          getRollerSchema(),
			"color":           getColorSchema(),
			"temp_protection": getTempProtectionSchema(),
			"schedule":        getScheduleSchema(),
			"energy_meter":    getEnergyMeterSchema(),
			"motion":          getMotionSchema(),
			"sensor":          getSensorSchema(),
		},
	}
}

func getWiFiSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "WiFi Settings",
		"description": "Configure WiFi connection for the device",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable WiFi",
				"description": "Enable or disable WiFi connection",
			},
			"ssid": map[string]interface{}{
				"type":        "string",
				"title":       "Network Name (SSID)",
				"description": "WiFi network name",
				"maxLength":   32,
			},
			"password": map[string]interface{}{
				"type":        "string",
				"title":       "Password",
				"description": "WiFi network password",
				"maxLength":   63,
				"format":      "password",
			},
			"ipv4mode": map[string]interface{}{
				"type":        "string",
				"title":       "IP Mode",
				"description": "How to obtain IP address",
				"enum":        []string{"dhcp", "static"},
				"default":     "dhcp",
			},
			"static_ip": map[string]interface{}{
				"type":        "object",
				"title":       "Static IP Settings",
				"description": "Required when IP mode is static",
				"properties": map[string]interface{}{
					"ip": map[string]interface{}{
						"type":        "string",
						"title":       "IP Address",
						"description": "Static IP address",
						"format":      "ipv4",
					},
					"netmask": map[string]interface{}{
						"type":        "string",
						"title":       "Netmask",
						"description": "Network mask",
						"format":      "ipv4",
						"default":     "255.255.255.0",
					},
					"gw": map[string]interface{}{
						"type":        "string",
						"title":       "Gateway",
						"description": "Default gateway",
						"format":      "ipv4",
					},
					"nameserver": map[string]interface{}{
						"type":        "string",
						"title":       "DNS Server",
						"description": "DNS server address",
						"format":      "ipv4",
					},
				},
			},
			"roam_threshold": map[string]interface{}{
				"type":        "integer",
				"title":       "Roaming Threshold",
				"description": "Signal strength threshold for roaming (dBm)",
				"minimum":     -100,
				"maximum":     -50,
			},
		},
		"if": map[string]interface{}{
			"properties": map[string]interface{}{"enable": map[string]interface{}{"const": true}},
		},
		"then": map[string]interface{}{
			"required": []string{"ssid"},
		},
	}
}

func getMQTTSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "MQTT Settings",
		"description": "Configure MQTT broker connection",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable MQTT",
				"description": "Enable or disable MQTT connection",
			},
			"server": map[string]interface{}{
				"type":        "string",
				"title":       "Server",
				"description": "MQTT broker hostname or IP address",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"title":       "Port",
				"description": "MQTT broker port",
				"minimum":     1,
				"maximum":     65535,
				"default":     1883,
			},
			"user": map[string]interface{}{
				"type":        "string",
				"title":       "Username",
				"description": "MQTT authentication username",
			},
			"pass": map[string]interface{}{
				"type":        "string",
				"title":       "Password",
				"description": "MQTT authentication password",
				"format":      "password",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"title":       "Client ID",
				"description": "MQTT client identifier",
				"maxLength":   128,
			},
			"clean_session": map[string]interface{}{
				"type":        "boolean",
				"title":       "Clean Session",
				"description": "Start with clean session",
				"default":     true,
			},
			"keep_alive": map[string]interface{}{
				"type":        "integer",
				"title":       "Keep Alive",
				"description": "Keep alive interval in seconds",
				"minimum":     1,
				"maximum":     3600,
				"default":     60,
			},
			"max_qos_msgs": map[string]interface{}{
				"type":        "integer",
				"title":       "Max Queued Messages",
				"description": "Maximum queued QoS messages",
				"minimum":     1,
				"maximum":     100,
			},
			"retain": map[string]interface{}{
				"type":        "integer",
				"title":       "Retain Period",
				"description": "Message retain period",
			},
			"update_period": map[string]interface{}{
				"type":        "integer",
				"title":       "Update Period",
				"description": "Status update period in seconds",
			},
			"topic_prefix": map[string]interface{}{
				"type":        "string",
				"title":       "Topic Prefix",
				"description": "Prefix for MQTT topics",
				"maxLength":   256,
			},
			"rpc_ntf": map[string]interface{}{
				"type":        "boolean",
				"title":       "RPC Notifications",
				"description": "Enable RPC notifications over MQTT",
			},
			"status_ntf": map[string]interface{}{
				"type":        "boolean",
				"title":       "Status Notifications",
				"description": "Enable status notifications over MQTT",
			},
		},
	}
}

func getAuthSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "Authentication",
		"description": "Configure device access authentication",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable Authentication",
				"description": "Require authentication to access the device",
			},
			"user": map[string]interface{}{
				"type":        "string",
				"title":       "Username",
				"description": "Authentication username",
				"maxLength":   64,
			},
			"pass": map[string]interface{}{
				"type":        "string",
				"title":       "Password",
				"description": "Authentication password",
				"format":      "password",
			},
			"realm": map[string]interface{}{
				"type":        "string",
				"title":       "Realm",
				"description": "Authentication realm",
				"maxLength":   64,
			},
		},
		"if": map[string]interface{}{
			"properties": map[string]interface{}{"enable": map[string]interface{}{"const": true}},
		},
		"then": map[string]interface{}{
			"required": []string{"user", "pass"},
		},
	}
}

func getSystemSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "System Settings",
		"description": "Configure system-level device settings",
		"properties": map[string]interface{}{
			"device": map[string]interface{}{
				"type":        "object",
				"title":       "Device Settings",
				"description": "General device settings",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"title":       "Device Name",
						"description": "Friendly name for the device",
						"maxLength":   64,
					},
					"hostname": map[string]interface{}{
						"type":        "string",
						"title":       "Hostname",
						"description": "Network hostname",
						"maxLength":   63,
					},
					"eco_mode": map[string]interface{}{
						"type":        "boolean",
						"title":       "Eco Mode",
						"description": "Enable power saving mode",
					},
					"discoverable": map[string]interface{}{
						"type":        "boolean",
						"title":       "Discoverable",
						"description": "Allow device discovery",
						"default":     true,
					},
					"fw_auto_update": map[string]interface{}{
						"type":        "boolean",
						"title":       "Auto-Update Firmware",
						"description": "Automatically update firmware",
					},
					"tz": map[string]interface{}{
						"type":        "string",
						"title":       "Timezone",
						"description": "Device timezone",
						"maxLength":   64,
					},
				},
			},
			"location": map[string]interface{}{
				"type":        "object",
				"title":       "Location",
				"description": "Device location for sunrise/sunset calculations",
				"properties": map[string]interface{}{
					"tz": map[string]interface{}{
						"type":        "string",
						"title":       "Timezone",
						"description": "IANA timezone identifier",
						"maxLength":   64,
					},
					"lat": map[string]interface{}{
						"type":        "number",
						"title":       "Latitude",
						"description": "Geographic latitude",
						"minimum":     -90,
						"maximum":     90,
					},
					"lng": map[string]interface{}{
						"type":        "number",
						"title":       "Longitude",
						"description": "Geographic longitude",
						"minimum":     -180,
						"maximum":     180,
					},
				},
			},
			"debug": map[string]interface{}{
				"type":        "object",
				"title":       "Debug Settings",
				"description": "Debug and logging configuration",
				"properties": map[string]interface{}{
					"level": map[string]interface{}{
						"type":        "integer",
						"title":       "Debug Level",
						"description": "Log verbosity level (0-4)",
						"minimum":     0,
						"maximum":     4,
						"default":     0,
					},
					"mqtt": map[string]interface{}{
						"type":        "boolean",
						"title":       "MQTT Debug Output",
						"description": "Send debug output over MQTT",
					},
					"websocket": map[string]interface{}{
						"type":        "boolean",
						"title":       "WebSocket Debug Output",
						"description": "Send debug output over WebSocket",
					},
				},
			},
			"sntp": map[string]interface{}{
				"type":        "object",
				"title":       "Time Server",
				"description": "SNTP time synchronization settings",
				"properties": map[string]interface{}{
					"server": map[string]interface{}{
						"type":        "string",
						"title":       "SNTP Server",
						"description": "NTP server hostname",
						"default":     "time.google.com",
					},
				},
			},
		},
	}
}

func getCloudSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "Cloud Settings",
		"description": "Configure Shelly cloud connection",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable Cloud",
				"description": "Connect to Shelly cloud service",
			},
			"server": map[string]interface{}{
				"type":        "string",
				"title":       "Cloud Server",
				"description": "Cloud server hostname",
			},
		},
	}
}

func getLocationSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "Location",
		"description": "Device geographic location",
		"properties": map[string]interface{}{
			"tz": map[string]interface{}{
				"type":        "string",
				"title":       "Timezone",
				"description": "IANA timezone identifier (e.g., Europe/London)",
				"maxLength":   64,
			},
			"lat": map[string]interface{}{
				"type":        "number",
				"title":       "Latitude",
				"description": "Geographic latitude in degrees",
				"minimum":     -90,
				"maximum":     90,
			},
			"lng": map[string]interface{}{
				"type":        "number",
				"title":       "Longitude",
				"description": "Geographic longitude in degrees",
				"minimum":     -180,
				"maximum":     180,
			},
		},
	}
}

func getRelaySchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Relay Settings",
		"description":         "Configure relay switch behavior",
		"x-device-capability": "relay",
		"properties": map[string]interface{}{
			"default_state": map[string]interface{}{
				"type":        "string",
				"title":       "Default State",
				"description": "Relay state after power-on",
				"enum":        []string{"off", "on", "last", "switch"},
				"default":     "off",
			},
			"btn_type": map[string]interface{}{
				"type":        "string",
				"title":       "Button Type",
				"description": "Type of connected switch",
				"enum":        []string{"momentary", "toggle", "edge", "detached"},
			},
			"auto_on": map[string]interface{}{
				"type":        "integer",
				"title":       "Auto On Timer",
				"description": "Automatically turn on after X seconds (0 = disabled)",
				"minimum":     0,
			},
			"auto_off": map[string]interface{}{
				"type":        "integer",
				"title":       "Auto Off Timer",
				"description": "Automatically turn off after X seconds (0 = disabled)",
				"minimum":     0,
			},
			"has_timer": map[string]interface{}{
				"type":        "boolean",
				"title":       "Timer Support",
				"description": "Whether the relay supports timer functions",
			},
			"max_power_limit": map[string]interface{}{
				"type":        "integer",
				"title":       "Max Power Limit",
				"description": "Maximum power limit in watts (0 = disabled)",
				"minimum":     0,
			},
			"relays": map[string]interface{}{
				"type":        "array",
				"title":       "Individual Relays",
				"description": "Configuration for multi-channel relays",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "integer",
							"title":       "Relay ID",
							"description": "Relay channel index",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"title":       "Name",
							"description": "Friendly name for this relay",
						},
						"default_state": map[string]interface{}{
							"type":  "string",
							"title": "Default State",
							"enum":  []string{"off", "on", "last", "switch"},
						},
						"auto_on": map[string]interface{}{
							"type":    "integer",
							"title":   "Auto On",
							"minimum": 0,
						},
						"auto_off": map[string]interface{}{
							"type":    "integer",
							"title":   "Auto Off",
							"minimum": 0,
						},
					},
				},
			},
		},
	}
}

func getLEDSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "LED Indicator",
		"description":         "Configure status LED behavior",
		"x-device-capability": "led",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable LED",
				"description": "Enable status LED indicator",
				"default":     true,
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"title":       "LED Mode",
				"description": "LED behavior mode",
				"enum":        []string{"off", "on", "auto"},
			},
			"brightness": map[string]interface{}{
				"type":        "integer",
				"title":       "Brightness",
				"description": "LED brightness level (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"night_mode": map[string]interface{}{
				"type":        "boolean",
				"title":       "Night Mode",
				"description": "Enable night mode with reduced brightness",
			},
			"night_mode_brightness": map[string]interface{}{
				"type":        "integer",
				"title":       "Night Brightness",
				"description": "LED brightness during night mode (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"night_mode_start": map[string]interface{}{
				"type":        "string",
				"title":       "Night Mode Start",
				"description": "Start time for night mode (HH:MM)",
				"pattern":     "^([01]?[0-9]|2[0-3]):[0-5][0-9]$",
			},
			"night_mode_end": map[string]interface{}{
				"type":        "string",
				"title":       "Night Mode End",
				"description": "End time for night mode (HH:MM)",
				"pattern":     "^([01]?[0-9]|2[0-3]):[0-5][0-9]$",
			},
			"power_indication": map[string]interface{}{
				"type":        "boolean",
				"title":       "Power Indication",
				"description": "LED indicates power state",
			},
			"network_indication": map[string]interface{}{
				"type":        "boolean",
				"title":       "Network Indication",
				"description": "LED indicates network status",
			},
		},
	}
}

func getPowerMeteringSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Power Metering",
		"description":         "Configure power measurement and protection",
		"x-device-capability": "power_metering",
		"properties": map[string]interface{}{
			"max_power": map[string]interface{}{
				"type":        "integer",
				"title":       "Max Power",
				"description": "Maximum power limit in watts",
				"minimum":     0,
			},
			"max_voltage": map[string]interface{}{
				"type":        "integer",
				"title":       "Max Voltage",
				"description": "Maximum voltage limit",
				"minimum":     0,
			},
			"max_current": map[string]interface{}{
				"type":        "number",
				"title":       "Max Current",
				"description": "Maximum current limit in amps",
				"minimum":     0,
			},
			"protection_action": map[string]interface{}{
				"type":        "string",
				"title":       "Protection Action",
				"description": "Action when limits exceeded",
				"enum":        []string{"off", "notify", "restart"},
			},
			"power_correction": map[string]interface{}{
				"type":        "number",
				"title":       "Power Correction",
				"description": "Power measurement correction factor",
				"minimum":     -100,
				"maximum":     100,
			},
			"voltage_correction": map[string]interface{}{
				"type":        "number",
				"title":       "Voltage Correction",
				"description": "Voltage measurement correction factor",
				"minimum":     -100,
				"maximum":     100,
			},
			"current_correction": map[string]interface{}{
				"type":        "number",
				"title":       "Current Correction",
				"description": "Current measurement correction factor",
				"minimum":     -100,
				"maximum":     100,
			},
			"reporting_period": map[string]interface{}{
				"type":        "integer",
				"title":       "Reporting Period",
				"description": "Power reporting interval in seconds",
				"minimum":     1,
			},
			"cost_per_kwh": map[string]interface{}{
				"type":        "number",
				"title":       "Cost per kWh",
				"description": "Energy cost per kilowatt-hour",
				"minimum":     0,
			},
			"currency": map[string]interface{}{
				"type":        "string",
				"title":       "Currency",
				"description": "Currency code (e.g., USD, EUR)",
				"maxLength":   3,
			},
		},
	}
}

func getInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Input Settings",
		"description":         "Configure input/button behavior",
		"x-device-capability": "input",
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"type":        "string",
				"title":       "Input Type",
				"description": "Type of connected input",
				"enum":        []string{"button", "switch", "analog"},
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"title":       "Input Mode",
				"description": "How the input controls the device",
				"enum":        []string{"momentary", "follow", "flip", "detached"},
			},
			"inverted": map[string]interface{}{
				"type":        "boolean",
				"title":       "Inverted",
				"description": "Invert input logic",
			},
			"debounce_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Debounce Time",
				"description": "Input debounce time in milliseconds",
				"minimum":     0,
				"maximum":     1000,
			},
			"long_push_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Long Push Time",
				"description": "Time for long push detection in milliseconds",
				"minimum":     100,
			},
			"multi_push_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Multi Push Time",
				"description": "Time window for multi-push detection in milliseconds",
				"minimum":     100,
			},
			"single_push_action": map[string]interface{}{
				"type":        "string",
				"title":       "Single Push Action",
				"description": "Action on single button press",
			},
			"double_push_action": map[string]interface{}{
				"type":        "string",
				"title":       "Double Push Action",
				"description": "Action on double button press",
			},
			"triple_push_action": map[string]interface{}{
				"type":        "string",
				"title":       "Triple Push Action",
				"description": "Action on triple button press",
			},
			"long_push_action": map[string]interface{}{
				"type":        "string",
				"title":       "Long Push Action",
				"description": "Action on long button press",
			},
			"inputs": map[string]interface{}{
				"type":        "array",
				"title":       "Individual Inputs",
				"description": "Configuration for multi-input devices",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":  "integer",
							"title": "Input ID",
						},
						"name": map[string]interface{}{
							"type":  "string",
							"title": "Name",
						},
						"type": map[string]interface{}{
							"type": "string",
							"enum": []string{"button", "switch", "analog"},
						},
						"mode": map[string]interface{}{
							"type": "string",
							"enum": []string{"momentary", "follow", "flip", "detached"},
						},
						"inverted": map[string]interface{}{
							"type": "boolean",
						},
					},
				},
			},
		},
	}
}

func getCoIoTSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "CoIoT Settings",
		"description": "Configure CoIoT protocol (local multicast)",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable CoIoT",
				"description": "Enable CoIoT protocol for local discovery",
			},
			"server": map[string]interface{}{
				"type":        "string",
				"title":       "Server",
				"description": "CoIoT server address (unicast mode)",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"title":       "Port",
				"description": "CoIoT port",
				"minimum":     1,
				"maximum":     65535,
				"default":     5683,
			},
			"period": map[string]interface{}{
				"type":        "integer",
				"title":       "Update Period",
				"description": "Status update interval in seconds",
				"minimum":     1,
			},
		},
	}
}

func getDimmingSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Dimmer Settings",
		"description":         "Configure dimmer behavior",
		"x-device-capability": "dimming",
		"properties": map[string]interface{}{
			"min_brightness": map[string]interface{}{
				"type":        "integer",
				"title":       "Minimum Brightness",
				"description": "Minimum brightness level (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"max_brightness": map[string]interface{}{
				"type":        "integer",
				"title":       "Maximum Brightness",
				"description": "Maximum brightness level (0-100)",
				"minimum":     0,
				"maximum":     100,
				"default":     100,
			},
			"default_brightness": map[string]interface{}{
				"type":        "integer",
				"title":       "Default Brightness",
				"description": "Default brightness when turned on (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"default_state": map[string]interface{}{
				"type":        "boolean",
				"title":       "Default State",
				"description": "Default on/off state after power-on",
			},
			"fade_rate": map[string]interface{}{
				"type":        "integer",
				"title":       "Fade Rate",
				"description": "Dimming transition rate",
				"minimum":     0,
			},
			"transition": map[string]interface{}{
				"type":        "integer",
				"title":       "Transition Time",
				"description": "Transition time in milliseconds",
				"minimum":     0,
			},
			"leading_edge": map[string]interface{}{
				"type":        "boolean",
				"title":       "Leading Edge",
				"description": "Use leading edge dimming (for resistive loads)",
			},
			"warmup_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Warmup Time",
				"description": "LED warmup time in milliseconds",
				"minimum":     0,
			},
			"night_mode": map[string]interface{}{
				"type":        "boolean",
				"title":       "Night Mode",
				"description": "Enable night mode",
			},
			"night_mode_brightness": map[string]interface{}{
				"type":        "integer",
				"title":       "Night Mode Brightness",
				"description": "Brightness during night mode (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"night_mode_start": map[string]interface{}{
				"type":        "string",
				"title":       "Night Mode Start",
				"description": "Start time (HH:MM)",
				"pattern":     "^([01]?[0-9]|2[0-3]):[0-5][0-9]$",
			},
			"night_mode_end": map[string]interface{}{
				"type":        "string",
				"title":       "Night Mode End",
				"description": "End time (HH:MM)",
				"pattern":     "^([01]?[0-9]|2[0-3]):[0-5][0-9]$",
			},
		},
	}
}

func getRollerSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Roller/Shutter Settings",
		"description":         "Configure roller shutter or blind behavior",
		"x-device-capability": "roller",
		"properties": map[string]interface{}{
			"motor_direction": map[string]interface{}{
				"type":        "string",
				"title":       "Motor Direction",
				"description": "Motor rotation direction",
				"enum":        []string{"normal", "reversed"},
			},
			"motor_speed": map[string]interface{}{
				"type":        "integer",
				"title":       "Motor Speed",
				"description": "Motor speed percentage (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"calibration_state": map[string]interface{}{
				"type":        "string",
				"title":       "Calibration State",
				"description": "Current calibration status",
				"enum":        []string{"uncalibrated", "calibrating", "calibrated"},
			},
			"max_open_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Max Open Time",
				"description": "Maximum time to fully open (seconds)",
				"minimum":     1,
			},
			"max_close_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Max Close Time",
				"description": "Maximum time to fully close (seconds)",
				"minimum":     1,
			},
			"default_position": map[string]interface{}{
				"type":        "integer",
				"title":       "Default Position",
				"description": "Default position after power-on (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"positioning_enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable Positioning",
				"description": "Allow setting specific positions",
			},
			"obstacle_detection": map[string]interface{}{
				"type":        "boolean",
				"title":       "Obstacle Detection",
				"description": "Enable obstacle detection",
			},
			"obstacle_power": map[string]interface{}{
				"type":        "integer",
				"title":       "Obstacle Power Threshold",
				"description": "Power threshold for obstacle detection (watts)",
				"minimum":     0,
			},
			"safety_switch": map[string]interface{}{
				"type":        "boolean",
				"title":       "Safety Switch",
				"description": "Enable safety switch",
			},
			"swap_inputs": map[string]interface{}{
				"type":        "boolean",
				"title":       "Swap Inputs",
				"description": "Swap up/down input functions",
			},
			"input_mode": map[string]interface{}{
				"type":        "string",
				"title":       "Input Mode",
				"description": "Button input mode",
				"enum":        []string{"single", "dual"},
			},
			"tilt_enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Tilt Control",
				"description": "Enable blind tilt control",
			},
			"tilt_position": map[string]interface{}{
				"type":        "integer",
				"title":       "Tilt Position",
				"description": "Current tilt position (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"max_tilt_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Max Tilt Time",
				"description": "Maximum tilt time (seconds)",
				"minimum":     0,
			},
		},
	}
}

func getColorSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Color/RGBW Settings",
		"description":         "Configure RGB(W) color output",
		"x-device-capability": "color",
		"properties": map[string]interface{}{
			"mode": map[string]interface{}{
				"type":        "string",
				"title":       "Color Mode",
				"description": "Color output mode",
				"enum":        []string{"color", "white", "color_white"},
			},
			"default_color": map[string]interface{}{
				"type":        "object",
				"title":       "Default Color",
				"description": "Default RGB color",
				"properties": map[string]interface{}{
					"r": map[string]interface{}{
						"type":    "integer",
						"title":   "Red",
						"minimum": 0,
						"maximum": 255,
					},
					"g": map[string]interface{}{
						"type":    "integer",
						"title":   "Green",
						"minimum": 0,
						"maximum": 255,
					},
					"b": map[string]interface{}{
						"type":    "integer",
						"title":   "Blue",
						"minimum": 0,
						"maximum": 255,
					},
				},
			},
			"default_white": map[string]interface{}{
				"type":        "integer",
				"title":       "Default White/Temperature",
				"description": "Default white level or color temperature",
				"minimum":     0,
				"maximum":     255,
			},
			"effects_enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable Effects",
				"description": "Enable lighting effects",
			},
			"active_effect": map[string]interface{}{
				"type":        "integer",
				"title":       "Active Effect",
				"description": "Currently active effect ID",
			},
			"effect_speed": map[string]interface{}{
				"type":        "integer",
				"title":       "Effect Speed",
				"description": "Effect animation speed (0-100)",
				"minimum":     0,
				"maximum":     100,
			},
			"red_calibration": map[string]interface{}{
				"type":        "number",
				"title":       "Red Calibration",
				"description": "Red channel calibration factor",
			},
			"green_calibration": map[string]interface{}{
				"type":        "number",
				"title":       "Green Calibration",
				"description": "Green channel calibration factor",
			},
			"blue_calibration": map[string]interface{}{
				"type":        "number",
				"title":       "Blue Calibration",
				"description": "Blue channel calibration factor",
			},
			"white_calibration": map[string]interface{}{
				"type":        "number",
				"title":       "White Calibration",
				"description": "White channel calibration factor",
			},
		},
	}
}

func getTempProtectionSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Temperature Protection",
		"description":         "Configure temperature-based protection",
		"x-device-capability": "temp_protection",
		"properties": map[string]interface{}{
			"max_temp": map[string]interface{}{
				"type":        "number",
				"title":       "Max Temperature",
				"description": "Maximum temperature threshold (Celsius)",
			},
			"min_temp": map[string]interface{}{
				"type":        "number",
				"title":       "Min Temperature",
				"description": "Minimum temperature threshold (Celsius)",
			},
			"warning_temp": map[string]interface{}{
				"type":        "number",
				"title":       "Warning Temperature",
				"description": "Temperature warning threshold (Celsius)",
			},
			"overheat_action": map[string]interface{}{
				"type":        "string",
				"title":       "Overheat Action",
				"description": "Action when overheating",
				"enum":        []string{"off", "alert", "reduce_power"},
			},
			"freeze_action": map[string]interface{}{
				"type":        "string",
				"title":       "Freeze Action",
				"description": "Action when freezing",
				"enum":        []string{"off", "alert", "heat"},
			},
			"hysteresis": map[string]interface{}{
				"type":        "number",
				"title":       "Hysteresis",
				"description": "Temperature hysteresis (degrees)",
				"minimum":     0,
			},
			"check_interval": map[string]interface{}{
				"type":        "integer",
				"title":       "Check Interval",
				"description": "Temperature check interval (seconds)",
				"minimum":     1,
			},
		},
	}
}

func getScheduleSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":        "object",
		"title":       "Schedule Settings",
		"description": "Configure automated schedules",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable Schedules",
				"description": "Enable schedule functionality",
			},
			"use_sunrise_sunset": map[string]interface{}{
				"type":        "boolean",
				"title":       "Sunrise/Sunset",
				"description": "Allow sunrise/sunset-based schedules",
			},
			"latitude": map[string]interface{}{
				"type":        "number",
				"title":       "Latitude",
				"description": "Location latitude for sunrise/sunset",
				"minimum":     -90,
				"maximum":     90,
			},
			"longitude": map[string]interface{}{
				"type":        "number",
				"title":       "Longitude",
				"description": "Location longitude for sunrise/sunset",
				"minimum":     -180,
				"maximum":     180,
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"title":       "Timezone",
				"description": "Schedule timezone",
			},
			"schedules": map[string]interface{}{
				"type":        "array",
				"title":       "Schedules",
				"description": "List of scheduled actions",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":  "integer",
							"title": "Schedule ID",
						},
						"name": map[string]interface{}{
							"type":  "string",
							"title": "Name",
						},
						"enabled": map[string]interface{}{
							"type":  "boolean",
							"title": "Enabled",
						},
						"time": map[string]interface{}{
							"type":        "string",
							"title":       "Time",
							"description": "Time (HH:MM) or sunrise+/-offset",
						},
						"days": map[string]interface{}{
							"type":  "array",
							"title": "Days",
							"items": map[string]interface{}{
								"type": "string",
								"enum": []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
							},
						},
						"action": map[string]interface{}{
							"type":        "string",
							"title":       "Action",
							"description": "Action to perform (on, off, toggle, dim:XX)",
						},
					},
				},
			},
		},
	}
}

func getEnergyMeterSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Energy Meter Settings",
		"description":         "Configure energy consumption tracking",
		"x-device-capability": "energy_meter",
		"properties": map[string]interface{}{
			"reporting_interval": map[string]interface{}{
				"type":        "integer",
				"title":       "Reporting Interval",
				"description": "Energy reporting interval (seconds)",
				"minimum":     1,
			},
			"retention_days": map[string]interface{}{
				"type":        "integer",
				"title":       "Retention Days",
				"description": "Days to keep energy history",
				"minimum":     1,
			},
			"daily_limit_kwh": map[string]interface{}{
				"type":        "number",
				"title":       "Daily Limit",
				"description": "Daily energy limit (kWh)",
				"minimum":     0,
			},
			"monthly_limit_kwh": map[string]interface{}{
				"type":        "number",
				"title":       "Monthly Limit",
				"description": "Monthly energy limit (kWh)",
				"minimum":     0,
			},
			"alert_email": map[string]interface{}{
				"type":        "string",
				"title":       "Alert Email",
				"description": "Email for energy alerts",
				"format":      "email",
			},
			"tariffs": map[string]interface{}{
				"type":        "array",
				"title":       "Tariffs",
				"description": "Time-based energy pricing",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":  "string",
							"title": "Tariff Name",
						},
						"start_time": map[string]interface{}{
							"type":  "string",
							"title": "Start Time",
						},
						"end_time": map[string]interface{}{
							"type":  "string",
							"title": "End Time",
						},
						"days": map[string]interface{}{
							"type":  "array",
							"title": "Days",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"price_per_kwh": map[string]interface{}{
							"type":    "number",
							"title":   "Price per kWh",
							"minimum": 0,
						},
					},
				},
			},
		},
	}
}

func getMotionSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Motion Sensor Settings",
		"description":         "Configure motion detection",
		"x-device-capability": "motion",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"title":       "Enable Motion Sensor",
				"description": "Enable motion detection",
			},
			"sensitivity": map[string]interface{}{
				"type":        "integer",
				"title":       "Sensitivity",
				"description": "Motion detection sensitivity (1-100)",
				"minimum":     1,
				"maximum":     100,
			},
			"blind_time": map[string]interface{}{
				"type":        "integer",
				"title":       "Blind Time",
				"description": "Time to ignore motion after trigger (seconds)",
				"minimum":     0,
			},
			"detection_timeout": map[string]interface{}{
				"type":        "integer",
				"title":       "Detection Timeout",
				"description": "Time to clear motion status (seconds)",
				"minimum":     1,
			},
			"on_motion_action": map[string]interface{}{
				"type":        "string",
				"title":       "On Motion Action",
				"description": "Action when motion detected",
			},
			"on_clear_action": map[string]interface{}{
				"type":        "string",
				"title":       "On Clear Action",
				"description": "Action when motion cleared",
			},
			"led_indication": map[string]interface{}{
				"type":        "boolean",
				"title":       "LED Indication",
				"description": "Flash LED on motion detection",
			},
		},
	}
}

func getSensorSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                "object",
		"title":               "Environmental Sensor Settings",
		"description":         "Configure temperature, humidity, and light sensors",
		"x-device-capability": "sensor",
		"properties": map[string]interface{}{
			"temp_unit": map[string]interface{}{
				"type":        "string",
				"title":       "Temperature Unit",
				"description": "Temperature display unit",
				"enum":        []string{"C", "F"},
				"default":     "C",
			},
			"temp_offset": map[string]interface{}{
				"type":        "number",
				"title":       "Temperature Offset",
				"description": "Temperature calibration offset",
			},
			"temp_reporting": map[string]interface{}{
				"type":        "integer",
				"title":       "Temperature Reporting",
				"description": "Temperature reporting interval (seconds)",
				"minimum":     1,
			},
			"humidity_offset": map[string]interface{}{
				"type":        "number",
				"title":       "Humidity Offset",
				"description": "Humidity calibration offset",
			},
			"humidity_reporting": map[string]interface{}{
				"type":        "integer",
				"title":       "Humidity Reporting",
				"description": "Humidity reporting interval (seconds)",
				"minimum":     1,
			},
			"lux_offset": map[string]interface{}{
				"type":        "number",
				"title":       "Lux Offset",
				"description": "Light level calibration offset",
			},
			"lux_reporting": map[string]interface{}{
				"type":        "integer",
				"title":       "Lux Reporting",
				"description": "Light level reporting interval (seconds)",
				"minimum":     1,
			},
			"temp_min": map[string]interface{}{
				"type":        "number",
				"title":       "Min Temperature Alert",
				"description": "Alert when temperature below this value",
			},
			"temp_max": map[string]interface{}{
				"type":        "number",
				"title":       "Max Temperature Alert",
				"description": "Alert when temperature above this value",
			},
			"humidity_min": map[string]interface{}{
				"type":        "number",
				"title":       "Min Humidity Alert",
				"description": "Alert when humidity below this value",
				"minimum":     0,
				"maximum":     100,
			},
			"humidity_max": map[string]interface{}{
				"type":        "number",
				"title":       "Max Humidity Alert",
				"description": "Alert when humidity above this value",
				"minimum":     0,
				"maximum":     100,
			},
			"lux_min": map[string]interface{}{
				"type":        "number",
				"title":       "Min Lux Alert",
				"description": "Alert when light below this value",
				"minimum":     0,
			},
			"lux_max": map[string]interface{}{
				"type":        "number",
				"title":       "Max Lux Alert",
				"description": "Alert when light above this value",
				"minimum":     0,
			},
		},
	}
}

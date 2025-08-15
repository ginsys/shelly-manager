package shelly

import (
	"encoding/json"
	"time"
)

// DeviceInfo represents basic device information
type DeviceInfo struct {
	ID         string `json:"id"`
	MAC        string `json:"mac"`
	Model      string `json:"model"`
	Generation int    `json:"gen"`
	FirmwareID string `json:"fw_id"`
	Version    string `json:"ver"`
	App        string `json:"app"`
	AuthEn     bool   `json:"auth_en"`
	AuthDomain string `json:"auth_domain,omitempty"`

	// Gen1 specific fields
	Type string `json:"type,omitempty"`
	FW   string `json:"fw,omitempty"`
	Auth bool   `json:"auth,omitempty"`

	// Metadata
	IP         string    `json:"-"`
	Discovered time.Time `json:"-"`
}

// DeviceStatus represents the current operational status of a device
type DeviceStatus struct {
	// Common status fields
	Temperature     float64      `json:"temperature,omitempty"`
	Overtemperature bool         `json:"overtemperature,omitempty"`
	WiFiStatus      *WiFiStatus  `json:"wifi_sta,omitempty"`
	Cloud           *CloudStatus `json:"cloud,omitempty"`
	MQTT            *MQTTStatus  `json:"mqtt,omitempty"`
	Time            string       `json:"time,omitempty"`
	Unixtime        int64        `json:"unixtime,omitempty"`
	HasUpdate       bool         `json:"has_update,omitempty"`
	RAMTotal        int          `json:"ram_total,omitempty"`
	RAMFree         int          `json:"ram_free,omitempty"`
	FSSize          int          `json:"fs_size,omitempty"`
	FSFree          int          `json:"fs_free,omitempty"`
	Uptime          int          `json:"uptime,omitempty"`

	// Component statuses (varies by device type)
	Switches []SwitchStatus `json:"switches,omitempty"`
	Lights   []LightStatus  `json:"lights,omitempty"`
	Inputs   []InputStatus  `json:"inputs,omitempty"`
	Rollers  []RollerStatus `json:"rollers,omitempty"`
	Meters   []MeterStatus  `json:"meters,omitempty"`

	// Raw data for device-specific fields
	Raw map[string]interface{} `json:"-"`
}

// DeviceConfig represents device configuration
type DeviceConfig struct {
	// Device settings
	Name     string  `json:"name,omitempty"`
	Timezone string  `json:"tz,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lng      float64 `json:"lng,omitempty"`

	// Network configuration
	WiFi     *WiFiConfig     `json:"wifi,omitempty"`
	Ethernet *EthernetConfig `json:"eth,omitempty"`

	// Security
	Auth *AuthConfig `json:"auth,omitempty"`

	// Cloud connectivity
	Cloud *CloudConfig `json:"cloud,omitempty"`

	// MQTT settings
	MQTT *MQTTConfig `json:"mqtt,omitempty"`

	// Component configurations (varies by device type)
	Switches []SwitchConfig `json:"switches,omitempty"`
	Lights   []LightConfig  `json:"lights,omitempty"`
	Inputs   []InputConfig  `json:"inputs,omitempty"`
	Rollers  []RollerConfig `json:"rollers,omitempty"`

	// System settings
	Debug bool `json:"debug,omitempty"`
	WebUI bool `json:"web_ui,omitempty"`

	// Raw configuration for device-specific fields
	Raw json.RawMessage `json:"-"`
}

// WiFiStatus represents WiFi connection status
type WiFiStatus struct {
	Connected bool   `json:"connected"`
	SSID      string `json:"ssid,omitempty"`
	IP        string `json:"ip,omitempty"`
	RSSI      int    `json:"rssi,omitempty"`
}

// WiFiConfig represents WiFi configuration
type WiFiConfig struct {
	Enable   bool   `json:"enable"`
	SSID     string `json:"ssid"`
	Password string `json:"pass,omitempty"`
	IPV4Mode string `json:"ipv4mode,omitempty"` // "dhcp" or "static"
	IP       string `json:"ip,omitempty"`
	Netmask  string `json:"netmask,omitempty"`
	Gateway  string `json:"gw,omitempty"`
	DNS      string `json:"nameserver,omitempty"`
}

// EthernetConfig represents Ethernet configuration
type EthernetConfig struct {
	Enable   bool   `json:"enable"`
	IPV4Mode string `json:"ipv4mode,omitempty"` // "dhcp" or "static"
	IP       string `json:"ip,omitempty"`
	Netmask  string `json:"netmask,omitempty"`
	Gateway  string `json:"gw,omitempty"`
	DNS      string `json:"nameserver,omitempty"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enable   bool   `json:"enable"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// CloudStatus represents cloud connection status
type CloudStatus struct {
	Enabled   bool `json:"enabled"`
	Connected bool `json:"connected"`
}

// CloudConfig represents cloud configuration
type CloudConfig struct {
	Enable bool   `json:"enable"`
	Server string `json:"server,omitempty"`
}

// MQTTStatus represents MQTT connection status
type MQTTStatus struct {
	Connected bool `json:"connected"`
}

// MQTTConfig represents MQTT configuration
type MQTTConfig struct {
	Enable          bool   `json:"enable"`
	Server          string `json:"server,omitempty"`
	User            string `json:"user,omitempty"`
	Password        string `json:"pass,omitempty"`
	ID              string `json:"id,omitempty"`
	CleanSession    bool   `json:"clean_session,omitempty"`
	RetainPublishes bool   `json:"retain,omitempty"`
	QoS             int    `json:"max_qos,omitempty"`
	KeepAlive       int    `json:"keep_alive,omitempty"`
}

// SwitchStatus represents the status of a switch/relay
type SwitchStatus struct {
	ID          int     `json:"id"`
	Output      bool    `json:"output"`
	APower      float64 `json:"apower,omitempty"` // Active power in Watts
	Voltage     float64 `json:"voltage,omitempty"`
	Current     float64 `json:"current,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Source      string  `json:"source,omitempty"` // Source of last command
}

// SwitchConfig represents switch/relay configuration
type SwitchConfig struct {
	ID           int     `json:"id"`
	Name         string  `json:"name,omitempty"`
	InMode       string  `json:"in_mode,omitempty"`       // "momentary", "follow", "flip", "detached"
	InitialState string  `json:"initial_state,omitempty"` // "on", "off", "restore_last", "match_input"
	AutoOn       int     `json:"auto_on,omitempty"`       // Auto-on delay in seconds
	AutoOff      int     `json:"auto_off,omitempty"`      // Auto-off delay in seconds
	PowerLimit   int     `json:"power_limit,omitempty"`   // Power limit in Watts
	VoltageLimit int     `json:"voltage_limit,omitempty"` // Voltage limit
	CurrentLimit float64 `json:"current_limit,omitempty"` // Current limit in Amps
}

// LightStatus represents the status of a light
type LightStatus struct {
	ID         int     `json:"id"`
	Output     bool    `json:"output"`
	Brightness int     `json:"brightness,omitempty"` // 0-100
	ColorTemp  int     `json:"temp,omitempty"`       // Color temperature in Kelvin
	Red        int     `json:"red,omitempty"`        // 0-255
	Green      int     `json:"green,omitempty"`      // 0-255
	Blue       int     `json:"blue,omitempty"`       // 0-255
	White      int     `json:"white,omitempty"`      // 0-255
	Power      float64 `json:"power,omitempty"`      // Power consumption in Watts
}

// LightConfig represents light configuration
type LightConfig struct {
	ID                  int    `json:"id"`
	Name                string `json:"name,omitempty"`
	InitialState        string `json:"initial_state,omitempty"`      // "on", "off", "restore_last"
	DefaultBrightness   int    `json:"default.brightness,omitempty"` // Default brightness 0-100
	AutoOn              int    `json:"auto_on,omitempty"`            // Auto-on delay in seconds
	AutoOff             int    `json:"auto_off,omitempty"`           // Auto-off delay in seconds
	NightModeEnable     bool   `json:"night_mode.enable,omitempty"`
	NightModeBrightness int    `json:"night_mode.brightness,omitempty"`
	NightModeStart      string `json:"night_mode.active_between.0,omitempty"` // "HH:MM"
	NightModeEnd        string `json:"night_mode.active_between.1,omitempty"` // "HH:MM"
}

// InputStatus represents the status of an input
type InputStatus struct {
	ID    int    `json:"id"`
	State bool   `json:"state"`
	Event string `json:"event,omitempty"` // "S" (short), "L" (long), "SS" (double), "SSS" (triple)
}

// InputConfig represents input configuration
type InputConfig struct {
	ID     int    `json:"id"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"` // "switch" or "button"
	Invert bool   `json:"invert,omitempty"`
}

// RollerStatus represents the status of a roller/shutter
type RollerStatus struct {
	ID              int     `json:"id"`
	State           string  `json:"state"`           // "open", "close", "stop"
	CurrentPosition int     `json:"current_pos"`     // 0-100
	Power           float64 `json:"power,omitempty"` // Power consumption in Watts
	Temperature     float64 `json:"temperature,omitempty"`
}

// RollerConfig represents roller/shutter configuration
type RollerConfig struct {
	ID                int    `json:"id"`
	Name              string `json:"name,omitempty"`
	MotorIdleTime     int    `json:"motor_idle_time,omitempty"` // ms
	MaxTime           int    `json:"maxtime,omitempty"`         // Max time for full open/close in seconds
	DefaultState      string `json:"default_state,omitempty"`   // "open", "close", "stop"
	InputMode         string `json:"input_mode,omitempty"`      // "openclose" or "onebutton"
	SwapInputs        bool   `json:"swap,omitempty"`
	SwapOutputs       bool   `json:"swap_outputs,omitempty"`
	ObstructionDetect bool   `json:"obstruction_detect,omitempty"`
	SafetySwitch      bool   `json:"safety_switch,omitempty"`
}

// MeterStatus represents power meter readings
type MeterStatus struct {
	ID            int     `json:"id"`
	Power         float64 `json:"power"` // Current power in Watts
	IsValid       bool    `json:"is_valid"`
	Total         float64 `json:"total,omitempty"`          // Total energy in Watt-hours
	TotalReturned float64 `json:"total_returned,omitempty"` // Total returned energy in Watt-hours
}

// UpdateInfo represents firmware update information
type UpdateInfo struct {
	HasUpdate    bool   `json:"has_update"`
	NewVersion   string `json:"new_version,omitempty"`
	OldVersion   string `json:"old_version,omitempty"`
	ReleaseNotes string `json:"release_notes,omitempty"`
}

// DeviceMetrics represents device performance metrics
type DeviceMetrics struct {
	Timestamp    time.Time `json:"timestamp"`
	Uptime       int       `json:"uptime"`
	RAMUsage     float64   `json:"ram_usage"` // Percentage
	FSUsage      float64   `json:"fs_usage"`  // Percentage
	Temperature  float64   `json:"temperature"`
	WiFiRSSI     int       `json:"wifi_rssi"`
	ResponseTime int64     `json:"response_time_ms"` // API response time in milliseconds
}

// EnergyData represents energy consumption data
type EnergyData struct {
	Timestamp     time.Time `json:"timestamp"`
	Power         float64   `json:"power"`          // Current power in Watts
	Total         float64   `json:"total"`          // Total energy in kWh
	TotalReturned float64   `json:"total_returned"` // Total returned energy in kWh
	Voltage       float64   `json:"voltage"`
	Current       float64   `json:"current"`
	PowerFactor   float64   `json:"pf,omitempty"` // Power factor
}

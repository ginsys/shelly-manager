package configuration

import "time"

// Core capability interfaces that devices can implement

// HasRelay indicates a device has relay switching capability
type HasRelay interface {
	GetRelayConfig() *RelayConfig
	SetRelayConfig(*RelayConfig)
}

// HasPowerMetering indicates a device can measure power consumption
type HasPowerMetering interface {
	GetPowerMeteringConfig() *PowerMeteringConfig
	SetPowerMeteringConfig(*PowerMeteringConfig)
}

// HasDimming indicates a device supports brightness control
type HasDimming interface {
	GetDimmingConfig() *DimmingConfig
	SetDimmingConfig(*DimmingConfig)
}

// HasRoller indicates a device can control roller shutters/blinds
type HasRoller interface {
	GetRollerConfig() *RollerConfig
	SetRollerConfig(*RollerConfig)
}

// HasInput indicates a device has input sensing capabilities
type HasInput interface {
	GetInputConfig() *InputConfig
	SetInputConfig(*InputConfig)
}

// HasLED indicates a device has configurable LED indicators
type HasLED interface {
	GetLEDConfig() *LEDConfig
	SetLEDConfig(*LEDConfig)
}

// HasColorControl indicates a device supports RGB/W color control
type HasColorControl interface {
	GetColorConfig() *ColorConfig
	SetColorConfig(*ColorConfig)
}

// HasTemperatureProtection indicates a device has temperature monitoring
type HasTemperatureProtection interface {
	GetTempProtectionConfig() *TempProtectionConfig
	SetTempProtectionConfig(*TempProtectionConfig)
}

// HasSchedule indicates a device supports scheduled operations
type HasSchedule interface {
	GetScheduleConfig() *ScheduleConfig
	SetScheduleConfig(*ScheduleConfig)
}

// HasCoIoT indicates a device supports CoIoT protocol
type HasCoIoT interface {
	GetCoIoTConfig() *CoIoTConfig
	SetCoIoTConfig(*CoIoTConfig)
}

// HasEnergyMeter indicates a device has energy consumption tracking
type HasEnergyMeter interface {
	GetEnergyMeterConfig() *EnergyMeterConfig
	SetEnergyMeterConfig(*EnergyMeterConfig)
}

// HasMotion indicates a device has motion detection capability
type HasMotion interface {
	GetMotionConfig() *MotionConfig
	SetMotionConfig(*MotionConfig)
}

// HasSensor indicates a device has environmental sensors
type HasSensor interface {
	GetSensorConfig() *SensorConfig
	SetSensorConfig(*SensorConfig)
}

// DeviceCapabilities combines all capability queries
type DeviceCapabilities interface {
	// Base identification
	GetDeviceType() string
	GetGeneration() int
	GetModel() string
	
	// Capability checks
	HasRelayCapability() bool
	HasPowerMeteringCapability() bool
	HasDimmingCapability() bool
	HasRollerCapability() bool
	HasInputCapability() bool
	HasLEDCapability() bool
	HasColorControlCapability() bool
	HasTemperatureProtectionCapability() bool
	HasScheduleCapability() bool
	HasCoIoTCapability() bool
	HasEnergyMeterCapability() bool
	HasMotionCapability() bool
	HasSensorCapability() bool
	
	// Get number of components
	GetRelayCount() int
	GetInputCount() int
	GetRollerCount() int
}

// ConfigurationProvider provides access to all device configurations
type ConfigurationProvider interface {
	DeviceCapabilities
	
	// Get base configuration
	GetBaseConfig() BaseDeviceConfig
	
	// Get all active capabilities as a list
	GetActiveCapabilities() []string
	
	// Serialize to JSON for storage
	MarshalConfig() ([]byte, error)
	
	// Deserialize from JSON
	UnmarshalConfig([]byte) error
	
	// Apply template configuration
	ApplyTemplate(template *ConfigTemplate) error
	
	// Calculate diff with another configuration
	DiffWith(other ConfigurationProvider) ([]ConfigDifference, error)
	
	// Validate configuration
	Validate() error
}

// BaseDeviceConfig contains common configuration for all devices
type BaseDeviceConfig struct {
	// Device identification
	DeviceID   string    `json:"device_id"`
	DeviceType string    `json:"device_type"`
	Name       string    `json:"name"`
	Generation int       `json:"generation"`
	
	// Network configuration
	WiFi WiFiConfig `json:"wifi"`
	
	// Authentication
	Auth AuthConfig `json:"auth"`
	
	// Cloud connectivity
	Cloud CloudConfig `json:"cloud"`
	
	// MQTT settings
	MQTT MQTTConfig `json:"mqtt"`
	
	// System settings
	Timezone   string    `json:"timezone"`
	Location   *Location `json:"location,omitempty"`
	
	// Metadata
	LastModified time.Time `json:"last_modified"`
	Version      int       `json:"version"`
}

// WiFiConfig represents WiFi configuration
type WiFiConfig struct {
	SSID     string `json:"ssid"`
	Password string `json:"password,omitempty"`
	IP       string `json:"ip,omitempty"`
	Netmask  string `json:"netmask,omitempty"`
	Gateway  string `json:"gateway,omitempty"`
	DNS      string `json:"dns,omitempty"`
	DHCP     bool   `json:"dhcp"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Realm    string `json:"realm,omitempty"` // For Gen2+ digest auth
}

// CloudConfig represents cloud connectivity configuration
type CloudConfig struct {
	Enabled bool   `json:"enabled"`
	Server  string `json:"server,omitempty"`
}

// MQTTConfig represents MQTT configuration
type MQTTConfig struct {
	Enabled        bool   `json:"enabled"`
	Server         string `json:"server"`
	Port           int    `json:"port"`
	User           string `json:"user,omitempty"`
	Password       string `json:"password,omitempty"`
	ClientID       string `json:"client_id"`
	TopicPrefix    string `json:"topic_prefix"`
	CleanSession   bool   `json:"clean_session"`
	RetainMessages bool   `json:"retain"`
	QoS            int    `json:"qos"`
	KeepAlive      int    `json:"keepalive"`
}

// Location represents geographic location
type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}
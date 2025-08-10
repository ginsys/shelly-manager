package configuration

import (
	"encoding/json"
	"time"
)

// ConfigLevel represents the configuration hierarchy level
type ConfigLevel int

const (
	SystemLevel ConfigLevel = iota
	TemplateLevel
	DeviceLevel
)

// ConfigTemplate represents a reusable configuration template
type ConfigTemplate struct {
	ID          uint            `json:"id" gorm:"primaryKey"`
	Name        string          `json:"name" gorm:"uniqueIndex;not null"`
	Description string          `json:"description"`
	DeviceType  string          `json:"device_type"` // e.g., "SHSW-1", "SHPLG-S", or "all"
	Generation  int             `json:"generation"`  // 1 for Gen1, 2 for Gen2+, 0 for all
	Config      json.RawMessage `json:"config" gorm:"type:text"`
	Variables   json.RawMessage `json:"variables" gorm:"type:text"` // Variable definitions for template
	IsDefault   bool            `json:"is_default"`                  // Default template for device type
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// DeviceConfig represents a device-specific configuration
type DeviceConfig struct {
	ID         uint            `json:"id" gorm:"primaryKey"`
	DeviceID   uint            `json:"device_id" gorm:"index;not null"`
	TemplateID *uint           `json:"template_id" gorm:"index"` // Optional template reference
	Config     json.RawMessage `json:"config" gorm:"type:text"`
	LastSynced *time.Time      `json:"last_synced"`
	SyncStatus string          `json:"sync_status"` // "synced", "pending", "error", "drift"
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// ConfigHistory tracks configuration changes
type ConfigHistory struct {
	ID        uint            `json:"id" gorm:"primaryKey"`
	DeviceID  uint            `json:"device_id" gorm:"index;not null"`
	ConfigID  uint            `json:"config_id" gorm:"index;not null"`
	Action    string          `json:"action"`     // "import", "export", "sync", "manual"
	OldConfig json.RawMessage `json:"old_config" gorm:"type:text"`
	NewConfig json.RawMessage `json:"new_config" gorm:"type:text"`
	Changes   json.RawMessage `json:"changes" gorm:"type:text"` // Diff between old and new
	ChangedBy string          `json:"changed_by"`                // User or system
	CreatedAt time.Time       `json:"created_at"`
}

// ConfigDrift represents detected configuration differences
type ConfigDrift struct {
	DeviceID       uint              `json:"device_id"`
	DeviceName     string            `json:"device_name"`
	LastSynced     *time.Time        `json:"last_synced"`
	DriftDetected  time.Time         `json:"drift_detected"`
	Differences    []ConfigDifference `json:"differences"`
	RequiresAction bool              `json:"requires_action"`
}

// ConfigDifference represents a single configuration difference
type ConfigDifference struct {
	Path     string      `json:"path"`     // JSON path to the difference
	Expected interface{} `json:"expected"` // Value in database
	Actual   interface{} `json:"actual"`   // Value on device
	Type     string      `json:"type"`     // "added", "removed", "modified"
}

// Gen1Config represents Gen1 device configuration
type Gen1Config struct {
	// Network settings
	WiFi struct {
		SSID     string `json:"ssid"`
		Password string `json:"pass,omitempty"`
		IP       string `json:"ip,omitempty"`
		Netmask  string `json:"netmask,omitempty"`
		Gateway  string `json:"gw,omitempty"`
		DNS      string `json:"dns,omitempty"`
	} `json:"wifi_sta"`

	// Authentication
	Auth struct {
		Enabled  bool   `json:"enabled"`
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
	} `json:"login"`

	// Cloud settings
	Cloud struct {
		Enabled bool   `json:"enabled"`
		Server  string `json:"server,omitempty"`
	} `json:"cloud"`

	// MQTT settings
	MQTT struct {
		Enable         bool   `json:"enable"`
		Server         string `json:"server"`
		User           string `json:"user,omitempty"`
		Password       string `json:"pass,omitempty"`
		ID             string `json:"id"`
		CleanSession   bool   `json:"clean_session"`
		RetainMessages bool   `json:"retain"`
		QoS            int    `json:"qos"`
		KeepAlive      int    `json:"keepalive"`
	} `json:"mqtt"`

	// Device name
	Name string `json:"name"`

	// Timezone
	Timezone string `json:"timezone"`

	// Device-specific settings (varies by device type)
	DeviceSettings json.RawMessage `json:"device,omitempty"`
}

// Gen2Config represents Gen2+ device configuration
type Gen2Config struct {
	// System configuration
	Sys struct {
		Device struct {
			Name string `json:"name"`
			MAC  string `json:"mac,omitempty"`
		} `json:"device"`
		Location struct {
			Timezone string  `json:"tz"`
			Lat      float64 `json:"lat,omitempty"`
			Lon      float64 `json:"lon,omitempty"`
		} `json:"location"`
		Debug struct {
			Level   string `json:"level"`
			FileLog bool   `json:"file_log"`
		} `json:"debug"`
	} `json:"sys"`

	// WiFi configuration
	WiFi struct {
		AP struct {
			SSID   string `json:"ssid"`
			Pass   string `json:"pass,omitempty"`
			Enable bool   `json:"enable"`
		} `json:"ap"`
		STA struct {
			SSID     string `json:"ssid"`
			Pass     string `json:"pass,omitempty"`
			Enable   bool   `json:"enable"`
			IP       string `json:"ip,omitempty"`
			Netmask  string `json:"netmask,omitempty"`
			Gateway  string `json:"gw,omitempty"`
			NameServer string `json:"nameserver,omitempty"`
		} `json:"sta"`
	} `json:"wifi"`

	// Cloud configuration
	Cloud struct {
		Enable bool   `json:"enable"`
		Server string `json:"server,omitempty"`
	} `json:"cloud"`

	// MQTT configuration
	MQTT struct {
		Enable   bool   `json:"enable"`
		Server   string `json:"server"`
		User     string `json:"user,omitempty"`
		Pass     string `json:"pass,omitempty"`
		ClientID string `json:"client_id"`
		Topic    string `json:"topic_prefix"`
	} `json:"mqtt"`

	// Component configurations (switches, lights, etc.)
	Components json.RawMessage `json:"components,omitempty"`
}

// TemplateVariable represents a variable in a template
type TemplateVariable struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Type         string      `json:"type"` // "string", "number", "boolean"
	DefaultValue interface{} `json:"default_value"`
	Required     bool        `json:"required"`
}

// BulkConfigOperation represents a bulk configuration operation
type BulkConfigOperation struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DeviceIDs   []uint    `json:"device_ids" gorm:"-"`
	TemplateID  *uint     `json:"template_id"`
	Config      json.RawMessage `json:"config" gorm:"type:text"`
	Status      string    `json:"status"` // "pending", "running", "completed", "failed"
	Progress    int       `json:"progress"` // Percentage
	Results     json.RawMessage `json:"results" gorm:"type:text"` // Per-device results
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
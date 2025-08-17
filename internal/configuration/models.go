package configuration

import (
	"encoding/json"
	"time"
)

// Device represents device information for configuration management
type Device struct {
	ID       uint      `json:"id"`
	MAC      string    `json:"mac"`
	IP       string    `json:"ip"`
	Type     string    `json:"type"`
	Name     string    `json:"name"`
	Settings string    `json:"settings"`
	LastSeen time.Time `json:"last_seen"`
}

// TableName returns the table name for GORM
func (Device) TableName() string {
	return "devices"
}

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
	IsDefault   bool            `json:"is_default"`                 // Default template for device type
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
	Action    string          `json:"action"` // "import", "export", "sync", "manual"
	OldConfig json.RawMessage `json:"old_config" gorm:"type:text"`
	NewConfig json.RawMessage `json:"new_config" gorm:"type:text"`
	Changes   json.RawMessage `json:"changes" gorm:"type:text"` // Diff between old and new
	ChangedBy string          `json:"changed_by"`               // User or system
	CreatedAt time.Time       `json:"created_at"`
}

// ConfigDrift represents detected configuration differences
type ConfigDrift struct {
	DeviceID       uint               `json:"device_id"`
	DeviceName     string             `json:"device_name"`
	LastSynced     *time.Time         `json:"last_synced"`
	DriftDetected  time.Time          `json:"drift_detected"`
	Differences    []ConfigDifference `json:"differences"`
	RequiresAction bool               `json:"requires_action"`
}

// ConfigDifference represents a single configuration difference
type ConfigDifference struct {
	Path        string      `json:"path"`        // JSON path to the difference
	Expected    interface{} `json:"expected"`    // Value in database
	Actual      interface{} `json:"actual"`      // Value on device
	Type        string      `json:"type"`        // "added", "removed", "modified"
	Severity    string      `json:"severity"`    // "critical", "warning", "info"
	Category    string      `json:"category"`    // "security", "network", "device", "system", "metadata"
	Description string      `json:"description"` // Human-readable description
	Impact      string      `json:"impact"`      // Potential impact of this change
	Suggestion  string      `json:"suggestion"`  // Recommended action
}

// ImportStatus represents the import status for a device
type ImportStatus struct {
	DeviceID   uint       `json:"device_id"`
	ConfigID   uint       `json:"config_id,omitempty"`
	Status     string     `json:"status"`  // "not_imported", "synced", "pending", "drift", "error"
	Message    string     `json:"message"` // Human-readable status message
	LastSynced *time.Time `json:"last_synced,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at,omitempty"`
}

// ImportResult represents the result of importing a single device
type ImportResult struct {
	DeviceID uint   `json:"device_id"`
	ConfigID uint   `json:"config_id,omitempty"`
	Status   string `json:"status"` // "success" or "error"
	Error    string `json:"error,omitempty"`
}

// BulkImportResult represents the result of a bulk import operation
type BulkImportResult struct {
	Total   int            `json:"total"`
	Success int            `json:"success"`
	Failed  int            `json:"failed"`
	Results []ImportResult `json:"results"`
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
			SSID       string `json:"ssid"`
			Pass       string `json:"pass,omitempty"`
			Enable     bool   `json:"enable"`
			IP         string `json:"ip,omitempty"`
			Netmask    string `json:"netmask,omitempty"`
			Gateway    string `json:"gw,omitempty"`
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
	ID          uint            `json:"id" gorm:"primaryKey"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	DeviceIDs   []uint          `json:"device_ids" gorm:"-"`
	TemplateID  *uint           `json:"template_id"`
	Config      json.RawMessage `json:"config" gorm:"type:text"`
	Status      string          `json:"status"`                   // "pending", "running", "completed", "failed"
	Progress    int             `json:"progress"`                 // Percentage
	Results     json.RawMessage `json:"results" gorm:"type:text"` // Per-device results
	StartedAt   *time.Time      `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// BulkDriftResult represents the results of bulk drift detection
type BulkDriftResult struct {
	Total       int           `json:"total"`
	InSync      int           `json:"in_sync"`
	Drifted     int           `json:"drifted"`
	Errors      int           `json:"errors"`
	Results     []DriftResult `json:"results"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
}

// DriftResult represents the drift detection result for a single device
type DriftResult struct {
	DeviceID        uint         `json:"device_id"`
	DeviceName      string       `json:"device_name"`
	DeviceIP        string       `json:"device_ip"`
	Status          string       `json:"status"` // "synced", "drift", "error"
	Error           string       `json:"error,omitempty"`
	DriftSummary    string       `json:"drift_summary,omitempty"`
	DifferenceCount int          `json:"difference_count,omitempty"`
	Drift           *ConfigDrift `json:"drift,omitempty"`
}

// DriftDetectionSchedule represents an automated drift detection schedule
type DriftDetectionSchedule struct {
	ID           uint            `json:"id" gorm:"primaryKey"`
	Name         string          `json:"name" gorm:"not null"`
	Description  string          `json:"description"`
	Enabled      bool            `json:"enabled" gorm:"default:true"`
	CronSpec     string          `json:"cron_spec" gorm:"not null"`      // Cron expression (e.g., "0 */6 * * *" for every 6 hours)
	DeviceIDs    []uint          `json:"device_ids" gorm:"-"`            // Device IDs to check (empty = all devices)
	DeviceFilter json.RawMessage `json:"device_filter" gorm:"type:text"` // JSON filter criteria
	LastRun      *time.Time      `json:"last_run"`
	NextRun      *time.Time      `json:"next_run"`
	RunCount     int             `json:"run_count" gorm:"default:0"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// DriftDetectionRun represents a single execution of a drift detection schedule
type DriftDetectionRun struct {
	ID          uint            `json:"id" gorm:"primaryKey"`
	ScheduleID  uint            `json:"schedule_id" gorm:"index;not null"`
	Status      string          `json:"status"` // "running", "completed", "failed"
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at"`
	Duration    *time.Duration  `json:"duration"`
	Results     json.RawMessage `json:"results" gorm:"type:text"` // BulkDriftResult JSON
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// DriftReport represents a comprehensive drift analysis report
type DriftReport struct {
	ID                  uint                  `json:"id" gorm:"primaryKey"`
	ReportType          string                `json:"report_type"` // "device", "bulk", "scheduled"
	DeviceID            *uint                 `json:"device_id,omitempty" gorm:"index"`
	ScheduleID          *uint                 `json:"schedule_id,omitempty" gorm:"index"`
	Summary             DriftSummary          `json:"summary" gorm:"embedded"`
	Devices             []DeviceDriftAnalysis `json:"devices" gorm:"-"`
	DevicesJSON         json.RawMessage       `json:"-" gorm:"column:devices;type:text"`
	Recommendations     []DriftRecommendation `json:"recommendations" gorm:"-"`
	RecommendationsJSON json.RawMessage       `json:"-" gorm:"column:recommendations;type:text"`
	GeneratedAt         time.Time             `json:"generated_at"`
	CreatedAt           time.Time             `json:"created_at"`
}

// DriftSummary provides high-level drift statistics
type DriftSummary struct {
	TotalDevices           int             `json:"total_devices"`
	DevicesInSync          int             `json:"devices_in_sync"`
	DevicesDrifted         int             `json:"devices_drifted"`
	DevicesErrored         int             `json:"devices_errored"`
	CriticalIssues         int             `json:"critical_issues"`
	WarningIssues          int             `json:"warning_issues"`
	InfoIssues             int             `json:"info_issues"`
	CategoriesAffected     map[string]int  `json:"categories_affected" gorm:"-"`
	CategoriesAffectedJSON json.RawMessage `json:"-" gorm:"column:categories_affected;type:text"`
	MostCommonDrifts       []CommonDrift   `json:"most_common_drifts" gorm:"-"`
	MostCommonDriftsJSON   json.RawMessage `json:"-" gorm:"column:most_common_drifts;type:text"`
	SecurityConcerns       int             `json:"security_concerns"`
	NetworkChanges         int             `json:"network_changes"`
}

// DeviceDriftAnalysis provides detailed analysis for a single device
type DeviceDriftAnalysis struct {
	DeviceID          uint               `json:"device_id"`
	DeviceName        string             `json:"device_name"`
	DeviceIP          string             `json:"device_ip"`
	DeviceType        string             `json:"device_type"`
	Generation        int                `json:"generation"`
	Status            string             `json:"status"`         // "synced", "drift", "error"
	DriftSeverity     string             `json:"drift_severity"` // "none", "low", "medium", "high", "critical"
	TotalDifferences  int                `json:"total_differences"`
	CriticalCount     int                `json:"critical_count"`
	WarningCount      int                `json:"warning_count"`
	InfoCount         int                `json:"info_count"`
	LastSyncTime      *time.Time         `json:"last_sync_time"`
	DriftDetectedTime time.Time          `json:"drift_detected_time"`
	Differences       []ConfigDifference `json:"differences"`
	HealthScore       float64            `json:"health_score"` // 0-100 based on drift severity
	RiskLevel         string             `json:"risk_level"`   // "low", "medium", "high", "critical"
	Error             string             `json:"error,omitempty"`
}

// CommonDrift represents frequently occurring drift patterns
type CommonDrift struct {
	Path        string  `json:"path"`
	Count       int     `json:"count"`
	Percentage  float64 `json:"percentage"`
	Severity    string  `json:"severity"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
}

// DriftRecommendation provides actionable recommendations
type DriftRecommendation struct {
	Priority        string              `json:"priority"`         // "high", "medium", "low"
	Category        string              `json:"category"`         // "security", "network", "maintenance", "compliance"
	Title           string              `json:"title"`            // Short recommendation title
	Description     string              `json:"description"`      // Detailed recommendation
	AffectedDevices []uint              `json:"affected_devices"` // Device IDs affected
	Actions         []RecommendedAction `json:"actions"`          // Specific actions to take
	Impact          string              `json:"impact"`           // Expected impact of following recommendation
}

// RecommendedAction represents a specific action to take
type RecommendedAction struct {
	Type        string `json:"type"`              // "auto-fix", "manual-review", "manual-action", "monitor"
	Description string `json:"description"`       // What to do
	Command     string `json:"command,omitempty"` // Specific command/config to apply
	Automated   bool   `json:"automated"`         // Whether this can be automated
}

// DriftTrend tracks drift patterns over time
type DriftTrend struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	DeviceID    uint       `json:"device_id" gorm:"index"`
	Path        string     `json:"path" gorm:"index"`
	Category    string     `json:"category"`
	Severity    string     `json:"severity"`
	FirstSeen   time.Time  `json:"first_seen"`
	LastSeen    time.Time  `json:"last_seen"`
	Occurrences int        `json:"occurrences"`
	Resolved    bool       `json:"resolved"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

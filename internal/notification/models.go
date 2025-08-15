package notification

import (
	"encoding/json"
	"time"
)

// NotificationChannel represents a configured notification destination
type NotificationChannel struct {
	ID          uint            `json:"id" gorm:"primaryKey"`
	Name        string          `json:"name" gorm:"uniqueIndex;not null"`
	Type        string          `json:"type" gorm:"not null"` // "email", "webhook", "slack", "discord"
	Enabled     bool            `json:"enabled" gorm:"default:true"`
	Config      json.RawMessage `json:"config" gorm:"type:text"` // Channel-specific configuration
	Description string          `json:"description"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject,omitempty"`
	Template   string   `json:"template,omitempty"`
}

// WebhookConfig represents webhook notification configuration
type WebhookConfig struct {
	URL      string            `json:"url"`
	Method   string            `json:"method,omitempty"` // Default: POST
	Headers  map[string]string `json:"headers,omitempty"`
	Secret   string            `json:"secret,omitempty"`
	Template string            `json:"template,omitempty"`
	Timeout  int               `json:"timeout,omitempty"` // Seconds, default: 30
	Retries  int               `json:"retries,omitempty"` // Default: 3
}

// SlackConfig represents Slack notification configuration
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel,omitempty"`
	Username   string `json:"username,omitempty"`
	IconEmoji  string `json:"icon_emoji,omitempty"`
	Template   string `json:"template,omitempty"`
}

// NotificationRule represents when and how to send notifications
type NotificationRule struct {
	ID          uint                `json:"id" gorm:"primaryKey"`
	Name        string              `json:"name" gorm:"uniqueIndex;not null"`
	Description string              `json:"description"`
	Enabled     bool                `json:"enabled" gorm:"default:true"`
	ChannelID   uint                `json:"channel_id" gorm:"not null"`
	Channel     NotificationChannel `json:"channel" gorm:"foreignKey:ChannelID"`

	// Trigger conditions
	AlertLevel     string          `json:"alert_level"`         // "critical", "warning", "info", "all"
	MinSeverity    string          `json:"min_severity"`        // "critical", "warning", "info"
	Categories     []string        `json:"categories" gorm:"-"` // "security", "network", "device", etc.
	CategoriesJSON json.RawMessage `json:"-" gorm:"column:categories;type:text"`
	DeviceFilter   json.RawMessage `json:"device_filter" gorm:"type:text"` // Device filtering criteria

	// Rate limiting
	MinIntervalMinutes int `json:"min_interval_minutes"` // Minimum time between notifications
	MaxPerHour         int `json:"max_per_hour"`         // Maximum notifications per hour

	// Scheduling
	ScheduleEnabled  bool            `json:"schedule_enabled"`
	ScheduleStart    string          `json:"schedule_start,omitempty"` // HH:MM format
	ScheduleEnd      string          `json:"schedule_end,omitempty"`   // HH:MM format
	ScheduleDays     []string        `json:"schedule_days" gorm:"-"`   // ["monday", "tuesday", ...]
	ScheduleDaysJSON json.RawMessage `json:"-" gorm:"column:schedule_days;type:text"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotificationHistory tracks sent notifications
type NotificationHistory struct {
	ID        uint                `json:"id" gorm:"primaryKey"`
	RuleID    uint                `json:"rule_id" gorm:"index;not null"`
	Rule      NotificationRule    `json:"rule" gorm:"foreignKey:RuleID"`
	ChannelID uint                `json:"channel_id" gorm:"index;not null"`
	Channel   NotificationChannel `json:"channel" gorm:"foreignKey:ChannelID"`

	// Context
	TriggerType string `json:"trigger_type"` // "drift_detected", "schedule_run", "manual"
	DeviceID    *uint  `json:"device_id,omitempty" gorm:"index"`
	ScheduleID  *uint  `json:"schedule_id,omitempty" gorm:"index"`
	ReportID    *uint  `json:"report_id,omitempty" gorm:"index"`

	// Content
	Subject             string          `json:"subject"`
	Message             string          `json:"message"`
	AlertLevel          string          `json:"alert_level"`
	AffectedDevices     []uint          `json:"affected_devices" gorm:"-"`
	AffectedDevicesJSON json.RawMessage `json:"-" gorm:"column:affected_devices;type:text"`

	// Delivery
	Status      string     `json:"status"` // "pending", "sent", "failed", "retry"
	SentAt      *time.Time `json:"sent_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	RetryCount  int        `json:"retry_count" gorm:"default:0"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertLevel represents notification severity levels
type AlertLevel string

const (
	AlertLevelCritical AlertLevel = "critical"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelInfo     AlertLevel = "info"
)

// NotificationTemplate represents message templates
type NotificationTemplate struct {
	ID            uint            `json:"id" gorm:"primaryKey"`
	Name          string          `json:"name" gorm:"uniqueIndex;not null"`
	Type          string          `json:"type"`    // "email", "webhook", "slack"
	Subject       string          `json:"subject"` // For email
	Body          string          `json:"body" gorm:"type:text"`
	Variables     []string        `json:"variables" gorm:"-"`
	VariablesJSON json.RawMessage `json:"-" gorm:"column:variables;type:text"`
	Description   string          `json:"description"`
	IsDefault     bool            `json:"is_default"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// NotificationEvent represents events that can trigger notifications
type NotificationEvent struct {
	Type            string                 `json:"type"`
	AlertLevel      AlertLevel             `json:"alert_level"`
	DeviceID        *uint                  `json:"device_id,omitempty"`
	DeviceName      string                 `json:"device_name,omitempty"`
	Title           string                 `json:"title"`
	Message         string                 `json:"message"`
	Timestamp       time.Time              `json:"timestamp"`
	AffectedDevices []uint                 `json:"affected_devices,omitempty"`
	Categories      []string               `json:"categories,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// DeviceFilter represents device filtering criteria
type DeviceFilter struct {
	DeviceIDs   []uint   `json:"device_ids,omitempty"`   // Specific device IDs
	DeviceTypes []string `json:"device_types,omitempty"` // Device types (SHSW-1, etc.)
	DeviceNames []string `json:"device_names,omitempty"` // Device name patterns
	Generations []int    `json:"generations,omitempty"`  // Gen1, Gen2+
	IPRanges    []string `json:"ip_ranges,omitempty"`    // CIDR notation
	Exclude     bool     `json:"exclude"`                // If true, exclude matching devices
}

// RateLimitState tracks rate limiting per rule
type RateLimitState struct {
	RuleID        uint      `json:"rule_id"`
	LastSentAt    time.Time `json:"last_sent_at"`
	HourlyCount   int       `json:"hourly_count"`
	HourlyResetAt time.Time `json:"hourly_reset_at"`
}

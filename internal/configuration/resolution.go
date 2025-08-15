package configuration

import (
	"encoding/json"
	"time"
)

// ResolutionPolicy represents automated resolution policies
type ResolutionPolicy struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"uniqueIndex;not null"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled" gorm:"default:true"`

	// Policy settings
	AutoFixEnabled   bool `json:"auto_fix_enabled"`
	SafeMode         bool `json:"safe_mode" gorm:"default:true"`
	ApprovalRequired bool `json:"approval_required"`

	// Scope filters
	Categories     []string        `json:"categories" gorm:"-"`
	CategoriesJSON json.RawMessage `json:"-" gorm:"column:categories;type:text"`
	Severities     []string        `json:"severities" gorm:"-"`
	SeveritiesJSON json.RawMessage `json:"-" gorm:"column:severities;type:text"`
	DeviceFilter   json.RawMessage `json:"device_filter" gorm:"type:text"`

	// Auto-fix configuration
	AutoFixCategories     []string        `json:"auto_fix_categories" gorm:"-"`
	AutoFixCategoriesJSON json.RawMessage `json:"-" gorm:"column:auto_fix_categories;type:text"`
	ExcludedPaths         []string        `json:"excluded_paths" gorm:"-"`
	ExcludedPathsJSON     json.RawMessage `json:"-" gorm:"column:excluded_paths;type:text"`

	// Timing
	MaxAge        int `json:"max_age"`        // Days - don't auto-fix old drift
	RetryInterval int `json:"retry_interval"` // Minutes between retry attempts
	MaxRetries    int `json:"max_retries"`    // Maximum retry attempts

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResolutionRequest represents a manual resolution request
type ResolutionRequest struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	DeviceID   uint   `json:"device_id" gorm:"index;not null"`
	DeviceName string `json:"device_name"`
	TrendID    *uint  `json:"trend_id,omitempty" gorm:"index"`
	ReportID   *uint  `json:"report_id,omitempty" gorm:"index"`

	// Request details
	RequestType string `json:"request_type"` // "manual_review", "auto_fix_failed", "policy_conflict"
	Priority    string `json:"priority"`     // "low", "medium", "high", "critical"
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Path        string `json:"path"` // Configuration path

	// Current state
	CurrentValue  json.RawMessage `json:"current_value" gorm:"type:text"`
	ExpectedValue json.RawMessage `json:"expected_value" gorm:"type:text"`
	Description   string          `json:"description"`
	Impact        string          `json:"impact"`

	// Resolution strategy
	Strategy      string          `json:"strategy"` // "restore", "update", "ignore", "custom"
	ProposedValue json.RawMessage `json:"proposed_value" gorm:"type:text"`
	Justification string          `json:"justification"`

	// Workflow status
	Status      string     `json:"status"` // "pending", "approved", "rejected", "completed", "failed"
	AssignedTo  string     `json:"assigned_to,omitempty"`
	ReviewedBy  string     `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	ReviewNotes string     `json:"review_notes"`

	// Scheduling
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Results
	ResolutionResult json.RawMessage `json:"resolution_result" gorm:"type:text"`
	Error            string          `json:"error,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResolutionHistory tracks completed resolutions
type ResolutionHistory struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	DeviceID   uint   `json:"device_id" gorm:"index;not null"`
	DeviceName string `json:"device_name"`
	RequestID  *uint  `json:"request_id,omitempty" gorm:"index"`
	PolicyID   *uint  `json:"policy_id,omitempty" gorm:"index"`

	// Resolution details
	Type     string `json:"type"` // "auto_fix", "manual", "scheduled"
	Category string `json:"category"`
	Severity string `json:"severity"`
	Path     string `json:"path"`

	// Changes made
	OldValue   json.RawMessage `json:"old_value" gorm:"type:text"`
	NewValue   json.RawMessage `json:"new_value" gorm:"type:text"`
	ChangeType string          `json:"change_type"` // "restore", "update", "remove"

	// Execution details
	Method   string `json:"method"` // "config_export", "api_call", "manual"
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	Duration int    `json:"duration"` // Milliseconds

	// Context
	TriggeredBy   string `json:"triggered_by"` // User or system
	Justification string `json:"justification"`

	ExecutedAt time.Time `json:"executed_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// ResolutionStrategy represents different resolution approaches
type ResolutionStrategy string

const (
	StrategyRestore ResolutionStrategy = "restore" // Restore to stored configuration
	StrategyUpdate  ResolutionStrategy = "update"  // Update stored configuration to match device
	StrategyIgnore  ResolutionStrategy = "ignore"  // Mark as resolved without changes
	StrategyCustom  ResolutionStrategy = "custom"  // Apply custom configuration
)

// ResolutionStatus represents the current status of a resolution request
type ResolutionStatus string

const (
	StatusPending   ResolutionStatus = "pending"
	StatusApproved  ResolutionStatus = "approved"
	StatusRejected  ResolutionStatus = "rejected"
	StatusCompleted ResolutionStatus = "completed"
	StatusFailed    ResolutionStatus = "failed"
	StatusScheduled ResolutionStatus = "scheduled"
)

// ResolutionPriority represents the urgency of a resolution
type ResolutionPriority string

const (
	PriorityLow      ResolutionPriority = "low"
	PriorityMedium   ResolutionPriority = "medium"
	PriorityHigh     ResolutionPriority = "high"
	PriorityCritical ResolutionPriority = "critical"
)

// AutoFixResult represents the result of an auto-fix attempt
type AutoFixResult struct {
	DeviceID uint                   `json:"device_id"`
	Path     string                 `json:"path"`
	Success  bool                   `json:"success"`
	Action   string                 `json:"action"` // "restored", "updated", "skipped"
	OldValue interface{}            `json:"old_value,omitempty"`
	NewValue interface{}            `json:"new_value,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration"`
	PolicyID *uint                  `json:"policy_id,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// BatchResolutionRequest represents a request to resolve multiple items
type BatchResolutionRequest struct {
	RequestIDs    []uint             `json:"request_ids"`
	Strategy      ResolutionStrategy `json:"strategy"`
	Justification string             `json:"justification"`
	ScheduledAt   *time.Time         `json:"scheduled_at,omitempty"`
}

// ResolutionSchedule represents scheduled resolution windows
type ResolutionSchedule struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"uniqueIndex;not null"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled" gorm:"default:true"`

	// Schedule configuration
	CronSpec          string `json:"cron_spec"`          // Cron expression
	TimeZone          string `json:"timezone"`           // Timezone for schedule
	MaintenanceWindow bool   `json:"maintenance_window"` // If true, only run during maintenance

	// Resolution settings
	AutoApprove    bool            `json:"auto_approve"`    // Auto-approve during window
	MaxResolutions int             `json:"max_resolutions"` // Max resolutions per window
	Categories     []string        `json:"categories" gorm:"-"`
	CategoriesJSON json.RawMessage `json:"-" gorm:"column:categories;type:text"`

	// Timing
	LastRun *time.Time `json:"last_run,omitempty"`
	NextRun *time.Time `json:"next_run,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResolutionMetrics tracks resolution performance
type ResolutionMetrics struct {
	ID   uint      `json:"id" gorm:"primaryKey"`
	Date time.Time `json:"date" gorm:"index"`

	// Request metrics
	RequestsCreated  int `json:"requests_created"`
	RequestsResolved int `json:"requests_resolved"`
	RequestsPending  int `json:"requests_pending"`

	// Auto-fix metrics
	AutoFixAttempts  int     `json:"auto_fix_attempts"`
	AutoFixSuccesses int     `json:"auto_fix_successes"`
	AutoFixFailures  int     `json:"auto_fix_failures"`
	AutoFixRate      float64 `json:"auto_fix_rate"`

	// Manual review metrics
	ManualReviews int     `json:"manual_reviews"`
	ApprovalRate  float64 `json:"approval_rate"`
	AvgReviewTime int     `json:"avg_review_time"` // Minutes

	// Category breakdown
	SecurityIssues int `json:"security_issues"`
	NetworkIssues  int `json:"network_issues"`
	DeviceIssues   int `json:"device_issues"`
	SystemIssues   int `json:"system_issues"`
	MetadataIssues int `json:"metadata_issues"`

	CreatedAt time.Time `json:"created_at"`
}

package plugins

import (
	"context"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// PluginType defines the type of plugin
type PluginType string

const (
	PluginTypeSync         PluginType = "sync"
	PluginTypeNotification PluginType = "notification"
	PluginTypeDiscovery    PluginType = "discovery"
)

// Plugin defines the base interface that all plugins must implement
type Plugin interface {
	// Core identification and metadata
	Info() PluginInfo
	Type() PluginType

	// Configuration management
	ConfigSchema() ConfigSchema
	ValidateConfig(config map[string]interface{}) error

	// Plugin lifecycle
	Initialize(logger *logging.Logger) error
	Cleanup() error
	Health() HealthStatus
}

// PluginInfo provides metadata about a plugin
type PluginInfo struct {
	Name             string         `json:"name"`
	Version          string         `json:"version"`
	Description      string         `json:"description"`
	Author           string         `json:"author"`
	Website          string         `json:"website,omitempty"`
	License          string         `json:"license"`
	SupportedFormats []string       `json:"supported_formats,omitempty"`
	Tags             []string       `json:"tags"`
	Category         PluginCategory `json:"category"`
	Dependencies     []string       `json:"dependencies,omitempty"`
	MinVersion       string         `json:"min_version,omitempty"`
}

// PluginCategory defines the category of a plugin
type PluginCategory string

const (
	CategoryBackup         PluginCategory = "backup"
	CategoryGitOps         PluginCategory = "gitops"
	CategoryHomeAutomation PluginCategory = "home_automation"
	CategoryNetworking     PluginCategory = "networking"
	CategoryMonitoring     PluginCategory = "monitoring"
	CategoryDocumentation  PluginCategory = "documentation"
	CategoryNotification   PluginCategory = "notification"
	CategoryDiscovery      PluginCategory = "discovery"
	CategoryCustom         PluginCategory = "custom"
)

// ConfigSchema defines the configuration schema for a plugin
type ConfigSchema struct {
	Version    string                    `json:"version"`
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required"`
	Examples   []map[string]interface{}  `json:"examples,omitempty"`
}

// PropertySchema defines a single configuration property
type PropertySchema struct {
	Type        string                    `json:"type"` // "string", "number", "boolean", "array", "object"
	Description string                    `json:"description"`
	Default     interface{}               `json:"default,omitempty"`
	Enum        []interface{}             `json:"enum,omitempty"`
	Pattern     string                    `json:"pattern,omitempty"` // regex for string validation
	Minimum     *float64                  `json:"minimum,omitempty"`
	Maximum     *float64                  `json:"maximum,omitempty"`
	Items       *PropertySchema           `json:"items,omitempty"`      // for arrays
	Properties  map[string]PropertySchema `json:"properties,omitempty"` // for objects
	Sensitive   bool                      `json:"sensitive,omitempty"`  // marks sensitive data like passwords
}

// HealthStatus represents the health status of a plugin
type HealthStatus struct {
	Status      HealthStatusType `json:"status"`
	LastChecked time.Time        `json:"last_checked"`
	Message     string           `json:"message,omitempty"`
	Details     interface{}      `json:"details,omitempty"`
}

// HealthStatusType defines possible health states
type HealthStatusType string

const (
	HealthStatusHealthy     HealthStatusType = "healthy"
	HealthStatusDegraded    HealthStatusType = "degraded"
	HealthStatusUnhealthy   HealthStatusType = "unhealthy"
	HealthStatusUnknown     HealthStatusType = "unknown"
	HealthStatusUnavailable HealthStatusType = "unavailable"
)

// PluginCapabilities describes what a plugin can do
type PluginCapabilities struct {
	SupportsIncremental    bool     `json:"supports_incremental"`
	SupportsScheduling     bool     `json:"supports_scheduling"`
	RequiresAuthentication bool     `json:"requires_authentication"`
	SupportedOutputs       []string `json:"supported_outputs"` // "file", "webhook", "api"
	MaxDataSize            int64    `json:"max_data_size"`
	ConcurrencyLevel       int      `json:"concurrency_level"`
	RequiresNetwork        bool     `json:"requires_network"`
	IsExperimental         bool     `json:"is_experimental"`
}

// PluginError represents an error specific to plugin operations
type PluginError struct {
	PluginName string    `json:"plugin_name"`
	PluginType string    `json:"plugin_type"`
	Operation  string    `json:"operation"`
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Cause      error     `json:"-"`
}

// Error implements the error interface
func (pe *PluginError) Error() string {
	return pe.Message
}

// Unwrap allows for error unwrapping
func (pe *PluginError) Unwrap() error {
	return pe.Cause
}

// NewPluginError creates a new plugin error
func NewPluginError(pluginName, pluginType, operation, code, message string, cause error) *PluginError {
	return &PluginError{
		PluginName: pluginName,
		PluginType: pluginType,
		Operation:  operation,
		Code:       code,
		Message:    message,
		Timestamp:  time.Now(),
		Cause:      cause,
	}
}

// PluginContext provides context information for plugin operations
type PluginContext struct {
	Context    context.Context        `json:"-"`
	RequestID  string                 `json:"request_id"`
	UserID     string                 `json:"user_id,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Logger     *logging.Logger        `json:"-"`
	StartTime  time.Time              `json:"start_time"`
	Timeout    time.Duration          `json:"timeout"`
	CancelFunc context.CancelFunc     `json:"-"`
}

// NewPluginContext creates a new plugin context
func NewPluginContext(ctx context.Context, requestID string, timeout time.Duration) *PluginContext {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	return &PluginContext{
		Context:    ctxWithTimeout,
		RequestID:  requestID,
		StartTime:  time.Now(),
		Timeout:    timeout,
		CancelFunc: cancel,
		Metadata:   make(map[string]interface{}),
	}
}

// Cancel cancels the plugin context
func (pc *PluginContext) Cancel() {
	if pc.CancelFunc != nil {
		pc.CancelFunc()
	}
}

// IsExpired checks if the context has expired
func (pc *PluginContext) IsExpired() bool {
	return time.Since(pc.StartTime) > pc.Timeout
}

// RemainingTime returns the remaining time before context expiration
func (pc *PluginContext) RemainingTime() time.Duration {
	elapsed := time.Since(pc.StartTime)
	if elapsed >= pc.Timeout {
		return 0
	}
	return pc.Timeout - elapsed
}

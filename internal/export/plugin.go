package export

import (
	"context"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// ExportPlugin defines the interface for all export plugins
type ExportPlugin interface {
	// Metadata
	Info() PluginInfo

	// Configuration
	ConfigSchema() ConfigSchema
	ValidateConfig(config map[string]interface{}) error

	// Export Operations
	Export(ctx context.Context, data *ExportData, config ExportConfig) (*ExportResult, error)
	Preview(ctx context.Context, data *ExportData, config ExportConfig) (*PreviewResult, error)

	// Capabilities
	Capabilities() PluginCapabilities

	// Lifecycle
	Initialize(logger *logging.Logger) error
	Cleanup() error
}

// PluginInfo provides metadata about a plugin
type PluginInfo struct {
	Name             string         `json:"name"`
	Version          string         `json:"version"`
	Description      string         `json:"description"`
	Author           string         `json:"author"`
	Website          string         `json:"website,omitempty"`
	License          string         `json:"license"`
	SupportedFormats []string       `json:"supported_formats"`
	Tags             []string       `json:"tags"`
	Category         PluginCategory `json:"category"`
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
	CategoryCustom         PluginCategory = "custom"
)

// PluginCapabilities describes what a plugin can do
type PluginCapabilities struct {
	SupportsIncremental    bool     `json:"supports_incremental"`
	SupportsScheduling     bool     `json:"supports_scheduling"`
	RequiresAuthentication bool     `json:"requires_authentication"`
	SupportedOutputs       []string `json:"supported_outputs"` // "file", "webhook", "api"
	MaxDataSize            int64    `json:"max_data_size"`
	ConcurrencyLevel       int      `json:"concurrency_level"`
}

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

// ExportConfig holds configuration for an export operation
type ExportConfig struct {
	PluginName string                 `json:"plugin_name"`
	Format     string                 `json:"format"`
	Config     map[string]interface{} `json:"config"`
	Filters    ExportFilters          `json:"filters"`
	Output     OutputConfig           `json:"output"`
	Options    ExportOptions          `json:"options"`
}

// ExportFilters defines what data to include in export
type ExportFilters struct {
	DeviceIDs     []uint     `json:"device_ids,omitempty"`
	DeviceTypes   []string   `json:"device_types,omitempty"`
	DeviceStatus  []string   `json:"device_status,omitempty"`
	LastSeenAfter *time.Time `json:"last_seen_after,omitempty"`
	HasConfig     *bool      `json:"has_config,omitempty"`
	TemplateIDs   []uint     `json:"template_ids,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
}

// OutputConfig defines where and how to output the export
type OutputConfig struct {
	Type        string            `json:"type"`                  // "file", "webhook", "response"
	Destination string            `json:"destination,omitempty"` // file path or webhook URL
	Headers     map[string]string `json:"headers,omitempty"`
	Webhook     *WebhookConfig    `json:"webhook,omitempty"`
	Compression string            `json:"compression,omitempty"` // "gzip", "zip", "none"
}

// WebhookConfig defines webhook delivery configuration
type WebhookConfig struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	AuthType   string            `json:"auth_type"` // "none", "bearer", "basic", "api_key"
	AuthConfig map[string]string `json:"auth_config"`
	Timeout    time.Duration     `json:"timeout"`
	Retries    int               `json:"retries"`
}

// ExportOptions provides additional export configuration
type ExportOptions struct {
	DryRun          bool `json:"dry_run"`
	IncludeHistory  bool `json:"include_history"`
	ValidateOnly    bool `json:"validate_only"`
	CompactOutput   bool `json:"compact_output"`
	IncludeMetadata bool `json:"include_metadata"`
}

// ExportResult contains the result of an export operation
type ExportResult struct {
	Success     bool                   `json:"success"`
	ExportID    string                 `json:"export_id"`
	PluginName  string                 `json:"plugin_name"`
	Format      string                 `json:"format"`
	OutputPath  string                 `json:"output_path,omitempty"`
	WebhookSent bool                   `json:"webhook_sent,omitempty"`
	RecordCount int                    `json:"record_count"`
	FileSize    int64                  `json:"file_size"`
	Checksum    string                 `json:"checksum"`
	Duration    time.Duration          `json:"duration"`
	Errors      []string               `json:"errors,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// PreviewResult contains a preview of what would be exported
type PreviewResult struct {
	Success       bool     `json:"success"`
	SampleData    []byte   `json:"sample_data"`
	RecordCount   int      `json:"record_count"`
	EstimatedSize int64    `json:"estimated_size"`
	Warnings      []string `json:"warnings,omitempty"`
}

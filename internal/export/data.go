package export

import (
	"time"
)

// ExportData contains all data that can be exported
type ExportData struct {
	Devices           []DeviceData           `json:"devices"`
	Configurations    []ConfigurationData    `json:"configurations"`
	Templates         []TemplateData         `json:"templates"`
	DiscoveredDevices []DiscoveredDeviceData `json:"discovered_devices,omitempty"`
	Metadata          ExportMetadata         `json:"metadata"`
	Timestamp         time.Time              `json:"timestamp"`
}

// DeviceData represents a device for export
type DeviceData struct {
	ID            uint                   `json:"id"`
	MAC           string                 `json:"mac"`
	IP            string                 `json:"ip"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	Model         string                 `json:"model"`
	Firmware      string                 `json:"firmware"`
	Status        string                 `json:"status"`
	LastSeen      time.Time              `json:"last_seen"`
	Settings      map[string]interface{} `json:"settings"`
	Configuration *ConfigurationData     `json:"configuration,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// ConfigurationData represents a device configuration for export
type ConfigurationData struct {
	DeviceID   uint                   `json:"device_id"`
	TemplateID *uint                  `json:"template_id,omitempty"`
	Config     map[string]interface{} `json:"config"`
	LastSynced *time.Time             `json:"last_synced,omitempty"`
	SyncStatus string                 `json:"sync_status"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// TemplateData represents a configuration template for export
type TemplateData struct {
	ID          uint                   `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	DeviceType  string                 `json:"device_type"`
	Generation  int                    `json:"generation"`
	Config      map[string]interface{} `json:"config"`
	Variables   map[string]interface{} `json:"variables"`
	IsDefault   bool                   `json:"is_default"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// DiscoveredDeviceData represents a discovered device for export
type DiscoveredDeviceData struct {
	MAC        string    `json:"mac"`
	SSID       string    `json:"ssid"`
	Model      string    `json:"model"`
	Generation int       `json:"generation"`
	IP         string    `json:"ip"`
	Signal     int       `json:"signal"`
	AgentID    string    `json:"agent_id"`
	Discovered time.Time `json:"discovered"`
}

// ExportMetadata contains metadata about the export operation
type ExportMetadata struct {
	ExportID       string `json:"export_id"`
	RequestedBy    string `json:"requested_by"`
	ExportType     string `json:"export_type"` // "manual", "scheduled", "api"
	TotalDevices   int    `json:"total_devices"`
	TotalConfigs   int    `json:"total_configs"`
	FilterApplied  bool   `json:"filter_applied"`
	FilterCriteria string `json:"filter_criteria,omitempty"`
	SystemVersion  string `json:"system_version"`
	DatabaseType   string `json:"database_type"`
}

// ExportRequest represents a request to export data
type ExportRequest struct {
	PluginName string                 `json:"plugin_name"`
	Format     string                 `json:"format"`
	Config     map[string]interface{} `json:"config"`
	Filters    ExportFilters          `json:"filters"`
	Output     OutputConfig           `json:"output"`
	Options    ExportOptions          `json:"options"`
}

// ImportRequest represents a request to import data
type ImportRequest struct {
	PluginName string                 `json:"plugin_name"`
	Format     string                 `json:"format"`
	Source     ImportSource           `json:"source"`
	Config     map[string]interface{} `json:"config"`
	Options    ImportOptions          `json:"options"`
}

// ImportSource defines where to import data from
type ImportSource struct {
	Type string `json:"type"` // "file", "url", "data"
	Path string `json:"path,omitempty"`
	URL  string `json:"url,omitempty"`
	Data []byte `json:"data,omitempty"`
}

// ImportOptions provides additional import configuration
type ImportOptions struct {
	DryRun         bool `json:"dry_run"`
	ForceOverwrite bool `json:"force_overwrite"`
	ValidateOnly   bool `json:"validate_only"`
	BackupBefore   bool `json:"backup_before"`
}

// ImportResult contains the result of an import operation
type ImportResult struct {
	Success         bool                   `json:"success"`
	ImportID        string                 `json:"import_id"`
	PluginName      string                 `json:"plugin_name"`
	Format          string                 `json:"format"`
	RecordsImported int                    `json:"records_imported"`
	RecordsSkipped  int                    `json:"records_skipped"`
	Duration        time.Duration          `json:"duration"`
	Changes         []ImportChange         `json:"changes,omitempty"`
	Errors          []string               `json:"errors,omitempty"`
	Warnings        []string               `json:"warnings,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// ImportChange describes a change made during import
type ImportChange struct {
	Type       string      `json:"type"`     // "create", "update", "delete"
	Resource   string      `json:"resource"` // "device", "config", "template"
	ResourceID string      `json:"resource_id"`
	OldValue   interface{} `json:"old_value,omitempty"`
	NewValue   interface{} `json:"new_value"`
	Field      string      `json:"field,omitempty"`
}

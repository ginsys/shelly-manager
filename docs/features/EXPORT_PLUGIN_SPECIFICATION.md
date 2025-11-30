# Export Plugin System Specification

## Overview

The Shelly Manager Export Plugin System provides a flexible, extensible framework for exporting device configurations and data to various external systems and formats. The system supports both built-in exporters and external plugins, with comprehensive validation, scheduling, and audit capabilities.

## Architecture Components

### Core Export Engine

```go
// internal/export/engine.go
type ExportEngine struct {
    plugins    map[string]ExportPlugin
    scheduler  *ExportScheduler
    validator  *ExportValidator
    history    *ExportHistory
    db         *gorm.DB
    logger     *logging.Logger
}

// Core engine methods
func (e *ExportEngine) RegisterPlugin(plugin ExportPlugin) error
func (e *ExportEngine) ListPlugins() []PluginInfo
func (e *ExportEngine) Export(request ExportRequest) (*ExportResult, error)
func (e *ExportEngine) ScheduleExport(schedule ExportSchedule) error
func (e *ExportEngine) ValidateExport(request ExportRequest) (*ValidationResult, error)
```

## Plugin Interface Definition

### Core Plugin Interface

```go
// internal/export/plugin.go
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

type PluginInfo struct {
    Name            string            `json:"name"`
    Version         string            `json:"version"`
    Description     string            `json:"description"`
    Author          string            `json:"author"`
    Website         string            `json:"website,omitempty"`
    License         string            `json:"license"`
    SupportedFormats []string          `json:"supported_formats"`
    Tags            []string          `json:"tags"`
    Category        PluginCategory    `json:"category"`
}

type PluginCategory string

const (
    CategoryHomeAutomation PluginCategory = "home_automation"
    CategoryNetworking     PluginCategory = "networking"
    CategoryMonitoring     PluginCategory = "monitoring"
    CategoryDocumentation  PluginCategory = "documentation"
    CategoryCustom         PluginCategory = "custom"
)

type PluginCapabilities struct {
    SupportsIncremental    bool     `json:"supports_incremental"`
    SupportsScheduling     bool     `json:"supports_scheduling"`
    RequiresAuthentication bool     `json:"requires_authentication"`
    SupportedOutputs       []string `json:"supported_outputs"` // "file", "webhook", "api"
    MaxDataSize            int64    `json:"max_data_size"`
    ConcurrencyLevel       int      `json:"concurrency_level"`
}
```

### Configuration Schema

```go
type ConfigSchema struct {
    Version    string                    `json:"version"`
    Properties map[string]PropertySchema `json:"properties"`
    Required   []string                  `json:"required"`
    Examples   []map[string]interface{}  `json:"examples,omitempty"`
}

type PropertySchema struct {
    Type        string                 `json:"type"` // "string", "number", "boolean", "array", "object"
    Description string                 `json:"description"`
    Default     interface{}            `json:"default,omitempty"`
    Enum        []interface{}          `json:"enum,omitempty"`
    Pattern     string                 `json:"pattern,omitempty"` // regex for string validation
    Minimum     *float64               `json:"minimum,omitempty"`
    Maximum     *float64               `json:"maximum,omitempty"`
    Items       *PropertySchema        `json:"items,omitempty"` // for arrays
    Properties  map[string]PropertySchema `json:"properties,omitempty"` // for objects
    Sensitive   bool                   `json:"sensitive,omitempty"` // marks sensitive data like passwords
}
```

### Export Data Structures

```go
type ExportData struct {
    Devices           []DeviceData           `json:"devices"`
    Configurations    []ConfigurationData    `json:"configurations"`
    Templates         []TemplateData         `json:"templates"`
    DiscoveredDevices []DiscoveredDeviceData `json:"discovered_devices,omitempty"`
    Metadata          ExportMetadata         `json:"metadata"`
    Timestamp         time.Time              `json:"timestamp"`
}

type DeviceData struct {
    ID           uint                   `json:"id"`
    MAC          string                 `json:"mac"`
    IP           string                 `json:"ip"`
    Type         string                 `json:"type"`
    Name         string                 `json:"name"`
    Model        string                 `json:"model"`
    Firmware     string                 `json:"firmware"`
    Status       string                 `json:"status"`
    LastSeen     time.Time              `json:"last_seen"`
    Settings     map[string]interface{} `json:"settings"`
    Configuration *ConfigurationData    `json:"configuration,omitempty"`
    CreatedAt    time.Time              `json:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at"`
}

type ConfigurationData struct {
    DeviceID     uint                   `json:"device_id"`
    TemplateID   *uint                  `json:"template_id,omitempty"`
    Config       map[string]interface{} `json:"config"`
    LastSynced   *time.Time             `json:"last_synced,omitempty"`
    SyncStatus   string                 `json:"sync_status"`
    UpdatedAt    time.Time              `json:"updated_at"`
}

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

type ExportMetadata struct {
    ExportID        string    `json:"export_id"`
    RequestedBy     string    `json:"requested_by"`
    ExportType      string    `json:"export_type"` // "manual", "scheduled", "api"
    TotalDevices    int       `json:"total_devices"`
    TotalConfigs    int       `json:"total_configs"`
    FilterApplied   bool      `json:"filter_applied"`
    FilterCriteria  string    `json:"filter_criteria,omitempty"`
}
```

### Export Request & Response

```go
type ExportRequest struct {
    PluginName    string                 `json:"plugin_name"`
    Format        string                 `json:"format"`
    Config        map[string]interface{} `json:"config"`
    Filters       ExportFilters          `json:"filters"`
    Output        OutputConfig           `json:"output"`
    Options       ExportOptions          `json:"options"`
}

type ExportFilters struct {
    DeviceIDs     []uint     `json:"device_ids,omitempty"`
    DeviceTypes   []string   `json:"device_types,omitempty"`
    DeviceStatus  []string   `json:"device_status,omitempty"`
    LastSeenAfter *time.Time `json:"last_seen_after,omitempty"`
    HasConfig     *bool      `json:"has_config,omitempty"`
    TemplateIDs   []uint     `json:"template_ids,omitempty"`
    Tags          []string   `json:"tags,omitempty"`
}

type OutputConfig struct {
    Type        string                 `json:"type"` // "file", "webhook", "response"
    Destination string                 `json:"destination,omitempty"` // file path or webhook URL
    Headers     map[string]string      `json:"headers,omitempty"`
    Webhook     *WebhookConfig         `json:"webhook,omitempty"`
    Compression string                 `json:"compression,omitempty"` // "gzip", "zip", "none"
}

type WebhookConfig struct {
    URL         string            `json:"url"`
    Method      string            `json:"method"`
    Headers     map[string]string `json:"headers"`
    AuthType    string            `json:"auth_type"` // "none", "bearer", "basic", "api_key"
    AuthConfig  map[string]string `json:"auth_config"`
    Timeout     time.Duration     `json:"timeout"`
    Retries     int               `json:"retries"`
}

type ExportOptions struct {
    DryRun          bool `json:"dry_run"`
    IncludeHistory  bool `json:"include_history"`
    ValidateOnly    bool `json:"validate_only"`
    CompactOutput   bool `json:"compact_output"`
    IncludeMetadata bool `json:"include_metadata"`
}

type ExportResult struct {
    Success       bool          `json:"success"`
    ExportID      string        `json:"export_id"`
    PluginName    string        `json:"plugin_name"`
    Format        string        `json:"format"`
    OutputPath    string        `json:"output_path,omitempty"`
    WebhookSent   bool          `json:"webhook_sent,omitempty"`
    RecordCount   int           `json:"record_count"`
    FileSize      int64         `json:"file_size"`
    Checksum      string        `json:"checksum"`
    Duration      time.Duration `json:"duration"`
    Errors        []string      `json:"errors,omitempty"`
    Warnings      []string      `json:"warnings,omitempty"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
    CreatedAt     time.Time     `json:"created_at"`
}

type PreviewResult struct {
    Success     bool     `json:"success"`
    SampleData  []byte   `json:"sample_data"`
    RecordCount int      `json:"record_count"`
    EstimatedSize int64  `json:"estimated_size"`
    Warnings    []string `json:"warnings,omitempty"`
}
```

## Built-in Export Plugins

### 1. Home Assistant Exporter

```go
type HomeAssistantExporter struct {
    logger *logging.Logger
}

func (h *HomeAssistantExporter) Info() PluginInfo {
    return PluginInfo{
        Name:        "home_assistant",
        Version:     "1.0.0",
        Description: "Export Shelly devices for Home Assistant integration",
        Author:      "Shelly Manager Team",
        License:     "MIT",
        SupportedFormats: []string{"mqtt_discovery", "yaml_config", "shell_script"},
        Tags:        []string{"home-assistant", "mqtt", "smart-home"},
        Category:    CategoryHomeAutomation,
    }
}

func (h *HomeAssistantExporter) ConfigSchema() ConfigSchema {
    return ConfigSchema{
        Version: "1.0",
        Properties: map[string]PropertySchema{
            "mqtt_prefix": {
                Type:        "string",
                Description: "MQTT discovery prefix",
                Default:     "homeassistant",
            },
            "device_class_mapping": {
                Type:        "object",
                Description: "Mapping of Shelly device types to HA device classes",
            },
            "include_diagnostic_sensors": {
                Type:        "boolean",
                Description: "Include diagnostic sensors (temperature, signal strength, etc.)",
                Default:     true,
            },
            "availability_topic": {
                Type:        "string",
                Description: "Base topic for device availability",
                Default:     "shelly/status",
            },
        },
        Required: []string{"mqtt_prefix"},
    }
}

// Export formats:
// - mqtt_discovery: JSON payloads for MQTT discovery
// - yaml_config: Home Assistant configuration.yaml format
// - shell_script: Shell script to configure devices via HA CLI
```

### 2. DHCP Reservation Exporter

```go
type DHCPExporter struct {
    logger *logging.Logger
}

func (d *DHCPExporter) Info() PluginInfo {
    return PluginInfo{
        Name:        "dhcp_reservations",
        Version:     "1.0.0", 
        Description: "Export static DHCP reservations for various DHCP servers",
        Author:      "Shelly Manager Team",
        License:     "MIT",
        SupportedFormats: []string{"opnsense", "pfsense", "isc_dhcp", "dnsmasq", "csv"},
        Tags:        []string{"dhcp", "networking", "reservations"},
        Category:    CategoryNetworking,
    }
}

func (d *DHCPExporter) ConfigSchema() ConfigSchema {
    return ConfigSchema{
        Version: "1.0",
        Properties: map[string]PropertySchema{
            "network": {
                Type:        "string",
                Description: "Target network CIDR (e.g., 192.168.1.0/24)",
                Pattern:     `^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$`,
            },
            "hostname_template": {
                Type:        "string",
                Description: "Template for generating hostnames",
                Default:     "shelly-{{.Type}}-{{.MAC | last 4}}",
            },
            "include_discovered": {
                Type:        "boolean",
                Description: "Include recently discovered devices",
                Default:     false,
            },
        },
        Required: []string{"network"},
    }
}

// Export formats:
// - opnsense: XML configuration import
// - pfsense: XML configuration import
// - isc_dhcp: dhcpd.conf format
// - dnsmasq: dnsmasq.conf format  
// - csv: Generic CSV format
```

### 3. Network Documentation Exporter

```go
type NetworkDocsExporter struct {
    logger *logging.Logger
}

func (n *NetworkDocsExporter) Info() PluginInfo {
    return PluginInfo{
        Name:        "network_docs",
        Version:     "1.0.0",
        Description: "Export network documentation in various formats",
        Author:      "Shelly Manager Team", 
        License:     "MIT",
        SupportedFormats: []string{"hosts", "ansible", "netbox", "markdown", "graphviz"},
        Tags:        []string{"documentation", "network", "inventory"},
        Category:    CategoryDocumentation,
    }
}

// Export formats:
// - hosts: /etc/hosts format
// - ansible: Ansible inventory YAML
// - netbox: NetBox JSON import format
// - markdown: Network documentation markdown
// - graphviz: Network topology diagram (DOT format)
```

### 4. Monitoring Integration Exporter

```go
type MonitoringExporter struct {
    logger *logging.Logger
}

func (m *MonitoringExporter) Info() PluginInfo {
    return PluginInfo{
        Name:        "monitoring",
        Version:     "1.0.0",
        Description: "Export device information for monitoring systems",
        Author:      "Shelly Manager Team",
        License:     "MIT", 
        SupportedFormats: []string{"prometheus", "nagios", "zabbix", "icinga", "checkmk"},
        Tags:        []string{"monitoring", "alerting", "metrics"},
        Category:    CategoryMonitoring,
    }
}

// Export formats:
// - prometheus: Prometheus targets and rules
// - nagios: Nagios host and service definitions
// - zabbix: Zabbix host import XML
// - icinga: Icinga2 configuration
// - checkmk: Check_MK host definitions
```

## Template-Based Export Engine

For simple format transformations, the system includes a template-based export engine:

```go
type TemplateExporter struct {
    name        string
    description string
    templates   map[string]*template.Template
    logger      *logging.Logger
}

type TemplateConfig struct {
    Name        string            `yaml:"name"`
    Description string            `yaml:"description"`
    Version     string            `yaml:"version"`
    Templates   map[string]string `yaml:"templates"`
    Functions   map[string]string `yaml:"functions,omitempty"`
    Validation  ValidationRules   `yaml:"validation,omitempty"`
}

// Example template configuration (YAML)
```yaml
name: "custom_csv"
description: "Custom CSV export format"
version: "1.0.0"
templates:
  csv: |
    {{- range .Devices }}
    {{.Name}},{{.IP}},{{.MAC}},{{.Type}},{{.Status}}
    {{- end }}
functions:
  mac_format: |
    {{- define "mac_format" -}}
    {{- . | replace ":" "-" | upper -}}
    {{- end -}}
validation:
  required_fields: ["Name", "IP", "MAC"]
  max_records: 1000
```

## Export Scheduling System

```go
type ExportScheduler struct {
    schedules map[string]*ExportSchedule
    cron      *cron.Cron
    engine    *ExportEngine
    logger    *logging.Logger
}

type ExportSchedule struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Description string        `json:"description"`
    Enabled     bool          `json:"enabled"`
    CronSpec    string        `json:"cron_spec"`
    Timezone    string        `json:"timezone"`
    Request     ExportRequest `json:"request"`
    NextRun     time.Time     `json:"next_run"`
    LastRun     *time.Time    `json:"last_run,omitempty"`
    LastResult  *ExportResult `json:"last_result,omitempty"`
    CreatedAt   time.Time     `json:"created_at"`
    UpdatedAt   time.Time     `json:"updated_at"`
}

// Example: Export to Home Assistant every day at 2 AM
// CronSpec: "0 2 * * *"
```

## Export Validation System

```go
type ExportValidator struct {
    rules  map[string]ValidationRule
    logger *logging.Logger
}

type ValidationRule interface {
    Validate(data *ExportData, config ExportConfig) []ValidationError
}

type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Level   string `json:"level"` // "error", "warning", "info"
}

type ValidationResult struct {
    Valid    bool              `json:"valid"`
    Errors   []ValidationError `json:"errors"`
    Warnings []ValidationError `json:"warnings"`
}

// Built-in validation rules:
// - RequiredFieldsRule: Ensures required fields are present
// - DataSizeRule: Validates export data size limits
// - FormatRule: Validates output format constraints
// - AuthenticationRule: Validates authentication requirements
```

## Export History & Audit

```go
type ExportHistory struct {
    db     *gorm.DB
    logger *logging.Logger
}

type ExportHistoryRecord struct {
    ID           uint      `json:"id" gorm:"primaryKey"`
    ExportID     string    `json:"export_id" gorm:"uniqueIndex"`
    PluginName   string    `json:"plugin_name"`
    Format       string    `json:"format"`
    RequestedBy  string    `json:"requested_by"`
    Success      bool      `json:"success"`
    RecordCount  int       `json:"record_count"`
    FileSize     int64     `json:"file_size"`
    Duration     int64     `json:"duration"` // milliseconds
    ErrorMessage string    `json:"error_message,omitempty"`
    Config       string    `json:"config" gorm:"type:text"` // JSON
    Metadata     string    `json:"metadata" gorm:"type:text"` // JSON
    CreatedAt    time.Time `json:"created_at"`
}

// Audit trail features:
// - Track all export attempts
// - Store configuration used
// - Performance metrics
// - Error analysis
// - Data retention policies
```

## API Endpoints

### Export Operations

```http
POST /api/v1/export
POST /api/v1/export/preview
GET  /api/v1/export/{export_id}
GET  /api/v1/export/{export_id}/download
DELETE /api/v1/export/{export_id}
```

### Plugin Management

```http
GET  /api/v1/export/plugins
GET  /api/v1/export/plugins/{plugin_name}
GET  /api/v1/export/plugins/{plugin_name}/schema
POST /api/v1/export/plugins/{plugin_name}/validate
```

### Scheduling

```http
GET    /api/v1/export/schedules
POST   /api/v1/export/schedules
GET    /api/v1/export/schedules/{schedule_id}
PUT    /api/v1/export/schedules/{schedule_id}
DELETE /api/v1/export/schedules/{schedule_id}
POST   /api/v1/export/schedules/{schedule_id}/run
```

### History & Audit

```http
GET /api/v1/export/history
GET /api/v1/export/history/{export_id}
GET /api/v1/export/statistics
```

## Security Considerations

### Plugin Security

1. **Input Validation**: All plugin inputs are validated against schema
2. **Resource Limits**: Memory and CPU usage limits per plugin
3. **Timeout Protection**: Maximum execution time per export
4. **File System Access**: Restricted to designated export directories
5. **Network Access**: Configurable network access controls

### Data Protection

1. **Sensitive Data Handling**: Automatic detection and masking of sensitive fields
2. **Access Control**: Role-based access to export functionality
3. **Audit Logging**: Complete audit trail of all export operations
4. **Encryption**: Optional encryption of export files
5. **Secure Transmission**: HTTPS/TLS for webhook deliveries

### Authentication & Authorization

```go
type ExportPermissions struct {
    CanExport          bool     `json:"can_export"`
    CanSchedule        bool     `json:"can_schedule"`
    CanViewHistory     bool     `json:"can_view_history"`
    AllowedPlugins     []string `json:"allowed_plugins"`
    AllowedFormats     []string `json:"allowed_formats"`
    MaxExportSize      int64    `json:"max_export_size"`
    MaxScheduledExports int     `json:"max_scheduled_exports"`
}
```

## Testing Framework

### Plugin Testing

```go
type PluginTestSuite struct {
    plugin     ExportPlugin
    testData   *ExportData
    testConfig map[string]interface{}
}

func (pts *PluginTestSuite) TestExportFormats() error
func (pts *PluginTestSuite) TestConfigValidation() error
func (pts *PluginTestSuite) TestErrorHandling() error
func (pts *PluginTestSuite) TestPerformance() error

// Automated testing for all plugins
// Performance benchmarking
// Error condition testing
// Format validation
```

## Migration & Deployment

### Plugin Deployment

1. **Built-in Plugins**: Compiled with the application
2. **Template Plugins**: Loaded from configuration files
3. **External Plugins**: Future support for external processes
4. **Hot Reloading**: Development-time plugin reloading
5. **Version Management**: Plugin versioning and compatibility

### Backward Compatibility

1. **API Versioning**: Versioned plugin interfaces
2. **Migration Tools**: Automatic migration of plugin configurations  
3. **Deprecation Warnings**: Grace period for deprecated features
4. **Configuration Updates**: Automatic configuration schema updates

---

**Document Version**: 1.0  
**Last Updated**: 2024-01-19  
**Next Review**: 2024-04-19  
**Owner**: Shelly Manager Development Team
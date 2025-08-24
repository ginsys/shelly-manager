# Shelly Manager Plugin Architecture

The Shelly Manager plugin architecture provides a flexible, extensible system for adding new functionality to the application. The architecture supports multiple plugin types and provides a type-safe registry system.

## Architecture Overview

The plugin system is organized into these components:

```
internal/plugins/
├── interfaces.go        # Common plugin interfaces and types
├── registry.go         # Generalized plugin registry
├── sync/               # Sync plugins (export/import operations)
├── notification/       # Notification plugins (email, slack, etc.)
└── discovery/          # Discovery plugins (mDNS, SSDP, etc.)
```

## Plugin Types

### Sync Plugins (`PluginTypeSync`)
Handle data export and import operations:
- **Backup Plugin**: Database backup and restore
- **GitOps Plugin**: Export configurations as GitOps-ready YAML
- **OPNsense Plugin**: Sync with OPNsense firewall

### Notification Plugins (`PluginTypeNotification`) 
Send notifications through various channels:
- Email notifications (SMTP)
- Slack notifications
- Discord notifications
- Webhook notifications

### Discovery Plugins (`PluginTypeDiscovery`)
Discover devices on the network:
- mDNS/Bonjour discovery
- SSDP/UPnP discovery
- Network scanning (nmap-style)
- Bluetooth discovery

## Core Plugin Interface

Every plugin must implement the base `Plugin` interface:

```go
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
```

## Plugin Metadata

Plugins provide metadata through the `PluginInfo` structure:

```go
type PluginInfo struct {
    Name             string         `json:"name"`
    Version          string         `json:"version"`
    Description      string         `json:"description"`
    Author           string         `json:"author"`
    License          string         `json:"license"`
    SupportedFormats []string       `json:"supported_formats,omitempty"`
    Tags             []string       `json:"tags"`
    Category         PluginCategory `json:"category"`
    Dependencies     []string       `json:"dependencies,omitempty"`
    MinVersion       string         `json:"min_version,omitempty"`
}
```

## Configuration Schema

Plugins define their configuration requirements using JSON Schema-style definitions:

```go
type ConfigSchema struct {
    Version    string                    `json:"version"`
    Properties map[string]PropertySchema `json:"properties"`
    Required   []string                  `json:"required"`
    Examples   []map[string]interface{}  `json:"examples,omitempty"`
}
```

## Health Monitoring

Plugins provide health status information:

```go
type HealthStatus struct {
    Status      HealthStatusType `json:"status"`
    LastChecked time.Time        `json:"last_checked"`
    Message     string           `json:"message,omitempty"`
    Details     interface{}      `json:"details,omitempty"`
}
```

Health status types:
- `HealthStatusHealthy`: Plugin is working normally
- `HealthStatusDegraded`: Plugin has minor issues but is functional
- `HealthStatusUnhealthy`: Plugin has significant issues
- `HealthStatusUnavailable`: Plugin is not available

## Plugin Registry

The plugin registry manages plugin lifecycle:

```go
// Register a plugin
registry := plugins.NewRegistry(logger)
err := registry.RegisterPlugin(myPlugin)

// Get a plugin
plugin, err := registry.GetPlugin(plugins.PluginTypeSync, "backup")

// List plugins by type
plugins := registry.GetPluginsByType(plugins.PluginTypeSync)

// Health check all plugins
health := registry.HealthCheck()
```

## Creating a Sync Plugin

1. **Implement the base Plugin interface**
2. **Extend with SyncPlugin interface** for sync-specific operations
3. **Provide plugin metadata** (name, version, description, etc.)
4. **Define configuration schema** for user-configurable options
5. **Implement Export/Import operations**

Example sync plugin:

```go
package myplugin

import (
    "context"
    "time"
    
    "github.com/ginsys/shelly-manager/internal/plugins"
    syncplugins "github.com/ginsys/shelly-manager/internal/plugins/sync"
    "github.com/ginsys/shelly-manager/internal/sync"
)

type MyPlugin struct {
    logger *logging.Logger
}

func NewPlugin() syncplugins.SyncPlugin {
    return &MyPlugin{}
}

func (p *MyPlugin) Type() plugins.PluginType {
    return plugins.PluginTypeSync
}

func (p *MyPlugin) Info() plugins.PluginInfo {
    return plugins.PluginInfo{
        Name:        "my-plugin",
        Version:     "1.0.0",
        Description: "My custom sync plugin",
        Author:      "Your Name",
        License:     "MIT",
        SupportedFormats: []string{"json", "yaml"},
        Tags:        []string{"export", "custom"},
        Category:    plugins.CategoryCustom,
    }
}

func (p *MyPlugin) ConfigSchema() plugins.ConfigSchema {
    return plugins.ConfigSchema{
        Version: "1.0",
        Properties: map[string]plugins.PropertySchema{
            "output_path": {
                Type:        "string",
                Description: "Output directory for exports",
                Default:     "/tmp/exports",
            },
        },
        Required: []string{"output_path"},
    }
}

func (p *MyPlugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
    // Implement export logic
    return &sync.ExportResult{
        Success: true,
        // ... other fields
    }, nil
}

// Implement other required methods...
```

## Creating a Notification Plugin

1. **Implement the base Plugin interface**
2. **Extend with NotificationPlugin interface**
3. **Implement Send and BatchSend operations**
4. **Support various notification types**

Example notification plugin:

```go
package email

import (
    "context"
    
    "github.com/ginsys/shelly-manager/internal/plugins"
    "github.com/ginsys/shelly-manager/internal/plugins/notification"
)

type EmailPlugin struct {
    smtpConfig SMTPConfig
    logger     *logging.Logger
}

func NewPlugin() notification.NotificationPlugin {
    return &EmailPlugin{}
}

func (p *EmailPlugin) Type() plugins.PluginType {
    return plugins.PluginTypeNotification
}

func (p *EmailPlugin) Send(ctx context.Context, notif notification.Notification) (*notification.SendResult, error) {
    // Implement email sending logic
    return &notification.SendResult{
        Success:     true,
        DeliveredTo: notif.Recipients,
        // ... other fields
    }, nil
}

// Implement other required methods...
```

## Creating a Discovery Plugin

1. **Implement the base Plugin interface**
2. **Extend with DiscoveryPlugin interface**
3. **Implement Discover and Scan operations**
4. **Support device monitoring**

Example discovery plugin:

```go
package mdns

import (
    "context"
    
    "github.com/ginsys/shelly-manager/internal/plugins"
    "github.com/ginsys/shelly-manager/internal/plugins/discovery"
)

type MDNSPlugin struct {
    logger *logging.Logger
}

func NewPlugin() discovery.DiscoveryPlugin {
    return &MDNSPlugin{}
}

func (p *MDNSPlugin) Type() plugins.PluginType {
    return plugins.PluginTypeDiscovery
}

func (p *MDNSPlugin) Discover(ctx context.Context, config discovery.DiscoveryConfig) (*discovery.DiscoveryResult, error) {
    // Implement mDNS discovery logic
    return &discovery.DiscoveryResult{
        Success:      true,
        DevicesFound: devices,
        // ... other fields
    }, nil
}

// Implement other required methods...
```

## Best Practices

### Configuration
- Use JSON Schema for configuration validation
- Provide sensible defaults for all optional settings
- Document all configuration options clearly
- Support environment variable substitution where appropriate

### Error Handling
- Use structured errors with context
- Provide clear error messages for users
- Log errors appropriately (don't spam logs)
- Implement graceful degradation when possible

### Performance
- Make operations cancellable via context
- Support concurrent operations when safe
- Implement proper timeouts
- Cache expensive operations when appropriate

### Security
- Validate all inputs thoroughly
- Mark sensitive configuration as `Sensitive: true`
- Never log sensitive information
- Implement proper authentication/authorization

### Testing
- Write comprehensive unit tests
- Test error conditions and edge cases
- Mock external dependencies
- Provide test data and fixtures
- Test configuration schema validation

### Documentation
- Document all public APIs
- Provide usage examples
- Document configuration options
- Include troubleshooting guides

## Plugin Registration

Plugins are automatically registered at startup through the registry system:

```go
// In main.go or plugin registration code
baseRegistry := plugins.NewRegistry(logger)
syncRegistry := syncplugins.NewRegistry(baseRegistry, logger)

// Register individual plugins
err := syncRegistry.RegisterPlugin(backup.NewGeneralizedPlugin())
err = syncRegistry.RegisterPlugin(gitops.NewGeneralizedPlugin())

// Or register all plugins at once
err := pluginRegistry.RegisterAllPlugins()
```

## Migration from Legacy System

The new plugin architecture maintains backward compatibility with the existing sync system:

1. **Legacy plugins** continue to work through adapter wrappers
2. **Existing APIs** remain unchanged
3. **Gradual migration** path from old to new system
4. **Feature parity** maintained during transition

## Future Extensions

The plugin architecture is designed to support future extensions:

- **Plugin dependencies** and load ordering
- **Dynamic plugin loading** at runtime  
- **Plugin configuration UI** generation from schema
- **Plugin marketplace** and distribution
- **Sandboxed plugin execution** for security
- **Cross-plugin communication** and event system

## Troubleshooting

### Plugin Registration Issues
- Check plugin implements all required interfaces
- Verify configuration schema is valid
- Check for naming conflicts with existing plugins
- Review logs for detailed error messages

### Plugin Initialization Failures
- Verify all dependencies are available
- Check configuration validation
- Review plugin health status
- Check file permissions for output directories

### Runtime Errors
- Check plugin health status regularly
- Monitor plugin logs for errors
- Verify network connectivity for network-dependent plugins
- Check resource availability (disk space, memory)

## Contributing

When contributing new plugins:

1. Follow the established patterns and interfaces
2. Write comprehensive tests
3. Document configuration options
4. Provide usage examples
5. Follow Go coding conventions
6. Test integration with existing system
package sync

import (
	"context"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// SyncPlugin extends the base Plugin interface for sync-specific operations
type SyncPlugin interface {
	plugins.Plugin

	// Export Operations
	Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error)
	Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error)

	// Import Operations
	Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error)

	// Sync-specific capabilities
	Capabilities() plugins.PluginCapabilities
}

// SyncPluginRegistry defines the interface for sync plugin management
type SyncPluginRegistry interface {
	RegisterPlugin(plugin SyncPlugin) error
	UnregisterPlugin(pluginName string) error
	GetPlugin(pluginName string) (SyncPlugin, error)
	ListPlugins() []plugins.PluginInfo
	GetPluginsByCategory(category plugins.PluginCategory) []SyncPlugin
}

// LegacySyncPluginAdapter adapts the old sync.SyncPlugin interface to the new structure
type LegacySyncPluginAdapter struct {
	legacyPlugin sync.SyncPlugin
}

// NewLegacySyncPluginAdapter creates a new adapter for legacy sync plugins
func NewLegacySyncPluginAdapter(legacyPlugin sync.SyncPlugin) *LegacySyncPluginAdapter {
	return &LegacySyncPluginAdapter{
		legacyPlugin: legacyPlugin,
	}
}

// Info returns plugin information
func (a *LegacySyncPluginAdapter) Info() plugins.PluginInfo {
	info := a.legacyPlugin.Info()
	return plugins.PluginInfo{
		Name:             info.Name,
		Version:          info.Version,
		Description:      info.Description,
		Author:           info.Author,
		Website:          info.Website,
		License:          info.License,
		SupportedFormats: info.SupportedFormats,
		Tags:             info.Tags,
		Category:         plugins.PluginCategory(info.Category),
	}
}

// Type returns the plugin type
func (a *LegacySyncPluginAdapter) Type() plugins.PluginType {
	return plugins.PluginTypeSync
}

// ConfigSchema returns the configuration schema
func (a *LegacySyncPluginAdapter) ConfigSchema() plugins.ConfigSchema {
	schema := a.legacyPlugin.ConfigSchema()
	props := make(map[string]plugins.PropertySchema)
	for k, v := range schema.Properties {
		prop := plugins.PropertySchema{
			Type:        v.Type,
			Description: v.Description,
			Default:     v.Default,
			Enum:        v.Enum,
			Pattern:     v.Pattern,
			Minimum:     v.Minimum,
			Maximum:     v.Maximum,
			Sensitive:   v.Sensitive,
		}

		if v.Items != nil {
			prop.Items = &plugins.PropertySchema{
				Type:        v.Items.Type,
				Description: v.Items.Description,
				Default:     v.Items.Default,
				Enum:        v.Items.Enum,
				Pattern:     v.Items.Pattern,
				Minimum:     v.Items.Minimum,
				Maximum:     v.Items.Maximum,
				Sensitive:   v.Items.Sensitive,
			}
		}

		if v.Properties != nil {
			itemProps := make(map[string]plugins.PropertySchema)
			for ik, iv := range v.Properties {
				itemProps[ik] = plugins.PropertySchema{
					Type:        iv.Type,
					Description: iv.Description,
					Default:     iv.Default,
					Enum:        iv.Enum,
					Pattern:     iv.Pattern,
					Minimum:     iv.Minimum,
					Maximum:     iv.Maximum,
					Sensitive:   iv.Sensitive,
				}
			}
			prop.Properties = itemProps
		}

		props[k] = prop
	}

	return plugins.ConfigSchema{
		Version:    schema.Version,
		Properties: props,
		Required:   schema.Required,
		Examples:   schema.Examples,
	}
}

// ValidateConfig validates the plugin configuration
func (a *LegacySyncPluginAdapter) ValidateConfig(config map[string]interface{}) error {
	return a.legacyPlugin.ValidateConfig(config)
}

// Initialize initializes the plugin
func (a *LegacySyncPluginAdapter) Initialize(logger *logging.Logger) error {
	return a.legacyPlugin.Initialize(logger)
}

// Cleanup cleans up plugin resources
func (a *LegacySyncPluginAdapter) Cleanup() error {
	return a.legacyPlugin.Cleanup()
}

// Health returns the health status
func (a *LegacySyncPluginAdapter) Health() plugins.HealthStatus {
	return plugins.HealthStatus{
		Status:      plugins.HealthStatusHealthy,
		LastChecked: time.Now(),
		Message:     "Legacy plugin adapter - health status not available",
	}
}

// Export performs the export operation
func (a *LegacySyncPluginAdapter) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	return a.legacyPlugin.Export(ctx, data, config)
}

// Preview generates a preview
func (a *LegacySyncPluginAdapter) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	return a.legacyPlugin.Preview(ctx, data, config)
}

// Import performs the import operation
func (a *LegacySyncPluginAdapter) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	return a.legacyPlugin.Import(ctx, source, config)
}

// Capabilities returns plugin capabilities
func (a *LegacySyncPluginAdapter) Capabilities() plugins.PluginCapabilities {
	caps := a.legacyPlugin.Capabilities()
	return plugins.PluginCapabilities{
		SupportsIncremental:    caps.SupportsIncremental,
		SupportsScheduling:     caps.SupportsScheduling,
		RequiresAuthentication: caps.RequiresAuthentication,
		SupportedOutputs:       caps.SupportedOutputs,
		MaxDataSize:            caps.MaxDataSize,
		ConcurrencyLevel:       caps.ConcurrencyLevel,
	}
}

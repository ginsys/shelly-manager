package backup

import (
	"time"

	"github.com/ginsys/shelly-manager/internal/plugins"
	syncplugins "github.com/ginsys/shelly-manager/internal/plugins/sync"
)

// Plugin wraps the backup plugin to implement the new plugin interfaces
type Plugin struct {
	*BackupPlugin
}

// NewGeneralizedPlugin creates a new backup plugin that implements the generalized plugin interface
func NewGeneralizedPlugin() syncplugins.SyncPlugin {
	// Use the existing NewPlugin function and cast to get the BackupPlugin
	basePlugin := NewPlugin().(*BackupPlugin)
	return &Plugin{
		BackupPlugin: basePlugin,
	}
}

// Type returns the plugin type
func (p *Plugin) Type() plugins.PluginType {
	return plugins.PluginTypeSync
}

// Info returns plugin information using the new format
func (p *Plugin) Info() plugins.PluginInfo {
	legacyInfo := p.BackupPlugin.Info()
	return plugins.PluginInfo{
		Name:             legacyInfo.Name,
		Version:          legacyInfo.Version,
		Description:      legacyInfo.Description,
		Author:           legacyInfo.Author,
		Website:          legacyInfo.Website,
		License:          legacyInfo.License,
		SupportedFormats: legacyInfo.SupportedFormats,
		Tags:             legacyInfo.Tags,
		Category:         plugins.PluginCategory(legacyInfo.Category),
		Dependencies:     []string{"database"},
		MinVersion:       "1.0.0",
	}
}

// ConfigSchema returns the configuration schema using the new format
func (p *Plugin) ConfigSchema() plugins.ConfigSchema {
	legacySchema := p.BackupPlugin.ConfigSchema()
	props := make(map[string]plugins.PropertySchema)

	for k, v := range legacySchema.Properties {
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
			prop.Properties = make(map[string]plugins.PropertySchema)
			for pk, pv := range v.Properties {
				prop.Properties[pk] = plugins.PropertySchema{
					Type:        pv.Type,
					Description: pv.Description,
					Default:     pv.Default,
					Enum:        pv.Enum,
					Pattern:     pv.Pattern,
					Minimum:     pv.Minimum,
					Maximum:     pv.Maximum,
					Sensitive:   pv.Sensitive,
				}
			}
		}

		props[k] = prop
	}

	return plugins.ConfigSchema{
		Version:    legacySchema.Version,
		Properties: props,
		Required:   legacySchema.Required,
		Examples:   legacySchema.Examples,
	}
}

// Health returns the health status of the plugin
func (p *Plugin) Health() plugins.HealthStatus {
	status := plugins.HealthStatusHealthy
	message := "Backup plugin is healthy"

	// Check if database manager is available
	if p.BackupPlugin.dbManager == nil {
		status = plugins.HealthStatusDegraded
		message = "Database manager not initialized"
	}

	return plugins.HealthStatus{
		Status:      status,
		LastChecked: time.Now(),
		Message:     message,
		Details: map[string]interface{}{
			"database_manager": p.BackupPlugin.dbManager != nil,
			"logger":           p.BackupPlugin.logger != nil,
		},
	}
}

// Capabilities returns plugin capabilities using the new format
func (p *Plugin) Capabilities() plugins.PluginCapabilities {
	legacyCaps := p.BackupPlugin.Capabilities()
	return plugins.PluginCapabilities{
		SupportsIncremental:    legacyCaps.SupportsIncremental,
		SupportsScheduling:     legacyCaps.SupportsScheduling,
		RequiresAuthentication: legacyCaps.RequiresAuthentication,
		SupportedOutputs:       legacyCaps.SupportedOutputs,
		MaxDataSize:            legacyCaps.MaxDataSize,
		ConcurrencyLevel:       legacyCaps.ConcurrencyLevel,
		RequiresNetwork:        false,
		IsExperimental:         false,
	}
}

// SetDatabaseManager sets the database manager for the backup plugin
func (p *Plugin) SetDatabaseManager(dbManager DatabaseManagerInterface) {
	p.BackupPlugin.dbManager = dbManager
}

// GetDatabaseManager returns the database manager
func (p *Plugin) GetDatabaseManager() DatabaseManagerInterface {
	return p.BackupPlugin.dbManager
}

// NewPluginWithDBManager creates a new backup plugin with a database manager
func NewPluginWithDBManager(dbManager DatabaseManagerInterface) syncplugins.SyncPlugin {
	plugin := &Plugin{
		BackupPlugin: NewBackupExporter(dbManager),
	}
	return plugin
}

// Ensure Plugin implements the required interfaces
var _ syncplugins.SyncPlugin = (*Plugin)(nil)
var _ plugins.Plugin = (*Plugin)(nil)

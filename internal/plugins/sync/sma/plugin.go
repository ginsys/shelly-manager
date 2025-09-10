package sma

import (
	"time"

	"github.com/ginsys/shelly-manager/internal/plugins"
	syncplugins "github.com/ginsys/shelly-manager/internal/plugins/sync"
)

// Plugin wraps the SMA plugin to implement the new plugin interfaces
type Plugin struct {
	*SMAPlugin
}

// NewGeneralizedPlugin creates a new SMA plugin that implements the generalized plugin interface
func NewGeneralizedPlugin() syncplugins.SyncPlugin {
	// Use the existing NewPlugin function and cast to get the SMAPlugin
	basePlugin := NewPlugin().(*SMAPlugin)
	return &Plugin{
		SMAPlugin: basePlugin,
	}
}

// Type returns the plugin type
func (p *Plugin) Type() plugins.PluginType {
	return plugins.PluginTypeSync
}

// Info returns plugin information using the new format
func (p *Plugin) Info() plugins.PluginInfo {
	legacyInfo := p.SMAPlugin.Info()
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
	legacySchema := p.SMAPlugin.ConfigSchema()
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
	message := "SMA plugin is healthy"

	// Basic health checks
	if p.logger == nil {
		status = plugins.HealthStatusDegraded
		message = "Logger not initialized"
	}

	return plugins.HealthStatus{
		Status:      status,
		LastChecked: time.Now(),
		Message:     message,
		Details: map[string]interface{}{
			"parser_available":    true,
			"generator_available": true,
			"logger":              p.logger != nil,
		},
	}
}

// Capabilities returns plugin capabilities using the new format
func (p *Plugin) Capabilities() plugins.PluginCapabilities {
	legacyCaps := p.SMAPlugin.Capabilities()
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

// Ensure Plugin implements the required interfaces
var _ syncplugins.SyncPlugin = (*Plugin)(nil)
var _ plugins.Plugin = (*Plugin)(nil)

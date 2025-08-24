package sync

import (
	"fmt"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins"
)

// Registry manages sync-specific plugins
type Registry struct {
	baseRegistry *plugins.Registry
	logger       *logging.Logger
}

// NewRegistry creates a new sync plugin registry
func NewRegistry(baseRegistry *plugins.Registry, logger *logging.Logger) *Registry {
	return &Registry{
		baseRegistry: baseRegistry,
		logger:       logger,
	}
}

// RegisterPlugin registers a sync plugin
func (r *Registry) RegisterPlugin(plugin SyncPlugin) error {
	return r.baseRegistry.RegisterPlugin(plugin)
}

// UnregisterPlugin unregisters a sync plugin
func (r *Registry) UnregisterPlugin(pluginName string) error {
	return r.baseRegistry.UnregisterPlugin(plugins.PluginTypeSync, pluginName)
}

// GetPlugin retrieves a sync plugin by name
func (r *Registry) GetPlugin(pluginName string) (SyncPlugin, error) {
	plugin, err := r.baseRegistry.GetPlugin(plugins.PluginTypeSync, pluginName)
	if err != nil {
		return nil, err
	}

	syncPlugin, ok := plugin.(SyncPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s is not a sync plugin", pluginName)
	}

	return syncPlugin, nil
}

// GetPluginsByCategory returns sync plugins of a specific category
func (r *Registry) GetPluginsByCategory(category plugins.PluginCategory) []SyncPlugin {
	allPlugins := r.baseRegistry.GetPluginsByCategory(category)
	syncPlugins := make([]SyncPlugin, 0, len(allPlugins))

	for _, plugin := range allPlugins {
		if plugin.Type() == plugins.PluginTypeSync {
			if syncPlugin, ok := plugin.(SyncPlugin); ok {
				syncPlugins = append(syncPlugins, syncPlugin)
			}
		}
	}

	return syncPlugins
}

// ListPlugins returns information about all sync plugins
func (r *Registry) ListPlugins() []plugins.PluginInfo {
	return r.baseRegistry.ListPluginsByType(plugins.PluginTypeSync)
}

// GetPlugins returns all sync plugins
func (r *Registry) GetPlugins() []SyncPlugin {
	allPlugins := r.baseRegistry.GetPluginsByType(plugins.PluginTypeSync)
	syncPlugins := make([]SyncPlugin, 0, len(allPlugins))

	for _, plugin := range allPlugins {
		if syncPlugin, ok := plugin.(SyncPlugin); ok {
			syncPlugins = append(syncPlugins, syncPlugin)
		}
	}

	return syncPlugins
}

// GetPluginCount returns the number of registered sync plugins
func (r *Registry) GetPluginCount() int {
	counts := r.baseRegistry.GetPluginCountByType()
	return counts[plugins.PluginTypeSync]
}

// HealthCheck performs health checks on all sync plugins
func (r *Registry) HealthCheck() map[string]plugins.HealthStatus {
	allHealth := r.baseRegistry.HealthCheck()
	syncHealth := make(map[string]plugins.HealthStatus)

	for key, health := range allHealth {
		// Key format is "type:name", filter for sync plugins
		if len(key) > 5 && key[:5] == "sync:" {
			syncHealth[key[5:]] = health // Remove "sync:" prefix
		}
	}

	return syncHealth
}

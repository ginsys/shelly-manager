package registry

import (
	"fmt"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins"
	syncplugins "github.com/ginsys/shelly-manager/internal/plugins/sync"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/backup"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/opnsense"
)

// PluginRegistry manages the registration of all sync plugins using the generalized system
type PluginRegistry struct {
	baseRegistry *plugins.Registry
	syncRegistry *syncplugins.Registry
	logger       *logging.Logger
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry(baseRegistry *plugins.Registry, logger *logging.Logger) *PluginRegistry {
	return &PluginRegistry{
		baseRegistry: baseRegistry,
		syncRegistry: syncplugins.NewRegistry(baseRegistry, logger),
		logger:       logger,
	}
}

// RegisterAllPlugins registers all available sync plugins
func (r *PluginRegistry) RegisterAllPlugins() error {
	plugins := []syncplugins.SyncPlugin{
		backup.NewGeneralizedPlugin(),
		gitops.NewGeneralizedPlugin(),
		opnsense.NewGeneralizedPlugin(),
	}

	var errors []error

	for _, plugin := range plugins {
		if err := r.syncRegistry.RegisterPlugin(plugin); err != nil {
			r.logger.Error("Failed to register sync plugin",
				"plugin", plugin.Info().Name,
				"error", err,
			)
			errors = append(errors, err)
		} else {
			r.logger.Info("Successfully registered sync plugin",
				"plugin", plugin.Info().Name,
				"version", plugin.Info().Version,
				"category", plugin.Info().Category,
			)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to register %d sync plugins: %v", len(errors), errors)
	}

	r.logger.Info("All sync plugins registered successfully",
		"total", len(plugins),
	)

	return nil
}

// RegisterPlugin registers a single sync plugin
func (r *PluginRegistry) RegisterPlugin(plugin syncplugins.SyncPlugin) error {
	err := r.syncRegistry.RegisterPlugin(plugin)
	if err != nil {
		r.logger.Error("Failed to register sync plugin",
			"plugin", plugin.Info().Name,
			"error", err,
		)
		return err
	}

	r.logger.Info("Successfully registered sync plugin",
		"plugin", plugin.Info().Name,
		"version", plugin.Info().Version,
		"category", plugin.Info().Category,
	)

	return nil
}

// UnregisterPlugin unregisters a sync plugin
func (r *PluginRegistry) UnregisterPlugin(pluginName string) error {
	err := r.syncRegistry.UnregisterPlugin(pluginName)
	if err != nil {
		r.logger.Error("Failed to unregister sync plugin",
			"plugin", pluginName,
			"error", err,
		)
		return err
	}

	r.logger.Info("Successfully unregistered sync plugin",
		"plugin", pluginName,
	)

	return nil
}

// GetPlugin retrieves a sync plugin by name
func (r *PluginRegistry) GetPlugin(pluginName string) (syncplugins.SyncPlugin, error) {
	return r.syncRegistry.GetPlugin(pluginName)
}

// ListPlugins returns information about all registered sync plugins
func (r *PluginRegistry) ListPlugins() []plugins.PluginInfo {
	return r.syncRegistry.ListPlugins()
}

// GetPluginsByCategory returns sync plugins of a specific category
func (r *PluginRegistry) GetPluginsByCategory(category plugins.PluginCategory) []syncplugins.SyncPlugin {
	return r.syncRegistry.GetPluginsByCategory(category)
}

// GetPlugins returns all sync plugins
func (r *PluginRegistry) GetPlugins() []syncplugins.SyncPlugin {
	return r.syncRegistry.GetPlugins()
}

// GetPluginCount returns the number of registered sync plugins
func (r *PluginRegistry) GetPluginCount() int {
	return r.syncRegistry.GetPluginCount()
}

// HealthCheck performs health checks on all sync plugins
func (r *PluginRegistry) HealthCheck() map[string]plugins.HealthStatus {
	return r.syncRegistry.HealthCheck()
}

// GetBaseRegistry returns the base plugin registry for advanced operations
func (r *PluginRegistry) GetBaseRegistry() *plugins.Registry {
	return r.baseRegistry
}

// GetSyncRegistry returns the sync-specific registry
func (r *PluginRegistry) GetSyncRegistry() *syncplugins.Registry {
	return r.syncRegistry
}

// DatabaseManagerAdapter adapts database.Manager to backup.DatabaseManagerInterface
type DatabaseManagerAdapter struct {
	*database.Manager
}

// GetDB returns the database connection as interface{}
func (a *DatabaseManagerAdapter) GetDB() interface{} {
	return a.Manager.GetDB()
}

// RegisterPluginWithDatabaseManager registers a backup plugin with a database manager
func (r *PluginRegistry) RegisterPluginWithDatabaseManager(dbManager interface{}) error {
	// Adapt the database manager
	if realDBManager, ok := dbManager.(*database.Manager); ok {
		adapter := &DatabaseManagerAdapter{Manager: realDBManager}

		// Check if backup plugin is already registered and unregister it
		if _, err := r.syncRegistry.GetPlugin("backup"); err == nil {
			r.logger.Info("Unregistering existing backup plugin to replace with database-enhanced version")
			if err := r.syncRegistry.UnregisterPlugin("backup"); err != nil {
				r.logger.Warn("Failed to unregister existing backup plugin",
					"error", err,
				)
				// Continue anyway - the RegisterPlugin call below will handle the duplicate
			}
		}

		// Create backup plugin with database manager
		backupPlugin := backup.NewPluginWithDBManager(adapter)

		// Register the plugin
		err := r.syncRegistry.RegisterPlugin(backupPlugin)
		if err != nil {
			r.logger.Error("Failed to register backup plugin with database manager",
				"error", err,
			)
			return err
		}

		r.logger.Info("Successfully registered backup plugin with database manager")
		return nil
	}

	return fmt.Errorf("unsupported database manager type")
}

// Backward compatibility methods that match the old registry interface

// RegisterAllPluginsLegacy provides backward compatibility with the old interface
func (r *PluginRegistry) RegisterAllPluginsLegacy() error {
	return r.RegisterAllPlugins()
}

// GetSyncEngine returns a legacy-compatible interface for the sync engine
// This is a placeholder to maintain compatibility during migration
func (r *PluginRegistry) GetSyncEngine() interface{} {
	// This would need to be implemented based on how the SyncEngine is used
	// For now, returning nil as a placeholder
	return nil
}

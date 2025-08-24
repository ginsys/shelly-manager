package registry

import (
	"fmt"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/sync/plugins/backup"
	"github.com/ginsys/shelly-manager/internal/sync/plugins/gitops"
	"github.com/ginsys/shelly-manager/internal/sync/plugins/opnsense"
)

// PluginRegistry manages the registration of all sync plugins
type PluginRegistry struct {
	engine *sync.SyncEngine
	logger *logging.Logger
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry(engine *sync.SyncEngine, logger *logging.Logger) *PluginRegistry {
	return &PluginRegistry{
		engine: engine,
		logger: logger,
	}
}

// RegisterAllPlugins registers all available plugins
func (r *PluginRegistry) RegisterAllPlugins() error {
	plugins := []sync.SyncPlugin{
		backup.NewPlugin(),
		gitops.NewPlugin(),
		opnsense.NewPlugin(),
	}

	var errors []error

	for _, plugin := range plugins {
		if err := r.engine.RegisterPlugin(plugin); err != nil {
			r.logger.Error("Failed to register plugin",
				"plugin", plugin.Info().Name,
				"error", err,
			)
			errors = append(errors, err)
		} else {
			r.logger.Info("Successfully registered plugin",
				"plugin", plugin.Info().Name,
				"version", plugin.Info().Version,
				"category", plugin.Info().Category,
			)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to register %d plugins: %v", len(errors), errors)
	}

	r.logger.Info("All plugins registered successfully",
		"total", len(plugins),
	)

	return nil
}

// RegisterPlugin registers a single plugin
func (r *PluginRegistry) RegisterPlugin(plugin sync.SyncPlugin) error {
	err := r.engine.RegisterPlugin(plugin)
	if err != nil {
		r.logger.Error("Failed to register plugin",
			"plugin", plugin.Info().Name,
			"error", err,
		)
		return err
	}

	r.logger.Info("Successfully registered plugin",
		"plugin", plugin.Info().Name,
		"version", plugin.Info().Version,
		"category", plugin.Info().Category,
	)

	return nil
}

// UnregisterPlugin unregisters a plugin
func (r *PluginRegistry) UnregisterPlugin(pluginName string) error {
	err := r.engine.UnregisterPlugin(pluginName)
	if err != nil {
		r.logger.Error("Failed to unregister plugin",
			"plugin", pluginName,
			"error", err,
		)
		return err
	}

	r.logger.Info("Successfully unregistered plugin",
		"plugin", pluginName,
	)

	return nil
}

// ListPlugins returns information about all registered plugins
func (r *PluginRegistry) ListPlugins() []sync.PluginInfo {
	return r.engine.ListPlugins()
}

package plugins

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Registry manages plugins of different types in a type-safe manner
type Registry struct {
	mu      sync.RWMutex
	plugins map[PluginType]map[string]Plugin // type -> name -> plugin
	logger  *logging.Logger
}

// NewRegistry creates a new plugin registry
func NewRegistry(logger *logging.Logger) *Registry {
	return &Registry{
		plugins: make(map[PluginType]map[string]Plugin),
		logger:  logger,
	}
}

// RegisterPlugin registers a plugin with the registry
func (r *Registry) RegisterPlugin(plugin Plugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin cannot be nil")
	}

	info := plugin.Info()
	pluginType := plugin.Type()

	if info.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize type map if it doesn't exist
	if r.plugins[pluginType] == nil {
		r.plugins[pluginType] = make(map[string]Plugin)
	}

	// Check if plugin already exists
	if _, exists := r.plugins[pluginType][info.Name]; exists {
		return fmt.Errorf("plugin %s of type %s already registered", info.Name, pluginType)
	}

	// Validate plugin configuration
	if err := plugin.ValidateConfig(map[string]interface{}{}); err != nil {
		r.logger.Debug("Plugin config validation completed", "plugin", info.Name, "result", err)
	}

	// Initialize the plugin
	if err := plugin.Initialize(r.logger); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", info.Name, err)
	}

	// Register the plugin
	r.plugins[pluginType][info.Name] = plugin

	r.logger.Info("Plugin registered successfully",
		"name", info.Name,
		"type", pluginType,
		"version", info.Version,
		"category", info.Category,
		"author", info.Author,
	)

	return nil
}

// UnregisterPlugin unregisters a plugin
func (r *Registry) UnregisterPlugin(pluginType PluginType, pluginName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	typePlugins, exists := r.plugins[pluginType]
	if !exists {
		return fmt.Errorf("no plugins of type %s registered", pluginType)
	}

	plugin, exists := typePlugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s of type %s not found", pluginName, pluginType)
	}

	// Cleanup the plugin
	if err := plugin.Cleanup(); err != nil {
		r.logger.Warn("Error during plugin cleanup",
			"plugin", pluginName,
			"type", pluginType,
			"error", err,
		)
	}

	// Remove from registry
	delete(typePlugins, pluginName)

	// Remove type map if empty
	if len(typePlugins) == 0 {
		delete(r.plugins, pluginType)
	}

	r.logger.Info("Plugin unregistered successfully",
		"name", pluginName,
		"type", pluginType,
	)

	return nil
}

// GetPlugin retrieves a plugin by type and name
func (r *Registry) GetPlugin(pluginType PluginType, pluginName string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typePlugins, exists := r.plugins[pluginType]
	if !exists {
		return nil, fmt.Errorf("no plugins of type %s registered", pluginType)
	}

	plugin, exists := typePlugins[pluginName]
	if !exists {
		return nil, fmt.Errorf("plugin %s of type %s not found", pluginName, pluginType)
	}

	return plugin, nil
}

// GetPluginsByType returns all plugins of a specific type
func (r *Registry) GetPluginsByType(pluginType PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typePlugins, exists := r.plugins[pluginType]
	if !exists {
		return []Plugin{}
	}

	plugins := make([]Plugin, 0, len(typePlugins))
	for _, plugin := range typePlugins {
		plugins = append(plugins, plugin)
	}

	// Sort plugins by name for consistent ordering
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Info().Name < plugins[j].Info().Name
	})

	return plugins
}

// GetPluginsByCategory returns all plugins of a specific category
func (r *Registry) GetPluginsByCategory(category PluginCategory) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []Plugin

	for _, typePlugins := range r.plugins {
		for _, plugin := range typePlugins {
			if plugin.Info().Category == category {
				plugins = append(plugins, plugin)
			}
		}
	}

	// Sort plugins by type then name for consistent ordering
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Type() != plugins[j].Type() {
			return plugins[i].Type() < plugins[j].Type()
		}
		return plugins[i].Info().Name < plugins[j].Info().Name
	})

	return plugins
}

// ListPlugins returns information about all registered plugins
func (r *Registry) ListPlugins() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var pluginInfos []PluginInfo

	for _, typePlugins := range r.plugins {
		for _, plugin := range typePlugins {
			info := plugin.Info()
			pluginInfos = append(pluginInfos, info)
		}
	}

	// Sort by type then name
	sort.Slice(pluginInfos, func(i, j int) bool {
		if pluginInfos[i].Category != pluginInfos[j].Category {
			return pluginInfos[i].Category < pluginInfos[j].Category
		}
		return pluginInfos[i].Name < pluginInfos[j].Name
	})

	return pluginInfos
}

// ListPluginsByType returns information about all plugins of a specific type
func (r *Registry) ListPluginsByType(pluginType PluginType) []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typePlugins, exists := r.plugins[pluginType]
	if !exists {
		return []PluginInfo{}
	}

	pluginInfos := make([]PluginInfo, 0, len(typePlugins))
	for _, plugin := range typePlugins {
		info := plugin.Info()
		pluginInfos = append(pluginInfos, info)
	}

	// Sort by name
	sort.Slice(pluginInfos, func(i, j int) bool {
		return pluginInfos[i].Name < pluginInfos[j].Name
	})

	return pluginInfos
}

// GetPluginTypes returns all registered plugin types
func (r *Registry) GetPluginTypes() []PluginType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]PluginType, 0, len(r.plugins))
	for pluginType := range r.plugins {
		types = append(types, pluginType)
	}

	// Sort for consistent ordering
	sort.Slice(types, func(i, j int) bool {
		return types[i] < types[j]
	})

	return types
}

// HealthCheck performs a health check on all plugins
func (r *Registry) HealthCheck() map[string]HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthStatuses := make(map[string]HealthStatus)

	for pluginType, typePlugins := range r.plugins {
		for pluginName, plugin := range typePlugins {
			key := fmt.Sprintf("%s:%s", pluginType, pluginName)
			healthStatuses[key] = plugin.Health()
		}
	}

	return healthStatuses
}

// GetPluginCount returns the total number of registered plugins
func (r *Registry) GetPluginCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, typePlugins := range r.plugins {
		count += len(typePlugins)
	}
	return count
}

// GetPluginCountByType returns the number of plugins for each type
func (r *Registry) GetPluginCountByType() map[PluginType]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[PluginType]int)
	for pluginType, typePlugins := range r.plugins {
		counts[pluginType] = len(typePlugins)
	}
	return counts
}

// Shutdown gracefully shuts down all plugins
func (r *Registry) Shutdown() []error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []error

	for pluginType, typePlugins := range r.plugins {
		for pluginName, plugin := range typePlugins {
			r.logger.Info("Shutting down plugin",
				"name", pluginName,
				"type", pluginType,
			)

			if err := plugin.Cleanup(); err != nil {
				pluginErr := fmt.Errorf("error shutting down plugin %s:%s: %w", pluginType, pluginName, err)
				errors = append(errors, pluginErr)
				r.logger.Error("Error shutting down plugin",
					"name", pluginName,
					"type", pluginType,
					"error", err,
				)
			}
		}
	}

	// Clear all plugins
	r.plugins = make(map[PluginType]map[string]Plugin)

	if len(errors) > 0 {
		r.logger.Warn("Plugin registry shutdown completed with errors",
			"error_count", len(errors),
		)
	} else {
		r.logger.Info("Plugin registry shutdown completed successfully")
	}

	return errors
}

// PluginStats returns statistics about the plugin registry
type PluginStats struct {
	TotalPlugins    int            `json:"total_plugins"`
	PluginsByType   map[string]int `json:"plugins_by_type"`
	HealthySummary  HealthSummary  `json:"health_summary"`
	LastHealthCheck time.Time      `json:"last_health_check"`
	RegisteredTypes []string       `json:"registered_types"`
}

// HealthSummary provides a summary of plugin health statuses
type HealthSummary struct {
	Healthy     int `json:"healthy"`
	Degraded    int `json:"degraded"`
	Unhealthy   int `json:"unhealthy"`
	Unknown     int `json:"unknown"`
	Unavailable int `json:"unavailable"`
}

// GetStats returns comprehensive statistics about the plugin registry
func (r *Registry) GetStats() PluginStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := PluginStats{
		TotalPlugins:    r.GetPluginCount(),
		PluginsByType:   make(map[string]int),
		LastHealthCheck: time.Now(),
		RegisteredTypes: make([]string, 0),
	}

	// Count plugins by type
	for pluginType, typePlugins := range r.plugins {
		stats.PluginsByType[string(pluginType)] = len(typePlugins)
		stats.RegisteredTypes = append(stats.RegisteredTypes, string(pluginType))
	}

	// Health summary
	healthStatuses := r.HealthCheck()
	for _, health := range healthStatuses {
		switch health.Status {
		case HealthStatusHealthy:
			stats.HealthySummary.Healthy++
		case HealthStatusDegraded:
			stats.HealthySummary.Degraded++
		case HealthStatusUnhealthy:
			stats.HealthySummary.Unhealthy++
		case HealthStatusUnavailable:
			stats.HealthySummary.Unavailable++
		default:
			stats.HealthySummary.Unknown++
		}
	}

	// Sort registered types for consistency
	sort.Strings(stats.RegisteredTypes)

	return stats
}

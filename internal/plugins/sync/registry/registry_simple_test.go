package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// SimpleMockPlugin implements SyncPlugin interface for testing with minimal requirements
type SimpleMockPlugin struct {
	name     string
	version  string
	category plugins.PluginCategory
}

func (m *SimpleMockPlugin) Info() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:     m.name,
		Version:  m.version,
		Category: m.category,
		License:  "MIT",
		Author:   "Test Author",
	}
}

func (m *SimpleMockPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeSync
}

func (m *SimpleMockPlugin) ConfigSchema() plugins.ConfigSchema {
	return plugins.ConfigSchema{
		Version: "1.0",
		Properties: map[string]plugins.PropertySchema{
			"test": {Type: "string"},
		},
	}
}

func (m *SimpleMockPlugin) ValidateConfig(config map[string]interface{}) error {
	return nil
}

func (m *SimpleMockPlugin) Initialize(logger *logging.Logger) error {
	return nil
}

func (m *SimpleMockPlugin) Cleanup() error {
	return nil
}

func (m *SimpleMockPlugin) Health() plugins.HealthStatus {
	return plugins.HealthStatus{
		Status:  plugins.HealthStatusHealthy,
		Message: "Mock plugin is healthy",
	}
}

func (m *SimpleMockPlugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	// Return nil for now - this is not the focus of registry testing
	return nil, nil
}

func (m *SimpleMockPlugin) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	// Return nil for now - this is not the focus of registry testing
	return nil, nil
}

func (m *SimpleMockPlugin) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	// Return nil for now - this is not the focus of registry testing
	return nil, nil
}

func (m *SimpleMockPlugin) Capabilities() plugins.PluginCapabilities {
	return plugins.PluginCapabilities{
		SupportsIncremental:    true,
		SupportsScheduling:     true,
		RequiresAuthentication: false,
		SupportedOutputs:       []string{"file", "api"},
		MaxDataSize:            1024 * 1024, // 1MB
		ConcurrencyLevel:       1,
		RequiresNetwork:        false,
		IsExperimental:         false,
	}
}

func setupSimpleTestPluginRegistry(t *testing.T) (*PluginRegistry, *logging.Logger) {
	logger, err := logging.New(logging.Config{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	})
	require.NoError(t, err)

	baseRegistry := plugins.NewRegistry(logger)
	pluginRegistry := NewPluginRegistry(baseRegistry, logger)

	return pluginRegistry, logger
}

func TestSimplePluginRegistry_NewPluginRegistry(t *testing.T) {
	logger, err := logging.New(logging.Config{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	})
	require.NoError(t, err)

	baseRegistry := plugins.NewRegistry(logger)
	pluginRegistry := NewPluginRegistry(baseRegistry, logger)

	assert.NotNil(t, pluginRegistry)
	assert.NotNil(t, pluginRegistry.baseRegistry)
	assert.NotNil(t, pluginRegistry.syncRegistry)
	assert.NotNil(t, pluginRegistry.logger)
	assert.Equal(t, baseRegistry, pluginRegistry.baseRegistry)
	assert.Equal(t, logger, pluginRegistry.logger)
}

func TestSimplePluginRegistry_RegisterAllPlugins(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Successful Registration", func(t *testing.T) {
		err := pluginRegistry.RegisterAllPlugins()
		assert.NoError(t, err)

		// Verify plugins are registered
		plugins := pluginRegistry.ListPlugins()
		assert.Len(t, plugins, 4) // backup, gitops, opnsense, sma

		// Verify plugin count
		count := pluginRegistry.GetPluginCount()
		assert.Equal(t, 4, count)
	})
}

func TestSimplePluginRegistry_RegisterPlugin(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Register Single Plugin", func(t *testing.T) {
		mockPlugin := &SimpleMockPlugin{
			name:     "test-plugin",
			version:  "1.0.0",
			category: plugins.CategoryBackup,
		}

		err := pluginRegistry.RegisterPlugin(mockPlugin)
		assert.NoError(t, err)

		// Verify plugin is registered
		registeredPlugin, err := pluginRegistry.GetPlugin("test-plugin")
		assert.NoError(t, err)
		assert.NotNil(t, registeredPlugin)
		assert.Equal(t, "test-plugin", registeredPlugin.Info().Name)
	})

	t.Run("Register Duplicate Plugin", func(t *testing.T) {
		mockPlugin1 := &SimpleMockPlugin{
			name:     "duplicate-plugin",
			version:  "1.0.0",
			category: plugins.CategoryBackup,
		}
		mockPlugin2 := &SimpleMockPlugin{
			name:     "duplicate-plugin",
			version:  "2.0.0",
			category: plugins.CategoryBackup,
		}

		err := pluginRegistry.RegisterPlugin(mockPlugin1)
		assert.NoError(t, err)

		// Attempting to register a plugin with the same name should fail
		err = pluginRegistry.RegisterPlugin(mockPlugin2)
		assert.Error(t, err)
	})
}

func TestSimplePluginRegistry_UnregisterPlugin(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Unregister Existing Plugin", func(t *testing.T) {
		// First register a plugin
		mockPlugin := &SimpleMockPlugin{
			name:     "unregister-test",
			version:  "1.0.0",
			category: plugins.CategoryBackup,
		}

		err := pluginRegistry.RegisterPlugin(mockPlugin)
		require.NoError(t, err)

		// Verify it's registered
		_, err = pluginRegistry.GetPlugin("unregister-test")
		assert.NoError(t, err)

		// Unregister it
		err = pluginRegistry.UnregisterPlugin("unregister-test")
		assert.NoError(t, err)

		// Verify it's no longer registered
		_, err = pluginRegistry.GetPlugin("unregister-test")
		assert.Error(t, err)
	})

	t.Run("Unregister Non-existent Plugin", func(t *testing.T) {
		err := pluginRegistry.UnregisterPlugin("non-existent-plugin")
		assert.Error(t, err)
	})
}

func TestSimplePluginRegistry_GetPlugin(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Get Existing Plugin", func(t *testing.T) {
		// Register all plugins first
		err := pluginRegistry.RegisterAllPlugins()
		require.NoError(t, err)

		// Get a specific plugin
		plugin, err := pluginRegistry.GetPlugin("backup")
		assert.NoError(t, err)
		assert.NotNil(t, plugin)
		assert.Equal(t, "backup", plugin.Info().Name)
	})

	t.Run("Get Non-existent Plugin", func(t *testing.T) {
		plugin, err := pluginRegistry.GetPlugin("non-existent")
		assert.Error(t, err)
		assert.Nil(t, plugin)
	})
}

func TestSimplePluginRegistry_ListPlugins(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("List Empty Registry", func(t *testing.T) {
		plugins := pluginRegistry.ListPlugins()
		assert.Empty(t, plugins)
	})

	t.Run("List Populated Registry", func(t *testing.T) {
		// Register some plugins
		err := pluginRegistry.RegisterAllPlugins()
		require.NoError(t, err)

		plugins := pluginRegistry.ListPlugins()
		assert.Len(t, plugins, 4)

		// Verify plugin info structure
		for _, plugin := range plugins {
			assert.NotEmpty(t, plugin.Name)
			assert.NotEmpty(t, plugin.Version)
			assert.NotEmpty(t, plugin.License)
		}
	})
}

func TestSimplePluginRegistry_GetPluginsByCategory(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	// Register all plugins
	err := pluginRegistry.RegisterAllPlugins()
	require.NoError(t, err)

	t.Run("Get Backup Plugins", func(t *testing.T) {
		backupPlugins := pluginRegistry.GetPluginsByCategory(plugins.CategoryBackup)
		assert.NotEmpty(t, backupPlugins)

		// Verify all returned plugins are backup category
		for _, plugin := range backupPlugins {
			assert.Equal(t, plugins.CategoryBackup, plugin.Info().Category)
		}
	})

	t.Run("Get Non-existent Category", func(t *testing.T) {
		nonExistentPlugins := pluginRegistry.GetPluginsByCategory(plugins.PluginCategory("nonexistent"))
		assert.Empty(t, nonExistentPlugins)
	})
}

func TestSimplePluginRegistry_GetPlugins(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Get All Plugins", func(t *testing.T) {
		// Register all plugins
		err := pluginRegistry.RegisterAllPlugins()
		require.NoError(t, err)

		allPlugins := pluginRegistry.GetPlugins()
		assert.Len(t, allPlugins, 4)

		// Verify we have the expected plugins
		pluginNames := make(map[string]bool)
		for _, plugin := range allPlugins {
			pluginNames[plugin.Info().Name] = true
		}

		assert.True(t, pluginNames["backup"])
		assert.True(t, pluginNames["gitops"])
		assert.True(t, pluginNames["opnsense"])
	})
}

func TestSimplePluginRegistry_GetPluginCount(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Empty Registry Count", func(t *testing.T) {
		count := pluginRegistry.GetPluginCount()
		assert.Equal(t, 0, count)
	})

	t.Run("Populated Registry Count", func(t *testing.T) {
		err := pluginRegistry.RegisterAllPlugins()
		require.NoError(t, err)

		count := pluginRegistry.GetPluginCount()
		assert.Equal(t, 4, count)
	})
}

func TestSimplePluginRegistry_HealthCheck(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Health Check All Plugins", func(t *testing.T) {
		err := pluginRegistry.RegisterAllPlugins()
		require.NoError(t, err)

		healthStatuses := pluginRegistry.HealthCheck()
		assert.Len(t, healthStatuses, 4)

		// Verify all plugins report health status
		for pluginName, status := range healthStatuses {
			assert.NotEmpty(t, pluginName)
			assert.NotEmpty(t, status.Status)
			assert.Contains(t, []plugins.HealthStatusType{
				plugins.HealthStatusHealthy,
				plugins.HealthStatusDegraded,
				plugins.HealthStatusUnhealthy,
			}, status.Status)
		}
	})
}

func TestSimplePluginRegistry_GetBaseRegistry(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	baseRegistry := pluginRegistry.GetBaseRegistry()
	assert.NotNil(t, baseRegistry)
	assert.Equal(t, pluginRegistry.baseRegistry, baseRegistry)
}

func TestSimplePluginRegistry_GetSyncRegistry(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	syncRegistry := pluginRegistry.GetSyncRegistry()
	assert.NotNil(t, syncRegistry)
	assert.Equal(t, pluginRegistry.syncRegistry, syncRegistry)
}

func TestSimplePluginRegistry_RegisterAllPluginsLegacy(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Legacy Registration Method", func(t *testing.T) {
		err := pluginRegistry.RegisterAllPluginsLegacy()
		assert.NoError(t, err)

		// Should work the same as RegisterAllPlugins
		count := pluginRegistry.GetPluginCount()
		assert.Equal(t, 4, count)
	})
}

func TestSimplePluginRegistry_GetSyncEngine(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Get Sync Engine", func(t *testing.T) {
		syncEngine := pluginRegistry.GetSyncEngine()
		// Currently returns nil as a placeholder
		assert.Nil(t, syncEngine)
	})
}

func TestSimplePluginRegistry_RegisterPluginWithDatabaseManager(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Register with Invalid Database Manager", func(t *testing.T) {
		err := pluginRegistry.RegisterPluginWithDatabaseManager("invalid-manager")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported database manager type")
	})

	t.Run("Register with Nil Database Manager", func(t *testing.T) {
		err := pluginRegistry.RegisterPluginWithDatabaseManager(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported database manager type")
	})
}

func TestDatabaseManagerAdapter_GetDB(t *testing.T) {
	t.Run("Adapter GetDB Method", func(t *testing.T) {
		// Create a simple mock for testing the adapter GetDB method
		adapter := &DatabaseManagerAdapter{}

		// Test that GetDB returns nil when no manager is set
		db := adapter.GetDB()
		assert.Nil(t, db)
	})
}

func TestSimplePluginRegistry_ConcurrentOperations(t *testing.T) {
	pluginRegistry, _ := setupSimpleTestPluginRegistry(t)

	t.Run("Concurrent Registration and Access", func(t *testing.T) {
		// Register all plugins
		err := pluginRegistry.RegisterAllPlugins()
		require.NoError(t, err)

		// Test concurrent read operations
		done := make(chan bool, 3)

		// Concurrent ListPlugins
		go func() {
			plugins := pluginRegistry.ListPlugins()
			assert.Len(t, plugins, 4)
			done <- true
		}()

		// Concurrent GetPluginCount
		go func() {
			count := pluginRegistry.GetPluginCount()
			assert.Equal(t, 4, count)
			done <- true
		}()

		// Concurrent HealthCheck
		go func() {
			health := pluginRegistry.HealthCheck()
			assert.Len(t, health, 4)
			done <- true
		}()

		// Wait for all goroutines to complete
		for i := 0; i < 3; i++ {
			<-done
		}
	})
}

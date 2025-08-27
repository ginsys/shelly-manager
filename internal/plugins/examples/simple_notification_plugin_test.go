package examples

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins"
)

func TestNewSimpleNotificationPlugin(t *testing.T) {
	plugin := NewSimpleNotificationPlugin()
	assert.NotNil(t, plugin)
	assert.IsType(t, &SimpleNotificationPlugin{}, plugin)
}

func TestSimpleNotificationPlugin_Type(t *testing.T) {
	plugin := &SimpleNotificationPlugin{}
	assert.Equal(t, plugins.PluginTypeNotification, plugin.Type())
}

func TestSimpleNotificationPlugin_Info(t *testing.T) {
	plugin := &SimpleNotificationPlugin{}
	info := plugin.Info()

	assert.Equal(t, "simple-notification", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.NotEmpty(t, info.Description)
	assert.Equal(t, "Shelly Manager Team", info.Author)
	assert.Equal(t, "MIT", info.License)
	assert.Contains(t, info.Tags, "notification")
	assert.Contains(t, info.Tags, "example")
	assert.Equal(t, plugins.CategoryCustom, info.Category)
	assert.Equal(t, "1.0.0", info.MinVersion)
}

func TestSimpleNotificationPlugin_ConfigSchema(t *testing.T) {
	plugin := &SimpleNotificationPlugin{}
	schema := plugin.ConfigSchema()

	assert.Equal(t, "1.0", schema.Version)
	assert.Contains(t, schema.Properties, "output_file")
	assert.Contains(t, schema.Properties, "format")

	// Check output_file property
	outputFileProp := schema.Properties["output_file"]
	assert.Equal(t, "string", outputFileProp.Type)
	assert.Equal(t, "/tmp/notifications.log", outputFileProp.Default)

	// Check format property
	formatProp := schema.Properties["format"]
	assert.Equal(t, "string", formatProp.Type)
	assert.Equal(t, "text", formatProp.Default)
	assert.Contains(t, formatProp.Enum, "text")
	assert.Contains(t, formatProp.Enum, "json")

	assert.Contains(t, schema.Required, "output_file")
	assert.NotEmpty(t, schema.Examples)
}

func TestSimpleNotificationPlugin_ValidateConfig(t *testing.T) {
	plugin := &SimpleNotificationPlugin{}

	t.Run("Valid Text Config", func(t *testing.T) {
		config := map[string]interface{}{
			"output_file": "/tmp/test.log",
			"format":      "text",
		}

		err := plugin.ValidateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Valid JSON Config", func(t *testing.T) {
		config := map[string]interface{}{
			"output_file": "/tmp/test.json",
			"format":      "json",
		}

		err := plugin.ValidateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("Invalid Output File Type", func(t *testing.T) {
		config := map[string]interface{}{
			"output_file": 12345, // Should be string
			"format":      "text",
		}

		err := plugin.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output_file must be a string")
	})

	t.Run("Invalid Format", func(t *testing.T) {
		config := map[string]interface{}{
			"output_file": "/tmp/test.log",
			"format":      "xml", // Unsupported format
		}

		err := plugin.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "format must be 'text' or 'json'")
	})

	t.Run("Invalid Format Type", func(t *testing.T) {
		config := map[string]interface{}{
			"output_file": "/tmp/test.log",
			"format":      123, // Should be string
		}

		err := plugin.ValidateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "format must be a string")
	})

	t.Run("Empty Config", func(t *testing.T) {
		config := map[string]interface{}{}

		err := plugin.ValidateConfig(config)
		assert.NoError(t, err) // Should handle missing optional fields
	})
}

func TestSimpleNotificationPlugin_Initialize(t *testing.T) {
	plugin := &SimpleNotificationPlugin{}

	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	err = plugin.Initialize(logger)
	assert.NoError(t, err)
	assert.Equal(t, logger, plugin.logger)
}

func TestSimpleNotificationPlugin_Health(t *testing.T) {
	plugin := &SimpleNotificationPlugin{}

	t.Run("Uninitialized Plugin Health", func(t *testing.T) {
		health := plugin.Health()
		assert.Equal(t, plugins.HealthStatusHealthy, health.Status)
		assert.NotEmpty(t, health.Message)
		assert.NotZero(t, health.LastChecked)

		if detailsMap, ok := health.Details.(map[string]interface{}); ok {
			if loggerInit, exists := detailsMap["logger_initialized"]; exists {
				assert.False(t, loggerInit.(bool))
			}
		}
	})

	t.Run("Initialized Plugin Health", func(t *testing.T) {
		logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
		require.NoError(t, err)

		err = plugin.Initialize(logger)
		require.NoError(t, err)

		health := plugin.Health()
		assert.Equal(t, plugins.HealthStatusHealthy, health.Status)
		assert.NotEmpty(t, health.Message)

		if detailsMap, ok := health.Details.(map[string]interface{}); ok {
			if loggerInit, exists := detailsMap["logger_initialized"]; exists {
				assert.True(t, loggerInit.(bool))
			}
		}
	})
}

func TestSimpleNotificationPlugin_Cleanup(t *testing.T) {
	plugin := &SimpleNotificationPlugin{}

	t.Run("Cleanup Uninitialized Plugin", func(t *testing.T) {
		err := plugin.Cleanup()
		assert.NoError(t, err)
	})

	t.Run("Cleanup Initialized Plugin", func(t *testing.T) {
		logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
		require.NoError(t, err)

		err = plugin.Initialize(logger)
		require.NoError(t, err)

		err = plugin.Cleanup()
		assert.NoError(t, err)
	})
}

func TestSimpleNotificationPlugin_Integration(t *testing.T) {
	plugin := NewSimpleNotificationPlugin()

	// Test plugin lifecycle
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	// 1. Initialize
	err = plugin.Initialize(logger)
	assert.NoError(t, err)

	// 2. Validate config
	config := map[string]interface{}{
		"output_file": "/tmp/integration-test.log",
		"format":      "json",
	}

	err = plugin.ValidateConfig(config)
	assert.NoError(t, err)

	// 3. Check health
	health := plugin.Health()
	assert.Equal(t, plugins.HealthStatusHealthy, health.Status)

	// 4. Cleanup
	err = plugin.Cleanup()
	assert.NoError(t, err)
}

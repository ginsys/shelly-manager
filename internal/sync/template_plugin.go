package sync

import (
	"context"
	"fmt"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TemplatePlugin wraps a template-based plugin configuration into an ExportPlugin
type TemplatePlugin struct {
	manifest       *PluginManifest
	templateEngine *AdvancedTemplateEngine
	logger         *logging.Logger
}

// NewTemplatePlugin creates a new template plugin from a manifest
func NewTemplatePlugin(manifest *PluginManifest, logger *logging.Logger) *TemplatePlugin {
	return &TemplatePlugin{
		manifest:       manifest,
		templateEngine: NewAdvancedTemplateEngine(logger),
		logger:         logger,
	}
}

// Info returns plugin information from the manifest
func (tp *TemplatePlugin) Info() PluginInfo {
	category := tp.manifest.Category
	if category == "" {
		category = CategoryCustom
	}

	return PluginInfo{
		Name:             tp.manifest.Name,
		Version:          tp.manifest.Version,
		Description:      tp.manifest.Description,
		Author:           tp.manifest.Author,
		Website:          tp.manifest.Website,
		License:          tp.manifest.License,
		SupportedFormats: tp.manifest.SupportedFormats,
		Tags:             tp.manifest.Tags,
		Category:         category,
	}
}

// ConfigSchema returns the configuration schema from the manifest
func (tp *TemplatePlugin) ConfigSchema() ConfigSchema {
	if tp.manifest.ConfigSchema != nil {
		return *tp.manifest.ConfigSchema
	}

	// Return empty schema if none provided
	return ConfigSchema{
		Version:    "1.0",
		Properties: make(map[string]PropertySchema),
		Required:   []string{},
	}
}

// ValidateConfig validates the plugin configuration
func (tp *TemplatePlugin) ValidateConfig(config map[string]interface{}) error {
	schema := tp.ConfigSchema()

	// Validate required fields
	for _, required := range schema.Required {
		if _, exists := config[required]; !exists {
			return fmt.Errorf("required field '%s' is missing", required)
		}
	}

	// Validate individual properties
	for key, value := range config {
		if propSchema, exists := schema.Properties[key]; exists {
			if err := tp.validateProperty(key, value, propSchema); err != nil {
				return err
			}
		}
	}

	return nil
}

// Export performs the template-based export
func (tp *TemplatePlugin) Export(ctx context.Context, data *ExportData, config ExportConfig) (*ExportResult, error) {
	tp.logger.Info("Starting template-based export",
		"plugin", tp.manifest.Name,
		"format", config.Format,
		"devices", len(data.Devices),
	)

	// Find the template for the requested format
	templateContent, exists := tp.manifest.Templates[config.Format]
	if !exists {
		return nil, fmt.Errorf("template for format '%s' not found", config.Format)
	}

	// Prepare template data
	templateData := tp.prepareTemplateData(data, config)

	// Render the template
	result, err := tp.templateEngine.RenderTemplate(templateContent, templateData)
	if err != nil {
		return nil, fmt.Errorf("template rendering failed: %w", err)
	}

	// Create export result
	exportResult := &ExportResult{
		Success:     true,
		RecordCount: len(data.Devices),
		FileSize:    int64(len(result)),
		Metadata: map[string]interface{}{
			"plugin_type": "template",
			"template":    config.Format,
		},
	}

	// Handle output based on configuration
	if err := tp.handleOutput(result, config.Output, exportResult); err != nil {
		return nil, fmt.Errorf("output handling failed: %w", err)
	}

	tp.logger.Info("Template-based export completed",
		"plugin", tp.manifest.Name,
		"format", config.Format,
		"output_size", len(result),
	)

	return exportResult, nil
}

// Import performs template-based import (not supported by template plugins)
func (tp *TemplatePlugin) Import(ctx context.Context, source ImportSource, config ImportConfig) (*ImportResult, error) {
	return nil, fmt.Errorf("import operations are not supported by template-based plugins")
}

// Preview generates a preview of the template output
func (tp *TemplatePlugin) Preview(ctx context.Context, data *ExportData, config ExportConfig) (*PreviewResult, error) {
	tp.logger.Info("Generating template preview",
		"plugin", tp.manifest.Name,
		"format", config.Format,
	)

	// Find the template for the requested format
	templateContent, exists := tp.manifest.Templates[config.Format]
	if !exists {
		return nil, fmt.Errorf("template for format '%s' not found", config.Format)
	}

	// Prepare limited template data for preview (first few items)
	previewData := tp.preparePreviewData(data, config, 5)

	// Render the template
	result, err := tp.templateEngine.RenderTemplate(templateContent, previewData)
	if err != nil {
		return nil, fmt.Errorf("template rendering failed: %w", err)
	}

	// Estimate full size
	estimatedSize := int64(len(result))
	if len(data.Devices) > 5 {
		estimatedSize = estimatedSize * int64(len(data.Devices)) / 5
	}

	return &PreviewResult{
		Success:       true,
		SampleData:    []byte(result),
		RecordCount:   len(data.Devices),
		EstimatedSize: estimatedSize,
	}, nil
}

// Capabilities returns plugin capabilities from the manifest
func (tp *TemplatePlugin) Capabilities() PluginCapabilities {
	if tp.manifest.Capabilities != nil {
		return *tp.manifest.Capabilities
	}

	// Return default capabilities for template plugins
	return PluginCapabilities{
		SupportsIncremental:    false,
		SupportsScheduling:     true,
		RequiresAuthentication: false,
		SupportedOutputs:       []string{"file", "response"},
		MaxDataSize:            1024 * 1024 * 10, // 10MB default
		ConcurrencyLevel:       1,
	}
}

// Initialize initializes the template plugin
func (tp *TemplatePlugin) Initialize(logger *logging.Logger) error {
	tp.logger = logger
	tp.logger.Info("Initialized template plugin", "name", tp.manifest.Name, "version", tp.manifest.Version)
	return nil
}

// Cleanup cleans up plugin resources
func (tp *TemplatePlugin) Cleanup() error {
	tp.logger.Info("Cleaning up template plugin", "name", tp.manifest.Name)
	return nil
}

// Private helper methods

// validateProperty validates a single configuration property
func (tp *TemplatePlugin) validateProperty(key string, value interface{}, schema PropertySchema) error {
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("property '%s' must be a string", key)
		}
		if strVal := value.(string); schema.Pattern != "" {
			// TODO: Add regex validation
			_ = strVal
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("property '%s' must be a number", key)
		}
		if numVal := value.(float64); schema.Minimum != nil && numVal < *schema.Minimum {
			return fmt.Errorf("property '%s' is below minimum value %v", key, *schema.Minimum)
		}
		if numVal := value.(float64); schema.Maximum != nil && numVal > *schema.Maximum {
			return fmt.Errorf("property '%s' is above maximum value %v", key, *schema.Maximum)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("property '%s' must be a boolean", key)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("property '%s' must be an array", key)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("property '%s' must be an object", key)
		}
	}

	return nil
}

// prepareTemplateData prepares data for template rendering
func (tp *TemplatePlugin) prepareTemplateData(data *ExportData, config ExportConfig) map[string]interface{} {
	templateData := map[string]interface{}{
		"Devices":           data.Devices,
		"Configurations":    data.Configurations,
		"Templates":         data.Templates,
		"DiscoveredDevices": data.DiscoveredDevices,
		"Metadata":          data.Metadata,
		"Timestamp":         data.Timestamp,
		"Config":            config.Config,
		"Options":           config.Options,
		"Format":            config.Format,
	}

	// Add manifest variables if available
	if len(tp.manifest.Variables) > 0 {
		for key, value := range tp.manifest.Variables {
			templateData[key] = value
		}
	}

	return templateData
}

// preparePreviewData prepares limited data for preview
func (tp *TemplatePlugin) preparePreviewData(data *ExportData, config ExportConfig, maxItems int) map[string]interface{} {
	// Limit devices for preview
	devices := data.Devices
	if len(devices) > maxItems {
		devices = devices[:maxItems]
	}

	// Limit other data as well
	configurations := data.Configurations
	if len(configurations) > maxItems {
		configurations = configurations[:maxItems]
	}

	templates := data.Templates
	if len(templates) > maxItems {
		templates = templates[:maxItems]
	}

	discoveredDevices := data.DiscoveredDevices
	if len(discoveredDevices) > maxItems {
		discoveredDevices = discoveredDevices[:maxItems]
	}

	templateData := map[string]interface{}{
		"Devices":           devices,
		"Configurations":    configurations,
		"Templates":         templates,
		"DiscoveredDevices": discoveredDevices,
		"Metadata":          data.Metadata,
		"Timestamp":         data.Timestamp,
		"Config":            config.Config,
		"Options":           config.Options,
		"Format":            config.Format,
		"IsPreview":         true,
		"MaxItems":          maxItems,
	}

	// Add manifest variables if available
	if len(tp.manifest.Variables) > 0 {
		for key, value := range tp.manifest.Variables {
			templateData[key] = value
		}
	}

	return templateData
}

// handleOutput handles the output based on configuration
func (tp *TemplatePlugin) handleOutput(content string, output OutputConfig, result *ExportResult) error {
	switch output.Type {
	case "file":
		if output.Destination == "" {
			return fmt.Errorf("file destination not specified")
		}

		// Write to file
		if err := writeToFile(output.Destination, []byte(content), output.Compression); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}

		result.OutputPath = output.Destination

	case "webhook":
		if output.Webhook == nil {
			return fmt.Errorf("webhook configuration not provided")
		}

		// Send via webhook
		if err := sendWebhook(output.Webhook, []byte(content)); err != nil {
			return fmt.Errorf("failed to send webhook: %w", err)
		}

		result.WebhookSent = true

	case "response":
		// Content is returned in the result
		// No additional handling needed

	default:
		return fmt.Errorf("unsupported output type: %s", output.Type)
	}

	return nil
}

// Helper functions for file and webhook operations
func writeToFile(destination string, content []byte, compression string) error {
	// TODO: Implement file writing with optional compression
	return fmt.Errorf("file writing not implemented")
}

func sendWebhook(config *WebhookConfig, content []byte) error {
	// TODO: Implement webhook sending
	return fmt.Errorf("webhook sending not implemented")
}

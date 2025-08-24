package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins"
	"github.com/ginsys/shelly-manager/internal/plugins/notification"
)

// SimpleNotificationPlugin is an example implementation of a notification plugin
// This plugin demonstrates the basic structure and interfaces required for
// creating new notification plugins.
type SimpleNotificationPlugin struct {
	logger *logging.Logger
	config SimpleNotificationConfig
}

// SimpleNotificationConfig holds configuration for the simple notification plugin
type SimpleNotificationConfig struct {
	OutputFile string `json:"output_file"`
	Format     string `json:"format"` // "text" or "json"
}

// NewSimpleNotificationPlugin creates a new instance of the simple notification plugin
func NewSimpleNotificationPlugin() notification.NotificationPlugin {
	return &SimpleNotificationPlugin{}
}

// Type returns the plugin type (notification)
func (p *SimpleNotificationPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeNotification
}

// Info returns metadata about the plugin
func (p *SimpleNotificationPlugin) Info() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:         "simple-notification",
		Version:      "1.0.0",
		Description:  "A simple file-based notification plugin for demonstration",
		Author:       "Shelly Manager Team",
		License:      "MIT",
		Tags:         []string{"notification", "file", "example", "demo"},
		Category:     plugins.CategoryCustom,
		Dependencies: []string{},
		MinVersion:   "1.0.0",
	}
}

// ConfigSchema returns the configuration schema for this plugin
func (p *SimpleNotificationPlugin) ConfigSchema() plugins.ConfigSchema {
	return plugins.ConfigSchema{
		Version: "1.0",
		Properties: map[string]plugins.PropertySchema{
			"output_file": {
				Type:        "string",
				Description: "File path to write notifications to",
				Default:     "/tmp/notifications.log",
			},
			"format": {
				Type:        "string",
				Description: "Output format for notifications",
				Default:     "text",
				Enum:        []interface{}{"text", "json"},
			},
		},
		Required: []string{"output_file"},
		Examples: []map[string]interface{}{
			{
				"output_file": "/var/log/shelly-notifications.log",
				"format":      "json",
			},
			{
				"output_file": "/tmp/notifications.txt",
				"format":      "text",
			},
		},
	}
}

// ValidateConfig validates the plugin configuration
func (p *SimpleNotificationPlugin) ValidateConfig(config map[string]interface{}) error {
	// Check output file
	if outputFile, exists := config["output_file"]; exists {
		if _, ok := outputFile.(string); !ok {
			return fmt.Errorf("output_file must be a string")
		}
	}

	// Check format
	if format, exists := config["format"]; exists {
		if formatStr, ok := format.(string); ok {
			if formatStr != "text" && formatStr != "json" {
				return fmt.Errorf("format must be 'text' or 'json'")
			}
		} else {
			return fmt.Errorf("format must be a string")
		}
	}

	return nil
}

// Initialize initializes the plugin with logger and configuration
func (p *SimpleNotificationPlugin) Initialize(logger *logging.Logger) error {
	p.logger = logger

	p.logger.Info("Simple notification plugin initialized",
		"plugin", p.Info().Name,
		"version", p.Info().Version,
	)

	return nil
}

// Cleanup cleans up plugin resources
func (p *SimpleNotificationPlugin) Cleanup() error {
	if p.logger != nil {
		p.logger.Info("Simple notification plugin cleaned up")
	}
	return nil
}

// Health returns the current health status of the plugin
func (p *SimpleNotificationPlugin) Health() plugins.HealthStatus {
	status := plugins.HealthStatusHealthy
	message := "Simple notification plugin is healthy"

	// Could add checks here like:
	// - File system permissions
	// - Disk space availability
	// - Configuration validity

	return plugins.HealthStatus{
		Status:      status,
		LastChecked: time.Now(),
		Message:     message,
		Details: map[string]interface{}{
			"logger_initialized": p.logger != nil,
			"output_file":        p.config.OutputFile,
			"format":             p.config.Format,
		},
	}
}

// Send sends a single notification
func (p *SimpleNotificationPlugin) Send(ctx context.Context, notif notification.Notification) (*notification.SendResult, error) {
	startTime := time.Now()

	p.logger.Info("Sending notification",
		"id", notif.ID,
		"type", notif.Type,
		"priority", notif.Priority,
		"recipients", len(notif.Recipients),
	)

	// In a real implementation, you would:
	// 1. Format the notification according to config
	// 2. Write to the specified output file
	// 3. Handle any errors that occur
	// 4. Return appropriate success/failure results

	// For this example, we'll just simulate the operation
	time.Sleep(10 * time.Millisecond) // Simulate processing time

	result := &notification.SendResult{
		Success:     true,
		MessageID:   fmt.Sprintf("simple-%s-%d", notif.ID, time.Now().Unix()),
		DeliveredTo: notif.Recipients,
		FailedTo:    []string{}, // No failures in this example
		Duration:    time.Since(startTime),
		DeliveredAt: time.Now(),
	}

	p.logger.Info("Notification sent successfully",
		"message_id", result.MessageID,
		"duration", result.Duration,
		"delivered_count", len(result.DeliveredTo),
	)

	return result, nil
}

// BatchSend sends multiple notifications in a batch
func (p *SimpleNotificationPlugin) BatchSend(ctx context.Context, notifications []notification.Notification) (*notification.BatchSendResult, error) {
	startTime := time.Now()

	p.logger.Info("Sending notification batch",
		"count", len(notifications),
	)

	results := make([]notification.SendResult, 0, len(notifications))
	totalSent := 0
	totalFailed := 0

	// Send each notification individually
	for _, notif := range notifications {
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing
			break
		default:
			result, err := p.Send(ctx, notif)
			if err != nil {
				totalFailed++
				results = append(results, notification.SendResult{
					Success:  false,
					Error:    err.Error(),
					Duration: time.Since(startTime),
				})
			} else {
				totalSent++
				results = append(results, *result)
			}
		}
	}

	batchResult := &notification.BatchSendResult{
		Success:     totalFailed == 0,
		TotalSent:   totalSent,
		TotalFailed: totalFailed,
		Results:     results,
		Duration:    time.Since(startTime),
		CompletedAt: time.Now(),
	}

	p.logger.Info("Notification batch completed",
		"total_sent", batchResult.TotalSent,
		"total_failed", batchResult.TotalFailed,
		"duration", batchResult.Duration,
	)

	return batchResult, nil
}

// TestConnection tests the plugin configuration and connectivity
func (p *SimpleNotificationPlugin) TestConnection(ctx context.Context, config map[string]interface{}) error {
	// Validate the configuration
	if err := p.ValidateConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// In a real plugin, you might:
	// - Test file system permissions
	// - Verify network connectivity
	// - Authenticate with external services
	// - Check required dependencies

	p.logger.Info("Connection test passed for simple notification plugin")
	return nil
}

// GetSupportedTypes returns the notification types this plugin supports
func (p *SimpleNotificationPlugin) GetSupportedTypes() []notification.NotificationType {
	return []notification.NotificationType{
		notification.NotificationTypeEmail,   // Example: could format as email-style
		notification.NotificationTypeWebhook, // Example: could format as webhook payload
	}
}

// Capabilities returns the capabilities of this notification plugin
func (p *SimpleNotificationPlugin) Capabilities() notification.NotificationCapabilities {
	return notification.NotificationCapabilities{
		SupportedTypes:        p.GetSupportedTypes(),
		SupportsBatchSend:     true,
		SupportsScheduling:    false, // This plugin doesn't support scheduling
		SupportsTemplates:     false, // This plugin doesn't support templates
		SupportsAttachments:   false, // This plugin doesn't support attachments
		SupportsMarkdown:      true,  // Could format markdown in text output
		SupportsHTML:          false, // This plugin doesn't support HTML
		MaxRecipientsPerBatch: 100,   // Arbitrary limit for demonstration
		MaxMessageSize:        10240, // 10KB limit for demonstration
		RateLimits: notification.RateLimits{
			RequestsPerMinute: 60,    // 1 per second
			RequestsPerHour:   3600,  // Same rate
			RequestsPerDay:    86400, // Same rate
			BurstLimit:        10,    // Allow short bursts
			ResetInterval:     time.Minute,
		},
		RequiresAuthentication: false, // File-based plugin doesn't need auth
		SupportsDeliveryStatus: false, // This plugin doesn't track delivery
	}
}

// Ensure SimpleNotificationPlugin implements the NotificationPlugin interface
var _ notification.NotificationPlugin = (*SimpleNotificationPlugin)(nil)
var _ plugins.Plugin = (*SimpleNotificationPlugin)(nil)

// Usage example:
//
// func main() {
//     // Create plugin registry
//     registry := plugins.NewRegistry(logger)
//
//     // Create and register plugin
//     plugin := NewSimpleNotificationPlugin()
//     err := registry.RegisterPlugin(plugin)
//     if err != nil {
//         log.Fatal("Failed to register plugin:", err)
//     }
//
//     // Use the plugin
//     notif := notification.Notification{
//         ID:       "test-1",
//         Type:     notification.NotificationTypeEmail,
//         Priority: notification.PriorityNormal,
//         Subject:  "Test Notification",
//         Message:  "This is a test notification",
//         Recipients: []string{"user@example.com"},
//     }
//
//     result, err := plugin.Send(context.Background(), notif)
//     if err != nil {
//         log.Fatal("Failed to send notification:", err)
//     }
//
//     log.Printf("Notification sent: %+v", result)
// }

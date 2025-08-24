package notification

import (
	"context"
	"time"

	"github.com/ginsys/shelly-manager/internal/plugins"
)

// NotificationPlugin extends the base Plugin interface for notification operations
type NotificationPlugin interface {
	plugins.Plugin

	// Send sends a notification
	Send(ctx context.Context, notification Notification) (*SendResult, error)

	// Batch sends multiple notifications
	BatchSend(ctx context.Context, notifications []Notification) (*BatchSendResult, error)

	// Test tests the notification configuration
	TestConnection(ctx context.Context, config map[string]interface{}) error

	// GetSupportedTypes returns the notification types this plugin supports
	GetSupportedTypes() []NotificationType

	// Notification-specific capabilities
	Capabilities() NotificationCapabilities
}

// NotificationType defines the type of notification
type NotificationType string

const (
	NotificationTypeEmail    NotificationType = "email"
	NotificationTypeSMS      NotificationType = "sms"
	NotificationTypeSlack    NotificationType = "slack"
	NotificationTypeDiscord  NotificationType = "discord"
	NotificationTypeTelegram NotificationType = "telegram"
	NotificationTypeWebhook  NotificationType = "webhook"
	NotificationTypePush     NotificationType = "push"
)

// NotificationPriority defines the priority level of a notification
type NotificationPriority string

const (
	PriorityLow      NotificationPriority = "low"
	PriorityNormal   NotificationPriority = "normal"
	PriorityHigh     NotificationPriority = "high"
	PriorityCritical NotificationPriority = "critical"
)

// Notification represents a notification to be sent
type Notification struct {
	ID          string                 `json:"id"`
	Type        NotificationType       `json:"type"`
	Priority    NotificationPriority   `json:"priority"`
	Subject     string                 `json:"subject"`
	Message     string                 `json:"message"`
	Recipients  []string               `json:"recipients"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	RetryCount  int                    `json:"retry_count,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
}

// SendResult contains the result of a notification send operation
type SendResult struct {
	Success     bool                   `json:"success"`
	MessageID   string                 `json:"message_id,omitempty"`
	DeliveredTo []string               `json:"delivered_to"`
	FailedTo    []string               `json:"failed_to"`
	Duration    time.Duration          `json:"duration"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RetryAfter  *time.Time             `json:"retry_after,omitempty"`
	RateLimited bool                   `json:"rate_limited,omitempty"`
	DeliveredAt time.Time              `json:"delivered_at"`
}

// BatchSendResult contains the result of a batch notification send operation
type BatchSendResult struct {
	Success     bool                   `json:"success"`
	TotalSent   int                    `json:"total_sent"`
	TotalFailed int                    `json:"total_failed"`
	Results     []SendResult           `json:"results"`
	Duration    time.Duration          `json:"duration"`
	Errors      []string               `json:"errors,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CompletedAt time.Time              `json:"completed_at"`
}

// NotificationCapabilities describes what a notification plugin can do
type NotificationCapabilities struct {
	SupportedTypes         []NotificationType `json:"supported_types"`
	SupportsBatchSend      bool               `json:"supports_batch_send"`
	SupportsScheduling     bool               `json:"supports_scheduling"`
	SupportsTemplates      bool               `json:"supports_templates"`
	SupportsAttachments    bool               `json:"supports_attachments"`
	SupportsMarkdown       bool               `json:"supports_markdown"`
	SupportsHTML           bool               `json:"supports_html"`
	MaxRecipientsPerBatch  int                `json:"max_recipients_per_batch"`
	MaxMessageSize         int64              `json:"max_message_size"`
	RateLimits             RateLimits         `json:"rate_limits"`
	RequiresAuthentication bool               `json:"requires_authentication"`
	SupportsDeliveryStatus bool               `json:"supports_delivery_status"`
}

// RateLimits defines rate limiting information for a notification plugin
type RateLimits struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	RequestsPerHour   int           `json:"requests_per_hour"`
	RequestsPerDay    int           `json:"requests_per_day"`
	BurstLimit        int           `json:"burst_limit"`
	ResetInterval     time.Duration `json:"reset_interval"`
}

// NotificationTemplate represents a notification template
type NotificationTemplate struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      NotificationType  `json:"type"`
	Subject   string            `json:"subject"`
	Message   string            `json:"message"`
	Variables []string          `json:"variables"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Example notification plugins that could be implemented:

// EmailNotificationPlugin would implement NotificationPlugin for email notifications
// SlackNotificationPlugin would implement NotificationPlugin for Slack notifications
// WebhookNotificationPlugin would implement NotificationPlugin for webhook notifications
// SMSNotificationPlugin would implement NotificationPlugin for SMS notifications
// PushNotificationPlugin would implement NotificationPlugin for push notifications

// Future features to consider:
// - Template management and rendering
// - Delivery status tracking and webhooks
// - Message queuing and retry logic
// - Multi-channel notification campaigns
// - A/B testing for notification content
// - Analytics and engagement tracking
// - Unsubscribe management
// - Notification preferences per user

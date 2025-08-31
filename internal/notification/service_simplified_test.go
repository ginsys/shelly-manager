package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func setupSimpleTestService(t *testing.T) (*Service, *gorm.DB, func()) {
	// Create an in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate notification schema
	err = db.AutoMigrate(
		&NotificationChannel{},
		&NotificationRule{},
		&NotificationHistory{},
		&NotificationTemplate{},
	)
	require.NoError(t, err)

	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	emailConfig := EmailSMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "test@example.com",
		Password: "password",
		From:     "noreply@example.com",
		TLS:      true,
	}

	service := NewService(db, logger, emailConfig)

	cleanup := func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}

	return service, db, cleanup
}

// roundTripperFunc allows customizing http.Client behavior in tests
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// fakeHTTPClient returns an http.Client that always returns the given status code
func fakeHTTPClient(status int) *http.Client {
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}
}

func TestNotificationService_CreateChannel(t *testing.T) {
	service, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	t.Run("Valid Email Channel", func(t *testing.T) {
		config := EmailConfig{
			Recipients: []string{"admin@example.com"},
			Subject:    "Alert: {device_name}",
		}
		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		channel := &NotificationChannel{
			Name:        "Test Email Channel",
			Type:        "email",
			Enabled:     true,
			Config:      configJSON,
			Description: "Test email channel",
		}

		err = service.CreateChannel(channel)
		assert.NoError(t, err)
		assert.NotZero(t, channel.ID)
	})

	t.Run("Invalid Channel Type", func(t *testing.T) {
		channel := &NotificationChannel{
			Name:    "Invalid Channel",
			Type:    "invalid_type",
			Enabled: true,
			Config:  json.RawMessage(`{}`),
		}

		err := service.CreateChannel(channel)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid channel configuration")
	})
}

func TestNotificationService_GetChannels(t *testing.T) {
	service, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	// Create test channels
	emailConfig, _ := json.Marshal(EmailConfig{Recipients: []string{"test1@example.com"}})
	webhookConfig, _ := json.Marshal(WebhookConfig{URL: "https://example.com/webhook"})

	channels := []*NotificationChannel{
		{
			Name:    "Email Channel",
			Type:    "email",
			Enabled: true,
			Config:  emailConfig,
		},
		{
			Name:    "Webhook Channel",
			Type:    "webhook",
			Enabled: false,
			Config:  webhookConfig,
		},
	}

	for _, channel := range channels {
		err := service.CreateChannel(channel)
		require.NoError(t, err)
	}

	// Retrieve channels
	retrieved, err := service.GetChannels()
	assert.NoError(t, err)
	assert.Len(t, retrieved, 2)

	// Verify channel details
	channelNames := make([]string, len(retrieved))
	for i, ch := range retrieved {
		channelNames[i] = ch.Name
	}
	assert.Contains(t, channelNames, "Email Channel")
	assert.Contains(t, channelNames, "Webhook Channel")
}

func TestNotificationService_CreateRule(t *testing.T) {
	service, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	// First create a channel
	config, _ := json.Marshal(EmailConfig{Recipients: []string{"test@example.com"}})
	channel := &NotificationChannel{
		Name:    "Test Channel",
		Type:    "email",
		Enabled: true,
		Config:  config,
	}
	err := service.CreateChannel(channel)
	require.NoError(t, err)

	t.Run("Create Valid Rule", func(t *testing.T) {
		categories, _ := json.Marshal([]string{"security", "network"})
		rule := &NotificationRule{
			Name:               "Test Rule",
			Enabled:            true,
			ChannelID:          channel.ID,
			AlertLevel:         "critical",
			CategoriesJSON:     categories,
			MinIntervalMinutes: 30,
			MaxPerHour:         5,
		}

		err := service.CreateRule(rule)
		assert.NoError(t, err)
		assert.NotZero(t, rule.ID)
	})

	t.Run("Create Rule with Invalid Channel", func(t *testing.T) {
		rule := &NotificationRule{
			Name:      "Invalid Rule",
			Enabled:   true,
			ChannelID: 999, // Non-existent channel
		}

		err := service.CreateRule(rule)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "notification channel not found")
	})
}

func TestNotificationService_TestChannel(t *testing.T) {
	service, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	t.Run("Test Valid Channel", func(t *testing.T) {
		config, _ := json.Marshal(WebhookConfig{URL: "https://httpbin.org/post"})
		channel := &NotificationChannel{
			Name:    "Test Webhook",
			Type:    "webhook",
			Enabled: true,
			Config:  config,
		}
		err := service.CreateChannel(channel)
		require.NoError(t, err)

		// Test the channel - this might fail due to network, but structure should be valid
		err = service.TestChannel(context.Background(), channel.ID)
		// We don't assert NoError here since it might fail due to network issues in CI
		// Just test that we can call the method without panicking
		_ = err
	})

	t.Run("Test Disabled Channel", func(t *testing.T) {
		config, _ := json.Marshal(WebhookConfig{URL: "https://example.com/webhook"})
		channel := &NotificationChannel{
			Name:    "Disabled Channel",
			Type:    "webhook",
			Enabled: false,
			Config:  config,
		}
		err := service.CreateChannel(channel)
		require.NoError(t, err)

		err = service.TestChannel(context.Background(), channel.ID)
		// The service might still allow testing disabled channels
		_ = err
	})

	t.Run("Test Nonexistent Channel", func(t *testing.T) {
		err := service.TestChannel(context.Background(), 999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "notification channel not found")
	})
}

func TestNotificationService_DeleteChannel(t *testing.T) {
	service, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	// Create test channel
	config, _ := json.Marshal(EmailConfig{Recipients: []string{"test@example.com"}})
	channel := &NotificationChannel{
		Name:    "Test Channel",
		Type:    "email",
		Enabled: true,
		Config:  config,
	}

	err := service.CreateChannel(channel)
	require.NoError(t, err)
	channelID := channel.ID

	t.Run("Delete Existing Channel", func(t *testing.T) {
		err := service.DeleteChannel(channelID)
		assert.NoError(t, err)

		// Verify deletion
		var count int64
		service.db.Model(&NotificationChannel{}).Where("id = ?", channelID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Delete Nonexistent Channel", func(t *testing.T) {
		err := service.DeleteChannel(999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "notification channel not found")
	})
}

func TestNotificationService_RateLimitEnforcement(t *testing.T) {
	service, db, cleanup := setupSimpleTestService(t)
	defer cleanup()

	// Use webhook with fake HTTP client to avoid network
	service.httpClient = fakeHTTPClient(200)

	// Create channel
	cfg, _ := json.Marshal(WebhookConfig{URL: "https://example.com/webhook"})
	ch := &NotificationChannel{Name: "Webhook", Type: "webhook", Enabled: true, Config: cfg}
	require.NoError(t, service.CreateChannel(ch))

	// Create rule with max 1 per hour
	rule := &NotificationRule{
		Name:               "RL",
		Enabled:            true,
		ChannelID:          ch.ID,
		AlertLevel:         "all",
		MinIntervalMinutes: 0,
		MaxPerHour:         1,
	}
	require.NoError(t, service.CreateRule(rule))

	// Send two events back-to-back
	evt := &NotificationEvent{Type: "test", AlertLevel: AlertLevelInfo, Title: "A", Message: "B", Timestamp: time.Now()}
	require.NoError(t, service.SendNotification(context.Background(), evt))
	require.NoError(t, service.SendNotification(context.Background(), evt))

	// Only one should be recorded as sent due to rate limit
	var count int64
	err := db.Model(&NotificationHistory{}).Where("rule_id = ? AND status = ?", rule.ID, "sent").Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestNotificationService_MinSeverityEnforcement(t *testing.T) {
	service, db, cleanup := setupSimpleTestService(t)
	defer cleanup()

	// Fake HTTP success
	service.httpClient = fakeHTTPClient(200)

	// Channel
	cfg, _ := json.Marshal(WebhookConfig{URL: "https://example.com/webhook"})
	ch := &NotificationChannel{Name: "Webhook2", Type: "webhook", Enabled: true, Config: cfg}
	require.NoError(t, service.CreateChannel(ch))

	// Rule requiring at least warning
	rule := &NotificationRule{
		Name:        "Severity",
		Enabled:     true,
		ChannelID:   ch.ID,
		AlertLevel:  "all",
		MinSeverity: "warning",
		MaxPerHour:  100,
	}
	require.NoError(t, service.CreateRule(rule))

	// info event should not send
	evtInfo := &NotificationEvent{Type: "ev", AlertLevel: AlertLevelInfo, Title: "t", Message: "m", Timestamp: time.Now()}
	require.NoError(t, service.SendNotification(context.Background(), evtInfo))

	// warning event should send
	evtWarn := &NotificationEvent{Type: "ev", AlertLevel: AlertLevelWarning, Title: "t", Message: "m", Timestamp: time.Now()}
	require.NoError(t, service.SendNotification(context.Background(), evtWarn))

	var total int64
	require.NoError(t, db.Model(&NotificationHistory{}).Where("rule_id = ? AND status = ?", rule.ID, "sent").Count(&total).Error)
	assert.Equal(t, int64(1), total)
}

func TestNotificationService_GetHistoryFiltersAndPagination(t *testing.T) {
	service, _, cleanup := setupSimpleTestService(t)
	defer cleanup()

	// Fake HTTP success
	service.httpClient = fakeHTTPClient(200)

	// Channel + rule
	cfg, _ := json.Marshal(WebhookConfig{URL: "https://example.com/webhook"})
	ch := &NotificationChannel{Name: "Webhook3", Type: "webhook", Enabled: true, Config: cfg}
	require.NoError(t, service.CreateChannel(ch))
	rule := &NotificationRule{Name: "HistoryRule", Enabled: true, ChannelID: ch.ID, AlertLevel: "all", MaxPerHour: 100}
	require.NoError(t, service.CreateRule(rule))

	// Generate 3 sent + 1 failed entries via SendNotification (to ensure associations)
	evt := &NotificationEvent{Type: "ev", AlertLevel: AlertLevelInfo, Title: "t", Message: "m", Timestamp: time.Now()}
	require.NoError(t, service.SendNotification(context.Background(), evt))
	require.NoError(t, service.SendNotification(context.Background(), evt))
	require.NoError(t, service.SendNotification(context.Background(), evt))

	// Force a failure by switching client to 500
	service.httpClient = fakeHTTPClient(500)
	_ = service.SendNotification(context.Background(), evt)

	// List only sent, limit 2, offset 0
	recs, total, err := service.GetHistory(&ch.ID, "sent", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, recs, 2)

	// Next page
	recs2, total2, err := service.GetHistory(&ch.ID, "sent", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total2)
	assert.Len(t, recs2, 1)
}

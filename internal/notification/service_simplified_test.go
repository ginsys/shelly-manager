package notification

import (
	"context"
	"encoding/json"
	"testing"

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

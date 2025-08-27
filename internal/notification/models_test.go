package notification

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationChannel(t *testing.T) {
	t.Run("Valid NotificationChannel", func(t *testing.T) {
		config := EmailConfig{
			Recipients: []string{"admin@example.com", "support@example.com"},
			Subject:    "Alert: {device_name}",
			Template:   "device_alert",
		}

		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		channel := &NotificationChannel{
			Name:        "Email Alert Channel",
			Type:        "email",
			Enabled:     true,
			Config:      configJSON,
			Description: "Email notifications for critical alerts",
		}

		assert.Equal(t, "Email Alert Channel", channel.Name)
		assert.Equal(t, "email", channel.Type)
		assert.True(t, channel.Enabled)
		assert.NotEmpty(t, channel.Config)
		assert.Contains(t, string(channel.Config), "admin@example.com")
	})

	t.Run("Webhook Channel Config", func(t *testing.T) {
		config := WebhookConfig{
			URL:     "https://api.example.com/webhooks/alerts",
			Method:  "POST",
			Headers: map[string]string{"Content-Type": "application/json"},
			Secret:  "webhook-secret-key",
			Timeout: 30,
			Retries: 3,
		}

		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		channel := &NotificationChannel{
			Name:    "Webhook Alert Channel",
			Type:    "webhook",
			Enabled: true,
			Config:  configJSON,
		}

		// Verify we can unmarshal the config back
		var parsedConfig WebhookConfig
		err = json.Unmarshal(channel.Config, &parsedConfig)
		require.NoError(t, err)
		assert.Equal(t, config.URL, parsedConfig.URL)
		assert.Equal(t, config.Method, parsedConfig.Method)
		assert.Equal(t, config.Secret, parsedConfig.Secret)
		assert.Equal(t, 30, parsedConfig.Timeout)
		assert.Equal(t, 3, parsedConfig.Retries)
	})

	t.Run("Slack Channel Config", func(t *testing.T) {
		config := SlackConfig{
			WebhookURL: "https://hooks.slack.com/services/ABC123/DEF456/xyz789",
			Channel:    "#alerts",
			Username:   "Shelly Manager",
			IconEmoji:  ":warning:",
			Template:   "slack_alert",
		}

		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		channel := &NotificationChannel{
			Name:    "Slack Alert Channel",
			Type:    "slack",
			Enabled: true,
			Config:  configJSON,
		}

		var parsedConfig SlackConfig
		err = json.Unmarshal(channel.Config, &parsedConfig)
		require.NoError(t, err)
		assert.Equal(t, config.WebhookURL, parsedConfig.WebhookURL)
		assert.Equal(t, config.Channel, parsedConfig.Channel)
		assert.Equal(t, config.Username, parsedConfig.Username)
		assert.Equal(t, config.IconEmoji, parsedConfig.IconEmoji)
	})
}

func TestNotificationRule(t *testing.T) {
	t.Run("Valid Notification Rule", func(t *testing.T) {
		categories := []string{"security", "network", "device"}
		categoriesJSON, err := json.Marshal(categories)
		require.NoError(t, err)

		scheduleDays := []string{"monday", "tuesday", "wednesday", "thursday", "friday"}
		scheduleDaysJSON, err := json.Marshal(scheduleDays)
		require.NoError(t, err)

		deviceFilter := DeviceFilter{
			DeviceTypes: []string{"SHSW-1", "SHSW-25"},
			Generations: []int{1, 2},
			IPRanges:    []string{"192.168.1.0/24"},
			Exclude:     false,
		}
		deviceFilterJSON, err := json.Marshal(deviceFilter)
		require.NoError(t, err)

		rule := &NotificationRule{
			Name:               "Critical Security Alerts",
			Description:        "Send notifications for critical security events",
			Enabled:            true,
			ChannelID:          1,
			AlertLevel:         "critical",
			MinSeverity:        "critical",
			CategoriesJSON:     categoriesJSON,
			DeviceFilter:       deviceFilterJSON,
			MinIntervalMinutes: 15,
			MaxPerHour:         10,
			ScheduleEnabled:    true,
			ScheduleStart:      "09:00",
			ScheduleEnd:        "17:00",
			ScheduleDaysJSON:   scheduleDaysJSON,
		}

		assert.Equal(t, "Critical Security Alerts", rule.Name)
		assert.True(t, rule.Enabled)
		assert.Equal(t, uint(1), rule.ChannelID)
		assert.Equal(t, "critical", rule.AlertLevel)
		assert.Equal(t, 15, rule.MinIntervalMinutes)
		assert.Equal(t, 10, rule.MaxPerHour)
		assert.True(t, rule.ScheduleEnabled)
	})
}

func TestNotificationHistory(t *testing.T) {
	t.Run("Notification History Creation", func(t *testing.T) {
		affectedDevices := []uint{1, 2, 3}
		affectedDevicesJSON, err := json.Marshal(affectedDevices)
		require.NoError(t, err)

		sentAt := time.Now()
		history := &NotificationHistory{
			RuleID:              1,
			ChannelID:           1,
			TriggerType:         "drift_detected",
			DeviceID:            &[]uint{1}[0],
			Subject:             "Critical Alert: Device Offline",
			Message:             "Device 'Living Room Switch' has gone offline",
			AlertLevel:          "critical",
			AffectedDevicesJSON: affectedDevicesJSON,
			Status:              "sent",
			SentAt:              &sentAt,
			RetryCount:          0,
		}

		assert.Equal(t, uint(1), history.RuleID)
		assert.Equal(t, uint(1), history.ChannelID)
		assert.Equal(t, "drift_detected", history.TriggerType)
		assert.NotNil(t, history.DeviceID)
		assert.Equal(t, uint(1), *history.DeviceID)
		assert.Equal(t, "sent", history.Status)
		assert.NotNil(t, history.SentAt)
		assert.Equal(t, 0, history.RetryCount)
	})

	t.Run("Failed Notification with Retry", func(t *testing.T) {
		nextRetry := time.Now().Add(5 * time.Minute)
		history := &NotificationHistory{
			RuleID:      1,
			ChannelID:   1,
			TriggerType: "schedule_run",
			Subject:     "Scheduled Report",
			Message:     "Weekly device status report",
			AlertLevel:  "info",
			Status:      "failed",
			Error:       "Connection timeout",
			RetryCount:  1,
			NextRetryAt: &nextRetry,
		}

		assert.Equal(t, "failed", history.Status)
		assert.Equal(t, "Connection timeout", history.Error)
		assert.Equal(t, 1, history.RetryCount)
		assert.NotNil(t, history.NextRetryAt)
	})
}

func TestNotificationEvent(t *testing.T) {
	t.Run("Security Event", func(t *testing.T) {
		event := &NotificationEvent{
			Type:            "security_breach",
			AlertLevel:      AlertLevelCritical,
			DeviceID:        &[]uint{123}[0],
			DeviceName:      "Front Door Sensor",
			Title:           "Unauthorized Access Attempt",
			Message:         "Multiple failed authentication attempts detected",
			Timestamp:       time.Now(),
			AffectedDevices: []uint{123, 124},
			Categories:      []string{"security", "authentication"},
			Metadata: map[string]interface{}{
				"attempts":   5,
				"source_ip":  "192.168.1.100",
				"user_agent": "automated-scanner",
			},
		}

		assert.Equal(t, "security_breach", event.Type)
		assert.Equal(t, AlertLevelCritical, event.AlertLevel)
		assert.NotNil(t, event.DeviceID)
		assert.Equal(t, uint(123), *event.DeviceID)
		assert.Equal(t, "Front Door Sensor", event.DeviceName)
		assert.Contains(t, event.Categories, "security")
		assert.Contains(t, event.Categories, "authentication")
		assert.Equal(t, 5, event.Metadata["attempts"])
	})

	t.Run("Device Event", func(t *testing.T) {
		event := &NotificationEvent{
			Type:            "device_offline",
			AlertLevel:      AlertLevelWarning,
			DeviceID:        &[]uint{456}[0],
			DeviceName:      "Kitchen Light Switch",
			Title:           "Device Offline",
			Message:         "Device has not responded to ping for 5 minutes",
			Timestamp:       time.Now(),
			AffectedDevices: []uint{456},
			Categories:      []string{"device", "connectivity"},
			Metadata: map[string]interface{}{
				"last_seen":        time.Now().Add(-5 * time.Minute).Unix(),
				"ping_failures":    3,
				"device_type":      "SHSW-1",
				"firmware_version": "20210226-100539/v1.10.1@57ac4ad8",
			},
		}

		assert.Equal(t, "device_offline", event.Type)
		assert.Equal(t, AlertLevelWarning, event.AlertLevel)
		assert.Equal(t, "Kitchen Light Switch", event.DeviceName)
		assert.Contains(t, event.Categories, "device")
		assert.Equal(t, 3, event.Metadata["ping_failures"])
	})
}

func TestDeviceFilter(t *testing.T) {
	t.Run("Include Filter", func(t *testing.T) {
		filter := DeviceFilter{
			DeviceIDs:   []uint{1, 2, 3},
			DeviceTypes: []string{"SHSW-1", "SHSW-25"},
			DeviceNames: []string{"Living Room*", "*Light*"},
			Generations: []int{1},
			IPRanges:    []string{"192.168.1.0/24", "10.0.0.0/16"},
			Exclude:     false,
		}

		assert.Contains(t, filter.DeviceIDs, uint(1))
		assert.Contains(t, filter.DeviceTypes, "SHSW-1")
		assert.Contains(t, filter.DeviceNames, "Living Room*")
		assert.Contains(t, filter.Generations, 1)
		assert.Contains(t, filter.IPRanges, "192.168.1.0/24")
		assert.False(t, filter.Exclude)
	})

	t.Run("Exclude Filter", func(t *testing.T) {
		filter := DeviceFilter{
			DeviceTypes: []string{"SHSW-PM"},
			IPRanges:    []string{"192.168.2.0/24"},
			Exclude:     true,
		}

		assert.Contains(t, filter.DeviceTypes, "SHSW-PM")
		assert.True(t, filter.Exclude)
	})
}

func TestRateLimitState(t *testing.T) {
	t.Run("Rate Limit State", func(t *testing.T) {
		now := time.Now()
		state := &RateLimitState{
			RuleID:        1,
			LastSentAt:    now,
			HourlyCount:   5,
			HourlyResetAt: now.Add(time.Hour),
		}

		assert.Equal(t, uint(1), state.RuleID)
		assert.Equal(t, 5, state.HourlyCount)
		assert.True(t, state.HourlyResetAt.After(state.LastSentAt))
	})
}

func TestAlertLevel(t *testing.T) {
	t.Run("Alert Level Constants", func(t *testing.T) {
		assert.Equal(t, AlertLevel("critical"), AlertLevelCritical)
		assert.Equal(t, AlertLevel("warning"), AlertLevelWarning)
		assert.Equal(t, AlertLevel("info"), AlertLevelInfo)
	})
}

func TestNotificationTemplate(t *testing.T) {
	t.Run("Email Template", func(t *testing.T) {
		variables := []string{"device_name", "alert_level", "timestamp", "message"}
		variablesJSON, err := json.Marshal(variables)
		require.NoError(t, err)

		template := &NotificationTemplate{
			Name:          "Default Email Alert",
			Type:          "email",
			Subject:       "Alert: {device_name} - {alert_level}",
			Body:          "Device: {device_name}\nAlert Level: {alert_level}\nTime: {timestamp}\nMessage: {message}",
			VariablesJSON: variablesJSON,
			Description:   "Default template for email alerts",
			IsDefault:     true,
		}

		assert.Equal(t, "Default Email Alert", template.Name)
		assert.Equal(t, "email", template.Type)
		assert.Contains(t, template.Subject, "{device_name}")
		assert.Contains(t, template.Body, "{alert_level}")
		assert.True(t, template.IsDefault)
	})

	t.Run("Slack Template", func(t *testing.T) {
		variables := []string{"device_name", "alert_level", "message"}
		variablesJSON, err := json.Marshal(variables)
		require.NoError(t, err)

		template := &NotificationTemplate{
			Name:          "Slack Alert Template",
			Type:          "slack",
			Subject:       "", // Slack doesn't use subject
			Body:          ":warning: *{alert_level}* Alert\n*Device:* {device_name}\n*Message:* {message}",
			VariablesJSON: variablesJSON,
			Description:   "Slack-formatted alert template",
			IsDefault:     false,
		}

		assert.Equal(t, "slack", template.Type)
		assert.Empty(t, template.Subject)
		assert.Contains(t, template.Body, ":warning:")
		assert.False(t, template.IsDefault)
	})
}

func TestJSONSerialization(t *testing.T) {
	t.Run("DeviceFilter JSON", func(t *testing.T) {
		filter := DeviceFilter{
			DeviceIDs:   []uint{1, 2, 3},
			DeviceTypes: []string{"SHSW-1"},
			Exclude:     false,
		}

		data, err := json.Marshal(filter)
		require.NoError(t, err)

		var unmarshaled DeviceFilter
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, filter.DeviceIDs, unmarshaled.DeviceIDs)
		assert.Equal(t, filter.DeviceTypes, unmarshaled.DeviceTypes)
		assert.Equal(t, filter.Exclude, unmarshaled.Exclude)
	})

	t.Run("NotificationEvent JSON", func(t *testing.T) {
		event := NotificationEvent{
			Type:       "test_event",
			AlertLevel: AlertLevelInfo,
			DeviceID:   &[]uint{123}[0],
			Title:      "Test Event",
			Message:    "This is a test",
			Timestamp:  time.Now().Truncate(time.Second), // Truncate for JSON comparison
			Metadata:   map[string]interface{}{"key": "value"},
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var unmarshaled NotificationEvent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, event.Type, unmarshaled.Type)
		assert.Equal(t, event.AlertLevel, unmarshaled.AlertLevel)
		assert.Equal(t, *event.DeviceID, *unmarshaled.DeviceID)
		assert.Equal(t, event.Title, unmarshaled.Title)
		assert.Equal(t, event.Message, unmarshaled.Message)
		assert.Equal(t, event.Timestamp, unmarshaled.Timestamp)
		assert.Equal(t, event.Metadata["key"], unmarshaled.Metadata["key"])
	})
}

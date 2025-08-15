package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"gorm.io/gorm"
)

// Service handles notification operations
type Service struct {
	db          *gorm.DB
	logger      *logging.Logger
	rateLimits  map[uint]*RateLimitState
	rateLimitMu sync.RWMutex
	httpClient  *http.Client

	// Configuration
	emailConfig EmailSMTPConfig
}

// EmailSMTPConfig represents SMTP configuration
type EmailSMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	TLS      bool   `json:"tls"`
}

// NewService creates a new notification service
func NewService(db *gorm.DB, logger *logging.Logger, emailConfig EmailSMTPConfig) *Service {
	return &Service{
		db:          db,
		logger:      logger,
		rateLimits:  make(map[uint]*RateLimitState),
		emailConfig: emailConfig,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateChannel creates a new notification channel
func (s *Service) CreateChannel(channel *NotificationChannel) error {
	if err := s.validateChannelConfig(channel); err != nil {
		return fmt.Errorf("invalid channel configuration: %w", err)
	}

	if err := s.db.Create(channel).Error; err != nil {
		return fmt.Errorf("failed to create notification channel: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"channel_id":   channel.ID,
		"channel_name": channel.Name,
		"channel_type": channel.Type,
		"component":    "notification",
	}).Info("Created notification channel")

	return nil
}

// GetChannels retrieves all notification channels
func (s *Service) GetChannels() ([]NotificationChannel, error) {
	var channels []NotificationChannel
	if err := s.db.Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("failed to get notification channels: %w", err)
	}
	return channels, nil
}

// UpdateChannel updates an existing notification channel
func (s *Service) UpdateChannel(channelID uint, updates *NotificationChannel) error {
	if err := s.validateChannelConfig(updates); err != nil {
		return fmt.Errorf("invalid channel configuration: %w", err)
	}

	result := s.db.Model(&NotificationChannel{}).Where("id = ?", channelID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update notification channel: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("notification channel not found")
	}

	s.logger.WithFields(map[string]any{
		"channel_id": channelID,
		"component":  "notification",
	}).Info("Updated notification channel")

	return nil
}

// DeleteChannel deletes a notification channel
func (s *Service) DeleteChannel(channelID uint) error {
	// Check if channel is used by any rules
	var ruleCount int64
	if err := s.db.Model(&NotificationRule{}).Where("channel_id = ?", channelID).Count(&ruleCount).Error; err != nil {
		return fmt.Errorf("failed to check channel usage: %w", err)
	}

	if ruleCount > 0 {
		return fmt.Errorf("cannot delete channel: used by %d notification rule(s)", ruleCount)
	}

	result := s.db.Delete(&NotificationChannel{}, channelID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete notification channel: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("notification channel not found")
	}

	s.logger.WithFields(map[string]any{
		"channel_id": channelID,
		"component":  "notification",
	}).Info("Deleted notification channel")

	return nil
}

// CreateRule creates a new notification rule
func (s *Service) CreateRule(rule *NotificationRule) error {
	// Validate channel exists
	var channel NotificationChannel
	if err := s.db.First(&channel, rule.ChannelID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("notification channel not found")
		}
		return fmt.Errorf("failed to validate channel: %w", err)
	}

	// Serialize JSON fields
	if categoriesJSON, err := json.Marshal(rule.Categories); err == nil {
		rule.CategoriesJSON = categoriesJSON
	}
	if daysJSON, err := json.Marshal(rule.ScheduleDays); err == nil {
		rule.ScheduleDaysJSON = daysJSON
	}

	if err := s.db.Create(rule).Error; err != nil {
		return fmt.Errorf("failed to create notification rule: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"rule_id":    rule.ID,
		"rule_name":  rule.Name,
		"channel_id": rule.ChannelID,
		"component":  "notification",
	}).Info("Created notification rule")

	return nil
}

// GetRules retrieves all notification rules
func (s *Service) GetRules() ([]NotificationRule, error) {
	var rules []NotificationRule
	if err := s.db.Preload("Channel").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to get notification rules: %w", err)
	}

	// Deserialize JSON fields
	for i := range rules {
		if len(rules[i].CategoriesJSON) > 0 {
			json.Unmarshal(rules[i].CategoriesJSON, &rules[i].Categories)
		}
		if len(rules[i].ScheduleDaysJSON) > 0 {
			json.Unmarshal(rules[i].ScheduleDaysJSON, &rules[i].ScheduleDays)
		}
	}

	return rules, nil
}

// SendNotification processes a notification event and sends to matching rules
func (s *Service) SendNotification(ctx context.Context, event *NotificationEvent) error {
	s.logger.WithFields(map[string]any{
		"event_type":  event.Type,
		"alert_level": event.AlertLevel,
		"device_id":   event.DeviceID,
		"device_name": event.DeviceName,
		"component":   "notification",
	}).Info("Processing notification event")

	// Get all enabled rules
	rules, err := s.getMatchingRules(event)
	if err != nil {
		return fmt.Errorf("failed to get matching rules: %w", err)
	}

	if len(rules) == 0 {
		s.logger.WithFields(map[string]any{
			"event_type": event.Type,
			"component":  "notification",
		}).Debug("No matching notification rules found")
		return nil
	}

	// Send notification for each matching rule
	for _, rule := range rules {
		if err := s.sendNotificationForRule(ctx, event, &rule); err != nil {
			s.logger.WithFields(map[string]any{
				"rule_id":   rule.ID,
				"rule_name": rule.Name,
				"error":     err.Error(),
				"component": "notification",
			}).Error("Failed to send notification")
			continue
		}
	}

	return nil
}

// TestChannel sends a test notification to verify channel configuration
func (s *Service) TestChannel(ctx context.Context, channelID uint) error {
	var channel NotificationChannel
	if err := s.db.First(&channel, channelID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("notification channel not found")
		}
		return fmt.Errorf("failed to get channel: %w", err)
	}

	// Create test event
	testEvent := &NotificationEvent{
		Type:       "test",
		AlertLevel: AlertLevelInfo,
		Title:      "Test Notification",
		Message:    fmt.Sprintf("This is a test notification from channel '%s'", channel.Name),
		Timestamp:  time.Now(),
	}

	// Create temporary history record
	history := &NotificationHistory{
		ChannelID:   channel.ID,
		TriggerType: "test",
		Subject:     testEvent.Title,
		Message:     testEvent.Message,
		AlertLevel:  string(testEvent.AlertLevel),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	return s.deliverNotification(ctx, &channel, history)
}

// validateChannelConfig validates channel configuration
func (s *Service) validateChannelConfig(channel *NotificationChannel) error {
	switch channel.Type {
	case "email":
		var config EmailConfig
		if err := json.Unmarshal(channel.Config, &config); err != nil {
			return fmt.Errorf("invalid email config: %w", err)
		}
		if len(config.Recipients) == 0 {
			return fmt.Errorf("email config must have at least one recipient")
		}

	case "webhook":
		var config WebhookConfig
		if err := json.Unmarshal(channel.Config, &config); err != nil {
			return fmt.Errorf("invalid webhook config: %w", err)
		}
		if config.URL == "" {
			return fmt.Errorf("webhook config must have a URL")
		}

	case "slack":
		var config SlackConfig
		if err := json.Unmarshal(channel.Config, &config); err != nil {
			return fmt.Errorf("invalid slack config: %w", err)
		}
		if config.WebhookURL == "" {
			return fmt.Errorf("slack config must have a webhook URL")
		}

	default:
		return fmt.Errorf("unsupported channel type: %s", channel.Type)
	}

	return nil
}

// getMatchingRules finds rules that match the notification event
func (s *Service) getMatchingRules(event *NotificationEvent) ([]NotificationRule, error) {
	var rules []NotificationRule

	query := s.db.Preload("Channel").Where("enabled = ?", true)

	// Filter by alert level
	if event.AlertLevel != "" {
		query = query.Where("alert_level IN (?, ?)", event.AlertLevel, "all")
	}

	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}

	// Additional filtering
	var matchingRules []NotificationRule
	for _, rule := range rules {
		if s.ruleMatches(&rule, event) {
			matchingRules = append(matchingRules, rule)
		}
	}

	return matchingRules, nil
}

// ruleMatches checks if a rule matches the event
func (s *Service) ruleMatches(rule *NotificationRule, event *NotificationEvent) bool {
	// Check rate limiting
	if s.isRateLimited(rule.ID) {
		return false
	}

	// Check schedule
	if rule.ScheduleEnabled && !s.isInSchedule(rule) {
		return false
	}

	// Check categories
	if len(rule.CategoriesJSON) > 0 {
		var ruleCategories []string
		if err := json.Unmarshal(rule.CategoriesJSON, &ruleCategories); err == nil {
			if !s.categoriesMatch(ruleCategories, event.Categories) {
				return false
			}
		}
	}

	// Check device filter
	if len(rule.DeviceFilter) > 0 && event.DeviceID != nil {
		var filter DeviceFilter
		if err := json.Unmarshal(rule.DeviceFilter, &filter); err == nil {
			if !s.deviceMatches(&filter, *event.DeviceID) {
				return false
			}
		}
	}

	return true
}

// isRateLimited checks if a rule is rate limited
func (s *Service) isRateLimited(ruleID uint) bool {
	s.rateLimitMu.RLock()
	defer s.rateLimitMu.RUnlock()

	state, exists := s.rateLimits[ruleID]
	if !exists {
		return false
	}

	now := time.Now()

	// Reset hourly count if needed
	if now.After(state.HourlyResetAt) {
		s.rateLimitMu.RUnlock()
		s.rateLimitMu.Lock()
		state.HourlyCount = 0
		state.HourlyResetAt = now.Add(time.Hour)
		s.rateLimitMu.Unlock()
		s.rateLimitMu.RLock()
	}

	// Check interval and hourly limits
	return state.HourlyCount >= 10 // Default max per hour
}

// isInSchedule checks if current time is within rule schedule
func (s *Service) isInSchedule(rule *NotificationRule) bool {
	now := time.Now()

	// Check day of week
	if len(rule.ScheduleDaysJSON) > 0 {
		var scheduleDays []string
		if err := json.Unmarshal(rule.ScheduleDaysJSON, &scheduleDays); err == nil {
			dayName := strings.ToLower(now.Weekday().String())
			dayMatched := false
			for _, day := range scheduleDays {
				if strings.ToLower(day) == dayName {
					dayMatched = true
					break
				}
			}
			if !dayMatched {
				return false
			}
		}
	}

	// Check time window
	if rule.ScheduleStart != "" && rule.ScheduleEnd != "" {
		currentTime := now.Format("15:04")
		if currentTime < rule.ScheduleStart || currentTime > rule.ScheduleEnd {
			return false
		}
	}

	return true
}

// categoriesMatch checks if rule categories match event categories
func (s *Service) categoriesMatch(ruleCategories, eventCategories []string) bool {
	if len(ruleCategories) == 0 {
		return true // No filter means match all
	}

	for _, eventCat := range eventCategories {
		for _, ruleCat := range ruleCategories {
			if eventCat == ruleCat {
				return true
			}
		}
	}

	return false
}

// deviceMatches checks if device matches filter
func (s *Service) deviceMatches(filter *DeviceFilter, deviceID uint) bool {
	// Simple implementation - check if device ID is in allowed list
	if len(filter.DeviceIDs) > 0 {
		found := false
		for _, id := range filter.DeviceIDs {
			if id == deviceID {
				found = true
				break
			}
		}
		return found != filter.Exclude
	}

	return !filter.Exclude // If no specific filter, match unless excluding
}

// sendNotificationForRule sends notification for a specific rule
func (s *Service) sendNotificationForRule(ctx context.Context, event *NotificationEvent, rule *NotificationRule) error {
	// Create history record
	history := &NotificationHistory{
		RuleID:      rule.ID,
		ChannelID:   rule.ChannelID,
		TriggerType: event.Type,
		DeviceID:    event.DeviceID,
		Subject:     event.Title,
		Message:     event.Message,
		AlertLevel:  string(event.AlertLevel),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	if affectedJSON, err := json.Marshal(event.AffectedDevices); err == nil {
		history.AffectedDevicesJSON = affectedJSON
	}

	// Save to database
	if err := s.db.Create(history).Error; err != nil {
		return fmt.Errorf("failed to create notification history: %w", err)
	}

	// Deliver notification
	if err := s.deliverNotification(ctx, &rule.Channel, history); err != nil {
		// Update status to failed
		s.db.Model(history).Updates(map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
		})
		return err
	}

	// Update rate limiting
	s.updateRateLimit(rule.ID)

	// Update status to sent
	now := time.Now()
	s.db.Model(history).Updates(map[string]interface{}{
		"status":  "sent",
		"sent_at": &now,
	})

	return nil
}

// deliverNotification handles the actual delivery
func (s *Service) deliverNotification(ctx context.Context, channel *NotificationChannel, history *NotificationHistory) error {
	switch channel.Type {
	case "email":
		return s.sendEmail(ctx, channel, history)
	case "webhook":
		return s.sendWebhook(ctx, channel, history)
	case "slack":
		return s.sendSlack(ctx, channel, history)
	default:
		return fmt.Errorf("unsupported channel type: %s", channel.Type)
	}
}

// sendEmail sends email notification
func (s *Service) sendEmail(ctx context.Context, channel *NotificationChannel, history *NotificationHistory) error {
	var config EmailConfig
	if err := json.Unmarshal(channel.Config, &config); err != nil {
		return fmt.Errorf("invalid email config: %w", err)
	}

	// Use default SMTP config if not provided in channel
	smtpConfig := s.emailConfig
	if smtpConfig.Host == "" {
		return fmt.Errorf("SMTP configuration not available")
	}

	// Prepare message
	subject := config.Subject
	if subject == "" {
		subject = history.Subject
	}

	body := history.Message
	if config.Template != "" {
		// Apply template (simplified)
		body = strings.ReplaceAll(config.Template, "{{.Message}}", history.Message)
		body = strings.ReplaceAll(body, "{{.Subject}}", subject)
		body = strings.ReplaceAll(body, "{{.AlertLevel}}", history.AlertLevel)
	}

	// Send to each recipient
	for _, recipient := range config.Recipients {
		msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", recipient, subject, body)

		// SMTP authentication
		auth := smtp.PlainAuth("", smtpConfig.Username, smtpConfig.Password, smtpConfig.Host)

		// Send email
		addr := fmt.Sprintf("%s:%d", smtpConfig.Host, smtpConfig.Port)
		if err := smtp.SendMail(addr, auth, smtpConfig.From, []string{recipient}, []byte(msg)); err != nil {
			return fmt.Errorf("failed to send email to %s: %w", recipient, err)
		}
	}

	s.logger.WithFields(map[string]any{
		"channel_id": channel.ID,
		"recipients": len(config.Recipients),
		"component":  "notification",
	}).Info("Sent email notification")

	return nil
}

// sendWebhook sends webhook notification
func (s *Service) sendWebhook(ctx context.Context, channel *NotificationChannel, history *NotificationHistory) error {
	var config WebhookConfig
	if err := json.Unmarshal(channel.Config, &config); err != nil {
		return fmt.Errorf("invalid webhook config: %w", err)
	}

	// Prepare payload
	payload := map[string]interface{}{
		"type":        history.TriggerType,
		"alert_level": history.AlertLevel,
		"subject":     history.Subject,
		"message":     history.Message,
		"timestamp":   history.CreatedAt.Unix(),
	}

	if history.DeviceID != nil {
		payload["device_id"] = *history.DeviceID
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	method := config.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, config.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "shelly-manager/1.0")

	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Add signature if secret is provided
	if config.Secret != "" {
		signature := s.generateSignature(payloadBytes, config.Secret)
		req.Header.Set("X-Signature", signature)
	}

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	s.logger.WithFields(map[string]any{
		"channel_id": channel.ID,
		"url":        config.URL,
		"status":     resp.StatusCode,
		"component":  "notification",
	}).Info("Sent webhook notification")

	return nil
}

// sendSlack sends Slack notification
func (s *Service) sendSlack(ctx context.Context, channel *NotificationChannel, history *NotificationHistory) error {
	var config SlackConfig
	if err := json.Unmarshal(channel.Config, &config); err != nil {
		return fmt.Errorf("invalid slack config: %w", err)
	}

	// Prepare Slack payload
	payload := map[string]interface{}{
		"text": history.Subject,
		"attachments": []map[string]interface{}{
			{
				"color":     s.getSlackColor(history.AlertLevel),
				"title":     history.Subject,
				"text":      history.Message,
				"timestamp": history.CreatedAt.Unix(),
				"footer":    "Shelly Manager",
			},
		},
	}

	if config.Channel != "" {
		payload["channel"] = config.Channel
	}
	if config.Username != "" {
		payload["username"] = config.Username
	}
	if config.IconEmoji != "" {
		payload["icon_emoji"] = config.IconEmoji
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	// Send to Slack
	req, err := http.NewRequestWithContext(ctx, "POST", config.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	s.logger.WithFields(map[string]any{
		"channel_id": channel.ID,
		"component":  "notification",
	}).Info("Sent Slack notification")

	return nil
}

// generateSignature generates HMAC signature for webhook
func (s *Service) generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// getSlackColor returns appropriate color for alert level
func (s *Service) getSlackColor(alertLevel string) string {
	switch alertLevel {
	case "critical":
		return "danger"
	case "warning":
		return "warning"
	case "info":
		return "good"
	default:
		return "#808080"
	}
}

// updateRateLimit updates rate limiting state
func (s *Service) updateRateLimit(ruleID uint) {
	s.rateLimitMu.Lock()
	defer s.rateLimitMu.Unlock()

	now := time.Now()

	state, exists := s.rateLimits[ruleID]
	if !exists {
		state = &RateLimitState{
			RuleID:        ruleID,
			HourlyResetAt: now.Add(time.Hour),
		}
		s.rateLimits[ruleID] = state
	}

	state.LastSentAt = now
	state.HourlyCount++

	// Reset hourly count if needed
	if now.After(state.HourlyResetAt) {
		state.HourlyCount = 1
		state.HourlyResetAt = now.Add(time.Hour)
	}
}

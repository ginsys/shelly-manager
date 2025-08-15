package notification

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/gorilla/mux"
)

// Handler handles HTTP requests for notification operations
type Handler struct {
	service *Service
	logger  *logging.Logger
}

// NewHandler creates a new notification handler
func NewHandler(service *Service, logger *logging.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// writeJSONResponse writes a JSON response and handles encoding errors
func (h *Handler) writeJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.WithFields(map[string]any{
			"component": "notification",
			"error":     err,
		}).Error("Failed to encode JSON response")
	}
}

// CreateChannel handles POST /api/v1/notifications/channels
func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var channel NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to decode channel request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.CreateChannel(&channel); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to create notification channel")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, channel, http.StatusCreated)
}

// GetChannels handles GET /api/v1/notifications/channels
func (h *Handler) GetChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.service.GetChannels()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to get notification channels")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"channels": channels,
		"total":    len(channels),
	}, http.StatusOK)
}

// UpdateChannel handles PUT /api/v1/notifications/channels/{id}
func (h *Handler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channelID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	var updates NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to decode channel update request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateChannel(uint(channelID), &updates); err != nil {
		h.logger.WithFields(map[string]any{
			"channel_id": channelID,
			"error":      err.Error(),
			"component":  "notification_api",
		}).Error("Failed to update notification channel")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]string{"status": "updated"}, http.StatusOK)
}

// DeleteChannel handles DELETE /api/v1/notifications/channels/{id}
func (h *Handler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channelID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteChannel(uint(channelID)); err != nil {
		h.logger.WithFields(map[string]any{
			"channel_id": channelID,
			"error":      err.Error(),
			"component":  "notification_api",
		}).Error("Failed to delete notification channel")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]string{"status": "deleted"}, http.StatusOK)
}

// CreateRule handles POST /api/v1/notifications/rules
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule NotificationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to decode rule request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.CreateRule(&rule); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to create notification rule")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, rule, http.StatusCreated)
}

// GetRules handles GET /api/v1/notifications/rules
func (h *Handler) GetRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.GetRules()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to get notification rules")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	}, http.StatusOK)
}

// TestChannel handles POST /api/v1/notifications/channels/{id}/test
func (h *Handler) TestChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channelID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	if err := h.service.TestChannel(r.Context(), uint(channelID)); err != nil {
		h.logger.WithFields(map[string]any{
			"channel_id": channelID,
			"error":      err.Error(),
			"component":  "notification_api",
		}).Error("Failed to test notification channel")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]string{
		"status":  "sent",
		"message": "Test notification sent successfully",
	}, http.StatusOK)
}

// GetHistory handles GET /api/v1/notifications/history
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")
	channelIDStr := query.Get("channel_id")
	status := query.Get("status")

	limit := 50 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	var channelID *uint
	if channelIDStr != "" {
		if parsed, err := strconv.ParseUint(channelIDStr, 10, 32); err == nil {
			id := uint(parsed)
			channelID = &id
		}
	}

	history, total, err := h.getNotificationHistory(channelID, status, limit, offset)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to get notification history")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"history": history,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}, http.StatusOK)
}

// getNotificationHistory retrieves notification history with filters
func (h *Handler) getNotificationHistory(channelID *uint, status string, limit, offset int) ([]NotificationHistory, int64, error) {
	// This would be implemented in the service layer
	// For now, return empty results
	return []NotificationHistory{}, 0, nil
}

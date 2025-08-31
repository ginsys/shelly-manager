package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
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

// NotifyEvent exposes a simple delegate to service.SendNotification for integration points
func (h *Handler) NotifyEvent(ctx context.Context, event *NotificationEvent) error {
	return h.service.SendNotification(ctx, event)
}

// Deprecated legacy JSON writer removed in favor of standardized responses.

// CreateChannel handles POST /api/v1/notifications/channels
func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var channel NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to decode channel request")
		apiresp.NewResponseWriter(h.logger).WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.service.CreateChannel(&channel); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to create notification channel")
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeValidationFailed, "Invalid channel configuration", err.Error())
		return
	}

	apiresp.NewResponseWriter(h.logger).WriteCreated(w, r, channel)
}

// GetChannels handles GET /api/v1/notifications/channels
func (h *Handler) GetChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.service.GetChannels()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to get notification channels")
		apiresp.NewResponseWriter(h.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(h.logger).WriteSuccess(w, r, map[string]interface{}{
		"channels": channels,
		"total":    len(channels),
	})
}

// UpdateChannel handles PUT /api/v1/notifications/channels/{id}
func (h *Handler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channelID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid channel ID", nil)
		return
	}

	var updates NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to decode channel update request")
		apiresp.NewResponseWriter(h.logger).WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.service.UpdateChannel(uint(channelID), &updates); err != nil {
		h.logger.WithFields(map[string]any{
			"channel_id": channelID,
			"error":      err.Error(),
			"component":  "notification_api",
		}).Error("Failed to update notification channel")
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeValidationFailed, "Invalid channel update", err.Error())
		return
	}

	apiresp.NewResponseWriter(h.logger).WriteSuccess(w, r, map[string]string{"status": "updated"})
}

// DeleteChannel handles DELETE /api/v1/notifications/channels/{id}
func (h *Handler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channelID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid channel ID", nil)
		return
	}

	if err := h.service.DeleteChannel(uint(channelID)); err != nil {
		h.logger.WithFields(map[string]any{
			"channel_id": channelID,
			"error":      err.Error(),
			"component":  "notification_api",
		}).Error("Failed to delete notification channel")
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeValidationFailed, "Cannot delete channel", err.Error())
		return
	}

	apiresp.NewResponseWriter(h.logger).WriteSuccess(w, r, map[string]string{"status": "deleted"})
}

// CreateRule handles POST /api/v1/notifications/rules
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule NotificationRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to decode rule request")
		apiresp.NewResponseWriter(h.logger).WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.service.CreateRule(&rule); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to create notification rule")
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeValidationFailed, "Invalid rule configuration", err.Error())
		return
	}

	apiresp.NewResponseWriter(h.logger).WriteCreated(w, r, rule)
}

// GetRules handles GET /api/v1/notifications/rules
func (h *Handler) GetRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.GetRules()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to get notification rules")
		apiresp.NewResponseWriter(h.logger).WriteInternalError(w, r, err)
		return
	}

	apiresp.NewResponseWriter(h.logger).WriteSuccess(w, r, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

// TestChannel handles POST /api/v1/notifications/channels/{id}/test
func (h *Handler) TestChannel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channelID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid channel ID", nil)
		return
	}

	if err := h.service.TestChannel(r.Context(), uint(channelID)); err != nil {
		h.logger.WithFields(map[string]any{
			"channel_id": channelID,
			"error":      err.Error(),
			"component":  "notification_api",
		}).Error("Failed to test notification channel")
		// If not found, return 404, otherwise internal error
		msg := err.Error()
		code := apiresp.ErrCodeInternalServer
		status := http.StatusInternalServerError
		if msg == "notification channel not found" {
			code = apiresp.ErrCodeNotFound
			status = http.StatusNotFound
		}
		apiresp.NewResponseWriter(h.logger).WriteError(w, r, status, code, msg, nil)
		return
	}

	apiresp.NewResponseWriter(h.logger).WriteSuccess(w, r, map[string]string{
		"status":  "sent",
		"message": "Test notification sent successfully",
	})
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

	history, total, err := h.service.GetHistory(channelID, status, limit, offset)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "notification_api",
		}).Error("Failed to get notification history")
		apiresp.NewResponseWriter(h.logger).WriteInternalError(w, r, err)
		return
	}

	// Translate limit/offset to pagination meta (1-based page)
	page := 1
	if limit > 0 {
		page = (offset / limit) + 1
	}
	meta := &apiresp.Metadata{}
	// include simple counts
	totalInt := int(total)
	count := len(history)
	meta.Count = &count
	meta.TotalCount = &totalInt

	// also include page meta if limit provided
	if limit > 0 {
		totalPages := (totalInt + limit - 1) / limit
		meta.Page = &apiresp.PaginationMeta{
			Page:       page,
			PageSize:   limit,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		}
	}

	apiresp.NewResponseWriter(h.logger).WriteSuccessWithMeta(w, r, map[string]interface{}{
		"history": history,
	}, meta)
}

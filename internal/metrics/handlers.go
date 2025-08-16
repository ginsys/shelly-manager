package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Handler handles HTTP requests for metrics operations
type Handler struct {
	service *Service
	logger  *logging.Logger
	wsHub   *WebSocketHub
}

// NewHandler creates a new metrics handler
func NewHandler(service *Service, logger *logging.Logger) *Handler {
	hub := NewWebSocketHub(service, logger)
	return &Handler{
		service: service,
		logger:  logger,
		wsHub:   hub,
	}
}

// GetWebSocketHub returns the WebSocket hub for external use
func (h *Handler) GetWebSocketHub() *WebSocketHub {
	return h.wsHub
}

// MetricsStatus represents the status of the metrics system
type MetricsStatus struct {
	Enabled            bool      `json:"enabled"`
	LastCollectionTime time.Time `json:"last_collection_time"`
	UptimeSeconds      float64   `json:"uptime_seconds"`
}

// GetMetricsStatus returns the current metrics system status
func (h *Handler) GetMetricsStatus(w http.ResponseWriter, r *http.Request) {
	status := MetricsStatus{
		Enabled:            h.service.IsEnabled(),
		LastCollectionTime: h.service.GetLastCollectionTime(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to encode metrics status response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Debug("Returned metrics status")
}

// EnableMetrics enables metrics collection
func (h *Handler) EnableMetrics(w http.ResponseWriter, r *http.Request) {
	h.service.Enable()

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "enabled"}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to encode enable metrics response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Info("Metrics collection enabled via API")
}

// DisableMetrics disables metrics collection
func (h *Handler) DisableMetrics(w http.ResponseWriter, r *http.Request) {
	h.service.Disable()

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"status": "disabled"}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to encode disable metrics response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Info("Metrics collection disabled via API")
}

// CollectMetrics triggers manual metrics collection
func (h *Handler) CollectMetrics(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if err := h.service.CollectMetrics(r.Context()); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to collect metrics")
		http.Error(w, "Failed to collect metrics", http.StatusInternalServerError)
		return
	}

	duration := time.Since(start)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"status":       "collected",
		"duration_ms":  duration.Milliseconds(),
		"collected_at": time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to encode collect metrics response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(map[string]any{
		"duration":  duration,
		"component": "metrics",
	}).Info("Manual metrics collection completed")
}

// PrometheusHandler returns the Prometheus metrics handler
func (h *Handler) PrometheusHandler() http.Handler {
	// Use the service's registry if it's a gatherer, otherwise use default
	if gatherer, ok := h.service.GetRegistry().(prometheus.Gatherer); ok {
		return promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})
	}
	return promhttp.Handler()
}

// GetDashboardMetrics returns dashboard metrics for HTTP requests
func (h *Handler) GetDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.wsHub.collectDashboardMetrics(r.Context())
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to collect dashboard metrics")
		http.Error(w, "Failed to collect dashboard metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to encode dashboard metrics response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Debug("Returned dashboard metrics")
}

// HandleWebSocket handles WebSocket connections for real-time metrics
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleWebSocket(w, r)
}

// SendTestAlert sends a test alert for dashboard testing
func (h *Handler) SendTestAlert(w http.ResponseWriter, r *http.Request) {
	// Get alert type and severity from query parameters
	alertType := r.URL.Query().Get("type")
	severity := r.URL.Query().Get("severity")

	if alertType == "" {
		alertType = "test"
	}
	if severity == "" {
		severity = "info"
	}

	message := fmt.Sprintf("Test alert sent at %s", time.Now().Format("15:04:05"))

	// Broadcast the test alert
	h.wsHub.BroadcastAlert(alertType, message, severity)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"status":     "sent",
		"alert_type": alertType,
		"message":    message,
		"severity":   severity,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to encode test alert response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(map[string]any{
		"alert_type": alertType,
		"severity":   severity,
		"component":  "metrics",
	}).Info("Test alert sent to dashboard")
}

// SetupMetricsRoutes adds metrics routes to the router
func SetupMetricsRoutes(router *mux.Router, handler *Handler) {
	metrics := router.PathPrefix("/metrics").Subrouter()

	// Prometheus metrics endpoint
	metrics.Handle("/prometheus", handler.PrometheusHandler()).Methods("GET")

	// Control endpoints
	metrics.HandleFunc("/status", handler.GetMetricsStatus).Methods("GET")
	metrics.HandleFunc("/enable", handler.EnableMetrics).Methods("POST")
	metrics.HandleFunc("/disable", handler.DisableMetrics).Methods("POST")
	metrics.HandleFunc("/collect", handler.CollectMetrics).Methods("POST")

	// Dashboard endpoints
	metrics.HandleFunc("/dashboard", handler.GetDashboardMetrics).Methods("GET")
	metrics.HandleFunc("/ws", handler.HandleWebSocket).Methods("GET")
}

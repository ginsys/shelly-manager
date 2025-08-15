package metrics

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler handles HTTP requests for metrics operations
type Handler struct {
	service *Service
	logger  *logging.Logger
}

// NewHandler creates a new metrics handler
func NewHandler(service *Service, logger *logging.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
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
}

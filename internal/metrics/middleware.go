package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPMetrics holds HTTP-related Prometheus metrics
type HTTPMetrics struct {
	requestsTotal     prometheus.CounterVec
	requestDuration   prometheus.HistogramVec
	responseSizeBytes prometheus.HistogramVec
}

// NewHTTPMetrics creates new HTTP metrics
func NewHTTPMetrics(registry prometheus.Registerer) *HTTPMetrics {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	return &HTTPMetrics{
		requestsTotal: *promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "shelly_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		requestDuration: *promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "shelly_http_request_duration_seconds",
				Help:    "Duration of HTTP requests",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		responseSizeBytes: *promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "shelly_http_response_size_bytes",
				Help:    "Size of HTTP responses",
				Buckets: prometheus.ExponentialBuckets(100, 10, 5),
			},
			[]string{"method", "path"},
		),
	}
}

// responseWriter wraps http.ResponseWriter to capture response metrics
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(data)
	rw.size += size
	return size, err
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// HTTPMiddleware creates HTTP metrics middleware
func (hm *HTTPMetrics) HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     200, // Default status code
			}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start)
			method := r.Method
			path := r.URL.Path
			statusCode := strconv.Itoa(wrapped.statusCode)

			hm.requestsTotal.WithLabelValues(method, path, statusCode).Inc()
			hm.requestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
			hm.responseSizeBytes.WithLabelValues(method, path).Observe(float64(wrapped.size))
		})
	}
}

// OperationTimer helps time operations for metrics
type OperationTimer struct {
	service *Service
	start   time.Time
}

// StartTimer creates a new operation timer
func (s *Service) StartTimer() *OperationTimer {
	return &OperationTimer{
		service: s,
		start:   time.Now(),
	}
}

// RecordDriftDetection records drift detection with timing
func (ot *OperationTimer) RecordDriftDetection(status, deviceType string) {
	duration := time.Since(ot.start)
	ot.service.RecordDriftDetection(status, deviceType, duration)
}

// RecordBulkDriftDetection records bulk drift detection with timing
func (ot *OperationTimer) RecordBulkDriftDetection(deviceCount int) {
	duration := time.Since(ot.start)
	ot.service.RecordBulkDriftDetection(deviceCount, duration)
}

// RecordNotificationSent records notification with timing
func (ot *OperationTimer) RecordNotificationSent(channelType, alertLevel string) {
	duration := time.Since(ot.start)
	ot.service.RecordNotificationSent(channelType, alertLevel, duration)
}

// NotificationMetricsWrapper wraps notification operations with metrics
type NotificationMetricsWrapper struct {
	metricsService *Service
	next           NotificationInterface
}

// NotificationInterface defines the interface for notification operations
type NotificationInterface interface {
	SendNotification(ctx context.Context, event *NotificationEvent) error
}

// NotificationEvent represents a notification event (simplified)
type NotificationEvent struct {
	Type       string
	AlertLevel string
	Channel    string
}

// NewNotificationMetricsWrapper creates a metrics wrapper for notifications
func NewNotificationMetricsWrapper(metricsService *Service, next NotificationInterface) *NotificationMetricsWrapper {
	return &NotificationMetricsWrapper{
		metricsService: metricsService,
		next:           next,
	}
}

// SendNotification wraps notification sending with metrics
func (nmw *NotificationMetricsWrapper) SendNotification(ctx context.Context, event *NotificationEvent) error {
	timer := nmw.metricsService.StartTimer()

	err := nmw.next.SendNotification(ctx, event)

	if err != nil {
		nmw.metricsService.RecordNotificationFailure(event.Channel, "send_error")
	} else {
		timer.RecordNotificationSent(event.Channel, event.AlertLevel)
	}

	return err
}

// DriftDetectionMetricsWrapper wraps drift detection operations with metrics
type DriftDetectionMetricsWrapper struct {
	metricsService *Service
	next           DriftDetectionInterface
}

// DriftDetectionInterface defines the interface for drift detection operations
type DriftDetectionInterface interface {
	DetectDrift(ctx context.Context, deviceID uint) (*DriftResult, error)
	BulkDetectDrift(ctx context.Context, deviceIDs []uint) (*BulkDriftResult, error)
}

// DriftResult represents drift detection result (simplified)
type DriftResult struct {
	DeviceID   uint
	DeviceType string
	Status     string
	HasDrift   bool
}

// BulkDriftResult represents bulk drift detection result
type BulkDriftResult struct {
	Results []DriftResult
	Summary struct {
		Total   int
		Drifted int
		Synced  int
	}
}

// NewDriftDetectionMetricsWrapper creates a metrics wrapper for drift detection
func NewDriftDetectionMetricsWrapper(metricsService *Service, next DriftDetectionInterface) *DriftDetectionMetricsWrapper {
	return &DriftDetectionMetricsWrapper{
		metricsService: metricsService,
		next:           next,
	}
}

// DetectDrift wraps single drift detection with metrics
func (ddmw *DriftDetectionMetricsWrapper) DetectDrift(ctx context.Context, deviceID uint) (*DriftResult, error) {
	timer := ddmw.metricsService.StartTimer()

	result, err := ddmw.next.DetectDrift(ctx, deviceID)

	if err != nil {
		ddmw.metricsService.RecordDriftDetection("error", "unknown", time.Since(timer.start))
	} else if result != nil {
		status := "synced"
		if result.HasDrift {
			status = "drift"
		}
		timer.RecordDriftDetection(status, result.DeviceType)
	}

	return result, err
}

// BulkDetectDrift wraps bulk drift detection with metrics
func (ddmw *DriftDetectionMetricsWrapper) BulkDetectDrift(ctx context.Context, deviceIDs []uint) (*BulkDriftResult, error) {
	timer := ddmw.metricsService.StartTimer()

	result, err := ddmw.next.BulkDetectDrift(ctx, deviceIDs)

	if err != nil {
		ddmw.metricsService.RecordBulkDriftDetection(len(deviceIDs), time.Since(timer.start))
	} else if result != nil {
		timer.RecordBulkDriftDetection(len(result.Results))

		// Record individual device metrics
		for _, deviceResult := range result.Results {
			status := "synced"
			if deviceResult.HasDrift {
				status = "drift"
			}
			ddmw.metricsService.RecordDriftDetection(status, deviceResult.DeviceType, 0)
		}
	}

	return result, err
}

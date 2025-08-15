package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func setupTestHTTPMetrics(t *testing.T) (*HTTPMetrics, prometheus.Gatherer) {
	t.Helper()

	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)

	return metrics, registry
}

func TestNewHTTPMetrics(t *testing.T) {
	metrics := NewHTTPMetrics(nil)

	if metrics == nil {
		t.Fatal("NewHTTPMetrics returned nil")
	}

	// Test with explicit registry
	registry := prometheus.NewRegistry()
	metrics2 := NewHTTPMetrics(registry)

	if metrics2 == nil {
		t.Fatal("NewHTTPMetrics with registry returned nil")
	}
}

func TestHTTPMiddleware(t *testing.T) {
	metrics, _ := setupTestHTTPMetrics(t)

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	middleware := metrics.HTTPMiddleware()
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", w.Body.String())
	}

	// Verify metrics were recorded
	requestCount := testutil.ToFloat64(metrics.requestsTotal.WithLabelValues("GET", "/test", "200"))
	if requestCount != 1 {
		t.Errorf("Expected request count 1, got %f", requestCount)
	}

	durationMetric := &dto.Metric{}
	if err := metrics.requestDuration.WithLabelValues("GET", "/test").(prometheus.Histogram).Write(durationMetric); err == nil {
		if durationMetric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected duration count 1, got %d", durationMetric.GetHistogram().GetSampleCount())
		}
	}

	responseSizeMetric := &dto.Metric{}
	if err := metrics.responseSizeBytes.WithLabelValues("GET", "/test").(prometheus.Histogram).Write(responseSizeMetric); err == nil {
		if responseSizeMetric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected response size count 1, got %d", responseSizeMetric.GetHistogram().GetSampleCount())
		}
	}
}

func TestHTTPMiddlewareWithDifferentMethods(t *testing.T) {
	metrics, _ := setupTestHTTPMetrics(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	})

	middleware := metrics.HTTPMiddleware()
	wrappedHandler := middleware(handler)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/test", nil)
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		// Verify metrics for each method
		requestCount := testutil.ToFloat64(metrics.requestsTotal.WithLabelValues(method, "/api/test", "201"))
		if requestCount != 1 {
			t.Errorf("Expected request count 1 for %s, got %f", method, requestCount)
		}
	}
}

func TestHTTPMiddlewareWithDifferentStatusCodes(t *testing.T) {
	metrics, _ := setupTestHTTPMetrics(t)

	statusCodes := []int{200, 201, 400, 401, 404, 500}

	for _, statusCode := range statusCodes {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			w.Write([]byte("Response"))
		})

		middleware := metrics.HTTPMiddleware()
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		// Verify metrics for each status code
		requestCount := testutil.ToFloat64(metrics.requestsTotal.WithLabelValues("GET", "/test", strconv.Itoa(statusCode)))
		if requestCount != 1 {
			t.Errorf("Expected request count 1 for status %d, got %f", statusCode, requestCount)
		}
	}
}

func TestHTTPMiddlewareResponseSize(t *testing.T) {
	metrics, _ := setupTestHTTPMetrics(t)

	testCases := []struct {
		body         string
		expectedSize int
	}{
		{"", 0},
		{"OK", 2},
		{"Hello, World!", 13},
		{strings.Repeat("A", 1000), 1000},
	}

	for i, tc := range testCases {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(tc.body))
		})

		middleware := metrics.HTTPMiddleware()
		wrappedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/size-test", nil)
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		// Check actual response size
		if w.Body.Len() != tc.expectedSize {
			t.Errorf("Test case %d: expected response size %d, got %d", i, tc.expectedSize, w.Body.Len())
		}
	}
}

func TestResponseWriterWrapper(t *testing.T) {
	originalWriter := httptest.NewRecorder()
	wrapper := &responseWriter{
		ResponseWriter: originalWriter,
		statusCode:     200,
	}

	// Test Write
	data := []byte("test data")
	n, err := wrapper.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected bytes written %d, got %d", len(data), n)
	}

	if wrapper.size != len(data) {
		t.Errorf("Expected wrapper size %d, got %d", len(data), wrapper.size)
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusNotFound)
	if wrapper.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, wrapper.statusCode)
	}

	// Test multiple writes
	moreData := []byte(" more")
	wrapper.Write(moreData)

	expectedSize := len(data) + len(moreData)
	if wrapper.size != expectedSize {
		t.Errorf("Expected total size %d, got %d", expectedSize, wrapper.size)
	}
}

func TestOperationTimer(t *testing.T) {
	service, _ := setupTestService(t)

	timer := service.StartTimer()

	if timer == nil {
		t.Fatal("StartTimer returned nil")
	}

	if timer.service != service {
		t.Error("Timer service not set correctly")
	}

	if timer.start.IsZero() {
		t.Error("Timer start time not set")
	}

	// Wait a bit to ensure some time passes
	time.Sleep(10 * time.Millisecond)

	// Test RecordDriftDetection
	timer.RecordDriftDetection("drift", "SHSW-1")

	// Verify metrics were recorded with timing
	driftCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("drift", "SHSW-1"))
	if driftCount != 1 {
		t.Errorf("Expected drift count 1, got %f", driftCount)
	}

	durationMetric := &dto.Metric{}
	if err := service.driftDetectionDuration.WithLabelValues("single").(prometheus.Histogram).Write(durationMetric); err == nil {
		if durationMetric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected duration count 1, got %d", durationMetric.GetHistogram().GetSampleCount())
		}
	}
}

func TestOperationTimerMethods(t *testing.T) {
	service, _ := setupTestService(t)

	timer := service.StartTimer()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Test all timer methods
	timer.RecordDriftDetection("synced", "SHSW-25")
	timer.RecordBulkDriftDetection(5)
	timer.RecordNotificationSent("email", "warning")

	// Verify all metrics were recorded
	driftCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("synced", "SHSW-25"))
	if driftCount != 1 {
		t.Errorf("Expected drift count 1, got %f", driftCount)
	}

	bulkDurationMetric := &dto.Metric{}
	if err := service.driftDetectionDuration.WithLabelValues("bulk").(prometheus.Histogram).Write(bulkDurationMetric); err == nil {
		if bulkDurationMetric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected bulk duration count 1, got %d", bulkDurationMetric.GetHistogram().GetSampleCount())
		}
	}

	notificationCount := testutil.ToFloat64(service.notificationsSent.WithLabelValues("email", "warning"))
	if notificationCount != 1 {
		t.Errorf("Expected notification count 1, got %f", notificationCount)
	}
}

// Mock implementations for testing wrappers

type mockNotificationService struct {
	shouldFail bool
}

func (m *mockNotificationService) SendNotification(ctx context.Context, event *NotificationEvent) error {
	if m.shouldFail {
		return errors.New("notification failed")
	}
	return nil
}

func TestNotificationMetricsWrapper(t *testing.T) {
	service, _ := setupTestService(t)

	mockService := &mockNotificationService{shouldFail: false}
	wrapper := NewNotificationMetricsWrapper(service, mockService)

	event := &NotificationEvent{
		Type:       "drift_detected",
		AlertLevel: "critical",
		Channel:    "email",
	}

	ctx := context.Background()
	err := wrapper.SendNotification(ctx, event)
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	// Verify success metrics
	sentCount := testutil.ToFloat64(service.notificationsSent.WithLabelValues("email", "critical"))
	if sentCount != 1 {
		t.Errorf("Expected sent count 1, got %f", sentCount)
	}
}

func TestNotificationMetricsWrapperFailure(t *testing.T) {
	service, _ := setupTestService(t)

	mockService := &mockNotificationService{shouldFail: true}
	wrapper := NewNotificationMetricsWrapper(service, mockService)

	event := &NotificationEvent{
		Type:       "drift_detected",
		AlertLevel: "warning",
		Channel:    "webhook",
	}

	ctx := context.Background()
	err := wrapper.SendNotification(ctx, event)
	if err == nil {
		t.Error("Expected SendNotification to fail")
	}

	// Verify failure metrics
	failureCount := testutil.ToFloat64(service.notificationFailures.WithLabelValues("webhook", "send_error"))
	if failureCount != 1 {
		t.Errorf("Expected failure count 1, got %f", failureCount)
	}
}

// Mock drift detection service

type mockDriftDetectionService struct {
	shouldFail bool
}

func (m *mockDriftDetectionService) DetectDrift(ctx context.Context, deviceID uint) (*DriftResult, error) {
	if m.shouldFail {
		return nil, errors.New("drift detection failed")
	}

	return &DriftResult{
		DeviceID:   deviceID,
		DeviceType: "SHSW-1",
		Status:     "drift",
		HasDrift:   true,
	}, nil
}

func (m *mockDriftDetectionService) BulkDetectDrift(ctx context.Context, deviceIDs []uint) (*BulkDriftResult, error) {
	if m.shouldFail {
		return nil, errors.New("bulk drift detection failed")
	}

	results := make([]DriftResult, len(deviceIDs))
	for i, deviceID := range deviceIDs {
		results[i] = DriftResult{
			DeviceID:   deviceID,
			DeviceType: "SHSW-1",
			Status:     "synced",
			HasDrift:   false,
		}
	}

	return &BulkDriftResult{
		Results: results,
		Summary: struct {
			Total   int
			Drifted int
			Synced  int
		}{
			Total:   len(deviceIDs),
			Drifted: 0,
			Synced:  len(deviceIDs),
		},
	}, nil
}

func TestDriftDetectionMetricsWrapper(t *testing.T) {
	service, _ := setupTestService(t)

	mockService := &mockDriftDetectionService{shouldFail: false}
	wrapper := NewDriftDetectionMetricsWrapper(service, mockService)

	ctx := context.Background()
	result, err := wrapper.DetectDrift(ctx, 123)
	if err != nil {
		t.Fatalf("DetectDrift failed: %v", err)
	}

	if result == nil {
		t.Fatal("DetectDrift returned nil result")
	}

	if result.DeviceID != 123 {
		t.Errorf("Expected device ID 123, got %d", result.DeviceID)
	}

	// Verify metrics
	driftCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("drift", "SHSW-1"))
	if driftCount != 1 {
		t.Errorf("Expected drift count 1, got %f", driftCount)
	}
}

func TestDriftDetectionMetricsWrapperFailure(t *testing.T) {
	service, _ := setupTestService(t)

	mockService := &mockDriftDetectionService{shouldFail: true}
	wrapper := NewDriftDetectionMetricsWrapper(service, mockService)

	ctx := context.Background()
	result, err := wrapper.DetectDrift(ctx, 123)
	if err == nil {
		t.Error("Expected DetectDrift to fail")
	}

	if result != nil {
		t.Error("Expected nil result on failure")
	}

	// Verify error metrics
	errorCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("error", "unknown"))
	if errorCount != 1 {
		t.Errorf("Expected error count 1, got %f", errorCount)
	}
}

func TestBulkDriftDetectionMetricsWrapper(t *testing.T) {
	service, _ := setupTestService(t)

	mockService := &mockDriftDetectionService{shouldFail: false}
	wrapper := NewDriftDetectionMetricsWrapper(service, mockService)

	deviceIDs := []uint{1, 2, 3, 4, 5}

	ctx := context.Background()
	result, err := wrapper.BulkDetectDrift(ctx, deviceIDs)
	if err != nil {
		t.Fatalf("BulkDetectDrift failed: %v", err)
	}

	if result == nil {
		t.Fatal("BulkDetectDrift returned nil result")
	}

	if len(result.Results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(result.Results))
	}

	// Verify bulk metrics
	bulkDurationMetric2 := &dto.Metric{}
	if err := service.driftDetectionDuration.WithLabelValues("bulk").(prometheus.Histogram).Write(bulkDurationMetric2); err == nil {
		if bulkDurationMetric2.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected bulk duration count 1, got %d", bulkDurationMetric2.GetHistogram().GetSampleCount())
		}
	}

	// Verify individual device metrics
	syncedCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("synced", "SHSW-1"))
	if syncedCount != 5 {
		t.Errorf("Expected synced count 5, got %f", syncedCount)
	}
}

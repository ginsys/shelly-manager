package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/prometheus/client_golang/prometheus"
)

func setupTestHandler(t *testing.T) (*Handler, *Service) {
	t.Helper()

	db := setupTestDB(t)
	logger := logging.GetDefault()
	registry := prometheus.NewRegistry()

	service := NewService(db, logger, registry)
	handler := NewHandler(service, logger)

	return handler, service
}

func TestNewHandler(t *testing.T) {
	handler, service := setupTestHandler(t)

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.service != service {
		t.Error("Handler service not set correctly")
	}

	if handler.logger == nil {
		t.Error("Handler logger not set")
	}
}

func TestGetMetricsStatus(t *testing.T) {
	handler, service := setupTestHandler(t)

	// Test enabled status
	req := httptest.NewRequest("GET", "/metrics/status", nil)
	w := httptest.NewRecorder()

	handler.GetMetricsStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var status MetricsStatus
	err := json.Unmarshal(w.Body.Bytes(), &status)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !status.Enabled {
		t.Error("Expected metrics to be enabled")
	}

	// Test disabled status
	service.Disable()

	req = httptest.NewRequest("GET", "/metrics/status", nil)
	w = httptest.NewRecorder()

	handler.GetMetricsStatus(w, req)

	err = json.Unmarshal(w.Body.Bytes(), &status)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Enabled {
		t.Error("Expected metrics to be disabled")
	}
}

func TestGetMetricsStatusWithLastCollection(t *testing.T) {
	handler, service := setupTestHandler(t)

	// Collect metrics to set last collection time
	ctx := context.Background()
	err := service.CollectMetrics(ctx)
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/metrics/status", nil)
	w := httptest.NewRecorder()

	handler.GetMetricsStatus(w, req)

	var status MetricsStatus
	err = json.Unmarshal(w.Body.Bytes(), &status)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.LastCollectionTime.IsZero() {
		t.Error("Expected last collection time to be set")
	}

	if time.Since(status.LastCollectionTime) > time.Minute {
		t.Error("Last collection time seems too old")
	}
}

func TestEnableMetrics(t *testing.T) {
	handler, service := setupTestHandler(t)

	// Disable first
	service.Disable()

	req := httptest.NewRequest("POST", "/metrics/enable", nil)
	w := httptest.NewRecorder()

	handler.EnableMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "enabled" {
		t.Errorf("Expected status 'enabled', got '%s'", response["status"])
	}

	// Verify service is actually enabled
	if !service.IsEnabled() {
		t.Error("Service should be enabled after enable request")
	}
}

func TestDisableMetrics(t *testing.T) {
	handler, service := setupTestHandler(t)

	// Should be enabled by default
	if !service.IsEnabled() {
		t.Fatal("Service should be enabled by default")
	}

	req := httptest.NewRequest("POST", "/metrics/disable", nil)
	w := httptest.NewRecorder()

	handler.DisableMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "disabled" {
		t.Errorf("Expected status 'disabled', got '%s'", response["status"])
	}

	// Verify service is actually disabled
	if service.IsEnabled() {
		t.Error("Service should be disabled after disable request")
	}
}

func TestCollectMetrics(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/metrics/collect", nil)
	w := httptest.NewRecorder()

	handler.CollectMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "collected" {
		t.Errorf("Expected status 'collected', got '%v'", response["status"])
	}

	// Check that duration is present and reasonable
	durationMs, ok := response["duration_ms"].(float64)
	if !ok {
		t.Error("Expected duration_ms in response")
	}

	if durationMs < 0 || durationMs > 10000 { // Should be reasonable (< 10 seconds)
		t.Errorf("Duration seems unreasonable: %f ms", durationMs)
	}

	// Check that collected_at is present
	_, ok = response["collected_at"].(string)
	if !ok {
		t.Error("Expected collected_at in response")
	}
}

func TestCollectMetricsWithContext(t *testing.T) {
	handler, _ := setupTestHandler(t)

	// Create request with context
	ctx := context.Background()
	req := httptest.NewRequest("POST", "/metrics/collect", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.CollectMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestPrometheusHandler(t *testing.T) {
	handler, service := setupTestHandler(t)

	// Record some metrics first
	service.RecordDriftDetection("drift", "SHSW-1", 100*time.Millisecond)
	service.RecordNotificationSent("email", "critical", 250*time.Millisecond)

	prometheusHandler := handler.PrometheusHandler()

	req := httptest.NewRequest("GET", "/metrics/prometheus", nil)
	w := httptest.NewRecorder()

	prometheusHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Check for some expected metrics
	expectedMetrics := []string{
		"shelly_drift_detection_total",
		"shelly_notifications_sent_total",
		"shelly_manager_uptime_seconds_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metric '%s' not found in Prometheus output", metric)
		}
	}

	// Check for actual values
	if !strings.Contains(body, `shelly_drift_detection_total{device_type="SHSW-1",status="drift"} 1`) {
		t.Error("Expected drift detection metric value not found")
	}

	if !strings.Contains(body, `shelly_notifications_sent_total{alert_level="critical",channel_type="email"} 1`) {
		t.Error("Expected notification sent metric value not found")
	}
}

func TestHandlerErrorResponses(t *testing.T) {
	// Test with invalid JSON marshaling (this is harder to trigger in practice)
	// For now, we'll test normal error paths

	handler, service := setupTestHandler(t)

	// Test with database error (simulate by closing database)
	// Since we can't easily simulate database errors with in-memory SQLite,
	// we'll test the normal happy path and rely on integration tests for error cases

	// Test collect metrics on disabled service
	service.Disable()

	req := httptest.NewRequest("POST", "/metrics/collect", nil)
	w := httptest.NewRecorder()

	handler.CollectMetrics(w, req)

	// Should still succeed but not actually collect
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 even when disabled, got %d", w.Code)
	}
}

func TestSetupMetricsRoutes(t *testing.T) {
	// This tests the route setup function
	// We'll create a minimal test to ensure routes are registered

	handler, _ := setupTestHandler(t)

	// Create a test router (simplified version)
	router := http.NewServeMux()

	// Register a few routes manually to test the pattern
	router.Handle("/metrics/prometheus", handler.PrometheusHandler())
	router.HandleFunc("/metrics/status", handler.GetMetricsStatus)
	router.HandleFunc("/metrics/enable", handler.EnableMetrics)
	router.HandleFunc("/metrics/disable", handler.DisableMetrics)
	router.HandleFunc("/metrics/collect", handler.CollectMetrics)

	// Test that routes respond
	tests := []struct {
		method   string
		path     string
		expected int
	}{
		{"GET", "/metrics/prometheus", http.StatusOK},
		{"GET", "/metrics/status", http.StatusOK},
		{"POST", "/metrics/enable", http.StatusOK},
		{"POST", "/metrics/disable", http.StatusOK},
		{"POST", "/metrics/collect", http.StatusOK},
	}

	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != test.expected {
			t.Errorf("Route %s %s: expected status %d, got %d",
				test.method, test.path, test.expected, w.Code)
		}
	}
}

func TestMetricsStatusStructure(t *testing.T) {
	// Test the MetricsStatus struct marshaling
	status := MetricsStatus{
		Enabled:            true,
		LastCollectionTime: time.Now(),
		UptimeSeconds:      123.45,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal MetricsStatus: %v", err)
	}

	var unmarshaled MetricsStatus
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal MetricsStatus: %v", err)
	}

	if unmarshaled.Enabled != status.Enabled {
		t.Error("Enabled field not preserved")
	}

	if unmarshaled.UptimeSeconds != status.UptimeSeconds {
		t.Error("UptimeSeconds field not preserved")
	}

	// Time comparison with some tolerance for JSON marshaling
	if abs(unmarshaled.LastCollectionTime.Sub(status.LastCollectionTime)) > time.Second {
		t.Error("LastCollectionTime field not preserved accurately")
	}
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func TestHandlerWithBadRequests(t *testing.T) {
	handler, _ := setupTestHandler(t)

	// Test with invalid request body (though our handlers don't currently parse JSON)
	tests := []struct {
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{"POST", "/metrics/enable", "invalid json", http.StatusOK},  // Handler doesn't parse body
		{"POST", "/metrics/disable", "invalid json", http.StatusOK}, // Handler doesn't parse body
		{"POST", "/metrics/collect", "invalid json", http.StatusOK}, // Handler doesn't parse body
	}

	for _, test := range tests {
		req := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		switch test.path {
		case "/metrics/enable":
			handler.EnableMetrics(w, req)
		case "/metrics/disable":
			handler.DisableMetrics(w, req)
		case "/metrics/collect":
			handler.CollectMetrics(w, req)
		}

		if w.Code != test.expectedCode {
			t.Errorf("Request %s %s: expected status %d, got %d",
				test.method, test.path, test.expectedCode, w.Code)
		}
	}
}

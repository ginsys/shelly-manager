package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func TestHealthz_OK(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()

	handler := NewHandlerWithLogger(db, nil, nil, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	handler.Healthz(w, req)

	testutil.AssertEqual(t, 200, w.Code)
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// Expect OK when DB is reachable
	testutil.AssertEqual(t, "ok", body.Status)
}

func TestReadyz_OK(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()

	handler := NewHandlerWithLogger(db, nil, nil, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.Readyz(w, req)

	testutil.AssertEqual(t, 200, w.Code)
	var body struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if body.Data == nil || body.Data["ready"] != true {
		t.Fatalf("expected ready=true, got: %+v", body.Data)
	}
}

// TestHealthEndpointsWithCurlUserAgent ensures health endpoints are accessible
// with curl user agent to prevent E2E test failures (regression test for GitHub Actions)
func TestHealthEndpointsWithCurlUserAgent(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()

	logger := logging.GetDefault()
	handler := NewHandlerWithLogger(db, nil, nil, nil, logger)

	// Set up router with security middleware (same as production)
	router := SetupRoutesWithSecurity(handler, logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())

	tests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"Health endpoint with curl", "/healthz", "GET"},
		{"Readiness endpoint with curl", "/readyz", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.endpoint, nil)
			// Set curl user agent (this would normally be blocked by validation middleware)
			req.Header.Set("User-Agent", "curl/8.5.0")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Health endpoints should return 200 OK even with curl user agent
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
			}

			// Verify response contains expected content
			body := w.Body.String()
			if tt.endpoint == "/healthz" && !strings.Contains(body, `"status"`) {
				t.Errorf("Expected health response to contain status field, got: %s", body)
			}
			if tt.endpoint == "/readyz" && !strings.Contains(body, `"ready"`) {
				t.Errorf("Expected readiness response to contain ready field, got: %s", body)
			}
		})
	}
}

// TestAPIEndpointsStillValidateUserAgent ensures that API endpoints still have validation
// This test verifies the validation middleware directly to avoid Prometheus registration conflicts
func TestAPIEndpointsStillValidateUserAgent(t *testing.T) {
	logger := logging.GetDefault()
	validationConfig := middleware.DefaultValidationConfig()

	// Create a simple handler that returns 200 OK (to test if middleware blocks it)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Apply validation middleware
	middleware := middleware.ValidateHeadersMiddleware(validationConfig, logger)
	wrappedHandler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	// Set curl user agent (this should be blocked by validation middleware)
	req.Header.Set("User-Agent", "curl/8.5.0")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Validation middleware should still block curl user agent
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Validation middleware should still block curl user agents", w.Code)
	}

	// Verify the response contains validation error
	body := w.Body.String()
	if !strings.Contains(body, "VALIDATION_FAILED") {
		t.Errorf("Expected validation error, got: %s", body)
	}
}

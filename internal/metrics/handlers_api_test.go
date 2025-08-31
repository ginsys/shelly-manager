package metrics

import (
	"net/http/httptest"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/prometheus/client_golang/prometheus"
)

func TestHealthEndpointAuth(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	svc := NewService(nil, logger, prometheus.NewRegistry())
	h := NewHandler(svc, logger)
	h.SetAdminAPIKey("secret")

	// Unauthorized
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics/health", nil)
	h.GetHealth(rr, req)
	if rr.Code != 401 {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// Authorized via header
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/metrics/health", nil)
	req2.Header.Set("Authorization", "Bearer secret")
	h.GetHealth(rr2, req2)
	if rr2.Code != 200 {
		t.Fatalf("expected 200, got %d", rr2.Code)
	}
}

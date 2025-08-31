package metrics

import (
	"net/http/httptest"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestWebSocketRequiresAdminWhenConfigured(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	svc := NewService(nil, logger, nil)
	h := NewHandler(svc, logger)
	h.SetAdminAPIKey("secret")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics/ws", nil)

	// Should reject without token/header
	h.HandleWebSocket(rr, req)
	if rr.Code != 401 {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

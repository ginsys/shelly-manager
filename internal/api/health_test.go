package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

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

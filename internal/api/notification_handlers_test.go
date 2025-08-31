package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fmt"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func setupNotificationTestRouter(t *testing.T) (*mux.Router, func()) {
	t.Helper()

	db, cleanup := testutil.TestDatabase(t)
	logger := logging.GetDefault()

	// Minimal service for handler; ShellyService not needed for these routes
	notifEmail := notification.EmailSMTPConfig{Host: "localhost", Port: 25, From: "noreply@example.com"}
	notifSvc := notification.NewService(db.GetDB(), logger, notifEmail)
	notifHandler := notification.NewHandler(notifSvc, logger)

	h := NewHandlerWithLogger(db, nil, notifHandler, nil, logger)
	// Build minimal router without heavy middleware to focus on handler behavior
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/notifications/channels", h.NotificationHandler.CreateChannel).Methods("POST")
	api.HandleFunc("/notifications/channels", h.NotificationHandler.GetChannels).Methods("GET")
	api.HandleFunc("/notifications/channels/{id}", h.NotificationHandler.UpdateChannel).Methods("PUT")
	api.HandleFunc("/notifications/channels/{id}", h.NotificationHandler.DeleteChannel).Methods("DELETE")
	api.HandleFunc("/notifications/channels/{id}/test", h.NotificationHandler.TestChannel).Methods("POST")
	api.HandleFunc("/notifications/rules", h.NotificationHandler.CreateRule).Methods("POST")
	api.HandleFunc("/notifications/rules", h.NotificationHandler.GetRules).Methods("GET")
	api.HandleFunc("/notifications/history", h.NotificationHandler.GetHistory).Methods("GET")
	return r, cleanup
}

func TestNotificationHandlers_ChannelsCRUD(t *testing.T) {
	router, cleanup := setupNotificationTestRouter(t)
	defer cleanup()

	// Create channel (email)
	createBody := map[string]interface{}{
		"name":    "Admins",
		"type":    "email",
		"enabled": true,
		"config": map[string]interface{}{
			"recipients": []string{"ops@example.com"},
			"subject":    "Test",
		},
		"description": "Admin alerts",
	}
	b, _ := json.Marshal(createBody)
	req := httptest.NewRequest("POST", "/api/v1/notifications/channels", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var wrap map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if ok, _ := wrap["success"].(bool); !ok {
		t.Fatalf("expected success=true, got: %v", wrap)
	}
	data := wrap["data"].(map[string]interface{})
	id := int(data["id"].(float64))

	// List channels
	reqL := httptest.NewRequest("GET", "/api/v1/notifications/channels", nil)
	reqL.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rrL := httptest.NewRecorder()
	router.ServeHTTP(rrL, reqL)
	if rrL.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", rrL.Code)
	}

	// Update channel
	upd := map[string]interface{}{
		"name": "Admins-Updated",
		"type": "email",
		"config": map[string]interface{}{
			"recipients": []string{"ops@example.com"},
		},
	}
	bu, _ := json.Marshal(upd)
	reqU := httptest.NewRequest("PUT",
		fmt.Sprintf("/api/v1/notifications/channels/%s", strconv.Itoa(id)), bytes.NewReader(bu))
	reqU.Header.Set("Content-Type", "application/json")
	reqU.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rrU := httptest.NewRecorder()
	router.ServeHTTP(rrU, reqU)
	if rrU.Code != http.StatusOK {
		t.Fatalf("update expected 200, got %d body=%s", rrU.Code, rrU.Body.String())
	}

	// Delete channel
	reqD := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/notifications/channels/%s", strconv.Itoa(id)), nil)
	reqD.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rrD := httptest.NewRecorder()
	router.ServeHTTP(rrD, reqD)
	if rrD.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d body=%s", rrD.Code, rrD.Body.String())
	}
}

func TestNotificationHandlers_TestChannelNotFound(t *testing.T) {
	router, cleanup := setupNotificationTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/v1/notifications/channels/999/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
	var wrap map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &wrap)
	if code, _ := wrap["error"].(map[string]interface{})["code"].(string); code == "" {
		t.Fatalf("expected error.code present, got: %v", wrap)
	}
}

func TestNotificationHandlers_HistoryEmpty(t *testing.T) {
	router, cleanup := setupNotificationTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/notifications/history?limit=2&offset=0", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var wrap map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &wrap)
	data := wrap["data"].(map[string]interface{})
	if _, ok := data["history"].([]interface{}); !ok {
		t.Fatalf("expected history array in data, got: %v", data)
	}
	// meta should include count/total_count and pagination
	if meta, ok := wrap["meta"].(map[string]interface{}); ok {
		_ = meta
	} else {
		t.Fatalf("expected meta in response")
	}
}

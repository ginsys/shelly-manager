package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/redact"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// Every value below is a credential for its destination. A response, a log line or an error text
// that carries any of them is a leak.
const (
	leakPath    = "LEAKPATH620"
	leakQuery   = "LEAKQUERY620"
	leakBearer  = "Bearer LEAKBEARER620"
	leakSecret  = "LEAKSECRET620"
	leakSlack   = "https://hooks.slack.com/services/T1/B1/LEAKSLACK620"
	leakUserPwd = "LEAKPWD620"
)

var allLeaks = []string{leakPath, leakQuery, "LEAKBEARER620", leakSecret, "LEAKSLACK620", leakUserPwd}

type notificationSecretsEnv struct {
	router *mux.Router
	svc    *notification.Service
	db     *gorm.DB
}

func setupNotificationSecrets(t *testing.T) *notificationSecretsEnv {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger := logging.GetDefault()
	svc := notification.NewService(db.GetDB(), logger, notification.EmailSMTPConfig{Host: "localhost", Port: 25, From: "noreply@example.com"})
	handler := notification.NewHandler(svc, logger)

	r := mux.NewRouter()
	a := r.PathPrefix("/api/v1").Subrouter()
	a.HandleFunc("/notifications/channels", handler.CreateChannel).Methods("POST")
	a.HandleFunc("/notifications/channels", handler.GetChannels).Methods("GET")
	a.HandleFunc("/notifications/channels/{id}", handler.UpdateChannel).Methods("PUT")
	a.HandleFunc("/notifications/channels/{id}/test", handler.TestChannel).Methods("POST")
	a.HandleFunc("/notifications/rules", handler.CreateRule).Methods("POST")
	a.HandleFunc("/notifications/rules", handler.GetRules).Methods("GET")
	a.HandleFunc("/notifications/history", handler.GetHistory).Methods("GET")
	return &notificationSecretsEnv{router: r, svc: svc, db: db.GetDB()}
}

func (e *notificationSecretsEnv) do(t *testing.T, method, path string, body any) (int, string) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr.Code, rr.Body.String()
}

func (e *notificationSecretsEnv) createChannel(t *testing.T, body map[string]any) (uint, string) {
	t.Helper()
	code, resp := e.do(t, "POST", "/notifications/channels", body)
	require.Equal(t, http.StatusCreated, code, resp)
	var wrap struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp), &wrap))
	return wrap.Data.ID, resp
}

func (e *notificationSecretsEnv) storedConfig(t *testing.T, id uint) map[string]any {
	t.Helper()
	var ch notification.NotificationChannel
	require.NoError(t, e.db.First(&ch, id).Error)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(ch.Config, &cfg))
	return cfg
}

// closedPort returns a loopback port nothing listens on, so a send to it fails with a
// connection error that quotes the URL.
func closedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

func assertNoLeaks(t *testing.T, what, text string) {
	t.Helper()
	for _, leak := range allLeaks {
		assert.NotContains(t, text, leak, "%s leaks %s", what, leak)
	}
}

func deadWebhook(port int) map[string]any {
	return map[string]any{
		"name": "ops-hook", "type": "webhook", "enabled": true,
		"config": map[string]any{
			"url":     fmt.Sprintf("http://hook:%s@127.0.0.1:%d/%s?t=%s", leakUserPwd, port, leakPath, leakQuery),
			"headers": map[string]string{"Authorization": leakBearer},
			"secret":  leakSecret,
			"method":  "POST",
			"timeout": 2,
		},
	}
}

// Every read that carries a channel config sends the placeholder, never the stored value, and the
// stored row keeps the real one.
func TestNotificationSecrets_ReadsMaskEveryCarrier(t *testing.T) {
	env := setupNotificationSecrets(t)
	port := closedPort(t)

	hookID, createResp := env.createChannel(t, deadWebhook(port))
	slackID, slackResp := env.createChannel(t, map[string]any{
		"name": "ops-slack", "type": "slack", "enabled": true,
		"config": map[string]any{"webhook_url": leakSlack, "channel": "#ops", "username": "bot"},
	})
	emailID, _ := env.createChannel(t, map[string]any{
		"name": "ops-mail", "type": "email", "enabled": true,
		"config": map[string]any{"recipients": []string{"ops@example.com"}, "subject": "alert"},
	})

	assertNoLeaks(t, "the webhook create response", createResp)
	assertNoLeaks(t, "the slack create response", slackResp)

	code, ruleResp := env.do(t, "POST", "/notifications/rules", map[string]any{
		"name": "all-to-hook", "enabled": true, "channel_id": hookID, "alert_level": "all",
	})
	require.Equal(t, http.StatusCreated, code, ruleResp)
	assertNoLeaks(t, "the rule create response", ruleResp)

	// A real send to the closed port fails and stores the error, which quotes the URL.
	require.NoError(t, env.svc.SendNotification(context.Background(), &notification.NotificationEvent{
		Type: "drift_detected", AlertLevel: notification.AlertLevelWarning, Title: "drift", Message: "m",
	}))

	for _, path := range []string{"/notifications/channels", "/notifications/rules", "/notifications/history"} {
		code, body := env.do(t, "GET", path, nil)
		require.Equal(t, http.StatusOK, code, body)
		assertNoLeaks(t, "GET "+path, body)
		assert.Contains(t, body, redact.Placeholder, "GET %s shows the placeholder", path)
	}

	// The history row exists and its error text did quote the URL (the control for the scrub).
	var raw string
	require.NoError(t, env.db.Raw("SELECT error FROM notification_histories WHERE channel_id = ?", hookID).Scan(&raw).Error)
	assert.Contains(t, raw, "127.0.0.1", "the stored delivery error quotes the destination")

	// The non-secret fields still come back, and an email channel is unchanged.
	_, channels := env.do(t, "GET", "/notifications/channels", nil)
	assert.Contains(t, channels, "ops@example.com")
	assert.Contains(t, channels, "#ops")
	_ = slackID
	_ = emailID

	// The stored rows keep the real values: a masked copy never changes them.
	stored := env.storedConfig(t, hookID)
	assert.Equal(t, leakSecret, stored["secret"])
	assert.Contains(t, stored["url"], leakPath)
	assert.Equal(t, leakBearer, stored["headers"].(map[string]any)["Authorization"])
	assert.Equal(t, leakSlack, env.storedConfig(t, slackID)["webhook_url"])

	// Reading twice gives the same answer.
	_, a := env.do(t, "GET", "/notifications/channels", nil)
	_, b := env.do(t, "GET", "/notifications/channels", nil)
	// Updated and created times do not move between reads, so the bodies match but for the request id.
	assert.Equal(t, strings.Count(a, redact.Placeholder), strings.Count(b, redact.Placeholder))
}

// A failed test send returns the delivery error as its 500 message; it must not quote the URL.
func TestNotificationSecrets_TestSendErrorCarriesNoURL(t *testing.T) {
	env := setupNotificationSecrets(t)

	id, _ := env.createChannel(t, deadWebhook(closedPort(t)))
	code, body := env.do(t, "POST", fmt.Sprintf("/notifications/channels/%d/test", id), nil)
	require.Equal(t, http.StatusInternalServerError, code, body)
	assertNoLeaks(t, "the failed test send", body)
	assert.Contains(t, body, redact.Placeholder)

	// A URL that does not parse fails inside http.NewRequest and quotes the raw string.
	badID, _ := env.createChannel(t, map[string]any{
		"name": "bad-url", "type": "webhook", "enabled": true,
		"config": map[string]any{"url": "http://[::1:" + leakPath},
	})
	code, body = env.do(t, "POST", fmt.Sprintf("/notifications/channels/%d/test", badID), nil)
	require.Equal(t, http.StatusInternalServerError, code, body)
	assertNoLeaks(t, "the failed test send of an unparseable URL", body)

	// The 404 for a missing channel is unchanged.
	code, _ = env.do(t, "POST", "/notifications/channels/9999/test", nil)
	assert.Equal(t, http.StatusNotFound, code)
}

type capturedRequest struct {
	mu      sync.Mutex
	headers http.Header
	path    string
}

func (c *capturedRequest) handler(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers, c.path = r.Header.Clone(), r.URL.RequestURI()
	w.WriteHeader(http.StatusOK)
}

// A save that sends the placeholder back keeps the stored value; a new value replaces it; a
// placeholder with nothing stored is dropped and never stored.
func TestNotificationSecrets_SaveKeepsStoredValueBehindThePlaceholder(t *testing.T) {
	env := setupNotificationSecrets(t)
	var got capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(got.handler))
	t.Cleanup(srv.Close)

	id, _ := env.createChannel(t, map[string]any{
		"name": "kept", "type": "webhook", "enabled": true,
		"config": map[string]any{
			"url":     srv.URL + "/" + leakPath,
			"headers": map[string]string{"Authorization": leakBearer, "X-Env": "prod"},
			"secret":  leakSecret,
			"timeout": 5,
		},
	})

	// The client read the config (every secret slot is the placeholder) and sends it back with one
	// plain field changed. The body is written out here, not taken from a read, so this tests the
	// restore on its own.
	cfg := map[string]any{
		"url":     redact.Placeholder,
		"headers": map[string]string{"Authorization": redact.Placeholder, "X-Env": redact.Placeholder},
		"secret":  redact.Placeholder,
		"timeout": 9,
	}
	code, resp := env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id),
		map[string]any{"name": "kept", "type": "webhook", "config": cfg})
	require.Equal(t, http.StatusOK, code, resp)

	stored := env.storedConfig(t, id)
	assert.Equal(t, leakSecret, stored["secret"], "the secret stays")
	assert.Equal(t, srv.URL+"/"+leakPath, stored["url"], "the url stays")
	headers := stored["headers"].(map[string]any)
	assert.Equal(t, leakBearer, headers["Authorization"])
	assert.Equal(t, "prod", headers["X-Env"], "every header value is a slot, so the placeholder restored it too")
	assert.EqualValues(t, 9, stored["timeout"], "the plain field changed")
	assert.NotContains(t, fmt.Sprint(stored), redact.Placeholder, "the placeholder is never stored")

	// A real send still carries the credentials.
	require.NoError(t, env.svc.TestChannel(context.Background(), id))
	got.mu.Lock()
	defer got.mu.Unlock()
	assert.Equal(t, leakBearer, got.headers.Get("Authorization"))
	assert.NotEmpty(t, got.headers.Get("X-Signature"), "the signing secret survived")
	assert.Equal(t, "/"+leakPath, got.path)
}

func TestNotificationSecrets_SaveReplacesAndDrops(t *testing.T) {
	env := setupNotificationSecrets(t)
	id, _ := env.createChannel(t, map[string]any{
		"name": "swap", "type": "webhook", "enabled": true,
		"config": map[string]any{"url": "http://old.example/" + leakPath, "secret": leakSecret},
	})

	// A new value replaces the stored one; the placeholder on the other slot keeps its value.
	code, resp := env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
		"name": "swap", "type": "webhook",
		"config": map[string]any{"url": "http://new.example/hook", "secret": redact.Placeholder},
	})
	require.Equal(t, http.StatusOK, code, resp)
	stored := env.storedConfig(t, id)
	assert.Equal(t, "http://new.example/hook", stored["url"])
	assert.Equal(t, leakSecret, stored["secret"])

	// A placeholder where nothing is stored is dropped, never stored.
	code, resp = env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
		"name": "swap", "type": "webhook",
		"config": map[string]any{
			"url":     "http://new.example/hook",
			"headers": map[string]string{"X-Auth": redact.Placeholder},
		},
	})
	require.Equal(t, http.StatusOK, code, resp)
	stored = env.storedConfig(t, id)
	assert.NotContains(t, fmt.Sprint(stored), redact.Placeholder)
	assert.NotContains(t, stored, "secret", "a client that leaves the secret out replaces the config, as before")

	// A save that changes the type and sends the placeholder where the stored config has no such
	// value is refused, and the channel is left as it was: the placeholder cannot pass validation.
	before := env.storedConfig(t, id)
	code, resp = env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
		"name": "swap", "type": "slack",
		"config": map[string]any{"webhook_url": redact.Placeholder},
	})
	assert.Equal(t, http.StatusBadRequest, code, resp)
	assert.Equal(t, before, env.storedConfig(t, id))

	// A webhook whose url arrives as the placeholder with nothing stored fails validation.
	code, resp = env.do(t, "POST", "/notifications/channels", map[string]any{
		"name": "no-url", "type": "webhook", "config": map[string]any{"url": redact.Placeholder},
	})
	assert.Equal(t, http.StatusBadRequest, code, resp)
}

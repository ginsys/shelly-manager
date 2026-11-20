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
	a.HandleFunc("/notifications/channels/{id}", handler.GetChannel).Methods("GET")
	a.HandleFunc("/notifications/channels/{id}", handler.UpdateChannel).Methods("PUT")
	a.HandleFunc("/notifications/channels/{id}/test", handler.TestChannel).Methods("POST")
	a.HandleFunc("/notifications/rules", handler.CreateRule).Methods("POST")
	a.HandleFunc("/notifications/rules", handler.GetRules).Methods("GET")
	a.HandleFunc("/notifications/rules/{id}", handler.DeleteRule).Methods("DELETE")
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

	for _, path := range []string{"/notifications/channels", "/notifications/rules"} {
		status, body := env.do(t, "GET", path, nil)
		require.Equal(t, http.StatusOK, status, body)
		assertNoLeaks(t, "GET "+path, body)
		assert.Contains(t, body, redact.Placeholder, "GET %s shows the placeholder", path)
	}

	// The history read carries the channel's masked config and the row's own error text, which names the
	// operation and the failure class and never the request URL.
	code, history := env.do(t, "GET", "/notifications/history", nil)
	require.Equal(t, http.StatusOK, code, history)
	assertNoLeaks(t, "GET /notifications/history", history)
	assert.Contains(t, history, redact.Placeholder)
	var raw string
	require.NoError(t, env.db.Raw("SELECT error FROM notification_histories WHERE channel_id = ?", hookID).Scan(&raw).Error)
	assert.Equal(t, "webhook request failed: Post connection refused", raw, "the stored row holds the class, not the URL")

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

// A failed test send is a 500 whose message names the operation and the failure class and never the
// request URL, for a webhook and for a Slack channel; a missing channel is a 404.
func TestNotificationSecrets_TestSendBehaviourIsUnchanged(t *testing.T) {
	env := setupNotificationSecrets(t)

	id, _ := env.createChannel(t, deadWebhook(closedPort(t)))
	code, body := env.do(t, "POST", fmt.Sprintf("/notifications/channels/%d/test", id), nil)
	assert.Equal(t, http.StatusInternalServerError, code, body)
	assert.Contains(t, body, "webhook request failed: Post connection refused")
	assertNoLeaks(t, "the webhook test-send response", body)
	assert.NotContains(t, body, "127.0.0.1")

	slack := fmt.Sprintf("http://127.0.0.1:%d/services/T1/B1/LEAKSLACK620", closedPort(t))
	slackID, _ := env.createChannel(t, map[string]any{
		"name": "dead-slack", "type": "slack", "enabled": true,
		"config": map[string]any{"webhook_url": slack},
	})
	code, body = env.do(t, "POST", fmt.Sprintf("/notifications/channels/%d/test", slackID), nil)
	assert.Equal(t, http.StatusInternalServerError, code, body)
	assert.Contains(t, body, "slack request failed: Post connection refused")
	assertNoLeaks(t, "the slack test-send response", body)

	code, _ = env.do(t, "POST", "/notifications/channels/9999/test", nil)
	assert.Equal(t, http.StatusNotFound, code)
}

// A config key in another casing is read by validation and delivery, so it is a credential slot too.
func TestNotificationSecrets_AnyKeyCasingIsMaskedAndRestored(t *testing.T) {
	env := setupNotificationSecrets(t)
	var got capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(got.handler))
	t.Cleanup(srv.Close)

	id, createResp := env.createChannel(t, map[string]any{
		"name": "cased", "type": "webhook", "enabled": true,
		"config": map[string]any{
			"URL":     srv.URL + "/" + leakPath,
			"Headers": map[string]string{"Authorization": leakBearer},
			"Secret":  leakSecret,
		},
	})
	assertNoLeaks(t, "the create response", createResp)
	_, list := env.do(t, "GET", "/notifications/channels", nil)
	assertNoLeaks(t, "the channel list", list)
	assert.Contains(t, list, redact.Placeholder)

	slackID, slackResp := env.createChannel(t, map[string]any{
		"name": "cased-slack", "type": "slack", "enabled": true,
		"config": map[string]any{"Webhook_URL": leakSlack},
	})
	assertNoLeaks(t, "the slack create response", slackResp)
	_, list = env.do(t, "GET", "/notifications/channels", nil)
	assertNoLeaks(t, "the channel list with the slack channel", list)
	_ = slackID

	// The client sends back the read, in the casing it read.
	code, resp := env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
		"name": "cased", "type": "webhook",
		"config": map[string]any{
			"URL":     redact.Placeholder,
			"Headers": redact.Placeholder,
			"Secret":  redact.Placeholder,
		},
	})
	require.Equal(t, http.StatusOK, code, resp)
	assert.NotContains(t, fmt.Sprint(env.storedConfig(t, id)), redact.Placeholder)

	require.NoError(t, env.svc.TestChannel(context.Background(), id))
	got.mu.Lock()
	defer got.mu.Unlock()
	assert.Equal(t, leakBearer, got.headers.Get("Authorization"))
	assert.Equal(t, "/"+leakPath, got.path)
}

// A config that repeats a member name is refused on create and on update with a 400, in any spelling
// of the name, and the stored channel is left as it was: encoding/json reads such a text in member
// order, so no restore of a placeholder can promise to keep the destination delivery reads.
func TestNotificationSecrets_RepeatedMembersAreRefusedOnSave(t *testing.T) {
	env := setupNotificationSecrets(t)
	id, _ := env.createChannel(t, map[string]any{
		"name": "once", "type": "webhook", "enabled": true,
		"config": map[string]any{"url": "http://old.example/hook", "headers": map[string]string{"Authorization": leakBearer}},
	})
	before := env.storedConfig(t, id)

	for name, cfg := range map[string]string{
		"two spellings of url":   `{"url":"http://old.example/hook","URL":"http://new.example/hook","headers":{"Authorization":"` + leakBearer + `"}}`,
		"the placeholder, null":  `{"url":"` + redact.Placeholder + `","url":null}`,
		"behind a huge number":   `{"n":1e1000,"url":"http://old.example","URL":"http://new.example","headers":{"Authorization":"` + redact.Placeholder + `"}}`,
		"repeated header member": `{"url":"http://h.example","headers":{"Authorization":"a","authorization":"b"}}`,
	} {
		code, resp := env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
			"name": "once", "type": "webhook", "config": json.RawMessage(cfg),
		})
		assert.Equal(t, http.StatusBadRequest, code, "update, %s: %s", name, resp)
		assert.Equal(t, before, env.storedConfig(t, id), "update, %s: the stored channel is unchanged", name)

		code, resp = env.do(t, "POST", "/notifications/channels", map[string]any{
			"name": "twice-" + name, "type": "webhook", "config": json.RawMessage(cfg),
		})
		assert.Equal(t, http.StatusBadRequest, code, "create, %s: %s", name, resp)
	}
	_, list := env.do(t, "GET", "/notifications/channels", nil)
	assert.Equal(t, 1, strings.Count(list, `"name"`), "no channel was created by a refused save")
}

// A list of credentials is one masked value on read and comes back whole when the client sends the read back.
func TestNotificationSecrets_ListOfSecretsRoundTrips(t *testing.T) {
	env := setupNotificationSecrets(t)
	id, createResp := env.createChannel(t, map[string]any{
		"name": "listed", "type": "webhook", "enabled": true,
		"config": map[string]any{"url": "http://h.example/x", "token_list": []string{"LEAKLIST1-620", "LEAKLIST2-620"}},
	})
	assert.NotContains(t, createResp, "LEAKLIST")

	var wrap struct {
		Data struct {
			Channels []struct {
				Config map[string]any `json:"config"`
			} `json:"channels"`
		} `json:"data"`
	}
	_, list := env.do(t, "GET", "/notifications/channels", nil)
	assert.NotContains(t, list, "LEAKLIST")
	require.NoError(t, json.Unmarshal([]byte(list), &wrap))
	require.Len(t, wrap.Data.Channels, 1)
	read := wrap.Data.Channels[0].Config
	assert.Equal(t, redact.Placeholder, read["token_list"])

	code, resp := env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id),
		map[string]any{"name": "listed", "type": "webhook", "config": read})
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, []any{"LEAKLIST1-620", "LEAKLIST2-620"}, env.storedConfig(t, id)["token_list"])

	// A new list replaces the stored one as sent.
	code, resp = env.do(t, "PUT", fmt.Sprintf("/notifications/channels/%d", id), map[string]any{
		"name": "listed", "type": "webhook",
		"config": map[string]any{"url": "http://h.example/x", "token_list": []string{"NEWLIST-620"}},
	})
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, []any{"NEWLIST-620"}, env.storedConfig(t, id)["token_list"])
}

// A list or an object under a url key is masked as one value, even on a channel type that never
// reads the key: the config still holds it and the read must not return it.
func TestNotificationSecrets_ContainerUnderURLKeyIsMasked(t *testing.T) {
	env := setupNotificationSecrets(t)
	id, createResp := env.createChannel(t, map[string]any{
		"name": "mail", "type": "email", "enabled": true,
		"config": map[string]any{
			"recipients":  []string{"ops@example.com"},
			"url":         []string{"https://hook.example/SECRET620"},
			"webhook_url": map[string]any{"a": map[string]any{"b": "https://hook.example/S2-620"}},
		},
	})
	assertNoLeaks(t, "the create response", createResp)
	assert.NotContains(t, createResp, "SECRET620")
	assert.NotContains(t, createResp, "S2-620")

	_, list := env.do(t, "GET", "/notifications/channels", nil)
	assert.NotContains(t, list, "SECRET620")
	assert.NotContains(t, list, "S2-620")
	assert.Contains(t, list, "ops@example.com")
	assert.Contains(t, fmt.Sprint(env.storedConfig(t, id)), "SECRET620", "the stored row keeps the value")
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
		"headers": redact.Placeholder,
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
	assert.Equal(t, "prod", headers["X-Env"], "the headers object comes back whole")
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
			"headers": redact.Placeholder,
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

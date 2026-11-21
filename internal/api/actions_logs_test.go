package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// An action URL carries a login (http://user:password@host/...), so no log line may hold one. The
// device can echo what it was sent, but the Gen1 client puts a device's text in an error only for a
// write answered 200 with an "error" field (client.go, postValues). So only the two write-refused
// rows (apply, bulk apply) can go red today, and they are the ones that fail when hideActionURLs is
// removed. The four read-refused rows guard a later client change that would carry a read body into
// an error: they are no proof of anything today. Every path logs through the one logger this test reads: the handler,
// the configuration services that apply, verify and detect drift, and the SQL trace.

const logCanary = "canary-pw-5b1e8d"

var logCanaryURL = "http://admin:" + logCanary + "@192.168.1.4/on?token=" + logCanary + "-t"

// actionsDevice is a Gen1 plug that keeps the action it is sent. A failing write answers with an
// error that echoes the form it was sent, as a device can; a failing read answers 400.
type actionsDevice struct {
	mu        sync.Mutex
	urls      []string
	enabled   bool
	failWrite bool
	failRead  bool
}

func (d *actionsDevice) set(write, read bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failWrite, d.failRead = write, read
}

func (d *actionsDevice) held() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.urls
}

func (d *actionsDevice) hold(urls ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.urls = urls
}

func (d *actionsDevice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case r.URL.Path == "/shelly":
		_, _ = w.Write([]byte(`{"type":"SHPLG-S","mac":"A4CF12345678","auth":false,"fw":"1.14.0"}`))
	case r.URL.Path == "/settings/actions" && r.Method == http.MethodPost:
		raw, _ := io.ReadAll(r.Body)
		if d.failWrite {
			// The client reads an "error" in a 200 answer as the device's message, so this is the
			// one answer whose text reaches the error that apply logs.
			_, _ = fmt.Fprintf(w, `{"error":%q}`, "cannot store "+string(raw))
			return
		}
		form, _ := url.ParseQuery(string(raw))
		d.urls, d.enabled = nil, form.Get("enabled") == "true"
		for _, u := range form["urls[]"] {
			if u != "" {
				d.urls = append(d.urls, u)
			}
		}
		_, _ = w.Write([]byte(`{}`))
	case r.URL.Path == "/settings/actions":
		if d.failRead {
			http.Error(w, "cannot read "+strings.Join(d.urls, " "), http.StatusBadRequest)
			return
		}
		quoted := make([]string, len(d.urls))
		for i, u := range d.urls {
			quoted[i] = fmt.Sprintf("%q", u)
		}
		_, _ = fmt.Fprintf(w, `{"actions":{"out_on_url":[{"index":0,"urls":[%s],"enabled":%t}]}}`, strings.Join(quoted, ","), d.enabled)
	default:
		_, _ = w.Write([]byte(`{}`))
	}
}

// logHarness is a handler whose services, handler and database all write to one log file.
type logHarness struct {
	handler *Handler
	device  *actionsDevice
	id      uint
	logPath string
}

func newLogHarness(t *testing.T) *logHarness {
	t.Helper()
	// `make test` turns this on, and the sqlite provider then swaps the GORM logger for one that
	// discards output: the trace has to be the production logger's.
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "false")
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: logPath})
	require.NoError(t, err)
	db, err := database.NewManagerWithLogger(provider.DatabaseConfig{
		Provider: "sqlite", DSN: filepath.Join(dir, "test.db"), LogLevel: "info",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_ = logger.Close()
	})

	svc := service.NewServiceWithLogger(db, testutil.TestConfig(), logger)
	svc.SetDiscoverFunc(noDiscovery)
	handler := NewHandlerWithLogger(db, svc, testNotificationHandler(t, db), nil, logger)

	fake := &actionsDevice{}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	device := deviceAt(t, db, strings.TrimPrefix(server.URL, "http://"), "A4:CF:12:34:56:78")
	return &logHarness{handler: handler, device: fake, id: device.ID, logPath: logPath}
}

func (h *logHarness) log(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(h.logPath)
	require.NoError(t, err)
	return string(raw)
}

// run calls one route and reads the log: the control line must be among the lines this call wrote,
// and those lines must not hold the canary. A missing control makes the read unknown, never clean.
func (h *logHarness) run(t *testing.T, control string, call func() *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()
	before := len(h.log(t))
	w := call()
	written := h.log(t)[before:]
	require.Contains(t, written, control, "this call wrote no line of its path, so a clean read proves nothing")
	assert.False(t, strings.Contains(written, logCanary), "the log holds an action URL password")
	return w
}

func (h *logHarness) route(fn func(http.ResponseWriter, *http.Request)) func() *httptest.ResponseRecorder {
	return func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d", h.id), nil)
		req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(h.id)})
		w := httptest.NewRecorder()
		fn(w, req)
		return w
	}
}

func (h *logHarness) bulk(call func(*Handler, http.ResponseWriter, *http.Request), path, body string) func() *httptest.ResponseRecorder {
	return func() *httptest.ResponseRecorder { return postBulk(h.handler, call, path, body) }
}

func logActionsBody(enabled bool) string {
	return fmt.Sprintf(`{"actions":[{"name":"out_on_url","index":0,"enabled":%t,"secret_urls":[%q]}]}`, enabled, logCanaryURL)
}

func TestActionLogs_NoPathLogsAnActionURL(t *testing.T) {
	h := newLogHarness(t)
	ids := fmt.Sprintf(`{"device_ids":[%d]}`, h.id)
	override := func(method string, enabled bool) func() *httptest.ResponseRecorder {
		return func() *httptest.ResponseRecorder {
			return sendOverrides(t, h.handler, method, h.id, logActionsBody(enabled))
		}
	}

	// The write routes. The statement that writes the row holding the URL is in the SQL trace.
	require.Equal(t, http.StatusOK, h.run(t, "Device overrides updated", override("PUT", true)).Code)
	require.Equal(t, http.StatusOK, h.run(t, "UPDATE `devices`", override("PATCH", false)).Code)
	// The template routes write the same list.
	created := h.run(t, "Template created via API", func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/config/templates/new",
			strings.NewReader(`{"name":"logs","scope":"global","config":`+logActionsBody(true)+`}`))
		w := httptest.NewRecorder()
		h.handler.CreateNewConfigTemplate(w, req)
		return w
	})
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var template struct {
		Data struct {
			Template TemplateResponse `json:"template"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &template))
	require.Equal(t, http.StatusOK, h.run(t, "Template updated via API", func() *httptest.ResponseRecorder {
		return putKeepTemplate(t, h.handler, template.Data.Template.ID, `{"config":`+logActionsBody(false)+`}`)
	}).Code)

	bulk := h.run(t, "Configuration apply completed", func() *httptest.ResponseRecorder {
		return postApply(h.handler, applyBody(logActionsBody(true), false, h.id))
	})
	assert.Equal(t, 1, decodeApply(t, bulk).Applied, "the bulk apply did not end applied: %s", bulk.Body.String())
	assert.Equal(t, []string{logCanaryURL}, h.device.held(), "bulk apply did not reach the device")

	// Apply, verify and drift on success, then with the device changed behind the stored one. A failed
	// apply and a mismatch also answer 200 and log completion, so each answer is read.
	var applied struct {
		Data struct{ Success bool } `json:"data"`
	}
	applyAnswer := h.run(t, "Apply and verify completed", h.route(h.handler.ApplyDeviceNewConfig))
	require.NoError(t, json.Unmarshal(applyAnswer.Body.Bytes(), &applied))
	assert.True(t, applied.Data.Success, "the apply did not succeed: %s", applyAnswer.Body.String())
	verifyMatch := func(w *httptest.ResponseRecorder) bool {
		var verified struct {
			Data struct{ Match bool } `json:"data"`
		}
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &verified))
		return verified.Data.Match
	}
	assert.True(t, verifyMatch(h.run(t, "Configuration verification completed", h.route(h.handler.VerifyDeviceNewConfig))))
	h.device.hold("http://other:" + logCanary + "-2@192.168.1.9/off")
	assert.False(t, verifyMatch(h.run(t, "difference_count=1", h.route(h.handler.VerifyDeviceNewConfig))))
	require.Equal(t, http.StatusCreated, h.run(t, "Configuration drift detected", h.route(h.handler.GenerateDeviceDriftReport)).Code)
	require.Equal(t, http.StatusOK, h.run(t, "Configuration drift detected during bulk operation",
		h.bulk((*Handler).BulkDetectConfigDrift, "/api/v1/config/bulk-drift-detect", ids)).Code)

	// A device failure: the device refuses the write, answers the write and refuses the read-back,
	// or refuses a read, and echoes the URLs it was sent each time.
	for _, tc := range []struct {
		name        string
		write, read bool
		control     string
		call        func() *httptest.ResponseRecorder
	}{
		{"apply, write refused", true, false, "Failed to apply Actions settings", h.route(h.handler.ApplyDeviceNewConfig)},
		{"apply, read-back refused", false, true, "Failed to apply Actions settings", h.route(h.handler.ApplyDeviceNewConfig)},
		{"bulk apply, write refused", true, false, "Failed to apply Actions settings", func() *httptest.ResponseRecorder {
			return postApply(h.handler, applyBody(logActionsBody(true), false, h.id))
		}},
		{"verify, read refused", false, true, "failed to read the device actions", h.route(h.handler.VerifyDeviceNewConfig)},
		{"drift report, read refused", false, true, "Failed to generate device drift report", h.route(h.handler.GenerateDeviceDriftReport)},
		{"bulk drift, read refused", false, true, "Failed to detect drift for device during bulk operation",
			h.bulk((*Handler).BulkDetectConfigDrift, "/api/v1/config/bulk-drift-detect", ids)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h.device.set(tc.write, tc.read)
			defer h.device.set(false, false)
			h.run(t, tc.control, tc.call)
		})
	}
}

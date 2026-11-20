package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
)

const contentTypeRefusal = "Content-Type header is required"

// The UI posts the metrics controls and the notification test with no data, and axios then sends no
// Content-Type. Through the production chain these answer as the route does, never with the content
// type refusal (ginsys/shelly-manager#767). The headers are the ones the UI client sends.
func TestBodylessPostsPassTheValidationChain(t *testing.T) {
	t.Setenv(testModeEnv, "")
	h, logger := guardWalkHandler(t, "admin-secret", "")
	router := SetupRoutesWithSecurity(h, logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())
	closeMonitor(t, h)

	send := func(method, path, body string) *httptest.ResponseRecorder {
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
		}
		req.Header.Set("Authorization", "Bearer admin-secret")
		req.Header.Set("User-Agent", "Shelly-Manager-UI/1.0 (axios)")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	// Routes whose handlers read no body: the UI's five bodyless calls (test-alert is left out: it
	// sends a notification) and one drift route that takes its id from the path.
	routes := []struct {
		method, path string
		wantStatus   int // 0: the route answers its own status for an id that does not exist
	}{
		{"POST", "/api/v1/metrics/enable", http.StatusOK},
		{"POST", "/api/v1/metrics/disable", http.StatusOK},
		{"POST", "/api/v1/metrics/collect", http.StatusOK},
		{"POST", "/api/v1/notifications/channels/999/test", 0},
		{"POST", "/api/v1/config/drift-schedules/999/toggle", 0},
	}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			rr := send(r.method, r.path, "")
			assert.NotContains(t, rr.Body.String(), contentTypeRefusal, "status %d", rr.Code)
			if r.wantStatus != 0 {
				assert.Equal(t, r.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}

	t.Run("a body with no type is still refused", func(t *testing.T) {
		rr := send("POST", "/api/v1/metrics/enable", `{"x":1}`)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), contentTypeRefusal)
	})

	// The GET the Metrics page calls for "Export Prometheus" never reaches the content type check.
	t.Run("GET prometheus answers 200", func(t *testing.T) {
		rr := send("GET", "/api/v1/metrics/prometheus", "")
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

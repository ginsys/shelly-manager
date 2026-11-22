package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The rate limiter, the security headers and the request validation stand on their own
// (ginsys/shelly-manager#876 removed the security monitor they never depended on).

// limitedRouter builds the production router with a low rate limit.
func limitedRouter(t *testing.T) *mux.Router {
	t.Helper()
	// make test / CI export this, which would return the test-mode router.
	t.Setenv(testModeEnv, "")

	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	msvc := metrics.NewService(db.GetDB(), logger, prometheus.NewRegistry())
	h := NewHandlerWithLogger(db, nil, testNotificationHandler(t, db), metrics.NewHandler(msvc, logger), logger)

	cfg := middleware.DefaultSecurityConfig()
	cfg.RateLimit = 3
	cfg.RateLimitWindow = time.Hour
	cfg.RateLimitByPath = map[string]int{}
	return SetupRoutesWithSecurity(h, logger, cfg, middleware.DefaultValidationConfig())
}

func apiGet(r http.Handler, ip, path, userAgent string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":4000"
	req.Header.Set("User-Agent", userAgent)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

const browserUA = "Mozilla/5.0 (X11; Linux x86_64)"

func TestOtherProtectionsStayOn(t *testing.T) {
	r := limitedRouter(t)

	t.Run("rate limiter", func(t *testing.T) {
		var last *httptest.ResponseRecorder
		for i := 0; i < 6; i++ {
			last = apiGet(r, "10.9.1.1", "/api/v1/devices", browserUA)
		}
		assert.Equal(t, http.StatusTooManyRequests, last.Code)
	})

	t.Run("security headers", func(t *testing.T) {
		rr := apiGet(r, "10.9.1.2", "/api/v1/devices", browserUA)
		assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
		assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy"))
	})

	t.Run("request validation", func(t *testing.T) {
		rr := apiGet(r, "10.9.1.3", "/api/v1/devices", "sqlmap/1.0")
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	})
}

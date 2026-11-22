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

// The deployment default is the security monitor off: no monitor, no tracking, no blocking, and no
// metrics route. The other protections do not depend on the monitor (ginsys/shelly-manager#861).

// monitoringOffRouter builds the production router from a configuration with the monitor and IP
// blocking off, as cmd/shelly-manager does when no key is set, and with a low rate limit.
func monitoringOffRouter(t *testing.T) (*mux.Router, *Handler) {
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
	cfg.EnableMonitoring = false
	cfg.EnableIPBlocking = false
	cfg.RateLimit = 3
	cfg.RateLimitWindow = time.Hour
	cfg.RateLimitByPath = map[string]int{}
	return SetupRoutesWithSecurity(h, logger, cfg, middleware.DefaultValidationConfig()), h
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

// Check 2: no monitor, no route, and a burst of bad requests blocks nobody.
func TestNoMonitorWhenOffRouteIs404AndNobodyIsBlocked(t *testing.T) {
	r, h := monitoringOffRouter(t)
	assert.Nil(t, h.securityMonitor, "no monitor is built")

	assert.Equal(t, http.StatusNotFound, apiGet(r, "10.9.0.1", "/api/v1/metrics/security", browserUA).Code)

	for i := 0; i < 30; i++ {
		rr := apiGet(r, "10.9.0.2", "/api/v1/no-such-route", browserUA)
		assert.NotEqual(t, http.StatusForbidden, rr.Code, "request %d: an IP is never refused as blocked", i)
	}
}

// Check 11: the rate limiter, the security headers and the request validation stay on.
func TestOtherProtectionsStayWithMonitoringOff(t *testing.T) {
	r, _ := monitoringOffRouter(t)

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

// Turned on, the route exists again and answers (the other half of check 2).
func TestMonitorOnServesTheMetricsRoute(t *testing.T) {
	t.Setenv(testModeEnv, "")
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	msvc := metrics.NewService(db.GetDB(), logger, prometheus.NewRegistry())
	h := NewHandlerWithLogger(db, nil, testNotificationHandler(t, db), metrics.NewHandler(msvc, logger), logger)

	r := SetupRoutesWithSecurity(h, logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())
	sm, ok := h.securityMonitor.(*middleware.SecurityMonitor)
	require.True(t, ok)
	t.Cleanup(sm.Close)

	assert.Equal(t, http.StatusOK, apiGet(r, "10.9.2.1", "/api/v1/metrics/security", browserUA).Code)
}

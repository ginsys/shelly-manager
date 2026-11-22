package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/testutil"

	"github.com/prometheus/client_golang/prometheus"
)

// The security monitor and IP blocking are removed (ginsys/shelly-manager#876): the default security
// configuration builds a router with no metrics route that exposes attackers, and no client is ever refused as
// blocked, however many suspicious requests it sends.

func removedMonitorRouter(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv(testModeEnv, "")
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	msvc := metrics.NewService(db.GetDB(), logger, prometheus.NewRegistry())
	h := NewHandlerWithLogger(db, nil, testNotificationHandler(t, db), metrics.NewHandler(msvc, logger), logger)
	h.SetAdminAPIKey("admin-key-876")

	cfg := middleware.DefaultSecurityConfig()
	cfg.RateLimit = 100000 // the rate limiter is not the subject here
	return SetupRoutesWithSecurity(h, logger, cfg, middleware.DefaultValidationConfig())
}

func TestMetricsSecurityRouteIsGone(t *testing.T) {
	r := removedMonitorRouter(t)

	assert.Equal(t, http.StatusNotFound, apiGet(r, "10.8.0.1", "/api/v1/metrics/security", browserUA).Code,
		"no security metrics route, with or without the admin key")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/security", nil)
	req.RemoteAddr = "10.8.0.2:4000"
	req.Header.Set("Authorization", "Bearer admin-key-876")
	req.Header.Set("User-Agent", browserUA)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestNoClientIsBlockedForSuspiciousRequests(t *testing.T) {
	r := removedMonitorRouter(t)
	for i := 0; i < 60; i++ {
		rr := apiGet(r, "10.8.1.1", "/api/v1/devices", "sqlmap/1.0")
		assert.NotEqual(t, http.StatusForbidden, rr.Code, "request %d: no client is refused as blocked", i)
	}
}

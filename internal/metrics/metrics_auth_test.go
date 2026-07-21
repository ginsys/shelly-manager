package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestMetricsAdminEndpoints_RequireAdmin verifies the fix for #246: the mutating
// metrics endpoints (enable/disable/collect/test-alert) were ungated while the
// read endpoints required an admin key — an inverted policy. Each mutating
// endpoint must now reject unauthenticated callers with a configured key AND
// produce no side effect on rejection. The data-read endpoints (status,
// dashboard) are gated too, for consistency with their already-protected
// siblings; /prometheus stays public by convention.
func TestMetricsAdminEndpoints_RequireAdmin(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	svc := NewService(nil, logger, prometheus.NewRegistry())
	h := NewHandler(svc, logger)
	h.SetAdminAPIKey("secret")

	var alertFired int32
	h.SetNotifier(func(_ context.Context, _, _, _ string) {
		atomic.AddInt32(&alertFired, 1)
	})

	call := func(fn http.HandlerFunc, method, path string, auth bool) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		if auth {
			req.Header.Set("Authorization", "Bearer secret")
		}
		fn(rr, req)
		return rr
	}

	// DisableMetrics: enabled -> unauthenticated disable must not change state.
	svc.Enable()
	require.True(t, svc.IsEnabled())
	require.Equal(t, http.StatusUnauthorized, call(h.DisableMetrics, "POST", "/metrics/disable", false).Code)
	require.True(t, svc.IsEnabled(), "unauthenticated disable must not change collection state")

	// EnableMetrics: disabled -> unauthenticated enable must not change state.
	svc.Disable()
	require.False(t, svc.IsEnabled())
	require.Equal(t, http.StatusUnauthorized, call(h.EnableMetrics, "POST", "/metrics/enable", false).Code)
	require.False(t, svc.IsEnabled(), "unauthenticated enable must not change collection state")

	// CollectMetrics: unauthenticated must not advance the last-collection time.
	before := svc.GetLastCollectionTime()
	require.Equal(t, http.StatusUnauthorized, call(h.CollectMetrics, "POST", "/metrics/collect", false).Code)
	require.Equal(t, before, svc.GetLastCollectionTime(), "unauthenticated collect must not run collection")

	// SendTestAlert: unauthenticated must not fire the notifier/broadcast.
	require.Equal(t, http.StatusUnauthorized, call(h.SendTestAlert, "POST", "/metrics/test-alert", false).Code)
	require.Equal(t, int32(0), atomic.LoadInt32(&alertFired), "unauthenticated test-alert must not broadcast")

	// Data-read endpoints are gated for consistency.
	require.Equal(t, http.StatusUnauthorized, call(h.GetMetricsStatus, "GET", "/metrics/status", false).Code)
	require.Equal(t, http.StatusUnauthorized, call(h.GetDashboardMetrics, "GET", "/metrics/dashboard", false).Code)

	// Sanity: with the key the gate opens (not 401), proving it is a real gate
	// and not a blanket block.
	require.NotEqual(t, http.StatusUnauthorized, call(h.EnableMetrics, "POST", "/metrics/enable", true).Code)
	require.NotEqual(t, http.StatusUnauthorized, call(h.DisableMetrics, "POST", "/metrics/disable", true).Code)
	require.NotEqual(t, http.StatusUnauthorized, call(h.GetMetricsStatus, "GET", "/metrics/status", true).Code)
}

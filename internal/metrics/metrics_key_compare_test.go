package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// The metrics handler compares the admin key through internal/security/apikey in two places: the
// requireAdmin guard and the WebSocket check, which also accepts the key as ?token=. Both must still
// accept the key as before and refuse everything else.
func TestMetricsAdminKey_BothChecksKeepTheirBehaviour(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	h := NewHandler(NewService(nil, logger, prometheus.NewRegistry()), logger)
	h.SetAdminAPIKey("secret")

	status := func(target string, header http.Header) int {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		for k, v := range header {
			req.Header[k] = v
		}
		h.GetMetricsStatus(rr, req)
		return rr.Code
	}
	socket := func(target string, header http.Header) int {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		for k, v := range header {
			req.Header[k] = v
		}
		h.HandleWebSocket(rr, req)
		return rr.Code
	}

	// requireAdmin: bearer and X-API-Key open it; anything else is a 401.
	require.NotEqual(t, http.StatusUnauthorized, status("/metrics/status", http.Header{"Authorization": {"Bearer secret"}}))
	require.NotEqual(t, http.StatusUnauthorized, status("/metrics/status", http.Header{"X-Api-Key": {"secret"}}))
	for name, header := range map[string]http.Header{
		"no credential":   nil,
		"wrong bearer":    {"Authorization": {"Bearer nope"}},
		"wrong X-API-Key": {"X-Api-Key": {"nope"}},
		"empty bearer":    {"Authorization": {"Bearer "}},
		"lowercase":       {"Authorization": {"bearer secret"}},
	} {
		require.Equal(t, http.StatusUnauthorized, status("/metrics/status", header), name)
	}

	// WebSocket: ?token= or a bearer header opens it (a plain request then fails the upgrade, not the
	// check); X-API-Key alone never did, and a wrong or empty token is a 401.
	require.NotEqual(t, http.StatusUnauthorized, socket("/metrics/ws?token=secret", nil), "token")
	require.NotEqual(t, http.StatusUnauthorized, socket("/metrics/ws", http.Header{"Authorization": {"Bearer secret"}}), "bearer")
	require.Equal(t, http.StatusUnauthorized, socket("/metrics/ws", nil), "no credential")
	require.Equal(t, http.StatusUnauthorized, socket("/metrics/ws?token=nope", nil), "wrong token")
	require.Equal(t, http.StatusUnauthorized, socket("/metrics/ws?token=", nil), "empty token")
	require.Equal(t, http.StatusUnauthorized, socket("/metrics/ws", http.Header{"X-Api-Key": {"secret"}}), "X-API-Key alone")
}

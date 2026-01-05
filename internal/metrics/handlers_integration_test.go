package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestSendTestAlert_NotifiesWhenConfigured(t *testing.T) {
	logger := logging.GetDefault()
	reg := prometheus.NewRegistry()
	svc := NewService(nil, logger, reg)
	h := NewHandler(svc, logger)

	notified := false
	var gotType, gotSeverity, gotMessage string
	h.SetNotifier(func(_ context.Context, alertType, severity, message string) {
		notified = true
		gotType, gotSeverity, gotMessage = alertType, severity, message
	})

	r := mux.NewRouter()
	SetupMetricsRoutes(r, h)

	req := httptest.NewRequest("POST", "/metrics/test-alert?type=unit&severity=warning", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.True(t, notified)
	require.Equal(t, "unit", gotType)
	require.Equal(t, "warning", gotSeverity)
	require.NotEmpty(t, gotMessage)
}

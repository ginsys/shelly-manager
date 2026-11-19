package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// ginsys/shelly-manager#707: an id that is not a number answered HTTP 200 with a string error and
// no timestamp. Its siblings answer 400 BAD_REQUEST in the envelope.
func TestGetCurrentDeviceConfigNormalized_InvalidIDIsBadRequest(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := NewHandlerWithLogger(db, testShellyService(t, db), testNotificationHandler(t, db), nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/devices/abc/config/current/normalized", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	w := httptest.NewRecorder()
	handler.GetCurrentDeviceConfigNormalized(w, req)

	requireError(t, w, http.StatusBadRequest, "BAD_REQUEST")
	requireTimestamp(t, w)
}

// requireTimestamp checks that the body carries the timestamp member of the envelope.
func requireTimestamp(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	assert.Contains(t, body, "timestamp", "the answer has no timestamp: %s", w.Body.String())
}

// ginsys/shelly-manager#707: three placeholder routes answered a bare object or array. They send
// the envelope like every other route, with what they sent under data.
func TestPlaceholderRoutesAnswerTheEnvelope(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := NewHandlerWithLogger(db, testShellyService(t, db), testNotificationHandler(t, db), nil, logging.GetDefault())

	tests := []struct {
		name   string
		method string
		path   string
		serve  http.HandlerFunc
		check  func(t *testing.T, data json.RawMessage)
	}{
		{
			name: "provisioning status", method: "GET", path: "/api/v1/provisioning/status",
			serve: handler.GetProvisioningStatus,
			check: func(t *testing.T, data json.RawMessage) {
				var status map[string]any
				require.NoError(t, json.Unmarshal(data, &status), string(data))
				assert.Equal(t, "idle", status["status"])
				assert.Contains(t, status, "last_run", "a null member is still sent")
			},
		},
		{
			name: "provision", method: "POST", path: "/api/v1/provisioning/provision",
			serve: handler.ProvisionDevices,
			check: func(t *testing.T, data json.RawMessage) {
				var result map[string]any
				require.NoError(t, json.Unmarshal(data, &result), string(data))
				assert.Equal(t, "provisioning_started", result["status"])
			},
		},
		{
			name: "dhcp reservations", method: "GET", path: "/api/v1/dhcp/reservations",
			serve: handler.GetDHCPReservations,
			check: func(t *testing.T, data json.RawMessage) {
				assert.JSONEq(t, `[]`, string(data), "an empty list is sent as data, not left out")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.serve(w, httptest.NewRequest(tt.method, tt.path, nil))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			env := decodeEnvelope(t, w)
			require.True(t, env.Success, "success must be true: %s", w.Body.String())
			requireTimestamp(t, w)
			require.NotEmpty(t, env.Data, "the answer has no data: %s", w.Body.String())
			tt.check(t, env.Data)
		})
	}
}

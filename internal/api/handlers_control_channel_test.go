package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A control request whose channel is not a whole number of 0 or more is the caller's mistake: it
// answers 400 with the reason, never a 500 with the message hidden, and no device is called.
func TestControlDevice_BadChannelAnswers400(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	handler := NewHandlerWithLogger(db, svc, testNotificationHandler(t, db), nil, logging.GetDefault())

	device := testutil.TestDevice()
	require.NoError(t, db.AddDevice(device))

	for _, channel := range []interface{}{1.5, -1, "1", 1e300} {
		t.Run(fmt.Sprint(channel), func(t *testing.T) {
			body, err := json.Marshal(map[string]interface{}{
				"action": "on",
				"params": map[string]interface{}{"channel": channel, "force": true},
			})
			require.NoError(t, err)

			req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d/control", device.ID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

			w := httptest.NewRecorder()
			handler.ControlDevice(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "channel")
		})
	}
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A device that does not answer is answered with 504 and a message that names the device and its
// address, never 500 "Internal server error" (ginsys/shelly-manager#755). The fake device accepts
// the connection and never answers, the way an unplugged device behind a live switch behaves. The
// answer must also come before the server's own request deadline, or the middleware answers 408 first.

// silentDevice is a Gen1 device that takes every request and never answers it.
func silentDevice(t *testing.T) (addr string) {
	t.Helper()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); server.Close() })
	return strings.TrimPrefix(server.URL, "http://")
}

func getDeviceRoute(handler *Handler, call func(*Handler, http.ResponseWriter, *http.Request), path string, id uint) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(id))})
	w := httptest.NewRecorder()
	call(handler, w, req)
	return w
}

func errorOf(t *testing.T, w *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Error.Code, body.Error.Message
}

func TestDeviceReads_AnswerA504NamingTheDeviceWhenItDoesNotAnswer(t *testing.T) {
	routes := []struct {
		name string
		path string
		call func(*Handler, http.ResponseWriter, *http.Request)
	}{
		{"status", "/api/v1/devices/%d/status", (*Handler).GetDeviceStatus},
		{"energy", "/api/v1/devices/%d/energy", (*Handler).GetDeviceEnergy},
		{"current config", "/api/v1/devices/%d/config/current", (*Handler).GetCurrentDeviceConfig},
		{"current config, normalized", "/api/v1/devices/%d/config/current/normalized", (*Handler).GetCurrentDeviceConfigNormalized},
	}
	// A read that gets no answer must end before the server's own deadline, or the request
	// middleware answers 408 first and the device is never named.
	deadline := middleware.DefaultSecurityConfig().RequestTimeout

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			addr := silentDevice(t)
			device := deviceAt(t, db, addr, storedMAC)
			path := strings.Replace(route.path, "%d", strconv.Itoa(int(device.ID)), 1)
			start := time.Now()

			w := getDeviceRoute(handler, route.call, path, device.ID)

			assert.Less(t, time.Since(start), deadline, "the answer came after the server's request deadline")
			require.Equal(t, http.StatusGatewayTimeout, w.Code, w.Body.String())
			code, message := errorOf(t, w)
			assert.Equal(t, "DEVICE_UNREACHABLE", code)
			assert.Contains(t, message, device.Name)
			assert.Contains(t, message, addr)
			assert.Contains(t, message, "did not answer")
		})
	}
}

// A device that answers with a refusal of the stored login keeps its own answer: 502, not 504.
func TestDeviceReads_ARejectedLoginIsStillA502(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	addr, _ := gen1Device(t, storedMAC, http.StatusUnauthorized)
	device := deviceAt(t, db, addr, storedMAC)

	w := getDeviceRoute(handler, (*Handler).GetCurrentDeviceConfig, "/api/v1/devices/"+strconv.Itoa(int(device.ID))+"/config/current", device.ID)

	assert.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
}

// An apply that may have written is never told "did not answer": the apply route reaches the same
// error writer, but only a read wrapped by the service as an unanswered one gets the 504.
func TestApplyDeviceNewConfig_NeverAnswersThatTheDeviceDidNotAnswer(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	addr := silentDevice(t)
	device := deviceAt(t, db, addr, storedMAC)
	require.NoError(t, db.GetDB().Model(device).Update("desired_config", `{"system":{"device":{"name":"fleet"}}}`).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req := httptest.NewRequest("POST", "/api/v1/devices/"+strconv.Itoa(int(device.ID))+"/config/new/apply", nil).WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})
	w := httptest.NewRecorder()

	handler.ApplyDeviceNewConfig(w, req)

	assert.NotEqual(t, http.StatusGatewayTimeout, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "did not answer")
}

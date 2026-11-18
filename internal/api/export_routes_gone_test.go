package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The apply is the one path that writes a configuration to a device: it holds the per-device lock,
// checks the MAC and keeps config_applied and pending_reboot true to the device
// (ginsys/shelly-manager#592). The two export routes wrote the stored import/export record to a
// device with none of that and are gone. A route table that brings one back fails here.
func TestConfigRoutes_ExportToDeviceIsGone(t *testing.T) {
	router := configRoutesRouter(t, "admin-secret")

	for _, path := range []string{
		"/api/v1/devices/1/config/export",
		"/api/v1/config/bulk-export",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"device_ids":[1]}`))
			req.Header.Set("Authorization", "Bearer admin-secret")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code, "the route answers a request that holds the admin key")
		})
	}

	// The control: a route that exists answers the same request with something other than 404.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/bulk-import", strings.NewReader(`{"device_ids":[1]}`))
	req.Header.Set("Authorization", "Bearer admin-secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusNotFound, w.Code, "the control route is registered")
}

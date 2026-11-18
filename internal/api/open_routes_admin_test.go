package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The routes that answered without the admin key until ginsys/shelly-manager#611: the device and
// tag writes, the notification routes, and the reads of the device inventory, the tags, the export
// plugins and two provisioning stubs. The walk in route_guards_test.go proves no route is left
// open; this test proves each of these refuses a wrong or missing key and accepts a right one,
// through every header the key may arrive in. The notification reads return stored credentials (a
// channel's config holds webhook URLs, headers and a secret), so they were guarded first.
var adminGuardedOpenRoutes = []struct{ method, path string }{
	// Writes.
	{"POST", "/api/v1/devices"},
	{"PUT", "/api/v1/devices/1"},
	{"DELETE", "/api/v1/devices/1"},
	{"POST", "/api/v1/devices/1/control"},
	{"POST", "/api/v1/devices/1/tags/new"},
	{"DELETE", "/api/v1/devices/1/tags/new/kitchen"},
	{"POST", "/api/v1/notifications/channels"},
	{"PUT", "/api/v1/notifications/channels/1"},
	{"DELETE", "/api/v1/notifications/channels/1"},
	{"POST", "/api/v1/notifications/channels/1/test"},
	{"POST", "/api/v1/notifications/rules"},

	// Reads that return a channel's stored configuration.
	{"GET", "/api/v1/notifications/channels"},
	{"GET", "/api/v1/notifications/rules"},
	{"GET", "/api/v1/notifications/history"},

	// The other reads: the device inventory and readings, tags, export plugins, and two stubs.
	{"GET", "/api/v1/devices"},
	{"GET", "/api/v1/devices/1"},
	{"GET", "/api/v1/devices/1/capabilities"},
	{"GET", "/api/v1/devices/1/energy"},
	{"GET", "/api/v1/devices/1/status"},
	{"GET", "/api/v1/devices/1/tags/new"},
	{"GET", "/api/v1/tags/new"},
	{"GET", "/api/v1/tags/new/kitchen/devices"},
	{"GET", "/api/v1/export/plugins"},
	{"GET", "/api/v1/export/plugins/json"},
	{"GET", "/api/v1/export/plugins/json/schema"},
	{"GET", "/api/v1/provisioning/status"},
	{"GET", "/api/v1/dhcp/reservations"},
}

// The routes of the two groups that stay open by design, whatever the key.
var openByDesignRoutes = []struct{ method, path string }{
	{"GET", "/api/v1/version"},
	{"GET", "/api/v1/metrics/prometheus"},
}

func TestOpenRoutes_RequireTheAdminKey(t *testing.T) {
	router := guardWalkRouter(t, "admin-secret")

	for _, route := range adminGuardedOpenRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, serveGuardRoute(router, route.method, route.path, nil), "no credentials")
			assert.Equal(t, http.StatusUnauthorized,
				serveGuardRoute(router, route.method, route.path, http.Header{"Authorization": {"Bearer wrong"}}), "wrong bearer key")
			assert.Equal(t, http.StatusUnauthorized,
				serveGuardRoute(router, route.method, route.path, http.Header{"X-Api-Key": {"wrong"}}), "wrong X-API-Key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serveGuardRoute(router, route.method, route.path, http.Header{"Authorization": {"Bearer admin-secret"}}), "valid bearer key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serveGuardRoute(router, route.method, route.path, http.Header{"X-Api-Key": {"admin-secret"}}), "valid X-API-Key")
		})
	}
}

func TestOpenRoutes_StayOpenByDesign(t *testing.T) {
	router := guardWalkRouter(t, "admin-secret")

	for _, route := range openByDesignRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			assert.NotEqual(t, http.StatusUnauthorized, serveGuardRoute(router, route.method, route.path, nil))
		})
	}
}

// With no admin key set the routes stay open, as every other admin-guarded route does: this change
// does not turn the default closed.
func TestOpenRoutes_AreOpenWithoutAnAdminKey(t *testing.T) {
	router := guardWalkRouter(t, "")

	for _, route := range adminGuardedOpenRoutes {
		assert.NotEqual(t, http.StatusUnauthorized, serveGuardRoute(router, route.method, route.path, nil), "%s %s", route.method, route.path)
	}
}

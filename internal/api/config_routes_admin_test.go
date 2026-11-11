package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// Each route of the configuration area must refuse a request without the admin key when one is
// set. A route that never calls requireAdmin answers anyone, so the test goes through the router
// and not through the handler: a guard that sits in the route table is what it checks.
var adminGuardedConfigRoutes = []struct{ method, path string }{
	{"GET", "/api/v1/config/templates"},
	{"POST", "/api/v1/config/templates"},
	{"PUT", "/api/v1/config/templates/1"},
	{"DELETE", "/api/v1/config/templates/1"},
	{"GET", "/api/v1/config/templates/new"},
	{"POST", "/api/v1/config/templates/new"},
	{"GET", "/api/v1/config/templates/new/1"},
	{"PUT", "/api/v1/config/templates/new/1"},
	{"DELETE", "/api/v1/config/templates/new/1"},
	{"GET", "/api/v1/config/schema"},
	{"GET", "/api/v1/devices/1/templates/new"},
	{"PUT", "/api/v1/devices/1/templates/new"},
	{"POST", "/api/v1/devices/1/templates/new/1"},
	{"DELETE", "/api/v1/devices/1/templates/new/1"},
}

// configRoutesRouter builds the production route table on a full handler, because the old
// template family reads the shelly service that the small test handler leaves out.
func configRoutesRouter(t *testing.T, adminKey string) *mux.Router {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	handler := NewHandlerWithLogger(db, testShellyService(t, db), testNotificationHandler(t, db), nil, logging.GetDefault())
	if adminKey != "" {
		handler.SetAdminAPIKey(adminKey)
	}
	router := mux.NewRouter()
	registerAPIRoutes(router.PathPrefix("/api/v1").Subrouter(), handler, logging.GetDefault(), nil)
	return router
}

func TestConfigRoutes_RequireTheAdminKey(t *testing.T) {
	router := configRoutesRouter(t, "admin-secret")

	serve := func(method, path string, header http.Header) int {
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		for k, v := range header {
			req.Header[k] = v
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	for _, route := range adminGuardedConfigRoutes {
		name := route.method + " " + route.path
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, serve(route.method, route.path, nil), "no credentials")
			assert.Equal(t, http.StatusUnauthorized,
				serve(route.method, route.path, http.Header{"Authorization": {"Bearer wrong"}}), "wrong bearer key")
			assert.Equal(t, http.StatusUnauthorized,
				serve(route.method, route.path, http.Header{"X-Api-Key": {"wrong"}}), "wrong X-API-Key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serve(route.method, route.path, http.Header{"Authorization": {"Bearer admin-secret"}}), "valid bearer key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serve(route.method, route.path, http.Header{"X-Api-Key": {"admin-secret"}}), "valid X-API-Key")
		})
	}
}

// With no admin key set the routes stay open, as every other admin-guarded route does.
func TestConfigRoutes_AreOpenWithoutAnAdminKey(t *testing.T) {
	router := configRoutesRouter(t, "")

	for _, route := range adminGuardedConfigRoutes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code, "%s %s", route.method, route.path)
	}
}

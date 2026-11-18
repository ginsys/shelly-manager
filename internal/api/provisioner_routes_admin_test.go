package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The provisioner agent routes carry device credentials and Wi-Fi settings (the task poll returns
// each task's configuration), so each one must refuse a request without the admin key when one is
// set. The test goes through the router: a guard in the route table is what it checks.
var adminGuardedProvisionerRoutes = []struct{ method, path string }{
	{"POST", "/api/v1/provisioner/agents/register"},
	{"GET", "/api/v1/provisioner/agents"},
	{"GET", "/api/v1/provisioner/agents/agent-1/tasks"},
	{"POST", "/api/v1/provisioner/tasks"},
	{"GET", "/api/v1/provisioner/tasks"},
	{"PUT", "/api/v1/provisioner/tasks/task-1/status"},
	{"POST", "/api/v1/provisioner/discovered-devices"},
	{"GET", "/api/v1/provisioner/discovered-devices"},
	{"GET", "/api/v1/provisioner/health"},

	// The discovery and provisioning routes next to them: both start work on devices.
	{"POST", "/api/v1/discover"},
	{"POST", "/api/v1/provisioning/provision"},
}

func TestProvisionerRoutes_RequireTheAdminKey(t *testing.T) {
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

	for _, route := range adminGuardedProvisionerRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
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

// With no admin key set the routes stay open, as every other admin-guarded route does: this change
// does not turn the default closed.
func TestProvisionerRoutes_AreOpenWithoutAnAdminKey(t *testing.T) {
	router := configRoutesRouter(t, "")

	for _, route := range adminGuardedProvisionerRoutes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code, "%s %s", route.method, route.path)
	}
}

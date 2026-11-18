package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// The provisioner agent routes carry device credentials and Wi-Fi settings (the task poll returns
// each task's configuration), so each one must refuse a request without the admin key when one is
// set. The test goes through the router: a guard in the route table is what it checks.
type routeCase struct{ method, path string }

// agentProvisionerRoutes are the five routes the provisioner agent calls. They accept the agent key
// as well as the admin key.
var agentProvisionerRoutes = []routeCase{
	{"POST", "/api/v1/provisioner/agents/register"},
	{"GET", "/api/v1/provisioner/agents/agent-1/tasks"},
	{"PUT", "/api/v1/provisioner/tasks/task-1/status"},
	{"POST", "/api/v1/provisioner/discovered-devices"},
	{"GET", "/api/v1/provisioner/health"},
}

// adminOnlyProvisionerRoutes create tasks, list what the agents reported, or start work on devices:
// the agent key opens none of them.
var adminOnlyProvisionerRoutes = []routeCase{
	{"GET", "/api/v1/provisioner/agents"},
	{"POST", "/api/v1/provisioner/tasks"},
	{"GET", "/api/v1/provisioner/tasks"},
	{"GET", "/api/v1/provisioner/discovered-devices"},

	// The discovery and provisioning routes next to them: both start work on devices.
	{"POST", "/api/v1/discover"},
	{"POST", "/api/v1/provisioning/provision"},
}

// adminGuardedProvisionerRoutes is every route here: each refuses a request with no credential.
var adminGuardedProvisionerRoutes = append(append([]routeCase{}, agentProvisionerRoutes...), adminOnlyProvisionerRoutes...)

func serveRoute(router *mux.Router, rc routeCase, header http.Header) int {
	req := httptest.NewRequest(rc.method, rc.path, strings.NewReader("{}"))
	for k, v := range header {
		req.Header[k] = v
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

// The agent routes accept the agent key in both header forms, the admin key too, and refuse the rest.
func TestProvisionerAgentRoutes_AcceptTheAgentKey(t *testing.T) {
	router := configRoutesRouterKeys(t, "admin-secret", "agent-secret")

	for _, rc := range agentProvisionerRoutes {
		t.Run(rc.method+" "+rc.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, serveRoute(router, rc, nil), "no credentials")
			assert.Equal(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"Authorization": {"Bearer wrong"}}), "wrong key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"Authorization": {"Bearer agent-secret"}}), "agent key, bearer")
			assert.NotEqual(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"X-Api-Key": {"agent-secret"}}), "agent key, X-API-Key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"Authorization": {"Bearer admin-secret"}}), "admin key")
		})
	}
}

// The agent key opens nothing that creates a task, lists agent data or starts work on devices.
func TestProvisionerAdminOnlyRoutes_RefuseTheAgentKey(t *testing.T) {
	router := configRoutesRouterKeys(t, "admin-secret", "agent-secret")

	for _, rc := range adminOnlyProvisionerRoutes {
		t.Run(rc.method+" "+rc.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"Authorization": {"Bearer agent-secret"}}), "agent key, bearer")
			assert.Equal(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"X-Api-Key": {"agent-secret"}}), "agent key, X-API-Key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"Authorization": {"Bearer admin-secret"}}), "admin key")
		})
	}
}

// With no agent key set the agent routes accept the admin key alone: an empty agent key is no
// credential, so an empty bearer or an empty X-API-Key header never passes.
func TestProvisionerAgentRoutes_EmptyAgentKeyAcceptsTheAdminKeyOnly(t *testing.T) {
	router := configRoutesRouterKeys(t, "admin-secret", "")

	for _, rc := range agentProvisionerRoutes {
		t.Run(rc.method+" "+rc.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, serveRoute(router, rc, nil), "no credentials")
			assert.Equal(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"Authorization": {"Bearer "}}), "empty bearer")
			assert.Equal(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"X-Api-Key": {""}}), "empty X-API-Key")
			assert.NotEqual(t, http.StatusUnauthorized,
				serveRoute(router, rc, http.Header{"Authorization": {"Bearer admin-secret"}}), "admin key")
		})
	}
}

// The agent key does not change what "no admin key configured" means: the guarded routes stay open.
func TestProvisionerAgentRoutes_AgentKeyAloneStaysOpen(t *testing.T) {
	router := configRoutesRouterKeys(t, "", "agent-secret")

	for _, rc := range agentProvisionerRoutes {
		assert.NotEqual(t, http.StatusUnauthorized, serveRoute(router, rc, nil), "%s %s", rc.method, rc.path)
	}
}

// Rotating the admin key leaves the agents working: the agent holds its own key.
func TestRotateAdminKey_LeavesTheAgentKeyWorking(t *testing.T) {
	router := configRoutesRouterKeys(t, "old-admin", "agent-secret")
	register := routeCase{"POST", "/api/v1/provisioner/agents/register"}

	req := httptest.NewRequest("POST", "/api/v1/admin/rotate-admin-key", strings.NewReader(`{"new_key":"new-admin"}`))
	req.Header.Set("Authorization", "Bearer old-admin")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "the rotation itself")

	assert.NotEqual(t, http.StatusUnauthorized,
		serveRoute(router, register, http.Header{"Authorization": {"Bearer agent-secret"}}), "agent key after rotation")
	assert.NotEqual(t, http.StatusUnauthorized,
		serveRoute(router, register, http.Header{"Authorization": {"Bearer new-admin"}}), "new admin key")
	assert.Equal(t, http.StatusUnauthorized,
		serveRoute(router, register, http.Header{"Authorization": {"Bearer old-admin"}}), "old admin key")
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

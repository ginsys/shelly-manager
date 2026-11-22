package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
)

const (
	authBodyAdminKey = "admin-secret"
	authBodyAgentKey = "agent-secret"
	malformedJSON    = `{bad`
)

// authBodyChain builds the production router, with its whole middleware chain, around a Handler that
// has the given keys. Origins is the CORS allowlist (nil keeps the default).
func authBodyChain(t *testing.T, adminKey, agentKey string, origins []string) *mux.Router {
	t.Helper()
	t.Setenv(testModeEnv, "")
	h, logger := guardWalkHandler(t, adminKey, agentKey)
	secCfg := middleware.DefaultSecurityConfig()
	if origins != nil {
		secCfg.CORSAllowedOrigins = origins
	}
	r := SetupRoutesWithSecurity(h, logger, secCfg, middleware.DefaultValidationConfig())
	return r
}

// authBodyDo sends one JSON request. An empty body sends none. header values are added as given.
func authBodyDo(r http.Handler, method, path, body string, header http.Header) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "TestAgent/1.0")
	for k, values := range header {
		for _, v := range values {
			req.Header.Add(k, v) // Add canonicalizes the name, as a client does
		}
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func bearer(key string) http.Header {
	return http.Header{"Authorization": {"Bearer " + key}}
}

func errorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body.Error.Code
}

// The reported defect: with an admin key set, a request that carries no credential is refused with
// 401 before its JSON body is read, so a malformed body does not draw a 400.
func TestAuthBeforeBody_KeylessMalformedBodyAnswers401(t *testing.T) {
	r := authBodyChain(t, authBodyAdminKey, "", nil)
	rr := authBodyDo(r, http.MethodPost, "/api/v1/devices", malformedJSON, nil)
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
	require.Equal(t, "UNAUTHORIZED", errorCode(t, rr))
}

// Every mutating route of the production table answers 401 to a keyless malformed body, on the full
// middleware chain. The walk is the census, as in TestRouteTable_EveryRouteRequiresTheAdminKey.
func TestAuthBeforeBody_EveryMutatingRouteRefusesKeylessMalformedBody(t *testing.T) {
	r := authBodyChain(t, authBodyAdminKey, "", nil)

	checked := 0
	for _, route := range walkGuardRoutes(t, r) {
		if !isMutating(route.method) {
			continue
		}
		checked++
		rr := authBodyDo(r, route.method, route.path, malformedJSON, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s answers a keyless malformed body with %d, not 401: %s", route.key(), rr.Code, rr.Body.String())
		}
	}
	// Control: the walk reached the mutating routes. An empty walk would pass the loop above.
	require.Greater(t, checked, 40, "the walk visited too few mutating routes to prove anything")
	t.Logf("%d mutating route+method pairs answered a keyless malformed body with 401", checked)
}

// A request that carries a key is not refused by the new step: the right key, and a wrong key, still
// reach the JSON check and draw its 400. The wrong-key case records the limit: only "no credential at
// all" is decided before the body.
func TestAuthBeforeBody_RequestsWithAKeyStillReachTheJSONCheck(t *testing.T) {
	r := authBodyChain(t, authBodyAdminKey, "", nil)
	for name, header := range map[string]http.Header{
		"right key":        bearer(authBodyAdminKey),
		"wrong key":        bearer("not-the-key"),
		"right X-API-Key":  {"X-API-Key": {authBodyAdminKey}},
		"wrong X-API-Key":  {"X-API-Key": {"not-the-key"}},
		"Basic credential": {"Authorization": {"Basic dXNlcjpwYXNz"}},
	} {
		rr := authBodyDo(r, http.MethodPost, "/api/v1/devices", malformedJSON, header)
		if name == "Basic credential" {
			// Not a channel the guards read: it is no credential, so it is refused like a keyless request.
			require.Equal(t, http.StatusUnauthorized, rr.Code, name)
			continue
		}
		require.Equal(t, http.StatusBadRequest, rr.Code, "%s: %s", name, rr.Body.String())
	}
}

// With no admin key set a route is open, and so is the body check: nothing is refused for a missing key.
func TestAuthBeforeBody_NoAdminKeyChangesNothing(t *testing.T) {
	r := authBodyChain(t, "", "", nil)
	rr := authBodyDo(r, http.MethodPost, "/api/v1/devices", malformedJSON, nil)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

	rr = authBodyDo(r, http.MethodPost, "/api/v1/devices", "{}", nil)
	require.NotEqual(t, http.StatusUnauthorized, rr.Code, "an unset key leaves the route open: %s", rr.Body.String())
}

// The agent key opens the agent routes, and the new step does not refuse it before the body check.
func TestAuthBeforeBody_AgentKeyStillReachesTheJSONCheck(t *testing.T) {
	r := authBodyChain(t, authBodyAdminKey, authBodyAgentKey, nil)
	rr := authBodyDo(r, http.MethodPost, "/api/v1/provisioner/agents/register", malformedJSON, bearer(authBodyAgentKey))
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

	rr = authBodyDo(r, http.MethodPost, "/api/v1/provisioner/agents/register", malformedJSON, nil)
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
}

// The open routes are GETs and stay open, even with a body; a keyless request with no body reaches
// the route's own guard, unchanged; a path that matches no route is still 404.
func TestAuthBeforeBody_OpenRoutesGuardsAndUnknownPathsKeepTheirAnswer(t *testing.T) {
	r := authBodyChain(t, authBodyAdminKey, "", nil)

	rr := authBodyDo(r, http.MethodGet, "/api/v1/version", `{"x":1}`, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = authBodyDo(r, http.MethodPost, "/api/v1/devices", "", http.Header{"Content-Type": {"application/json"}})
	require.Equal(t, http.StatusUnauthorized, rr.Code, "the route's own guard answers a keyless request with no body: %s", rr.Body.String())

	rr = authBodyDo(r, http.MethodPost, "/api/v1/no-such-route", malformedJSON, nil)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

// The step answers before the CORS middleware runs, so it sets the CORS headers itself: a browser UI on
// another origin must see the 401 to open its admin-key prompt (the UI keys it on the UNAUTHORIZED code).
func TestAuthBeforeBody_KeylessRefusalCarriesTheCORSHeaders(t *testing.T) {
	r := authBodyChain(t, authBodyAdminKey, "", []string{corsListed})

	// A valid body: on main this is also a 401 with the CORS headers, from the route's own guard.
	for _, body := range []string{`{}`, malformedJSON} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/devices", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.Header.Set("Origin", corsListed)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code, "%s: %s", body, rr.Body.String())
		require.Equal(t, "UNAUTHORIZED", errorCode(t, rr), body)
		require.Equal(t, corsListed, rr.Header().Get("Access-Control-Allow-Origin"), body)
		require.Equal(t, "Origin", rr.Header().Get("Vary"), body)
	}

	// An origin outside the allowlist gets the 401 and no Access-Control-* header, as the middleware gives today.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices", strings.NewReader(malformedJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.Header.Set("Origin", corsUnlisted)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
	require.Empty(t, corsHeaders(rr))
	require.Equal(t, "Origin", rr.Header().Get("Vary"))
}

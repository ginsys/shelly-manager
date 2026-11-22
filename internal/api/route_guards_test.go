package api

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// With an admin key set, every route of the production table must refuse a request that carries
// no credential, except the routes listed below. The walk is the census: it visits every route and
// every method, so a new route with no guard fails the test, and a GET counts like a POST (the
// task poll returns device credentials).
//
// The two lists name the routes that may answer without the key, each with its reason. A stale
// entry fails the test: a route that is listed but answers 401, or that no longer exists, has to
// leave the list. A new open route is added here with a reason, in the same change.

// knownOpenMutating lists mutating routes that answer without the admin key. It is empty: every
// mutating route needs the key when one is set.
// Key: "METHOD /api/v1/path/{template}". Value: why it is open.
var knownOpenMutating = map[string]string{}

// openReads lists non-mutating routes that answer without the admin key, each by design.
var openReads = map[string]string{
	"GET /api/v1/version":            "by design: version info for the web UI and health tooling",
	"GET /api/v1/metrics/prometheus": "by design: the Prometheus scrape endpoint",
}

// guardWalkRequired are routes the walk must have visited: one guarded in the table, one guarded in
// its handler, one provisioner route, one listed route. An empty or partial walk cannot pass.
var guardWalkRequired = []string{
	"GET /api/v1/config/templates",
	"POST /api/v1/admin/rotate-admin-key",
	"POST /api/v1/provisioner/agents/register",
	"GET /api/v1/version", // a listed route
}

type guardWalkRoute struct {
	method, template, path string
	route                  *mux.Route
}

// samplePath fills a route template with values the route accepts: "1" for a plain {param}, and a
// UUID for a {param:regexp} (the one such route is the export result id, a UUID).
func samplePath(template string) string {
	var out strings.Builder
	for i := 0; i < len(template); i++ {
		if template[i] != '{' {
			out.WriteByte(template[i])
			continue
		}
		depth, j := 1, i+1
		for ; j < len(template) && depth > 0; j++ {
			switch template[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if strings.Contains(template[i:j], ":") {
			out.WriteString("00000000-0000-0000-0000-000000000001")
		} else {
			out.WriteString("1")
		}
		i = j - 1
	}
	return out.String()
}

func (r guardWalkRoute) key() string { return r.method + " " + r.template }

// guardWalkHandler builds a Handler with every optional handler set and the admin and agent keys
// given (an empty key stays unset).
func guardWalkHandler(t *testing.T, adminKey, agentKey string) (*Handler, *logging.Logger) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)

	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	msvc := metrics.NewService(db.GetDB(), logger, prometheus.NewRegistry())
	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(&mockSyncPlugin{}))

	// testShellyService never scans a network: the walk runs every route that is not guarded.
	mhandler := metrics.NewHandler(msvc, logger)
	h := NewHandlerWithLogger(db, testShellyService(t, db), testNotificationHandler(t, db), mhandler, logger)
	t.Cleanup(h.WaitBackground) // runs before the database cleanup above
	h.ExportHandlers = NewSyncHandlers(engine, logger)
	h.ImportHandlers = NewImportHandlers(engine, logger)
	// The server gives the same key to the four handlers that check it (cmd/shelly-manager startServer).
	if adminKey != "" {
		h.SetAdminAPIKey(adminKey)
		h.ExportHandlers.SetAdminAPIKey(adminKey)
		h.ImportHandlers.SetAdminAPIKey(adminKey)
		mhandler.SetAdminAPIKey(adminKey)
	}
	// The agent key lives on the Handler alone: export, import and metrics have no agent routes.
	if agentKey != "" {
		h.SetAgentAPIKey(agentKey)
	}
	return h, logger
}

// guardWalkRouter builds the production /api/v1 table on a plain router: no middleware sits in
// front of the guards, so a 401 is the guard's own answer.
func guardWalkRouter(t *testing.T, adminKey, agentKey string) *mux.Router {
	t.Helper()
	h, _ := guardWalkHandler(t, adminKey, agentKey)

	router := mux.NewRouter()
	registerAPIRoutes(router.PathPrefix("/api/v1").Subrouter(), h)
	return router
}

// walkGuardRoutes returns every route and method of the table except OPTIONS, the generic CORS
// preflight responder (it answers no data and no state).
func walkGuardRoutes(t *testing.T, router *mux.Router) []guardWalkRoute {
	t.Helper()
	var routes []guardWalkRoute
	err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		methods, err := route.GetMethods()
		if err != nil {
			// A subrouter entry has no handler and no method of its own. A handler with no method
			// answers every method, so it cannot be walked and must not pass unseen.
			if route.GetHandler() != nil {
				tmpl, _ := route.GetPathTemplate()
				t.Errorf("%s is registered without .Methods(): give it methods so the walk can test it", tmpl)
			}
			return nil
		}
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return err
		}
		for _, m := range methods {
			if m == http.MethodOptions {
				continue
			}
			routes = append(routes, guardWalkRoute{m, tmpl, samplePath(tmpl), route})
		}
		return nil
	})
	require.NoError(t, err)
	sort.Slice(routes, func(i, j int) bool { return routes[i].key() < routes[j].key() })
	return routes
}

// serveGuardRoute sends one request and returns the status, or 0 when the handler panicked: a
// route that runs on an empty request and panics was not refused.
func serveGuardRoute(router *mux.Router, method, path string, header http.Header) (code int) {
	defer func() {
		if recover() != nil {
			code = 0
		}
	}()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	for k, v := range header {
		req.Header[k] = v
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func TestRouteTable_EveryRouteRequiresTheAdminKey(t *testing.T) {
	router := guardWalkRouter(t, "admin-secret", "")
	routes := walkGuardRoutes(t, router)

	seen := map[string]bool{}
	for _, r := range routes {
		seen[r.key()] = true
	}
	for _, key := range guardWalkRequired {
		require.True(t, seen[key], "the walk did not visit %q: an empty or partial walk proves nothing", key)
	}
	for key := range knownOpenMutating {
		method, _, _ := strings.Cut(key, " ")
		require.True(t, isMutating(method), "%q is in knownOpenMutating but is not a mutating route", key)
	}
	for key := range openReads {
		method, _, _ := strings.Cut(key, " ")
		require.False(t, isMutating(method), "%q is in openReads but is a mutating route", key)
	}

	refused, listed := 0, 0
	for _, r := range routes {
		// The sample path must reach its own route: a 404 from a path that matches nothing would
		// read as "refused" and hide an unguarded route.
		var match mux.RouteMatch
		if !router.Match(httptest.NewRequest(r.method, r.path, nil), &match) || match.Route != r.route {
			t.Errorf("%s: the sample path %s does not reach this route", r.key(), r.path)
			continue
		}
		code := serveGuardRoute(router, r.method, r.path, nil)
		list := openReads
		if isMutating(r.method) {
			list = knownOpenMutating
		}
		why, isListed := list[r.key()]
		switch {
		case code == http.StatusUnauthorized && isListed:
			t.Errorf("%s is listed (%s) but answers 401: remove it from the list", r.key(), why)
		case code == http.StatusUnauthorized:
			refused++
		case isListed:
			listed++
		default:
			t.Errorf("%s answers without the admin key (status %d): guard it, or list it with its reason", r.key(), code)
		}
	}
	for _, list := range []map[string]string{knownOpenMutating, openReads} {
		for key := range list {
			if !seen[key] {
				t.Errorf("%s is listed but is no longer a route: remove it from the list", key)
			}
		}
	}
	t.Logf("walked %d route+method pairs: %d refused, %d listed open", len(routes), refused, listed)
}

// agentRouteKeys are the two routes the shared agent key opens: enrolment and the pre-registration
// connectivity check. Poll, task status and report act for one agent and need the token issued at
// registration, so the agent key alone is refused there.
var agentRouteKeys = map[string]bool{
	"POST /api/v1/provisioner/agents/register": true,
	"GET /api/v1/provisioner/health":           true,
}

// The agent key opens the two agent routes above and nothing else: every other route of the table
// answers 401 to a request that carries only the agent key, except the routes listed as open, which
// answer anyone. Export, import and metrics keep their own copy of the admin key and never see the
// agent key.
func TestRouteTable_AgentKeyOpensOnlyTheAgentRoutes(t *testing.T) {
	router := guardWalkRouter(t, "admin-secret", "agent-secret")
	routes := walkGuardRoutes(t, router)
	agentHeader := http.Header{"Authorization": {"Bearer agent-secret"}}

	opened, refused, listed := map[string]bool{}, 0, 0
	for _, r := range routes {
		code := serveGuardRoute(router, r.method, r.path, agentHeader)
		_, openRead := openReads[r.key()]
		_, openMutating := knownOpenMutating[r.key()]
		switch {
		case agentRouteKeys[r.key()]:
			if code == http.StatusUnauthorized {
				t.Errorf("%s refused the agent key", r.key())
			}
			opened[r.key()] = true
		case openRead || openMutating:
			listed++ // answers anyone, so it also answers the agent key
		case code == http.StatusUnauthorized:
			refused++
		default:
			t.Errorf("%s answers the agent key (status %d): the agent key may open only the agent routes", r.key(), code)
		}
	}
	// Controls: every agent route was walked and opened, and the refusals are the rest of the table.
	require.Len(t, opened, len(agentRouteKeys), "an agent route was not walked: %v", opened)
	require.Equal(t, len(routes), len(opened)+refused+listed, "every walked route is classified once")
	require.Greater(t, refused, 0, "no route refused the agent key: the walk proves nothing")
	t.Logf("agent key: %d routes open, %d refused, %d listed open of %d", len(opened), refused, listed, len(routes))
}

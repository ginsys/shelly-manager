package api

import (
	"os"
	"sort"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// testModeRouteExceptions are method+template pairs the production router
// registers and test mode does not. Test mode does not build the security
// monitor, and GET /api/v1/metrics/security closes over that monitor.
// Any other difference is a bug: both constructors must share one registrar.
const testModeOnlyProductionRoute = "GET /api/v1/metrics/security"

func TestTestModeRouterMatchesProductionRouteTable(t *testing.T) {
	// make test / CI export this, which would make SetupRoutesWithSecurity
	// return the test-mode router and hide the difference this test exists for.
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "")
	// Setenv leaves the variable set to empty; the constructor checks == "true".
	if os.Getenv("SHELLY_SECURITY_VALIDATION_TEST_MODE") == "true" {
		t.Fatal("failed to disable test mode for the production router")
	}

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()

	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	msvc := metrics.NewService(db.GetDB(), logger, prometheus.NewRegistry())
	mhandler := metrics.NewHandler(msvc, logger)

	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(&mockSyncPlugin{}))

	h := NewHandlerWithLogger(db, nil, testNotificationHandler(t, db), mhandler, logger)
	h.ExportHandlers = NewSyncHandlers(engine, logger)
	h.ImportHandlers = NewImportHandlers(engine, logger)

	prod := SetupRoutesWithSecurity(h, logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())
	testMode := SetupTestModeRoutes(h, logger)

	prodRoutes := routeTable(t, prod)
	testRoutes := routeTable(t, testMode)

	if _, ok := prodRoutes[testModeOnlyProductionRoute]; !ok {
		t.Fatalf("production router missing %s; documented exception is not registered.\nproduction routes:\n%s",
			testModeOnlyProductionRoute, formatRoutes(prodRoutes))
	}
	if _, ok := testRoutes[testModeOnlyProductionRoute]; ok {
		t.Fatalf("test mode registered %s; that route closes over the production security monitor", testModeOnlyProductionRoute)
	}
	delete(prodRoutes, testModeOnlyProductionRoute)

	missing, extra := diffRoutes(prodRoutes, testRoutes)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("test-mode route table differs from production beyond %s\nmissing from test mode:\n%s\nextra on test mode:\n%s",
			testModeOnlyProductionRoute, formatList(missing), formatList(extra))
	}
}

func routeTable(t *testing.T, r *mux.Router) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	err := r.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tmpl, tmplErr := route.GetPathTemplate()
		methods, methodErr := route.GetMethods()
		if methodErr != nil {
			// Subrouter PathPrefix routes carry no methods. They are not endpoints.
			return nil
		}
		if tmplErr != nil {
			t.Fatalf("route has methods %v but no path template: %v", methods, tmplErr)
		}
		for _, method := range methods {
			key := method + " " + tmpl
			if _, exists := out[key]; exists {
				t.Fatalf("duplicate route registration: %s", key)
			}
			out[key] = struct{}{}
		}
		return nil
	})
	require.NoError(t, err)
	return out
}

func diffRoutes(want, got map[string]struct{}) (missing, extra []string) {
	for route := range want {
		if _, ok := got[route]; !ok {
			missing = append(missing, route)
		}
	}
	for route := range got {
		if _, ok := want[route]; !ok {
			extra = append(extra, route)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

func formatRoutes(routes map[string]struct{}) string {
	list := make([]string, 0, len(routes))
	for route := range routes {
		list = append(list, route)
	}
	return formatList(list)
}

func formatList(routes []string) string {
	if len(routes) == 0 {
		return "(none)"
	}
	sort.Strings(routes)
	out := ""
	for _, route := range routes {
		out += "  " + route + "\n"
	}
	return out
}

package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// TestBulkConfigEndpoints_RequireAdmin verifies that the four bulk-config
// endpoints reject unauthenticated callers when an admin key is configured, and
// crucially that a rejected request contacts no device (#245: these routes push
// stored config to every physical device, so the gate must run before any I/O).
//
// The routes are registered exactly as in router.go:184-188. A plain mux router
// is used rather than SetupRoutesWithSecurity because requireAdmin is enforced
// in the handler, not the middleware, and the production router registers
// Prometheus collectors on the global registry (panics if built twice per
// process, e.g. alongside admin_rotate_test).
func TestBulkConfigEndpoints_RequireAdmin(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()

	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	// A counting server stands in for a physical device: any contact increments
	// hits. On the rejection path there must be zero contact.
	var hits int32
	deviceSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer deviceSrv.Close()
	deviceIP := deviceSrv.URL[len("http://"):]

	// Seed a *reachable* device so an unguarded handler would actually contact
	// hardware — that is what makes the zero-hits assertion meaningful.
	require.NoError(t, db.AddDevice(&database.Device{
		IP:   deviceIP,
		MAC:  "00:11:22:33:44:55",
		Name: "seed",
		Type: "SHSW-1",
	}))

	svc := testShellyService(t, db)
	h := NewHandlerWithLogger(db, svc, nil, nil, logger)

	const adminKey = "bulk-secret"
	h.SetAdminAPIKey(adminKey)

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/config/bulk-import", h.BulkImportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-export", h.BulkExportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-drift-detect", h.BulkDetectConfigDrift).Methods("POST")
	api.HandleFunc("/config/bulk-drift-detect-enhanced", h.EnhancedBulkDetectConfigDrift).Methods("POST")

	routes := []string{
		"/api/v1/config/bulk-import",
		"/api/v1/config/bulk-export",
		"/api/v1/config/bulk-drift-detect",
		"/api/v1/config/bulk-drift-detect-enhanced",
	}

	// 1) Unauthenticated -> 401 for every route.
	for _, path := range routes {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TestAgent/1.0")
		r.ServeHTTP(rr, req)
		require.Equalf(t, http.StatusUnauthorized, rr.Code, "unauthenticated %s: %s", path, rr.Body.String())
	}

	// The security guarantee: no device was contacted while rejecting.
	require.Equal(t, int32(0), atomic.LoadInt32(&hits),
		"no device may be contacted on the rejection path")

	// 2) Authenticated -> the gate opens (not 401). Downstream per-device results
	// may vary; we only assert the request is no longer rejected for auth, which
	// proves the fix is a real gate and not a blanket block.
	for _, path := range routes {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.Header.Set("Authorization", "Bearer "+adminKey)
		r.ServeHTTP(rr, req)
		require.NotEqualf(t, http.StatusUnauthorized, rr.Code, "authenticated %s should pass the gate: %s", path, rr.Body.String())
	}
}

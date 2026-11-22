package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// Reuse mockSyncPlugin from sync_handlers_result_test.go via package scope

func TestRotateAdminKey_AuthAndPropagation(t *testing.T) {
	// This test requires the production router (not test-mode) to verify
	// authentication and key propagation work correctly with full middleware.
	// Temporarily unset test mode so SetupRoutesWithSecurity returns production router.
	origTestMode := os.Getenv("SHELLY_SECURITY_VALIDATION_TEST_MODE")
	_ = os.Unsetenv("SHELLY_SECURITY_VALIDATION_TEST_MODE")
	defer func() {
		if origTestMode != "" {
			_ = os.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", origTestMode)
		}
	}()

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()

	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	// Metrics handler (admin-key protected endpoints)
	msvc := metrics.NewService(db.GetDB(), logger, nil)
	mhandler := metrics.NewHandler(msvc, logger)

	// Sync engine for export/import with mock plugin
	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(&mockSyncPlugin{}))
	exp := NewSyncHandlers(engine, logger)
	imp := NewImportHandlers(engine, logger)

	// API handler
	h := NewHandlerWithLogger(db, nil, nil, mhandler, logger)
	h.ExportHandlers = exp
	h.ImportHandlers = imp
	h.ShareAdminKey()

	// Initial admin key
	initialKey := "old-secret"
	h.SetAdminAPIKey(initialKey)
	exp.SetAdminAPIKey(initialKey)
	imp.SetAdminAPIKey(initialKey)
	mhandler.SetAdminAPIKey(initialKey)

	// Secure router with middleware but disable JSON validation to avoid body consumption in tests
	secCfg := middleware.DefaultSecurityConfig()
	valCfg := middleware.DefaultValidationConfig()
	valCfg.ValidateJSON = false
	r := SetupRoutesWithSecurity(h, logger, secCfg, valCfg)

	// 1) Rotate without auth -> 401
	body, _ := json.Marshal(map[string]string{"new_key": "new-secret"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/rotate-admin-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())

	// 2) Rotate with old key -> 200 {rotated:true}
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/admin/rotate-admin-key", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	req2.Header.Set("Authorization", "Bearer "+initialKey)
	r.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code, rr2.Body.String())
	var wrap map[string]any
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &wrap))
	require.Equal(t, true, wrap["data"].(map[string]any)["rotated"]) // standardized response

	// 3) Metrics admin endpoint should now require new key
	// old key -> 401
	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/api/v1/metrics/health", nil)
	req3.Header.Set("Authorization", "Bearer "+initialKey)
	req3.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr3, req3)
	require.Equal(t, http.StatusUnauthorized, rr3.Code)
	// new key -> 200
	rr4 := httptest.NewRecorder()
	req4 := httptest.NewRequest("GET", "/api/v1/metrics/health", nil)
	req4.Header.Set("Authorization", "Bearer new-secret")
	req4.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr4, req4)
	require.Equal(t, http.StatusOK, rr4.Code, rr4.Body.String())

	// 4) Export endpoint should also reflect new key requirement
	exportReq := map[string]any{
		"plugin_name": "mockfile",
		"format":      "txt",
		"config":      map[string]any{},
		"filters":     map[string]any{},
		"output": map[string]any{
			"type":        "file",
			"destination": "",
		},
		"options": map[string]any{},
	}
	b, _ := json.Marshal(exportReq)

	// old key -> 401
	rr5 := httptest.NewRecorder()
	req5 := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b))
	req5.Header.Set("Content-Type", "application/json")
	req5.Header.Set("Authorization", "Bearer "+initialKey)
	req5.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr5, req5)
	require.Equal(t, http.StatusUnauthorized, rr5.Code)

	// new key -> 200
	rr6 := httptest.NewRecorder()
	req6 := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b))
	req6.Header.Set("Content-Type", "application/json")
	req6.Header.Set("Authorization", "Bearer new-secret")
	req6.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr6, req6)
	require.Equal(t, http.StatusOK, rr6.Code, rr6.Body.String())
}

func TestRotateAdminKey_InvalidBody(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()

	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	h.SetAdminAPIKey("secret")
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/admin/rotate-admin-key", h.RotateAdminKey).Methods("POST")

	// Missing/invalid JSON
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/rotate-admin-key", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)

	// Empty new_key
	rr2 := httptest.NewRecorder()
	b, _ := json.Marshal(map[string]string{"new_key": "  "})
	req2 := httptest.NewRequest("POST", "/api/v1/admin/rotate-admin-key", bytes.NewReader(b))
	req2.Header.Set("Authorization", "Bearer secret")
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusBadRequest, rr2.Code)
}

// rotateRouter serves only the rotation route, guarded by the admin key "secret".
func rotateRouter(t *testing.T) *mux.Router {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)

	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	h.SetAdminAPIKey("secret")
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/admin/rotate-admin-key", h.RotateAdminKey).Methods("POST")
	return r
}

// rotate posts new_key to the rotation route as the holder of the bearer key.
func rotate(t *testing.T, r http.Handler, bearer, newKey string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(map[string]string{"new_key": newKey})
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/rotate-admin-key", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(rr, req)
	return rr
}

// A key with leading or trailing whitespace cannot be presented again (a header
// value loses it), so the endpoint refuses it and keeps the stored key. The
// assertion that carries the test is that the old key still works afterwards.
func TestRotateAdminKey_RefusesEdgeWhitespace(t *testing.T) {
	for name, key := range map[string]string{
		"leading space":      " k3y",
		"trailing space":     "k3y ",
		"leading tab":        "\tk3y",
		"trailing newline":   "k3y\n",
		"trailing nbsp":      "k3y ",
		"both ends":          "  k3y  ",
		"inner and trailing": "a b ",
	} {
		t.Run(name, func(t *testing.T) {
			r := rotateRouter(t)

			rr := rotate(t, r, "secret", key)
			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			require.Contains(t, rr.Body.String(), "whitespace")

			// The old key still authenticates: a key was not stored.
			require.Equal(t, http.StatusOK, rotate(t, r, "secret", "next").Code)
			// The refused key was not stored either.
			require.Equal(t, http.StatusUnauthorized, rotate(t, r, key, "again").Code)
		})
	}
}

// Whitespace inside a key is fine: the key is stored and compared exactly as sent.
func TestRotateAdminKey_AcceptsInnerWhitespace(t *testing.T) {
	r := rotateRouter(t)

	require.Equal(t, http.StatusOK, rotate(t, r, "secret", "a b\tc").Code)
	require.Equal(t, http.StatusUnauthorized, rotate(t, r, "secret", "next").Code, "the old key is replaced")
	require.Equal(t, http.StatusOK, rotate(t, r, "a b\tc", "next").Code, "the key authenticates exactly as sent")
}

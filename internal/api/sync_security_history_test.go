package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// setup router with admin key and access to sync engine
func setupSecuredRouter(t *testing.T, adminKey string) (*mux.Router, *SyncHandlers, *ImportHandlers, func()) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(&mockSyncPlugin{}))

	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	imp := NewImportHandlers(engine, logger)
	exp.SetAdminAPIKey(adminKey)
	imp.SetAdminAPIKey(adminKey)
	h.ExportHandlers = exp
	h.ImportHandlers = imp

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	h.ImportHandlers.AddImportRoutes(api)
	return r, exp, imp, cleanup
}

func TestRBAC_AdminKeyRequired(t *testing.T) {
	router, _, _, cleanup := setupSecuredRouter(t, "secret-key")
	defer cleanup()

	body := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"filters":     map[string]interface{}{},
		"output": map[string]interface{}{
			"type":        "file",
			"destination": "",
		},
		"options": map[string]interface{}{},
	}
	b, _ := json.Marshal(body)

	// Without auth should be 401
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)

	// With correct auth should be 200
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer secret-key")
	router.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code, rr2.Body.String())
}

func TestDownloadPathRestriction(t *testing.T) {
	router, exp, _, cleanup := setupSecuredRouter(t, "")
	defer cleanup()

	base := t.TempDir()
	exp.SetExportBaseDir(base)

	// Export inside base dir
	dst := filepath.Join(base, "in.txt")
	body := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"filters":     map[string]interface{}{},
		"output": map[string]interface{}{
			"type":        "file",
			"destination": dst,
		},
		"options": map[string]interface{}{},
	}
	b, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var wrap map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &wrap)
	expID := wrap["data"].(map[string]interface{})["export_id"].(string)
	// Download should work
	rrD := httptest.NewRecorder()
	router.ServeHTTP(rrD, httptest.NewRequest("GET", "/api/v1/export/"+expID+"/download", nil))
	require.Equal(t, http.StatusOK, rrD.Code)

	// Export outside base dir
	other := filepath.Join(t.TempDir(), "out.txt")
	body2 := body
	body2["output"].(map[string]interface{})["destination"] = other
	b2, _ := json.Marshal(body2)
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b2))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code)
	var wrap2 map[string]interface{}
	_ = json.Unmarshal(rr2.Body.Bytes(), &wrap2)
	expID2 := wrap2["data"].(map[string]interface{})["export_id"].(string)
	// Download should be forbidden
	rrD2 := httptest.NewRecorder()
	router.ServeHTTP(rrD2, httptest.NewRequest("GET", "/api/v1/export/"+expID2+"/download", nil))
	require.Equal(t, http.StatusForbidden, rrD2.Code)
}

func TestHistoryEndpoints(t *testing.T) {
	router, _, _, cleanup := setupSecuredRouter(t, "")
	defer cleanup()

	// Perform an export
	body := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"filters":     map[string]interface{}{},
		"output": map[string]interface{}{
			"type":        "file",
			"destination": "",
		},
		"options": map[string]interface{}{},
	}
	b, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "tester")
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var wrap map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &wrap)
	expID := wrap["data"].(map[string]interface{})["export_id"].(string)

	// Perform an import
	ibody := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"source":      map[string]interface{}{"type": "data"},
		"config":      map[string]interface{}{},
		"options":     map[string]interface{}{},
	}
	ib, _ := json.Marshal(ibody)
	irr := httptest.NewRecorder()
	ireq := httptest.NewRequest("POST", "/api/v1/import", bytes.NewReader(ib))
	ireq.Header.Set("Content-Type", "application/json")
	ireq.Header.Set("X-User-ID", "tester")
	router.ServeHTTP(irr, ireq)
	require.Equal(t, http.StatusOK, irr.Code)

	// List export history
	lrr := httptest.NewRecorder()
	router.ServeHTTP(lrr, httptest.NewRequest("GET", "/api/v1/export/history?page=1&page_size=10", nil))
	require.Equal(t, http.StatusOK, lrr.Code, lrr.Body.String())

	// Get specific export history
	grr := httptest.NewRecorder()
	router.ServeHTTP(grr, httptest.NewRequest("GET", "/api/v1/export/history/"+expID, nil))
	require.Equal(t, http.StatusOK, grr.Code, grr.Body.String())

	// Stats endpoints
	srr := httptest.NewRecorder()
	router.ServeHTTP(srr, httptest.NewRequest("GET", "/api/v1/export/statistics", nil))
	require.Equal(t, http.StatusOK, srr.Code)

	isrr := httptest.NewRecorder()
	router.ServeHTTP(isrr, httptest.NewRequest("GET", "/api/v1/import/statistics", nil))
	require.Equal(t, http.StatusOK, isrr.Code)
}

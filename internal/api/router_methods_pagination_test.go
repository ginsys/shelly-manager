package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func setupRouterForAPI(t *testing.T) (*Handler, *httptest.Server, func()) {
	t.Helper()
	// Isolate Prometheus registrations per test to avoid duplicate collector panics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	db, cleanup := testutil.TestDatabase(t)
	logger := logging.GetDefault()
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)

	// Use security middleware stack with default configs
	// Isolate Prometheus registry before building router to avoid duplicate metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	// Build a minimal router with only export/import routes to avoid Prometheus duplication
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	return h, srv, func() { srv.Close(); cleanup() }
}

func TestDevicesListPaginationMeta(t *testing.T) {
	// Call handler directly to assert pagination meta
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger := logging.GetDefault()
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)

	devs := []*database.Device{
		{IP: "192.0.2.10", MAC: "00:00:00:00:00:01", Type: "t", Name: "d1"},
		{IP: "192.0.2.11", MAC: "00:00:00:00:00:02", Type: "t", Name: "d2"},
		{IP: "192.0.2.12", MAC: "00:00:00:00:00:03", Type: "t", Name: "d3"},
	}
	for _, d := range devs {
		require.NoError(t, h.DB.AddDevice(d))
	}

	req := httptest.NewRequest("GET", "/api/v1/devices?page_size=2&page=2", nil)
	w := httptest.NewRecorder()
	h.GetDevices(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var wrap map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&wrap))
	require.Equal(t, true, wrap["success"])
	data := wrap["data"].(map[string]any)
	devices := data["devices"].([]any)
	require.Equal(t, 1, len(devices))
	meta := wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	require.Equal(t, float64(2), pag["page"])
	require.Equal(t, float64(2), pag["page_size"])
	require.Equal(t, float64(2), pag["total_pages"])
	require.Equal(t, false, pag["has_next"])
	require.Equal(t, true, pag["has_previous"])
	require.Equal(t, float64(1), meta["count"])
	require.Equal(t, float64(3), meta["total_count"])
}

// Content-Type validation is already covered in middleware/validation_test.go

func TestMethodNotAllowed_Returns404(t *testing.T) {
	// Gorilla mux without MethodNotAllowed handler returns 404 for wrong methods
	_, srv, done := setupRouterForAPI(t)
	defer done()

	req, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/devices", nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestExportHistoryPaginationMeta(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})

	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(&mockSyncPlugin{}))

	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	imp := NewImportHandlers(engine, logger)
	h.ExportHandlers = exp
	h.ImportHandlers = imp
	// Set admin key and use it on requests
	admin := "k"
	h.SetAdminAPIKey(admin)
	exp.SetAdminAPIKey(admin)
	imp.SetAdminAPIKey(admin)

	// Minimal router: only export/import routes (avoids Prometheus duplication)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Create two history records directly via engine
	req1 := sync.ExportRequest{PluginName: "mockfile", Format: "txt"}
	res1 := &sync.ExportResult{Success: true, OutputPath: "", RecordCount: 0, ExportID: "exp-1", PluginName: "mockfile", Format: "txt"}
	require.NoError(t, engine.SaveExportHistory(context.Background(), req1, res1, "tester"))
	req2 := sync.ExportRequest{PluginName: "mockfile", Format: "txt"}
	res2 := &sync.ExportResult{Success: true, OutputPath: "", RecordCount: 0, ExportID: "exp-2", PluginName: "mockfile", Format: "txt"}
	require.NoError(t, engine.SaveExportHistory(context.Background(), req2, res2, "tester"))

	// Query history with page_size=1&page=2
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/export/history?page_size=1&page=2", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	require.Equal(t, true, wrap["success"])
	data := wrap["data"].(map[string]any)
	hist := data["history"].([]any)
	require.Equal(t, 1, len(hist))
	meta := wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	require.Equal(t, float64(2), pag["page"])
	require.Equal(t, float64(1), pag["page_size"])
	require.Equal(t, float64(2), pag["total_pages"])
	require.Equal(t, false, pag["has_next"])
	require.Equal(t, true, pag["has_previous"])
}

func TestImportHistoryPaginationMeta(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})

	engine := sync.NewSyncEngine(db, logger)

	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	imp := NewImportHandlers(engine, logger)
	h.ExportHandlers = exp
	h.ImportHandlers = imp

	admin := "k"
	h.SetAdminAPIKey(admin)
	exp.SetAdminAPIKey(admin)
	imp.SetAdminAPIKey(admin)

	// Minimal router with only import routes
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Seed two import history records with unique IDs
	ir1 := sync.ImportRequest{PluginName: "mockfile", Format: "txt"}
	res1 := &sync.ImportResult{Success: true, RecordsImported: 0, RecordsSkipped: 0, ImportID: "imp-1"}
	require.NoError(t, engine.SaveImportHistory(context.Background(), ir1, res1, "tester"))

	ir2 := sync.ImportRequest{PluginName: "mockfile", Format: "txt"}
	res2 := &sync.ImportResult{Success: true, RecordsImported: 0, RecordsSkipped: 0, ImportID: "imp-2"}
	require.NoError(t, engine.SaveImportHistory(context.Background(), ir2, res2, "tester"))

	// Query page 2 of size 1
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/history?page_size=1&page=2", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	require.Equal(t, true, wrap["success"])
	data := wrap["data"].(map[string]any)
	hist := data["history"].([]any)
	require.Equal(t, 1, len(hist))
	meta := wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	require.Equal(t, float64(2), pag["page"])
	require.Equal(t, float64(1), pag["page_size"])
	require.Equal(t, float64(2), pag["total_pages"])
	require.Equal(t, false, pag["has_next"])
	require.Equal(t, true, pag["has_previous"])
	// count/total_count may be omitted for history endpoints; pagination is required
}

func TestExportHistoryFilterPluginAndSuccess(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})

	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	h.ExportHandlers = exp

	admin := "k"
	h.SetAdminAPIKey(admin)
	exp.SetAdminAPIKey(admin)

	// Minimal router
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Seed three records: two for mockfile (true/false), one for other(true)
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: true, RecordCount: 0, ExportID: "exp-a", PluginName: "mockfile", Format: "txt"}, "tester")
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: false, RecordCount: 0, ExportID: "exp-b", PluginName: "mockfile", Format: "txt"}, "tester")
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "other", Format: "txt"}, &sync.ExportResult{Success: true, RecordCount: 0, ExportID: "exp-c", PluginName: "other", Format: "txt"}, "tester")

	// Filter by plugin=mockfile and success=true
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/export/history?plugin=mockfile&success=true&page_size=10&page=1", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	require.Equal(t, true, wrap["success"])
	items := wrap["data"].(map[string]any)["history"].([]any)
	require.Equal(t, 1, len(items))
	item := items[0].(map[string]any)
	require.Equal(t, "mockfile", item["plugin_name"])
	require.Equal(t, true, item["success"])
}

func TestImportHistoryFilterPluginAndSuccess(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})

	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	imp := NewImportHandlers(engine, logger)
	h.ImportHandlers = imp

	admin := "k"
	h.SetAdminAPIKey(admin)
	imp.SetAdminAPIKey(admin)

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Seed three import history records
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ImportResult{Success: true, RecordsImported: 0, RecordsSkipped: 0, ImportID: "imp-a"}, "tester")
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ImportResult{Success: false, RecordsImported: 0, RecordsSkipped: 0, ImportID: "imp-b"}, "tester")
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "other", Format: "txt"}, &sync.ImportResult{Success: true, RecordsImported: 0, RecordsSkipped: 0, ImportID: "imp-c"}, "tester")

	// Filter by plugin=mockfile and success=false
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/history?plugin=mockfile&success=false&page_size=10&page=1", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	require.Equal(t, true, wrap["success"])
	items := wrap["data"].(map[string]any)["history"].([]any)
	require.Equal(t, 1, len(items))
	item := items[0].(map[string]any)
	require.Equal(t, "mockfile", item["plugin_name"])
	require.Equal(t, false, item["success"])
}

func TestExportHistoryPaginationBoundsAndUnknownPlugin(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})

	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	h.ExportHandlers = exp

	admin := "k"
	h.SetAdminAPIKey(admin)
	exp.SetAdminAPIKey(admin)

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Seed 3 history records (any plugin)
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: true, ExportID: "exp-b1", PluginName: "mockfile", Format: "txt"}, "tester")
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: true, ExportID: "exp-b2", PluginName: "mockfile", Format: "txt"}, "tester")
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "other", Format: "txt"}, &sync.ExportResult{Success: true, ExportID: "exp-b3", PluginName: "other", Format: "txt"}, "tester")

	// page=0 (invalid) and page_size=1000 (>100): expect defaults page=1, page_size=20
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/export/history?page=0&page_size=1000", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	meta := wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	require.Equal(t, float64(1), pag["page"])       // corrected to 1
	require.Equal(t, float64(20), pag["page_size"]) // enforced default limit

	// Unknown plugin filter returns empty list
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/export/history?plugin=unknown&page_size=10", nil)
	req2.Header.Set("Authorization", "Bearer "+admin)
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	wrap = map[string]any{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&wrap))
	items := wrap["data"].(map[string]any)["history"].([]any)
	require.Equal(t, 0, len(items))
}

func TestImportHistoryInvalidSuccessAndAuthRequired(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})

	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	imp := NewImportHandlers(engine, logger)
	h.ImportHandlers = imp

	admin := "k"
	h.SetAdminAPIKey(admin)
	imp.SetAdminAPIKey(admin)

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Seed 2 records with mixed success
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ImportResult{Success: true, ImportID: "imp-x1"}, "tester")
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ImportResult{Success: false, ImportID: "imp-x2"}, "tester")

	// Invalid success value resolves to false by implementation; expect only failed records
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/history?plugin=mockfile&success=maybe&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	items := wrap["data"].(map[string]any)["history"].([]any)
	require.Equal(t, 1, len(items))
	first := items[0].(map[string]any)
	require.Equal(t, false, first["success"])

	// Auth required: no Authorization header should yield 401
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/history", nil)
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
}

func TestDevicesPaginationBeyondTotalAndZeroPageSize(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	h := NewHandlerWithLogger(db, nil, nil, nil, logging.GetDefault())

	// Seed 2 devices
	_ = h.DB.AddDevice(&database.Device{IP: "192.0.2.21", MAC: "00:00:00:00:10:01", Name: "d1"})
	_ = h.DB.AddDevice(&database.Device{IP: "192.0.2.22", MAC: "00:00:00:00:10:02", Name: "d2"})

	// page way beyond total
	req := httptest.NewRequest("GET", "/api/v1/devices?page_size=1&page=99", nil)
	w := httptest.NewRecorder()
	h.GetDevices(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var wrap map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&wrap))
	data := wrap["data"].(map[string]any)
	devs := data["devices"].([]any)
	require.Equal(t, 0, len(devs))
	meta := wrap["meta"].(map[string]any)
	require.Equal(t, "v1", meta["version"]) // meta version present

	// page_size omitted or zero => single page with all items
	req2 := httptest.NewRequest("GET", "/api/v1/devices?page_size=0", nil)
	w2 := httptest.NewRecorder()
	h.GetDevices(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	wrap = map[string]any{}
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&wrap))
	data = wrap["data"].(map[string]any)
	devs = data["devices"].([]any)
	require.Equal(t, 2, len(devs))
	meta = wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	require.Equal(t, float64(1), pag["page"])
	require.Equal(t, float64(2), pag["page_size"]) // equals total
	require.Equal(t, false, pag["has_next"])
}

func TestExportHistoryNonIntegerPageDefaults(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	h.ExportHandlers = exp
	admin := "k"
	h.SetAdminAPIKey(admin)
	exp.SetAdminAPIKey(admin)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Seed 1 record
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: true, ExportID: "exp-n1", PluginName: "mockfile", Format: "txt"}, "tester")

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/export/history?page=abc&page_size=xyz", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	meta := wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	// Defaults page=1, page_size=20
	require.Equal(t, float64(1), pag["page"])
	require.Equal(t, float64(20), pag["page_size"])
}

func TestImportHistoryNonIntegerPageDefaults(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	imp := NewImportHandlers(engine, logger)
	h.ImportHandlers = imp
	admin := "k"
	h.SetAdminAPIKey(admin)
	imp.SetAdminAPIKey(admin)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ImportResult{Success: true, ImportID: "imp-n1"}, "tester")

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/history?page=abc&page_size=xyz", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	meta := wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	require.Equal(t, float64(1), pag["page"])
	require.Equal(t, float64(20), pag["page_size"])
}

func TestExportImportStatisticsReflectTotals(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	imp := NewImportHandlers(engine, logger)
	h.ExportHandlers = exp
	h.ImportHandlers = imp
	admin := "k"
	h.SetAdminAPIKey(admin)
	exp.SetAdminAPIKey(admin)
	imp.SetAdminAPIKey(admin)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Seed export history: 2 success, 1 failure for plugin mockfile
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: true, ExportID: "ex-s1", PluginName: "mockfile", Format: "txt"}, "tester")
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: true, ExportID: "ex-s2", PluginName: "mockfile", Format: "txt"}, "tester")
	_ = engine.SaveExportHistory(context.Background(), sync.ExportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ExportResult{Success: false, ExportID: "ex-f1", PluginName: "mockfile", Format: "txt"}, "tester")

	// Seed import history: 1 success, 2 failure for plugin other
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "other", Format: "txt"}, &sync.ImportResult{Success: true, ImportID: "im-s1"}, "tester")
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "other", Format: "txt"}, &sync.ImportResult{Success: false, ImportID: "im-f1"}, "tester")
	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "other", Format: "txt"}, &sync.ImportResult{Success: false, ImportID: "im-f2"}, "tester")

	// Export statistics
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/export/statistics", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var s map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&s))
	expData := s["data"].(map[string]any)
	require.Equal(t, float64(3), expData["total"])
	require.Equal(t, float64(2), expData["success"])
	require.Equal(t, float64(1), expData["failure"])
	// by_plugin counts
	byPlugin := expData["by_plugin"].(map[string]any)
	require.Equal(t, float64(3), byPlugin["mockfile"]) // two success + one failure

	// Import statistics
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/statistics", nil)
	req2.Header.Set("Authorization", "Bearer "+admin)
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	s = map[string]any{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&s))
	impData := s["data"].(map[string]any)
	require.Equal(t, float64(3), impData["total"])
	require.Equal(t, float64(1), impData["success"])
	require.Equal(t, float64(2), impData["failure"])
	ibp := impData["by_plugin"].(map[string]any)
	require.Equal(t, float64(3), ibp["other"]) // one success + two failures
}

func TestDevicesNonIntegerPageDefaults(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	h := NewHandlerWithLogger(db, nil, nil, nil, logging.GetDefault())
	_ = h.DB.AddDevice(&database.Device{IP: "192.0.2.31", MAC: "00:00:00:00:20:01", Name: "d1"})
	_ = h.DB.AddDevice(&database.Device{IP: "192.0.2.32", MAC: "00:00:00:00:20:02", Name: "d2"})
	_ = h.DB.AddDevice(&database.Device{IP: "192.0.2.33", MAC: "00:00:00:00:20:03", Name: "d3"})

	req := httptest.NewRequest("GET", "/api/v1/devices?page=abc&page_size=xyz", nil)
	w := httptest.NewRecorder()
	h.GetDevices(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var wrap map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&wrap))
	meta := wrap["meta"].(map[string]any)
	pag := meta["pagination"].(map[string]any)
	require.Equal(t, float64(1), pag["page"])      // default
	require.Equal(t, float64(3), pag["page_size"]) // total
}

func TestImportHistoryUnknownPluginAndCaseSensitivity(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	imp := NewImportHandlers(engine, logger)
	h.ImportHandlers = imp
	admin := "k"
	h.SetAdminAPIKey(admin)
	imp.SetAdminAPIKey(admin)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ImportHandlers.AddImportRoutes(api)
	srv := httptest.NewServer(r)
	defer srv.Close()

	_ = engine.SaveImportHistory(context.Background(), sync.ImportRequest{PluginName: "mockfile", Format: "txt"}, &sync.ImportResult{Success: true, ImportID: "imp-cs1"}, "tester")

	// Unknown plugin
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/history?plugin=unknown", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var wrap map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wrap))
	items := wrap["data"].(map[string]any)["history"].([]any)
	require.Equal(t, 0, len(items))

	// Case-sensitive filter
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/import/history?plugin=MOCKFILE", nil)
	req2.Header.Set("Authorization", "Bearer "+admin)
	req2.Header.Set("User-Agent", "TestAgent/1.0")
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	wrap = map[string]any{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&wrap))
	items = wrap["data"].(map[string]any)["history"].([]any)
	require.Equal(t, 0, len(items))
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// local mock plugin (duplicated to keep tests self-contained)
type mockSchedPlugin struct{}

func (m *mockSchedPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{Name: "mockfile", Version: "1.0.0", SupportedFormats: []string{"txt"}}
}
func (m *mockSchedPlugin) ConfigSchema() sync.ConfigSchema {
	return sync.ConfigSchema{Version: "1.0.0"}
}
func (m *mockSchedPlugin) ValidateConfig(map[string]interface{}) error { return nil }
func (m *mockSchedPlugin) Capabilities() sync.PluginCapabilities       { return sync.PluginCapabilities{} }
func (m *mockSchedPlugin) Initialize(*logging.Logger) error            { return nil }
func (m *mockSchedPlugin) Cleanup() error                              { return nil }
func (m *mockSchedPlugin) Preview(ctx context.Context, data *sync.ExportData, cfg sync.ExportConfig) (*sync.PreviewResult, error) {
	return &sync.PreviewResult{Success: true, RecordCount: 0}, nil
}
func (m *mockSchedPlugin) Export(_ context.Context, _ *sync.ExportData, cfg sync.ExportConfig) (*sync.ExportResult, error) {
	if cfg.Output.Destination != "" {
		_ = os.MkdirAll(filepath.Dir(cfg.Output.Destination), 0o755)
		_ = os.WriteFile(cfg.Output.Destination, []byte("ok"), 0o644)
	}
	return &sync.ExportResult{Success: true, OutputPath: cfg.Output.Destination, RecordCount: 0}, nil
}
func (m *mockSchedPlugin) Import(_ context.Context, _ sync.ImportSource, _ sync.ImportConfig) (*sync.ImportResult, error) {
	return &sync.ImportResult{Success: true}, nil
}

func setupScheduleRouter(t *testing.T) (*mux.Router, *sync.SyncEngine, func()) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(&mockSchedPlugin{}))
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	h.ExportHandlers = NewSyncHandlers(engine, logger)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	return r, engine, cleanup
}

func TestExportSchedulesCRUDAndRun(t *testing.T) {
	router, _, cleanup := setupScheduleRouter(t)
	defer cleanup()

	// Create
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "sched.txt")
	create := map[string]interface{}{
		"name":         "daily",
		"interval_sec": 5,
		"enabled":      true,
		"request": map[string]interface{}{
			"plugin_name": "mockfile",
			"format":      "txt",
			"output": map[string]interface{}{
				"type":        "file",
				"destination": outPath,
			},
		},
	}
	b, _ := json.Marshal(create)
	req := httptest.NewRequest("POST", "/api/v1/export/schedules", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	sch := wrap["data"].(map[string]interface{})
	id := sch["id"].(string)

	// List
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, httptest.NewRequest("GET", "/api/v1/export/schedules", nil))
	require.Equal(t, http.StatusOK, rr2.Code)

	// Get
	rr3 := httptest.NewRecorder()
	router.ServeHTTP(rr3, httptest.NewRequest("GET", "/api/v1/export/schedules/"+id, nil))
	require.Equal(t, http.StatusOK, rr3.Code)

	// Update
	upd := map[string]interface{}{"enabled": false}
	bu, _ := json.Marshal(upd)
	rru := httptest.NewRecorder()
	reqU := httptest.NewRequest("PUT", "/api/v1/export/schedules/"+id, bytes.NewReader(bu))
	reqU.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rru, reqU)
	require.Equal(t, http.StatusOK, rru.Code)

	// Run now
	rrRun := httptest.NewRecorder()
	router.ServeHTTP(rrRun, httptest.NewRequest("POST", "/api/v1/export/schedules/"+id+"/run", nil))
	require.Equal(t, http.StatusOK, rrRun.Code, rrRun.Body.String())
	// Allow filesystem flush
	time.Sleep(10 * time.Millisecond)
	_, err := os.Stat(outPath)
	require.NoError(t, err)

	// Delete
	rrDel := httptest.NewRecorder()
	router.ServeHTTP(rrDel, httptest.NewRequest("DELETE", "/api/v1/export/schedules/"+id, nil))
	require.Equal(t, http.StatusOK, rrDel.Code)
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// mockSyncPlugin implements a minimal sync plugin for testing
type mockSyncPlugin struct{}

func (m *mockSyncPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{Name: "mockfile", Version: "1.0.0", SupportedFormats: []string{"txt"}}
}
func (m *mockSyncPlugin) ConfigSchema() sync.ConfigSchema             { return sync.ConfigSchema{Version: "1.0.0"} }
func (m *mockSyncPlugin) ValidateConfig(map[string]interface{}) error { return nil }
func (m *mockSyncPlugin) Capabilities() sync.PluginCapabilities       { return sync.PluginCapabilities{} }
func (m *mockSyncPlugin) Initialize(*logging.Logger) error            { return nil }
func (m *mockSyncPlugin) Cleanup() error                              { return nil }
func (m *mockSyncPlugin) Preview(ctx context.Context, data *sync.ExportData, cfg sync.ExportConfig) (*sync.PreviewResult, error) {
	return &sync.PreviewResult{Success: true, RecordCount: 0}, nil
}

// Export writes a small file to the destination (if provided) and returns OutputPath
func (m *mockSyncPlugin) Export(_ context.Context, _ *sync.ExportData, cfg sync.ExportConfig) (*sync.ExportResult, error) {
	path := cfg.Output.Destination
	if path != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte("hello world"), 0o644)
	}
	return &sync.ExportResult{Success: true, OutputPath: path, RecordCount: 0}, nil
}
func (m *mockSyncPlugin) Import(_ context.Context, _ sync.ImportSource, _ sync.ImportConfig) (*sync.ImportResult, error) {
	return &sync.ImportResult{Success: true, RecordsImported: 0, RecordsSkipped: 0}, nil
}

func setupSyncTestRouter(t *testing.T) (*mux.Router, *sync.SyncEngine, *logging.Logger, func()) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(&mockSyncPlugin{}))

	// Build a minimal router without heavy middleware to focus on handler behavior
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	h.ExportHandlers = NewSyncHandlers(engine, logger)
	h.ImportHandlers = NewImportHandlers(engine, logger)

	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	h.ImportHandlers.AddImportRoutes(api)
	return r, engine, logger, cleanup
}

func TestExportResultAndDownload(t *testing.T) {
	router, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "export.txt")

	body := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"filters":     map[string]interface{}{},
		"output": map[string]interface{}{
			"type":        "file",
			"destination": outPath,
		},
		"options": map[string]interface{}{},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// Parse response
	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	data := wrap["data"].(map[string]interface{})
	exportID := data["export_id"].(string)

	// Retrieve result
	req2 := httptest.NewRequest("GET", "/api/v1/export/"+exportID, nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code, rr2.Body.String())

	// Download
	req3 := httptest.NewRequest("GET", "/api/v1/export/"+exportID+"/download", nil)
	req3.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr3 := httptest.NewRecorder()
	router.ServeHTTP(rr3, req3)
	require.Equal(t, http.StatusOK, rr3.Code)
	require.NotEmpty(t, rr3.Body.Bytes())
}

func TestImportResultRetrieval(t *testing.T) {
	router, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	body := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"source":      map[string]interface{}{"type": "data"},
		"config":      map[string]interface{}{},
		"options":     map[string]interface{}{},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/import", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// Parse response to get import_id
	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	data := wrap["data"].(map[string]interface{})
	importID := data["import_id"].(string)

	// Retrieve result via /import/{id}
	req2 := httptest.NewRequest("GET", "/api/v1/import/"+importID, nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	require.Equal(t, http.StatusOK, rr2.Code, rr2.Body.String())
}

// retain io import usage to avoid unused import in other build tags
var _ io.Reader

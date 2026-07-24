package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	smaplugin "github.com/ginsys/shelly-manager/internal/plugins/sync/sma"
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

// notImplementedImportPlugin fails closed on import the way a plugin with no
// persistence layer does (e.g. SMA until #284 lands): it wraps
// sync.ErrImportNotImplemented so the handler can map it to a 501.
type notImplementedImportPlugin struct{ mockSyncPlugin }

func (notImplementedImportPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{Name: "notimpl", Version: "1.0.0", SupportedFormats: []string{"txt"}}
}

type failingExportPlugin struct{ mockSyncPlugin }

func (failingExportPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{Name: "failing", Version: "1.0.0", SupportedFormats: []string{"txt"}}
}

func (failingExportPlugin) Export(context.Context, *sync.ExportData, sync.ExportConfig) (*sync.ExportResult, error) {
	return nil, fmt.Errorf("deliberate export failure")
}

type provenancePlugin struct {
	mockSyncPlugin
	metadata sync.ExportMetadata
}

func (p *provenancePlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{Name: "provenance", Version: "1.0.0", SupportedFormats: []string{"txt"}}
}

func (p *provenancePlugin) Export(_ context.Context, data *sync.ExportData, _ sync.ExportConfig) (*sync.ExportResult, error) {
	p.metadata = data.Metadata
	return &sync.ExportResult{Success: true}, nil
}
func (notImplementedImportPlugin) Import(_ context.Context, _ sync.ImportSource, _ sync.ImportConfig) (*sync.ImportResult, error) {
	return &sync.ImportResult{Success: false}, fmt.Errorf("notimpl plugin: %w", sync.ErrImportNotImplemented)
}

func setupSyncTestRouter(t *testing.T) (*mux.Router, *sync.SyncEngine, *logging.Logger, *database.Manager, func()) {
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
	return r, engine, logger, db, cleanup
}

func TestExportResultAndDownload(t *testing.T) {
	router, _, _, _, cleanup := setupSyncTestRouter(t)
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
	router, _, _, _, cleanup := setupSyncTestRouter(t)
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

// TestGenericImportNotImplementedReturns501 verifies that a plugin failing
// closed with sync.ErrImportNotImplemented is surfaced to API clients as a
// 501 NOT_IMPLEMENTED, not a generic 500 that hides the reason (#272).
func TestGenericImportNotImplementedReturns501(t *testing.T) {
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	require.NoError(t, engine.RegisterPlugin(&notImplementedImportPlugin{}))

	body := map[string]interface{}{
		"plugin_name": "notimpl",
		"format":      "txt",
		"source":      map[string]interface{}{"type": "data"},
		"config":      map[string]interface{}{},
		"options":     map[string]interface{}{"dry_run": false},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/import", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotImplemented, rr.Code, rr.Body.String())

	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	require.Equal(t, false, wrap["success"])
	errObj, ok := wrap["error"].(map[string]interface{})
	require.True(t, ok, "expected error object, got %s", rr.Body.String())
	require.Equal(t, "NOT_IMPLEMENTED", errObj["code"])
}

func TestExportPreviewSummary(t *testing.T) {
	router, _, _, _, cleanup := setupSyncTestRouter(t)
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
	req := httptest.NewRequest("POST", "/api/v1/export/preview", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	data := wrap["data"].(map[string]interface{})
	if _, ok := data["preview"].(map[string]interface{}); !ok {
		t.Fatalf("expected preview in response: %v", data)
	}
	if _, ok := data["summary"].(map[string]interface{}); !ok {
		t.Fatalf("expected summary in response: %v", data)
	}
}

func TestImportPreviewSummary(t *testing.T) {
	router, _, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	body := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"source":      map[string]interface{}{"type": "data"},
		"config":      map[string]interface{}{},
		"options":     map[string]interface{}{},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/import/preview", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	data := wrap["data"].(map[string]interface{})
	if _, ok := data["preview"].(map[string]interface{}); !ok {
		t.Fatalf("expected preview in response: %v", data)
	}
	if _, ok := data["summary"].(map[string]interface{}); !ok {
		t.Fatalf("expected summary in response: %v", data)
	}
}

func TestSMAGzipBase64ImportPreviewUsesRealPlugin(t *testing.T) {
	router, engine, _, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	require.NoError(t, engine.RegisterPlugin(smaplugin.NewPlugin()))

	canonical, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sma", "archive-2026.1.canonical.json"))
	require.NoError(t, err)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err = writer.Write(canonical)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	body := map[string]interface{}{
		"plugin_name": "sma",
		"format":      "sma",
		"source": map[string]interface{}{
			"type": "data",
			"data": base64.StdEncoding.EncodeToString(compressed.Bytes()),
		},
		"config": map[string]interface{}{},
		"options": map[string]interface{}{
			"dry_run":       false,
			"validate_only": false,
		},
	}
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/preview", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var response struct {
		Data struct {
			ChangesCount int `json:"changes_count"`
			Summary      struct {
				WillCreate int `json:"will_create"`
			} `json:"summary"`
			Preview sync.ImportResult `json:"preview"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	require.Equal(t, 1, response.Data.ChangesCount)
	require.Equal(t, 1, response.Data.Summary.WillCreate)
	require.Equal(t, "sma", response.Data.Preview.PluginName)

	history, total, err := engine.ListImportHistory(context.Background(), 1, 20, "", nil)
	require.NoError(t, err)
	require.Empty(t, history)
	require.Zero(t, total)
	for _, model := range []interface{}{
		&database.Device{},
		&database.DiscoveredDevice{},
		&database.ConfigTemplate{},
	} {
		var count int64
		require.NoError(t, db.GetDB().Model(model).Count(&count).Error)
		require.Zero(t, count)
	}
}

func TestInvalidExportPathReturnsValidationResponse(t *testing.T) {
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	engine.SetExportBaseDir(t.TempDir())

	body := map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"output": map[string]interface{}{
			"type":        "file",
			"destination": filepath.Join(t.TempDir(), "outside.txt"),
		},
	}
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), "invalid export path")
}

func TestFailedExportHistoryRetainsPluginAndFormat(t *testing.T) {
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	require.NoError(t, engine.RegisterPlugin(&failingExportPlugin{}))

	body := map[string]interface{}{
		"plugin_name": "failing",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"output":      map[string]interface{}{"type": "response"},
	}
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())

	historyReq := httptest.NewRequest(http.MethodGet, "/api/v1/export/history?plugin=failing", nil)
	historyResponse := httptest.NewRecorder()
	router.ServeHTTP(historyResponse, historyReq)
	require.Equal(t, http.StatusOK, historyResponse.Code, historyResponse.Body.String())
	var response struct {
		Data struct {
			History []database.ExportHistory `json:"history"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(historyResponse.Body.Bytes(), &response))
	require.Len(t, response.Data.History, 1)
	require.Equal(t, "failing", response.Data.History[0].PluginName)
	require.Equal(t, "txt", response.Data.History[0].Format)
	require.False(t, response.Data.History[0].Success)
}

func TestAPIExportUsesNonAuthenticationArchiveProvenance(t *testing.T) {
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	plugin := &provenancePlugin{}
	require.NoError(t, engine.RegisterPlugin(plugin))

	body := map[string]interface{}{
		"plugin_name": "provenance",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"output":      map[string]interface{}{"type": "response"},
	}
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer top-secret")
	req.Header.Set("X-User-ID", "operator")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "api", plugin.metadata.RequestedBy)
	require.Equal(t, "api", plugin.metadata.ExportType)
	require.NotContains(t, plugin.metadata.RequestedBy, "secret")
	require.NotContains(t, plugin.metadata.RequestedBy, "operator")
}

func TestRemovedExportScheduleRoutesReturnPlain404(t *testing.T) {
	router, _, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	for _, path := range []string{
		"/api/v1/export/schedules",
		"/api/v1/export/schedules/01234567-89ab-cdef-0123-456789abcdef",
		"/api/v1/export/schedules/01234567-89ab-cdef-0123-456789abcdef/run",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			require.Equal(t, http.StatusNotFound, rr.Code)
			require.Equal(t, "404 page not found\n", rr.Body.String())
		})
	}
}

func TestGenericExportResultRoutesRequireLowercaseUUID(t *testing.T) {
	router, _, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	for _, path := range []string{
		"/api/v1/export/not-a-uuid",
		"/api/v1/export/01234567-89AB-CDEF-0123-456789ABCDEF",
		"/api/v1/export/01234567-89AB-CDEF-0123-456789ABCDEF/download",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			require.Equal(t, http.StatusNotFound, rr.Code)
			require.Equal(t, "404 page not found\n", rr.Body.String())
		})
	}
}

func TestRequesterFromUsesOnlyExplicitUserHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header map[string]string
		want   string
	}{
		{name: "user id wins", header: map[string]string{"X-User-ID": "  user-id  ", "X-User": "user"}, want: "user-id"},
		{name: "user fallback", header: map[string]string{"X-User-ID": " ", "X-User": "  user  "}, want: "user"},
		{name: "authentication material ignored", header: map[string]string{"Authorization": "Bearer secret", "X-API-Key": "secret"}, want: "api"},
		{name: "remote address ignored", want: "api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/export", nil)
			req.RemoteAddr = "192.0.2.10:1234"
			for key, value := range tt.header {
				req.Header.Set(key, value)
			}
			require.Equal(t, tt.want, requesterFrom(req))
		})
	}
}

// retain io import usage to avoid unused import in other build tags
var _ io.Reader

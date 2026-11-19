package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// restartedRouter is a second engine and router on the same database: the in-memory result store
// starts empty, as it does after the server restarts, and only the history tables remain.
func restartedRouter(t *testing.T, db *database.Manager, logger *logging.Logger) *mux.Router {
	t.Helper()
	engine := sync.NewSyncEngine(db, logger)
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	h.ExportHandlers = NewSyncHandlers(engine, logger)
	h.ImportHandlers = NewImportHandlers(engine, logger)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	h.ExportHandlers.AddExportRoutes(api)
	h.ImportHandlers.AddImportRoutes(api)
	return r
}

// getJSON calls a route and returns the status and the decoded response body.
func getJSON(t *testing.T, router http.Handler, path string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap), rr.Body.String())
	return rr.Code, wrap
}

func resultData(t *testing.T, wrap map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := wrap["data"].(map[string]interface{})
	require.True(t, ok, "expected a data object, got %v", wrap)
	return data
}

func postJSON(t *testing.T, router http.Handler, path string, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var wrap map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	return resultData(t, wrap)
}

const (
	missingExportID    = "00000000-0000-4000-8000-000000000647"
	directExportID     = "11111111-1111-4111-8111-111111111647"
	directImportID     = "import-direct-647"
	missingImportID    = "import-missing-647"
	recordCreatedAtRaw = "2026-11-01T09:30:00Z"
)

// After a restart an export that the history lists answers its detail routes from the history
// record. The answer carries what the record holds and leaves out what it lacks.
func TestExportDetailAfterRestart_AnswersFromHistory(t *testing.T) {
	router, _, logger, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	out := filepath.Join(t.TempDir(), "export.txt")
	created := postJSON(t, router, "/api/v1/export", map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"config":      map[string]interface{}{},
		"filters":     map[string]interface{}{},
		"output":      map[string]interface{}{"type": "file", "destination": out},
		"options":     map[string]interface{}{},
	})
	id := created["export_id"].(string)

	var rec database.ExportHistory
	require.NoError(t, db.GetDB().Where("export_id = ?", id).First(&rec).Error, "the export writes a history row")

	after := restartedRouter(t, db, logger)
	for _, path := range []string{"/api/v1/export/" + id, "/api/v1/export/backup/" + id} {
		code, wrap := getJSON(t, after, path)
		require.Equal(t, http.StatusOK, code, "%s: %v", path, wrap)
		data := resultData(t, wrap)

		require.Equal(t, id, data["export_id"])
		require.Equal(t, rec.PluginName, data["plugin_name"])
		require.Equal(t, rec.Format, data["format"])
		require.Equal(t, rec.Success, data["success"])
		require.Equal(t, out, data["output_path"])
		require.EqualValues(t, rec.RecordCount, data["record_count"])
		require.EqualValues(t, rec.FileSize, data["file_size"])
		// The record keeps whole milliseconds, and a quick export can round to zero of them.
		require.EqualValues(t, time.Duration(rec.DurationMs)*time.Millisecond, time.Duration(data["duration"].(float64)))
		require.Equal(t, "", data["checksum"], "a checksum the record never held is not invented")
		require.NotContains(t, data, "warnings")
		require.NotContains(t, data, "errors", "a successful export has no error to show")
	}
}

// A failed export and a row with every field set: the detail route shows the stored error and
// the stored numbers, as the history list shows them.
func TestExportDetailAfterRestart_ShowsAStoredRow(t *testing.T) {
	_, _, logger, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	createdAt, err := time.Parse(time.RFC3339, recordCreatedAtRaw)
	require.NoError(t, err)
	require.NoError(t, db.GetDB().Create(&database.ExportHistory{
		ExportID: directExportID, PluginName: "mockfile", Format: "txt", Success: false,
		RecordCount: 3, FileSize: 11, FilePath: "/exports/old.txt", DurationMs: 1500,
		ErrorMessage: "disk full", CreatedAt: createdAt,
	}).Error)

	code, wrap := getJSON(t, restartedRouter(t, db, logger), "/api/v1/export/"+directExportID)
	require.Equal(t, http.StatusOK, code, "%v", wrap)
	data := resultData(t, wrap)

	require.Equal(t, false, data["success"])
	require.Equal(t, "/exports/old.txt", data["output_path"])
	require.EqualValues(t, 3, data["record_count"])
	require.EqualValues(t, 11, data["file_size"])
	require.EqualValues(t, int64(1500*time.Millisecond), data["duration"])
	require.Equal(t, []interface{}{"disk full"}, data["errors"])
	require.Equal(t, recordCreatedAtRaw, data["created_at"])
	require.Equal(t, "", data["checksum"])
}

// An id that no record holds is still a 404, on every route that serves a result.
func TestResultDetail_NoRecordIsStillNotFound(t *testing.T) {
	_, _, logger, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	after := restartedRouter(t, db, logger)

	for _, path := range []string{
		"/api/v1/export/" + missingExportID,
		"/api/v1/export/backup/" + missingExportID,
		"/api/v1/import/" + missingImportID,
	} {
		code, wrap := getJSON(t, after, path)
		require.Equal(t, http.StatusNotFound, code, "%s: %v", path, wrap)
		errObj, ok := wrap["error"].(map[string]interface{})
		require.True(t, ok, "%s: expected an error object, got %v", path, wrap)
		require.Equal(t, "NOT_FOUND", errObj["code"], path)
	}
}

// A history table that cannot be read is a server error. It is not a not-found answer, which
// would tell a user that a result they can see in the list does not exist.
func TestResultDetail_UnreadableHistoryIsAServerError(t *testing.T) {
	_, _, logger, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	require.NoError(t, db.GetDB().Migrator().DropTable(&database.ExportHistory{}, &database.ImportHistory{}))
	after := restartedRouter(t, db, logger)

	for _, path := range []string{
		"/api/v1/export/" + missingExportID,
		"/api/v1/import/" + missingImportID,
	} {
		code, wrap := getJSON(t, after, path)
		require.Equal(t, http.StatusInternalServerError, code, "%s: %v", path, wrap)
	}
}

// The same for an import: the detail route answers from the history record after a restart.
func TestImportDetailAfterRestart_AnswersFromHistory(t *testing.T) {
	router, _, logger, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	created := postJSON(t, router, "/api/v1/import", map[string]interface{}{
		"plugin_name": "mockfile",
		"format":      "txt",
		"source":      map[string]interface{}{"type": "data"},
		"config":      map[string]interface{}{},
		"options":     map[string]interface{}{},
	})
	id := created["import_id"].(string)

	var rec database.ImportHistory
	require.NoError(t, db.GetDB().Where("import_id = ?", id).First(&rec).Error, "the import writes a history row")

	code, wrap := getJSON(t, restartedRouter(t, db, logger), "/api/v1/import/"+id)
	require.Equal(t, http.StatusOK, code, "%v", wrap)
	data := resultData(t, wrap)

	require.Equal(t, id, data["import_id"])
	require.Equal(t, rec.PluginName, data["plugin_name"])
	require.Equal(t, rec.Format, data["format"])
	require.Equal(t, rec.Success, data["success"])
	require.EqualValues(t, rec.RecordsImported, data["records_imported"])
	require.EqualValues(t, rec.RecordsSkipped, data["records_skipped"])
	require.EqualValues(t, time.Duration(rec.DurationMs)*time.Millisecond, time.Duration(data["duration"].(float64)))
	require.NotContains(t, data, "changes", "the record holds no change list, so none is invented")
	require.NotContains(t, data, "warnings")
}

// A row with every field set: the stored counts and the stored error, and no change list.
func TestImportDetailAfterRestart_ShowsAStoredRow(t *testing.T) {
	_, _, logger, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	createdAt, err := time.Parse(time.RFC3339, recordCreatedAtRaw)
	require.NoError(t, err)
	require.NoError(t, db.GetDB().Create(&database.ImportHistory{
		ImportID: directImportID, PluginName: "sma", Format: "sma", Success: false,
		RecordsImported: 4, RecordsSkipped: 2, DurationMs: 250,
		ErrorMessage: "template exists", CreatedAt: createdAt,
	}).Error)

	code, wrap := getJSON(t, restartedRouter(t, db, logger), "/api/v1/import/"+directImportID)
	require.Equal(t, http.StatusOK, code, "%v", wrap)
	data := resultData(t, wrap)

	require.Equal(t, false, data["success"])
	require.EqualValues(t, 4, data["records_imported"])
	require.EqualValues(t, 2, data["records_skipped"])
	require.EqualValues(t, int64(250*time.Millisecond), data["duration"])
	require.Equal(t, []interface{}{"template exists"}, data["errors"])
	require.Equal(t, recordCreatedAtRaw, data["created_at"])
	require.NotContains(t, data, "changes")
}

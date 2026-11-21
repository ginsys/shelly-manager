package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// With no export root configured, exports and downloads use ./data/exports: a file under it is served, a
// file outside it (one recorded earlier, or one a request names now) is not.
func TestExportAndDownloadUseTheDefaultRootWhenNoneIsConfigured(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	engine := sync.NewSyncEngine(db, logger)
	engine.SetExportBaseDir(string(filepath.Separator)) // only to record an export outside the default root
	require.NoError(t, engine.RegisterPlugin(&mockSyncPlugin{}))
	h := NewHandlerWithLogger(db, nil, nil, nil, logger)
	exp := NewSyncHandlers(engine, logger)
	exp.SetExportBaseDir(string(filepath.Separator))
	h.ExportHandlers = exp
	r := mux.NewRouter()
	exp.AddExportRoutes(r.PathPrefix("/api/v1").Subrouter())

	work := t.TempDir()
	t.Chdir(work)
	require.NoError(t, os.MkdirAll(filepath.Join(work, "data", "exports"), 0o755))

	export := func(destination string) (*httptest.ResponseRecorder, string) {
		body, err := json.Marshal(map[string]interface{}{
			"plugin_name": "mockfile", "format": "txt",
			"config": map[string]interface{}{}, "filters": map[string]interface{}{},
			"output":  map[string]interface{}{"type": "file", "destination": destination},
			"options": map[string]interface{}{},
		})
		require.NoError(t, err)
		req := httptest.NewRequest("POST", "/api/v1/export", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		var wrap map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &wrap)
		id := ""
		if data, ok := wrap["data"].(map[string]interface{}); ok {
			id, _ = data["export_id"].(string)
		}
		return rr, id
	}
	download := func(id string) int {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/export/"+id+"/download", nil))
		return rr.Code
	}

	// An export recorded while the root was wide open, outside ./data/exports
	outside := filepath.Join(t.TempDir(), "earlier.txt")
	rr, earlierID := export(outside)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	// No root configured from here on: the engine and the handlers use the default one
	engine.SetExportBaseDir("")
	exp.SetExportBaseDir("")

	require.Equal(t, http.StatusForbidden, download(earlierID), "an export outside the default root was served")

	rr, id := export("inside.txt")
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.FileExists(t, filepath.Join(work, "data", "exports", "inside.txt"))
	require.Equal(t, http.StatusOK, download(id))

	refused := filepath.Join(t.TempDir(), "refused.txt")
	rr, _ = export(refused)
	require.GreaterOrEqual(t, rr.Code, 400, rr.Body.String())
	require.NoFileExists(t, refused)
}

// With no export root configured a backup is written to ./data/backups by the backup plugin, so a backup
// recorded in the history is served from there, while any other plugin's file outside ./data/exports is not.
func TestBackupDownloadWithNoRootIsServedFromTheBackupDirectory(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	exp := NewSyncHandlers(sync.NewSyncEngine(db, logger), logger)
	r := mux.NewRouter()
	exp.AddExportRoutes(r.PathPrefix("/api/v1").Subrouter())

	work := t.TempDir()
	t.Chdir(work)
	for _, dir := range []string{"exports", "backups", "elsewhere"} {
		require.NoError(t, os.MkdirAll(filepath.Join(work, "data", dir), 0o755))
	}
	record := func(id, plugin, dir string) {
		file := filepath.Join(work, "data", dir, id+".txt")
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
		require.NoError(t, db.GetDB().Create(&database.ExportHistory{
			ExportID: id, PluginName: plugin, Format: "txt", Success: true, FilePath: file,
		}).Error)
	}
	record("bk", "backup", "backups")
	record("bk-elsewhere", "backup", "elsewhere")
	record("other", "mockfile", "backups")

	download := func(path string) int {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		return rr.Code
	}
	require.Equal(t, http.StatusOK, download("/api/v1/export/backup/bk/download"))
	require.Equal(t, http.StatusForbidden, download("/api/v1/export/backup/bk-elsewhere/download"))
	require.Equal(t, http.StatusForbidden, download("/api/v1/export/json/other/download"), "only a backup may be served from the backup directory")
}

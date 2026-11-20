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
	"github.com/ginsys/shelly-manager/internal/plugins/sync/backup"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/registry"
	"github.com/ginsys/shelly-manager/internal/sync"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

const goodBackupName = "shelly-backup-20261120-081500-0a1b2c3d.sqlite"

type restoreRig struct {
	router http.Handler
	db     *database.Manager
	dir    string
}

// newRestoreRig builds the import routes over a real engine with the real backup plugin, and
// confines stored backups to a temporary directory when withBase is set.
func newRestoreRig(t *testing.T, withBase bool) *restoreRig {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	engine := sync.NewSyncEngine(db, logger)
	require.NoError(t, engine.RegisterPlugin(backup.NewBackupExporter(&registry.DatabaseManagerAdapter{Manager: db})))
	imp := NewImportHandlers(engine, logger)
	dir := t.TempDir()
	if withBase {
		imp.SetExportBaseDir(dir)
	}
	r := mux.NewRouter()
	imp.AddImportRoutes(r.PathPrefix("/api/v1").Subrouter())
	return &restoreRig{router: r, db: db, dir: dir}
}

func (rg *restoreRig) record(t *testing.T, id, plugin, path string, success bool) {
	t.Helper()
	require.NoError(t, rg.db.GetDB().Create(&database.ExportHistory{
		ExportID: id, PluginName: plugin, Format: "sqlite", Name: "nightly", Success: success,
		RecordCount: 12, FileSize: 4, FilePath: path,
	}).Error)
}

func (rg *restoreRig) post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/import/restore-preview", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rg.router.ServeHTTP(rr, req)
	return rr
}

func (rg *restoreRig) file(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(rg.dir, name)
	require.NoError(t, os.WriteFile(p, []byte("sqlite bytes"), 0o600))
	return p
}

func TestRestorePreviewChecksAStoredBackup(t *testing.T) {
	rg := newRestoreRig(t, true)
	rg.record(t, "ok-1", "backup", rg.file(t, goodBackupName), true)

	rr := rg.post(t, `{"backup_id":"ok-1"}`)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var wrap struct {
		Data struct {
			BackupID    string   `json:"backup_id"`
			Valid       bool     `json:"valid"`
			FileSize    int64    `json:"file_size"`
			RecordCount int      `json:"record_count"`
			Warnings    []string `json:"warnings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	require.Equal(t, "ok-1", wrap.Data.BackupID)
	require.True(t, wrap.Data.Valid)
	require.Equal(t, int64(len("sqlite bytes")), wrap.Data.FileSize)
	require.Equal(t, 12, wrap.Data.RecordCount, "the record count comes from the history row")
	require.NotEmpty(t, wrap.Data.Warnings, "the answer says how little was checked")
	require.NotContains(t, rr.Body.String(), rg.dir, "the server path is never returned")
}

func TestRestorePreviewRefusesWhatIsNotAStoredBackup(t *testing.T) {
	rg := newRestoreRig(t, true)
	good := rg.file(t, goodBackupName)
	rg.record(t, "other-plugin", "gitops", good, true)
	rg.record(t, "failed", "backup", good, false)

	outside := filepath.Join(t.TempDir(), goodBackupName)
	require.NoError(t, os.WriteFile(outside, []byte("elsewhere"), 0o600))
	rg.record(t, "outside-base", "backup", outside, true)
	rg.record(t, "traversal", "backup", filepath.Join(rg.dir, "..", "..", "etc", "passwd"), true)
	rg.record(t, "wrong-name", "backup", rg.file(t, "notes.txt"), true)
	rg.record(t, "missing", "backup", filepath.Join(rg.dir, "shelly-backup-20261120-081500-ffffffff.sqlite"), true)

	link := filepath.Join(rg.dir, "shelly-backup-20261120-081501-0a1b2c3d.sqlite")
	if err := os.Symlink(good, link); err == nil {
		rg.record(t, "symlink", "backup", link, true)
	}
	// A symlinked directory inside the base that leads outside it: the path string stays under the
	// base and the last element is a regular file, so only resolving the links refuses it.
	outsideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, goodBackupName), []byte("elsewhere"), 0o600))
	linkedDir := filepath.Join(rg.dir, "sub")
	linkedDirOK := os.Symlink(outsideDir, linkedDir) == nil
	if linkedDirOK {
		rg.record(t, "linked-dir", "backup", filepath.Join(linkedDir, goodBackupName), true)
	}
	dirAsFile := filepath.Join(rg.dir, "shelly-backup-20261120-081502-0a1b2c3d.sqlite")
	require.NoError(t, os.Mkdir(dirAsFile, 0o755))
	rg.record(t, "directory", "backup", dirAsFile, true)

	cases := map[string]int{
		"does-not-exist": http.StatusNotFound,
		"other-plugin":   http.StatusNotFound,
		"failed":         http.StatusUnprocessableEntity,
		"outside-base":   http.StatusUnprocessableEntity,
		"traversal":      http.StatusUnprocessableEntity, // refused by the file-name pattern, not by containment
		"wrong-name":     http.StatusUnprocessableEntity,
		"missing":        http.StatusUnprocessableEntity,
		"directory":      http.StatusUnprocessableEntity,
	}
	if linkedDirOK {
		cases["linked-dir"] = http.StatusUnprocessableEntity
	}
	if _, err := os.Lstat(link); err == nil {
		cases["symlink"] = http.StatusUnprocessableEntity
	}
	for id, want := range cases {
		rr := rg.post(t, `{"backup_id":"`+id+`"}`)
		require.Equal(t, want, rr.Code, "%s: %s", id, rr.Body.String())
		require.NotContains(t, rr.Body.String(), rg.dir, "%s: the answer must not name a server path", id)
		require.NotContains(t, rr.Body.String(), "passwd", "%s: the answer must not name a server path", id)
	}
}

// An empty or unreadable file is a check result (200, valid false), and the answer never carries
// the provider's error text, which names the server path.
func TestRestorePreviewReportsAnUnusableFileWithoutAPath(t *testing.T) {
	rg := newRestoreRig(t, true)

	empty := filepath.Join(rg.dir, "shelly-backup-20261120-081503-0a1b2c3d.sqlite")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	rg.record(t, "empty", "backup", empty, true)

	unreadable := filepath.Join(rg.dir, "shelly-backup-20261120-081504-0a1b2c3d.sqlite")
	require.NoError(t, os.WriteFile(unreadable, []byte("sqlite bytes"), 0o000))
	rg.record(t, "unreadable", "backup", unreadable, true)
	canOpen := false
	if f, err := os.Open(unreadable); err == nil {
		_ = f.Close()
		canOpen = true // running as root: the permission does not stop the open
	}

	ids := []string{"empty"}
	if !canOpen {
		ids = append(ids, "unreadable")
	}
	for _, id := range ids {
		rr := rg.post(t, `{"backup_id":"`+id+`"}`)
		require.Equal(t, http.StatusOK, rr.Code, "%s: %s", id, rr.Body.String())
		require.NotContains(t, rr.Body.String(), rg.dir, "%s: the answer must not name a server path", id)

		var wrap struct {
			Data struct {
				Valid  bool     `json:"valid"`
				Errors []string `json:"errors"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
		require.False(t, wrap.Data.Valid, id)
		if id == "unreadable" {
			var sized struct {
				Data struct {
					FileSize int64 `json:"file_size"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &sized))
			require.Equal(t, int64(len("sqlite bytes")), sized.Data.FileSize, "the size of the file the server could stat")
		}
		require.Equal(t, []string{"The backup file is empty or cannot be opened"}, wrap.Data.Errors, id)
	}
}

// Without a configured export directory the file name and the regular-file check still hold.
func TestRestorePreviewWithoutABaseDirectoryStillChecksNameAndType(t *testing.T) {
	rg := newRestoreRig(t, false)
	rg.record(t, "wrong-name", "backup", rg.file(t, "passwd"), true)
	require.Equal(t, http.StatusUnprocessableEntity, rg.post(t, `{"backup_id":"wrong-name"}`).Code)
	rg.record(t, "ok", "backup", rg.file(t, goodBackupName), true)
	require.Equal(t, http.StatusOK, rg.post(t, `{"backup_id":"ok"}`).Code)
}

// The route takes a backup id and nothing else: a path in the body is an error, never ignored.
func TestRestorePreviewTakesNoPathFromTheRequest(t *testing.T) {
	rg := newRestoreRig(t, true)
	rg.record(t, "ok-1", "backup", rg.file(t, goodBackupName), true)

	for _, body := range []string{
		`{"backup_id":"ok-1","backup_path":"/etc/passwd"}`,
		`{"backup_id":"ok-1","path":"/etc/passwd"}`,
		`{"backup_path":"/etc/passwd"}`,
		`{}`,
		`{"backup_id":""}`,
		`not json`,
	} {
		rr := rg.post(t, body)
		require.Equal(t, http.StatusBadRequest, rr.Code, "%s: %s", body, rr.Body.String())
	}
}

// The two client-path routes were removed: a request to either reaches no handler.
func TestClientPathBackupRoutesAreGone(t *testing.T) {
	rg := newRestoreRig(t, true)
	for _, path := range []string{"/api/v1/import/backup", "/api/v1/import/backup/validate"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{"backup_path":"/etc/passwd"}`)))
		rr := httptest.NewRecorder()
		rg.router.ServeHTTP(rr, req)
		require.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rr.Code, path)
	}
}

package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/database/restore"
)

// sqliteFile writes a SQLite file with a devices table holding one row per name.
func sqliteFile(t *testing.T, path string, names ...string) {
	t.Helper()
	db, err := sql.Open(sqlite.DriverName, path)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	_, err = db.Exec("CREATE TABLE devices (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	for _, n := range names {
		_, err = db.Exec("INSERT INTO devices (name) VALUES (?)", n)
		require.NoError(t, err)
	}
}

// stageRig is the restore rig with a live SQLite file the restore replaces at the next start.
type stageRig struct {
	*restoreRig
	live string
}

func newStageRig(t *testing.T) *stageRig {
	t.Helper()
	rg := newRestoreRig(t, true)
	live := filepath.Join(t.TempDir(), "shelly.db")
	sqliteFile(t, live, "live-1")
	rg.imp.SetRestoreDatabase(provider.DatabaseConfig{Provider: "sqlite", DSN: live})
	return &stageRig{restoreRig: rg, live: live}
}

func (rg *restoreRig) do(t *testing.T, method, path, body string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rr := httptest.NewRecorder()
	rg.router.ServeHTTP(rr, req)
	return rr
}

const restorePath = "/api/v1/import/restore"

func (rg *stageRig) goodBackup(t *testing.T, id string) {
	t.Helper()
	p := filepath.Join(rg.dir, goodBackupName)
	sqliteFile(t, p, "backup-1", "backup-2")
	rg.record(t, id, "backup", p, true)
}

func TestStageRestoreStagesACheckedBackup(t *testing.T) {
	rg := newStageRig(t)
	rg.goodBackup(t, "ok-1")
	before, err := os.ReadFile(rg.live)
	require.NoError(t, err)

	rr := rg.do(t, http.MethodPost, restorePath, `{"backup_id":"ok-1"}`)
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "at the next start")
	assert.NotContains(t, rr.Body.String(), rg.dir, "the server path is never returned")

	after, err := os.ReadFile(rg.live)
	require.NoError(t, err)
	assert.Equal(t, before, after, "staging never touches the live database")
	p, err := restore.Pending(rg.live)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "ok-1", p.BackupID)

	st := rg.do(t, http.MethodGet, restorePath, "")
	require.Equal(t, http.StatusOK, st.Code, st.Body.String())
	var wrap struct {
		Data struct {
			Pending *struct {
				BackupID string `json:"backup_id"`
			} `json:"pending"`
			LastResult any `json:"last_result"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(st.Body.Bytes(), &wrap))
	require.NotNil(t, wrap.Data.Pending)
	assert.Equal(t, "ok-1", wrap.Data.Pending.BackupID)
	assert.Nil(t, wrap.Data.LastResult)
}

func TestStageRestoreDryRunStagesNothing(t *testing.T) {
	rg := newStageRig(t)
	rg.goodBackup(t, "ok-1")

	rr := rg.do(t, http.MethodPost, restorePath, `{"backup_id":"ok-1","dry_run":true}`)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"dry_run":true`)
	p, err := restore.Pending(rg.live)
	require.NoError(t, err)
	assert.Nil(t, p)

	// dry_run is read from the body field only: a query string does not turn a real restore into one.
	rr = rg.do(t, http.MethodPost, restorePath+"?dry_run=true", `{"backup_id":"ok-1"}`)
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
	p, err = restore.Pending(rg.live)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestStageRestoreTakesNoPathFromTheRequest(t *testing.T) {
	rg := newStageRig(t)
	rg.goodBackup(t, "ok-1")
	for _, body := range []string{
		`{"backup_id":"ok-1","backup_path":"/etc/passwd"}`,
		`{"backup_id":"ok-1","path":"/etc/passwd"}`,
		`{"backup_id":"ok-1","dry_run":"yes"}`,
		`{"backup_path":"/etc/passwd"}`,
		`{}`,
		`{"backup_id":""}`,
		`not json`,
	} {
		rr := rg.do(t, http.MethodPost, restorePath, body)
		require.Equal(t, http.StatusBadRequest, rr.Code, "%s: %s", body, rr.Body.String())
	}
	p, err := restore.Pending(rg.live)
	require.NoError(t, err)
	assert.Nil(t, p, "no refused request staged anything")
}

func TestStageRestoreRefusesWhatCannotBeRestored(t *testing.T) {
	rg := newStageRig(t)
	rg.goodBackup(t, "ok-1")
	rg.record(t, "other-plugin", "gitops", filepath.Join(rg.dir, goodBackupName), true)

	bad := filepath.Join(rg.dir, "shelly-backup-20261120-081501-0a1b2c3d.sqlite")
	require.NoError(t, os.WriteFile(bad, []byte(strings.Repeat("not a database ", 300)), 0o600))
	rg.record(t, "corrupt", "backup", bad, true)

	outside := filepath.Join(t.TempDir(), goodBackupName)
	sqliteFile(t, outside, "x")
	rg.record(t, "outside-base", "backup", outside, true)

	cases := map[string]int{
		"does-not-exist": http.StatusNotFound,
		"other-plugin":   http.StatusNotFound,
		"corrupt":        http.StatusUnprocessableEntity,
		"outside-base":   http.StatusUnprocessableEntity,
	}
	for id, want := range cases {
		rr := rg.do(t, http.MethodPost, restorePath, `{"backup_id":"`+id+`"}`)
		require.Equal(t, want, rr.Code, "%s: %s", id, rr.Body.String())
		assert.NotContains(t, rr.Body.String(), rg.dir, "%s: the answer names no server path", id)
	}
	p, err := restore.Pending(rg.live)
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestCancelRestoreRemovesAPendingRestore(t *testing.T) {
	rg := newStageRig(t)
	rg.goodBackup(t, "ok-1")

	require.Equal(t, http.StatusNotFound, rg.do(t, http.MethodDelete, restorePath, "").Code, "nothing pending yet")
	require.Equal(t, http.StatusAccepted, rg.do(t, http.MethodPost, restorePath, `{"backup_id":"ok-1"}`).Code)
	require.Equal(t, http.StatusNoContent, rg.do(t, http.MethodDelete, restorePath, "").Code)
	p, err := restore.Pending(rg.live)
	require.NoError(t, err)
	assert.Nil(t, p)
	require.Equal(t, http.StatusNotFound, rg.do(t, http.MethodDelete, restorePath, "").Code)
}

// Only SQLite restores: with any other database, or none set, all three routes answer a named 501.
func TestRestoreRoutesAnswer501ForOtherDatabases(t *testing.T) {
	for name, cfg := range map[string]*provider.DatabaseConfig{
		"not configured": nil,
		"postgresql":     {Provider: "postgresql", DSN: "host=db"},
		"mysql":          {Provider: "mysql", DSN: "user@tcp(db)/x"},
		"in memory":      {Provider: "sqlite", DSN: ":memory:"},
	} {
		t.Run(name, func(t *testing.T) {
			rg := newRestoreRig(t, true)
			if cfg != nil {
				rg.imp.SetRestoreDatabase(*cfg)
			}
			rg.record(t, "ok-1", "backup", rg.file(t, goodBackupName), true)
			for _, c := range []struct{ method, body string }{
				{http.MethodPost, `{"backup_id":"ok-1"}`},
				{http.MethodGet, ""},
				{http.MethodDelete, ""},
			} {
				rr := rg.do(t, c.method, restorePath, c.body)
				require.Equal(t, http.StatusNotImplemented, rr.Code, "%s: %s", c.method, rr.Body.String())
				assert.Contains(t, rr.Body.String(), "SQLite")
			}
		})
	}
}

func TestRestoreRoutesNeedTheAdminKey(t *testing.T) {
	rg := newStageRig(t)
	rg.imp.SetAdminAPIKey("secret-key")
	rg.goodBackup(t, "ok-1")

	for _, c := range []struct{ method, body string }{
		{http.MethodPost, `{"backup_id":"ok-1"}`},
		{http.MethodGet, ""},
		{http.MethodDelete, ""},
	} {
		rr := rg.do(t, c.method, restorePath, c.body)
		require.Equal(t, http.StatusUnauthorized, rr.Code, "%s: %s", c.method, rr.Body.String())
	}
	p, err := restore.Pending(rg.live)
	require.NoError(t, err)
	assert.Nil(t, p, "an unauthorised request stages nothing")

	rr := rg.do(t, http.MethodPost, restorePath, `{"backup_id":"ok-1"}`, "Authorization", "Bearer secret-key")
	require.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
}

package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/database/restore"
)

// markerDB opens path with the manager (so the schema exists), writes one marker row and closes it.
func markerDB(t *testing.T, path, marker string) {
	t.Helper()
	m, err := NewManagerFromPath(path)
	require.NoError(t, err)
	require.NoError(t, m.GetDB().Exec("CREATE TABLE IF NOT EXISTS restore_marker (v TEXT)").Error)
	require.NoError(t, m.GetDB().Exec("INSERT INTO restore_marker (v) VALUES (?)", marker).Error)
	require.NoError(t, m.Close())
}

func markerOf(t *testing.T, m *Manager) string {
	t.Helper()
	var v string
	require.NoError(t, m.GetDB().Raw("SELECT v FROM restore_marker LIMIT 1").Scan(&v).Error)
	return v
}

func TestStartAppliesAStagedRestore(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "shelly.db")
	backupPath := filepath.Join(dir, "backup.sqlite")
	markerDB(t, backupPath, "from-backup")
	markerDB(t, live, "from-live")

	_, err := restore.Stage(context.Background(), live, backupPath, "export-1", false)
	require.NoError(t, err)

	m, err := NewManagerFromPath(live)
	require.NoError(t, err)
	defer func() { _ = m.Close() }()
	assert.Equal(t, "from-backup", markerOf(t, m), "the staged restore is applied before the database is opened")
	res, err := restore.LastResult(live)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, restore.StatusApplied, res.Status)
}

// A restored file can pass every check at stage and still fail to migrate. The start must then put
// the previous database back instead of ending in a crash loop on the restored file.
func TestStartRollsBackARestoreThatCannotBeMigrated(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "shelly.db")
	markerDB(t, live, "from-live")

	// A devices table passes the stage check; a view named like a model's table makes the migration
	// fail when it tries to create that table.
	bad := filepath.Join(dir, "bad.sqlite")
	db, err := sql.Open(sqlite.DriverName, bad)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE devices (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("CREATE VIEW notification_channels AS SELECT 1 AS id")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = restore.Stage(context.Background(), live, bad, "export-bad", false)
	require.NoError(t, err)

	m, err := NewManagerFromPath(live)
	require.NoError(t, err, "the start recovers on the previous database")
	defer func() { _ = m.Close() }()
	assert.Equal(t, "from-live", markerOf(t, m))

	res, err := restore.LastResult(live)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, restore.StatusFailed, res.Status)
	assert.Equal(t, "export-bad", res.BackupID)
	pending, err := restore.Pending(live)
	require.NoError(t, err)
	assert.Nil(t, pending)
	_, err = os.Stat(live + ".pre-restore")
	assert.NoError(t, err, "the pre-restore copy stays")
}

func TestStartWithoutAStagedRestoreChangesNothing(t *testing.T) {
	live := filepath.Join(t.TempDir(), "shelly.db")
	markerDB(t, live, "from-live")
	m, err := NewManagerFromPath(live)
	require.NoError(t, err)
	defer func() { _ = m.Close() }()
	assert.Equal(t, "from-live", markerOf(t, m))
	_, err = os.Stat(restore.Dir(live))
	assert.True(t, os.IsNotExist(err), "no restore directory is created by an ordinary start")
}

func TestRestoreTarget(t *testing.T) {
	cases := []struct {
		name, provider, dsn, want string
		ok                        bool
	}{
		{"sqlite path", "sqlite", "/data/shelly.db", "/data/shelly.db", true},
		{"sqlite3 alias, upper case", "SQLite3", "data/a.db?x=y", "data/a.db", true},
		{"memory", "sqlite", ":memory:", "", false},
		{"postgres", "postgresql", "host=db", "", false},
		{"mysql", "mysql", "user@tcp(db)/x", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := RestoreTarget(provider.DatabaseConfig{Provider: c.provider, DSN: c.dsn})
			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

package provider

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// ginsys/shelly-manager#837: the driver opens "file:x.db" as x.db, so every place that reads the DSN as a
// path must drop the prefix. The tests that call t.Chdir are never parallel: the working directory is the
// process's.

func TestDsnPath(t *testing.T) {
	for dsn, want := range map[string]string{
		"":                            "",
		":memory:":                    ":memory:",
		"data/x.db":                   "data/x.db",
		"data/x.db?_pragma=a/b":       "data/x.db",
		"file:x.db":                   "x.db",
		"file:./data/x.db":            "./data/x.db",
		"file:/abs/x.db":              "/abs/x.db",
		"file:///abs/x.db":            "///abs/x.db",
		"file:data/x.db?mode=rwc&a=/": "data/x.db",
	} {
		if got := dsnPath(dsn); got != want {
			t.Errorf("dsnPath(%q) = %q, want %q", dsn, got, want)
		}
	}
}

// noFilePrefixed fails if dir holds an entry whose name starts with "file:", which is what a DSN prefix
// read as a directory leaves behind.
func noFilePrefixed(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "file:") {
			t.Errorf("%s holds %q, made from the file: prefix of a DSN", dir, e.Name())
		}
	}
}

func connectWithRow(t *testing.T, dsn string) *SQLiteProvider {
	t.Helper()
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "false")
	p := NewSQLiteProvider(logging.GetDefault())
	if err := p.Connect(DatabaseConfig{Provider: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("Connect(%q): %v", dsn, err)
	}
	t.Cleanup(func() { _ = p.Close() })
	for _, stmt := range []string{
		"CREATE TABLE notes (id INTEGER PRIMARY KEY, v TEXT)",
		"INSERT INTO notes (v) VALUES ('" + lastWriteValue + "')",
	} {
		if err := p.GetDB().Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	return p
}

func TestCreateBackupOnFileDSN(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for name, c := range map[string]struct{ dsn, dir string }{
		"relative": {"file:data/live.db?_busy_timeout=5000", filepath.Join(root, "data")},
		"absolute": {"file:" + filepath.Join(root, "abs", "live.db"), filepath.Join(root, "abs")},
	} {
		t.Run(name, func(t *testing.T) {
			// The directory exists already, so only the snapshot is under test here: Connect makes a missing
			// one itself (next test).
			if err := os.MkdirAll(c.dir, 0o755); err != nil {
				t.Fatal(err)
			}
			p := connectWithRow(t, c.dsn)
			var out bytes.Buffer
			if _, err := p.CreateBackup(context.Background(), BackupConfig{BackupPath: filepath.Join(root, "b.sqlite"), BackupType: BackupTypeFull, Output: &out}); err != nil {
				t.Fatalf("CreateBackup: %v", err)
			}
			finished := filepath.Join(t.TempDir(), "backup.sqlite")
			if err := os.WriteFile(finished, out.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := readLastWrite(t, unpack(t, finished, "live.db")); got != lastWriteValue {
				t.Fatalf("the backup's last note = %q, want %q", got, lastWriteValue)
			}
			noFilePrefixed(t, root)
			assertNoSnapshotDir(t, c.dir)
		})
	}
}

func TestConnectFileDSNCreatesTheRealDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	connectWithRow(t, "file:data/live.db")
	if fi, err := os.Stat(filepath.Join(root, "data", "live.db")); err != nil || fi.Size() == 0 {
		t.Fatalf("data/live.db: stat err %v; the database must be at the path the DSN names", err)
	}
	noFilePrefixed(t, root)
}

// The size in the stats is the size of the file the DSN names, whatever the DSN's form.
func TestStatsDatabaseSizeFollowsTheDSNPath(t *testing.T) {
	for name, dsn := range map[string]func(string) string{
		"query only":    func(live string) string { return live + "?_busy_timeout=5000" },
		"file:":         func(live string) string { return "file:" + live },
		"file: + query": func(live string) string { return "file:" + live + "?_busy_timeout=5000" },
	} {
		t.Run(name, func(t *testing.T) {
			p := connectWithRow(t, dsn(filepath.Join(t.TempDir(), "live.db")))
			if err := p.GetDB().Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
				t.Fatal(err)
			}
			if size := p.GetStats().DatabaseSize; size <= 0 {
				t.Fatalf("DatabaseSize = %d after a write, want > 0", size)
			}
		})
	}
}

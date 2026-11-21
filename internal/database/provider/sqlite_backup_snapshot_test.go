package provider

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

const lastWriteValue = "the last write, never checkpointed"

// walProvider connects a real file database in WAL mode and writes rows through the driver with no
// checkpoint, so the last row lives only in the -wal file. It checks that, because a test where the
// row reached the main file proves nothing about the backup.
func walProvider(t *testing.T, live string) *SQLiteProvider {
	t.Helper()
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "false")
	p := NewSQLiteProvider(logging.GetDefault())
	if err := p.Connect(DatabaseConfig{Provider: "sqlite", DSN: live}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	db := p.GetDB()
	var mode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, err %v; the test needs WAL", mode, err)
	}
	for _, stmt := range []string{
		"CREATE TABLE notes (id INTEGER PRIMARY KEY, v TEXT)",
		"INSERT INTO notes (v) VALUES ('first')",
		"INSERT INTO notes (v) VALUES ('" + lastWriteValue + "')",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if fi, err := os.Stat(live + "-wal"); err != nil || fi.Size() == 0 {
		t.Fatalf("the writes must sit in the -wal file (stat err %v), or the test proves nothing", err)
	}
	return p
}

// unpack returns the path of a plain SQLite file holding the backup: the backup itself, or the one
// file inside a .gz or .zip. A zip must hold exactly one entry, named like the live file.
func unpack(t *testing.T, backup, liveBase string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "unpacked.db")
	write := func(r io.Reader) {
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		if _, err := io.Copy(f, r); err != nil {
			t.Fatal(err)
		}
	}
	switch strings.ToLower(filepath.Ext(backup)) {
	case ".gz":
		f, err := os.Open(backup)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("not a gzip file: %v", err)
		}
		write(gz)
	case ".zip":
		zr, err := zip.OpenReader(backup)
		if err != nil {
			t.Fatalf("not a zip file: %v", err)
		}
		defer func() { _ = zr.Close() }()
		if len(zr.File) != 1 || zr.File[0].Name != liveBase {
			t.Fatalf("zip entries = %d, first %q; want one entry named %q", len(zr.File), zr.File[0].Name, liveBase)
		}
		rc, err := zr.File[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rc.Close() }()
		write(rc)
	default:
		return backup
	}
	return out
}

// readLastWrite opens the file as a fresh database, checks its integrity and returns the newest note.
func readLastWrite(t *testing.T, path string) string {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open backup as a database: %v", err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	var check string
	if err := db.Raw("PRAGMA integrity_check").Scan(&check).Error; err != nil || check != "ok" {
		t.Fatalf("integrity_check = %q, err %v", check, err)
	}
	var last string
	if err := db.Raw("SELECT v FROM notes ORDER BY id DESC LIMIT 1").Scan(&last).Error; err != nil {
		t.Fatalf("read the last note: %v", err)
	}
	return last
}

// createBackupFile does what the backup plugin does: it creates the file at dst (and its directory) and
// hands it to CreateBackup as Output. The provider never creates the file itself.
func createBackupFile(ctx context.Context, p *SQLiteProvider, dst string) (*BackupResult, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(dst)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return p.CreateBackup(ctx, BackupConfig{BackupPath: dst, BackupType: BackupTypeFull, Output: f})
}

func TestCreateBackupIncludesUncheckpointedWAL(t *testing.T) {
	for _, ext := range []string{".sqlite", ".gz", ".zip"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			live := filepath.Join(dir, "live.db")
			p := walProvider(t, live)

			dst := filepath.Join(dir, "backups", "backup"+ext)
			res, err := createBackupFile(context.Background(), p, dst)
			if err != nil || !res.Success {
				t.Fatalf("CreateBackup: %v %+v", err, res)
			}
			if res.BackupPath != dst || res.Size == 0 || res.Checksum == "" {
				t.Fatalf("BackupResult = %+v", res)
			}
			// The result describes the finished file: its size and its SHA-256.
			written, err := os.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			if res.Size != int64(len(written)) || res.Checksum != fmt.Sprintf("%x", sha256.Sum256(written)) {
				t.Fatalf("result size %d checksum %s; the file has %d bytes, sha256 %x", res.Size, res.Checksum, len(written), sha256.Sum256(written))
			}
			if got := readLastWrite(t, unpack(t, dst, "live.db")); got != lastWriteValue {
				t.Fatalf("the backup's last note = %q, want %q", got, lastWriteValue)
			}
		})
	}
}

func TestCreateBackupSnapshotFailureLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "live.db")
	p := walProvider(t, live)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dst := filepath.Join(dir, "backups", "backup.sqlite")
	var out bytes.Buffer
	if _, err := p.CreateBackup(ctx, BackupConfig{BackupPath: dst, BackupType: BackupTypeFull, Output: &out}); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateBackup error = %v, want context.Canceled", err)
	}
	if out.Len() != 0 {
		t.Fatalf("a failed snapshot wrote %d bytes to the output", out.Len())
	}
	if _, err := os.Stat(filepath.Dir(dst)); !os.IsNotExist(err) {
		t.Fatalf("the provider created the destination directory (stat err %v)", err)
	}
	assertNoSnapshotDir(t, dir)
	var n int64
	if err := p.GetDB().Raw("SELECT count(*) FROM notes").Scan(&n).Error; err != nil || n != 2 {
		t.Fatalf("live database after the failure: %d notes, err %v; want 2", n, err)
	}
}

func TestCreateBackupRefusesDotDotBeforeCreatingAnything(t *testing.T) {
	dir := t.TempDir()
	p := walProvider(t, filepath.Join(dir, "live.db"))

	// filepath.Join would clean the ".." away, so the path is built by hand.
	dst := dir + string(filepath.Separator) + "outer" + string(filepath.Separator) + ".." +
		string(filepath.Separator) + "inner" + string(filepath.Separator) + "backup.sqlite"
	if _, err := p.CreateBackup(context.Background(), BackupConfig{BackupPath: dst, BackupType: BackupTypeFull, Output: io.Discard}); err == nil {
		t.Fatal("CreateBackup accepted a path with a .. component")
	}
	assertNoSnapshotDir(t, dir)
	for _, name := range []string{"outer", "inner"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("a refused path still created %q (stat err %v)", name, err)
		}
	}
}

// BackupResult.BackupPath is the path the caller gave, not its cleaned form: a caller that keeps it
// and later passes it to DeleteBackup (which acts only on an id with a separator) must get the same
// string back. "./backup.sqlite" came back unchanged before the path check and must still.
func TestCreateBackupReturnsThePathAsGiven(t *testing.T) {
	dir := t.TempDir()
	p := walProvider(t, filepath.Join(dir, "live.db"))
	t.Chdir(dir)

	const given = "./backup.sqlite"
	res, err := createBackupFile(context.Background(), p, given)
	if err != nil || !res.Success {
		t.Fatalf("CreateBackup: %v %+v", err, res)
	}
	if res.BackupPath != given {
		t.Fatalf("BackupPath = %q, want the path as given, %q", res.BackupPath, given)
	}
	if err := p.DeleteBackup(res.BackupPath); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backup.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("DeleteBackup(%q) left the backup behind (stat err %v)", res.BackupPath, err)
	}
}

func TestCreateBackupPathWithQuote(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "live.db")
	p := walProvider(t, live)

	dst := filepath.Join(dir, `it's "quoted"`, "backup.sqlite")
	if res, err := createBackupFile(context.Background(), p, dst); err != nil || !res.Success {
		t.Fatalf("CreateBackup: %v %+v", err, res)
	}
	if got := readLastWrite(t, dst); got != lastWriteValue {
		t.Fatalf("the backup's last note = %q, want %q", got, lastWriteValue)
	}
	entries, err := os.ReadDir(filepath.Dir(dst))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("the backup directory holds %d entries, want only the backup", len(entries))
	}
}

// writerFunc is an io.Writer made of a function.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }

// assertNoSnapshotDir fails when a ".backup-*" snapshot directory is left in dir.
func assertNoSnapshotDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".backup-") {
			t.Fatalf("a snapshot directory %q was left in %s", e.Name(), dir)
		}
	}
}

// ginsys/shelly-manager#815: the finished file reaches the caller's Output, never a path. The snapshot
// (VACUUM INTO takes a path, so it cannot be rooted) goes in a private directory beside the live database
// file, not beside BackupPath, where a link in the export directory could redirect it. BackupPath's
// directory does not exist here: a provider that created or used it would fail or create it.
func TestCreateBackupWritesOnlyToOutputAndSnapshotsBesideTheLiveFile(t *testing.T) {
	for _, ext := range []string{".sqlite", ".gz", ".zip"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			live := filepath.Join(dir, "data", "live.db")
			p := walProvider(t, live)

			dst := filepath.Join(dir, "export", "backup"+ext)
			// The first write to Output comes after the snapshot is made and before it is removed: the snapshot
			// directory must be beside the live file then.
			var out bytes.Buffer
			var snapshots []string
			seen := false
			output := writerFunc(func(b []byte) (int, error) {
				if !seen {
					seen = true
					snapshots, _ = filepath.Glob(filepath.Join(filepath.Dir(live), ".backup-*"))
				}
				return out.Write(b)
			})
			res, err := p.CreateBackup(context.Background(), BackupConfig{BackupPath: dst, BackupType: BackupTypeFull, Output: output})
			if err != nil || !res.Success {
				t.Fatalf("CreateBackup: %v %+v", err, res)
			}
			if len(snapshots) != 1 {
				t.Fatalf("snapshot directories beside the live file while the backup was written: %v, want exactly one", snapshots)
			}
			if _, err := os.Stat(filepath.Dir(dst)); !os.IsNotExist(err) {
				t.Fatalf("the provider created or used the BackupPath directory (stat err %v)", err)
			}
			if res.Size != int64(out.Len()) || res.Checksum != fmt.Sprintf("%x", sha256.Sum256(out.Bytes())) {
				t.Fatalf("result size %d checksum %s; the output has %d bytes, sha256 %x", res.Size, res.Checksum, out.Len(), sha256.Sum256(out.Bytes()))
			}
			assertNoSnapshotDir(t, filepath.Dir(live))

			finished := filepath.Join(t.TempDir(), "backup"+ext)
			if err := os.WriteFile(finished, out.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := readLastWrite(t, unpack(t, finished, "live.db")); got != lastWriteValue {
				t.Fatalf("the backup's last note = %q, want %q", got, lastWriteValue)
			}
		})
	}
}

func TestCreateBackupRefusesANilOutput(t *testing.T) {
	dir := t.TempDir()
	p := walProvider(t, filepath.Join(dir, "live.db"))
	if _, err := p.CreateBackup(context.Background(), BackupConfig{BackupPath: filepath.Join(dir, "b.sqlite"), BackupType: BackupTypeFull}); err == nil {
		t.Fatal("CreateBackup went ahead without an output")
	}
	if _, err := os.Stat(filepath.Join(dir, "b.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("a refused backup created a file (stat err %v)", err)
	}
}

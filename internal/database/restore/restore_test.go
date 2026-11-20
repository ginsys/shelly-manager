package restore

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func testLogger(t *testing.T) *logging.Logger {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	return logger
}

// makeDB writes a SQLite file in WAL mode (the production mode, so the header says WAL) holding a
// devices table with one row per name. Closing it checkpoints, so no -wal file is left behind.
func makeDB(t *testing.T, path string, withDevices bool, names ...string) {
	t.Helper()
	db, err := sql.Open(sqlite.DriverName, path)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)
	table := "devices"
	if !withDevices {
		table = "other"
	}
	_, err = db.Exec("CREATE TABLE " + table + " (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	for _, n := range names {
		_, err = db.Exec("INSERT INTO "+table+" (name) VALUES (?)", n)
		require.NoError(t, err)
	}
}

// readNames returns the device names of a SQLite file, or fails when it is not a usable database.
func readNames(t *testing.T, path string) []string {
	t.Helper()
	db, err := sql.Open(sqlite.DriverName, path)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	rows, err := db.Query("SELECT name FROM devices ORDER BY id")
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	var out []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		out = append(out, n)
	}
	require.NoError(t, rows.Err())
	return out
}

func writeGzip(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	f, err := os.Create(dst)
	require.NoError(t, err)
	gz := gzip.NewWriter(f)
	_, err = gz.Write(data)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.NoError(t, f.Close())
}

func writeZip(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	f, err := os.Create(dst)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	w, err := zw.Create(filepath.Base(src))
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	return string(sum[:])
}

// rig holds a live database with the rows "old-1" and "old-2", and a directory for backups.
type rig struct {
	live string
	dir  string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	dir := t.TempDir()
	r := &rig{live: filepath.Join(dir, "data", "shelly.db"), dir: filepath.Join(dir, "backups")}
	require.NoError(t, os.MkdirAll(filepath.Dir(r.live), 0o755))
	require.NoError(t, os.MkdirAll(r.dir, 0o755))
	makeDB(t, r.live, true, "old-1", "old-2")
	return r
}

// backup writes a backup of a database holding names, in the format the extension names.
func (r *rig) backup(t *testing.T, name string, names ...string) string {
	t.Helper()
	raw := filepath.Join(r.dir, strings.TrimSuffix(strings.TrimSuffix(name, ".gz"), ".zip"))
	makeDB(t, raw, true, names...)
	switch {
	case strings.HasSuffix(name, ".gz"):
		writeGzip(t, raw, filepath.Join(r.dir, name))
		require.NoError(t, os.Remove(raw))
	case strings.HasSuffix(name, ".zip"):
		writeZip(t, raw, filepath.Join(r.dir, name))
		require.NoError(t, os.Remove(raw))
	}
	return filepath.Join(r.dir, name)
}

func TestStageThenApplyRestoresEachBackupFormat(t *testing.T) {
	for _, name := range []string{"b.sqlite.gz", "b.sqlite.zip", "b.sqlite"} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			path := r.backup(t, name, "new-1", "new-2", "new-3")

			require.NoError(t, Stage(context.Background(), r.live, path, "export-1", false))
			p, err := Pending(r.live)
			require.NoError(t, err)
			require.NotNil(t, p)
			assert.Equal(t, "export-1", p.BackupID)
			assert.Equal(t, []string{"old-1", "old-2"}, readNames(t, r.live), "staging never touches the live file")
			// The backup header says WAL: opening it for the check must not leave -wal or -shm files.
			entries, err := os.ReadDir(Dir(r.live))
			require.NoError(t, err)
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			assert.ElementsMatch(t, []string{"pending.json", "staged.sqlite"}, names)

			require.True(t, ApplyPending(r.live, testLogger(t)))

			assert.Equal(t, []string{"new-1", "new-2", "new-3"}, readNames(t, r.live))
			assert.Equal(t, []string{"old-1", "old-2"}, readNames(t, r.live+".pre-restore"), "one copy of the live file is kept")
			res, err := LastResult(r.live)
			require.NoError(t, err)
			require.NotNil(t, res)
			assert.Equal(t, StatusApplied, res.Status)
			assert.Equal(t, "export-1", res.BackupID)
			p, err = Pending(r.live)
			require.NoError(t, err)
			assert.Nil(t, p, "an applied restore is no longer pending")

			// The backup header says WAL; neither the check nor the apply may leave side files.
			for _, side := range []string{"-wal", "-shm"} {
				assert.False(t, exists(filepath.Join(Dir(r.live), "staged.sqlite"+side)), "staged.sqlite%s", side)
				assert.False(t, exists(r.live+side), "live%s", side)
			}
		})
	}
}

func TestStageRefusesWhatCannotBeRestored(t *testing.T) {
	r := newRig(t)
	good := r.backup(t, "good.sqlite", "new-1")
	data, err := os.ReadFile(good)
	require.NoError(t, err)

	truncated := filepath.Join(r.dir, "truncated.sqlite")
	require.NoError(t, os.WriteFile(truncated, data[:len(data)/2+100], 0o600))
	notSQLite := filepath.Join(r.dir, "text.sqlite")
	require.NoError(t, os.WriteFile(notSQLite, []byte(strings.Repeat("this is not a database\n", 200)), 0o600))
	noDevices := filepath.Join(r.dir, "other.sqlite")
	makeDB(t, noDevices, false, "x")
	twoEntries := filepath.Join(r.dir, "two.sqlite.zip")
	zf, err := os.Create(twoEntries)
	require.NoError(t, err)
	zw := zip.NewWriter(zf)
	for _, n := range []string{"a", "b"} {
		w, werr := zw.Create(n)
		require.NoError(t, werr)
		_, werr = w.Write(data)
		require.NoError(t, werr)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, zf.Close())
	badGzip := filepath.Join(r.dir, "bad.sqlite.gz")
	require.NoError(t, os.WriteFile(badGzip, []byte("not gzip at all"), 0o600))

	before := fileHash(t, r.live)
	for name, path := range map[string]string{
		"truncated database": truncated, "not SQLite": notSQLite, "no devices table": noDevices,
		"zip with two entries": twoEntries, "bad gzip": badGzip,
	} {
		t.Run(name, func(t *testing.T) {
			err := Stage(context.Background(), r.live, path, "export-x", false)
			var ce *CheckError
			require.True(t, errors.As(err, &ce), "want a CheckError, got %v", err)
			assert.NotContains(t, ce.Reason, r.dir, "the reason never names a server path")
			assert.Equal(t, before, fileHash(t, r.live), "the live database is untouched")
			p, perr := Pending(r.live)
			require.NoError(t, perr)
			assert.Nil(t, p)
			assert.False(t, exists(filepath.Join(Dir(r.live), "staged.sqlite")))
			assert.False(t, exists(filepath.Join(Dir(r.live), "staged.sqlite.tmp")), "the temporary copy is removed")
		})
	}
}

func TestDryRunChecksAndStagesNothing(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite.gz", "new-1")

	require.NoError(t, Stage(context.Background(), r.live, path, "export-1", true))

	entries, rerr := os.ReadDir(Dir(r.live))
	if rerr == nil {
		assert.Empty(t, entries, "a dry run leaves nothing in the restore directory")
	} else {
		assert.True(t, os.IsNotExist(rerr))
	}
	p, err := Pending(r.live)
	require.NoError(t, err)
	assert.Nil(t, p)

	// The same dry run refuses what a real stage refuses.
	bad := filepath.Join(r.dir, "text.sqlite")
	require.NoError(t, os.WriteFile(bad, []byte(strings.Repeat("x", 4096)), 0o600))
	err = Stage(context.Background(), r.live, bad, "export-2", true)
	var ce *CheckError
	require.True(t, errors.As(err, &ce))
}

func TestFailedApplyKeepsTheOldDatabaseAndRecordsTheError(t *testing.T) {
	t.Run("staged file changed after staging", func(t *testing.T) {
		r := newRig(t)
		path := r.backup(t, "b.sqlite", "new-1")
		err := Stage(context.Background(), r.live, path, "export-1", false)
		require.NoError(t, err)
		staged := filepath.Join(Dir(r.live), "staged.sqlite")
		f, err := os.OpenFile(staged, os.O_APPEND|os.O_WRONLY, 0)
		require.NoError(t, err)
		_, err = f.Write([]byte("tamper"))
		require.NoError(t, err)
		require.NoError(t, f.Close())
		before := fileHash(t, r.live)

		assert.False(t, ApplyPending(r.live, testLogger(t)))

		assert.Equal(t, before, fileHash(t, r.live))
		assert.Equal(t, []string{"old-1", "old-2"}, readNames(t, r.live))
		res, err := LastResult(r.live)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, StatusFailed, res.Status)
		assert.NotEmpty(t, res.Error)
		assert.NotContains(t, res.Error, r.live, "the recorded error names no server path")
		p, err := Pending(r.live)
		require.NoError(t, err)
		assert.Nil(t, p, "a failed apply is not retried at every start")
	})

	t.Run("the pre-restore copy cannot be written", func(t *testing.T) {
		r := newRig(t)
		path := r.backup(t, "b.sqlite", "new-1")
		err := Stage(context.Background(), r.live, path, "export-1", false)
		require.NoError(t, err)
		// A non-empty directory where the copy goes makes creating the file fail.
		blocker := r.live + ".pre-restore.tmp"
		require.NoError(t, os.MkdirAll(filepath.Join(blocker, "x"), 0o755))

		assert.False(t, ApplyPending(r.live, testLogger(t)))

		assert.Equal(t, []string{"old-1", "old-2"}, readNames(t, r.live))
		res, err := LastResult(r.live)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, StatusFailed, res.Status)
	})
}

func TestApplyRemovesStaleWalAndShm(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite", "new-1")
	err := Stage(context.Background(), r.live, path, "export-1", false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(r.live+"-wal", []byte("stale wal"), 0o600))
	require.NoError(t, os.WriteFile(r.live+"-shm", []byte("stale shm"), 0o600))

	require.True(t, ApplyPending(r.live, testLogger(t)))

	assert.False(t, exists(r.live+"-wal"), "a stale -wal would be replayed into the restored file")
	assert.False(t, exists(r.live+"-shm"))
	assert.Equal(t, []string{"new-1"}, readNames(t, r.live))
	kept, err := os.ReadFile(r.live + ".pre-restore-wal")
	require.NoError(t, err)
	assert.Equal(t, "stale wal", string(kept), "the old -wal is kept beside the pre-restore copy")
}

func TestApplyKeepsAndRemovesAHotJournal(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite", "new-1")
	require.NoError(t, Stage(context.Background(), r.live, path, "export-1", false))
	require.NoError(t, os.WriteFile(r.live+"-journal", []byte("hot journal"), 0o600))

	require.True(t, ApplyPending(r.live, testLogger(t)))

	assert.False(t, exists(r.live+"-journal"), "a hot journal would be replayed into the restored file")
	assert.Equal(t, []string{"new-1"}, readNames(t, r.live))
	kept, err := os.ReadFile(r.live + ".pre-restore-journal")
	require.NoError(t, err)
	assert.Equal(t, "hot journal", string(kept), "the old journal stays with the pre-restore copy")

	require.True(t, Rollback(r.live, errors.New("migrate failed"), testLogger(t)))
	back, err := os.ReadFile(r.live + "-journal")
	require.NoError(t, err)
	assert.Equal(t, "hot journal", string(back), "a rollback puts the journal back with the old database")
}

func TestFailedApplyAfterTheSideFilesWentGivesThemBack(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite", "new-1")
	require.NoError(t, Stage(context.Background(), r.live, path, "export-1", false))
	require.NoError(t, os.WriteFile(r.live+"-wal", []byte("committed frames"), 0o600))
	// The -shm is removed after the -wal and cannot be: a directory with a file in it.
	require.NoError(t, os.MkdirAll(filepath.Join(r.live+"-shm", "x"), 0o755))

	require.False(t, ApplyPending(r.live, testLogger(t)))

	wal, err := os.ReadFile(r.live + "-wal")
	require.NoError(t, err, "the -wal was removed before the failure and must be back")
	assert.Equal(t, "committed frames", string(wal))
	assert.Equal(t, []string{"old-1", "old-2"}, readNames(t, r.live), "the old database is untouched")
}

func TestFailedApplyDoesNotLeaveAnOlderResultStanding(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite", "new-1")
	require.NoError(t, Stage(context.Background(), r.live, path, "export-1", false))
	require.True(t, ApplyPending(r.live, testLogger(t)))
	res, err := LastResult(r.live)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, StatusApplied, res.Status)

	// The next apply fails (tampered staged file) and the result cannot be written.
	require.NoError(t, Stage(context.Background(), r.live, path, "export-2", false))
	require.NoError(t, os.WriteFile(filepath.Join(Dir(r.live), "staged.sqlite"), []byte("changed"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(Dir(r.live), "result.json.tmp"), 0o755))
	require.False(t, ApplyPending(r.live, testLogger(t)))

	res, err = LastResult(r.live)
	require.NoError(t, err)
	assert.Nil(t, res, "an old applied result must not stand for a start that failed")
}

func TestOnePreRestoreCopyAndOnePendingRestore(t *testing.T) {
	r := newRig(t)
	a := r.backup(t, "a.sqlite", "a-1")
	b := r.backup(t, "b.sqlite", "b-1")

	err := Stage(context.Background(), r.live, a, "export-a", false)
	require.NoError(t, err)
	err = Stage(context.Background(), r.live, b, "export-b", false)
	require.NoError(t, err)
	p, err := Pending(r.live)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "export-b", p.BackupID, "a new stage replaces the pending one")

	require.True(t, ApplyPending(r.live, testLogger(t)))
	assert.Equal(t, []string{"b-1"}, readNames(t, r.live))

	err = Stage(context.Background(), r.live, a, "export-a", false)
	require.NoError(t, err)
	require.True(t, ApplyPending(r.live, testLogger(t)))
	assert.Equal(t, []string{"a-1"}, readNames(t, r.live))
	assert.Equal(t, []string{"b-1"}, readNames(t, r.live+".pre-restore"), "the second apply replaced the first copy")

	err = Stage(context.Background(), r.live, b, "export-b", false)
	require.NoError(t, err)
	cancelled, err := Cancel(r.live)
	require.NoError(t, err)
	assert.True(t, cancelled)
	p, err = Pending(r.live)
	require.NoError(t, err)
	assert.Nil(t, p)
	assert.False(t, exists(filepath.Join(Dir(r.live), "staged.sqlite")))
	cancelled, err = Cancel(r.live)
	require.NoError(t, err)
	assert.False(t, cancelled, "nothing pending, nothing cancelled")
	assert.False(t, ApplyPending(r.live, testLogger(t)), "no pending restore is a no-op")
}

func TestStageIsCappedByTheFreeSpace(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite.gz", "new-1")

	old := freeBytes
	t.Cleanup(func() { freeBytes = old })
	freeBytes = func(string) (uint64, error) { return 8 * 1024, nil } // less than the live file plus the backup

	err := Stage(context.Background(), r.live, path, "export-1", false)
	var ce *CheckError
	require.True(t, errors.As(err, &ce), "want a CheckError, got %v", err)
	assert.Contains(t, ce.Reason, "free")
	assert.False(t, exists(filepath.Join(Dir(r.live), "staged.sqlite")))
	assert.False(t, exists(filepath.Join(Dir(r.live), "staged.sqlite.tmp")))
}

func TestRestagingDoesNotCountTheStagedFileAsFree(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite", "new-1")
	require.NoError(t, Stage(context.Background(), r.live, path, "export-1", false))
	fi, err := os.Stat(filepath.Join(Dir(r.live), "staged.sqlite"))
	require.NoError(t, err)

	// The first staged file stays on disk while the second is unpacked, so it is not free space.
	old := freeBytes
	t.Cleanup(func() { freeBytes = old })
	freeBytes = func(string) (uint64, error) { return uint64(liveSize(r.live)) + uint64(fi.Size())/2, nil }

	err = Stage(context.Background(), r.live, path, "export-2", false)
	var ce *CheckError
	require.True(t, errors.As(err, &ce), "want a CheckError, got %v", err)
	p, perr := Pending(r.live)
	require.NoError(t, perr)
	require.NotNil(t, p)
	assert.Equal(t, "export-1", p.BackupID, "a refused restage keeps the pending restore")
}

func TestCancelThatCannotRemoveTheRestoreSaysSo(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite", "new-1")
	require.NoError(t, Stage(context.Background(), r.live, path, "export-1", false))

	// The marker cannot be removed: swap it for a directory with a file in it.
	marker := filepath.Join(Dir(r.live), "pending.json")
	require.NoError(t, os.Remove(marker))
	require.NoError(t, os.MkdirAll(filepath.Join(marker, "x"), 0o755))

	cancelled, err := Cancel(r.live)
	require.Error(t, err)
	assert.False(t, cancelled, "a restore that is still there is not reported as cancelled")
}

func TestFailedRestageLeavesNothingPending(t *testing.T) {
	r := newRig(t)
	a := r.backup(t, "a.sqlite", "a-1")
	b := r.backup(t, "b.sqlite", "b-1")
	require.NoError(t, Stage(context.Background(), r.live, a, "export-a", false))

	// The marker cannot be written: its temporary name is taken by a directory.
	require.NoError(t, os.Mkdir(filepath.Join(Dir(r.live), "pending.json.tmp"), 0o755))
	err := Stage(context.Background(), r.live, b, "export-b", false)
	require.Error(t, err)

	p, perr := Pending(r.live)
	require.NoError(t, perr)
	assert.Nil(t, p, "the old marker must not point at the bytes the failed stage replaced")
	assert.False(t, exists(filepath.Join(Dir(r.live), "staged.sqlite")))
}

func TestRollbackPutsThePreRestoreCopyBack(t *testing.T) {
	r := newRig(t)
	path := r.backup(t, "b.sqlite", "new-1")
	err := Stage(context.Background(), r.live, path, "export-1", false)
	require.NoError(t, err)
	require.True(t, ApplyPending(r.live, testLogger(t)))
	require.Equal(t, []string{"new-1"}, readNames(t, r.live))

	require.True(t, Rollback(r.live, errors.New("migrate failed"), testLogger(t)))

	assert.Equal(t, []string{"old-1", "old-2"}, readNames(t, r.live))
	assert.False(t, exists(r.live+".pre-restore"), "the copy is moved into place, which needs no free space")
	assert.False(t, exists(r.live+".rollback.tmp"))
	res, err := LastResult(r.live)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, StatusFailed, res.Status)
	assert.Equal(t, "export-1", res.BackupID)
	assert.NotContains(t, res.Error, "migrate failed", "the cause goes to the log, not to the marker")

	assert.False(t, Rollback(filepath.Join(t.TempDir(), "none.db"), errors.New("x"), testLogger(t)), "no pre-restore copy, nothing to put back")
}

func TestLivePath(t *testing.T) {
	cases := map[string]struct {
		dsn  string
		want string
		ok   bool
	}{
		"plain path":            {"data/shelly.db", "data/shelly.db", true},
		"path with parameters":  {"/data/shelly.db?_pragma=busy_timeout(5000)", "/data/shelly.db", true},
		"memory":                {":memory:", "", false},
		"file uri":              {"file:/data/shelly.db", "", false},
		"empty":                 {"", "", false},
		"parameters only":       {"?cache=shared", "", false},
		"question mark in name": {"/data/a?b.db", "/data/a", true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := LivePath(c.dsn)
			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

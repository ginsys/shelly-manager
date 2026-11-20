// Package restore replaces a SQLite database with a stored backup in two steps. Stage checks a copy
// of the backup beside the live file while the server runs and records that a restore is pending;
// ApplyPending swaps the checked copy in at the next start, before the database is opened. The live
// file is never replaced under a running server, because services keep the handle they took at
// start and nothing holds their writers back.
package restore

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/glebarez/sqlite"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Status values of a Result.
const (
	StatusApplied = "applied"
	StatusFailed  = "failed"
)

const (
	stagedName    = "staged.sqlite"
	pendingName   = "pending.json"
	resultName    = "result.json"
	requiredTable = "devices" // the one table a backup from any earlier version has; AutoMigrate adds the rest
	sqliteHeader  = "SQLite format 3\x00"
)

// mu serialises Stage, Cancel and ApplyPending: they all work on one directory.
var mu sync.Mutex

// freeBytes reports the space available to an unprivileged writer in dir. A variable, so a test can
// shrink it without filling a disk.
var freeBytes = func(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil //nolint:gosec // block counts are not negative
}

// CheckError is a backup that cannot be restored. Its Reason is safe to show to a client: it names
// no server path.
type CheckError struct{ Reason string }

func (e *CheckError) Error() string { return e.Reason }

func refuse(format string, args ...any) error {
	return &CheckError{Reason: fmt.Sprintf(format, args...)}
}

// PendingInfo describes a staged restore.
type PendingInfo struct {
	BackupID string    `json:"backup_id"`
	StagedAt time.Time `json:"staged_at"`
	SHA256   string    `json:"sha256"`
}

// Result is what the last apply did. Its error is a fixed sentence and names no path: the detail
// goes to the log.
type Result struct {
	Status   string    `json:"status"`
	BackupID string    `json:"backup_id,omitempty"`
	At       time.Time `json:"at"`
	Error    string    `json:"error,omitempty"`
}

// Dir is the directory that holds the staged file and the markers. It sits beside the live file, so
// the final rename stays on one filesystem.
func Dir(live string) string { return live + ".restore" }

// LivePath returns the database file a SQLite DSN names, or false when the DSN is not a plain file
// path (in-memory, a file: URI, empty). The path is the part before "?", as the provider reads it.
func LivePath(dsn string) (string, bool) {
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return "", false
	}
	path, _, _ := strings.Cut(dsn, "?")
	if path == "" {
		return "", false
	}
	return path, true
}

// Stage checks the backup at backupPath and, unless dryRun is set, stages it to replace the live
// database at the next start. A staged restore that already exists is replaced. Every check runs on
// a copy in the restore directory; a failure removes the copy and leaves the live file and any
// pending restore as they were. The copy is capped by the free space, after reserving what the apply
// will need for the copy of the live file.
func Stage(ctx context.Context, live, backupPath, backupID string, dryRun bool) error {
	mu.Lock()
	defer mu.Unlock()

	dir := Dir(live)
	staged := filepath.Join(dir, stagedName)
	free, err := freeBytes(filepath.Dir(live))
	if err != nil {
		return fmt.Errorf("read the free space: %w", err)
	}
	if fi, statErr := os.Stat(staged); statErr == nil {
		free += uint64(fi.Size()) //nolint:gosec // a file size is not negative
	}
	reserve := liveSize(live)
	need := reserve + free/20
	if free <= need {
		return refuse("There is not enough free space beside the database: about %s are needed for the previous database, and %s are free", human(reserve), human(free))
	}
	budget := free - need

	dirExisted := exists(dir)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create the restore directory: %w", err)
	}
	tmp := filepath.Join(dir, stagedName+".tmp")
	defer func() {
		_ = os.Remove(tmp)
		if dryRun && !dirExisted {
			_ = os.Remove(dir) // only succeeds while empty
		}
	}()

	if err = expand(backupPath, tmp, budget); err != nil {
		return err
	}
	if err = checkFile(ctx, tmp); err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	sum, err := fileSHA(tmp)
	if err != nil {
		return fmt.Errorf("hash the staged file: %w", err)
	}
	if err = os.Rename(tmp, staged); err != nil {
		return fmt.Errorf("stage the file: %w", err)
	}
	info := PendingInfo{BackupID: backupID, StagedAt: time.Now().UTC(), SHA256: sum}
	if err = writeJSON(filepath.Join(dir, pendingName), info); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("record the pending restore: %w", err)
	}
	return nil
}

// Pending returns the staged restore, or nil when there is none.
func Pending(live string) (*PendingInfo, error) {
	mu.Lock()
	defer mu.Unlock()
	return readPending(Dir(live))
}

// LastResult returns what the last apply did, or nil when none has run.
func LastResult(live string) (*Result, error) {
	mu.Lock()
	defer mu.Unlock()
	var res Result
	found, err := readJSON(filepath.Join(Dir(live), resultName), &res)
	if err != nil || !found {
		return nil, err
	}
	return &res, nil
}

// Cancel removes a pending restore, an unreadable marker included. It reports whether there was one.
func Cancel(live string) bool {
	mu.Lock()
	defer mu.Unlock()
	dir := Dir(live)
	p, err := readPending(dir)
	clearPending(dir)
	return p != nil || err != nil
}

// ApplyPending swaps a staged restore into place. Call it before the database is opened. It reports
// whether the swap happened; a failure keeps the live file, records a failed Result and clears the
// pending restore, so the same failure is not retried at every start.
func ApplyPending(live string, logger *logging.Logger) bool {
	mu.Lock()
	defer mu.Unlock()

	dir := Dir(live)
	p, err := readPending(dir)
	if err != nil {
		logger.Error("Restore not applied: the pending marker is unreadable", "error", err)
		recordResult(dir, Result{Status: StatusFailed, Error: "The pending restore marker could not be read"})
		clearPending(dir)
		return false
	}
	if p == nil {
		return false
	}
	staged := filepath.Join(dir, stagedName)
	fail := func(msg string, cause error) bool {
		logger.Error("Restore not applied", "backup_id", p.BackupID, "reason", msg, "error", cause)
		recordResult(dir, Result{Status: StatusFailed, BackupID: p.BackupID, Error: msg})
		clearPending(dir)
		return false
	}

	sum, err := fileSHA(staged)
	if err != nil {
		return fail("The staged file is missing; an earlier apply may have completed", err)
	}
	if sum != p.SHA256 {
		return fail("The staged file changed after it was checked", nil)
	}
	if err := keepPreRestore(live); err != nil {
		return fail("The live database could not be copied, so nothing was replaced", err)
	}
	for _, side := range []string{"-wal", "-shm"} {
		if err := os.Remove(live + side); err != nil && !os.IsNotExist(err) {
			return fail("A stale write-ahead file could not be removed, so nothing was replaced", err)
		}
	}
	if fi, statErr := os.Stat(live); statErr == nil {
		_ = os.Chmod(staged, fi.Mode().Perm())
	}
	if err := os.Rename(staged, live); err != nil {
		return fail("The staged file could not be moved into place", err)
	}
	recordResult(dir, Result{Status: StatusApplied, BackupID: p.BackupID})
	_ = os.Remove(filepath.Join(dir, pendingName))
	logger.Info("Restore applied", "backup_id", p.BackupID)
	return true
}

// Rollback puts the pre-restore copy back after the restored database could not be opened or
// migrated. It reports whether a copy was there to put back. The cause goes to the log only.
func Rollback(live string, cause error, logger *logging.Logger) bool {
	mu.Lock()
	defer mu.Unlock()

	pre := live + ".pre-restore"
	if !exists(pre) {
		return false
	}
	dir := Dir(live)
	var backupID string
	var last Result
	if found, _ := readJSON(filepath.Join(dir, resultName), &last); found {
		backupID = last.BackupID
	}
	msg := "The restored database could not be opened or migrated; the previous database was put back"
	for _, side := range []string{"-wal", "-shm"} {
		_ = os.Remove(live + side)
	}
	tmp := live + ".rollback.tmp"
	if err := copyFile(pre, tmp); err != nil {
		_ = os.Remove(tmp)
		logger.Error("Restore rollback failed", "error", err, "cause", cause)
		return false
	}
	if err := os.Rename(tmp, live); err != nil {
		_ = os.Remove(tmp)
		logger.Error("Restore rollback failed", "error", err, "cause", cause)
		return false
	}
	if exists(pre + "-wal") {
		if err := copyFile(pre+"-wal", live+"-wal"); err != nil {
			logger.Error("Restore rollback could not put the write-ahead file back", "error", err)
		}
	}
	recordResult(dir, Result{Status: StatusFailed, BackupID: backupID, Error: msg})
	logger.Error("Restored database could not be opened; the previous database was put back", "backup_id", backupID, "error", cause)
	return true
}

// keepPreRestore keeps one copy of the live file, and of its -wal when it has one, beside it. The
// previous copy is removed first, so the copy needs no more room than the live file.
func keepPreRestore(live string) error {
	if !exists(live) {
		return nil
	}
	pre := live + ".pre-restore"
	for _, old := range []string{pre, pre + "-wal"} {
		if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	tmp := pre + ".tmp"
	if err := copyFile(live, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, pre); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if exists(live + "-wal") {
		if err := copyFile(live+"-wal", pre+"-wal"); err != nil {
			return err
		}
	}
	return nil
}

// expand writes the backup's database bytes to dst: a .gz or a .zip with one file is unpacked, any
// other name is copied as it is. It stops at budget bytes, so a small archive cannot fill the disk.
func expand(src, dst string, budget uint64) error {
	var in io.Reader
	switch strings.ToLower(filepath.Ext(src)) {
	case ".gz":
		f, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("open the backup: %w", err)
		}
		defer func() { _ = f.Close() }()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return refuse("The backup is not a valid gzip file")
		}
		defer func() { _ = gz.Close() }()
		in = gz
	case ".zip":
		zr, err := zip.OpenReader(src)
		if err != nil {
			return refuse("The backup is not a valid zip file")
		}
		defer func() { _ = zr.Close() }()
		if len(zr.File) != 1 || zr.File[0].FileInfo().IsDir() {
			return refuse("A zip backup must hold exactly one file")
		}
		rc, err := zr.File[0].Open()
		if err != nil {
			return refuse("The backup is not a valid zip file")
		}
		defer func() { _ = rc.Close() }()
		in = rc
	default:
		f, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("open the backup: %w", err)
		}
		defer func() { _ = f.Close() }()
		in = f
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create the staged copy: %w", err)
	}
	cr := &readErrors{r: io.LimitReader(in, int64(budget)+1)} //nolint:gosec // a budget above the int64 range is not a real disk
	n, copyErr := io.Copy(out, cr)
	if cr.err != nil {
		_ = out.Close()
		return refuse("The backup file is damaged or not a valid archive")
	}
	if copyErr != nil {
		_ = out.Close()
		return fmt.Errorf("write the staged copy: %w", copyErr)
	}
	if uint64(n) > budget { //nolint:gosec // n is not negative
		_ = out.Close()
		return refuse("The backup is larger than the free space beside the database allows (about %s)", human(budget))
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return fmt.Errorf("sync the staged copy: %w", err)
	}
	return out.Close()
}

// readErrors remembers an error from the source, so a damaged archive can be told from a full disk.
type readErrors struct {
	r   io.Reader
	err error
}

func (c *readErrors) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		c.err = err
	}
	return n, err
}

// checkFile runs the checks a restorable database passes: the SQLite header, a clean
// integrity_check, and the devices table. The file is opened read-only and immutable, so a backup
// whose header says WAL cannot make SQLite create -wal or -shm files beside it.
func checkFile(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open the staged copy: %w", err)
	}
	head := make([]byte, len(sqliteHeader))
	_, readErr := io.ReadFull(f, head)
	_ = f.Close()
	if readErr != nil || string(head) != sqliteHeader {
		return refuse("The file is not a SQLite database")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro&immutable=1"}
	db, err := sql.Open(sqlite.DriverName, u.String())
	if err != nil {
		return fmt.Errorf("open the staged copy: %w", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return refuse("The database is damaged and cannot be read")
	}
	var verdicts []string
	for rows.Next() {
		var v string
		if scanErr := rows.Scan(&v); scanErr != nil {
			_ = rows.Close()
			return refuse("The database is damaged and cannot be read")
		}
		verdicts = append(verdicts, v)
	}
	iterErr := rows.Err()
	_ = rows.Close()
	if iterErr != nil || len(verdicts) != 1 || verdicts[0] != "ok" {
		return refuse("The database failed its integrity check")
	}

	var n int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", requiredTable).Scan(&n); err != nil {
		return refuse("The database is damaged and cannot be read")
	}
	if n == 0 {
		return refuse("The database has no %s table, so it is not a Shelly Manager database", requiredTable)
	}
	return nil
}

func liveSize(live string) uint64 {
	var total uint64
	for _, p := range []string{live, live + "-wal"} {
		if fi, err := os.Stat(p); err == nil {
			total += uint64(fi.Size()) //nolint:gosec // a file size is not negative
		}
	}
	return total
}

func copyFile(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readPending(dir string) (*PendingInfo, error) {
	var p PendingInfo
	found, err := readJSON(filepath.Join(dir, pendingName), &p)
	if err != nil || !found {
		return nil, err
	}
	return &p, nil
}

func clearPending(dir string) {
	_ = os.Remove(filepath.Join(dir, pendingName))
	_ = os.Remove(filepath.Join(dir, stagedName))
}

func recordResult(dir string, res Result) {
	res.At = time.Now().UTC()
	_ = writeJSON(filepath.Join(dir, resultName), res)
}

func readJSON(path string, v any) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return true, nil
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func human(n uint64) string {
	const mib = 1024 * 1024
	if n < mib {
		return fmt.Sprintf("%d KiB", n/1024)
	}
	return fmt.Sprintf("%d MiB", n/mib)
}

package backup

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// backupFormats are the three files a backup can be: the provider picks the format by the file's extension.
var backupFormats = []string{".sqlite", ".sqlite.gz", ".sqlite.zip"}

func liveProvider(t *testing.T) (*BackupPlugin, provider.BackupProvider, string) {
	t.Helper()
	plugin, livePath, _ := liveBackup(t)
	return plugin, plugin.dbManager.GetProvider().(provider.BackupProvider), filepath.Dir(livePath)
}

// ginsys/shelly-manager#815: the path check (ResolveUnderRoot) can pass and a link be swapped in before the
// write. The backup file is created and written through the opened backup root, so a link that leads out of
// it is refused, and nothing is created outside: neither the file nor a snapshot of the database.
func TestBackupRefusesALinkSwappedInAfterTheCheck(t *testing.T) {
	for _, swapped := range []string{"the output directory", "a directory below it"} {
		{ // one block per case
			t.Run(swapped+" swapped for a link", func(t *testing.T) {
				plugin, bp, _ := liveProvider(t)
				base := t.TempDir()
				rootDir := filepath.Join(base, "root")
				outside := filepath.Join(base, "outside")
				for _, d := range []string{rootDir, outside} {
					if err := os.MkdirAll(d, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				plugin.SetExportBaseDir(rootDir)

				requested := "export"
				if swapped == "a directory below it" {
					requested = filepath.Join("export", "below")
					if err := os.MkdirAll(filepath.Join(rootDir, "export"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				outputPath, err := plugin.resolveOutputDir(requested) // passes: nothing is there yet
				if err != nil {
					t.Fatal(err)
				}
				if err = os.Symlink(outside, outputPath); err != nil {
					t.Skipf("symlink: %v", err)
				}
				root, rel, err := sync.OpenOutputRoot(plugin.backupRoot(), outputPath)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = root.Close() }()

				name := "backup.sqlite"
				cfg := provider.BackupConfig{BackupPath: filepath.Join(outputPath, name), BackupType: provider.BackupTypeFull}
				if _, err = plugin.writeBackup(context.Background(), bp, root, rel, name, cfg); err == nil {
					t.Error("the backup was written through a link that leads out of the backup root")
				}
				if entries, _ := os.ReadDir(outside); len(entries) > 0 {
					t.Errorf("%d entries were created outside the backup root", len(entries))
				}
			})
		}
	}
}

// The cases above fail at the directory, before any file is written. This one reaches the write: rel is a
// real directory inside the root, and the path the backup reports is a link that leads out of it. The file
// must land below rel, never at the reported path, and the result must describe that file. A write built
// from the path by name would put the file, or the snapshot, outside.
func TestBackupWritesBelowTheRootDirectoryNotAtTheReportedPath(t *testing.T) {
	for _, ext := range backupFormats {
		t.Run(ext, func(t *testing.T) {
			plugin, bp, liveDir := liveProvider(t)
			base := t.TempDir()
			rootDir := filepath.Join(base, "root")
			outside := filepath.Join(base, "outside")
			for _, d := range []string{filepath.Join(rootDir, "export"), outside} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			reported := filepath.Join(rootDir, "reported")
			if err := os.Symlink(outside, reported); err != nil {
				t.Skipf("symlink: %v", err)
			}
			root, err := os.OpenRoot(rootDir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()

			name := "backup" + ext
			cfg := provider.BackupConfig{BackupPath: filepath.Join(reported, name), BackupType: provider.BackupTypeFull}
			res, err := plugin.writeBackup(context.Background(), bp, root, "export", name, cfg)
			if err != nil || !res.Success {
				t.Fatalf("writeBackup: %v %+v", err, res)
			}
			if entries, _ := os.ReadDir(outside); len(entries) > 0 {
				t.Errorf("%d entries were created outside the backup root", len(entries))
			}
			if found, _ := filepath.Glob(filepath.Join(liveDir, ".backup-*")); len(found) > 0 {
				t.Errorf("a snapshot directory was left beside the live database: %v", found)
			}
			written, err := os.ReadFile(filepath.Join(rootDir, "export", name))
			if err != nil || len(written) == 0 {
				t.Fatalf("the backup is not below the root directory (%d bytes, err %v)", len(written), err)
			}
			if res.Size != int64(len(written)) || res.Checksum != fmt.Sprintf("%x", sha256.Sum256(written)) {
				t.Errorf("result size %d checksum %s; the file has %d bytes, sha256 %x", res.Size, res.Checksum, len(written), sha256.Sum256(written))
			}
		})
	}
}

// noResultProvider returns neither a result nor an error, which the interface allows.
type noResultProvider struct{ MockBackupProvider }

func (noResultProvider) CreateBackup(context.Context, provider.BackupConfig) (*provider.BackupResult, error) {
	return nil, nil
}

// A backup that fails leaves no file behind: the plugin creates the file before the provider runs, so it
// removes it, through the root, when the provider fails, reports no success or returns no result.
func TestBackupRemovesTheFileItCreatedWhenTheProviderFails(t *testing.T) {
	for name, mock := range map[string]provider.BackupProvider{
		"an error":      &MockBackupProvider{createBackupError: errors.New("snapshot failed")},
		"no success":    &MockBackupProvider{createBackupResult: &provider.BackupResult{Success: false, Error: "refused"}},
		"no result":     &noResultProvider{},
		"a good result": &MockBackupProvider{},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			_, err = (&BackupPlugin{}).writeBackup(context.Background(), mock, root, "export", "backup.sqlite", provider.BackupConfig{})
			_, statErr := os.Stat(filepath.Join(dir, "export", "backup.sqlite"))
			if name == "a good result" {
				if err != nil || statErr != nil {
					t.Errorf("a good backup: err %v, stat %v; want the file kept", err, statErr)
				}
				return
			}
			if !os.IsNotExist(statErr) {
				t.Errorf("a failed backup left its file behind (stat err %v)", statErr)
			}
		})
	}
}

// Export itself: the file lands below the configured backup root, and the result's path, size and checksum
// describe that file, for the plain, gzip and zip backups.
func TestExportWritesBelowTheBackupRootAndReportsTheFile(t *testing.T) {
	configs := map[string]map[string]interface{}{
		"plain": {},
		"gzip":  {"compression": true},
		"zip":   {"compression": true, "compression_algo": "zip"},
	}
	for name, cfg := range configs {
		t.Run(name, func(t *testing.T) {
			plugin, _, _ := liveProvider(t)
			rootDir := filepath.Join(t.TempDir(), "root")
			plugin.SetExportBaseDir(rootDir)
			cfg["output_path"] = "nightly"

			res, err := plugin.Export(context.Background(), &sync.ExportData{Timestamp: time.Now()}, sync.ExportConfig{Format: "sqlite", Config: cfg})
			if err != nil || !res.Success {
				t.Fatalf("Export: %v %+v", err, res)
			}
			wantDir := filepath.Join(rootDir, "nightly")
			if filepath.Dir(res.OutputPath) != wantDir {
				t.Fatalf("OutputPath = %q, want a file in %q", res.OutputPath, wantDir)
			}
			written, err := os.ReadFile(res.OutputPath)
			if err != nil || len(written) == 0 {
				t.Fatalf("the backup file is missing or empty (%d bytes, err %v)", len(written), err)
			}
			if res.FileSize != int64(len(written)) || res.Checksum != fmt.Sprintf("%x", sha256.Sum256(written)) {
				t.Errorf("result size %d checksum %s; the file has %d bytes, sha256 %x", res.FileSize, res.Checksum, len(written), sha256.Sum256(written))
			}
			if entries, _ := os.ReadDir(wantDir); len(entries) != 1 {
				t.Errorf("%d entries in the backup directory, want only the backup", len(entries))
			}
		})
	}
}

// Export reports the size and checksum the provider counted while writing, not a second reading of the file.
type fixedResultProvider struct {
	*provider.SQLiteProvider
}

func (f *fixedResultProvider) CreateBackup(ctx context.Context, cfg provider.BackupConfig) (*provider.BackupResult, error) {
	res, err := f.SQLiteProvider.CreateBackup(ctx, cfg)
	if res != nil {
		res.Size, res.Checksum = 4242, "provider-checksum"
	}
	return res, err
}

func TestExportReportsTheProvidersSizeAndChecksum(t *testing.T) {
	plugin, _, _ := liveProvider(t)
	plugin.SetExportBaseDir(filepath.Join(t.TempDir(), "root"))
	plugin.dbManager = &MockDatabaseManager{provider: &fixedResultProvider{plugin.dbManager.GetProvider().(*provider.SQLiteProvider)}}

	res, err := plugin.Export(context.Background(), &sync.ExportData{Timestamp: time.Now()},
		sync.ExportConfig{Format: "sqlite", Config: map[string]interface{}{"output_path": "nightly"}})
	if err != nil || !res.Success {
		t.Fatalf("Export: %v %+v", err, res)
	}
	if res.FileSize != 4242 || res.Checksum != "provider-checksum" {
		t.Errorf("FileSize %d, Checksum %q; want the provider's 4242 and provider-checksum", res.FileSize, res.Checksum)
	}
}

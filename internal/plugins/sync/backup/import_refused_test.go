package backup

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// liveBackup builds a real SQLite provider on a temporary database, takes a backup of it, then
// changes a row, so a restore that really ran would replace the file with different bytes.
func liveBackup(t *testing.T) (plugin *BackupPlugin, livePath, backupPath string) {
	t.Helper()
	dir := t.TempDir()
	livePath = filepath.Join(dir, "live.db")
	backupPath = filepath.Join(dir, "backup.sqlite")

	p := provider.NewSQLiteProvider(logging.GetDefault())
	if err := p.Connect(provider.DatabaseConfig{Provider: "sqlite", DSN: livePath}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	db := p.GetDB()
	if err := db.Exec("CREATE TABLE notes (v TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO notes (v) VALUES ('before the backup')").Error; err != nil {
		t.Fatal(err)
	}
	res, err := p.CreateBackup(context.Background(), provider.BackupConfig{BackupPath: backupPath, BackupType: provider.BackupTypeFull})
	if err != nil || !res.Success {
		t.Fatalf("CreateBackup: %v %+v", err, res)
	}
	if err := db.Exec("INSERT INTO notes (v) VALUES ('after the backup')").Error; err != nil {
		t.Fatal(err)
	}

	plugin = NewBackupExporter(&MockDatabaseManager{provider: p})
	if err := plugin.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return plugin, livePath, backupPath
}

func fileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

// The generic import (POST /import) and its preview (POST /import/preview) both reach
// BackupPlugin.Import with a client-named path. The provider's restore replaces the live database
// and ignores dry run, so the plugin must refuse every source type until a safe restore exists.
func TestBackupImportNeverRestores(t *testing.T) {
	// The request shapes of the two routes: the preview sets Options.DryRun, which the plugin
	// never sees, and may or may not carry config.dry_run.
	cases := map[string]map[string]interface{}{
		"import":                  {},
		"preview without dry_run": {},
		"preview with dry_run":    {"dry_run": true},
	}
	for name, cfg := range cases {
		for _, srcType := range []string{"file", "data", "url", "other"} {
			t.Run(name+"/"+srcType, func(t *testing.T) {
				plugin, livePath, backupPath := liveBackup(t)
				before := fileDigest(t, livePath)

				_, err := plugin.Import(context.Background(),
					sync.ImportSource{Type: srcType, Path: backupPath},
					sync.ImportConfig{Config: cfg})

				if !errors.Is(err, sync.ErrImportNotImplemented) {
					t.Errorf("want ErrImportNotImplemented, got %v", err)
				}
				if fileDigest(t, livePath) != before {
					t.Error("the live database file changed: the import restored the backup")
				}
			})
		}
	}
}

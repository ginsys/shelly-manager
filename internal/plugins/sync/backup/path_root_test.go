package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/sync"
)

// exportTo runs the plugin's Export with one output_path and returns what it reports.
func exportTo(t *testing.T, plugin *BackupPlugin, outputPath string) (*sync.ExportResult, error) {
	t.Helper()
	cfg := map[string]interface{}{"compression": false}
	if outputPath != "" {
		cfg["output_path"] = outputPath
	}
	return plugin.Export(context.Background(), &sync.ExportData{Timestamp: time.Now()},
		sync.ExportConfig{Format: "sqlite", Config: cfg})
}

// mustNotExist fails when anything matches the glob pattern: the refused backup left a trace.
func mustNotExist(t *testing.T, pattern string) {
	t.Helper()
	if found, _ := filepath.Glob(pattern); len(found) > 0 {
		t.Errorf("%v exists: the refused backup was written", found)
	}
}

// With sync.export_base_dir set (the engine hands it over through SetExportBaseDir) that directory
// is the backup root: a relative output_path joins under it and an absolute one must lie inside it.
func TestBackupOutputPathUnderConfiguredRoot(t *testing.T) {
	plugin, _, _ := liveBackup(t)
	work := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	base := filepath.Join(t.TempDir(), "exports")
	plugin.SetExportBaseDir(base)

	res, err := exportTo(t, plugin, "nightly")
	if err != nil || !res.Success {
		t.Fatalf("Export(nightly): %v %+v", err, res)
	}
	if want := filepath.Join(base, "nightly"); filepath.Dir(res.OutputPath) != want {
		t.Errorf("backup written to %s, want it under %s", res.OutputPath, want)
	}
	mustNotExist(t, filepath.Join(work, "nightly"))

	for _, outputPath := range []string{filepath.Join(t.TempDir(), "x"), "../x", "./data/backups"} {
		err := plugin.ValidateConfig(map[string]interface{}{"output_path": outputPath})
		if outputPath == "./data/backups" {
			// relative, so it joins under the root: accepted, and it lands in base/data/backups
			if err != nil {
				t.Errorf("ValidateConfig(%q): %v", outputPath, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), base) {
			t.Errorf("ValidateConfig(%q) = %v, want a refusal naming %s", outputPath, err, base)
		}
	}
}

// A backup is written only inside the backup root. With no configured root that is ./data/backups
// under the working directory, and it does not exist yet on a fresh install.
func TestBackupOutputPathStaysInsideRoot(t *testing.T) {
	plugin, _, _ := liveBackup(t)

	work := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	outside := t.TempDir()
	root := filepath.Join(work, "data", "backups")

	accepted := map[string]string{
		"empty path, fresh install": "",
		"the default":               "./data/backups",
		"a sub-directory":           "data/backups/nightly",
		"absolute, inside":          filepath.Join(root, "abs"),
	}
	for name, outputPath := range accepted {
		t.Run("accepts "+name, func(t *testing.T) {
			if err := plugin.ValidateConfig(map[string]interface{}{"output_path": outputPath}); err != nil {
				t.Fatalf("ValidateConfig(%q): %v", outputPath, err)
			}
			res, err := exportTo(t, plugin, outputPath)
			if err != nil || !res.Success {
				t.Fatalf("Export(%q): %v %+v", outputPath, err, res)
			}
			written, _ := filepath.Abs(res.OutputPath)
			if rel, err := filepath.Rel(root, written); err != nil || !filepath.IsLocal(rel) {
				t.Errorf("backup written to %s, outside %s", written, root)
			}
			if _, err := os.Stat(written); err != nil {
				t.Errorf("backup file missing: %v", err)
			}
		})
	}

	refused := map[string]struct{ outputPath, made string }{
		"absolute, outside":    {filepath.Join(outside, "x"), filepath.Join(outside, "x")},
		"parent of the work":   {"../x", filepath.Join(filepath.Dir(work), "x")},
		"back out of the root": {"data/backups/../../x", filepath.Join(work, "x")},
		"sibling of the root":  {"data/backups-evil", filepath.Join(work, "data", "backups-evil")},
		"parent of the root":   {"data", filepath.Join(work, "data", "shelly-backup-*")},
	}
	for name, c := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			err := plugin.ValidateConfig(map[string]interface{}{"output_path": c.outputPath})
			if err == nil || !strings.Contains(err.Error(), "backup directory") {
				t.Fatalf("ValidateConfig(%q) = %v, want a refusal naming the backup directory", c.outputPath, err)
			}
			if res, err := exportTo(t, plugin, c.outputPath); err == nil {
				t.Fatalf("Export(%q) succeeded: %+v", c.outputPath, res)
			}
			mustNotExist(t, c.made)
		})
	}
}

// The shared check follows symbolic links: a directory inside the backup root that leads outside it
// cannot be written through, and nothing is created outside.
func TestBackupOutputPathRefusesASymbolicLinkOutOfTheRoot(t *testing.T) {
	plugin, _, _ := liveBackup(t)
	work := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	root := filepath.Join(work, "data", "backups")
	outside := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	for _, outputPath := range []string{"data/backups/link", "data/backups/link/new/sub"} {
		err := plugin.ValidateConfig(map[string]interface{}{"output_path": outputPath})
		if err == nil || !strings.Contains(err.Error(), "backup directory") {
			t.Errorf("ValidateConfig(%q) = %v, want a refusal naming the backup directory", outputPath, err)
		}
		if res, err := exportTo(t, plugin, outputPath); err == nil {
			t.Errorf("Export(%q) succeeded: %+v", outputPath, res)
		}
	}
	if entries, _ := os.ReadDir(outside); len(entries) > 0 {
		t.Errorf("%d entries were created outside the backup directory", len(entries))
	}
}

// With no root configured the backup root is ./data/backups, never the export root, and the first
// request creates it.
func TestBackupRootWithNoKeyIsTheBackupDirectoryAndIsCreated(t *testing.T) {
	plugin, _, _ := liveBackup(t)
	work := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)

	dir, err := plugin.resolveOutputDir("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(work, "data", "backups")
	realDir, _ := filepath.EvalSymlinks(dir)
	realWant, _ := filepath.EvalSymlinks(want)
	if realDir != realWant {
		t.Errorf("root = %s, want %s", dir, want)
	}
	if fi, err := os.Stat(want); err != nil || !fi.IsDir() {
		t.Errorf("the backup root was not created: %v", err)
	}
	mustNotExist(t, filepath.Join(work, "data", "exports"))
}

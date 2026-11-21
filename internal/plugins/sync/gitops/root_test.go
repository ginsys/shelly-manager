package gitops

import (
	"os"
	"path/filepath"
)

// newTestExporter returns an exporter whose export root is the file system root, so the tests that write
// to a t.TempDir() exercise the output directory's own rules. The root's own confinement is tested in
// internal/plugins/sync/exportroot.
func newTestExporter() *GitOpsPlugin {
	exporter := NewGitOpsExporter()
	exporter.SetExportBaseDir(string(filepath.Separator))
	return exporter
}

// regularFile is true for a regular file, false for a symlink or a directory.
func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

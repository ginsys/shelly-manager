package gitops

import "path/filepath"

// newTestExporter returns an exporter whose export root is the file system root, so the tests that write
// to a t.TempDir() exercise the output directory's own rules. The root's own confinement is tested in
// internal/plugins/sync/exportroot.
func newTestExporter() *GitOpsPlugin {
	exporter := NewGitOpsExporter()
	exporter.SetExportBaseDir(string(filepath.Separator))
	return exporter
}

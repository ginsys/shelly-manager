package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/sync"
)

// os.ReadDir sorts by name and File.ReadDir does not. The clean removes entries in the order listed, so
// readDirIn has to sort, or a half-finished clean depends on the file system's own order.
func TestReadDirInListsEntriesSortedByName(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"export-summary.yaml", "sub", "b", "a", "z"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	entries, err := readDirIn(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := []string{"a", "b", "export-summary.yaml", "sub", "z"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("entries = %v, want %v", got, want)
	}
}

// The up-front check (resolveOutputDir) can pass and a link be swapped in before the writes. Every write
// goes through the opened export root, so a link that leads out of it is refused and nothing is created
// outside.
func TestGitOpsWritesStayInsideTheRootAfterTheCheck(t *testing.T) {
	base := t.TempDir()
	rootDir := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{rootDir, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	g := NewGitOpsExporter()
	g.SetExportBaseDir(rootDir)

	assertNothingOutside := func(t *testing.T) {
		t.Helper()
		if entries, _ := os.ReadDir(outside); len(entries) > 0 {
			t.Fatalf("%d entries were created outside the export root", len(entries))
		}
	}

	t.Run("the output directory is swapped for a link after the check", func(t *testing.T) {
		outputPath, err := g.resolveOutputDir("swapped") // passes: nothing is there yet
		if err != nil {
			t.Fatal(err)
		}
		if err = os.Symlink(outside, filepath.Join(rootDir, "swapped")); err != nil {
			t.Skipf("symlink: %v", err)
		}
		root, rel, err := sync.OpenOutputRoot(rootDir, outputPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = root.Close() }()
		if err := g.prepareOutputDir(root, rel); err == nil {
			t.Fatal("prepareOutputDir wrote through a link that leads out of the export root")
		}
		assertNothingOutside(t)
	})

	t.Run("a directory below the output directory is swapped for a link", func(t *testing.T) {
		inner := filepath.Join(rootDir, "export")
		if err := os.MkdirAll(inner, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(inner, "groups")); err != nil {
			t.Skipf("symlink: %v", err)
		}
		root, rel, err := sync.OpenOutputRoot(rootDir, inner)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = root.Close() }()
		if err := g.writeYAMLFile(root, filepath.Join(rel, "groups", "group.yaml"), map[string]string{"a": "b"}); err == nil {
			t.Error("writeYAMLFile wrote through a link that leads out of the export root")
		}
		assertNothingOutside(t)
	})
}

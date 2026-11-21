package yamlexport

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/sync"
)

// ginsys/shelly-manager#815: the path check (ResolveUnderRoot) can pass and a link be swapped in before the
// write. The export writes through the opened export root, so a link that leads out of it is refused and
// nothing is created outside, for the plain, gzip and zip writes.
func TestExportRefusesALinkSwappedInAfterTheCheck(t *testing.T) {
	configs := map[string]map[string]interface{}{
		"plain": {},
		"gzip":  {"compression": true},
		"zip":   {"compression": true, "compression_algo": "zip"},
	}
	for name, cfg := range configs {
		for _, swapped := range []string{"the output directory", "a directory below it"} {
			t.Run(name+" with "+swapped+" swapped for a link", func(t *testing.T) {
				base := t.TempDir()
				rootDir := filepath.Join(base, "root")
				outside := filepath.Join(base, "outside")
				for _, d := range []string{rootDir, outside} {
					if err := os.MkdirAll(d, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				p := NewPlugin().(*Plugin)
				p.SetBaseDir(rootDir)

				requested := "export"
				if swapped == "a directory below it" {
					requested = filepath.Join("export", "below")
					if err := os.MkdirAll(filepath.Join(rootDir, "export"), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				outputPath, err := sync.ResolveUnderRoot(rootDir, requested) // passes: nothing is there yet
				if err != nil {
					t.Fatal(err)
				}
				if err = os.Symlink(outside, outputPath); err != nil {
					t.Skipf("symlink: %v", err)
				}
				root, rel, err := sync.OpenOutputRoot(rootDir, outputPath)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = root.Close() }()

				if _, err = p.exportInto(root, rel, outputPath, &sync.ExportData{}, sync.ExportConfig{Config: cfg}, time.Now()); err == nil {
					t.Error("the export wrote through a link that leads out of the export root")
				}
				if entries, _ := os.ReadDir(outside); len(entries) > 0 {
					t.Errorf("%d entries were created outside the export root", len(entries))
				}
			})
		}
	}
}

// The cases above fail at the directory, before any file is written. This one reaches the write: rel is a real
// directory inside the root, and the path the export reports is a link that leads out of it. The file must land
// below rel, never at the reported path, and the size and checksum must come from that file. A write, stat or
// checksum built from the path by name would put the file outside, or read nothing.
func TestExportWritesBelowTheRootDirectoryNotAtTheReportedPath(t *testing.T) {
	for name, cfg := range map[string]map[string]interface{}{
		"plain": {},
		"gzip":  {"compression": true},
		"zip":   {"compression": true, "compression_algo": "zip"},
	} {
		t.Run(name, func(t *testing.T) {
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

			res, err := NewPlugin().(*Plugin).exportInto(root, "export", reported, &sync.ExportData{}, sync.ExportConfig{Config: cfg}, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if entries, _ := os.ReadDir(outside); len(entries) > 0 {
				t.Errorf("%d entries were created outside the export root", len(entries))
			}
			written, _ := os.ReadDir(filepath.Join(rootDir, "export"))
			if len(written) != 1 {
				t.Fatalf("%d files below the root directory, want 1", len(written))
			}
			info, err := written[0].Info()
			if err != nil {
				t.Fatal(err)
			}
			wantSum, err := sync.FileSHA256(root, filepath.Join("export", written[0].Name()))
			if err != nil {
				t.Fatal(err)
			}
			if res.FileSize != info.Size() || res.Checksum != wantSum || res.OutputPath != filepath.Join(reported, written[0].Name()) {
				t.Errorf("result = size %d, checksum %q, path %q; want the written file's size %d, checksum %q and the reported path", res.FileSize, res.Checksum, res.OutputPath, info.Size(), wantSum)
			}
		})
	}
}

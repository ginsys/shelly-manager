package gitops

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

func exportInto(t *testing.T, outputPath string, devices []sync.DeviceData, extra map[string]interface{}) error {
	t.Helper()
	exporter := NewGitOpsExporter()
	if err := exporter.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	cfg := map[string]interface{}{"output_path": outputPath, "group_by": "type"}
	for k, v := range extra {
		cfg[k] = v
	}
	data := &sync.ExportData{
		Devices:  devices,
		Metadata: sync.ExportMetadata{ExportID: "output-dir-test", SystemVersion: "1.0.0", TotalDevices: len(devices)},
	}
	_, err := exporter.Export(context.Background(), data, sync.ExportConfig{Format: "yaml", Config: cfg})
	return err
}

func oneDevice() []sync.DeviceData {
	return []sync.DeviceData{{ID: 1, Name: "Kitchen Switch", Type: "SHSW-1", MAC: "00:11:22:33:44:66"}}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// T1: a directory that is not empty and was not written by an export is never cleaned.
func TestExportRefusesAForeignDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "keep.txt"), "mine")
	writeFile(t, filepath.Join(dir, "nested", "deep.txt"), "mine too")

	err := exportInto(t, dir, oneDevice(), nil)
	if !errors.Is(err, sync.ErrInvalidExportPath) {
		t.Fatalf("want ErrInvalidExportPath, got %v", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("the error should name the path %q, got %q", dir, err)
	}
	for _, p := range []string{"keep.txt", filepath.Join("nested", "deep.txt")} {
		if !exists(filepath.Join(dir, p)) {
			t.Errorf("%s was removed", p)
		}
	}
	if exists(filepath.Join(dir, markerName)) || exists(filepath.Join(dir, "export-summary.yaml")) {
		t.Error("nothing may be written into a refused directory")
	}
}

// T3: an empty, absent, marked or legacy-marked directory is replaced.
func TestExportReplacesItsOwnDirectory(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"empty": func(t *testing.T) string { return t.TempDir() },
		"absent": func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "new", "out")
		},
		"marker": func(t *testing.T) string {
			d := t.TempDir()
			writeFile(t, filepath.Join(d, markerName), "x\n")
			writeFile(t, filepath.Join(d, "stale.yaml"), "old")
			writeFile(t, filepath.Join(d, "groups", "old", "group.yaml"), "old")
			return d
		},
		"legacy summary": func(t *testing.T) string {
			d := t.TempDir()
			writeFile(t, filepath.Join(d, "export-summary.yaml"), "old")
			writeFile(t, filepath.Join(d, "stale.yaml"), "old")
			return d
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir := setup(t)
			if err := exportInto(t, dir, oneDevice(), nil); err != nil {
				t.Fatalf("Export: %v", err)
			}
			if exists(filepath.Join(dir, "stale.yaml")) || exists(filepath.Join(dir, "groups", "old")) {
				t.Error("a file of the previous export survived")
			}
			if !exists(filepath.Join(dir, markerName)) {
				t.Error("the marker is missing after the export")
			}
			if !exists(filepath.Join(dir, "export-summary.yaml")) {
				t.Error("the summary is missing after the export")
			}
		})
	}
}

// T3b: a marker that is a symlink or a directory is not a marker.
func TestExportDoesNotTrustAMarkerThatIsNotARegularFile(t *testing.T) {
	for _, name := range []string{markerName, "export-summary.yaml"} {
		t.Run("symlink "+name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(t.TempDir(), "real")
			writeFile(t, target, "x")
			if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
				t.Skipf("symlink: %v", err)
			}
			writeFile(t, filepath.Join(dir, "keep.txt"), "mine")
			if err := exportInto(t, dir, oneDevice(), nil); !errors.Is(err, sync.ErrInvalidExportPath) {
				t.Fatalf("want ErrInvalidExportPath, got %v", err)
			}
			if !exists(filepath.Join(dir, "keep.txt")) {
				t.Error("keep.txt was removed")
			}
		})
		t.Run("directory "+name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(dir, "keep.txt"), "mine")
			if err := exportInto(t, dir, oneDevice(), nil); !errors.Is(err, sync.ErrInvalidExportPath) {
				t.Fatalf("want ErrInvalidExportPath, got %v", err)
			}
			if !exists(filepath.Join(dir, "keep.txt")) {
				t.Error("keep.txt was removed")
			}
		})
	}
}

// T4: a regular file is an error, a symlinked owned directory keeps its link, and a run that fails
// halfway leaves a directory the next run may replace.
func TestExportOutputPathShapes(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file")
		writeFile(t, f, "x")
		if err := exportInto(t, f, oneDevice(), nil); err == nil {
			t.Fatal("exporting into a regular file must fail")
		}
		if b, _ := os.ReadFile(f); string(b) != "x" {
			t.Error("the file was changed")
		}
	})
	t.Run("symlink to an owned directory", func(t *testing.T) {
		real := t.TempDir()
		writeFile(t, filepath.Join(real, markerName), "x\n")
		writeFile(t, filepath.Join(real, "stale.yaml"), "old")
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("symlink: %v", err)
		}
		if err := exportInto(t, link, oneDevice(), nil); err != nil {
			t.Fatalf("Export: %v", err)
		}
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&fs.ModeSymlink == 0 {
			t.Errorf("the link did not survive: %v", err)
		}
		if exists(filepath.Join(real, "stale.yaml")) {
			t.Error("the old file survived")
		}
	})
	t.Run("a clean that stops halfway leaves a directory the next run may replace", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory modes")
		}
		// A legacy directory: only export-summary.yaml says it is ours. The summary sorts before
		// "sub", so the clean removes it first and then fails on the unwritable child. The next run
		// is admitted only because the marker was written before the clean started.
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "export-summary.yaml"), "old")
		sub := filepath.Join(dir, "sub")
		writeFile(t, filepath.Join(sub, "child.txt"), "old")
		if err := os.Chmod(sub, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })
		if err := exportInto(t, dir, oneDevice(), nil); err == nil {
			t.Fatal("a clean that cannot remove an entry must fail")
		}
		if exists(filepath.Join(dir, "export-summary.yaml")) {
			t.Fatal("the clean did not reach the summary: the test does not model a partial clean")
		}
		if !isRegularFile(filepath.Join(dir, markerName)) {
			t.Fatal("the marker must be there after a clean that stopped halfway")
		}
		if err := os.Chmod(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := exportInto(t, dir, oneDevice(), nil); err != nil {
			t.Fatalf("the next run must succeed: %v", err)
		}
		if exists(sub) {
			t.Error("the next run left the old entry")
		}
	})
}

// T5: with an export base directory the engine accepts the base itself, so the plugin has to
// refuse to wipe it when it holds another plugin's export.
func TestExportRefusesABaseDirectoryHoldingOtherExports(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "shelly-backup-1.sqlite"), "another plugin's file")
	if err := exportInto(t, base, oneDevice(), nil); !errors.Is(err, sync.ErrInvalidExportPath) {
		t.Fatalf("want ErrInvalidExportPath, got %v", err)
	}
	if !exists(filepath.Join(base, "shelly-backup-1.sqlite")) {
		t.Error("the other plugin's export was removed")
	}
}

// walkFiles lists every path under root, relative to it.
func walkFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// assertConfined checks that the export wrote its device file and that nothing else was created
// beside the output directory.
func assertConfined(t *testing.T, root, deviceFile string) {
	t.Helper()
	found := false
	for _, p := range walkFiles(t, root) {
		switch {
		case p == "." || p == "out":
		case strings.HasPrefix(p, "out"+string(filepath.Separator)):
			if filepath.Base(p) == deviceFile {
				found = true
			}
		default:
			t.Errorf("%q was created outside the output directory", p)
		}
	}
	if !found {
		t.Errorf("the export wrote no %s under the output directory", deviceFile)
	}
}

// T6: a group or device type name never leaves the output directory, whatever it holds. Each name is
// deep enough to escape under the old unchecked join, from the group directory (two levels below the
// output directory), the type directory below it (three) and the ungrouped type directory (one).
func TestExportConfinesGroupAndTypeNames(t *testing.T) {
	hostile := []string{
		"../../../x", "../../x", "../../../../x/export-summary.yaml", `..\..\..\x`,
		"..", ".", "a/b", "",
	}
	for _, name := range hostile {
		t.Run("custom group "+name, func(t *testing.T) {
			root := t.TempDir()
			devices := oneDevice()
			extra := map[string]interface{}{
				"group_by":      "custom",
				"group_mapping": map[string]interface{}{devices[0].Name: name},
			}
			if err := exportInto(t, filepath.Join(root, "out"), devices, extra); err != nil {
				t.Fatalf("Export: %v", err)
			}
			assertConfined(t, root, "kitchen-switch.yaml")
		})
		t.Run("device type in a group "+name, func(t *testing.T) {
			root := t.TempDir()
			devices := []sync.DeviceData{{ID: 1, Name: "Kitchen Switch", Type: name, MAC: "00:11:22:33:44:66"}}
			if err := exportInto(t, filepath.Join(root, "out"), devices, map[string]interface{}{"group_by": "default"}); err != nil {
				t.Fatalf("Export: %v", err)
			}
			assertConfined(t, root, "kitchen-switch.yaml")
		})
		t.Run("device type ungrouped "+name, func(t *testing.T) {
			root := t.TempDir()
			// No location word in the name, so location grouping leaves the device ungrouped.
			devices := []sync.DeviceData{{ID: 1, Name: "Lamp", Type: name, MAC: "00:11:22:33:44:66"}}
			if err := exportInto(t, filepath.Join(root, "out"), devices, map[string]interface{}{"group_by": "location"}); err != nil {
				t.Fatalf("Export: %v", err)
			}
			assertConfined(t, root, "lamp.yaml")
		})
	}
}

// A marker that is a symlink beside a legacy summary must not be written through: the summary
// admits the directory, and the marker write would otherwise truncate the link's target.
func TestExportDoesNotWriteThroughAMarkerSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "keep.txt")
	writeFile(t, outside, "precious")
	writeFile(t, filepath.Join(dir, "export-summary.yaml"), "old")
	if err := os.Symlink(outside, filepath.Join(dir, markerName)); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if err := exportInto(t, dir, oneDevice(), nil); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if b, _ := os.ReadFile(outside); string(b) != "precious" {
		t.Errorf("the symlink target was written: %q", b)
	}
	if !isRegularFile(filepath.Join(dir, markerName)) {
		t.Error("the marker is not a regular file after the export")
	}
}

// Two names that sanitize to one directory share it, and both keep their device files.
func TestExportCollidingGroupNamesKeepTheirDevices(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	devices := []sync.DeviceData{
		{ID: 1, Name: "Lamp One", Type: "SHSW-1", MAC: "00:11:22:33:44:01"},
		{ID: 2, Name: "Lamp Two", Type: "SHSW-1", MAC: "00:11:22:33:44:02"},
	}
	extra := map[string]interface{}{
		"group_by":      "custom",
		"group_mapping": map[string]interface{}{"Lamp One": "a/b", "Lamp Two": "a-b"},
	}
	if err := exportInto(t, out, devices, extra); err != nil {
		t.Fatalf("Export: %v", err)
	}
	for _, f := range []string{"lamp-one.yaml", "lamp-two.yaml"} {
		if !exists(filepath.Join(out, "groups", "a-b", "shsw-1", f)) {
			t.Errorf("%s is missing from the shared group directory", f)
		}
	}
}

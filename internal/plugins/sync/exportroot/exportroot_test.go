// Package exportroot_test runs one table over every export plugin that writes a request-named
// directory: an export lands inside the export root or is refused, and a refusal creates nothing.
package exportroot_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/jsonexport"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/sma"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/yamlexport"
	"github.com/ginsys/shelly-manager/internal/sync"
)

var plugins = map[string]func() sync.SyncPlugin{
	"gitops": gitops.NewPlugin,
	"json":   jsonexport.NewPlugin,
	"sma":    sma.NewPlugin,
	"yaml":   yamlexport.NewPlugin,
}

func exportData() *sync.ExportData {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &sync.ExportData{
		Devices: []sync.DeviceData{{
			ID: 1, MAC: "aabbccddeeff", IP: "192.0.2.1", Type: "switch", Name: "Kitchen",
			Model: "Shelly", Firmware: "1", Status: "online", LastSeen: now, CreatedAt: now, UpdatedAt: now,
		}},
		Metadata: sync.ExportMetadata{
			ExportID: "123e4567-e89b-42d3-a456-426614174000", RequestedBy: "test",
			ExportType: "manual", SystemVersion: "test", DatabaseType: "sqlite", TotalDevices: 1,
		},
		Timestamp: now,
	}
}

// setRoot hands the root to the plugin the way the engine does.
func setRoot(p sync.SyncPlugin, root string) {
	if r, ok := p.(sync.ExportPathRestrictedPlugin); ok {
		r.SetExportBaseDir(root)
	} else if r, ok := p.(sync.PathRestrictedPlugin); ok {
		r.SetBaseDir(root)
	}
}

func entries(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	if err := filepath.WalkDir(dir, func(_ string, _ os.DirEntry, err error) error {
		if err == nil {
			n++
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return n - 1 // the directory itself
}

// TestExportPathsStayInsideTheRoot: for each plugin and each way of naming the root (handed over by
// the engine, or the default ./data/exports with nothing handed over), a path inside the root is
// written and every path that leaves it is refused with nothing created outside.
func TestExportPathsStayInsideTheRoot(t *testing.T) {
	for _, mode := range []string{"configured", "default"} {
		for name, newPlugin := range plugins {
			t.Run(mode+"/"+name, func(t *testing.T) {
				base := t.TempDir()
				outside := filepath.Join(base, "outside")
				if err := os.Mkdir(outside, 0o755); err != nil {
					t.Fatal(err)
				}
				var root string
				p := newPlugin()
				if err := p.Initialize(logging.GetDefault()); err != nil {
					t.Fatal(err)
				}
				if mode == "configured" {
					root = filepath.Join(base, "root")
					setRoot(p, root)
				} else {
					work := filepath.Join(base, "work")
					if err := os.Mkdir(work, 0o755); err != nil {
						t.Fatal(err)
					}
					t.Chdir(work)
					root = filepath.Join(work, "data", "exports")
				}
				if err := os.MkdirAll(root, 0o755); err != nil { // startup creates it
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
				dotdot := root + string(filepath.Separator) + "a" + string(filepath.Separator) + ".." +
					string(filepath.Separator) + ".." + string(filepath.Separator) + "outside" + string(filepath.Separator) + "x"

				export := func(path string) error {
					cfg := map[string]interface{}{"include_discovered": true}
					if path != "" {
						cfg["output_path"] = path
					}
					_, err := p.Export(context.Background(), exportData(), sync.ExportConfig{Format: name, Config: cfg})
					return err
				}

				refused := map[string]string{
					"absolute outside":                     filepath.Join(outside, "abs"),
					"dot-dot":                              dotdot,
					"relative dot-dot":                     filepath.Join("..", "outside", "rel"),
					"symlink directory":                    filepath.Join("link", "x"),
					"symlink directory, new part below it": filepath.Join("link", "new", "sub"),
				}
				for label, path := range refused {
					if err := export(path); err == nil {
						t.Errorf("%s: %q was accepted", label, path)
					}
					if n := entries(t, outside); n != 0 {
						t.Fatalf("%s: %q left %d entries outside the root", label, path, n)
					}
				}

				if err := export("sub"); err != nil {
					t.Fatalf("a plain sub-directory was refused: %v", err)
				}
				if fi, err := os.Stat(filepath.Join(root, "sub")); err != nil || !fi.IsDir() || entries(t, filepath.Join(root, "sub")) == 0 {
					t.Errorf("nothing was written under the root's sub directory (stat err %v)", err)
				}

				if err := export(filepath.Join(root, "abs-inside")); err != nil {
					t.Fatalf("an absolute path inside the root was refused: %v", err)
				}
				if err := export(""); err != nil {
					t.Fatalf("an empty output_path was refused: %v", err)
				}
				if n := entries(t, outside); n != 0 {
					t.Fatalf("an accepted export left %d entries outside the root", n)
				}
			})
		}
	}
}

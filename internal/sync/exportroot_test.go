package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUnderRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{root, outside, filepath.Join(root, "sub")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "sub"), filepath.Join(base, "rootlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)

	accepted := map[string]struct{ root, path, want string }{
		"empty path is the root":        {root, "", root},
		"plain sub directory":           {root, "sub", filepath.Join(root, "sub")},
		"sub directory not created yet": {root, filepath.Join("new", "deep"), filepath.Join(root, "new", "deep")},
		"absolute inside":               {root, filepath.Join(root, "sub"), filepath.Join(root, "sub")},
		"dot-dot that stays inside":     {root, filepath.Join("sub", "..", "other"), filepath.Join(root, "other")},
		"the root is itself a symlink":  {filepath.Join(base, "rootlink"), "x", filepath.Join(base, "rootlink", "x")},
		"a symlink that points inside":  {root, filepath.Join("sub", "x"), filepath.Join(root, "sub", "x")},
		"the root has a trailing slash": {root + sep, "sub", filepath.Join(root, "sub")},
	}
	for name, c := range accepted {
		got, err := ResolveUnderRoot(c.root, c.path)
		if err != nil || got != c.want {
			t.Errorf("%s: ResolveUnderRoot(%q, %q) = %q, %v; want %q", name, c.root, c.path, got, err, c.want)
		}
	}

	refused := map[string]struct{ root, path string }{
		"absolute outside":                   {root, filepath.Join(outside, "x")},
		"dot-dot":                            {root, root + sep + "sub" + sep + ".." + sep + ".." + sep + "outside"},
		"relative dot-dot":                   {root, filepath.Join("..", "outside")},
		"the root's parent":                  {root, base},
		"a sibling that shares the prefix":   {root, root + "-other"},
		"symlink directory pointing out":     {root, filepath.Join("link", "x")},
		"new part below a symlink":           {root, filepath.Join("link", "new", "sub")},
		"symlink directory itself":           {root, "link"},
		"a root that does not exist":         {filepath.Join(base, "missing"), "sub"},
		"a root that is a file":              {filepath.Join(outside, "file"), "sub"},
		"absolute inside through a symlink":  {root, filepath.Join(root, "link", "x")},
		"outside, written through the alias": {filepath.Join(base, "rootlink"), filepath.Join(outside, "x")},
	}
	for name, c := range refused {
		got, err := ResolveUnderRoot(c.root, c.path)
		if err == nil {
			t.Errorf("%s: ResolveUnderRoot(%q, %q) = %q, nil; want a refusal", name, c.root, c.path, got)
			continue
		}
		if !strings.Contains(err.Error(), "export directory") {
			t.Errorf("%s: the refusal %q does not name the export directory", name, err)
		}
	}
	for _, d := range []string{filepath.Join(outside, "x"), filepath.Join(outside, "new"), filepath.Join(base, "missing")} {
		if _, err := os.Lstat(d); err == nil {
			t.Errorf("%q exists: the helper must create nothing", d)
		}
	}
}

func TestResolveUnderRootEmptyRootIsTheDefault(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	if err := os.MkdirAll(filepath.Join(work, "data", "exports"), 0o755); err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(DefaultExportRoot)
	got, err := ResolveUnderRoot("", "sub")
	if err != nil || got != filepath.Join(abs, "sub") {
		t.Fatalf("ResolveUnderRoot(\"\", \"sub\") = %q, %v; want %q", got, err, filepath.Join(abs, "sub"))
	}
	if _, err := ResolveUnderRoot("", filepath.Join(work, "elsewhere")); err == nil {
		t.Fatal("a path outside the default root was accepted")
	}
}

func TestEnsureExportRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "a", "b")
	if err := EnsureExportRoot(root); err != nil {
		t.Fatalf("EnsureExportRoot: %v", err)
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		t.Fatalf("the root was not created (stat err %v)", err)
	}
	if err := EnsureExportRoot(root); err != nil {
		t.Fatalf("EnsureExportRoot on an existing root: %v", err)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureExportRoot(file); err == nil {
		t.Fatal("a root that is a file was accepted")
	}
	if err := EnsureExportRoot(filepath.Join(file, "sub")); err == nil {
		t.Fatal("a root below a file was accepted")
	}
	work := t.TempDir()
	t.Chdir(work)
	if err := EnsureExportRoot(""); err != nil {
		t.Fatalf("EnsureExportRoot with the default: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(work, "data", "exports")); err != nil || !fi.IsDir() {
		t.Fatalf("the default root was not created (stat err %v)", err)
	}
}

package docscheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var (
	makeTarget = regexp.MustCompile(`\bmake +([a-z][a-z0-9_-]*)`)
	codeSpan   = regexp.MustCompile("`[^`]*`")
	// A plan names a target it proposes. Nothing else is excused.
	proposed = "docs/testing/E2E_TEST_OPTIMIZATION_PLAN.md test-e2e-optimized"
)

// deadMakeTargets lists "file:line target" for each `make <target>` that a code span or a fenced block of a
// Markdown file under root names and the Makefile has no rule for. Prose ("make sure") is not read. It walks the
// tree, so it needs no git, and it reads untracked Markdown too: a stray local note can fail it where CI cannot
// see the note. It skips dot directories, node_modules, vendor and changelog.d, and CHANGELOG.md.
func deadMakeTargets(t *testing.T, root string) (dead []string) {
	fsys := os.DirFS(root)
	mk, err := fs.ReadFile(fsys, "Makefile")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && path != "." && (strings.HasPrefix(d.Name(), ".") || slices.Contains([]string{"node_modules", "vendor", "changelog.d"}, d.Name())):
			return fs.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".md") || path == "CHANGELOG.md":
			return nil
		}
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		fenced := false
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
			} else if !fenced {
				line = strings.Join(codeSpan.FindAllString(line, -1), " ")
			}
			for _, m := range makeTarget.FindAllStringSubmatch(line, -1) {
				rule := regexp.MustCompile("(?m)^" + regexp.QuoteMeta(m[1]) + ":")
				if !rule.Match(mk) && path+" "+m[1] != proposed {
					dead = append(dead, fmt.Sprintf("%s:%d %s", path, i+1, m[1]))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dead
}

// ginsys/shelly-manager#832: a doc that names a make target with no rule sends a reader to a command that fails.
func TestDocsNameOnlyMakeTargetsThatExist(t *testing.T) {
	for _, d := range deadMakeTargets(t, "../..") {
		t.Errorf("%s has no rule in the Makefile", d)
	}
}

// Prose is not read ("make sure", "make a" outside a code span), and node_modules, vendor and dot directories
// are skipped at any depth.
func TestDeadMakeTargetsReadsOnlyCodeSpansAndFences(t *testing.T) {
	root := t.TempDir()
	body := "make sure of it. Run `make test` or `make gone`.\n\n```\nmake test\nmake absent\n```\nmake a thing\n"
	files := map[string]string{"Makefile": "test:\n\ttrue\n", "a.md": body}
	for _, skipped := range []string{"ui/node_modules/p", "vendor", "x/vendor", ".hidden"} {
		files[skipped+"/b.md"] = "`make skipped`\n"
	}
	for name, text := range files {
		if err := os.MkdirAll(filepath.Dir(root+"/"+name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(root+"/"+name, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := strings.Join(deadMakeTargets(t, root), ","), "a.md:1 gone,a.md:5 absent"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

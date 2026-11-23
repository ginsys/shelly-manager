package version

// Tests of scripts/release/release.sh and .github/workflows/release.yml against real git repositories:
// a bare repository is the remote, a clone holds the two real scripts at their real paths. They are Go
// tests because `make test-ci` is `go test ./...` and the verify run needs the named test in that
// output; the bash fixture tests of this repository run in their own workflow.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const releaseDate = "2026-11-23"

const fixtureChangelog = "# Changelog\n\n## [Unreleased]\n"

// releaseFixture is a bare "origin" and a working clone of it on main.
type releaseFixture struct {
	t              *testing.T
	root           string
	origin, work   string
	githubOutput   string
	changelogStart string
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func copyScript(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dst, string(b), 0o755)
}

// newReleaseFixture makes the repository with VERSION=version, an empty "## [Unreleased]" CHANGELOG.md,
// two fragments, and both scripts copied from this repository.
func newReleaseFixture(t *testing.T, version string) *releaseFixture {
	t.Helper()
	root := t.TempDir()
	f := &releaseFixture{
		t: t, root: root,
		origin:         filepath.Join(root, "origin.git"),
		work:           filepath.Join(root, "work"),
		githubOutput:   filepath.Join(root, "github_output"),
		changelogStart: fixtureChangelog,
	}
	git(t, root, "init", "--bare", "-b", "main", f.origin)
	git(t, root, "init", "-b", "main", f.work)
	git(t, f.work, "config", "user.name", "fixture")
	git(t, f.work, "config", "user.email", "fixture@example.test")
	git(t, f.work, "remote", "add", "origin", f.origin)
	writeFile(t, filepath.Join(f.work, "VERSION"), version+"\n", 0o644)
	writeFile(t, filepath.Join(f.work, "CHANGELOG.md"), f.changelogStart, 0o644)
	writeFile(t, filepath.Join(f.work, "changelog.d", "1-first.md"), "Added\n- First.\n", 0o644)
	writeFile(t, filepath.Join(f.work, "changelog.d", "2-second.md"), "Fixed\n- Second.\n", 0o644)
	copyScript(t, "../../scripts/release/release.sh", filepath.Join(f.work, "scripts", "release", "release.sh"))
	copyScript(t, "../../scripts/changelog-assemble.sh", filepath.Join(f.work, "scripts", "changelog-assemble.sh"))
	git(t, f.work, "add", "-A")
	git(t, f.work, "commit", "-m", "base")
	git(t, f.work, "push", "origin", "main")
	return f
}

// env is the script's environment: nothing inherited from a CI runner decides a case.
func (f *releaseFixture) env(action, dryRun string, ci bool, branch string) []string {
	c := "false"
	if ci {
		c = "true"
	}
	return []string{
		"RELEASE_ACTION=" + action, "RELEASE_BRANCH=" + branch, "RELEASE_REF_TYPE=branch",
		"DRY_RUN=" + dryRun, "CI=" + c, "CHANGELOG_DATE=" + releaseDate,
		"GITHUB_OUTPUT=" + f.githubOutput, "GITHUB_STEP_SUMMARY=", "GITHUB_ACTIONS=",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
	}
}

func (f *releaseFixture) run(env []string, args ...string) (stdout, stderr string, code int) {
	f.t.Helper()
	return runScript(f.t, filepath.Join(f.work, "scripts", "release", "release.sh"), f.work, env, args...)
}

// snapshot is everything a refused or dry run must leave alone: the branches and tags of both
// repositories, the work tree's state, and the contents of VERSION and CHANGELOG.md.
func (f *releaseFixture) snapshot() string {
	f.t.Helper()
	v, _ := os.ReadFile(filepath.Join(f.work, "VERSION"))
	c, _ := os.ReadFile(filepath.Join(f.work, "CHANGELOG.md"))
	return strings.Join([]string{
		git(f.t, f.work, "for-each-ref", "refs/heads", "refs/tags"), // not refs/remotes: a fetch moves it
		git(f.t, f.origin, "for-each-ref"),
		git(f.t, f.work, "status", "--porcelain", "--untracked-files=all"),
		string(v), string(c),
	}, "\n--\n")
}

func (f *releaseFixture) remoteTags() string {
	f.t.Helper()
	return git(f.t, f.origin, "tag", "--list")
}

type releaseCase struct {
	name, version, action string
	tag, next             string // tag is "" for the actions that only move VERSION
	commits               int    // commits the push adds to main
}

var releaseCases = []releaseCase{
	{"release", "v0.6.0-alpha.1", "release", "v0.6.0-alpha.1", "v0.6.0-alpha.2", 2},
	{"release-as-beta from alpha", "v0.6.0-alpha.3", "release-as-beta", "v0.6.0-beta.0", "v0.6.0-beta.1", 2},
	{"release-as-beta from beta", "v0.6.0-beta.2", "release-as-beta", "v0.6.0-beta.2", "v0.6.0-beta.3", 2},
	{"release-as-rc from alpha", "v0.6.0-alpha.3", "release-as-rc", "v0.6.0-rc.0", "v0.6.0-rc.1", 2},
	{"release-as-stable", "v0.6.0-rc.1", "release-as-stable", "v0.6.0", "v0.6.1-alpha.0", 2},
	{"start-next-minor", "v0.6.0-alpha.3", "start-next-minor", "", "v0.7.0-alpha.0", 1},
	{"start-next-major", "v0.6.0-alpha.3", "start-next-major", "", "v1.0.0-alpha.0", 1},
	{"skip-prerelease-number", "v0.6.0-alpha.3", "skip-prerelease-number", "", "v0.6.0-alpha.4", 1},
}

// Each of the seven actions moves VERSION and tags, or does not, as the design says; a tagging action
// assembles the changelog into the tagged commit and removes the fragments, a VERSION-only one touches
// neither; everything goes to the remote in one push.
func TestReleaseScriptActions(t *testing.T) {
	for _, c := range releaseCases {
		t.Run(c.name, func(t *testing.T) {
			f := newReleaseFixture(t, c.version)
			base := git(t, f.origin, "rev-parse", "main")
			stdout, stderr, code := f.run(f.env(c.action, "0", true, "main"))
			if code != 0 {
				t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}
			head := git(t, f.work, "rev-parse", "HEAD")
			if got := git(t, f.origin, "rev-parse", "main"); got != head {
				t.Errorf("remote main %s, want the pushed HEAD %s", got, head)
			}
			if got := git(t, f.origin, "rev-list", "--count", base+"..main"); got != strconv.Itoa(c.commits) {
				t.Errorf("the push added %s commits, want %d", got, c.commits)
			}
			if got := git(t, f.origin, "show", "main:VERSION"); got != c.next {
				t.Errorf("VERSION on main is %q, want %q", got, c.next)
			}
			out, _ := os.ReadFile(f.githubOutput)
			if c.tag == "" {
				if tags := f.remoteTags(); tags != "" {
					t.Errorf("remote tags %q, want none", tags)
				}
				if strings.Contains(string(out), "tag=") {
					t.Errorf("GITHUB_OUTPUT names a tag %q for a VERSION-only action", out)
				}
				if got := git(t, f.origin, "show", "main:CHANGELOG.md"); got != strings.TrimSpace(f.changelogStart) {
					t.Errorf("CHANGELOG.md changed for a VERSION-only action:\n%s", got)
				}
				git(t, f.origin, "cat-file", "-e", "main:changelog.d/1-first.md") // still there
				return
			}
			if tags := f.remoteTags(); tags != c.tag {
				t.Fatalf("remote tags %q, want only %q", tags, c.tag)
			}
			if got := git(t, f.origin, "cat-file", "-t", c.tag); got != "tag" {
				t.Errorf("%s is a %s object, want an annotated tag", c.tag, got)
			}
			if got := git(t, f.origin, "rev-parse", c.tag+"^{commit}"); got != git(t, f.origin, "rev-parse", "main~1") {
				t.Errorf("%s does not point at the release commit (main~1)", c.tag)
			}
			if got := git(t, f.origin, "show", c.tag+":VERSION"); got != c.tag {
				t.Errorf("VERSION at the tag is %q, want %q", got, c.tag)
			}
			heading := "## [" + strings.TrimPrefix(c.tag, "v") + "] - " + releaseDate
			for _, rev := range []string{c.tag, "main"} {
				cl := git(t, f.origin, "show", rev+":CHANGELOG.md")
				for _, want := range []string{heading, "### Added", "- First.", "### Fixed", "- Second."} {
					if !strings.Contains(cl, want) {
						t.Errorf("CHANGELOG.md at %s lacks %q:\n%s", rev, want, cl)
					}
				}
			}
			if names := git(t, f.origin, "ls-tree", "--name-only", c.tag, "changelog.d/"); names != "" {
				t.Errorf("fragments left in the tagged commit: %q", names)
			}
			if got := git(t, f.origin, "log", "-1", "--format=%s", c.tag+"^{commit}"); got != "release: "+c.tag {
				t.Errorf("release commit subject %q", got)
			}
			if got := git(t, f.origin, "log", "-1", "--format=%an", "main"); got != "shelly-release-bot" {
				t.Errorf("author of the bump commit %q, want the release bot", got)
			}
			if !strings.Contains(string(out), "tag="+c.tag+"\n") {
				t.Errorf("GITHUB_OUTPUT %q lacks tag=%s", out, c.tag)
			}
			// The release's own notes step reads the section the script wrote.
			notes, nerr, ncode := runGuard(t, f.work, "notes", c.tag, "CHANGELOG.md")
			if ncode != 0 || !strings.Contains(notes, "- First.") || !strings.Contains(notes, upgradeParagraph) {
				t.Errorf("publish-guard notes %s: exit %d, stderr %q, stdout %q", c.tag, ncode, nerr, notes)
			}
		})
	}
}

// The dry run prints the version, the tag, the changelog section and the push it would make, and
// changes nothing, in either repository.
func TestReleaseScriptDryRun(t *testing.T) {
	for _, c := range []releaseCase{releaseCases[0], releaseCases[5]} {
		t.Run(c.name, func(t *testing.T) {
			f := newReleaseFixture(t, c.version)
			before := f.snapshot()
			stdout, stderr, code := f.run(f.env(c.action, "1", false, "main"))
			if code != 0 {
				t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}
			if after := f.snapshot(); after != before {
				t.Errorf("a dry run changed the repositories:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			want := []string{c.version, c.next, "git push --atomic origin HEAD:refs/heads/main"}
			if c.tag != "" {
				want = append(want, "git push --atomic origin HEAD:refs/heads/main refs/tags/"+c.tag,
					"### Added", "- First.", "### Fixed", "- Second.")
			}
			for _, w := range want {
				if !strings.Contains(stdout, w) {
					t.Errorf("dry run output lacks %q:\n%s", w, stdout)
				}
			}
			if c.tag == "" && strings.Contains(stdout, "refs/tags/") {
				t.Errorf("a VERSION-only dry run names a tag:\n%s", stdout)
			}
		})
	}
	t.Run("from another branch", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		before := f.snapshot()
		stdout, stderr, code := f.run(f.env("release", "1", false, "feat/x"))
		if code != 0 || !strings.Contains(stdout+stderr, "feat/x") {
			t.Errorf("exit %d, want a warned preview\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if f.snapshot() != before {
			t.Error("a dry run from a branch changed the repositories")
		}
	})
}

// Whatever is wrong is refused before anything changes: nothing is written, committed, tagged or pushed.
func TestReleaseScriptGuard(t *testing.T) {
	refused := func(t *testing.T, f *releaseFixture, env []string, want string) {
		t.Helper()
		before := f.snapshot()
		stdout, stderr, code := f.run(env)
		if code != 1 || !strings.Contains(stderr, want) {
			t.Errorf("exit %d, want 1 naming %q\nstdout: %s\nstderr: %s", code, want, stdout, stderr)
		}
		if after := f.snapshot(); after != before {
			t.Errorf("a refused run changed the repositories:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	}
	t.Run("a real run off main", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		refused(t, f, f.env("release", "0", true, "feat/x"), "main")
	})
	t.Run("a tag as the ref", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		env := append(f.env("release", "0", true, "v0.6.0-alpha.1"), "RELEASE_REF_TYPE=tag")
		refused(t, f, env, "tag")
	})
	t.Run("an unknown action", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		refused(t, f, f.env("release-it", "0", true, "main"), "Unknown action")
	})
	t.Run("dry_run neither 0 nor 1", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		refused(t, f, f.env("release", "2", true, "main"), "DRY_RUN")
	})
	t.Run("--guard alone", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		if _, stderr, code := f.run(f.env("release", "0", true, "main"), "--guard"); code != 0 {
			t.Errorf("--guard on main: exit %d, %s", code, stderr)
		}
		refused(t, f, append(f.env("release", "0", true, "feat/x"), "RELEASE_REF_TYPE=branch"), "main")
	})
	t.Run("going back a stage", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-rc.1")
		refused(t, f, f.env("release-as-beta", "0", true, "main"), "going back")
	})
	t.Run("a stable VERSION on main", func(t *testing.T) {
		f := newReleaseFixture(t, "v1.0.0")
		refused(t, f, f.env("release", "0", true, "main"), "not a prerelease")
	})
	t.Run("a VERSION that is not a version", func(t *testing.T) {
		f := newReleaseFixture(t, "banana")
		refused(t, f, f.env("release", "0", true, "main"), "Invalid version")
	})
	t.Run("an invalid fragment", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		writeFile(t, filepath.Join(f.work, "changelog.d", "3-bad.md"), "Nonsense\n- Bad.\n", 0o644)
		git(t, f.work, "add", "-A")
		git(t, f.work, "commit", "-m", "bad fragment")
		git(t, f.work, "push", "origin", "main")
		refused(t, f, f.env("release", "0", true, "main"), "")
	})
	// A tag that exists, as the tag or as the next VERSION, on the remote or only here, is never reused.
	for _, c := range releaseCases {
		for _, which := range []string{"tag", "next"} {
			name := which
			tag := c.tag
			if which == "next" {
				tag = c.next
			}
			if tag == "" {
				continue
			}
			t.Run("existing "+name+" of "+c.name, func(t *testing.T) {
				f := newReleaseFixture(t, c.version)
				git(t, f.origin, "tag", tag, "main")
				refused(t, f, f.env(c.action, "0", true, "main"), "already exists")
			})
		}
	}
	t.Run("a tag that exists only in the checkout", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		git(t, f.work, "tag", "v0.6.0-alpha.1")
		refused(t, f, f.env("release", "1", false, "main"), "already exists")
	})
	t.Run("main moved after the run started", func(t *testing.T) {
		f := newReleaseFixture(t, "v0.6.0-alpha.1")
		other := filepath.Join(f.root, "other")
		git(t, f.root, "clone", f.origin, other)
		writeFile(t, filepath.Join(other, "later.txt"), "merged during the Test wait\n", 0o644)
		git(t, other, "add", "-A")
		git(t, other, "commit", "-m", "later")
		git(t, other, "push", "origin", "main")
		refused(t, f, f.env("release", "0", true, "main"), "tip")
		if tags := f.remoteTags(); tags != "" {
			t.Errorf("remote tags %q, want none", tags)
		}
	})
}

// A tagging action with no fragment left is refused with exit 3, in a dry run too, before anything is
// written; an action that only moves VERSION needs none.
func TestReleaseScriptNoFragments(t *testing.T) {
	prepare := func(t *testing.T, version string) *releaseFixture {
		f := newReleaseFixture(t, version)
		git(t, f.work, "rm", "-q", "-r", "changelog.d")
		git(t, f.work, "commit", "-m", "no fragments")
		git(t, f.work, "push", "origin", "main")
		return f
	}
	for _, dry := range []string{"0", "1"} {
		f := prepare(t, "v0.6.0-alpha.1")
		before := f.snapshot()
		stdout, stderr, code := f.run(f.env("release", dry, dry == "0", "main"))
		if code != 3 || !strings.Contains(stderr, "fragments") {
			t.Errorf("DRY_RUN=%s: exit %d, want 3 naming the fragments\nstdout: %s\nstderr: %s", dry, code, stdout, stderr)
		}
		if f.snapshot() != before {
			t.Errorf("DRY_RUN=%s: the refused run changed the repositories", dry)
		}
	}
	f := prepare(t, "v0.6.0-alpha.3")
	if stdout, stderr, code := f.run(f.env("skip-prerelease-number", "0", true, "main")); code != 0 {
		t.Errorf("skip-prerelease-number with no fragment: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if got := git(t, f.origin, "show", "main:VERSION"); got != "v0.6.0-alpha.4" {
		t.Errorf("VERSION on main %q, want v0.6.0-alpha.4", got)
	}
}

// main and the tag go in one atomic push: a remote that refuses main (a hook that looks at each ref
// alone, so a non-atomic push would still land the tag) ends with neither.
func TestReleaseScriptAtomicPush(t *testing.T) {
	f := newReleaseFixture(t, "v0.6.0-alpha.1")
	writeFile(t, filepath.Join(f.origin, "hooks", "update"),
		"#!/bin/sh\n[ \"$1\" = refs/heads/main ] && exit 1\nexit 0\n", 0o755)
	base := git(t, f.origin, "rev-parse", "main")
	stdout, stderr, code := f.run(f.env("release", "0", true, "main"))
	if code != 1 || !strings.Contains(stderr, "nothing was published") {
		t.Errorf("exit %d, want 1 and 'nothing was published'\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if tags := f.remoteTags(); tags != "" {
		t.Errorf("the refused push left tags on the remote: %q", tags)
	}
	if got := git(t, f.origin, "rev-parse", "main"); got != base {
		t.Errorf("remote main moved to %s after a refused push", got)
	}
}

type workflowFile struct {
	On struct {
		Dispatch struct {
			Inputs map[string]struct {
				Type    string   `yaml:"type"`
				Default any      `yaml:"default"`
				Options []string `yaml:"options"`
			} `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	Concurrency struct {
		Group            string `yaml:"group"`
		CancelInProgress bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]map[string]any `yaml:"jobs"`
}

// The workflow's shape: a dry run by default, one run at a time, secrets only in the one job that
// pushes, which is also the only one in the `release` environment.
func TestReleaseWorkflowShape(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var w workflowFile
	if err := yaml.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	in := w.On.Dispatch.Inputs
	if got := in["dry_run"].Default; got != true {
		t.Errorf("dry_run defaults to %v, want true", got)
	}
	wantActions := []string{"release", "release-as-beta", "release-as-rc", "release-as-stable",
		"start-next-minor", "start-next-major", "skip-prerelease-number"}
	if a := in["action"]; a.Type != "choice" || strings.Join(a.Options, " ") != strings.Join(wantActions, " ") {
		t.Errorf("action input is %+v, want the choice of %v", a, wantActions)
	}
	if w.Concurrency.Group == "" || w.Concurrency.CancelInProgress {
		t.Errorf("concurrency %+v: want one group that never cancels a running release", w.Concurrency)
	}
	for _, name := range []string{"guard", "test", "release"} {
		if _, ok := w.Jobs[name]; !ok {
			t.Fatalf("job %q is missing", name)
		}
	}
	for name, job := range w.Jobs {
		body, err := yaml.Marshal(job)
		if err != nil {
			t.Fatal(err)
		}
		hasSecrets := strings.Contains(string(body), "secrets.") || job["secrets"] != nil
		if name == "release" {
			if job["environment"] != "release" {
				t.Errorf("job release has environment %v, want release", job["environment"])
			}
			if !hasSecrets {
				t.Error("job release reads no secret: it cannot mint the App token")
			}
			continue
		}
		if hasSecrets {
			t.Errorf("job %s reads a secret; only the release job may", name)
		}
		if job["environment"] != nil {
			t.Errorf("job %s is in an environment; only the release job may be", name)
		}
	}
	if got := w.Jobs["test"]["uses"]; got != "./.github/workflows/test.yml" {
		t.Errorf("job test uses %v, want ./.github/workflows/test.yml", got)
	}
}

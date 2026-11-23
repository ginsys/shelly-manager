package version

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// upgradeParagraph is the one fixed paragraph every release's notes end with (design section 5).
const upgradeParagraph = "Back up the database before you upgrade. " +
	"To roll back, restore that backup and pin the previous tag."

// runGuard runs scripts/release/publish-guard.sh in dir ("" for the package directory).
func runGuard(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runGuardEnv(t, dir, nil, args...)
}

// runGuardEnv is runGuard with extra environment entries (KEY=value).
func runGuardEnv(t *testing.T, dir string, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	script, err := filepath.Abs("../../scripts/release/publish-guard.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	runErr := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run %v: %v", args, runErr)
	}
	return out.String(), errOut.String(), code
}

// A tag that is not vX.Y.Z or vX.Y.Z-alpha|beta|rc.N is refused, so nothing after the first step runs.
func TestPublishGuardTagForm(t *testing.T) {
	for _, tag := range []string{"v0.6.0-alpha.1", "v1.2.3", "v1.0.0-rc.2", "v0.6.0-beta.0", "v10.20.30"} {
		if _, stderr, code := runGuard(t, "", "form", tag); code != 0 {
			t.Errorf("form %q: exit %d, want 0 (%s)", tag, code, stderr)
		}
	}
	for _, tag := range []string{
		"", "0.6.0", "v1.2", "v1.2.3-alpha", "v1.2.3-alpha.01", "v01.2.3", "v1.2.3-snapshot.1",
		"v1.2.3+meta", "v1.2.3-alpha.1.2", "v1.2.3 ", "v1.2.3\nv4.5.6", "vv1.2.3",
	} {
		if _, stderr, code := runGuard(t, "", "form", tag); code != 1 || !strings.Contains(stderr, "tag") {
			t.Errorf("form %q: exit %d, stderr %q, want exit 1 naming the tag", tag, code, stderr)
		}
	}
}

const sampleChangelog = `# Changelog

## [Unreleased]

### Added
- Not released yet.

## [0.6.0-alpha.20] - 2026-12-01

### Added
- Twentieth alpha.

## [0.6.0-alpha.2] - 2026-11-30

### Added
- Second alpha thing.

### Fixed
- A fixed thing.

## [0.6.0-alpha.3] - 2026-11-30

## [0.6.0-alpha.1] - 2026-11-23

### Added
- First alpha thing.
`

// The notes of a tag are its own section's body and the fixed paragraph, never a neighbour's.
func TestReleaseNotesFromChangelog(t *testing.T) {
	file := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(file, []byte(sampleChangelog), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runGuard(t, "", "notes", "v0.6.0-alpha.2", file)
	want := "### Added\n- Second alpha thing.\n\n### Fixed\n- A fixed thing.\n\n" + upgradeParagraph + "\n"
	if code != 0 || stdout != want {
		t.Errorf("notes v0.6.0-alpha.2: exit %d, stderr %q, stdout\n%q\nwant\n%q", code, stderr, stdout, want)
	}

	if _, stderr, code = runGuard(t, "", "notes", "v0.6.0-alpha.9", file); code != 1 ||
		!strings.Contains(stderr, "no section") {
		t.Errorf("missing section: exit %d, stderr %q, want exit 1 saying no section", code, stderr)
	}
	if _, stderr, code = runGuard(t, "", "notes", "v0.6.0-alpha.3", file); code != 1 ||
		!strings.Contains(stderr, "empty") {
		t.Errorf("empty section: exit %d, stderr %q, want exit 1 saying empty", code, stderr)
	}
	if _, stderr, code = runGuard(t, "", "notes", "v0.6.0-alpha.2", file+".missing"); code != 1 {
		t.Errorf("missing file: exit %d, stderr %q, want exit 1", code, stderr)
	}

	// The real file is read the same way: a tag it has no section for is refused.
	if _, stderr, code = runGuard(t, "", "notes", "v0.0.1-alpha.0", "../../CHANGELOG.md"); code != 1 ||
		!strings.Contains(stderr, "no section") {
		t.Errorf("real CHANGELOG.md: exit %d, stderr %q, want exit 1 saying no section", code, stderr)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// A tag is released only from a commit that is on main: a commit that only a side branch holds is refused.
func TestPublishGuardOnMain(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "a")
	first := git(t, repo, "rev-parse", "HEAD")
	git(t, repo, "branch", "side")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "b")
	tip := git(t, repo, "rev-parse", "HEAD")
	git(t, repo, "checkout", "-q", "side")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "side only")
	sideOnly := git(t, repo, "rev-parse", "HEAD")

	for name, sha := range map[string]string{"an older main commit": first, "the tip of main": tip} {
		if _, stderr, code := runGuard(t, repo, "on-main", sha, "main"); code != 0 {
			t.Errorf("%s: exit %d, want 0 (%s)", name, code, stderr)
		}
	}
	if _, stderr, code := runGuard(t, repo, "on-main", sideOnly, "main"); code != 1 ||
		!strings.Contains(stderr, sideOnly) || !strings.Contains(stderr, "main") {
		t.Errorf("a side-branch commit: exit %d, stderr %q, want exit 1 naming the commit and main", code, stderr)
	}
	unknown := strings.Repeat("0", 40)
	if _, stderr, code := runGuard(t, repo, "on-main", unknown, "main"); code != 1 {
		t.Errorf("an unknown commit: exit %d, stderr %q, want 1", code, stderr)
	}
	if _, stderr, code := runGuard(t, repo, "on-main", tip, "no-such-ref"); code != 1 {
		t.Errorf("an unknown ref: exit %d, stderr %q, want 1", code, stderr)
	}
}

// A tag with a release is refused, and so is an answer that is neither 200 nor 404: a failed read is no proof.
func TestPublishGuardNoRelease(t *testing.T) {
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "args")
	fake := "#!/bin/sh\necho \"$*\" > '" + argsFile + "'\n" +
		"[ -n \"$FAKE_STATUS\" ] && echo \"HTTP/2.0 $FAKE_STATUS Fake\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fake), 0o700); err != nil { //nolint:gosec // a fake executable
		t.Fatal(err)
	}
	run := func(status string) (string, int) {
		env := []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
			"FAKE_STATUS=" + status, "GITHUB_REPOSITORY=o/r"}
		_, stderr, code := runGuardEnv(t, "", env, "no-release", "v1.2.3")
		return stderr, code
	}

	if stderr, code := run("404"); code != 0 {
		t.Errorf("404: exit %d, want 0 (%s)", code, stderr)
	}
	got, err := os.ReadFile(argsFile)
	if err != nil || !strings.Contains(string(got), "repos/o/r/releases/tags/v1.2.3") {
		t.Errorf("gh was called with %q (%v), want the releases/tags path of o/r and v1.2.3", got, err)
	}
	for status, want := range map[string]string{
		"200": "already has a release", "500": "cannot tell", "403": "cannot tell", "": "cannot tell",
	} {
		if stderr, code := run(status); code != 1 || !strings.Contains(stderr, want) {
			t.Errorf("status %q: exit %d, stderr %q, want exit 1 saying %q", status, code, stderr, want)
		}
	}
}

package version

// This is a Go test, not a bash one, because `make test-ci` is `go test ./...` and verify needs a named
// test in that output; this repository's bash fixture tests run in their own workflow.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// runScript runs a repository script (a path relative to this package) with bash.
func runScript(t *testing.T, path, dir string, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	script, err := filepath.Abs(path)
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

// fakeGH writes a gh that records its arguments, fails when FAKE_FAIL is set, and otherwise applies the
// --jq expression it is given to FAKE_JSON, as gh does.
func fakeGH(t *testing.T) (binDir, argsFile string) {
	t.Helper()
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is not installed")
	}
	binDir = t.TempDir()
	argsFile = filepath.Join(binDir, "args")
	fake := "#!/bin/sh\necho \"$*\" > '" + argsFile + "'\n" +
		"[ -n \"$FAKE_FAIL\" ] && { echo \"gh: $FAKE_FAIL\" >&2; exit 1; }\n" +
		"expr=; while [ $# -gt 0 ]; do [ \"$1\" = --jq ] && expr=$2; shift; done\n" +
		"printf '%s' \"$FAKE_JSON\" | jq -r \"$expr\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(fake), 0o700); err != nil { //nolint:gosec // a fake executable
		t.Fatal(err)
	}
	return binDir, argsFile
}

// versions is a package versions answer: newest first, a release (no latest), an untagged per-platform
// manifest, then the images main built.
func versions(newest, older string) string {
	return `[
 {"id":4,"metadata":{"container":{"tags":["20261123-100000-aaaaaaa","v0.6.0-alpha.1"]}}},
 {"id":3,"metadata":{"container":{"tags":[]}}},
 {"id":2,"metadata":{"container":{"tags":["` + newest + `","latest"]}}},
 {"id":1,"metadata":{"container":{"tags":["` + older + `","latest"]}}}
]`
}

// The base of the change filter is the commit the newest main image was built from: the sha in the
// date-sha tag of the newest `latest` version, as a full sha. Any doubt prints nothing, which builds.
func TestLastBuiltCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "older")
	older := git(t, repo, "rev-parse", "HEAD")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "newer")
	newer := git(t, repo, "rev-parse", "HEAD")
	tagOf := func(sha string) string { return "20261120-090000-" + sha[:7] }

	bin, argsFile := fakeGH(t)
	run := func(json, fail string) (stdout, stderr string, code int) {
		env := []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
			"FAKE_JSON=" + json, "FAKE_FAIL=" + fail, "GITHUB_REPOSITORY_OWNER=ginsys"}
		return runScript(t, "../../scripts/ci/last-built-commit.sh", repo, env, "shelly-provisioner")
	}

	// The newest latest version wins over an older one; a release and an untagged manifest are skipped;
	// the 7-hex short sha of the tag comes out as the full 40-hex sha, which is what the action needs.
	stdout, stderr, code := run(versions(tagOf(newer), tagOf(older)), "")
	if code != 0 || stdout != newer+"\n" || !regexp.MustCompile(`^[0-9a-f]{40}\n$`).MatchString(stdout) {
		t.Errorf("newest latest: exit %d, stdout %q, stderr %q, want %q", code, stdout, stderr, newer)
	}
	got, err := os.ReadFile(argsFile)
	if err != nil || !strings.Contains(string(got), "orgs/ginsys/packages/container/shelly-provisioner/versions") {
		t.Errorf("gh was called with %q (%v), want the versions of ginsys/shelly-provisioner", got, err)
	}

	for name, c := range map[string]struct{ json, fail string }{
		"no version tagged latest":    {`[{"id":1,"metadata":{"container":{"tags":["20261120-090000-aaaaaaa"]}}}]`, ""},
		"latest without a date tag":   {`[{"id":1,"metadata":{"container":{"tags":["latest"]}}}]`, ""},
		"no versions":                 {`[]`, ""},
		"a sha not in this history":   {versions("20261120-090000-0000000", "20261120-090000-0000000"), ""},
		"a failed read (rate limit)":  {"", "HTTP 403: API rate limit exceeded"},
		"a failed read (too many)":    {"", "HTTP 429: Too Many Requests"},
		"a failed read (no package)":  {"", "HTTP 404: Not Found"},
		"a tag with a non-hex suffix": {versions("20261120-090000-zzzzzzz", "20261120-090000-zzzzzzz"), ""},
	} {
		stdout, stderr, code = run(c.json, c.fail)
		if code != 0 || stdout != "" {
			t.Errorf("%s: exit %d, stdout %q, stderr %q, want exit 0 and nothing (the run builds)", name, code, stdout, stderr)
		}
		if c.fail != "" && !strings.Contains(stderr, "warning") {
			t.Errorf("%s: stderr %q, want a warning that the run has no base", name, stderr)
		}
	}
}

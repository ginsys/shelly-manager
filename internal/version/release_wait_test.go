package version

// Tests of scripts/release/wait-publish.sh, the Release workflow's last step, with a fake `gh`: the
// wait is the one part of the workflow no offline run reaches, so it is a script a test can drive.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// waitFakeGH is a `gh` that answers `run list` with list (or fails with listErr) and each `run view` with
// the next line of view (the last line repeats). A line `ERR:<text>` fails with <text> on stderr, and
// `EMPTY` answers nothing.
const waitFakeGH = `#!/usr/bin/env bash
case "$1 $2" in
"run list")
    if [ -n "${FAKE_LIST_ERR:-}" ]; then echo "$FAKE_LIST_ERR" >&2; exit 1; fi
    printf '%s' "${FAKE_LIST:-}"
    ;;
"run view")
    n=$(cat "$FAKE_DIR/n" 2>/dev/null || echo 0)
    n=$((n + 1))
    echo "$n" > "$FAKE_DIR/n"
    line=$(sed -n "${n}p" "$FAKE_DIR/view")
    [ -n "$line" ] || line=$(tail -n 1 "$FAKE_DIR/view")
    case "$line" in
    ERR:*) echo "${line#ERR:}" >&2; exit 1 ;;
    EMPTY) ;;
    *) echo "$line" ;;
    esac
    ;;
esac
`

type waitCase struct {
	name      string
	list      string   // the run id `run list` answers ("" = not started)
	listErr   string   // `run list` fails with this
	view      []string // the answers of `run view`, in order
	env       []string // tunables
	wantCode  int
	wantOut   string // in stdout or stderr
	maxViews  int    // `run view` calls allowed (0 = no limit)
	wantViews int    // at least this many (0 = none required)
}

func TestWaitPublish(t *testing.T) {
	fast := []string{"WAIT_POLL=0.05", "WAIT_START_TRIES=3", "WAIT_MISSES=3", "WAIT_DEADLINE=30"}
	cases := []waitCase{
		{name: "success after two waits", list: "123", view: []string{"in_progress", "in_progress", "success"},
			env: fast, wantOut: "completed", wantViews: 3},
		{name: "a failed Publish", list: "123", view: []string{"in_progress", "failure"},
			env: fast, wantCode: 1, wantOut: "concluded failure"},
		{name: "a cancelled Publish", list: "123", view: []string{"cancelled"},
			env: fast, wantCode: 1, wantOut: "concluded cancelled"},
		{name: "a run that never starts", list: "", env: fast, wantCode: 1, wantOut: "did not start"},
		{name: "a bad token on the run list", listErr: "HTTP 403: Resource not accessible by integration",
			env: fast, wantCode: 1, wantOut: "Resource not accessible"},
		{name: "a bad token on the run view", list: "123",
			view: []string{"ERR:HTTP 403: Resource not accessible by integration"},
			env:  fast, wantCode: 1, wantOut: "Resource not accessible", maxViews: 3, wantViews: 3},
		{name: "an empty answer again and again", list: "123", view: []string{"EMPTY"},
			env: fast, wantCode: 1, wantOut: "empty answer", maxViews: 3, wantViews: 3},
		{name: "one failed read between good ones", list: "123",
			view: []string{"in_progress", "ERR:HTTP 502", "in_progress", "ERR:HTTP 502", "success"},
			env:  fast, wantOut: "completed", wantViews: 5},
		{name: "a run that never finishes", list: "123", view: []string{"in_progress"},
			env:      []string{"WAIT_POLL=0.05", "WAIT_START_TRIES=3", "WAIT_MISSES=3", "WAIT_DEADLINE=1"},
			wantCode: 1, wantOut: "did not finish"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "bin", "gh"), waitFakeGH, 0o755)
			writeFile(t, filepath.Join(dir, "view"), strings.Join(append(c.view, ""), "\n"), 0o644)
			env := append([]string{
				"PATH=" + filepath.Join(dir, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
				"FAKE_DIR=" + dir, "FAKE_LIST=" + c.list, "FAKE_LIST_ERR=" + c.listErr,
				"GITHUB_REPOSITORY=ginsys/shelly-manager", "GH_TOKEN=x",
			}, c.env...)
			stdout, stderr, code := runScript(t, "../../scripts/release/wait-publish.sh", dir, env, "v0.6.0-alpha.1")
			if code != c.wantCode || !strings.Contains(stdout+stderr, c.wantOut) {
				t.Errorf("exit %d, want %d naming %q\nstdout: %s\nstderr: %s", code, c.wantCode, c.wantOut, stdout, stderr)
			}
			n := 0 // the `run view` reads the fake counted
			if b, err := os.ReadFile(filepath.Join(dir, "n")); err == nil {
				n, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			}
			if c.maxViews > 0 && n > c.maxViews {
				t.Errorf("%d `run view` reads, want at most %d: the wait did not stop", n, c.maxViews)
			}
			if n < c.wantViews {
				t.Errorf("%d `run view` reads, want at least %d", n, c.wantViews)
			}
		})
	}
}

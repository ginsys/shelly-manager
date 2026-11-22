package version

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// semVerPrerelease is the SemVer 2.0 grammar with a leading "v" and a required pre-release part.
var semVerPrerelease = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*)` +
	`(?:\+[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*)?$`)

// An unstamped build says "dev". The test binary is built without -X, so this reads the default.
func TestVersionDefaultsToDev(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("Version = %q, want %q for a build with no -X", Version, "dev")
	}
}

// The VERSION file holds one line: the next release, as a tag will carry it.
func TestVersionFileIsSemVerWithPrerelease(t *testing.T) {
	raw, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), "\n") || strings.Count(string(raw), "\n") != 1 {
		t.Fatalf("VERSION must be exactly one line ending in a newline, got %q", raw)
	}
	v := strings.TrimSuffix(string(raw), "\n")
	if !semVerPrerelease.MatchString(v) {
		t.Fatalf("VERSION %q is not SemVer 2.0 with a leading v and a pre-release part", v)
	}
}

// makeStamp runs `make -n build-manager` in the repository root, with no APP_VERSION or make flags
// inherited from the caller, and returns the version that the printed -X flag would stamp and whether
// the flag is printed at all: a flag with an empty value would overwrite "dev" with "". appVersion is
// passed on the command line unless it is the string "<unset>".
func makeStamp(t *testing.T, appVersion string) (value string, stamped bool) {
	t.Helper()
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not installed")
	}
	args := []string{"-C", "../..", "-n", "build-manager"}
	if appVersion != "<unset>" {
		args = append(args, "APP_VERSION="+appVersion)
	}
	cmd := exec.Command("make", args...)
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "APP_VERSION", "MAKEFLAGS", "MFLAGS", "MAKELEVEL":
		default:
			cmd.Env = append(cmd.Env, kv)
		}
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("make %v: %v", args, err)
	}
	const flag = "-X github.com/ginsys/shelly-manager/internal/version.Version="
	_, after, found := strings.Cut(string(out), flag)
	if !found {
		return "", false
	}
	value, _, _ = strings.Cut(after, "'")
	return value, true
}

// A plain build stamps the VERSION file plus -dev, a release build stamps the bare tag it is given,
// and an empty APP_VERSION stamps nothing, so the binary says "dev".
func TestMakeBuildStamp(t *testing.T) {
	raw, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	file := strings.TrimSpace(string(raw))

	cases := []struct {
		name        string
		appVersion  string
		wantStamped bool
		want        string
	}{
		{"plain build", "<unset>", true, file + "-dev"},
		{"release build", "v1.2.3", true, "v1.2.3"},
		{"empty APP_VERSION", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, stamped := makeStamp(t, tc.appVersion)
			if stamped != tc.wantStamped || got != tc.want {
				t.Fatalf("stamped=%v value %q, want stamped=%v value %q", stamped, got, tc.wantStamped, tc.want)
			}
		})
	}
}

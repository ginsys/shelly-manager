package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The deploy files, scripts and docs once named environment variables the app
// never reads (GIN_MODE, DATABASE_PROVIDER, SHELLY_PROVISIONING_AUTO, ...), and
// they configured nothing for whoever copied them. TestDeployEnvNames holds the
// kustomize tree to the names the app reads; this test holds everything else a
// person copies from (compose files, Dockerfiles, scripts, docs) to the same rule.

// docEnvScope is every file or directory a reader copies variable names from.
// The .env files are not read here: agents cannot open them, and they are fixed by hand.
var docEnvScope = []string{
	"../../README.md",
	"../../docs",
	"../../scripts",
	"../../deploy/docker",
	"../../deploy/docker-compose",
	"../../deploy/kubernetes/kustomize/README.md",
	"../../deploy/kubernetes/helm/README.md",
}

var (
	// A SHELLY_ name, whole: every letter, digit and underscore after the prefix, so a trailing
	// underscore or a lowercase suffix is part of the name and fails. A name that ends in an
	// underscore and a * is a wildcard in prose (SHELLY_DATABASE_*); a * after any other name is
	// Markdown emphasis and the name is checked.
	docShellyNameRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(SHELLY_[A-Za-z0-9_]*)(\*?)`)
	// A ${NAME} or ${NAME:-default} interpolation is not an app setting: Compose fills it from the
	// shell or a .env file. It is blanked out before the names are read, so a YAML flow mapping such
	// as {SHELLY_X: 1} is still read.
	// One level of nesting is read: ${A:-${B}}.
	docInterpolationRe = regexp.MustCompile(`\$\{(?:[^{}]|\$\{[^{}]*\})*\}`)
	// Names the app does not read, written without the SHELLY_ prefix. A ${NAME}
	// interpolation is not one: Compose fills it from the shell or a .env file. The
	// POSTGRES_* names are not here on purpose. POSTGRES_USER, _PASSWORD and _DB are
	// the postgres image's own, so they are right on the database container and
	// wrong on the app's, and POSTGRES_HOST and _PORT are shell variables in the
	// psql and pgbench examples. This test cannot tell those uses from a wrong one
	// in an app container, so those are fixed by reading.
	docStaleNameRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(GIN_MODE|DATABASE_PROVIDER|DATABASE_DSN)\b`)
	// A name Go code reads on its own, outside the config keys: os.Getenv("SHELLY_...").
	goEnvReadRe = regexp.MustCompile(`(?:Getenv|LookupEnv)\("(SHELLY_[A-Z0-9_]+)"`)
	// A name with a _FILE form: the quoted keys of secrets.ApplyToConfig, nothing else.
	secretFileKeyRe = regexp.MustCompile(`"(SHELLY_[A-Z0-9_]+)"`)
)

// secretFileKeys is every key that also takes a _FILE form, read from the one function that
// applies them. The secrets provider could read any key's _FILE, but only these are asked for it.
func secretFileKeys(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile("../security/secrets/apply.go")
	if err != nil {
		t.Fatalf("read secrets/apply.go: %v", err)
	}
	keys := map[string]bool{}
	for _, m := range secretFileKeyRe.FindAllStringSubmatch(string(src), -1) {
		keys[m[1]] = true
	}
	if len(keys) < 3 {
		t.Fatalf("found %d _FILE keys in secrets/apply.go; the scan is broken", len(keys))
	}
	return keys
}

// goReadEnvNames is every SHELLY_ name that Go code (tests included) reads
// through os.Getenv or os.LookupEnv, found by scanning the source.
func goReadEnvNames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, root := range []string{"../../cmd", "../../internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range goEnvReadRe.FindAllStringSubmatch(string(src), -1) {
				names[m[1]] = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
	return names
}

// docEnvFiles lists the files of docEnvScope a reader copies names from.
func docEnvFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range docEnvScope {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			base := d.Name()
			if strings.HasPrefix(base, ".env") {
				return nil
			}
			switch filepath.Ext(base) {
			case ".md", ".yml", ".yaml", ".sh":
				files = append(files, path)
			default:
				if strings.HasPrefix(base, "Dockerfile") {
					files = append(files, path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
	sort.Strings(files)
	return files
}

// docEnvProblems lists the variable names in text that the app does not read.
// A *_FILE name passes only when the name without the suffix is one of fileForm.
func docEnvProblems(source, text string, readable, fileForm map[string]bool) []string {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		where := source + ":" + strconv.Itoa(i+1) + ": "
		line = docInterpolationRe.ReplaceAllString(line, " ")
		for _, m := range docShellyNameRe.FindAllStringSubmatch(line, -1) {
			name := m[1]
			if name == "SHELLY_" || (m[2] == "*" && strings.HasSuffix(name, "_")) { // the bare prefix or a wildcard in prose
				continue
			}
			if readable[name] || (strings.HasSuffix(name, "_FILE") && fileForm[strings.TrimSuffix(name, "_FILE")]) {
				continue
			}
			out = append(out, where+name+" is not a name the app reads (SHELLY_ + config key, or a name Go code reads)")
		}
		for _, m := range docStaleNameRe.FindAllStringSubmatch(line, -1) {
			out = append(out, where+m[1]+" is not read by the app: use SHELLY_ + the config key")
		}
	}
	return out
}

func TestDocEnvNames(t *testing.T) {
	readable := readableEnvNames(t)
	fromGo := goReadEnvNames(t)
	if len(fromGo) < 3 {
		t.Fatalf("found only %d SHELLY_ names read through os.Getenv; the scan is broken", len(fromGo))
	}
	for name := range fromGo {
		readable[name] = true
	}
	fileForm := secretFileKeys(t)
	files := docEnvFiles(t)
	// About 70 files today; docs/ alone holds about 50, so a scope that loses it falls under the floor.
	if len(files) < 50 {
		t.Fatalf("read only %d files from the doc and deploy scope; the walk is broken", len(files))
	}
	for _, f := range files {
		text, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, p := range docEnvProblems(strings.TrimPrefix(f, "../../"), string(text), readable, fileForm) {
			t.Error(p)
		}
	}
}

// The checker is only worth running if it fails on the mistakes it is for.
func TestDocEnvNames_CheckerFindsTheMistakes(t *testing.T) {
	readable := map[string]bool{"SHELLY_DATABASE_PROVIDER": true, "SHELLY_API_KEY": true, "SHELLY_SERVER_PORT": true}
	fileForm := map[string]bool{"SHELLY_API_KEY": true}
	clean := "SHELLY_DATABASE_PROVIDER=postgresql\nSHELLY_API_KEY_FILE=/run/secrets/k\n" +
		"POSTGRES_USER=shelly\nPOSTGRES_PASSWORD=x\nPOSTGRES_DB=shelly\npsql -h \"$POSTGRES_HOST\"\n" +
		"      - SHELLY_DATABASE_PROVIDER=${DATABASE_PROVIDER:-sqlite}\n" +
		"      - SHELLY_DATABASE_PROVIDER=${SHELLY_DB_BACKEND:-sqlite}\n" +
		"      - SHELLY_API_KEY=${API_KEY:-${PREFIX:-}SHELLY_FALLBACK}\n" +
		"The app reads SHELLY_ plus the key, as in SHELLY_* or SHELLY_DATABASE_*.\n"
	if got := docEnvProblems("clean.md", clean, readable, fileForm); len(got) != 0 {
		t.Fatalf("clean text reported: %v", got)
	}
	for name, text := range map[string]string{
		"unread SHELLY_ name":   "SHELLY_HTTP_PORT=8081 ./bin/app\n",
		"unread _FILE name":     "SHELLY_HTTP_PORT_FILE=/x\n",
		"_FILE of a plain key":  "SHELLY_DATABASE_PROVIDER_FILE=/x\n",
		"other suffix":          "SHELLY_API_KEY_PATH=/x\n",
		"trailing underscore":   "SHELLY_SERVER_PORT_=8081\n",
		"in bold":               "set **SHELLY_HTTP_PORT** to 8081\n",
		"in italics":            "set *SHELLY_HTTP_PORT* to 8081\n",
		"in a flow mapping":     "environment: {SHELLY_HTTP_PORT: 8081}\n",
		"stale in a flow map":   "environment: {GIN_MODE: release}\n",
		"lowercase suffix":      "SHELLY_SERVER_PORT_typo=8081\n",
		"dead GIN_MODE":         "GIN_MODE=release \\\n",
		"unprefixed provider":   "export DATABASE_PROVIDER=sqlite\n",
		"unprefixed DSN":        "      - DATABASE_DSN=${DATABASE_DSN}\n",
		"after a punctuation":   "(GIN_MODE)\n",
		"quoted unread name":    "`SHELLY_PROVISIONING_AUTO`\n",
		"stale name in a table": "| DATABASE_DSN | the DSN |\n",
	} {
		if got := docEnvProblems("x.md", text, readable, fileForm); len(got) == 0 {
			t.Errorf("%s: the checker reported nothing for %q", name, text)
		}
	}
}

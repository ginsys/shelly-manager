package version

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// dryRunExpr is the one dry-run value of the Release workflow. On a pull_request event the workflow
// input `inputs.dry_run` is empty, so a test of `inputs.dry_run` alone would read a pull request as a
// real run: it would push images or publish a release from a pull request.
const dryRunExpr = "github.event_name == 'pull_request' || inputs.dry_run"

// dryRunMisuse reads a workflow file and returns how many `if:` and `with:` values name dry_run in any
// form, and the ones that name it outside dryRunExpr. Every mention must sit inside the expression.
func dryRunMisuse(t *testing.T, raw []byte) (checked int, bad []string) {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("workflow does not parse: %v", err)
	}
	check := func(where, value string) {
		if !strings.Contains(value, "dry_run") {
			return
		}
		checked++
		if strings.Contains(strings.ReplaceAll(value, dryRunExpr, ""), "dry_run") {
			bad = append(bad, fmt.Sprintf("%s: %q", where, value))
		}
	}
	var visit func(where string, node any)
	visit = func(where string, node any) {
		switch n := node.(type) {
		case map[string]any:
			for key, v := range n {
				switch key {
				case "if":
					check(where+".if", fmt.Sprint(v))
				case "with":
					if args, ok := v.(map[string]any); ok {
						for name, a := range args {
							check(where+".with."+name, fmt.Sprint(a))
						}
					}
				}
				visit(where+"."+key, v)
			}
		case []any:
			for i, v := range n {
				visit(fmt.Sprintf("%s[%d]", where, i), v)
			}
		}
	}
	jobs, _ := doc["jobs"].(map[string]any)
	if jobs == nil {
		t.Fatal("workflow has no jobs")
	}
	visit("jobs", jobs)
	return checked, bad
}

// TestReleaseCallerDryRunExpression pins that every dry-run test in release.yml is the one expression
// that also covers a pull_request event, with a broken copy that the check must reject.
func TestReleaseCallerDryRunExpression(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	checked, bad := dryRunMisuse(t, raw)
	if checked == 0 {
		t.Fatal("found no if: or with: value that names dry_run, so the check read nothing")
	}
	for _, b := range bad {
		t.Errorf("a dry-run test is not the one expression %q: %s", dryRunExpr, b)
	}

	// The check must reject a copy that tests inputs.dry_run alone, in a with: value and in an if:.
	broken := strings.ReplaceAll(string(raw), dryRunExpr, "inputs.dry_run")
	if _, bad = dryRunMisuse(t, []byte(broken)); len(bad) == 0 {
		t.Error("a copy that passes inputs.dry_run alone was accepted")
	}
	const bare = "jobs:\n  images:\n    if: ${{ !inputs.dry_run }}\n    uses: ./.github/workflows/x.yml\n"
	if checked, bad = dryRunMisuse(t, []byte(bare)); checked != 1 || len(bad) != 1 {
		t.Errorf("a bare if: !inputs.dry_run: checked %d, bad %d, want 1 and 1", checked, len(bad))
	}
}

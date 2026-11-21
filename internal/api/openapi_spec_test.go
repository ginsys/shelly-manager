package api

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// ginsys/shelly-manager#816: a viewer, a client generator or a lint reads docs/api/openapi.yaml, and a plain
// scalar holding ": " made it unparseable with nothing in CI to say so.
func TestOpenAPISpecParses(t *testing.T) {
	raw, err := os.ReadFile("../../docs/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("docs/api/openapi.yaml does not parse: %v", err)
	}
	for _, key := range []string{"openapi", "info", "paths"} {
		if spec[key] == nil {
			t.Errorf("docs/api/openapi.yaml has no top-level %q", key)
		}
	}
	if paths, _ := spec["paths"].(map[string]any); len(paths) == 0 {
		t.Error("docs/api/openapi.yaml lists no paths")
	}
}

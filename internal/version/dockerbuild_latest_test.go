package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDockerBuildKeepsLatestOffReleases pins that `latest` means the newest main image: a call of
// docker-build.yml with a version input (a release or a pre-release) must not tag or push it, and
// every other trigger keeps pushing it. Both image tag lists carry the same switch, so a release
// can never move `latest` on either image.
func TestDockerBuildKeepsLatestOffReleases(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "docker-build.yml"))
	if err != nil {
		t.Fatal(err)
	}
	const (
		bare = "type=raw,value=latest"
		// An empty version input is every trigger but a release call, so the tag stays on for
		// them and goes off exactly when a version is given.
		want = "type=raw,value=latest,enable=${{ inputs.version == '' }}"
	)
	var found int
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, bare) {
			continue
		}
		found++
		if line != want {
			t.Errorf("a latest tag line is %q, want %q", line, want)
		}
	}
	if found != 2 {
		t.Errorf("found %d latest tag lines (one per image), want 2", found)
	}
}

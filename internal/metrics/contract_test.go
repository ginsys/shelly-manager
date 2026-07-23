package metrics

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manifestPath locates the frontend message-type manifest relative to this
// package (internal/metrics -> repo root -> ui/...).
const manifestPath = "../../ui/src/api/metricsMessages.ts"

var (
	manifestArrayRe = regexp.MustCompile(`(?s)METRICS_WS_MESSAGE_TYPES\s*=\s*\[(.*?)\]\s*as const`)
	manifestTokenRe = regexp.MustCompile(`'([^']+)'`)
)

// TestMessageTypeManifestParity enforces the Go/TS WebSocket contract across the
// language boundary: the frontend's METRICS_WS_MESSAGE_TYPES manifest must list
// exactly the types the Go hub can emit (AllMessageTypes). A backend type added
// without updating the manifest — or a stale frontend entry — fails here in CI,
// which a frontend-only switch could never detect.
func TestMessageTypeManifestParity(t *testing.T) {
	path, err := filepath.Abs(manifestPath)
	require.NoError(t, err)

	raw, err := os.ReadFile(path)
	require.NoError(t, err, "frontend manifest not found at %s", path)

	block := manifestArrayRe.FindSubmatch(raw)
	require.Len(t, block, 2, "could not locate METRICS_WS_MESSAGE_TYPES array literal in %s", path)

	var manifest []string
	for _, m := range manifestTokenRe.FindAllStringSubmatch(string(block[1]), -1) {
		manifest = append(manifest, m[1])
	}

	require.NotEmpty(t, manifest, "manifest array parsed as empty")
	assert.ElementsMatch(t, AllMessageTypes(), manifest,
		"Go AllMessageTypes() and frontend METRICS_WS_MESSAGE_TYPES are out of sync; "+
			"update ui/src/api/metricsMessages.ts to match internal/metrics/websocket.go")
}

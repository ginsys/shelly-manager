package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
)

func postGitOpsExport(t *testing.T, router http.Handler, outputPath string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(map[string]interface{}{
		"config": map[string]interface{}{"output_path": outputPath},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export/gitops", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// A request that points the GitOps export at a directory it did not write is refused with a 400
// that names the path, and the files in it are left alone. With no export base directory the
// engine accepts any absolute path, so this is the only thing between a request and the delete.
func TestGitOpsExportRefusesAForeignDirectory(t *testing.T) {
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	require.NoError(t, engine.RegisterPlugin(gitops.NewPlugin()))

	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	require.NoError(t, os.WriteFile(keep, []byte("mine"), 0o600))

	rr := postGitOpsExport(t, router, dir)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), dir)
	b, err := os.ReadFile(keep)
	require.NoError(t, err)
	require.Equal(t, "mine", string(b))
}

// An empty directory is a valid target, so the same route still works for a first export.
func TestGitOpsExportIntoAnEmptyDirectory(t *testing.T) {
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	require.NoError(t, engine.RegisterPlugin(gitops.NewPlugin()))

	dir := t.TempDir()
	rr := postGitOpsExport(t, router, dir)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.FileExists(t, filepath.Join(dir, "export-summary.yaml"))
}

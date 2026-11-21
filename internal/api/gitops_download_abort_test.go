package api

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
)

// A download that fails after zip bytes reached the client must end as a failed transfer. This runs
// through the production router with its whole middleware chain on a real server: the recovery and
// timeout layers both sit between the handler and the client, and each used to swallow the abort.
func TestGitOpsDownloadThatFailsMidBodyAbortsTheTransfer(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 file")
	}
	t.Setenv(testModeEnv, "")
	base := t.TempDir()
	h, logger := guardWalkHandler(t, "", "")
	require.NoError(t, h.ExportHandlers.syncEngine.RegisterPlugin(gitops.NewPlugin()))
	h.ExportHandlers.SetExportBaseDir(base)
	h.ExportHandlers.syncEngine.SetExportBaseDir(base)
	router := SetupRoutesWithSecurity(h, logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())
	closeMonitor(t, h)

	dir := filepath.Join(base, "gitops")
	body, err := json.Marshal(map[string]any{"config": map[string]any{
		"name": "nightly", "output_path": dir, "group_by": "location",
		"include_common": true, "include_templates": true,
	}})
	require.NoError(t, err)
	created := authBodyDo(router, http.MethodPost, "/api/v1/export/gitops", string(body), nil)
	require.Equal(t, http.StatusOK, created.Code, created.Body.String())
	var wrap struct {
		Data struct {
			ExportID string `json:"export_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &wrap))
	require.NotEmpty(t, wrap.Data.ExportID)

	// The first file is big and incompressible, so zip bytes reach the client before the walk meets
	// the unreadable one, which sorts after every file the export wrote.
	big := make([]byte, 256<<10)
	_, err = rand.Read(big)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "0-big.bin"), big, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zz-locked.yaml"), []byte("x"), 0o000))

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/export/gitops/"+wrap.Data.ExportID+"/download", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, readErr := io.ReadAll(resp.Body)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
	require.Error(t, readErr, "a zip cut short must reach the client as a failed transfer, not a 200 that ends whole")
}

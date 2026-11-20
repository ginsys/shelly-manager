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

	"github.com/ginsys/shelly-manager/internal/plugins/sync/backup"
	"github.com/ginsys/shelly-manager/internal/plugins/sync/registry"
)

// The generic import and its preview both take a source path from the client. For the backup plugin
// that path used to reach a restore that replaces the live database, whatever the dry-run flag said.
// Both routes now answer 501 and leave the database file as it was.
func TestImportOfABackupIsRefusedOnBothRoutes(t *testing.T) {
	router, engine, _, db, cleanup := setupSyncTestRouter(t)
	defer cleanup()
	require.NoError(t, engine.RegisterPlugin(backup.NewBackupExporter(&registry.DatabaseManagerAdapter{Manager: db})))

	backupFile := filepath.Join(t.TempDir(), "shelly-backup-1.sqlite")
	require.NoError(t, os.WriteFile(backupFile, []byte("not a database"), 0o600))

	for _, route := range []string{"/api/v1/import", "/api/v1/import/preview"} {
		for _, dry := range []bool{false, true} {
			body, err := json.Marshal(map[string]interface{}{
				"plugin_name": "backup",
				"format":      "sqlite",
				"source":      map[string]interface{}{"type": "file", "path": backupFile},
				"config":      map[string]interface{}{},
				"options":     map[string]interface{}{"dry_run": dry},
			})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			require.Equal(t, http.StatusNotImplemented, rr.Code, "%s dry_run=%v: %s", route, dry, rr.Body.String())
			require.Contains(t, rr.Body.String(), "NOT_IMPLEMENTED")
		}
	}
}

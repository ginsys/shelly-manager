package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// gitopsRoutesRig is the sync router with the real GitOps plugin registered, so a test drives
// the same create, list and delete calls the GitOps page makes.
func gitopsRoutesRig(t *testing.T) http.Handler {
	t.Helper()
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	t.Cleanup(cleanup)
	require.NoError(t, engine.RegisterPlugin(gitops.NewPlugin()))
	return router
}

func callGitOps(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Test)")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// createGitOpsExport posts what the GitOps page posts and returns the export id and the
// directory the plugin wrote.
func createGitOpsExport(t *testing.T, h http.Handler) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "gitops")
	rr := callGitOps(t, h, http.MethodPost, "/api/v1/export/gitops", map[string]any{
		"config": map[string]any{
			"name":              "nightly",
			"output_path":       dir,
			"group_by":          "location",
			"include_common":    true,
			"include_templates": true,
		},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var wrap struct {
		Data struct {
			ExportID string `json:"export_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	require.NotEmpty(t, wrap.Data.ExportID)
	return wrap.Data.ExportID, dir
}

func dirContentSize(t *testing.T, dir string) int64 {
	t.Helper()
	var total int64
	require.NoError(t, filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	}))
	return total
}

// The history list is the page's list, so these are the fields the page reads.
// A failure here means the page and the route have drifted apart again.
func TestGitOpsHistoryCarriesTheFieldsThePageReads(t *testing.T) {
	h := gitopsRoutesRig(t)
	id, dir := createGitOpsExport(t, h)

	rr := callGitOps(t, h, http.MethodGet, "/api/v1/export/history?plugin=gitops&page_size=1", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var wrap struct {
		Data struct {
			History []map[string]any `json:"history"`
		} `json:"data"`
		Meta struct {
			Pagination struct {
				TotalPages int `json:"total_pages"`
			} `json:"pagination"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	// With page_size=1 the page count is the item count: the page reads its totals from it.
	require.Equal(t, 1, wrap.Meta.Pagination.TotalPages)
	require.Len(t, wrap.Data.History, 1)
	rec := wrap.Data.History[0]
	for _, key := range []string{"export_id", "name", "format", "success", "record_count", "file_size", "file_path", "created_at"} {
		require.Contains(t, rec, key)
	}
	require.Equal(t, id, rec["export_id"])
	require.Equal(t, "nightly", rec["name"])
	require.Equal(t, dir, rec["file_path"])
}

// The history size of a GitOps export is the size of its files, not the size of the directory
// entry the operating system reports for a directory.
func TestGitOpsHistoryFileSizeIsTheSizeOfTheFiles(t *testing.T) {
	h := gitopsRoutesRig(t)
	id, dir := createGitOpsExport(t, h)

	rr := callGitOps(t, h, http.MethodGet, "/api/v1/export/history/"+id, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var wrap struct {
		Data struct {
			FileSize int64 `json:"file_size"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	want := dirContentSize(t, dir)
	require.NotZero(t, want, "the export wrote no files")
	require.Equal(t, want, wrap.Data.FileSize)
}

func TestDeleteGitOpsExportRemovesTheRecordAndKeepsTheFiles(t *testing.T) {
	h := gitopsRoutesRig(t)
	id, dir := createGitOpsExport(t, h)
	before := dirContentSize(t, dir)

	rr := callGitOps(t, h, http.MethodDelete, "/api/v1/export/gitops/"+id, nil)
	require.Equal(t, http.StatusNoContent, rr.Code, rr.Body.String())

	rr = callGitOps(t, h, http.MethodGet, "/api/v1/export/history/"+id, nil)
	require.Equal(t, http.StatusNotFound, rr.Code, "the history record is still there")
	_, err := os.Stat(filepath.Join(dir, "export-summary.yaml"))
	require.NoError(t, err, "the delete removed the exported files")
	require.Equal(t, before, dirContentSize(t, dir))
}

func TestDeleteGitOpsExportRefusesWhatIsNotAGitOpsExport(t *testing.T) {
	h := gitopsRoutesRig(t)

	// A record of another plugin: the mock plugin's export.
	out := filepath.Join(t.TempDir(), "export.txt")
	rr := callGitOps(t, h, http.MethodPost, "/api/v1/export", map[string]any{
		"plugin_name": "mockfile",
		"format":      "txt",
		"output":      map[string]any{"type": "file", "destination": out},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var wrap struct {
		Data struct {
			ExportID string `json:"export_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	other := wrap.Data.ExportID
	require.NotEmpty(t, other)
	if rr = callGitOps(t, h, http.MethodGet, "/api/v1/export/history/"+other, nil); rr.Code != http.StatusOK {
		t.Skip("the mock export left no history record to protect")
	}

	rr = callGitOps(t, h, http.MethodDelete, "/api/v1/export/gitops/"+other, nil)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	rr = callGitOps(t, h, http.MethodGet, "/api/v1/export/history/"+other, nil)
	require.Equal(t, http.StatusOK, rr.Code, "a GitOps delete removed another plugin's record")

	rr = callGitOps(t, h, http.MethodDelete, "/api/v1/export/gitops/no-such-export", nil)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

// The GitOps page counts its exports from `total_pages` of a one-item page: the response meta
// has no total. This pins that detail, so a change to the pagination meta fails here and not in
// the page (ui/src/api/export.ts, countGitOpsExports).
func TestHistoryPageCountWithPageSizeOneIsTheRowCount(t *testing.T) {
	router, engine, _, _, cleanup := setupSyncTestRouter(t)
	defer cleanup()

	totalPages := func(query string) int {
		t.Helper()
		rr := callGitOps(t, router, http.MethodGet, "/api/v1/export/history?page_size=1&"+query, nil)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var wrap struct {
			Meta struct {
				Pagination struct {
					TotalPages int `json:"total_pages"`
				} `json:"pagination"`
			} `json:"meta"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
		return wrap.Meta.Pagination.TotalPages
	}
	require.Equal(t, 0, totalPages("plugin=gitops"))
	require.Equal(t, 0, totalPages("plugin=gitops&success=true"))

	save := func(plugin, id string, ok bool) {
		t.Helper()
		req := sync.ExportRequest{PluginName: plugin, Format: "yaml"}
		res := &sync.ExportResult{Success: ok, ExportID: id}
		require.NoError(t, engine.SaveExportHistory(context.Background(), req, res, "test"))
	}
	save("gitops", "g1", true)
	save("gitops", "g2", true)
	save("gitops", "g3", false)
	save("mockfile", "m1", true)

	require.Equal(t, 3, totalPages("plugin=gitops"))
	require.Equal(t, 2, totalPages("plugin=gitops&success=true"))
}

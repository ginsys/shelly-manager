package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/plugins/sync/gitops"
)

const gitopsMarker = ".shelly-manager-gitops"

// gitopsDownloadRig is the export router with the real GitOps plugin and a base directory, so a
// test drives the same create and download calls the page makes.
func gitopsDownloadRig(t *testing.T, base string) (*mux.Router, *SyncHandlers) {
	t.Helper()
	router, exp, _, cleanup := setupSecuredRouter(t, "")
	t.Cleanup(cleanup)
	require.NoError(t, exp.syncEngine.RegisterPlugin(gitops.NewPlugin()))
	if base != "" {
		exp.SetExportBaseDir(base)
		exp.syncEngine.SetExportBaseDir(base)
	}
	return router, exp
}

func createGitOpsExportAt(t *testing.T, router http.Handler, dir string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"config": map[string]any{
		"name": "nightly", "output_path": dir, "group_by": "location",
		"include_common": true, "include_templates": true,
	}})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export/gitops", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var wrap struct {
		Data struct {
			ExportID string `json:"export_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wrap))
	require.NotEmpty(t, wrap.Data.ExportID)
	return wrap.Data.ExportID
}

func downloadGitOps(router http.Handler, id string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/export/gitops/"+id+"/download", nil))
	return rr
}

func zipEntries(t *testing.T, body []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err, "the body is not a zip")
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		out[f.Name] = string(data)
	}
	return out
}

// regularFilesBelow is what the archive has to hold: every regular file, by its slash path below dir,
// without the marker the plugin keeps for itself.
func regularFilesBelow(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() || d.Name() == gitopsMarker {
			return err
		}
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		rel, err := filepath.Rel(dir, p)
		require.NoError(t, err)
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	}))
	return out
}

func TestGitOpsDownloadIsAZipOfTheExportTree(t *testing.T) {
	base := t.TempDir()
	router, _ := gitopsDownloadRig(t, base)
	dir := filepath.Join(base, "gitops")
	id := createGitOpsExportAt(t, router, dir)

	// A nested file the export did not write: the archive keeps relative paths at any depth.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "groups", "kitchen"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "groups", "kitchen", "lamp.yaml"), []byte("name: lamp\n"), 0o644))

	rr := downloadGitOps(router, id)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "application/zip", rr.Header().Get("Content-Type"))
	require.Regexp(t, `^attachment; filename="[^"/]+\.zip"$`, rr.Header().Get("Content-Disposition"))

	got := zipEntries(t, rr.Body.Bytes())
	want := regularFilesBelow(t, dir)
	require.Contains(t, want, "export-summary.yaml")
	require.Contains(t, want, "groups/kitchen/lamp.yaml")
	require.Equal(t, want, got, "the archive holds the exported files and nothing else")
	require.NotContains(t, got, gitopsMarker)
}

func TestGitOpsDownloadNeverFollowsASymlinkOutOfTheTree(t *testing.T) {
	base := t.TempDir()
	router, _ := gitopsDownloadRig(t, base)
	dir := filepath.Join(base, "gitops")
	id := createGitOpsExportAt(t, router, dir)

	outside := t.TempDir()
	const secret = "outside-secret-4711"
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte(secret), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "leak-file.txt")))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "leak-dir")))

	rr := downloadGitOps(router, id)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	got := zipEntries(t, rr.Body.Bytes())
	for name := range got {
		require.NotContains(t, name, "leak", "a symlink is skipped, never archived")
	}
	require.NotContains(t, rr.Body.String(), secret)
	require.Contains(t, got, "export-summary.yaml")
}

func TestGitOpsDownloadRefusesASymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	router, _ := gitopsDownloadRig(t, base)
	dir := filepath.Join(base, "gitops")
	id := createGitOpsExportAt(t, router, dir)

	// Another marked tree inside the base directory, and the record's path now points at it through a link.
	other := filepath.Join(base, "other")
	require.NoError(t, os.MkdirAll(other, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(other, gitopsMarker), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(other, "private.yaml"), []byte("private-9\n"), 0o644))
	require.NoError(t, os.RemoveAll(dir))
	require.NoError(t, os.Symlink(other, dir))

	rr := downloadGitOps(router, id)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	require.NotContains(t, rr.Body.String(), "private-9")
}

// The record's path is what the request supplied, and the directory may have been repurposed since:
// only a directory that still holds the plugin's marker as a regular file is zipped.
func TestGitOpsDownloadRefusesADirectoryWithoutTheMarker(t *testing.T) {
	base := t.TempDir()
	router, _ := gitopsDownloadRig(t, base)
	dir := filepath.Join(base, "gitops")
	id := createGitOpsExportAt(t, router, dir)

	const private = "not-an-export-5150"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(private), 0o644))

	require.NoError(t, os.Remove(filepath.Join(dir, gitopsMarker)))
	rr := downloadGitOps(router, id)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	require.NotContains(t, rr.Body.String(), private)

	// A symlink in the marker's place proves nothing either.
	require.NoError(t, os.Symlink(filepath.Join(dir, "notes.txt"), filepath.Join(dir, gitopsMarker)))
	rr = downloadGitOps(router, id)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	require.NotContains(t, rr.Body.String(), private)
}

// A failure before the first byte is the normal error answer, not an empty zip with a 200.
func TestGitOpsDownloadAnswersAnErrorWhenNothingWasSentYet(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 file")
	}
	base := t.TempDir()
	router, _ := gitopsDownloadRig(t, base)
	dir := filepath.Join(base, "gitops")
	id := createGitOpsExportAt(t, router, dir)
	// Sorts before every file the export wrote, so the first file the walk reads is the unreadable one.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "0-locked.yaml"), []byte("x"), 0o000))

	rr := downloadGitOps(router, id)
	require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	require.NotEqual(t, "application/zip", rr.Header().Get("Content-Type"))
	require.Empty(t, rr.Header().Get("Content-Disposition"))
}

// The walk and the open are two steps. A directory swapped for a symlink in between must not be
// followed out of the tree: the open goes through the root, which refuses it.
func TestAddZipFileRefusesAPathThatNowLeavesTheTree(t *testing.T) {
	tree, outside := t.TempDir(), t.TempDir()
	secretPath := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secretPath, []byte("outside-secret-4711"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(tree, "sub")))
	root, err := os.OpenRoot(tree)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	info, err := os.Lstat(secretPath)
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// The walk saw sub/secret.txt as a regular file; by the time of the open, sub is the link.
	require.Error(t, addZipFile(zw, root, "sub/secret.txt", fs.FileInfoToDirEntry(info)))
	require.NoError(t, zw.Close())
	require.NotContains(t, buf.String(), "outside-secret-4711")
}

// The Lstat and the open are two steps too: a path replaced in between by a link to another directory
// is not the directory that was seen, and is refused.
func TestOpenExportRootRefusesADirectoryOtherThanTheOneSeen(t *testing.T) {
	seenDir, other := t.TempDir(), t.TempDir()
	seen, err := os.Lstat(seenDir)
	require.NoError(t, err)

	root, err := openExportRoot(seenDir, seen)
	require.NoError(t, err)
	require.NoError(t, root.Close())

	_, err = openExportRoot(other, seen)
	require.Error(t, err)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(other, link))
	_, err = openExportRoot(link, seen)
	require.Error(t, err)
}

func TestGitOpsDownloadKeepsTheBaseDirectoryContainment(t *testing.T) {
	base := t.TempDir()
	router, exp := gitopsDownloadRig(t, "")
	dir := filepath.Join(t.TempDir(), "gitops") // outside the base directory set below
	id := createGitOpsExportAt(t, router, dir)
	exp.SetExportBaseDir(base)

	rr := downloadGitOps(router, id)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	require.False(t, strings.HasPrefix(rr.Header().Get("Content-Type"), "application/zip"))
}

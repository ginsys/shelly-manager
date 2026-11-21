package api

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// serveExportPath serves what an export wrote at path: a file as it is, a directory as a zip.
// The caller has already checked that path lies inside the export base directory.
func (eh *SyncHandlers) serveExportPath(w http.ResponseWriter, r *http.Request, path string) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		eh.serveDirZip(w, r, path)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(path)))
	setContentTypeForPath(w, path)
	http.ServeFile(w, r, path)
}

// serveDirZip streams a zip of an export directory, built as it is sent. The record's path is what the
// request supplied and the directory may have been repurposed since, so only a directory that is not
// itself a symlink and still holds the export marker is zipped. The tree is read through an os.Root, so
// a directory swapped for a symlink while the zip is built is never followed out of it; symlinks inside
// the tree are skipped, and the marker is left out.
func (eh *SyncHandlers) serveDirZip(w http.ResponseWriter, r *http.Request, path string) {
	rw := apiresp.NewResponseWriter(eh.logger)
	refuse := func() {
		rw.WriteError(w, r, http.StatusForbidden, apiresp.ErrCodeForbidden, "Download path is not an export directory", nil)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		refuse()
		return
	}
	root, err := openExportRoot(path, info)
	if err != nil {
		refuse()
		return
	}
	defer func() { _ = root.Close() }()
	marker, err := root.Lstat(sync.ExportDirMarker)
	if err != nil || !marker.Mode().IsRegular() {
		refuse()
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", filepath.Base(path)))
	sent := &countingWriter{w: w}
	zw := zip.NewWriter(sent)
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() || name == sync.ExportDirMarker {
			return err
		}
		return addZipFile(zw, root, name, d)
	})
	if err == nil {
		err = zw.Close()
	}
	if err == nil {
		return
	}
	eh.logger.Error("GitOps export download failed", "path", path, "error", err)
	if sent.n == 0 {
		// Nothing has reached the client: answer with the normal error instead of an empty zip.
		w.Header().Del("Content-Disposition")
		rw.WriteInternalError(w, r, errors.New("could not build the export archive"))
	}
	// Otherwise the status is sent. A zip with no central directory is rejected by every reader, so
	// the client sees a failed download and not a short one that looks whole.
}

// openExportRoot opens path as a root and keeps it only if it is the directory seen holds, so a path
// replaced by a symlink between the Lstat and the open is not followed to another directory.
func openExportRoot(path string, seen os.FileInfo) (*os.Root, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(seen, opened) {
		_ = root.Close()
		return nil, errors.New("the export directory changed while it was opened")
	}
	return root, nil
}

// addZipFile adds one regular file, opened through the root so a path that now leaves the tree is
// refused. It then checks that what it holds is still the entry the walk saw.
func addZipFile(zw *zip.Writer, root *os.Root, name string, d fs.DirEntry) error {
	seen, err := d.Info()
	if err != nil {
		return err
	}
	f, err := root.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(seen, opened) {
		return nil
	}
	hdr, err := zip.FileInfoHeader(opened)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	out, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, f)
	return err
}

// countingWriter counts what reached the client, to tell a failure before the first byte from one after.
type countingWriter struct {
	w http.ResponseWriter
	n int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += n
	return n, err
}

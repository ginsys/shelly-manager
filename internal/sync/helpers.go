package sync

import (
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// FileSHA256 calculates the SHA-256 checksum of a file.
// Returns hex-encoded string of the checksum.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// WriteGzip compresses data using gzip and writes to path.
func WriteGzip(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	if _, err := gz.Write(data); err != nil {
		_ = gz.Close()
		return fmt.Errorf("failed to write gzip: %w", err)
	}

	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip: %w", err)
	}

	return f.Sync()
}

// WriteZipSingle creates a ZIP archive with a single file entry.
// entryName is the name of the file inside the ZIP.
func WriteZipSingle(path string, entryName string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: entryName, Method: zip.Deflate}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		_ = zw.Close()
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		_ = zw.Close()
		return fmt.Errorf("failed to write zip entry: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close zip: %w", err)
	}

	return f.Sync()
}

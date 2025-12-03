package sync

import (
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// FileSHA256 returns the hex-encoded SHA-256 checksum of path.
// Used to verify export integrity and detect file changes.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// WriteGzip writes gzip-compressed data to path using the default level.
func WriteGzip(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("write gzip: %w", err)
	}

	// Let defer handle gz.Close()
	return f.Sync()
}

// WriteZipSingle creates a ZIP archive at path with a single entry (entryName).
// Prefer gzip for single files unless ZIP compatibility is required.
func WriteZipSingle(path, entryName string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	w, err := zw.Create(entryName)
	if err != nil {
		return fmt.Errorf("create zip entry: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write zip entry: %w", err)
	}

	// Let defer handle zw.Close()
	return f.Sync()
}

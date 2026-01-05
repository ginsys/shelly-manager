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
//
// Use Case: Verify export integrity, detect file changes
// For hobbyist project: SHA-256 provides good balance of speed and security
//
// Example:
//
//	checksum, err := sync.FileSHA256("/path/to/export.json")
//	if err != nil { return err }
//	fmt.Printf("Export checksum: %s\n", checksum)
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// WriteGzip compresses data using gzip and writes to path.
//
// Use Case: Reduce file size for JSON/YAML exports (typically 70-80% reduction)
// Compression level: Best (level 9) - acceptable for hobbyist use
//
// Example:
//
//	data := []byte(`{"devices": [...]}`)
//	err := sync.WriteGzip("/tmp/export.json.gz", data)
func WriteGzip(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()

	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("failed to write gzip: %w", err)
	}

	// Let defer handle gz.Close()
	return f.Sync()
}

// WriteZipSingle creates a ZIP archive with a single file entry.
// entryName is the name of the file inside the ZIP.
//
// Use Case: Windows-friendly compression, better for multiple files (future)
// Note: For single files, GZIP is more efficient. Use ZIP for Windows compatibility.
//
// Example:
//
//	data := []byte(`{"devices": [...]}`)
//	err := sync.WriteZipSingle("/tmp/export.zip", "export.json", data)
func WriteZipSingle(path string, entryName string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	defer func() { _ = zw.Close() }()

	hdr := &zip.FileHeader{Name: entryName, Method: zip.Deflate}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write zip entry: %w", err)
	}

	// Let defer handle zw.Close()
	return f.Sync()
}

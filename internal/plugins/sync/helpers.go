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
// Use Case: Verify export integrity, detect file changes.
// For hobbyist projects, SHA-256 provides a good balance of speed and security.
//
// Example:
//
//	checksum, err := FileSHA256("/path/to/export.json")
//	if err != nil { return err }
//	fmt.Printf("Export checksum: %s\n", checksum)
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

// WriteGzip compresses data using gzip and writes to path.
//
// Use Case: Reduce file size for JSON/YAML exports (typically 70–80% reduction).
// Compression level uses the default configured by gzip.NewWriter (suitable balance).
//
// Example:
//
//	data := []byte(`{"devices": [...]}`)
//	_ = WriteGzip("/tmp/export.json.gz", data)
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

// WriteZipSingle creates a ZIP archive with a single file entry.
// entryName is the name of the file inside the ZIP.
//
// Use Case: Windows-friendly compression; consider for multi-file archives.
// Note: For single files, gzip is often more efficient; use ZIP for compatibility.
//
// Example:
//
//	data := []byte(`{"devices": [...]}`)
//	_ = WriteZipSingle("/tmp/export.zip", "export.json", data)
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

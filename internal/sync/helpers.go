package sync

import (
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// The helpers below take the opened export root (OpenOutputRoot) and a name below it. The root refuses a name
// that leaves it, by `..` or by a symbolic link swapped in after the path check.

// FileSHA256 calculates the SHA-256 checksum of a file below the root.
// Returns hex-encoded string of the checksum.
//
// Use Case: Verify export integrity, detect file changes
// For hobbyist project: SHA-256 provides good balance of speed and security
//
// Example:
//
//	checksum, err := sync.FileSHA256(root, "export.json")
//	if err != nil { return err }
//	fmt.Printf("Export checksum: %s\n", checksum)
func FileSHA256(root *os.Root, name string) (string, error) {
	f, err := root.Open(name)
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

// WriteGzip compresses data using gzip and writes it to name below the root.
//
// Use Case: Reduce file size for JSON/YAML exports (typically 70-80% reduction)
// Compression level: Best (level 9) - acceptable for hobbyist use
//
// Example:
//
//	data := []byte(`{"devices": [...]}`)
//	err := sync.WriteGzip(root, "export.json.gz", data)
func WriteGzip(root *os.Root, name string, data []byte) error {
	f, err := root.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	return gzipTo(f, data)
}

// syncWriteCloser is what the archive writers need of a file; *os.File satisfies it.
type syncWriteCloser interface {
	io.WriteCloser
	Sync() error
}

// gzipTo writes data as one gzip stream to f and closes f. The trailer is written by the gzip writer's Close,
// so that Close runs before the Sync, and the file's own Close error is returned too: a full disk shows up in
// one of them, and an archive without its trailer is not a successful export.
func gzipTo(f syncWriteCloser, data []byte) (err error) {
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("failed to close file: %w", cerr)
		}
	}()

	gz := gzip.NewWriter(f)
	if _, err = gz.Write(data); err != nil {
		return fmt.Errorf("failed to write gzip: %w", err)
	}
	if err = gz.Close(); err != nil {
		return fmt.Errorf("failed to finish gzip: %w", err)
	}
	return f.Sync()
}

// WriteZipSingle creates a ZIP archive with a single file entry, at name below the root.
// entryName is the name of the file inside the ZIP.
//
// Use Case: Windows-friendly compression, better for multiple files (future)
// Note: For single files, GZIP is more efficient. Use ZIP for Windows compatibility.
//
// Example:
//
//	data := []byte(`{"devices": [...]}`)
//	err := sync.WriteZipSingle(root, "export.zip", "export.json", data)
func WriteZipSingle(root *os.Root, name string, entryName string, data []byte) error {
	f, err := root.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	return zipTo(f, entryName, data)
}

// zipTo writes data as a one-entry zip archive to f and closes f, in the order gzipTo explains: the zip
// writer's Close writes the central directory, so it runs before the Sync.
func zipTo(f syncWriteCloser, entryName string, data []byte) (err error) {
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("failed to close file: %w", cerr)
		}
	}()

	zw := zip.NewWriter(f)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: entryName, Method: zip.Deflate})
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}
	if _, err = w.Write(data); err != nil {
		return fmt.Errorf("failed to write zip entry: %w", err)
	}
	if err = zw.Close(); err != nil {
		return fmt.Errorf("failed to finish zip: %w", err)
	}
	return f.Sync()
}

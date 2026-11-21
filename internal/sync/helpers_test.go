package sync

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openTestRoot opens a new temporary directory as the export root, as OpenOutputRoot does for a request.
func openTestRoot(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

func TestFileSHA256(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{
			name:    "empty file",
			content: []byte{},
			want:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:    "simple content",
			content: []byte("hello world"),
			want:    fmt.Sprintf("%x", sha256.Sum256([]byte("hello world"))),
		},
		{
			name:    "json content",
			content: []byte(`{"test": "data"}`),
			want:    fmt.Sprintf("%x", sha256.Sum256([]byte(`{"test": "data"}`))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dir := openTestRoot(t)
			if err := os.WriteFile(filepath.Join(dir, "test.txt"), tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := FileSHA256(root, "test.txt")
			if err != nil {
				t.Fatalf("FileSHA256() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("FileSHA256() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileSHA256_InvalidPath(t *testing.T) {
	root, _ := openTestRoot(t)
	_, err := FileSHA256(root, "nonexistent/path/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestWriteGzip(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "small data",
			data:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "json data",
			data:    []byte(`{"devices": [{"id": 1, "name": "test"}]}`),
			wantErr: false,
		},
		{
			name:    "large data",
			data:    bytes.Repeat([]byte("x"), 10000),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dir := openTestRoot(t)
			tmpFile := filepath.Join(dir, "test.gz")
			err := WriteGzip(root, "test.gz", tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteGzip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify file exists
			if _, statErr := os.Stat(tmpFile); os.IsNotExist(statErr) {
				t.Errorf("Output file not created: %s", tmpFile)
				return
			}

			// Verify gzip content
			f, err := os.Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to open output file: %v", err)
			}
			defer func() { _ = f.Close() }()

			gz, err := gzip.NewReader(f)
			if err != nil {
				t.Fatalf("Failed to create gzip reader: %v", err)
			}
			defer func() { _ = gz.Close() }()

			decompressed, err := io.ReadAll(gz)
			if err != nil {
				t.Fatalf("Failed to decompress: %v", err)
			}

			if !bytes.Equal(decompressed, tt.data) {
				t.Errorf("Decompressed data doesn't match original. Got %d bytes, want %d bytes", len(decompressed), len(tt.data))
			}
		})
	}
}

func TestWriteGzip_InvalidPath(t *testing.T) {
	root, _ := openTestRoot(t)
	err := WriteGzip(root, "nonexistent/invalid/path/test.gz", []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestWriteZipSingle(t *testing.T) {
	tests := []struct {
		name      string
		entryName string
		data      []byte
		wantErr   bool
	}{
		{
			name:      "empty data",
			entryName: "export.json",
			data:      []byte{},
			wantErr:   false,
		},
		{
			name:      "small data",
			entryName: "export.json",
			data:      []byte("hello world"),
			wantErr:   false,
		},
		{
			name:      "json data",
			entryName: "backup.json",
			data:      []byte(`{"devices": [{"id": 1, "name": "test"}]}`),
			wantErr:   false,
		},
		{
			name:      "yaml entry",
			entryName: "export.yaml",
			data:      []byte("devices:\n  - id: 1\n    name: test"),
			wantErr:   false,
		},
		{
			name:      "large data",
			entryName: "large.json",
			data:      bytes.Repeat([]byte("x"), 10000),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dir := openTestRoot(t)
			tmpFile := filepath.Join(dir, "test.zip")
			err := WriteZipSingle(root, "test.zip", tt.entryName, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteZipSingle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify file exists
			if _, statErr := os.Stat(tmpFile); os.IsNotExist(statErr) {
				t.Errorf("Output file not created: %s", tmpFile)
				return
			}

			// Verify zip content
			zr, err := zip.OpenReader(tmpFile)
			if err != nil {
				t.Fatalf("Failed to open zip: %v", err)
			}
			defer func() { _ = zr.Close() }()

			if len(zr.File) != 1 {
				t.Fatalf("Expected 1 file in zip, got %d", len(zr.File))
			}

			zipEntry := zr.File[0]
			if zipEntry.Name != tt.entryName {
				t.Errorf("Entry name = %v, want %v", zipEntry.Name, tt.entryName)
			}

			rc, err := zipEntry.Open()
			if err != nil {
				t.Fatalf("Failed to open zip entry: %v", err)
			}
			defer func() { _ = rc.Close() }()

			extracted, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("Failed to read zip entry: %v", err)
			}

			if !bytes.Equal(extracted, tt.data) {
				t.Errorf("Extracted data doesn't match original. Got %d bytes, want %d bytes", len(extracted), len(tt.data))
			}
		})
	}
}

func TestWriteZipSingle_InvalidPath(t *testing.T) {
	root, _ := openTestRoot(t)
	err := WriteZipSingle(root, "nonexistent/invalid/path/test.zip", "test.json", []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

// ginsys/shelly-manager#815: the helpers write through the opened export root, so a name that leaves it, by
// `..` or by a symbolic link swapped in after the path check, is refused and nothing is created outside.
func TestHelpersRefuseANameThatLeavesTheRoot(t *testing.T) {
	root, dir := openTestRoot(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink: %v", err)
	}

	for _, name := range []string{"../evil", "link/evil"} {
		t.Run(name, func(t *testing.T) {
			if err := WriteGzip(root, name+".gz", []byte("test")); err == nil {
				t.Error("WriteGzip wrote a name that leaves the root")
			}
			if err := WriteZipSingle(root, name+".zip", "test.json", []byte("test")); err == nil {
				t.Error("WriteZipSingle wrote a name that leaves the root")
			}
		})
	}
	if _, err := FileSHA256(root, "link/secret"); err == nil {
		t.Error("FileSHA256 read a file through a link that leaves the root")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 1 {
		t.Errorf("%d entries below the directory outside the root, want only the one that was there", len(entries))
	}
}

// archiveFile is a file that fails where a full disk would: a write after failWriteAfter bytes, a sync or a close.
type archiveFile struct {
	buf            bytes.Buffer
	failWriteAfter int // -1: never
	syncErr        error
	closeErr       error
	closed         bool
	syncedLen      int // bytes in the file when Sync was called
}

func (a *archiveFile) Write(p []byte) (int, error) {
	if a.failWriteAfter >= 0 && a.buf.Len()+len(p) > a.failWriteAfter {
		return 0, errors.New("no space left on device")
	}
	return a.buf.Write(p)
}
func (a *archiveFile) Sync() error  { a.syncedLen = a.buf.Len(); return a.syncErr }
func (a *archiveFile) Close() error { a.closed = true; return a.closeErr }

// The gzip trailer and the zip central directory are written when the archive writer is closed, so a failure
// there, or in the file's own sync or close, must reach the caller: an archive without its end is not an export.
func TestArchiveWritersReturnTheFirstFailureOfTheEnd(t *testing.T) {
	writers := map[string]struct {
		write      func(f syncWriteCloser) error
		headerSize int // bytes the writer emits before the end, which the file accepts
	}{
		"gzip": {func(f syncWriteCloser) error { return gzipTo(f, []byte("data")) }, 10},
		"zip":  {func(f syncWriteCloser) error { return zipTo(f, "e.json", []byte("data")) }, 0},
	}
	for name, w := range writers {
		t.Run(name, func(t *testing.T) {
			closeErr, syncErr := errors.New("close failed"), errors.New("sync failed")
			cases := []struct {
				desc string
				file *archiveFile
				want error  // nil: any error
				text string // the error must say this, so that a failure elsewhere cannot satisfy the case
			}{
				// the sync would fail too: the failure of the archive close, which comes first, is the one returned
				{"the end cannot be written", &archiveFile{failWriteAfter: w.headerSize, syncErr: syncErr}, nil, "failed to finish " + name},
				{"the sync fails", &archiveFile{failWriteAfter: -1, syncErr: syncErr}, syncErr, ""},
				{"the close fails", &archiveFile{failWriteAfter: -1, closeErr: closeErr}, closeErr, ""},
			}
			for _, c := range cases {
				t.Run(c.desc, func(t *testing.T) {
					err := w.write(c.file)
					if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
						t.Errorf("err = %v, want a failure (%v)", err, c.want)
					}
					if err != nil && !strings.Contains(err.Error(), c.text) {
						t.Errorf("err = %v, want it to say %q", err, c.text)
					}
					if !c.file.closed {
						t.Error("the file was not closed")
					}
				})
			}
			ok := &archiveFile{failWriteAfter: -1}
			if err := w.write(ok); err != nil || !ok.closed || ok.buf.Len() == 0 {
				t.Errorf("a healthy file: err = %v, closed = %v, %d bytes", err, ok.closed, ok.buf.Len())
			}
			// the sync must see the whole archive: a sync before the archive close leaves the end unsynced
			if ok.syncedLen != ok.buf.Len() {
				t.Errorf("the file held %d of %d bytes when it was synced", ok.syncedLen, ok.buf.Len())
			}
		})
	}
}

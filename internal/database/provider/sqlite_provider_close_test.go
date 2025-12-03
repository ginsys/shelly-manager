package provider

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteProvider_gzipFile_ClosePatterns(t *testing.T) {
	s := NewSQLiteProvider(nil)

	dir := t.TempDir()
	src := filepath.Join(dir, "input.txt")
	dst := filepath.Join(dir, "output.txt.gz")

	// Write source content
	const content = "hello world"
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := s.gzipFile(src, dst); err != nil {
		t.Fatalf("gzipFile error: %v", err)
	}

	// Read and validate gzip output
	f, err := os.Open(dst)
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()

	out, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	if string(out) != content {
		t.Fatalf("unexpected contents: got %q want %q", string(out), content)
	}
}

func TestSQLiteProvider_zipFile_ClosePatterns(t *testing.T) {
	s := NewSQLiteProvider(nil)

	dir := t.TempDir()
	src := filepath.Join(dir, "input.txt")
	dst := filepath.Join(dir, "output.zip")

	// Write source content
	const content = "zip contents"
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := s.zipFile(src, dst); err != nil {
		t.Fatalf("zipFile error: %v", err)
	}

	// Open and validate zip
	zf, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zf.Close()

	if len(zf.File) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(zf.File))
	}

	rc, err := zf.File[0].Open()
	if err != nil {
		t.Fatalf("open zip entry: %v", err)
	}
	defer rc.Close()

	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read zip entry: %v", err)
	}
	if !bytes.Equal(out, []byte(content)) {
		t.Fatalf("unexpected contents: got %q want %q", string(out), content)
	}
}

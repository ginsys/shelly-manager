package sync

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

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
			tmpFile := filepath.Join(t.TempDir(), "test.txt")
			if err := os.WriteFile(tmpFile, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := FileSHA256(tmpFile)
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
	_, err := FileSHA256("/nonexistent/path/file.txt")
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
			tmpFile := filepath.Join(t.TempDir(), "test.gz")
			err := WriteGzip(tmpFile, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteGzip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify file exists
			if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
				t.Errorf("Output file not created: %s", tmpFile)
				return
			}

			// Verify gzip content
			f, err := os.Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to open output file: %v", err)
			}
			defer f.Close()

			gz, err := gzip.NewReader(f)
			if err != nil {
				t.Fatalf("Failed to create gzip reader: %v", err)
			}
			defer gz.Close()

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
	err := WriteGzip("/nonexistent/invalid/path/test.gz", []byte("test"))
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
			tmpFile := filepath.Join(t.TempDir(), "test.zip")
			err := WriteZipSingle(tmpFile, tt.entryName, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteZipSingle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify file exists
			if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
				t.Errorf("Output file not created: %s", tmpFile)
				return
			}

			// Verify zip content
			zr, err := zip.OpenReader(tmpFile)
			if err != nil {
				t.Fatalf("Failed to open zip: %v", err)
			}
			defer zr.Close()

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
			defer rc.Close()

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
	err := WriteZipSingle("/nonexistent/invalid/path/test.zip", "test.json", []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

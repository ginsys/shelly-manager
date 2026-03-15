package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func newTestSQLiteProvider(t *testing.T) *SQLiteProvider {
	t.Helper()
	return &SQLiteProvider{
		logger: logging.GetDefault(),
	}
}

func TestValidateBackupPath(t *testing.T) {
	s := newTestSQLiteProvider(t)

	tests := []struct {
		name      string
		path      string
		wantErr   bool
		wantClean string
	}{
		{"clean absolute path", "/tmp/backup.db", false, "/tmp/backup.db"},
		{"clean relative path", "backups/backup.db", false, "backups/backup.db"},
		{"traversal attack", "../../etc/passwd", true, ""},
		{"embedded traversal", "/tmp/backups/../../../etc/passwd", true, ""},
		{"dot-dot only", "..", true, ""},
		{"simple filename", "backup.db", false, "backup.db"},
		{"trailing slash cleaned", "/tmp/backup/", false, "/tmp/backup"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.validateBackupPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for path %q, got nil", tt.path)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for path %q: %v", tt.path, err)
				return
			}
			if got != tt.wantClean {
				t.Errorf("validateBackupPath(%q) = %q, want %q", tt.path, got, tt.wantClean)
			}
		})
	}
}

func TestDeleteBackupPathTraversal(t *testing.T) {
	s := newTestSQLiteProvider(t)

	traversalPaths := []string{
		"../../etc/passwd",
		"/tmp/../../../etc/shadow",
		"backups/../../secret",
	}

	for _, path := range traversalPaths {
		t.Run(path, func(t *testing.T) {
			err := s.DeleteBackup(path)
			if err == nil {
				t.Errorf("expected error for traversal path %q, got nil", path)
			}
		})
	}
}

func TestDeleteBackupValidPath(t *testing.T) {
	s := newTestSQLiteProvider(t)

	// Create a temp file to delete
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-backup.db")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Should succeed
	if err := s.DeleteBackup(tmpFile); err != nil {
		t.Errorf("unexpected error deleting valid backup: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestDeleteBackupEmpty(t *testing.T) {
	s := newTestSQLiteProvider(t)

	// Empty backupID should be a no-op
	if err := s.DeleteBackup(""); err != nil {
		t.Errorf("unexpected error for empty backupID: %v", err)
	}
}

func TestDeleteBackupNonexistent(t *testing.T) {
	s := newTestSQLiteProvider(t)

	// Non-existent file should not error (os.IsNotExist is swallowed)
	if err := s.DeleteBackup("/tmp/nonexistent-backup-file-12345.db"); err != nil {
		t.Errorf("unexpected error for nonexistent file: %v", err)
	}
}

func TestFileSHA256PathTraversal(t *testing.T) {
	_, err := fileSHA256("../../etc/passwd")
	if err == nil {
		t.Error("expected error for traversal path, got nil")
	}
}

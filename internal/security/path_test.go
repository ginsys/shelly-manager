package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "path-validation-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	tests := []struct {
		name      string
		baseDir   string
		userPath  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid relative path",
			baseDir:  tmpDir,
			userPath: "file.txt",
			wantErr:  false,
		},
		{
			name:     "valid nested path",
			baseDir:  tmpDir,
			userPath: "subdir/file.txt",
			wantErr:  false,
		},
		{
			name:      "path traversal with ..",
			baseDir:   tmpDir,
			userPath:  "../etc/passwd",
			wantErr:   true,
			errSubstr: "path traversal blocked",
		},
		{
			name:      "path traversal with multiple ..",
			baseDir:   tmpDir,
			userPath:  "subdir/../../etc/passwd",
			wantErr:   true,
			errSubstr: "path traversal blocked",
		},
		{
			name:      "absolute path outside base",
			baseDir:   tmpDir,
			userPath:  "/etc/passwd",
			wantErr:   true,
			errSubstr: "path traversal blocked",
		},
		{
			name:      "empty base directory",
			baseDir:   "",
			userPath:  "file.txt",
			wantErr:   true,
			errSubstr: "base directory not configured",
		},
		{
			name:      "empty user path",
			baseDir:   tmpDir,
			userPath:  "",
			wantErr:   true,
			errSubstr: "path cannot be empty",
		},
		{
			name:     "path traversal with encoded characters",
			baseDir:  tmpDir,
			userPath: "..%2f..%2fetc/passwd",
			wantErr:  false, // This becomes a literal filename, not traversal
		},
		{
			name:     "path with dot prefix",
			baseDir:  tmpDir,
			userPath: "./file.txt",
			wantErr:  false,
		},
		{
			name:     "path referencing current directory",
			baseDir:  tmpDir,
			userPath: ".",
			wantErr:  false,
		},
		{
			name:     "path with null byte",
			baseDir:  tmpDir,
			userPath: "file\x00.txt",
			wantErr:  false, // filepath.Clean handles this
		},
		// Note: On Linux, backslashes are literal characters in filenames
		// not path separators, so this becomes a literal filename
		{
			name:     "path with backslash (literal on Linux)",
			baseDir:  tmpDir,
			userPath: "..\\etc\\passwd",
			wantErr:  false, // Backslash is not a path separator on Linux
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePath(tt.baseDir, tt.userPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePath() expected error containing %q, got nil (result: %s)", tt.errSubstr, result)
					return
				}
				if tt.errSubstr != "" && !containsSubstring(err.Error(), tt.errSubstr) {
					t.Errorf("ValidatePath() error = %v, want error containing %q", err, tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePath() unexpected error: %v", err)
					return
				}

				// Verify the result is within the base directory
				absBase, _ := filepath.Abs(tt.baseDir)
				if result != absBase && !isWithinDir(result, absBase) {
					t.Errorf("ValidatePath() result %q is not within base %q", result, absBase)
				}
			}
		})
	}
}

func TestValidatePath_EdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "path-edge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test case: base directory with similar prefix
	// Create /tmp/test and /tmp/test_backup
	testDir := filepath.Join(tmpDir, "exports")
	testDirBackup := filepath.Join(tmpDir, "exports_backup")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	if err := os.MkdirAll(testDirBackup, 0755); err != nil {
		t.Fatalf("failed to create test backup directory: %v", err)
	}

	// Try to escape to exports_backup from exports base
	_, err = ValidatePath(testDir, "../exports_backup/file.txt")
	if err == nil {
		t.Error("ValidatePath() should block escaping to sibling directory with similar prefix")
	}
}

func TestValidatePathWithSymlinks(t *testing.T) {
	// Skip on platforms that don't support symlinks well
	tmpDir, err := os.MkdirTemp("", "symlink-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create base directory structure
	baseDir := filepath.Join(tmpDir, "base")
	outsideDir := filepath.Join(tmpDir, "outside")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("failed to create base directory: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("failed to create outside directory: %v", err)
	}

	// Create a file outside the base directory
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to create outside file: %v", err)
	}

	// Create a symlink inside base pointing outside
	symlinkPath := filepath.Join(baseDir, "link")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// ValidatePathWithSymlinks should block access through symlink
	_, err = ValidatePathWithSymlinks(baseDir, "link/secret.txt")
	if err == nil {
		t.Error("ValidatePathWithSymlinks() should block symlink escaping base directory")
	}

	// Regular ValidatePath might not catch this (depending on implementation)
	// as it doesn't resolve symlinks
}

func TestIsPathSafe(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "issafe-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		baseDir  string
		userPath string
		want     bool
	}{
		{"safe path", tmpDir, "file.txt", true},
		{"unsafe traversal", tmpDir, "../etc/passwd", false},
		{"empty base", "", "file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathSafe(tt.baseDir, tt.userPath); got != tt.want {
				t.Errorf("IsPathSafe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"normal filename", "file.txt", "file.txt"},
		{"path traversal", "../etc/passwd", "__etc_passwd"},
		{"forward slashes", "path/to/file.txt", "path_to_file.txt"},
		{"backslashes", "path\\to\\file.txt", "path_to_file.txt"},
		{"double dots", "..file..txt", "_file_txt"},
		{"null byte", "file\x00.txt", "file.txt"},
		{"leading dot", ".hidden", "hidden"},
		{"trailing dot", "file.", "file"},
		{"leading space", " file.txt", "file.txt"},
		{"trailing space", "file.txt ", "file.txt"},
		{"empty after sanitize", "...", "_"},
		{"only spaces", "   ", "unnamed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeFilename(tt.filename); got != tt.want {
				t.Errorf("SanitizeFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Helper functions

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func isWithinDir(path, dir string) bool {
	// Add separator to ensure we match directory and not just prefix
	dirWithSep := dir
	if dirWithSep[len(dirWithSep)-1] != filepath.Separator {
		dirWithSep += string(filepath.Separator)
	}
	return len(path) > len(dir) && path[:len(dirWithSep)] == dirWithSep
}

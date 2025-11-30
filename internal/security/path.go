// Package security provides security utilities for the shelly-manager application.
package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePath ensures userPath resolves to a location within baseDir,
// preventing path traversal attacks. It returns the validated absolute path
// or an error if the path would escape the base directory.
//
// This function should be used whenever user-supplied paths are used for
// file system operations.
func ValidatePath(baseDir, userPath string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("base directory not configured")
	}

	if userPath == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Get absolute path of base directory
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}

	// Clean and join the paths
	// filepath.Join automatically cleans the path, but we use Clean explicitly
	// for clarity about what transformations are applied
	cleanUser := filepath.Clean(userPath)

	// If the user path is absolute, we need to check it directly
	// rather than joining it with the base
	var fullPath string
	if filepath.IsAbs(cleanUser) {
		fullPath = cleanUser
	} else {
		fullPath = filepath.Join(absBase, cleanUser)
	}

	// Get absolute path of the full path
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Ensure the resolved path is within the base directory
	// We add a trailing separator to ensure we match the directory itself
	// and not just a prefix (e.g., /data/exports vs /data/exports_backup)
	basePrefixCheck := absBase
	if !strings.HasSuffix(basePrefixCheck, string(filepath.Separator)) {
		basePrefixCheck += string(filepath.Separator)
	}

	// Check if absPath is the base directory itself or within it
	if absPath != absBase && !strings.HasPrefix(absPath, basePrefixCheck) {
		return "", fmt.Errorf("path traversal blocked: path escapes base directory")
	}

	return absPath, nil
}

// ValidatePathWithSymlinks is like ValidatePath but also resolves symlinks
// to ensure they don't point outside the base directory.
// This provides additional protection against symlink attacks.
func ValidatePathWithSymlinks(baseDir, userPath string) (string, error) {
	// First validate without symlink resolution
	absPath, err := ValidatePath(baseDir, userPath)
	if err != nil {
		return "", err
	}

	// Resolve the base directory symlinks
	realBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		// If base doesn't exist yet, fall back to Abs
		realBase, err = filepath.Abs(baseDir)
		if err != nil {
			return "", fmt.Errorf("invalid base path: %w", err)
		}
	}

	// Try to resolve symlinks in the path
	// If the path doesn't exist yet, we can't resolve symlinks
	// so we just use the validated absolute path
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Path doesn't exist yet, which is fine for write operations
		// Use the validated path from ValidatePath
		return absPath, nil
	}

	// Ensure the real path (after symlink resolution) is still within base
	basePrefixCheck := realBase
	if !strings.HasSuffix(basePrefixCheck, string(filepath.Separator)) {
		basePrefixCheck += string(filepath.Separator)
	}

	if realPath != realBase && !strings.HasPrefix(realPath, basePrefixCheck) {
		return "", fmt.Errorf("path traversal blocked: symlink escapes base directory")
	}

	return realPath, nil
}

// IsPathSafe is a convenience function that returns true if the path is safe,
// false otherwise. Use this when you only need a boolean check.
func IsPathSafe(baseDir, userPath string) bool {
	_, err := ValidatePath(baseDir, userPath)
	return err == nil
}

// SanitizeFilename removes or replaces potentially dangerous characters
// from a filename. This should be used in addition to ValidatePath when
// dealing with user-supplied filenames.
func SanitizeFilename(filename string) string {
	// Remove any path separators
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Remove null bytes
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Remove other potentially dangerous characters
	filename = strings.ReplaceAll(filename, "..", "_")

	// Trim leading/trailing spaces and dots
	filename = strings.Trim(filename, " .")

	// If empty after sanitization, return a default
	if filename == "" {
		return "unnamed"
	}

	return filename
}

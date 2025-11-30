# Code Comments for Compression

**Priority**: HIGH - Post-Commit Required
**Status**: not-started
**Effort**: 20 minutes

## Context

The compression functions need better documentation for future maintainers to understand their purpose and usage.

## Success Criteria

- [ ] `go doc` displays documentation correctly
- [ ] Comments are helpful and accurate
- [ ] Examples compile and make sense
- [ ] Code is self-documenting for future maintainers

## Implementation

### File 1: `internal/plugins/sync/helpers.go`

Enhance function documentation:

```go
// FileSHA256 calculates the SHA-256 checksum of a file.
// Returns hex-encoded string of the checksum.
//
// Use Case: Verify export integrity, detect file changes
// For hobbyist project: SHA-256 provides good balance of speed and security
//
// Example:
//   checksum, err := FileSHA256("/path/to/export.json")
//   if err != nil { return err }
//   fmt.Printf("Export checksum: %s\n", checksum)
func FileSHA256(path string) (string, error) {
    // ... existing code ...
}

// WriteGzip compresses data using gzip and writes to path.
//
// Use Case: Reduce file size for JSON/YAML exports (typically 70-80% reduction)
// Compression level: Best (level 9) - acceptable for hobbyist use
//
// Example:
//   data := []byte(`{"devices": [...]}`)
//   err := WriteGzip("/tmp/export.json.gz", data)
func WriteGzip(path string, data []byte) error {
    // ... existing code ...
}

// WriteZipSingle creates a ZIP archive with a single file entry.
// entryName is the name of the file inside the ZIP.
//
// Use Case: Windows-friendly compression, better for multiple files (future)
// Note: For single files, GZIP is more efficient. Use ZIP for Windows compatibility.
//
// Example:
//   data := []byte(`{"devices": [...]}`)
//   err := WriteZipSingle("/tmp/export.zip", "export.json", data)
func WriteZipSingle(path, entryName string, data []byte) error {
    // ... existing code ...
}
```

### File 2: `internal/api/sync_handlers.go`

Add comment for compression query parameter:

```go
// CreateJSONExport creates a JSON export with optional compression.
//
// Compression options (via config.compression_algo):
//   - "none": No compression (fastest, largest file)
//   - "gzip": GZIP compression (recommended, 70-80% size reduction)
//   - "zip":  ZIP compression (Windows-friendly, similar to gzip)
//
// Default: "none" for compatibility
func (eh *SyncHandlers) CreateJSONExport(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...
}
```

## Validation

```bash
# Verify documentation displays correctly
go doc ./internal/plugins/sync

# Code review: Check that comments are helpful and accurate
# Verify examples compile and make sense
```

## Dependencies

- Task 211 (helpers extracted)

## Risk

None - Documentation only

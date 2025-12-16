# Extract Duplicate Helper Functions

**Priority**: HIGH - Post-Commit Required
**Status**: done
**Effort**: 60 minutes

## Context

The export plugins (JSON, YAML, backup) contain duplicate helper functions for file operations. These should be extracted to a shared location to reduce code duplication and maintenance burden.

## Success Criteria

- [x] All plugins use shared helpers
- [x] No duplicate implementations
- [x] Tests pass
- [x] `grep -r "func fileSHA256" internal/plugins/sync/` shows only helpers.go

## Implementation

Create `internal/plugins/sync/helpers.go`:

```go
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
// For hobbyist project: best compression level is fine.
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
```

**Files to Update** (remove duplicates, add import):

1. `internal/plugins/sync/jsonexport/json.go`
   - Remove: `fileSHA256`, `writeGzip`, `writeZipSingle` functions
   - Add import: `"github.com/ginsys/shelly-manager/internal/sync"`
   - Replace calls: `fileSHA256(...)` -> `sync.FileSHA256(...)`

2. `internal/plugins/sync/yamlexport/yaml.go`
   - Same changes as above

3. `internal/plugins/sync/backup/backup.go` (if has duplicates)
   - Same changes as above

## Validation

```bash
# Run all plugin tests
go test ./internal/plugins/sync/...

# Verify no duplicates
grep -r "func fileSHA256" internal/plugins/sync/
# Should only show: internal/plugins/sync/helpers.go

# Build succeeds
go build ./cmd/shelly-manager
```

## Dependencies

- Phase 1 complete (commit)

## Risk

Low - Pure extraction, no logic changes

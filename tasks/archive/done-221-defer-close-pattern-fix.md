# Defer/Close Pattern Fix

**Priority**: HIGH - Post-Commit Required
**Status**: not-started
**Effort**: 20 minutes

## Context

The compression helper functions have a bug where file handles are explicitly closed before the deferred close executes, causing double-close issues. This should rely on defer only.

## Success Criteria

- [ ] Tests pass with `-race` flag
- [ ] No resource leak warnings
- [ ] Standard Go idiom followed

## Implementation

**File**: `internal/plugins/sync/helpers.go` (after Task 211 extraction)

### Fix WriteGzip

```go
// BEFORE (INCORRECT - double close)
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

    // PROBLEM: Explicit close before defer executes
    if err := gz.Close(); err != nil {
        return fmt.Errorf("close gzip: %w", err)
    }

    return f.Sync()
}

// AFTER (CORRECT - rely on defer only)
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
    // Sync ensures writes are flushed
    return f.Sync()
}
```

### Fix WriteZipSingle

```go
// BEFORE (INCORRECT)
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

    // PROBLEM: Explicit close before defer
    if err := zw.Close(); err != nil {
        return fmt.Errorf("close zip: %w", err)
    }

    return f.Sync()
}

// AFTER (CORRECT)
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

## Why This Matters

- Prevents double-close (defer + explicit close)
- Simpler code, easier to maintain
- Standard Go idiom

## Validation

```bash
# Run tests with race detector
go test -race ./internal/plugins/sync/...

# Verify no resource leaks
go test -v ./internal/plugins/sync/jsonexport/ -run TestPlugin_Export

# Manual test: Create large export, verify file is complete
```

## Dependencies

- Task 211 (helpers extracted)

## Risk

Low - Standard Go idiom correction

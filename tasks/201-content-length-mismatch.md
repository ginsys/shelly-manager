# Content-Length Mismatch on Error Responses

**Priority**: HIGH
**Status**: not-started
**Effort**: 2 hours

## Context
Every 500 error response triggers a secondary error: "http: wrote more than the declared Content-Length".
This is an HTTP protocol violation occurring 38 times in a single session during testing.

## Evidence
```
level=ERROR msg="Failed to encode JSON response" error="http: wrote more than the declared Content-Length"
```

Occurs after every 500 Internal Server Error response, suggesting an issue with error response serialization or buffering.

## Success Criteria
- [ ] Identify root cause in error response serialization
- [ ] Fix Content-Length calculation for error responses
- [ ] Verify no protocol violations in logs after testing
- [ ] Run `make test-ci` to ensure no regressions

## Files to Investigate
- `internal/api/response.go` (or similar error response handler)
- `internal/api/middleware.go`
- `internal/api/handlers*.go` (error handling patterns)

## Validation
```bash
# Run the application and trigger errors
# Monitor logs for "Content-Length" errors
tail -f data/shelly-manager.log | grep "Content-Length"

# Should see zero occurrences after fix
make test-ci
```

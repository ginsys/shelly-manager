# Downgrade WebSocket Close to DEBUG Level

**Priority**: LOW
**Status**: not-started
**Effort**: 30 minutes

## Context
Normal WebSocket disconnects (close code 1000) are logged as ERROR level, polluting logs with non-error information.

## Evidence
```
level=ERROR msg="WebSocket error" error="websocket: close 1000 (normal): Client disconnect"
level=ERROR msg="Failed to write close message" error="websocket: close sent"
```

Close code 1000 is a normal, graceful disconnect and should not be logged as an error.

## Success Criteria
- [ ] Change log level to DEBUG for normal close (code 1000)
- [ ] Keep ERROR level for abnormal disconnects (codes 1001-1015)
- [ ] Verify log output during normal WebSocket usage
- [ ] Run `make test-ci` to ensure no regressions

## Implementation Approach
1. Check WebSocket close code in handler
2. Log as DEBUG for code 1000
3. Log as ERROR for other codes
4. Update error message for "close sent" to be INFO/DEBUG

## Files to Investigate
- `internal/api/websocket.go`
- `internal/api/handlers_websocket.go` (if separate)

## Validation
```bash
# Connect and disconnect WebSocket client
# Should not see ERROR for normal disconnect

tail -f data/shelly-manager.log | grep -i websocket
make test-ci
```

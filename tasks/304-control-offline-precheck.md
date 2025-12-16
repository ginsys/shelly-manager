# Device Control Pre-Check for Offline Status

**Priority**: MEDIUM
**Status**: not-started
**Effort**: 2 hours

## Context
Control commands (on/off) to offline devices block for 30 seconds before timing out. Should fail fast if device is known to be offline.

## Evidence
- Device 4 (online @ 172.31.103.101): 66ms, 127ms, 231ms response times
- Device 1 (offline @ 172.31.103.102): 29873ms, 26047ms timeouts

## Success Criteria
- [ ] Check cached online status before attempting control command
- [ ] Fail immediately with "device offline" message if cached status is offline
- [ ] Optionally provide "force attempt anyway" button for user override
- [ ] Update device online/offline status cache on connectivity checks
- [ ] Run `make test-ci` to ensure no regressions

## Implementation Approach
1. Add online status to device cache with TTL
2. Check cache before control command
3. Return immediate error if offline
4. Allow force flag to bypass check

## Files to Investigate
- `internal/api/handlers_control.go`
- `internal/device/control.go`
- `internal/device/cache.go` (if exists)

## Validation
```bash
# Attempt control command on offline device
# Should fail immediately (<1s) with clear message

make test-ci
```

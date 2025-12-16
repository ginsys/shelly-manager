# Offline Device Detection and Fast Fail

**Priority**: HIGH
**Status**: not-started
**Effort**: 4 hours

## Context
Viewing details of offline devices blocks for ~59 seconds due to sequential timeout requests to /status, /energy, /config/drift. Online devices respond in ~1 second total.

User feedback: "when clicking a device to get details, the backend seems to want to communicate with the device, yielding in a long wait time if the device is not online"

## Evidence
- Online device (3): ~1s total (253ms + 270ms + 409ms)
- Offline device (1 @ 172.31.103.102): Multiple 10-second timeouts stacking to ~59s

## Root Cause
Sequential HTTP requests to device endpoints with full timeout waits. No caching of device online/offline status.

## Success Criteria
- [ ] Implement parallel device info requests (status, energy, config)
- [ ] Add quick connectivity pre-check with short timeout (<2s)
- [ ] Cache offline status to prevent repeated timeout waits
- [ ] Device detail page loads in <5s even for offline devices
- [ ] Run `make test-ci` to ensure no regressions

## Implementation Approach
1. Modify device handlers to fetch /status, /energy, /config/drift in parallel using goroutines
2. Add lightweight ping/connectivity check before full request
3. Cache device online/offline status with TTL
4. Return cached data for offline devices with appropriate error messages

## Files to Investigate
- `internal/api/handlers_devices.go`
- `internal/device/client.go`
- `internal/device/cache.go` (if exists)

## Validation
```bash
# Test with offline device
# Should complete in <5s instead of ~59s

make test-ci
```

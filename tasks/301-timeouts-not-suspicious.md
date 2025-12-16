# Timeouts Should Not Trigger Suspicious Activity

**Priority**: MEDIUM
**Status**: not-started
**Effort**: 2 hours

## Context
Every 408 Request Timeout triggers a "general_suspicious" security alert in the logs. Timeouts to offline devices are normal operation, not attacks.

## Evidence
Multiple `level=WARN msg="general_suspicious detected"` warnings corresponding to every 408 timeout response, polluting logs with false positives.

## Success Criteria
- [ ] Exclude timeout responses (408) from suspicious activity detection
- [ ] Only flag actual attack patterns (SQL injection, XSS attempts, path traversal, etc.)
- [ ] Reduce log noise from false positives
- [ ] Document what constitutes "suspicious" activity
- [ ] Run `make test-ci` to ensure no regressions

## Files to Investigate
- `internal/security/monitor.go`
- `internal/api/middleware/` (security middleware)
- `internal/security/detection.go` (if exists)

## Validation
```bash
# Test with offline devices causing timeouts
# Should not see "general_suspicious" warnings

tail -f data/shelly-manager.log | grep "suspicious"
make test-ci
```

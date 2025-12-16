# Fix Localhost IP Blocking False Positive

**Priority**: HIGH
**Status**: not-started
**Effort**: 2 hours

## Context
Security monitoring blocked localhost (127.0.0.1) during normal testing, preventing legitimate development/testing activities.

User feedback: "I also see a lot of logs about suspicious requests - whilst I'm the only one to test here on localhost"

## Evidence
```
level=WARN msg="IP address blocked due to suspicious activity" client_ip=127.0.0.1
suspicious_requests=1 rate_violations=0 attack_types=map[general_suspicious:1]
```

## Root Cause
Security monitoring system triggers on timeout patterns (408 responses) which are normal for offline device testing. No development mode or localhost exemption.

## Success Criteria
- [ ] Add localhost (127.0.0.1, ::1) exemption from IP blocking
- [ ] Add development/testing mode configuration flag
- [ ] Tune suspicious activity detection thresholds
- [ ] Prevent timeouts from triggering "suspicious" flags
- [ ] Verify legitimate requests aren't blocked
- [ ] Run `make test-ci` to ensure no regressions

## Files to Investigate
- `internal/security/monitor.go`
- `internal/security/rate_limiter.go`
- `internal/api/middleware/security.go`
- Configuration files for security settings

## Validation
```bash
# Test with localhost requests that trigger timeouts
# Should not see IP blocking warnings in logs

tail -f data/shelly-manager.log | grep "blocked"
make test-ci
```

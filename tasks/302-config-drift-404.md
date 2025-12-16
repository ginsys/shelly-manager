# Config Drift Should Return 404 for Missing Config

**Priority**: MEDIUM
**Status**: not-started
**Effort**: 1 hour

## Context
`/api/v1/devices/{id}/config/drift` returns 500 Internal Server Error when no stored configuration exists. This is not an internal error - it's an expected condition that should return 404 or 200 with null values.

## Evidence
```
level=ERROR msg="Failed to detect config drift" error="no stored configuration found for device"
level=ERROR msg="Internal server error" path=/api/v1/devices/{id}/config/drift
```

## Success Criteria
- [ ] Return 404 with clear message when no stored config exists
- [ ] Alternative: Return 200 with `{ "stored_config": null, "drift": null }`
- [ ] Update frontend to handle this gracefully
- [ ] Add API documentation for this case
- [ ] Run `make test-ci` to ensure no regressions

## Implementation Options
1. **404 Approach**: Return 404 with message "No stored configuration found for device"
2. **200 Approach**: Return 200 with null fields indicating no baseline for comparison

Recommendation: 404 is more semantically correct (the drift resource doesn't exist without a stored config).

## Files to Investigate
- `internal/api/handlers_config.go`
- `internal/configuration/drift.go`
- `web/src/` (frontend drift detection handling)

## Validation
```bash
# Test drift detection for device without stored config
# Should return 404, not 500

make test-ci
```

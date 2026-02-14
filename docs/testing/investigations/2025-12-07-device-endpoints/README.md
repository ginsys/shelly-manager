# Device Endpoints Investigation

**Date**: 2025-12-07
**Type**: Exploratory Testing
**Status**: Complete - Tasks Created

## Summary

Comprehensive testing of all 27 device API endpoints to identify bugs and verify correct behavior.

**Results**:
- ✅ 24 endpoints working correctly (89%)
- ❌ 2 endpoints returning unexpected 500 errors
- ⚠️ 1 endpoint with known issue (already tracked)

## Files

- **INVESTIGATION-SUMMARY.md** - Full investigation report with findings, root cause analysis, and recommendations
- **test-device-endpoints.sh** - Bash script to test all 27 device endpoints (recommended)
- **device-endpoints-investigation.spec.ts** - Playwright E2E version (not recommended due to setup conflicts)

## Issues Discovered

### Critical Bugs Found

1. **PUT /api/v1/devices/{id}** → Returns 500 instead of 200
   - Root cause: Handler doesn't support partial updates
   - Tracked: GitHub Issue #74 (originally task-207)
   - Priority: HIGH

2. **POST /api/v1/devices/{id}/config/apply-template** → Returns 500 instead of 404
   - Root cause: Missing resource returns 500 instead of 404
   - Tracked: GitHub Issue #75 (originally task-208)
   - Priority: HIGH

## Usage

To run the investigation script:

```bash
cd docs/testing/investigations/2025-12-07-device-endpoints
chmod +x test-device-endpoints.sh

# Ensure server is running
cd ../../../../
CGO_ENABLED=1 go build -o bin/shelly-manager ./cmd/shelly-manager
SHELLY_DATABASE_PROVIDER=sqlite ./bin/shelly-manager server

# In another terminal, run the test script
cd docs/testing/investigations/2025-12-07-device-endpoints
./test-device-endpoints.sh
```

## Related Issues

- Issue #74: UpdateDevice should support partial updates (HIGH priority, originally task-207)
- Issue #75: ApplyConfigTemplate should return 404 for missing templates (HIGH priority, originally task-208)
- Issue #77: Config drift should return 404 for missing config (MEDIUM priority, originally task-302)

## Next Steps

1. Implement fixes for Issues #74 and #75
2. Run `make test-ci` to verify fixes
3. Re-run this investigation script to confirm 500 errors resolved

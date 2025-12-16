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
   - Task created: [207-fix-update-device-partial-updates.md](../../../../tasks/207-fix-update-device-partial-updates.md)
   - Priority: HIGH

2. **POST /api/v1/devices/{id}/config/apply-template** → Returns 500 instead of 404
   - Root cause: Missing resource returns 500 instead of 404
   - Task created: [208-fix-apply-template-404.md](../../../../tasks/208-fix-apply-template-404.md)
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

## Related Tasks

- Task 207: Fix UpdateDevice to Support Partial Updates (HIGH priority)
- Task 208: Fix ApplyConfigTemplate to Return 404 for Missing Templates (HIGH priority)
- Task 302: Config Drift Should Return 404 for Missing Config (MEDIUM priority - pre-existing)

## Next Steps

1. Implement fixes for Task 207 and Task 208
2. Run `make test-ci` to verify fixes
3. Re-run this investigation script to confirm 500 errors resolved
4. Update task files to completed status

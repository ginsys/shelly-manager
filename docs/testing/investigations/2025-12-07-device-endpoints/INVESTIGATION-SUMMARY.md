# Device Endpoints Investigation Summary

**Date**: 2025-12-07
**Session**: Endpoint testing and 500 error investigation

## Overview

Tested all 27 device API endpoints to identify which are working and which have issues. Found 2 endpoints returning unexpected 500 errors.

## Test Results

**Working Endpoints**: 24 / 27 (89%)
**Failed Endpoints**: 2 / 27 (7%)
**Expected Failures**: 1 (config/drift - known issue, tracked in Issue #77)

## Issues Found

### Issue 1: PUT /api/v1/devices/{id} → 500 Internal Server Error

**Status**: ❌ Bug - Returns 500 instead of 200
**Tracked In**: Issue #74 (originally task-207)
**Priority**: HIGH

**Test Case**:
```bash
curl -X PUT http://localhost:8080/api/v1/devices/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated"}'
```

**Error**:
```
level=ERROR msg="Database operation failed" component=database device_id=1
  error="constraint failed: UNIQUE constraint failed: devices.mac (2067)"
level=ERROR msg="Internal server error" method=PUT path=/api/v1/devices/1
  error="constraint failed: UNIQUE constraint failed: devices.mac (2067)"
```

**Root Cause**: Handler decodes request into a new Device struct, missing fields get zero values (including empty MAC), causing UNIQUE constraint violation.

**Impact**: Cannot perform partial updates to devices

**Fix**: Implement partial update logic (see `207-proposed-fix.txt`)

---

### Issue 2: POST /api/v1/devices/{id}/config/apply-template → 500 Internal Server Error

**Status**: ❌ Bug - Returns 500 instead of 404
**Tracked In**: Issue #75 (originally task-208)
**Priority**: HIGH

**Test Case**:
```bash
curl -X POST http://localhost:8080/api/v1/devices/1/config/apply-template \
  -H "Content-Type: application/json" \
  -d '{"template_id":1}'
```

**Error**:
```
level=ERROR msg="Failed to apply config template" device_id=1 template_id=1
  error="template not found: record not found"
level=ERROR msg="Internal server error" method=POST
  path=/api/v1/devices/1/config/apply-template
  error="template not found: record not found"
```

**Root Cause**: Handler returns 500 for all errors, doesn't distinguish "not found" from server errors.

**Impact**: Poor API semantics - missing resources should return 404, not 500

**Fix**: Check error type and return 404 for missing templates (see `208-proposed-fix.txt`)

---

## Test Files in This Directory

### 1. Bash Script (Recommended for Quick Testing)
**File**: `test-device-endpoints.sh`
**Status**: ✅ Working
**Purpose**: Standalone script to test all 27 endpoints
**Usage**:
```bash
cd docs/testing/investigations
chmod +x test-device-endpoints.sh
./test-device-endpoints.sh
```

**Features**:
- Color-coded output (green ✓ = working, red ✗ = unexpected)
- Tests all CRUD, control, config, and capability endpoints
- Shows HTTP status codes and error messages
- No dependencies on test infrastructure

---

### 2. Playwright E2E Test
**File**: `device-endpoints-investigation.spec.ts`
**Status**: ⚠️ Conflicts with global setup
**Purpose**: E2E investigation of all 27 endpoints
**Issues**:
- Requires server running
- Conflicts with global setup device creation
- Slower than bash script

**Note**: Bash script is more practical for this type of investigation

---

## GitHub Issues Created

| Issue | Priority | Title | Status |
|-------|----------|-------|--------|
| #74 | HIGH | UpdateDevice should support partial updates | open |
| #75 | HIGH | ApplyConfigTemplate should return 404 for missing templates | open |

Both tasks include:
- ✅ Context and evidence
- ✅ Success criteria
- ✅ Implementation approach
- ✅ Validation commands
- ✅ Proposed code fixes

---

## Endpoint Coverage Summary

### ✅ Working (24 endpoints)

**Core CRUD**:
- GET /api/v1/devices
- POST /api/v1/devices
- GET /api/v1/devices/{id}
- DELETE /api/v1/devices/{id}

**Control & Status** (timeouts expected for offline devices):
- POST /api/v1/devices/{id}/control
- GET /api/v1/devices/{id}/status
- GET /api/v1/devices/{id}/energy

**Configuration**:
- GET /api/v1/devices/{id}/config
- PUT /api/v1/devices/{id}/config
- GET /api/v1/devices/{id}/config/current
- GET /api/v1/devices/{id}/config/current/normalized
- GET /api/v1/devices/{id}/config/typed/normalized
- POST /api/v1/devices/{id}/config/import
- GET /api/v1/devices/{id}/config/status
- POST /api/v1/devices/{id}/config/export
- GET /api/v1/devices/{id}/config/history
- GET /api/v1/devices/{id}/config/typed
- PUT /api/v1/devices/{id}/config/typed

**Capability-Specific Config**:
- PUT /api/v1/devices/{id}/config/relay
- PUT /api/v1/devices/{id}/config/dimming
- PUT /api/v1/devices/{id}/config/roller
- PUT /api/v1/devices/{id}/config/power-metering
- PUT /api/v1/devices/{id}/config/auth

**Other**:
- GET /api/v1/devices/{id}/capabilities

### ❌ Broken (2 endpoints)

- PUT /api/v1/devices/{id} → 500 (Issue #74)
- POST /api/v1/devices/{id}/config/apply-template → 500 (Issue #75)

### ⚠️ Known Issue (1 endpoint)

- GET /api/v1/devices/{id}/config/drift → 500 when no config stored (Issue #77)

---

## Next Steps

1. **Fix Issue #74**: UpdateDevice partial updates
   - Choose approach (map-based or pointer-based struct)
   - Add constraint violation handling (return 409, not 500)
   - Add tests for partial updates

2. **Fix Issue #75**: ApplyConfigTemplate 404
   - Add error type checking
   - Return 404 for "not found" errors
   - Return 400 for validation errors

3. **Validation**:
   - Run `make test-ci` after both fixes
   - Re-run endpoint tests to verify 500s are resolved

---

## Files Reference

**Proposed Fixes**:
- `207-proposed-fix.txt` (code proposal for Issue #74)
- `208-proposed-fix.txt` (code proposal for Issue #75)

**Code to Modify**:
- `internal/api/handlers.go` (lines 333-373, 1063-1098)

**Tests to Add**:
- `internal/api/handlers_test.go`

---

**Investigation completed**: 2025-12-07 00:50
**Total task effort**: 3 hours
**Server stopped**: Ready for implementation

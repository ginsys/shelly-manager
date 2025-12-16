# Fix UpdateDevice to Support Partial Updates

**Priority**: HIGH
**Status**: not-started
**Effort**: 2 hours

## Context
PUT /api/v1/devices/{id} returns 500 Internal Server Error when updating a device with partial data.

The handler decodes the request body into a new Device struct, which sets missing fields to zero values (including empty MAC address). When it tries to save, it violates UNIQUE constraint on MAC address.

## Evidence
```
level=ERROR msg="Database operation failed" component=database device_id=1 error="constraint failed: UNIQUE constraint failed: devices.mac (2067)"
level=ERROR msg="Internal server error" method=PUT path=/api/v1/devices/1 error="constraint failed: UNIQUE constraint failed: devices.mac (2067)"
```

Test case:
```bash
curl -X PUT http://localhost:8080/api/v1/devices/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated"}'
```

Expected: 200 OK with only name field updated
Actual: 500 Internal Server Error due to constraint violation

## Root Cause
Handler at `internal/api/handlers.go:333-373` does full struct replacement:
```go
var updatedDevice database.Device
if err := json.NewDecoder(r.Body).Decode(&updatedDevice); err != nil {
    h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
    return
}
// Missing fields get zero values, including MAC=""
updatedDevice.ID = existingDevice.ID
if err := h.DB.UpdateDevice(&updatedDevice); err != nil {
    h.responseWriter().WriteInternalError(w, r, err)
    return
}
```

## Success Criteria
- [ ] Support partial updates - only update fields present in request
- [ ] Return 409 Conflict for constraint violations, not 500
- [ ] Verify test case works: `{"name":"Updated"}` updates only name
- [ ] Add test for partial update preserving other fields
- [ ] Run `make test-ci` to ensure no regressions

## Implementation Approach
1. Fetch existing device first
2. Use JSON unmarshal into map to detect which fields are present
3. Only update fields that are present in request
4. Or use GORM's `Updates()` with struct that omits zero values
5. Return 409 for constraint violations instead of 500

## Files to Modify
- `internal/api/handlers.go` (lines 333-373)
- Add tests in `internal/api/handlers_test.go`

## Proposed Implementation
See `docs/testing/investigations/2025-12-07-device-endpoints/207-proposed-fix.txt` for two implementation approaches:
1. Map-based approach to detect present fields
2. Pointer-based struct approach with GORM's Updates()

## Validation
```bash
# Should update only name field
curl -X PUT http://localhost:8080/api/v1/devices/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated Device"}'

# Should preserve MAC, IP, and other fields
curl http://localhost:8080/api/v1/devices/1 | jq '.data'

make test-ci
```

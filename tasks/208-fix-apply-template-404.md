# Fix ApplyConfigTemplate to Return 404 for Missing Templates

**Priority**: HIGH
**Status**: not-started
**Effort**: 1 hour

## Context
POST /api/v1/devices/{id}/config/apply-template returns 500 Internal Server Error when template doesn't exist. Should return 404 Not Found.

## Evidence
```
level=ERROR msg="Failed to apply config template" device_id=1 template_id=1 error="template not found: record not found"
level=ERROR msg="Internal server error" method=POST path=/api/v1/devices/1/config/apply-template error="template not found: record not found"
```

Test case:
```bash
curl -X POST http://localhost:8080/api/v1/devices/1/config/apply-template \
  -H "Content-Type: application/json" \
  -d '{"template_id":1}'
```

Expected: 404 Not Found with message "Template not found"
Actual: 500 Internal Server Error

## Root Cause
Handler at `internal/api/handlers.go:1063-1098` doesn't distinguish between "template not found" and other errors:
```go
if err := h.Service.ApplyConfigTemplate(uint(id), req.TemplateID, req.Variables); err != nil {
    h.logger.WithFields(map[string]any{
        "device_id":   id,
        "template_id": req.TemplateID,
        "error":       err.Error(),
    }).Error("Failed to apply config template")
    h.responseWriter().WriteInternalError(w, r, err)  // Always 500
    return
}
```

The error message "template not found: record not found" indicates this is a missing resource, not a server error.

## Success Criteria
- [ ] Return 404 when template doesn't exist
- [ ] Return 404 when device doesn't exist
- [ ] Return 500 only for actual server errors
- [ ] Add test for missing template scenario
- [ ] Run `make test-ci` to ensure no regressions

## Implementation Approach
1. Check error type/message from Service.ApplyConfigTemplate()
2. Return 404 for "not found" errors (template or device)
3. Return 400 for validation errors
4. Return 500 only for unexpected errors

Example:
```go
if err := h.Service.ApplyConfigTemplate(uint(id), req.TemplateID, req.Variables); err != nil {
    if strings.Contains(err.Error(), "not found") {
        h.responseWriter().WriteNotFoundError(w, r, "Template or device")
        return
    }
    h.responseWriter().WriteInternalError(w, r, err)
    return
}
```

## Files to Modify
- `internal/api/handlers.go` (lines 1063-1098)
- Add tests in `internal/api/handlers_test.go`

## Proposed Implementation
See `docs/testing/investigations/2025-12-07-device-endpoints/208-proposed-fix.txt` for implementation with error type checking.

## Validation
```bash
# Should return 404 for non-existent template
curl -X POST http://localhost:8080/api/v1/devices/1/config/apply-template \
  -H "Content-Type: application/json" \
  -d '{"template_id":9999}'

# Response should be 404 with proper error message
make test-ci
```

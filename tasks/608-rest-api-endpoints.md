# REST API Endpoints

**Priority**: HIGH
**Status**: completed
**Effort**: 6 hours
**Completed**: 2026-01-07
**Depends On**: 606, 607

## Context

Expose the configuration management functionality via REST API endpoints. These endpoints will be used by the UI and can also be used for automation/scripting.

## API Design Principles

- RESTful resource-based URLs
- JSON request/response bodies
- Consistent error responses
- Pagination for list endpoints
- OpenAPI documentation
- Keep namespace consistent: prefer `/api/v1/config/...` for config management endpoints, and remove legacy `/api/v1/configuration/*` endpoints as part of task 609 cleanup

## Success Criteria

- [ ] Template CRUD endpoints
- [ ] Device template assignment endpoints
- [ ] Device tag endpoints
- [ ] Device override endpoints
- [ ] Desired config endpoint (with source tracking)
- [ ] Apply config endpoint
- [ ] Config status endpoint (applied/pending)
- [ ] Proper error handling and status codes
- [ ] Request validation
- [ ] OpenAPI/Swagger documentation
- [ ] Define and test secret redaction contract
- [ ] Integration tests for all endpoints

## Response Envelope & Secrets

### Standard Response Envelope

All endpoints should use the existing API response envelope used elsewhere in the backend:
- `success` (bool)
- `data` (payload)
- `error` (standard error object)
- `meta` (pagination)

Unless explicitly shown, the examples below describe the `data` payload only.

### Sensitive Fields (Write-Only)

These configuration values must be treated as secrets:
- WiFi credentials (e.g., `wifi_sta.pass`, `wifi_ap.pass`)
- MQTT password and client credentials
- Device auth credentials

API contract:
- Secrets are **accepted on write** (create/update template, set overrides).
- Secrets are **not returned** on read. Either omit the field entirely or return `null`.
- Provide explicit boolean indicators such as `has_mqtt_password`, `has_wifi_password`, `has_auth_password` where needed.
- For updates, support partial updates that do not require resubmitting secrets.

This prevents accidental credential exposure via the UI or API.

## API Endpoints

### Templates

```
GET    /api/v1/config/templates
       Query params: scope=global|group|device_type, device_type=SHPLG-S
       Response: { "templates": [...], "meta": { "total": N } }

POST   /api/v1/config/templates
       Body: { "name": "...", "scope": "...", "device_type": "..." (if scope=device_type), "config": {...} }
       Response: { "template": {...} }

GET    /api/v1/config/templates/{id}
       Response: { "template": {...} }

PUT    /api/v1/config/templates/{id}
       Body: { "name": "...", "description": "...", "config": {...} }
       Response: { "template": {...}, "affected_devices": N }

DELETE /api/v1/config/templates/{id}
       Response: 204 No Content
       Error 409: Template is assigned to devices
```

### Device Template Assignment

```
GET    /api/v1/devices/{id}/templates
       Response: { "templates": [...], "template_ids": [1, 5, 12] }

PUT    /api/v1/devices/{id}/templates
       Body: { "template_ids": [1, 5, 12] }
       Response: { "templates": [...], "desired_config": {...} }

POST   /api/v1/devices/{id}/templates/{templateId}
       Query params: position=0 (optional, default=append)
       Response: { "templates": [...] }

DELETE /api/v1/devices/{id}/templates/{templateId}
       Response: { "templates": [...] }
```

### Device Tags

```
GET    /api/v1/devices/{id}/tags
       Response: { "tags": ["office", "floor-2"] }

POST   /api/v1/devices/{id}/tags
       Body: { "tag": "office" }
       Response: { "tags": [...] }

DELETE /api/v1/devices/{id}/tags/{tag}
       Response: { "tags": [...] }

GET    /api/v1/tags
       Response: { "tags": ["office", "floor-2", "high-power"], "counts": {"office": 5, ...} }

GET    /api/v1/tags/{tag}/devices
       Response: { "devices": [...] }
```

### Device Overrides

```
GET    /api/v1/devices/{id}/overrides
       Response: { "overrides": {...} }

PUT    /api/v1/devices/{id}/overrides
       Body: { "system": {...}, "mqtt": {...}, ... }
       Response: { "overrides": {...}, "desired_config": {...} }

PATCH  /api/v1/devices/{id}/overrides
       Body: { "mqtt": { "server": "new.mqtt.host" } }
       Response: { "overrides": {...} }

DELETE /api/v1/devices/{id}/overrides
       Response: 204 No Content
```

### Desired Configuration

```
GET    /api/v1/devices/{id}/desired-config
       Response: {
         "config": {...},
         "sources": {
           "mqtt.server": "global-template",
           "mqtt.user": "device-override",
           "system.name": "device-override"
         }
       }
```

### Apply & Status

```
POST   /api/v1/devices/{id}/config/apply
       Response: {
         "success": true,
         "applied_count": 15,
         "failed_count": 0,
         "requires_reboot": false,
         "failures": []
       }

GET    /api/v1/devices/{id}/config/status
       Response: {
         "config_applied": false,
         "has_overrides": true,
         "template_count": 2,
         "last_applied": "2025-01-05T10:30:00Z",
         "pending_changes": true
       }

POST   /api/v1/devices/{id}/config/verify
       Response: {
         "match": false,
         "differences": [
           {"path": "mqtt.server", "expected": "mqtt.local", "actual": "old.mqtt.host"}
         ]
       }

POST   /api/v1/devices/{id}/config/reboot-and-verify
       Response: {
         "rebooted": true,
         "verify_result": {...}
       }
```

### Bulk Operations

```
POST   /api/v1/config/apply-bulk
       Body: { "device_ids": [1, 2, 3] }
       Response: {
         "results": [
           {"device_id": 1, "success": true},
           {"device_id": 2, "success": false, "error": "..."}
         ]
       }

GET    /api/v1/config/pending
       Response: {
         "devices": [
           {"id": 1, "name": "Kitchen", "template_count": 2}
         ],
         "total": 5
       }
```

## Request/Response Types

```go
// Template request
type CreateTemplateRequest struct {
    Name        string                `json:"name" validate:"required,min=1,max=100"`
    Description string                `json:"description" validate:"max=500"`
    Scope       string                `json:"scope" validate:"required,oneof=global group device_type"`
    DeviceType  string                `json:"device_type" validate:"required_if=Scope device_type"`
    Config      *DeviceConfiguration  `json:"config" validate:"required"`
}

// Template response
type TemplateResponse struct {
    ID          uint                 `json:"id"`
    Name        string               `json:"name"`
    Description string               `json:"description,omitempty"`
    Scope       string               `json:"scope"`
    DeviceType  string               `json:"device_type,omitempty"`
    Config      *DeviceConfiguration `json:"config"`
    CreatedAt   time.Time            `json:"created_at"`
    UpdatedAt   time.Time            `json:"updated_at"`
}

// Desired config response
type DesiredConfigResponse struct {
    Config  *DeviceConfiguration `json:"config"`
    Sources map[string]string    `json:"sources"`
}
```

## Files to Create/Modify

- `internal/api/handlers_config.go` (NEW - or extend existing handlers.go)
- `internal/api/handlers_config_test.go` (NEW)
- `internal/api/router.go` (modify - add routes)
- `internal/api/requests_config.go` (NEW - request types)
- `internal/api/responses_config.go` (NEW - response types)
- `docs/api/configuration.md` (NEW - API documentation)

## Error Responses

All error responses use the standard envelope.

```json
{
  "success": false,
  "error": {
    "code": "TEMPLATE_IN_USE",
    "message": "Cannot delete template: assigned to 3 devices",
    "details": {
      "device_ids": [1, 5, 12]
    }
  }
}
```

## Validation

```bash
make test-ci
go test -v ./internal/api/... -run TestConfig

# Manual API testing
curl -X GET http://localhost:8080/api/v1/config/templates
curl -X POST http://localhost:8080/api/v1/config/templates -d '{"name":"test","scope":"global","config":{}}'

# Note: config payloads should not include secrets when echoing/logging requests
```

## Notes

These endpoints are the contract between backend and UI. Design carefully for:
- Ease of use from frontend
- Proper HTTP semantics (status codes, methods)
- Useful error messages
- Performance (eager loading where needed)

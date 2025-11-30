# Shelly Manager API Overview Report

## Executive Summary

The Shelly Manager exposes **112+ REST API endpoints** organized across 6 handler modules, with a comprehensive security middleware stack and standardized response format. The API follows REST conventions with a single version (`/api/v1`) and supports real-time WebSocket communication for metrics.

---

## API Base Structure

| Base Path | Purpose |
|-----------|---------|
| `/healthz`, `/readyz`, `/version` | Health & status checks (no auth) |
| `/api/v1/*` | All protected API endpoints |
| `/metrics/*` | Metrics and monitoring endpoints |

---

## Endpoint Categories

### 1. Health & Version (3 endpoints)

| Method | Endpoint | Description | Input | Output |
|--------|----------|-------------|-------|--------|
| GET | `/healthz` | Liveness probe | - | `{"status": "ok"}` |
| GET | `/readyz` | Readiness probe | - | `{"status": "ready"}` |
| GET | `/version` | API version info | - | `{"version": "...", "build": "..."}` |

---

### 2. Device Management (8 endpoints)

| Method | Endpoint | Description | Input | Output |
|--------|----------|-------------|-------|--------|
| GET | `/api/v1/devices` | List all devices | Query: `page`, `page_size` | Paginated device list |
| POST | `/api/v1/devices` | Add new device | `{ip, mac, type, name, firmware, settings}` | Created device |
| GET | `/api/v1/devices/{id}` | Get single device | Path: `id` | Device object |
| PUT | `/api/v1/devices/{id}` | Update device | Path: `id`, Body: device fields | Updated device |
| DELETE | `/api/v1/devices/{id}` | Delete device | Path: `id` | Success confirmation |
| POST | `/api/v1/devices/{id}/control` | Control device | `{action, params}` | Action result |
| GET | `/api/v1/devices/{id}/status` | Get device status | Path: `id` | Status object |
| GET | `/api/v1/devices/{id}/energy` | Get energy metrics | Path: `id` | Energy data |

**Device Model:**
```json
{
  "id": 1,
  "ip": "192.168.1.100",
  "mac": "AA:BB:CC:DD:EE:FF",
  "type": "SHSW-1",
  "name": "Living Room Switch",
  "firmware": "1.0.0",
  "status": "online",
  "last_seen": "2025-11-30T12:00:00Z",
  "settings": "{...}",
  "created_at": "...",
  "updated_at": "..."
}
```

---

### 3. Device Configuration (11 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices/{id}/config` | Get stored device config |
| PUT | `/api/v1/devices/{id}/config` | Update stored config |
| GET | `/api/v1/devices/{id}/config/current` | Get live config from device |
| GET | `/api/v1/devices/{id}/config/current/normalized` | Get normalized live config |
| GET | `/api/v1/devices/{id}/config/typed/normalized` | Get typed normalized config |
| POST | `/api/v1/devices/{id}/config/import` | Import config to device |
| GET | `/api/v1/devices/{id}/config/status` | Get import status |
| POST | `/api/v1/devices/{id}/config/export` | Export config from device |
| GET | `/api/v1/devices/{id}/config/drift` | Detect configuration drift |
| POST | `/api/v1/devices/{id}/config/apply-template` | Apply template to device |
| GET | `/api/v1/devices/{id}/config/history` | Get config change history |

---

### 4. Capability-Specific Configuration (5 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| PUT | `/api/v1/devices/{id}/config/relay` | Update relay settings |
| PUT | `/api/v1/devices/{id}/config/dimming` | Update dimming settings |
| PUT | `/api/v1/devices/{id}/config/roller` | Update roller/shutter settings |
| PUT | `/api/v1/devices/{id}/config/power-metering` | Update power metering settings |
| PUT | `/api/v1/devices/{id}/config/auth` | Update device authentication |

---

### 5. Configuration Templates (4 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| GET | `/api/v1/config/templates` | List templates | Query: pagination |
| POST | `/api/v1/config/templates` | Create template | `{name, description, device_type, generation, config, variables}` |
| PUT | `/api/v1/config/templates/{id}` | Update template | Template fields |
| DELETE | `/api/v1/config/templates/{id}` | Delete template | Path: `id` |

**Template Model:**
```json
{
  "id": 1,
  "name": "Office Lights Standard",
  "description": "Standard config for office light switches",
  "device_type": "SHSW-1",
  "generation": 2,
  "config": {...},
  "variables": {...},
  "is_default": false
}
```

---

### 6. Template Operations (4 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/configuration/preview-template` | Preview template rendering | `{template, variables}` |
| POST | `/api/v1/configuration/validate-template` | Validate template syntax | `{template}` |
| POST | `/api/v1/configuration/templates` | Save template | `{name, template, variables}` |
| GET | `/api/v1/configuration/template-examples` | Get example templates | - |

---

### 7. Typed Configuration (8 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices/{id}/config/typed` | Get typed config |
| PUT | `/api/v1/devices/{id}/config/typed` | Update typed config |
| GET | `/api/v1/devices/{id}/capabilities` | Get device capabilities |
| POST | `/api/v1/configuration/validate-typed` | Validate typed config |
| POST | `/api/v1/configuration/convert-to-typed` | Convert raw to typed |
| POST | `/api/v1/configuration/convert-to-raw` | Convert typed to raw |
| GET | `/api/v1/configuration/schema` | Get configuration schema |
| POST | `/api/v1/configuration/bulk-validate` | Bulk validate configs |

---

### 8. Bulk Operations (4 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/config/bulk-import` | Import configs to multiple devices | Device IDs + config |
| POST | `/api/v1/config/bulk-export` | Export configs from multiple devices | Device IDs |
| POST | `/api/v1/config/bulk-drift-detect` | Detect drift on multiple devices | Device IDs |
| POST | `/api/v1/config/bulk-drift-detect-enhanced` | Enhanced drift detection | Device IDs + options |

---

### 9. Drift Detection Schedules (7 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/config/drift-schedules` | List drift schedules |
| POST | `/api/v1/config/drift-schedules` | Create schedule |
| GET | `/api/v1/config/drift-schedules/{id}` | Get schedule |
| PUT | `/api/v1/config/drift-schedules/{id}` | Update schedule |
| DELETE | `/api/v1/config/drift-schedules/{id}` | Delete schedule |
| POST | `/api/v1/config/drift-schedules/{id}/toggle` | Enable/disable schedule |
| GET | `/api/v1/config/drift-schedules/{id}/runs` | Get schedule run history |

---

### 10. Drift Reporting (4 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/config/drift-reports` | Get all drift reports |
| GET | `/api/v1/config/drift-trends` | Get drift trends over time |
| POST | `/api/v1/config/drift-trends/{id}/resolve` | Mark trend as resolved |
| POST | `/api/v1/devices/{id}/drift-report` | Generate device drift report |

**Drift Difference Model:**
```json
{
  "path": "relay.0.name",
  "expected": "Kitchen Light",
  "actual": "Light 1",
  "type": "modified",
  "severity": "warning",
  "category": "device",
  "description": "Relay name changed",
  "impact": "Device identification affected",
  "suggestion": "Sync config to restore expected value"
}
```

---

### 11. Export/Backup Operations (21 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/export/plugins` | List available export plugins |
| GET | `/api/v1/export/plugins/{name}` | Get plugin details |
| GET | `/api/v1/export/plugins/{name}/schema` | Get plugin configuration schema |
| POST | `/api/v1/export/backup` | Create backup |
| GET | `/api/v1/export/backup/{id}` | Get backup details |
| GET | `/api/v1/export/backup/{id}/download` | Download backup file |
| DELETE | `/api/v1/export/backup/{id}` | Delete backup |
| GET | `/api/v1/export/backups` | List backups (compat) |
| GET | `/api/v1/export/backup-statistics` | Get backup statistics |
| POST | `/api/v1/export/json` | Create JSON export |
| POST | `/api/v1/export/sma` | Create SMA format export |
| POST | `/api/v1/export/yaml` | Create YAML export |
| POST | `/api/v1/export/gitops` | Create GitOps export |
| GET | `/api/v1/export/gitops/{id}/download` | Download GitOps export |
| POST | `/api/v1/export` | Generic export |
| POST | `/api/v1/export/preview` | Preview export |
| GET | `/api/v1/export/{id}` | Get export result |
| GET | `/api/v1/export/{id}/download` | Download export |
| GET | `/api/v1/export/history` | List export history |
| GET | `/api/v1/export/history/{id}` | Get history item |
| GET | `/api/v1/export/statistics` | Get export statistics |

**Export Request Model:**
```json
{
  "plugin_name": "sma",
  "format": "json",
  "config": {},
  "filters": {
    "device_ids": [1, 2, 3],
    "device_types": ["SHSW-1"],
    "time_range": {"start": "...", "end": "..."}
  },
  "output": {"type": "file", "path": "/exports/"},
  "options": {
    "compression": "gzip",
    "include_metadata": true
  }
}
```

---

### 12. Export Scheduling (6 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/export/schedules` | List export schedules |
| POST | `/api/v1/export/schedules` | Create schedule (cron-based) |
| GET | `/api/v1/export/schedules/{id}` | Get schedule |
| PUT | `/api/v1/export/schedules/{id}` | Update schedule |
| DELETE | `/api/v1/export/schedules/{id}` | Delete schedule |
| POST | `/api/v1/export/schedules/{id}/run` | Manually run schedule |

---

### 13. Import Operations (10 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/import/backup` | Restore from backup |
| POST | `/api/v1/import/backup/validate` | Validate backup file |
| POST | `/api/v1/import/gitops` | Import GitOps config |
| POST | `/api/v1/import/gitops/preview` | Preview GitOps import |
| GET | `/api/v1/import/history` | List import history |
| GET | `/api/v1/import/history/{id}` | Get history item |
| GET | `/api/v1/import/statistics` | Get import statistics |
| POST | `/api/v1/import` | Generic import |
| POST | `/api/v1/import/preview` | Preview import |
| GET | `/api/v1/import/{id}` | Get import result |

**Import Options:**
```json
{
  "dry_run": true,
  "validate_only": false,
  "skip_errors": false
}
```

---

### 14. Notification System (8 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/notifications/channels` | Create notification channel |
| GET | `/api/v1/notifications/channels` | List channels |
| PUT | `/api/v1/notifications/channels/{id}` | Update channel |
| DELETE | `/api/v1/notifications/channels/{id}` | Delete channel |
| POST | `/api/v1/notifications/channels/{id}/test` | Test channel |
| POST | `/api/v1/notifications/rules` | Create notification rule |
| GET | `/api/v1/notifications/rules` | List rules |
| GET | `/api/v1/notifications/history` | Get notification history |

**Channel Types:** `email`, `webhook`, `slack`, `discord`

**Channel Config Example (Webhook):**
```json
{
  "name": "Ops Webhook",
  "type": "webhook",
  "enabled": true,
  "config": {
    "url": "https://hooks.example.com/alert",
    "method": "POST",
    "headers": {"Authorization": "Bearer ..."},
    "timeout": 30,
    "retries": 3
  }
}
```

---

### 15. Metrics & Monitoring (15 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/metrics/prometheus` | Prometheus metrics export |
| GET | `/metrics/status` | Metrics collection status |
| POST | `/metrics/enable` | Enable metrics collection |
| POST | `/metrics/disable` | Disable metrics collection |
| POST | `/metrics/collect` | Trigger metrics collection |
| GET | `/metrics/dashboard` | Dashboard metrics summary |
| GET | `/metrics/ws` | WebSocket real-time stream |
| POST | `/metrics/test-alert` | Send test alert |
| GET | `/metrics/health` | Metrics system health |
| GET | `/metrics/system` | System metrics |
| GET | `/metrics/devices` | Device metrics |
| GET | `/metrics/drift` | Drift summary metrics |
| GET | `/metrics/notifications` | Notification metrics |
| GET | `/metrics/resolution` | Resolution metrics |
| GET | `/metrics/security` | Security metrics |

---

### 16. Discovery & Provisioning (3 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/discover` | Discover devices on network | `{network, import_config}` |
| GET | `/api/v1/provisioning/status` | Get provisioning status | - |
| POST | `/api/v1/provisioning/provision` | Provision discovered devices | Device list |

---

### 17. Provisioner Agent Management (9 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/provisioner/agents/register` | Register provisioner agent |
| GET | `/api/v1/provisioner/agents` | List registered agents |
| GET | `/api/v1/provisioner/agents/{id}/tasks` | Poll tasks for agent |
| POST | `/api/v1/provisioner/tasks` | Create provisioning task |
| GET | `/api/v1/provisioner/tasks` | List tasks |
| PUT | `/api/v1/provisioner/tasks/{id}/status` | Update task status |
| POST | `/api/v1/provisioner/discovered-devices` | Report discovered devices |
| GET | `/api/v1/provisioner/discovered-devices` | Get discovered devices |
| GET | `/api/v1/provisioner/health` | Provisioner health check |

---

### 18. DHCP (1 endpoint)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/dhcp/reservations` | Get DHCP reservations |

---

### 19. Admin Operations (1 endpoint)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/admin/rotate-admin-key` | Rotate API key | `{new_key}` |

---

## Standardized Response Format

All API responses follow this envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": { ... }
  },
  "meta": {
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total_pages": 5,
      "has_next": true,
      "has_previous": false
    },
    "count": 20,
    "total_count": 100,
    "version": "v1"
  },
  "timestamp": "2025-11-30T12:00:00Z",
  "request_id": "abc123"
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request format |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `METHOD_NOT_ALLOWED` | 405 | HTTP method not supported |
| `CONFLICT` | 409 | Resource conflict |
| `VALIDATION_FAILED` | 400 | Input validation error |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `REQUEST_TOO_LARGE` | 413 | Payload too large |
| `INTERNAL_SERVER_ERROR` | 500 | Server error |
| `DEVICE_NOT_FOUND` | 404 | Device does not exist |
| `DEVICE_OFFLINE` | 503 | Device unreachable |
| `CONFIGURATION_ERROR` | 400 | Config operation failed |
| `TEMPLATE_ERROR` | 400 | Template processing error |

---

## Security & Middleware

### Middleware Stack (15 layers)
1. Recovery (panic handling)
2. IP Blocking
3. Security Monitoring
4. Security Logging
5. Security Headers (CSP, HSTS, etc.)
6. Request Timeout (30s default)
7. Rate Limiting (1000 req/hour default)
8. Request Size Limiting (10MB)
9. Header Validation
10. Content-Type Validation
11. Query Parameter Validation
12. JSON Validation (depth: 10, array: 1000)
13. CORS
14. HTTP Logging
15. Prometheus Metrics

### Rate Limits by Path
| Path Pattern | Limit |
|--------------|-------|
| Default | 1,000/hour |
| `/devices/{id}/control` | 100/hour |
| `/provisioning/*` | 50/hour |
| `/config/bulk*` | 20/hour |

### Authentication
- Header: `Authorization: Bearer {api_key}`
- Header: `X-API-Key: {api_key}`

---

## Key Files Reference

| File | Purpose |
|------|---------|
| `internal/api/router.go` | Route registration |
| `internal/api/handlers.go` | Core device/config handlers |
| `internal/api/typed_config_handlers.go` | Typed config handlers |
| `internal/api/sync_handlers.go` | Export/backup handlers |
| `internal/api/import_handlers.go` | Import handlers |
| `internal/api/provisioner_handlers.go` | Provisioner handlers |
| `internal/api/response/response.go` | Response formatting |
| `internal/api/middleware/security.go` | Security middleware |
| `internal/api/middleware/validation.go` | Validation middleware |
| `internal/notification/handlers.go` | Notification handlers |
| `internal/metrics/handlers.go` | Metrics handlers |
| `internal/database/models.go` | Database models |
| `internal/configuration/models.go` | Config models |
| `internal/sync/data.go` | Export/import models |

---

## Summary Statistics

- **Total Endpoints**: 112+
- **Handler Modules**: 6 main files
- **HTTP Methods**: GET, POST, PUT, DELETE, OPTIONS
- **API Version**: v1 (single version)
- **WebSocket**: `/metrics/ws` for real-time data
- **Export Formats**: JSON, YAML, SMA, GitOps
- **Notification Channels**: Email, Webhook, Slack, Discord

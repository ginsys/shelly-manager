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
| GET | `/version`, `/api/v1/version` | API version info. The UI uses the `/api/v1` path. | - | Standard envelope; `data` holds `api_version`, `server_started_at`, `git_sha` (the revision the server was built from, empty when the build has none) and, when known, `database_provider_name` and `database_provider_version` |

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

### 3. Device Configuration (14 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices/{id}/config` | Get stored device config |
| PUT | `/api/v1/devices/{id}/config` | Update stored config |
| GET | `/api/v1/devices/{id}/config/current` | Get live config from device (stores nothing; drift keeps its baseline) |
| GET | `/api/v1/devices/{id}/config/current/normalized` | Get normalized live config (stores nothing) |
| GET | `/api/v1/devices/{id}/config/typed/normalized` | Get typed normalized config |
| POST | `/api/v1/devices/{id}/config/import` | Import config to device |
| GET | `/api/v1/devices/{id}/config/status` | Get import status |
| POST | `/api/v1/devices/{id}/config/export` | Export config from device |
| GET | `/api/v1/devices/{id}/config/drift` | Detect configuration drift |
| POST | `/api/v1/devices/{id}/config/apply-template` | Apply template to device |
| GET | `/api/v1/devices/{id}/config/history` | Get config change history |
| GET | `/api/v1/devices/{id}/desired-config` | Get the desired configuration and what the device inherits from its templates |
| GET | `/api/v1/devices/{id}/config/new/status` | Get whether the desired configuration is applied |
| POST | `/api/v1/devices/{id}/config/new/apply` | Write the desired configuration to the device (admin key) |
| POST | `/api/v1/devices/{id}/config/new/verify` | Compare the desired configuration with the device |

#### The desired configuration and its inherited part

`GET /devices/{id}/desired-config` returns `config` and `sources`: the composed result of the device's templates and overrides, with the layer each value comes from. It also returns `inherited` and `inherited_sources`: the template layers alone, which is what a field falls back to when its override is removed. A field that a template and an override both set shows only the override in `config`, and the template value in `inherited`. A set secret is removed from `config`. In `inherited` it shows as the mask `********`, never as its value, so a client can see that a password is set. `PUT /devices/{id}/templates/new` returns the same two fields beside its `desired_config` and `sources`.

The device overrides routes (`GET`, `PUT` and `PATCH /devices/{id}/overrides/new`) show a saved secret as the same mask `********`. The secrets are the Wi-Fi password, the access point password and key, the MQTT password, the authentication password, and the station and access point passwords of the `network.wifi` part. The desired-config, template and overrides routes remove the secrets (`config`, `desired_config`) or mask them (`inherited`, `overrides`). A write that sends the mask back keeps the stored secret. If no secret is stored, the field is dropped. The mask is never stored as a value. Any other value replaces the stored secret.

#### Secrets in the device routes

The device routes show a saved secret as the mask `********` and never return its value: `GET /devices`, `GET`, `POST` and `PUT /devices/{id}`, and the three configuration reads `GET /devices/{id}/config`, `POST /devices/{id}/config/import` and `GET /devices/{id}/config/current`. A device holds its secrets inside JSON text (`settings`, `overrides`, `desired_config`), and the configuration reads return the device's own raw settings, so these routes mask by key name: a string or number value under a key that contains `pass`, `pwd`, `psk`, `secret` or `token`, or that ends in `key`, such as the Gen1 `wifi_sta.key`, `wifi_ap.key`, `login.password` and `mqtt.pass`, or the Gen2 `wifi.sta.pass`, `wifi.ap.pass` and `mqtt.pass`. An empty value stays empty. A text that is not valid JSON cannot be read for key names, so it is returned as the mask whole. The stored record keeps the real values, so drift detection and apply are unchanged. `PUT /devices/{id}`, `POST /devices` and `PUT /devices/{id}/config` treat a value that comes back as the mask like the overrides routes do: it takes the stored value, or is dropped when none is stored. The answer to `PUT /devices/{id}/config` shows a secret as the mask too. `PUT /devices/{id}/config/auth` takes the stored login password when the password is the mask, and refuses it with 400 when none is stored. `GET /devices/{id}/config/current/normalized` carries no secret field. The typed routes `GET /devices/{id}/config/typed` and `.../typed/normalized` mask the same way, in the typed sections and, by key name, in the `raw` part. `PUT /devices/{id}/config/typed` follows the rules of the overrides routes: a secret that is the mask or is left out keeps its stored value, an empty string or `null` removes it, and the mask is never stored. The same holds for a secret inside `raw`. A configuration stored in the device's own (Gen1) shape converts `wifi_sta.key` to the typed Wi-Fi password and `login.password` to the typed login password, so the typed read masks them and a save keeps them; a `pass` that is present, even as `null`, takes precedence over these spellings. A save stores the typed shape. In an array, a masked or omitted secret returns only to the stored entry at the same position that matches it on everything but its secrets; where more than one stored entry matches and their secrets differ, the request is refused with 400 and asks for the secret value. The answer is masked too, validation messages included. Limit: the `network.wifi.*.pass` fields are plain strings, so an empty value there keeps the stored password; send `null` to clear one.

#### Apply and verify the desired configuration

The desired configuration is the composed result of the device's templates and overrides. Neither route takes a request body.

`POST /devices/{id}/config/new/apply` writes the desired configuration to the device. Then it reads the configuration back and compares it. If the server has an admin key configured, the route needs that key, as `Authorization: Bearer <key>` or `X-API-Key: <key>`. Without the key it then returns 401 `UNAUTHORIZED`. If no admin key is configured, the route is open, like the other admin routes. Apply clears `config_applied` before the write. It sets the flag again only if the read-back matches and the desired configuration did not change during the apply. Applies to one device run one at a time in one server process. Run one manager replica if you use apply, because applies on different replicas are not serialized (ginsys/shelly-manager#425).

The response `data` holds these fields:

| Field | Meaning |
|-------|---------|
| `success` | `true` if the read-back matched and `config_applied` is set |
| `applied_count`, `failed_count` | Settings the device accepted and refused |
| `requires_reboot` | The device needs a reboot for a change to take effect |
| `failures` | Each refused setting by path, and each read-back difference. Secret values are masked |
| `warnings` | Problems that did not stop the apply |

`POST /devices/{id}/config/new/verify` reads the device and compares it with the desired configuration. It writes nothing. The response `data` holds `match` and `differences`. Each difference has `path`, `expected` and `actual`. Secret values (passwords, keys, tokens) are masked.

Supported devices and settings:

- Gen1 only. Gen2 and Gen3 return 501 `NOT_IMPLEMENTED`.
- Supported Gen1 settings: name, timezone, location, eco mode, discoverable, max power, LED, MQTT and relays. A MQTT server given as `host:port` matches the device's split read-back. The MQTT password is write-only on Gen1, so verify does not compare it.
- Relays: for each entry of `relays`, apply writes `default_state`, `auto_on`, `auto_off` and `name` to that channel with `POST /settings/relay/<n>`, then reads the channel back. A value the device did not keep fails with the path, for example `relays[0].auto_off`. Each relay `id` must equal its position in the list, because the position is the channel written. Any other relay key, such as `schedule`, and an `id` that differs from its position return 422 `VALIDATION_FAILED`.
- On apply, any other section (Wi-Fi, cloud, CoIoT, login, inputs) returns 422 `VALIDATION_FAILED`, and nothing is written. A section the converter cannot map, such as SNTP, returns 422 too. The error names the sections. Verify does not refuse these sections. It compares them with the device like the supported ones.
- On both routes, a stored desired configuration with fields the configuration model does not know returns 422. An example is a legacy row from an SMA import.
- If the device rejects the stored login, both routes return 502 `EXTERNAL_SERVICE_ERROR`. Store the right login with `PUT /devices/{id}/config/auth`.
- An unknown device returns 404.

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
| GET | `/api/v1/config/templates` | List templates (admin key) | Query: pagination |
| POST | `/api/v1/config/templates` | Create template (admin key) | `{name, description, device_type, generation, config, variables}` |
| PUT | `/api/v1/config/templates/{id}` | Update template (admin key) | Template fields |
| DELETE | `/api/v1/config/templates/{id}` | Delete template (admin key) | Path: `id` |

**Admin key.** If the server has an admin key configured, these routes need it, as `Authorization: Bearer <key>` or `X-API-Key: <key>`. Without it they return 401 `UNAUTHORIZED`. The same holds for every route under `/api/v1/config/templates/new`, the four routes under `/api/v1/devices/{id}/templates/new` and `GET /api/v1/config/schema`. If no admin key is configured, the routes are open, like the other admin routes. The web UI sends the key it holds.

`POST` and `PUT` on `/api/v1/config/templates/new` check the `config` body against the schema of the template (the model for a `device_type` template, every stored section for a `global` or `group` template) and refuse it with 400 `VALIDATION_FAILED`, one entry per field in `error.details.fields`, and store nothing. The check covers the value of each field and the keys: an unknown key, a key that differs from the stored key only in case (`Relay` for `relay`), and a repeated key are refused with the key named, because the stored body is decoded without regard to case and merges repeated keys. A request key that is repeated or differs in case is refused the same way. The response returns a saved password as `has_wifi_password`, `has_mqtt_password` or `has_auth_password` and leaves the value out. It also returns `stored_secrets`, the path of each secret the template holds (for example `wifi.ap.pass`), which the three flags cannot give: `has_wifi_password` covers both the station and the access point password. A `PUT` without `config` keeps the stored config.

A `PUT` with a `config` replaces the config, except for secrets. A secret the body leaves out keeps its stored value, whether or not the section around it is in the body. A secret sent as the mask `********` takes the stored value, and the mask is never stored (a `POST` drops it). An empty string clears the secret. JSON `null` removes it (the JSON merge patch convention, RFC 7396): on the device overrides, the device then inherits the secret of its templates again, which is what the editor's Reset sends. So a secret path takes four values: omitted keeps the stored value, `""` clears it, the mask restores it and `null` removes it. The device overrides `PUT` follows the same rule, and an empty override secret is kept as empty, so it still beats the secret of a template. The station and access point passwords of the `network.wifi` part follow the same rule. The fields of `network.ethernet` and `network.wifi` are optional in the JSON: a field that is not set is left out, and a layer merge treats a missing field as inherited and an empty or `false` value as a real one. `PATCH` on the device overrides merges into the stored overrides, so it keeps every secret it leaves out. A secret path takes the same four values on `PATCH`. JSON `null` removes any field, not only a secret: `{"mqtt":{"server":null}}` removes the stored `mqtt.server`, and a `null` section such as `{"mqtt":null}` removes the whole section. A section that the removal leaves empty is removed too. A `null` for a field that is not stored changes nothing.

A `device_type` template names a model from `GET /api/v1/config/device-types`. `POST /api/v1/config/templates/new` refuses a `device_type` that is not in that list with 400 `VALIDATION_FAILED` and a `device_type` entry in `error.details.fields`, and stores nothing. An empty `device_type` on a `device_type` template is refused as before, and a `global` or `group` template ignores the field. `PUT /api/v1/config/templates/new/{id}` never changes the type of a template. It refuses a `device_type` in the body only when it differs from the stored type and is not in the list, so a template stored with another type stays editable. Only these two routes check the type. The older `/api/v1/config/templates` routes, the startup migration and the SMA import only require a `device_type` template to have a type, and accept any non-empty one.

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

### 7. Typed Configuration (9 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices/{id}/config/typed` | Get typed config |
| PUT | `/api/v1/devices/{id}/config/typed` | Update typed config |
| GET | `/api/v1/devices/{id}/capabilities` | Get device capabilities |
| POST | `/api/v1/configuration/validate-typed` | Validate typed config |
| POST | `/api/v1/configuration/convert-to-typed` | Convert raw to typed |
| POST | `/api/v1/configuration/convert-to-raw` | Convert typed to raw |
| GET | `/api/v1/config/schema` | Get configuration schema (admin key). With `?device_type=<model>` or `?device_id=<id>`: the stored-configuration schema for that model, with each field marked `x-applicable` when apply can write it. With `?scope=global` or `?scope=group`: the schema of such a template, every stored section |
| GET | `/api/v1/config/device-types` | List the models a `device_type` template can name. `data.device_types` holds `{device_type, name, generation}` for each of the six supported Gen1 models (`SHPLG-S`, `SHPLG-1`, `SHSW-1`, `SHSW-PM`, `SHSW-25`, `SHIX3-1`) (admin key) |
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

> **Fail-closed (#270):** drift schedules are stored but never executed in this
> release. The four execution-implying routes below return **HTTP 501
> Not Implemented** with no database side effects; only list, detail and delete
> are functional. Scheduled execution is tracked as #279.

| Method | Endpoint | Description | Status |
|--------|----------|-------------|--------|
| GET | `/api/v1/config/drift-schedules` | List drift schedules | ✅ Functional |
| POST | `/api/v1/config/drift-schedules` | Create schedule | ⛔ 501 Not Implemented |
| GET | `/api/v1/config/drift-schedules/{id}` | Get schedule | ✅ Functional |
| PUT | `/api/v1/config/drift-schedules/{id}` | Update schedule | ⛔ 501 Not Implemented |
| DELETE | `/api/v1/config/drift-schedules/{id}` | Delete schedule | ✅ Functional |
| POST | `/api/v1/config/drift-schedules/{id}/toggle` | Enable/disable schedule | ⛔ 501 Not Implemented |
| GET | `/api/v1/config/drift-schedules/{id}/runs` | Get schedule run history | ⛔ 501 Not Implemented |

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

### 12. Import Operations (10 endpoints)

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

### 13. Notification System (8 endpoints)

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

### 14. Metrics & Monitoring (15 endpoints)

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

### 15. Discovery & Provisioning (3 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/discover` | Discover devices on network | `{network, import_config}` |
| GET | `/api/v1/provisioning/status` | Get provisioning status | - |
| POST | `/api/v1/provisioning/provision` | Provision discovered devices | Device list |

---

### 16. Provisioner Agent Management (9 endpoints)

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

### 17. DHCP (1 endpoint)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/dhcp/reservations` | Get DHCP reservations |

---

### 18. Admin Operations (1 endpoint)

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
| `EXTERNAL_SERVICE_ERROR` | 502 | A device rejected its stored login on a live read (`/devices/{id}/config/current` and `/current/normalized`). Store the right login with `PUT /devices/{id}/config/auth`. |

`PUT /devices/{id}/config/auth` takes `{"username", "password"}`. A username with an empty password returns 400 `VALIDATION_FAILED`. An empty username turns device auth off.

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

### Client Requirements
- Every request needs a `User-Agent` header. An empty one is treated as an
  attack signal: the request fails with `400 VALIDATION_FAILED` and the security
  monitor counts it as suspicious activity from that client IP, which it can
  block. Browsers, `curl`, `wget`, `python-requests`, `axios`, `node-fetch` and
  Go's `net/http` send one by default. Node's core `http` module does not, so a
  client built on it must set one. Known scanner names such as `sqlmap` or
  `nmap` are refused too.
- `POST`, `PUT` and `PATCH` requests need a `Content-Type` header. A missing one
  fails with `400 VALIDATION_FAILED`.
- A client that sends no `User-Agent` (for example `curl -H 'User-Agent:'`) must
  set one.

### Authentication
- Header: `Authorization: Bearer {api_key}`
- Header: `X-API-Key: {api_key}`

**Admin key.** If the server has an admin key configured, these configuration routes need it and return 401 `UNAUTHORIZED` without it: every route under `/api/v1/devices/{id}/config` (the stored, live, typed, import, export, drift, history, apply-template, capability and apply routes), `/api/v1/devices/{id}/overrides/new`, `/api/v1/devices/{id}/desired-config` and `/api/v1/devices/{id}/drift-report`. The typed conversion and validation routes (`/api/v1/config/validate-typed`, `convert-to-typed`, `convert-to-raw`, `bulk-validate`) and the drift schedule and drift report routes under `/api/v1/config` need it too, as do the bulk import, export and drift routes, which check it in their handlers. The template routes under `/api/v1/config/templates`, the template assignment routes and `GET /api/v1/config/schema` need it too. If no admin key is configured, the routes are open. The web UI sends the key it holds on every call. `GET /api/v1/devices/{id}/capabilities` stays open.

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

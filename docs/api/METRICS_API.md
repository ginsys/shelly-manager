# Metrics API

This document summarizes the HTTP endpoints and WebSocket interface for metrics.

Note: The WebSocket endpoint may be registered at `/metrics/ws` (top-level) and is also available as `/metrics/ws` under the `/metrics` subrouter in some setups.

## HTTP Endpoints

Authentication policy (when `security.admin_api_key` is configured): every
endpoint below marked `(admin)` requires `Authorization: Bearer <ADMIN_KEY>` or
`X-API-Key: <ADMIN_KEY>` and returns `401` otherwise. `/metrics/prometheus` is
`(public)` by convention — standard scrapers do not send the admin bearer, so
secure it at the network layer. When no admin key is configured, all endpoints
are open.

- `GET /metrics/prometheus` (public)
  - Exposes Prometheus metrics. Suitable for Prometheus scraping.

- `GET /metrics/status` (admin)
  - Returns JSON status: `{ "enabled": true|false, "last_collection_time": "...", "uptime_seconds": n }`.

- `POST /metrics/enable` / `POST /metrics/disable` (admin)
  - Enables or disables metrics collection. Returns `{ "status": "enabled|disabled" }`.

- `POST /metrics/collect` (admin)
  - Triggers a manual collection. Response includes `{ "status": "collected", "duration_ms": n, "collected_at": "..." }`.

- `GET /metrics/dashboard` (admin)
  - Returns aggregated dashboard metrics as JSON (HTTP route; separate from WebSocket real-time stream).

- `POST /metrics/test-alert?type=<t>&severity=<s>` (admin)
  - Sends a synthetic alert over WebSocket broadcast for testing dashboards.

- `GET /metrics/health` (admin)
  - Overall health (enabled, last_collection_time, uptime_seconds).

- `GET /metrics/system` (admin)
  - SystemStatus block (uptime, metrics enabled, last collection, device counts).

- `GET /metrics/devices` (admin)
  - DeviceMetrics array for dashboards.

- `GET /metrics/drift` (admin)
  - DriftMetrics summary: totals, severity/category breakdowns.

- `GET /metrics/notifications` (admin)
  - NotificationMetrics summary: sent/failed and latency outline.

- `GET /metrics/resolution` (admin)
  - ResolutionMetrics summary: totals and success rates.

## WebSocket: `/metrics/ws`

- Connect with standard WebSocket client.
- Messages are broadcast by the server to connected clients to provide real-time dashboard updates and alerts.

Example client (JS):
```
const ws = new WebSocket("ws://localhost:8080/metrics/ws");
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log("metrics message", msg);
};
```

### Security

- When `security.admin_api_key` is configured, the WebSocket requires authentication.
- Authenticate by:
  - Header: `Authorization: Bearer <ADMIN_KEY>`, or
  - Query param: `/metrics/ws?token=<ADMIN_KEY>`
- Origins are restricted based on server CORS configuration.
- The server applies per-IP connection limits; excessive connections receive HTTP 429 before upgrade.

Example (browser) with token query param:
```
// Prefer wss:// in production
const token = "<ADMIN_KEY>";
const ws = new WebSocket(`wss://manager.example.com/metrics/ws?token=${encodeURIComponent(token)}`);
ws.onopen = () => console.log("connected");
ws.onmessage = (e) => console.log(JSON.parse(e.data));
ws.onclose = () => console.log("closed");
```

### Message Types

The server emits exactly the types below. The source of truth is
`internal/metrics/websocket.go` (`AllMessageTypes()`); the frontend mirror is
`ui/src/api/metricsMessages.ts`, and a cross-language contract test
(`TestMessageTypeManifestParity`) fails CI if the two diverge. Every frame is
`{ "type": <one of below>, "timestamp": "<RFC3339>", "data": {…} }`.

| `type` | When | `data` |
|--------|------|--------|
| `initial_metrics` | Once, immediately after a client connects | Full `DashboardMetrics` snapshot |
| `metrics_update` | Every 5s | Full `DashboardMetrics` snapshot |
| `alert` | `/metrics/test-alert` or backend sources | `{ alert_type, message, severity }` |
| `device_status_change` | A device goes online/offline | `{ device_id, device_name, old_status, new_status, timestamp }` |
| `drift_detected` | Configuration drift detected | `{ device_id, device_name, drift_count, severity, timestamp }` |

`DashboardMetrics` = `{ system_status, device_metrics[], drift_metrics, notification_metrics, resolution_metrics }`.
`system_status` = `{ uptime_seconds, metrics_enabled, last_collection_time, total_devices, online_devices, devices_with_drift }` — note there is **no** CPU/memory/disk telemetry; charts derive from device/drift counts.

Snapshot (`initial_metrics` / `metrics_update`):
```json
{
  "type": "metrics_update",
  "timestamp": "2026-01-01T00:00:00Z",
  "data": {
    "system_status": { "total_devices": 10, "online_devices": 7, "devices_with_drift": 2, "uptime_seconds": 3600, "metrics_enabled": true, "last_collection_time": "2026-01-01T00:00:00Z" },
    "device_metrics": [ { "id": "1", "name": "Living Room", "type": "shelly1", "status": "online", "config_synced": true, "last_seen": "2026-01-01T00:00:00Z" } ],
    "drift_metrics": { "total_drift_issues": 3, "severity_distribution": { "high": 1, "low": 2 }, "category_distribution": {}, "trend_analysis": [] },
    "notification_metrics": { "total_sent": 0, "total_failed": 0, "channel_breakdown": {}, "alert_level_breakdown": {}, "average_latency_seconds": 0 },
    "resolution_metrics": { "total_resolutions": 0, "auto_fix_success_rate": {}, "resolutions_by_category": {}, "average_review_time_seconds": 0 }
  }
}
```

Alert:
```json
{ "type": "alert", "timestamp": "…", "data": { "alert_type": "test", "severity": "warning", "message": "…" } }
```

**Client contract.** Treat the type set as a closed enum: an unrecognized `type`, or a payload that fails validation, must be surfaced (logged/counted) rather than silently applied, and must not be treated as a live feed. The reference UI keeps REST polling active until the first valid snapshot is applied, reports "live" only while snapshots keep arriving (a watchdog demotes a silent feed back to polling), and applies `device_status_change`/`drift_detected`/`alert` to a live-events feed.

## Production guidance

- Retention knobs and collection intervals are configured via `metrics.*` in the app config (see `configs/shelly-manager.yaml`).
- Restrict WebSocket origins via security config when deploying behind proxies.
- Prometheus scraping should be configured at controlled intervals; consider rate limiting at ingress.

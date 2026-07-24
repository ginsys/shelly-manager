# Operational Observability

This guide describes response metadata (version, pagination), request ID propagation, and log fields to improve traceability.

It also documents baseline health endpoints and Prometheus HTTP metrics exposed by the server.

## Response Metadata

All responses include a `timestamp`, and now `meta.version` is set by default:

```
{
  "success": true,
  "data": { ... },
  "meta": { "version": "v1" },
  "timestamp": "...",
  "request_id": "..."
}
```

List endpoints include pagination metadata when `page`/`page_size` are provided; otherwise they return a single page with all items:

```
{
  "success": true,
  "data": { "devices": [ ... ] },
  "meta": {
    "version": "v1",
    "pagination": {
      "page": 1,
      "page_size": 25,
      "total_pages": 4,
      "has_next": true,
      "has_previous": false
    },
    "count": 25,
    "total_count": 98
  },
  "timestamp": "...",
  "request_id": "..."
}
```

Endpoints updated to include pagination support:
- `GET /api/v1/devices` (single-page default when `page_size` omitted or `0`)
- `GET /api/v1/export/history` (with `plugin` and `success` filters)
- `GET /api/v1/import/history` (with `plugin` and `success` filters)
- `GET /api/v1/export/plugins`

## Request ID Propagation

- The HTTP logging middleware assigns a unique `request_id` to each request and stores it in the context.
- The API response writer extracts this value and returns it as `request_id` in every response.
- Use it to correlate client calls with server logs and traces.

## Log Fields

The API response layer and security middleware emit structured logs with:
- `method`, `path`: HTTP method and route path
- `status_code`: Response code for errors
- `error_code`, `error_msg`: Standardized error classification
- `request_id`: For correlating logs with responses
- `component`: Logical component tag (e.g., `api_response`, `security_monitor`, `rate_limiter`)

Security middleware also adds fields for rate limiting, suspicious patterns, and IP blocking with `security_event` classification.

## Health Endpoints

- `GET /healthz`: Liveness. Returns `{ "status": "ok" | "degraded" }`. Degraded if DB check fails, but process is up.
- `GET /readyz`: Readiness. Returns `503` until core dependencies (DB) are reachable. Success body: `{ "ready": true }`.

These endpoints are unauthenticated and lightweight, suitable for Kubernetes liveness/readiness probes.

## Prometheus HTTP Metrics

The server exposes baseline HTTP metrics via the `/metrics/prometheus` handler. Additional counters/histograms include:
- `shelly_http_requests_total{method, path, status_code}`
- `shelly_http_request_duration_seconds{method, path}`
- `shelly_http_response_size_bytes{method, path}`

Use `metrics.prometheus_enabled: true` in config and scrape `/metrics/prometheus`.

## Real-Time Metrics via WebSocket

Shelly Manager provides real-time metrics updates via WebSocket connection for live dashboard monitoring.

### Connection

Connect to the WebSocket endpoint:

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/metrics/ws')

ws.onopen = () => {
  console.log('WebSocket connected')
}

ws.onmessage = (event) => {
  const metrics = JSON.parse(event.data)
  console.log('Received metrics:', metrics)
}

ws.onerror = (error) => {
  console.error('WebSocket error:', error)
  // Fallback to REST polling
}

ws.onclose = (event) => {
  console.log('WebSocket closed:', event.code, event.reason)
  // Implement reconnection logic if needed
}
```

For authenticated connections, pass the admin key as a query parameter:

```javascript
const adminKey = 'your-admin-key'
const ws = new WebSocket(`ws://localhost:8080/api/v1/metrics/ws?token=${encodeURIComponent(adminKey)}`)
```

### Message Format

The server emits five message types, each `{ "type", "timestamp": "<RFC3339>", "data" }`.
The authoritative set lives in `internal/metrics/websocket.go` (`AllMessageTypes()`),
is mirrored in `ui/src/api/metricsMessages.ts`, and is enforced across the boundary
by a contract test. See [Metrics API — Message Types](../api/METRICS_API.md#message-types)
for the full payloads.

- `initial_metrics` / `metrics_update` — a full `DashboardMetrics` snapshot
  (`system_status`, `device_metrics[]`, `drift_metrics`, `notification_metrics`,
  `resolution_metrics`). `initial_metrics` is sent once on connect; `metrics_update`
  every 5s.
- `alert`, `device_status_change`, `drift_detected` — discrete events.

Snapshot example:

```json
{
  "type": "metrics_update",
  "timestamp": "2026-01-15T10:30:00Z",
  "data": {
    "system_status": { "total_devices": 25, "online_devices": 23, "devices_with_drift": 2, "uptime_seconds": 3600, "metrics_enabled": true, "last_collection_time": "2026-01-15T10:30:00Z" },
    "device_metrics": [],
    "drift_metrics": { "total_drift_issues": 3, "severity_distribution": { "high": 1, "low": 2 }, "category_distribution": {}, "trend_analysis": [] },
    "notification_metrics": { "total_sent": 0, "total_failed": 0, "channel_breakdown": {}, "alert_level_breakdown": {}, "average_latency_seconds": 0 },
    "resolution_metrics": { "total_resolutions": 0, "auto_fix_success_rate": {}, "resolutions_by_category": {}, "average_review_time_seconds": 0 }
  }
}
```

> There is **no** CPU/memory/disk telemetry in `system_status`; the dashboard's
> trend chart is built from device/drift counts, not host resource usage.

### Update Frequency

- Snapshots (`metrics_update`) are broadcast every 5 seconds; `initial_metrics`
  is sent once immediately on connect.
- Event messages (`alert` / `device_status_change` / `drift_detected`) are pushed
  as they occur.

### Automatic Failover

The UI treats WebSocket as an accelerator over REST, keyed to *applied data*, not
connection state:

- REST polling (every 30s) stays active until the **first valid snapshot is applied**,
  and resumes if the feed goes stale — an open socket alone is never treated as "live".
- A reactive watchdog demotes the feed from `live` to `stale` once ~20s pass with no
  applied snapshot, re-enabling REST.
- The client heartbeat recycles the socket (close-then-connect) after ~45s of silence;
  reconnection uses exponential backoff with jitter.
- A late frame from a superseded socket (e.g. during a recycle) is ignored via a
  connection-generation guard, and any frame that fails validation is surfaced rather
  than applied.

### WebSocket Lifecycle

1. **Connection**: Client connects to `/api/v1/metrics/ws`
2. **Authentication**: Token validated (if provided)
3. **Streaming**: Server sends periodic metric updates
4. **Heartbeat**: Implicit via regular messages (30s timeout)
5. **Disconnection**: Clean close on client disconnect or server shutdown
6. **Reconnection**: Client implements exponential backoff on connection loss

### Error Handling

WebSocket close codes:

- `1000`: Normal closure - client disconnect
- `1006`: Abnormal closure - connection lost (trigger reconnect)
- `1008`: Policy violation - authentication failed
- `1011`: Internal error - server error (trigger reconnect)

### Production Considerations

For production deployments with TLS:

```javascript
const ws = new WebSocket('wss://shelly.example.com/api/v1/metrics/ws')
```

Ensure your reverse proxy (nginx, traefik) supports WebSocket upgrades:

```nginx
location /api/v1/metrics/ws {
    proxy_pass http://shelly-manager:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 86400;  # 24 hours
}
```

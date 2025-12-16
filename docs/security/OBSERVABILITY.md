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
- `GET /api/v1/export/schedules`

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

The WebSocket sends periodic metrics updates in JSON format:

```json
{
  "type": "system",
  "data": {
    "cpu": 15.2,
    "memory": 45.8,
    "disk": 62.3,
    "timestamp": "2025-01-15T10:30:00Z"
  },
  "timestamp": "2025-01-15T10:30:00Z"
}
```

```json
{
  "type": "devices",
  "data": {
    "total": 25,
    "online": 23,
    "offline": 2
  },
  "timestamp": "2025-01-15T10:30:00Z"
}
```

```json
{
  "type": "drift",
  "data": {
    "total_drifts": 3,
    "unresolved": 2,
    "resolved": 1
  },
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### Update Frequency

- Metrics are broadcast every 5 seconds by default
- System metrics (CPU, memory, disk) update every 5s
- Device counts update on change or every 30s
- Drift summaries update on change or every 60s

### Automatic Failover

The UI automatically falls back to REST polling if WebSocket connection fails:

- **Initial connection timeout**: 5 seconds
- **Reconnection attempts**: 3 attempts with exponential backoff
- **Backoff delays**: 1s, 2s, 4s
- **Fallback polling interval**: 30 seconds via REST API
- **Heartbeat timeout**: 30 seconds of no messages triggers reconnection

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

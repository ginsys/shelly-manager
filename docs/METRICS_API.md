# Metrics API

This document summarizes the HTTP endpoints and WebSocket interface for metrics.

Note: The WebSocket endpoint may be registered at `/metrics/ws` (top-level) and is also available as `/metrics/ws` under the `/metrics` subrouter in some setups.

## HTTP Endpoints

- `GET /metrics/prometheus`
  - Exposes Prometheus metrics. Suitable for Prometheus scraping.

- `GET /metrics/status`
  - Returns JSON status: `{ "enabled": true|false, "last_collection_time": "...", "uptime_seconds": n }`.

- `POST /metrics/enable` / `POST /metrics/disable`
  - Enables or disables metrics collection. Returns `{ "status": "enabled|disabled" }`.

- `POST /metrics/collect`
  - Triggers a manual collection. Response includes `{ "status": "collected", "duration_ms": n, "collected_at": "..." }`.

- `GET /metrics/dashboard`
  - Returns aggregated dashboard metrics as JSON (HTTP route; separate from WebSocket real-time stream).

- `POST /metrics/test-alert?type=<t>&severity=<s>`
  - Sends a synthetic alert over WebSocket broadcast for testing dashboards.

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

### Message Types (typical)

- Dashboard update:
```
{
  "type": "dashboard",
  "timestamp": "...",
  "data": {
    // aggregated counters and gauges suitable for live UI
  }
}
```

- Alert broadcast (from `/metrics/test-alert` or back-end sources):
```
{
  "type": "alert",
  "timestamp": "...",
  "data": {
    "alert_type": "test|sql_injection|xss_attempt|...",
    "severity": "info|low|medium|high|critical",
    "message": "..."
  }
}
```

## Production guidance

- Retention knobs and collection intervals are configured via `metrics.*` in the app config (see `configs/shelly-manager.yaml`).
- Restrict WebSocket origins via security config when deploying behind proxies.
- Prometheus scraping should be configured at controlled intervals; consider rate limiting at ingress.

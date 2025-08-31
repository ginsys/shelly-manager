# Operational Observability

This guide describes response metadata (version, pagination), request ID propagation, and log fields to improve traceability.

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
- `GET /api/v1/devices`
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


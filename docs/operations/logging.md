# API Logging & Request Tracing

This document summarizes structured logging fields, request ID propagation, and response metadata for monitoring and debugging.

## Structured Log Fields

Standard log entries (HTTP/middleware/response) include:

- method: HTTP method
- path: Request path
- status_code: Response status (for request/error logs)
- duration: Request duration in milliseconds (HTTP middleware)
- request_id: Unique ID to correlate client responses and server logs
- component: Logical area (http, api_response, cors, rbac, database, security_monitor)

Additional fields appear contextually (e.g., error_code, error_msg, remote_addr). See also docs/security/OBSERVABILITY.md.

## Request ID Propagation

1. Generated in HTTP logging middleware (or preserved if already present) and added to the request context.
2. Response writer reads request_id from context and includes it in the JSON response as request_id.
3. Error responses also log request_id to simplify correlation.
4. For upstream proxies, propagate request IDs consistently (e.g., via X-Request-ID) and inject into the context early.

## Response Metadata

All responses include timestamp. Meta version is populated automatically for observability:

```
{
  "success": true,
  "data": { },
  "meta": { "version": "v1" },
  "timestamp": "...",
  "request_id": "..."
}
```

List endpoints return pagination metadata when applicable:

```
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
}
```

The pagination shape above is the standard across all list endpoints.


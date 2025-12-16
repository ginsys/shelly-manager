# Logging & Request Tracing

This guide documents the structured logging framework and request tracing capabilities in Shelly Manager for operational observability and debugging.

## Overview

Shelly Manager implements comprehensive structured logging with:
- Request ID propagation for distributed tracing
- Standardized log fields across all components
- JSON-formatted output for log aggregation
- Security event logging
- Performance monitoring

## Log Format

All logs are output in JSON format for easy parsing and aggregation:

```json
{
  "level": "info",
  "timestamp": "2025-12-03T10:15:30Z",
  "component": "api",
  "request_id": "8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c",
  "method": "GET",
  "path": "/api/v1/devices",
  "status_code": 200,
  "duration_ms": 45,
  "client_ip": "192.168.1.100",
  "user_agent": "Mozilla/5.0...",
  "message": "Request completed successfully"
}
```

## Standard Log Fields

### Core Fields (All Logs)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `level` | string | Log level | `debug`, `info`, `warn`, `error` |
| `timestamp` | string | ISO 8601 timestamp | `2025-12-03T10:15:30Z` |
| `component` | string | Component generating log | `api`, `database`, `plugin` |
| `message` | string | Human-readable message | `Device status updated` |

### Request Fields (HTTP Requests)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `request_id` | string | Unique request identifier (UUID) | `8f4c2a1b-3d5e-4f6a...` |
| `method` | string | HTTP method | `GET`, `POST`, `PUT`, `DELETE` |
| `path` | string | Request path | `/api/v1/devices/123` |
| `status_code` | integer | HTTP status code | `200`, `404`, `500` |
| `duration_ms` | integer | Request duration in milliseconds | `45`, `1250` |
| `client_ip` | string | Client IP address | `192.168.1.100` |
| `user_agent` | string | Client user agent | `Mozilla/5.0...` |

### Error Fields (Error Logs)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `error_code` | string | Error classification code | `DEVICE_NOT_FOUND` |
| `error_msg` | string | Error message | `Device with ID 123 not found` |
| `stack_trace` | string | Stack trace (debug builds) | `goroutine 1 [running]...` |

### Security Fields (Security Events)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `security_event` | string | Event classification | `rate_limit_exceeded` |
| `ip_address` | string | Source IP | `203.0.113.42` |
| `blocked` | boolean | Whether request was blocked | `true`, `false` |
| `threshold` | integer | Rate limit threshold | `100` |
| `count` | integer | Current count | `150` |

### Database Fields (Database Operations)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `query_duration_ms` | integer | Query execution time | `12`, `450` |
| `rows_affected` | integer | Number of rows modified | `1`, `25` |
| `database_error` | string | Database error message | `connection timeout` |

### Plugin Fields (Plugin Operations)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `plugin_name` | string | Plugin identifier | `gitops-exporter` |
| `plugin_version` | string | Plugin version | `1.0.0` |
| `operation` | string | Plugin operation | `export`, `sync`, `validate` |

## Request ID Propagation

Request IDs enable tracing a single request through the entire system.

### Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Client Request                                           │
│    GET /api/v1/devices                                      │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. HTTP Middleware                                          │
│    - Generate UUID: 8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c  │
│    - Store in context                                       │
│    - Set X-Request-ID header                                │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Handler Processing                                       │
│    - Extract request_id from context                        │
│    - Include in all log entries                             │
│    - Pass to downstream services                            │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Response                                                 │
│    - Include request_id in response body                    │
│    - Include X-Request-ID in response headers               │
│    - Log completion with request_id                         │
└─────────────────────────────────────────────────────────────┘
```

### Implementation

Request IDs are:
1. **Generated** by HTTP middleware at request entry
2. **Stored** in `context.Context` with key `request_id`
3. **Propagated** to all downstream operations
4. **Logged** in every log entry for that request
5. **Returned** in HTTP response headers (`X-Request-ID`)
6. **Included** in API response body (`request_id` field)

### Using Request IDs

#### In API Responses

All API responses include the request ID:

```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "version": "v1",
    "timestamp": "2025-12-03T10:15:30Z"
  },
  "request_id": "8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c"
}
```

#### In HTTP Headers

```bash
curl -I https://shelly.example.com/api/v1/devices

HTTP/1.1 200 OK
X-Request-ID: 8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c
Content-Type: application/json
...
```

#### In Logs

All log entries for a request share the same request_id:

```json
{"level":"info","timestamp":"2025-12-03T10:15:30.100Z","component":"api","request_id":"8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c","method":"GET","path":"/api/v1/devices","message":"Request received"}

{"level":"debug","timestamp":"2025-12-03T10:15:30.120Z","component":"database","request_id":"8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c","query_duration_ms":15,"rows_affected":25,"message":"Query executed"}

{"level":"info","timestamp":"2025-12-03T10:15:30.145Z","component":"api","request_id":"8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c","status_code":200,"duration_ms":45,"message":"Request completed"}
```

## Log Levels

Shelly Manager uses standard log levels:

| Level | Description | When to Use | Example |
|-------|-------------|-------------|---------|
| `debug` | Detailed diagnostic | Development debugging | Variable values, detailed state |
| `info` | General informational | Normal operations | Request completed, service started |
| `warn` | Warning conditions | Potential issues | Deprecated API used, retry attempt |
| `error` | Error conditions | Operation failures | Database error, API call failed |

### Configuration

Set log level via environment variable or config file:

```yaml
# config.yaml
logging:
  level: info  # debug, info, warn, error
  format: json # json, text
```

Or via environment variable:

```bash
export SHELLY_LOGGING_LEVEL=debug
./bin/shelly-manager server
```

## Component-Specific Logging

### API Layer

API logs include request/response details:

```json
{
  "level": "info",
  "component": "api",
  "request_id": "8f4c2a1b-3d5e-4f6a...",
  "method": "POST",
  "path": "/api/v1/devices",
  "status_code": 201,
  "duration_ms": 125,
  "user_agent": "Mozilla/5.0...",
  "message": "Device created successfully"
}
```

### Database Layer

Database logs include query performance:

```json
{
  "level": "debug",
  "component": "database",
  "request_id": "8f4c2a1b-3d5e-4f6a...",
  "query": "SELECT * FROM devices WHERE id = ?",
  "query_duration_ms": 12,
  "rows_affected": 1,
  "message": "Query executed"
}
```

### Plugin System

Plugin logs include plugin metadata:

```json
{
  "level": "info",
  "component": "plugins",
  "request_id": "8f4c2a1b-3d5e-4f6a...",
  "plugin_name": "gitops-exporter",
  "plugin_version": "1.0.0",
  "operation": "export",
  "message": "Plugin export completed"
}
```

### Security Monitor

Security events are logged with detailed context:

```json
{
  "level": "warn",
  "component": "security_monitor",
  "security_event": "rate_limit_exceeded",
  "request_id": "8f4c2a1b-3d5e-4f6a...",
  "ip_address": "203.0.113.42",
  "path": "/api/v1/devices",
  "threshold": 100,
  "count": 150,
  "blocked": true,
  "message": "Rate limit exceeded"
}
```

## Log Aggregation

### Parsing JSON Logs

All logs are JSON-formatted for easy parsing:

```bash
# Filter logs by request_id
cat shelly-manager.log | jq 'select(.request_id == "8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c")'

# Find slow requests (>1000ms)
cat shelly-manager.log | jq 'select(.duration_ms > 1000)'

# Count errors by component
cat shelly-manager.log | jq 'select(.level == "error") | .component' | sort | uniq -c

# Track security events
cat shelly-manager.log | jq 'select(.security_event != null)'
```

### Integration with Log Aggregators

#### Fluentd Configuration

```yaml
<source>
  @type tail
  path /var/log/shelly-manager/*.log
  pos_file /var/log/td-agent/shelly-manager.pos
  tag shelly.manager
  <parse>
    @type json
    time_key timestamp
    time_format %Y-%m-%dT%H:%M:%S.%LZ
  </parse>
</source>

<match shelly.manager>
  @type elasticsearch
  host elasticsearch.local
  port 9200
  index_name shelly-manager-%Y%m%d
  <buffer>
    @type file
    path /var/log/td-agent/buffer/shelly-manager
    flush_interval 10s
  </buffer>
</match>
```

#### ELK Stack (Elasticsearch + Logstash + Kibana)

Logstash configuration:

```ruby
input {
  file {
    path => "/var/log/shelly-manager/*.log"
    codec => "json"
  }
}

filter {
  json {
    source => "message"
  }

  date {
    match => ["timestamp", "ISO8601"]
    target => "@timestamp"
  }

  if [duration_ms] {
    mutate {
      convert => { "duration_ms" => "integer" }
    }
  }
}

output {
  elasticsearch {
    hosts => ["localhost:9200"]
    index => "shelly-manager-%{+YYYY.MM.dd}"
  }
}
```

#### Prometheus Metrics from Logs

Extract metrics from logs using mtail or similar:

```perl
# /etc/mtail/shelly-manager.mtail
counter http_requests_total by method, path, status_code
histogram http_request_duration_ms by method, path buckets 10, 50, 100, 500, 1000, 5000

/request completed/ {
  http_requests_total[$method][$path][$status_code]++
  http_request_duration_ms[$method][$path] = $duration_ms
}
```

## Troubleshooting with Logs

### Debugging a Failed Request

1. Get request_id from client error response or API response
2. Filter all logs for that request_id
3. Trace the request path through all components

```bash
# Extract all logs for a specific request
REQUEST_ID="8f4c2a1b-3d5e-4f6a-8b9c-1d2e3f4a5b6c"
cat /var/log/shelly-manager/app.log | jq "select(.request_id == \"$REQUEST_ID\")"
```

### Identifying Performance Bottlenecks

```bash
# Find slowest endpoints (top 10)
cat /var/log/shelly-manager/app.log | \
  jq 'select(.duration_ms != null) | {path, duration_ms}' | \
  jq -s 'sort_by(.duration_ms) | reverse | .[0:10]'

# Average response time by endpoint
cat /var/log/shelly-manager/app.log | \
  jq -s 'group_by(.path) | map({path: .[0].path, avg_ms: (map(.duration_ms) | add / length)})'
```

### Tracking Security Events

```bash
# List all security events
cat /var/log/shelly-manager/app.log | jq 'select(.security_event != null)'

# Count rate limit violations by IP
cat /var/log/shelly-manager/app.log | \
  jq 'select(.security_event == "rate_limit_exceeded") | .ip_address' | \
  sort | uniq -c | sort -rn

# Find blocked IPs
cat /var/log/shelly-manager/app.log | \
  jq 'select(.blocked == true) | .ip_address' | sort -u
```

### Monitoring Error Rates

```bash
# Error rate per minute
cat /var/log/shelly-manager/app.log | \
  jq 'select(.level == "error") | .timestamp[0:16]' | \
  uniq -c

# Top error messages
cat /var/log/shelly-manager/app.log | \
  jq 'select(.level == "error") | .error_msg' | \
  sort | uniq -c | sort -rn | head -10
```

## Log Rotation

### Using logrotate

```bash
# /etc/logrotate.d/shelly-manager
/var/log/shelly-manager/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0644 shelly shelly
    sharedscripts
    postrotate
        killall -SIGUSR1 shelly-manager || true
    endscript
}
```

### Using systemd-journald

For systemd deployments, logs are automatically managed:

```bash
# View logs
journalctl -u shelly-manager.service

# Follow logs
journalctl -u shelly-manager.service -f

# Filter by request ID
journalctl -u shelly-manager.service | grep "8f4c2a1b-3d5e-4f6a"

# View logs for specific time range
journalctl -u shelly-manager.service --since "2025-12-03 10:00:00" --until "2025-12-03 11:00:00"
```

## Best Practices

### For Operators

1. **Always include request_id** when reporting issues
2. **Set appropriate log levels** (info for production, debug for troubleshooting)
3. **Aggregate logs centrally** for distributed deployments
4. **Set up alerts** on error rates and security events
5. **Rotate logs regularly** to manage disk space
6. **Archive logs** for compliance and long-term analysis

### For Developers

1. **Use structured logging** (key-value pairs, not string concatenation)
2. **Include context** (request_id, user_id, resource_id)
3. **Log at appropriate levels** (don't log normal operations as errors)
4. **Avoid logging sensitive data** (passwords, API keys, PII)
5. **Use consistent field names** across components
6. **Add request_id** to all log entries within request context

### Security Considerations

**Do NOT log:**
- Passwords or API keys
- Session tokens or authentication credentials
- Personally identifiable information (PII) unless necessary
- Credit card numbers or payment details
- Full request/response bodies (may contain sensitive data)

**Do log:**
- Authentication attempts (success/failure) with username only
- Authorization decisions
- Security events (rate limiting, blocked IPs)
- Error messages with sanitized inputs
- Request metadata (method, path, status, duration)

## Reference

### Configuration Options

```yaml
# config.yaml
logging:
  # Log level: debug, info, warn, error
  level: info

  # Log format: json, text
  format: json

  # Log output: stdout, stderr, file path
  output: stdout

  # File rotation (if output is file path)
  max_size_mb: 100
  max_backups: 10
  max_age_days: 30
  compress: true
```

### Environment Variables

```bash
SHELLY_LOGGING_LEVEL=info       # Log level
SHELLY_LOGGING_FORMAT=json      # Log format
SHELLY_LOGGING_OUTPUT=stdout    # Log output destination
```

## See Also

- [Observability Guide](./OBSERVABILITY.md) - Metrics and monitoring
- [Security Guide](./SECURITY.md) - Security best practices
- [Operations Guide](../guides/OPERATIONS.md) - Operational procedures

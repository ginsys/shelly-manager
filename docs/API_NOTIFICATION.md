# Notification API

This document describes the Notification endpoints, standardized responses, and examples.

All endpoints return the standard API wrapper with `success|error`, `data`, optional `meta`, `timestamp`, and `request_id`.

## Endpoints

- Channels
  - `POST /api/v1/notifications/channels` — create channel
  - `GET /api/v1/notifications/channels` — list channels
  - `PUT /api/v1/notifications/channels/{id}` — update channel
  - `DELETE /api/v1/notifications/channels/{id}` — delete channel
  - `POST /api/v1/notifications/channels/{id}/test` — send test notification

- Rules
  - `POST /api/v1/notifications/rules` — create rule
  - `GET /api/v1/notifications/rules` — list rules (preloads channel)

- History
  - `GET /api/v1/notifications/history?limit=&offset=&channel_id=&status=` — list sent notifications with pagination/meta

## Channel object

```
{
  "id": 1,
  "name": "Admins",
  "type": "email|webhook|slack",
  "enabled": true,
  "config": { ... type-specific ... },
  "description": "...",
  "created_at": "...",
  "updated_at": "..."
}
```

Type-specific config examples:
- Email: `{ "recipients": ["ops@example.com"], "subject": "...", "template": "..." }`
- Webhook: `{ "url": "https://...", "method": "POST", "headers": {..}, "secret": "..." }`
- Slack: `{ "webhook_url": "https://hooks.slack.com/...", "channel": "#alerts" }`

## Rule object (selected fields)

```
{
  "id": 1,
  "name": "Critical Drift",
  "enabled": true,
  "channel_id": 1,
  "alert_level": "critical|warning|info|all",
  "min_severity": "critical|warning|info",
  "categories": ["security","device"],
  "min_interval_minutes": 30,
  "max_per_hour": 5,
  "schedule_enabled": true,
  "schedule_start": "08:00",
  "schedule_end": "20:00",
  "schedule_days": ["monday","tuesday",...]
}
```

## History response

```
GET /api/v1/notifications/history?limit=50&offset=0&channel_id=1&status=sent
200 OK
{
  "success": true,
  "data": {
    "history": [
      {
        "id": 10,
        "rule_id": 3,
        "channel_id": 1,
        "trigger_type": "drift_detected|schedule_run|manual|test",
        "device_id": 42,
        "subject": "...",
        "message": "...",
        "alert_level": "critical|warning|info",
        "status": "pending|sent|failed|retry",
        "sent_at": "...",
        "error": "..."
      }
    ]
  },
  "meta": {
    "count": 1,
    "total_count": 3,
    "pagination": { "page": 1, "page_size": 50, "total_pages": 1, "has_next": false, "has_previous": false }
  },
  "timestamp": "..."
}
```

## Error model

Common error codes: `VALIDATION_FAILED`, `NOT_FOUND`, `INTERNAL_SERVER_ERROR`.
Examples:
- Invalid body → `VALIDATION_FAILED` with details.
- Channel not found (test) → `NOT_FOUND`.
- Delete channel used by rules → `VALIDATION_FAILED` with explanatory message.

## Notes

- Rate limiting is enforced per rule via `min_interval_minutes` and `max_per_hour`.
- `min_severity` is honored in rule matching.
- Test endpoint triggers a synthetic notification without changing persisted rules.


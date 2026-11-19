# Notification API

This document describes the Notification endpoints, standardized responses, and examples.

All endpoints return the standard API wrapper with `success|error`, `data`, optional `meta`, `timestamp`, and `request_id`.

**Admin key.** If the server has an admin key configured, every notification route needs it
(`Authorization: Bearer <key>` or `X-API-Key: <key>`) and answers 401 `UNAUTHORIZED` without it. With no
admin key configured the routes are open. Even with the key, a read never returns a credential stored in a
channel's config (see "Secrets in a channel's config").

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

## Secrets in a channel's config

A channel's config holds credentials for its destination: a webhook `url` and `webhook_url` (a Slack or
Discord webhook URL is the token), `headers` (such as `Authorization`), and any key named like a secret
(`secret`, `token`, `password`, `api_key`, ...). Key names are matched without regard to case (`URL` and
`Headers` are accepted by the server, so they are credentials too). The value under such a key is one
credential, whatever its type: a string, a number, an object (`headers`) or a list (`"token_list": ["a", "b"]`).

- **Reads.** `GET /notifications/channels`, `GET /notifications/rules`, `GET /notifications/history` and the
  `POST /notifications/channels` response send `********` in place of that whole value (`"headers": "********"`).
  The same holds for the channel inside a rule or a history entry. Other fields (`recipients`, `method`,
  `channel`, `timeout`, ...) are returned as stored. An empty value stays empty.
- **Saves.** `PUT /notifications/channels/{id}` takes the stored value back for a credential key sent as
  `********`, so a client that read the config can send it back with one field changed. Any other value is a new
  credential and replaces the stored one: to change one header or one list entry, send the whole new object or
  list. A credential left out is removed: an update replaces the whole config, as before.
  The placeholder is never stored in place of a missing value: where nothing is stored for it, the key is
  dropped, and a channel left without a required `url` or `webhook_url` is refused. A stored config with two keys
  that differ only in case (`URL` and `url`) restores neither: send the value. A config that repeats a member
  name, with names matched the way the server's JSON decoder matches them and at any depth, is refused with 400
  on create and on update, because the server reads such a text in member order.
- **Delivery errors.** The error text of a failed delivery (`error` in `GET /notifications/history`, the message
  of a failed `POST /notifications/channels/{id}/test`, the log lines) names the operation and the failure class
  only, for example `webhook request failed: Post timed out`. It never carries the request URL, which holds the
  webhook token.
- **Old history rows.** A history row written by an earlier version holds an error text that quoted the request
  URL. At every start the server replaces the whole text of such a row by its first words and a note (for
  example `failed to send webhook: details removed`) and logs how many rows it changed, never their content. The
  old text is not kept.

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


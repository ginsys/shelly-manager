# Shelly Manager Docs Index

This index bundles the primary API docs for Phase 7.2 work. Each page includes endpoints, payload models, and example flows.

## API Docs

- Notification API: API_NOTIFICATION.md
  - Endpoints for channels, rules, test sends, and history with pagination. Includes rate limits and `min_severity` behavior.

- Export/Import API: API_EXPORT_IMPORT.md
  - Export/Import preview endpoints with summaries, dry-run and validate-only semantics, and schema notes.

- Metrics API: METRICS_API.md
  - Prometheus scrape endpoint, status/enable/disable/collect, dashboard websocket, and `/metrics/test-alert` for emitting test alerts.

## Related

- API Security Framework: API_SECURITY_FRAMEWORK.md
  - Authentication, authorization, and response standardization used across APIs.

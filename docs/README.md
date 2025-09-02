# Shelly Manager Docs Index

This index bundles the primary API docs for Phase 7.2 work. Each page includes endpoints, payload models, and example flows.

## API Docs

- Notification API: API_NOTIFICATION.md
  - Endpoints for channels, rules, test sends, and history with pagination. Includes rate limits and `min_severity` behavior.

- Export/Import API: API_EXPORT_IMPORT.md
  - Export/Import preview endpoints with summaries, dry-run and validate-only semantics, schema notes, history & statistics endpoints, and security (admin key + safe downloads).
  - Filters: `plugin` (case-sensitive), `success` (true/false/1/0/yes/no). Unknown plugin returns empty results.
  - Pagination: `page` and `page_size` with defaulting (`page<=0`→1, `page_size>100`→20, non-integer → defaults). Pagination metadata returned in `meta.pagination`.

- Metrics API: METRICS_API.md
  - Prometheus scrape endpoint, status/enable/disable/collect, dashboard websocket, and `/metrics/test-alert` for emitting test alerts.

## Related

- API Security Framework: API_SECURITY_FRAMEWORK.md
  - Authentication, authorization, and response standardization used across APIs.
 
- Phase 8 Plan: development/PHASE_8_WEB_UI_PLAN.md
  - SPA roadmap, milestones, and current progress for the new UI.
  
- Secrets & Secure Config: SECURITY_SECRETS.md
  - Using env vars and Kubernetes Secrets for admin key, SMTP/OPNSense credentials, provisioner API key, and safe export downloads (with *_FILE support).

- TLS/Proxy Hardening: SECURITY_TLS_PROXY.md
  - NGINX/Traefik examples for HTTPS enforcement, HSTS, and secure headers.

- Observability: OBSERVABILITY.md
  - Response `meta.version`, pagination metadata for list endpoints, request_id propagation, and log fields.

## Admin Operations

- Admin Key Rotation
  - `POST /api/v1/admin/rotate-admin-key` (requires current admin key): rotates in-memory key across API/WS/export/import handlers. Useful for emergency key rotation without restart.

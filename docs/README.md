# Shelly Manager Docs Index

This index bundles the primary API docs for Phase 7.2 work. Each page includes endpoints, payload models, and example flows.

## API Docs

- Notification API: API_NOTIFICATION.md
  - Endpoints for channels, rules, test sends, and history with pagination. Includes rate limits and `min_severity` behavior.

- Export/Import API: API_EXPORT_IMPORT.md
  - Export/Import preview endpoints with summaries, dry-run and validate-only semantics, schema notes, history & statistics endpoints, and security (admin key + safe downloads).

- Metrics API: METRICS_API.md
  - Prometheus scrape endpoint, status/enable/disable/collect, dashboard websocket, and `/metrics/test-alert` for emitting test alerts.

## Related

- API Security Framework: API_SECURITY_FRAMEWORK.md
  - Authentication, authorization, and response standardization used across APIs.
 
- Phase 8 Plan: development/PHASE_8_WEB_UI_PLAN.md
  - SPA roadmap, milestones, and current progress for the new UI.
  
- Secrets & Secure Config: SECURITY_SECRETS.md
  - Using env vars and Kubernetes Secrets for `ADMIN_API_KEY` and safe export downloads.

- TLS/Proxy Hardening: SECURITY_TLS_PROXY.md
  - NGINX/Traefik examples for HTTPS enforcement, HSTS, and secure headers.

- Observability: OBSERVABILITY.md
  - Response meta.version, pagination metadata, request_id propagation, and log fields.

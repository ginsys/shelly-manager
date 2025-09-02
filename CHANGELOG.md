# Changelog

All notable changes to this project are documented here. The project follows Conventional Commits.

## [Unreleased]

### Added
- Tests: API pagination and filters hardening
  - Devices: pagination meta, beyond-total pages, zero/omitted page_size, non-integer defaults, meta.version asserted.
  - Export/Import history: pagination meta, `plugin` and `success` filters (case-sensitive plugin), bounds/defaults (page=0 → 1, page_size>100 → 20, non-integer values → defaults), unknown plugin returns empty.
  - Statistics endpoints: asserted totals/success/failure and `by_plugin` counts.
- Secrets management (Phase 7.3.a):
  - Centralized secret resolution with `*_FILE` support (`internal/security/secrets`).
  - Env/file overrides for SMTP password, OPNSense API key/secret, admin key, and provisioner API key.
  - Docs: `docs/SECURITY_SECRETS.md` expanded with Compose/K8s and `*_FILE` examples; `.env.example` updated; Compose examples annotated.
- Admin key rotation endpoint:
  - `POST /api/v1/admin/rotate-admin-key` (guarded by current admin key) rotates in-memory key across API/WS/export/import handlers; logs audit event.
- TLS/Proxy hardening (Phase 7.3.b):
  - Expanded `docs/SECURITY_TLS_PROXY.md` with NGINX/Traefik examples, WS timeout annotation, and Kubernetes probe snippets.
- Operational observability (Phase 7.3.c):
  - Liveness `GET /healthz` and readiness `GET /readyz` endpoints.
  - Prometheus HTTP metrics middleware: request totals, durations, and response sizes.
- Notification API enablement (Phase 7.2.b):
  - Standardized responses for channels/rules/test/history.
  - History endpoint with filters (`channel_id`, `status`) and pagination meta.
  - Per‑rule rate limits (`min_interval_minutes`, `max_per_hour`) and `min_severity` matching.
  - API tests for channel CRUD, test endpoint, and history; service tests for rate limiting and severity.
- Export/Import readiness (Phase 7.2.a):
  - Export preview returns `data.preview` + `data.summary` (record_count, estimated_size).
  - Import preview enforces dry‑run + validate‑only and returns `data.preview` + `data.summary` (will_create/update/delete).
  - Tests for export/import preview summaries.
- Metrics documentation (Phase 7.2.c):
  - Documented HTTP endpoints and WebSocket usage with example message types.
  - Added `/metrics/test-alert` to metrics routes.
- Notification emitters integration (Phase 7.2.d):
  - Drift detection notifier hook in configuration service, wired to Notification Service in server startup.
  - Metrics test‑alert notifier hook wired to Notification Service.
- Documentation: added API docs
  - `docs/API_EXPORT_IMPORT.md`
  - `docs/API_NOTIFICATION.md`
  - `docs/METRICS_API.md`

### UI (Phase 8)
- New SPA scaffolding under `ui/` (Vue 3 + TS + Vite) with layout, router, and typed API layer.
- Devices list (pagination/search/sort) and Device detail pages.
- Dev convenience: server serves `/app-config.js` and `make run` auto-injects admin key (when configured) for development.
- Tests: Vitest unit tests for API layer (mocked) and a Playwright smoke test for Devices page.

### Metrics (Phase 7.5 backend)
- WebSocket security for `/metrics/ws` (admin‑key auth via Bearer token or `?token=`) with per‑IP connection limits and origin checks.
- New admin‑protected summary endpoints:
  - `GET /metrics/health`, `/metrics/system`, `/metrics/devices`, `/metrics/drift`, `/metrics/notifications`, `/metrics/resolution`.
- Service uptime seconds added to status/health.
- Docs: updated `docs/METRICS_API.md` with WS security notes and token client example.
- Tests: WS auth negative/positive paths; health endpoint auth.

### Security (Phase 7.3)
- Admin RBAC guard on export/import/schedules/history/statistics (config: `security.admin_api_key`).
- Safe download restriction to `export.output_directory` (403 when outside).
- Persisted export/import history; added history list/detail and statistics endpoints.
- Tests for RBAC, path restriction, and history endpoints.
- TLS/Proxy hardening docs: Added `docs/SECURITY_TLS_PROXY.md` with NGINX/Traefik examples (HTTPS redirect, HSTS, headers).

### Changed
- README: linked to detailed API docs and changelog.

### CI
- Ensured `make test-ci` passes (coverage 43.0%, lint green).

## [0.5.4-alpha] - existing baseline
- Refer to repository history for prior changes.

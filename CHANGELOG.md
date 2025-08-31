# Changelog

All notable changes to this project are documented here. The project follows Conventional Commits.

## [Unreleased]

### Added
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
- Ensured `make test-ci` passes (coverage 41.8%, lint green).

## [0.5.4-alpha] - existing baseline
- Refer to repository history for prior changes.

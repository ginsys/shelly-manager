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

### Changed
- README: linked to detailed API docs and changelog.

### CI
- Ensured `make test-ci` passes (coverage 41.8%, lint green).

## [0.5.4-alpha] - existing baseline
- Refer to repository history for prior changes.


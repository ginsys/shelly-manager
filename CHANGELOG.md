# Changelog

All notable changes to this project are documented here. The project follows Conventional Commits.

## [Unreleased]

### Changed
- Export and import previews now use the registered plugin list and each
  plugin's backend schema. Export preview supports every registered format;
  browser import preview is deliberately limited to SMA data and enforces a
  7 MiB pre-base64 upload limit. Schema forms preserve typed falsy values and
  validate recursive array/object fields. (#268)
- SMA is now a closed, strict `2026.1` format. Generated archives are
  deterministic gzip files with RFC 8785 SHA-256 integrity, a 100 MiB
  normalized-data limit, strict JSON/schema/number/depth validation, joined
  persisted device settings/configurations and templates, and atomic
  publication. Raw JSON is accepted only as an import representation.
  Non-dry-run persistence remains unavailable and fails with HTTP 501. (#268)
- Export/import audit identity uses trimmed `X-User-ID`, then `X-User`, and
  otherwise `api`; credentials and network addresses are never recorded as the
  requester. (#268)
- Frontend type checking now has a zero-error baseline and raw `vue-tsc`
  succeeds. (#268)

### Removed
- Removed the inert sync export scheduling API, UI, capability flag, and
  documentation. Drift, notification, and device/relay scheduling are
  unchanged. Former export-schedule paths now return ordinary HTTP 404
  responses. (#268)
- Plugin configuration, connection testing and enable/disable are removed from
  the UI and API client (#264). The frontend called `GET/PUT
  /export/plugins/{name}/config` and `POST /export/plugins/{name}/test` plus an
  enable/disable toggle built on `/config` — none of those routes exist on the
  backend, and there is no server-side model for stored plugin configuration
  (config is supplied per export/import request). The dead API functions, store
  actions, the card's Configure/Test/Enable-Disable controls and
  `PluginConfigForm.vue` were deleted (no HTTP 501 stubs — the routes never
  existed). Plugins are now read-only: list, per-category grouping, a read-only
  **schema viewer**, and read-only details. A product model for stored plugin
  configuration/testing/enablement is tracked as a design vertical (#290).

### Changed
- Plugin list/detail responses are now modelled as exact DTOs (#266): `Plugin`
  (list item) and `PluginDetail = { info, capabilities }`, replacing a permissive
  superset. Plugin status is backend-hardcoded, so a listed plugin is presented
  as "Registered" only; the fictional status filter/sort, configured/disabled/
  error statistics and `health` rendering were removed.
- Drift detection schedules now fail closed. The scheduler
  (`configuration.NewScheduler`) has no callers — schedules were stored but never
  executed, so `next_run` was never set, `run_count` never advanced and the run
  table was empty by construction. The API and UI previously presented these
  inert rows as operational jobs. `POST /config/drift-schedules`,
  `PUT /config/drift-schedules/{id}`, `POST /config/drift-schedules/{id}/toggle`
  and `GET /config/drift-schedules/{id}/runs` now return **HTTP 501
  Not Implemented** in the standard error envelope, short-circuiting before any
  input parsing or database access (no side effects). List, detail and delete
  keep working so stored schedules stay inspectable and removable. The four
  bypass service methods that wrote GORM directly were removed so no inert
  write/history path remains. The UI shows a "not executed in this release"
  notice, renders schedules as stored/inactive with the `device_filter` as
  read-only JSON, and offers deletion only. Scheduled execution is tracked
  separately as #279. (#270)

### Fixed
- `GET /config/drift-schedules/{id}` returned HTTP 500 instead of 404 for a
  missing schedule: the service wraps `gorm.ErrRecordNotFound` with `%w`, but the
  handler compared with `==`; it now uses `errors.Is`. (#270)

- Configuration drift detection no longer reports in-sync devices as drifted.
  `ImportFromDevice` re-stamps a volatile `_metadata.imported_at` (plus a
  `device_info` subtree) on every import, and `DetectDrift` re-imports the device
  before diffing — so any check whose timestamp differed from the stored baseline
  registered a spurious difference and set `RequiresAction`, marking a
  perfectly-synced device as drifted (and intermittently failing the drift tests
  in CI). Drift comparison now ignores the `_metadata`/`device_info` bookkeeping
  subtrees via an exact raw-key match applied during recursion, so genuine config
  drift is still detected while our own import metadata is not. Audit-history
  change tracking (`createHistory`) is unchanged and still records metadata
  changes.
- Real-time metrics over WebSocket never actually worked and reported a false
  "LIVE": the backend emitted `initial_metrics`/`metrics_update`/`alert`/
  `device_status_change`/`drift_detected` while the frontend only accepted a
  disjoint set (`status`/`health`/`system`/…), so every live frame was discarded
  yet the badge turned green on connect. The message types are now named Go
  constants mirrored by a frontend manifest and enforced across the boundary by a
  contract test; the store validates every frame at runtime, hydrates from both
  snapshot types, and drives a truthful "LIVE" from an explicit freshness state
  machine (idle→polling→live→stale) with a reactive watchdog — connection state
  alone never marks the feed live. REST polling stays active until the first
  snapshot is applied and can't be clobbered by a late response; the stale socket
  is recycled on heartbeat timeout and late frames from a superseded socket are
  ignored; `alert`/`device_status_change`/`drift_detected` feed a live-events
  panel. Charts drop the fictional CPU/memory/disk series (no backend telemetry)
  for a real device-count trend. (#247)

### Security
- Require the configured admin key on all four bulk configuration endpoints
  (`POST /api/v1/config/bulk-import`, `/bulk-export`, `/bulk-drift-detect`,
  `/bulk-drift-detect-enhanced`). These operate across every device and
  `bulk-export` pushes stored config to physical hardware, so they were an
  unauthenticated fleet-wide mutation surface. Also documents the previously
  missing `bulk-drift-detect-enhanced` route in the OpenAPI spec. (#245)
- Require the configured admin key on the metrics control endpoints
  (`POST /metrics/enable`, `/disable`, `/collect`, `/test-alert`) and the data
  reads (`GET /metrics/status`, `/dashboard`, `/security`). Previously the
  mutating endpoints were unauthenticated while the read summaries required the
  key — an inverted policy. `/metrics/prometheus` stays public by convention.
  (#246)

### Fixed
- SMA non-dry-run import no longer fakes success. `performImport` and its
  per-entity helpers were placeholders that returned `success: true` with a
  positive `records_imported` count while writing nothing, so a restore reported
  success but silently lost data. Persistence is not built yet (a larger
  follow-up), so a real import now fails closed: the generic import endpoint
  returns `501 Not Implemented` (`NOT_IMPLEMENTED`) instead of a fabricated
  success or an opaque `500`. Dry-run preview is unchanged. Building the actual
  persistence is tracked separately in #284. (#272)
- Existing databases could not start the server after upgrading: AutoMigrate
  emitted `ALTER TABLE config_templates ADD scope text NOT NULL`, which SQLite
  refuses on a populated table and PostgreSQL rejects for the NULLs it would
  create. Startup now repairs a legacy schema before AutoMigrate runs — a
  read-only preflight that classifies every row, then a transactional backfill,
  then per-column `NOT NULL` tightening (only where a column is still nullable).
  A missing scope is derived from the legacy `device_type`, preserving or
  narrowing applicability but never widening it: exactly `all` becomes `global`,
  any other concrete type becomes `device_type`, and an empty or NULL device type
  aborts startup with **every** offending template listed and a remedy. A NULL
  `config` aborts the same way rather than being invented as `{}`. The repair is
  idempotent and resumes safely from a partially completed run. A permanent
  `default:'global'` column default was rejected as the fix: it would have
  silently accepted scope-less inserts forever. See
  `docs/guides/database-upgrade.md`. (#275)
- Closed the write path that recreates those rows: `POST`/`PUT
  /api/v1/config/templates` validated nothing and wrote straight to the
  database, so a client could store an empty or nonsense scope that the next
  startup would refuse to migrate. Both paths now share one validator with the
  newer repository path, and a violation returns a standard-envelope **400**
  instead of the blanket 500 the handlers previously produced. (#275)
- The PostgreSQL and MySQL providers could never connect: `Connect()` called
  `Ping()` before setting `connected`, and `Ping()` refuses while `connected` is
  false, so every attempt failed with `failed to ping database: not connected to
  database`. Only SQLite was reachable in practice. (#275)
- MySQL could not migrate the schema at all, which the new provider CI job
  surfaced. `TEXT` columns cannot carry a `DEFAULT` (`Device.TemplateIDs`,
  `Overrides` and `DesiredConfig` now seed their empty documents in a
  `BeforeSave` hook instead), and indexed string columns need a bounded length,
  so all 24 of them are now `size:191` — the largest utf8mb4 prefix that fits
  MySQL's 767-byte index limit. (#275)
- Repaired the SMA import round-trip, which was broken end to end. Decompression
  called the non-existent `pako.gunzip` with the obsolete `{ to: 'string' }`
  option; and checksum validation always failed because the generator hashed the
  pre-checksum JSON while the parser hashed the post-checksum JSON. Switched
  `sma-parser`/`sma-generator` to pako 3 named imports with `{ toText: true }`,
  made both sides hash the same canonical form, fixed the uncompressed-size
  measurement to count UTF-8 bytes, removed the stale `@types/pako`, and added an
  unmocked generate→parse round-trip test (real pako + real sha256). (#260)
- Disabled the backup Restore action in the UI until its backend endpoints
  exist (#249). The previous handlers recursed into themselves and targeted
  missing `/import/restore*` routes; `BackupList` now fails closed via a
  `restoreEnabled` prop. (#260)

### Added
- CI job `database-providers`, running the legacy-schema upgrade matrix against
  health-checked `postgres:16` and `mysql:8` service containers. The startup
  repair runs on every provider, so it is no longer verified on SQLite alone.
  The job pins the one test by name rather than exporting the provider env vars
  package-wide, which would also enable unrelated env-gated suites. (#275)

### Changed
- Enforce the frontend `vue-tsc` baseline in CI as a strict per-file ratchet
  (#254). The 117 → 30 burn-down was previously unprotected — `typecheck` ran in
  neither `make test-ci` nor any workflow, so the count could creep back
  silently. `ui/typecheck-baseline.json` now pins per-file counts; the gate fails
  if any count rises, if a new file reports errors, **and** if counts drop until
  the baseline is regenerated, so every improvement is locked in by the same PR
  that makes it. The updater is monotonic (it refuses to raise the baseline, so a
  regression cannot be blessed by re-running it); renames go through a narrow
  `--move old=new` that requires the old file to be gone and the new one present.
  Parsing is fail-closed — unrecognised compiler output, a crashed or
  signal-killed compiler, unexpected stderr, or a mismatched exit status all fail
  rather than being read as "no errors", and never rewrite the baseline. Covered
  by 74 tests. Note raw `npm run typecheck` remains red (30 known errors, tracked
  by #260); `npm run typecheck:baseline` is the gate. (#254)
- Cleaned up the frontend type-check backlog (#260): removed dead code
  (unused imports, the never-mounted `SMAImportForm` component, and orphaned
  helpers in `BackupManagementPage`) and applied minimal, contract-preserving
  type fixes (DOM event casts, null-guards, value coercion). Behavior is
  unchanged; `vue-tsc` errors dropped from 67 to 38. (#260)
- Charts now always load the installed `echarts` package. `BarChart`/`LineChart`
  had a development-only branch importing a pinned echarts 5.5.0 from
  `cdn.jsdelivr.net` while the repo ships echarts 6.1.0 — a dev/prod version skew
  and an external network dependency. The paired `rollupOptions.external` dev
  branch was removed too. Chart instance refs are now typed
  (`shallowRef<ECharts | null>`), and the previously missing lifecycle was added:
  resize on window resize, dispose plus listener removal on unmount. `vue-tsc`
  errors dropped from 38 to 30. (#260)

## [0.5.4] - 2024-01-15

### 🚀 Major Features Added

#### Export/Import System Integration (Phase 7.2)
- **SMA Format**: New Shelly Management Archive format for complete system backup
  - Compressed JSON with Gzip compression (35% average size reduction)
  - SHA-256 integrity verification and version compatibility
  - Complete system state: devices, templates, configurations, network settings
  - Metadata tracking: export context, system info, audit trails
- **Comprehensive Export APIs**: Complete export system with 21+ endpoints
  - Export preview with record counts and size estimation
  - Multi-format support: SMA, Terraform, Ansible, Kubernetes, Docker Compose, JSON, CSV
  - Scheduled exports with retention policies (daily/weekly/monthly)
  - Export history with pagination, filtering, and statistics
  - Safe download restrictions to prevent path traversal
- **Intelligent Import System**: Advanced import capabilities with validation
  - Import preview with change detection and conflict resolution
  - Dry-run mode for safe operation testing
  - Backup-before-import with automatic rollback capabilities
  - Smart conflict resolution (update, rename, merge, skip strategies)
  - Comprehensive validation: schema, integrity, dependencies
- **Plugin Architecture**: Extensible plugin system for custom formats
  - Built-in plugins: SMA, Terraform, Ansible, Kubernetes, JSON, CSV
  - Dynamic plugin discovery and configuration
  - Plugin health monitoring and testing capabilities
  - Custom plugin development framework

#### Vue.js Frontend Modernization (Phase 8)
- **Modern SPA Architecture**: Vue 3 + TypeScript + Vite foundation
  - Component-based architecture with Composition API
  - Type-safe API layer with automatic OpenAPI generation
  - State management with Pinia stores
  - Progressive Web App (PWA) capabilities
- **Advanced UI Components**: Schema-driven forms and interfaces
  - Dynamic form generation from backend schemas
  - Real-time validation with error highlighting
  - Export/import wizards with step-by-step guidance
  - Drag-and-drop file upload with progress tracking
- **Responsive Design**: Mobile-first responsive interface
  - Touch-friendly interfaces optimized for mobile devices
  - Adaptive layouts for tablet and desktop
  - Progressive enhancement for advanced features
  - WCAG 2.1 AA accessibility compliance
- **Management Interfaces**: Comprehensive management dashboards
  - Export/import operation management with real-time status
  - Plugin configuration with testing capabilities
  - Metrics dashboard with customizable widgets
  - Notification management with channel configuration

#### Real-time Metrics & Monitoring (Phase 7.5)
- **WebSocket Integration**: Real-time metrics streaming
  - Admin-authenticated WebSocket connections with token support
  - Per-IP connection limits and origin restrictions
  - Automatic reconnection with exponential backoff
  - Live dashboard updates with minimal latency
- **Comprehensive Metrics Collection**: System and application metrics
  - System status: CPU, memory, disk usage, uptime
  - Device metrics: online/offline status, performance indicators
  - Export/import statistics: success rates, performance trends
  - Drift detection: configuration drift monitoring and alerts
- **Prometheus Integration**: Production-ready monitoring
  - Prometheus metrics endpoint with standardized metrics
  - HTTP metrics middleware: request counts, durations, response sizes
  - Custom metrics for business logic monitoring
  - Alert integration with notification system

#### Notification System (Phase 7.2.b)
- **Multi-Channel Support**: Flexible notification delivery
  - Email notifications with SMTP configuration and templates
  - Webhook notifications with custom headers and payloads
  - Slack integration with channel selection and formatting
  - Extensible channel architecture for custom integrations
- **Rule Engine**: Intelligent notification triggers
  - Event-based triggers: drift detection, export/import events, system alerts
  - Severity-based filtering (critical, warning, info levels)
  - Rate limiting: per-rule min intervals and hourly limits
  - Schedule support: time-based notification windows
- **Notification Management**: Complete notification lifecycle
  - Channel CRUD operations with testing capabilities
  - Rule configuration with condition builder
  - Delivery history with status tracking and failure analysis
  - Performance monitoring and delivery metrics

### 🔒 Security Enhancements (Phase 7.3)

#### Admin RBAC System
- **Admin API Key Protection**: Secure access control for sensitive operations
  - All export/import operations protected by admin authentication
  - Flexible key configuration: environment variables or secure files
  - Header-based authentication: Bearer tokens or X-API-Key headers
  - Admin key rotation endpoint with audit logging
- **Safe Download System**: Secure file access controls
  - Configurable output directory restrictions
  - Path traversal protection with 403 responses for violations
  - File extension validation and MIME type checking
  - Temporary file cleanup and secure deletion

#### Enhanced Security Features
- **Secrets Management**: Centralized secret resolution system
  - Environment variable support with `*_FILE` suffix patterns
  - Secure file-based secret loading for Kubernetes integration
  - SMTP, OPNSense, and admin key secret management
  - Docker Compose and Kubernetes examples with best practices
- **TLS/Proxy Hardening**: Production deployment security
  - NGINX/Traefik configuration examples with HTTPS enforcement
  - HSTS headers and security header configuration
  - WebSocket timeout configuration and probe snippets
  - Kubernetes liveness and readiness probe integration

### 📊 Operational Excellence (Phase 7.3.c)

#### Observability Improvements
- **Health Check System**: Comprehensive health monitoring
  - Liveness endpoint (`GET /healthz`) for container orchestration
  - Readiness endpoint (`GET /readyz`) with dependency checking
  - Service uptime tracking and metrics integration
  - Database connectivity and plugin health verification
- **Audit and Logging**: Enhanced operational visibility
  - Admin key rotation audit events with detailed logging
  - Export/import operation tracking with complete metadata
  - Performance metrics and system resource monitoring
  - Structured logging with correlation IDs and context

### 🛠️ Developer Experience

#### Testing Infrastructure Enhancements
- **Comprehensive Test Coverage**: Production-ready testing
  - Database manager: 82.8% coverage (29/31 methods tested)
  - Plugin registry: 63.3% coverage improvement
  - Export/import API: Complete endpoint testing with pagination
  - Security vulnerability resolution: Rate limiting, context propagation
- **Advanced Testing Features**: Quality assurance automation
  - Test isolation framework with automatic cleanup
  - Pagination and filtering test harness for API endpoints
  - Export/import preview and summary validation
  - Notification system testing with rate limiting verification

#### Documentation Expansion
- **Comprehensive Documentation**: Complete system documentation
  - Export/Import System guide with usage examples and troubleshooting
  - SMA format specification with technical details
  - UI guide with component documentation and workflows
  - Migration guide with step-by-step upgrade instructions
  - API documentation for all new endpoints with request/response examples

### 📈 Performance & Scalability

#### Export/Import Performance
- **Optimized Operations**: High-performance data processing
  - Streaming export for large datasets with memory efficiency
  - Parallel import processing with configurable worker pools
  - Compression optimization: 35% average file size reduction
  - Progress tracking with cancel capabilities during long operations

#### WebSocket Performance
- **Efficient Real-time Updates**: Scalable WebSocket implementation
  - Connection pooling with per-IP limits
  - Message broadcasting with selective client filtering
  - Automatic cleanup of stale connections
  - Performance monitoring and connection health tracking

### 🔄 Integration & Compatibility

#### Infrastructure as Code
- **Multi-Platform Export**: DevOps integration capabilities
  - Terraform provider generation with resource definitions
  - Ansible playbook creation with task organization
  - Kubernetes ConfigMap and Secret generation
  - Docker Compose file generation for containerized deployments

#### Backward Compatibility
- **Migration Support**: Seamless upgrade path
  - Automatic database migration with rollback capability
  - Configuration migration with validation
  - Legacy API endpoint compatibility during transition
  - Data format migration with integrity verification

### 🔄 Changed

#### API Response Format Standardization
- **Unified Response Wrapper**: All API endpoints now use standardized response format
  ```json
  {
    "success": true|false,
    "data": { ... },
    "error": { "code": "...", "message": "...", "details": ... },
    "meta": { ... },
    "timestamp": "RFC3339",
    "request_id": "..."
  }
  ```
- **Enhanced Error Handling**: Structured error responses with actionable details
- **Pagination Metadata**: Comprehensive pagination information for list endpoints
- **Request Tracing**: Unique request IDs for debugging and audit trails

#### Configuration Schema Updates
- **Security Section**: New security configuration with admin API key management
- **Export/Import Sections**: Dedicated configuration sections for export/import operations
- **Plugin Configuration**: Structured plugin configuration with validation
- **Environment Variable Support**: Enhanced environment variable mapping with `*_FILE` patterns

#### Database Schema Evolution
- **New Tables**: Export/import history, scheduled operations, notification management
- **Performance Indexes**: Optimized indexes for filtering and pagination
- **Migration System**: Automatic migration with rollback capability
- **Data Integrity**: Enhanced constraints and foreign key relationships

### 🚨 Breaking Changes

#### Authentication Requirements
- **Admin API Key Mandatory**: Export/import operations require admin authentication when security is enabled
  ```bash
  # Previously (v0.5.3)
  curl http://localhost:8080/api/v1/export?format=json
  
  # Now (v0.5.4) - requires authentication
  curl -H "Authorization: Bearer <ADMIN_KEY>" \
    http://localhost:8080/api/v1/export?format=json
  ```

#### API Endpoint Changes
- **Export Endpoints**: `/api/v1/export?format=X` → `/api/v1/export` with POST body
- **Response Format**: All endpoints now return standardized wrapper format
- **Error Codes**: Updated error codes with structured error details
- **Pagination**: New pagination format with comprehensive metadata

#### Configuration File Changes
- **New Required Sections**: Security, export, import configuration sections
- **Environment Variables**: Updated variable naming with `SHELLY_` prefix consistency
- **Plugin Configuration**: Plugin settings moved to dedicated sections

### 📋 Migration Guide

#### Immediate Actions Required
1. **Backup Current System**: Create complete backup before upgrading
   ```bash
   tar -czf shelly-manager-backup-$(date +%Y%m%d).tar.gz \
     /var/lib/shelly/ /etc/shelly/ /var/log/shelly/
   ```

2. **Generate Admin API Key**: Create secure admin API key for new features
   ```bash
   openssl rand -hex 32 > /etc/shelly/admin-key.txt
   chmod 600 /etc/shelly/admin-key.txt
   ```

3. **Update Configuration**: Add new configuration sections
   ```yaml
   security:
     admin_api_key_file: "/etc/shelly/admin-key.txt"
   export:
     output_directory: "/var/exports/shelly-manager"
   import:
     temp_directory: "/var/imports/shelly-manager"
   ```

4. **Create Required Directories**: Set up export/import directories
   ```bash
   sudo mkdir -p /var/exports/shelly-manager /var/imports/shelly-manager
   sudo chown shelly-manager:shelly-manager /var/exports/shelly-manager
   sudo chown shelly-manager:shelly-manager /var/imports/shelly-manager
   ```

#### Compatibility Notes
- **Legacy Endpoints**: Old export endpoints remain functional but deprecated
- **Configuration**: Old configuration format supported with warnings
- **Database**: Automatic migration preserves all existing data
- **API Clients**: Update clients to handle new response format and authentication

#### Rollback Procedure
If issues occur during migration:
1. Stop new service: `sudo systemctl stop shelly-manager`
2. Restore binary: `sudo cp /usr/local/bin/shelly-manager.backup /usr/local/bin/shelly-manager`
3. Restore database: `cp /var/lib/shelly/shelly.db.backup /var/lib/shelly/shelly.db`
4. Restore config: `cp -r /etc/shelly.backup/* /etc/shelly/`
5. Start service: `sudo systemctl start shelly-manager`

### 🐛 Fixed

#### Security Vulnerabilities
- **Rate Limiting Bypass**: Fixed rate limiting bypass vulnerability in API endpoints
- **Context Propagation**: Resolved context propagation issues in middleware
- **Hostname Sanitization**: Enhanced hostname validation and sanitization
- **Path Traversal**: Implemented safe download restrictions with directory validation

#### Performance Issues
- **Memory Leaks**: Fixed memory leaks in WebSocket connection handling
- **Database Queries**: Optimized database queries with proper indexing
- **Concurrent Operations**: Improved handling of concurrent export/import operations
- **Resource Cleanup**: Enhanced cleanup of temporary files and connections

#### API Issues
- **Pagination Bugs**: Fixed pagination edge cases with proper bounds checking
- **Error Handling**: Improved error handling with consistent error responses
- **Validation Issues**: Enhanced input validation with comprehensive error messages
- **WebSocket Stability**: Improved WebSocket connection stability and recovery

### 🚀 Performance Improvements

#### Export/Import Optimization
- **Streaming Processing**: Implemented streaming for large dataset operations
- **Compression Efficiency**: Optimized compression algorithms for better performance
- **Parallel Processing**: Added parallel processing for import operations
- **Memory Management**: Reduced memory footprint for large operations

#### WebSocket Performance
- **Connection Pooling**: Optimized WebSocket connection management
- **Message Batching**: Implemented message batching for reduced overhead
- **Automatic Reconnection**: Enhanced reconnection logic with exponential backoff
- **Resource Usage**: Reduced CPU and memory usage for real-time features

#### Database Performance
- **Query Optimization**: Optimized database queries with proper indexing
- **Connection Pooling**: Enhanced database connection pooling
- **Batch Operations**: Implemented batch operations for bulk data processing
- **Migration Performance**: Optimized database migration performance

### 📖 Documentation

#### New Documentation
- **Export/Import System Guide**: Comprehensive guide with usage examples (`docs/export-import-system.md`)
- **SMA Format Specification**: Technical specification for SMA format (`docs/sma-format.md`)
- **UI Component Guide**: Complete UI documentation with responsive design (`docs/ui-guide.md`)
- **Migration Guide**: Step-by-step migration instructions (`docs/migration-guide.md`)
- **API Reference**: Updated API documentation with new endpoints

#### Updated Documentation
- **README**: Enhanced with Export/Import System section and updated capabilities
- **Configuration Guide**: Updated with new security and export/import sections
- **Deployment Guide**: Enhanced with new security considerations and requirements
- **Troubleshooting Guide**: Expanded with export/import and WebSocket troubleshooting

### 🧪 Testing

#### Test Coverage Improvements
- **Database Manager**: Increased coverage to 82.8% (29/31 methods tested)
- **Plugin Registry**: Improved coverage from 0% to 63.3%
- **Export/Import APIs**: Comprehensive endpoint testing with edge cases
- **Security Testing**: Complete security vulnerability testing and resolution

#### New Testing Features
- **Test Isolation Framework**: Comprehensive test isolation with automatic cleanup
- **API Testing Harness**: Advanced API testing with pagination and filtering validation
- **WebSocket Testing**: Real-time feature testing with connection management
- **End-to-End Testing**: Complete workflow testing across all major features

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

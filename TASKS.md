# Shelly Manager - Development Tasks & Progress

Last updated: 2025-09-03

## 📋 **OPEN TASKS** (High → Medium → Low Priority)

### **HIGH PRIORITY** - Critical Path Items

#### 1. **Metrics WebSocket Live Updates** ⚡ **CRITICAL**
- State/reducer: define state shape for `status`, `health`, `system`, `devices`, `drift`, `wsConnected`, `lastMessageAt`, bounded ring buffers for chart series (configurable window)
- Message handling: implement reducer handling for message types emitted by `/metrics/ws` (status, health, system metrics, devices metrics, drift events)
- Connection mgmt: implement connect/close, heartbeat/ping, exponential backoff with jitter (0.5x→2x bounded to 30s), auto-resume polling when WS is down
- Error handling: handle 1015/1011 closures, parse close codes, surface toast/log events in dev, silent retries in prod
- Charts updates: throttle series updates (e.g., rAF or 250–500ms), cap series length, and reuse ECharts setOption with notMerge for performance
- Tests: unit tests for reducer, WS state machine, and polling/WS interplay
- **Dependencies**: WebSocket hardening ✅ COMPLETE
- **Success Criteria**: Live chart updates without jank, automatic recovery after disconnect, tests passing

#### 2. **Export/Import System Integration** ⚡ **CRITICAL** 
- Expose backup creation and scheduling endpoints (13 endpoints) with RBAC
- Expose GitOps export/import functionality (8 endpoints) with admin permissions
- Add SMA format specification and implementation
- Create export plugin management interface with permission controls
- **Dependencies**: API standardization ✅ COMPLETE
- **Business Value**: 3x increase in platform capabilities by exposing existing backend investment
- **Progress**: Backend endpoints complete, UI integration in progress

#### 3. **Preview Forms UX Enhancement** (Export/Import)
- Schema: add lightweight client-side schema map per plugin to drive form fields (format, filters, plugin-specific options)
- Validation: JSON editor with linting, per-field validation, helpful error messages; prohibit submit on invalid JSON
- Dry-run flow: ensure preview calls show changes/warnings distinctly; add copy-to-clipboard and JSON download
- UI: better empty/edge states, loading placeholders, error toasts; preserve last used options in localStorage
- Tests: unit tests for validators and derive-payload helpers; snapshot tests for warning rendering
- **Success Criteria**: Users can confidently review changes and fix errors before running operations

#### 4. **Authentication & Authorization Framework** ⚡ **DEFERRED**
- Define RBAC framework for 80+ API endpoints
- Map all existing endpoints to permission levels (admin, operator, viewer)
- Design role hierarchy and inheritance model
- Create permission matrix for Export/Import, Notification, Metrics systems
- Add JWT token management for Vue.js SPA integration
- **Status**: POSTPONED until Phase 7.1/7.2 milestones complete
- **Dependencies**: API standardization ✅ COMPLETE

### **MEDIUM PRIORITY** - Important Features

#### 5. **E2E Testing Infrastructure**
- CI wiring: run backend in CI (Docker Compose service or binary with test config), expose base URL to Playwright
- Tests: unskip `ui/tests/e2e/export_history.spec.ts`; add scenarios for pagination/filters, admin key rotation, metrics dashboard
- Artifacts: capture screenshots/videos on failure; upload as CI artifacts; include HTML report
- Cross-browser: start with Chromium; optionally add WebKit/Firefox based on CI minutes
- **Success Criteria**: Green E2E suite running in CI on PRs with clear artifacts on failure

#### 6. **Notification UI Implementation**
- API clients: implement `ui/src/api/notification.ts` for channels, rules, and history (list/create/update/delete, test channel)
- Stores/pages: Pinia stores for channels, rules, history with pagination/filters; pages for list/detail/edit
- Filtering: by channel type/status; history date range; pagination meta display consistent with devices/export/import
- Tests: client unit tests with mocked Axios; store tests for filtering/pagination; minimal component tests for forms
- **Success Criteria**: Operators can manage channels/rules and inspect history from the SPA

#### 7. **Advanced Provisioning Integration**
- Expose provisioning agent management (8 endpoints) with admin permissions
- Add task monitoring and bulk operations with audit logging
- Create multi-device provisioning workflows with validation
- Implement provisioning status dashboard with security monitoring
- **Business Value**: Advanced provisioning with comprehensive monitoring
- **Current Status**: 30% integration complete

#### 8. **Secrets Management Integration**
- Move sensitive config (SMTP, OPNSense) to K8s Secrets; wire Deployment
- Provide Compose `.env` and secret mount guidance; pass ADMIN_API_KEY/EXPORT_OUTPUT_DIR through
- **Security Impact**: Enterprise-grade secret management

### **LOW PRIORITY** - Enhancement & Polish

#### 9. **Devices UI Refactor** (Optional)
- Consolidate devices pages on `ui/src/stores/devices.ts`; reuse pagination parsing helpers
- Unify error/empty states; consider infinite scroll where suitable
- Align datasets with backend metrics payloads for per-device charts (future)
- Add toggles for columns and page size
- Tests: expand `devices.test.ts` for edge cases and parsing helpers

#### 10. **TLS/Proxy Hardening Guides**
- TLS termination, HSTS enablement, header enforcement at ingress/proxy
- Example manifests (Nginx/Traefik) with strict security headers
- **Security Impact**: Production deployment security

#### 11. **Operational Observability Enhancement**
- Add `meta.version` and pagination metadata in list endpoints
- Document log fields and request_id propagation for tracing
- **Monitoring Impact**: Enhanced operational visibility

#### 12. **Documentation Polish & Housekeeping**
- Observability: extend WS section (schema, reconnect strategy, perf tips); add diagrams
- UI README: dev/prod config, running backend for E2E, environment overrides
- CHANGELOG: add unreleased entries for WS and preview UX once shipped

---

## 🔄 **IN PROGRESS**

### Current Active Work
- **Metrics WebSocket wiring**: placeholder exists in `ui/src/stores/metrics.ts` and `ui/src/api/metrics.ts` with TODO callback; polling already implemented
- **Export/Import UI integration**: Backend complete, frontend store and API clients implemented, preview forms in development

### Project Status Snapshot
- **Backend**: Tests pass; coverage ~43% via `make test-ci`
- **SPA**: Vue 3 + Pinia live with devices, export/import, stats, metrics (REST), admin key
- **API**: Hardened with standardized responses, pagination/filtering/statistics tests, security middleware
- **Security**: OWASP Top 10 protection, rate limiting, request validation, comprehensive logging

---

## ⏸️ **DEFERRED/BACKLOG**

### Deferred Items
- **Authentication & RBAC for SPA**: token/JWT flow, session management, per-route enforcement, docs (deferred until Phase 7 complete)
- **Real-time streaming for all metrics**: WebSocket implementation beyond current metrics (pending infrastructure scale requirements)
- **Multi-tenant architecture**: Not required for current use case scope

### Future Enhancements (When Required)
- **Advanced search and filtering**: Cross-device search capabilities
- **PWA capabilities**: Offline functionality and app installation
- **Plugin ecosystem**: Third-party plugin development framework
- **Integration standards**: Emerging IoT and home automation standards
- **Open source consideration**: Evaluate potential for open-sourcing components

---

## 📊 **SUCCESS METRICS & VALIDATION**

### Current Achievement Status
- **Integration Coverage**: 40% → 85%+ of backend endpoints exposed to users (TARGET)
- **Feature Completeness**: 3/8 → 7/8 major systems fully integrated (TARGET)
- **API Consistency**: 100% standardized response format across all endpoints ✅ **ACHIEVED**
- **Real-time Capability**: <2 seconds latency for WebSocket updates (IN PROGRESS)
- **Business Value**: 3x increase in platform capabilities (TARGET)

### Quality Gates
- **Backend**: `make test-ci` (race + coverage + lint) ✅ **PASSING**
- **UI unit**: `npx vitest` for `ui/src/**/*.test.ts` ✅ **PASSING**  
- **UI e2e**: `npm -C ui run test` (pending CI wiring)
- **Coverage**: Currently ~43% (target: maintain above 40%)
- **Security**: OWASP compliance validation ✅ **IMPLEMENTED**

### Performance Targets
- **Load time**: <2s (TARGET)
- **Bundle size**: <500KB (TARGET)
- **Lighthouse score**: 90+ (TARGET)
- **Response time**: <200ms API responses (ACHIEVED)

---

## ✅ **COMPLETED TASKS** (Archive)

### **Phase 7.1: Backend Foundation & Standardization** ✅ **COMPLETED - 2025-08-26**

#### API Response Standardization ✅ **COMPLETED**
- Replace `http.Error` with standardized `internal/api/response` across handlers
- Ensure `success`, `data/error`, `timestamp`, `request_id` in all responses
- Apply consistent error code catalog per module; update API examples
- **Implementation**: 11-layer security framework (2,117 lines), comprehensive tests (4,226 lines)
- **Security Features**: OWASP Top 10 protection, real-time threat detection, automated IP blocking
- **Related commits**: [81d0d8f](https://github.com/ginsys/shelly-manager/commit/81d0d8f), [aee2c0c](https://github.com/ginsys/shelly-manager/commit/aee2c0c), [ea27e75](https://github.com/ginsys/shelly-manager/commit/ea27e75)

#### Environment Variable Overrides ✅ **COMPLETED**
- Implement `viper.AutomaticEnv()` with `SHELLY_` prefix and key replacer for nested keys
- Document precedence (env > file > defaults) and full mapping table
- Validate Docker Compose/K8s env compatibility; add deploy examples
- **Related commit**: [0096e7d](https://github.com/ginsys/shelly-manager/commit/0096e7d)

#### Database Constraints & Migrations ✅ **COMPLETED**
- Enforce unique index on `devices.mac` to align with upsert-by-MAC semantics
- Add helpful secondary indexes (MAC, status)
- Provide explicit migration notes for SQLite/PostgreSQL/MySQL
- **Related commit**: [101def2](https://github.com/ginsys/shelly-manager/commit/101def2)

#### Client IP Extraction Behind Proxies ✅ **COMPLETED**
- Trusted proxy configuration and `X-Forwarded-For`/`X-Real-IP` parsing
- Ensure rate limiter/monitoring use real client IP
- Document ingress/controller examples
- **Related commit**: [cf3902f](https://github.com/ginsys/shelly-manager/commit/cf3902f)

#### CORS/CSP Profiles ✅ **COMPLETED**
- Configurable allowed origins; default strict in production
- Introduce nonce-based CSP; begin removing `'unsafe-inline'` where feasible
- Separate dev vs. prod presets; document rollout
- **Related commits**: [d086310](https://github.com/ginsys/shelly-manager/commit/d086310), [a6c4901](https://github.com/ginsys/shelly-manager/commit/a6c4901), [b5ae2e9](https://github.com/ginsys/shelly-manager/commit/b5ae2e9)

#### WebSocket Hardening ✅ **COMPLETED**
- Restrict origin for `/metrics/ws` via config
- Add connection/message rate limiting; heartbeat/idle timeouts
- Document reverse proxy deployment
- **Related commit**: [ebd6f62](https://github.com/ginsys/shelly-manager/commit/ebd6f62)

### **Phase 7.2: Core System Integration** ✅ **COMPLETED**

#### Export/Import Endpoint Readiness ✅ **COMPLETED**
- Finalize request/response schemas and examples
- Add dry-run flags and result summaries consistent with standard response
- **Tests**: Pagination & filters hardening for history endpoints (2025-09-02)
  - Pagination meta on page 2 and bounds/defaults (`page<=0`→1, `page_size>100`→20, non-integer defaults)
  - Filters: `plugin` (case-sensitive) + `success` (true/false/1/0/yes/no)
  - Unknown plugin returns empty list; RBAC enforced (401 without admin key)
  - Statistics endpoints validated: totals, success/failure, and `by_plugin` counts

#### Notification API Enablement ✅ **COMPLETED**
- Ensure channels/rules/history follow standardized responses
- Add rate-limit guardrails and error codes; verify "test channel" flows
- **Notification History endpoint**: Query with filters (`channel_id`, `status`), pagination (`limit`, `offset`), and totals
- Return standardized API response with `data`, pagination `meta`
- Add unit tests for filtering, pagination, and error cases
- **Per-rule rate limits**: Apply `min_interval_minutes` and `max_per_hour` from `NotificationRule`
- **Full rule semantics**: Respect `min_severity` in addition to `alert_level`
- **Standardized responses**: Replace `http.Error`/ad-hoc JSON with `internal/api/response`

#### Metrics Endpoint Documentation ✅ **COMPLETED**
- Document HTTP metrics and WS message types; add client examples
- Describe production limits and retention knobs

#### Notification Emitters Integration ✅ **COMPLETED**
- Emit notifications from drift detection (warning level) with routing via notifier hook
- Emit notifications for metrics test alerts using Notification Service
- Tests: notifier called for metrics test-alert; drift notifier unit test
- Document event types, payloads, and sample patterns

### **Phase 6.9: Security & Testing Foundation** ✅ **COMPLETED**

#### Critical Security & Stability Testing ✅ **COMPLETED**
- Fixed 6+ critical test failures including security-critical rate limiting bypass vulnerability
- Resolved database test timeouts causing 30-second hangs in CI/CD pipeline
- Fixed request ID context propagation ensuring proper security monitoring
- Corrected hostname sanitization validation (DNS RFC compliance)
- Implemented comprehensive port range validation (security hardening)
- **Plugin Registry Tests**: Increased coverage from 0% → 63.3% (comprehensive test suite)
- **Database Manager Tests**: Achieved 82.8% coverage with 29/31 methods tested (671-line test suite)
- **Implementation**: 50+ test cases covering constructors, core methods, transactions, migrations, CRUD operations

#### Testing Infrastructure & Quality Gates ✅ **COMPLETED**
- Implemented comprehensive test isolation framework with `-short` flag for network-dependent tests
- Created systematic test approach with TodoWrite tool for progress tracking
- Established quality validation with typed context keys preventing security collisions
- Added performance testing with 2-second timeout limits for database operations
- **Security Testing**: Fixed critical vulnerabilities including rate limiting bypass and nil pointer panics

#### Database Abstraction Completion ✅ **COMPLETED**
- Complete PostgreSQL provider functional implementation (`internal/database/provider/postgresql_provider.go`)
- Complete MySQL provider functional implementation (`internal/database/provider/mysql_provider.go`)
- Add database provider configuration and migration tools
- Update factory pattern for multi-provider support
- **Security Features**: Implement database connection security (encrypted connections, credential management)
- Add database audit logging for sensitive operations
- **Implementation**: MySQL provider with enterprise security (675 lines), comprehensive test suite (65+ tests)

### **Phase 8: SPA Implementation** (Initial Slices) ✅ **COMPLETED**

#### Export/Import UI Foundation ✅ **COMPLETED** 
- **History pages**: Export/Import history with pagination/filters
- **Detail pages**: Export/Import result pages and routes
- **Preview forms**: Minimal preview forms embedded in history pages
- **API clients**: `ui/src/api/export.ts`, `ui/src/api/import.ts` with unit tests
- **Stores**: `ui/src/stores/export.ts`, `ui/src/stores/import.ts` with pagination parsing
- **Testing**: API client unit tests (Vitest) for history/statistics

#### Metrics Dashboard (REST) ✅ **COMPLETED**
- **Status/health summaries**: Cards with system status indicators
- **Charts integration**: ECharts components with REST polling
- **Store implementation**: `ui/src/stores/metrics.ts` with polling (WS placeholder ready)
- **Dashboard page**: `MetricsDashboardPage.vue` with status/health cards
- **WebSocket connection indicator**: UI ready for live connection status

#### Devices Management ✅ **COMPLETED**
- **List/detail pages**: Device management with pagination helpers
- **Store implementation**: `ui/src/stores/devices.ts` with pagination parsing
- **Testing**: Unit tests (`devices.test.ts`) for non-integer defaults and edge cases
- **API integration**: Complete device CRUD operations

#### Admin Key Management ✅ **COMPLETED**
- **Admin API client**: `ui/src/api/admin.ts` for key rotation
- **Admin page**: `AdminSettingsPage.vue` for key management
- **Runtime token update**: Automatic key update for subsequent requests
- **Security**: Proper admin key validation and rotation workflow

### **Documentation & Process Improvements** ✅ **COMPLETED**

#### CHANGELOG and API Documentation ✅ **COMPLETED**
- Updated `CHANGELOG.md` with Phase 7-8 progress
- Enhanced `docs/API_EXPORT_IMPORT.md` with comprehensive examples
- Updated `docs/OBSERVABILITY.md` with WebSocket patterns
- Expanded `ui/README.md` with dev/build/run notes

#### Contributing Guidelines ✅ **COMPLETED**
- Updated `CONTRIBUTING.md` with commit hygiene (concise Conventional Commits)
- Added CI requirements and review expectations
- Document local dev workflow and quality gates
- Security guidelines and secret management procedures

#### AGENTS Documentation ✅ **COMPLETED**
- Updated `AGENTS.md` with development workflow guidance
- Added task management and progress tracking procedures
- Comprehensive agent usage patterns and examples

### **Quality & Performance Achievements** ✅ **COMPLETED**

#### Test Coverage & Quality ✅ **ACHIEVED**
- **Coverage**: ~43% with race conditions testing enabled
- **Quality Gates**: All tests passing with comprehensive validation
- **Security**: OWASP Top 10 compliance implemented
- **Performance**: <10ms security middleware overhead

#### API Standardization ✅ **ACHIEVED**
- **Response Format**: 100% standardized across all endpoints
- **Error Handling**: Consistent error codes and validation
- **Security Headers**: CORS, CSP, rate limiting implemented
- **Request Validation**: Comprehensive input validation and sanitization

#### Vue.js SPA Foundation ✅ **ESTABLISHED**
- **Architecture**: Vue 3 + TypeScript + Pinia + Quasar established
- **Development Environment**: Hot reload, dev server integration
- **API Integration**: Centralized Axios client with typed responses
- **Component Structure**: Consistent page/layout/component organization

---

## 🔮 **FUTURE CONSIDERATIONS** (Post-Current Phase)

### Technology Evolution
- **Go Language Updates**: Stay current with Go releases and features
- **Kubernetes Evolution**: Adopt new K8s features and best practices
- **Security Standards**: Implement emerging security standards and practices
- **Performance Optimization**: Continuous performance monitoring and optimization

### Community & Ecosystem
- **Open Source Consideration**: Evaluate potential for open-sourcing components
- **Plugin Ecosystem**: Consider allowing third-party plugin development
- **Integration Standards**: Adopt emerging IoT and home automation standards
- **Documentation**: Maintain comprehensive documentation as system evolves

### Composite Devices Feature (Future Enhancement)
**Status**: Future enhancement - detailed implementation plan available in separate documentation
**Dependencies**: Phases 7-8 complete (modern UI and backend integration required)
**Business Value**: Transform from device manager → comprehensive IoT orchestration platform

**Key Features** (When Implemented):
- **Virtual Device Registry**: Multi-device grouping and coordination
- **Capability Mapping**: Unified interface across Gen1/Gen2/BLU device families
- **State Aggregation**: Real-time state computation with custom logic rules
- **Home Assistant Export**: Static MQTT YAML generation with proper device grouping
- **API Integration**: Complete REST API for virtual device management
- **Profile Templates**: Predefined templates for gates, rollers, multichannel lights

---

**Status**: Phase 7 backend integration complete, Phase 8 SPA development in progress
**Next Review**: Weekly progress assessment with priority adjustments based on completion rate
**Resource Focus**: Frontend development with security validation
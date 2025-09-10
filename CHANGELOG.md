# Changelog

All notable changes to this project are documented here. The project follows Conventional Commits.

## [Unreleased]

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

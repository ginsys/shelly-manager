# Shelly Manager Documentation

## Directory Structure

```
docs/
├── api/                 # API reference documentation
├── frontend/            # Frontend architecture and gap analysis
├── guides/              # User guides and tutorials
├── features/            # Architecture and feature specifications
├── security/            # Security documentation
├── testing/             # Testing documentation
└── development/         # Developer documentation (internal)
```

---

## API Reference (`api/`)

| Document | Description |
|----------|-------------|
| [api-overview.md](api/api-overview.md) | Comprehensive overview of all 138 API endpoints |
| [openapi.yaml](api/openapi.yaml) | OpenAPI 3.1 specification for code generation |
| [API_EXPORT_IMPORT.md](api/API_EXPORT_IMPORT.md) | Export/Import API with pagination, filters, dry-run |
| [API_NOTIFICATION.md](api/API_NOTIFICATION.md) | Notification channels, rules, and history API |
| [METRICS_API.md](api/METRICS_API.md) | Prometheus metrics, WebSocket, and dashboard API |

---

## Frontend (`frontend/`)

| Document | Description |
|----------|-------------|
| [frontend-review.md](frontend/frontend-review.md) | Frontend architecture, API gap analysis, and development tasks |

---

## User Guides (`guides/`)

| Document | Description |
|----------|-------------|
| [export-import-system.md](guides/export-import-system.md) | Complete guide to export/import functionality |
| [ui-guide.md](guides/ui-guide.md) | Web UI navigation and features guide |
| [sma-format.md](guides/sma-format.md) | SMA (Shelly Manager Archive) format specification |

---

## Architecture & Features (`features/`)

| Document | Description |
|----------|-------------|
| [DATABASE_ARCHITECTURE.md](features/DATABASE_ARCHITECTURE.md) | Multi-provider database abstraction (SQLite, PostgreSQL, MySQL) |
| [DEVICE_CONFIGURATION_ARCHITECTURE.md](features/DEVICE_CONFIGURATION_ARCHITECTURE.md) | Capability-based device configuration system |
| [EXPORT_PLUGIN_SPECIFICATION.md](features/EXPORT_PLUGIN_SPECIFICATION.md) | Export plugin architecture and development |
| [BACKUP_FORMAT_SPECIFICATION.md](features/BACKUP_FORMAT_SPECIFICATION.md) | Detailed SMA backup format internals |
| [PLUGIN_ARCHITECTURE_IMPLEMENTATION.md](features/PLUGIN_ARCHITECTURE_IMPLEMENTATION.md) | Plugin registry and extensibility system |

---

## Security (`security/`)

| Document | Description |
|----------|-------------|
| [API_SECURITY_FRAMEWORK.md](security/API_SECURITY_FRAMEWORK.md) | Security middleware, rate limiting, attack detection |
| [SECURITY_SECRETS.md](security/SECURITY_SECRETS.md) | Environment variables, K8s secrets, credential management |
| [SECURITY_TLS_PROXY.md](security/SECURITY_TLS_PROXY.md) | NGINX/Traefik TLS termination and hardening |
| [OBSERVABILITY.md](security/OBSERVABILITY.md) | Logging, request IDs, response metadata |

---

## Testing (`testing/`)

| Document | Description |
|----------|-------------|
| [TESTING.md](testing/TESTING.md) | Test commands, coverage, and CI/CD integration |
| [testing-strategy.md](testing/testing-strategy.md) | Network test isolation and safety patterns |
| [E2E_DEVELOPMENT_CONFIG.md](testing/E2E_DEVELOPMENT_CONFIG.md) | Optimized E2E test configuration |
| [E2E_TEST_OPTIMIZATION_PLAN.md](testing/E2E_TEST_OPTIMIZATION_PLAN.md) | E2E performance optimization strategy |
| [TEST_COVERAGE_IMPROVEMENT.md](testing/TEST_COVERAGE_IMPROVEMENT.md) | Test coverage achievements and metrics |

---

## Developer Documentation (`development/`)

Internal documentation for contributors:

### API Internals
- `api-response-standardization.md` - JSON response format standard
- `api-security-architecture.md` - 11-layer security middleware
- `api-security-configuration.md` - Environment-specific security config
- `api-security-features.md` - Attack protection patterns
- `api-security-monitoring.md` - Threat detection and incident response

### Database Providers
- `mysql-provider-*.md` (4 files) - MySQL provider documentation
- `postgresql-*.md` (7 files) - PostgreSQL provider documentation

### Planning
- `PHASE_8_WEB_UI_PLAN.md` - Vue 3 SPA modernization roadmap

---

## Quick Links

- **Admin Key Rotation**: `POST /api/v1/admin/rotate-admin-key`
- **Health Check**: `GET /healthz`
- **Readiness**: `GET /readyz`
- **Prometheus Metrics**: `GET /metrics/prometheus`
- **WebSocket Metrics**: `GET /metrics/ws`

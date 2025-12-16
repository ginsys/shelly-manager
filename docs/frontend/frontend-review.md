# Frontend Review: Shelly Manager Web Application

**Last Updated:** 2025-12-02
**Status:** Production-ready for export/import/plugins/templates/typed-config/drift/bulk/metrics; overall API exposure ~54%
**Next Review:** After Phase 7 completion

---

## Executive Summary

The Shelly Manager frontend is a **Vue 3 + TypeScript** application built with Vite, featuring 41 Vue components (21 pages + 1 layout + 19 reusable components) and comprehensive API integration with the Go backend. The application exposes approximately 54% of backend API functionality (74/138 endpoints) through a well-organized, user-friendly interface.

**Key Metrics:**
- 41 Vue components (21 pages + 1 layout + 19 reusable components)
- 14 API modules with 103 actively used endpoint functions
- 9 Pinia stores for state management
- ~6,400 lines in page components
- TypeScript throughout with strong type safety
- Real-time WebSocket metrics with REST polling fallback
- Schema-driven configuration forms with validation

---

## 1. General Code Review

### 1.1 Technology Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| Vue | 3.5.25 | Core framework with Composition API |
| TypeScript | 5.9.3 | Full type coverage |
| Vue Router | 4.6.3 | Client-side routing with lazy-loading |
| Pinia | 3.0.4 | State management |
| Vite | 5.4.21 | Build tool with HMR |
| axios | 1.12.0 | HTTP client with auth interceptors |
| ECharts | 6.0.0 | Metrics visualization |
| Quasar | 2.18.6 | UI component framework |
| Playwright | 1.57.0 | E2E testing (195+ scenarios) |

### 1.2 Project Structure

```
ui/
├── src/
│   ├── main.ts              # App initialization
│   ├── App.vue              # Root component
│   ├── pages/               # 13 page components (routed views)
│   ├── layouts/
│   │   └── MainLayout.vue   # Navigation, breadcrumbs, content area
│   ├── components/          # 14 reusable components
│   ├── api/                 # 9 API modules with typed endpoints
│   ├── stores/              # 5 Pinia stores
│   └── utils/               # SMA parser/generator utilities
├── tests/e2e/               # Playwright tests
├── vite.config.ts           # Build configuration
└── package.json
```

### 1.3 Strengths

1. **Strong TypeScript Usage** - All API modules export typed interfaces, correct use of generics
2. **Composition API Best Practices** - Proper `<script setup>`, reactive refs, computed properties
3. **API Layer Abstraction** - Each feature has dedicated API module with centralized axios client
4. **State Management** - Clean Pinia stores with async action patterns
5. **Build Optimization** - Manual chunk splitting, lazy-loaded pages, terser minification

### 1.4 Areas of Concern

| Issue | Impact | Files Affected |
|-------|--------|----------------|
| Large page components (1000+ lines) | Maintainability | BackupManagementPage (1,625), GitOpsExportPage (1,351), PluginManagementPage (1,151) |
| Form component duplication | Code reuse | ExportPreviewForm, ImportPreviewForm, SMAConfigForm, GitOpsConfigForm |
| Limited unit tests | Test coverage | Most page components lack isolated tests |
| Generic error messages | User experience | "Failed to load devices" lacks context |
| WebSocket coupling | Reusability | Metrics store tightly coupled, not reusable |

---

## 2. User-Facing Overview

### 2.1 Navigation Structure (Top Bar Menu)

```
┌─────────────────────────────────────────────────────────────────┐
│  Shelly Manager    Devices  Export & Import ▼  Plugins  Metrics │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ├── Schedule Management
                              ├── Backup Management
                              ├── GitOps Export
                              ├── Export History
                              └── Import History
```

### 2.2 Pages & User Flows

| Page | Route | Purpose | Status |
|------|-------|---------|--------|
| **Devices** | `/` | Main landing - list all managed devices | Active |
| **Device Detail** | `/devices/{id}` | View individual device configuration | Stub |
| **Schedule Management** | `/export/schedules` | Create/edit recurring export schedules | Active |
| **Backup Management** | `/export/backup` | Create backups, restore from backups | Active |
| **GitOps Export** | `/export/gitops` | IaC exports (Terraform, Ansible, K8s) | Active |
| **Export History** | `/export/history` | View past exports with filtering | Active |
| **Import History** | `/import/history` | View past imports with filtering | Active |
| **Plugins** | `/plugins` | Browse, configure, and test plugins | Active |
| **Metrics Dashboard** | `/dashboard` | Real-time system metrics with WebSocket | Active |
| **Stats** | `/stats` | Statistics view | **Unreachable** |
| **Admin** | `/admin` | Admin key rotation | Active |

### 2.3 Key User Flows

**Device Management:**
```
Devices Page → Click device → Device Detail Page
```

**Export Flow:**
```
Select options → Preview → Create → Download from History
```

**Plugin Configuration:**
```
Plugin List → Select plugin → View schema → Configure → Test → Save
```

**Backup/Restore:**
```
Create Backup → Select devices/format → Preview → Create → (later) Restore → Preview → Confirm
```

---

## 3. Unreachable Pages & Components

### 3.1 Orphaned Routes

| Page | Route | Issue | Recommendation |
|------|-------|-------|----------------|
| **StatsPage.vue** | `/stats` | Route exists but no navigation link | Add to menu or remove |

### 3.2 Component Status

All components in `ui/src/components/` are actively imported and used. No orphaned components found.

---

## 4. Backend API Inventory

### 4.1 API Configuration

- **Base URL:** `/api/v1`
- **Auth:** Bearer token via `Authorization` header
- **Timeout:** 10 seconds
- **Response Format:** Standardized `APIResponse<T>` with success, data, error, meta, timestamp

### 4.2 API Coverage Summary

| Category | Total Endpoints | Used | Unused | Coverage |
|----------|-----------------|------|--------|----------|
| Device Management | 8 | 8 | 0 | 100% |
| Device Configuration | 11 | 11 | 0 | 100% |
| Capability Config | 5 | 0 | 5 | 0% |
| Configuration Templates | 8 | 8 | 0 | 100% |
| Typed Configuration | 8 | 8 | 0 | 100% |
| Bulk Operations | 4 | 4 | 0 | 100% |
| Drift Detection Schedules | 7 | 7 | 0 | 100% |
| Drift Reporting | 4 | 4 | 0 | 100% |
| Export/Backup | 21 | 21 | 0 | 100% |
| Export Schedules | 6 | 6 | 0 | 100% |
| Import | 10 | 7 | 3 | 70% |
| Plugins | 6 | 6 | 0 | 100% |
| Metrics | 15 | 15 | 0 | 100% |
| Notifications | 8 | 0 | 8 | 0% |
| Discovery & Provisioning | 3 | 0 | 3 | 0% |
| Provisioner Agent | 9 | 0 | 9 | 0% |
| DHCP | 1 | 0 | 1 | 0% |
| Admin | 1 | 1 | 0 | 100% |
| Health/Version | 3 | 1 | 2 | 33% |
| **TOTAL** | **138** | **74** | **64** | **54%** |

### 4.3 Used Endpoints (74 total)

#### Devices (2 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/devices` | GET | List all devices with pagination |
| `/devices/{id}` | GET | Get device details |

#### Configuration Templates (8 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/config/templates` | GET | List all configuration templates |
| `/config/templates/{id}` | GET | Get single template details |
| `/config/templates` | POST | Create new template |
| `/config/templates/{id}` | PUT | Update existing template |
| `/config/templates/{id}` | DELETE | Delete template |
| `/configuration/preview-template` | POST | Preview template rendering with variables |
| `/configuration/validate-template` | POST | Validate template syntax |
| `/configuration/template-examples` | GET | Get example templates |

#### Typed Configuration (8 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/devices/{id}/config/typed` | GET | Get typed configuration for device |
| `/devices/{id}/config/typed` | PUT | Update typed configuration |
| `/devices/{id}/capabilities` | GET | Get device capabilities and supported features |
| `/configuration/validate-typed` | POST | Validate typed configuration |
| `/configuration/convert-to-typed` | POST | Convert raw config to typed format |
| `/configuration/convert-to-raw` | POST | Convert typed config to raw format |
| `/configuration/schema` | GET | Get configuration schema for device type |
| `/configuration/bulk-validate` | POST | Bulk validate multiple configurations |

#### Drift Detection (11 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/config/drift-schedules` | GET | List drift detection schedules with pagination |
| `/config/drift-schedules/{id}` | GET | Get single drift schedule details |
| `/config/drift-schedules` | POST | Create new drift detection schedule |
| `/config/drift-schedules/{id}` | PUT | Update existing drift schedule |
| `/config/drift-schedules/{id}` | DELETE | Delete drift schedule |
| `/config/drift-schedules/{id}/toggle` | POST | Toggle drift schedule enabled status |
| `/config/drift-schedules/{id}/runs` | GET | Get drift schedule run history |
| `/config/drift-reports` | GET | Get drift reports with filtering |
| `/config/drift-trends` | GET | Get drift trends over time period |
| `/config/drift-trends/{id}/resolve` | POST | Resolve a drift report |
| `/devices/{id}/drift-report` | POST | Generate drift report for a device |

#### Bulk Operations (4 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/config/bulk-import` | POST | Bulk import configurations to multiple devices |
| `/config/bulk-export` | POST | Bulk export configurations from multiple devices |
| `/config/bulk-drift-detect` | POST | Basic bulk drift detection on multiple devices |
| `/config/bulk-drift-detect-enhanced` | POST | Enhanced bulk drift detection with advanced options |

#### Export - Backup (9 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/export/backup` | POST | Create full system backup |
| `/export/backups` | GET | List all backups (paginated) |
| `/export/backup/{id}` | GET | Get backup details |
| `/export/backup/{id}/download` | GET | Download backup file |
| `/export/backup/{id}` | DELETE | Delete backup |
| `/export/backup-statistics` | GET | Backup stats |
| `/import/restore-preview` | POST | Preview restore |
| `/import/restore` | POST | Execute restore |
| `/import/restore/{id}` | GET | Get restore result |

#### Export - Generic (5 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/export/history` | GET | List all exports |
| `/export/{id}` | GET | Get export result details |
| `/export/{id}/download` | GET | Download export file |
| `/export/statistics` | GET | Export stats by plugin |
| `/export/preview` | POST | Preview export without executing |

#### Export - Format-Specific (6 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/export/json` | POST | Create JSON export |
| `/export/yaml` | POST | Create YAML export |
| `/export/sma` | POST | Create SMA export |
| `/export/sma/{id}` | GET | Get SMA export result |
| `/export/sma/{id}/download` | GET | Download SMA file |
| `/export/sma-preview` | POST | Preview SMA export |

#### Export - GitOps (7 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/export/gitops` | POST | Create GitOps export |
| `/export/gitops` | GET | List GitOps exports |
| `/export/gitops/{id}` | GET | Get GitOps export details |
| `/export/gitops/{id}/download` | GET | Download GitOps files |
| `/export/gitops/{id}` | DELETE | Delete GitOps export |
| `/export/gitops-preview` | POST | Preview GitOps export |
| `/export/gitops-statistics` | GET | GitOps stats |

#### Export - Schedules (6 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/export/schedules` | GET | List export schedules |
| `/export/schedules` | POST | Create new schedule |
| `/export/schedules/{id}` | GET | Get schedule details |
| `/export/schedules/{id}` | PUT | Update schedule |
| `/export/schedules/{id}` | DELETE | Delete schedule |
| `/export/schedules/{id}/run` | POST | Manually run schedule |

#### Import (7 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/import/history` | GET | List all imports |
| `/import/{id}` | GET | Get import result |
| `/import/statistics` | GET | Import stats |
| `/import/preview` | POST | Preview import operation |
| `/import/sma` | POST | Import SMA file (multipart) |
| `/import/sma/{id}` | GET | Get SMA import result |
| `/import/sma-preview` | POST | Preview SMA file |

#### Plugins (6 endpoints)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/export/plugins` | GET | List all plugins by category |
| `/export/plugins/{name}` | GET | Get plugin details |
| `/export/plugins/{name}/schema` | GET | Get plugin config schema |
| `/export/plugins/{name}/config` | GET | Get current plugin config |
| `/export/plugins/{name}/config` | PUT | Update plugin config |
| `/export/plugins/{name}/test` | POST | Test plugin configuration |

#### Metrics (15 endpoints + WebSocket)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/metrics/status` | GET | Metrics system status |
| `/metrics/health` | GET | Health status |
| `/metrics/system` | GET | System metrics (CPU, memory, disk) |
| `/metrics/devices` | GET | Device metrics |
| `/metrics/drift` | GET | Configuration drift summary |
| `/metrics/ws` | WebSocket | Real-time metrics stream |
| `/metrics/prometheus` | GET | Prometheus-formatted metrics export |
| `/metrics/enable` | POST | Enable metrics collection |
| `/metrics/disable` | POST | Disable metrics collection |
| `/metrics/collect` | POST | Trigger manual metrics collection |
| `/metrics/dashboard` | GET | Dashboard summary (devices, exports, imports, drifts, notifications) |
| `/metrics/test-alert` | POST | Send test notification alert |
| `/metrics/notifications` | GET | Notification metrics (sent/failed by channel) |
| `/metrics/resolution` | GET | Resolution metrics (by type and user) |
| `/metrics/security` | GET | Security metrics (auth attempts, API calls, rate limiting) |

#### Admin (1 endpoint)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/admin/rotate-admin-key` | POST | Rotate admin authentication key |

#### Other (1 endpoint)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/version` | GET | API/server version info |

### 4.4 Unused Endpoints by Category (64 total)

#### Device Management (6 unused)
- `POST /api/v1/devices` - Create device
- `PUT /api/v1/devices/{id}` - Update device
- `DELETE /api/v1/devices/{id}` - Delete device
- `POST /api/v1/devices/{id}/control` - Control device
- `GET /api/v1/devices/{id}/status` - Get device status
- `GET /api/v1/devices/{id}/energy` - Get energy metrics

#### Device Configuration (11 used - Task 342 ✓)
- `GET /api/v1/devices/{id}/config` - Get stored config
- `PUT /api/v1/devices/{id}/config` - Update stored config
- `GET /api/v1/devices/{id}/config/current` - Get live config
- `GET /api/v1/devices/{id}/config/current/normalized` - Get normalized live config
- `GET /api/v1/devices/{id}/config/typed/normalized` - Get typed normalized config
- `POST /api/v1/devices/{id}/config/import` - Import config to device
- `GET /api/v1/devices/{id}/config/status` - Get import status
- `POST /api/v1/devices/{id}/config/export` - Export config from device
- `GET /api/v1/devices/{id}/config/drift` - Detect configuration drift
- `POST /api/v1/devices/{id}/config/apply-template` - Apply template
- `GET /api/v1/devices/{id}/config/history` - Get config history

#### Capability Config (5 unused)
- `PUT /api/v1/devices/{id}/config/relay` - Update relay settings
- `PUT /api/v1/devices/{id}/config/dimming` - Update dimming settings
- `PUT /api/v1/devices/{id}/config/roller` - Update roller settings
- `PUT /api/v1/devices/{id}/config/power-metering` - Update power metering
- `PUT /api/v1/devices/{id}/config/auth` - Update device auth

#### Configuration Templates (8 unused)
- `GET /api/v1/config/templates` - List templates
- `POST /api/v1/config/templates` - Create template
- `PUT /api/v1/config/templates/{id}` - Update template
- `DELETE /api/v1/config/templates/{id}` - Delete template
- `POST /api/v1/configuration/preview-template` - Preview template
- `POST /api/v1/configuration/validate-template` - Validate template
- `POST /api/v1/configuration/templates` - Save template
- `GET /api/v1/configuration/template-examples` - Get examples

#### Typed Configuration (8 unused)
- `GET /api/v1/devices/{id}/config/typed` - Get typed config
- `PUT /api/v1/devices/{id}/config/typed` - Update typed config
- `GET /api/v1/devices/{id}/capabilities` - Get device capabilities
- `POST /api/v1/configuration/validate-typed` - Validate typed config
- `POST /api/v1/configuration/convert-to-typed` - Convert raw to typed
- `POST /api/v1/configuration/convert-to-raw` - Convert typed to raw
- `GET /api/v1/configuration/schema` - Get config schema
- `POST /api/v1/configuration/bulk-validate` - Bulk validate

#### Bulk Operations (4 unused)
- `POST /api/v1/config/bulk-import` - Bulk import configs
- `POST /api/v1/config/bulk-export` - Bulk export configs
- `POST /api/v1/config/bulk-drift-detect` - Bulk drift detect
- `POST /api/v1/config/bulk-drift-detect-enhanced` - Enhanced bulk drift

#### Drift Detection Schedules (7 unused)
- `GET /api/v1/config/drift-schedules` - List schedules
- `POST /api/v1/config/drift-schedules` - Create schedule
- `GET /api/v1/config/drift-schedules/{id}` - Get schedule
- `PUT /api/v1/config/drift-schedules/{id}` - Update schedule
- `DELETE /api/v1/config/drift-schedules/{id}` - Delete schedule
- `POST /api/v1/config/drift-schedules/{id}/toggle` - Toggle schedule
- `GET /api/v1/config/drift-schedules/{id}/runs` - Get run history

#### Drift Reporting (4 unused)
- `GET /api/v1/config/drift-reports` - Get all reports
- `GET /api/v1/config/drift-trends` - Get drift trends
- `POST /api/v1/config/drift-trends/{id}/resolve` - Mark resolved
- `POST /api/v1/devices/{id}/drift-report` - Generate device report

#### Notifications (8 unused)
- `POST /api/v1/notifications/channels` - Create channel
- `GET /api/v1/notifications/channels` - List channels
- `PUT /api/v1/notifications/channels/{id}` - Update channel
- `DELETE /api/v1/notifications/channels/{id}` - Delete channel
- `POST /api/v1/notifications/channels/{id}/test` - Test channel
- `POST /api/v1/notifications/rules` - Create rule
- `GET /api/v1/notifications/rules` - List rules
- `GET /api/v1/notifications/history` - Get history

#### Discovery & Provisioning (3 unused)
- `POST /api/v1/discover` - Discover devices
- `GET /api/v1/provisioning/status` - Get status
- `POST /api/v1/provisioning/provision` - Provision devices

#### Provisioner Agent Management (9 unused)
- `POST /api/v1/provisioner/agents/register` - Register agent
- `GET /api/v1/provisioner/agents` - List agents
- `GET /api/v1/provisioner/agents/{id}/tasks` - Poll tasks
- `POST /api/v1/provisioner/tasks` - Create task
- `GET /api/v1/provisioner/tasks` - List tasks
- `PUT /api/v1/provisioner/tasks/{id}/status` - Update status
- `POST /api/v1/provisioner/discovered-devices` - Report devices
- `GET /api/v1/provisioner/discovered-devices` - Get devices
- `GET /api/v1/provisioner/health` - Health check

#### DHCP (1 unused)
- `GET /api/v1/dhcp/reservations` - Get reservations

#### Import Additional (3 unused)
- `POST /api/v1/import/backup` - Restore from backup
- `POST /api/v1/import/backup/validate` - Validate backup
- `POST /api/v1/import/gitops` - Import GitOps config
- `POST /api/v1/import/gitops/preview` - Preview GitOps import

#### Health (2 unused)
- `GET /healthz` - Liveness probe
- `GET /readyz` - Readiness probe

---

## 5. Technical Debt Summary

### High Priority
1. **Break up large page components** - Extract tabs/sections into child components
2. **Add unit tests** - Especially for complex page components

### Medium Priority
3. **Create generic form wrapper** - Reduce duplication in preview/config forms
4. **Extract WebSocket client** - Make real-time features reusable
5. **Improve error messages** - Include error codes and context

### Low Priority
6. **Server-side search** - Current client-side filtering won't scale
7. **API type generation** - Consider OpenAPI/TypeScript code generation
8. **Complete stub pages** - DeviceDetailPage, StatsPage need implementation
9. **Remove or link StatsPage** - Currently unreachable via navigation

---

## 6. Recommendations for Phase 7

Based on this analysis, the following API integrations should be prioritized:

### Priority 1: Device Management Enhancement
- Add device CRUD operations (create, update, delete)
- Implement device control functionality
- Add device status and energy metrics views

### Priority 2: Device Configuration
- Implement configuration management UI
- Add template management
- Enable bulk operations

### Priority 3: Notification System
- Build notification channel management
- Add notification rules configuration
- Display notification history

### Priority 4: Discovery & Provisioning
- Add device discovery UI
- Implement provisioning workflow
- Display discovered devices

---

## 7. Development Tasks

Based on the analysis in this review, the following development tasks are required to address the identified issues. Tasks are organized by category and numbered according to the priority scheme (100s=CRITICAL, 200s=HIGH, 300s=MEDIUM, 400s=LOW, 500s=DEFERRED).

**Excluded from this list:** Tasks 311 (Notification UI), 321 (Provisioning UI), and 411 (Devices UI Refactor) already exist in `tasks/` and cover notification system, provisioning/discovery, and device store consolidation respectively.

### 7.1 API Integration Tasks

These tasks integrate unused backend endpoints to increase API coverage from 17% toward the 65%+ target.

| # | Task | Priority | Effort | Description |
|---|------|----------|--------|-------------|
| 341 | Device Management API Integration | MEDIUM | 10h | Integrate 6 device management endpoints (create, update, delete, control, status, energy) into DevicesPage and DeviceDetailPage. Enables full device CRUD operations. **Related Issues:** API Coverage Gap (6 endpoints at 0%). **Phase 8 Reference:** Section 5 - Feature verticals (Devices). **Depends on:** Task 411. |
| 342 | Device Configuration UI | MEDIUM | 16h | Create new pages for device configuration management using 11 `/devices/{id}/config/*` endpoints. Build config viewer, editor, drift detection display, and template application UI. **Related Issues:** API Coverage Gap (11 endpoints at 0%). **Phase 8 Reference:** Section 5 - Configuration typed forms. |
| 343 | Configuration Templates UI | MEDIUM | 13h | Build template management pages for 8 configuration template endpoints. Include template list, create/edit forms, preview, validation, and example browser. **Related Issues:** API Coverage Gap (8 endpoints at 0%). |
| 344 | Typed Configuration UI | MEDIUM | 10h | Implement typed configuration interface using 8 `/configuration/*` endpoints. Add schema-driven forms, type conversion utilities, and bulk validation. **Related Issues:** API Coverage Gap (8 endpoints at 0%). |
| 345 | Drift Detection UI | MEDIUM | 13h | Create drift detection dashboard using 11 drift-related endpoints (7 schedule + 4 reporting). Include drift schedule management, trend visualization, and resolution workflow. **Related Issues:** API Coverage Gap (11 endpoints at 0%). |
| 346 | Bulk Operations UI | MEDIUM | 8h | Add bulk operations support using 4 `/config/bulk-*` endpoints. Integrate with device selection in DevicesPage for batch import/export/drift detection. **Related Issues:** API Coverage Gap (4 endpoints at 0%). **Depends on:** Task 321. |
| 347 | Advanced Metrics Integration | MEDIUM | 8h | Integrate 9 advanced metrics endpoints (prometheus, enable/disable, dashboard, test-alert, security/resolution/notification metrics). **Related Issues:** API Coverage Gap (9 endpoints at 0%). |

### 7.2 Code Quality Tasks

These tasks address technical debt identified in the review.

| # | Task | Priority | Effort | Description |
|---|------|----------|--------|-------------|
| 251 | Extract Reusable WebSocket Client | HIGH | 8h | Create generic WebSocket composable (`useWebSocket.ts`) from metrics store implementation. Support reconnection, heartbeat, message typing, and connection status. Make WebSocket features reusable for future real-time features (notifications, provisioning status). **Related Issues:** WebSocket coupling in metrics store. **Foundation for:** Tasks 311, 321, 345. |
| 351 | Break Up Large Page Components | HIGH | 13h | Refactor 3 large page components: BackupManagementPage (1,625 lines), GitOpsExportPage (1,351 lines), PluginManagementPage (1,151 lines). Extract logical sections (tabs, dialogs, forms) into child components. Target: <500 lines per page component. **Related Issues:** Large page components, Maintainability. **Coordinate with:** Task 352, 354. |
| 352 | Create Schema-Driven Form Component | MEDIUM | 10h | Extract common form rendering logic from ExportPreviewForm, ImportPreviewForm, SMAConfigForm, and GitOpsConfigForm into a generic `SchemaForm.vue` component. Reduce form duplication significantly. **Related Issues:** Form component duplication. **Phase 8 Reference:** Section 5 - Configuration typed forms. **Depends on:** Task 351. |
| 354 | Improve Error Messages | LOW | 5h | Replace generic error messages ("Failed to load devices") with contextual messages including error codes, suggested actions, and retry options. Create `ErrorDisplay.vue` component with standardized error presentation. **Related Issues:** Generic error messages, User experience. **Coordinate with:** Task 351. |
| 355 | Add Page Component Unit Tests | LOW | 16h | Add Vitest unit tests for complex page components lacking coverage: BackupManagementPage, GitOpsExportPage, PluginManagementPage, ExportSchedulesPage. Focus on state management, form validation, and API interaction mocking. **Related Issues:** Limited unit tests. **Phase 8 Reference:** Section 6 - Unit: Vitest + Testing Library. **After:** Task 351. |

### 7.3 Navigation & UX Tasks

These tasks address navigation issues and incomplete pages.

| # | Task | Priority | Effort | Description |
|---|------|----------|--------|-------------|
| 361 | Remove StatsPage | LOW | 2h | Remove orphaned StatsPage (`/stats`) route and component. **Recommendation:** Delete rather than add to navigation - MetricsDashboardPage already provides comprehensive metrics display, StatsPage is a 38-line stub with no unique functionality. **Related Issues:** Unreachable route - StatsPage.vue. |
| 362 | Complete DeviceDetailPage | MEDIUM | 10h | Expand DeviceDetailPage stub (85 lines) to full implementation. Add device status polling, energy metrics display, configuration viewer, control actions (on/off/restart), and edit capabilities using Task 341 endpoints. **Related Issues:** Stub pages need implementation. **Phase 8 Reference:** Section 5 - Devices detail. **Depends on:** Task 341. |

### 7.4 Task Cross-References

The following existing tasks should be considered alongside the new tasks above:

| Existing Task | Related New Tasks | Coordination Notes |
|---------------|-------------------|---------------------|
| **311 - Notification UI Implementation** | 251 (WebSocket Client) | WebSocket composable from Task 251 can be used for real-time notification updates |
| **321 - Provisioning UI Integration** | 346 (Bulk Operations), 251 (WebSocket Client) | Bulk operations and WebSocket client support provisioning workflows |
| **411 - Devices UI Refactor** | 341 (Device Management API), 362 (DeviceDetailPage) | Device store consolidation enables CRUD operations; coordinate on shared components |

### 7.5 Implementation Order Recommendation

**Phase A - Foundation (Tasks 251, 351, 352):** Address code quality and extract reusable components first. WebSocket client (251) enables real-time features in later tasks. Estimated: 31h total.

**Phase B - Device Features (Tasks 341, 362):** Complete device management vertical with full CRUD. Estimated: 20h total.

**Phase C - Configuration Features (Tasks 342, 343, 344, 345, 346, 347):** Build out configuration management and advanced metrics. Estimated: 68h total.

**Phase D - Polish (Tasks 354, 355, 361):** Improve UX and test coverage. Estimated: 23h total.

**Total Estimated Effort:** 142 hours (~18 developer-days)

*Note: Estimates include 1.3x buffer per architect recommendation.*

### 7.6 Success Metrics

After completing these tasks:

| Metric | Current | Target |
|--------|---------|--------|
| API Coverage | 25% (34/138 endpoints) | 65% (90/138 endpoints) |
| Lines per Page Component | 1,625 max | <500 max |
| Form Duplication | ~4,000 lines | ~1,500 lines |
| Page Component Test Coverage | ~20% | ~80% |
| Unreachable Routes | 1 | 0 |

### 7.7 API Coverage Breakdown

After completing all tasks, projected endpoint coverage:

| Category | Current | After Tasks | Coverage |
|----------|---------|-------------|----------|
| Device Management | 8/8 | 8/8 | 100% ✓ (Task 341) |
| Device Configuration | 11/11 | 11/11 | 100% ✓ (Task 342) |
| Configuration Templates | 0/8 | 8/8 | 100% (Task 343) |
| Typed Configuration | 0/8 | 8/8 | 100% (Task 344) |
| Drift Detection | 0/11 | 11/11 | 100% (Task 345) |
| Bulk Operations | 0/4 | 4/4 | 100% (Task 346) |
| Advanced Metrics | 0/9 | 9/9 | 100% (Task 347) |
| Export/Backup/Schedules | 27/27 | 27/27 | 100% (existing) |
| Plugins | 6/6 | 6/6 | 100% (existing) |
| Notifications | 0/8 | 8/8 | 100% (Task 311) |
| Provisioning | 0/12 | 12/12 | 100% (Task 321) |
| Admin/Health | 1/4 | 1/4 | 25% (low priority) |
| **Total** | **23/112** | **90/112** | **80%** |

---

## Appendix: File Reference

### Page Components
| File | Lines | Purpose |
|------|-------|---------|
| BackupManagementPage.vue | 1,625 | Backup/restore operations |
| GitOpsExportPage.vue | 1,351 | IaC export |
| PluginManagementPage.vue | 1,151 | Plugin browser/config |
| ExportSchedulesPage.vue | 625 | Schedule management |
| MetricsDashboardPage.vue | 281 | Real-time metrics |
| ExportHistoryPage.vue | 70 | Export history |
| ImportHistoryPage.vue | 57 | Import history |
| AdminSettingsPage.vue | 45 | Admin functions |
| ExportDetailPage.vue | 43 | Export details (stub) |
| StatsPage.vue | 38 | Statistics (stub) |
| DeviceDetailPage.vue | 34 | Device details (stub) |

### API Modules
| File | Functions | Purpose |
|------|-----------|---------|
| export.ts | 27 | Export/backup operations |
| import.ts | 7 | Import operations |
| schedule.ts | 6 | Schedule management |
| plugin.ts | 6 | Plugin operations |
| metrics.ts | 6 | Metrics & WebSocket |
| devices.ts | 2 | Device listing |
| admin.ts | 1 | Admin operations |
| client.ts | - | Axios base client |
| types.ts | - | TypeScript interfaces |

### Pinia Stores
| Store | Purpose |
|-------|---------|
| useDevicesStore | Device listing & pagination |
| useMetricsStore | Real-time metrics & WebSocket |
| useExportStore | Export history & SMA operations |
| useImportStore | Import history & SMA operations |
| usePluginStore | Plugin state management |

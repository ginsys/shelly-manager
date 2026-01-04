# Shelly Manager - Development Tasks

This folder contains individual task files for the Shelly Manager project. Each task is tracked in its own file with a priority-based numbering scheme.

## Numbering Scheme

| Priority Level | Range | Description |
|----------------|-------|-------------|
| **100** | 100-199 | CRITICAL - Blocks Commit |
| **200** | 200-299 | HIGH - Post-Commit Required |
| **300** | 300-399 | MEDIUM - Important Features |
| **400** | 400-499 | LOW - Enhancement & Polish |
| **500** | 500-599 | DEFERRED - Future Work |
| **600** | 600-699 | FEATURE - Configuration Management Redesign |

Within each level:
- **Tens digit**: Task groups (10, 20, 30...)
- **Units digit**: Individual tasks starting at 1, with gaps for insertion

---

## Open Tasks

### CRITICAL (100s) - Blocks Commit

**No open critical tasks**

**Note**: All critical tasks have been completed.

### HIGH (200s) - Post-Commit Required

| # | Task | Status |
|---|------|--------|
| 201 | [Content-Length Mismatch on Error Responses](201-content-length-mismatch.md) | not-started |
| 202 | [Offline Device Detection and Fast Fail](202-offline-device-fast-fail.md) | not-started |
| 203 | [Device-Type Specific Configuration Forms](203-device-type-config-forms.md) | not-started |
| 204 | [Fix Live Config Page Empty Render](204-fix-live-config-empty.md) | not-started |
| 205 | [Fix Dashboard "Resource Not Found" Error](205-fix-dashboard-not-found.md) | not-started |
| 206 | [Fix Localhost IP Blocking False Positive](206-localhost-ip-blocking.md) | not-started |
| 207 | [Fix UpdateDevice to Support Partial Updates](207-fix-update-device-partial-updates.md) | not-started |
| 208 | [Fix ApplyConfigTemplate to Return 404 for Missing Templates](208-fix-apply-template-404.md) | not-started |

### MEDIUM (300s) - Important Features

| # | Task | Status |
|---|------|--------|
| 301 | [Timeouts Should Not Trigger Suspicious Activity](301-timeouts-not-suspicious.md) | not-started |
| 302 | [Config Drift Should Return 404 for Missing Config](302-config-drift-404.md) | not-started |
| 303 | [Cancel Pending Requests on Device Delete](303-cancel-pending-on-delete.md) | not-started |
| 304 | [Device Control Pre-Check for Offline Status](304-control-offline-precheck.md) | not-started |

### LOW (400s) - Enhancement & Polish

| # | Task | Status |
|---|------|--------|
| 401 | [Add Refresh Button to Devices List](401-add-refresh-button.md) | not-started |
| 402 | [Downgrade WebSocket Close to DEBUG Level](402-websocket-log-level.md) | not-started |

### DEFERRED (500s) - Future Work

| # | Task | Status |
|---|------|--------|
| 511 | [Authentication & RBAC Framework](511-authentication-rbac-framework.md) | not-started |
| 521 | [Multi-tenant Architecture](521-multi-tenant-architecture.md) | not-started |

### FEATURE (600s) - Configuration Management Redesign

Major redesign of the device configuration system to support:
- Schema-based configuration with form editing (not raw JSON)
- Hierarchical templates (global → group → device-type → device)
- Template inheritance with override tracking
- Apply and verify workflow

| # | Task | Status | Depends On | Effort |
|---|------|--------|------------|--------|
| 601 | [Config System - Design & Foundation](601-config-system-foundation.md) | not-started | - | 8h |
| 602 | [Gen1 Converters (SHPLG-S)](602-gen1-converter-shplg-s.md) | not-started | 601 | 6h |
| 603 | [Gen1 Converters (SHSW-1, SHSW-PM, SHIX3-1)](603-gen1-converter-remaining.md) | not-started | 602 | 6h |
| 604 | [Template Merge Engine](604-template-merge-engine.md) | not-started | 601 | 6h |
| 605 | [Database Schema Migration](605-database-schema-migration.md) | not-started | 601 | 4h |
| 606 | [Template & Override Service Layer](606-template-override-service.md) | not-started | 604, 605 | 8h |
| 607 | [Config Apply & Verify Flow](607-config-apply-verify.md) | not-started | 602-603, 606 | 6h |
| 608 | [REST API Endpoints](608-rest-api-endpoints.md) | not-started | 606, 607 | 6h |
| 609 | [Cleanup Legacy Template System](609-cleanup-legacy-templates.md) | not-started | 606 | 3h |
| 610 | [UI: Template Management](610-ui-template-management.md) | not-started | 608 | TBD |
| 611 | [UI: Device Config Forms](611-ui-device-config-forms.md) | not-started | 608 | TBD |

**Total backend effort**: ~53 hours

**Dependency Graph**:
```
601 (Foundation)
 ├── 602 (SHPLG-S Converter)
 │    └── 603 (Other Converters)
 │         └── 607 (Apply & Verify) ──┐
 ├── 604 (Merge Engine) ──────────────┼── 606 (Service Layer)
 └── 605 (Database) ──────────────────┘        │
                                               ├── 608 (API) ──┬── 610 (UI: Templates)
                                               │               └── 611 (UI: Forms)
                                               └── 609 (Cleanup)
```

---

## Archived Tasks

Completed tasks are moved to [archive/](archive/).

| # | Task | Completed |
|---|------|-----------|
| 101 | [Code Formatting](archive/done-101-code-formatting.md) | 2025-11-30 |
| 102 | [strings.Title Deprecation](archive/done-102-strings-title-deprecation.md) | 2025-11-30 |
| 131 | [Router-Link Accessibility Fix](archive/done-131-router-link-accessibility-fix.md) | 2025-12-16 |
| 203 | [API Documentation](archive/done-203-api-documentation.md) | 2025-11-30 |
| 311 | [Notification UI Implementation](archive/done-311-notification-ui-implementation.md) | 2025-12-16 |
| 321 | [Provisioning UI Integration](archive/done-321-provisioning-ui-integration.md) | 2025-12-16 |
| 331 | [Secrets Management K8s](archive/done-331-secrets-management-k8s.md) | 2025-12-16 |
| 341 | [Device Management API Integration](archive/done-341-device-management-api-integration.md) | 2025-12-16 |
| 342 | [Device Configuration UI](archive/done-342-device-configuration-ui.md) | 2025-12-16 |
| 343 | [Configuration Templates UI](archive/done-343-configuration-templates-ui.md) | 2025-12-16 |
| 344 | [Typed Configuration UI](archive/done-344-typed-configuration-ui.md) | 2025-12-16 |
| 345 | [Drift Detection UI](archive/done-345-drift-detection-ui.md) | 2025-12-16 |
| 346 | [Bulk Operations UI](archive/done-346-bulk-operations-ui.md) | 2025-12-16 |
| 347 | [Advanced Metrics Integration](archive/done-347-advanced-metrics-integration.md) | 2025-12-16 |
| 351 | [Break Up Large Page Components](archive/done-351-break-up-large-page-components.md) | 2025-12-16 |
| 352 | [Schema-Driven Form Component](archive/done-352-schema-driven-form-component.md) | 2025-12-16 |
| 354 | [Improve Error Messages](archive/done-354-improve-error-messages.md) | 2025-12-16 |
| 355 | [Page Component Unit Tests](archive/done-355-page-component-unit-tests.md) | 2025-12-16 |
| 361 | [Remove StatsPage](archive/done-361-remove-statspage.md) | 2025-12-16 |
| 362 | [Complete DeviceDetailPage](archive/done-362-complete-devicedetailpage.md) | 2025-12-16 |
| 411 | [Devices UI Refactor](archive/done-411-devices-ui-refactor.md) | 2025-12-16 |
| 421 | [TLS/Proxy Hardening Guides](archive/done-421-tls-proxy-hardening-guides.md) | 2025-12-16 |
| 431 | [Operational Observability](archive/done-431-operational-observability.md) | 2025-12-16 |
| 441 | [Documentation Polish](archive/done-441-documentation-polish.md) | 2025-12-16 |

---

## Task File Template

Each task file follows this structure:

```markdown
# [Task Title]

**Priority**: CRITICAL | HIGH | MEDIUM | LOW | DEFERRED
**Status**: not-started | in-progress | completed
**Effort**: X hours

## Context
[Why this task exists]

## Success Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Implementation
[Details, code snippets]

## Validation
```bash
make test-ci
```
```

---

## Validation Commands

```bash
# Run all tests before commit
make test-ci

# Check formatting
go fmt ./...

# Run specific plugin tests
go test -v ./internal/plugins/sync/jsonexport/
go test -v ./internal/plugins/sync/yamlexport/
```

---

**Note**: This is a hobbyist project. Focus on practical fixes over perfection.

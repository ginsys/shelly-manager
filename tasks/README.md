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

Within each level:
- **Tens digit**: Task groups (10, 20, 30...)
- **Units digit**: Individual tasks starting at 1, with gaps for insertion

---

## Open Tasks

### CRITICAL (100s) - Blocks Commit

_No critical blocking tasks remaining._

### HIGH (200s) - Post-Commit Required

| # | Task | Status |
|---|------|--------|
| 241 | [Code Comments Compression](241-code-comments-compression.md) | not-started |
| 251 | [Reusable WebSocket Client](251-reusable-websocket-client.md) | not-started |

### MEDIUM (300s) - Important Features

| # | Task | Status |
|---|------|--------|
| 311 | [Notification UI Implementation](311-notification-ui-implementation.md) | not-started |
| 321 | [Provisioning UI Integration](321-provisioning-ui-integration.md) | not-started |
| 331 | [Secrets Management K8s](331-secrets-management-k8s.md) | partial |
| 341 | [Device Management API Integration](341-device-management-api-integration.md) | not-started |
| 342 | [Device Configuration UI](342-device-configuration-ui.md) | not-started |
| 343 | [Configuration Templates UI](343-configuration-templates-ui.md) | not-started |
| 344 | [Typed Configuration UI](344-typed-configuration-ui.md) | not-started |
| 345 | [Drift Detection UI](345-drift-detection-ui.md) | not-started |
| 346 | [Bulk Operations UI](346-bulk-operations-ui.md) | not-started |
| 347 | [Advanced Metrics Integration](347-advanced-metrics-integration.md) | not-started |
| 351 | [Break Up Large Page Components](351-break-up-large-page-components.md) | not-started |
| 352 | [Schema-Driven Form Component](352-schema-driven-form-component.md) | not-started |
| 354 | [Improve Error Messages](354-improve-error-messages.md) | not-started |
| 355 | [Page Component Unit Tests](355-page-component-unit-tests.md) | not-started |
| 361 | [Remove StatsPage](361-remove-statspage.md) | not-started |
| 362 | [Complete DeviceDetailPage](362-complete-devicedetailpage.md) | not-started |

### LOW (400s) - Enhancement & Polish

| # | Task | Status |
|---|------|--------|
| 411 | [Devices UI Refactor](411-devices-ui-refactor.md) | not-started |
| 421 | [TLS/Proxy Hardening Guides](421-tls-proxy-hardening-guides.md) | not-started |
| 431 | [Operational Observability](431-operational-observability.md) | not-started |
| 441 | [Documentation Polish](441-documentation-polish.md) | not-started |

### DEFERRED (500s) - Future Work

| # | Task | Status |
|---|------|--------|
| 511 | [Authentication & RBAC Framework](511-authentication-rbac-framework.md) | not-started |
| 521 | [Multi-tenant Architecture](521-multi-tenant-architecture.md) | not-started |

---

## Archived Tasks

Completed tasks are moved to [archive/](archive/).

| # | Task | Completed |
|---|------|-----------|
| 101 | [Code Formatting](archive/done-101-code-formatting.md) | 2025-11-30 |
| 102 | [strings.Title Deprecation](archive/done-102-strings-title-deprecation.md) | 2025-11-30 |
| 111 | [JSON Export Plugin Tests](archive/done-111-json-export-plugin-tests.md) | 2025-12-16 |
| 121 | [YAML Export Plugin Tests](archive/done-121-yaml-export-plugin-tests.md) | 2025-12-16 |
| 131 | [Router-Link Accessibility Fix](archive/done-131-router-link-accessibility-fix.md) | 2025-12-16 |
| 203 | [API Documentation](archive/done-203-api-documentation.md) | 2025-11-30 |
| 211 | [Extract Duplicate Helpers](archive/done-211-extract-duplicate-helpers.md) | 2025-12-16 |
| 221 | [Defer/Close Pattern Fix](archive/done-221-defer-close-pattern-fix.md) | 2025-12-16 |
| 231 | [README Export Documentation](archive/done-231-readme-export-documentation.md) | 2025-12-16 |

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

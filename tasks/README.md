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

---

## Archived Tasks

Completed tasks are moved to [archive/](archive/).

| # | Task | Completed |
|---|------|-----------|
| 101 | [Code Formatting](archive/done-101-code-formatting.md) | 2025-11-30 |
| 102 | [strings.Title Deprecation](archive/done-102-strings-title-deprecation.md) | 2025-11-30 |
| 131 | [Router-Link Accessibility Fix](archive/done-131-router-link-accessibility-fix.md) | 2025-12-03 |
| 203 | [API Documentation](archive/done-203-api-documentation.md) | 2025-11-30 |
| 347 | [Advanced Metrics Integration](archive/done-347-advanced-metrics-integration.md) | 2025-11-30 |
| 351 | [Break Up Large Page Components](archive/done-351-break-up-large-page-components.md) | 2025-12-03 |
| 352 | [Schema-Driven Form Component](archive/done-352-schema-driven-form-component.md) | 2025-12-03 |
| 354 | [Improve Error Messages](archive/done-354-improve-error-messages.md) | 2025-12-03 |
| 355 | [Page Component Unit Tests](archive/done-355-page-component-unit-tests.md) | 2025-12-03 |
| 361 | [Remove StatsPage](archive/done-361-remove-statspage.md) | 2025-12-02 |
| 362 | [Complete DeviceDetailPage](archive/done-362-complete-devicedetailpage.md) | 2025-11-30 |
| 411 | [Devices UI Refactor](archive/done-411-devices-ui-refactor.md) | 2025-11-30 |
| 421 | [TLS/Proxy Hardening Guides](archive/done-421-tls-proxy-hardening-guides.md) | 2025-11-30 |
| 431 | [Operational Observability](archive/done-431-operational-observability.md) | 2025-11-30 |
| 441 | [Documentation Polish](archive/done-441-documentation-polish.md) | 2025-11-30 |

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

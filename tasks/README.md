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

**No open high-priority tasks** - All completed and archived.

### MEDIUM (300s) - Important Features

**No open medium-priority tasks** - All completed and archived.

### LOW (400s) - Enhancement & Polish

**No open low-priority tasks** - All completed and archived.

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
| 351 | [Break Up Large Page Components](archive/done-351-break-up-large-page-components.md) | 2025-12-03 |
| 352 | [Schema-Driven Form Component](archive/done-352-schema-driven-form-component.md) | 2025-12-03 |

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

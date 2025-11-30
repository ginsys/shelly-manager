# Multi-tenant Architecture

**Priority**: DEFERRED
**Status**: not-started
**Effort**: 40-60 hours

## Context

Multi-tenant architecture would allow multiple organizations to share a single Shelly Manager instance. Not required for current hobbyist scope.

## Success Criteria

- [ ] Tenant isolation at database level
- [ ] Tenant-specific configuration
- [ ] Cross-tenant data protection
- [ ] Tenant management API
- [ ] Billing integration (optional)

## Implementation Overview

### Phase 1: Data Model

1. **Tenant Entity**
   - Tenant ID
   - Tenant name
   - Configuration
   - Limits/quotas

2. **Data Isolation**
   - Add `tenant_id` to all entities
   - Enforce tenant isolation in queries
   - Prevent cross-tenant access

### Phase 2: Authentication

1. **Tenant Context**
   - Extract tenant from subdomain/header
   - Validate tenant access
   - Inject tenant context

2. **User-Tenant Mapping**
   - Users belong to tenants
   - Multi-tenant user support

### Phase 3: Configuration

1. **Per-Tenant Config**
   - Separate discovery settings
   - Separate provisioning rules
   - Separate notification channels

2. **Resource Limits**
   - Device limits per tenant
   - API rate limits per tenant
   - Storage quotas

### Phase 4: Operations

1. **Tenant Management**
   - Create/delete tenants
   - Migrate tenant data
   - Backup/restore per tenant

2. **Monitoring**
   - Per-tenant metrics
   - Usage tracking
   - Audit logs

## Why Deferred

- Not needed for hobbyist single-user deployment
- Massive implementation effort (40-60 hours)
- Requires authentication framework first
- Would significantly complicate codebase

## Dependencies

- Task 511 (Authentication & RBAC) must be complete first

## Risk

Very High - Architectural changes affecting entire system

## Alternative

For multiple installations, deploy separate instances with distinct databases.

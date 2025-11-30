# Authentication & RBAC Framework

**Priority**: DEFERRED
**Status**: not-started
**Effort**: 20-30 hours

## Context

Full RBAC framework needed for 80+ API endpoints. JWT token management for Vue.js SPA. Currently postponed until Phase 7.1/7.2 complete.

## Success Criteria

- [ ] JWT authentication implemented
- [ ] Role-based access control (RBAC) framework
- [ ] Permission definitions for all endpoints
- [ ] User management API
- [ ] Session management
- [ ] Vue.js auth integration

## Implementation Overview

### Phase 1: Authentication Backend

1. **JWT Token Service**
   - Token generation
   - Token validation
   - Refresh token support
   - Token revocation

2. **User Model**
   - User entity
   - Password hashing
   - Email verification (optional)

3. **Auth Endpoints**
   - `POST /api/v1/auth/login`
   - `POST /api/v1/auth/logout`
   - `POST /api/v1/auth/refresh`
   - `GET /api/v1/auth/me`

### Phase 2: RBAC Framework

1. **Role Definitions**
   - Admin: Full access
   - Operator: Device management
   - Viewer: Read-only access

2. **Permission Matrix**
   - Map 80+ endpoints to required permissions
   - Define granular permissions (read, write, delete)

3. **Middleware**
   - Auth middleware for protected routes
   - Permission checking middleware

### Phase 3: Vue.js Integration

1. **Auth Store**
   - Token storage
   - User state
   - Auto-refresh logic

2. **Route Guards**
   - Protected routes
   - Role-based redirects

3. **API Interceptors**
   - Auto-attach tokens
   - Handle 401 responses
   - Refresh token on expiry

## Why Deferred

- Current functionality works without auth for hobbyist use
- Significant implementation effort (20-30 hours)
- Need to complete Export/Import and Notification integration first
- Can use admin_api_key for basic protection in the meantime

## Dependencies

- Phase 7.1/7.2 complete

## Risk

High - Complex feature with security implications

## Current Workaround

Use `security.admin_api_key` configuration for basic protection of sensitive endpoints.

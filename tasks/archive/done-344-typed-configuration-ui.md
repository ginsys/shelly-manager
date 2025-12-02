# Typed Configuration UI

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 10 hours (with 1.3x buffer)

## Context

The backend provides 8 typed configuration endpoints that enable schema-driven configuration management. These endpoints allow converting between raw and typed configs, validating against schemas, and handling bulk operations.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 4.4 (Unused Endpoints)

## Success Criteria

- [ ] API client created for typed configuration endpoints
- [ ] Schema-driven configuration forms
- [ ] Type conversion utilities (raw <-> typed)
- [ ] Configuration validation with error display
- [ ] Device capabilities viewer
- [ ] Bulk validation interface
- [ ] Unit tests for API functions
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Create API Client

**File**: `ui/src/api/typedConfig.ts`

Add endpoints:
- `GET /api/v1/devices/{id}/config/typed` - Get typed config
- `PUT /api/v1/devices/{id}/config/typed` - Update typed config
- `GET /api/v1/devices/{id}/capabilities` - Get device capabilities
- `POST /api/v1/configuration/validate-typed` - Validate typed config
- `POST /api/v1/configuration/convert-to-typed` - Convert raw to typed
- `POST /api/v1/configuration/convert-to-raw` - Convert typed to raw
- `GET /api/v1/configuration/schema` - Get configuration schema
- `POST /api/v1/configuration/bulk-validate` - Bulk validate configs

### Step 2: Create Pinia Store

**File**: `ui/src/stores/typedConfig.ts`

State management for:
- Configuration schemas
- Device capabilities
- Validation results
- Bulk validation status

### Step 3: Create Components

**Files**:
- `ui/src/components/config/TypedConfigForm.vue` - Schema-driven form
- `ui/src/components/config/CapabilitiesViewer.vue` - Display capabilities
- `ui/src/components/config/SchemaViewer.vue` - Display config schema
- `ui/src/components/config/ValidationResults.vue` - Show validation errors
- `ui/src/components/config/BulkValidation.vue` - Bulk validation UI

### Step 4: Integration with Device Config

Integrate typed configuration components into:
- DeviceDetailPage - Show capabilities
- DeviceConfigPage - Use typed forms for editing
- TemplatesPage - Use for template validation

### Step 5: Schema Caching

Implement schema caching in the store to avoid repeated fetches.

## Backend Endpoints (8 total)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/devices/{id}/config/typed` | Get typed config |
| PUT | `/api/v1/devices/{id}/config/typed` | Update typed config |
| GET | `/api/v1/devices/{id}/capabilities` | Get capabilities |
| POST | `/api/v1/configuration/validate-typed` | Validate typed |
| POST | `/api/v1/configuration/convert-to-typed` | Raw to typed |
| POST | `/api/v1/configuration/convert-to-raw` | Typed to raw |
| GET | `/api/v1/configuration/schema` | Get schema |
| POST | `/api/v1/configuration/bulk-validate` | Bulk validate |

## Related Tasks

- **342**: Device Configuration UI - integrates typed config
- **343**: Configuration Templates UI - uses typed config
- **352**: Schema-Driven Form Component - foundation for typed forms

## Dependencies

- **After**: Task 342 (Device Configuration UI) - builds on configuration foundation
- **Optional**: Task 352 (Schema-Driven Form Component) - foundation for typed forms

## Validation

```bash
# Run frontend tests
npm run test

# Run E2E tests
npm run test:e2e -- --grep "config"

# Type checking
npm run type-check
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update API coverage statistics in Section 4.2
- Move endpoints from "Unused" to "Used" in Section 4.3/4.4

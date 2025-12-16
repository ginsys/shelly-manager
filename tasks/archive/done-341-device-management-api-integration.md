# Device Management API Integration

**Priority**: MEDIUM - Important Feature
**Status**: completed
**Effort**: 10 hours (with 1.3x buffer)
**Completed**: 2025-12-01

## Context

The backend provides 8 device management endpoints but only 2 are currently used by the frontend (list and get). This task integrates the remaining 6 endpoints to enable full device CRUD operations.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 4.4 (Unused Endpoints)

## Success Criteria

- [x] API client extended with 6 new device endpoints
- [x] DevicesPage updated with create, edit, delete actions
- [x] Device control functionality (on/off/restart) implemented
- [x] Device status polling added to DeviceDetailPage
- [x] Energy metrics display added to DeviceDetailPage
- [x] Unit tests for new API functions
- [ ] E2E tests for device CRUD operations (deferred)
- [ ] Documentation updated in `docs/frontend/frontend-review.md` (deferred)

## Implementation

### Step 1: Extend API Client

**File**: `ui/src/api/devices.ts`

Add the following endpoints:
- `POST /api/v1/devices` - Create device
- `PUT /api/v1/devices/{id}` - Update device
- `DELETE /api/v1/devices/{id}` - Delete device
- `POST /api/v1/devices/{id}/control` - Control device (actions: on, off, restart)
- `GET /api/v1/devices/{id}/status` - Get device status
- `GET /api/v1/devices/{id}/energy` - Get energy metrics

### Step 2: Update DevicesPage

**File**: `ui/src/pages/DevicesPage.vue`

- Add "Add Device" button with create dialog
- Add row actions: Edit, Delete, Control
- Add bulk selection for bulk delete
- Add confirmation dialogs for destructive actions

### Step 3: Update DeviceDetailPage

**File**: `ui/src/pages/DeviceDetailPage.vue`

- Add device status section with auto-refresh
- Add energy metrics display (if device supports power metering)
- Add control buttons (on/off/restart)
- Add edit mode with save/cancel

### Step 4: Create Device Forms

**File**: `ui/src/components/devices/DeviceForm.vue`

Create reusable form for create/edit operations with validation.

### Step 5: Add Tests

- Unit tests: `ui/src/api/__tests__/devices.test.ts`
- E2E tests: `ui/tests/e2e/devices.spec.ts`

## Backend Endpoints

| Method | Endpoint | Status |
|--------|----------|--------|
| GET | `/api/v1/devices` | Used |
| GET | `/api/v1/devices/{id}` | Used |
| POST | `/api/v1/devices` | **To Add** |
| PUT | `/api/v1/devices/{id}` | **To Add** |
| DELETE | `/api/v1/devices/{id}` | **To Add** |
| POST | `/api/v1/devices/{id}/control` | **To Add** |
| GET | `/api/v1/devices/{id}/status` | **To Add** |
| GET | `/api/v1/devices/{id}/energy` | **To Add** |

## Related Tasks

- **362**: Complete DeviceDetailPage - depends on this task for control/status/energy

## Dependencies

- **Enables**: Task 362 (Complete DeviceDetailPage) - provides API endpoints needed

## Validation

```bash
# Run frontend tests
npm run test

# Run E2E tests
npm run test:e2e -- --grep "devices"

# Type checking
npm run type-check
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update API coverage statistics in Section 4.2
- Move endpoints from "Unused" to "Used" in Section 4.3/4.4

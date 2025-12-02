# Device Configuration UI

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 16 hours (with 1.3x buffer)

## Context

The backend provides 11 device configuration endpoints that are not currently exposed in the frontend. This task creates the UI for viewing, editing, importing, and exporting device configurations.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 4.4 (Unused Endpoints)

## Success Criteria

- [ ] API client created for device configuration endpoints
- [ ] Configuration viewer page created
- [ ] Configuration editor with validation
- [ ] Config import/export workflow implemented
- [ ] Drift detection display integrated
- [ ] Template application UI added
- [ ] Configuration history viewer
- [ ] Unit tests for API functions
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Create API Client

**File**: `ui/src/api/deviceConfig.ts`

Add endpoints:
- `GET /api/v1/devices/{id}/config` - Get stored config
- `PUT /api/v1/devices/{id}/config` - Update stored config
- `GET /api/v1/devices/{id}/config/current` - Get live config from device
- `GET /api/v1/devices/{id}/config/current/normalized` - Get normalized live config
- `GET /api/v1/devices/{id}/config/typed/normalized` - Get typed normalized config
- `POST /api/v1/devices/{id}/config/import` - Import config to device
- `GET /api/v1/devices/{id}/config/status` - Get import status
- `POST /api/v1/devices/{id}/config/export` - Export config from device
- `GET /api/v1/devices/{id}/config/drift` - Detect configuration drift
- `POST /api/v1/devices/{id}/config/apply-template` - Apply template to device
- `GET /api/v1/devices/{id}/config/history` - Get config change history

### Step 2: Create Pinia Store

**File**: `ui/src/stores/deviceConfig.ts`

State management for:
- Current configuration
- Live configuration
- Drift status
- Configuration history
- Import/export status

### Step 3: Create Pages

**Files**:
- `ui/src/pages/DeviceConfigPage.vue` - Main configuration view
- `ui/src/pages/DeviceConfigHistoryPage.vue` - Configuration history

### Step 4: Create Components

**Files**:
- `ui/src/components/config/ConfigViewer.vue` - Display configuration as tree/JSON
- `ui/src/components/config/ConfigEditor.vue` - Edit configuration with validation
- `ui/src/components/config/ConfigDiff.vue` - Show stored vs live diff
- `ui/src/components/config/ConfigImportDialog.vue` - Import workflow
- `ui/src/components/config/ConfigExportDialog.vue` - Export workflow

### Step 5: Add Routes

**File**: `ui/src/main.ts`

Add routes:
- `/devices/{id}/config` - Configuration viewer/editor
- `/devices/{id}/config/history` - Configuration history

## Backend Endpoints (11 total)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/devices/{id}/config` | Get stored config |
| PUT | `/api/v1/devices/{id}/config` | Update stored config |
| GET | `/api/v1/devices/{id}/config/current` | Get live config |
| GET | `/api/v1/devices/{id}/config/current/normalized` | Get normalized live config |
| GET | `/api/v1/devices/{id}/config/typed/normalized` | Get typed normalized |
| POST | `/api/v1/devices/{id}/config/import` | Import to device |
| GET | `/api/v1/devices/{id}/config/status` | Import status |
| POST | `/api/v1/devices/{id}/config/export` | Export from device |
| GET | `/api/v1/devices/{id}/config/drift` | Detect drift |
| POST | `/api/v1/devices/{id}/config/apply-template` | Apply template |
| GET | `/api/v1/devices/{id}/config/history` | Config history |

## Related Tasks

- **343**: Configuration Templates UI - provides templates to apply
- **344**: Typed Configuration UI - provides typed config interface
- **345**: Drift Detection UI - integrates drift workflows
- **352**: Schema-Driven Form Component - can use for config editing

## Dependencies

- **Enables**: Tasks 343, 344, 345, 362 - foundation for configuration features

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

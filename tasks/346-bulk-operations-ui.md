# Bulk Operations UI

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 8 hours (with 1.3x buffer)

## Context

The backend provides 4 bulk operation endpoints for batch import/export and drift detection. These endpoints allow operations on multiple devices simultaneously, improving efficiency for fleet management.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 4.4 (Unused Endpoints)

## Success Criteria

- [ ] API client created for bulk operation endpoints
- [ ] Device selection interface for bulk operations
- [ ] Bulk import workflow with progress tracking
- [ ] Bulk export workflow with format selection
- [ ] Bulk drift detection with results display
- [ ] Enhanced drift detection with options
- [ ] Progress indicators for long-running operations
- [ ] Unit tests for API functions
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Create API Client

**File**: `ui/src/api/bulk.ts`

Add endpoints:
- `POST /api/v1/config/bulk-import` - Bulk import configs
- `POST /api/v1/config/bulk-export` - Bulk export configs
- `POST /api/v1/config/bulk-drift-detect` - Bulk drift detection
- `POST /api/v1/config/bulk-drift-detect-enhanced` - Enhanced bulk drift

### Step 2: Create Components

**Files**:
- `ui/src/components/bulk/DeviceSelector.vue` - Multi-select device picker
- `ui/src/components/bulk/BulkImportDialog.vue` - Import workflow
- `ui/src/components/bulk/BulkExportDialog.vue` - Export workflow
- `ui/src/components/bulk/BulkDriftDialog.vue` - Drift detection
- `ui/src/components/bulk/OperationProgress.vue` - Progress tracking

### Step 3: Integration with DevicesPage

**File**: `ui/src/pages/DevicesPage.vue`

Add bulk operation controls:
- Checkbox column for device selection
- "Select All" / "Deselect All" buttons
- Bulk actions dropdown (Import, Export, Detect Drift)
- Selection count indicator

### Step 4: Progress Tracking

Implement polling or WebSocket updates for operation progress:
- Show progress bar during operations
- Display individual device results
- Handle partial failures gracefully
- Allow cancellation where supported

### Step 5: Results Display

Create results summary component:
- Success/failure counts
- Per-device status
- Error messages for failures
- Export/download results

## Backend Endpoints (4 total)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v1/config/bulk-import` | Import to multiple devices |
| POST | `/api/v1/config/bulk-export` | Export from multiple devices |
| POST | `/api/v1/config/bulk-drift-detect` | Basic bulk drift |
| POST | `/api/v1/config/bulk-drift-detect-enhanced` | Enhanced with options |

## Related Tasks

- **341**: Device Management API - device selection
- **342**: Device Configuration UI - config operations
- **345**: Drift Detection UI - drift workflows

## Dependencies

- **After**: Task 341 (Device Management API) - uses device selection from device store

## Validation

```bash
# Run frontend tests
npm run test

# Run E2E tests
npm run test:e2e -- --grep "bulk"

# Type checking
npm run type-check
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update API coverage statistics in Section 4.2
- Move endpoints from "Unused" to "Used" in Section 4.3/4.4

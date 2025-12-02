# Break Up Large Page Components

**Priority**: HIGH - Code Quality
**Status**: in-progress
**Effort**: 13 hours (with 1.3x buffer)

## Context

Three page components exceed 1,000 lines of code, making them difficult to maintain, test, and understand. This task refactors these components by extracting logical sections into smaller child components.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 1.4 (Areas of Concern)

## Success Criteria

- [ ] BackupManagementPage.vue reduced from 1,625 to <500 lines
- [x] Extract backup statistics into `components/backup/BackupStatistics.vue`
- [ ] GitOpsExportPage.vue reduced from 1,351 to <500 lines
- [ ] PluginManagementPage.vue reduced from 1,151 to <500 lines
- [x] Extract plugin card into `components/plugins/PluginCard.vue`
- [x] Extract plugin modals into `components/plugins/PluginConfigModal.vue` and `PluginDetailsModal.vue`
 - [x] Extract plugin filters into `components/plugins/PluginFilters.vue`
- [ ] All existing functionality preserved
- [ ] E2E tests pass without modification
- [ ] Type safety maintained
- [ ] No new TypeScript errors
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Analyze BackupManagementPage (1,625 lines)

**File**: `ui/src/pages/BackupManagementPage.vue`

Identify extractable sections:
- Backup creation form → `BackupCreateForm.vue`
- Backup list/table → `BackupList.vue`
- Restore preview → `RestorePreview.vue`
- Backup details dialog → `BackupDetailsDialog.vue`
- Statistics section → `BackupStatistics.vue`
- Filter controls → shared `FilterBar.vue`

### Step 2: Analyze GitOpsExportPage (1,351 lines)

**File**: `ui/src/pages/GitOpsExportPage.vue`

Identify extractable sections:
- Export form → `GitOpsExportForm.vue`
- Repository config → `GitOpsRepoConfig.vue`
- Template selector → `GitOpsTemplateSelector.vue`
- Export list → `GitOpsExportList.vue`
- Preview dialog → `GitOpsPreviewDialog.vue`
- Variable editor → `GitOpsVariables.vue`

### Step 3: Analyze PluginManagementPage (1,151 lines)

**File**: `ui/src/pages/PluginManagementPage.vue`

Identify extractable sections:
- Plugin list → `PluginList.vue`
- Plugin details panel → `PluginDetailsPanel.vue`
- Configuration form → shared with `PluginConfigForm.vue`
- Health status → `PluginHealthStatus.vue`
- Test results → `PluginTestResults.vue`

### Step 4: Create Shared Components

**Directory**: `ui/src/components/shared/`

Create reusable components:
- `FilterBar.vue` - Common filter controls
- `StatisticsCard.vue` - Stats display
- `ConfirmDialog.vue` - Confirmation dialogs
- `LoadingState.vue` - Loading indicators
- `EmptyState.vue` - Empty state displays

### Step 5: Refactor Each Page

For each page:
1. Extract child components
2. Define clear props/emits interfaces
3. Use provide/inject for deep state where appropriate
4. Keep page component as orchestrator only
5. Run E2E tests after each refactor

### Step 6: Update Imports

Ensure all new components are properly imported and exported.

## Target Structure

```
ui/src/
├── pages/
│   ├── BackupManagementPage.vue      # <500 lines (orchestrator)
│   ├── GitOpsExportPage.vue          # <500 lines (orchestrator)
│   └── PluginManagementPage.vue      # <500 lines (orchestrator)
├── components/
│   ├── backup/
│   │   ├── BackupCreateForm.vue
│   │   ├── BackupList.vue
│   │   ├── BackupDetailsDialog.vue
│   │   ├── BackupStatistics.vue
│   │   └── RestorePreview.vue
│   ├── gitops/
│   │   ├── GitOpsExportForm.vue
│   │   ├── GitOpsRepoConfig.vue
│   │   ├── GitOpsTemplateSelector.vue
│   │   ├── GitOpsExportList.vue
│   │   ├── GitOpsPreviewDialog.vue
│   │   └── GitOpsVariables.vue
│   ├── plugins/
│   │   ├── PluginList.vue
│   │   ├── PluginDetailsPanel.vue
│   │   ├── PluginHealthStatus.vue
│   │   └── PluginTestResults.vue
│   └── shared/
│       ├── FilterBar.vue
│       ├── StatisticsCard.vue
│       ├── ConfirmDialog.vue
│       ├── LoadingState.vue
│       └── EmptyState.vue
```

## Related Tasks

- **352**: Schema-Driven Form Component - depends on this refactor
- **354**: Improve Error Messages - coordinate on error patterns
- **355**: Page Component Unit Tests - easier to test after refactor

## Dependencies

- **Enables**: Tasks 352, 354, 355 (should be completed first)

## Validation

```bash
# Run type checking
npm run type-check

# Run all E2E tests
npm run test:e2e

# Run specific page tests
npm run test:e2e -- --grep "backup"
npm run test:e2e -- --grep "gitops"
npm run test:e2e -- --grep "plugin"

# Verify no regressions
make test-ci
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update line counts in Appendix: File Reference
- Update Section 1.4 to mark issue as resolved
- Update Section 7.6 Success Metrics

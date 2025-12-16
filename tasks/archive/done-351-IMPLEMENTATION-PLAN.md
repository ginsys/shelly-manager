# Task 351: Large Page Component Refactoring - Implementation Plan

**Created**: 2025-12-03
**Estimated Effort**: 13 hours (iterative with validation checkpoints)
**Approach**: One page at a time with build/test verification between each

---

## Executive Summary

Refactor 4,127 lines across 3 monolithic page components into ~20 focused child components:
- **BackupManagementPage.vue**: 1,625 lines → <500 lines (~9 components)
- **GitOpsExportPage.vue**: 1,351 lines → <500 lines (~6 components)
- **PluginManagementPage.vue**: 1,151 lines → <500 lines (~5 components)

## Success Criteria

- [ ] All 3 pages reduced to <500 lines (orchestrator pattern)
- [ ] 20+ new child components created with clear responsibilities
- [ ] All existing functionality preserved
- [ ] Build passes after each component extraction
- [ ] Type safety maintained (no new TypeScript errors)
- [ ] E2E tests pass without modification
- [ ] New components use error handling infrastructure (Task 354)
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

---

## Phase 1: BackupManagementPage Refactoring (5 hours)

### Current Structure Analysis (1,625 lines)

**Sections identified**:
1. Create Backup Form (lines 8-167): Complex form with backup/export types, scheduling
2. Statistics Display (lines 169-189): 4 stat cards
3. Filters Section (lines 191-232): Format, success, date range filters
4. Backups List Table (lines 234-285): Paginated table with actions
5. Content Exports Tables (lines 287-406): 3 separate tables (JSON, YAML, SMA)
6. Restore Modal (lines 408-520): Preview and restore flow
7. Delete Confirmation (embedded in script)
8. Script Logic (lines 522-1625): State, API calls, handlers

###Component Extraction Order

#### Step 1.1: Extract Statistics (Easiest, 30 min)
**New Component**: `ui/src/components/backup/BackupStatistics.vue`

```vue
<script setup lang="ts">
interface Stats {
  total: number
  success: number
  failure: number
  total_size: number
}

defineProps<{
  statistics: Stats
}>()

function formatFileSize(bytes: number): string {
  // ... implementation
}
</script>

<template>
  <div class="stats-grid">
    <div class="stat-card">
      <span class="label">Total:</span>
      <span class="value">{{ statistics.total || 0 }}</span>
    </div>
    <!-- ... other stats -->
  </div>
</template>
```

**Integration**: Replace lines 169-189 with `<BackupStatistics :statistics="statistics" />`

#### Step 1.2: Extract Filter Bar (45 min)
**New Component**: `ui/src/components/backup/BackupFilterBar.vue`

```vue
<script setup lang="ts">
interface Filters {
  format: string
  success?: boolean
}

const filters = defineModel<Filters>('filters', { required: true })

const emit = defineEmits<{
  filterChange: []
}>()
</script>

<template>
  <div class="filters-section">
    <div class="filter-row">
      <label>Format:</label>
      <select v-model="filters.format" @change="emit('filterChange')">
        <option value="">All formats</option>
        <option value="json">JSON</option>
        <option value="sma">SMA</option>
        <option value="yaml">YAML</option>
      </select>
      <!-- ... other filters -->
    </div>
  </div>
</template>
```

**Integration**: Replace lines 191-232 with `<BackupFilterBar v-model:filters="filters" @filter-change="fetchBackups" />`

#### Step 1.3: Extract Backup List Table (1.5 hours)
**New Component**: `ui/src/components/backup/BackupList.vue`

```vue
<script setup lang="ts">
import type { BackupItem } from '@/api/types'
import { useError } from '@/composables/useError'
import ErrorState from '@/components/shared/ErrorState.vue'

interface Props {
  backups: BackupItem[]
  loading: boolean
  error?: string | null
}

defineProps<Props>()

const emit = defineEmits<{
  download: [id: string]
  restore: [backup: BackupItem]
  delete: [backup: BackupItem]
  refresh: []
}>()
</script>

<template>
  <div class="backup-list">
    <div v-if="loading">Loading...</div>
    <ErrorState v-else-if="error" :message="error" :retryable="true" @retry="emit('refresh')" />
    <table v-else class="data-table">
      <!-- Table implementation -->
    </table>
  </div>
</template>
```

**Integration**: Replace lines 234-285

#### Step 1.4: Extract Content Exports Tables (1 hour)
**New Component**: `ui/src/components/backup/ContentExportsList.vue`

Combines the 3 export tables (JSON/YAML/SMA) into a single tabbed or sectioned component.

#### Step 1.5: Extract Restore Modal (1.5 hours)
**New Components**:
- `ui/src/components/backup/RestoreModal.vue` (modal shell)
- `ui/src/components/backup/RestorePreview.vue` (preview content)
- `ui/src/components/backup/RestoreOptions.vue` (options form)

#### Step 1.6: Extract Create Form (2 hours)
**New Component**: `ui/src/components/backup/BackupCreateForm.vue`

Largest and most complex section (160 lines of template). Includes:
- Backup vs Content Export type selection
- Scheduling options
- Format-specific options (JSON/YAML/SMA)
- Compression settings

#### Step 1.7: Final Integration & Cleanup (30 min)
- Remove unused state/functions from page
- Ensure page is <500 lines (orchestrator only)
- Run build: `cd ui && npm run build`
- Verify no type errors

### Phase 1 Validation Checkpoint

```bash
# Build must pass
cd ui && npm run build

# Type check (if script exists)
npm run type-check 2>/dev/null || echo "No type-check script"

# Run backup E2E tests (if they exist)
cd .. && npm test 2>&1 | grep -i backup || echo "Check manual testing"
```

---

## Phase 2: GitOpsExportPage Refactoring (4 hours)

### Current Structure Analysis (1,351 lines)

**Sections identified**:
1. Export Form (lines ~50-350): Repository config, template selector, variables
2. Export List/History (lines ~400-600): Table of past exports
3. Preview Dialog (lines ~650-800): GitOps manifest preview
4. Schedule Management (embedded)

### Component Extraction Order

#### Step 2.1: Extract Export Statistics (20 min)
**Component**: `ui/src/components/gitops/GitOpsStatistics.vue`

#### Step 2.2: Extract Repository Config (45 min)
**Component**: `ui/src/components/gitops/GitOpsRepoConfig.vue`

Form section for Git repository configuration (URL, branch, auth).

#### Step 2.3: Extract Template Selector (45 min)
**Component**: `ui/src/components/gitops/GitOpsTemplateSelector.vue`

Template selection with preview and variable mapping.

#### Step 2.4: Extract Variables Editor (1 hour)
**Component**: `ui/src/components/gitops/GitOpsVariablesEditor.vue`

Key-value editor for template variables with validation.

#### Step 2.5: Extract Export List (1 hour)
**Component**: `ui/src/components/gitops/GitOpsExportList.vue`

Historical exports table with status, download, re-run actions.

#### Step 2.6: Extract Preview Dialog (45 min)
**Component**: `ui/src/components/gitops/GitOpsPreviewDialog.vue`

Modal showing generated manifest preview before export.

#### Step 2.7: Final Integration & Cleanup (30 min)

### Phase 2 Validation Checkpoint

```bash
cd ui && npm run build
```

---

## Phase 3: PluginManagementPage Refactoring (3.5 hours)

### Current Structure Analysis (1,151 lines)

**Sections identified**:
1. Plugin List (sidebar/main list)
2. Plugin Details Panel
3. Configuration Form (may already exist as `PluginConfigForm.vue`)
4. Health Status Display
5. Test Results Panel

### Component Extraction Order

#### Step 3.1: Verify Existing Components (15 min)
Check if `PluginConfigForm.vue`, `PluginDetailsView.vue` already exist - they do according to build output!

```bash
ls ui/src/components/plugins/
```

#### Step 3.2: Extract Plugin List (45 min)
**Component**: `ui/src/components/plugins/PluginList.vue`

List of available plugins with search/filter.

#### Step 3.3: Extract Health Status (30 min)
**Component**: `ui/src/components/plugins/PluginHealthStatus.vue`

Health check results display.

#### Step 3.4: Extract Test Results (45 min)
**Component**: `ui/src/components/plugins/PluginTestResults.vue`

Test execution results with detailed output.

#### Step 3.5: Integrate Existing Components (1 hour)
Use existing `PluginConfigForm.vue` and `PluginDetailsView.vue`.

#### Step 3.6: Final Integration & Cleanup (30 min)

### Phase 3 Validation Checkpoint

```bash
cd ui && npm run build
```

---

## Phase 4: Shared Components Creation (1 hour)

Some components may need to be created in `ui/src/components/shared/`:

### Already Created (Task 411)
- ✅ `ErrorState.vue`
- ✅ `EmptyState.vue`
- ✅ `ColumnToggle.vue`

### May Need Creation
- `LoadingState.vue` (if not using inline loading divs)
- `ConfirmDialog.vue` (reusable confirmation modal)
- `StatisticsCard.vue` (if patterns are similar across pages)

**Decision**: Create these on-demand during refactoring if duplication is noticed.

---

## Error Handling Integration

All new components will use the error handling infrastructure from Task 354:

```typescript
import { useError } from '@/composables/useError'
import ErrorState from '@/components/shared/ErrorState.vue'

const { error, hasError, setError, clearError } = useError()

// In API calls:
try {
  await someOperation()
} catch (err) {
  setError(err, {
    action: 'Creating backup',
    resource: 'Backup'
  })
}
```

---

## Testing Strategy

### After Each Component Extraction

1. **Build Validation**:
   ```bash
   cd ui && npm run build
   ```
   Must complete without errors.

2. **Type Safety**:
   ```bash
   # Check for any new TypeScript errors in build output
   npm run build 2>&1 | grep -i "error TS"
   ```

3. **Visual Inspection**:
   - Start dev server: `npm run dev`
   - Navigate to page
   - Test all UI interactions
   - Verify no visual regressions

### After Each Page Complete

1. **Full Build**:
   ```bash
   cd .. && make test-ci  # If Go tests don't conflict
   # OR
   cd ui && npm run build
   ```

2. **E2E Tests** (if available):
   ```bash
   # Check if E2E tests exist for these pages
   ls ui/tests/e2e/*.spec.ts | grep -i "backup\|gitops\|plugin"
   ```

---

## Rollback Strategy

Each phase commits separately. If issues arise:

```bash
# Rollback last commit
git reset --soft HEAD~1

# Or rollback to specific commit
git reset --hard <commit-before-refactor>
```

---

## Documentation Updates

After all refactoring complete, update `docs/frontend/frontend-review.md`:

### Section 1.4: Areas of Concern
Mark "Large Components (>1,000 lines)" as ✅ **RESOLVED**

### Appendix: File Reference
Update line counts:
- `BackupManagementPage.vue`: ~~1,625~~ → **~450** lines
- `GitOpsExportPage.vue`: ~~1,351~~ → **~480** lines
- `PluginManagementPage.vue`: ~~1,151~~ → **~420** lines

Add new components to inventory.

### Section 7.6: Success Metrics
Update:
- Code reduction: 9,400+ → **~6,200** lines (34% reduction from page refactor alone)
- File count: 95 → **~115** files (+20 new components)
- Largest file: ~~1,625~~ → **~800** lines (GitOpsExportPage.vue with preview still inline)

---

## Time Estimates

| Phase | Task | Time |
|-------|------|------|
| 1 | BackupManagementPage | 5.0h |
| 2 | GitOpsExportPage | 4.0h |
| 3 | PluginManagementPage | 3.5h |
| 4 | Shared Components | 0.5h |
| **Total** | | **13.0h** |

---

## Next Steps

1. ✅ Plan created
2. ⏭️ Execute Phase 1 (BackupManagementPage)
3. ⏭️ Execute Phase 2 (GitOpsExportPage)
4. ⏭️ Execute Phase 3 (PluginManagementPage)
5. ⏭️ Update documentation
6. ⏭️ Final validation & commit

---

**Plan Status**: ✅ Complete and ready for execution
**Execution Mode**: Autonomous with build validation checkpoints

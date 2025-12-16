# Add Page Component Unit Tests

**Priority**: MEDIUM - Testing
**Status**: not-started
**Effort**: 16 hours (with 1.3x buffer)

## Context

Most complex page components lack unit tests. While E2E tests provide coverage for user flows, unit tests are needed for testing component logic, state management, and edge cases in isolation.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 1.4 (Areas of Concern)
**Phase 8 Reference**: Section 6 - Unit: Vitest + Testing Library

## Success Criteria

- [ ] Unit tests added for BackupManagementPage
- [ ] Unit tests added for GitOpsExportPage
- [ ] Unit tests added for PluginManagementPage
- [ ] Unit tests added for ExportSchedulesPage
- [ ] Test coverage increased from ~20% to ~80%
- [ ] API mocking patterns established
- [ ] Store mocking patterns established
- [ ] All tests pass in CI
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Setup Test Utilities

**File**: `ui/src/test/utils.ts`

Create test utilities for common patterns:

```typescript
import { mount, VueWrapper } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'

export function mountWithPlugins(component: Component, options = {}) {
  return mount(component, {
    global: {
      plugins: [
        createTestingPinia({ stubActions: false }),
        router
      ],
      stubs: ['router-link', 'router-view']
    },
    ...options
  })
}

export function mockApiCall(module: string, method: string, data: unknown) {
  vi.mock(`@/api/${module}`, () => ({
    [method]: vi.fn().mockResolvedValue({ data })
  }))
}
```

### Step 2: Test BackupManagementPage

**File**: `ui/src/pages/__tests__/BackupManagementPage.test.ts`

Test cases:
- Initial data loading
- Backup creation flow
- Restore preview and execution
- Error handling
- Pagination
- Filtering
- Delete confirmation

### Step 3: Test GitOpsExportPage

**File**: `ui/src/pages/__tests__/GitOpsExportPage.test.ts`

Test cases:
- Export form validation
- Template selection
- Repository configuration
- Preview generation
- Export creation
- Download functionality

### Step 4: Test PluginManagementPage

**File**: `ui/src/pages/__tests__/PluginManagementPage.test.ts`

Test cases:
- Plugin list loading
- Plugin selection
- Configuration form
- Test plugin functionality
- Save configuration
- Health status display

### Step 5: Test ExportSchedulesPage

**File**: `ui/src/pages/__tests__/ExportSchedulesPage.test.ts`

Test cases:
- Schedule list loading
- Create schedule
- Edit schedule
- Delete schedule
- Run schedule manually
- Schedule validation

### Step 6: API Mocking Patterns

**File**: `ui/src/test/mocks/api.ts`

Create reusable API mocks:

```typescript
export const mockBackupApi = {
  createBackup: vi.fn().mockResolvedValue({ data: { id: '1' } }),
  listBackups: vi.fn().mockResolvedValue({ data: { backups: [] } }),
  deleteBackup: vi.fn().mockResolvedValue({ data: { success: true } })
}

export const mockPluginApi = {
  listPlugins: vi.fn().mockResolvedValue({ data: { plugins: [] } }),
  getPluginSchema: vi.fn().mockResolvedValue({ data: { schema: {} } }),
  testPlugin: vi.fn().mockResolvedValue({ data: { success: true } })
}
```

### Step 7: Store Mocking Patterns

**File**: `ui/src/test/mocks/stores.ts`

Create reusable store mocks:

```typescript
export function createMockExportStore() {
  return {
    items: [],
    loading: false,
    error: null,
    fetchHistory: vi.fn(),
    createExport: vi.fn()
  }
}
```

### Step 8: Integration with CI

Ensure tests run in CI pipeline:
- Add to `npm run test` script
- Coverage thresholds in vitest.config.ts
- Coverage report generation

## Test Coverage Targets

| Component | Current | Target |
|-----------|---------|--------|
| BackupManagementPage | 0% | 80% |
| GitOpsExportPage | 0% | 80% |
| PluginManagementPage | 0% | 80% |
| ExportSchedulesPage | 50% | 80% |
| Overall Pages | ~20% | ~80% |

## Related Tasks

- **351**: Break Up Large Page Components - refactor first makes testing easier
- **352**: Schema-Driven Form Component - test form component
- **354**: Improve Error Messages - test error handling

## Dependencies

- **After**: Task 351 (testing is easier after refactor)

## Validation

```bash
# Run all unit tests
npm run test

# Run with coverage
npm run test:coverage

# Run specific page tests
npm run test -- --grep "BackupManagementPage"
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update Section 1.4 to mark test coverage as improved
- Update Section 7.6 Success Metrics with new coverage percentage

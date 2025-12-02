# Remove StatsPage

**Priority**: MEDIUM - Navigation Cleanup
**Status**: not-started
**Effort**: 2 hours

## Context

The StatsPage.vue exists at route `/stats` but is not accessible from the navigation menu. This creates an orphaned route that users cannot discover or access.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 3 (Unreachable Pages)
**Architect Review**: Recommended Option B (removal)

## Recommendation: Remove Page

**Rationale:**
- StatsPage is a 38-line stub with no unique functionality
- MetricsDashboardPage already provides comprehensive metrics display
- Maintaining redundant routes adds confusion
- No plans to expand StatsPage functionality

## Success Criteria

- [ ] Route removed from router configuration
- [ ] StatsPage.vue component deleted
- [ ] No orphaned routes remain
- [ ] E2E tests updated (remove any /stats references)
- [ ] No broken imports
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Remove Route

**File**: `ui/src/main.ts`

Remove the stats route:

```typescript
// Remove this route
{ path: '/stats', name: 'stats', component: StatsPage }
```

### Step 2: Delete Component

```bash
rm ui/src/pages/StatsPage.vue
```

### Step 3: Remove Import

**File**: `ui/src/main.ts`

Remove the StatsPage import if it exists.

### Step 4: Update Tests

Search for and remove any E2E tests referencing `/stats` route:

```bash
grep -r "/stats" ui/tests/
```

### Step 5: Verify No Broken References

```bash
# Check for any remaining references
grep -r "StatsPage" ui/src/
grep -r "stats" ui/src/main.ts
```

## Related Tasks

- None - standalone cleanup task

## Validation

```bash
# Run type checking (catch broken imports)
npm run type-check

# Run E2E tests
npm run test:e2e

# Manual verification
# - Verify /stats returns 404 or redirects
# - Verify all menu items work
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update Section 3 (Unreachable Pages) to mark as resolved
- Remove StatsPage from Section 2.2 (Pages & User Flows)
- Update Appendix: File Reference (remove StatsPage line count)
- Update Section 7.6 Success Metrics (Unreachable Routes: 1 → 0)

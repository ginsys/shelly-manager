# Devices UI Refactor

**Priority**: LOW - Enhancement
**Status**: not-started
**Effort**: 6-8 hours

## Context

The devices pages could benefit from consolidation and improved UX. This is an optional enhancement to improve code maintainability and user experience.

## Success Criteria

- [ ] Devices pages consolidated on `ui/src/stores/devices.ts`
- [ ] Error and empty states unified
- [ ] Column toggles added
- [ ] Page size controls added
- [ ] Code duplication reduced

## Implementation

### Step 1: Consolidate Store

**File**: `ui/src/stores/devices.ts`

Ensure all device-related state is managed in a single store with:
- Pagination state
- Filter state
- Column visibility preferences
- Page size preference

### Step 2: Unify Error States

Create shared error component:

**File**: `ui/src/components/shared/ErrorState.vue`

```vue
<template>
  <div class="error-state">
    <span class="error-icon">!</span>
    <h3>{{ title }}</h3>
    <p>{{ message }}</p>
    <button v-if="retryable" @click="$emit('retry')">Retry</button>
  </div>
</template>

<script setup>
defineProps<{
  title: string
  message: string
  retryable?: boolean
}>()

defineEmits<{
  retry: []
}>()
</script>
```

### Step 3: Unify Empty States

Create shared empty component:

**File**: `ui/src/components/shared/EmptyState.vue`

```vue
<template>
  <div class="empty-state">
    <span class="empty-icon">...</span>
    <h3>{{ title }}</h3>
    <p>{{ message }}</p>
    <slot name="action" />
  </div>
</template>

<script setup>
defineProps<{
  title: string
  message: string
}>()
</script>
```

### Step 4: Add Column Toggles

Create column toggle component:

**File**: `ui/src/components/shared/ColumnToggle.vue`

Allow users to show/hide table columns and persist preference.

### Step 5: Add Page Size Controls

Add page size selector (10, 25, 50, 100) with localStorage persistence.

## Validation

```bash
# Run frontend tests
npm run test

# Type checking
npm run type-check

# Manual testing in browser
```

## Dependencies

None

## Risk

Low - UI improvement only

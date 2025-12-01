# Router-Link Accessibility Fix

**Priority**: CRITICAL - Blocks Commit
**Status**: not-started
**Effort**: 10 minutes

## Context

The `ExportSchedulesPage.vue` uses `<router-link>` styled as a button, which causes accessibility issues. Screen readers announce it as a "link" instead of "button", and keyboard users don't get proper button behavior (Space key doesn't work).

## Success Criteria

- [ ] Navigation works correctly
- [ ] Press Space key on focused button activates it
- [ ] Screen reader announces as "button Create Schedule"
- [ ] Lighthouse accessibility audit passes (no button-inside-link warnings)

## Implementation

**File**: `ui/src/pages/ExportSchedulesPage.vue`

Replace the router-link with a proper button:

```vue
<!-- BEFORE (incorrect - accessibility issue) -->
<router-link
  class="primary-button"
  to="/export/backup?schedule=1#create-backup"
>
  + Create Schedule
</router-link>

<!-- AFTER (correct - proper button semantics) -->
<button
  class="primary-button"
  @click="navigateToScheduleCreation"
>
  + Create Schedule
</button>
```

Add to `<script setup>` section:

```vue
<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()

function navigateToScheduleCreation() {
  router.push('/export/backup?schedule=1#create-backup')
}
</script>
```

## Why This Matters

- Screen readers announce "button" not "link" (correct semantics)
- Keyboard users get proper button behavior (Space key works)
- Follows WCAG 2.1 accessibility guidelines
- Fixes semantic HTML violation

## Validation

1. Click button - should navigate correctly
2. Press Space key on focused button - should activate
3. Screen reader test: Should announce as "button Create Schedule"
4. Run Lighthouse accessibility audit: No "button inside link" warnings

## Dependencies

None

## Risk

Low - Direct router API usage, standard Vue pattern

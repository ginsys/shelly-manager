# Improve Error Messages

**Priority**: MEDIUM - Enhancement
**Status**: not-started
**Effort**: 5 hours (with 1.3x buffer)

## Context

Error messages in the frontend are generic (e.g., "Failed to load devices") and lack context for debugging or user action. This task improves error handling with contextual messages, error codes, and suggested actions.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 1.4 (Areas of Concern)

## Success Criteria

- [ ] Error display component created with standardized presentation
- [ ] Error messages include error codes from backend
- [ ] Contextual information added (what action failed)
- [ ] Suggested actions provided where applicable
- [ ] Retry functionality integrated
- [ ] All pages updated to use new error handling
- [ ] Unit tests for error component
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Define Error Types

**File**: `ui/src/types/errors.ts`

```typescript
interface AppError {
  code: string
  message: string
  details?: string
  context?: {
    action: string
    resource?: string
    resourceId?: string
  }
  suggestions?: string[]
  retryable?: boolean
}

// Error code mapping
const ERROR_MESSAGES: Record<string, string> = {
  'DEVICE_NOT_FOUND': 'The device could not be found. It may have been deleted.',
  'DEVICE_OFFLINE': 'The device is currently offline. Check network connectivity.',
  'VALIDATION_FAILED': 'The input data is invalid. Please check your entries.',
  'UNAUTHORIZED': 'You are not authorized. Please check your API key.',
  'RATE_LIMIT_EXCEEDED': 'Too many requests. Please wait a moment and try again.',
  // ... more mappings
}
```

### Step 2: Create Error Display Component

**File**: `ui/src/components/shared/ErrorDisplay.vue`

```vue
<template>
  <div class="error-display" :class="severity">
    <div class="error-header">
      <q-icon :name="icon" />
      <span class="error-title">{{ title }}</span>
      <span class="error-code" v-if="error.code">[{{ error.code }}]</span>
    </div>
    <p class="error-message">{{ error.message }}</p>
    <p class="error-details" v-if="error.details">{{ error.details }}</p>
    <ul class="error-suggestions" v-if="error.suggestions?.length">
      <li v-for="suggestion in error.suggestions" :key="suggestion">
        {{ suggestion }}
      </li>
    </ul>
    <div class="error-actions">
      <q-btn v-if="error.retryable" @click="$emit('retry')" label="Retry" />
      <q-btn @click="$emit('dismiss')" label="Dismiss" flat />
    </div>
  </div>
</template>
```

### Step 3: Create Error Composable

**File**: `ui/src/composables/useError.ts`

```typescript
export function useError() {
  const error = ref<AppError | null>(null)
  const hasError = computed(() => error.value !== null)

  function setError(err: unknown, context?: ErrorContext) {
    error.value = normalizeError(err, context)
  }

  function clearError() {
    error.value = null
  }

  function normalizeError(err: unknown, context?: ErrorContext): AppError {
    // Handle axios errors
    if (isAxiosError(err)) {
      return {
        code: err.response?.data?.error?.code || 'NETWORK_ERROR',
        message: err.response?.data?.error?.message || 'Network error occurred',
        details: err.response?.data?.error?.details,
        context,
        suggestions: getSuggestions(err),
        retryable: isRetryable(err)
      }
    }
    // Handle other error types
    return { code: 'UNKNOWN', message: String(err), context }
  }

  return { error, hasError, setError, clearError }
}
```

### Step 4: Update API Client

**File**: `ui/src/api/client.ts`

Add error interceptor that preserves backend error structure:

```typescript
api.interceptors.response.use(
  response => response,
  error => {
    // Preserve error details from backend
    if (error.response?.data?.error) {
      error.appError = error.response.data.error
    }
    return Promise.reject(error)
  }
)
```

### Step 5: Update Pages

Update major pages to use new error handling:
- DevicesPage.vue
- BackupManagementPage.vue
- GitOpsExportPage.vue
- PluginManagementPage.vue
- MetricsDashboardPage.vue

Replace generic error handling with:

```typescript
const { error, hasError, setError, clearError } = useError()

try {
  await fetchDevices()
} catch (err) {
  setError(err, { action: 'Loading devices' })
}
```

### Step 6: Add Tests

**File**: `ui/src/components/shared/__tests__/ErrorDisplay.test.ts`

Test cases:
- Error code display
- Suggestion rendering
- Retry button visibility
- Dismiss functionality

## Related Tasks

- **351**: Break Up Large Page Components - coordinate on error patterns
- **355**: Page Component Unit Tests - test error handling

## Dependencies

- **Coordinate with**: Task 351 (establish error patterns during refactor)

## Validation

```bash
# Run unit tests
npm run test -- --grep "ErrorDisplay"

# Run type checking
npm run type-check

# Manual testing - trigger various error conditions
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update Section 1.4 to mark error messages as resolved
- Add error handling documentation to Appendix

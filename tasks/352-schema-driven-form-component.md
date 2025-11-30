# Create Schema-Driven Form Component

**Priority**: MEDIUM - Code Quality
**Status**: not-started
**Effort**: 10 hours (with 1.3x buffer)

## Context

Multiple form components (ExportPreviewForm, ImportPreviewForm, SMAConfigForm, GitOpsConfigForm) contain overlapping form rendering logic. This task extracts the common logic into a generic schema-driven form component.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 1.4 (Areas of Concern)
**Phase 8 Reference**: Section 5 - Configuration typed forms

## Success Criteria

- [ ] Generic `SchemaForm.vue` component created
- [ ] Component supports dynamic field generation from schema
- [ ] Validation integration with error display
- [ ] Support for nested objects and arrays
- [ ] Existing forms refactored to use SchemaForm
- [ ] Form duplication reduced significantly
- [ ] Unit tests for SchemaForm component
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Analyze Existing Forms

Review form patterns in:
- `ui/src/components/ExportPreviewForm.vue`
- `ui/src/components/ImportPreviewForm.vue`
- `ui/src/components/SMAConfigForm.vue`
- `ui/src/components/GitOpsConfigForm.vue`

Identify common patterns:
- Field type rendering (text, number, select, checkbox)
- Validation logic
- Error display
- Form state management
- Submit handling

### Step 2: Design Schema Format

Define schema structure:

```typescript
interface FormSchema {
  fields: FieldDefinition[]
  validation?: ValidationRules
  layout?: LayoutOptions
}

interface FieldDefinition {
  name: string
  type: 'text' | 'number' | 'select' | 'checkbox' | 'textarea' | 'json' | 'array'
  label: string
  required?: boolean
  placeholder?: string
  options?: SelectOption[]  // for select fields
  default?: unknown
  validation?: FieldValidation[]
  depends?: DependencyRule  // conditional display
  description?: string
}
```

### Step 3: Create SchemaForm Component

**File**: `ui/src/components/shared/SchemaForm.vue`

Features:
- Dynamic field rendering based on schema
- Two-way binding with v-model
- Built-in validation with Quasar rules
- Error message display
- Loading/disabled states
- Submit/cancel buttons
- Slot for custom field types

### Step 4: Create Field Components

**Directory**: `ui/src/components/shared/form/`

- `FormField.vue` - Field wrapper with label/error
- `TextField.vue` - Text input
- `NumberField.vue` - Number input
- `SelectField.vue` - Dropdown select
- `CheckboxField.vue` - Checkbox
- `TextareaField.vue` - Multi-line text
- `JsonField.vue` - JSON editor
- `ArrayField.vue` - Array items

### Step 5: Create Form Composable

**File**: `ui/src/composables/useSchemaForm.ts`

```typescript
export function useSchemaForm(schema: FormSchema) {
  const formData = ref({})
  const errors = ref({})
  const isDirty = ref(false)

  function validate() { ... }
  function reset() { ... }
  function submit() { ... }

  return { formData, errors, isDirty, validate, reset, submit }
}
```

### Step 6: Refactor Existing Forms

Replace form logic in each existing form with SchemaForm:
1. Define schema for the form
2. Replace template with `<SchemaForm :schema="schema" v-model="data" />`
3. Keep custom logic in parent component
4. Test thoroughly

### Step 7: Add Tests

**File**: `ui/src/components/shared/__tests__/SchemaForm.test.ts`

Test cases:
- Field rendering for each type
- Validation execution
- Error display
- Form submission
- Conditional fields
- Default values

## Related Tasks

- **342**: Device Configuration UI - uses schema forms
- **343**: Configuration Templates UI - uses schema forms
- **344**: Typed Configuration UI - uses schema forms
- **351**: Break Up Large Page Components - should be completed first

## Dependencies

- **Depends on**: Task 351 (Break Up Large Page Components)

## Validation

```bash
# Run unit tests
npm run test -- --grep "SchemaForm"

# Run type checking
npm run type-check

# Run E2E tests for forms
npm run test:e2e -- --grep "form"
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update Section 1.4 to mark form duplication as resolved
- Update Section 7.6 Success Metrics with new duplication percentage

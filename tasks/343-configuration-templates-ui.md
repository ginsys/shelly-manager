# Configuration Templates UI

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 13 hours (with 1.3x buffer)

## Context

The backend provides 8 configuration template endpoints that are not exposed in the frontend. Templates allow users to define reusable device configurations that can be applied to multiple devices.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 4.4 (Unused Endpoints)

## Success Criteria

- [ ] API client created for template endpoints
- [ ] Template list page with filtering by device type
- [ ] Template create/edit forms with validation
- [ ] Template preview functionality
- [ ] Template validation before save
- [ ] Example templates browser
- [ ] Apply template to device workflow
- [ ] Unit tests for API functions
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Create API Client

**File**: `ui/src/api/templates.ts`

Add endpoints:
- `GET /api/v1/config/templates` - List templates (with pagination)
- `POST /api/v1/config/templates` - Create template
- `PUT /api/v1/config/templates/{id}` - Update template
- `DELETE /api/v1/config/templates/{id}` - Delete template
- `POST /api/v1/configuration/preview-template` - Preview template rendering
- `POST /api/v1/configuration/validate-template` - Validate template syntax
- `POST /api/v1/configuration/templates` - Save template (alternate)
- `GET /api/v1/configuration/template-examples` - Get example templates

### Step 2: Create Pinia Store

**File**: `ui/src/stores/templates.ts`

State management for:
- Template list with pagination
- Current template being edited
- Template preview results
- Validation errors

### Step 3: Create Pages

**Files**:
- `ui/src/pages/TemplatesPage.vue` - Template list with filters
- `ui/src/pages/TemplateDetailPage.vue` - View/edit template
- `ui/src/pages/TemplateExamplesPage.vue` - Browse examples

### Step 4: Create Components

**Files**:
- `ui/src/components/templates/TemplateForm.vue` - Create/edit form
- `ui/src/components/templates/TemplatePreview.vue` - Preview rendered template
- `ui/src/components/templates/TemplateVariables.vue` - Variable input form
- `ui/src/components/templates/TemplateApplyDialog.vue` - Apply to device

### Step 5: Add Routes and Navigation

**File**: `ui/src/main.ts`

Add routes:
- `/templates` - Template list
- `/templates/:id` - Template detail
- `/templates/examples` - Example browser

Add to navigation menu under a new "Configuration" dropdown or as sub-item.

## Backend Endpoints (8 total)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/config/templates` | List templates |
| POST | `/api/v1/config/templates` | Create template |
| PUT | `/api/v1/config/templates/{id}` | Update template |
| DELETE | `/api/v1/config/templates/{id}` | Delete template |
| POST | `/api/v1/configuration/preview-template` | Preview rendering |
| POST | `/api/v1/configuration/validate-template` | Validate syntax |
| POST | `/api/v1/configuration/templates` | Save template |
| GET | `/api/v1/configuration/template-examples` | Get examples |

## Related Tasks

- **342**: Device Configuration UI - uses templates for application
- **344**: Typed Configuration UI - integrates with template system
- **352**: Schema-Driven Form Component - can use for template editing

## Dependencies

- **After**: Task 342 (Device Configuration UI) - builds on configuration foundation
- **Optional**: Task 352 (Schema-Driven Form Component) - can use for template editing

## Validation

```bash
# Run frontend tests
npm run test

# Run E2E tests
npm run test:e2e -- --grep "template"

# Type checking
npm run type-check
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update API coverage statistics in Section 4.2
- Move endpoints from "Unused" to "Used" in Section 4.3/4.4

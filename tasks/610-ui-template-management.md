# UI: Template Management

**Priority**: MEDIUM
**Status**: not-started
**Effort**: TBD (separate agent)
**Depends On**: 608

## Context

Implement the UI for managing configuration templates. This task is to be implemented by a separate agent focused on frontend work.

The backend API endpoints from task 608 must be available before starting this task.

## Scope

### Template List Page (`/templates`)
- List all templates with filtering by scope (global, group, device_type)
- Search by name
- Show template count per scope
- Quick actions: Edit, Delete, Duplicate

### Template Create/Edit Form
- Name and description fields
- Scope selector (global, group, device_type)
- Device type selector (when scope = device_type, required)
- Configuration editor:
  - Form-based editing for known fields
  - JSON editor fallback for advanced users
  - Validation on save
  - Clear indication of which fields are set vs. inherited (nil = inherit)

### Template Detail Page (`/templates/:id`)
- Show template metadata
- Show configuration with field labels
- List devices using this template
- "Edit" and "Delete" buttons
- Impact preview when editing (which devices affected)

### Device Assignment (from template view)
- "Assign to Devices" action
- Device selector (multi-select with search)
- Show current device count

## UI Requirements

- Use existing UI patterns and components
- Consistent with other management pages (notifications, plugins, etc.)
- Schema-driven form for template config editing
- Show affected devices count before destructive actions
- Confirmation dialogs for delete

## API Endpoints Used

```
GET    /api/v1/config/templates
POST   /api/v1/config/templates
GET    /api/v1/config/templates/{id}
PUT    /api/v1/config/templates/{id}
DELETE /api/v1/config/templates/{id}
```

## Mockups/Wireframes

### Template List
```
┌─────────────────────────────────────────────────────────────┐
│ Configuration Templates                    [+ New Template] │
├─────────────────────────────────────────────────────────────┤
│ Scope: [All ▼]  Search: [_______________]                   │
├─────────────────────────────────────────────────────────────┤
│ Name              │ Scope       │ Device Type │ Devices │   │
│───────────────────│─────────────│─────────────│─────────│───│
│ Global MQTT       │ global      │ -           │ 17      │ ⋮ │
│ Office Settings   │ group       │ -           │ 5       │ ⋮ │
│ Plug Defaults     │ device_type │ SHPLG-S     │ 5       │ ⋮ │
└─────────────────────────────────────────────────────────────┘
```

### Template Edit Form
```
┌─────────────────────────────────────────────────────────────┐
│ Edit Template: Global MQTT                                  │
├─────────────────────────────────────────────────────────────┤
│ Name:        [Global MQTT_____________]                     │
│ Description: [MQTT settings for all devices___]             │
│ Scope:       [Global ▼]                                     │
├─────────────────────────────────────────────────────────────┤
│ Configuration                                               │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ MQTT Settings                                    [−]    │ │
│ │   Enable:    [✓]                                        │ │
│ │   Server:    [mqtt.local_____________]                  │ │
│ │   Port:      [1883___]                                  │ │
│ │   User:      [iot_____]                                 │ │
│ │   Password:  [••••••__]                                 │ │
│ └─────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Location Settings                                [+]    │ │
│ └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│ ⚠ This change will affect 17 devices                       │
│                                    [Cancel]  [Save Changes] │
└─────────────────────────────────────────────────────────────┘
```

## Files to Create/Modify

- `ui/src/pages/TemplatesPage.vue` (modify existing or replace)
- `ui/src/pages/TemplateDetailPage.vue` (modify or replace)
- `ui/src/pages/TemplateEditPage.vue` (NEW)
- `ui/src/components/TemplateForm.vue` (NEW)
- `ui/src/components/ConfigEditor.vue` (NEW - reusable config form)
- `ui/src/stores/templates.ts` (modify)
- `ui/src/api/templates.ts` (modify)

## Notes

This task will be handed off to a frontend-focused agent with:
- API endpoints from task 608 available and documented
- Backend running locally for testing
- Design patterns from existing UI as reference

The existing `TemplatesPage.vue` and related components can serve as starting point, but may need significant refactoring to work with the new API.

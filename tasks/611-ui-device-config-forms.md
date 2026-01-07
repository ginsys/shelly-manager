# UI: Device Config Forms

**Priority**: MEDIUM
**Status**: completed
**Effort**: 6 hours
**Depends On**: 608
**Completed**: 2026-01-08

## Context

Implement form-based device configuration editing, replacing the current raw JSON display. This task is to be implemented by a separate agent focused on frontend work.

The backend API endpoints from task 608 must be available before starting this task.

## Scope

### Device Configuration Page (`/devices/:id/config`)

Replace raw JSON view with structured form:

1. **Template Inheritance Display**
   - Show which templates are applied (in order)
   - Allow reordering templates
   - Add/remove template assignment

2. **Configuration Form**
   - Form fields derived from device type schema
   - Grouped by category (System, Network, MQTT, etc.)
   - Visual indication of value source per field:
     - 🌍 Global template
     - 🏷️ Group template
     - 📦 Device-type template
     - ✏️ Device override
   - Expand/collapse sections

3. **Override Management**
   - Edit field → creates device override
   - "Reset to template" button per field
   - Warning when overriding template value
   - Clear visual distinction for overridden fields

4. **Apply Workflow**
   - "Pending changes" indicator
   - Preview changes before applying
   - Apply button with confirmation
   - Progress indicator during apply
   - Success/failure feedback

5. **Config Status Display**
   - Applied ✓ / Pending ⏳ / Drift ⚠️
   - Last applied timestamp
   - Verify button to check device matches desired

## UI Requirements

- Use existing SchemaForm component where applicable
- Consistent with other form-based pages
- Responsive design
- Loading states for async operations
- Clear error messages

## API Endpoints Used

```
GET    /api/v1/devices/{id}/templates
PUT    /api/v1/devices/{id}/templates
GET    /api/v1/devices/{id}/overrides
PUT    /api/v1/devices/{id}/overrides
PATCH  /api/v1/devices/{id}/overrides
GET    /api/v1/devices/{id}/desired-config
POST   /api/v1/devices/{id}/config/apply
GET    /api/v1/devices/{id}/config/status
POST   /api/v1/devices/{id}/config/verify
```

## Mockups/Wireframes

### Device Config Page
```
┌─────────────────────────────────────────────────────────────┐
│ Device: Kitchen Plug (SHPLG-S)                              │
│ Status: ⏳ Pending changes                    [Apply Config] │
├─────────────────────────────────────────────────────────────┤
│ Templates Applied (in order):                               │
│ ┌───────────────────────────────────────────────────────┐   │
│ │ 1. 🌍 Global MQTT        [↑] [↓] [×]                  │   │
│ │ 2. 📦 SHPLG-S Defaults   [↑] [↓] [×]                  │   │
│ └───────────────────────────────────────────────────────┘   │
│                                      [+ Add Template]       │
├─────────────────────────────────────────────────────────────┤
│ Configuration                                               │
│                                                             │
│ ▼ System Settings                                           │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ Name:         [Kitchen Plug_____] ✏️ [Reset]        │   │
│   │ Eco Mode:     [✓]                 🌍               │   │
│   │ Discoverable: [✓]                 🌍               │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                             │
│ ▼ MQTT Settings                                             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ Enable:    [✓]                    🌍               │   │
│   │ Server:    [mqtt.local________]   🌍               │   │
│   │ Port:      [1883__]               🌍               │   │
│   │ User:      [kitchen_plug______]   ✏️ [Reset]        │   │
│   │ Password:  [••••••]               ✏️ [Reset]        │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                             │
│ ▶ Network Settings (click to expand)                        │
│ ▶ Cloud Settings                                            │
│ ▶ Switch Settings                                           │
│ ▶ LED Settings                                              │
└─────────────────────────────────────────────────────────────┘
```

### Apply Confirmation Dialog
```
┌─────────────────────────────────────────────────────────────┐
│ Apply Configuration Changes                                 │
├─────────────────────────────────────────────────────────────┤
│ The following changes will be applied to the device:        │
│                                                             │
│ ✎ system.name: "Kitchen Plug" (was: "shellyplug-s-ABC123") │
│ ✎ mqtt.user: "kitchen_plug" (was: "iot")                   │
│ ✎ mqtt.password: ******* (changed)                         │
│                                                             │
│ ⚠️ This will modify the physical device configuration.     │
│                                                             │
│                            [Cancel]  [Apply to Device]      │
└─────────────────────────────────────────────────────────────┘
```

## Field Source Legend

| Icon | Meaning |
|------|---------|
| 🌍 | Value from Global template |
| 🏷️ | Value from Group template |
| 📦 | Value from Device-type template |
| ✏️ | Device override (user-set) |
| ⚙️ | Default value (no template) |

## Files to Create/Modify

- `ui/src/pages/DeviceConfigPage.vue` (major refactor or replace)
- `ui/src/components/DeviceConfigForm.vue` (NEW)
- `ui/src/components/ConfigField.vue` (NEW - single field with source indicator)
- `ui/src/components/TemplateAssignment.vue` (NEW)
- `ui/src/components/ApplyConfigDialog.vue` (NEW)
- `ui/src/stores/deviceConfig.ts` (modify)
- `ui/src/api/deviceConfig.ts` (modify)

## Implementation Notes

### Field Schema

The form needs to know:
- Field path (e.g., "mqtt.server")
- Field type (string, number, boolean, enum)
- Validation rules
- Human-readable label
- Help text

This could come from:
1. Static schema definitions in frontend
2. Schema endpoint from backend
3. Combination (backend provides schema, frontend renders)

### Source Tracking

The `/desired-config` endpoint returns source tracking:
```json
{
  "config": { "mqtt": { "server": "mqtt.local" } },
  "sources": { "mqtt.server": "Global MQTT" }
}
```

Use this to display source icons and enable "Reset to template" buttons.

## Notes

This task will be handed off to a frontend-focused agent with:
- API endpoints from task 608 available and documented
- Backend running locally for testing
- Existing DeviceConfigPage.vue as reference
- Design patterns from other form pages

This is the most complex UI task - consider breaking into subtasks:
1. Template assignment UI
2. Config form with source display
3. Override management (edit/reset)
4. Apply workflow

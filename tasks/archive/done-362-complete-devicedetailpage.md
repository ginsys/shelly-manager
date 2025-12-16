# Complete DeviceDetailPage

**Priority**: MEDIUM - Feature Completion
**Status**: not-started
**Effort**: 10 hours (with 1.3x buffer)

## Context

DeviceDetailPage is currently a stub implementation (85 lines) that only displays basic device information. This task expands it to a full implementation with status, energy metrics, configuration viewer, and control actions.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 5 (Technical Debt)
**Phase 8 Reference**: Section 5 - Devices detail

## Success Criteria

- [ ] Device status polling with auto-refresh
- [ ] Energy metrics display (for power-metering devices)
- [ ] Configuration viewer (read-only, links to full config)
- [ ] Control actions (on/off/restart)
- [ ] Edit device information
- [ ] Device capabilities display
- [ ] Navigation to configuration pages
- [ ] Loading and error states
- [ ] E2E tests for device detail page
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Expand Page Structure

**File**: `ui/src/pages/DeviceDetailPage.vue`

Create comprehensive layout with sections:
- Header with device name, type, status indicator
- Quick actions toolbar (control, edit, delete)
- Status section with live data
- Energy metrics section (conditional)
- Configuration summary with link to full config
- Capabilities section
- Last seen / connection info

### Step 2: Add Status Polling

Implement status polling with configurable interval:

```typescript
const { data: status, isLoading, error, refetch } = useQuery({
  queryKey: ['device-status', deviceId],
  queryFn: () => getDeviceStatus(deviceId.value),
  refetchInterval: 5000 // 5 second refresh
})
```

Or use WebSocket from Task 251 for real-time updates.

### Step 3: Add Energy Metrics Section

**Component**: `ui/src/components/devices/EnergyMetrics.vue`

Display energy data:
- Current power (W)
- Voltage (V)
- Current (A)
- Total energy (kWh)
- Chart showing power over time (ECharts)

Conditional rendering based on device capabilities.

### Step 4: Add Control Actions

**Component**: `ui/src/components/devices/DeviceControls.vue`

Control buttons:
- Power On/Off toggle
- Restart button
- Custom actions based on device type

Confirmation dialogs for destructive actions.

### Step 5: Add Configuration Summary

**Component**: `ui/src/components/devices/ConfigSummary.vue`

Display:
- Key configuration values
- Drift status indicator
- "View Full Configuration" button
- "Edit Configuration" button

### Step 6: Add Edit Mode

**Component**: `ui/src/components/devices/DeviceEditDialog.vue`

Editable fields:
- Device name
- Custom settings
- Notes/description

### Step 7: Add Capabilities Display

**Component**: `ui/src/components/devices/CapabilitiesList.vue`

Show device capabilities:
- Relay, Dimmer, Roller, Power Metering, etc.
- Feature icons
- Expandable details

### Step 8: Add Navigation Links

Add navigation to related pages:
- Configuration: `/devices/{id}/config`
- History: `/devices/{id}/config/history`
- Drift: `/devices/{id}/drift`

## Dependencies

- **Depends on**: Task 341 (Device Management API) - provides endpoints for status, energy, control
- **After**: Task 342 (Device Configuration UI) - provides config pages to link to
- **Optional**: Task 251 (Reusable WebSocket Client) - enables real-time updates

## Page Structure

```
┌─────────────────────────────────────────────────────────────┐
│  Device Name                    [Edit] [Control ▼] [Delete] │
│  Type: SHSW-1  •  Status: Online  •  Last seen: 2 min ago  │
├─────────────────────────────────────────────────────────────┤
│  Status                                                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                     │
│  │ Power On │ │ Temp 32°C│ │ WiFi -45 │                     │
│  └──────────┘ └──────────┘ └──────────┘                     │
├─────────────────────────────────────────────────────────────┤
│  Energy Metrics                        [View History]        │
│  Power: 45W  Voltage: 230V  Energy: 1.2 kWh today          │
│  [Power consumption chart over time]                         │
├─────────────────────────────────────────────────────────────┤
│  Configuration                         [View Full Config]    │
│  ┌─ Drift Status: OK ────────────────────────────────┐      │
│  │ Name: Kitchen Light  •  AP Password: ******       │      │
│  │ MQTT: Enabled  •  Eco Mode: Off                   │      │
│  └───────────────────────────────────────────────────┘      │
├─────────────────────────────────────────────────────────────┤
│  Capabilities                                                │
│  [Relay] [Power Metering] [WiFi] [Bluetooth]                │
└─────────────────────────────────────────────────────────────┘
```

## Validation

```bash
# Run E2E tests
npm run test:e2e -- --grep "device detail"

# Run type checking
npm run type-check

# Manual testing
# - Navigate to various device types
# - Test control actions
# - Verify status polling
# - Check energy metrics
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update Section 5 to mark stub implementation as complete
- Update line count in Appendix: File Reference
- Add DeviceDetailPage to Section 2.2 Pages overview

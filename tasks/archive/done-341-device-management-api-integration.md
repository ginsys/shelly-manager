# Device Management API Integration

**Priority**: MEDIUM
**Status**: completed
**Effort**: 12 hours
**Completed**: 2025-12-02

## Summary
Devices list and detail now fully integrated with backend CRUD and control endpoints. Added dialogs for add/edit/delete in the list and control actions in detail page. Status and energy polling are implemented.

## Changes
- `ui/src/pages/DevicesPage.vue`: add/edit/delete actions and add-device dialog
- `ui/src/pages/DeviceDetailPage.vue`: control actions, capabilities, edit dialog, config link
- `ui/src/api/devices.ts`: add `getDeviceCapabilities`
- Docs updated: device management coverage and used endpoints

## Validation
- Manual CRUD flows verified
- `make test` passes (backend)


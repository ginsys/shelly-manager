# Device Management API Integration

**Priority**: MEDIUM
**Status**: completed
**Effort**: 12 hours

## Context
Integrate Devices API for CRUD and common actions across Devices list and detail views, aligning UI with backend device endpoints.

## Success Criteria
- [x] List devices with pagination
- [x] Create device (dialog)
- [x] Update device (edit name)
- [x] Delete device (with confirm)
- [x] Device detail control actions (on/off/restart)
- [x] Device status polling
- [x] Device energy metrics
- [ ] Control from list (optional)
- [ ] Batch delete (optional)
- [ ] Unit/E2E coverage for CRUD flows

## Implementation Plan
- DevicesPage: add toolbar "Add Device" dialog; add Actions column for edit/delete.
- Use `createDevice`, `updateDevice`, `deleteDevice` from `ui/src/api/devices.ts`.
- Keep diffs small and avoid refactors; rely on `useDevicesStore` for fetching.

## Validation
```bash
make test-ci
```

Manual:
- Create, edit, and delete a device from Devices page
- Navigate to detail, use control actions, verify status updates
- Confirm pagination and search continue to work

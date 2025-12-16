# Device-Type Specific Configuration Forms

**Priority**: HIGH
**Status**: not-started
**Effort**: 6 hours

## Context
User feedback: "the configuration details are just a json blob, I would expect a proper edit form that matches the device type; pretty sure that was architected as such"

User clarification: "the design was established as such, but i don't know to what extend it's implemented in the backend either; there was also some design about templating to configure common settings (like e.g. mqtt settings)"

The backend returns properly structured configuration data (2149 bytes), but the frontend displays it as raw JSON instead of a device-type-specific form.

## Evidence
- Backend returns 200 OK with structured config data
- Frontend shows "Failed to get current device configuration" error
- Falls back to raw JSON display

## Success Criteria
- [ ] Investigate existing device-type schema architecture in backend
- [ ] Investigate templating design for common settings (MQTT, etc.)
- [ ] Determine extent of backend implementation
- [ ] Implement form rendering based on device type (Shelly 1, Shelly PM, etc.)
- [ ] Map configuration fields to appropriate input controls
- [ ] Implement template application for common settings
- [ ] Validate configuration changes before submission
- [ ] Show human-readable labels instead of JSON keys
- [ ] Run `make test-ci` to ensure no regressions

## Investigation Phase
Before implementation, review:
1. Backend schema definitions in `internal/configuration/`
2. Templating system design and implementation status
3. Frontend form generation capabilities
4. Current JSON display fallback mechanism

## Files to Investigate
- `web/src/views/DeviceEditPage.vue` (or similar)
- `web/src/components/device/` (device-specific components)
- `internal/configuration/` (backend schema definitions)
- `internal/configuration/template*.go` (templating system)

## Validation
```bash
# Test configuration editing for different device types
# Verify form renders appropriately for each type
# Test template application for MQTT settings

make test-ci
npm run test
```

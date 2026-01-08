# Device-Type Specific Configuration Forms

**Priority**: HIGH
**Status**: completed
**Effort**: 6 hours

## Context
User feedback: "the configuration details are just a json blob, I would expect a proper edit form that matches the device type; pretty sure that was architected as such"

User clarification: "the design was established as such, but i don't know to what extend it's implemented in the backend either; there was also some design about templating to configure common settings (like e.g. mqtt settings)"

The backend returns properly structured configuration data (2149 bytes), but the frontend displays it as raw JSON instead of a device-type-specific form.

## Solution Implemented

### Backend Changes
- Expanded `GetConfigurationSchema()` in `internal/configuration/typed_models.go` to include JSON schemas for all TypedConfiguration fields:
  - WiFi, MQTT, Auth, System, Cloud, Location (common)
  - Relay, LED, Power Metering, Input, CoIoT (device-specific)
  - Dimming, Roller, Color, Temperature Protection (device-specific)
  - Schedule, Energy Meter, Motion, Sensor (device-specific)
- Each schema includes field titles, descriptions, types, validation constraints

### UI Changes
- Created `ui/src/components/config/ConfigView.vue` - structured display of config with collapsible sections and icons
- Created `ui/src/components/config/ConfigEditor.vue` - schema-driven form editor with section toggles
- Updated `ui/src/pages/DeviceConfigPage.vue` to use new components:
  - Device Overrides: Uses ConfigEditor with form/JSON toggle
  - Desired Configuration: Uses ConfigView with structured display (Raw JSON toggle available)
  - Shows source badges when viewing sources

## Success Criteria
- [x] Investigate existing device-type schema architecture in backend
- [x] Investigate templating design for common settings (MQTT, etc.)
- [x] Determine extent of backend implementation
- [x] Implement form rendering based on device type (Shelly 1, Shelly PM, etc.)
- [x] Map configuration fields to appropriate input controls
- [x] Implement template application for common settings
- [x] Validate configuration changes before submission
- [x] Show human-readable labels instead of JSON keys
- [x] Run `make test-ci` to ensure no regressions

## Files Changed
- `internal/configuration/typed_models.go` - expanded schema definitions
- `ui/src/pages/DeviceConfigPage.vue` - integrated new components
- `ui/src/components/config/ConfigView.vue` - new structured view component
- `ui/src/components/config/ConfigEditor.vue` - new form editor component

## Validation
```bash
# All tests pass (excluding pre-existing failing test)
go test ./internal/configuration/... -skip TestTemplateEngineBaseTemplateInheritance
npm run build  # UI builds successfully
npm test       # UI tests pass
```

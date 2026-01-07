# Configuration Management System - Design & Foundation

**Priority**: HIGH
**Status**: completed
**Effort**: 8 hours
**Completed**: 2026-01-06

## Context

The current configuration system has several issues:
1. Device config is displayed as raw JSON - no form-based editing
2. Template system uses variable substitution (`{{.Device.MAC}}`) which is overcomplicated
3. No clear separation between imported config, desired config, and device overrides
4. Templates cannot be hierarchically applied (global → group → device-type → device)

This task establishes the foundation for a redesigned configuration system where:
- Schemas define valid settings per device type
- Templates are partial configs applied in hierarchy
- Device overrides are tracked separately
- Desired config is computed and cached
- Config is only converted to/from API format at import/export boundaries

## Design Decisions (Confirmed)

1. **Storage Model**:
   - `imported_config`: Raw API JSON (read-only snapshot from device)
   - `templates`: Internal struct (JSON in DB) - partial configs
   - `device.overrides`: Internal struct (JSON in DB) - device-specific values
   - `device.desired_config`: Internal struct (JSON in DB) - computed, cached
   - `device.config_applied`: Boolean - whether desired matches device

2. **Internal Struct**: Normalized, generation-agnostic, capability-based
   - All configurable fields use pointers (nil = inherit from template)
   - Converters translate to/from Gen1/Gen2 API formats

3. **Template Hierarchy**: Global → Group → Device-Type → Device Overrides
   - Later values override earlier; nil = inherit
   - Template assignment is manual via `device.template_ids` array
   - Tags on devices are simple labels for filtering/bulk operations (independent of templates)

4. **Template Assignment**: Stored on device as ordered list of template IDs

5. **Change Propagation**: Immediate recompute of desired_config, mark config_applied=false

## Success Criteria

- [ ] Define normalized internal config structs in `internal/configuration/normalized_config.go`
- [ ] Define all capability settings structs (System, Network, MQTT, Auth, Switch, etc.)
- [ ] All pointer fields for nil = inherit semantics
- [ ] Struct tags for JSON serialization with omitempty
- [ ] Unit tests for JSON marshal/unmarshal round-trip
- [ ] Documentation of field semantics in comments

## Implementation

### Normalized Config Structure

```go
// DeviceConfiguration is the normalized internal representation
// Used for templates, overrides, and desired_config
type DeviceConfiguration struct {
    // Common settings (all devices)
    System   *SystemSettings   `json:"system,omitempty"`
    Network  *NetworkSettings  `json:"network,omitempty"`
    Cloud    *CloudSettings    `json:"cloud,omitempty"`
    MQTT     *MQTTSettings     `json:"mqtt,omitempty"`
    Auth     *AuthSettings     `json:"auth,omitempty"`
    Location *LocationSettings `json:"location,omitempty"`
    CoIoT    *CoIoTSettings    `json:"coiot,omitempty"`
    
    // Capability-specific (present if device has capability)
    Switches []SwitchSettings  `json:"switches,omitempty"`
    Inputs   []InputSettings   `json:"inputs,omitempty"`
    Meters   []MeterSettings   `json:"meters,omitempty"`
    Roller   *RollerSettings   `json:"roller,omitempty"`
    Dimmer   *DimmerSettings   `json:"dimmer,omitempty"`
    LED      *LEDSettings      `json:"led,omitempty"`
}

// All pointer fields for nil = "inherit" semantics
type MQTTSettings struct {
    Enable       *bool   `json:"enable,omitempty"`
    Server       *string `json:"server,omitempty"`
    Port         *int    `json:"port,omitempty"`
    User         *string `json:"user,omitempty"`
    Password     *string `json:"password,omitempty"`
    ClientID     *string `json:"client_id,omitempty"`
    TopicPrefix  *string `json:"topic_prefix,omitempty"`
    CleanSession *bool   `json:"clean_session,omitempty"`
    Retain       *bool   `json:"retain,omitempty"`
    QoS          *int    `json:"qos,omitempty"`
    KeepAlive    *int    `json:"keep_alive,omitempty"`
    UpdatePeriod *int    `json:"update_period,omitempty"`
}

// Similar pattern for all other settings structs...
```

### Key Differences from Existing typed_models.go

The existing `typed_models.go` uses non-pointer fields which cannot distinguish "not set" from "set to zero". The new normalized config uses pointers throughout to support template inheritance.

## Files to Create/Modify

- `internal/configuration/normalized_config.go` (NEW)
- `internal/configuration/normalized_config_test.go` (NEW)

## Validation

```bash
make test-ci
go test -v ./internal/configuration/...
```

## Notes

This task creates the foundation. Subsequent tasks build on this:
- 602/603: Converters from Gen1 API format
- 604: Merge engine for template hierarchy
- 605: Database schema updates

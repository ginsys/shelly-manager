# Gen1 API Converters (SHSW-1, SHSW-PM, SHIX3-1)

**Priority**: HIGH
**Status**: completed
**Effort**: 6 hours
**Completed**: 2026-01-06
**Depends On**: 602

## Context

Extend Gen1 converter to support remaining device types in the test network:
- **SHSW-1**: Simple 1-relay switch (8 devices) - no power metering
- **SHSW-PM**: 1-relay switch with power metering (3 devices)
- **SHIX3-1**: 3-input controller (1 device) - no relay output, 3 inputs

## Device Capabilities

| Model | Switches | Meters | Inputs | Roller | Notes |
|-------|----------|--------|--------|--------|-------|
| SHPLG-S | 1 | 1 | 0 | No | Done in 602 |
| SHSW-1 | 1 | 0 | 1 | No | Simple relay |
| SHSW-PM | 1 | 1 | 1 | No | Relay + meter |
| SHIX3-1 | 0 | 0 | 3 | No | Input-only |

## Field Mapping Differences

### SHSW-1 (vs SHPLG-S)
- No `max_power` field (no meter)
- Has `relays[0]` like SHPLG-S
- Has single `inputs[0]` (button/switch input)

### SHSW-PM (vs SHPLG-S)
- Similar to SHPLG-S (relay + meter)
- Has `meters[0]` with power/energy data
- Has single `inputs[0]`

### SHIX3-1 (Input Controller)
- No `relays` array
- Has `inputs[0]`, `inputs[1]`, `inputs[2]`
- Each input has separate configuration
- Used for scenes/actions triggering

## Success Criteria

- [ ] SHSW-1 FromAPIConfig and ToAPIConfig
- [ ] SHSW-PM FromAPIConfig and ToAPIConfig
- [ ] SHIX3-1 FromAPIConfig and ToAPIConfig
- [ ] Test fixtures from actual device configs in database
- [ ] Round-trip tests for each device type
- [ ] Handle device-specific fields appropriately
- [ ] Graceful handling of missing capabilities

## Implementation

Extend `Gen1Converter` to handle device-specific logic:

```go
func (c *Gen1Converter) FromAPIConfig(apiJSON json.RawMessage, deviceType string) (*DeviceConfiguration, error) {
    config := &DeviceConfiguration{}
    
    // Parse common fields (all devices have these)
    c.parseCommonFields(apiJSON, config)
    
    // Parse device-specific fields based on type
    switch deviceType {
    case "SHPLG-S", "SHSW-PM":
        c.parseRelays(apiJSON, config)
        c.parseMeters(apiJSON, config)
    case "SHSW-1":
        c.parseRelays(apiJSON, config)
        c.parseInputs(apiJSON, config) // Single input
    case "SHIX3-1":
        c.parseInputs(apiJSON, config) // 3 inputs
    default:
        // Fallback: try to parse all known fields
        c.parseAllOptional(apiJSON, config)
    }
    
    return config, nil
}
```

## Files to Modify

- `internal/configuration/gen1_converter.go` (extend)
- `internal/configuration/gen1_converter_test.go` (add tests)

## Files to Create

- `internal/configuration/testdata/shsw_1_config.json` (NEW - test fixture)
- `internal/configuration/testdata/shsw_pm_config.json` (NEW - test fixture)
- `internal/configuration/testdata/shix3_1_config.json` (NEW - test fixture)

## Test Strategy

1. Extract actual configs from database for each device type
2. Test FromAPIConfig produces correct DeviceConfiguration
3. Verify capability arrays have correct length:
   - SHSW-1: 1 switch, 0 meters, 1 input
   - SHSW-PM: 1 switch, 1 meter, 1 input
   - SHIX3-1: 0 switches, 0 meters, 3 inputs
4. Round-trip tests for each type
5. Test that missing capabilities result in nil/empty slices

## Validation

```bash
make test-ci
go test -v ./internal/configuration/... -run TestGen1Converter
```

## Notes

After this task, the Gen1 converter supports all device types in the test network. Gen2 converter can be added later following the same pattern.

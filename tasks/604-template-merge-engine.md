# Template Merge Engine

**Priority**: HIGH
**Status**: completed
**Effort**: 6 hours
**Completed**: 2026-01-06
**Depends On**: 601

## Context

Implement the merge logic that combines templates and device overrides into final desired_config. Templates are applied in hierarchy order:

```
Global Template → Group Templates (by tag) → Device-Type Template → Device Overrides
```

Merge rule: Later non-nil values override earlier values. Nil = "inherit from previous layer".

## Nil Semantics

| Value in Layer | Meaning |
|----------------|---------|
| `nil` (field absent) | Inherit from previous layer |
| Non-nil pointer | Override with this value |
| Pointer to zero value | Explicitly set to zero/empty/false |

This is why all fields in `DeviceConfiguration` are pointers - to distinguish "not set" from "set to zero".

## Success Criteria

- [ ] `MergeConfigurations()` function that takes ordered list of config layers
- [ ] Proper nil handling (nil = inherit, non-nil = override)
- [ ] Deep merge for nested structs (e.g., `MQTT.Server` can be overridden without losing `MQTT.Port`)
- [ ] Slice merge strategy for capabilities (Switches, Inputs, Meters)
- [ ] Source tracking: return map of field path → source name
- [ ] Unit tests for merge precedence
- [ ] Unit tests for partial configs (templates with few fields set)
- [ ] Unit tests for nil vs zero-value distinction

## Implementation

```go
// ConfigLayer represents one layer in the merge hierarchy
type ConfigLayer struct {
    Name   string               // e.g., "global", "office-devices", "SHPLG-S", "device-override"
    Config *DeviceConfiguration
}

// MergeResult contains the merged config and source tracking
type MergeResult struct {
    Config  *DeviceConfiguration
    Sources map[string]string  // field path → source name (e.g., "mqtt.server" → "global")
}

// Engine is the concrete merge implementation.
// It is intentionally stateless.
type Engine struct{}

func (Engine) Merge(layers []ConfigLayer) (*MergeResult, error) {
    return MergeConfigurations(layers)
}

// MergeConfigurations merges configs in priority order (first = lowest, last = highest)
func MergeConfigurations(layers []ConfigLayer) (*MergeResult, error) {
    if len(layers) == 0 {
        return &MergeResult{Config: &DeviceConfiguration{}, Sources: map[string]string{}}, nil
    }
    
    result := &DeviceConfiguration{}
    sources := make(map[string]string)
    
    for _, layer := range layers {
        if layer.Config == nil {
            continue
        }
        mergeLayer(result, layer.Config, sources, layer.Name, "")
    }
    
    return &MergeResult{Config: result, Sources: sources}, nil
}

// mergeLayer recursively merges a config layer into result
func mergeLayer(result, layer interface{}, sources map[string]string, sourceName, pathPrefix string) {
    // Use reflection to iterate over struct fields
    // For each non-nil field in layer, copy to result and record source
}
```

## Slice Merge Strategy

For capability arrays (Switches, Inputs, Meters), merge by index.

Semantics (v1):
- A missing index in a higher-priority layer means "inherit" that index from lower-priority layers.
- There is no concept of "delete"/"remove" a capability entry because the device's capabilities determine the final slice length.
- To clear a value (e.g., empty name), use an explicit pointer to the zero value on the field (`Name: ptr("")`).

If we later need "disable entry" semantics for devices with multiple channels, introduce explicit per-index enable flags rather than truncating slices.

```go
// If layer has Switches[0], merge with result.Switches[0]
// If result.Switches is shorter, extend it
// Individual switch fields merge like other structs
```

Example:
```
Global Template:     Switches[0].AutoOff = 3600
Device Override:     Switches[0].Name = "Kitchen Light"
Result:              Switches[0].AutoOff = 3600, Switches[0].Name = "Kitchen Light"
```

## Source Tracking

The `Sources` map uses dot-notation paths:

```go
sources["mqtt.server"] = "global"
sources["mqtt.user"] = "device-override"
sources["switches.0.name"] = "device-override"
sources["switches.0.auto_off"] = "SHPLG-S"
sources["location.timezone"] = "global"
```

This enables the UI to show "this value comes from Global Template" for each field.

## Files to Create

- `internal/configuration/merge.go` (NEW)
- `internal/configuration/merge_test.go` (NEW)

## Test Cases

1. **Empty layers**: Returns empty config
2. **Single layer**: Returns copy of that layer
3. **Two layers, no overlap**: Both values preserved
4. **Two layers, overlap**: Later wins
5. **Three layers**: Correct precedence (last wins)
6. **Nil vs zero**: `Enable: nil` inherits, `Enable: ptr(false)` overrides to false
7. **Nested struct merge**: Override `MQTT.Server` without losing `MQTT.Port`
8. **Slice merge**: Override `Switches[0].Name` without losing `Switches[0].AutoOff`
9. **Source tracking**: Correct source recorded for each field

## Validation

```bash
make test-ci
go test -v ./internal/configuration/... -run TestMerge
```

## Notes

The merge engine is a pure function with no database or I/O dependencies. This makes it easy to test and reason about. The service layer (task 606) will orchestrate fetching templates and calling the merge engine.

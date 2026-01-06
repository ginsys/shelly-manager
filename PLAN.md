# Plan: Task 603 - Gen1 Converters (Remaining Devices) - RESEARCH COMPLETE

## Research Findings ✅

### Question 1: InputConfig Structure ✅

**Answer:** HYBRID structure - Both top-level AND per-input fields

```go
type InputConfig struct {
    // Top-level defaults (apply to all inputs if not overridden)
    Type             *string
    Mode             *string
    Inverted         *bool
    DebounceTime     *int
    LongPushTime     *int
    MultiPushTime    *int
    SinglePushAction *string
    DoublePushAction *string
    TriplePushAction *string
    LongPushAction   *string
    
    // Per-input specific settings (overrides top-level)
    Inputs           []SingleInputConfig
}

type SingleInputConfig struct {
    ID               int     // Required
    Name             *string
    Type             *string  // Overrides top-level
    Mode             *string  // Overrides top-level
    Inverted         *bool    // Overrides top-level
    SinglePushAction *string  // Overrides top-level
    LongPushAction   *string  // Overrides top-level
}
```

**Mapping Strategy:**
- Gen1 API `inputs[i].*` → `Input.Inputs[i].*` (per-input)
- Top-level Input.* fields are for defaults (not present in Gen1 API)
- For Gen1 conversion, populate only Input.Inputs[] array

### Question 2: SHSW-25 (Shelly 2.5) Inclusion ✅

**Answer:** INCLUDE in Task 603

**Evidence:**
1. ✅ Already in SupportedDeviceTypes() - gen1_converter.go:138
2. ✅ Mentioned throughout codebase (60 occurrences)
3. ✅ Test fixtures exist (ui/tests/fixtures/devices.json)
4. ✅ Has test coverage (configs_test.go, service_test.go)
5. ✅ Simple relay mode (2 relays) - no special roller handling needed yet

**Device Count:** 8 SHSW-1 + 3 SHSW-PM + 1 SHIX3-1 + SHSW-25s in test network

**SHSW-25 Capabilities:**
- 2 relays (relays[0], relays[1]) in relay mode
- 2 power meters (meters[0], meters[1])
- Roller mode exists but defer to later task (Task 603 focuses on relay mode)

**Decision:** Add SHSW-25 relay mode support (simple extension of SHSW-PM logic)

### Question 3: Test Fixture Source ✅

**Answer:** CRAFT realistic fixtures based on Gen1 API docs + existing patterns

**Rationale:**
1. ❌ No real Gen1 /settings responses found in codebase
2. ✅ Have base_gen1.json template as reference
3. ✅ Have SHPLG-S fixture from Task 602 as pattern
4. ✅ Shelly Gen1 API docs available (internal/shelly/gen1/devices.md)
5. ✅ Test fixtures just need to be valid/realistic, not real device dumps

**Approach:**
- Use SHPLG-S fixture as base
- Modify for device-specific differences:
  - SHSW-1: Remove max_power, led_* fields
  - SHSW-PM: Same as SHPLG-S but no led_* fields
  - SHSW-25: Add relays[1], meters[1], 2 inputs
  - SHIX3-1: Remove relays[], add inputs[0-2]

## Updated Implementation Plan

### Phase 1: Add Input Support (1h)

**Tasks:**
- [ ] Add `convertInput()` method - map Gen1 inputs[i] to Input.Inputs[i]
- [ ] Add `exportInput()` method - reverse mapping
- [ ] Call convertInput in FromAPIConfig (all devices have inputs)
- [ ] Call exportInput in ToAPIConfig

**Input Mapping:**
```
Gen1 inputs[i].* → Input.Inputs[i].*
inputs[i].name → Inputs[i].Name
inputs[i].type → Inputs[i].Type
inputs[i].mode → Inputs[i].Mode (e.g., "button", "switch")
inputs[i].inverted → Inputs[i].Inverted
inputs[i].single_push_action → Inputs[i].SinglePushAction
inputs[i].long_push_action → Inputs[i].LongPushAction
```

### Phase 2: Device-Specific Logic (1h)

**Tasks:**
- [ ] Add device checks in `convertPowerMetering()`:
  - Skip for SHSW-1 (no meter)
  - Skip for SHIX3-1 (no meter)
  - Process for SHPLG-S, SHSW-PM, SHSW-25
  
- [ ] Add device checks in `convertLED()`:
  - Only for SHPLG-S, SHPLG-1 (plugs have LED ring)
  - Skip for SHSW-1, SHSW-PM, SHSW-25, SHIX3-1
  
- [ ] Add device checks in `convertRelay()`:
  - Skip for SHIX3-1 (input-only, no relays)
  - Single relay: SHSW-1, SHSW-PM, SHPLG-S
  - Dual relay: SHSW-25 (relays[0], relays[1])
  
- [ ] Update `SupportedDeviceTypes()`:
  - Already has SHSW-25 ✅
  - Verify SHSW-1, SHSW-PM, SHIX3-1 listed

### Phase 3: Test Fixtures (1h)

**Create 4 fixtures:**
- [ ] `testdata/shsw_1_settings.json` - Based on SHPLG-S, remove max_power, led_*
- [ ] `testdata/shsw_pm_settings.json` - Based on SHPLG-S, remove led_*
- [ ] `testdata/shsw_25_settings.json` - Based on SHPLG-S, add relays[1], meters[1]
- [ ] `testdata/shix3_1_settings.json` - Based on base_gen1.json, add inputs[0-2], remove relays

**SHIX3-1 specific:**
```json
{
  "inputs": [
    {
      "name": "Input 1",
      "type": "button",
      "mode": "momentary",
      "inverted": false,
      "single_push_action": "on",
      "long_push_action": "off"
    },
    {
      "name": "Input 2",
      "type": "button",
      "mode": "momentary",
      "inverted": false,
      "single_push_action": "toggle",
      "long_push_action": "off"
    },
    {
      "name": "Input 3",
      "type": "button",
      "mode": "momentary",
      "inverted": false,
      "single_push_action": "off",
      "long_push_action": "on"
    }
  ]
}
```

### Phase 4: Comprehensive Tests (2.5h)

**12 new tests (3 per device × 4 devices):**

**SHSW-1:**
- [ ] TestGen1Converter_FromAPIConfig_SHSW1
- [ ] TestGen1Converter_ToAPIConfig_SHSW1
- [ ] TestGen1Converter_RoundTrip_SHSW1
- Verify: 1 relay, NO power metering, NO LED, 1 input

**SHSW-PM:**
- [ ] TestGen1Converter_FromAPIConfig_SHSWPM
- [ ] TestGen1Converter_ToAPIConfig_SHSWPM
- [ ] TestGen1Converter_RoundTrip_SHSWPM
- Verify: 1 relay, YES power metering, NO LED, 1 input

**SHSW-25:**
- [ ] TestGen1Converter_FromAPIConfig_SHSW25
- [ ] TestGen1Converter_ToAPIConfig_SHSW25
- [ ] TestGen1Converter_RoundTrip_SHSW25
- Verify: 2 relays, YES power metering (2 meters), NO LED, 2 inputs

**SHIX3-1:**
- [ ] TestGen1Converter_FromAPIConfig_SHIX31
- [ ] TestGen1Converter_ToAPIConfig_SHIX31
- [ ] TestGen1Converter_RoundTrip_SHIX31
- Verify: NO relays, NO power metering, NO LED, 3 inputs

### Phase 5: Validation & Documentation (0.5h)

**Tasks:**
- [ ] Verify SupportedDeviceTypes() includes all 6: SHPLG-S, SHPLG-1, SHSW-1, SHSW-PM, SHSW-25, SHIX3-1
- [ ] Run: `go test -v ./internal/configuration -run TestGen1Converter`
- [ ] Run: `make test-ci`
- [ ] Verify no regressions in SHPLG-S tests
- [ ] Document device-specific handling in code comments
- [ ] Update task 603 markdown to completed

## Expected Changes

**Files Modified:**
- `internal/configuration/gen1_converter.go` (+200 lines)
  - Add `convertInput()` / `exportInput()` (+80 lines)
  - Add device-type conditionals (+30 lines)
  - Handle SHSW-25 dual relays (+40 lines)
  - Handle SHIX3-1 3 inputs (+50 lines)
  
**Files Created:**
- `internal/configuration/testdata/shsw_1_settings.json` (~80 lines)
- `internal/configuration/testdata/shsw_pm_settings.json` (~90 lines)
- `internal/configuration/testdata/shsw_25_settings.json` (~110 lines)
- `internal/configuration/testdata/shix3_1_settings.json` (~70 lines)

**Tests Added:**
- 12 new test functions (+600 lines)
- Expected coverage: maintain 90%+

## Success Criteria

- [x] Research questions answered
- [ ] Gen1Converter supports SHSW-1, SHSW-PM, SHSW-25, SHIX3-1
- [ ] Input support implemented (all devices have inputs)
- [ ] Device-specific fields handled correctly
- [ ] 12 comprehensive tests added (3 per device)
- [ ] All tests pass with 90%+ coverage
- [ ] Round-trip tests pass for all devices
- [ ] `make test-ci` passes
- [ ] No regressions in SHPLG-S tests

## Time Estimate (Updated)

- Phase 1: 1h (Input support)
- Phase 2: 1h (Device-specific logic)
- Phase 3: 1h (4 test fixtures instead of 3)
- Phase 4: 2.5h (12 tests instead of 9)
- Phase 5: 0.5h (Validation)
- **Total: 6h** ✅ (still within task estimate)

## Key Implementation Notes

1. **InputConfig mapping:** Only populate Input.Inputs[] array, not top-level defaults
2. **SHSW-25:** Handle as dual-relay variant (relays[0-1], meters[0-1])
3. **SHIX3-1:** Skip relay/power/LED conversion entirely
4. **All devices:** Have at least 1 input (SHSW-1/PM/PLGS=1, SHSW-25=2, SHIX3=3)

## Next: Start Phase 1

Implement convertInput() and exportInput() methods.

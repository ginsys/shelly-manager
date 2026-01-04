# Configuration Apply & Verify Flow

**Priority**: HIGH
**Status**: not-started
**Effort**: 6 hours
**Depends On**: 602, 603, 606

## Context

Implement the workflow to apply desired_config to a physical device and verify it was applied correctly:

1. Convert desired_config (internal format) → API format (Gen1/Gen2)
2. Export to device via Shelly API
3. Import from device to get actual state
4. Compare imported vs desired
5. Update config_applied flag based on result

## Workflow

```
┌─────────────────────────────────────────────────────────────────┐
│                    APPLY CONFIGURATION                          │
└─────────────────────────────────────────────────────────────────┘

1. Get desired_config from database
                │
                ▼
2. Convert internal → Gen1/Gen2 API format
   (using converters from 602/603)
                │
                ▼
3. Export to device via Shelly API
   - POST to /settings (Gen1)
   - RPC calls (Gen2)
                │
                ▼
4. Wait for device to apply settings
   (some settings require reboot)
                │
                ▼
5. Import current config from device
   - GET /settings (Gen1)
   - Shelly.GetConfig (Gen2)
                │
                ▼
6. Convert imported API → internal format
                │
                ▼
7. Compare imported vs desired
   - Field-by-field comparison
   - Generate diff report
                │
                ▼
8. Update device record
   - config_applied = true (if match)
   - config_applied = false (if mismatch)
   - Store imported_config
```

## Success Criteria

- [ ] ApplyConfigToDevice() - exports desired config to device
- [ ] VerifyConfigApplied() - imports and compares to desired
- [ ] Full ApplyAndVerify() workflow
- [ ] Handle partial failures (some settings applied, some failed)
- [ ] Handle settings that require reboot
- [ ] Detailed diff report for mismatches
- [ ] Skip read-only fields in comparison
- [ ] Integration tests with mock device client
- [ ] Mark config_applied=true only on full match

## Service Methods

```go
// ApplyConfigToDevice exports desired_config to the physical device
func (s *ConfigurationService) ApplyConfigToDevice(deviceID uint) (*ApplyResult, error)

type ApplyResult struct {
    Success        bool           // All settings applied successfully
    SettingsCount  int            // Number of settings sent
    AppliedCount   int            // Number of settings accepted
    FailedCount    int            // Number of settings rejected
    Failures       []ApplyFailure // Details of failures
    RequiresReboot bool           // Device needs reboot to apply
    Warnings       []string       // Non-fatal issues
}

type ApplyFailure struct {
    Path    string // e.g., "mqtt.server"
    Value   string // What we tried to set
    Error   string // Error from device
}

// VerifyConfigApplied imports config from device and compares to desired
func (s *ConfigurationService) VerifyConfigApplied(deviceID uint) (*VerifyResult, error)

type VerifyResult struct {
    Match       bool               // Imported matches desired
    Differences []ConfigDifference // List of mismatches
    Imported    *DeviceConfiguration // What we got from device
    Desired     *DeviceConfiguration // What we expected
}

type ConfigDifference struct {
    Path     string      // e.g., "mqtt.server"
    Expected interface{} // What we wanted
    Actual   interface{} // What device has
    Severity string      // "error" or "warning"
}

// ApplyAndVerify combines apply + verify in single operation
func (s *ConfigurationService) ApplyAndVerify(deviceID uint) (*ApplyVerifyResult, error)

type ApplyVerifyResult struct {
    ApplyResult  *ApplyResult
    VerifyResult *VerifyResult
    ConfigApplied bool // Final status
}
```

## Comparison Logic

Some fields should be compared differently:

```go
type FieldCompareRule struct {
    Path      string
    SkipCompare bool   // Don't compare (read-only or volatile)
    Tolerance   float64 // For numeric fields (e.g., lat/lng)
    Normalize   func(interface{}) interface{} // Normalize before compare
}

var compareRules = []FieldCompareRule{
    // Skip read-only fields
    {Path: "system.mac", SkipCompare: true},
    {Path: "system.firmware", SkipCompare: true},
    
    // Tolerance for coordinates
    {Path: "location.latitude", Tolerance: 0.0001},
    {Path: "location.longitude", Tolerance: 0.0001},
    
    // Normalize timezone names
    {Path: "location.timezone", Normalize: normalizeTimezone},
}
```

## Reboot Handling

Some settings require device reboot:
- WiFi SSID/password changes
- Authentication enable/disable
- Some system settings

```go
func (s *ConfigurationService) ApplyConfigToDevice(deviceID uint) (*ApplyResult, error) {
    // ... apply settings ...
    
    result := &ApplyResult{Success: true}
    
    // Check if reboot needed
    if s.requiresReboot(oldConfig, newConfig) {
        result.RequiresReboot = true
        result.Warnings = append(result.Warnings, 
            "Device reboot required. Run RebootAndVerify() to complete.")
    }
    
    return result, nil
}

// RebootAndVerify reboots device and verifies config after restart
func (s *ConfigurationService) RebootAndVerify(deviceID uint) (*VerifyResult, error) {
    // 1. Send reboot command
    // 2. Wait for device to come back online (poll with timeout)
    // 3. Verify config
}
```

## Files to Create

- `internal/configuration/apply.go` (NEW)
- `internal/configuration/apply_test.go` (NEW)
- `internal/configuration/verify.go` (NEW)
- `internal/configuration/verify_test.go` (NEW)
- `internal/configuration/compare.go` (NEW - field comparison logic)
- `internal/configuration/compare_test.go` (NEW)

## Test Strategy

1. Unit tests with mock Shelly client
2. Test successful apply + verify
3. Test partial failure (some settings rejected)
4. Test mismatch detection
5. Test read-only field skipping
6. Test tolerance-based comparison
7. Integration test with real device (manual, optional)

## Validation

```bash
make test-ci
go test -v ./internal/configuration/... -run TestApply
go test -v ./internal/configuration/... -run TestVerify
go test -v ./internal/configuration/... -run TestCompare
```

## Notes

This task connects the configuration service to actual devices. It uses:
- Converters (602/603) for format translation
- Shelly client for device communication
- Service layer (606) for database updates

Error handling is critical here - device communication can fail in many ways.

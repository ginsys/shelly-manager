# Gen1 API Converters (SHPLG-S)

**Priority**: HIGH
**Status**: completed
**Effort**: 6 hours
**Completed**: 2026-01-06
**Depends On**: 601

## Context

Implement bidirectional converters between Gen1 Shelly API JSON format and normalized internal config structs. Start with SHPLG-S (Smart Plug with Power Metering) as it's the most common device type in the test network.

The converter must:
1. **FromAPI**: Parse raw Gen1 `/settings` JSON → DeviceConfiguration struct
2. **ToAPI**: Convert DeviceConfiguration struct → Gen1 API format for `/settings` POST

## Reference Data

Use actual imported_config from database (device_id=1, SHPLG-S) as test fixture. The database contains 5 SHPLG-S devices with real configuration data.

## Field Mapping (Gen1 API → Internal)

| Gen1 API Field | Internal Field | Notes |
|----------------|----------------|-------|
| `wifi_sta.ssid`, `wifi_sta.enabled`, etc. | `Network.WiFiSTA.*` | Primary WiFi |
| `wifi_sta1.*` | `Network.WiFiSTA1.*` | Backup WiFi |
| `wifi_ap.*` | `Network.WiFiAP.*` | Access point mode |
| `mqtt.*` | `MQTT.*` | MQTT settings |
| `login.*` | `Auth.*` | Authentication |
| `name` | `System.Name` | Device name |
| `timezone`, `tzautodetect`, `tz_utc_offset`, `tz_dst`, `tz_dst_auto` | `Location.Timezone.*` | Time settings |
| `lat`, `lng` | `Location.Latitude`, `Location.Longitude` | Coordinates |
| `sntp.*` | `Location.SNTP.*` | NTP settings |
| `cloud.*` | `Cloud.*` | Cloud settings |
| `coiot.*` | `CoIoT.*` | CoIoT protocol |
| `eco_mode_enabled` | `System.EcoMode` | Power saving |
| `discoverable` | `System.Discoverable` | mDNS visibility |
| `led_power_disable`, `led_status_disable` | `LED.*` | LED indicators |
| `max_power` | `Meters[0].MaxPower` | Power limit |
| `relays[0].*` | `Switches[0].*` | Relay config |

## Read-Only Fields (Skip on ToAPI)

These fields are returned by the device but cannot be set:
- `device.*` (hostname, mac, num_meters, num_outputs, type)
- `hwinfo.*` (batch_id, hw_revision)
- `build_info.*`
- `fw`
- `time`, `unixtime`

## Success Criteria

- [ ] Implement `ConfigConverter` interface
- [ ] Implement `Gen1Converter` struct with `FromAPIConfig()` method
- [ ] Implement `Gen1Converter.ToAPIConfig()` method
- [ ] Round-trip test: import → convert to internal → convert to API → compare
- [ ] Test with actual SHPLG-S config from database
- [ ] Document any intentional differences (read-only fields stripped, etc.)
- [ ] Handle missing/optional fields gracefully
- [ ] Validate converted config passes internal validation

## Implementation

```go
// ConfigConverter defines the interface for API ↔ internal conversion
type ConfigConverter interface {
    // FromAPIConfig converts raw API JSON to internal struct
    FromAPIConfig(apiJSON json.RawMessage, deviceType string) (*DeviceConfiguration, error)
    
    // ToAPIConfig converts internal struct to API JSON
    ToAPIConfig(config *DeviceConfiguration, deviceType string) (json.RawMessage, error)
    
    // SupportedDeviceTypes returns list of supported device types
    SupportedDeviceTypes() []string
    
    // Generation returns the Shelly generation (1 or 2)
    Generation() int
}

// Gen1Converter handles Gen1 device API conversion
type Gen1Converter struct {
    logger *logging.Logger
}

func NewGen1Converter(logger *logging.Logger) *Gen1Converter
func (c *Gen1Converter) FromAPIConfig(apiJSON json.RawMessage, deviceType string) (*DeviceConfiguration, error)
func (c *Gen1Converter) ToAPIConfig(config *DeviceConfiguration, deviceType string) (json.RawMessage, error)
func (c *Gen1Converter) SupportedDeviceTypes() []string
func (c *Gen1Converter) Generation() int
```

## Files to Create

- `internal/configuration/converter.go` (NEW - interface definition)
- `internal/configuration/gen1_converter.go` (NEW - Gen1 implementation)
- `internal/configuration/gen1_converter_test.go` (NEW - tests)
- `internal/configuration/testdata/shplg_s_config.json` (NEW - test fixture from DB)

## Test Strategy

1. Extract actual SHPLG-S config from database as test fixture
2. Test FromAPIConfig produces valid DeviceConfiguration
3. Test ToAPIConfig produces valid Gen1 JSON
4. Round-trip test: FromAPI → ToAPI should produce equivalent JSON
5. Test nil handling for optional fields
6. Test that read-only fields are excluded from ToAPI output

## Validation

```bash
make test-ci
go test -v ./internal/configuration/... -run TestGen1Converter
```

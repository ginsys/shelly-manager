# Device Configuration Architecture

## Overview

This document describes the composition-based configuration architecture for Shelly device management, designed to handle the diverse capabilities of different Shelly device models while maintaining type safety and extensibility.

## Core Design Principles

1. **Composition over Inheritance**: Devices combine multiple capability mixins rather than inheriting from a rigid hierarchy
2. **Type Safety**: Strongly-typed configuration structures with compile-time checking
3. **Extensibility**: Easy to add new capabilities and device types
4. **DRY Principle**: No duplication of configuration structures
5. **Template Power**: Templates can target capabilities, not just device models

## Configuration Layers

```
Base Config (Generation-specific: Gen1 or Gen2+)
    ↓
Device Category Config (by functionality group)
    ↓
Device Model Config (specific device features)
```

## Capability Interfaces

### Core Interfaces

```go
// Core capability interfaces
type HasRelay interface {
    GetRelayConfig() *RelayConfig
}

type HasPowerMetering interface {
    GetPowerMeteringConfig() *PowerMeteringConfig
}

type HasDimming interface {
    GetDimmingConfig() *DimmingConfig
}

type HasRoller interface {
    GetRollerConfig() *RollerConfig
}

type HasInput interface {
    GetInputConfig() *InputConfig
}

type HasLED interface {
    GetLEDConfig() *LEDConfig
}

type HasColorControl interface {
    GetColorConfig() *ColorConfig
}

type HasTemperatureProtection interface {
    GetTempProtectionConfig() *TempProtectionConfig
}

type HasSchedule interface {
    GetScheduleConfig() *ScheduleConfig
}
```

## Capability Configuration Blocks

### RelayConfig
```go
type RelayConfig struct {
    DefaultState   string `json:"default_state"`   // "on", "off", "last", "switch"
    AutoOn        *int    `json:"auto_on"`         // seconds, nil = disabled
    AutoOff       *int    `json:"auto_off"`        // seconds, nil = disabled
    HasTimer      bool    `json:"has_timer"`       // supports scheduling
}
```

### PowerMeteringConfig
```go
type PowerMeteringConfig struct {
    MaxPower         *int      `json:"max_power"`          // Watts
    MaxVoltage       *int      `json:"max_voltage"`        // Volts
    MaxCurrent       *float64  `json:"max_current"`        // Amps
    PowerCorrection  float64   `json:"power_correction"`   // calibration
    EnergyReset      *time.Time `json:"energy_reset"`      // last reset
    ReportingPeriod  int       `json:"reporting_period"`   // seconds
    ProtectionAction string    `json:"protection_action"`  // "off", "restart", "alert"
}
```

### DimmingConfig
```go
type DimmingConfig struct {
    MinBrightness    int    `json:"min_brightness"`     // 1-100
    MaxBrightness    int    `json:"max_brightness"`     // 1-100
    DefaultBrightness int   `json:"default_brightness"` // 1-100
    FadeRate         int    `json:"fade_rate"`          // ms per step
    TransitionTime   int    `json:"transition"`         // ms
    EdgeType         string `json:"edge_type"`          // "leading", "trailing"
    NightMode        bool   `json:"night_mode"`
    NightBrightness  int    `json:"night_brightness"`
}
```

### InputConfig
```go
type InputConfig struct {
    ButtonType      string            `json:"btn_type"`        // "momentary", "toggle", "edge", "detached"
    ButtonReverse   bool              `json:"btn_reverse"`
    LongPressTime   int               `json:"longpress_time"`  // ms
    MultiPressTime  int               `json:"multipress_time"` // ms
    Actions         map[string]string `json:"actions"`         // action mappings
}
```

### RollerConfig
```go
type RollerConfig struct {
    MaxTime          int    `json:"maxtime"`           // seconds for full travel
    DefaultState     string `json:"default_state"`     // "open", "close", "stop"
    SwapInputs       bool   `json:"swap"`
    ObstacleMode     string `json:"obstacle_mode"`     // "disabled", "while_opening", "while_closing", "while_moving"
    ObstaclePower    int    `json:"obstacle_power"`    // Watts
    SafetySwitch     bool   `json:"safety_switch"`
    PositionControl  bool   `json:"positioning"`       // precise positioning
    CurrentCalibration bool `json:"current_cal"`      // auto-calibration
}
```

## Device Categories & Common Settings

### Relay/Switch Devices
**Models**: SHSW-1, SHSW-PM, SHSW-25 (relay mode), SHPLG-S  
**Common Capabilities**:
- RelayConfig
- InputConfig
- Optional: PowerMeteringConfig, TempProtectionConfig, LEDConfig

### Metering Devices
**Models**: SHSW-PM, SHSW-25, SHPLG-S, SHEM  
**Common Capabilities**:
- PowerMeteringConfig
- TempProtectionConfig

### Lighting Controllers
**Models**: SHDM-1/2, SHRGBW2, Bulbs  
**Common Capabilities**:
- DimmingConfig
- Optional: ColorConfig, PowerMeteringConfig

### Motor Controllers
**Models**: SHSW-25 (roller mode), Dedicated roller devices  
**Common Capabilities**:
- RollerConfig
- PowerMeteringConfig
- InputConfig

### Input Devices
**Models**: SHIX3-1, Button devices  
**Common Capabilities**:
- InputConfig
- EventConfig

### Sensor Devices
**Models**: SHHT-1, SHWT-1, SHDW-2  
**Common Capabilities**:
- SensorConfig
- BatteryConfig

## Device-Specific Configurations

### SHSW-1 (Basic Switch)
```go
type SHSW1Config struct {
    RelayConfig
    InputConfig
}
```

### SHSW-PM (Switch with Power Metering)
```go
type SHSWPMConfig struct {
    RelayConfig
    PowerMeteringConfig
    InputConfig
    TempProtectionConfig
}
```

### SHPLG-S (Plug with Power Metering and LED)
```go
type SHPLGSConfig struct {
    RelayConfig
    PowerMeteringConfig
    LEDConfig
    TempProtectionConfig
}
```

### SHSW-25 (Dual Mode Device)
```go
type SHSW25Config struct {
    Mode string `json:"mode"` // "relay" or "roller"
    
    // Common capabilities
    PowerMeteringConfig
    TempProtectionConfig
    InputConfig
    
    // Mode-specific configs
    RelayMode  *SHSW25RelayMode  `json:"relay_mode,omitempty"`
    RollerMode *SHSW25RollerMode `json:"roller_mode,omitempty"`
}

type SHSW25RelayMode struct {
    Relay1 RelayConfig `json:"relay_0"`
    Relay2 RelayConfig `json:"relay_1"`
}

type SHSW25RollerMode struct {
    RollerConfig
}
```

### SHDM-2 (Dimmer)
```go
type SHDM2Config struct {
    DimmingConfig
    PowerMeteringConfig
    InputConfig
    TempProtectionConfig
}
```

### SHRGBW2 (Multi-mode Lighting)
```go
type SHRGBW2Config struct {
    Mode string `json:"mode"` // "color", "white", "multi_white"
    
    PowerMeteringConfig
    
    // Mode-specific configs
    ColorMode  *ColorModeConfig  `json:"color_mode,omitempty"`
    WhiteMode  *WhiteModeConfig  `json:"white_mode,omitempty"`
}

type ColorModeConfig struct {
    ColorConfig
    DimmingConfig  // for overall brightness
    Effects []string `json:"effects"`
}

type WhiteModeConfig struct {
    Channels []DimmingConfig `json:"channels"` // up to 4
}
```

## Template System

### Template Structure
```go
type ConfigTemplate struct {
    ID          uint   `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    
    // Target by capability instead of device type
    TargetCapabilities []string `json:"target_capabilities"` // ["relay", "power_metering"]
    
    // Or specific devices
    TargetDevices []string `json:"target_devices"` // ["SHSW-PM", "SHPLG-S"]
    
    // Configuration blocks
    RelayConfig    *RelayConfig         `json:"relay,omitempty"`
    PowerConfig    *PowerMeteringConfig `json:"power,omitempty"`
    DimmingConfig  *DimmingConfig       `json:"dimming,omitempty"`
    RollerConfig   *RollerConfig        `json:"roller,omitempty"`
    InputConfig    *InputConfig         `json:"input,omitempty"`
    LEDConfig      *LEDConfig           `json:"led,omitempty"`
    // ... other capability configs
}
```

### Template Examples

#### Power Limit Template
```go
powerLimitTemplate := ConfigTemplate{
    Name: "Power Limit 2000W",
    TargetCapabilities: []string{"power_metering"},
    PowerConfig: &PowerMeteringConfig{
        MaxPower: intPtr(2000),
        ProtectionAction: "restart",
    },
}
```

#### Default OFF Template
```go
relayDefaultTemplate := ConfigTemplate{
    Name: "Default OFF",
    TargetCapabilities: []string{"relay"},
    RelayConfig: &RelayConfig{
        DefaultState: "off",
    },
}
```

#### Plug Configuration Template
```go
shellyPlugTemplate := ConfigTemplate{
    Name: "Plug Configuration",
    TargetDevices: []string{"SHPLG-S"},
    RelayConfig: &RelayConfig{
        DefaultState: "last",
        AutoOff: intPtr(3600), // 1 hour auto-off
    },
    PowerConfig: &PowerMeteringConfig{
        MaxPower: intPtr(2300),
    },
    LEDConfig: &LEDConfig{
        StatusLED: true,
        Brightness: 50,
        PowerIndication: true,
    },
}
```

## Dynamic Capability Discovery

```go
// Runtime capability checking
func GetDeviceCapabilities(deviceType string) []string {
    capabilities := map[string][]string{
        "SHSW-1":   {"relay", "input"},
        "SHSW-PM":  {"relay", "power_metering", "input", "temp_protection"},
        "SHPLG-S":  {"relay", "power_metering", "led", "temp_protection"},
        "SHSW-25":  {"relay", "roller", "power_metering", "input", "temp_protection"},
        "SHDM-2":   {"dimming", "power_metering", "input", "temp_protection"},
        "SHRGBW2":  {"color", "dimming", "power_metering", "effects"},
        "SHIX3-1":  {"input", "events"},
        // ... more devices
    }
    return capabilities[deviceType]
}

// Check if device supports a capability
func SupportsCapability(config interface{}, capability string) bool {
    switch capability {
    case "relay":
        _, ok := config.(HasRelay)
        return ok
    case "power_metering":
        _, ok := config.(HasPowerMetering)
        return ok
    case "dimming":
        _, ok := config.(HasDimming)
        return ok
    // ... other capabilities
    }
    return false
}
```

## Migration Path

### Phase 1: Core Implementation
1. Define all capability interfaces
2. Implement configuration blocks
3. Create device-specific configurations

### Phase 2: Integration
1. Update `internal/configuration/models.go`
2. Replace `json.RawMessage` with typed structures
3. Update configuration service

### Phase 3: Template Enhancement
1. Implement capability-based targeting
2. Add variable substitution
3. Create template validation

### Phase 4: UI Updates
1. Update web interface for capability display
2. Add template builder UI
3. Implement capability filters

## Benefits

1. **Type Safety**: Compile-time checking of configuration fields
2. **IDE Support**: Autocomplete and inline documentation
3. **Validation**: Easy to validate required fields per device type
4. **Clear Documentation**: Self-documenting code structure
5. **Template Composition**: Templates can target specific capabilities
6. **Migration Path**: Easy to evolve as new devices are added
7. **Flexibility**: Devices can mix and match capabilities
8. **DRY Principle**: No duplication of configuration structures
9. **Extensibility**: Easy to add new capabilities
10. **Future-Proof**: New devices just compose existing capabilities

## Implementation Priority

1. **Core Capability Definitions** - Define interfaces and base configs
2. **Common Device Configs** - Implement SHSW-1, SHSW-PM, SHPLG-S
3. **Complex Devices** - SHSW-25 dual mode, SHRGBW2 multi-mode
4. **Template System** - Capability-based template targeting
5. **Service Integration** - Update configuration service
6. **API Enhancements** - Expose capability information
7. **UI Updates** - Reflect capabilities in web interface
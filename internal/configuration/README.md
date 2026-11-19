# Configuration Management System

## Overview

The Shelly Manager configuration management system provides a 3-level hierarchical configuration approach:

1. **Device-Level**: Individual settings stored on each physical Shelly device
2. **Template/Profile**: Reusable configurations for device groups (stored in DB)
3. **System/Global**: Default settings for new devices (from config.yaml)

## Architecture

### Configuration Hierarchy

```
System Level (config.yaml)
    ↓
Template Level (Database)
    ↓
Device Level (Physical Device)
```

### Key Features

- **Import**: Pull configuration from physical devices to database
- **Apply**: Write a device's desired configuration (templates and overrides) to the device, one device at a time (`POST /api/v1/devices/{id}/config/new/apply`)
- **Drift Detection**: Monitor configuration changes
- **History**: Track configuration changes over time
- **Templates**: Reusable configuration profiles
- **Bulk Operations**: Import and drift detection for several devices; there is no bulk apply yet

## Configuration Model

### Device Configuration
- Network settings (WiFi, IP, DNS)
- Authentication settings
- Cloud connectivity
- MQTT settings
- Power settings
- Schedule settings
- Hardware-specific settings (relay, dimmer, etc.)

### Template Configuration
- Named configuration profiles
- Device type compatibility
- Variable substitution support
- Inheritance from system defaults

### System Configuration
- Global defaults for all devices
- Discovery settings
- Provisioning defaults
- Network defaults

## Operations

### Import Configuration
```go
// Import configuration from device to database
config, err := configService.ImportFromDevice(deviceID)
```

### Detect Drift
```go
// Compare the live device with its desired configuration (the one apply
// writes and verify reads). A device with no desired configuration returns
// ErrNoDesiredConfig; the stored device_configs row is not consulted.
drift, err := configService.DetectDrift(deviceID, client)
```

### Apply Template
```go
// Apply template to device
err := configService.ApplyTemplate(deviceID, templateID)
```

## Database Schema

### ConfigTemplates Table
- ID
- Name
- Description
- DeviceType (compatibility)
- Configuration (JSON)
- Variables (JSON)
- CreatedAt
- UpdatedAt

### DeviceConfigs Table
- ID
- DeviceID
- TemplateID (optional)
- Configuration (JSON)
- LastSynced
- CreatedAt
- UpdatedAt

### ConfigHistory Table
- ID
- DeviceID
- ConfigID
- Action (import/export/sync)
- OldConfig (JSON)
- NewConfig (JSON)
- ChangedBy
- CreatedAt
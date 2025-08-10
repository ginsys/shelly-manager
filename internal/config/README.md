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
- **Export**: Push configuration from database to physical devices
- **Sync**: Bidirectional synchronization with conflict resolution
- **Drift Detection**: Monitor configuration changes
- **History**: Track configuration changes over time
- **Templates**: Reusable configuration profiles
- **Bulk Operations**: Apply configurations to multiple devices

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

### Export Configuration
```go
// Export configuration from database to device
err := configService.ExportToDevice(deviceID, configID)
```

### Detect Drift
```go
// Check if device configuration matches database
drift, err := configService.DetectDrift(deviceID)
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
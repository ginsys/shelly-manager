# Template & Override Service Layer

**Priority**: HIGH
**Status**: not-started
**Effort**: 8 hours
**Depends On**: 604, 605

## Context

Implement the service layer that orchestrates template and override management:
- Template CRUD with validation
- Device template assignment (add/remove/reorder)
- Device tag management for group templates
- Device override management
- Automatic desired_config recomputation when templates or overrides change

## Core Responsibilities

1. **Template Management**: CRUD operations with scope validation
2. **Device Assignment**: Which templates apply to which devices
3. **Tag Management**: Tags for group template matching
4. **Override Management**: Device-specific configuration values
5. **Desired Config Computation**: Merge templates + overrides, cache result
6. **Change Propagation**: Recompute affected devices when template changes

## Success Criteria

- [ ] Template CRUD operations (Create, Read, Update, Delete)
- [ ] Template validation (scope, device_type consistency)
- [ ] List templates by scope (global, group, device_type)
- [ ] Device template assignment (set ordered list of template IDs)
- [ ] Device tag management (add/remove tags)
- [ ] Get devices by tag
- [ ] Device override get/set
- [ ] Automatic desired_config recompute on any change
- [ ] Batch recompute when template changes (affects multiple devices)
- [ ] Mark config_applied=false when desired_config changes
- [ ] Conflict warning when template scopes overlap
- [ ] Unit tests for all operations

## Service Interface

```go
type ConfigurationService struct {
    db      *database.Manager
    merger  Merger
    logger  *logging.Logger
}

// Merger abstracts the merge engine for testability.
// It should be a pure function implementation (no DB/I/O).
type Merger interface {
    Merge(layers []ConfigLayer) (*MergeResult, error)
}

func NewConfigurationService(db *database.Manager, logger *logging.Logger) *ConfigurationService

// ==================== Template Operations ====================

// CreateTemplate creates a new configuration template
func (s *ConfigurationService) CreateTemplate(template *ConfigTemplate) error

// GetTemplate retrieves a template by ID
func (s *ConfigurationService) GetTemplate(id uint) (*ConfigTemplate, error)

// UpdateTemplate updates an existing template
// Triggers recompute of all affected devices
func (s *ConfigurationService) UpdateTemplate(template *ConfigTemplate) error

// DeleteTemplate removes a template
// Returns error if template is assigned to devices (must unassign first)
func (s *ConfigurationService) DeleteTemplate(id uint) error

// ListTemplates returns templates, optionally filtered by scope
func (s *ConfigurationService) ListTemplates(scope string) ([]ConfigTemplate, error)

// GetTemplatesForDeviceType returns templates applicable to a device type
func (s *ConfigurationService) GetTemplatesForDeviceType(deviceType string) ([]ConfigTemplate, error)

// ==================== Device Template Assignment ====================

// SetDeviceTemplates sets the ordered list of templates for a device
// Triggers desired_config recomputation
func (s *ConfigurationService) SetDeviceTemplates(deviceID uint, templateIDs []uint) error

// GetDeviceTemplates returns templates assigned to a device in order
func (s *ConfigurationService) GetDeviceTemplates(deviceID uint) ([]ConfigTemplate, error)

// AddTemplateToDevice adds a template to device's list (at end or specified position)
func (s *ConfigurationService) AddTemplateToDevice(deviceID, templateID uint, position int) error

// RemoveTemplateFromDevice removes a template from device's list
func (s *ConfigurationService) RemoveTemplateFromDevice(deviceID, templateID uint) error

// ==================== Device Tags ====================

// AddDeviceTag adds a tag to a device
func (s *ConfigurationService) AddDeviceTag(deviceID uint, tag string) error

// RemoveDeviceTag removes a tag from a device
func (s *ConfigurationService) RemoveDeviceTag(deviceID uint, tag string) error

// GetDeviceTags returns all tags for a device
func (s *ConfigurationService) GetDeviceTags(deviceID uint) ([]string, error)

// GetDevicesByTag returns all devices with a specific tag
func (s *ConfigurationService) GetDevicesByTag(tag string) ([]Device, error)

// ListAllTags returns all unique tags in use
func (s *ConfigurationService) ListAllTags() ([]string, error)

// ==================== Device Overrides ====================

// SetDeviceOverrides sets the device-specific configuration overrides
// Triggers desired_config recomputation
func (s *ConfigurationService) SetDeviceOverrides(deviceID uint, overrides *DeviceConfiguration) error

// GetDeviceOverrides returns the device-specific overrides
func (s *ConfigurationService) GetDeviceOverrides(deviceID uint) (*DeviceConfiguration, error)

// PatchDeviceOverrides merges partial overrides into existing
func (s *ConfigurationService) PatchDeviceOverrides(deviceID uint, patch *DeviceConfiguration) error

// ClearDeviceOverrides removes all device-specific overrides
func (s *ConfigurationService) ClearDeviceOverrides(deviceID uint) error

// ==================== Desired Config ====================

// GetDesiredConfig returns the computed desired config for a device
// Also returns source tracking map (field path → source name)
func (s *ConfigurationService) GetDesiredConfig(deviceID uint) (*DeviceConfiguration, map[string]string, error)

// RecomputeDesiredConfig forces recomputation of desired_config for a device
func (s *ConfigurationService) RecomputeDesiredConfig(deviceID uint) error

// GetAffectedDevices returns device IDs that would be affected by template change
func (s *ConfigurationService) GetAffectedDevices(templateID uint) ([]uint, error)

// RecomputeAffectedDevices recomputes desired_config for all devices using a template
func (s *ConfigurationService) RecomputeAffectedDevices(templateID uint) (int, error)

// ==================== Status ====================

// GetConfigStatus returns whether device config is applied/pending
func (s *ConfigurationService) GetConfigStatus(deviceID uint) (*ConfigStatus, error)

type ConfigStatus struct {
    DeviceID      uint
    ConfigApplied bool
    HasOverrides  bool
    TemplateCount int
    LastUpdated   time.Time
}
```

## Recomputation Logic

```go
func (s *ConfigurationService) recomputeDesiredConfig(device *Device) error {
    // 1. Get assigned templates in order
    templates, err := s.GetDeviceTemplates(device.ID)
    
    // 2. Build layers for merge
    layers := []ConfigLayer{}
    for _, tmpl := range templates {
        layers = append(layers, ConfigLayer{
            Name:   tmpl.Name,
            Config: parseConfig(tmpl.Config),
        })
    }
    
    // 3. Add device overrides as final layer
    // Avoid string comparisons on JSON; parse and check semantic emptiness.
    overridesCfg, hasOverrides := parseConfigIfNonEmpty(device.Overrides)
    if hasOverrides {
        layers = append(layers, ConfigLayer{
            Name:   "device-override",
            Config: overridesCfg,
        })
    }
    
    // 4. Merge
    result, err := MergeConfigurations(layers)
    
    // 5. Update device
    device.DesiredConfig = serializeConfig(result.Config)
    device.ConfigApplied = false  // Mark as needing apply
    
    return s.db.UpdateDevice(device)
}
```

## Files to Create

- `internal/configuration/config_service.go` (NEW)
- `internal/configuration/config_service_test.go` (NEW)

## Validation

```bash
make test-ci
go test -v ./internal/configuration/... -run TestConfigService
```

## Notes

This is the central orchestration layer. It connects:
- Database (models from 605)
- Merge engine (from 604)
- Business logic (validation, recomputation)

The API layer (608) will be a thin wrapper around this service.

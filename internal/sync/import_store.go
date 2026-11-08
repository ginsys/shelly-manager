package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

// ErrImportStoreUnavailable means the engine has no database to import into.
var ErrImportStoreUnavailable = errors.New("import store unavailable")

// ImportApplyResult reports what ApplyImport wrote.
type ImportApplyResult struct {
	Imported int
	Skipped  int
	Changes  []ImportChange
}

// ImportStore persists decoded import data. The engine implements it and hands it to plugins
// that implement ImportStoreAware, so a plugin decodes and validates its format while the
// engine owns the database.
type ImportStore interface {
	// ApplyImport writes devices, templates and discovered devices in one transaction: either
	// every record is written or none is. A record that already exists is skipped unless
	// opts.ForceOverwrite is set, in which case it is updated.
	ApplyImport(ctx context.Context, data *ExportData, opts ImportOptions) (*ImportApplyResult, error)
}

// ImportStoreAware is implemented by plugins that persist imports through the engine.
type ImportStoreAware interface {
	SetImportStore(store ImportStore)
}

var _ ImportStore = (*SyncEngine)(nil)

// discoveredDeviceTTL is how long an imported discovered device stays listed. It matches the
// lifetime the provisioning handlers give a freshly discovered device.
const discoveredDeviceTTL = 24 * time.Hour

// ApplyImport implements ImportStore.
func (e *SyncEngine) ApplyImport(ctx context.Context, data *ExportData, opts ImportOptions) (*ImportApplyResult, error) {
	if data == nil {
		return nil, fmt.Errorf("%w: no data", ErrInvalidImportData)
	}
	db := e.dbManager.GetDB()
	if db == nil {
		return nil, ErrImportStoreUnavailable
	}

	result := &ImportApplyResult{Changes: []ImportChange{}}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := importDevices(tx, data, opts, result); err != nil {
			return err
		}
		if err := importTemplates(tx, data, opts, result); err != nil {
			return err
		}
		return importDiscovered(tx, data, opts, result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func importDevices(tx *gorm.DB, data *ExportData, opts ImportOptions, result *ImportApplyResult) error {
	for _, in := range data.Devices {
		if strings.TrimSpace(in.MAC) == "" {
			return fmt.Errorf("%w: device without a MAC address", ErrInvalidImportData)
		}
		settings, err := marshalObject(in.Settings)
		if err != nil {
			return fmt.Errorf("device %s settings: %w", in.MAC, err)
		}
		desired := ""
		if in.Configuration != nil && in.Configuration.Config != nil {
			if desired, err = marshalObject(in.Configuration.Config); err != nil {
				return fmt.Errorf("device %s configuration: %w", in.MAC, err)
			}
		}

		var existing database.Device
		lookup := tx.Where("mac = ?", in.MAC).First(&existing)
		switch {
		case errors.Is(lookup.Error, gorm.ErrRecordNotFound):
			device := database.Device{
				IP: in.IP, MAC: in.MAC, Type: in.Type, Name: in.Name, Firmware: in.Firmware,
				Status: in.Status, LastSeen: in.LastSeen, Settings: settings, DesiredConfig: desired,
				CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt,
			}
			if err := tx.Create(&device).Error; err != nil {
				return fmt.Errorf("create device %s: %w", in.MAC, err)
			}
			result.Imported++
			result.Changes = append(result.Changes, ImportChange{
				Type: "create", Resource: "device", ResourceID: in.MAC,
				NewValue: fmt.Sprintf("Device: %s (%s)", in.Name, in.Type),
			})
		case lookup.Error != nil:
			return fmt.Errorf("look up device %s: %w", in.MAC, lookup.Error)
		case !opts.ForceOverwrite:
			result.Skipped++
		default:
			updates := map[string]interface{}{
				"ip": in.IP, "type": in.Type, "name": in.Name, "firmware": in.Firmware,
				"status": in.Status, "last_seen": in.LastSeen, "settings": settings,
			}
			if desired != "" {
				// A new desired configuration has not reached the device yet.
				updates["desired_config"] = desired
				updates["config_applied"] = false
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update device %s: %w", in.MAC, err)
			}
			result.Imported++
			result.Changes = append(result.Changes, ImportChange{
				Type: "update", Resource: "device", ResourceID: in.MAC,
				OldValue: fmt.Sprintf("Device: %s (%s)", existing.Name, existing.Type),
				NewValue: fmt.Sprintf("Device: %s (%s)", in.Name, in.Type),
			})
		}
	}
	return nil
}

func importTemplates(tx *gorm.DB, data *ExportData, opts ImportOptions, result *ImportApplyResult) error {
	for _, in := range data.Templates {
		if strings.TrimSpace(in.Name) == "" {
			return fmt.Errorf("%w: template without a name", ErrInvalidImportData)
		}
		config, err := marshalObject(in.Config)
		if err != nil {
			return fmt.Errorf("template %s config: %w", in.Name, err)
		}
		variables, err := marshalObject(in.Variables)
		if err != nil {
			return fmt.Errorf("template %s variables: %w", in.Name, err)
		}

		var existing configuration.ConfigTemplate
		lookup := tx.Where("name = ?", in.Name).First(&existing)
		switch {
		case errors.Is(lookup.Error, gorm.ErrRecordNotFound):
			template := configuration.ConfigTemplate{
				Name: in.Name, Description: in.Description, Scope: templateScope(in.DeviceType),
				DeviceType: in.DeviceType, Generation: in.Generation, IsDefault: in.IsDefault,
				Config: json.RawMessage(config), Variables: json.RawMessage(variables),
				CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt,
			}
			if err := tx.Create(&template).Error; err != nil {
				return fmt.Errorf("create template %s: %w", in.Name, err)
			}
			result.Imported++
			result.Changes = append(result.Changes, ImportChange{
				Type: "create", Resource: "template", ResourceID: in.Name,
				NewValue: fmt.Sprintf("Template: %s for %s", in.Name, in.DeviceType),
			})
		case lookup.Error != nil:
			return fmt.Errorf("look up template %s: %w", in.Name, lookup.Error)
		case !opts.ForceOverwrite:
			result.Skipped++
		default:
			updates := map[string]interface{}{
				"description": in.Description, "scope": templateScope(in.DeviceType),
				"device_type": in.DeviceType, "generation": in.Generation, "is_default": in.IsDefault,
				"config": json.RawMessage(config), "variables": json.RawMessage(variables),
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update template %s: %w", in.Name, err)
			}
			result.Imported++
			result.Changes = append(result.Changes, ImportChange{
				Type: "update", Resource: "template", ResourceID: in.Name,
				NewValue: fmt.Sprintf("Template: %s for %s", in.Name, in.DeviceType),
			})
		}
	}
	return nil
}

func importDiscovered(tx *gorm.DB, data *ExportData, opts ImportOptions, result *ImportApplyResult) error {
	for _, in := range data.DiscoveredDevices {
		if strings.TrimSpace(in.MAC) == "" {
			return fmt.Errorf("%w: discovered device without a MAC address", ErrInvalidImportData)
		}
		expires := time.Now().Add(discoveredDeviceTTL)

		var existing database.DiscoveredDevice
		lookup := tx.Where("mac = ? AND agent_id = ?", in.MAC, in.AgentID).First(&existing)
		switch {
		case errors.Is(lookup.Error, gorm.ErrRecordNotFound):
			discovered := database.DiscoveredDevice{
				MAC: in.MAC, SSID: in.SSID, Model: in.Model, Generation: in.Generation,
				IP: in.IP, Signal: in.Signal, AgentID: in.AgentID, Discovered: in.Discovered,
				ExpiresAt: expires,
			}
			if err := tx.Create(&discovered).Error; err != nil {
				return fmt.Errorf("create discovered device %s: %w", in.MAC, err)
			}
			result.Imported++
			result.Changes = append(result.Changes, ImportChange{
				Type: "create", Resource: "discovered_device", ResourceID: in.MAC,
				NewValue: fmt.Sprintf("Discovered: %s (%s)", in.Model, in.MAC),
			})
		case lookup.Error != nil:
			return fmt.Errorf("look up discovered device %s: %w", in.MAC, lookup.Error)
		case !opts.ForceOverwrite:
			result.Skipped++
		default:
			// Assign fields and save: GORM names the SSID column ss_id, which a map key
			// would get wrong.
			existing.SSID, existing.Model, existing.Generation, existing.IP = in.SSID, in.Model, in.Generation, in.IP
			existing.Signal, existing.Discovered, existing.ExpiresAt = in.Signal, in.Discovered, expires
			if err := tx.Save(&existing).Error; err != nil {
				return fmt.Errorf("update discovered device %s: %w", in.MAC, err)
			}
			result.Imported++
			result.Changes = append(result.Changes, ImportChange{
				Type: "update", Resource: "discovered_device", ResourceID: in.MAC,
				NewValue: fmt.Sprintf("Discovered: %s (%s)", in.Model, in.MAC),
			})
		}
	}
	return nil
}

// templateScope gives an imported template the scope its device type implies: the archive does
// not carry one.
func templateScope(deviceType string) string {
	if deviceType == "" || deviceType == "all" {
		return "global"
	}
	return "device_type"
}

// marshalObject returns the JSON text of m, with a nil map stored as an empty object.
func marshalObject(m map[string]interface{}) (string, error) {
	if m == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

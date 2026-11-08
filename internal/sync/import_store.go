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
	// Redacted counts redacted values in the archive. An existing record keeps its stored value
	// for each of them; a new record is written without it.
	Redacted int
	Changes  []ImportChange
}

// RedactedValue is what an export writes in place of a sensitive value. It is never a credential,
// so an import must not store it.
const RedactedValue = "[REDACTED]"

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

		var existing database.Device
		lookup := tx.Where("mac = ?", in.MAC).First(&existing)
		// A record that does not exist yet has no stored values to keep in place of a redaction.
		var storedSettings, storedDesired string
		if lookup.Error == nil {
			storedSettings = existing.Settings
			storedDesired = storedDesiredConfig(tx, existing)
		}
		settings, err := restoreObject(in.Settings, storedSettings, result)
		if err != nil {
			return fmt.Errorf("device %s settings: %w", in.MAC, err)
		}
		desired := ""
		if in.Configuration != nil && in.Configuration.Config != nil {
			if desired, err = restoreObject(in.Configuration.Config, storedDesired, result); err != nil {
				return fmt.Errorf("device %s configuration: %w", in.MAC, err)
			}
		}

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
			// Updates writes the new values back into existing, so read the old ones first.
			oldValue := fmt.Sprintf("Device: %s (%s)", existing.Name, existing.Type)
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update device %s: %w", in.MAC, err)
			}
			result.Imported++
			result.Changes = append(result.Changes, ImportChange{
				Type: "update", Resource: "device", ResourceID: in.MAC,
				OldValue: oldValue,
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

		var existing configuration.ConfigTemplate
		lookup := tx.Where("name = ?", in.Name).First(&existing)
		var storedConfig, storedVariables string
		if lookup.Error == nil {
			storedConfig, storedVariables = string(existing.Config), string(existing.Variables)
		}
		config, err := restoreObject(in.Config, storedConfig, result)
		if err != nil {
			return fmt.Errorf("template %s config: %w", in.Name, err)
		}
		variables, err := restoreObject(in.Variables, storedVariables, result)
		if err != nil {
			return fmt.Errorf("template %s variables: %w", in.Name, err)
		}
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
				// The scope stays as it is: the archive does not carry one, and a group template
				// must not turn global because its device type is empty.
				"description": in.Description,
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

// storedDesiredConfig returns the configuration an export of device would have written: the
// desired configuration, or the older device_configs row when that is empty.
func storedDesiredConfig(tx *gorm.DB, device database.Device) string {
	raw := strings.TrimSpace(device.DesiredConfig)
	if raw != "" && raw != "{}" && raw != "null" {
		return device.DesiredConfig
	}
	if !tx.Migrator().HasTable(&configuration.DeviceConfig{}) {
		return device.DesiredConfig
	}
	var legacy configuration.DeviceConfig
	if err := tx.Where("device_id = ?", device.ID).First(&legacy).Error; err != nil {
		return device.DesiredConfig
	}
	return string(legacy.Config)
}

// restoreObject returns the JSON text of m with each redacted value replaced by the one in the
// stored JSON object at the same path, or removed when there is none. It adds the number of
// redacted values to result.
func restoreObject(m map[string]interface{}, stored string, result *ImportApplyResult) (string, error) {
	var current map[string]interface{}
	if strings.TrimSpace(stored) != "" {
		// A stored value that is not an object has nothing to keep.
		_ = json.Unmarshal([]byte(stored), &current)
	}
	result.Redacted += resolveRedacted(m, current)
	return marshalObject(m)
}

// resolveRedacted replaces every RedactedValue in config with the value at the same path in
// current, or deletes the key when current has none. It returns how many it found. An array has
// no key to match its elements by, so an element takes the stored one at the same position.
func resolveRedacted(config, current map[string]interface{}) int {
	found := 0
	for key, value := range config {
		switch v := value.(type) {
		case string:
			if v != RedactedValue {
				continue
			}
			found++
			if kept, ok := current[key]; ok && !holdsPlaceholder(kept) {
				config[key] = kept
			} else {
				delete(config, key)
			}
		case map[string]interface{}:
			child, _ := current[key].(map[string]interface{})
			found += resolveRedacted(v, child)
		case []interface{}:
			stored, _ := current[key].([]interface{})
			found += resolveRedactedList(v, stored)
		}
	}
	return found
}

// holdsPlaceholder reports whether a stored value is, or contains, the redaction placeholder. Such
// a value is not a credential, so it is never restored.
func holdsPlaceholder(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v == RedactedValue
	case map[string]interface{}:
		for _, child := range v {
			if holdsPlaceholder(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range v {
			if holdsPlaceholder(child) {
				return true
			}
		}
	}
	return false
}

// resolveRedactedList resolves the redacted values inside the objects and nested lists of list.
// Element i is resolved against stored element i. A value with no stored element at that position
// is left out. A stored list that was reordered since the export pairs the wrong elements, which
// is the limit of matching by position.
func resolveRedactedList(list, stored []interface{}) int {
	found := 0
	for i, item := range list {
		var kept interface{}
		if i < len(stored) {
			kept = stored[i]
		}
		switch v := item.(type) {
		case map[string]interface{}:
			child, _ := kept.(map[string]interface{})
			found += resolveRedacted(v, child)
		case []interface{}:
			child, _ := kept.([]interface{})
			found += resolveRedactedList(v, child)
		}
	}
	return found
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

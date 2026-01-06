package database

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func (m *Manager) CreateTemplate(template *ConfigTemplate) error {
	start := time.Now()
	result := m.GetDB().Create(template)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"template_name": template.Name,
			"error":         result.Error.Error(),
			"duration":      duration,
			"operation":     "create",
			"table":         "config_templates",
			"component":     "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"template_id":   template.ID,
		"template_name": template.Name,
		"duration":      duration,
		"operation":     "create",
		"table":         "config_templates",
		"component":     "database",
	}).Info("Template created successfully")

	return nil
}

func (m *Manager) GetTemplate(id uint) (*ConfigTemplate, error) {
	var template ConfigTemplate
	start := time.Now()
	result := m.GetDB().First(&template, id)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"template_id": id,
			"error":       result.Error.Error(),
			"duration":    duration,
			"operation":   "select",
			"table":       "config_templates",
			"component":   "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	return &template, nil
}

func (m *Manager) GetTemplateByName(name string) (*ConfigTemplate, error) {
	var template ConfigTemplate
	start := time.Now()
	result := m.GetDB().Where("name = ?", name).First(&template)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"template_name": name,
			"error":         result.Error.Error(),
			"duration":      duration,
			"operation":     "select",
			"table":         "config_templates",
			"component":     "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	return &template, nil
}

func (m *Manager) GetTemplatesByScope(scope string) ([]ConfigTemplate, error) {
	var templates []ConfigTemplate
	start := time.Now()
	result := m.GetDB().Where("scope = ?", scope).Find(&templates)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"scope":     scope,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "select",
			"table":     "config_templates",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	m.logger.WithFields(map[string]any{
		"scope":     scope,
		"count":     len(templates),
		"duration":  duration,
		"operation": "select",
		"table":     "config_templates",
		"component": "database",
	}).Debug("Retrieved templates by scope")

	return templates, nil
}

func (m *Manager) GetTemplatesByDeviceType(deviceType string) ([]ConfigTemplate, error) {
	var templates []ConfigTemplate
	start := time.Now()
	result := m.GetDB().Where("device_type = ?", deviceType).Find(&templates)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_type": deviceType,
			"error":       result.Error.Error(),
			"duration":    duration,
			"operation":   "select",
			"table":       "config_templates",
			"component":   "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_type": deviceType,
		"count":       len(templates),
		"duration":    duration,
		"operation":   "select",
		"table":       "config_templates",
		"component":   "database",
	}).Debug("Retrieved templates by device type")

	return templates, nil
}

func (m *Manager) UpdateTemplate(template *ConfigTemplate) error {
	start := time.Now()
	result := m.GetDB().Save(template)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"template_id": template.ID,
			"error":       result.Error.Error(),
			"duration":    duration,
			"operation":   "update",
			"table":       "config_templates",
			"component":   "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"template_id": template.ID,
		"duration":    duration,
		"operation":   "update",
		"table":       "config_templates",
		"component":   "database",
	}).Info("Template updated successfully")

	return nil
}

func (m *Manager) DeleteTemplate(id uint) error {
	start := time.Now()
	result := m.GetDB().Delete(&ConfigTemplate{}, id)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"template_id": id,
			"error":       result.Error.Error(),
			"duration":    duration,
			"operation":   "delete",
			"table":       "config_templates",
			"component":   "database",
		}).Error("Database operation failed")
		return result.Error
	}

	if result.RowsAffected == 0 {
		m.logger.WithFields(map[string]any{
			"template_id": id,
			"duration":    duration,
			"operation":   "delete",
			"table":       "config_templates",
			"component":   "database",
		}).Warn("Template not found for deletion")
		return gorm.ErrRecordNotFound
	}

	m.logger.WithFields(map[string]any{
		"template_id": id,
		"duration":    duration,
		"operation":   "delete",
		"table":       "config_templates",
		"component":   "database",
	}).Info("Template deleted successfully")

	return nil
}

func (m *Manager) ListTemplates() ([]ConfigTemplate, error) {
	var templates []ConfigTemplate
	start := time.Now()
	result := m.GetDB().Find(&templates)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "select",
			"table":     "config_templates",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	m.logger.WithFields(map[string]any{
		"count":     len(templates),
		"duration":  duration,
		"operation": "select",
		"table":     "config_templates",
		"component": "database",
	}).Debug("Retrieved all templates")

	return templates, nil
}

func (m *Manager) AddDeviceTag(deviceID uint, tag string) error {
	start := time.Now()
	deviceTag := &DeviceTag{
		DeviceID: deviceID,
		Tag:      tag,
	}

	result := m.GetDB().Where("device_id = ? AND tag = ?", deviceID, tag).FirstOrCreate(deviceTag)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"tag":       tag,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "create",
			"table":     "device_tags",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"tag":       tag,
		"duration":  duration,
		"operation": "create",
		"table":     "device_tags",
		"component": "database",
	}).Info("Device tag added successfully")

	return nil
}

func (m *Manager) RemoveDeviceTag(deviceID uint, tag string) error {
	start := time.Now()
	result := m.GetDB().Where("device_id = ? AND tag = ?", deviceID, tag).Delete(&DeviceTag{})
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"tag":       tag,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "delete",
			"table":     "device_tags",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"tag":       tag,
		"duration":  duration,
		"operation": "delete",
		"table":     "device_tags",
		"component": "database",
	}).Info("Device tag removed successfully")

	return nil
}

func (m *Manager) GetDeviceTags(deviceID uint) ([]string, error) {
	var tags []DeviceTag
	start := time.Now()
	result := m.GetDB().Where("device_id = ?", deviceID).Find(&tags)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "select",
			"table":     "device_tags",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	tagStrings := make([]string, len(tags))
	for i, tag := range tags {
		tagStrings[i] = tag.Tag
	}

	m.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"count":     len(tagStrings),
		"duration":  duration,
		"operation": "select",
		"table":     "device_tags",
		"component": "database",
	}).Debug("Retrieved device tags")

	return tagStrings, nil
}

func (m *Manager) GetDevicesByTag(tag string) ([]Device, error) {
	var deviceTags []DeviceTag
	start := time.Now()
	result := m.GetDB().Where("tag = ?", tag).Find(&deviceTags)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"tag":       tag,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "select",
			"table":     "device_tags",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	if len(deviceTags) == 0 {
		return []Device{}, nil
	}

	deviceIDs := make([]uint, len(deviceTags))
	for i, dt := range deviceTags {
		deviceIDs[i] = dt.DeviceID
	}

	var devices []Device
	result = m.GetDB().Where("id IN ?", deviceIDs).Find(&devices)
	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"tag":       tag,
			"error":     result.Error.Error(),
			"duration":  time.Since(start),
			"operation": "select",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	m.logger.WithFields(map[string]any{
		"tag":       tag,
		"count":     len(devices),
		"duration":  time.Since(start),
		"operation": "select",
		"table":     "device_tags",
		"component": "database",
	}).Debug("Retrieved devices by tag")

	return devices, nil
}

func (m *Manager) ListAllTags() ([]string, error) {
	var tags []string
	start := time.Now()
	result := m.GetDB().Model(&DeviceTag{}).Distinct("tag").Pluck("tag", &tags)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "select",
			"table":     "device_tags",
			"component": "database",
		}).Error("Database operation failed")
		return nil, result.Error
	}

	m.logger.WithFields(map[string]any{
		"count":     len(tags),
		"duration":  duration,
		"operation": "select",
		"table":     "device_tags",
		"component": "database",
	}).Debug("Retrieved all tags")

	return tags, nil
}

func (m *Manager) UpdateDeviceTemplates(deviceID uint, templateIDs []uint) error {
	templateIDsJSON, err := json.Marshal(templateIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal template IDs: %w", err)
	}

	start := time.Now()
	result := m.GetDB().Model(&Device{}).Where("id = ?", deviceID).Update("template_ids", string(templateIDsJSON))
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "update",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"duration":  duration,
		"operation": "update",
		"table":     "devices",
		"component": "database",
	}).Info("Device templates updated successfully")

	return nil
}

func (m *Manager) UpdateDeviceOverrides(deviceID uint, overrides json.RawMessage) error {
	start := time.Now()
	result := m.GetDB().Model(&Device{}).Where("id = ?", deviceID).Update("overrides", string(overrides))
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "update",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"duration":  duration,
		"operation": "update",
		"table":     "devices",
		"component": "database",
	}).Info("Device overrides updated successfully")

	return nil
}

func (m *Manager) UpdateDeviceDesiredConfig(deviceID uint, config json.RawMessage) error {
	start := time.Now()
	result := m.GetDB().Model(&Device{}).Where("id = ?", deviceID).Update("desired_config", string(config))
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "update",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"duration":  duration,
		"operation": "update",
		"table":     "devices",
		"component": "database",
	}).Info("Device desired config updated successfully")

	return nil
}

func (m *Manager) SetDeviceConfigApplied(deviceID uint, applied bool) error {
	start := time.Now()
	result := m.GetDB().Model(&Device{}).Where("id = ?", deviceID).Update("config_applied", applied)
	duration := time.Since(start)

	if result.Error != nil {
		m.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"applied":   applied,
			"error":     result.Error.Error(),
			"duration":  duration,
			"operation": "update",
			"table":     "devices",
			"component": "database",
		}).Error("Database operation failed")
		return result.Error
	}

	m.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"applied":   applied,
		"duration":  duration,
		"operation": "update",
		"table":     "devices",
		"component": "database",
	}).Info("Device config applied status updated successfully")

	return nil
}

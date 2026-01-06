package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

var (
	ErrTemplateNotFound    = errors.New("template not found")
	ErrTemplateAssigned    = errors.New("template is assigned to devices")
	ErrDeviceNotFound      = errors.New("device not found")
	ErrInvalidScope        = errors.New("invalid scope: must be 'global', 'group', or 'device_type'")
	ErrDeviceTypeRequired  = errors.New("device_type required when scope is 'device_type'")
	ErrTemplateIDsNotFound = errors.New("one or more template IDs not found")
)

type ServiceConfigTemplate struct {
	ID          uint            `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Scope       string          `json:"scope"`
	DeviceType  string          `json:"device_type,omitempty"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type ServiceDevice struct {
	ID            uint      `json:"id"`
	TemplateIDs   string    `json:"template_ids"`
	Overrides     string    `json:"overrides"`
	DesiredConfig string    `json:"desired_config"`
	ConfigApplied bool      `json:"config_applied"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ConfigRepository interface {
	CreateTemplate(template *ServiceConfigTemplate) error
	GetTemplate(id uint) (*ServiceConfigTemplate, error)
	UpdateTemplate(template *ServiceConfigTemplate) error
	DeleteTemplate(id uint) error
	ListTemplates() ([]ServiceConfigTemplate, error)
	GetTemplatesByScope(scope string) ([]ServiceConfigTemplate, error)
	GetTemplatesByDeviceType(deviceType string) ([]ServiceConfigTemplate, error)

	GetDevice(id uint) (*ServiceDevice, error)
	GetDevices() ([]ServiceDevice, error)
	UpdateDeviceTemplates(deviceID uint, templateIDs []uint) error
	UpdateDeviceOverrides(deviceID uint, overrides json.RawMessage) error
	UpdateDeviceDesiredConfig(deviceID uint, config json.RawMessage) error
	SetDeviceConfigApplied(deviceID uint, applied bool) error

	AddDeviceTag(deviceID uint, tag string) error
	RemoveDeviceTag(deviceID uint, tag string) error
	GetDeviceTags(deviceID uint) ([]string, error)
	GetDevicesByTag(tag string) ([]ServiceDevice, error)
	ListAllTags() ([]string, error)
}

type Merger interface {
	Merge(layers []ConfigLayer) (*MergeResult, error)
}

type ConfigurationService struct {
	repo   ConfigRepository
	merger Merger
	logger *logging.Logger
}

type ConfigStatus struct {
	DeviceID      uint      `json:"device_id"`
	ConfigApplied bool      `json:"config_applied"`
	HasOverrides  bool      `json:"has_overrides"`
	TemplateCount int       `json:"template_count"`
	LastUpdated   time.Time `json:"last_updated"`
}

func NewConfigurationService(repo ConfigRepository, merger Merger, logger *logging.Logger) *ConfigurationService {
	if logger == nil {
		logger = logging.GetDefault()
	}
	if merger == nil {
		merger = Engine{}
	}
	return &ConfigurationService{
		repo:   repo,
		merger: merger,
		logger: logger,
	}
}

func (s *ConfigurationService) CreateTemplate(template *ServiceConfigTemplate) error {
	if err := s.validateTemplateScope(template); err != nil {
		return err
	}

	if err := s.repo.CreateTemplate(template); err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"template_id":   template.ID,
		"template_name": template.Name,
		"scope":         template.Scope,
		"component":     "config_service",
	}).Info("Template created")

	return nil
}

func (s *ConfigurationService) GetTemplate(id uint) (*ServiceConfigTemplate, error) {
	template, err := s.repo.GetTemplate(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return template, nil
}

func (s *ConfigurationService) UpdateTemplate(template *ServiceConfigTemplate) error {
	if err := s.validateTemplateScope(template); err != nil {
		return err
	}

	if err := s.repo.UpdateTemplate(template); err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	affectedCount, err := s.RecomputeAffectedDevices(template.ID)
	if err != nil {
		s.logger.WithFields(map[string]any{
			"template_id": template.ID,
			"error":       err.Error(),
			"component":   "config_service",
		}).Warn("Failed to recompute affected devices after template update")
	}

	s.logger.WithFields(map[string]any{
		"template_id":      template.ID,
		"template_name":    template.Name,
		"affected_devices": affectedCount,
		"component":        "config_service",
	}).Info("Template updated")

	return nil
}

func (s *ConfigurationService) DeleteTemplate(id uint) error {
	affected, err := s.GetAffectedDevices(id)
	if err != nil {
		return fmt.Errorf("failed to check template usage: %w", err)
	}

	if len(affected) > 0 {
		return fmt.Errorf("%w: used by %d device(s)", ErrTemplateAssigned, len(affected))
	}

	if err := s.repo.DeleteTemplate(id); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"template_id": id,
		"component":   "config_service",
	}).Info("Template deleted")

	return nil
}

func (s *ConfigurationService) ListTemplates(scope string) ([]ServiceConfigTemplate, error) {
	if scope == "" {
		return s.repo.ListTemplates()
	}
	return s.repo.GetTemplatesByScope(scope)
}

func (s *ConfigurationService) GetTemplatesForDeviceType(deviceType string) ([]ServiceConfigTemplate, error) {
	return s.repo.GetTemplatesByDeviceType(deviceType)
}

func (s *ConfigurationService) SetDeviceTemplates(deviceID uint, templateIDs []uint) error {
	if err := s.validateTemplateIDs(templateIDs); err != nil {
		return err
	}

	if err := s.repo.UpdateDeviceTemplates(deviceID, templateIDs); err != nil {
		return fmt.Errorf("failed to set device templates: %w", err)
	}

	if err := s.RecomputeDesiredConfig(deviceID); err != nil {
		return fmt.Errorf("failed to recompute desired config: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"device_id":      deviceID,
		"template_count": len(templateIDs),
		"component":      "config_service",
	}).Info("Device templates updated")

	return nil
}

func (s *ConfigurationService) GetDeviceTemplates(deviceID uint) ([]ServiceConfigTemplate, error) {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	templateIDs, err := s.parseTemplateIDs(device.TemplateIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template IDs: %w", err)
	}

	if len(templateIDs) == 0 {
		return []ServiceConfigTemplate{}, nil
	}

	templates := make([]ServiceConfigTemplate, 0, len(templateIDs))
	for _, id := range templateIDs {
		tmpl, err := s.repo.GetTemplate(id)
		if err != nil {
			continue
		}
		templates = append(templates, *tmpl)
	}

	return templates, nil
}

func (s *ConfigurationService) AddTemplateToDevice(deviceID, templateID uint, position int) error {
	if _, err := s.repo.GetTemplate(templateID); err != nil {
		return ErrTemplateNotFound
	}

	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	templateIDs, err := s.parseTemplateIDs(device.TemplateIDs)
	if err != nil {
		return fmt.Errorf("failed to parse template IDs: %w", err)
	}

	for _, id := range templateIDs {
		if id == templateID {
			return nil
		}
	}

	if position < 0 || position >= len(templateIDs) {
		templateIDs = append(templateIDs, templateID)
	} else {
		templateIDs = append(templateIDs[:position], append([]uint{templateID}, templateIDs[position:]...)...)
	}

	return s.SetDeviceTemplates(deviceID, templateIDs)
}

func (s *ConfigurationService) RemoveTemplateFromDevice(deviceID, templateID uint) error {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	templateIDs, err := s.parseTemplateIDs(device.TemplateIDs)
	if err != nil {
		return fmt.Errorf("failed to parse template IDs: %w", err)
	}

	newIDs := make([]uint, 0, len(templateIDs))
	for _, id := range templateIDs {
		if id != templateID {
			newIDs = append(newIDs, id)
		}
	}

	return s.SetDeviceTemplates(deviceID, newIDs)
}

func (s *ConfigurationService) AddDeviceTag(deviceID uint, tag string) error {
	return s.repo.AddDeviceTag(deviceID, tag)
}

func (s *ConfigurationService) RemoveDeviceTag(deviceID uint, tag string) error {
	return s.repo.RemoveDeviceTag(deviceID, tag)
}

func (s *ConfigurationService) GetDeviceTags(deviceID uint) ([]string, error) {
	return s.repo.GetDeviceTags(deviceID)
}

func (s *ConfigurationService) GetDevicesByTag(tag string) ([]ServiceDevice, error) {
	return s.repo.GetDevicesByTag(tag)
}

func (s *ConfigurationService) ListAllTags() ([]string, error) {
	return s.repo.ListAllTags()
}

func (s *ConfigurationService) SetDeviceOverrides(deviceID uint, overrides *DeviceConfiguration) error {
	overridesJSON, err := json.Marshal(overrides)
	if err != nil {
		return fmt.Errorf("failed to marshal overrides: %w", err)
	}

	if err := s.repo.UpdateDeviceOverrides(deviceID, overridesJSON); err != nil {
		return fmt.Errorf("failed to update overrides: %w", err)
	}

	if err := s.RecomputeDesiredConfig(deviceID); err != nil {
		return fmt.Errorf("failed to recompute desired config: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "config_service",
	}).Info("Device overrides updated")

	return nil
}

func (s *ConfigurationService) GetDeviceOverrides(deviceID uint) (*DeviceConfiguration, error) {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	if device.Overrides == "" || device.Overrides == "{}" {
		return &DeviceConfiguration{}, nil
	}

	var overrides DeviceConfiguration
	if err := json.Unmarshal([]byte(device.Overrides), &overrides); err != nil {
		return nil, fmt.Errorf("failed to unmarshal overrides: %w", err)
	}

	return &overrides, nil
}

func (s *ConfigurationService) PatchDeviceOverrides(deviceID uint, patch *DeviceConfiguration) error {
	existing, err := s.GetDeviceOverrides(deviceID)
	if err != nil {
		return err
	}

	layers := []ConfigLayer{
		{Name: "existing", Config: existing},
		{Name: "patch", Config: patch},
	}

	result, err := s.merger.Merge(layers)
	if err != nil {
		return fmt.Errorf("failed to merge overrides: %w", err)
	}

	return s.SetDeviceOverrides(deviceID, result.Config)
}

func (s *ConfigurationService) ClearDeviceOverrides(deviceID uint) error {
	if err := s.repo.UpdateDeviceOverrides(deviceID, json.RawMessage("{}")); err != nil {
		return fmt.Errorf("failed to clear overrides: %w", err)
	}

	if err := s.RecomputeDesiredConfig(deviceID); err != nil {
		return fmt.Errorf("failed to recompute desired config: %w", err)
	}

	s.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"component": "config_service",
	}).Info("Device overrides cleared")

	return nil
}

func (s *ConfigurationService) GetDesiredConfig(deviceID uint) (*DeviceConfiguration, map[string]string, error) {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get device: %w", err)
	}

	if device.DesiredConfig == "" || device.DesiredConfig == "{}" {
		return &DeviceConfiguration{}, map[string]string{}, nil
	}

	var config DeviceConfiguration
	if unmarshalErr := json.Unmarshal([]byte(device.DesiredConfig), &config); unmarshalErr != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal desired config: %w", unmarshalErr)
	}

	templates, err := s.GetDeviceTemplates(deviceID)
	if err != nil {
		return &config, map[string]string{}, nil
	}

	layers := make([]ConfigLayer, 0, len(templates)+1)
	for _, tmpl := range templates {
		var tmplConfig DeviceConfiguration
		if parseErr := json.Unmarshal(tmpl.Config, &tmplConfig); parseErr != nil {
			continue
		}
		layers = append(layers, ConfigLayer{Name: tmpl.Name, Config: &tmplConfig})
	}

	overrides, _ := s.GetDeviceOverrides(deviceID)
	if overrides != nil && !isEmptyConfig(overrides) {
		layers = append(layers, ConfigLayer{Name: "device-override", Config: overrides})
	}

	if len(layers) == 0 {
		return &config, map[string]string{}, nil
	}

	result, mergeErr := s.merger.Merge(layers)
	if mergeErr != nil {
		return &config, map[string]string{}, nil
	}

	return &config, result.Sources, nil
}

func (s *ConfigurationService) RecomputeDesiredConfig(deviceID uint) error {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	templates, err := s.GetDeviceTemplates(deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device templates: %w", err)
	}

	layers := make([]ConfigLayer, 0, len(templates)+1)
	for _, tmpl := range templates {
		var tmplConfig DeviceConfiguration
		if parseErr := json.Unmarshal(tmpl.Config, &tmplConfig); parseErr != nil {
			s.logger.WithFields(map[string]any{
				"template_id": tmpl.ID,
				"error":       parseErr.Error(),
				"component":   "config_service",
			}).Warn("Failed to parse template config")
			continue
		}
		layers = append(layers, ConfigLayer{Name: tmpl.Name, Config: &tmplConfig})
	}

	if device.Overrides != "" && device.Overrides != "{}" {
		var overrides DeviceConfiguration
		if parseErr := json.Unmarshal([]byte(device.Overrides), &overrides); parseErr == nil {
			if !isEmptyConfig(&overrides) {
				layers = append(layers, ConfigLayer{Name: "device-override", Config: &overrides})
			}
		}
	}

	var desiredConfig *DeviceConfiguration
	if len(layers) > 0 {
		result, mergeErr := s.merger.Merge(layers)
		if mergeErr != nil {
			return fmt.Errorf("failed to merge configurations: %w", mergeErr)
		}
		desiredConfig = result.Config
	} else {
		desiredConfig = &DeviceConfiguration{}
	}

	desiredJSON, err := json.Marshal(desiredConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal desired config: %w", err)
	}

	if err := s.repo.UpdateDeviceDesiredConfig(deviceID, desiredJSON); err != nil {
		return fmt.Errorf("failed to update desired config: %w", err)
	}

	if err := s.repo.SetDeviceConfigApplied(deviceID, false); err != nil {
		return fmt.Errorf("failed to mark config as pending: %w", err)
	}

	return nil
}

func (s *ConfigurationService) GetAffectedDevices(templateID uint) ([]uint, error) {
	devices, err := s.repo.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	affected := []uint{}
	for _, device := range devices {
		templateIDs, err := s.parseTemplateIDs(device.TemplateIDs)
		if err != nil {
			continue
		}
		for _, id := range templateIDs {
			if id == templateID {
				affected = append(affected, device.ID)
				break
			}
		}
	}

	return affected, nil
}

func (s *ConfigurationService) RecomputeAffectedDevices(templateID uint) (int, error) {
	affected, err := s.GetAffectedDevices(templateID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, deviceID := range affected {
		if err := s.RecomputeDesiredConfig(deviceID); err != nil {
			s.logger.WithFields(map[string]any{
				"device_id":   deviceID,
				"template_id": templateID,
				"error":       err.Error(),
				"component":   "config_service",
			}).Warn("Failed to recompute device config")
			continue
		}
		count++
	}

	return count, nil
}

func (s *ConfigurationService) GetConfigStatus(deviceID uint) (*ConfigStatus, error) {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	templateIDs, _ := s.parseTemplateIDs(device.TemplateIDs)
	hasOverrides := device.Overrides != "" && device.Overrides != "{}"

	return &ConfigStatus{
		DeviceID:      device.ID,
		ConfigApplied: device.ConfigApplied,
		HasOverrides:  hasOverrides,
		TemplateCount: len(templateIDs),
		LastUpdated:   device.UpdatedAt,
	}, nil
}

func (s *ConfigurationService) validateTemplateScope(template *ServiceConfigTemplate) error {
	switch template.Scope {
	case "global", "group":
		return nil
	case "device_type":
		if template.DeviceType == "" {
			return ErrDeviceTypeRequired
		}
		return nil
	default:
		return ErrInvalidScope
	}
}

func (s *ConfigurationService) validateTemplateIDs(templateIDs []uint) error {
	for _, id := range templateIDs {
		if _, err := s.repo.GetTemplate(id); err != nil {
			return fmt.Errorf("%w: template ID %d", ErrTemplateIDsNotFound, id)
		}
	}
	return nil
}

func (s *ConfigurationService) parseTemplateIDs(templateIDsJSON string) ([]uint, error) {
	if templateIDsJSON == "" || templateIDsJSON == "[]" {
		return []uint{}, nil
	}

	var ids []uint
	if err := json.Unmarshal([]byte(templateIDsJSON), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func isEmptyConfig(config *DeviceConfiguration) bool {
	if config == nil {
		return true
	}
	return config.WiFi == nil &&
		config.MQTT == nil &&
		config.Auth == nil &&
		config.System == nil &&
		config.Network == nil &&
		config.Cloud == nil &&
		config.Location == nil &&
		config.CoIoT == nil &&
		config.Relay == nil &&
		config.PowerMetering == nil &&
		config.Dimming == nil &&
		config.Roller == nil &&
		config.Input == nil &&
		config.LED == nil
}

package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	// ErrNoDesiredConfig is what a drift check and a verify answer for a device that has no desired
	// configuration: there is nothing to compare the device with, which is not the same as no drift
	// or a match.
	ErrNoDesiredConfig = errors.New("no desired configuration is set for this device, so there is nothing to compare it with")
)

// ErrDesiredChanged is returned by SetDeviceDesiredConfigPending when the stored template ids, overrides
// or desired configuration are no longer what the recompute read: nothing was written.
var ErrDesiredChanged = errors.New("the device configuration changed during the recompute")

// DesiredRead is the stored state a recompute computed its result from. The write applies only while
// the device still holds it.
type DesiredRead struct {
	TemplateIDs string
	Overrides   string
	Desired     string
}

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
	ID uint `json:"id"`
	// MAC is the stored hardware address, in whatever spelling it was entered. Apply compares it
	// with the MAC the device reports before it writes anything.
	MAC           string `json:"mac"`
	TemplateIDs   string `json:"template_ids"`
	Overrides     string `json:"overrides"`
	DesiredConfig string `json:"desired_config"`
	ConfigApplied bool   `json:"config_applied"`
	// PendingReboot is set once an apply has written a setting the device only runs after a
	// reboot, and cleared once that reboot happened.
	PendingReboot bool      `json:"pending_reboot"`
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
	// SetDeviceDesiredConfigPending stores the desired configuration and clears the applied flag
	// in one write, so no failure or concurrent apply can leave the flag true against a changed
	// configuration.
	SetDeviceDesiredConfigPending(deviceID uint, read DesiredRead, config json.RawMessage) error
	// GetDesiredHistory returns one version of the device's desired configuration, or
	// ErrHistoryNotFound when the id is not one of this device's.
	GetDesiredHistory(deviceID, historyID uint) (*DesiredConfigHistory, error)
	// RollbackDeviceDesired stores a rollback's template ids, overrides and desired configuration,
	// pending, with its history row, in one transaction, and only while the device still holds read.
	RollbackDeviceDesired(deviceID uint, read DesiredRead, write DesiredWrite, restoredFrom uint) error
	SetDeviceConfigApplied(deviceID uint, applied bool) error
	// SetDeviceConfigAppliedIfDesired sets the applied flag only while the stored desired
	// configuration still equals desiredConfig. It reports whether the flag was set.
	SetDeviceConfigAppliedIfDesired(deviceID uint, desiredConfig string) (bool, error)
	// SetDevicePendingReboot writes the pending-reboot flag with one targeted UPDATE of that
	// column, so no whole-row device write can race it.
	SetDevicePendingReboot(deviceID uint, pending bool) error

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

	// applyLocks holds one lock per device ID, a channel of capacity one: a full channel is a held
	// lock. It serializes applies to the same device so a slow apply cannot overwrite a newer one and
	// leave the flag claiming applied. A waiter stops waiting when its context ends.
	applyLocks sync.Map

	// noApplyLockOnce logs once that the repository offers no cross-replica apply lock.
	noApplyLockOnce sync.Once

	// loginHooks is set once at startup by the code that owns the device credentials. While it
	// is nil, apply refuses a login instead of writing one it could not store.
	loginHooks LoginHooks
}

// SetLoginHooks lets apply write a device's login. Call it before the service handles requests.
func (s *ConfigurationService) SetLoginHooks(hooks LoginHooks) {
	s.loginHooks = hooks
}

// lockApply serializes applies to one device and returns the unlock function, which is safe to call
// more than once. It waits until the lock is free or ctx ends and then returns the context's error;
// a waiter that gives up leaves nothing behind, because it never took the lock. A free lock is taken
// even when ctx has already ended.
func (s *ConfigurationService) lockApply(ctx context.Context, deviceID uint) (func(), error) {
	l, _ := s.applyLocks.LoadOrStore(deviceID, make(chan struct{}, 1))
	lock := l.(chan struct{})
	select {
	case lock <- struct{}{}:
	default:
		select {
		case lock <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var once sync.Once
	return func() { once.Do(func() { <-lock }) }, nil
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

// PatchDeviceOverrides merges patch into the stored overrides. A path in removed was sent as JSON
// null: the merge leaves it out, and it is removed from the result so the device inherits it again.
func (s *ConfigurationService) PatchDeviceOverrides(deviceID uint, patch *DeviceConfiguration, removed [][]string) error {
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

	// A path through a channel list holds the index in the patch body, and the merge puts a channel
	// where its id is.
	RemovePaths(result.Config, remapChannelPaths(removed, existing, patch))

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

	config, err := parseDesiredConfig(device.DesiredConfig)
	if err != nil {
		return nil, nil, err
	}

	templates, err := s.GetDeviceTemplates(deviceID)
	if err != nil {
		return config, map[string]string{}, nil
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
		return config, map[string]string{}, nil
	}

	result, mergeErr := s.merger.Merge(layers)
	if mergeErr != nil {
		return config, map[string]string{}, nil
	}

	return config, result.Sources, nil
}

// GetInheritedConfig returns what the device falls back to when its own overrides are removed:
// the merge of its template layers alone, with the template each value comes from. The desired
// configuration cannot give this, because the merge keeps only the winning layer of a field, so
// a field that a template and the device override both set shows no template value there.
func (s *ConfigurationService) GetInheritedConfig(deviceID uint) (*DeviceConfiguration, map[string]string, error) {
	templates, err := s.GetDeviceTemplates(deviceID)
	if err != nil {
		return nil, nil, err
	}

	layers := make([]ConfigLayer, 0, len(templates))
	for _, tmpl := range templates {
		var tmplConfig DeviceConfiguration
		if parseErr := json.Unmarshal(tmpl.Config, &tmplConfig); parseErr != nil {
			continue
		}
		layers = append(layers, ConfigLayer{Name: tmpl.Name, Config: &tmplConfig})
	}

	result, mergeErr := s.merger.Merge(layers)
	if mergeErr != nil {
		return nil, nil, fmt.Errorf("failed to merge template configurations: %w", mergeErr)
	}
	return result.Config, result.Sources, nil
}

// parseDesiredConfig decodes a stored desired configuration. An empty value, or an empty
// object, is an empty configuration.
func parseDesiredConfig(raw string) (*DeviceConfiguration, error) {
	if raw == "" || raw == "{}" {
		return &DeviceConfiguration{}, nil
	}
	var config DeviceConfiguration
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal desired config: %w", err)
	}
	return &config, nil
}

// hasDesiredConfig reports whether a device has a desired configuration at all: an empty value,
// null and an empty object, however it is spelled, all mean none. A value that is not valid JSON
// counts as set, so the parse that follows reports it.
func hasDesiredConfig(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return true
	}
	if object, ok := v.(map[string]any); ok {
		return len(object) > 0
	}
	return v != nil
}

// RequireDesiredConfig returns ErrNoDesiredConfig when the device has no desired configuration. A
// drift run calls it before it contacts the device, so a device that cannot be checked is never
// asked for a client or a login.
func (s *ConfigurationService) RequireDesiredConfig(deviceID uint) error {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeviceNotFound, err)
	}
	if !hasDesiredConfig(device.DesiredConfig) {
		return ErrNoDesiredConfig
	}
	return nil
}

// parseApplicableDesiredConfig decodes a stored desired configuration for apply and verify.
// It refuses fields the typed configuration does not know, such as a legacy device_configs
// row restored by an SMA import, because a plain decode would drop them: apply would then
// write nothing and verify would report a match.
func parseApplicableDesiredConfig(raw string) (*DeviceConfiguration, error) {
	if raw == "" || raw == "{}" {
		return &DeviceConfiguration{}, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var config DeviceConfiguration
	if err := decoder.Decode(&config); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return nil, fmt.Errorf("%w: %v", ErrUnsupportedSettings, err)
		}
		return nil, fmt.Errorf("failed to unmarshal desired config: %w", err)
	}
	return &config, nil
}

// StoredDesiredConfig decodes the desired configuration stored with a device, for a read that hands
// it out, such as a download. It returns ErrNoDesiredConfig when the device has none, and
// ErrUnsupportedSettings when the typed configuration would drop a field on decode, so a lossy
// decode is never presented as the whole configuration. It works on the value the caller already
// read, so the answer and the configuration come from one snapshot.
func StoredDesiredConfig(raw string) (*DeviceConfiguration, error) {
	if !hasDesiredConfig(raw) {
		return nil, ErrNoDesiredConfig
	}
	return parseApplicableDesiredConfig(raw)
}

// recomputeAttempts is how many times a recompute reads and writes before it gives up on a device that
// keeps changing under it.
const recomputeAttempts = 3

// transactional is implemented by a repository that works inside a transaction its caller owns.
type transactional interface{ InTransaction() bool }

// RecomputeDesiredConfig rebuilds the desired configuration from the templates and overrides and
// stores it, pending. The write applies only to the state it read, so on a miss it reads again, a
// bounded number of times. Inside a transaction a second read can see the same snapshot and miss
// again, so there a miss returns the error at once: the caller fails.
func (s *ConfigurationService) RecomputeDesiredConfig(deviceID uint) error {
	attempts := recomputeAttempts
	if tx, ok := s.repo.(transactional); ok && tx.InTransaction() {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		if err = s.recomputeDesiredConfigOnce(deviceID); !errors.Is(err, ErrDesiredChanged) {
			return err
		}
	}
	return err
}

func (s *ConfigurationService) recomputeDesiredConfigOnce(deviceID uint) error {
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

	if layer, ok := overrideLayer(device.Overrides); ok {
		layers = append(layers, layer)
	}

	desiredJSON, err := s.desiredFromLayers(layers)
	if err != nil {
		return err
	}

	if err := s.repo.SetDeviceDesiredConfigPending(deviceID, DesiredRead{TemplateIDs: device.TemplateIDs, Overrides: device.Overrides, Desired: device.DesiredConfig}, desiredJSON); err != nil {
		return fmt.Errorf("failed to store desired config as pending: %w", err)
	}

	return nil
}

// overrideLayer is how the recompute reads a device's overrides text: an empty text, "{}", text that does
// not parse and a configuration with no section are no layer. A rollback reads its target the same way,
// so it never accepts a text the recompute would read differently.
func overrideLayer(text string) (ConfigLayer, bool) {
	if text == "" || text == "{}" {
		return ConfigLayer{}, false
	}
	var overrides DeviceConfiguration
	if json.Unmarshal([]byte(text), &overrides) != nil || isEmptyConfig(&overrides) {
		return ConfigLayer{}, false
	}
	return ConfigLayer{Name: "device-override", Config: &overrides}, true
}

// desiredFromLayers merges the layers and marshals the result, the recompute's own step: no layer is the
// empty configuration.
func (s *ConfigurationService) desiredFromLayers(layers []ConfigLayer) (json.RawMessage, error) {
	desiredConfig := &DeviceConfiguration{}
	if len(layers) > 0 {
		result, err := s.merger.Merge(layers)
		if err != nil {
			return nil, fmt.Errorf("failed to merge configurations: %w", err)
		}
		desiredConfig = result.Config
	}
	desiredJSON, err := json.Marshal(desiredConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal desired config: %w", err)
	}
	return desiredJSON, nil
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
	return ValidateTemplateScope(template.Scope, template.DeviceType)
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
		config.LED == nil &&
		config.Actions == nil
}

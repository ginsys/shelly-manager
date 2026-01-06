package configuration

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository is a test double for ConfigRepository
type mockRepository struct {
	templates      map[uint]*ServiceConfigTemplate
	devices        map[uint]*ServiceDevice
	deviceTags     map[uint][]string
	tagDevices     map[string][]uint
	nextTemplateID uint
	createErr      error
	getErr         error
	updateErr      error
	deleteErr      error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		templates:      make(map[uint]*ServiceConfigTemplate),
		devices:        make(map[uint]*ServiceDevice),
		deviceTags:     make(map[uint][]string),
		tagDevices:     make(map[string][]uint),
		nextTemplateID: 1,
	}
}

func (m *mockRepository) CreateTemplate(template *ServiceConfigTemplate) error {
	if m.createErr != nil {
		return m.createErr
	}
	template.ID = m.nextTemplateID
	m.nextTemplateID++
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()
	m.templates[template.ID] = template
	return nil
}

func (m *mockRepository) GetTemplate(id uint) (*ServiceConfigTemplate, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if t, ok := m.templates[id]; ok {
		return t, nil
	}
	return nil, ErrTemplateNotFound
}

func (m *mockRepository) UpdateTemplate(template *ServiceConfigTemplate) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.templates[template.ID]; !ok {
		return ErrTemplateNotFound
	}
	template.UpdatedAt = time.Now()
	m.templates[template.ID] = template
	return nil
}

func (m *mockRepository) DeleteTemplate(id uint) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.templates[id]; !ok {
		return ErrTemplateNotFound
	}
	delete(m.templates, id)
	return nil
}

func (m *mockRepository) ListTemplates() ([]ServiceConfigTemplate, error) {
	result := make([]ServiceConfigTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, *t)
	}
	return result, nil
}

func (m *mockRepository) GetTemplatesByScope(scope string) ([]ServiceConfigTemplate, error) {
	result := []ServiceConfigTemplate{}
	for _, t := range m.templates {
		if t.Scope == scope {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockRepository) GetTemplatesByDeviceType(deviceType string) ([]ServiceConfigTemplate, error) {
	result := []ServiceConfigTemplate{}
	for _, t := range m.templates {
		if t.DeviceType == deviceType {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockRepository) GetDevice(id uint) (*ServiceDevice, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if d, ok := m.devices[id]; ok {
		return d, nil
	}
	return nil, ErrDeviceNotFound
}

func (m *mockRepository) GetDevices() ([]ServiceDevice, error) {
	result := make([]ServiceDevice, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, *d)
	}
	return result, nil
}

func (m *mockRepository) UpdateDeviceTemplates(deviceID uint, templateIDs []uint) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}
	idsJSON, _ := json.Marshal(templateIDs)
	m.devices[deviceID].TemplateIDs = string(idsJSON)
	m.devices[deviceID].UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) UpdateDeviceOverrides(deviceID uint, overrides json.RawMessage) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}
	m.devices[deviceID].Overrides = string(overrides)
	m.devices[deviceID].UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) UpdateDeviceDesiredConfig(deviceID uint, config json.RawMessage) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}
	m.devices[deviceID].DesiredConfig = string(config)
	m.devices[deviceID].UpdatedAt = time.Now()
	return nil
}

func (m *mockRepository) SetDeviceConfigApplied(deviceID uint, applied bool) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}
	m.devices[deviceID].ConfigApplied = applied
	return nil
}

func (m *mockRepository) AddDeviceTag(deviceID uint, tag string) error {
	if _, ok := m.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}
	if m.deviceTags[deviceID] == nil {
		m.deviceTags[deviceID] = []string{}
	}
	m.deviceTags[deviceID] = append(m.deviceTags[deviceID], tag)
	if m.tagDevices[tag] == nil {
		m.tagDevices[tag] = []uint{}
	}
	m.tagDevices[tag] = append(m.tagDevices[tag], deviceID)
	return nil
}

func (m *mockRepository) RemoveDeviceTag(deviceID uint, tag string) error {
	if _, ok := m.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}
	newTags := []string{}
	for _, t := range m.deviceTags[deviceID] {
		if t != tag {
			newTags = append(newTags, t)
		}
	}
	m.deviceTags[deviceID] = newTags
	return nil
}

func (m *mockRepository) GetDeviceTags(deviceID uint) ([]string, error) {
	if _, ok := m.devices[deviceID]; !ok {
		return nil, ErrDeviceNotFound
	}
	return m.deviceTags[deviceID], nil
}

func (m *mockRepository) GetDevicesByTag(tag string) ([]ServiceDevice, error) {
	result := []ServiceDevice{}
	for _, id := range m.tagDevices[tag] {
		if d, ok := m.devices[id]; ok {
			result = append(result, *d)
		}
	}
	return result, nil
}

func (m *mockRepository) ListAllTags() ([]string, error) {
	tags := []string{}
	for tag := range m.tagDevices {
		tags = append(tags, tag)
	}
	return tags, nil
}

func (m *mockRepository) addDevice(id uint) {
	m.devices[id] = &ServiceDevice{
		ID:            id,
		TemplateIDs:   "[]",
		Overrides:     "{}",
		DesiredConfig: "{}",
		ConfigApplied: true,
		UpdatedAt:     time.Now(),
	}
}

// --- Tests ---

func TestNewConfigurationService(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, nil, nil)
	require.NotNil(t, svc)
	assert.NotNil(t, svc.repo)
	assert.NotNil(t, svc.merger)
	assert.NotNil(t, svc.logger)
}

func TestConfigurationService_CreateTemplate_Valid(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
		},
	}
	configJSON, _ := json.Marshal(config)

	template := &ServiceConfigTemplate{
		Name:   "Test Template",
		Scope:  "global",
		Config: configJSON,
	}

	err := svc.CreateTemplate(template)
	require.NoError(t, err)
	assert.Equal(t, uint(1), template.ID)
	assert.NotZero(t, template.CreatedAt)
}

func TestConfigurationService_CreateTemplate_InvalidScope(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	template := &ServiceConfigTemplate{
		Name:  "Test Template",
		Scope: "invalid_scope",
	}

	err := svc.CreateTemplate(template)
	assert.ErrorIs(t, err, ErrInvalidScope)
}

func TestConfigurationService_CreateTemplate_DeviceTypeScopeRequiresType(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	template := &ServiceConfigTemplate{
		Name:  "Test Template",
		Scope: "device_type",
	}

	err := svc.CreateTemplate(template)
	assert.ErrorIs(t, err, ErrDeviceTypeRequired)

	template.DeviceType = "SHPLG-S"
	err = svc.CreateTemplate(template)
	require.NoError(t, err)
}

func TestConfigurationService_GetTemplate(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	original := &ServiceConfigTemplate{
		Name:  "Test",
		Scope: "global",
	}
	_ = svc.CreateTemplate(original)

	retrieved, err := svc.GetTemplate(original.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test", retrieved.Name)
}

func TestConfigurationService_UpdateTemplate(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	template := &ServiceConfigTemplate{
		Name:  "Original",
		Scope: "global",
	}
	_ = svc.CreateTemplate(template)

	template.Name = "Updated"
	err := svc.UpdateTemplate(template)
	require.NoError(t, err)

	retrieved, _ := svc.GetTemplate(template.ID)
	assert.Equal(t, "Updated", retrieved.Name)
}

func TestConfigurationService_DeleteTemplate_NotAssigned(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	template := &ServiceConfigTemplate{
		Name:  "Test",
		Scope: "global",
	}
	_ = svc.CreateTemplate(template)

	err := svc.DeleteTemplate(template.ID)
	require.NoError(t, err)

	_, err = svc.GetTemplate(template.ID)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestConfigurationService_DeleteTemplate_AssignedFails(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	template := &ServiceConfigTemplate{
		Name:  "Test",
		Scope: "global",
	}
	_ = svc.CreateTemplate(template)

	// Assign template to device
	err := svc.SetDeviceTemplates(1, []uint{template.ID})
	require.NoError(t, err)

	// Try to delete - should fail
	err = svc.DeleteTemplate(template.ID)
	assert.ErrorIs(t, err, ErrTemplateAssigned)
}

func TestConfigurationService_ListTemplates(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	_ = svc.CreateTemplate(&ServiceConfigTemplate{Name: "Global1", Scope: "global"})
	_ = svc.CreateTemplate(&ServiceConfigTemplate{Name: "Global2", Scope: "global"})
	_ = svc.CreateTemplate(&ServiceConfigTemplate{Name: "Group1", Scope: "group"})

	all, err := svc.ListTemplates("")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	globalOnly, err := svc.ListTemplates("global")
	require.NoError(t, err)
	assert.Len(t, globalOnly, 2)
}

func TestConfigurationService_SetDeviceTemplates(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Create templates
	t1 := &ServiceConfigTemplate{Name: "T1", Scope: "global"}
	t2 := &ServiceConfigTemplate{Name: "T2", Scope: "global"}
	_ = svc.CreateTemplate(t1)
	_ = svc.CreateTemplate(t2)

	// Assign to device
	err := svc.SetDeviceTemplates(1, []uint{t1.ID, t2.ID})
	require.NoError(t, err)

	// Verify assignment
	templates, err := svc.GetDeviceTemplates(1)
	require.NoError(t, err)
	assert.Len(t, templates, 2)
}

func TestConfigurationService_SetDeviceTemplates_InvalidTemplate(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	err := svc.SetDeviceTemplates(1, []uint{999})
	assert.ErrorIs(t, err, ErrTemplateIDsNotFound)
}

func TestConfigurationService_AddRemoveTemplateFromDevice(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	t1 := &ServiceConfigTemplate{Name: "T1", Scope: "global"}
	t2 := &ServiceConfigTemplate{Name: "T2", Scope: "global"}
	_ = svc.CreateTemplate(t1)
	_ = svc.CreateTemplate(t2)

	// Add template
	err := svc.AddTemplateToDevice(1, t1.ID, -1)
	require.NoError(t, err)

	templates, _ := svc.GetDeviceTemplates(1)
	assert.Len(t, templates, 1)

	// Add another at position 0
	err = svc.AddTemplateToDevice(1, t2.ID, 0)
	require.NoError(t, err)

	templates, _ = svc.GetDeviceTemplates(1)
	assert.Len(t, templates, 2)
	assert.Equal(t, "T2", templates[0].Name)

	// Remove template
	err = svc.RemoveTemplateFromDevice(1, t2.ID)
	require.NoError(t, err)

	templates, _ = svc.GetDeviceTemplates(1)
	assert.Len(t, templates, 1)
	assert.Equal(t, "T1", templates[0].Name)
}

func TestConfigurationService_DeviceTags(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	repo.addDevice(2)
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Add tags
	err := svc.AddDeviceTag(1, "kitchen")
	require.NoError(t, err)
	err = svc.AddDeviceTag(1, "lights")
	require.NoError(t, err)
	err = svc.AddDeviceTag(2, "kitchen")
	require.NoError(t, err)

	// Get device tags
	tags, err := svc.GetDeviceTags(1)
	require.NoError(t, err)
	assert.Contains(t, tags, "kitchen")
	assert.Contains(t, tags, "lights")

	// Get devices by tag
	devices, err := svc.GetDevicesByTag("kitchen")
	require.NoError(t, err)
	assert.Len(t, devices, 2)

	// List all tags
	allTags, err := svc.ListAllTags()
	require.NoError(t, err)
	assert.Contains(t, allTags, "kitchen")
	assert.Contains(t, allTags, "lights")

	// Remove tag
	err = svc.RemoveDeviceTag(1, "kitchen")
	require.NoError(t, err)

	tags, _ = svc.GetDeviceTags(1)
	assert.NotContains(t, tags, "kitchen")
}

func TestConfigurationService_DeviceOverrides(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Set overrides
	overrides := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("device.broker"),
		},
	}

	err := svc.SetDeviceOverrides(1, overrides)
	require.NoError(t, err)

	// Get overrides
	retrieved, err := svc.GetDeviceOverrides(1)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.MQTT)
	assert.Equal(t, "device.broker", *retrieved.MQTT.Server)

	// Patch overrides
	patch := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Port: IntPtr(8883),
		},
	}

	err = svc.PatchDeviceOverrides(1, patch)
	require.NoError(t, err)

	retrieved, _ = svc.GetDeviceOverrides(1)
	assert.Equal(t, "device.broker", *retrieved.MQTT.Server)
	assert.Equal(t, 8883, *retrieved.MQTT.Port)

	// Clear overrides
	err = svc.ClearDeviceOverrides(1)
	require.NoError(t, err)

	retrieved, _ = svc.GetDeviceOverrides(1)
	assert.Nil(t, retrieved.MQTT)
}

func TestConfigurationService_RecomputeDesiredConfig(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Create a template with config
	globalConfig := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("global.broker"),
			Port:   IntPtr(1883),
		},
	}
	configJSON, _ := json.Marshal(globalConfig)

	template := &ServiceConfigTemplate{
		Name:   "Global MQTT",
		Scope:  "global",
		Config: configJSON,
	}
	_ = svc.CreateTemplate(template)

	// Assign template to device
	err := svc.SetDeviceTemplates(1, []uint{template.ID})
	require.NoError(t, err)

	// Check that desired config was computed
	device, _ := repo.GetDevice(1)
	assert.NotEqual(t, "{}", device.DesiredConfig)
	assert.False(t, device.ConfigApplied)

	var desired DeviceConfiguration
	_ = json.Unmarshal([]byte(device.DesiredConfig), &desired)
	assert.Equal(t, "global.broker", *desired.MQTT.Server)
}

func TestConfigurationService_GetDesiredConfig(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Create template
	globalConfig := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("global.broker"),
		},
	}
	configJSON, _ := json.Marshal(globalConfig)

	template := &ServiceConfigTemplate{
		Name:   "Global",
		Scope:  "global",
		Config: configJSON,
	}
	_ = svc.CreateTemplate(template)
	_ = svc.SetDeviceTemplates(1, []uint{template.ID})

	// Add override
	overrides := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Port: IntPtr(8883),
		},
	}
	_ = svc.SetDeviceOverrides(1, overrides)

	// Get desired config with sources
	config, sources, err := svc.GetDesiredConfig(1)
	require.NoError(t, err)
	assert.NotNil(t, config.MQTT)
	assert.Equal(t, "Global", sources["mqtt.server"])
	assert.Equal(t, "device-override", sources["mqtt.port"])
}

func TestConfigurationService_GetConfigStatus(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Create and assign template
	template := &ServiceConfigTemplate{Name: "T", Scope: "global"}
	_ = svc.CreateTemplate(template)
	_ = svc.SetDeviceTemplates(1, []uint{template.ID})

	// Add override
	_ = svc.SetDeviceOverrides(1, &DeviceConfiguration{
		MQTT: &MQTTConfiguration{Server: StringPtr("test")},
	})

	status, err := svc.GetConfigStatus(1)
	require.NoError(t, err)
	assert.Equal(t, uint(1), status.DeviceID)
	assert.False(t, status.ConfigApplied)
	assert.True(t, status.HasOverrides)
	assert.Equal(t, 1, status.TemplateCount)
}

func TestConfigurationService_RecomputeAffectedDevices(t *testing.T) {
	repo := newMockRepository()
	repo.addDevice(1)
	repo.addDevice(2)
	repo.addDevice(3)
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Create template
	template := &ServiceConfigTemplate{Name: "Shared", Scope: "global"}
	_ = svc.CreateTemplate(template)

	// Assign to devices 1 and 2
	_ = svc.SetDeviceTemplates(1, []uint{template.ID})
	_ = svc.SetDeviceTemplates(2, []uint{template.ID})

	// Get affected devices
	affected, err := svc.GetAffectedDevices(template.ID)
	require.NoError(t, err)
	assert.Len(t, affected, 2)
	assert.Contains(t, affected, uint(1))
	assert.Contains(t, affected, uint(2))
	assert.NotContains(t, affected, uint(3))

	// Recompute affected
	count, err := svc.RecomputeAffectedDevices(template.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestConfigurationService_RepositoryErrors(t *testing.T) {
	repo := newMockRepository()
	svc := NewConfigurationService(repo, Engine{}, nil)

	// Test create error
	repo.createErr = errors.New("db error")
	err := svc.CreateTemplate(&ServiceConfigTemplate{Name: "T", Scope: "global"})
	assert.Error(t, err)
	repo.createErr = nil

	// Test get error
	repo.getErr = errors.New("db error")
	_, err = svc.GetTemplate(1)
	assert.Error(t, err)
	repo.getErr = nil
}

package database

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTemplate(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	config := json.RawMessage(`{"mqtt":{"server":"broker.local"}}`)
	template := &ConfigTemplate{
		Name:        "test-template",
		Description: "Test template",
		Scope:       "global",
		Config:      config,
	}

	err := manager.CreateTemplate(template)
	require.NoError(t, err)
	assert.NotZero(t, template.ID)
	assert.Equal(t, "test-template", template.Name)
}

func TestCreateTemplate_UniqueConstraint(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	config := json.RawMessage(`{}`)
	template1 := &ConfigTemplate{
		Name:   "duplicate-name",
		Scope:  "global",
		Config: config,
	}

	err := manager.CreateTemplate(template1)
	require.NoError(t, err)

	template2 := &ConfigTemplate{
		Name:   "duplicate-name",
		Scope:  "group",
		Config: config,
	}

	err = manager.CreateTemplate(template2)
	assert.Error(t, err)
}

func TestGetTemplate(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	config := json.RawMessage(`{"wifi":{"ssid":"TestNet"}}`)
	template := &ConfigTemplate{
		Name:   "get-test",
		Scope:  "global",
		Config: config,
	}

	err := manager.CreateTemplate(template)
	require.NoError(t, err)

	retrieved, err := manager.GetTemplate(template.ID)
	require.NoError(t, err)
	assert.Equal(t, template.ID, retrieved.ID)
	assert.Equal(t, "get-test", retrieved.Name)
	assert.JSONEq(t, string(config), string(retrieved.Config))
}

func TestGetTemplateByName(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	config := json.RawMessage(`{"auth":{"enable":true}}`)
	template := &ConfigTemplate{
		Name:       "name-lookup",
		Scope:      "device_type",
		DeviceType: "SHPLG-S",
		Config:     config,
	}

	err := manager.CreateTemplate(template)
	require.NoError(t, err)

	retrieved, err := manager.GetTemplateByName("name-lookup")
	require.NoError(t, err)
	assert.Equal(t, template.ID, retrieved.ID)
	assert.Equal(t, "SHPLG-S", retrieved.DeviceType)
}

func TestGetTemplatesByScope(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	config := json.RawMessage(`{}`)

	templates := []*ConfigTemplate{
		{Name: "global-1", Scope: "global", Config: config},
		{Name: "global-2", Scope: "global", Config: config},
		{Name: "group-1", Scope: "group", Config: config},
	}

	for _, tmpl := range templates {
		err := manager.CreateTemplate(tmpl)
		require.NoError(t, err)
	}

	globalTemplates, err := manager.GetTemplatesByScope("global")
	require.NoError(t, err)
	assert.Len(t, globalTemplates, 2)

	groupTemplates, err := manager.GetTemplatesByScope("group")
	require.NoError(t, err)
	assert.Len(t, groupTemplates, 1)
}

func TestUpdateTemplate(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	config := json.RawMessage(`{"mqtt":{"server":"old.local"}}`)
	template := &ConfigTemplate{
		Name:   "update-test",
		Scope:  "global",
		Config: config,
	}

	err := manager.CreateTemplate(template)
	require.NoError(t, err)

	newConfig := json.RawMessage(`{"mqtt":{"server":"new.local"}}`)
	template.Config = newConfig
	template.Description = "Updated description"

	err = manager.UpdateTemplate(template)
	require.NoError(t, err)

	retrieved, err := manager.GetTemplate(template.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", retrieved.Description)
	assert.JSONEq(t, string(newConfig), string(retrieved.Config))
}

func TestDeleteTemplate(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	config := json.RawMessage(`{}`)
	template := &ConfigTemplate{
		Name:   "delete-test",
		Scope:  "global",
		Config: config,
	}

	err := manager.CreateTemplate(template)
	require.NoError(t, err)

	err = manager.DeleteTemplate(template.ID)
	require.NoError(t, err)

	_, err = manager.GetTemplate(template.ID)
	assert.Error(t, err)
}

func TestAddDeviceTag(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC:  "AA:BB:CC:DD:EE:01",
		IP:   "192.168.1.1",
		Name: "test-device",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)

	err = manager.AddDeviceTag(device.ID, "office")
	require.NoError(t, err)

	tags, err := manager.GetDeviceTags(device.ID)
	require.NoError(t, err)
	assert.Contains(t, tags, "office")
}

func TestAddDeviceTag_Duplicate(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC:  "AA:BB:CC:DD:EE:02",
		IP:   "192.168.1.2",
		Name: "test-device-2",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)

	err = manager.AddDeviceTag(device.ID, "production")
	require.NoError(t, err)

	err = manager.AddDeviceTag(device.ID, "production")
	require.NoError(t, err)

	tags, err := manager.GetDeviceTags(device.ID)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
}

func TestRemoveDeviceTag(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC:  "AA:BB:CC:DD:EE:03",
		IP:   "192.168.1.3",
		Name: "test-device-3",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)

	err = manager.AddDeviceTag(device.ID, "staging")
	require.NoError(t, err)

	err = manager.RemoveDeviceTag(device.ID, "staging")
	require.NoError(t, err)

	tags, err := manager.GetDeviceTags(device.ID)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestGetDeviceTags(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC:  "AA:BB:CC:DD:EE:04",
		IP:   "192.168.1.4",
		Name: "test-device-4",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)

	tags := []string{"office", "critical", "monitored"}
	for _, tag := range tags {
		err = manager.AddDeviceTag(device.ID, tag)
		require.NoError(t, err)
	}

	retrieved, err := manager.GetDeviceTags(device.ID)
	require.NoError(t, err)
	assert.Len(t, retrieved, 3)
	assert.ElementsMatch(t, tags, retrieved)
}

func TestGetDevicesByTag(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device1 := &Device{
		MAC:  "AA:BB:CC:DD:EE:05",
		IP:   "192.168.1.5",
		Name: "device-1",
	}
	device2 := &Device{
		MAC:  "AA:BB:CC:DD:EE:06",
		IP:   "192.168.1.6",
		Name: "device-2",
	}

	err := manager.AddDevice(device1)
	require.NoError(t, err)
	err = manager.AddDevice(device2)
	require.NoError(t, err)

	err = manager.AddDeviceTag(device1.ID, "production")
	require.NoError(t, err)
	err = manager.AddDeviceTag(device2.ID, "production")
	require.NoError(t, err)

	devices, err := manager.GetDevicesByTag("production")
	require.NoError(t, err)
	assert.Len(t, devices, 2)
}

func TestUpdateDeviceTemplates(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC:  "AA:BB:CC:DD:EE:07",
		IP:   "192.168.1.7",
		Name: "test-device-7",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)

	templateIDs := []uint{1, 2, 3}
	err = manager.UpdateDeviceTemplates(device.ID, templateIDs)
	require.NoError(t, err)

	retrieved, err := manager.GetDevice(device.ID)
	require.NoError(t, err)

	var retrievedIDs []uint
	err = json.Unmarshal([]byte(retrieved.TemplateIDs), &retrievedIDs)
	require.NoError(t, err)
	assert.Equal(t, templateIDs, retrievedIDs)
}

func TestMigration_ExistingDevices(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	device := &Device{
		MAC:  "AA:BB:CC:DD:EE:08",
		IP:   "192.168.1.8",
		Name: "existing-device",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)

	retrieved, err := manager.GetDevice(device.ID)
	require.NoError(t, err)

	assert.Equal(t, "[]", retrieved.TemplateIDs)
	assert.Equal(t, "{}", retrieved.Overrides)
	assert.Equal(t, "{}", retrieved.DesiredConfig)
	assert.False(t, retrieved.ConfigApplied)
}

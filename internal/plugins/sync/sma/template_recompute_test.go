package sma

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/sync"
)

const (
	staleDesired    = `{"mqtt":{"enable":false}}`
	archivedMQTTOn  = `{"mqtt":{"enable":true}}`
	outsideDeviceIP = "192.0.2.30"
)

// recomputeFixture holds a template and a device outside the archive that uses it. The archive
// still holds the template's earlier config, so a forced import changes the stored template.
type recomputeFixture struct {
	*persistFixture
	archive    string
	templateID uint
	outsideID  uint
}

func newRecomputeFixture(t *testing.T) *recomputeFixture {
	t.Helper()
	f := newPersistFixture(t)
	f.seed(t)
	archive := f.export(t)

	var template configuration.ConfigTemplate
	require.NoError(t, f.db.GetDB().Where("name = ?", "plug-defaults").First(&template).Error)
	require.NoError(t, f.db.GetDB().Model(&template).Update("config", json.RawMessage(staleDesired)).Error)

	outside := &database.Device{
		IP: outsideDeviceIP, MAC: "ffeeddccbbaa", Type: "Smart Plug", Name: "outside", Status: "online",
		TemplateIDs: "[" + itoa(template.ID) + "]", DesiredConfig: staleDesired, ConfigApplied: true,
	}
	require.NoError(t, f.db.AddDevice(outside))
	return &recomputeFixture{persistFixture: f, archive: archive, templateID: template.ID, outsideID: outside.ID}
}

func itoa(id uint) string {
	encoded, _ := json.Marshal(id)
	return string(encoded)
}

func (f *recomputeFixture) device(t *testing.T, id uint) database.Device {
	t.Helper()
	var device database.Device
	require.NoError(t, f.db.GetDB().First(&device, id).Error)
	return device
}

func mqttEnabled(t *testing.T, desired string) bool {
	t.Helper()
	var parsed struct {
		MQTT *struct {
			Enable bool `json:"enable"`
		} `json:"mqtt"`
	}
	require.NoError(t, json.Unmarshal([]byte(desired), &parsed), desired)
	require.NotNil(t, parsed.MQTT, "the merged desired configuration has an mqtt section: %s", desired)
	return parsed.MQTT.Enable
}

// A forced import that replaces a template must refresh the devices that use it, as saving the
// template through the API does. A device outside the archive keeps no other route to the change.
func TestForcedImportRecomputesADeviceOutsideTheArchive(t *testing.T) {
	f := newRecomputeFixture(t)

	_, err := f.importFile(t, f.archive, sync.ImportOptions{ForceOverwrite: true})

	require.NoError(t, err)
	outside := f.device(t, f.outsideID)
	assert.True(t, mqttEnabled(t, outside.DesiredConfig), "the restored template now shapes the device: %s", outside.DesiredConfig)
	assert.False(t, outside.ConfigApplied, "the new configuration has not reached the device")
}

// A device the archive restores with its own desired configuration keeps exactly that
// configuration: recomputing it would overwrite what the import just wrote.
func TestForcedImportKeepsTheDesiredConfigOfADeviceItRestores(t *testing.T) {
	f := newRecomputeFixture(t)
	var inArchive database.Device
	require.NoError(t, f.db.GetDB().Where("mac = ?", "aabbccddeeff").First(&inArchive).Error)
	require.NoError(t, f.db.GetDB().Model(&inArchive).Update("template_ids", "["+itoa(f.templateID)+"]").Error)

	_, err := f.importFile(t, f.archive, sync.ImportOptions{ForceOverwrite: true})

	require.NoError(t, err)
	restored := f.device(t, inArchive.ID)
	assert.JSONEq(t, persistDesiredConfig, restored.DesiredConfig, "the archived desired configuration is the point of the archive")
}

// The API path logs a device it cannot recompute and carries on. An import is all or nothing, so
// the same failure must roll the replaced template back with it.
func TestForcedImportRollsBackWhenADeviceCannotBeRecomputed(t *testing.T) {
	f := newRecomputeFixture(t)
	require.NoError(t, f.db.GetDB().Exec(
		`CREATE TRIGGER refuse_desired_update BEFORE UPDATE OF desired_config ON devices
		 WHEN NEW.id = `+itoa(f.outsideID)+` BEGIN SELECT RAISE(ABORT, 'desired_config refused'); END`).Error)

	_, err := f.importFile(t, f.archive, sync.ImportOptions{ForceOverwrite: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "recompute device")
	var template configuration.ConfigTemplate
	require.NoError(t, f.db.GetDB().First(&template, f.templateID).Error)
	assert.JSONEq(t, staleDesired, string(template.Config), "the replaced template was rolled back")
	outside := f.device(t, f.outsideID)
	assert.True(t, outside.ConfigApplied)
}

// Without force a stored template is left alone, so nothing is recomputed.
func TestImportWithoutForceRecomputesNothing(t *testing.T) {
	f := newRecomputeFixture(t)

	_, err := f.importFile(t, f.archive, sync.ImportOptions{})

	require.NoError(t, err)
	outside := f.device(t, f.outsideID)
	assert.JSONEq(t, staleDesired, outside.DesiredConfig)
	assert.True(t, outside.ConfigApplied, "an untouched device stays applied")
}

// A validate-only or dry-run import writes nothing, so it recomputes nothing either.
func TestPreviewImportRecomputesNothing(t *testing.T) {
	for name, options := range map[string]sync.ImportOptions{
		"validate_only": {ValidateOnly: true, ForceOverwrite: true},
		"dry_run":       {DryRun: true, ForceOverwrite: true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newRecomputeFixture(t)

			_, err := f.importFile(t, f.archive, options)

			require.NoError(t, err)
			outside := f.device(t, f.outsideID)
			assert.JSONEq(t, staleDesired, outside.DesiredConfig)
			assert.True(t, outside.ConfigApplied)
		})
	}
}

package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Drift compares the live device with the desired configuration, the one apply writes and verify
// reads, and never with the stored device_configs row.

const driftTestMAC = "AA:BB:CC:DD:EE:FF" // the MAC createTestDevice stores

// desiredNamed is a desired configuration that sets only the device name.
func desiredNamed(t *testing.T, name string) string {
	t.Helper()
	raw, err := json.Marshal(&DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr(name)}},
	})
	require.NoError(t, err)
	return string(raw)
}

// newDriftService returns a service whose device 1 carries the given desired configuration. The
// devices table of this package has no desired_config column, so the desired configuration is
// served by a mock repository, as the apply tests do.
func newDriftService(t *testing.T, desired string) (*Service, *mockRepository, *gorm.DB) {
	t.Helper()
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")
	repo := newMockRepository()
	repo.devices[1] = &ServiceDevice{ID: 1, MAC: driftTestMAC, DesiredConfig: desired}
	service.ConfigurationSvc = NewConfigurationService(repo, nil, nil)
	return service, repo, db
}

// driftClient is a Gen1 device that reports liveName as its name and the given MAC.
func driftClient(liveName, mac string) *mockShellyClient {
	c := new(mockShellyClient)
	c.On("GetInfo", mock.Anything).Return(&shelly.DeviceInfo{ID: "dev", MAC: mac, Generation: 1, Model: "SHSW-1"}, nil).Maybe()
	c.On("GetConfig", mock.Anything).Return(&shelly.DeviceConfig{
		Raw: json.RawMessage(fmt.Sprintf(`{"name":%q}`, liveName)),
	}, nil).Maybe()
	c.On("GetGeneration").Return(1).Maybe()
	c.On("GetIP").Return("192.168.1.100").Maybe()
	return c
}

// A device just applied holds its desired configuration, so it is not drifted, whatever the stored
// import record says.
func TestDetectDrift_AppliedDeviceReportsNoDrift(t *testing.T) {
	service, _, db := newDriftService(t, desiredNamed(t, "tv-office"))
	require.NoError(t, db.Create(&DeviceConfig{
		DeviceID: 1, Config: json.RawMessage(`{"name":"old-import"}`), SyncStatus: "synced",
	}).Error)

	drift, err := service.DetectDrift(1, driftClient("tv-office", driftTestMAC))

	require.NoError(t, err)
	assert.Nil(t, drift, "the device holds its desired configuration")
}

// A device that left its desired configuration is drifted, even when it still equals the stored
// import record.
func TestDetectDrift_DeviceThatLeftItsDesiredConfigReportsDrift(t *testing.T) {
	service, _, db := newDriftService(t, desiredNamed(t, "tv-office"))
	require.NoError(t, db.Create(&DeviceConfig{
		DeviceID: 1, Config: json.RawMessage(`{"name":"somebody-renamed-it"}`), SyncStatus: "synced",
	}).Error)
	notified := 0
	service.SetDriftNotifier(func(_ context.Context, _ uint, _ string, _ int) { notified++ })

	drift, err := service.DetectDrift(1, driftClient("somebody-renamed-it", driftTestMAC))

	require.NoError(t, err)
	require.NotNil(t, drift, "the live name differs from the desired one")
	assert.Equal(t, "Test Device", drift.DeviceName)
	assert.True(t, drift.RequiresAction)
	require.NotEmpty(t, drift.Differences)
	assert.Contains(t, drift.Differences[0].Path, "name")
	assert.Equal(t, 1, notified)
}

// A device with no desired configuration has no baseline: that is an error and the device is
// never read, never "in sync".
func TestDetectDrift_NoDesiredConfigIsNotInSync(t *testing.T) {
	for _, desired := range []string{"", "{}", "null", "  {}  ", "{ }", "{\n}", " null\n"} {
		t.Run(fmt.Sprintf("%q", desired), func(t *testing.T) {
			service, _, _ := newDriftService(t, desired)
			client := driftClient("anything", driftTestMAC)

			drift, err := service.DetectDrift(1, client)

			assert.Nil(t, drift)
			assert.ErrorIs(t, err, ErrNoDesiredConfig)
			client.AssertNotCalled(t, "GetConfig", mock.Anything)
		})
	}
}

// The check needs no stored row and writes nothing: the stored row, if any, keeps its status and
// the history stays empty.
func TestDetectDrift_NeedsNoStoredRowAndWritesNothing(t *testing.T) {
	service, _, db := newDriftService(t, desiredNamed(t, "tv-office"))

	drift, err := service.DetectDrift(1, driftClient("other", driftTestMAC))
	require.NoError(t, err)
	require.NotNil(t, drift)

	var rows, histories int64
	require.NoError(t, db.Model(&DeviceConfig{}).Count(&rows).Error)
	require.NoError(t, db.Model(&ConfigHistory{}).Count(&histories).Error)
	assert.Zero(t, rows, "a drift check created a stored row")
	assert.Zero(t, histories)

	require.NoError(t, db.Create(&DeviceConfig{
		DeviceID: 1, Config: json.RawMessage(`{"name":"x"}`), SyncStatus: "pending",
	}).Error)
	before := readDriftCheckState(t, db, 1)
	_, err = service.DetectDrift(1, driftClient("other", driftTestMAC))
	require.NoError(t, err)
	requireBaselineUntouched(t, before, readDriftCheckState(t, db, 1), "pending")
}

// A reading that moves on its own (the device clock) is not drift: the desired configuration holds
// only settings, so a live device that differs from the one it was applied from by a clock tick
// still matches it.
func TestDetectDrift_MovingDeviceClockIsNotDrift(t *testing.T) {
	fixture, err := os.ReadFile("testdata/shplg_s_settings.json")
	require.NoError(t, err)
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text"})
	desiredCfg, err := NewGen1Converter(logger).FromAPIConfig(fixture, "SHPLG-S")
	require.NoError(t, err)
	desired, err := json.Marshal(desiredCfg)
	require.NoError(t, err)

	var live map[string]any
	require.NoError(t, json.Unmarshal(fixture, &live))
	live["time"] = "23:59"
	live["unixtime"] = 1999999999
	liveRaw, err := json.Marshal(live)
	require.NoError(t, err)

	service, _, db := newDriftService(t, string(desired))
	require.NoError(t, db.Model(&Device{}).Where("id = ?", 1).Update("type", "SHPLG-S").Error)
	client := new(mockShellyClient)
	client.On("GetInfo", mock.Anything).Return(&shelly.DeviceInfo{ID: "dev", MAC: driftTestMAC, Generation: 1, Model: "SHPLG-S"}, nil)
	client.On("GetConfig", mock.Anything).Return(&shelly.DeviceConfig{Raw: liveRaw}, nil)
	client.On("GetGeneration").Return(1).Maybe()
	client.On("GetIP").Return("192.168.1.100").Maybe()

	drift, err := service.DetectDrift(1, client)

	require.NoError(t, err)
	assert.Nil(t, drift, "a moved clock is not a configuration difference")
}

// A device that answers with another MAC is not compared: verify refuses, and so does drift.
func TestDetectDrift_WrongMACIsRefused(t *testing.T) {
	service, _, _ := newDriftService(t, desiredNamed(t, "tv-office"))

	drift, err := service.DetectDrift(1, driftClient("tv-office", "11:22:33:44:55:66"))

	assert.Nil(t, drift)
	assert.ErrorIs(t, err, ErrDeviceIdentity)
}

// In a bulk run a device with no desired configuration is not checked: it is skipped with the
// reason, it does not count as an error (so stopOnError does not skip the rest because of it), and
// the report leaves it out of its totals.
func TestBulkDetectDrift_NoDesiredConfigIsSkippedNotAnError(t *testing.T) {
	service, repo, db := newDriftService(t, "")
	createTestDevice(t, db, 2, "Second", "SHSW-1")
	repo.devices[2] = &ServiceDevice{ID: 2, MAC: driftTestMAC, DesiredConfig: desiredNamed(t, "tv-office")}
	var asked []uint
	getter := func(id uint) (shelly.Client, error) {
		asked = append(asked, id)
		if id == 1 {
			// A login failure on the device that cannot be checked must not become its answer.
			return nil, fmt.Errorf("authentication failed")
		}
		return driftClient("tv-office", driftTestMAC), nil
	}

	result, err := service.BulkDetectDrift([]uint{1, 2}, getter, true)
	assert.Equal(t, []uint{2}, asked, "a device with no desired configuration is not contacted")

	require.NoError(t, err)
	assert.Equal(t, 0, result.Errors)
	assert.Equal(t, 1, result.Skipped)
	assert.Equal(t, 1, result.InSync, "the device after the one without a desired configuration was still checked")
	require.Len(t, result.Results, 2)
	assert.Equal(t, BulkStatusSkipped, result.Results[0].Status)
	assert.Contains(t, result.Results[0].Error, "no desired configuration")
	assert.Equal(t, "synced", result.Results[1].Status)

	report, err := service.EnhanceBulkDriftResult(result, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Summary.TotalDevices, "a device that was not checked has no place in the totals")
}

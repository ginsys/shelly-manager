package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// fakeGen1Device keeps Gen1 settings between SetConfig and GetConfig, like a
// real device. With ignoreWrites set it accepts writes but changes nothing.
type fakeGen1Device struct {
	applyTestClient
	settings     map[string]interface{}
	ignoreWrites bool
	setCalls     int
}

func newFakeGen1Device() *fakeGen1Device {
	d := &fakeGen1Device{settings: map[string]interface{}{"name": ""}}
	d.ip = "192.168.1.50"
	d.generation = 1
	return d
}

func (d *fakeGen1Device) SetConfig(_ context.Context, cfg map[string]interface{}) error {
	d.setCalls++
	if d.setErr != nil {
		return d.setErr
	}
	if d.ignoreWrites {
		return nil
	}
	for k, v := range cfg {
		d.settings[k] = v
	}
	return nil
}

func (d *fakeGen1Device) GetConfig(_ context.Context) (*shelly.DeviceConfig, error) {
	raw, err := json.Marshal(d.settings)
	if err != nil {
		return nil, err
	}
	return &shelly.DeviceConfig{Raw: raw}, nil
}

func newServiceWithDesiredName(t *testing.T, name string) (*ConfigurationService, *mockRepository) {
	t.Helper()
	repo := newMockRepository()
	desired, err := json.Marshal(&DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr(name)}},
	})
	require.NoError(t, err)
	repo.devices[1] = &ServiceDevice{ID: 1, DesiredConfig: string(desired)}
	return NewConfigurationService(repo, nil, nil), repo
}

func TestApplyDesiredConfig_MarksAppliedOnVerifiedMatch(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, "tv-office", dev.settings["name"])
	assert.Equal(t, 0, result.ApplyResult.FailedCount)
	assert.Greater(t, result.ApplyResult.AppliedCount, 0)
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_StaysPendingWhenDeviceIgnoresWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.ignoreWrites = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied, "read-back differs, so not applied")
	require.NotNil(t, result.VerifyResult)
	assert.False(t, result.VerifyResult.Match)
	assert.NotEmpty(t, result.VerifyResult.Differences)
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_StaysPendingWhenDeviceRejectsWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.setErr = errors.New("401 unauthorized")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, result.ApplyResult.Success)
	assert.Greater(t, result.ApplyResult.FailedCount, 0)
	assert.Equal(t, 0, result.ApplyResult.AppliedCount)
	assert.Nil(t, result.VerifyResult, "no verify after a failed apply")
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_Gen2IsRefusedWithoutWriting(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.generation = 2

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedGeneration))
	assert.Equal(t, 0, dev.setCalls)
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_UnknownDevice(t *testing.T) {
	svc, _ := newServiceWithDesiredName(t, "tv-office")

	_, err := svc.ApplyDesiredConfig(context.Background(), 99, newFakeGen1Device(), "SHPLG-S")

	require.Error(t, err)
}

func TestVerifyDesiredConfig_ReportsDriftAndWritesNothing(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Match)
	require.NotEmpty(t, result.Differences)
	assert.Equal(t, 0, dev.setCalls)
	assert.False(t, repo.devices[1].ConfigApplied)

	dev.settings["name"] = "tv-office"
	result, err = svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
	require.NoError(t, err)
	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
}

func TestVerifyDesiredConfig_Gen2IsRefused(t *testing.T) {
	svc, _ := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.generation = 3

	_, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "x")

	assert.True(t, errors.Is(err, ErrUnsupportedGeneration))
}

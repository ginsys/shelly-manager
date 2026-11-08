package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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
	posted       []map[string]interface{}
	onWrite      func() // runs after each accepted write, to simulate a concurrent edit
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
	d.posted = append(d.posted, cfg)
	if d.ignoreWrites {
		return nil
	}
	for k, v := range cfg {
		// Like a real Gen1 device: flat mqtt_* parameters in, a nested "mqtt" document out.
		if sub, ok := strings.CutPrefix(k, "mqtt_"); ok {
			doc, _ := d.settings["mqtt"].(map[string]interface{})
			if doc == nil {
				doc = map[string]interface{}{}
			}
			doc[sub] = v
			d.settings["mqtt"] = doc
			continue
		}
		d.settings[k] = v
	}
	if d.onWrite != nil {
		d.onWrite()
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

func setDesired(t *testing.T, repo *mockRepository, cfg *DeviceConfiguration) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo.devices[1].DesiredConfig = string(raw)
}

func TestApplyDesiredConfig_MQTTIsSentFlatAndVerifies(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("broker.local"),
			Port:   IntPtr(1883),
			User:   StringPtr("iot"),
		},
	})
	dev := newFakeGen1Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "read-back of the nested mqtt document matches")
	for _, posted := range dev.posted {
		assert.NotContains(t, posted, "mqtt", "a nested map must never reach the device")
	}
	mqtt, _ := dev.settings["mqtt"].(map[string]interface{})
	assert.Equal(t, "broker.local:1883", mqtt["server"])
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_RefusesUnsupportedSettingsBeforeWriting(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office")}},
		WiFi:   &WiFiConfiguration{SSID: StringPtr("other-net"), Password: StringPtr("s3cret")},
	})
	repo.devices[1].ConfigApplied = true
	dev := newFakeGen1Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "wifi_sta")
	assert.NotContains(t, err.Error(), "s3cret")
	assert.Equal(t, 0, dev.setCalls, "nothing may be written, not even the supported name")
	assert.Equal(t, "", dev.settings["name"])
	assert.True(t, repo.devices[1].ConfigApplied, "a refused request leaves the flag alone")
}

func TestApplyDesiredConfig_RefusesSettingsTheConverterDrops(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{Name: StringPtr("tv-office")},
			SNTP:   &SNTPConfig{Server: "ntp.example.net"},
		},
	})
	dev := newFakeGen1Device()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "sntp")
	assert.Equal(t, 0, dev.setCalls, "the supported name must not be written either")
}

func TestApplyDesiredConfig_MQTTPasswordNeverReadBackStillVerifies(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable:   BoolPtr(true),
			Server:   StringPtr("broker.local"),
			Port:     IntPtr(1883),
			Password: StringPtr("s3cret"),
		},
	})
	dev := newFakeGen1Device() // like a real Gen1 device, never returns mqtt.pass

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "a write-only secret must not keep the device pending")
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_CustomLocationDisablesAutoDetect(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		Location: &LocationConfiguration{Timezone: StringPtr("Europe/Brussels")},
	})
	dev := newFakeGen1Device()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	require.NotEmpty(t, dev.posted)
	assert.Equal(t, false, dev.posted[0]["tzautodetect"])
	assert.Equal(t, "Europe/Brussels", dev.posted[0]["timezone"])
}

func TestApplyDesiredConfig_SerializesPerDevice(t *testing.T) {
	svc, _ := newServiceWithDesiredName(t, "tv-office")
	release := svc.lockApply(1)

	done := make(chan struct{})
	go func() {
		_, _ = svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("a second apply to the same device ran while the first held the lock")
	case <-time.After(100 * time.Millisecond):
	}
	release()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the second apply did not run after the lock was released")
	}
}

func TestApplyDesiredConfig_FailedReapplyClearsAppliedFlag(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	repo.devices[1].ConfigApplied = true // previously applied, then the device drifted
	dev := newFakeGen1Device()
	dev.ignoreWrites = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied, "status must not keep claiming applied")
}

func TestApplyDesiredConfig_ConfigChangedDuringApplyStaysPending(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.onWrite = func() {
		// An override edit lands while the device is being written: recompute stores a newer
		// desired configuration and marks the device pending.
		setDesired(t, repo, &DeviceConfiguration{
			System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("newer-name")}},
		})
		repo.devices[1].ConfigApplied = false
	}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied, "the newer configuration was never sent")
	assert.Contains(t, strings.Join(result.ApplyResult.Warnings, " "), "changed while it was being applied")
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

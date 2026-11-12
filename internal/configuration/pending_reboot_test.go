package configuration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// mqttAndSNTP is a desired configuration with one reboot-needing write (MQTT, applied before)
// and one write that comes after it (SNTP).
func mqttAndSNTP() *DeviceConfiguration {
	return &DeviceConfiguration{
		MQTT:   &MQTTConfiguration{UpdatePeriod: IntPtr(60)},
		System: &SystemConfiguration{SNTP: &SNTPConfig{Server: "ntp.example.net"}},
	}
}

// newDeviceWithMQTT is a Gen1 device that reports its mqtt document, as a real one does, so a
// changed value can be told from the stored one.
func newDeviceWithMQTT() *fakeGen1Device {
	dev := newFakeGen1Device()
	dev.settings["mqtt"] = map[string]interface{}{"update_period": float64(30)}
	return dev
}

// A write that needs a reboot succeeds, a later write fails, and the retry finds the new value
// already stored. The retry must still reboot: the device has not run that value yet.
func TestApplyDesiredConfig_RetryAfterFailedWriteStillReboots(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, mqttAndSNTP())
	dev := newDeviceWithMQTT()
	dev.onWrite = func() { dev.setErr = errors.New("device busy") } // every write after the MQTT one fails

	first, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
	require.NoError(t, err)
	require.False(t, first.ApplyResult.Success, "the SNTP write failed")
	assert.Zero(t, dev.reboots, "a failed apply does not reboot")
	assert.True(t, repo.devices[1].PendingReboot, "the MQTT write is waiting for a reboot")

	dev.onWrite, dev.setErr = nil, nil
	retry, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, retry.ApplyResult.RequiresReboot, "the stored MQTT value matches, but it was never run")
	assert.Equal(t, 1, dev.reboots)
	assert.True(t, retry.ConfigApplied)
	assert.False(t, repo.devices[1].PendingReboot, "cleared once the device rebooted")
}

// A reboot that fails leaves the flag set, so the next apply tries again.
func TestApplyDesiredConfig_FailedRebootKeepsThePendingFlag(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(60)}})
	dev := newDeviceWithMQTT()
	dev.rebootErr = errors.New("no answer")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.ErrorContains(t, err, "reboot failed")
	assert.True(t, repo.devices[1].PendingReboot)

	dev.rebootErr = nil
	retry, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
	require.NoError(t, err)
	assert.True(t, retry.ApplyResult.RequiresReboot)
	assert.False(t, repo.devices[1].PendingReboot)
}

// A refused write may still have landed (the answer is lost, the device applies it late), so it
// owes the reboot like a write that succeeded. The cost of being wrong is one extra reboot.
func TestApplyDesiredConfig_RefusedWriteStillOwesTheReboot(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(60)}})
	dev := newDeviceWithMQTT()
	dev.setErr = errors.New("device busy")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ApplyResult.Success)
	assert.True(t, result.ApplyResult.RequiresReboot)
	assert.True(t, repo.devices[1].PendingReboot)
}

// lostAnswer stores a write on the device and then reports an error, like a connection that drops
// after the device accepted the request.
type lostAnswer struct{ *fakeGen1Device }

func (c *lostAnswer) SetConfig(ctx context.Context, cfg map[string]interface{}) error {
	if err := c.fakeGen1Device.SetConfig(ctx, cfg); err != nil {
		return err
	}
	return errors.New("connection reset by peer")
}

func TestApplyDesiredConfig_WriteWhoseAnswerWasLostStillRebootsOnRetry(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(60)}})
	dev := newDeviceWithMQTT()

	first, err := svc.ApplyDesiredConfig(context.Background(), 1, &lostAnswer{dev}, "SHPLG-S")
	require.NoError(t, err)
	require.False(t, first.ApplyResult.Success)
	mqtt, _ := dev.settings["mqtt"].(map[string]interface{})
	require.EqualValues(t, 60, mqtt["update_period"], "the device stored the value")

	retry, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, retry.ApplyResult.RequiresReboot)
	assert.Equal(t, 1, dev.reboots)
	assert.False(t, repo.devices[1].PendingReboot)
}

// A setting that needs no reboot never sets the flag, and an apply that rebooted leaves it clear.
func TestApplyDesiredConfig_NoRebootSettingLeavesTheFlagClear(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newDeviceWithMQTT()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ApplyResult.RequiresReboot)
	assert.Zero(t, dev.reboots)
	assert.False(t, repo.devices[1].PendingReboot)
}

// The reboot is recorded before the first write. If it cannot be stored nothing is written, so a
// retry has lost nothing, and once the store works again the retry applies and reboots.
func TestApplyDesiredConfig_WritesNothingWhenThePendingFlagCannotBeStored(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(60)}})
	repo.pendingRebootErr = errors.New("database is locked")
	dev := newDeviceWithMQTT()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.ErrorContains(t, err, "database is locked")
	assert.Zero(t, dev.setCalls, "nothing was written")
	assert.Zero(t, dev.reboots)

	repo.pendingRebootErr = nil
	retry, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
	require.NoError(t, err)
	assert.True(t, retry.ApplyResult.RequiresReboot)
	assert.Equal(t, 1, dev.reboots)
	assert.True(t, retry.ConfigApplied)
}

// unreadable fails the first read of the settings, like a device that does not answer for a moment.
type unreadable struct {
	*fakeGen1Device
	failures int
}

func (c *unreadable) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	if c.failures > 0 {
		c.failures--
		return nil, errors.New("timeout")
	}
	return c.fakeGen1Device.GetConfig(ctx)
}

// Without the stored settings the reboot cannot be decided, so the apply stops before writing: it
// could otherwise write the MQTT value, fail on a later write and leave nothing owed.
func TestApplyDesiredConfig_StopsBeforeWritingWhenTheSettingsCannotBeRead(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, mqttAndSNTP())
	dev := newDeviceWithMQTT()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, &unreadable{fakeGen1Device: dev, failures: 1}, "SHPLG-S")

	require.ErrorContains(t, err, "nothing was written")
	assert.Zero(t, dev.setCalls)
	assert.False(t, repo.devices[1].PendingReboot)

	retry, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
	require.NoError(t, err)
	assert.True(t, retry.ApplyResult.RequiresReboot)
	assert.Equal(t, 1, dev.reboots)
}

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

// A station network change makes the device reconnect about a second after the write, possibly on
// another network or address. These tests cover what the apply reports when the device then stops
// answering. The fake device is on "home", enabled, and has no ipv4mode in its document.

var errNoRoute = errors.New("dial tcp 192.168.1.60:80: i/o timeout")

// flakyGen2Device loses the answer of the station write and stops answering reads once a write was
// attempted, like a device that moved to another network.
type flakyGen2Device struct {
	*fakeGen2Device
	writeErr  error           // the answer of the WiFi.SetConfig call
	dropWrite bool            // the write is not stored when its answer is lost
	readErr   error           // GetConfig fails with this once a write was attempted
	readRaw   json.RawMessage // GetConfig returns this document once a write was attempted
	attempted bool
}

func (d *flakyGen2Device) CallConfigSetter(ctx context.Context, method string, params map[string]interface{}) (bool, error) {
	d.attempted = true
	if d.writeErr != nil && method == "WiFi.SetConfig" {
		if !d.dropWrite {
			_, _ = d.fakeGen2Device.CallConfigSetter(ctx, method, params)
		}
		return false, d.writeErr
	}
	return d.fakeGen2Device.CallConfigSetter(ctx, method, params)
}

func (d *flakyGen2Device) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	if d.attempted {
		if d.readErr != nil {
			return nil, d.readErr
		}
		if d.readRaw != nil {
			return &shelly.DeviceConfig{Raw: d.readRaw}, nil
		}
	}
	return d.fakeGen2Device.GetConfig(ctx)
}

func newFlakyGen2Device() *flakyGen2Device {
	return &flakyGen2Device{fakeGen2Device: newFakeGen2Device()}
}

func deviceAnswer() error {
	return &shelly.DeviceError{IP: "192.168.1.60", Generation: 2, Operation: "GetConfig", StatusCode: 500, Message: "internal error"}
}

func staticStationDesired() *DeviceConfiguration {
	return &DeviceConfiguration{WiFi: &WiFiConfiguration{
		IPv4Mode: StringPtr("static"),
		StaticIP: &StaticIPConfig{
			IP: StringPtr("192.168.1.61"), Netmask: StringPtr("255.255.255.0"),
			Gateway: StringPtr("192.168.1.1"), Nameserver: StringPtr("192.168.1.2"),
		},
	}}
}

// assertUnconfirmed checks the state of a station change that nothing could confirm: neither a
// success nor a failure, no error, and the device stays pending.
func assertUnconfirmed(t *testing.T, result *ApplyVerifyResult, repo *mockRepository) {
	t.Helper()
	require.NotNil(t, result)
	require.NotNil(t, result.ApplyResult)
	assert.True(t, result.ApplyResult.Unconfirmed)
	assert.Contains(t, result.ApplyResult.Warnings, wifiUnconfirmedWarning)
	assert.Empty(t, result.ApplyResult.Failures)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_Gen2ANewNetworkThatStopsAnsweringIsUnconfirmed(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assertUnconfirmed(t, result, repo)
}

func TestApplyDesiredConfig_Gen2AStaticAddressThatStopsAnsweringIsUnconfirmed(t *testing.T) {
	svc, repo := newGen2Service(t, staticStationDesired())
	dev := newFlakyGen2Device()
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assertUnconfirmed(t, result, repo)
}

func TestApplyDesiredConfig_Gen2AChangeFromDHCPToStaticThatStopsAnsweringIsUnconfirmed(t *testing.T) {
	svc, repo := newGen2Service(t, staticStationDesired())
	dev := newFlakyGen2Device()
	dev.doc["wifi"].(map[string]interface{})["sta"].(map[string]interface{})["ipv4mode"] = "dhcp"
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assertUnconfirmed(t, result, repo)
}

// The reboot the device asked for is the next device read: a device that has moved never answers it.
func TestApplyDesiredConfig_Gen2ARebootThatGetsNoAnswerAfterANetworkChangeIsUnconfirmed(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.restart = true
	dev.rebootErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assertUnconfirmed(t, result, repo)
	assert.True(t, result.ApplyResult.RequiresReboot, "the reboot stays owed")
}

// A non-ssid network key is a network change too: the station is never written blind.
func TestApplyDesiredConfig_Gen2RefusesANetworkKeyWriteWhenTheDeviceCannotBeRead(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{IPv4Mode: StringPtr("dhcp")}}
	svc, repo := newGen2Service(t, desired)
	dev := unreadableGen2Device{newFakeGen2Device()}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Empty(t, dev.calls)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// A write whose answer is lost may have landed: it is neither a failure nor an applied setting, so
// the read that follows decides. Here the read is silent too.
func TestApplyDesiredConfig_Gen2ALostWriteAnswerThenSilenceIsUnconfirmed(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.writeErr = errNoRoute
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assertUnconfirmed(t, result, repo)
	assert.False(t, result.ApplyResult.RequiresReboot, "the restart answer is lost, so none is requested")
}

// The same lost answer, but the device answers the read and holds the new network.
func TestApplyDesiredConfig_Gen2ALostWriteAnswerThatLandedVerifies(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.writeErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Empty(t, result.ApplyResult.Failures)
	assert.False(t, result.ApplyResult.Unconfirmed)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	assert.True(t, repo.devices[1].ConfigApplied)
}

// The same lost answer, the write never landed, the device answers the read: the read reports the
// difference, which is neither a failure of the write nor unconfirmed.
func TestApplyDesiredConfig_Gen2ALostWriteAnswerThatDidNotLandShowsTheDifference(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.writeErr = errNoRoute
	dev.dropWrite = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Empty(t, result.ApplyResult.Failures)
	assert.False(t, result.ApplyResult.Unconfirmed)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
	require.NotNil(t, result.VerifyResult)
	assert.False(t, result.VerifyResult.Match)
}

// Guards: the rule does not widen past a network change that nobody could see.

func TestApplyDesiredConfig_Gen2AChangeThatMovesNothingKeepsTheReadError(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{SSID: StringPtr("home"), Enable: BoolPtr(true)}}
	svc, _ := newGen2Service(t, desired)
	dev := newFlakyGen2Device()
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errNoRoute)
}

func TestApplyDesiredConfig_Gen2APasswordAloneKeepsTheReadError(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{Password: StringPtr("wifi-pass-2")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFlakyGen2Device()
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestApplyDesiredConfig_Gen2ANameServerAloneKeepsTheReadError(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{StaticIP: &StaticIPConfig{Nameserver: StringPtr("192.168.1.9")}}}
	svc, _ := newGen2Service(t, desired)
	dev := newFlakyGen2Device()
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestApplyDesiredConfig_Gen2ADeviceThatAnswersTheReadWithAnErrorKeepsIt(t *testing.T) {
	svc, _ := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.readErr = deviceAnswer()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	var deviceErr *shelly.DeviceError
	assert.ErrorAs(t, err, &deviceErr)
}

// A device that answers with a document that does not convert is on its network: no silence.
func TestApplyDesiredConfig_Gen2ADocumentThatDoesNotConvertKeepsTheError(t *testing.T) {
	svc, _ := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.readRaw = json.RawMessage(`[]`)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestApplyDesiredConfig_Gen2AnotherSectionThatStopsAnsweringKeepsTheReadError(t *testing.T) {
	desired := &DeviceConfiguration{System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}}}
	svc, _ := newGen2Service(t, desired)
	dev := newFlakyGen2Device()
	dev.readErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
}

// A write the device refused with an answer is a failure, whatever the network change.
func TestApplyDesiredConfig_Gen2AWriteThatTheDeviceRefusedWithAnAnswerStaysAFailure(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := newFlakyGen2Device()
	dev.writeErr = deviceAnswer()
	dev.dropWrite = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ApplyResult.Unconfirmed)
	assert.NotEmpty(t, result.ApplyResult.Failures)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// A lost answer with no network change (a password alone) stays a failure.
func TestApplyDesiredConfig_Gen2ALostWriteAnswerWithNoNetworkChangeStaysAFailure(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{Password: StringPtr("wifi-pass-2")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFlakyGen2Device()
	dev.writeErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ApplyResult.Unconfirmed)
	assert.NotEmpty(t, result.ApplyResult.Failures)
}

package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mqttDesired() *DeviceConfiguration {
	return &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable:             BoolPtr(true),
			Server:             StringPtr("broker.example.net"),
			Port:               IntPtr(1883),
			User:               StringPtr("iot"),
			Password:           StringPtr("s3cret-pass"),
			ClientID:           StringPtr("lamp"),
			TopicPrefix:        StringPtr("home/lamp"),
			RPCNotifications:   BoolPtr(true),
			StatusNotification: BoolPtr(false),
		},
		Cloud: &CloudConfiguration{Enable: BoolPtr(false), Server: StringPtr("cloud.example.net:6012/jrpc")},
	}
}

func TestGen2Converter_ReadsMQTTAndCloud(t *testing.T) {
	device := newFakeGen2Device()
	device.doc["mqtt"] = map[string]interface{}{
		"enable": true, "server": "broker.example.net:1883", "client_id": "lamp", "user": "iot",
		"topic_prefix": "home/lamp", "rpc_ntf": true, "status_ntf": false, "pass": nil,
	}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	require.NotNil(t, got.MQTT)
	assert.True(t, *got.MQTT.Enable)
	assert.Equal(t, "broker.example.net", *got.MQTT.Server)
	assert.Equal(t, 1883, *got.MQTT.Port)
	assert.Equal(t, "iot", *got.MQTT.User)
	assert.Equal(t, "lamp", *got.MQTT.ClientID)
	assert.Equal(t, "home/lamp", *got.MQTT.TopicPrefix)
	assert.True(t, *got.MQTT.RPCNotifications)
	assert.False(t, *got.MQTT.StatusNotification)
	assert.Nil(t, got.MQTT.Password, "the device never returns the password")
	require.NotNil(t, got.Cloud)
	assert.True(t, *got.Cloud.Enable)
	assert.Equal(t, "iot.shelly.cloud:6012/jrpc", *got.Cloud.Server)
}

// A null server is an MQTT broker that is not set, never an empty one.
func TestGen2Converter_ANullMQTTServerIsUnset(t *testing.T) {
	raw, err := json.Marshal(newFakeGen2Device().doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Nil(t, got.MQTT.Server)
	assert.Nil(t, got.MQTT.Port)
}

func TestGen2Converter_WritesTheMQTTAndCloudShape(t *testing.T) {
	raw, err := NewGen2Converter(nil).ToAPIConfig(mqttDesired(), "SNSW-001X16EU")

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"mqtt":{"enable":true,"server":"broker.example.net:1883","user":"iot","pass":"s3cret-pass",
			"client_id":"lamp","topic_prefix":"home/lamp","rpc_ntf":true,"status_ntf":false},
		"cloud":{"enable":false,"server":"cloud.example.net:6012/jrpc"}}`, string(raw))
}

// An empty password, or the mask the API shows in place of one, is not a password to write.
func TestGen2Converter_NeverWritesAnEmptyOrMaskedPassword(t *testing.T) {
	for _, pass := range []string{"", "********"} {
		desired := &DeviceConfiguration{MQTT: &MQTTConfiguration{User: StringPtr("iot"), Password: StringPtr(pass)}}

		raw, err := NewGen2Converter(nil).ToAPIConfig(desired, "SNSW-001X16EU")

		require.NoError(t, err)
		assert.JSONEq(t, `{"mqtt":{"user":"iot"}}`, string(raw), "password %q", pass)
	}
}

func TestApplyDesiredConfig_Gen2WritesMQTTAndCloud(t *testing.T) {
	svc, repo := newGen2Service(t, mqttDesired())
	dev := newFakeGen2Device()
	dev.restart = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify matches although the password is never read back")
	assert.True(t, repo.devices[1].ConfigApplied)
	require.Len(t, dev.calls, 2)
	assert.Equal(t, "MQTT.SetConfig", dev.calls[0].method)
	assert.Equal(t, "s3cret-pass", dev.calls[0].config["pass"])
	assert.Equal(t, "iot", dev.calls[0].config["user"])
	assert.Equal(t, "Cloud.SetConfig", dev.calls[1].method)
	assert.Equal(t, 10, result.ApplyResult.AppliedCount, "eight mqtt keys and two cloud keys")
	assert.Equal(t, 1, dev.reboots, "one reboot, because the answer asked for one")
}

func TestApplyDesiredConfig_Gen2WritesSysBeforeMQTT(t *testing.T) {
	desired := mqttDesired()
	desired.System = &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 3)
	assert.Equal(t, []string{"Sys.SetConfig", "MQTT.SetConfig", "Cloud.SetConfig"},
		[]string{dev.calls[0].method, dev.calls[1].method, dev.calls[2].method})
	assert.Equal(t, 0, dev.reboots, "no answer asked for a restart")
}

// A device refuses MQTT: the failure names the keys, never the password, and the cloud section
// is still tried.
func TestApplyDesiredConfig_Gen2KeepsThePasswordOutOfFailuresAndLogs(t *testing.T) {
	svc, repo := newGen2Service(t, mqttDesired())
	dev := newFakeGen2Device()
	dev.refuse = errors.New("invalid argument: pass s3cret-pass")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
	dumped, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(dumped), "s3cret-pass")
	var paths []string
	for _, f := range result.ApplyResult.Failures {
		paths = append(paths, f.Path)
	}
	assert.Contains(t, paths, "mqtt.pass")
	assert.Contains(t, paths, "mqtt.server")
	assert.Contains(t, paths, "cloud.enable")
}

// A device that echoes the request as JSON writes a quote in the password as \". The plain text of
// the password is then not in the error, and the escaped form must go too.
func TestApplyDesiredConfig_Gen2RedactsAnEscapedPasswordFromAnError(t *testing.T) {
	desired := mqttDesired()
	desired.MQTT.Password = StringPtr(`p"ass\word`)
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()
	dev.refuse = errors.New(`invalid argument: {"pass":"p\"ass\\word"}`)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	require.NotEmpty(t, result.ApplyResult.Failures)
	for _, f := range result.ApplyResult.Failures {
		assert.NotContains(t, f.Error, `p\"ass`, f.Path)
		assert.NotContains(t, f.Error, `p"ass`, f.Path)
		assert.NotContains(t, f.Value, "ass", f.Path)
	}
}

// A server that already carries a port, with a separate port, would reach the device as
// host:port:port.
func TestApplyDesiredConfig_Gen2RefusesAServerThatAlreadyHasAPort(t *testing.T) {
	desired := &DeviceConfiguration{MQTT: &MQTTConfiguration{Server: StringPtr("broker.example.net:1883"), Port: IntPtr(1883)}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "mqtt.server")
	assert.Empty(t, dev.calls)
}

func TestGen2Converter_WritesAnIPv6BrokerWithItsPortInBrackets(t *testing.T) {
	desired := &DeviceConfiguration{MQTT: &MQTTConfiguration{Server: StringPtr("fd00::1"), Port: IntPtr(1883)}}

	raw, err := NewGen2Converter(nil).ToAPIConfig(desired, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.JSONEq(t, `{"mqtt":{"server":"[fd00::1]:1883"}}`, string(raw))
}

func TestVerifyDesiredConfig_Gen2IgnoresTheMQTTPassword(t *testing.T) {
	desired := &DeviceConfiguration{MQTT: &MQTTConfiguration{User: StringPtr("iot"), Password: StringPtr("s3cret-pass")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()
	dev.doc["mqtt"] = map[string]interface{}{"enable": false, "server": nil, "user": "iot", "pass": nil}

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.Match, "%v", result.Differences)
}

// A device password needs the user in the same call, and a Gen2 MQTT setting with no mapping stays
// refused before anything is written.
func TestApplyDesiredConfig_Gen2RefusesMQTTSettingsItCannotWrite(t *testing.T) {
	cases := map[string]struct {
		mqtt *MQTTConfiguration
		path string
	}{
		"password without user": {&MQTTConfiguration{Password: StringPtr("s3cret-pass")}, "mqtt.pass"},
		"keep alive":            {&MQTTConfiguration{KeepAlive: IntPtr(60)}, "mqtt.keep_alive"},
		"clean session":         {&MQTTConfiguration{CleanSession: BoolPtr(true)}, "mqtt.clean_session"},
		"update period":         {&MQTTConfiguration{UpdatePeriod: IntPtr(30)}, "mqtt.update_period"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, &DeviceConfiguration{MQTT: tc.mqtt})
			dev := newFakeGen2Device()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.NotContains(t, err.Error(), "s3cret-pass")
			assert.Empty(t, dev.calls)
		})
	}
}

func TestGen2Leaves_ReportAPathPerKey(t *testing.T) {
	leaves := gen2Leaves("mqtt", map[string]interface{}{"user": "iot", "pass": "x"})

	var paths []string
	for _, l := range leaves {
		paths = append(paths, l.path)
	}
	assert.Equal(t, "mqtt.pass,mqtt.user", strings.Join(paths, ","))
}

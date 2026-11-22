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

// gen2WiFiDesired is a station on a new network with its password. The fake device is on "home".
func gen2WiFiDesired() *DeviceConfiguration {
	return &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable:   BoolPtr(true),
			SSID:     StringPtr("office"),
			Password: StringPtr("wifi-pass-1"),
			IPv4Mode: StringPtr("dhcp"),
		},
	}
}

// unreadableGen2Device is a Gen2 device that answers a write but cannot be read.
type unreadableGen2Device struct{ *fakeGen2Device }

func (unreadableGen2Device) GetConfig(context.Context) (*shelly.DeviceConfig, error) {
	return nil, errors.New("no route to host")
}

func TestGen2Converter_ReadsTheWiFiStation(t *testing.T) {
	device := newFakeGen2Device()
	device.doc["wifi"] = map[string]interface{}{
		"ap": map[string]interface{}{"ssid": "ShellyPlus2PM-A8032AB636EC", "is_open": true, "enable": true},
		"sta": map[string]interface{}{
			"ssid": "home", "is_open": false, "enable": true, "ipv4mode": "static",
			"ip": "192.168.1.60", "netmask": "255.255.255.0", "gw": "192.168.1.1", "nameserver": "192.168.1.2",
		},
		"sta1": map[string]interface{}{"ssid": nil, "enable": false, "ipv4mode": "dhcp"},
		"roam": map[string]interface{}{"rssi_thr": -80.0, "interval": 60.0},
	}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	require.NotNil(t, got.WiFi)
	assert.Equal(t, "home", *got.WiFi.SSID)
	assert.True(t, *got.WiFi.Enable)
	assert.Equal(t, "static", *got.WiFi.IPv4Mode)
	require.NotNil(t, got.WiFi.StaticIP)
	assert.Equal(t, "192.168.1.60", *got.WiFi.StaticIP.IP)
	assert.Equal(t, "255.255.255.0", *got.WiFi.StaticIP.Netmask)
	assert.Equal(t, "192.168.1.1", *got.WiFi.StaticIP.Gateway)
	assert.Equal(t, "192.168.1.2", *got.WiFi.StaticIP.Nameserver)
	assert.Nil(t, got.WiFi.Password, "the device never returns the password")
	require.NotNil(t, got.WiFi.AccessPoint)
	assert.True(t, *got.WiFi.AccessPoint.Enable)
	assert.Equal(t, "ShellyPlus2PM-A8032AB636EC", *got.WiFi.AccessPoint.SSID)
	assert.Nil(t, got.WiFi.AccessPoint.Password, "the device never returns the access point password")
	require.NotNil(t, got.WiFi.RoamThreshold)
	assert.Equal(t, -80, *got.WiFi.RoamThreshold)
}

// A null address is an unset one, so a DHCP station has no static address.
func TestGen2Converter_ANullStationAddressIsUnset(t *testing.T) {
	device := newFakeGen2Device()
	device.doc["wifi"] = map[string]interface{}{"sta": map[string]interface{}{
		"ssid": "home", "enable": true, "ipv4mode": "dhcp", "ip": nil, "netmask": nil, "gw": nil, "nameserver": nil,
	}}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Nil(t, got.WiFi.StaticIP)
}

func TestGen2Converter_WritesTheWiFiStationShape(t *testing.T) {
	desired := gen2WiFiDesired()
	desired.WiFi.IPv4Mode = StringPtr("static")
	desired.WiFi.StaticIP = &StaticIPConfig{
		IP: StringPtr("192.168.1.60"), Netmask: StringPtr("255.255.255.0"),
		Gateway: StringPtr("192.168.1.1"), Nameserver: StringPtr("192.168.1.2"),
	}

	raw, err := NewGen2Converter(nil).ToAPIConfig(desired, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.JSONEq(t, `{"wifi":{"sta":{"enable":true,"ssid":"office","pass":"wifi-pass-1","ipv4mode":"static",
		"ip":"192.168.1.60","netmask":"255.255.255.0","gw":"192.168.1.1","nameserver":"192.168.1.2"}}}`, string(raw))
}

// An empty password, or a mask the API shows in place of one, is not a password to write.
func TestGen2Converter_NeverWritesAnEmptyOrMaskedWiFiPassword(t *testing.T) {
	for _, pass := range []string{"", "********", "[REDACTED]", "***"} {
		desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{SSID: StringPtr("office"), Password: StringPtr(pass)}}

		raw, err := NewGen2Converter(nil).ToAPIConfig(desired, "SNSW-001X16EU")

		require.NoError(t, err)
		assert.JSONEq(t, `{"wifi":{"sta":{"ssid":"office"}}}`, string(raw), "password %q", pass)
	}
}

// The station is written last, after the channels: the device reconnects about a second after the
// write, so nothing should depend on the connection after it.
func TestApplyDesiredConfig_Gen2WritesTheWiFiStationLast(t *testing.T) {
	desired := switchInputDesired()
	desired.WiFi = gen2WiFiDesired().WiFi
	svc, repo := newGen2Service(t, desired)
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	assert.True(t, repo.devices[1].ConfigApplied)
	last := dev.calls[len(dev.calls)-1]
	assert.Equal(t, "WiFi.SetConfig", last.method)
	assert.Equal(t, -1, last.id)
	assert.Equal(t, map[string]interface{}{"sta": map[string]interface{}{
		"enable": true, "ssid": "office", "pass": "wifi-pass-1", "ipv4mode": "dhcp",
	}}, last.config)
	for _, call := range dev.calls[:len(dev.calls)-1] {
		assert.NotEqual(t, "WiFi.SetConfig", call.method)
	}
	assert.Equal(t, 0, dev.legacyCalls)
}

func TestApplyDesiredConfig_Gen2WritesAStaticAddress(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{
		IPv4Mode: StringPtr("static"),
		StaticIP: &StaticIPConfig{
			IP: StringPtr("192.168.1.60"), Netmask: StringPtr("255.255.255.0"),
			Gateway: StringPtr("192.168.1.1"), Nameserver: StringPtr("192.168.1.2"),
		},
	}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{"sta": map[string]interface{}{
		"ipv4mode": "static", "ip": "192.168.1.60", "netmask": "255.255.255.0", "gw": "192.168.1.1", "nameserver": "192.168.1.2",
	}}, dev.calls[0].config)
}

// The device is on "home". A write that names the network it is on, with no password, leaves the
// ssid out: the device wants a password with every ssid it is given.
func TestApplyDesiredConfig_Gen2LeavesTheSSIDOutWhenItIsTheCurrentNetworkWithNoPassword(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{Enable: BoolPtr(true), SSID: StringPtr("home")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	for round := 1; round <= 2; round++ {
		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

		require.NoError(t, err, "round %d", round)
		assert.True(t, result.ConfigApplied, "round %d: %v", round, result.VerifyResult)
	}
	require.Len(t, dev.calls, 2, "the second apply writes the same thing again and changes nothing")
	for _, call := range dev.calls {
		assert.Equal(t, map[string]interface{}{"sta": map[string]interface{}{"enable": true}}, call.config)
	}
	assert.Equal(t, "home", dev.doc["wifi"].(map[string]interface{})["sta"].(map[string]interface{})["ssid"])
}

func TestApplyDesiredConfig_Gen2SendsTheSSIDOfTheCurrentNetworkWhenThereIsAPassword(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{SSID: StringPtr("home"), Password: StringPtr("wifi-pass-1")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{"sta": map[string]interface{}{"ssid": "home", "pass": "wifi-pass-1"}}, dev.calls[0].config)
}

// A new network with no password that can be sent would leave the device unable to join it, so
// nothing is written.
func TestApplyDesiredConfig_Gen2RefusesANewNetworkWithNoPassword(t *testing.T) {
	for _, pass := range []*string{nil, StringPtr(""), StringPtr("********")} {
		desired := &DeviceConfiguration{
			System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}},
			WiFi:   &WiFiConfiguration{SSID: StringPtr("office"), Password: pass},
		}
		svc, repo := newGen2Service(t, desired)
		dev := newFakeGen2Device()

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		assert.Contains(t, err.Error(), "wifi.ssid")
		assert.Empty(t, dev.calls, "nothing is written, the system section included")
		assert.False(t, repo.devices[1].ConfigApplied)
	}
}

// The station is never written blind: a device that cannot be read is refused before the first write.
func TestApplyDesiredConfig_Gen2RefusesAStationWriteWhenTheDeviceCannotBeRead(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := unreadableGen2Device{newFakeGen2Device()}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Empty(t, dev.calls)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// Disabling the station would leave the device with no way back from the manager, because the
// access point is not written here. The device would accept it, so it is refused by name.
func TestApplyDesiredConfig_Gen2RefusesDisablingTheStation(t *testing.T) {
	desired := &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}},
		WiFi:   &WiFiConfiguration{Enable: BoolPtr(false)},
	}
	svc, repo := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "wifi.enable")
	assert.Empty(t, dev.calls, "nothing is written, the system section included")
	assert.False(t, repo.devices[1].ConfigApplied)
}

// The access point and roaming are not written yet: each is refused with its own path, before
// anything is written.
func TestApplyDesiredConfig_Gen2RefusesWiFiSettingsItCannotWrite(t *testing.T) {
	cases := map[string]struct {
		wifi *WiFiConfiguration
		path string
	}{
		// The Gen2 access point has one secret, pass. The key field has no Gen2 meaning.
		"access point key":       {&WiFiConfiguration{AccessPoint: &AccessPointConfig{Key: StringPtr("ap-pass-1234")}}, "wifi.ap.key"},
		"access point key, ssid": {&WiFiConfiguration{AccessPoint: &AccessPointConfig{Key: StringPtr("ap-pass-1234"), SSID: StringPtr("lamp-ap")}}, "wifi.ap.key"},
		// The roaming flag and the recovery reboot are Gen1 settings: Gen2 has only the threshold.
		"roaming flag":    {&WiFiConfiguration{RoamEnable: BoolPtr(true)}, "wifi.roam_enable"},
		"recovery reboot": {&WiFiConfiguration{RecoveryReboot: BoolPtr(true)}, "wifi.recovery_reboot"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, &DeviceConfiguration{
				System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}},
				WiFi:   tc.wifi,
			})
			dev := newFakeGen2Device()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.NotContains(t, err.Error(), "ap-pass-1234")
			assert.Empty(t, dev.calls)
		})
	}
}

// The password is write-only: it is never read back, so verify must not report it, and the
// failure of a write that carried it names the keys and never its value or the device's echo of it.
func TestApplyDesiredConfig_Gen2KeepsTheWiFiPasswordOutOfFailuresAndLogs(t *testing.T) {
	svc, repo := newGen2Service(t, gen2WiFiDesired())
	dev := newFakeGen2Device()
	// A device that refuses a write answers with an error of its own. A bare transport error
	// would be no answer at all, which a network change treats as possibly applied.
	dev.refuse = &shelly.DeviceError{
		IP: "192.168.1.60", Generation: 2, Operation: "WiFi.SetConfig", StatusCode: 400,
		Message: `invalid argument: {"pass":"wifi-pass-1"}`,
	}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
	dumped, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(dumped), "wifi-pass-1")
	var paths []string
	for _, f := range result.ApplyResult.Failures {
		paths = append(paths, f.Path)
		assert.Contains(t, f.Error, "WiFi.SetConfig", f.Path)
	}
	assert.Contains(t, paths, "wifi.sta.ssid")
	assert.Contains(t, paths, "wifi.sta.pass")
}

// A password alone changes the password of the network the device is on: the device wants a
// password with an ssid, and nothing requires an ssid with a password. The password is
// write-only, so it must not make the round trip report the whole section as unmapped.
func TestApplyDesiredConfig_Gen2WritesAPasswordAlone(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{Password: StringPtr("wifi-pass-2")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{"sta": map[string]interface{}{"pass": "wifi-pass-2"}}, dev.calls[0].config)
}

func TestApplyDesiredConfig_Gen2AsksForARebootWhenTheWiFiAnswerDoes(t *testing.T) {
	svc, _ := newGen2Service(t, gen2WiFiDesired())
	dev := newFakeGen2Device()
	dev.restart = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ApplyResult.RequiresReboot)
	assert.Contains(t, result.ApplyResult.Warnings[0], "WiFi.SetConfig")
	assert.Equal(t, 1, dev.reboots)
}

func TestVerifyDesiredConfig_Gen2IgnoresTheWiFiPassword(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{SSID: StringPtr("home"), Password: StringPtr("wifi-pass-1")}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.Match, "%v", result.Differences)
}

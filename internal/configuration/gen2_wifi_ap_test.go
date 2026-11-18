package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// The Gen2 access point and roaming. The access point is written with WiFi.SetConfig {"ap": ...},
// roaming with {"roam": ...}, both before the station, which stays last. Two rules keep a device
// reachable and closed: an apply that switches the access point off needs a steady station, and an
// apply never switches an open access point on. The fake device is on "home", enabled, with an
// access point that is on and open.

func apDesired(ap *AccessPointConfig) *DeviceConfiguration {
	return &DeviceConfiguration{WiFi: &WiFiConfiguration{AccessPoint: ap}}
}

func liveWiFi(dev *fakeGen2Device, part, key string, value interface{}) {
	section := dev.doc["wifi"].(map[string]interface{})[part].(map[string]interface{})
	if value == nil {
		delete(section, key)
		return
	}
	section[key] = value
}

// writtenKeys lists the first config key of each WiFi.SetConfig call, in call order.
func writtenKeys(dev *fakeGen2Device) []string {
	var keys []string
	for _, call := range dev.calls {
		var names []string
		for name := range call.config {
			names = append(names, name)
		}
		sort.Strings(names)
		keys = append(keys, names...)
	}
	return keys
}

// assertRefusedBeforeWrite checks a refusal that stops before the device is touched.
func assertRefusedBeforeWrite(t *testing.T, result *ApplyVerifyResult, err error, dev *fakeGen2Device, repo *mockRepository) {
	t.Helper()
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Empty(t, dev.calls, "nothing is written when the apply is refused")
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_Gen2WritesTheAccessPointBeforeTheStation(t *testing.T) {
	desired := gen2WiFiDesired()
	desired.WiFi.AccessPoint = &AccessPointConfig{Enable: BoolPtr(true), SSID: StringPtr("lamp-ap"), Password: StringPtr("ap-pass-1")}
	svc, repo := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	require.Len(t, dev.calls, 2)
	assert.Equal(t, []string{"ap", "sta"}, writtenKeys(dev))
	ap, _ := dev.calls[0].config["ap"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"enable": true, "ssid": "lamp-ap", "pass": "ap-pass-1"}, ap)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	assert.True(t, repo.devices[1].ConfigApplied)
}

// A password alone changes the password of the access point. It is write-only, so the round trip
// does not compare it and the section is not reported as unmapped.
func TestApplyDesiredConfig_Gen2WritesAnAccessPointPasswordAlone(t *testing.T) {
	svc, _ := newGen2Service(t, apDesired(&AccessPointConfig{Password: StringPtr("ap-pass-2")}))
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	require.Len(t, dev.calls, 1)
	ap, _ := dev.calls[0].config["ap"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"pass": "ap-pass-2"}, ap)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
}

func TestApplyDesiredConfig_Gen2SwitchesTheAccessPointOffWhileTheStationIsSteady(t *testing.T) {
	svc, repo := newGen2Service(t, apDesired(&AccessPointConfig{Enable: BoolPtr(false)}))
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Equal(t, []string{"ap"}, writtenKeys(dev))
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_Gen2WritesTheRoamThreshold(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{RoamThreshold: IntPtr(-70)}}
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	require.Len(t, dev.calls, 1)
	roam, _ := dev.calls[0].config["roam"].(map[string]interface{})
	assert.Equal(t, map[string]interface{}{"rssi_thr": float64(-70)}, roam)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
}

// The station reconnects about a second after its write, so it is the last call.
func TestApplyDesiredConfig_Gen2WritesTheAccessPointRoamingAndStationInOrder(t *testing.T) {
	desired := gen2WiFiDesired()
	desired.WiFi.AccessPoint = &AccessPointConfig{SSID: StringPtr("lamp-ap")}
	desired.WiFi.RoamThreshold = IntPtr(-75)
	svc, _ := newGen2Service(t, desired)
	dev := newFakeGen2Device()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Equal(t, []string{"ap", "roam", "sta"}, writtenKeys(dev))
}

func TestApplyDesiredConfig_Gen2KeepsTheAccessPointPasswordOutOfFailuresAndLogs(t *testing.T) {
	desired := apDesired(&AccessPointConfig{SSID: StringPtr("lamp-ap"), Password: StringPtr("ap-pass-secret")})
	svc, repo := newGen2Service(t, desired)
	dev := newFakeGen2Device()
	dev.refuse = &shelly.DeviceError{
		IP: "192.168.1.60", Generation: 2, Operation: "WiFi.SetConfig", StatusCode: 400,
		Message: `invalid argument: {"pass":"ap-pass-secret"}`,
	}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
	dumped, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(dumped), "ap-pass-secret")
	var paths []string
	for _, f := range result.ApplyResult.Failures {
		paths = append(paths, f.Path)
		assert.Contains(t, f.Error, "WiFi.SetConfig", f.Path)
	}
	assert.Contains(t, paths, "wifi.ap.ssid")
	assert.Contains(t, paths, "wifi.ap.pass")
}

// The access point is not the manager's route to the device: a write that gets no answer is a
// failure, never "unconfirmed", which is reserved for a station network change.
func TestApplyDesiredConfig_Gen2ALostAnswerOnTheAccessPointWriteIsAFailure(t *testing.T) {
	svc, _ := newGen2Service(t, apDesired(&AccessPointConfig{SSID: StringPtr("lamp-ap")}))
	dev := newFlakyGen2Device()
	dev.writeErr = errNoRoute

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ApplyResult.Unconfirmed)
	assert.NotEmpty(t, result.ApplyResult.Failures)
}

// Each WiFi.SetConfig answer carries its own restart_required: one true among the three asks for
// the reboot.
type restartOnDevice struct {
	*fakeGen2Device
	key string // the config key whose answer asks for a restart
}

func (d *restartOnDevice) CallConfigSetter(ctx context.Context, method string, params map[string]interface{}) (bool, error) {
	_, err := d.fakeGen2Device.CallConfigSetter(ctx, method, params)
	config, _ := params["config"].(map[string]interface{})
	_, asks := config[d.key]
	return asks, err
}

func TestApplyDesiredConfig_Gen2OneRestartAmongThreeWritesRequestsTheReboot(t *testing.T) {
	desired := gen2WiFiDesired()
	desired.WiFi.AccessPoint = &AccessPointConfig{SSID: StringPtr("lamp-ap")}
	desired.WiFi.RoamThreshold = IntPtr(-75)

	for key, wantReboot := range map[string]bool{"roam": true, "none": false} {
		t.Run(key, func(t *testing.T) {
			svc, _ := newGen2Service(t, desired)
			dev := &restartOnDevice{fakeGen2Device: newFakeGen2Device(), key: key}

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.NoError(t, err)
			assert.Equal(t, wantReboot, result.ApplyResult.RequiresReboot)
			if wantReboot {
				assert.Equal(t, 1, dev.reboots)
			} else {
				assert.Equal(t, 0, dev.reboots)
			}
		})
	}
}

// The strand guard: switching the access point off needs a station that stays exactly as it is.

func TestApplyDesiredConfig_Gen2RefusesSwitchingTheAccessPointOffWhenTheStationMoves(t *testing.T) {
	cases := map[string]*WiFiConfiguration{
		"new ssid": {AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)}, SSID: StringPtr("office"), Password: StringPtr("wifi-pass-1")},
		"new ip": {AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)}, IPv4Mode: StringPtr("static"),
			StaticIP: &StaticIPConfig{IP: StringPtr("192.168.1.61"), Netmask: StringPtr("255.255.255.0"), Gateway: StringPtr("192.168.1.1")}},
	}
	for name, wifi := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newGen2Service(t, &DeviceConfiguration{WiFi: wifi})
			dev := newFakeGen2Device()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			assertRefusedBeforeWrite(t, result, err, dev, repo)
			assert.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), "needs a steady station")
		})
	}
}

// A station password is no network change, but a wrong one leaves the device unable to join:
// the station is not steady in an apply that sends one.
func TestApplyDesiredConfig_Gen2RefusesSwitchingTheAccessPointOffWhileTheStationPasswordChanges(t *testing.T) {
	wifi := &WiFiConfiguration{AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)}, Password: StringPtr("wrong-password")}
	svc, repo := newGen2Service(t, &DeviceConfiguration{WiFi: wifi})
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	assertRefusedBeforeWrite(t, result, err, dev, repo)
	assert.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "needs a steady station")
}

// A name server is no way to lose the device: it stays allowed beside an access point switched off.
func TestApplyDesiredConfig_Gen2SwitchesTheAccessPointOffBesideANameServerChange(t *testing.T) {
	wifi := &WiFiConfiguration{AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)}, StaticIP: &StaticIPConfig{Nameserver: StringPtr("192.168.1.9")}}
	svc, _ := newGen2Service(t, &DeviceConfiguration{WiFi: wifi})
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Equal(t, []string{"ap", "sta"}, writtenKeys(dev))
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
}

func TestApplyDesiredConfig_Gen2RefusesSwitchingTheAccessPointOffWithoutASteadyStation(t *testing.T) {
	cases := map[string]func(*fakeGen2Device){
		"station disabled":  func(d *fakeGen2Device) { liveWiFi(d, "sta", "enable", false) },
		"station no ssid":   func(d *fakeGen2Device) { liveWiFi(d, "sta", "ssid", "") },
		"station no enable": func(d *fakeGen2Device) { liveWiFi(d, "sta", "enable", nil) },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newGen2Service(t, apDesired(&AccessPointConfig{Enable: BoolPtr(false)}))
			dev := newFakeGen2Device()
			setup(dev)

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			assertRefusedBeforeWrite(t, result, err, dev, repo)
			assert.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), "needs a steady station")
		})
	}
}

func TestApplyDesiredConfig_Gen2RefusesSwitchingTheAccessPointOffOnAnUnreadableDevice(t *testing.T) {
	svc, repo := newGen2Service(t, apDesired(&AccessPointConfig{Enable: BoolPtr(false)}))
	dev := unreadableGen2Device{newFakeGen2Device()}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	assertRefusedBeforeWrite(t, result, err, dev.fakeGen2Device, repo)
	assert.Contains(t, err.Error(), "could not read the device")
}

// Rule O: an apply never switches an open access point on.

func TestApplyDesiredConfig_Gen2RefusesSwitchingAnOpenAccessPointOn(t *testing.T) {
	cases := map[string]func(*fakeGen2Device){
		"open":                func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", false) },
		"is_open missing":     func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", false); liveWiFi(d, "ap", "is_open", nil) },
		"enable not a bool":   func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", nil) },
		"empty password only": func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", false) },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			ap := &AccessPointConfig{Enable: BoolPtr(true)}
			if name == "empty password only" {
				ap.Password = StringPtr("")
			}
			svc, repo := newGen2Service(t, apDesired(ap))
			dev := newFakeGen2Device()
			setup(dev)

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			assertRefusedBeforeWrite(t, result, err, dev, repo)
			assert.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), "set wifi.ap.pass")
		})
	}
}

// Guards: what stays allowed.

func TestApplyDesiredConfig_Gen2SwitchesAnAccessPointOnWithAPasswordOrAClosedNetwork(t *testing.T) {
	cases := map[string]struct {
		ap    *AccessPointConfig
		setup func(*fakeGen2Device)
	}{
		"with a password": {&AccessPointConfig{Enable: BoolPtr(true), Password: StringPtr("ap-pass-1")}, func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", false) }},
		"already closed":  {&AccessPointConfig{Enable: BoolPtr(true)}, func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", false); liveWiFi(d, "ap", "is_open", false) }},
		"already running": {&AccessPointConfig{Enable: BoolPtr(true)}, func(d *fakeGen2Device) {}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, apDesired(tc.ap))
			dev := newFakeGen2Device()
			tc.setup(dev)

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.NoError(t, err)
			assert.Empty(t, result.ApplyResult.Failures)
			assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
		})
	}
}

func TestApplyDesiredConfig_Gen2SwitchingAnAccessPointOffThatIsAlreadyOffChangesNothing(t *testing.T) {
	svc, _ := newGen2Service(t, apDesired(&AccessPointConfig{Enable: BoolPtr(false)}))
	dev := newFakeGen2Device()
	liveWiFi(dev, "ap", "enable", false)
	liveWiFi(dev, "sta", "enable", false)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
}

// A station change with no access point write keeps its behaviour, whatever state the access
// point is in: the guard covers what this change adds.
func TestApplyDesiredConfig_Gen2AStationChangeIsStillWrittenWhenTheAccessPointIsOff(t *testing.T) {
	cases := map[string]func(*fakeGen2Device){
		"ap off":          func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", false) },
		"ap enable unset": func(d *fakeGen2Device) { liveWiFi(d, "ap", "enable", nil) },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, gen2WiFiDesired())
			dev := newFakeGen2Device()
			setup(dev)

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.NoError(t, err)
			assert.Equal(t, []string{"sta"}, writtenKeys(dev))
			assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
		})
	}
}

func TestApplyDesiredConfig_Gen2StillRefusesTheAccessPointKeyAndSwitchingTheStationOff(t *testing.T) {
	cases := map[string]*WiFiConfiguration{
		"ap key":      {AccessPoint: &AccessPointConfig{Key: StringPtr("k")}},
		"station off": {Enable: BoolPtr(false), AccessPoint: &AccessPointConfig{Enable: BoolPtr(true), Password: StringPtr("ap-pass-1")}},
	}
	for name, wifi := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newGen2Service(t, &DeviceConfiguration{WiFi: wifi})
			dev := newFakeGen2Device()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			assertRefusedBeforeWrite(t, result, err, dev, repo)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		})
	}
}

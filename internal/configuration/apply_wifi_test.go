package configuration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// fakeWiFiDevice is a Gen1 device with the Wi-Fi routes: POST /settings/sta and /settings/ap write
// flat parameters, and GET /settings reports wifi_sta and wifi_ap, never the station key. It keeps
// the writes between a POST and the next GET, like a real device.
type fakeWiFiDevice struct {
	*fakeGen1Device
	staWrites, apWrites []map[string]interface{}
	staErr, apErr       error
	gets                int
	failGetFrom         int // GET /settings number from which the device does not answer; 0 never
}

func newFakeWiFiDevice() *fakeWiFiDevice {
	d := &fakeWiFiDevice{fakeGen1Device: newFakeGen1Device()}
	d.settings["wifi_sta"] = map[string]interface{}{
		"enabled": true, "ssid": "office", "ipv4_method": "dhcp",
		"ip": "192.168.1.50", "gw": "192.168.1.1", "mask": "255.255.255.0", "dns": "192.168.1.1",
	}
	d.settings["wifi_ap"] = map[string]interface{}{"enabled": false, "ssid": "shelly1-AABBCC", "key": ""}
	return d
}

func (d *fakeWiFiDevice) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	d.gets++
	if d.failGetFrom > 0 && d.gets >= d.failGetFrom {
		return nil, errors.New("dial tcp 192.168.1.50:80: connect: no route to host")
	}
	return d.fakeGen1Device.GetConfig(ctx)
}

// store keeps a written form in the device's section. The device reports the netmask as mask and
// the gateway as gw, a flag as a bool, and never the key.
func (d *fakeWiFiDevice) store(section string, form map[string]interface{}) {
	if d.ignoreWrites {
		return
	}
	doc := d.settings[section].(map[string]interface{})
	for k, v := range form {
		switch k {
		case "key":
			continue
		case "netmask":
			k = "mask"
		case "gateway":
			k = "gw"
		case "enabled":
			if n, ok := v.(int); ok {
				v = n != 0
			}
		}
		doc[k] = v
	}
}

func (d *fakeWiFiDevice) SetWiFiConfig(_ context.Context, form map[string]interface{}) error {
	if d.staErr != nil {
		return d.staErr
	}
	d.staWrites = append(d.staWrites, form)
	d.store("wifi_sta", form)
	return nil
}

func (d *fakeWiFiDevice) SetAPConfig(_ context.Context, form map[string]interface{}) error {
	if d.apErr != nil {
		return d.apErr
	}
	d.apWrites = append(d.apWrites, form)
	d.store("wifi_ap", form)
	return nil
}

func wifiDesired(w *WiFiConfiguration) *DeviceConfiguration { return &DeviceConfiguration{WiFi: w} }

func applyWiFi(t *testing.T, dev ShellyClient, desired *DeviceConfiguration) (*ApplyResult, error) {
	t.Helper()
	return NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), dev, desired, "SHSW-1")
}

func TestApplyConfig_WiFiStationOnIsWrittenToSettingsSta(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.settings["wifi_sta"].(map[string]interface{})["enabled"] = false

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		Enable: BoolPtr(true), SSID: StringPtr("office"), Password: StringPtr("s3cret-pass"), IPv4Mode: StringPtr("dhcp"),
	}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, dev.staWrites, 1)
	assert.Equal(t, map[string]interface{}{"enabled": 1, "ssid": "office", "key": "s3cret-pass", "ipv4_method": "dhcp"}, dev.staWrites[0])
	assert.Empty(t, dev.apWrites)
	// Never the nested wifi_sta document on POST /settings, which the device ignores.
	assert.Empty(t, dev.posted)
	assert.Equal(t, 4, result.SettingsCount)
	assert.Equal(t, 4, result.AppliedCount)
	assert.False(t, result.Unconfirmed)
	assert.False(t, result.RequiresReboot, "no reboot is documented for the Wi-Fi routes")
}

func TestApplyConfig_WiFiStationOffSendsOnlyTheFlag(t *testing.T) {
	dev := newFakeWiFiDevice()

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{Enable: BoolPtr(false)}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, dev.staWrites, 1)
	assert.Equal(t, map[string]interface{}{"enabled": 0}, dev.staWrites[0])
	assert.Empty(t, dev.apWrites)
}

func TestApplyConfig_WiFiStaticAddressUsesTheDeviceFormNames(t *testing.T) {
	dev := newFakeWiFiDevice()

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		IPv4Mode: StringPtr("static"),
		StaticIP: &StaticIPConfig{
			IP: StringPtr("192.168.1.60"), Netmask: StringPtr("255.255.255.0"),
			Gateway: StringPtr("192.168.1.254"), Nameserver: StringPtr("192.168.1.2"),
		},
	}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, dev.staWrites, 1)
	// The write form says netmask and gateway, where the GET document says mask and gw.
	assert.Equal(t, map[string]interface{}{
		"ipv4_method": "static", "ip": "192.168.1.60", "netmask": "255.255.255.0",
		"gateway": "192.168.1.254", "dns": "192.168.1.2",
	}, dev.staWrites[0])
}

func TestApplyConfig_WiFiAccessPointOnIsWrittenToSettingsAp(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.settings["wifi_sta"].(map[string]interface{})["enabled"] = false

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		AccessPoint: &AccessPointConfig{Enable: BoolPtr(true), Password: StringPtr("ap-s3cret")},
	}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, dev.apWrites, 1)
	assert.Equal(t, map[string]interface{}{"enabled": 1, "key": "ap-s3cret"}, dev.apWrites[0])
	assert.Empty(t, dev.staWrites, "an empty station section is not a write")
	assert.Empty(t, dev.posted)
}

func TestApplyConfig_WiFiAccessPointOffSendsOnlyTheFlag(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.settings["wifi_ap"].(map[string]interface{})["enabled"] = true

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)},
	}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, dev.apWrites, 1)
	assert.Equal(t, map[string]interface{}{"enabled": 0}, dev.apWrites[0])
}

func TestApplyConfig_WiFiKeyThatIsNotSetIsNotSent(t *testing.T) {
	dev := newFakeWiFiDevice()

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		IPv4Mode:    StringPtr("dhcp"),
		AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)},
	}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, dev.staWrites, 1)
	assert.Equal(t, map[string]interface{}{"ipv4_method": "dhcp"}, dev.staWrites[0], "no default for an unset key")
	require.Len(t, dev.apWrites, 1)
	assert.Equal(t, map[string]interface{}{"enabled": 0}, dev.apWrites[0])
}

func TestApplyConfig_WiFiNothingSetWritesNothing(t *testing.T) {
	dev := newFakeWiFiDevice()

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{}))

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, dev.staWrites)
	assert.Empty(t, dev.apWrites)
	assert.Equal(t, 1, dev.gets, "only the read that every apply makes, and no read-back")
}

func TestApplyConfig_WiFiSecretThatIsEmptyOrRedactedIsNeverSent(t *testing.T) {
	for _, secret := range []string{"", "[REDACTED]", "***"} {
		dev := newFakeWiFiDevice()

		result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
			SSID:        StringPtr("office"), // the network the device is on, so no key is needed
			Password:    StringPtr(secret),
			AccessPoint: &AccessPointConfig{Enable: BoolPtr(true), Password: StringPtr(secret)},
		}))

		require.NoError(t, err, "secret %q", secret)
		assert.True(t, result.Success, "secret %q failures: %+v", secret, result.Failures)
		require.Len(t, dev.staWrites, 1)
		assert.Equal(t, map[string]interface{}{"ssid": "office"}, dev.staWrites[0], "secret %q", secret)
		require.Len(t, dev.apWrites, 1)
		assert.Equal(t, map[string]interface{}{"enabled": 1}, dev.apWrites[0], "secret %q", secret)
	}
}

func TestApplyConfig_WiFiNewNetworkWithoutAUsableKeyIsRefusedBeforeAnyWrite(t *testing.T) {
	for _, secret := range []*string{nil, StringPtr(""), StringPtr("[REDACTED]"), StringPtr("***")} {
		dev := newFakeWiFiDevice()

		desired := &DeviceConfiguration{
			System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("renamed")}},
			WiFi:   &WiFiConfiguration{SSID: StringPtr("guest"), Password: secret},
		}
		result, err := applyWiFi(t, dev, desired)

		require.ErrorIs(t, err, ErrUnsupportedSettings)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "wifi.ssid")
		// Nothing at all was written, the name included: the refusal comes before the first section.
		assert.Zero(t, dev.setCalls)
		assert.Empty(t, dev.staWrites)
		assert.Empty(t, dev.apWrites)
	}
}

func TestApplyConfig_WiFiNewNetworkWithAKeyIsWritten(t *testing.T) {
	dev := newFakeWiFiDevice()

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		SSID: StringPtr("guest"), Password: StringPtr("guest-pass"),
	}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	assert.Equal(t, map[string]interface{}{"ssid": "guest", "key": "guest-pass"}, dev.staWrites[0])
}

func TestApplyConfig_WiFiIsNotWrittenBlindWhenTheDeviceCannotBeRead(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.failGetFrom = 1

	desired := &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("renamed")}},
		WiFi:   &WiFiConfiguration{SSID: StringPtr("office"), Password: StringPtr("pw")},
	}
	result, err := applyWiFi(t, dev, desired)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.NotErrorIs(t, err, ErrUnsupportedSettings, "a read failure is not a refusal of the settings")
	assert.Contains(t, err.Error(), "nothing was written")
	assert.Zero(t, dev.setCalls)
	assert.Empty(t, dev.staWrites)
}

func TestApplyConfig_WiFiNetworkChangeThenSilenceIsUnconfirmedNotAFailure(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.failGetFrom = 2 // the pre-write read answers, the read-back does not

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		SSID: StringPtr("guest"), Password: StringPtr("guest-pass"),
	}))

	require.NoError(t, err)
	assert.True(t, result.Unconfirmed)
	assert.Empty(t, result.Failures, "the write was accepted, so it is not a failed setting")
	assert.Zero(t, result.FailedCount)
	assert.Zero(t, result.AppliedCount, "and an unread write is not an applied one")
	assert.Contains(t, strings.Join(result.Warnings, " "), "may have changed network")
	assert.False(t, result.RequiresReboot)
}

func TestApplyAndVerify_UnconfirmedWiFiIsNeitherVerifiedNorApplied(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.failGetFrom = 2
	verifier := NewConfigVerifier(NewGen1Converter(nil), nil)

	result, err := verifier.ApplyAndVerify(context.Background(), dev, wifiDesired(&WiFiConfiguration{
		SSID: StringPtr("guest"), Password: StringPtr("guest-pass"),
	}), "SHSW-1")

	require.NoError(t, err, "an unreachable device after a network change is not a verify error")
	require.NotNil(t, result.ApplyResult)
	assert.True(t, result.ApplyResult.Unconfirmed)
	assert.False(t, result.ConfigApplied)
	assert.Nil(t, result.VerifyResult, "no verify against a device that may be on another network")
	assert.Equal(t, 2, dev.gets, "and no read after the silent one")
}

func TestApplyConfig_WiFiReadBackSilenceWithoutANetworkChangeIsAFailure(t *testing.T) {
	// The same ssid with a key is not a network change, so a silent device is a plain failure.
	dev := newFakeWiFiDevice()
	dev.failGetFrom = 2

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		SSID: StringPtr("office"), Password: StringPtr("pw"),
	}))

	require.NoError(t, err)
	assert.False(t, result.Unconfirmed)
	assert.False(t, result.Success)
	require.NotEmpty(t, result.Failures)
	for _, f := range result.Failures {
		assert.Contains(t, f.Error, "read-back failed")
		assert.NotContains(t, f.Value, "pw")
	}
}

func TestApplyConfig_WiFiAccessPointReadBackSilenceIsAFailure(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.failGetFrom = 2

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)},
	}))

	require.NoError(t, err)
	assert.False(t, result.Unconfirmed, "an access point change does not move the station network")
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "wifi_ap.enabled", result.Failures[0].Path)
}

func TestApplyConfig_WiFiReadBackThatAnswersAnotherValueIsAFailure(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.ignoreWrites = true // the device answers 200 and keeps its old network

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		SSID: StringPtr("guest"), Password: StringPtr("guest-pass"),
	}))

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.False(t, result.Unconfirmed)
	require.Len(t, result.Failures, 1, "the key is never read back, so only the ssid can differ")
	assert.Equal(t, "wifi_sta.ssid", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, "office")
}

func TestApplyConfig_WiFiWriteFailureNeverCarriesTheKey(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.staErr = errors.New("400 bad request: key hunter2 is too short")

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		SSID: StringPtr("office"), Password: StringPtr("hunter2"),
	}))

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.NotEmpty(t, result.Failures)
	for _, f := range result.Failures {
		assert.NotContains(t, f.Error, "hunter2")
		assert.NotContains(t, f.Value, "hunter2")
	}
}

func TestApplyConfig_WiFiClientWithoutTheRoutesFailsInsteadOfFallingBack(t *testing.T) {
	// A plain client has no /settings/sta route. Apply reports that and never sends the nested
	// wifi_sta document on POST /settings, which the device ignores.
	client := newApplyTestClient()
	client.currentRaw = []byte(`{"wifi_sta":{"ssid":"office"}}`)

	result, err := applyWiFi(t, client, wifiDesired(&WiFiConfiguration{SSID: StringPtr("office")}))

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "wifi_sta", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, errNoWiFiClient.Error())
	assert.Empty(t, client.configs)
}

func TestApplyConfig_WiFiStationAndAccessPointFailuresAreSeparate(t *testing.T) {
	dev := newFakeWiFiDevice()
	dev.apErr = errors.New("503 busy")

	result, err := applyWiFi(t, dev, wifiDesired(&WiFiConfiguration{
		SSID: StringPtr("office"), Password: StringPtr("pw"),
		AccessPoint: &AccessPointConfig{Enable: BoolPtr(false)},
	}))

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, dev.staWrites, 1, "a failed access point write does not undo the station write")
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "wifi_ap.enabled", result.Failures[0].Path)
}

func TestUnsupportedApplySettings_WiFiRules(t *testing.T) {
	cases := []struct {
		name string
		wifi *WiFiConfiguration
		want []string
	}{
		{"station settings", &WiFiConfiguration{
			Enable: BoolPtr(true), SSID: StringPtr("office"), Password: StringPtr("pw"), IPv4Mode: StringPtr("dhcp"),
		}, nil},
		{"a static address", &WiFiConfiguration{
			IPv4Mode: StringPtr("static"),
			StaticIP: &StaticIPConfig{IP: StringPtr("192.168.1.60"), Netmask: StringPtr("255.255.255.0"),
				Gateway: StringPtr("192.168.1.1"), Nameserver: StringPtr("192.168.1.1")},
		}, nil},
		{"access point on and off", &WiFiConfiguration{
			AccessPoint: &AccessPointConfig{Enable: BoolPtr(false), Password: StringPtr("ap-pass")},
		}, nil},
		{"the access point ssid cannot be changed", &WiFiConfiguration{
			AccessPoint: &AccessPointConfig{SSID: StringPtr("my-own-name")},
		}, []string{"wifi.ap.ssid"}},
		{"the roam threshold has no Gen1 setting", &WiFiConfiguration{RoamThreshold: IntPtr(-70)},
			[]string{"wifi.roam_threshold"}},
		{"the access point key field has no Gen1 setting", &WiFiConfiguration{
			AccessPoint: &AccessPointConfig{Key: StringPtr("other")},
		}, []string{"wifi.ap.key"}},
		{"station and access point cannot both be on", &WiFiConfiguration{
			Enable: BoolPtr(true), AccessPoint: &AccessPointConfig{Enable: BoolPtr(true)},
		}, []string{"wifi.ap.enable", "wifi.enable"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, unsupportedFor(t, "SHSW-1", wifiDesired(c.wifi)))
		})
	}
}

func TestApplyDesiredConfig_AppliesWiFiAndVerifiesIt(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "plug")
	setDesired(t, repo, &DeviceConfiguration{WiFi: &WiFiConfiguration{
		Enable: BoolPtr(true), SSID: StringPtr("office"), Password: StringPtr("s3cret-pass"),
		IPv4Mode: StringPtr("static"),
		StaticIP: &StaticIPConfig{IP: StringPtr("192.168.1.60"), Netmask: StringPtr("255.255.255.0"),
			Gateway: StringPtr("192.168.1.1"), Nameserver: StringPtr("192.168.1.1")},
		AccessPoint: &AccessPointConfig{Enable: BoolPtr(false), Password: StringPtr("ap-s3cret")},
	}})
	dev := newFakeWiFiDevice()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHSW-1")

	require.NoError(t, err)
	// The device never returns either key, so verify must not count their absence as a difference.
	assert.True(t, result.ConfigApplied, "differences: %+v", result.VerifyResult)
	sta := dev.settings["wifi_sta"].(map[string]interface{})
	assert.Equal(t, "192.168.1.60", sta["ip"])
	assert.Equal(t, "static", sta["ipv4_method"])
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_RefusesAWiFiAccessPointNameBeforeTouchingTheDevice(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "plug")
	setDesired(t, repo, &DeviceConfiguration{WiFi: &WiFiConfiguration{
		AccessPoint: &AccessPointConfig{SSID: StringPtr("my-own-name")},
	}})
	dev := newFakeWiFiDevice()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHSW-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "wifi.ap.ssid")
	assert.Zero(t, dev.setCalls)
	assert.Empty(t, dev.apWrites)
}

func TestApplyDesiredConfig_UnconfirmedWiFiStaysPending(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "plug")
	setDesired(t, repo, &DeviceConfiguration{WiFi: &WiFiConfiguration{
		SSID: StringPtr("guest"), Password: StringPtr("guest-pass"),
	}})
	dev := newFakeWiFiDevice()
	dev.failGetFrom = 2

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotNil(t, result.ApplyResult)
	assert.True(t, result.ApplyResult.Unconfirmed)
	assert.False(t, repo.devices[1].ConfigApplied, "an unconfirmed write is not an applied configuration")
}

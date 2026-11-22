package configuration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gen1 Wi-Fi roaming and the recovery reboot flag. Both ride the one POST /settings as flat
// parameters (ap_roaming_enabled, ap_roaming_threshold, wifirecovery_reboot_enabled) and are read
// back from GET /settings as the ap_roaming object and a flat flag. A device whose GET does not
// report a key does not have it, so the apply is refused before anything is written.

// gen1RecoveryModels are the supported models the reference lists for wifirecovery_reboot_enabled.
var gen1RecoveryModels = []string{"SHSW-1", "SHSW-PM", "SHSW-25", "SHPLG-S", "SHPLG-1"}

// newRoamingDevice is a Gen1 device whose firmware reports both keys.
func newRoamingDevice() *fakeWiFiDevice {
	d := newFakeWiFiDevice()
	d.settings["ap_roaming"] = map[string]interface{}{"enabled": false, "threshold": float64(-70)}
	d.settings["wifirecovery_reboot_enabled"] = false
	return d
}

func applyWiFiAs(t *testing.T, model string, dev ShellyClient, desired *DeviceConfiguration) (*ApplyResult, error) {
	t.Helper()
	return NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), dev, desired, model)
}

func TestApplyConfig_RoamingIsWrittenToSettingsAndReadBack(t *testing.T) {
	dev := newRoamingDevice()
	desired := wifiDesired(&WiFiConfiguration{RoamEnable: BoolPtr(true), RoamThreshold: IntPtr(-75)})

	verifier := NewConfigVerifier(NewGen1Converter(nil), nil)
	got, err := verifier.ApplyAndVerify(context.Background(), dev, desired, "SHSW-1")

	require.NoError(t, err)
	require.True(t, got.ApplyResult.Success, "failures: %+v", got.ApplyResult.Failures)
	require.Len(t, dev.posted, 1)
	assert.Equal(t, map[string]interface{}{"ap_roaming_enabled": true, "ap_roaming_threshold": float64(-75)}, dev.posted[0])
	assert.Equal(t, 2, got.ApplyResult.SettingsCount)
	assert.Equal(t, 2, got.ApplyResult.AppliedCount)
	assert.Empty(t, dev.staWrites)
	assert.Empty(t, dev.apWrites)
	assert.False(t, got.ApplyResult.RequiresReboot, "the reference names no reboot for roaming")
	assert.True(t, got.ConfigApplied, "differences: %+v", got.VerifyResult.Differences)
	assert.Equal(t, map[string]interface{}{"enabled": true, "threshold": float64(-75)}, dev.settings["ap_roaming"])
}

func TestApplyConfig_RoamingReadBackThatAnswersAnotherValueIsADifference(t *testing.T) {
	dev := newRoamingDevice()
	dev.dropKeys = map[string]bool{"ap_roaming_threshold": true}
	desired := wifiDesired(&WiFiConfiguration{RoamThreshold: IntPtr(-75)})

	verifier := NewConfigVerifier(NewGen1Converter(nil), nil)
	got, err := verifier.ApplyAndVerify(context.Background(), dev, desired, "SHSW-1")

	require.NoError(t, err)
	assert.False(t, got.ConfigApplied)
	require.NotNil(t, got.VerifyResult)
	paths := []string{}
	for _, diff := range got.VerifyResult.Differences {
		paths = append(paths, diff.Path)
	}
	assert.Contains(t, paths, "wifi.roam_threshold")
}

func TestUnsupportedApplySettings_RoamingRange(t *testing.T) {
	for _, tc := range []struct {
		threshold int
		want      []string
	}{
		{-100, nil}, {-50, nil}, {-70, nil},
		{-101, []string{"wifi.roam_threshold"}}, {-49, []string{"wifi.roam_threshold"}}, {0, []string{"wifi.roam_threshold"}},
	} {
		got := unsupportedFor(t, "SHSW-1", wifiDesired(&WiFiConfiguration{RoamThreshold: IntPtr(tc.threshold)}))
		assert.Equal(t, tc.want, got, "threshold %d", tc.threshold)
	}
}

func TestApplyConfig_ThresholdOutsideTheRangeIsRefusedBeforeAnyWrite(t *testing.T) {
	dev := newRoamingDevice()
	desired := &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("renamed")}},
		WiFi:   &WiFiConfiguration{RoamThreshold: IntPtr(-101)},
	}

	svc, repo := newServiceWithDesiredName(t, "plug")
	setDesired(t, repo, desired)
	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHSW-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "wifi.roam_threshold")
	assert.Zero(t, dev.setCalls)
}

func TestUnsupportedApplySettings_RecoveryFlagPerModel(t *testing.T) {
	desired := wifiDesired(&WiFiConfiguration{RecoveryReboot: BoolPtr(true)})
	for _, model := range gen1RecoveryModels {
		assert.Empty(t, unsupportedFor(t, model, desired), model)
	}
	for _, model := range []string{"SHIX3-1"} {
		assert.Equal(t, []string{"wifi.recovery_reboot"}, unsupportedFor(t, model, desired), model)
	}
}

func TestApplyConfig_RecoveryFlagIsWrittenOnTheModelsThatHaveIt(t *testing.T) {
	for _, model := range gen1RecoveryModels {
		t.Run(model, func(t *testing.T) {
			dev := newRoamingDevice()

			result, err := applyWiFiAs(t, model, dev, wifiDesired(&WiFiConfiguration{RecoveryReboot: BoolPtr(true)}))

			require.NoError(t, err)
			assert.True(t, result.Success, "failures: %+v", result.Failures)
			require.Len(t, dev.posted, 1)
			assert.Equal(t, map[string]interface{}{"wifirecovery_reboot_enabled": true}, dev.posted[0])
			assert.Equal(t, 1, result.SettingsCount)
			assert.Equal(t, 1, result.AppliedCount)
			assert.Equal(t, true, dev.settings["wifirecovery_reboot_enabled"])
		})
	}
}

func TestApplyDesiredConfig_RecoveryFlagOnTheI3IsRefusedBeforeAnyWrite(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "i3")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("renamed")}},
		WiFi:   &WiFiConfiguration{RecoveryReboot: BoolPtr(true)},
	})
	dev := newRoamingDevice()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHIX3-1")

	require.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "wifi.recovery_reboot")
	assert.Zero(t, dev.setCalls)
}

// A device whose GET /settings reports no ap_roaming object, or no recovery flag, predates the
// key (reference changelog v1.10.0). Whether it ignores or refuses the parameter is not stated,
// and a refusal would fail the name and the timezone that ride the same POST, so nothing is sent.
func TestApplyConfig_AKeyTheDeviceDoesNotReportIsRefusedBeforeAnyWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		live   func(d *fakeWiFiDevice)
		wifi   *WiFiConfiguration
		wantIn string
	}{
		{"no ap_roaming object, a threshold", func(d *fakeWiFiDevice) { delete(d.settings, "ap_roaming") },
			&WiFiConfiguration{RoamThreshold: IntPtr(-70)}, "ap_roaming"},
		{"no ap_roaming object, the enable flag", func(d *fakeWiFiDevice) { delete(d.settings, "ap_roaming") },
			&WiFiConfiguration{RoamEnable: BoolPtr(true)}, "ap_roaming"},
		{"an ap_roaming object without a threshold, a threshold", func(d *fakeWiFiDevice) {
			d.settings["ap_roaming"] = map[string]interface{}{"enabled": false}
		}, &WiFiConfiguration{RoamThreshold: IntPtr(-70)}, "ap_roaming.threshold"},
		{"no recovery flag", func(d *fakeWiFiDevice) { delete(d.settings, "wifirecovery_reboot_enabled") },
			&WiFiConfiguration{RecoveryReboot: BoolPtr(true)}, "wifirecovery_reboot_enabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dev := newRoamingDevice()
			tc.live(dev)
			desired := &DeviceConfiguration{
				System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("renamed")}},
				WiFi:   tc.wifi,
			}

			result, err := applyWiFiAs(t, "SHSW-1", dev, desired)

			require.ErrorIs(t, err, ErrUnsupportedSettings)
			var refused *refusedBeforeWriteError
			require.ErrorAs(t, err, &refused, "the device still holds what it held")
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.wantIn)
			assert.Contains(t, err.Error(), "firmware")
			assert.Zero(t, dev.setCalls, "the name rides the same POST and must not be sent")
		})
	}
}

func TestApplyConfig_AKeyTheDeviceReportsPerKeyIsWritten(t *testing.T) {
	dev := newRoamingDevice()
	dev.settings["ap_roaming"] = map[string]interface{}{"enabled": false} // no threshold reported

	result, err := applyWiFiAs(t, "SHSW-1", dev, wifiDesired(&WiFiConfiguration{RoamEnable: BoolPtr(true)}))

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, dev.posted, 1)
	assert.Equal(t, map[string]interface{}{"ap_roaming_enabled": true}, dev.posted[0])
}

func TestApplyConfig_RoamingNeedsTheDeviceToBeReadable(t *testing.T) {
	dev := newRoamingDevice()
	dev.failGetFrom = 1

	result, err := applyWiFiAs(t, "SHSW-1", dev, wifiDesired(&WiFiConfiguration{RoamThreshold: IntPtr(-70)}))

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nothing was written")
	assert.Zero(t, dev.setCalls)
}

func TestApplyConfig_NoRoamingSettingNeedsNoKeyOnTheDevice(t *testing.T) {
	dev := newFakeWiFiDevice() // reports neither key

	result, err := applyWiFiAs(t, "SHSW-1", dev, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("renamed")}},
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, dev.setCalls)
}

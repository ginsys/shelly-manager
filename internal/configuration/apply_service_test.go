package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	onWrite      func()          // runs after each accepted write, to simulate a concurrent edit
	dropKeys     map[string]bool // form keys the device accepts but does not store
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
		if d.dropKeys[k] {
			continue
		}
		// Like a real Gen1 device: flat coiot_* parameters in, a nested "coiot" document out
		// with enabled, update_period and peer ("mcast" is stored as an empty peer).
		if sub, ok := strings.CutPrefix(k, "coiot_"); ok {
			doc, _ := d.settings["coiot"].(map[string]interface{})
			if doc == nil {
				doc = map[string]interface{}{}
			}
			if sub == "enable" {
				sub = "enabled"
			}
			if sub == "peer" && v == "mcast" {
				v = ""
			}
			doc[sub] = v
			d.settings["coiot"] = doc
			continue
		}
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

// SetCloudConfig is POST /settings/cloud. Like a real Gen1 device it stores the flag and reports
// the cloud section as {enabled, connected} on GET /settings, with no server.
func (d *fakeGen1Device) SetCloudConfig(ctx context.Context, enabled bool, server string) error {
	if err := d.applyTestClient.SetCloudConfig(ctx, enabled, server); err != nil {
		return err
	}
	if !d.ignoreWrites {
		d.settings["cloud"] = map[string]interface{}{"enabled": enabled, "connected": false}
	}
	return nil
}

// GetConfig returns the stored settings. Like a real Gen1 device it never returns the MQTT
// password: GET /settings omits mqtt.pass even after a write.
func (d *fakeGen1Device) GetConfig(_ context.Context) (*shelly.DeviceConfig, error) {
	out := make(map[string]interface{}, len(d.settings))
	for k, v := range d.settings {
		out[k] = v
	}
	if doc, ok := d.settings["mqtt"].(map[string]interface{}); ok {
		visible := make(map[string]interface{}, len(doc))
		for k, v := range doc {
			if k != "pass" {
				visible[k] = v
			}
		}
		out["mqtt"] = visible
	}
	raw, err := json.Marshal(out)
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

// A desired server in the combined host:port form is what a Gen1 device stores, but its
// read-back is split into server and port. The comparison must not report that as drift.
func TestApplyDesiredConfig_MQTTCombinedHostPortVerifies(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("broker.local:1883"),
		},
	})
	dev := newFakeGen1Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	mqtt, _ := dev.settings["mqtt"].(map[string]interface{})
	require.Equal(t, "broker.local:1883", mqtt["server"], "the device stores the combined form")
	assert.True(t, result.ConfigApplied, "the split read-back matches the combined desired value")
	assert.True(t, repo.devices[1].ConfigApplied)

	verify, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
	require.NoError(t, err)
	assert.True(t, verify.Match)
	assert.Empty(t, verify.Differences)
	assert.Equal(t, "broker.local:1883", *verify.Desired.MQTT.Server,
		"the reported desired value is the one the user stored")
}

// Control: a different port on the device is still drift after the endpoint is normalized.
func TestVerifyDesiredConfig_MQTTCombinedHostPortReportsPortDrift(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		MQTT: &MQTTConfiguration{Server: StringPtr("broker.local:1883")},
	})
	dev := newFakeGen1Device()
	dev.settings["mqtt"] = map[string]interface{}{"server": "broker.local:8883"}

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Match)
	require.NotEmpty(t, result.Differences)
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

func TestApplyDesiredConfig_AppliesCloudAndVerifiesIt(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office")}},
		Cloud:  &CloudConfiguration{Enable: BoolPtr(false)},
	})
	dev := newFakeGen1Device()
	dev.settings["cloud"] = map[string]interface{}{"enabled": true, "connected": true}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, []bool{false}, dev.cloudWrites)
	assert.Equal(t, map[string]interface{}{"enabled": false, "connected": false}, dev.settings["cloud"])
	assert.Equal(t, 0, result.ApplyResult.FailedCount)
	assert.True(t, repo.devices[1].ConfigApplied)
}

// A device can answer 200 and keep the old cloud flag. The read-back must catch it.
func TestApplyDesiredConfig_CloudStaysPendingWhenDeviceKeepsTheOldFlag(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		Cloud: &CloudConfiguration{Enable: BoolPtr(false)},
	})
	dev := newFakeGen1Device()
	dev.ignoreWrites = true
	dev.settings["cloud"] = map[string]interface{}{"enabled": true, "connected": true}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotNil(t, result.VerifyResult)
	assert.False(t, result.VerifyResult.Match)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// Gen1 reports no cloud server and POST /settings/cloud takes none, so a desired server can be
// neither written nor confirmed. It is refused before the device is touched.
func TestApplyDesiredConfig_RefusesACloudServer(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office")}},
		Cloud:  &CloudConfiguration{Enable: BoolPtr(true), Server: StringPtr("cloud.example.net:6012/jrpc")},
	})
	dev := newFakeGen1Device()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "cloud.server")
	assert.NotContains(t, err.Error(), "cloud.enable", "the flag itself is supported")
	assert.Equal(t, 0, dev.setCalls, "the supported name must not be written either")
	assert.Empty(t, dev.cloudWrites)
}

func TestApplyDesiredConfig_AppliesCoIoTAndVerifiesIt(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office")}},
		CoIoT:  &CoIoTConfiguration{Enable: BoolPtr(true), UpdatePeriod: IntPtr(30), Peer: StringPtr("")},
	})
	dev := newFakeGen1Device()
	dev.settings["coiot"] = map[string]interface{}{"enabled": false, "update_period": float64(15), "peer": "10.0.0.9:5683"}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, map[string]interface{}{"enabled": true, "update_period": float64(30), "peer": ""}, dev.settings["coiot"])
	assert.Equal(t, 0, result.ApplyResult.FailedCount)
	assert.True(t, repo.devices[1].ConfigApplied)
}

// A device can answer 200 and keep the old CoIoT values. The read-back must catch it.
func TestApplyDesiredConfig_CoIoTStaysPendingWhenDeviceKeepsTheOldValues(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		CoIoT: &CoIoTConfiguration{Enable: BoolPtr(true), UpdatePeriod: IntPtr(30)},
	})
	dev := newFakeGen1Device()
	dev.ignoreWrites = true
	dev.settings["coiot"] = map[string]interface{}{"enabled": false, "update_period": float64(15), "peer": ""}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotNil(t, result.VerifyResult)
	assert.False(t, result.VerifyResult.Match)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// The device accepts 15..65535 seconds. A value outside it is refused before anything is
// written, and so is the literal peer "mcast", which the device reads back as an empty peer and
// so could never verify.
func TestApplyDesiredConfig_RefusesCoIoTValuesTheDeviceCannotKeep(t *testing.T) {
	cases := map[string]struct {
		coiot *CoIoTConfiguration
		path  string
	}{
		"period below the range": {&CoIoTConfiguration{UpdatePeriod: IntPtr(14)}, "coiot.update_period"},
		"period zero":            {&CoIoTConfiguration{UpdatePeriod: IntPtr(0)}, "coiot.update_period"},
		"period above the range": {&CoIoTConfiguration{UpdatePeriod: IntPtr(65536)}, "coiot.update_period"},
		"literal mcast peer":     {&CoIoTConfiguration{Peer: StringPtr("mcast")}, "coiot.peer"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "tv-office")
			setDesired(t, repo, &DeviceConfiguration{
				System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office")}},
				CoIoT:  tc.coiot,
			})
			dev := newFakeGen1Device()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Equal(t, 0, dev.setCalls, "the supported name must not be written either")
		})
	}
}

func TestApplyDesiredConfig_AcceptsCoIoTPeriodBounds(t *testing.T) {
	for _, period := range []int{15, 65535} {
		svc, repo := newServiceWithDesiredName(t, "tv-office")
		setDesired(t, repo, &DeviceConfiguration{CoIoT: &CoIoTConfiguration{UpdatePeriod: IntPtr(period)}})

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")

		require.NoError(t, err, "period %d", period)
		assert.True(t, result.ConfigApplied, "period %d", period)
	}
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

// A stored desired configuration can hold fields the typed configuration does not know, for
// example a legacy device_configs row restored by an SMA import in the device's own flat
// shape. A plain decode drops them, so apply would write nothing, verify would report a
// match and the device would be marked applied. Apply and verify must refuse instead.
func TestApplyAndVerifyDesiredConfig_RefuseUnknownStoredFields(t *testing.T) {
	cases := map[string]string{
		"legacy flat device shape": `{"name":"tv-office"}`,
		"unknown nested field":     `{"mqtt":{"enable":true,"srv":"broker.local"}}`,
	}
	for name, stored := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "tv-office")
			repo.devices[1].DesiredConfig = stored
			dev := newFakeGen1Device()
			dev.settings["name"] = "old"

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Equal(t, 0, dev.setCalls, "nothing may be written")
			assert.False(t, repo.devices[1].ConfigApplied, "an unreadable configuration is never applied")

			verify, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

			require.Error(t, err)
			assert.Nil(t, verify)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		})
	}
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
	mqtt, _ := dev.settings["mqtt"].(map[string]interface{})
	require.Equal(t, "s3cret", mqtt["pass"], "the password was written")
	assert.True(t, result.ConfigApplied, "a write-only secret must not keep the device pending")
	assert.True(t, repo.devices[1].ConfigApplied)
}

// Control for the test above: skipping the password on read-back must not hide a real MQTT
// difference.
func TestApplyDesiredConfig_MQTTFieldTheDeviceDropsStaysPending(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable:   BoolPtr(true),
			Server:   StringPtr("broker.local"),
			Port:     IntPtr(1883),
			User:     StringPtr("iot"),
			Password: StringPtr("s3cret"),
		},
	})
	dev := newFakeGen1Device()
	dev.dropKeys = map[string]bool{"mqtt_user": true}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied, "the device did not store mqtt.user")
	require.NotNil(t, result.VerifyResult)
	assert.NotEmpty(t, result.VerifyResult.Differences)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// A device that rejects the login on the write is an error the API maps to 502, not a failed
// setting in a 200 response.
func TestApplyDesiredConfig_RejectedLoginOnWriteIsAnAuthError(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	repo.devices[1].ConfigApplied = true
	dev := newFakeGen1Device()
	dev.setErr = fmt.Errorf("POST /settings: %w", shelly.ErrAuthRequired)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, shelly.IsAuthError(err), "got %v", err)
	assert.Equal(t, 1, dev.setCalls, "no further write after the login is rejected")
	assert.False(t, repo.devices[1].ConfigApplied, "the device stays pending")
}

// The configuration written is the snapshot the compare-and-set checks, even when the stored
// configuration changes and changes back during the apply.
func TestApplyDesiredConfig_WritesTheSnapshotItChecks(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	original := repo.devices[1].DesiredConfig
	other, err := json.Marshal(&DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("other-name")}},
	})
	require.NoError(t, err)
	repo.afterGetDevice = func(call int) {
		switch call {
		case 1: // an edit lands right after the snapshot
			repo.devices[1].DesiredConfig = string(other)
		case 2: // and is undone before the compare-and-set
			repo.devices[1].DesiredConfig = original
		}
	}
	dev := newFakeGen1Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "tv-office", dev.settings["name"], "the snapshot is what was written")
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

// MySQL compares text with the column collation, often case-insensitive, so the compare-and-set
// must force a byte comparison there. The other providers compare exactly already.
func TestDesiredConfigMatchCondition_ExactOnMySQL(t *testing.T) {
	assert.Contains(t, desiredConfigMatchCondition("mysql"), "BINARY COALESCE(desired_config, '') = ?")
	for _, dialect := range []string{"sqlite", "postgres"} {
		assert.Equal(t, "id = ? AND COALESCE(desired_config, '') = ?", desiredConfigMatchCondition(dialect), dialect)
	}
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

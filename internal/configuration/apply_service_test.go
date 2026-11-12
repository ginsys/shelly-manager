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

	// HTTP Basic login: loginOn, loginUser and loginPass are the stored login; asUser and asPass
	// are what the original client sends with every request.
	loginOn              bool
	loginUser, loginPass string
	asUser, asPass       string
}

func newFakeGen1Device() *fakeGen1Device {
	d := &fakeGen1Device{settings: map[string]interface{}{"name": ""}}
	d.ip = "192.168.1.50"
	d.generation = 1
	return d
}

func (d *fakeGen1Device) SetConfig(_ context.Context, cfg map[string]interface{}) error {
	return d.setConfigAs(d.asUser, d.asPass, cfg)
}

// setConfigAs is POST /settings with the given credentials.
func (d *fakeGen1Device) setConfigAs(user, pass string, cfg map[string]interface{}) error {
	d.setCalls++
	if err := d.authorize(user, pass); err != nil {
		return err
	}
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
		// The flat sntp_server parameter in, a nested "sntp" document out (key names unverified
		// until the hardware readback).
		if k == "sntp_server" {
			doc, _ := d.settings["sntp"].(map[string]interface{})
			if doc == nil {
				doc = map[string]interface{}{}
			}
			doc["server"] = v
			d.settings["sntp"] = doc
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

// authorize is HTTP Basic auth on the device: once a login is on, a request that does not carry
// the stored username and password gets 401, whatever the request is.
func (d *fakeGen1Device) authorize(user, pass string) error {
	if d.loginOn && (user != d.loginUser || pass != d.loginPass) {
		return fmt.Errorf("401: %w", shelly.ErrAuthFailed)
	}
	return nil
}

// SetAuth is POST /settings/login with enabled=true. Like a real Gen1 device it reports the login
// as {enabled, unprotected, username} on GET /settings and never the password.
func (d *fakeGen1Device) SetAuth(ctx context.Context, username, password string) error {
	if err := d.authorize(d.asUser, d.asPass); err != nil {
		return err
	}
	if err := d.applyTestClient.SetAuth(ctx, username, password); err != nil {
		return err
	}
	if !d.ignoreWrites {
		d.loginOn, d.loginUser, d.loginPass = true, username, password
		d.settings["login"] = map[string]interface{}{"enabled": true, "unprotected": false, "username": username}
	}
	return nil
}

// ResetAuth is POST /settings/login with enabled=false.
func (d *fakeGen1Device) ResetAuth(ctx context.Context) error {
	if err := d.authorize(d.asUser, d.asPass); err != nil {
		return err
	}
	if err := d.applyTestClient.ResetAuth(ctx); err != nil {
		return err
	}
	if !d.ignoreWrites {
		d.loginOn = false
		d.settings["login"] = map[string]interface{}{"enabled": false, "unprotected": false, "username": d.loginUser}
	}
	return nil
}

// Reboot needs the credentials too.
func (d *fakeGen1Device) Reboot(ctx context.Context) error {
	if err := d.authorize(d.asUser, d.asPass); err != nil {
		return err
	}
	return d.applyTestClient.Reboot(ctx)
}

// loggedIn is the same device seen through a client that sends the given credentials.
type loggedIn struct {
	*fakeGen1Device
	user, pass string
}

func (c *loggedIn) SetConfig(_ context.Context, cfg map[string]interface{}) error {
	return c.setConfigAs(c.user, c.pass, cfg)
}

func (c *loggedIn) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	if err := c.authorize(c.user, c.pass); err != nil {
		return nil, err
	}
	return c.read()
}

func (c *loggedIn) Reboot(ctx context.Context) error {
	if err := c.authorize(c.user, c.pass); err != nil {
		return err
	}
	return c.applyTestClient.Reboot(ctx)
}

// GetConfig is GET /settings as the original client (asUser, asPass) sends it.
func (d *fakeGen1Device) GetConfig(_ context.Context) (*shelly.DeviceConfig, error) {
	if err := d.authorize(d.asUser, d.asPass); err != nil {
		return nil, err
	}
	return d.read()
}

// read returns the stored settings. Like a real Gen1 device it never returns the MQTT
// password: GET /settings omits mqtt.pass even after a write.
func (d *fakeGen1Device) read() (*shelly.DeviceConfig, error) {
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

// fakeLoginHooks stands in for the service that stores a device's credentials and builds its
// client. calls records the order in which the apply used it.
type fakeLoginHooks struct {
	dev       *fakeGen1Device
	calls     []string
	stored    []LoginChange
	storeErr  error
	clientErr error
}

func (h *fakeLoginHooks) Store(_ context.Context, deviceID uint, change LoginChange) error {
	h.calls = append(h.calls, "store")
	if h.storeErr != nil {
		return h.storeErr
	}
	h.stored = append(h.stored, change)
	return nil
}

// Client builds from the stored login: the last one stored, or the device's own before any.
func (h *fakeLoginHooks) Client(_ context.Context, _ uint) (ShellyClient, error) {
	h.calls = append(h.calls, "client")
	if h.clientErr != nil {
		return nil, h.clientErr
	}
	if len(h.stored) == 0 {
		return &loggedIn{fakeGen1Device: h.dev, user: h.dev.loginUser, pass: h.dev.loginPass}, nil
	}
	last := h.stored[len(h.stored)-1]
	if !last.Enabled {
		return &loggedIn{fakeGen1Device: h.dev}, nil
	}
	return &loggedIn{fakeGen1Device: h.dev, user: last.Username, pass: last.Password}, nil
}

func loginDesired(enable *bool, user, pass *string) *DeviceConfiguration {
	return &DeviceConfiguration{Auth: &AuthConfiguration{Enable: enable, Username: user, Password: pass}}
}

// Login apply is wired only once the service can store the new credentials and rebuild its
// client. Without hooks the login stays refused, as before, and nothing is written.
func TestApplyDesiredConfig_RefusesLoginWithoutLoginHooks(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))
	dev := newFakeGen1Device()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "login")
	assert.Empty(t, dev.loginWrites)
	assert.NotContains(t, err.Error(), testLoginPassword)
}

func TestApplyDesiredConfig_AppliesALoginStoresItAndVerifiesWithTheNewClient(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	hooks := &fakeLoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "the read-back must use the new credentials, the old client gets 401")
	assert.Equal(t, []string{"client", "store", "client"}, hooks.calls,
		"a client under the lock, then the device accepts, then store, then the client is rebuilt")
	assert.Equal(t, []LoginChange{{Enabled: true, Username: "admin", Password: testLoginPassword}}, hooks.stored)
	assert.True(t, dev.loginOn)
	assert.True(t, repo.devices[1].ConfigApplied)
	raw, jerr := json.Marshal(result)
	require.NoError(t, jerr)
	assert.NotContains(t, string(raw), testLoginPassword, "the password never leaves the apply")
}

func TestApplyDesiredConfig_DisablingTheLoginStoresNoCredentials(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.loginOn, dev.loginUser, dev.loginPass = true, "admin", "old"
	dev.asUser, dev.asPass = "admin", "old"
	dev.settings["login"] = map[string]interface{}{"enabled": true, "unprotected": false, "username": "admin"}
	hooks := &fakeLoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)
	setDesired(t, repo, loginDesired(BoolPtr(false), nil, nil))

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Equal(t, []LoginChange{{Enabled: false}}, hooks.stored)
	assert.False(t, dev.loginOn)
}

// A write the device rejects changes nothing, so nothing is stored.
func TestApplyDesiredConfig_RejectedLoginWriteStoresNothing(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.loginErr = errors.New("500 busy")
	hooks := &fakeLoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.Equal(t, []string{"client"}, hooks.calls, "nothing is stored after a rejected write")
	assert.False(t, repo.devices[1].ConfigApplied)
	assert.False(t, dev.loginOn)
}

// An earlier setting that fails keeps the login unwritten. Writing it would leave the device on
// a login that the stored credentials and the cached client do not know.
func TestApplyDesiredConfig_LoginIsNotWrittenAfterAnEarlierFailure(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.setErr = errors.New("503 busy")
	hooks := &fakeLoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)
	desired := loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword))
	desired.System = &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office")}}
	setDesired(t, repo, desired)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.Empty(t, dev.loginWrites, "the login must stay unwritten")
	assert.Equal(t, []string{"client"}, hooks.calls)
}

// A queued apply holds the client it was called with, which may predate the login that the apply
// ahead of it changed. The lock holder builds its own from the stored login.
func TestApplyDesiredConfig_UsesAClientBuiltUnderTheLock(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.loginOn, dev.loginUser, dev.loginPass = true, "admin", "new"
	dev.settings["login"] = map[string]interface{}{"enabled": true, "unprotected": false, "username": "admin"}
	hooks := &fakeLoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)
	setDesired(t, repo, &DeviceConfiguration{System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office-2")}}})
	stale := &loggedIn{fakeGen1Device: dev, user: "admin", pass: "old"}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, stale, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "the stale client would get 401")
}

// A read-back with no login section must not put the desired password into a difference.
func TestVerifyDesiredConfig_MissingLoginReadBackNeverEchoesThePassword(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))
	dev := newFakeGen1Device()

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Match)
	raw, jerr := json.Marshal(result)
	require.NoError(t, jerr)
	assert.NotContains(t, string(raw), testLoginPassword)
}

// The device accepted the login but the new credentials could not be stored. The apply says
// so by error and never claims applied, because the stored login is now stale.
func TestApplyDesiredConfig_StoreFailureAfterAnAcceptedLoginIsAnError(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	hooks := &fakeLoginHooks{dev: dev, storeErr: errors.New("database is locked")}
	svc.SetLoginHooks(hooks)
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "could not be stored")
	assert.NotContains(t, err.Error(), testLoginPassword)
	assert.Equal(t, []string{"client", "store"}, hooks.calls, "no client is rebuilt after a failed store")
	assert.False(t, repo.devices[1].ConfigApplied)
}

// A device that answers 200 and keeps the old login stays pending.
func TestApplyDesiredConfig_LoginStaysPendingWhenDeviceKeepsTheOldLogin(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.ignoreWrites = true
	hooks := &fakeLoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// The device takes a username and password of 1 to 50 characters, both together. Anything else
// is refused before any write, and so is a credential with the flag off or unset, which could
// never read back as written.
func TestApplyDesiredConfig_RefusesALoginTheDeviceCannotKeep(t *testing.T) {
	long := strings.Repeat("x", 51)
	cases := map[string]struct {
		desired *DeviceConfiguration
		path    string
	}{
		"enabled without a username":  {loginDesired(BoolPtr(true), nil, StringPtr("pw")), "auth.user"},
		"enabled with empty username": {loginDesired(BoolPtr(true), StringPtr(""), StringPtr("pw")), "auth.user"},
		"enabled without a password":  {loginDesired(BoolPtr(true), StringPtr("admin"), nil), "auth.pass"},
		"username too long":           {loginDesired(BoolPtr(true), StringPtr(long), StringPtr("pw")), "auth.user"},
		"password too long":           {loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(long)), "auth.pass"},
		"username without the flag":   {loginDesired(nil, StringPtr("admin"), nil), "auth.enable"},
		"disabled with a username":    {loginDesired(BoolPtr(false), StringPtr("admin"), nil), "auth.user"},
		"disabled with a password":    {loginDesired(BoolPtr(false), nil, StringPtr("pw")), "auth.pass"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "tv-office")
			dev := newFakeGen1Device()
			svc.SetLoginHooks(&fakeLoginHooks{dev: dev})
			tc.desired.System = &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv-office")}}
			setDesired(t, repo, tc.desired)

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Equal(t, 0, dev.setCalls, "the supported name must not be written either")
			assert.Empty(t, dev.loginWrites)
			assert.NotContains(t, err.Error(), testLoginPassword)
		})
	}
}

func TestApplyDesiredConfig_AcceptsLoginLengthBounds(t *testing.T) {
	for _, n := range []int{1, 50} {
		svc, repo := newServiceWithDesiredName(t, "tv-office")
		dev := newFakeGen1Device()
		svc.SetLoginHooks(&fakeLoginHooks{dev: dev})
		setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr(strings.Repeat("u", n)), StringPtr(strings.Repeat("p", n))))

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

		require.NoError(t, err, "length %d", n)
		assert.True(t, result.ConfigApplied, "length %d", n)
	}
}

// Verify alone never needs the password: the device does not return it.
func TestVerifyDesiredConfig_DoesNotCompareTheLoginPassword(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, loginDesired(BoolPtr(true), StringPtr("admin"), StringPtr(testLoginPassword)))
	dev := newFakeGen1Device()
	dev.settings["login"] = map[string]interface{}{"enabled": true, "unprotected": false, "username": "admin"}

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Match, "differences: %+v", result.Differences)
}

func TestLoginChangeNeverPrintsThePassword(t *testing.T) {
	change := LoginChange{Enabled: true, Username: "admin", Password: testLoginPassword}

	for _, text := range []string{fmt.Sprintf("%v", change), fmt.Sprintf("%+v", change), fmt.Sprintf("%#v", change), fmt.Sprint(&change)} {
		assert.NotContains(t, text, testLoginPassword)
	}
	raw, err := json.Marshal(change)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), testLoginPassword)
}

func TestApplyDesiredConfig_RefusesSettingsTheConverterDrops(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{Name: StringPtr("tv-office")},
			Debug:  &DebugConfig{Level: 2},
		},
	})
	dev := newFakeGen1Device()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "debug")
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

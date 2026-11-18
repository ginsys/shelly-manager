package configuration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// loginGen2Device is a Gen2 device with a login. It reports auth_en in its device info, answers a
// protected read only to the login it holds, and records the order of the writes it gets.
type loginGen2Device struct {
	*fakeGen2Device
	authOn      bool
	user, pass  string
	ignoreLogin bool // the device accepts Shelly.SetAuth but keeps its old login
	sequence    []string
}

func newLoginGen2Device() *loginGen2Device {
	return &loginGen2Device{fakeGen2Device: newFakeGen2Device()}
}

func (d *loginGen2Device) CallConfigSetter(ctx context.Context, method string, params map[string]interface{}) (bool, error) {
	d.sequence = append(d.sequence, method)
	return d.fakeGen2Device.CallConfigSetter(ctx, method, params)
}

func (d *loginGen2Device) SetAuth(ctx context.Context, username, password string) error {
	if err := d.fakeGen2Device.SetAuth(ctx, username, password); err != nil {
		return err
	}
	d.sequence = append(d.sequence, "login")
	if !d.ignoreLogin {
		d.authOn, d.user, d.pass = true, username, password
	}
	return nil
}

func (d *loginGen2Device) ResetAuth(ctx context.Context) error {
	if err := d.fakeGen2Device.ResetAuth(ctx); err != nil {
		return err
	}
	d.sequence = append(d.sequence, "login")
	if !d.ignoreLogin {
		d.authOn, d.user, d.pass = false, "", ""
	}
	return nil
}

func (d *loginGen2Device) GetInfo(ctx context.Context) (*shelly.DeviceInfo, error) {
	info, err := d.fakeGen2Device.GetInfo(ctx)
	if err != nil || info == nil {
		return info, err
	}
	info.AuthEn = d.authOn
	return info, nil
}

// loggedInGen2 is a client that sends a login: a protected read fails with the login's error
// unless the credentials are the device's.
type loggedInGen2 struct {
	*loginGen2Device
	user, pass string
}

func (c *loggedInGen2) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	if c.authOn && (c.user != c.loginGen2Device.user || c.pass != c.loginGen2Device.pass) {
		return nil, shelly.ErrAuthRequired
	}
	return c.loginGen2Device.GetConfig(ctx)
}

// gen2LoginHooks stands in for the service that stores a device's credentials and builds its client.
type gen2LoginHooks struct {
	dev       *loginGen2Device
	calls     []string
	stored    []LoginChange
	storeErr  error
	clientErr error
}

func (h *gen2LoginHooks) Store(_ context.Context, _ uint, change LoginChange) error {
	h.calls = append(h.calls, "store")
	if h.storeErr != nil {
		return h.storeErr
	}
	h.stored = append(h.stored, change)
	return nil
}

func (h *gen2LoginHooks) Client(_ context.Context, _ uint) (ShellyClient, error) {
	h.calls = append(h.calls, "client")
	if h.clientErr != nil {
		return nil, h.clientErr
	}
	if len(h.stored) == 0 {
		return &loggedInGen2{loginGen2Device: h.dev, user: h.dev.user, pass: h.dev.pass}, nil
	}
	last := h.stored[len(h.stored)-1]
	if !last.Enabled {
		return &loggedInGen2{loginGen2Device: h.dev}, nil
	}
	return &loggedInGen2{loginGen2Device: h.dev, user: last.Username, pass: last.Password}, nil
}

func gen2LoginService(t *testing.T, desired *DeviceConfiguration, dev *loginGen2Device) (*ConfigurationService, *mockRepository, *gen2LoginHooks) {
	t.Helper()
	svc, repo := newGen2Service(t, desired)
	hooks := &gen2LoginHooks{dev: dev}
	svc.SetLoginHooks(hooks)
	return svc, repo, hooks
}

func loginOn(user, pass string) *AuthConfiguration {
	return &AuthConfiguration{Enable: BoolPtr(true), Username: StringPtr(user), Password: StringPtr(pass)}
}

func TestApplyDesiredConfig_Gen2WritesTheLoginLastAndStoresIt(t *testing.T) {
	desired := systemDesired()
	desired.Auth = loginOn("admin", "secret-pw")
	dev := newLoginGen2Device()
	svc, repo, hooks := gen2LoginService(t, desired, dev)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Equal(t, []string{"Sys.SetConfig", "login"}, dev.sequence, "the login is the last write")
	assert.Equal(t, []loginWrite{{username: "admin", password: "secret-pw"}}, dev.loginWrites)
	assert.Equal(t, []LoginChange{{Enabled: true, Username: "admin", Password: "secret-pw"}}, hooks.stored)
	assert.Equal(t, []string{"client", "store", "client"}, hooks.calls, "stored once the device accepted it, then the client is rebuilt")
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	assert.True(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_Gen2WritesTheLoginAfterTheWiFiStation(t *testing.T) {
	// The network the device is on, with a new password: written, but no network move.
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{SSID: StringPtr("home"), Password: StringPtr("wifi-pass-1")}}
	desired.Auth = loginOn("admin", "secret-pw")
	dev := newLoginGen2Device()
	svc, _, _ := gen2LoginService(t, desired, dev)

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	require.NotEmpty(t, dev.sequence)
	assert.Equal(t, "login", dev.sequence[len(dev.sequence)-1])
}

func TestApplyDesiredConfig_Gen2SwitchesTheLoginOffAndStoresNone(t *testing.T) {
	desired := &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(false)}}
	dev := newLoginGen2Device()
	dev.authOn, dev.user, dev.pass = true, "admin", "old-pw"
	svc, repo, hooks := gen2LoginService(t, desired, dev)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Equal(t, []loginWrite{{reset: true}}, dev.loginWrites)
	assert.Equal(t, []LoginChange{{Enabled: false}}, hooks.stored)
	assert.True(t, result.ConfigApplied, "%v", result.VerifyResult)
	assert.True(t, repo.devices[1].ConfigApplied)
}

// A login the device accepted but does not report is not a match: the read-back compares the
// enable flag the device reports in its device info.
func TestApplyDesiredConfig_Gen2ALoginTheDeviceDoesNotReportIsNotApplied(t *testing.T) {
	desired := &DeviceConfiguration{Auth: loginOn("admin", "secret-pw")}
	dev := newLoginGen2Device()
	dev.ignoreLogin = true
	svc, repo, _ := gen2LoginService(t, desired, dev)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
	require.NotNil(t, result.VerifyResult)
	paths := []string{}
	for _, diff := range result.VerifyResult.Differences {
		paths = append(paths, diff.Path)
	}
	assert.Contains(t, paths, "auth.enable")
}

func TestVerifyConfig_Gen2ReadsTheLoginFromTheDeviceInfo(t *testing.T) {
	dev := newLoginGen2Device()
	dev.authOn = true
	verifier := NewConfigVerifier(NewGen2Converter(nil), nil)

	match, err := verifier.VerifyConfig(context.Background(), dev, &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(true)}}, "")
	require.NoError(t, err)
	assert.True(t, match.Match, "%v", match.Differences)

	differ, err := verifier.VerifyConfig(context.Background(), dev, &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(false)}}, "")
	require.NoError(t, err)
	assert.False(t, differ.Match)
}

func TestVerifyConfig_Gen2AFailedDeviceInfoReadFailsTheVerify(t *testing.T) {
	dev := newLoginGen2Device()
	dev.infoErr = errNoRoute
	verifier := NewConfigVerifier(NewGen2Converter(nil), nil)

	result, err := verifier.VerifyConfig(context.Background(), dev, &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(true)}}, "")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestApplyDesiredConfig_Gen2AnEarlierFailureLeavesTheLoginUnwritten(t *testing.T) {
	desired := systemDesired()
	desired.Auth = loginOn("admin", "secret-pw")
	dev := newLoginGen2Device()
	dev.refuse = &shelly.DeviceError{IP: "192.168.1.60", Generation: 2, Operation: "Sys.SetConfig", StatusCode: 400, Message: "invalid"}
	svc, _, hooks := gen2LoginService(t, desired, dev)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.NotEmpty(t, result.ApplyResult.Failures)
	assert.Empty(t, dev.loginWrites, "a failed section leaves the login unwritten")
	assert.Empty(t, hooks.stored)
}

func TestApplyDesiredConfig_Gen2KeepsTheLoginPasswordOutOfAFailure(t *testing.T) {
	desired := &DeviceConfiguration{Auth: loginOn("admin", "secret-pw")}
	dev := newLoginGen2Device()
	dev.loginErr = &shelly.DeviceError{IP: "192.168.1.60", Generation: 2, Operation: "Shelly.SetAuth", StatusCode: 400, Message: "bad login secret-pw"}
	svc, _, hooks := gen2LoginService(t, desired, dev)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	if err != nil {
		assert.NotContains(t, err.Error(), "secret-pw")
	}
	require.NotNil(t, result)
	require.NotEmpty(t, result.ApplyResult.Failures)
	for _, failure := range result.ApplyResult.Failures {
		assert.NotContains(t, failure.Error, "secret-pw")
		assert.NotContains(t, failure.Value, "secret-pw")
	}
	assert.Empty(t, hooks.stored, "a refused login stores nothing")
	assert.False(t, result.ConfigApplied)
}

// A SetAuth answer that never arrives may have landed. It is a failure, never a success, nothing is
// stored, and the text says the stored login may now be stale.
func TestApplyDesiredConfig_Gen2ALostLoginAnswerIsAFailureThatStoresNothing(t *testing.T) {
	desired := &DeviceConfiguration{Auth: loginOn("admin", "secret-pw")}
	dev := newLoginGen2Device()
	dev.loginErr = errNoRoute
	svc, repo, hooks := gen2LoginService(t, desired, dev)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	require.NotNil(t, result.ApplyResult)
	require.Len(t, result.ApplyResult.Failures, 1)
	assert.Equal(t, "auth", result.ApplyResult.Failures[0].Path)
	assert.Contains(t, result.ApplyResult.Failures[0].Error, "stored login may")
	assert.Empty(t, hooks.stored)
	assert.False(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_Gen2ReportsALoginThatCouldNotBeStored(t *testing.T) {
	desired := &DeviceConfiguration{Auth: loginOn("admin", "secret-pw")}
	dev := newLoginGen2Device()
	svc, repo, hooks := gen2LoginService(t, desired, dev)
	hooks.storeErr = errors.New("database is locked")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "could not be stored")
	assert.NotContains(t, err.Error(), "secret-pw")
	assert.False(t, repo.devices[1].ConfigApplied)
}

// Refusals leave the device and the stored flag alone.
func TestApplyDesiredConfig_Gen2RefusesLoginsThatCouldLockTheManagerOut(t *testing.T) {
	for _, tc := range []struct {
		name    string
		desired *DeviceConfiguration
		path    string
	}{
		{"a user other than admin", &DeviceConfiguration{Auth: loginOn("operator", "secret-pw")}, "auth.user"},
		{"login on with no password", &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(true), Username: StringPtr("admin")}}, "auth.pass"},
		{"login on with an empty password", &DeviceConfiguration{Auth: loginOn("admin", "")}, "auth.pass"},
		{"login on with the mask as password", &DeviceConfiguration{Auth: loginOn("admin", "********")}, "auth.pass"},
		{"login off with a user", &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(false), Username: StringPtr("admin")}}, "auth.user"},
		{"login off with a password", &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(false), Password: StringPtr("secret-pw")}}, "auth.pass"},
		{"a user with no enable", &DeviceConfiguration{Auth: &AuthConfiguration{Username: StringPtr("admin")}}, "auth.enable"},
		{"a realm", &DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(true), Username: StringPtr("admin"), Password: StringPtr("secret-pw"), Realm: StringPtr("shelly")}}, "auth.realm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dev := newLoginGen2Device()
			svc, repo, hooks := gen2LoginService(t, tc.desired, dev)

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, ErrUnsupportedSettings)
			assert.Contains(t, err.Error(), tc.path)
			assert.NotContains(t, err.Error(), "secret-pw")
			assert.Empty(t, dev.sequence, "nothing is written when the apply is refused")
			assert.Empty(t, hooks.stored)
			assert.False(t, repo.devices[1].ConfigApplied)
		})
	}
}

// A login change in the same apply as a station network move: a lost answer could be either, and
// the stored credentials could not be trusted.
func TestApplyDesiredConfig_Gen2RefusesALoginTogetherWithAStationNetworkMove(t *testing.T) {
	desired := gen2WiFiDesired() // a new network
	desired.Auth = loginOn("admin", "secret-pw")
	dev := newLoginGen2Device()
	svc, repo, hooks := gen2LoginService(t, desired, dev)

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "auth")
	assert.Empty(t, dev.sequence, "nothing is written when the apply is refused")
	assert.Empty(t, hooks.stored)
	assert.False(t, repo.devices[1].ConfigApplied)
}

func TestApplyDesiredConfig_Gen2RefusesTheLoginWhereNothingCanStoreIt(t *testing.T) {
	svc, repo := newGen2Service(t, &DeviceConfiguration{Auth: loginOn("admin", "secret-pw")})
	dev := newLoginGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrUnsupportedSettings)
	assert.Contains(t, err.Error(), "auth")
	assert.Empty(t, dev.sequence)
	assert.False(t, repo.devices[1].ConfigApplied)
}

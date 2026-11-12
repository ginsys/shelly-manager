package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
)

type applyTestClient struct {
	ip          string
	generation  int
	configErr   error
	setErr      error
	rebootErr   error
	reboots     int // Reboot calls, failed ones included
	testConnErr error
	configs     []map[string]interface{}
	currentRaw  json.RawMessage

	// Login surface (LoginSettingsClient): POST /settings/login. order records "config" and
	// "login" writes in the sequence they arrive.
	loginWrites []loginWrite
	loginErr    error
	order       []string

	// Per-channel relay surface (RelaySettingsClient). relayLive is what GET /settings/relay/<n>
	// returns; when nil the device echoes the last form written to that channel.
	relayWrites []relayWrite
	relayLive   map[int]map[string]interface{}
	relaySetErr error
	relayGetErr error
	// relaySetErrAt refuses the write of one channel only.
	relaySetErrAt map[int]error

	// Cloud surface (CloudSettingsClient): POST /settings/cloud. cloudServers records the server
	// argument of each call, so a test can prove that apply never sends one.
	cloudWrites  []bool
	cloudServers []string
	cloudSetErr  error
}

func (m *applyTestClient) SetCloudConfig(ctx context.Context, enabled bool, server string) error {
	if m.cloudSetErr != nil {
		return m.cloudSetErr
	}
	m.cloudWrites = append(m.cloudWrites, enabled)
	m.cloudServers = append(m.cloudServers, server)
	return nil
}

type relayWrite struct {
	channel int
	form    map[string]interface{}
}

func (m *applyTestClient) SetRelaySettings(ctx context.Context, channel int, settings map[string]interface{}) error {
	if m.relaySetErr != nil {
		return m.relaySetErr
	}
	if err, ok := m.relaySetErrAt[channel]; ok {
		return err
	}
	m.relayWrites = append(m.relayWrites, relayWrite{channel: channel, form: settings})
	return nil
}

func (m *applyTestClient) GetRelaySettings(ctx context.Context, channel int) (map[string]interface{}, error) {
	if m.relayGetErr != nil {
		return nil, m.relayGetErr
	}
	if live, ok := m.relayLive[channel]; ok {
		return live, nil
	}
	echo := map[string]interface{}{}
	for _, w := range m.relayWrites {
		if w.channel != channel {
			continue
		}
		for k, v := range w.form {
			// A device returns JSON numbers as float64.
			if n, ok := v.(int); ok {
				v = float64(n)
			}
			echo[k] = v
		}
	}
	return echo, nil
}

func newApplyTestClient() *applyTestClient {
	return &applyTestClient{
		ip:         "192.168.1.100",
		generation: 1,
		configs:    []map[string]interface{}{},
		currentRaw: json.RawMessage(`{"name":"test"}`),
	}
}

func (m *applyTestClient) SetConfig(ctx context.Context, config map[string]interface{}) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.configs = append(m.configs, config)
	m.order = append(m.order, "config")
	return nil
}

// loginWrite is one call to POST /settings/login: SetAuth, or ResetAuth when reset is true.
type loginWrite struct {
	reset    bool
	username string
	password string
}

// SetAuth is POST /settings/login with enabled=true (LoginSettingsClient).
func (m *applyTestClient) SetAuth(_ context.Context, username, password string) error {
	if m.loginErr != nil {
		return m.loginErr
	}
	m.loginWrites = append(m.loginWrites, loginWrite{username: username, password: password})
	m.order = append(m.order, "login")
	return nil
}

// ResetAuth is POST /settings/login with enabled=false (LoginSettingsClient).
func (m *applyTestClient) ResetAuth(_ context.Context) error {
	if m.loginErr != nil {
		return m.loginErr
	}
	m.loginWrites = append(m.loginWrites, loginWrite{reset: true})
	m.order = append(m.order, "login")
	return nil
}

func (m *applyTestClient) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	if m.configErr != nil {
		return nil, m.configErr
	}
	return &shelly.DeviceConfig{
		Raw: m.currentRaw,
	}, nil
}

func (m *applyTestClient) GetInfo(ctx context.Context) (*shelly.DeviceInfo, error) {
	return &shelly.DeviceInfo{
		ID:         "test-device",
		Generation: m.generation,
	}, nil
}

func (m *applyTestClient) Reboot(ctx context.Context) error {
	m.reboots++
	return m.rebootErr
}

func (m *applyTestClient) TestConnection(ctx context.Context) error {
	return m.testConnErr
}

func (m *applyTestClient) GetGeneration() int {
	return m.generation
}

func (m *applyTestClient) GetIP() string {
	return m.ip
}

func TestNewConfigApplier(t *testing.T) {
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)
	require.NotNil(t, applier)
	assert.NotNil(t, applier.converter)
	assert.NotNil(t, applier.comparator)
	assert.NotNil(t, applier.logger)
}

func TestApplyConfig_EmptyConfig(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{}
	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.FailedCount)
}

func TestApplyConfig_MQTTSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("broker.local"),
			Port:   IntPtr(1883),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.FailedCount)
	assert.Greater(t, result.AppliedCount, 0)

	// Gen1 only accepts flat mqtt_* form parameters; a nested map would be sent as "map[...]".
	var sent map[string]interface{}
	for _, cfg := range client.configs {
		if _, ok := cfg["mqtt_enable"]; ok {
			sent = cfg
		}
		assert.NotContains(t, cfg, "mqtt", "a nested mqtt map must never be sent")
	}
	require.NotNil(t, sent, "flat MQTT parameters should have been sent")
	assert.Equal(t, true, sent["mqtt_enable"])
	assert.Equal(t, "broker.local:1883", sent["mqtt_server"])
}

func TestApplyConfig_RelaySettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("Kitchen"), DefaultState: StringPtr("off")},
			},
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

// The applier reaches the relay endpoints through RelaySettingsClient, so the real Gen1 client
// must satisfy it, or apply fails every relay on a real device.
var _ RelaySettingsClient = (*gen1.Client)(nil)
var _ CloudSettingsClient = (*gen1.Client)(nil)
var _ LoginSettingsClient = (*gen1.Client)(nil)
var _ WiFiSettingsClient = (*gen1.Client)(nil)

func relayApplyConfig() *DeviceConfiguration {
	return &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("Heater"), DefaultState: StringPtr("last"), AutoOn: IntPtr(5), AutoOff: IntPtr(3600)},
			},
		},
	}
}

func TestApplyConfig_RelayRequestShape(t *testing.T) {
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, relayApplyConfig(), "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.relayWrites, 1)
	assert.Equal(t, 0, client.relayWrites[0].channel)
	assert.Equal(t, map[string]interface{}{
		"name":          "Heater",
		"default_state": "last",
		"auto_on":       float64(5),
		"auto_off":      float64(3600),
	}, client.relayWrites[0].form)
	// The nested document goes nowhere: Gen1 ignores it.
	for _, cfg := range client.configs {
		assert.NotContains(t, cfg, "relays")
	}
	assert.Equal(t, 4, result.AppliedCount)
	assert.Equal(t, 4, result.SettingsCount)
}

func TestApplyConfig_RelayWritesEachChannel(t *testing.T) {
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	config := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, AutoOff: IntPtr(60)},
				{ID: 1, DefaultState: StringPtr("off")},
			},
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHSW-25")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.relayWrites, 2)
	assert.Equal(t, 0, client.relayWrites[0].channel)
	assert.Equal(t, map[string]interface{}{"auto_off": float64(60)}, client.relayWrites[0].form)
	assert.Equal(t, 1, client.relayWrites[1].channel)
	assert.Equal(t, map[string]interface{}{"default_state": "off"}, client.relayWrites[1].form)
}

func TestApplyConfig_RelayReadBackMismatchFails(t *testing.T) {
	client := newApplyTestClient()
	// The device answers 200 to the write and keeps the old auto_off.
	client.relayLive = map[int]map[string]interface{}{
		0: {"name": "Heater", "default_state": "last", "auto_on": float64(5), "auto_off": float64(0)},
	}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, relayApplyConfig(), "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 1, result.FailedCount)
	assert.Equal(t, 3, result.AppliedCount)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "relays[0].auto_off", result.Failures[0].Path)
	assert.Equal(t, "3600", result.Failures[0].Value)
	assert.Contains(t, result.Failures[0].Error, "0")
}

func TestApplyConfig_RelayWriteErrorFailsEveryKey(t *testing.T) {
	client := newApplyTestClient()
	client.relaySetErr = errors.New("device rejected settings")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, relayApplyConfig(), "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 4, result.FailedCount)
	assert.Equal(t, 0, result.AppliedCount)
	assert.Empty(t, client.relayWrites)
}

func TestApplyConfig_RelayReadBackErrorFailsEveryKey(t *testing.T) {
	client := newApplyTestClient()
	client.relayGetErr = errors.New("timeout")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, relayApplyConfig(), "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 4, result.FailedCount)
	require.NotEmpty(t, result.Failures)
	assert.Contains(t, result.Failures[0].Error, "read-back failed")
}

// plainApplyClient has no per-channel relay surface.
type plainApplyClient struct{ *applyTestClient }

func (plainApplyClient) SetRelaySettings() {}

func TestApplyConfig_RelayClientWithoutRelaySurfaceFails(t *testing.T) {
	// plainApplyClient shadows SetRelaySettings with a different signature, so it does not
	// satisfy RelaySettingsClient.
	client := plainApplyClient{newApplyTestClient()}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, relayApplyConfig(), "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "relays", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, "per-channel relay settings")
}

func TestUnsupportedApplySettings_RelayKeys(t *testing.T) {
	converter := NewGen1Converter(nil)

	ok := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, Name: StringPtr("a"), DefaultState: StringPtr("off"), AutoOn: IntPtr(1), AutoOff: IntPtr(2)},
	}}}
	unsupported, err := unsupportedApplySettings(converter, ok, "SHPLG-S", false)
	require.NoError(t, err)
	assert.Empty(t, unsupported)

	// The schedule, its rules, the appliance type and the channel's max power are written too.
	more := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, Schedule: BoolPtr(true), ScheduleRules: []string{"0000-0123456-on"}, ApplianceType: StringPtr("heater"), MaxPower: IntPtr(1500)},
	}}}
	unsupported, err = unsupportedApplySettings(converter, more, "SHSW-PM", false)
	require.NoError(t, err)
	assert.Empty(t, unsupported)
}

// The converted configuration carries no channel id: the list position is the channel. A
// desired id that disagrees with its position would write another channel than the one named,
// so the refusal check rejects it before the device is touched.
func TestUnsupportedApplySettings_RelayIDMustMatchPosition(t *testing.T) {
	converter := NewGen1Converter(nil)

	wrongChannel := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 1, AutoOff: IntPtr(60)},
	}}}
	unsupported, err := unsupportedApplySettings(converter, wrongChannel, "SHSW-25", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"relay.relays[0].id"}, unsupported)

	inOrder := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, AutoOff: IntPtr(60)},
		{ID: 1, AutoOff: IntPtr(30)},
	}}}
	unsupported, err = unsupportedApplySettings(converter, inOrder, "SHSW-25", false)
	require.NoError(t, err)
	assert.Empty(t, unsupported)

	// An id left out decodes as 0 and cannot be told from a stated 0, so the rule is strict:
	// a second entry with id 0 (stated or left out) would write channel 1 while naming channel 0,
	// and the final verification would then fail on the id. Both forms are refused.
	for name, relays := range map[string][]SingleRelayConfig{
		"id left out":   {{AutoOff: IntPtr(60)}, {AutoOff: IntPtr(30)}},
		"stated zero":   {{ID: 0}, {ID: 0, AutoOff: IntPtr(60)}},
		"id out of run": {{ID: 0, AutoOff: IntPtr(60)}, {ID: 2, AutoOff: IntPtr(30)}},
	} {
		cfg := &DeviceConfiguration{Relay: &RelayConfig{Relays: relays}}
		unsupported, err = unsupportedApplySettings(converter, cfg, "SHSW-25", false)
		require.NoError(t, err, name)
		assert.Equal(t, []string{"relay.relays[1].id"}, unsupported, name)
	}
}

// A device that ignores the write and reports a null name must not pass for a desired name that
// prints the same as nil.
func TestApplyConfig_RelayReadBackComparesStringTypes(t *testing.T) {
	client := newApplyTestClient()
	client.relayLive = map[int]map[string]interface{}{0: {"name": nil}}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	config := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
		{ID: 0, Name: StringPtr("<nil>")},
	}}}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 0, result.AppliedCount)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "relays[0].name", result.Failures[0].Path)
}

// Case 1 of ginsys/shelly-manager#434: one channel refuses its write and the other does not. The
// refused channel counts its keys as failed, the other channel is still written and counted as
// applied, and apply returns no error, because a refused setting is a result and not a stop.
func TestApplyConfig_RelayPartialFailure(t *testing.T) {
	refused := errors.New("device rejected settings")
	for _, tc := range []struct {
		name        string
		failChannel int
		okChannel   int
	}{
		{"channel 1 refused after channel 0", 1, 0},
		{"channel 0 refused before channel 1", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newApplyTestClient()
			client.relaySetErrAt = map[int]error{tc.failChannel: refused}
			config := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("A"), AutoOff: IntPtr(60)},
				{ID: 1, Name: StringPtr("B"), AutoOff: IntPtr(30)},
			}}}

			result, err := NewConfigApplier(NewGen1Converter(nil), nil).
				ApplyConfig(context.Background(), client, config, "SHSW-25")

			require.NoError(t, err)
			assert.False(t, result.Success)
			assert.Equal(t, 4, result.SettingsCount)
			assert.Equal(t, 2, result.AppliedCount)
			assert.Equal(t, 2, result.FailedCount)
			require.Len(t, client.relayWrites, 1)
			assert.Equal(t, tc.okChannel, client.relayWrites[0].channel)
			require.Len(t, result.Failures, 2)
			assert.Equal(t, fmt.Sprintf("relays[%d].auto_off", tc.failChannel), result.Failures[0].Path)
			assert.Equal(t, fmt.Sprintf("relays[%d].name", tc.failChannel), result.Failures[1].Path)
			for _, failure := range result.Failures {
				assert.Equal(t, refused.Error(), failure.Error)
			}
		})
	}
}

// Case 2: a negative id never equals a list position, so the refusal check names it before the
// device is touched, wherever it stands in the list.
func TestUnsupportedApplySettings_RelayNegativeID(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, tc := range []struct {
		name   string
		relays []SingleRelayConfig
		want   []string
	}{
		{"first entry", []SingleRelayConfig{{ID: -1, AutoOff: IntPtr(60)}}, []string{"relay.relays[0].id"}},
		{"second entry", []SingleRelayConfig{{ID: 0, AutoOff: IntPtr(60)}, {ID: -1, AutoOff: IntPtr(30)}}, []string{"relay.relays[1].id"}},
		{"both entries", []SingleRelayConfig{{ID: -2}, {ID: -1}}, []string{"relay.relays[0].id", "relay.relays[1].id"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &DeviceConfiguration{Relay: &RelayConfig{Relays: tc.relays}}

			unsupported, err := unsupportedApplySettings(converter, cfg, "SHSW-25", false)

			require.NoError(t, err)
			assert.Equal(t, tc.want, unsupported)
		})
	}
}

// Case 3: the device answers the read-back without a key apply wrote. That key counts as failed
// with the reported value <nil>, and the keys that did come back still count as applied.
func TestApplyConfig_RelayReadBackMissingKey(t *testing.T) {
	full := func() map[string]interface{} {
		return map[string]interface{}{"name": "Heater", "default_state": "last", "auto_on": float64(5), "auto_off": float64(3600)}
	}
	for _, key := range []string{"name", "default_state", "auto_on", "auto_off"} {
		t.Run("missing "+key, func(t *testing.T) {
			live := full()
			delete(live, key)
			client := newApplyTestClient()
			client.relayLive = map[int]map[string]interface{}{0: live}

			result, err := NewConfigApplier(NewGen1Converter(nil), nil).
				ApplyConfig(context.Background(), client, relayApplyConfig(), "SHPLG-S")

			require.NoError(t, err)
			assert.False(t, result.Success)
			assert.Equal(t, 3, result.AppliedCount)
			assert.Equal(t, 1, result.FailedCount)
			require.Len(t, result.Failures, 1)
			assert.Equal(t, "relays[0]."+key, result.Failures[0].Path)
			assert.Equal(t, "device reports <nil> after the write", result.Failures[0].Error)
		})
	}

	t.Run("empty reply", func(t *testing.T) {
		client := newApplyTestClient()
		client.relayLive = map[int]map[string]interface{}{0: {}}

		result, err := NewConfigApplier(NewGen1Converter(nil), nil).
			ApplyConfig(context.Background(), client, relayApplyConfig(), "SHPLG-S")

		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, 0, result.AppliedCount)
		assert.Equal(t, 4, result.FailedCount)
	})
}

// Case 4: a number and a string that print the same are not the same value, in either direction.
// A written 0 does not read back as "0", and a written "0" does not read back as 0. An int that
// the device returns as the same float64 still matches, and a different number does not.
func TestApplyConfig_RelayReadBackNumberAgainstString(t *testing.T) {
	for _, tc := range []struct {
		name        string
		relay       SingleRelayConfig
		live        map[string]interface{}
		wantApplied bool
		wantPath    string
	}{
		{"number written, string returned", SingleRelayConfig{AutoOn: IntPtr(0)}, map[string]interface{}{"auto_on": "0"}, false, "relays[0].auto_on"},
		{"string written, number returned", SingleRelayConfig{Name: StringPtr("0")}, map[string]interface{}{"name": float64(0)}, false, "relays[0].name"},
		{"number written, same number returned", SingleRelayConfig{AutoOn: IntPtr(5)}, map[string]interface{}{"auto_on": float64(5)}, true, ""},
		{"number written, other number returned", SingleRelayConfig{AutoOn: IntPtr(5)}, map[string]interface{}{"auto_on": 5.5}, false, "relays[0].auto_on"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := newApplyTestClient()
			client.relayLive = map[int]map[string]interface{}{0: tc.live}
			config := &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{tc.relay}}}

			result, err := NewConfigApplier(NewGen1Converter(nil), nil).
				ApplyConfig(context.Background(), client, config, "SHPLG-S")

			require.NoError(t, err)
			assert.Equal(t, tc.wantApplied, result.Success, "failures: %+v", result.Failures)
			if tc.wantApplied {
				assert.Equal(t, 1, result.AppliedCount)
				return
			}
			assert.Equal(t, 0, result.AppliedCount)
			require.Len(t, result.Failures, 1)
			assert.Equal(t, tc.wantPath, result.Failures[0].Path)
		})
	}
}

// Case 5: an empty relay name is a stated name, not a missing one. Apply writes it as an empty
// string, counts it as applied when the device returns an empty string, and counts it as failed
// when the device returns nothing or a non-string.
func TestApplyConfig_RelayEmptyName(t *testing.T) {
	config := func() *DeviceConfiguration {
		return &DeviceConfiguration{Relay: &RelayConfig{Relays: []SingleRelayConfig{
			{ID: 0, Name: StringPtr("")},
		}}}
	}

	t.Run("device returns an empty string", func(t *testing.T) {
		client := newApplyTestClient()

		result, err := NewConfigApplier(NewGen1Converter(nil), nil).
			ApplyConfig(context.Background(), client, config(), "SHPLG-S")

		require.NoError(t, err)
		assert.True(t, result.Success, "failures: %+v", result.Failures)
		assert.Equal(t, 1, result.AppliedCount)
		require.Len(t, client.relayWrites, 1)
		assert.Equal(t, map[string]interface{}{"name": ""}, client.relayWrites[0].form)
	})

	for name, live := range map[string]map[string]interface{}{
		"device returns null":       {"name": nil},
		"device returns no name":    {},
		"device returns the number": {"name": float64(0)},
	} {
		t.Run(name, func(t *testing.T) {
			client := newApplyTestClient()
			client.relayLive = map[int]map[string]interface{}{0: live}

			result, err := NewConfigApplier(NewGen1Converter(nil), nil).
				ApplyConfig(context.Background(), client, config(), "SHPLG-S")

			require.NoError(t, err)
			assert.False(t, result.Success)
			assert.Equal(t, 0, result.AppliedCount)
			require.Len(t, result.Failures, 1)
			assert.Equal(t, "relays[0].name", result.Failures[0].Path)
		})
	}
}

func TestApplyConfig_SetConfigError(t *testing.T) {
	client := newApplyTestClient()
	client.setErr = errors.New("device rejected settings")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Greater(t, result.FailedCount, 0)
	assert.NotEmpty(t, result.Failures)
}

func TestApplyConfig_RequiresReboot(t *testing.T) {
	client := newApplyTestClient()
	client.currentRaw = json.RawMessage(`{"mqtt":{"server":"old-broker:1883"}}`)

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("new-broker:1883"),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.RequiresReboot)
	assert.Contains(t, result.Warnings[0], "reboot")
}

func TestApplyConfig_AuthChangeRequiresReboot(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Auth: &AuthConfiguration{
			Enable: BoolPtr(true),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.RequiresReboot)
}

const testLoginPassword = "s3cret-pw-9"

func loginApplier() *ConfigApplier {
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	applier.writeLogin = true
	return applier
}

// The login is written with POST /settings/login, after every other section, so a failure of
// an earlier section never leaves a changed login behind.
func TestApplyConfig_LoginIsWrittenLastWithSetAuth(t *testing.T) {
	client := newApplyTestClient()

	result, err := loginApplier().ApplyConfig(context.Background(), client, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("tv")}},
		Auth: &AuthConfiguration{
			Enable: BoolPtr(true), Username: StringPtr("admin"), Password: StringPtr(testLoginPassword),
		},
	}, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, []loginWrite{{username: "admin", password: testLoginPassword}}, client.loginWrites)
	assert.Equal(t, []string{"config", "login"}, client.order)
	for _, cfg := range client.configs {
		assert.NotContains(t, cfg, "login", "login goes to /settings/login, never to /settings")
	}
	require.NotNil(t, result.login)
	assert.Equal(t, LoginChange{Enabled: true, Username: "admin", Password: testLoginPassword}, *result.login)
	assert.Equal(t, 4, result.AppliedCount)
}

func TestApplyConfig_LoginDisabledUsesResetAuth(t *testing.T) {
	client := newApplyTestClient()

	result, err := loginApplier().ApplyConfig(context.Background(), client,
		&DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(false)}}, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, []loginWrite{{reset: true}}, client.loginWrites)
	require.NotNil(t, result.login)
	assert.False(t, result.login.Enabled)
	assert.Equal(t, 1, result.AppliedCount)
}

// Login is opt-in on the applier: a caller that cannot store new credentials and rebuild its
// client must never change the login behind its own back.
func TestApplyConfig_LoginIsNotWrittenUnlessEnabled(t *testing.T) {
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, &DeviceConfiguration{
		Auth: &AuthConfiguration{Enable: BoolPtr(true), Username: StringPtr("a"), Password: StringPtr("b")},
	}, "SHPLG-S")

	require.NoError(t, err)
	assert.Empty(t, client.loginWrites)
	assert.Nil(t, result.login)
}

func TestApplyConfig_LoginFailureIsReportedByPathWithoutThePassword(t *testing.T) {
	client := newApplyTestClient()
	client.loginErr = fmt.Errorf("POST /settings/login password=%s: 500", testLoginPassword)

	result, err := loginApplier().ApplyConfig(context.Background(), client, &DeviceConfiguration{
		Auth: &AuthConfiguration{
			Enable: BoolPtr(true), Username: StringPtr("admin"), Password: StringPtr(testLoginPassword),
		},
	}, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "login", result.Failures[0].Path)
	assert.Equal(t, "enabled,password,username", result.Failures[0].Value)
	assert.NotContains(t, result.Failures[0].Error, testLoginPassword)
	assert.Equal(t, 3, result.FailedCount)
	assert.Nil(t, result.login, "a failed write records no login change")
}

func TestApplyConfig_LoginNeedsALoginClient(t *testing.T) {
	inner := newApplyTestClient()

	result, err := loginApplier().ApplyConfig(context.Background(), struct{ ShellyClient }{inner}, &DeviceConfiguration{
		Auth: &AuthConfiguration{Enable: BoolPtr(true), Username: StringPtr("a"), Password: StringPtr("b")},
	}, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "login", result.Failures[0].Path)
	assert.Nil(t, result.login)
}

func TestApplyConfig_MeasuresDuration(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{}
	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestApplyConfig_GetConfigError(t *testing.T) {
	client := newApplyTestClient()
	client.configErr = errors.New("cannot get config")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.RequiresReboot)
}

func TestApplyConfig_CloudSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Cloud: &CloudConfiguration{
			Enable: BoolPtr(false),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, []bool{false}, client.cloudWrites, "cloud is written with POST /settings/cloud")
	assert.Equal(t, []string{""}, client.cloudServers, "Gen1 has no cloud server parameter, so none is sent")
	assert.Equal(t, 1, result.AppliedCount)
	assert.Empty(t, client.configs, "a nested cloud map on POST /settings is ignored by Gen1 and must not be sent")
}

func TestApplyConfig_CloudSettingFailureIsReportedByPath(t *testing.T) {
	client := newApplyTestClient()
	client.cloudSetErr = errors.New("503 busy")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client,
		&DeviceConfiguration{Cloud: &CloudConfiguration{Enable: BoolPtr(true)}}, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "cloud.enabled", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, "503 busy")
	assert.Equal(t, 0, result.AppliedCount)
}

// A client without the cloud surface cannot write the setting. Apply says so as a failure
// and does not fall back to the nested map that Gen1 ignores.
func TestApplyConfig_CloudNeedsACloudClient(t *testing.T) {
	inner := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), struct{ ShellyClient }{inner},
		&DeviceConfiguration{Cloud: &CloudConfiguration{Enable: BoolPtr(false)}}, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "cloud.enabled", result.Failures[0].Path)
	assert.Empty(t, inner.configs)
}

func TestApplyConfig_CoIoTSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		CoIoT: &CoIoTConfiguration{
			Enable:       BoolPtr(true),
			UpdatePeriod: IntPtr(60),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	// Gen1 only accepts flat coiot_* parameters on POST /settings; a nested map is ignored.
	require.Len(t, client.configs, 1)
	assert.Equal(t, map[string]interface{}{
		"coiot_enable":        true,
		"coiot_update_period": float64(60),
	}, client.configs[0])
	assert.Equal(t, 2, result.AppliedCount)
}

// An empty peer is how a device reports multicast, and the device takes "mcast" to switch to it.
func TestApplyConfig_CoIoTPeerIsWrittenInTheShapeTheDeviceTakes(t *testing.T) {
	for peer, want := range map[string]string{
		"":                 "mcast",
		"192.168.1.5:5683": "192.168.1.5:5683",
	} {
		client := newApplyTestClient()
		applier := NewConfigApplier(NewGen1Converter(nil), nil)

		result, err := applier.ApplyConfig(context.Background(), client,
			&DeviceConfiguration{CoIoT: &CoIoTConfiguration{Peer: StringPtr(peer)}}, "SHPLG-S")

		require.NoError(t, err)
		assert.True(t, result.Success)
		require.Len(t, client.configs, 1, "peer %q", peer)
		assert.Equal(t, map[string]interface{}{"coiot_peer": want}, client.configs[0], "peer %q", peer)
	}
}

func TestApplyConfig_CoIoTFailureIsReportedByPath(t *testing.T) {
	client := newApplyTestClient()
	client.setErr = errors.New("503 busy")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client,
		&DeviceConfiguration{CoIoT: &CoIoTConfiguration{Enable: BoolPtr(true), UpdatePeriod: IntPtr(60)}}, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "coiot", result.Failures[0].Path)
	assert.Equal(t, "coiot_enable,coiot_update_period", result.Failures[0].Value)
	assert.Equal(t, 2, result.FailedCount)
	assert.Equal(t, 0, result.AppliedCount)
}

func TestApplyConfig_SystemSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{
				Name:    StringPtr("MyDevice"),
				EcoMode: BoolPtr(true),
			},
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestApplyConfig_LocationSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Location: &LocationConfiguration{
			Timezone:  StringPtr("Europe/Brussels"),
			Latitude:  Float64Ptr(50.85),
			Longitude: Float64Ptr(4.35),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestApplyConfig_LEDSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		LED: &LEDConfig{
			PowerIndication:   BoolPtr(false),
			NetworkIndication: BoolPtr(true),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestRebootAndWait_Success(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	err := applier.RebootAndWait(context.Background(), client, 5*time.Second)
	assert.NoError(t, err)
}

func TestRebootAndWait_RebootError(t *testing.T) {
	client := newApplyTestClient()
	client.rebootErr = errors.New("reboot failed")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	err := applier.RebootAndWait(context.Background(), client, 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reboot")
}

func TestRebootAndWait_Timeout(t *testing.T) {
	client := newApplyTestClient()
	client.testConnErr = errors.New("device offline")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	err := applier.RebootAndWait(context.Background(), client, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "did not come back")
}

func TestApplyConfig_MultipleSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("broker.local"),
		},
		Cloud: &CloudConfiguration{
			Enable: BoolPtr(false),
		},
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{
				Name: StringPtr("TestDevice"),
			},
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, result.AppliedCount, 1)
}

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

func TestApplyConfig_SNTPServerIsWrittenFlat(t *testing.T) {
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client,
		&DeviceConfiguration{System: &SystemConfiguration{SNTP: &SNTPConfig{Server: "ntp.example.net"}}}, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	// Gen1 takes the time server as the flat sntp_server parameter, never as a nested map.
	require.Len(t, client.configs, 1)
	assert.Equal(t, map[string]interface{}{"sntp_server": "ntp.example.net"}, client.configs[0])
	assert.Equal(t, 1, result.AppliedCount)
}

func TestApplyConfig_MQTTUpdatePeriodIsWrittenFlat(t *testing.T) {
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client,
		&DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(60)}}, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, client.configs, 1)
	assert.Equal(t, map[string]interface{}{"mqtt_update_period": float64(60)}, client.configs[0])
}

// The device stores an MQTT change but only runs it after a reboot, so a change to any MQTT key
// that apply writes must ask for one. For each key, a changed value must report it and, where the
// device reads the value back, an unchanged one must not (the control). The password is never read
// back, so it has no control row.
func TestApplyConfig_AnyMQTTKeyChangeRequiresReboot(t *testing.T) {
	cases := map[string]struct {
		desired *MQTTConfiguration
		changed string // the device's stored mqtt document before the apply
		same    string // the same document with the desired value already stored; empty: no control
	}{
		"enable": {&MQTTConfiguration{Enable: BoolPtr(true)},
			`{"enable":false}`, `{"enable":true}`},
		"server": {&MQTTConfiguration{Server: StringPtr("broker.example.net"), Port: IntPtr(1883)},
			`{"server":"old.example.net:1883"}`, `{"server":"broker.example.net:1883"}`},
		"user": {&MQTTConfiguration{User: StringPtr("bob")},
			`{"user":"alice"}`, `{"user":"bob"}`},
		"pass": {&MQTTConfiguration{Password: StringPtr("s3cret")},
			`{}`, ""},
		"id": {&MQTTConfiguration{ClientID: StringPtr("shelly-1")},
			`{"id":"other"}`, `{"id":"shelly-1"}`},
		"clean_session": {&MQTTConfiguration{CleanSession: BoolPtr(true)},
			`{"clean_session":false}`, `{"clean_session":true}`},
		"keep_alive": {&MQTTConfiguration{KeepAlive: IntPtr(60)},
			`{"keep_alive":30}`, `{"keep_alive":60}`},
		"update_period": {&MQTTConfiguration{UpdatePeriod: IntPtr(60)},
			`{"update_period":30}`, `{"update_period":60}`},
	}
	for key := range gen1MQTTKeys {
		_, ok := cases[key]
		require.True(t, ok, "mqtt key %q is applied but has no reboot case", key)
	}
	require.Len(t, cases, len(gen1MQTTKeys))

	run := func(t *testing.T, desired *MQTTConfiguration, stored string) bool {
		client := newApplyTestClient()
		client.currentRaw = json.RawMessage(`{"mqtt":` + stored + `}`)
		applier := NewConfigApplier(NewGen1Converter(nil), nil)

		result, err := applier.ApplyConfig(context.Background(), client,
			&DeviceConfiguration{MQTT: desired}, "SHPLG-S")

		require.NoError(t, err)
		return result.RequiresReboot
	}
	for key, tc := range cases {
		t.Run(key+" changed", func(t *testing.T) {
			assert.True(t, run(t, tc.desired, tc.changed))
		})
		if tc.same != "" {
			t.Run(key+" unchanged", func(t *testing.T) {
				assert.False(t, run(t, tc.desired, tc.same))
			})
		}
	}
}

// A password the device omits from its read-back is a change whatever its text is. A password of
// "<nil>" formats like the missing value, so a string comparison alone would call it unchanged.
func TestApplyConfig_AnyMQTTPasswordRequiresRebootEvenWhenItReadsLikeNil(t *testing.T) {
	for _, password := range []string{"s3cret", "<nil>", ""} {
		client := newApplyTestClient()
		client.currentRaw = json.RawMessage(`{"mqtt":{"user":"bob"}}`)
		applier := NewConfigApplier(NewGen1Converter(nil), nil)

		result, err := applier.ApplyConfig(context.Background(), client,
			&DeviceConfiguration{MQTT: &MQTTConfiguration{Password: StringPtr(password)}}, "SHPLG-S")

		require.NoError(t, err)
		assert.True(t, result.RequiresReboot, "password %q", password)
	}
}

func TestApplyConfig_SNTPFailureIsReportedByPath(t *testing.T) {
	client := newApplyTestClient()
	client.setErr = errors.New("503 busy")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client,
		&DeviceConfiguration{System: &SystemConfiguration{SNTP: &SNTPConfig{Server: "ntp.example.net"}}}, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "sntp", result.Failures[0].Path)
	assert.Equal(t, 1, result.FailedCount)
	assert.Equal(t, 0, result.AppliedCount)
}

func TestApplyDesiredConfig_AppliesSNTPAndMQTTPeriodAndVerifiesThem(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{Name: StringPtr("tv-office")},
			SNTP:   &SNTPConfig{Server: "ntp.example.net"},
		},
		MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(60)},
	})
	dev := newFakeGen1Device()
	dev.settings["sntp"] = map[string]interface{}{"server": "time.google.com"}
	dev.settings["mqtt"] = map[string]interface{}{"update_period": float64(30)}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "differences: %+v", result.VerifyResult)
	assert.Equal(t, map[string]interface{}{"server": "ntp.example.net"}, dev.settings["sntp"])
	assert.Equal(t, map[string]interface{}{"update_period": float64(60)}, dev.settings["mqtt"])
	assert.Equal(t, 0, result.ApplyResult.FailedCount)
	assert.True(t, repo.devices[1].ConfigApplied)
}

// A device can answer 200 and keep the old values. The read-back must catch it.
func TestApplyDesiredConfig_SNTPAndMQTTPeriodStayPendingWhenTheDeviceKeepsTheOldValues(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{SNTP: &SNTPConfig{Server: "ntp.example.net"}},
		MQTT:   &MQTTConfiguration{UpdatePeriod: IntPtr(60)},
	})
	dev := newFakeGen1Device()
	dev.ignoreWrites = true
	dev.settings["sntp"] = map[string]interface{}{"server": "time.google.com"}
	dev.settings["mqtt"] = map[string]interface{}{"update_period": float64(30)}

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotNil(t, result.VerifyResult)
	assert.False(t, result.VerifyResult.Match)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// Verify reads the time server back from the device, so a drifted server is reported.
func TestVerifyDesiredConfig_ReportsADriftedTimeServer(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	setDesired(t, repo, &DeviceConfiguration{
		System: &SystemConfiguration{SNTP: &SNTPConfig{Server: "ntp.example.net"}},
	})
	dev := newFakeGen1Device()
	dev.settings["sntp"] = map[string]interface{}{"server": "time.google.com"}

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Match)
	require.Len(t, result.Differences, 1)
	assert.Equal(t, "system.sntp.server", result.Differences[0].Path)
}

// A time server the device cannot resolve stops its clock and every schedule that depends on it,
// so it is refused before anything is written. A period below zero means nothing.
func TestApplyDesiredConfig_RefusesATimeServerOrPeriodTheDeviceCannotUse(t *testing.T) {
	cases := map[string]struct {
		desired *DeviceConfiguration
		path    string
	}{
		"server with a space":    {sntpDesired("ntp example.net"), "system.sntp.server"},
		"server with a scheme":   {sntpDesired("http://ntp.example.net"), "system.sntp.server"},
		"IPv6 literal":           {sntpDesired("2001:db8::1"), "system.sntp.server"},
		"server with a port":     {sntpDesired("ntp.example.net:123"), "system.sntp.server"},
		"empty label":            {sntpDesired("ntp..example.net"), "system.sntp.server"},
		"label starts with dash": {sntpDesired("-ntp.example.net"), "system.sntp.server"},
		"label ends with dash":   {sntpDesired("ntp-.example.net"), "system.sntp.server"},
		"label over 63 bytes":    {sntpDesired(strings.Repeat("a", 64) + ".example.net"), "system.sntp.server"},
		"name over 253 bytes":    {sntpDesired(strings.Repeat("a.", 127) + "a"), "system.sntp.server"},
		"non ASCII letter":       {sntpDesired("ntp.exämple.net"), "system.sntp.server"},
		"negative update period": {&DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(-1)}}, "mqtt.update_period"},
		"far negative period":    {&DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(-3600)}}, "mqtt.update_period"},
		"254 bytes, no dot":      {sntpDesired(longTimeServer(62)), "system.sntp.server"},
		"254 bytes before a dot": {sntpDesired(longTimeServer(62) + "."), "system.sntp.server"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "tv-office")
			tc.desired.System = mergeName(tc.desired.System)
			setDesired(t, repo, tc.desired)
			dev := newFakeGen1Device()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Equal(t, 0, dev.setCalls, "the supported name must not be written either")
		})
	}
}

func TestApplyDesiredConfig_AcceptsTimeServersAndPeriodsTheDeviceCanUse(t *testing.T) {
	servers := []string{"time.google.com", "ntp.example.net.", "pool", "192.168.1.1", strings.Repeat("a", 63) + ".example.net",
		// The longest valid name is 253 bytes, and the optional terminal dot is not counted.
		longTimeServer(61), longTimeServer(61) + "."}
	for _, server := range servers {
		svc, repo := newServiceWithDesiredName(t, "tv-office")
		setDesired(t, repo, sntpDesired(server))

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")

		require.NoError(t, err, "server %q", server)
		assert.True(t, result.ConfigApplied, "server %q", server)
	}
	for _, period := range []int{0, 1, 3600} {
		svc, repo := newServiceWithDesiredName(t, "tv-office")
		setDesired(t, repo, &DeviceConfiguration{MQTT: &MQTTConfiguration{UpdatePeriod: IntPtr(period)}})

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")

		require.NoError(t, err, "period %d", period)
		assert.True(t, result.ConfigApplied, "period %d", period)
	}
}

// longTimeServer returns three 63-byte labels and one of last bytes: 192+last+3 bytes in all, so
// 61 gives the 253-byte maximum.
func longTimeServer(last int) string {
	label := strings.Repeat("a", 63)
	return label + "." + label + "." + label + "." + strings.Repeat("b", last)
}

func sntpDesired(server string) *DeviceConfiguration {
	return &DeviceConfiguration{System: &SystemConfiguration{SNTP: &SNTPConfig{Server: server}}}
}

// mergeName keeps a supported setting in every refused configuration, so each case proves that
// nothing is written, not even the supported part.
func mergeName(system *SystemConfiguration) *SystemConfiguration {
	if system == nil {
		system = &SystemConfiguration{}
	}
	system.Device = &TypedDeviceConfig{Name: StringPtr("tv-office")}
	return system
}

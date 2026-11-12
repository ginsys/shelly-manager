package configuration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
)

// The applier reaches the roller endpoints through RollerSettingsClient, so the real Gen1 client
// must satisfy it, or apply fails every roller on a real device.
var _ RollerSettingsClient = (*gen1.Client)(nil)

// rollerApplyClient adds the per-channel roller surface to the shared apply test client.
// rollerLive is what GET /settings/roller/<n> returns; when nil the device echoes the last form
// written to that channel.
type rollerApplyClient struct {
	*applyTestClient
	rollerWrites []relayWrite
	rollerLive   map[int]map[string]interface{}
	rollerSetErr error
	rollerGetErr error
}

func (m *rollerApplyClient) SetRollerSettingsForm(ctx context.Context, channel int, settings map[string]interface{}) error {
	if m.rollerSetErr != nil {
		return m.rollerSetErr
	}
	m.rollerWrites = append(m.rollerWrites, relayWrite{channel: channel, form: settings})
	return nil
}

func (m *rollerApplyClient) GetRollerSettingsForm(ctx context.Context, channel int) (map[string]interface{}, error) {
	if m.rollerGetErr != nil {
		return nil, m.rollerGetErr
	}
	if live, ok := m.rollerLive[channel]; ok {
		return live, nil
	}
	echo := map[string]interface{}{}
	for _, w := range m.rollerWrites {
		if w.channel != channel {
			continue
		}
		for k, v := range w.form {
			// A device returns JSON numbers as float64.
			if n, ok := v.(int); ok {
				v = float64(n)
			}
			if k == "btn_type" {
				// The device reads the button type back under another name than it is written under.
				k = "button_type"
			}
			echo[k] = v
		}
	}
	return echo, nil
}

func newRollerApplyClient() *rollerApplyClient {
	return &rollerApplyClient{applyTestClient: newApplyTestClient()}
}

func rollerApplyConfig() *DeviceConfiguration {
	return &DeviceConfiguration{Roller: &RollerConfig{
		MaxOpenTime:    IntPtr(20),
		MaxCloseTime:   IntPtr(22),
		MotorDirection: StringPtr("reversed"),
		SwapInputs:     BoolPtr(true),
		InputMode:      StringPtr("dual"),
	}}
}

func TestApplyConfig_RollerRequestShape(t *testing.T) {
	client := newRollerApplyClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, rollerApplyConfig(), "SHSW-25")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.rollerWrites, 1)
	assert.Equal(t, 0, client.rollerWrites[0].channel)
	assert.Equal(t, map[string]interface{}{
		"maxtime_open":  float64(20),
		"maxtime_close": float64(22),
		"swap":          true,
		"swap_inputs":   true,
		"input_mode":    "openclose",
	}, client.rollerWrites[0].form)
	// The nested document goes nowhere: Gen1 ignores it on POST /settings.
	for _, cfg := range client.configs {
		assert.NotContains(t, cfg, "rollers")
	}
	assert.Equal(t, 5, result.AppliedCount)
	assert.Equal(t, 5, result.SettingsCount)
}

func TestApplyConfig_RollerWritesOnlyTheKeysSet(t *testing.T) {
	client := newRollerApplyClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	config := &DeviceConfiguration{Roller: &RollerConfig{MaxCloseTime: IntPtr(30)}}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHSW-25")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.rollerWrites, 1)
	assert.Equal(t, map[string]interface{}{"maxtime_close": float64(30)}, client.rollerWrites[0].form)
}

func TestApplyConfig_RollerReadBackMismatchFails(t *testing.T) {
	client := newRollerApplyClient()
	// The device answers 200 to the write and keeps the old closing time.
	client.rollerLive = map[int]map[string]interface{}{
		0: {"maxtime_open": float64(20), "maxtime_close": float64(60), "swap": true, "swap_inputs": true, "input_mode": "openclose"},
	}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, rollerApplyConfig(), "SHSW-25")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 1, result.FailedCount)
	assert.Equal(t, 4, result.AppliedCount)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "rollers[0].maxtime_close", result.Failures[0].Path)
	assert.Equal(t, "22", result.Failures[0].Value)
	assert.Contains(t, result.Failures[0].Error, "60")
}

func TestApplyConfig_RollerWriteErrorFailsEveryKey(t *testing.T) {
	client := newRollerApplyClient()
	client.rollerSetErr = errors.New("device is in relay mode")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, rollerApplyConfig(), "SHSW-25")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 5, result.FailedCount)
	assert.Equal(t, 0, result.AppliedCount)
	assert.Empty(t, client.rollerWrites)
	require.NotEmpty(t, result.Failures)
	assert.Contains(t, result.Failures[0].Error, "device is in relay mode")
}

func TestApplyConfig_RollerReadBackErrorFailsEveryKey(t *testing.T) {
	client := newRollerApplyClient()
	client.rollerGetErr = errors.New("timeout")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, rollerApplyConfig(), "SHSW-25")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 5, result.FailedCount)
	require.NotEmpty(t, result.Failures)
	assert.Contains(t, result.Failures[0].Error, "read-back failed")
}

// A client without the roller surface cannot write a roller, and apply says so instead of
// sending the nested document, which Gen1 ignores.
func TestApplyConfig_RollerClientWithoutRollerSurfaceFails(t *testing.T) {
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, rollerApplyConfig(), "SHSW-25")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "rollers", result.Failures[0].Path)
	assert.Equal(t, 1, result.FailedCount)
}

// Apply refuses what the device would not keep, before the device is touched.
func TestUnsupportedApplySettings_Roller(t *testing.T) {
	converter := NewGen1Converter(nil)

	t.Run("a writable roller is accepted", func(t *testing.T) {
		unsupported, err := unsupportedApplySettings(converter, rollerApplyConfig(), "SHSW-25", true)
		require.NoError(t, err)
		assert.Empty(t, unsupported)
	})

	t.Run("fields with no Gen1 key are refused", func(t *testing.T) {
		config := &DeviceConfiguration{Roller: &RollerConfig{
			MaxOpenTime: IntPtr(20), MotorSpeed: IntPtr(50), SafetySwitch: BoolPtr(true),
		}}
		unsupported, err := unsupportedApplySettings(converter, config, "SHSW-25", true)
		require.NoError(t, err)
		assert.Equal(t, []string{"roller.motor_speed", "roller.safety_switch"}, unsupported)
	})

	t.Run("a section none of whose fields has a Gen1 key is refused whole", func(t *testing.T) {
		config := &DeviceConfiguration{Roller: &RollerConfig{MotorSpeed: IntPtr(50)}}
		unsupported, err := unsupportedApplySettings(converter, config, "SHSW-25", true)
		require.NoError(t, err)
		assert.Equal(t, []string{"roller"}, unsupported)
	})

	t.Run("a travel time below one second is refused", func(t *testing.T) {
		for _, seconds := range []int{0, -1} {
			config := &DeviceConfiguration{Roller: &RollerConfig{MaxOpenTime: IntPtr(seconds), MaxCloseTime: IntPtr(seconds)}}
			unsupported, err := unsupportedApplySettings(converter, config, "SHSW-25", true)
			require.NoError(t, err)
			assert.Equal(t, []string{"roller.max_close_time", "roller.max_open_time"}, unsupported, "%d seconds", seconds)
		}

		one := &DeviceConfiguration{Roller: &RollerConfig{MaxOpenTime: IntPtr(1), MaxCloseTime: IntPtr(1)}}
		unsupported, err := unsupportedApplySettings(converter, one, "SHSW-25", true)
		require.NoError(t, err)
		assert.Empty(t, unsupported, "one second is the shortest the device keeps")
	})

	t.Run("an unknown spelling is refused", func(t *testing.T) {
		config := &DeviceConfiguration{Roller: &RollerConfig{
			MaxOpenTime: IntPtr(20), MotorDirection: StringPtr("sideways"), InputMode: StringPtr("triple"),
		}}
		unsupported, err := unsupportedApplySettings(converter, config, "SHSW-25", true)
		require.NoError(t, err)
		assert.Equal(t, []string{"roller.input_mode", "roller.motor_direction"}, unsupported)
	})

	t.Run("a model without roller mode refuses the section", func(t *testing.T) {
		config := &DeviceConfiguration{Roller: &RollerConfig{MaxOpenTime: IntPtr(20)}}
		unsupported, err := unsupportedApplySettings(converter, config, "SHSW-1", true)
		require.NoError(t, err)
		assert.Equal(t, []string{"roller"}, unsupported)
	})
}

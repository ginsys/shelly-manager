package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// fakeGen2DeviceWithCover is a device in cover profile. cover:0 holds the keys the Shelly Cover
// page lists, with its defaults. The page says swap_inputs exists with two inputs only and
// safety_switch with two inputs, so a test removes a key to model a device that lacks it.
func fakeGen2DeviceWithCover() *fakeGen2Device {
	d := newFakeGen2Device()
	d.doc["cover:0"] = map[string]interface{}{
		"id": 0.0, "name": nil, "in_mode": "dual", "initial_state": "stopped", "in_locked": false,
		"swap_inputs": false, "invert_directions": false, "maxtime_open": 60.0, "maxtime_close": 60.0,
		"motor": map[string]interface{}{"idle_power_thr": 2.0},
		"obstruction_detection": map[string]interface{}{
			"enable": false, "direction": "both", "action": "stop", "power_thr": 1000.0, "holdoff": 1.0,
		},
		"safety_switch": map[string]interface{}{
			"enable": false, "direction": "both", "action": "stop", "allowed_move": nil,
		},
	}
	return d
}

// coverReads counts the reads of the configuration document, to show a refusal made before the
// device was contacted.
type coverReads struct {
	*fakeGen2Device
	reads int
}

func (c *coverReads) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	c.reads++
	return c.fakeGen2Device.GetConfig(ctx)
}

func coverDesired() *DeviceConfiguration {
	return &DeviceConfiguration{Roller: &RollerConfig{
		MaxOpenTime: IntPtr(30), MaxCloseTime: IntPtr(25), SwapInputs: BoolPtr(true), InputMode: StringPtr("single"),
		ObstacleMode: StringPtr("while_closing"), ObstacleAction: StringPtr("reverse"), ObstaclePower: IntPtr(800), ObstacleDelay: IntPtr(2),
		SafetyMode: StringPtr("while_opening"), SafetyAction: StringPtr("stop"),
	}}
}

func coverBody(t *testing.T, config *DeviceConfiguration) map[string]interface{} {
	t.Helper()
	raw, err := NewGen2Converter(nil).ToAPIConfig(config, "SNSW-102P16EU")
	require.NoError(t, err)
	var doc map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc["cover:0"]
}

func TestGen2Converter_WritesTheCoverShape(t *testing.T) {
	raw, err := NewGen2Converter(nil).ToAPIConfig(coverDesired(), "SNSW-102P16EU")

	require.NoError(t, err)
	assert.JSONEq(t, `{"cover:0":{
		"maxtime_open":30,"maxtime_close":25,"swap_inputs":true,"in_mode":"single",
		"obstruction_detection":{"enable":true,"direction":"close","action":"reverse","power_thr":800,"holdoff":2},
		"safety_switch":{"enable":true,"direction":"open","action":"stop"}}}`, string(raw))
}

// The nested objects carry the sub-keys that are set and nothing else, as the Sys section does.
func TestGen2Converter_CoverNestedObjectsCarryOnlyTheKeysSet(t *testing.T) {
	cases := map[string]struct {
		roller *RollerConfig
		want   map[string]interface{}
	}{
		"power alone":  {&RollerConfig{ObstaclePower: IntPtr(800)}, map[string]interface{}{"obstruction_detection": map[string]interface{}{"power_thr": 800.0}}},
		"action alone": {&RollerConfig{SafetyAction: StringPtr("pause")}, map[string]interface{}{"safety_switch": map[string]interface{}{"action": "pause"}}},
		"disabled":     {&RollerConfig{ObstacleMode: StringPtr("disabled")}, map[string]interface{}{"obstruction_detection": map[string]interface{}{"enable": false}}},
		"moving":       {&RollerConfig{SafetyMode: StringPtr("while_moving")}, map[string]interface{}{"safety_switch": map[string]interface{}{"enable": true, "direction": "both"}}},
		"direction":    {&RollerConfig{MotorDirection: StringPtr("reversed")}, map[string]interface{}{"invert_directions": true}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, coverBody(t, &DeviceConfiguration{Roller: tc.roller}))
		})
	}
}

func TestGen2Converter_ReadsTheCover(t *testing.T) {
	device := fakeGen2DeviceWithCover()
	cover := device.doc["cover:0"].(map[string]interface{})
	cover["invert_directions"] = true
	cover["obstruction_detection"] = map[string]interface{}{"enable": true, "direction": "open", "action": "reverse", "power_thr": 900.0, "holdoff": 3.0}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-102P16EU")

	require.NoError(t, err)
	require.NotNil(t, got.Roller)
	assert.Equal(t, 60, *got.Roller.MaxOpenTime)
	assert.Equal(t, 60, *got.Roller.MaxCloseTime)
	assert.Equal(t, "reversed", *got.Roller.MotorDirection)
	assert.Equal(t, "dual", *got.Roller.InputMode)
	assert.False(t, *got.Roller.SwapInputs)
	assert.Equal(t, "while_opening", *got.Roller.ObstacleMode)
	assert.Equal(t, "reverse", *got.Roller.ObstacleAction)
	assert.Equal(t, 900, *got.Roller.ObstaclePower)
	assert.Equal(t, 3, *got.Roller.ObstacleDelay)
	assert.Equal(t, "disabled", *got.Roller.SafetyMode, "enable false is disabled")
	assert.Equal(t, "stop", *got.Roller.SafetyAction)
}

// What the device reports that has no stored word, or no whole-second value, is left unread, so a
// desired value shows as a difference and is never matched by a guess.
func TestGen2Converter_CoverReadBackNeverInventsAValue(t *testing.T) {
	device := fakeGen2DeviceWithCover()
	cover := device.doc["cover:0"].(map[string]interface{})
	cover["in_mode"] = "detached"
	cover["maxtime_open"] = 0.5
	cover["maxtime_close"] = 45.5
	cover["obstruction_detection"] = map[string]interface{}{"enable": true, "direction": "sideways", "holdoff": 0.5}
	cover["safety_switch"] = map[string]interface{}{"enable": false, "direction": "open"}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-102P16EU")

	require.NoError(t, err)
	require.NotNil(t, got.Roller)
	assert.Nil(t, got.Roller.InputMode, "detached is a page value with no stored word")
	assert.Nil(t, got.Roller.MaxOpenTime, "a fraction is never truncated")
	assert.Nil(t, got.Roller.MaxCloseTime)
	assert.Nil(t, got.Roller.ObstacleMode, "an unknown direction names no mode")
	assert.Nil(t, got.Roller.ObstacleDelay)
	assert.Equal(t, "disabled", *got.Roller.SafetyMode, "enable false is disabled whatever direction the device reports")
}

// Only cover:0 is the stored roller: a second cover stays an unread key.
func TestGen2Converter_OnlyCoverZeroIsRead(t *testing.T) {
	device := fakeGen2DeviceWithCover()
	device.doc["cover:1"] = map[string]interface{}{"id": 1.0, "maxtime_open": 10.0}
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	keys, err := NewGen2Converter(nil).UnreadKeys(raw)

	require.NoError(t, err)
	assert.Equal(t, []string{"cover:1"}, keys)
}

func TestApplyDesiredConfig_Gen2WritesCover(t *testing.T) {
	svc, repo := newGen2Service(t, coverDesired())
	dev := fakeGen2DeviceWithCover()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "the cover is read back as the stored roller")
	assert.True(t, repo.devices[1].ConfigApplied)
	assert.Equal(t, 0, dev.legacyCalls)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "Cover.SetConfig", dev.calls[0].method)
	assert.Equal(t, 0, dev.calls[0].id)
	assert.Equal(t, coverBody(t, coverDesired()), dev.calls[0].config)
	assert.Equal(t, 0, result.ApplyResult.FailedCount)
	assert.False(t, result.ApplyResult.RequiresReboot)
}

// The Gen2 apply sends no profile change: a desired roller on a device in switch profile is
// refused before any write.
func TestApplyDesiredConfig_Gen2RefusesARollerOnASwitchProfileDevice(t *testing.T) {
	svc, repo := newGen2Service(t, coverDesired())
	dev := fakeGen2DeviceWithChannels()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "roller")
	assert.Contains(t, err.Error(), "cover:0")
	assert.Empty(t, dev.calls, "no cover write and no profile change")
	assert.Equal(t, 0, dev.legacyCalls)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// A value below the page's floor, or above its ceiling, is refused before the device is touched.
func TestApplyDesiredConfig_Gen2RefusesCoverValuesOutOfRange(t *testing.T) {
	cases := map[string]struct {
		roller *RollerConfig
		path   string
	}{
		"open time zero":     {&RollerConfig{MaxOpenTime: IntPtr(0)}, "roller.max_open_time"},
		"open time negative": {&RollerConfig{MaxOpenTime: IntPtr(-5)}, "roller.max_open_time"},
		"close time zero":    {&RollerConfig{MaxCloseTime: IntPtr(0)}, "roller.max_close_time"},
		"open time too long": {&RollerConfig{MaxOpenTime: IntPtr(301)}, "roller.max_open_time"},
		"holdoff zero":       {&RollerConfig{ObstacleDelay: IntPtr(0)}, "roller.obstacle_delay"},
		"holdoff too long":   {&RollerConfig{ObstacleDelay: IntPtr(301)}, "roller.obstacle_delay"},
		"negative power":     {&RollerConfig{ObstaclePower: IntPtr(-1)}, "roller.obstacle_power"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, &DeviceConfiguration{Roller: tc.roller})
			dev := &coverReads{fakeGen2Device: fakeGen2DeviceWithCover()}

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls)
			assert.Equal(t, 0, dev.reads, "refused before the device was read")
		})
	}
}

func TestApplyDesiredConfig_Gen2AcceptsTheCoverBounds(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{Roller: &RollerConfig{
		MaxOpenTime: IntPtr(1), MaxCloseTime: IntPtr(300), ObstacleDelay: IntPtr(300),
	}})
	dev := fakeGen2DeviceWithCover()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
}

// The page says changing invert_directions needs a reboot, whether or not the answer carries
// restart_required. An unchanged value is not a change.
func TestApplyDesiredConfig_Gen2OwesARestartForADirectionChange(t *testing.T) {
	cases := map[string]struct {
		live    bool
		answer  bool
		desired string
		owed    bool
	}{
		"changed, answer says no":    {false, false, "reversed", true},
		"changed, answer says yes":   {false, true, "reversed", true},
		"unchanged, answer says no":  {true, false, "reversed", false},
		"other key, answer says yes": {true, true, "reversed", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newGen2Service(t, &DeviceConfiguration{Roller: &RollerConfig{
				MotorDirection: StringPtr(tc.desired), MaxOpenTime: IntPtr(30),
			}})
			dev := fakeGen2DeviceWithCover()
			dev.doc["cover:0"].(map[string]interface{})["invert_directions"] = tc.live
			dev.restart = tc.answer

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

			require.NoError(t, err)
			assert.Equal(t, tc.owed, result.ApplyResult.RequiresReboot)
			if tc.live != (tc.desired == "reversed") && !tc.answer {
				assert.Contains(t, result.ApplyResult.Warnings, "Device reboot required for some settings to take effect (a changed motor direction needs a restart)")
			}
			if tc.owed {
				assert.Equal(t, 1, dev.reboots)
			} else {
				assert.Equal(t, 0, dev.reboots)
				assert.False(t, repo.devices[1].PendingReboot)
			}
		})
	}
}

// A setting the live cover does not report is refused: the page lists swap_inputs, in_mode and
// safety_switch as present only on a device that has the inputs for them.
func TestApplyDesiredConfig_Gen2RefusesCoverKeysTheDeviceDoesNotReport(t *testing.T) {
	cases := map[string]struct {
		roller *RollerConfig
		drop   string
		path   string
	}{
		"swap_inputs": {&RollerConfig{SwapInputs: BoolPtr(true)}, "swap_inputs", "roller.swap_inputs"},
		"in_mode":     {&RollerConfig{InputMode: StringPtr("single")}, "in_mode", "roller.input_mode"},
		"safety mode": {&RollerConfig{SafetyMode: StringPtr("while_opening")}, "safety_switch", "roller.safety_mode"},
		"safety act":  {&RollerConfig{SafetyAction: StringPtr("stop")}, "safety_switch", "roller.safety_action"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, &DeviceConfiguration{Roller: tc.roller})
			dev := fakeGen2DeviceWithCover()
			delete(dev.doc["cover:0"].(map[string]interface{}), tc.drop)

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls)
		})
	}
}

// The page says a safety_switch action of reverse needs allowed_move to be reverse. The stored
// roller has no word for allowed_move, so a reverse is written only where the device already
// reports it.
func TestApplyDesiredConfig_Gen2SafetyReverseNeedsAllowedMove(t *testing.T) {
	desired := &DeviceConfiguration{Roller: &RollerConfig{SafetyAction: StringPtr("reverse")}}

	t.Run("not reverse: refused", func(t *testing.T) {
		svc, _ := newGen2Service(t, desired)
		dev := fakeGen2DeviceWithCover()

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		assert.Contains(t, err.Error(), "roller.safety_action")
		assert.Empty(t, dev.calls)
	})
	t.Run("reverse: written", func(t *testing.T) {
		svc, _ := newGen2Service(t, desired)
		dev := fakeGen2DeviceWithCover()
		dev.doc["cover:0"].(map[string]interface{})["safety_switch"].(map[string]interface{})["allowed_move"] = "reverse"

		result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

		require.NoError(t, err)
		assert.True(t, result.ConfigApplied)
		require.Len(t, dev.calls, 1)
		assert.Equal(t, map[string]interface{}{"safety_switch": map[string]interface{}{"action": "reverse"}}, dev.calls[0].config)
	})
}

// A roller setting with no Cover key is named, and nothing is written.
func TestApplyDesiredConfig_Gen2RefusesRollerSettingsWithNoCoverKey(t *testing.T) {
	cases := map[string]struct {
		roller *RollerConfig
		path   string
	}{
		"motor speed":           {&RollerConfig{MotorSpeed: IntPtr(50)}, "roller.motor_speed"},
		"default state":         {&RollerConfig{DefaultState: StringPtr("stop")}, "roller.default_state"},
		"obstacle detection":    {&RollerConfig{ObstacleDetection: BoolPtr(true)}, "roller.obstacle_detection"},
		"safety switch":         {&RollerConfig{SafetySwitch: BoolPtr(true)}, "roller.safety_switch"},
		"safety allowed":        {&RollerConfig{SafetyAllowedOnTrigger: StringPtr("open")}, "roller.safety_allowed_on_trigger"},
		"mixed with a good key": {&RollerConfig{MaxOpenTime: IntPtr(30), OffPower: IntPtr(5)}, "roller.off_power"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newGen2Service(t, &DeviceConfiguration{Roller: tc.roller})
			dev := fakeGen2DeviceWithCover()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls)
		})
	}
}

// A word outside the stored sets has no Cover word. The converter leaves it out, so the check must
// name it before the device is touched: an apply that dropped it would report success and write
// nothing for that field.
func TestApplyDesiredConfig_Gen2RefusesRollerWordsWithNoCoverWord(t *testing.T) {
	words := map[string]struct {
		set  func(*RollerConfig, *string)
		path string
	}{
		"motor_direction": {func(r *RollerConfig, w *string) { r.MotorDirection = w }, "roller.motor_direction"},
		"input_mode":      {func(r *RollerConfig, w *string) { r.InputMode = w }, "roller.input_mode"},
		"obstacle_mode":   {func(r *RollerConfig, w *string) { r.ObstacleMode = w }, "roller.obstacle_mode"},
		"obstacle_action": {func(r *RollerConfig, w *string) { r.ObstacleAction = w }, "roller.obstacle_action"},
		"safety_mode":     {func(r *RollerConfig, w *string) { r.SafetyMode = w }, "roller.safety_mode"},
		"safety_action":   {func(r *RollerConfig, w *string) { r.SafetyAction = w }, "roller.safety_action"},
	}
	for name, field := range words {
		for _, beside := range []bool{false, true} {
			t.Run(name+fmt.Sprintf(" beside a good key %v", beside), func(t *testing.T) {
				roller := &RollerConfig{}
				if beside {
					roller.MaxOpenTime = IntPtr(30)
				}
				field.set(roller, StringPtr("pause-and-sing"))
				svc, _ := newGen2Service(t, &DeviceConfiguration{Roller: roller})
				dev := &coverReads{fakeGen2Device: fakeGen2DeviceWithCover()}

				_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-102P16EU")

				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrUnsupportedSettings))
				assert.Contains(t, err.Error(), field.path)
				assert.Empty(t, dev.calls)
				assert.Equal(t, 0, dev.reads, "refused before the device was read")
			})
		}
	}
}

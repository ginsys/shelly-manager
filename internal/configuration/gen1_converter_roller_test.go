package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Gen1 roller settings, as the device lists them in the rollers array of GET /settings: the
// five keys the converter maps, and others it leaves alone.
const gen1RollerSettingsJSON = `{"rollers":[{
	"maxtime":60,"maxtime_open":20,"maxtime_close":22,
	"default_state":"stop","swap":true,"swap_inputs":false,"input_mode":"openclose",
	"button_type":"momentary","obstacle_mode":"disabled"}]}`

func TestGen1Converter_RollerFromAPI(t *testing.T) {
	config, err := NewGen1Converter(nil).FromAPIConfig(json.RawMessage(gen1RollerSettingsJSON), "SHSW-25")

	require.NoError(t, err)
	require.NotNil(t, config.Roller)
	assert.Equal(t, &RollerConfig{
		MaxOpenTime:    IntPtr(20),
		MaxCloseTime:   IntPtr(22),
		MotorDirection: StringPtr("reversed"),
		SwapInputs:     BoolPtr(false),
		InputMode:      StringPtr("dual"),
	}, config.Roller)
}

func TestGen1Converter_RollerToAPI(t *testing.T) {
	config := &DeviceConfiguration{Roller: &RollerConfig{
		MaxOpenTime:    IntPtr(20),
		MaxCloseTime:   IntPtr(22),
		MotorDirection: StringPtr("normal"),
		SwapInputs:     BoolPtr(true),
		InputMode:      StringPtr("single"),
	}}

	raw, err := NewGen1Converter(nil).ToAPIConfig(config, "SHSW-25")

	require.NoError(t, err)
	var gen1 map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &gen1))
	assert.Equal(t, []interface{}{map[string]interface{}{
		"maxtime_open":  float64(20),
		"maxtime_close": float64(22),
		"swap":          false,
		"swap_inputs":   true,
		"input_mode":    "onebutton",
	}}, gen1["rollers"])
}

// A field with no one-to-one Gen1 key never reaches the device document, so the refusal check
// reports it instead of apply dropping it silently.
func TestGen1Converter_RollerDropsFieldsWithoutAGen1Key(t *testing.T) {
	config := &DeviceConfiguration{Roller: &RollerConfig{
		MotorSpeed:         IntPtr(50),
		CalibrationState:   StringPtr("calibrated"),
		DefaultPosition:    IntPtr(30),
		PositioningEnabled: BoolPtr(true),
		ObstacleDetection:  BoolPtr(true),
		ObstaclePower:      IntPtr(100),
		SafetySwitch:       BoolPtr(true),
		TiltEnabled:        BoolPtr(true),
	}}

	raw, err := NewGen1Converter(nil).ToAPIConfig(config, "SHSW-25")

	require.NoError(t, err)
	var gen1 map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &gen1))
	assert.NotContains(t, gen1, "rollers")
}

// A spelling that has no Gen1 counterpart is dropped, and a Gen1 value this form cannot name is
// not invented on import.
func TestGen1Converter_RollerUnknownSpellings(t *testing.T) {
	converter := NewGen1Converter(nil)

	raw, err := converter.ToAPIConfig(&DeviceConfiguration{Roller: &RollerConfig{
		MotorDirection: StringPtr("sideways"),
		InputMode:      StringPtr("triple"),
	}}, "SHSW-25")
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "rollers")

	config, err := converter.FromAPIConfig(json.RawMessage(`{"rollers":[{"input_mode":"unheard-of","swap":"yes"}]}`), "SHSW-25")
	require.NoError(t, err)
	assert.Nil(t, config.Roller)
}

// Only a model with roller mode has a roller section.
func TestGen1Converter_RollerOnlyOnAModelWithRollerMode(t *testing.T) {
	converter := NewGen1Converter(nil)

	config, err := converter.FromAPIConfig(json.RawMessage(gen1RollerSettingsJSON), "SHPLG-S")
	require.NoError(t, err)
	assert.Nil(t, config.Roller)

	raw, err := converter.ToAPIConfig(&DeviceConfiguration{Roller: &RollerConfig{MaxOpenTime: IntPtr(20)}}, "SHPLG-S")
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "rollers")
}

// The stored times are seconds, as are Gen1 maxtime_open and maxtime_close, so no conversion
// happens in either direction.
func TestGen1Converter_RollerTimesAreSeconds(t *testing.T) {
	converter := NewGen1Converter(nil)

	config, err := converter.FromAPIConfig(json.RawMessage(`{"rollers":[{"maxtime_open":7,"maxtime_close":9}]}`), "SHSW-25")
	require.NoError(t, err)
	require.NotNil(t, config.Roller)
	assert.Equal(t, 7, *config.Roller.MaxOpenTime)
	assert.Equal(t, 9, *config.Roller.MaxCloseTime)
}

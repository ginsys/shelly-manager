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

// modeDevice is a Shelly 2.5 that keeps its mode between a write and a read, and refuses the roller
// endpoint while it runs as a relay, as the real device does. events lists every call in order.
type modeDevice struct {
	*rollerApplyClient
	mode       string
	events     []string
	modeErr    error // POST /settings mode is refused
	ignoreMode bool  // POST /settings mode is accepted and not kept

	configErrAfterMode bool // GET /settings fails once a mode was written
	modeWritten        bool
	omitRelays         bool // the settings hold only the rollers, as before a switch to relay mode
}

func newModeDevice(mode string) *modeDevice {
	return &modeDevice{rollerApplyClient: newRollerApplyClient(), mode: mode}
}

func (d *modeDevice) SetConfig(ctx context.Context, cfg map[string]interface{}) error {
	if want, ok := cfg["mode"]; ok {
		d.events = append(d.events, fmt.Sprintf("mode=%v", want))
		d.modeWritten = true
		if d.modeErr != nil {
			return d.modeErr
		}
		if !d.ignoreMode {
			d.mode, _ = want.(string)
		}
		return nil
	}
	d.events = append(d.events, "settings")
	return d.rollerApplyClient.SetConfig(ctx, cfg)
}

func (d *modeDevice) GetConfig(context.Context) (*shelly.DeviceConfig, error) {
	d.events = append(d.events, "get")
	if d.configErrAfterMode && d.modeWritten {
		return nil, errors.New("connection reset")
	}
	settings := map[string]interface{}{
		"mode":      d.mode,
		"max_power": 4600,
		"relays":    []interface{}{map[string]interface{}{"max_power": 4600}, map[string]interface{}{"max_power": 4600}},
	}
	if d.omitRelays {
		delete(settings, "relays")
		settings["rollers"] = []interface{}{map[string]interface{}{}}
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	return &shelly.DeviceConfig{Raw: raw}, nil
}

func (d *modeDevice) SetRollerSettingsForm(ctx context.Context, channel int, settings map[string]interface{}) error {
	d.events = append(d.events, "roller")
	if d.mode != "roller" {
		return errors.New("400: the device is in relay mode")
	}
	return d.rollerApplyClient.SetRollerSettingsForm(ctx, channel, settings)
}

func (d *modeDevice) SetRelaySettings(ctx context.Context, channel int, settings map[string]interface{}) error {
	d.events = append(d.events, "relay")
	return d.applyTestClient.SetRelaySettings(ctx, channel, settings)
}

func withMode(config *DeviceConfiguration, mode string) *DeviceConfiguration {
	config.System = &SystemConfiguration{Device: &TypedDeviceConfig{Mode: StringPtr(mode)}}
	return config
}

// withNamedMode states a name beside the mode, so a refusal that names the mode alone is not
// widened to the whole system section by the round trip.
func withNamedMode(mode string) *DeviceConfiguration {
	config := withMode(&DeviceConfiguration{}, mode)
	config.System.Device.Name = StringPtr("device")
	return config
}

func applyOnModeDevice(t *testing.T, d *modeDevice, config *DeviceConfiguration) *ApplyResult {
	t.Helper()
	result, err := NewConfigApplier(NewGen1Converter(nil), nil).ApplyConfig(context.Background(), d, config, "SHSW-25")
	require.NoError(t, err)
	return result
}

func TestApplyConfig_ModeIsWrittenAndReadBackBeforeTheRollerKeys(t *testing.T) {
	d := newModeDevice("relay")

	result := applyOnModeDevice(t, d, withMode(&DeviceConfiguration{Roller: &RollerConfig{MaxCloseTime: IntPtr(22)}}, "roller"))

	assert.True(t, result.Success, "failures: %+v", result.Failures)
	// The first get is the read before any write; the second is the read-back of the mode.
	assert.Equal(t, []string{"get", "mode=roller", "get", "roller"}, d.events)
	assert.Equal(t, "roller", d.mode)
	assert.Equal(t, 2, result.SettingsCount)
	assert.Equal(t, 2, result.AppliedCount)
}

func TestApplyConfig_ModeAlreadyOnTheDeviceSendsNothing(t *testing.T) {
	d := newModeDevice("roller")

	result := applyOnModeDevice(t, d, withMode(&DeviceConfiguration{Roller: &RollerConfig{MaxCloseTime: IntPtr(22)}}, "roller"))

	assert.True(t, result.Success, "failures: %+v", result.Failures)
	assert.Equal(t, []string{"get", "roller"}, d.events)
	assert.Equal(t, 2, result.SettingsCount)
	assert.Equal(t, 2, result.AppliedCount)
}

func TestApplyConfig_ModeThatFailsStopsEverySectionAfterIt(t *testing.T) {
	desired := func() *DeviceConfiguration {
		config := withMode(&DeviceConfiguration{Roller: &RollerConfig{MaxCloseTime: IntPtr(22)}}, "roller")
		config.System.Device.Name = StringPtr("shutter")
		return config
	}
	cases := map[string]func(d *modeDevice){
		"the write is refused":            func(d *modeDevice) { d.modeErr = errors.New("500: refused") },
		"the device keeps the old mode":   func(d *modeDevice) { d.ignoreMode = true },
		"the read-back of the mode fails": func(d *modeDevice) { d.configErrAfterMode = true },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			d := newModeDevice("relay")
			setup(d)

			result := applyOnModeDevice(t, d, desired())

			assert.False(t, result.Success)
			require.NotEmpty(t, result.Failures)
			assert.Equal(t, "system.device.mode", result.Failures[0].Path)
			// Nothing after the mode: not the main settings (the name), not the roller keys.
			assert.NotContains(t, d.events, "settings")
			assert.NotContains(t, d.events, "roller")
			assert.Empty(t, d.configs)
			assert.Empty(t, d.rollerWrites)
		})
	}
}

func TestUnsupportedApplySettings_ModeAgainstChannelSectionsOfTheOtherMode(t *testing.T) {
	converter := NewGen1Converter(nil)
	relays := &RelayConfig{Relays: []SingleRelayConfig{{ID: 0, Name: StringPtr("lamp")}}}
	roller := &RollerConfig{MaxCloseTime: IntPtr(22)}

	t.Run("roller with a relay section names both", func(t *testing.T) {
		unsupported, err := unsupportedApplySettings(converter, withMode(&DeviceConfiguration{Relay: relays}, "roller"), "SHSW-25", true)
		require.NoError(t, err)
		assert.Contains(t, unsupported, "system.device.mode")
		assert.Contains(t, unsupported, "relay.relays (the mode is roller)")
	})
	t.Run("relay with a roller section names both", func(t *testing.T) {
		unsupported, err := unsupportedApplySettings(converter, withMode(&DeviceConfiguration{Roller: roller}, "relay"), "SHSW-25", true)
		require.NoError(t, err)
		assert.Contains(t, unsupported, "system.device.mode")
		assert.Contains(t, unsupported, "roller (the mode is relay)")
	})
	t.Run("the matching sections are accepted", func(t *testing.T) {
		for mode, config := range map[string]*DeviceConfiguration{"relay": {Relay: relays}, "roller": {Roller: roller}} {
			unsupported, err := unsupportedApplySettings(converter, withMode(config, mode), "SHSW-25", true)
			require.NoError(t, err)
			assert.Empty(t, unsupported, mode)
		}
	})
	t.Run("a word that is no mode", func(t *testing.T) {
		unsupported, err := unsupportedApplySettings(converter, withNamedMode("dual"), "SHSW-25", true)
		require.NoError(t, err)
		assert.Equal(t, []string{"system.device.mode"}, unsupported)
	})
}

func TestUnsupportedApplySettings_ModeOnAModelWithoutTheSwitch(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, deviceType := range []string{"SHSW-1", "SHSW-PM", "SHIX3-1", "SHPLG-S", "SHDM-2"} {
		for _, mode := range []string{"relay", "roller"} {
			unsupported, err := unsupportedApplySettings(converter, withNamedMode(mode), deviceType, true)
			require.NoError(t, err)
			assert.Equal(t, []string{"system.device.mode"}, unsupported, "%s %s", deviceType, mode)
		}
	}
}

// A Gen2 device has no Gen1 mode. The Gen2 check refuses it through its round trip, and this pins it.
func TestUnsupportedGen2Settings_RefusesAGen1Mode(t *testing.T) {
	unsupported, err := unsupportedGen2Settings(NewGen2Converter(nil), withNamedMode("roller"), true)

	require.NoError(t, err)
	assert.Equal(t, []string{"system.device.mode"}, unsupported)
}

// Once the mode is stated, the converted document carries it, and the read-back of a 2.5 in relay
// mode keeps max_power on the channels: a desired model-wide max_power must still be accepted.
func TestUnsupportedApplySettings_MaxPowerWithAStatedMode(t *testing.T) {
	converter := NewGen1Converter(nil)
	for _, mode := range []string{"relay", "roller"} {
		config := withMode(&DeviceConfiguration{PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(3000)}}, mode)
		unsupported, err := unsupportedApplySettings(converter, config, "SHSW-25", true)
		require.NoError(t, err)
		assert.Empty(t, unsupported, mode)
	}
}

func TestApplyConfig_MaxPowerFollowsTheModeThatTheApplyWrites(t *testing.T) {
	power := func() *DeviceConfiguration {
		return &DeviceConfiguration{PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(3000)}}
	}

	t.Run("relay to roller: the value goes to POST /settings", func(t *testing.T) {
		d := newModeDevice("relay")

		result := applyOnModeDevice(t, d, withMode(power(), "roller"))

		assert.True(t, result.Success, "failures: %+v", result.Failures)
		assert.Equal(t, []string{"get", "mode=roller", "get", "settings"}, d.events)
		require.Len(t, d.configs, 1)
		assert.Equal(t, float64(3000), d.configs[0]["max_power"])
		assert.Empty(t, d.relayWrites)
	})
	t.Run("roller to relay: the value goes to every channel", func(t *testing.T) {
		d := newModeDevice("roller")

		result := applyOnModeDevice(t, d, withMode(power(), "relay"))

		assert.True(t, result.Success, "failures: %+v", result.Failures)
		assert.Empty(t, d.configs)
		require.Len(t, d.relayWrites, 2)
		for _, write := range d.relayWrites {
			assert.Equal(t, map[string]interface{}{"max_power": float64(3000)}, write.form)
		}
	})
	t.Run("roller to relay with no relay channel reported before the switch", func(t *testing.T) {
		d := newModeDevice("roller")
		d.omitRelays = true

		result := applyOnModeDevice(t, d, withMode(power(), "relay"))

		assert.True(t, result.Success, "failures: %+v", result.Failures)
		assert.Empty(t, d.configs, "relay mode needs per-channel writes, never POST /settings max_power")
		require.Len(t, d.relayWrites, 2, "a 2.5 has two channels whatever the settings listed")
		for _, write := range d.relayWrites {
			assert.Equal(t, map[string]interface{}{"max_power": float64(3000)}, write.form)
		}
	})
}

func TestGen1Converter_ModeOfASwitchableModel(t *testing.T) {
	converter := NewGen1Converter(nil)

	t.Run("a 2.5 reads and exports its mode", func(t *testing.T) {
		config, err := converter.FromAPIConfig([]byte(`{"mode":"roller","name":"shutter"}`), "SHSW-25")
		require.NoError(t, err)
		require.NotNil(t, config.System)
		require.NotNil(t, config.System.Device)
		assert.Equal(t, "roller", *config.System.Device.Mode)

		out, err := converter.ToAPIConfig(config, "SHSW-25")
		require.NoError(t, err)
		var doc map[string]interface{}
		require.NoError(t, json.Unmarshal(out, &doc))
		assert.Equal(t, "roller", doc["mode"])
	})
	t.Run("a mode alone is a system section", func(t *testing.T) {
		config, err := converter.FromAPIConfig([]byte(`{"mode":"relay"}`), "SHSW-25")
		require.NoError(t, err)
		require.NotNil(t, config.System)
		assert.Equal(t, "relay", *config.System.Device.Mode)
	})
	t.Run("another model keeps no mode", func(t *testing.T) {
		config, err := converter.FromAPIConfig([]byte(`{"mode":"roller","name":"plug"}`), "SHSW-1")
		require.NoError(t, err)
		require.NotNil(t, config.System)
		assert.Nil(t, config.System.Device.Mode)
	})
}

// schemaLeaf returns the schema node at the dotted path of properties, or nil.
func schemaLeaf(schema map[string]interface{}, path ...string) map[string]interface{} {
	node := schema
	for _, key := range path {
		props, _ := node["properties"].(map[string]interface{})
		next, _ := props[key].(map[string]interface{})
		if next == nil {
			return nil
		}
		node = next
	}
	return node
}

func TestDeviceSchema_ModeIsOnlyOnAModelThatHasIt(t *testing.T) {
	path := []string{"system", "device", "mode"}

	mode := schemaLeaf(GetDeviceConfigurationSchema("SHSW-25"), path...)
	require.NotNil(t, mode)
	assert.Equal(t, true, mode[SchemaKeyApplicable])
	assert.Equal(t, []string{"relay", "roller"}, mode["enum"])

	for _, deviceType := range []string{"SHSW-1", "SHSW-PM", "SHIX3-1", "SHPLG-S"} {
		assert.Nil(t, schemaLeaf(GetDeviceConfigurationSchema(deviceType), path...), deviceType)
	}

	// A model apply does not support, and a template, keep the leaf: the first as not applicable, the
	// second as applicable, because a template can reach a 2.5.
	unsupported := schemaLeaf(GetDeviceConfigurationSchema("NOT-A-MODEL"), path...)
	require.NotNil(t, unsupported)
	assert.Equal(t, false, unsupported[SchemaKeyApplicable])
	template := schemaLeaf(GetTemplateConfigurationSchema(), path...)
	require.NotNil(t, template)
	assert.Equal(t, true, template[SchemaKeyApplicable])
}

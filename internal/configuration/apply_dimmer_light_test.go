package configuration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lightApplyConfig() *DeviceConfiguration {
	return &DeviceConfiguration{
		Light: &LightConfig{Lights: []SingleLightConfig{{
			ID:             0,
			DefaultState:   StringPtr("last"),
			AutoOff:        IntPtr(60),
			ButtonType:     StringPtr("toggle"),
			ButtonDebounce: IntPtr(80),
			SwapInputs:     BoolPtr(true),
		}}},
	}
}

// The light channel is a form POST to /settings/light/<n> with only the keys that are set, then a
// read of the same path. SettingsCount and AppliedCount both count one per key written.
func TestApplyDimmer_WritesTheLightChannelAndReadsItBack(t *testing.T) {
	client := newDimmerDevice()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, lightApplyConfig(), "SHDM-1")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.lightWrites, 1)
	assert.Equal(t, 0, client.lightWrites[0].channel)
	assert.Equal(t, map[string]interface{}{
		"default_state": "last",
		"auto_off":      float64(60),
		"btn_type":      "toggle",
		"btn_debounce":  float64(80),
		"swap_inputs":   float64(1),
	}, client.lightWrites[0].form)
	assert.Empty(t, client.relayWrites)
	// The nested document goes nowhere: Gen1 ignores it.
	for _, cfg := range client.configs {
		assert.NotContains(t, cfg, "lights")
	}
	assert.Equal(t, 5, result.SettingsCount)
	assert.Equal(t, 5, result.AppliedCount)
}

func TestApplyDimmer_LightReadBackMismatchFails(t *testing.T) {
	client := newDimmerDevice()
	// The device answers 200 to the write and keeps the old auto_off.
	client.lightLive = map[int]map[string]interface{}{
		0: {"default_state": "last", "auto_off": float64(0), "btn_type": "toggle", "btn_debounce": float64(80), "swap_inputs": float64(1)},
	}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, lightApplyConfig(), "SHDM-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "lights[0].auto_off", result.Failures[0].Path)
	assert.Equal(t, 4, result.AppliedCount)
}

// A client without the light route cannot apply a light, and says so: it never falls back to the
// nested "lights" document, which the device ignores.
func TestApplyDimmer_ClientWithoutTheLightRouteFails(t *testing.T) {
	client := newFakeGen1Device()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, lightApplyConfig(), "SHDM-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "lights", result.Failures[0].Path)
}

// The whole path through the service: the desired light passes the check, is written, and the verify
// reads the device's lights array back as equal.
func TestApplyDesiredDimmer_LightAppliesAndVerifies(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "dimmer")
	setDesired(t, repo, lightApplyConfig())
	dev := newDimmerDevice()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHDM-1")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied, "verify: %+v", result.VerifyResult)
	assert.Len(t, dev.lightWrites, 1)
	assert.Empty(t, dev.relayWrites)
}

// A schedule is the whole list the device keeps: a rule left on the device is a difference, and an
// empty desired list clears.
func TestCompare_LightScheduleRulesAreAnExactList(t *testing.T) {
	rules := func(r ...string) *DeviceConfiguration {
		list := append([]string{}, r...)
		return &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{{ID: 0, ScheduleRules: &list}}}}
	}
	comparator := NewConfigComparator()

	assert.True(t, comparator.Compare(rules("0000-0123456-on"), rules("0000-0123456-on")).Match)
	assert.False(t, comparator.Compare(rules("0000-0123456-on"), rules("0000-0123456-on", "0800-0123456-off")).Match,
		"a rule left on the device is not the desired schedule")
	assert.False(t, comparator.Compare(rules(), rules("0800-0123456-off")).Match,
		"an empty desired list clears the rules")
}

// A light list merges by id like a relay list (ginsys/shelly-manager#537): a sparse override keeps
// each setting on its own channel.
func TestMergeConfigurations_SparseLightOverrideKeepsEachSettingOnItsChannel(t *testing.T) {
	layer := func(name string, lights ...SingleLightConfig) ConfigLayer {
		return ConfigLayer{Name: name, Config: &DeviceConfiguration{Light: &LightConfig{Lights: lights}}}
	}

	result, err := MergeConfigurations([]ConfigLayer{
		layer("template", SingleLightConfig{ID: 0, AutoOn: IntPtr(30)}, SingleLightConfig{ID: 1, AutoOff: IntPtr(60)}),
		layer("override", SingleLightConfig{ID: 1, ButtonType: StringPtr("toggle")}),
	})

	require.NoError(t, err)
	lights := result.Config.Light.Lights
	require.Len(t, lights, 2)
	assert.Equal(t, IntPtr(30), lights[0].AutoOn)
	assert.Nil(t, lights[0].ButtonType)
	assert.Equal(t, IntPtr(60), lights[1].AutoOff)
	assert.Equal(t, StringPtr("toggle"), lights[1].ButtonType)
	assert.Equal(t, "override", result.Sources["light.lights.1.btn_type"])
}

// A patch body's removal path holds the index of the element in the patch. The merge puts a light
// where its id is, so the index becomes the position of the light with that id.
func TestRemapChannelPaths_LightIndexBecomesThePositionOfItsId(t *testing.T) {
	existing := &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{{ID: 0}, {ID: 2}}}}
	patch := &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{{ID: 1}}}}

	remapped := remapChannelPaths([][]string{{"light", "lights", "0", "auto_on"}}, existing, patch)

	assert.Equal(t, [][]string{{"light", "lights", "1", "auto_on"}}, remapped)
}

// The schema enums and the refusal read one list per word set: every word the schema offers is
// accepted, so the form can never offer a word the apply then refuses.
func TestLightSchemaEnumsAreTheWordsTheCheckAccepts(t *testing.T) {
	props := schemaProperties(schemaProperties(lightSchemaOf(getLightSchema(), "SHDM-1", true))["lights"].(map[string]interface{})["items"])
	enum := func(key string) []string { return props[key].(map[string]interface{})["enum"].([]string) }

	require.NotEmpty(t, enum("default_state"))
	for _, word := range enum("default_state") {
		assert.Empty(t, unsupportedLightValues(0, SingleLightConfig{DefaultState: StringPtr(word)}), word)
	}
	assert.ElementsMatch(t, []string{"on", "off", "last", "switch"}, enum("default_state"))

	// The unnarrowed schema serves a Gen2 light too, so it offers the Gen2 words the Dimmer refuses.
	template := schemaProperties(schemaProperties(getLightSchema())["lights"].(map[string]interface{})["items"])
	assert.Subset(t, template["default_state"].(map[string]interface{})["enum"], []string{"restore_last", "toggle"})

	require.NotEmpty(t, enum("btn_type"))
	for _, word := range enum("btn_type") {
		assert.Empty(t, unsupportedLightValues(0, SingleLightConfig{ButtonType: StringPtr(word)}), word)
	}
	assert.Contains(t, enum("btn_type"), "toggle")
	assert.NotContains(t, enum("btn_type"), "toglle")
}

func TestUnsupportedLightValues(t *testing.T) {
	light := func(l SingleLightConfig) *DeviceConfiguration {
		return &DeviceConfiguration{Light: &LightConfig{Lights: []SingleLightConfig{l}}}
	}
	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = "0000-0123456-on"
	}
	for _, tc := range []struct {
		name   string
		config *DeviceConfiguration
		model  string
		want   string // empty: accepted
	}{
		{"debounce below the range", light(SingleLightConfig{ButtonDebounce: IntPtr(59)}), "SHDM-1", "light.lights[0].btn_debounce"},
		{"debounce at the low bound", light(SingleLightConfig{ButtonDebounce: IntPtr(60)}), "SHDM-1", ""},
		{"debounce at the high bound", light(SingleLightConfig{ButtonDebounce: IntPtr(200)}), "SHDM-1", ""},
		{"debounce above the range", light(SingleLightConfig{ButtonDebounce: IntPtr(201)}), "SHDM-1", "light.lights[0].btn_debounce"},
		{"a Gen2 default state", light(SingleLightConfig{DefaultState: StringPtr("restore_last")}), "SHDM-1", "light.lights[0].default_state"},
		{"an unknown default state", light(SingleLightConfig{DefaultState: StringPtr("dim")}), "SHDM-1", "light.lights[0].default_state"},
		{"every default state word", light(SingleLightConfig{DefaultState: StringPtr("switch")}), "SHDM-1", ""},
		{"the typo of the parameter table", light(SingleLightConfig{ButtonType: StringPtr("toglle")}), "SHDM-1", "light.lights[0].btn_type"},
		{"an unknown button type", light(SingleLightConfig{ButtonType: StringPtr("momentary")}), "SHDM-1", "light.lights[0].btn_type"},
		{"a button type of the list", light(SingleLightConfig{ButtonType: StringPtr("dual_button")}), "SHDM-1", ""},
		{"a negative auto_on", light(SingleLightConfig{AutoOn: IntPtr(-1)}), "SHDM-1", "light.lights[0].auto_on"},
		{"a negative auto_off", light(SingleLightConfig{AutoOff: IntPtr(-1)}), "SHDM-1", "light.lights[0].auto_off"},
		{"too many rules", light(SingleLightConfig{ScheduleRules: &tooMany}), "SHDM-1", "light.lights[0].schedule_rules"},
		{"an empty rule", light(SingleLightConfig{ScheduleRules: &[]string{""}}), "SHDM-1", "light.lights[0].schedule_rules"},
		{"a rule with a comma", light(SingleLightConfig{ScheduleRules: &[]string{"a,b"}}), "SHDM-1", "light.lights[0].schedule_rules"},
		{"an id that is not the position", light(SingleLightConfig{ID: 1, AutoOn: IntPtr(1)}), "SHDM-1", "light.lights[0].id"},
		{"the second Dimmer takes the same keys", light(SingleLightConfig{ButtonDebounce: IntPtr(61)}), "SHDM-2", ""},
		{"a model with no light channel", light(SingleLightConfig{AutoOn: IntPtr(1)}), "SHSW-1", "light"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unsupported, err := unsupportedApplySettings(NewGen1Converter(nil), tc.config, tc.model, true)

			require.NoError(t, err)
			if tc.want == "" {
				assert.Empty(t, unsupported)
				return
			}
			assert.Contains(t, unsupported, tc.want, fmt.Sprint(unsupported))
		})
	}
}

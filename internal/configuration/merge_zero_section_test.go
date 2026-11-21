package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A layer that holds only zeros of plain fields (an empty server, a level 0, a false flag) sets nothing:
// a plain field cannot tell a zero from unset (merge_plain_test.go). It must also leave no section, because
// an empty section is something Apply refuses (ginsys/shelly-manager#843).

// zeroJSON gives raw the same shape with every value set to its zero.
func zeroJSON(t *testing.T, raw string) string {
	t.Helper()
	var tree any
	require.NoError(t, json.Unmarshal([]byte(raw), &tree))
	var zero func(v any) any
	zero = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			for key, child := range x {
				x[key] = zero(child)
			}
			return x
		case bool:
			return false
		case float64:
			return float64(0)
		case string:
			return ""
		}
		return v
	}
	out, err := json.Marshal(zero(tree))
	require.NoError(t, err)
	return string(out)
}

func TestMergeConfigurations_ZeroOnlyLayerLeavesNoSection(t *testing.T) {
	for _, tc := range plainScalarCases {
		t.Run(tc.path, func(t *testing.T) {
			layer := parseLayer(t, zeroJSON(t, tc.json))
			require.NotNil(t, layer.System, "the layer must hold the section, or this proves nothing")

			result, err := MergeConfigurations([]ConfigLayer{{Name: "override", Config: layer}})
			require.NoError(t, err)
			assert.Nil(t, result.Config.System, "a zero-only layer left a section")
			assert.Empty(t, result.Sources)

			gen1, gen2 := plainScalarRefusals(t, result.Config)
			assert.Empty(t, gen1)
			assert.Empty(t, gen2)
		})
	}
}

// The body of ginsys/shelly-manager#843, as the only layer.
func TestMergeConfigurations_AnEmptyServerAloneIsNotRefusedByApply(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{
		{Name: "device-override", Config: parseLayer(t, `{"system":{"sntp":{"server":""}}}`)},
	})
	require.NoError(t, err)
	_, held := mergedPath(t, result.Config, "system.sntp")
	assert.False(t, held)

	gen1, gen2 := plainScalarRefusals(t, result.Config)
	assert.Empty(t, gen1)
	assert.Empty(t, gen2)
}

func TestMergeConfigurations_AZeroSectionBesideARealOneIsDropped(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{
		{Name: "override", Config: parseLayer(t, `{"system":{"sleep_mode":{"period":0},"sntp":{"server":"time.example.org"}}}`)},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Config.System)
	assert.Nil(t, result.Config.System.Sleep)
	assert.Equal(t, "time.example.org", result.Config.System.SNTP.Server)
}

// A pointer is set even at false or 0, so its section stays and the value reaches the result.
func TestMergeConfigurations_APointerZeroKeepsItsSection(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{{Name: "template", Config: &DeviceConfiguration{
		MQTT: &MQTTConfiguration{Enable: BoolPtr(false), Port: IntPtr(0)},
	}}})
	require.NoError(t, err)
	require.NotNil(t, result.Config.MQTT)
	require.NotNil(t, result.Config.MQTT.Enable)
	assert.False(t, *result.Config.MQTT.Enable)
	require.NotNil(t, result.Config.MQTT.Port)
	assert.Zero(t, *result.Config.MQTT.Port)
	assert.Equal(t, "template", result.Sources["mqtt.enable"])
	assert.Equal(t, "template", result.Sources["mqtt.port"])
}

// A section a zero-only layer did not make is made by the next layer that sets something in it.
func TestMergeConfigurations_ALaterValueMakesTheSectionAnEarlierZeroDidNot(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{
		{Name: "template", Config: parseLayer(t, `{"system":{"sntp":{"server":""}}}`)},
		{Name: "override", Config: parseLayer(t, `{"system":{"sntp":{"server":"time.example.org"}}}`)},
	})
	require.NoError(t, err)
	assert.Equal(t, "time.example.org", result.Config.System.SNTP.Server)
	assert.Equal(t, "override", result.Sources["system.sntp.server"])
}

package configuration

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plainScalarCases holds one case per plain (non-pointer) scalar field a layer can carry. Apply writes
// only system.sntp.server; the others have no converter mapping (ginsys/shelly-manager#839).
var plainScalarCases = []struct {
	path string
	json string
}{
	{"system.debug.level", `{"system":{"debug":{"level":3}}}`},
	{"system.debug.file_level", `{"system":{"debug":{"file_level":2}}}`},
	{"system.debug.mqtt", `{"system":{"debug":{"mqtt":true}}}`},
	{"system.debug.websocket", `{"system":{"debug":{"websocket":true}}}`},
	{"system.debug.udp", `{"system":{"debug":{"udp":true}}}`},
	{"system.device.ble.enable", `{"system":{"device":{"ble":{"enable":true}}}}`},
	{"system.device.ble.rpc.dst_addr", `{"system":{"device":{"ble":{"rpc":{"dst_addr":"10.0.0.1"}}}}}`},
	{"system.device.ble.rpc.listen_port", `{"system":{"device":{"ble":{"rpc":{"listen_port":8080}}}}}`},
	{"system.rpc_udp.dst_addr", `{"system":{"rpc_udp":{"dst_addr":"10.0.0.1"}}}`},
	{"system.rpc_udp.listen_port", `{"system":{"rpc_udp":{"listen_port":8080}}}`},
	{"system.sleep_mode.enable", `{"system":{"sleep_mode":{"enable":true}}}`},
	{"system.sleep_mode.period", `{"system":{"sleep_mode":{"period":5}}}`},
	{"system.sleep_mode.unit", `{"system":{"sleep_mode":{"unit":"m"}}}`},
	{"system.sntp.server", `{"system":{"sntp":{"server":"time.example.org"}}}`},
	{"system.ui_data.sleep_mode", `{"system":{"ui_data":{"sleep_mode":true}}}`},
}

// plainScalarsNotCovered are the plain scalar fields the table above leaves out on purpose: a channel
// id (merged by id, not as a layer field), the name and index of an action (a layer that names actions
// replaces the whole list, so no field of an entry is merged; TestActionConfig_AMergedLayerReplacesTheListWhole
// holds that) and the time zone internals of an opaque time value.
func plainScalarsNotCovered(path string) bool {
	return path == "relay.relays.id" || path == "input.inputs.id" ||
		path == "actions.name" || path == "actions.index" ||
		strings.HasPrefix(path, "power_metering.energy_reset")
}

func parseLayer(t *testing.T, raw string) *DeviceConfiguration {
	t.Helper()
	config, err := parseApplicableDesiredConfig(raw)
	require.NoError(t, err)
	return config
}

func mergedPath(t *testing.T, config *DeviceConfiguration, path string) (any, bool) {
	t.Helper()
	raw, err := json.Marshal(config)
	require.NoError(t, err)
	var tree any
	require.NoError(t, json.Unmarshal(raw, &tree))
	for _, key := range strings.Split(path, ".") {
		node, ok := tree.(map[string]any)
		if !ok {
			return nil, false
		}
		if tree, ok = node[key]; !ok {
			return nil, false
		}
	}
	return tree, true
}

func plainScalarRefusals(t *testing.T, config *DeviceConfiguration) (gen1, gen2 []string) {
	t.Helper()
	var err error
	gen1, err = unsupportedApplySettings(NewGen1Converter(nil), config, "SHSW-1", true)
	require.NoError(t, err)
	gen2, err = unsupportedGen2Settings(NewGen2Converter(nil), config, true)
	require.NoError(t, err)
	return gen1, gen2
}

func TestMergeConfigurations_PlainScalars(t *testing.T) {
	for _, tc := range plainScalarCases {
		t.Run(tc.path, func(t *testing.T) {
			layer := parseLayer(t, tc.json)
			want, ok := mergedPath(t, layer, tc.path)
			require.True(t, ok, "the case must hold its own path")

			result, err := MergeConfigurations([]ConfigLayer{{Name: "template", Config: layer}})
			require.NoError(t, err)

			got, ok := mergedPath(t, result.Config, tc.path)
			assert.True(t, ok, "the merged configuration lost %s", tc.path)
			assert.Equal(t, want, got)
			assert.Equal(t, "template", result.Sources[tc.path])
		})
	}
}

// A new plain field cannot be added without a case above.
func TestMergeConfigurations_PlainScalarFieldsAreCovered(t *testing.T) {
	covered := map[string]bool{}
	for _, tc := range plainScalarCases {
		covered[tc.path] = true
	}

	var walk func(typ reflect.Type, path string, seen map[reflect.Type]bool)
	walk = func(typ reflect.Type, path string, seen map[reflect.Type]bool) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		defer delete(seen, typ)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			fieldPath := name
			if path != "" {
				fieldPath = path + "." + name
			}
			switch field.Type.Kind() {
			case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64, reflect.String:
				if !plainScalarsNotCovered(fieldPath) {
					assert.True(t, covered[fieldPath], "plain field %s has no case in plainScalarCases", fieldPath)
				}
			case reflect.Pointer, reflect.Slice, reflect.Struct:
				walk(field.Type, fieldPath, seen)
			}
		}
	}
	walk(reflect.TypeOf(DeviceConfiguration{}), "", map[reflect.Type]bool{})
}

// A zero is "not set": a later layer's zero keeps an earlier value, and two layers that set different
// fields of one section both survive.
func TestMergeConfigurations_PlainScalarZeroIsUnset(t *testing.T) {
	t.Run("empty server in a later layer keeps the earlier one, and Apply writes it", func(t *testing.T) {
		result, err := MergeConfigurations([]ConfigLayer{
			{Name: "template", Config: parseLayer(t, `{"system":{"sntp":{"server":"time.example.org"}}}`)},
			{Name: "override", Config: parseLayer(t, `{"system":{"sntp":{"server":""}}}`)},
		})
		require.NoError(t, err)
		assert.Equal(t, "time.example.org", result.Config.System.SNTP.Server)
		assert.Equal(t, "template", result.Sources["system.sntp.server"])

		gen1, gen2 := plainScalarRefusals(t, result.Config)
		assert.Empty(t, gen1)
		assert.Empty(t, gen2)
	})

	t.Run("a patch with an empty value keeps the override's own value", func(t *testing.T) {
		result, err := MergeConfigurations([]ConfigLayer{
			{Name: "existing", Config: parseLayer(t, `{"system":{"sntp":{"server":"time.example.org"}}}`)},
			{Name: "patch", Config: parseLayer(t, `{"system":{"sntp":{"server":""}}}`)},
		})
		require.NoError(t, err)
		assert.Equal(t, "time.example.org", result.Config.System.SNTP.Server)
	})

	t.Run("false and zero do not clear", func(t *testing.T) {
		result, err := MergeConfigurations([]ConfigLayer{
			{Name: "template", Config: parseLayer(t, `{"system":{"debug":{"level":3,"udp":true}}}`)},
			{Name: "override", Config: parseLayer(t, `{"system":{"debug":{"level":0,"udp":false}}}`)},
		})
		require.NoError(t, err)
		assert.Equal(t, 3, result.Config.System.Debug.Level)
		assert.True(t, result.Config.System.Debug.UDPOutput)
	})

	t.Run("different fields of one section both survive", func(t *testing.T) {
		result, err := MergeConfigurations([]ConfigLayer{
			{Name: "template", Config: parseLayer(t, `{"system":{"debug":{"level":3}}}`)},
			{Name: "override", Config: parseLayer(t, `{"system":{"debug":{"udp":true}}}`)},
		})
		require.NoError(t, err)
		assert.Equal(t, 3, result.Config.System.Debug.Level)
		assert.True(t, result.Config.System.Debug.UDPOutput)
		assert.Equal(t, "template", result.Sources["system.debug.level"])
		assert.Equal(t, "override", result.Sources["system.debug.udp"])
	})

	t.Run("a zero alone records no source", func(t *testing.T) {
		result, err := MergeConfigurations([]ConfigLayer{
			{Name: "template", Config: parseLayer(t, `{"system":{"sleep_mode":{"enable":false}}}`)},
		})
		require.NoError(t, err)
		assert.NotContains(t, result.Sources, "system.sleep_mode.enable")
	})
}

func TestMergeConfigurations_PlainScalarLaterLayerWins(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{
		{Name: "global", Config: parseLayer(t, `{"system":{"sntp":{"server":"time.example.org"}}}`)},
		{Name: "device", Config: parseLayer(t, `{"system":{"sntp":{"server":"ntp.example.net"}}}`)},
	})
	require.NoError(t, err)
	assert.Equal(t, "ntp.example.net", result.Config.System.SNTP.Server)
	assert.Equal(t, "device", result.Sources["system.sntp.server"])
}

// refusedSection is the section Apply names when a template holds the plain field at path next to
// system.device.name: the section the field belongs to, as it is the part that does not survive.
func refusedSection(t *testing.T, path string) string {
	t.Helper()
	for _, section := range []string{
		"system.debug", "system.device.ble", "system.rpc_udp", "system.sleep_mode", "system.ui_data",
	} {
		if strings.HasPrefix(path, section+".") {
			return section
		}
	}
	require.Failf(t, "no section", "%s belongs to no section Apply refuses", path)
	return ""
}

// Apply refuses a populated field that does not survive the round trip, so a device whose template holds
// a field Apply cannot write is not appliable, with the refusal naming its section; only the SNTP
// server, which Apply writes, is accepted. A layer that holds only zeros of these fields leaves no
// section to refuse (merge_zero_section_test.go).
func TestApplyRefusalsFollowPlainScalars(t *testing.T) {
	for _, withName := range []bool{false, true} {
		for _, tc := range plainScalarCases {
			name := tc.path
			if withName {
				name += " with a device name"
			}
			t.Run(name, func(t *testing.T) {
				layer := parseLayer(t, tc.json)
				if withName {
					if layer.System.Device == nil {
						layer.System.Device = &TypedDeviceConfig{}
					}
					layer.System.Device.Name = StringPtr("lamp")
				}
				result, err := MergeConfigurations([]ConfigLayer{{Name: "template", Config: layer}})
				require.NoError(t, err)

				gen1, gen2 := plainScalarRefusals(t, result.Config)
				if tc.path == "system.sntp.server" {
					assert.Empty(t, gen1)
					assert.Empty(t, gen2)
					return
				}
				want := []string{"system"}
				if withName {
					want = []string{refusedSection(t, tc.path)}
				}
				assert.Equal(t, want, gen1, "a field Apply cannot write must still be refused on Gen1, naming the same section")
				assert.Equal(t, want, gen2, "a field Apply cannot write must still be refused on Gen2, naming the same section")
			})
		}
	}
}

package configuration

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stamp stands for any type with no exported field, as time.Time is: the merge cannot set a field of
// it, so it has to take the value whole.
type stamp struct{ n int }

type leafElem struct {
	Name   *string `json:"name"`
	Stamp  stamp   `json:"stamp"`
	PStamp *stamp  `json:"pstamp"`
}

// leafRoot reaches every place the merge handles a struct: a pointer field of the top struct, the
// elements of a list that are such values, the elements that are pointers to them, and the two kinds of
// field of a struct that is a list element.
type leafRoot struct {
	Ptr     *stamp     `json:"ptr"`
	Stamps  []stamp    `json:"stamps"`
	PStamps []*stamp   `json:"pstamps"`
	List    []leafElem `json:"list"`
}

func stampPtr(n int) *stamp { return &stamp{n: n} }

func strPtr(s string) *string { return &s }

func TestMergeInto_AStructWithNoExportedFieldIsTakenWhole(t *testing.T) {
	a := &leafRoot{
		Ptr:     stampPtr(1),
		Stamps:  []stamp{{n: 10}, {n: 11}},
		PStamps: []*stamp{stampPtr(20)},
		List:    []leafElem{{Name: strPtr("a"), Stamp: stamp{n: 30}, PStamp: stampPtr(31)}},
	}
	b := &leafRoot{
		Ptr:     stampPtr(2),
		Stamps:  []stamp{{n: 12}},
		PStamps: []*stamp{stampPtr(21)},
		List:    []leafElem{{Stamp: stamp{n: 32}}},
	}

	got := &leafRoot{}
	sources := map[string]string{}
	mergeInto(got, a, sources, "a", "", true)
	mergeInto(got, b, sources, "b", "", true)

	require.NotNil(t, got.Ptr)
	assert.Equal(t, 2, got.Ptr.n)
	require.Len(t, got.Stamps, 2)
	assert.Equal(t, []stamp{{n: 12}, {n: 11}}, got.Stamps)
	require.Len(t, got.PStamps, 1)
	assert.Equal(t, 21, got.PStamps[0].n)
	require.Len(t, got.List, 1)
	assert.Equal(t, "a", *got.List[0].Name)
	assert.Equal(t, stamp{n: 32}, got.List[0].Stamp)
	require.NotNil(t, got.List[0].PStamp, "a later layer that leaves a pointer nil keeps the earlier value")
	assert.Equal(t, 31, got.List[0].PStamp.n)

	assert.Equal(t, map[string]string{
		"ptr":           "b",
		"stamps.0":      "b",
		"stamps.1":      "a",
		"pstamps.0":     "b",
		"list.0.name":   "a",
		"list.0.stamp":  "b",
		"list.0.pstamp": "a",
	}, sources)

	assert.NotSame(t, a.Ptr, got.Ptr)
	assert.NotSame(t, b.Ptr, got.Ptr)
	assert.NotSame(t, b.PStamps[0], got.PStamps[0])
	assert.NotSame(t, a.List[0].PStamp, got.List[0].PStamp)
}

func TestMergeConfigurations_EnergyResetSurvivesTheMergeWithItsSource(t *testing.T) {
	first := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	second := first.Add(time.Hour)
	layer := func(name string, reset *time.Time) ConfigLayer {
		return ConfigLayer{Name: name, Config: &DeviceConfiguration{
			PowerMetering: &PowerMeteringConfig{EnergyReset: reset, MaxPower: IntPtr(100)},
		}}
	}

	t.Run("one layer", func(t *testing.T) {
		in := layer("a", &first)
		got, err := MergeConfigurations([]ConfigLayer{in})
		require.NoError(t, err)

		require.NotNil(t, got.Config.PowerMetering.EnergyReset)
		assert.True(t, first.Equal(*got.Config.PowerMetering.EnergyReset), "got %v", got.Config.PowerMetering.EnergyReset)
		assert.Equal(t, "a", got.Sources["power_metering.energy_reset"])
		assert.NotSame(t, in.Config.PowerMetering.EnergyReset, got.Config.PowerMetering.EnergyReset)
	})

	t.Run("the later layer wins and owns the source", func(t *testing.T) {
		got, err := MergeConfigurations([]ConfigLayer{layer("a", &first), layer("b", &second)})
		require.NoError(t, err)

		require.NotNil(t, got.Config.PowerMetering.EnergyReset)
		assert.True(t, second.Equal(*got.Config.PowerMetering.EnergyReset), "got %v", got.Config.PowerMetering.EnergyReset)
		assert.Equal(t, "b", got.Sources["power_metering.energy_reset"])
	})

	t.Run("a later layer that sets none leaves the earlier value and source", func(t *testing.T) {
		got, err := MergeConfigurations([]ConfigLayer{layer("a", &first), layer("b", nil)})
		require.NoError(t, err)

		require.NotNil(t, got.Config.PowerMetering.EnergyReset)
		assert.True(t, first.Equal(*got.Config.PowerMetering.EnergyReset), "got %v", got.Config.PowerMetering.EnergyReset)
		assert.Equal(t, "a", got.Sources["power_metering.energy_reset"])
	})

	t.Run("a layer that never sets it leaves it unset", func(t *testing.T) {
		got, err := MergeConfigurations([]ConfigLayer{layer("a", nil)})
		require.NoError(t, err)

		assert.Nil(t, got.Config.PowerMetering.EnergyReset)
		assert.NotContains(t, got.Sources, "power_metering.energy_reset")
	})
}

// hasExportedField is the walk's own test for an opaque struct, written apart from the merge's.
func hasExportedField(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			return true
		}
	}
	return false
}

// opaqueFieldPaths lists the Sources path of every field, at any depth, whose type is a struct with no
// exported field (or a list of them, at index 0), in the form the merge writes it.
func opaqueFieldPaths(t reflect.Type, prefix string) []string {
	var paths []string
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		path := buildFieldPath(prefix, sf)
		base := sf.Type
		for base.Kind() == reflect.Pointer {
			base = base.Elem()
		}
		if base.Kind() == reflect.Slice {
			base = base.Elem()
			path += ".0"
			for base.Kind() == reflect.Pointer {
				base = base.Elem()
			}
		}
		if base.Kind() != reflect.Struct {
			continue
		}
		if !hasExportedField(base) {
			paths = append(paths, path)
			continue
		}
		paths = append(paths, opaqueFieldPaths(base, path)...)
	}
	return paths
}

// A new type with no exported field fails this by name, and the repair is to read what the merge does
// with it, never to add it to the list unread.
func TestMergeConfigurations_EveryFieldOfAStructWithNoExportedFieldMergesWithItsSource(t *testing.T) {
	paths := opaqueFieldPaths(reflect.TypeOf(DeviceConfiguration{}), "")
	sort.Strings(paths)
	require.Equal(t, []string{"power_metering.energy_reset"}, paths)

	layers := filledLayers(t, []layerSpec{{1, 2, idsOverlap}, {2, 2, idsOverlap}})
	got, err := MergeConfigurations(layers)
	require.NoError(t, err)

	for _, p := range paths {
		assert.Equal(t, "layer1", got.Sources[p], "source of %s", p)
	}
	want := layers[1].Config.PowerMetering.EnergyReset
	require.NotNil(t, want)
	require.NotNil(t, got.Config.PowerMetering.EnergyReset)
	assert.True(t, want.Equal(*got.Config.PowerMetering.EnergyReset))
}

package configuration

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// How the channel ids of a filled layer are chosen: the same ids in every layer, so the by-id merge
// pairs them; one repeated id inside a list, so the merge falls back to list positions; or ids no other
// layer holds, so the merge adds channels.
const (
	idsOverlap  = "overlap"
	idsRepeated = "repeated"
	idsDisjoint = "disjoint"
)

type layerSpec struct {
	seed  int
	elems int
	ids   string
}

// A time.Time holds only values and an immutable location, so a copy by assignment shares nothing a
// caller can change: the walks below treat it as one value.
var timeType = reflect.TypeOf(time.Time{})

// filler gives every field of a configuration a value that no other layer holds, and reports any field
// the merge copies by assignment that the tests below cannot prove unshared: a map, an interface, an
// array or an unexported field, none of which cloneValue copies.
type filler struct {
	seed    int
	elems   int
	ids     string
	counter int
}

func (f *filler) fill(v reflect.Value, path string, idx int) error {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f.counter++
		v.SetInt(int64(f.seed*1000 + f.counter))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f.counter++
		v.SetUint(uint64(f.seed*1000 + f.counter))
	case reflect.Float32, reflect.Float64:
		f.counter++
		v.SetFloat(float64(f.seed*1000+f.counter) + 0.5)
	case reflect.String:
		f.counter++
		v.SetString(fmt.Sprintf("v%d-%d", f.seed, f.counter))
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		return f.fill(v.Elem(), path, idx)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), f.elems, f.elems)
		for i := 0; i < f.elems; i++ {
			if err := f.fill(s.Index(i), fmt.Sprintf("%s.%d", path, i), i); err != nil {
				return err
			}
		}
		v.Set(s)
	case reflect.Struct:
		if v.Type() == timeType {
			f.counter++
			v.Set(reflect.ValueOf(time.Unix(int64(f.seed*1000+f.counter), 0).UTC()))
			return nil
		}
		for i := 0; i < v.NumField(); i++ {
			sf := v.Type().Field(i)
			fieldPath := path + "." + sf.Name
			if !v.Field(i).CanSet() {
				return fmt.Errorf("%s: unexported field, which cloneValue shares", fieldPath)
			}
			if sf.Name == "ID" && v.Field(i).Kind() == reflect.Int {
				v.Field(i).SetInt(int64(f.channelID(idx)))
				continue
			}
			if err := f.fill(v.Field(i), fieldPath, idx); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%s: a %s field, which cloneValue does not copy", path, v.Kind())
	}
	return nil
}

func (f *filler) channelID(idx int) int {
	switch f.ids {
	case idsRepeated:
		return 7
	case idsDisjoint:
		return f.seed*10 + idx
	default:
		return idx
	}
}

// poison writes the zero value through every pointer and into every list element it reaches, the way a
// caller that edits a configuration in place would.
func poison(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			poison(v.Elem())
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			poison(v.Index(i))
		}
	case reflect.Struct:
		if v.Type() == timeType {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			poison(v.Field(i))
		}
	default:
		if v.CanSet() {
			v.Set(reflect.Zero(v.Type()))
		}
	}
}

func filledLayers(t *testing.T, specs []layerSpec) []ConfigLayer {
	t.Helper()
	layers := make([]ConfigLayer, 0, len(specs))
	for i, spec := range specs {
		cfg := &DeviceConfiguration{}
		f := &filler{seed: spec.seed, elems: spec.elems, ids: spec.ids}
		require.NoError(t, f.fill(reflect.ValueOf(cfg).Elem(), "", 0))
		layers = append(layers, ConfigLayer{Name: fmt.Sprintf("layer%d", i), Config: cfg})
	}
	return layers
}

func snapshot(t *testing.T, cfg *DeviceConfiguration) string {
	t.Helper()
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	return string(b)
}

var aliasingCases = []struct {
	name  string
	specs []layerSpec
}{
	{"one layer", []layerSpec{{1, 2, idsOverlap}}},
	{"two layers, same channel ids", []layerSpec{{1, 2, idsOverlap}, {2, 2, idsOverlap}}},
	{"three layers, lists of different lengths", []layerSpec{{1, 3, idsOverlap}, {2, 2, idsOverlap}, {3, 2, idsOverlap}}},
	{"two layers, channel ids no other layer holds", []layerSpec{{1, 2, idsDisjoint}, {2, 2, idsDisjoint}}},
	{"two layers, a repeated channel id", []layerSpec{{1, 2, idsRepeated}, {2, 3, idsRepeated}}},
}

func TestMergeConfigurations_FilledLayerReachesEveryKindOfField(t *testing.T) {
	layer := filledLayers(t, []layerSpec{{1, 2, idsOverlap}})[0].Config
	got := snapshot(t, layer)

	// A pointer to a scalar, a list of channels, a pointer to a list, and a plain list of scalars.
	for _, key := range []string{`"wifi"`, `"relays"`, `"inputs"`, `"schedule_rules"`, `"lat_lon"`} {
		assert.Contains(t, got, key)
	}
}

func TestMergeConfigurations_FillerNamesAFieldItCannotProve(t *testing.T) {
	type holder struct {
		Fine  *int
		Table map[string]int
	}
	err := (&filler{elems: 1}).fill(reflect.ValueOf(&holder{}).Elem(), "holder", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "holder.Table")

	type hidden struct{ secret int }
	h := hidden{secret: 1}
	err = (&filler{elems: 1}).fill(reflect.ValueOf(&h).Elem(), "hidden", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hidden.secret")
}

func TestMergeConfigurations_MergedResultSharesNoMemoryWithItsLayers(t *testing.T) {
	for _, tc := range aliasingCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("a change to the merged result leaves every layer alone", func(t *testing.T) {
				layers := filledLayers(t, tc.specs)
				before := make([]string, len(layers))
				for i, l := range layers {
					before[i] = snapshot(t, l.Config)
				}

				merged, err := MergeConfigurations(layers)
				require.NoError(t, err)
				poison(reflect.ValueOf(merged.Config).Elem())

				for i, l := range layers {
					assert.JSONEq(t, before[i], snapshot(t, l.Config), "%s changed through the merged result", l.Name)
				}
			})

			t.Run("a change to a layer leaves the merged result alone", func(t *testing.T) {
				layers := filledLayers(t, tc.specs)

				merged, err := MergeConfigurations(layers)
				require.NoError(t, err)
				before := snapshot(t, merged.Config)
				for _, l := range layers {
					poison(reflect.ValueOf(l.Config).Elem())
				}

				assert.JSONEq(t, before, snapshot(t, merged.Config), "the merged result changed through a layer")
			})
		})
	}
}

// The merge must give the same values and the same sources whether it reads the layers or copies of them.
func TestMergeConfigurations_SameResultFromCopiedLayers(t *testing.T) {
	for _, tc := range aliasingCases {
		t.Run(tc.name, func(t *testing.T) {
			layers := filledLayers(t, tc.specs)
			copies := make([]ConfigLayer, len(layers))
			for i, l := range layers {
				c := &DeviceConfiguration{}
				require.NoError(t, json.Unmarshal([]byte(snapshot(t, l.Config)), c))
				copies[i] = ConfigLayer{Name: l.Name, Config: c}
			}

			want, err := MergeConfigurations(copies)
			require.NoError(t, err)
			got, err := MergeConfigurations(layers)
			require.NoError(t, err)

			assert.Equal(t, want.Config, got.Config)
			assert.Equal(t, want.Sources, got.Sources)
		})
	}
}

package configuration

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type ConfigLayer struct {
	Name   string
	Config *DeviceConfiguration
}

type MergeResult struct {
	Config  *DeviceConfiguration
	Sources map[string]string
}

type Engine struct{}

func (Engine) Merge(layers []ConfigLayer) (*MergeResult, error) {
	return MergeConfigurations(layers)
}

func MergeConfigurations(layers []ConfigLayer) (*MergeResult, error) {
	if len(layers) == 0 {
		return &MergeResult{
			Config:  &DeviceConfiguration{},
			Sources: map[string]string{},
		}, nil
	}

	result := &DeviceConfiguration{}
	sources := make(map[string]string)

	for _, layer := range layers {
		if layer.Config == nil {
			continue
		}
		mergeInto(result, layer.Config, sources, layer.Name, "")
	}

	return &MergeResult{
		Config:  result,
		Sources: sources,
	}, nil
}

func mergeInto(dst, src interface{}, sources map[string]string, sourceName, pathPrefix string) {
	dstVal := reflect.ValueOf(dst)
	srcVal := reflect.ValueOf(src)

	if !dstVal.IsValid() || !srcVal.IsValid() {
		return
	}

	if dstVal.Kind() == reflect.Pointer {
		if dstVal.IsNil() {
			dstVal.Set(reflect.New(dstVal.Type().Elem()))
		}
		dstVal = dstVal.Elem()
	}

	if srcVal.Kind() == reflect.Pointer {
		if srcVal.IsNil() {
			return
		}
		srcVal = srcVal.Elem()
	}

	if dstVal.Kind() != reflect.Struct || srcVal.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		srcTypeField := srcVal.Type().Field(i)
		dstField := dstVal.FieldByName(srcTypeField.Name)

		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}

		fieldPath := buildFieldPath(pathPrefix, srcTypeField)

		if srcField.Kind() == reflect.Pointer {
			if srcField.IsNil() {
				continue
			}

			if isPointerToBasicType(srcField.Type()) {
				dstField.Set(srcField)
				sources[fieldPath] = sourceName
			} else if srcField.Elem().Kind() == reflect.Struct {
				if dstField.IsNil() {
					dstField.Set(reflect.New(dstField.Type().Elem()))
				}
				mergeInto(dstField.Interface(), srcField.Interface(), sources, sourceName, fieldPath)
			} else if srcField.Elem().Kind() == reflect.Slice {
				// A pointer to a list replaces the list whole, an empty one included: nil leaves it alone.
				dstField.Set(srcField)
				sources[fieldPath] = sourceName
			}
		} else if srcField.Kind() == reflect.Slice {
			if srcField.Len() == 0 {
				continue
			}

			mergeSlices(dstField, srcField, sources, sourceName, fieldPath)
		}
	}
}

func mergeSlices(dstField, srcField reflect.Value, sources map[string]string, sourceName, fieldPath string) {
	if mergeChannelsByID(dstField, srcField, sources, sourceName, fieldPath) {
		return
	}

	srcLen := srcField.Len()

	if dstField.IsNil() || dstField.Len() < srcLen {
		newSlice := reflect.MakeSlice(dstField.Type(), srcLen, srcLen)
		if !dstField.IsNil() {
			reflect.Copy(newSlice, dstField)
		}
		dstField.Set(newSlice)
	}

	for i := 0; i < srcLen; i++ {
		srcElem := srcField.Index(i)
		dstElem := dstField.Index(i)

		elemPath := fieldPath + "." + strconv.Itoa(i)

		if srcElem.Kind() == reflect.Struct {
			mergeStructs(dstElem, srcElem, sources, sourceName, elemPath)
		} else if srcElem.Kind() == reflect.Pointer {
			if !srcElem.IsNil() {
				if srcElem.Elem().Kind() == reflect.Struct {
					if dstElem.IsNil() {
						dstElem.Set(reflect.New(dstElem.Type().Elem()))
					}
					mergeStructs(dstElem.Elem(), srcElem.Elem(), sources, sourceName, elemPath)
				} else {
					dstElem.Set(srcElem)
					sources[elemPath] = sourceName
				}
			}
		} else {
			if dstElem.CanSet() {
				dstElem.Set(srcElem)
				sources[elemPath] = sourceName
			}
		}
	}
}

// channelIDs returns the channel id of each element of a list of structs that carry an int ID, and
// false for any other list or when two elements share an id. An id left out of a stored template
// decodes as 0, so in a list of more than one entry whose ids are all 0 the id is the list index,
// which is what the Gen1 apply requires of a channel (id equals position).
func channelIDs(list reflect.Value) ([]int, bool) {
	elem := list.Type().Elem()
	if elem.Kind() != reflect.Struct {
		return nil, false
	}
	if field, ok := elem.FieldByName("ID"); !ok || field.Type.Kind() != reflect.Int {
		return nil, false
	}

	ids := make([]int, list.Len())
	allZero := true
	for i := range ids {
		ids[i] = int(list.Index(i).FieldByName("ID").Int())
		allZero = allZero && ids[i] == 0
	}
	if allZero && len(ids) > 1 {
		for i := range ids {
			ids[i] = i
		}
	}

	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, false
		}
		seen[id] = true
	}
	return ids, true
}

// pairChannels returns, for each channel of src, its position in the list that merging src into dst
// gives: the position of the channel with the same id in dst, or the next position after the end of
// dst for a channel dst lacks. It reports false for a list that cannot be paired by id.
func pairChannels(dst, src reflect.Value) ([]int, bool) {
	dstIDs, ok := channelIDs(dst)
	if !ok {
		return nil, false
	}
	srcIDs, ok := channelIDs(src)
	if !ok {
		return nil, false
	}

	at := make(map[int]int, len(dstIDs)+len(srcIDs))
	for i, id := range dstIDs {
		at[id] = i
	}

	positions := make([]int, len(srcIDs))
	for i, id := range srcIDs {
		pos, found := at[id]
		if !found {
			pos = len(at)
			at[id] = pos
		}
		positions[i] = pos
	}
	return positions, true
}

// mergeChannelsByID merges a list of channels by id, so that a sparse layer that names channel 1
// changes channel 1 and never the first channel of the list. A channel the destination lacks is
// appended as a copy. It reports false, and changes nothing, for a list it cannot pair by id; the
// caller then merges by position.
func mergeChannelsByID(dstField, srcField reflect.Value, sources map[string]string, sourceName, fieldPath string) bool {
	positions, ok := pairChannels(dstField, srcField)
	if !ok {
		return false
	}
	srcIDs, _ := channelIDs(srcField)

	for i, pos := range positions {
		if pos == dstField.Len() {
			dstField.Set(reflect.Append(dstField, reflect.Zero(dstField.Type().Elem())))
		}
		elemPath := fieldPath + "." + strconv.Itoa(pos)
		mergeStructs(dstField.Index(pos), cloneValue(srcField.Index(i)), sources, sourceName, elemPath)
		dstField.Index(pos).FieldByName("ID").SetInt(int64(srcIDs[i]))
	}
	return true
}

// remapChannelPaths turns the paths of a patch body into paths of the configuration that merging
// the patch into existing gives. A path through a list holds the index of the element in the patch
// body. The merge puts a channel where its id is, so in a relay or input channel list the index is
// changed to the position of the channel with the same id. A list that is merged by position, and
// every other path, keeps its index. The paths given are not changed.
func remapChannelPaths(paths [][]string, existing, patch *DeviceConfiguration) [][]string {
	var existingRelays, patchRelays []SingleRelayConfig
	if existing.Relay != nil {
		existingRelays = existing.Relay.Relays
	}
	if patch.Relay != nil {
		patchRelays = patch.Relay.Relays
	}
	var existingInputs, patchInputs []SingleInputConfig
	if existing.Input != nil {
		existingInputs = existing.Input.Inputs
	}
	if patch.Input != nil {
		patchInputs = patch.Input.Inputs
	}

	relayAt, _ := pairChannels(reflect.ValueOf(existingRelays), reflect.ValueOf(patchRelays))
	inputAt, _ := pairChannels(reflect.ValueOf(existingInputs), reflect.ValueOf(patchInputs))

	remapped := make([][]string, len(paths))
	for i, path := range paths {
		remapped[i] = path
		if len(path) < 3 {
			continue
		}
		var positions []int
		switch {
		case strings.EqualFold(path[0], "relay") && strings.EqualFold(path[1], "relays"):
			positions = relayAt
		case strings.EqualFold(path[0], "input") && strings.EqualFold(path[1], "inputs"):
			positions = inputAt
		}
		index, err := strconv.Atoi(path[2])
		if err != nil || index < 0 || index >= len(positions) {
			continue
		}
		remapped[i] = append([]string{path[0], path[1], strconv.Itoa(positions[index])}, path[3:]...)
	}
	return remapped
}

// cloneValue returns a deep copy of v, so that a value taken from one layer shares no pointer or
// list with the merged result.
func cloneValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		clone := reflect.New(v.Type().Elem())
		clone.Elem().Set(cloneValue(v.Elem()))
		return clone
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		clone := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			clone.Index(i).Set(cloneValue(v.Index(i)))
		}
		return clone
	case reflect.Struct:
		clone := reflect.New(v.Type()).Elem()
		clone.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if clone.Field(i).CanSet() {
				clone.Field(i).Set(cloneValue(v.Field(i)))
			}
		}
		return clone
	default:
		return v
	}
}

func mergeStructs(dstVal, srcVal reflect.Value, sources map[string]string, sourceName, pathPrefix string) {
	if !dstVal.IsValid() || !srcVal.IsValid() {
		return
	}

	if srcVal.Kind() != reflect.Struct || dstVal.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		srcTypeField := srcVal.Type().Field(i)
		dstField := dstVal.FieldByName(srcTypeField.Name)

		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}

		fieldPath := buildFieldPath(pathPrefix, srcTypeField)

		if srcField.Kind() == reflect.Pointer {
			if srcField.IsNil() {
				continue
			}

			if isPointerToBasicType(srcField.Type()) {
				dstField.Set(srcField)
				sources[fieldPath] = sourceName
			} else if srcField.Elem().Kind() == reflect.Struct {
				if dstField.IsNil() {
					dstField.Set(reflect.New(dstField.Type().Elem()))
				}
				mergeStructs(dstField.Elem(), srcField.Elem(), sources, sourceName, fieldPath)
			} else if srcField.Elem().Kind() == reflect.Slice {
				// A pointer to a list replaces the list whole, an empty one included: nil leaves it alone.
				dstField.Set(srcField)
				sources[fieldPath] = sourceName
			}
		} else if srcField.Kind() == reflect.Struct {
			mergeStructs(dstField, srcField, sources, sourceName, fieldPath)
		} else {
			if dstField.CanSet() {
				dstField.Set(srcField)
				sources[fieldPath] = sourceName
			}
		}
	}
}

func buildFieldPath(prefix string, field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		if fieldName != "" && fieldName != "-" {
			if prefix == "" {
				return fieldName
			}
			return prefix + "." + fieldName
		}
	}

	fieldName := toSnakeCase(field.Name)
	if prefix == "" {
		return fieldName
	}
	return prefix + "." + fieldName
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func isPointerToBasicType(t reflect.Type) bool {
	if t.Kind() != reflect.Pointer {
		return false
	}

	elem := t.Elem()
	kind := elem.Kind()

	return kind == reflect.Bool ||
		kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 ||
		kind == reflect.Int32 || kind == reflect.Int64 ||
		kind == reflect.Uint || kind == reflect.Uint8 || kind == reflect.Uint16 ||
		kind == reflect.Uint32 || kind == reflect.Uint64 ||
		kind == reflect.Float32 || kind == reflect.Float64 ||
		kind == reflect.String
}

func GetFieldSource(sources map[string]string, path string) (string, error) {
	source, ok := sources[path]
	if !ok {
		return "", fmt.Errorf("no source found for path: %s", path)
	}
	return source, nil
}

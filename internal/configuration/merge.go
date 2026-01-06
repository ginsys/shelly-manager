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

	if dstVal.Kind() == reflect.Ptr {
		if dstVal.IsNil() {
			dstVal.Set(reflect.New(dstVal.Type().Elem()))
		}
		dstVal = dstVal.Elem()
	}

	if srcVal.Kind() == reflect.Ptr {
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

		if srcField.Kind() == reflect.Ptr {
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
		} else if srcElem.Kind() == reflect.Ptr {
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

		if srcField.Kind() == reflect.Ptr {
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
	if t.Kind() != reflect.Ptr {
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

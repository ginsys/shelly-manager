package configuration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// NullPaths lists the path of every field that a raw JSON body sets to null. A decoded struct
// cannot tell null from an omitted field, so the raw text is read. Keys are lower case, because the
// decoder matches keys without regard to case, so the keys keep their own spelling here and
// RemovePaths folds them the way the decoder does. A path through an array holds the index of the
// element. A key that appears twice in one object, in any casing, is an error: the struct decode
// and this reading could not agree on which one wins.
func NullPaths(raw []byte) ([][]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var paths [][]string
	if err := readNulls(dec, nil, &paths); err != nil {
		return nil, err
	}
	return paths, nil
}

func readNulls(dec *json.Decoder, path []string, paths *[][]string) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch t := tok.(type) {
	case nil:
		if len(path) > 0 {
			*paths = append(*paths, append([]string{}, path...))
		}
	case json.Delim:
		switch t {
		case '{':
			var seen []string
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return err
				}
				key := keyTok.(string)
				for _, other := range seen {
					if strings.EqualFold(key, other) {
						return fmt.Errorf("duplicate key %q", key)
					}
				}
				seen = append(seen, key)
				if err := readNulls(dec, append(path, key), paths); err != nil {
					return err
				}
			}
		case '[':
			for i := 0; dec.More(); i++ {
				if err := readNulls(dec, append(path, strconv.Itoa(i)), paths); err != nil {
					return err
				}
			}
		}
		if _, err := dec.Token(); err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

// RemovePaths removes the fields at the given paths from config. An optional field (a pointer) is
// removed. A plain field returns to its zero value, and one without omitempty is still written.
// A section that a removal leaves with no JSON content is removed too. A path that is not stored
// changes nothing, and neither does the section around it.
func RemovePaths(config *DeviceConfiguration, paths [][]string) {
	for _, path := range paths {
		removeAt(reflect.ValueOf(config), path)
	}
}

// removeAt reports whether it removed a field below v.
func removeAt(v reflect.Value, path []string) bool {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		field, ok := fieldByJSONName(v, path[0])
		if !ok {
			return false
		}
		if len(path) == 1 {
			field.Set(reflect.Zero(field.Type()))
			return true
		}
		if !removeAt(field, path[1:]) {
			return false
		}
		if field.Kind() == reflect.Pointer && !field.IsNil() && writesNothing(field.Interface()) {
			field.Set(reflect.Zero(field.Type()))
		}
		return true
	case reflect.Slice:
		i, err := strconv.Atoi(path[0])
		if err != nil || i < 0 || i >= v.Len() || len(path) == 1 {
			return false
		}
		return removeAt(v.Index(i), path[1:])
	}
	return false
}

// writesNothing reports whether a section has no JSON content. A field without omitempty is
// content even at its zero value, so the zero struct is not always empty.
func writesNothing(section any) bool {
	data, err := json.Marshal(section)
	return err == nil && string(data) == "{}"
}

// fieldByJSONName finds the settable field that the decoder fills for key.
func fieldByJSONName(v reflect.Value, key string) (reflect.Value, bool) {
	for i := 0; i < v.NumField(); i++ {
		sf := v.Type().Field(i)
		if sf.PkgPath != "" {
			continue
		}
		name, _, _ := strings.Cut(sf.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = sf.Name
		}
		if strings.EqualFold(name, key) {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

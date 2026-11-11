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
// decoder matches keys without regard to case. A path through an array holds the index of the
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
			seen := map[string]bool{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return err
				}
				key := strings.ToLower(keyTok.(string))
				if seen[key] {
					return fmt.Errorf("duplicate key %q", key)
				}
				seen[key] = true
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
// A section that the removal leaves empty is removed too. A path that is not stored changes
// nothing.
func RemovePaths(config *DeviceConfiguration, paths [][]string) {
	for _, path := range paths {
		removeAt(reflect.ValueOf(config), path)
	}
}

func removeAt(v reflect.Value, path []string) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		field, ok := fieldByJSONName(v, path[0])
		if !ok {
			return
		}
		if len(path) == 1 {
			field.Set(reflect.Zero(field.Type()))
			return
		}
		removeAt(field, path[1:])
		if field.Kind() == reflect.Pointer && !field.IsNil() && field.Elem().Kind() == reflect.Struct && field.Elem().IsZero() {
			field.Set(reflect.Zero(field.Type()))
		}
	case reflect.Slice:
		i, err := strconv.Atoi(path[0])
		if err != nil || i < 0 || i >= v.Len() || len(path) == 1 {
			return
		}
		removeAt(v.Index(i), path[1:])
	}
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

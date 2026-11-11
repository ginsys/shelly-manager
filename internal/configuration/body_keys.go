package configuration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// CheckConfigKeys checks the keys of a raw configuration body against the keys of the stored
// DeviceConfiguration. encoding/json matches a key without regard to case, ignores an unknown
// key and merges a repeated one, so a body can be stored as a different configuration than the
// one a schema check read. This check refuses an unknown key, a key in the wrong case and a
// repeated key, and names the key. The errors are sorted by path.
func CheckConfigKeys(raw []byte) []FieldError {
	var errs []FieldError
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	walkKeys(raw, reflect.TypeOf(DeviceConfiguration{}), "", true, true, &errs)
	sort.Slice(errs, func(i, j int) bool { return errs[i].Path < errs[j].Path })
	return errs
}

// CheckRequestKeys checks the top-level keys of a raw request body against the fields of the
// request type. It refuses a repeated key and a key that differs from a field only in case. A
// key that matches no field is left alone, as the typed decoder leaves it. It does not look
// inside the values.
func CheckRequestKeys(raw []byte, request reflect.Type) []FieldError {
	var errs []FieldError
	walkKeys(raw, request, "", false, false, &errs)
	sort.Slice(errs, func(i, j int) bool { return errs[i].Path < errs[j].Path })
	return errs
}

func walkKeys(raw []byte, t reflect.Type, path string, refuseUnknown, descend bool, errs *[]FieldError) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return
	}
	switch {
	case raw[0] == '{' && t.Kind() == reflect.Struct:
		fields := jsonFieldTypes(t)
		dec := json.NewDecoder(bytes.NewReader(raw))
		if _, err := dec.Token(); err != nil {
			return
		}
		seen := map[string]bool{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return
			}
			key, _ := keyTok.(string)
			var value json.RawMessage
			if err := dec.Decode(&value); err != nil {
				return
			}
			keyPath := joinKeyPath(path, key)
			if seen[key] {
				*errs = append(*errs, FieldError{Path: keyPath, Message: fmt.Sprintf("Duplicate key %q", key)})
				continue
			}
			seen[key] = true
			fieldType, known := fields[key]
			if !known {
				if want, folded := foldedKey(fields, key); folded {
					*errs = append(*errs, FieldError{Path: keyPath,
						Message: fmt.Sprintf("Unknown key %q; the key is %q", key, want)})
				} else if refuseUnknown {
					*errs = append(*errs, FieldError{Path: keyPath, Message: fmt.Sprintf("Unknown key %q", key)})
				}
				continue
			}
			if descend {
				walkKeys(value, fieldType, keyPath, refuseUnknown, descend, errs)
			}
		}
	case raw[0] == '[' && (t.Kind() == reflect.Slice || t.Kind() == reflect.Array):
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return
		}
		for i, item := range items {
			walkKeys(item, t.Elem(), fmt.Sprintf("%s.%d", path, i), refuseUnknown, descend, errs)
		}
	}
}

func joinKeyPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// foldedKey finds the stored key that key matches only without regard to case.
func foldedKey(fields map[string]reflect.Type, key string) (string, bool) {
	var names []string
	for name := range fields {
		if strings.EqualFold(name, key) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", false
	}
	sort.Strings(names)
	return names[0], true
}

// jsonFieldTypes maps each JSON key of a struct to the type of its field, as encoding/json
// names them: the tag name, else the field name; an untagged embedded struct adds its fields.
func jsonFieldTypes(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				for k, v := range jsonFieldTypes(ft) {
					out[k] = v
				}
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[name] = f.Type
	}
	return out
}

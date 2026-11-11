// Package redact holds the one rule that says which JSON key names a secret, and the walk that
// masks the values under such keys. The API routes and the exports share it, so a key that is
// masked in one is masked in the other.
package redact

import (
	"encoding/json"
	"strings"
)

// IsKey reports whether a JSON key names a secret value: a name that contains pass, pwd, psk,
// secret or token, or that ends in key. A key that merely looks like one is masked, and that costs
// a masked display and nothing else.
func IsKey(key string) bool {
	k := strings.ToLower(key)
	for _, word := range []string{"pass", "pwd", "psk", "secret", "token"} {
		if strings.Contains(k, word) {
			return true
		}
	}
	return strings.HasSuffix(k, "key")
}

// IsScalar reports whether a decoded JSON value is one that a secret key can hold: a non-empty
// string or a number. A number is a secret too ({"auth_pass": 12345678}).
func IsScalar(v any) bool {
	switch x := v.(type) {
	case string:
		return x != ""
	case json.Number, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	}
	return false
}

// Mask replaces, in place, every secret scalar under a secret-named key of a decoded JSON tree
// (maps and slices) with mask. A secret key that holds an object or a list is secret as a whole:
// every string and number inside it is replaced, so {"api_tokens":["a","b"]} keeps nothing. An
// empty secret stays empty.
func Mask(v any, mask string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			switch {
			case IsKey(k) && IsScalar(child):
				x[k] = mask
			case IsKey(k):
				maskAll(child, mask)
			default:
				Mask(child, mask)
			}
		}
	case []any:
		for _, child := range x {
			Mask(child, mask)
		}
	}
}

// maskAll masks every scalar inside a container held by a secret-named key.
func maskAll(v any, mask string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if IsScalar(child) {
				x[k] = mask
				continue
			}
			maskAll(child, mask)
		}
	case []any:
		for i, child := range x {
			if IsScalar(child) {
				x[i] = mask
				continue
			}
			maskAll(child, mask)
		}
	}
}

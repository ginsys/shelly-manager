package api

import (
	"encoding/json"
	"strings"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

const redactedValue = "[redacted]"

// secretKeyFragments mark a configuration key as holding a secret: password, pass, passphrase,
// psk, secret, token, key (the Wi-Fi access point key, API keys). Matching is on the lowercased key.
var secretKeyFragments = []string{"pass", "psk", "secret", "token", "key"}

func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	for _, fragment := range secretKeyFragments {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	return false
}

// redactDifferenceValue hides secrets in one side of a configuration difference. A difference at
// a secret path (for example "wifi.password") is masked whole; a difference that holds a whole
// section (for example "mqtt") is masked key by key.
func redactDifferenceValue(path string, value any) any {
	if value == nil {
		return nil
	}
	last := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		last = path[i+1:]
	}
	if isSecretKey(last) {
		return redactedValue
	}
	switch value.(type) {
	case string, bool, float64, float32, int, int64, uint, uint64:
		return value
	}
	raw, err := json.Marshal(value)
	if err != nil {
		// Cannot inspect it, so do not return it.
		return redactedValue
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return redactedValue
	}
	return redactTree(generic)
}

func redactTree(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if isSecretKey(key) && child != nil {
				v[key] = redactedValue
				continue
			}
			v[key] = redactTree(child)
		}
		return v
	case []any:
		for i, child := range v {
			v[i] = redactTree(child)
		}
		return v
	default:
		return value
	}
}

// redactDifference returns d with secrets removed from both sides.
func redactDifference(d configuration.ConfigDifference) (path string, expected, actual any) {
	return d.Path, redactDifferenceValue(d.Path, d.Expected), redactDifferenceValue(d.Path, d.Actual)
}

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

// A device carries saved secrets inside JSON text: Overrides, DesiredConfig and Settings are
// strings that hold a document, and the live configuration is the device's own raw settings. None of
// these has the DeviceConfiguration shape (the live one is the device's own: Gen1 wifi_sta.key,
// login.password, mqtt.pass), so the typed secret slots cannot reach them. These helpers walk the
// document instead and decide by the key name.

// secretKey reports whether a JSON key names a secret value. It is deliberately the rule that
// secret_invariant_test.go already uses for the typed configuration (a field named like a secret
// is one), so the two agree. A key that merely looks like one is masked on a read and restored by
// the stored value on a save, so a false match costs a masked display and nothing else.
func secretKey(key string) bool {
	k := strings.ToLower(key)
	for _, word := range []string{"pass", "pwd", "psk", "secret", "token"} {
		if strings.Contains(k, word) {
			return true
		}
	}
	return strings.HasSuffix(k, "key")
}

// secretScalar reports whether a JSON value is one that a secret key can hold: a non-empty string
// or a number. A number is a secret too ({"auth_pass": 12345678}), so it is masked like a string.
func secretScalar(v any) bool {
	switch x := v.(type) {
	case string:
		return x != ""
	case json.Number:
		return true
	}
	return false
}

func decodeDocument(text string) (any, bool) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false // trailing data
	}
	return doc, true
}

func encodeDocument(doc any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return secretMask
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// maskJSONText returns text with every non-empty string held under a secret-named key replaced by
// secretMask. Text that is not valid JSON cannot be walked, and guessing key names in it fails
// (escapes, truncation), so it is returned as the mask whole: a malformed text never leaks.
func maskJSONText(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	doc, ok := decodeDocument(text)
	if !ok {
		return secretMask // text we cannot parse is masked whole, whatever it holds
	}
	maskValue(doc)
	return encodeDocument(doc)
}

func maskValue(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if secretKey(k) && secretScalar(child) {
				x[k] = secretMask
				continue
			}
			maskValue(child)
		}
	case []any:
		for _, child := range x {
			maskValue(child)
		}
	}
}

// restoreMaskedText answers a client that sends back what maskJSONText gave it. A secret that
// arrives as the mask becomes the stored value, or is dropped when none is stored: the mask itself
// is never stored. Any other value is a new secret and stays as sent. A text that is not valid JSON
// is the client's own and stays as sent, except the mask standing in for a whole malformed text,
// which becomes the stored text.
func restoreMaskedText(incoming, stored string) string {
	if incoming == secretMask {
		return stored
	}
	doc, ok := decodeDocument(incoming)
	if !ok {
		return incoming
	}
	old, _ := decodeDocument(stored)
	restoreValue(doc, old)
	return encodeDocument(doc)
}

func restoreValue(v, stored any) {
	switch x := v.(type) {
	case map[string]any:
		old, _ := stored.(map[string]any)
		for k, child := range x {
			if s, isString := child.(string); isString && s == secretMask && secretKey(k) {
				if kept, ok := old[k]; ok && secretScalar(kept) && kept != secretMask {
					x[k] = kept
				} else {
					delete(x, k)
				}
				continue
			}
			restoreValue(child, old[k])
		}
	case []any:
		old, _ := stored.([]any)
		for i, child := range x {
			// An entry gets the secret of the stored entry at its position only when the two
			// agree on everything but their secrets. A reordered or removed entry would otherwise
			// take another entry's password, so its mask is dropped instead.
			var counterpart any
			if i < len(old) && encodeDocument(withoutSecrets(child)) == encodeDocument(withoutSecrets(old[i])) {
				counterpart = old[i]
			}
			restoreValue(child, counterpart)
		}
	}
}

// withoutSecrets returns a copy of a decoded document without the values held under secret keys.
func withoutSecrets(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			if secretKey(k) && secretScalar(child) {
				continue
			}
			out[k] = withoutSecrets(child)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = withoutSecrets(child)
		}
		return out
	}
	return v
}

// storedAuthPassword returns the device login password held in a device's Settings text: the flat
// auth_pass the client builders read, else the nested auth.password an older version wrote. It is
// empty when none is stored.
func storedAuthPassword(settings string) string {
	doc, _ := decodeDocument(settings)
	m, _ := doc.(map[string]any)
	if pass, ok := m["auth_pass"].(string); ok && pass != "" {
		return pass
	}
	nested, _ := m["auth"].(map[string]any)
	pass, _ := nested["password"].(string)
	return pass
}

// maskDeviceSecrets returns a copy of the device that is safe to send out. The stored record is
// never changed: drift detection and apply read the real values from it.
func maskDeviceSecrets(d database.Device) database.Device {
	d.Settings = maskJSONText(d.Settings)
	d.Overrides = maskJSONText(d.Overrides)
	d.DesiredConfig = maskJSONText(d.DesiredConfig)
	return d
}

// restoreDeviceSecrets undoes maskDeviceSecrets on a device a client sends back, from the stored
// device (nil for a new one).
func restoreDeviceSecrets(incoming *database.Device, stored *database.Device) {
	var old database.Device
	if stored != nil {
		old = *stored
	}
	incoming.Settings = restoreMaskedText(incoming.Settings, old.Settings)
	incoming.Overrides = restoreMaskedText(incoming.Overrides, old.Overrides)
	incoming.DesiredConfig = restoreMaskedText(incoming.DesiredConfig, old.DesiredConfig)
}

// maskDeviceConfig returns a copy of a stored or live device configuration that is safe to send
// out. Config is the device's own raw settings. A masked malformed text is not valid JSON, so it is
// sent as a JSON string.
func maskDeviceConfig(c *configuration.DeviceConfig) *configuration.DeviceConfig {
	if c == nil {
		return nil
	}
	out := *c
	out.Config = maskRawJSON(out.Config)
	return &out
}

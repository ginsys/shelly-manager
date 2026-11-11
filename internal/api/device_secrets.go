package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/redact"
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
func secretKey(key string) bool { return redact.IsKey(key) }

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
//
// It returns errAmbiguousEntry when a masked secret sits in an array entry that matches more than
// one stored entry, because then no stored secret can be told to belong to it.
func restoreMaskedText(incoming, stored string) (string, error) {
	if incoming == secretMask {
		return stored, nil
	}
	doc, ok := decodeDocument(incoming)
	if !ok {
		return incoming, nil
	}
	old, _ := decodeDocument(stored)
	if err := restoreValue(doc, old); err != nil {
		return "", err
	}
	return encodeDocument(doc), nil
}

// errAmbiguousEntry is returned when an array entry that needs a stored secret (it sends the mask,
// or leaves the secret out) matches more than one stored entry whose secrets differ. Guessing would
// give it another entry's password, so the request is refused and asks for the value.
var errAmbiguousEntry = errors.New("an array entry matches more than one stored entry, so its saved secret cannot be told apart: send the secret value instead of the mask or leaving it out")

func restoreValue(v, stored any) error {
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
			if err := restoreValue(child, old[k]); err != nil {
				return err
			}
		}
	case []any:
		old, _ := stored.([]any)
		for i, child := range x {
			// An entry gets the secret of the stored entry at its position only when the two
			// agree on everything but their secrets. A reordered or removed entry would otherwise
			// take another entry's password, so its mask is dropped instead.
			var counterpart any
			if i < len(old) && sameWithoutSecrets(child, old[i]) {
				if ambiguousEntry(child, old, false) {
					return errAmbiguousEntry
				}
				counterpart = old[i]
			}
			if err := restoreValue(child, counterpart); err != nil {
				return err
			}
		}
	}
	return nil
}

func sameWithoutSecrets(a, b any) bool {
	return encodeDocument(canonicalNumbers(withoutSecrets(a))) == encodeDocument(canonicalNumbers(withoutSecrets(b)))
}

// ambiguousEntry reports whether an entry matches more than one stored entry (they agree on
// everything but their secrets) and those entries disagree at a secret the entry gets back from
// them. Entries that differ only in a secret the entry does not restore (it sends a new value) are
// not ambiguous. omitted says whether a secret the entry leaves out counts as restored, as in the
// typed raw part, or only a mask does.
func ambiguousEntry(entry any, stored []any, omitted bool) bool {
	var first string
	matches := 0
	for _, s := range stored {
		if !sameWithoutSecrets(entry, s) {
			continue
		}
		view := encodeDocument(canonicalNumbers(restoredView(entry, s, omitted)))
		if matches == 0 {
			first = view
		} else if view != first {
			return true
		}
		matches++
	}
	return false
}

// restoredView returns what stored would give back to v: the saved secret under each key where v
// sends the mask and, when omitted is set, under each key it leaves out. It is nil when nothing is
// restored. A mask no stored entry holds a secret for is dropped, so it gives back nothing.
func restoredView(v, stored any, omitted bool) any {
	switch x := v.(type) {
	case map[string]any:
		old, _ := stored.(map[string]any)
		out := map[string]any{}
		for k, child := range x {
			if s, isString := child.(string); isString && s == secretMask && secretKey(k) {
				if kept, ok := old[k]; ok && secretScalar(kept) && kept != secretMask {
					out[k] = kept
				}
				continue
			}
			if view := restoredView(child, old[k], omitted); view != nil {
				out[k] = view
			}
		}
		if omitted {
			for k, sv := range old {
				if _, present := x[k]; !present && holdsSecret(k, sv) {
					out[k] = sv
				}
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		old, _ := stored.([]any)
		out := make([]any, len(x))
		found := false
		for i, child := range x {
			if i < len(old) && sameWithoutSecrets(child, old[i]) {
				if view := restoredView(child, old[i], omitted); view != nil {
					out[i] = view
					found = true
				}
			}
		}
		if !found {
			return nil
		}
		return out
	}
	return nil
}

// canonicalNumbers returns a copy of a decoded document in which every number is spelled in one
// canonical form, so two numbers compare equal when they have the same value (1.0, 1 and 1e0).
func canonicalNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		return json.Number(canonicalNumber(string(x)))
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			out[k] = canonicalNumbers(child)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = canonicalNumbers(child)
		}
		return out
	}
	return v
}

// canonicalNumber spells a JSON number as its significant digits and a power of ten, with no
// leading or trailing zeros ("1.0" and "1e0" are both "1e0", and every zero is "0"). It works on the
// text, so a number larger than a float64 holds exactly keeps every digit, and two different values
// never meet.
func canonicalNumber(s string) string {
	body := strings.TrimPrefix(s, "-")
	expText := ""
	if i := strings.IndexAny(body, "eE"); i >= 0 {
		expText, body = body[i+1:], body[:i]
	}
	intPart, frac, _ := strings.Cut(body, ".")
	digits := intPart + frac
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return s
	}
	trimmed := strings.TrimLeft(digits, "0")
	significant := strings.TrimRight(trimmed, "0")
	if significant == "" {
		return "0" // zero is zero, whatever its exponent
	}
	expNeg := strings.HasPrefix(expText, "-")
	expDigits := strings.TrimLeft(strings.TrimLeft(expText, "+-"), "0") // leading zeros carry no value
	if strings.Trim(expDigits, "0123456789") != "" {
		return s
	}
	exp := addToExponent(expNeg, expDigits, int64(len(trimmed)-len(significant)-len(frac)))
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
	}
	return sign + significant + "e" + exp
}

// addToExponent adds k to the exponent held as a sign and a digit string with no leading zeros, and
// returns it in the same canonical form. It works on the digits, so an exponent of any length
// compares by value in time linear in its length, and |k| is never larger than the number's own
// digit count.
func addToExponent(neg bool, digits string, k int64) string {
	if len(digits) <= 18 { // fits an int64 with room for k
		n, _ := strconv.ParseInt(digits, 10, 64)
		if neg {
			n = -n
		}
		return strconv.FormatInt(n+k, 10)
	}
	// More than 18 digits: the magnitude exceeds |k|, so the sign cannot change.
	delta := k
	if neg {
		delta = -k // the magnitude moves the other way
	}
	b := []byte(digits)
	carry := 0
	if delta >= 0 {
		add := uint64(delta)
		for i := len(b) - 1; i >= 0 && (add > 0 || carry > 0); i-- {
			d := int(b[i]-'0') + int(add%10) + carry
			add /= 10
			b[i], carry = byte('0'+d%10), d/10
		}
		if carry > 0 {
			b = append([]byte{'1'}, b...)
		}
	} else {
		sub := uint64(-delta)
		for i := len(b) - 1; i >= 0 && (sub > 0 || carry > 0); i-- {
			d := int(b[i]-'0') - int(sub%10) - carry
			sub /= 10
			carry = 0
			if d < 0 {
				d, carry = d+10, 1
			}
			b[i] = byte('0' + d)
		}
		b = []byte(strings.TrimLeft(string(b), "0")) // a borrow can leave a leading zero
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func isContainer(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

// withoutSecrets returns a copy of a decoded document without the values held under secret keys,
// whatever they hold: a secret the client cleared ("" or null) is as much a secret as one it kept.
func withoutSecrets(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			if secretKey(k) && !isContainer(child) {
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
func restoreDeviceSecrets(incoming *database.Device, stored *database.Device) error {
	var old database.Device
	if stored != nil {
		old = *stored
	}
	var err error
	if incoming.Settings, err = restoreMaskedText(incoming.Settings, old.Settings); err != nil {
		return err
	}
	if incoming.Overrides, err = restoreMaskedText(incoming.Overrides, old.Overrides); err != nil {
		return err
	}
	incoming.DesiredConfig, err = restoreMaskedText(incoming.DesiredConfig, old.DesiredConfig)
	return err
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

package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// The walker masks a decoded JSON document for a read and takes the saved secrets back from the stored
// document when a client sends the placeholder back. Device texts and notification channel configs both
// use it, so a rule fixed here is fixed for both.
//
// A key is a secret by SecretKey. The value under it is masked whole, whatever its JSON type: a string or
// a number becomes the placeholder, and so does an object or a list, with nothing below it walked. An empty
// value stays empty. A caller adds names that are secret at the document root only (Slots).

// Slots are names that hold a credential at the document root and nowhere else, compared without regard to
// case. A notification channel adds url, webhook_url and headers: a webhook URL carries its token in the
// path, the query or the userinfo, and a header such as Authorization does not look like a secret.
type Slots []string

func (s Slots) has(name string) bool {
	for _, n := range s {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

// ErrRepeatedMember is returned for a text in which a member name appears twice.
var ErrRepeatedMember = errors.New("repeats a member name: send each name once")

// ErrAmbiguousEntry is returned when a list entry that needs a stored secret (it sends the placeholder, or
// leaves the secret out) matches more than one stored entry whose secrets differ. Guessing would give it
// another entry's secret, so the request is refused and asks for the value.
var ErrAmbiguousEntry = errors.New("an array entry matches more than one stored entry, so its saved secret cannot be told apart: send the secret value instead of the mask or leaving it out")

// FoldName returns a name folded the way encoding/json folds it when it matches a JSON member to a struct
// field: every rune becomes the smallest rune of its unicode.SimpleFold set, so two names fold together
// exactly when strings.EqualFold says they are equal. (U+0131 and U+0130 fold to nothing else, so "id" and
// "ıd" stay two names.)
func FoldName(s string) string {
	var b strings.Builder
	for _, r := range s {
		for {
			next := unicode.SimpleFold(r)
			if next <= r {
				r = next
				break
			}
			r = next
		}
		b.WriteRune(r)
	}
	return b.String()
}

// SecretKey is IsKey on the folded name, so a key the decoder reads as a secret name (U+017F for s, U+212A
// for k) is one here too.
func SecretKey(name string) bool { return IsKey(FoldName(name)) }

func isSecretSlot(k string, root bool, slots Slots) bool {
	return SecretKey(k) || (root && slots.has(k))
}

// DecodeJSON decodes one JSON document, keeping numbers as written. It reports false for text that is not
// one document (an error, or data after it).
func DecodeJSON(raw []byte) (any, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, false // trailing data
	}
	return doc, true
}

// EncodeJSON encodes a decoded document without a trailing newline and without HTML escaping, so a URL
// keeps its & and <.
func EncodeJSON(doc any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func isContainer(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

func isPlaceholder(v any) bool {
	s, isString := v.(string)
	return isString && s == Placeholder
}

// holdsValue reports whether v is worth masking: a non-empty scalar, or an object or list with something
// in it. An empty secret stays empty, and a number is a secret too ({"api_key": 12345678}).
func holdsValue(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		return len(x) > 0
	case []any:
		return len(x) > 0
	}
	return IsScalar(v)
}

// MaskTree replaces, in place, the value under every secret key of a decoded document with the
// placeholder. root names the extra keys that are secret at the document root.
func MaskTree(doc any, root Slots) { maskNode(doc, root, true) }

func maskNode(v any, slots Slots, top bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			switch {
			case isSecretSlot(k, top, slots):
				if holdsValue(child) {
					x[k] = Placeholder
				}
			case isContainer(child):
				maskNode(child, slots, false)
			}
		}
	case []any:
		for _, e := range x {
			maskNode(e, slots, false)
		}
	}
}

// lookupFold returns the value stored under k, matching the key without regard to case. Two stored keys
// that fold together ({"URL":..., "url":...}) are ambiguous, because decoding picks one by position and a
// map has none: nothing is returned, so the placeholder is dropped and the client must send a value.
func lookupFold(m map[string]any, k string) any {
	var found any
	n := 0
	for key, v := range m {
		if strings.EqualFold(key, k) {
			found = v
			n++
		}
	}
	if n != 1 {
		return nil
	}
	return found
}

// MatchKey returns the key of m that folds to k, matching without regard to case, as lookupFold does.
func MatchKey(m map[string]any, k string) (string, bool) {
	for key := range m {
		if strings.EqualFold(key, k) {
			return key, true
		}
	}
	return "", false
}

// FoldCount returns how many keys of m fold to k. More than one means a lookup by k is ambiguous.
func FoldCount(m map[string]any, k string) int {
	n := 0
	for key := range m {
		if strings.EqualFold(key, k) {
			n++
		}
	}
	return n
}

// savedSecret returns the stored value a placeholder under key k takes back, and false when there is none:
// nothing stored, an ambiguous key, an empty value, or a stored placeholder (which is never copied back).
func savedSecret(old map[string]any, k string) (any, bool) {
	prior := lookupFold(old, k)
	return prior, holdsValue(prior) && !isPlaceholder(prior)
}

// RestoreTree answers a client that sends back what MaskTree gave it, in place. A secret key whose value
// arrives as the placeholder gets the stored value at the same key, or loses the key when none is stored:
// the placeholder is never stored in its place. Any other value under a secret key is a new credential and
// stays as sent, whatever its type. A credential the client leaves out stays out.
//
// An entry of a list gets the secret of the stored entry at its position only when the two agree on
// everything but their secrets. A reordered or removed entry would otherwise take another entry's secret,
// so its placeholder is dropped instead. It returns ErrAmbiguousEntry when an entry matches more than one
// stored entry and those disagree at a secret the entry gets back.
func RestoreTree(doc, stored any, root Slots) error {
	return restoreNode(doc, stored, root, true, false)
}

// RestoreTreeInner is RestoreTree for a caller whose clients may still hold a container read member by member:
// a container under a secret key that is not the placeholder is walked, and each placeholder inside it takes
// the stored member or is dropped. RestoreTree leaves such a container as sent, so a replacement is literal.
func RestoreTreeInner(doc, stored any, root Slots) error {
	return restoreNode(doc, stored, root, true, true)
}

func restoreNode(v, stored any, slots Slots, top, inner bool) error {
	switch x := v.(type) {
	case map[string]any:
		old, _ := stored.(map[string]any)
		for k, child := range x {
			switch {
			case isSecretSlot(k, top, slots):
				switch {
				case isPlaceholder(child):
					if prior, ok := savedSecret(old, k); ok {
						x[k] = prior
					} else {
						delete(x, k)
					}
				case inner && isContainer(child):
					// A container read before it was masked whole still holds masked members.
					if err := restoreNode(child, lookupFold(old, k), slots, false, inner); err != nil {
						return err
					}
				}
			case isContainer(child):
				if err := restoreNode(child, lookupFold(old, k), slots, false, inner); err != nil {
					return err
				}
			}
		}
	case []any:
		old, _ := stored.([]any)
		for i, child := range x {
			var counterpart any
			if i < len(old) && SameWithoutSecrets(child, old[i]) {
				if AmbiguousEntry(child, old, false) {
					return ErrAmbiguousEntry
				}
				counterpart = old[i]
			}
			if err := restoreNode(child, counterpart, slots, false, inner); err != nil {
				return err
			}
		}
	}
	return nil
}

// HasRepeatedMember reports whether any object in raw holds two members whose names fold together (the way
// encoding/json matches them), and also for a text it cannot read on. encoding/json reads a repeated member
// in text order (a later null is ignored, names fold together), while the generic decode keeps the last
// value and loses the order, so no restore of a placeholder can promise to keep what a reader of the stored
// text sees: the client sends each name once.
func HasRepeatedMember(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // a number such as 1e1000 must not end the scan
	var walk func() (repeats, ok bool)
	walk = func() (bool, bool) {
		tok, err := dec.Token()
		if err != nil {
			return false, false
		}
		delim, isDelim := tok.(json.Delim)
		if !isDelim {
			return false, true // a scalar
		}
		seen := map[string]bool{}
		for dec.More() {
			if delim == '{' {
				nameTok, nameErr := dec.Token()
				name, isName := nameTok.(string)
				if nameErr != nil || !isName {
					return false, false
				}
				folded := FoldName(name)
				if seen[folded] {
					return true, true
				}
				seen[folded] = true
			}
			if repeats, ok := walk(); repeats || !ok {
				return repeats, ok
			}
		}
		_, _ = dec.Token() // the closing delimiter
		return false, true
	}
	repeats, ok := walk()
	return repeats || !ok
}

// SameWithoutSecrets reports whether two decoded documents agree on everything but the values under their
// secret keys and the spelling of their numbers.
func SameWithoutSecrets(a, b any) bool {
	return encodeForCompare(canonicalNumbers(withoutSecrets(a))) == encodeForCompare(canonicalNumbers(withoutSecrets(b)))
}

func encodeForCompare(doc any) string {
	out, err := EncodeJSON(doc)
	if err != nil {
		return Placeholder
	}
	return string(out)
}

// withoutSecrets returns a copy of a decoded document without the values held under secret keys, whatever
// they hold: a secret the client cleared ("" or null) is as much a secret as one it kept.
func withoutSecrets(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			if SecretKey(k) {
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

// AmbiguousEntry reports whether an entry matches more than one stored entry (they agree on everything but
// their secrets) and those entries disagree at a secret the entry gets back from them. Entries that differ
// only in a secret the entry does not restore (it sends a new value) are not ambiguous. omitted says
// whether a secret the entry leaves out counts as restored, as in the typed raw part, or only a placeholder
// does.
func AmbiguousEntry(entry any, stored []any, omitted bool) bool {
	var first string
	matches := 0
	for _, s := range stored {
		if !SameWithoutSecrets(entry, s) {
			continue
		}
		view := encodeForCompare(canonicalNumbers(restoredView(entry, s, omitted)))
		if matches == 0 {
			first = view
		} else if view != first {
			return true
		}
		matches++
	}
	return false
}

// restoredView returns what stored would give back to v: the saved secret under each key where v sends the
// placeholder and, when omitted is set, under each key it leaves out. It is nil when nothing is restored. A
// placeholder no stored entry holds a secret for is dropped, so it gives back nothing.
func restoredView(v, stored any, omitted bool) any {
	switch x := v.(type) {
	case map[string]any:
		old, _ := stored.(map[string]any)
		out := map[string]any{}
		for k, child := range x {
			if isPlaceholder(child) && SecretKey(k) {
				if prior, ok := savedSecret(old, k); ok {
					out[k] = prior
				}
				continue
			}
			if view := restoredView(child, lookupFold(old, k), omitted); view != nil {
				out[k] = view
			}
		}
		if omitted {
			for k, sv := range old {
				if _, present := x[k]; !present && HoldsSecret(k, sv) {
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
			if i < len(old) && SameWithoutSecrets(child, old[i]) {
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

// HoldsSecret reports whether a value under key is a secret or contains one. The placeholder is never a
// saved secret: a stored placeholder is dropped, not copied back.
func HoldsSecret(key string, v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if HoldsSecret(k, child) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if HoldsSecret("", child) {
				return true
			}
		}
	}
	return SecretKey(key) && holdsValue(v) && !isPlaceholder(v)
}

// canonicalNumbers returns a copy of a decoded document in which every number is spelled in one canonical
// form, so two numbers compare equal when they have the same value (1.0, 1 and 1e0).
func canonicalNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		return json.Number(CanonicalNumber(string(x)))
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

// CanonicalNumber spells a JSON number as its significant digits and a power of ten, with no leading or
// trailing zeros ("1.0" and "1e0" are both "1e0", and every zero is "0"). It works on the text, so a number
// larger than a float64 holds exactly keeps every digit, and two different values never meet.
func CanonicalNumber(s string) string {
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

// addToExponent adds k to the exponent held as a sign and a digit string with no leading zeros, and returns
// it in the same canonical form. It works on the digits, so an exponent of any length compares by value in
// time linear in its length, and |k| is never larger than the number's own digit count.
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

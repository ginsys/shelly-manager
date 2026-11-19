package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/ginsys/shelly-manager/internal/redact"
)

// A channel's config holds the credentials for its destination, so a read sends the placeholder in
// their place and a save that sends the placeholder back keeps the stored value. The key rule is the
// shared one (redact.IsKey: secret, token, password, ... or a name ending in key). A channel adds
// three slots that rule cannot see: the top-level url and webhook_url, because a webhook URL carries
// its token in the path, the query or the userinfo (a Slack or Discord webhook is the token), and every
// value under headers, because Authorization and custom auth headers do not look like secrets.
// Keys are compared without regard to case, because encoding/json accepts "URL" and "Headers" for the
// fields validation and delivery read.
//
// A key that holds an object or a list is secret as a whole, so every scalar below it is a credential.
// secret says that an enclosing key already made the node secret; top says the object is the root.

func isURLKey(k string) bool {
	return strings.EqualFold(k, "url") || strings.EqualFold(k, "webhook_url")
}

// secretName is redact.IsKey on a key folded the way encoding/json folds field names: it reads the
// long s (U+017F) as s and the Kelvin sign (U+212A) as k, which a plain lower-casing does not.
func secretName(k string) bool {
	return redact.IsKey(strings.ToLower(strings.ToUpper(k)))
}

// holdsAll reports whether every scalar below the entry (k, container) is a credential.
func holdsAll(k string, top, secret bool) bool {
	return secret || secretName(k) || (top && strings.EqualFold(k, "headers"))
}

// isSlot reports whether the scalar entry k holds a credential.
func isSlot(k string, top, secret bool) bool {
	return secret || secretName(k) || (top && isURLKey(k))
}

func isContainer(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

func decodeDocument(raw []byte) (any, bool) {
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

func encodeDocument(doc any) json.RawMessage {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // a URL keeps its & and <
	if err := enc.Encode(doc); err != nil {
		return quotedPlaceholder()
	}
	return json.RawMessage(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
}

func quotedPlaceholder() json.RawMessage {
	quoted, _ := json.Marshal(redact.Placeholder)
	return quoted
}

// maskNode replaces, in place, every credential scalar below v with the placeholder. An empty secret
// stays empty, and a number is a secret too ({"api_key": 12345678}).
func maskNode(v any, top, secret bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if isContainer(child) {
				maskNode(child, false, holdsAll(k, top, secret))
			} else if isSlot(k, top, secret) && redact.IsScalar(child) {
				x[k] = redact.Placeholder
			}
		}
	case []any:
		for i, e := range x {
			if isContainer(e) {
				maskNode(e, false, secret)
			} else if secret && redact.IsScalar(e) {
				x[i] = redact.Placeholder
			}
		}
	}
}

// maskConfig returns a copy of a stored config that is safe to send out. A config that is not valid
// JSON cannot be walked, so it is masked whole: a malformed text never leaks.
func maskConfig(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return raw
	}
	doc, ok := decodeDocument(trimmed)
	if !ok {
		return quotedPlaceholder()
	}
	maskNode(doc, true, false)
	return encodeDocument(doc)
}

// lookupFold returns the value stored under k, matching the key without regard to case. Two stored keys
// that fold together ({"URL":..., "url":...}) are ambiguous, because decoding picks one by position and
// a map has none: nothing is returned, so the placeholder is dropped and the client must send a value.
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

// kept reports whether stored is a credential that can stand in for a placeholder.
func kept(stored any) bool {
	if s, isString := stored.(string); isString && s == redact.Placeholder {
		return false
	}
	return redact.IsScalar(stored)
}

// restoreNode answers a client that sends back what maskNode gave it. A credential that arrives as the
// placeholder becomes the stored value at the same key, or is dropped when none is stored: the placeholder
// is never stored. Any other value is a new credential and stays as sent. A credential the client leaves
// out stays out, so a save replaces the config as it always did.
//
// A list has no key to pair its entries by, and pairing by position would give an entry another entry's
// credential after a reorder. So a secret list comes back whole only when every entry is the placeholder
// and the stored list has as many entries; any other placeholder in a list is dropped.
func restoreNode(v, stored any, top, secret bool) any {
	switch x := v.(type) {
	case map[string]any:
		old, _ := stored.(map[string]any)
		for k, child := range x {
			if isContainer(child) {
				x[k] = restoreNode(child, lookupFold(old, k), false, holdsAll(k, top, secret))
				continue
			}
			if s, isString := child.(string); isString && s == redact.Placeholder && isSlot(k, top, secret) {
				if prior := lookupFold(old, k); kept(prior) {
					x[k] = prior
				} else {
					delete(x, k)
				}
			}
		}
		return x
	case []any:
		if secret {
			if whole, ok := restoreWholeList(x, stored); ok {
				return whole
			}
		}
		out := make([]any, 0, len(x))
		for _, e := range x {
			if s, isString := e.(string); isString && s == redact.Placeholder && secret {
				continue
			}
			out = append(out, restoreNode(e, nil, false, secret))
		}
		return out
	}
	return v
}

func restoreWholeList(incoming []any, stored any) ([]any, bool) {
	old, isList := stored.([]any)
	if !isList || len(old) != len(incoming) || len(incoming) == 0 {
		return nil, false
	}
	for i, e := range incoming {
		if s, isString := e.(string); !isString || s != redact.Placeholder || !kept(old[i]) {
			return nil, false
		}
	}
	return old, true
}

// errRepeatedMember refuses a config in which a member name appears twice. encoding/json reads such a
// config in text order (a later null is ignored, names fold together), the generic decode used here
// keeps the last value and loses the order, so no restore of a placeholder can promise to keep the
// destination delivery reads. The client sends each name once.
var errRepeatedMember = errors.New("channel config repeats a member name: send each name once")

// restoreConfig takes the stored credential back for each placeholder in a config a client sends. It
// refuses a config that repeats a member name.
func restoreConfig(incoming, stored json.RawMessage) (json.RawMessage, error) {
	// The text is decoded before anything is decided: a JSON escape of a star is the placeholder too.
	doc, ok := decodeDocument(bytes.TrimSpace(incoming))
	if !ok {
		return incoming, nil // not ours to read: validation decides
	}
	if s, isString := doc.(string); isString && s == redact.Placeholder {
		return stored, nil // the whole malformed text, masked
	}
	m, isMap := doc.(map[string]any)
	if !isMap {
		return incoming, nil
	}
	if repeatsMember(bytes.TrimSpace(incoming)) {
		return nil, errRepeatedMember
	}
	var old any
	if storedDoc, storedOK := decodeDocument(bytes.TrimSpace(stored)); storedOK {
		old = storedDoc
	}
	before := encodeDocument(m)
	restored := encodeDocument(restoreNode(m, old, true, false))
	if bytes.Equal(before, restored) {
		return incoming, nil // nothing to restore: keep the client's own bytes
	}
	return restored, nil
}

// repeatsMember reports whether any object in raw holds two members whose names fold together (the
// way encoding/json matches them). raw has been decoded already, so a text this cannot read on is
// refused as well.
func repeatsMember(raw []byte) bool {
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
				folded := strings.ToLower(strings.ToUpper(name))
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

// maskedChannel returns a copy of c that is safe to send out. The stored record is never changed:
// delivery reads the real values from it.
func maskedChannel(c NotificationChannel) NotificationChannel {
	c.Config = maskConfig(c.Config)
	return c
}

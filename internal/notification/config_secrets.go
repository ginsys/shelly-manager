package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"

	"github.com/ginsys/shelly-manager/internal/redact"
)

// A channel's config holds the credentials for its destination, so a read sends the placeholder in
// their place and a save that sends the placeholder back keeps the stored value. The key rule is the
// shared one (redact.IsKey: secret, token, password, ... or a name ending in key). A channel adds
// three slots that rule cannot see: the top-level url and webhook_url, because a webhook URL carries
// its token in the path, the query or the userinfo (a Slack or Discord webhook is the token), and
// headers, because Authorization and custom auth headers do not look like secrets.
// Keys are compared without regard to case, because encoding/json accepts "URL" and "Headers" for the
// fields validation and delivery read.
//
// The value under such a key is masked whole, whatever its JSON type: a string or a number becomes the
// placeholder, and so does an object or a list (headers, a list of tokens). Nothing below a secret key
// is walked, so no shape under it can keep a credential in view.

// secretKey reports whether the value under key k is a credential. top says the object is the root.
func secretKey(k string, top bool) bool {
	return redact.IsKey(foldName(k)) ||
		(top && (strings.EqualFold(k, "url") || strings.EqualFold(k, "webhook_url") || strings.EqualFold(k, "headers")))
}

// holdsValue reports whether v is a value worth masking: a non-empty scalar, or an object or list
// with something in it. An empty secret stays empty, and a number is a secret too ({"api_key": 12345678}).
func holdsValue(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		return len(x) > 0
	case []any:
		return len(x) > 0
	}
	return redact.IsScalar(v)
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

// maskNode replaces, in place, the value under every secret key below v with the placeholder.
func maskNode(v any, top bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			switch {
			case secretKey(k, top):
				if holdsValue(child) {
					x[k] = redact.Placeholder
				}
			case isContainer(child):
				maskNode(child, false)
			}
		}
	case []any:
		for _, e := range x {
			maskNode(e, false)
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
	maskNode(doc, true)
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

func isPlaceholder(v any) bool {
	s, isString := v.(string)
	return isString && s == redact.Placeholder
}

// restoreNode answers a client that sends back what maskNode gave it. A secret key whose value arrives
// as the placeholder gets the stored value at the same key, or loses the key when none is stored: the
// placeholder is never stored in its place. Any other value under a secret key is a new credential and
// stays as sent, whatever its type. A credential the client leaves out stays out, so a save replaces the
// config as it always did.
//
// A list has no key to pair its entries by, so the objects in a list are restored against nothing: a
// placeholder inside them is dropped.
func restoreNode(v, stored any, top bool) {
	switch x := v.(type) {
	case map[string]any:
		old, _ := stored.(map[string]any)
		for k, child := range x {
			switch {
			case secretKey(k, top):
				if isPlaceholder(child) {
					if prior := lookupFold(old, k); holdsValue(prior) && !isPlaceholder(prior) {
						x[k] = prior
					} else {
						delete(x, k)
					}
				}
			case isContainer(child):
				restoreNode(child, lookupFold(old, k), false)
			}
		}
	case []any:
		for _, e := range x {
			restoreNode(e, nil, false)
		}
	}
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
	if isPlaceholder(doc) {
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
	restoreNode(m, old, true)
	restored := encodeDocument(m)
	if bytes.Equal(before, restored) {
		return incoming, nil // nothing to restore: keep the client's own bytes
	}
	return restored, nil
}

// foldName returns a name folded the way encoding/json folds it when it matches a JSON member to a
// struct field: every rune becomes the smallest rune of its unicode.SimpleFold set, so two names fold
// together exactly when strings.EqualFold says they are equal. (U+0131 and U+0130 fold to nothing
// else, so "id" and "ıd" stay two names.)
func foldName(s string) string {
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
				folded := foldName(name)
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

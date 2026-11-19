package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strconv"
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

// restoreConfig takes the stored credential back for each placeholder in a config a client sends.
func restoreConfig(incoming, stored json.RawMessage) json.RawMessage {
	// The text is decoded before anything is decided: a JSON escape of a star is the placeholder too.
	doc, ok := decodeDocument(bytes.TrimSpace(incoming))
	if !ok {
		return incoming // not ours to read: validation decides
	}
	if s, isString := doc.(string); isString && s == redact.Placeholder {
		return stored // the whole malformed text, masked
	}
	m, isMap := doc.(map[string]any)
	if !isMap {
		return incoming
	}
	var old any
	if storedDoc, storedOK := decodeDocument(bytes.TrimSpace(stored)); storedOK {
		old = storedDoc
	}
	// A repeated member is read one way by the generic decode above (the last wins) and another by the
	// struct decode that validation and delivery use (in text order, a null is ignored), and re-encoding
	// sorts the members. So where a member repeats and the text carries the placeholder, there is no
	// faithful restore: the config is refused (a string is no channel config). Without the placeholder
	// there is nothing to restore, and the client's own bytes keep the order delivery reads.
	repeats, hasPlaceholder := scanMembers(bytes.TrimSpace(incoming))
	if repeats && hasPlaceholder {
		return quotedPlaceholder()
	}
	before := encodeDocument(m)
	restored := encodeDocument(restoreNode(m, old, true, false))
	if bytes.Equal(before, restored) {
		return incoming // nothing to restore: keep the client's own bytes
	}
	return restored
}

// scanMembers reports whether any object in raw holds two members whose names fold together, and
// whether any string in it is the placeholder.
func scanMembers(raw []byte) (repeats, placeholder bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() bool // false once the text cannot be read on
	walk = func() bool {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch v := tok.(type) {
		case string:
			placeholder = placeholder || v == redact.Placeholder
		case json.Delim:
			switch v {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					nameTok, nameErr := dec.Token()
					name, isName := nameTok.(string)
					if nameErr != nil || !isName {
						return false
					}
					folded := strings.ToLower(strings.ToUpper(name))
					repeats = repeats || seen[folded]
					seen[folded] = true
					if !walk() {
						return false
					}
				}
			case '[':
				for dec.More() {
					if !walk() {
						return false
					}
				}
			}
			_, _ = dec.Token() // the closing delimiter
		}
		return true
	}
	walk()
	return repeats, placeholder
}

// maskedChannel returns a copy of c that is safe to send out. The stored record is never changed:
// delivery reads the real values from it.
func maskedChannel(c NotificationChannel) NotificationChannel {
	c.Config = maskConfig(c.Config)
	return c
}

// collectSecrets appends every credential string below v.
func collectSecrets(v any, top, secret bool, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if isContainer(child) {
				collectSecrets(child, false, holdsAll(k, top, secret), out)
			} else if s, isString := child.(string); isString && isSlot(k, top, secret) {
				*out = append(*out, s)
			}
		}
	case []any:
		for _, e := range x {
			if s, isString := e.(string); isString && secret {
				*out = append(*out, s)
			} else if isContainer(e) {
				collectSecrets(e, false, secret, out)
			}
		}
	}
}

// quotedURLError matches the part of a *url.Error that holds the request URL: `Post "<url>": `, where
// the URL is written with %q. net/http spells that URL its own way (percent-encoded, the password
// written ***, an empty port dropped), so the stored value cannot be matched against it, and the shape
// is cut out whatever the spelling. The Op is the request method, or "parse" for a URL that does not parse.
//
// The Op is any HTTP method token (RFC 9110: letters, digits and !#$%&'*+-.^_`|~), because the method is
// configurable and Go accepts every token.
//
// The match ends at the colon and leaves the space after it, because nested errors share that space
// (`failed to parse Location header "<url>": parse "<url>": ...`).
var quotedURLError = regexp.MustCompile("(?:^|\\s)[!#$%&'*+.^_`|~0-9A-Za-z-]+ \"(?:[^\"\\\\]|\\\\.)*\":")

// spellings lists how an error can write the credential s: as stored, and with %q (a quote, a backslash
// or a control character escaped), for the text a *url.Error does not hold.
func spellings(s string) []string {
	quoted := strconv.Quote(s)
	return []string{s, quoted[1 : len(quoted)-1]}
}

// scrubText replaces, in text, every credential held by the channel config with the placeholder. A
// failed send quotes its destination, and the text is stored with the history row and returned by a
// test send, so the request URL of a *url.Error and the config's own values are cut out of it.
func scrubText(text string, config json.RawMessage) string {
	doc, ok := decodeDocument(bytes.TrimSpace(config))
	if !ok || text == "" {
		return text
	}
	var secrets []string
	collectSecrets(doc, true, false, &secrets)
	seen := map[string]bool{}
	var forms []string
	for _, s := range secrets {
		if s == "" || s == redact.Placeholder {
			continue
		}
		for _, f := range spellings(s) {
			if !seen[f] {
				seen[f] = true
				forms = append(forms, f)
			}
		}
	}
	sort.Slice(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) }) // a longer value first
	text = quotedURLError.ReplaceAllStringFunc(text, func(m string) string {
		return m[:strings.IndexByte(m, '"')] + `"` + redact.Placeholder + `":`
	})
	if len(forms) == 0 {
		return text
	}
	// One pass over the text: a placeholder already written is never searched again.
	pairs := make([]string, 0, 2*len(forms))
	for _, f := range forms {
		pairs = append(pairs, f, redact.Placeholder)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// scrubError returns err with the channel's credentials cut out of its text; an error that holds none
// is returned as it is.
func scrubError(err error, config json.RawMessage) error {
	if err == nil {
		return nil
	}
	if text := scrubText(err.Error(), config); text != err.Error() {
		return errors.New(text)
	}
	return err
}

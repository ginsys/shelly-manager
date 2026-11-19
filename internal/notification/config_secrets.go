package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/ginsys/shelly-manager/internal/redact"
)

// A channel's config holds the credentials for its destination, so a read sends the placeholder in
// their place and a save that sends the placeholder back keeps the stored value. The key rule is the
// shared one (redact.IsKey: secret, token, password, ... or a name ending in key). A channel adds
// three slots that rule cannot see: the top-level url and webhook_url, because a webhook URL carries
// its token in the path, the query or the userinfo (a Slack or Discord webhook is the token), and every
// value under headers, because Authorization and custom auth headers do not look like secrets.
// A channel config is a flat object with one headers map, so the walk goes through objects only.

// isSlot reports whether the entry key holds a credential. top says the entry sits at the top level,
// all says an enclosing key already made everything below it a credential.
func isSlot(key string, top, all bool) bool {
	return all || redact.IsKey(key) || (top && (key == "url" || key == "webhook_url"))
}

// eachSlot calls visit for every scalar entry of the object m that holds a credential, with the keys
// that lead to it. visit may set or delete parent[key].
func eachSlot(m map[string]any, path []string, all bool, visit func(path []string, parent map[string]any, key string)) {
	top := len(path) == 0
	for k, v := range m {
		if child, ok := v.(map[string]any); ok {
			eachSlot(child, append(path[:len(path):len(path)], k), all || redact.IsKey(k) || (top && k == "headers"), visit)
			continue
		}
		if isSlot(k, top, all) {
			visit(path, m, k)
		}
	}
}

func decodeObject(raw []byte) (any, bool) {
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

func encodeObject(doc any) json.RawMessage {
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

// maskConfig returns a copy of a stored config that is safe to send out. A config that is not valid
// JSON cannot be walked, so it is masked whole: a malformed text never leaks.
func maskConfig(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return raw
	}
	doc, ok := decodeObject(trimmed)
	if !ok {
		return quotedPlaceholder()
	}
	redact.Mask(doc, redact.Placeholder)
	if m, isMap := doc.(map[string]any); isMap {
		eachSlot(m, nil, false, func(_ []string, parent map[string]any, key string) {
			if redact.IsScalar(parent[key]) {
				parent[key] = redact.Placeholder
			}
		})
	}
	return encodeObject(doc)
}

// restoreConfig answers a client that sends back what maskConfig gave it. A credential that arrives
// as the placeholder becomes the stored value, or is dropped when none is stored: the placeholder is
// never stored. Any other value is a new credential and stays as sent. A credential the client leaves
// out stays out, so a save replaces the config as it always did.
func restoreConfig(incoming, stored json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(incoming)
	if bytes.Equal(trimmed, quotedPlaceholder()) {
		return stored // the whole malformed text, masked
	}
	doc, ok := decodeObject(trimmed)
	m, isMap := doc.(map[string]any)
	if !ok || !isMap {
		return incoming // not ours to read: validation decides
	}
	var old map[string]any
	if storedDoc, storedOK := decodeObject(bytes.TrimSpace(stored)); storedOK {
		old, _ = storedDoc.(map[string]any)
	}
	changed := false
	eachSlot(m, nil, false, func(path []string, parent map[string]any, key string) {
		if s, isString := parent[key].(string); !isString || s != redact.Placeholder {
			return
		}
		changed = true
		if kept, found := lookup(old, path, key); found && redact.IsScalar(kept) && kept != redact.Placeholder {
			parent[key] = kept
		} else {
			delete(parent, key)
		}
	})
	if !changed {
		return incoming
	}
	return encodeObject(m)
}

func lookup(root map[string]any, path []string, key string) (any, bool) {
	cur := root
	for _, step := range path {
		next, ok := cur[step].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = next
	}
	v, ok := cur[key]
	return v, ok
}

// maskedChannel returns a copy of c that is safe to send out. The stored record is never changed:
// delivery reads the real values from it.
func maskedChannel(c NotificationChannel) NotificationChannel {
	c.Config = maskConfig(c.Config)
	return c
}

// scrubText replaces, in text, every credential held by the channel config with the placeholder. A
// failed send quotes its destination (a *url.Error carries the URL), and the text is stored with the
// history row and returned by a test send, so the config's own values are cut out of it. A value that
// parses as a URL with a password is also matched in the spelling net/http writes, where the password
// is "***": that string is not the stored value.
func scrubText(text string, config json.RawMessage) string {
	doc, ok := decodeObject(bytes.TrimSpace(config))
	m, isMap := doc.(map[string]any)
	if !ok || !isMap || text == "" {
		return text
	}
	var secrets []string
	eachSlot(m, nil, false, func(_ []string, parent map[string]any, key string) {
		s, isString := parent[key].(string)
		if !isString || s == "" {
			return
		}
		secrets = append(secrets, s)
		if u, err := url.Parse(s); err == nil && u.User != nil {
			if _, set := u.User.Password(); set {
				secrets = append(secrets, strings.Replace(u.String(), u.User.String()+"@", u.User.Username()+":***@", 1))
			}
		}
	})
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) }) // a longer value first
	for _, s := range secrets {
		text = strings.ReplaceAll(text, s, redact.Placeholder)
	}
	return text
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

package notification

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ginsys/shelly-manager/internal/redact"
)

// A channel's config holds the credentials for its destination, so a read sends the placeholder in
// their place and a save that sends the placeholder back keeps the stored value. The walk is the shared
// one (redact.MaskTree and redact.RestoreTree): the key rule is redact.SecretKey (secret, token,
// password, ... or a name ending in key, matched the way encoding/json matches a member name). A channel
// adds three slots that rule cannot see: the top-level url and webhook_url, because a webhook URL carries
// its token in the path, the query or the userinfo (a Slack or Discord webhook is the token), and
// headers, because Authorization and custom auth headers do not look like secrets.
// Keys are compared without regard to case, because encoding/json accepts "URL" and "Headers" for the
// fields validation and delivery read.
//
// The value under such a key is masked whole, whatever its JSON type: a string or a number becomes the
// placeholder, and so does an object or a list (headers, a list of tokens). Nothing below a secret key
// is walked, so no shape under it can keep a credential in view.
var channelSlots = redact.Slots{"url", "webhook_url", "headers"}

// errRepeatedMember refuses a config in which a member name appears twice. encoding/json reads such a
// config in text order (a later null is ignored, names fold together), the generic decode used here
// keeps the last value and loses the order, so no restore of a placeholder can promise to keep the
// destination delivery reads. The client sends each name once.
var errRepeatedMember = redact.ErrRepeatedMember

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
	doc, ok := redact.DecodeJSON(trimmed)
	if !ok {
		return quotedPlaceholder()
	}
	redact.MaskTree(doc, channelSlots)
	masked, err := redact.EncodeJSON(doc)
	if err != nil {
		return quotedPlaceholder()
	}
	return masked
}

// restoreConfig takes the stored credential back for each placeholder in a config a client sends. It
// refuses a config that repeats a member name, and one whose list entry cannot be told from another
// stored entry.
func restoreConfig(incoming, stored json.RawMessage) (json.RawMessage, error) {
	// The text is decoded before anything is decided: a JSON escape of a star is the placeholder too.
	doc, ok := redact.DecodeJSON(bytes.TrimSpace(incoming))
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
	if redact.HasRepeatedMember(bytes.TrimSpace(incoming)) {
		return nil, fmt.Errorf("channel config %w", errRepeatedMember)
	}
	var old any
	if storedDoc, storedOK := redact.DecodeJSON(bytes.TrimSpace(stored)); storedOK {
		old = storedDoc
	}
	before, err := redact.EncodeJSON(m)
	if err != nil {
		return nil, err
	}
	if err = redact.RestoreTree(m, old, channelSlots); err != nil {
		return nil, fmt.Errorf("channel config: %w", err)
	}
	restored, err := redact.EncodeJSON(m)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(before, restored) {
		return incoming, nil // nothing to restore: keep the client's own bytes
	}
	return restored, nil
}

// maskedChannel returns a copy of c that is safe to send out. The stored record is never changed:
// delivery reads the real values from it.
func maskedChannel(c NotificationChannel) NotificationChannel {
	c.Config = maskConfig(c.Config)
	return c
}

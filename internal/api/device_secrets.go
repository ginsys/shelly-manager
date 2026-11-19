package api

import (
	"fmt"
	"strings"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/redact"
)

// A device carries saved secrets inside JSON text: Overrides, DesiredConfig and Settings are
// strings that hold a document, and the live configuration is the device's own raw settings. None of
// these has the DeviceConfiguration shape (the live one is the device's own: Gen1 wifi_sta.key,
// login.password, mqtt.pass), so the typed secret slots cannot reach them. These helpers walk the
// document instead and decide by the key name, with the walker the notification channels use
// (redact.MaskTree and redact.RestoreTreeInner): the key rule is redact.SecretKey, so it agrees with the rule
// secret_invariant_test.go uses for the typed configuration. A key that merely looks like a secret is
// masked on a read and restored by the stored value on a save, so a false match costs a masked display
// and nothing else.

func decodeDocument(text string) (any, bool) { return redact.DecodeJSON([]byte(text)) }

func encodeDocument(doc any) string {
	out, err := redact.EncodeJSON(doc)
	if err != nil {
		return secretMask
	}
	return string(out)
}

// maskJSONText returns text with the value under every secret-named key replaced by secretMask. Text
// that is not valid JSON cannot be walked, and guessing key names in it fails (escapes, truncation), so
// it is returned as the mask whole: a malformed text never leaks.
func maskJSONText(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	doc, ok := decodeDocument(text)
	if !ok {
		return secretMask // text we cannot parse is masked whole, whatever it holds
	}
	redact.MaskTree(doc, nil)
	return encodeDocument(doc)
}

// errAmbiguousEntry is returned when an array entry that needs a stored secret (it sends the mask,
// or leaves the secret out) matches more than one stored entry whose secrets differ. Guessing would
// give it another entry's password, so the request is refused and asks for the value.
var errAmbiguousEntry = redact.ErrAmbiguousEntry

// restoreMaskedText answers a client that sends back what maskJSONText gave it. A secret that
// arrives as the mask becomes the stored value, or is dropped when none is stored: the mask itself
// is never stored. Any other value is a new secret and stays as sent. A text that is not valid JSON
// is the client's own and stays as sent, except the mask standing for a whole malformed text (bare, or
// as a JSON string), which becomes the stored text.
//
// It returns errAmbiguousEntry when a masked secret sits in an array entry that matches more than
// one stored entry, because then no stored secret can be told to belong to it. It refuses a text that
// repeats a member name: the masked read of such a text and the stored one would disagree.
func restoreMaskedText(incoming, stored string) (string, error) {
	if incoming == secretMask {
		return stored, nil
	}
	doc, ok := decodeDocument(incoming)
	if !ok {
		return incoming, nil
	}
	if s, isString := doc.(string); isString && s == secretMask {
		return stored, nil
	}
	if redact.HasRepeatedMember([]byte(incoming)) {
		return "", fmt.Errorf("device configuration text %w", redact.ErrRepeatedMember)
	}
	old, _ := decodeDocument(stored)
	if err := redact.RestoreTreeInner(doc, old, nil); err != nil {
		return "", err
	}
	return encodeDocument(doc), nil
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

package api

import (
	"encoding/json"
	"strings"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

// The typed configuration shares its Wi-Fi, MQTT, authentication and network sections with
// DeviceConfiguration, so the secret slots of device_secrets.go and handlers_config_templates.go
// reach every typed secret through a view that points at the same sections. The typed Raw part
// holds the settings the typed model does not know, under any key, so it is walked by key name.

// typedSecretView returns a DeviceConfiguration whose sections are the typed configuration's own.
// A change through its secret slots is a change to c. A section the view creates is not in c until
// adoptTypedSections copies it back.
func typedSecretView(c *configuration.TypedConfiguration) *configuration.DeviceConfiguration {
	if c == nil {
		return nil
	}
	return &configuration.DeviceConfiguration{WiFi: c.WiFi, MQTT: c.MQTT, Auth: c.Auth, Network: c.Network}
}

// adoptTypedSections stores the sections of a view back into c.
func adoptTypedSections(c *configuration.TypedConfiguration, view *configuration.DeviceConfiguration) {
	c.WiFi, c.MQTT, c.Auth, c.Network = view.WiFi, view.MQTT, view.Auth, view.Network
}

// maskRawJSON masks the secrets in a raw JSON part. A masked malformed text is not valid JSON, so
// it is sent as a JSON string.
func maskRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	masked := maskJSONText(string(raw))
	if !json.Valid([]byte(masked)) {
		quoted, _ := json.Marshal(masked)
		masked = string(quoted)
	}
	return json.RawMessage(masked)
}

// maskTypedSecrets replaces every saved secret in c with the mask. It changes c, so it is called
// on a configuration that is about to be sent and never on the one that is stored.
func maskTypedSecrets(c *configuration.TypedConfiguration) {
	if c == nil {
		return
	}
	maskSecrets(typedSecretView(c))
	c.Raw = maskRawJSON(c.Raw)
}

// templateMessageCodes are the validation codes whose message quotes a fragment of the configuration
// text: a template expression, which may hold a saved secret or a default for one. The validator
// chooses which fragment a message quotes, so no secret value can be searched for in it.
var templateMessageCodes = map[string]bool{
	"TEMPLATE_SYNTAX_ERROR":       true,
	"DANGEROUS_TEMPLATE_FUNCTION": true,
	"ENV_VARIABLE_REFERENCE":      true,
	"CUSTOM_VARIABLE_REFERENCE":   true,
}

// scrubValidation replaces what follows the colon in each message that quotes a template fragment
// with the mask. The validator reads the real configuration, and the answer carries it masked.
func scrubValidation(result *configuration.ValidationResult) {
	if result == nil {
		return
	}
	scrub := func(code, message string) string {
		if !templateMessageCodes[code] {
			return message
		}
		if head, _, found := strings.Cut(message, ": "); found {
			return head + ": " + secretMask
		}
		return message
	}
	for i, e := range result.Errors {
		result.Errors[i].Message = scrub(e.Code, e.Message)
	}
	for i, w := range result.Warnings {
		result.Warnings[i].Message = scrub(w.Code, w.Message)
	}
	for i, n := range result.Info {
		result.Info[i].Message = scrub(n.Code, n.Message)
	}
}

// restoreTypedSecrets answers a client that sends back what maskTypedSecrets gave it, from the
// stored configuration (nil when none is stored), by the same rules as the overrides routes: a
// mask becomes the stored secret (or is dropped when there is none, since the mask is never
// stored), a secret the body leaves out keeps its stored value, an explicit "" clears it, and a
// path in removed, sent as null, is not kept.
//
// It returns errAmbiguousEntry when a raw array entry needs a stored secret and matches more than
// one stored entry.
func restoreTypedSecrets(incoming, stored *configuration.TypedConfiguration, removed map[string]bool) error {
	view := typedSecretView(incoming)
	storedView := typedSecretView(stored)
	restoreMaskedSecrets(view, storedView)
	keepOmittedSecrets(view, storedView, removed)
	adoptTypedSections(incoming, view)

	var storedRaw json.RawMessage
	if stored != nil {
		storedRaw = stored.Raw
	}
	raw, err := restoreRawJSON(incoming.Raw, storedRaw)
	if err != nil {
		return err
	}
	incoming.Raw = raw
	return nil
}

// restoreRawJSON undoes maskRawJSON: the mask standing for the whole part, sent as a JSON string,
// becomes the stored part, and a mask inside it becomes the stored secret.
func restoreRawJSON(incoming, stored json.RawMessage) (json.RawMessage, error) {
	var whole string
	if json.Unmarshal(incoming, &whole) == nil && whole == secretMask {
		return stored, nil
	}
	if len(incoming) > 0 {
		text, err := restoreMaskedText(string(incoming), string(stored))
		if err != nil {
			return nil, err
		}
		incoming = json.RawMessage(text)
	}
	return keepOmittedRawSecrets(incoming, stored)
}

// keepOmittedRawSecrets copies into incoming each stored secret that it leaves out, the rule the
// typed secrets follow. A raw part the body omits altogether is the stored one. A raw secret sent
// as "" or null is present and stays as sent, and a raw part that is null stays null.
func keepOmittedRawSecrets(incoming, stored json.RawMessage) (json.RawMessage, error) {
	if len(incoming) == 0 {
		return stored, nil
	}
	storedDoc, _ := decodeDocument(string(stored))
	doc, _ := decodeDocument(string(incoming))
	storedMap, storedIsMap := storedDoc.(map[string]any)
	docMap, docIsMap := doc.(map[string]any)
	if storedIsMap && docIsMap {
		if err := keepOmittedValue(docMap, storedMap); err != nil {
			return nil, err
		}
		return json.RawMessage(encodeDocument(docMap)), nil
	}
	// A part that is an array at the top pairs its entries by position, as one inside an object does.
	storedList, storedIsList := storedDoc.([]any)
	docList, docIsList := doc.([]any)
	if storedIsList && docIsList {
		if err := keepOmittedIn(docList, storedList); err != nil {
			return nil, err
		}
		return json.RawMessage(encodeDocument(docList)), nil
	}
	return incoming, nil
}

// keepOmittedValue adds to in, in place, what stored holds that in leaves out, where that is or
// contains a secret. A key that in leaves out comes back whole when its stored value holds a secret
// anywhere, an array included, since the client dropped that part and not one secret in it. A key
// that is present is followed into an object only: null removes it, and any other value is the
// client's own.
func keepOmittedValue(in, stored map[string]any) error {
	for k, sv := range stored {
		cur, present := in[k]
		switch {
		case !present:
			if holdsSecret(k, sv) {
				in[k] = sv
			}
		case cur == nil:
			// null removes
		default:
			if err := keepOmittedIn(cur, sv); err != nil {
				return err
			}
		}
	}
	return nil
}

// keepOmittedIn follows a present value into an object, or into an array whose entries pair with the
// stored ones by position. An entry pairs only when the two agree on everything but their secrets,
// the same test that restores a mask, so a reordered or removed entry takes nothing from another.
// An entry is ambiguous when it matches more than one stored entry and those disagree at a secret it
// leaves out.
func keepOmittedIn(cur, stored any) error {
	switch s := stored.(type) {
	case map[string]any:
		if c, ok := cur.(map[string]any); ok {
			return keepOmittedValue(c, s)
		}
	case []any:
		c, ok := cur.([]any)
		if !ok {
			return nil
		}
		for i := range c {
			if i >= len(s) || !sameWithoutSecrets(c[i], s[i]) {
				continue
			}
			if ambiguousEntry(c[i], s, true) {
				return errAmbiguousEntry
			}
			if err := keepOmittedIn(c[i], s[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// holdsSecret reports whether a value under key is a secret or contains one.
func holdsSecret(key string, v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if holdsSecret(k, child) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if holdsSecret("", child) {
				return true
			}
		}
	}
	// The mask is never a saved secret: a stored mask is dropped, not copied back.
	return secretKey(key) && secretScalar(v) && v != secretMask
}

// storedTypedConfig returns the typed form of the configuration stored for a device, or nil when
// none is stored or it cannot be read as one.
func (h *Handler) storedTypedConfig(id uint, device *database.Device) *configuration.TypedConfiguration {
	stored, err := h.Service.GetDeviceConfig(id)
	if err != nil {
		return nil
	}
	typed, _, err := h.convertToTypedConfig(stored.Config, device)
	if err != nil {
		return nil
	}
	return typed
}

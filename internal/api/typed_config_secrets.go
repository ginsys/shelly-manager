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
func restoreTypedSecrets(incoming, stored *configuration.TypedConfiguration, removed map[string]bool) {
	view := typedSecretView(incoming)
	storedView := typedSecretView(stored)
	restoreMaskedSecrets(view, storedView)
	keepOmittedSecrets(view, storedView, removed)
	adoptTypedSections(incoming, view)

	var storedRaw json.RawMessage
	if stored != nil {
		storedRaw = stored.Raw
	}
	incoming.Raw = restoreRawJSON(incoming.Raw, storedRaw)
}

// restoreRawJSON undoes maskRawJSON: the mask standing for the whole part, sent as a JSON string,
// becomes the stored part, and a mask inside it becomes the stored secret.
func restoreRawJSON(incoming, stored json.RawMessage) json.RawMessage {
	var whole string
	if json.Unmarshal(incoming, &whole) == nil && whole == secretMask {
		return stored
	}
	if len(incoming) > 0 {
		incoming = json.RawMessage(restoreMaskedText(string(incoming), string(stored)))
	}
	return keepOmittedRawSecrets(incoming, stored)
}

// keepOmittedRawSecrets copies into incoming each stored secret that it leaves out, the rule the
// typed secrets follow. A raw part the body omits altogether is the stored one. A raw secret sent
// as "" or null is present and stays as sent, and a raw part that is null stays null.
func keepOmittedRawSecrets(incoming, stored json.RawMessage) json.RawMessage {
	if len(incoming) == 0 {
		return stored
	}
	storedDoc, ok := decodeDocument(string(stored))
	if !ok || string(incoming) == "null" {
		return incoming
	}
	doc, ok := decodeDocument(string(incoming))
	if !ok {
		return incoming
	}
	return json.RawMessage(encodeDocument(keepOmittedValue(doc, storedDoc)))
}

// keepOmittedValue returns in with the secrets of stored that in leaves out added, or nil when in is
// absent and stored holds none. It follows objects only: an array is the client's own.
func keepOmittedValue(in, stored any) any {
	storedMap, ok := stored.(map[string]any)
	if !ok {
		return in
	}
	inMap, isMap := in.(map[string]any)
	if in != nil && !isMap {
		return in
	}
	for k, sv := range storedMap {
		cur, present := inMap[k]
		if present && cur == nil {
			continue // null removes
		}
		var kept any
		switch {
		case secretKey(k) && secretScalar(sv):
			if present {
				continue
			}
			kept = sv
		default:
			if kept = keepOmittedValue(cur, sv); kept == nil {
				continue
			}
		}
		if inMap == nil {
			inMap = map[string]any{}
		}
		inMap[k] = kept
	}
	if inMap == nil {
		return in
	}
	return inMap
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

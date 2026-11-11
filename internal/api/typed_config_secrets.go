package api

import (
	"encoding/json"

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
	if len(incoming) == 0 {
		return incoming
	}
	var whole string
	if json.Unmarshal(incoming, &whole) == nil && whole == secretMask {
		return stored
	}
	return json.RawMessage(restoreMaskedText(string(incoming), string(stored)))
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

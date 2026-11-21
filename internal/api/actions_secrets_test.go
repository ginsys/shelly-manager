package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/redact"
)

// secretActionURLs carry a password in the userinfo and a token in the query, the two places a URL
// keeps a secret. No response of the API may hold any of these fragments.
var secretActionURLs = []string{
	"http://admin:hunter2@192.168.1.4/on?token=s3cr3t",
	"http://ops:pw2@192.168.1.5/off",
}

var urlSecretFragments = []string{"hunter2", "s3cr3t", "pw2"}

func actionURLs(urls ...string) *[]string { return &urls }

func actionsOf(entries ...configuration.ActionConfig) *configuration.DeviceConfiguration {
	return &configuration.DeviceConfiguration{Actions: &entries}
}

func actionEntryOf(name string, index int, urls ...string) configuration.ActionConfig {
	return configuration.ActionConfig{Name: name, Index: index, SecretURLs: actionURLs(urls...)}
}

func marshalActions(t *testing.T, c *configuration.DeviceConfiguration) string {
	t.Helper()
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	return string(raw)
}

func assertNoURLSecret(t *testing.T, what, text string) {
	t.Helper()
	for _, fragment := range urlSecretFragments {
		assert.NotContains(t, text, fragment, "%s carries a URL secret", what)
	}
}

func urlsOf(entry configuration.ActionConfig) []string {
	if entry.SecretURLs == nil {
		return nil
	}
	return *entry.SecretURLs
}

func TestActionSecrets_EveryKeyRuleMasksSecretURLs(t *testing.T) {
	assert.True(t, redact.IsKey("secret_urls"), "redact.IsKey lets the typed key through")
	assert.True(t, isSecretKey("secret_urls"), "the drift and verify rule lets the typed key through")
}

func TestActionSecrets_MaskHidesEveryURLAndKeepsAClear(t *testing.T) {
	config := actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs...),
		actionEntryOf("out_on_url", 1, secretActionURLs[0]),
		actionEntryOf("out_off_url", 0), // a clear holds no secret
	)

	maskSecrets(config)

	body := marshalActions(t, config)
	assertNoURLSecret(t, "the masked config", body)
	entries := *config.Actions
	assert.Equal(t, []string{secretMask}, urlsOf(entries[0]), "a list is one mask, whatever its length")
	assert.Equal(t, []string{secretMask}, urlsOf(entries[1]))
	require.NotNil(t, entries[2].SecretURLs)
	assert.Empty(t, *entries[2].SecretURLs, "a clear is not a secret and must stay visible")
}

func TestActionSecrets_RedactRemovesEveryURL(t *testing.T) {
	config := actionsOf(actionEntryOf("btn_on_url", 0, secretActionURLs...))

	redactSecrets(config)

	assertNoURLSecret(t, "the redacted config", marshalActions(t, config))
	assert.Nil(t, (*config.Actions)[0].SecretURLs)
}

func TestActionSecrets_AMaskSentBackBecomesTheStoredListEvenWhenReordered(t *testing.T) {
	stored := actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs...),
		actionEntryOf("out_on_url", 0, "http://other/1"),
	)
	// The client dropped the first entry and reordered the rest.
	incoming := actionsOf(
		actionEntryOf("out_on_url", 0, secretMask),
		actionEntryOf("btn_on_url", 0, secretMask),
	)

	restoreMaskedSecrets(incoming, stored)

	entries := *incoming.Actions
	assert.Equal(t, []string{"http://other/1"}, urlsOf(entries[0]), "restored by identity, not by position")
	assert.Equal(t, secretActionURLs, urlsOf(entries[1]))
}

func TestActionSecrets_AMaskWithNothingStoredIsNeverKeptAndNeverAClear(t *testing.T) {
	incoming := actionsOf(actionEntryOf("btn_on_url", 0, secretMask))

	restoreMaskedSecrets(incoming, &configuration.DeviceConfiguration{})

	assert.Nil(t, (*incoming.Actions)[0].SecretURLs, "an empty list would be the explicit clear and erase the device's URLs")
	assert.NotContains(t, marshalActions(t, incoming), secretMask)
}

func TestActionSecrets_ARealListSentBackIsANewList(t *testing.T) {
	stored := actionsOf(actionEntryOf("btn_on_url", 0, secretActionURLs...))
	incoming := actionsOf(actionEntryOf("btn_on_url", 0, "http://new/1", "http://new/2"))

	restoreMaskedSecrets(incoming, stored)

	assert.Equal(t, []string{"http://new/1", "http://new/2"}, urlsOf((*incoming.Actions)[0]))
}

func TestActionSecrets_ADuplicateIdentityIsMaskedAndNeverRestored(t *testing.T) {
	stored := actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs[0]),
		actionEntryOf("btn_on_url", 0, secretActionURLs[1]),
	)
	masked := actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs[0]),
		actionEntryOf("btn_on_url", 0, secretActionURLs[1]),
	)

	maskSecrets(masked)
	assertNoURLSecret(t, "a masked duplicate", marshalActions(t, masked))
	for _, entry := range *masked.Actions {
		assert.Equal(t, []string{secretMask}, urlsOf(entry), "every entry is masked, whatever its identity")
	}

	restoreMaskedSecrets(masked, stored)

	for i, entry := range *masked.Actions {
		assert.Nil(t, entry.SecretURLs, "entry %d took a stored list although its identity is not unique", i)
	}
	assertNoURLSecret(t, "a restored duplicate", marshalActions(t, masked))
}

func TestActionSecrets_AUniqueIncomingEntryTakesNothingFromAStoredDuplicate(t *testing.T) {
	stored := actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs[0]),
		actionEntryOf("btn_on_url", 0, secretActionURLs[1]),
	)
	incoming := actionsOf(actionEntryOf("btn_on_url", 0, secretMask))

	restoreMaskedSecrets(incoming, stored)

	assert.Nil(t, (*incoming.Actions)[0].SecretURLs)
}

func TestActionSecrets_StoredSecretPathsNameNoURL(t *testing.T) {
	paths := storedSecretPaths(actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs...),
		actionEntryOf("out_off_url", 0),
	))

	require.Len(t, paths, 1, "a clear is not a stored secret")
	assertNoURLSecret(t, "the stored secret paths", paths[0])
}

// A clear holds no secret, and dropping it from a response changes what the entry means from
// "no URLs" to "leave alone".
func TestRedactSecrets_RemovesURLsButKeepsAClear(t *testing.T) {
	config := actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs...),
		configuration.ActionConfig{Name: "out_on_url", Index: 0, SecretURLs: &[]string{}},
		configuration.ActionConfig{Name: "out_off_url", Index: 0, Enabled: configuration.BoolPtr(true)},
	)

	redactSecrets(config)

	entries := *config.Actions
	assert.Nil(t, entries[0].SecretURLs, "the URLs are removed")
	require.NotNil(t, entries[1].SecretURLs, "the clear was dropped")
	assert.Empty(t, *entries[1].SecretURLs)
	assert.Nil(t, entries[2].SecretURLs, "an entry with no list gains none")
	assertNoURLSecret(t, "redacted config", marshalActions(t, config))
	assert.Contains(t, marshalActions(t, config), `"secret_urls":[]`)
}

// An explicit null removes the stored override, as it does for every other secret: the stored list
// is not brought back, and a null on one entry reaches that entry only.
func TestKeepOmittedActionSecrets_AnExplicitNullRemovesTheStoredList(t *testing.T) {
	stored := actionsOf(
		actionEntryOf("btn_on_url", 0, secretActionURLs[0]),
		configuration.ActionConfig{Name: "out_on_url", Index: 0, SecretURLs: &[]string{}},
		actionEntryOf("out_off_url", 0, secretActionURLs[1]),
	)
	body := func(second string) []byte {
		return []byte(`{"actions":[{"name":"btn_on_url","index":0},{"name":"out_on_url","index":0` + second +
			`},{"name":"out_off_url","index":0}]}`)
	}
	decode := func(raw []byte) *configuration.DeviceConfiguration {
		var c configuration.DeviceConfiguration
		require.NoError(t, json.Unmarshal(raw, &c))
		return &c
	}

	t.Run("a null on the stored clear", func(t *testing.T) {
		raw := body(`,"secret_urls":null`)
		removed, err := nullSecretPaths(raw)
		require.NoError(t, err)
		incoming := decode(raw)

		keepOmittedSecrets(incoming, stored, removed)

		entries := *incoming.Actions
		assert.Equal(t, secretActionURLs[:1], urlsOf(entries[0]), "an omitted list still keeps its stored one")
		assert.Nil(t, entries[1].SecretURLs, "the null was overridden by the stored clear")
		assert.Equal(t, secretActionURLs[1:], urlsOf(entries[2]))
	})

	t.Run("an omitted key keeps the stored clear, as the control", func(t *testing.T) {
		raw := body("")
		removed, err := nullSecretPaths(raw)
		require.NoError(t, err)
		incoming := decode(raw)

		keepOmittedSecrets(incoming, stored, removed)

		require.NotNil(t, (*incoming.Actions)[1].SecretURLs)
		assert.Empty(t, *(*incoming.Actions)[1].SecretURLs)
	})

	t.Run("a null under the wrapper of a template body", func(t *testing.T) {
		raw := []byte(`{"config":{"actions":[{"name":"btn_on_url","index":0,"secret_urls":null}]}}`)
		removed, err := wrappedNullSecretPaths(raw, "config")
		require.NoError(t, err)
		incoming := decode([]byte(`{"actions":[{"name":"btn_on_url","index":0}]}`))

		keepOmittedSecrets(incoming, stored, removed)

		assert.Nil(t, (*incoming.Actions)[0].SecretURLs)
	})
}

func TestKeepOmittedActionSecrets(t *testing.T) {
	removed := map[string]bool{}

	t.Run("an entry that omits its list keeps the stored one", func(t *testing.T) {
		stored := actionsOf(actionEntryOf("btn_on_url", 0, secretActionURLs...))
		incoming := actionsOf(configuration.ActionConfig{Name: "btn_on_url", Index: 0, Enabled: configuration.BoolPtr(true)})

		keepOmittedSecrets(incoming, stored, removed)

		assert.Equal(t, secretActionURLs, urlsOf((*incoming.Actions)[0]))
	})

	t.Run("a stored clear survives an omitted field", func(t *testing.T) {
		stored := actionsOf(actionEntryOf("btn_on_url", 0))
		incoming := actionsOf(configuration.ActionConfig{Name: "btn_on_url", Index: 0})

		keepOmittedSecrets(incoming, stored, removed)

		got := (*incoming.Actions)[0].SecretURLs
		require.NotNil(t, got, "the clear was lost")
		assert.Empty(t, *got)
	})

	t.Run("a stored duplicate hands nothing to an incoming entry", func(t *testing.T) {
		stored := actionsOf(
			actionEntryOf("btn_on_url", 0, secretActionURLs[0]),
			actionEntryOf("btn_on_url", 0, secretActionURLs[1]),
		)
		incoming := actionsOf(configuration.ActionConfig{Name: "btn_on_url", Index: 0})

		keepOmittedSecrets(incoming, stored, removed)

		assert.Nil(t, (*incoming.Actions)[0].SecretURLs)
	})

	t.Run("a duplicate on both sides takes nothing, even at the same position", func(t *testing.T) {
		stored := actionsOf(
			actionEntryOf("btn_on_url", 0, secretActionURLs[0]),
			actionEntryOf("btn_on_url", 0, secretActionURLs[1]),
		)
		incoming := actionsOf(
			configuration.ActionConfig{Name: "btn_on_url", Index: 0},
			configuration.ActionConfig{Name: "btn_on_url", Index: 0},
		)

		keepOmittedSecrets(incoming, stored, removed)

		for i, entry := range *incoming.Actions {
			assert.Nil(t, entry.SecretURLs, "duplicate %d took the stored list at its position", i)
		}
	})

	t.Run("an incoming duplicate takes nothing", func(t *testing.T) {
		stored := actionsOf(actionEntryOf("btn_on_url", 0, secretActionURLs[0]))
		incoming := actionsOf(
			configuration.ActionConfig{Name: "btn_on_url", Index: 0},
			configuration.ActionConfig{Name: "btn_on_url", Index: 0},
		)

		keepOmittedSecrets(incoming, stored, removed)

		for i, entry := range *incoming.Actions {
			assert.Nil(t, entry.SecretURLs, "duplicate %d took a stored list", i)
		}
	})

	t.Run("an entry the client dropped is not brought back", func(t *testing.T) {
		stored := actionsOf(actionEntryOf("btn_on_url", 0, secretActionURLs[0]), actionEntryOf("out_on_url", 0, secretActionURLs[1]))
		incoming := actionsOf(configuration.ActionConfig{Name: "btn_on_url", Index: 0})

		keepOmittedSecrets(incoming, stored, removed)

		require.Len(t, *incoming.Actions, 1)
		assert.Equal(t, secretActionURLs[:1], urlsOf((*incoming.Actions)[0]))
	})

	t.Run("a config with no actions section gains none", func(t *testing.T) {
		stored := actionsOf(actionEntryOf("btn_on_url", 0, secretActionURLs[0]))
		incoming := &configuration.DeviceConfiguration{}

		keepOmittedSecrets(incoming, stored, removed)

		assert.Nil(t, incoming.Actions)
	})
}

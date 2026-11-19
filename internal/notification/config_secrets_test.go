package notification

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/redact"
)

const ph = redact.Placeholder

func decodeMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m), string(raw))
	return m
}

func mustRestore(t *testing.T, incoming, stored json.RawMessage) json.RawMessage {
	t.Helper()
	out, err := restoreConfig(incoming, stored)
	require.NoError(t, err, string(incoming))
	return out
}

func TestMaskConfig_MasksEveryChannelSlot(t *testing.T) {
	raw := json.RawMessage(`{"url":"https://h.example/hook?t=tok","method":"POST","timeout":5,"secret":"s3",` +
		`"headers":{"Authorization":"Bearer b","X-Env":"prod"},"api_key":12345,"token_list":["a","b"],` +
		`"opts":{"password":"pw","name":"n"}}`)
	before := string(raw)

	m := decodeMap(t, maskConfig(raw))

	assert.Equal(t, ph, m["url"])
	assert.Equal(t, ph, m["secret"])
	assert.Equal(t, ph, m["api_key"], "a number under a secret key is masked too")
	assert.Equal(t, []any{ph, ph}, m["token_list"], "a secret key that holds a list is secret as a whole")
	assert.Equal(t, map[string]any{"Authorization": ph, "X-Env": ph}, m["headers"], "every header value is a credential")
	assert.Equal(t, map[string]any{"password": ph, "name": "n"}, m["opts"])
	assert.Equal(t, "POST", m["method"])
	assert.EqualValues(t, 5, m["timeout"])
	assert.Equal(t, before, string(raw), "the stored bytes are never changed")
}

func TestMaskConfig_SlackAndEmailAndOddInputs(t *testing.T) {
	m := decodeMap(t, maskConfig(json.RawMessage(`{"webhook_url":"https://hooks.slack.com/services/T/B/X","channel":"#ops"}`)))
	assert.Equal(t, ph, m["webhook_url"])
	assert.Equal(t, "#ops", m["channel"])

	email := json.RawMessage(`{"recipients":["a@example.com"],"subject":"s"}`)
	assert.JSONEq(t, string(email), string(maskConfig(email)), "an email channel holds no credential")

	assert.Equal(t, `{"url":""}`, string(maskConfig(json.RawMessage(`{"url":""}`))), "an empty secret stays empty")
	assert.Equal(t, "", string(maskConfig(nil)))
	assert.Equal(t, "null", string(maskConfig(json.RawMessage("null"))))
	assert.Equal(t, `"`+ph+`"`, string(maskConfig(json.RawMessage(`{"url":"x"`))), "text that is not JSON is masked whole")
	assert.Equal(t, `"`+ph+`"`, string(maskConfig(json.RawMessage(`{"a":1} trailing`))))
}

func TestMaskConfig_KeepsURLCharactersReadable(t *testing.T) {
	m := decodeMap(t, maskConfig(json.RawMessage(`{"template":"a&b<c>","url":"x"}`)))
	assert.Equal(t, "a&b<c>", m["template"])
}

func TestRestoreConfig(t *testing.T) {
	stored := json.RawMessage(`{"url":"https://h.example/hook?t=tok","secret":"s3","headers":{"Authorization":"Bearer b"},"timeout":5}`)

	t.Run("a placeholder takes the stored value, a plain field is as sent", func(t *testing.T) {
		incoming := json.RawMessage(`{"url":"` + ph + `","secret":"` + ph + `","headers":{"Authorization":"` + ph + `"},"timeout":9}`)
		m := decodeMap(t, mustRestore(t, incoming, stored))
		assert.Equal(t, "https://h.example/hook?t=tok", m["url"])
		assert.Equal(t, "s3", m["secret"])
		assert.Equal(t, map[string]any{"Authorization": "Bearer b"}, m["headers"])
		assert.EqualValues(t, 9, m["timeout"])
	})

	t.Run("a new value replaces the stored one", func(t *testing.T) {
		m := decodeMap(t, mustRestore(t, json.RawMessage(`{"url":"https://new.example","secret":"`+ph+`"}`), stored))
		assert.Equal(t, "https://new.example", m["url"])
		assert.Equal(t, "s3", m["secret"])
	})

	t.Run("a placeholder with nothing stored is dropped", func(t *testing.T) {
		m := decodeMap(t, mustRestore(t, json.RawMessage(`{"url":"u","secret":"`+ph+`","headers":{"X":"`+ph+`"}}`), nil))
		assert.NotContains(t, m, "secret")
		assert.Equal(t, map[string]any{}, m["headers"])
	})

	t.Run("a secret left out stays out", func(t *testing.T) {
		m := decodeMap(t, mustRestore(t, json.RawMessage(`{"url":"u"}`), stored))
		assert.NotContains(t, m, "secret")
	})

	t.Run("a config with no placeholder comes back untouched", func(t *testing.T) {
		incoming := json.RawMessage(`{"url": "u",   "secret":"x"}`)
		assert.Equal(t, string(incoming), string(mustRestore(t, incoming, stored)))
	})

	t.Run("the masked whole text and text that is not ours", func(t *testing.T) {
		assert.Equal(t, string(stored), string(mustRestore(t, json.RawMessage(`"`+ph+`"`), stored)))
		assert.Equal(t, `{"a":`, string(mustRestore(t, json.RawMessage(`{"a":`), stored)))
		assert.Equal(t, `[1]`, string(mustRestore(t, json.RawMessage(`[1]`), stored)))
	})
}

// encoding/json accepts any casing for a struct field, so "URL" and "Headers" are valid config keys
// that validation and delivery read. The mask and the restore read them the same way.
func TestMaskConfig_AnyKeyCasing(t *testing.T) {
	m := decodeMap(t, maskConfig(json.RawMessage(
		`{"URL":"https://h.example/SECRET","Webhook_URL":"https://h.example/S2","Headers":{"Authorization":"Bearer b"},"Secret":"s","Method":"POST"}`)))

	assert.Equal(t, ph, m["URL"])
	assert.Equal(t, ph, m["Webhook_URL"])
	assert.Equal(t, ph, m["Secret"])
	assert.Equal(t, map[string]any{"Authorization": ph}, m["Headers"])
	assert.Equal(t, "POST", m["Method"])

	// Only the top level has url and headers slots: a nested "url" is a plain field.
	nested := decodeMap(t, maskConfig(json.RawMessage(`{"opts":{"URL":"plain","Headers":{"a":"b"}}}`)))
	assert.Equal(t, map[string]any{"URL": "plain", "Headers": map[string]any{"a": "b"}}, nested["opts"])
}

func TestRestoreConfig_AnyKeyCasing(t *testing.T) {
	stored := json.RawMessage(`{"URL":"https://h.example/SECRET","Headers":{"Authorization":"Bearer b"},"Secret":"s3"}`)
	incoming := json.RawMessage(`{"url":"` + ph + `","HEADERS":{"authorization":"` + ph + `"},"secret":"` + ph + `"}`)

	m := decodeMap(t, mustRestore(t, incoming, stored))

	assert.Equal(t, "https://h.example/SECRET", m["url"])
	assert.Equal(t, map[string]any{"authorization": "Bearer b"}, m["HEADERS"])
	assert.Equal(t, "s3", m["secret"])
}

// A list has no key to pair its entries by. The list comes back whole when the client sent the
// placeholder for every entry of a stored list of the same length; any other placeholder in a list is
// dropped, and a plain entry is a new credential.
func TestRestoreConfig_Lists(t *testing.T) {
	stored := json.RawMessage(`{"token_list":["a","b"],"opts":{"keys":["x","y","z"]},"name":"n"}`)

	t.Run("the masked read comes back whole", func(t *testing.T) {
		masked := maskConfig(stored)
		require.Contains(t, string(masked), ph)
		m := decodeMap(t, mustRestore(t, masked, stored))
		assert.Equal(t, []any{"a", "b"}, m["token_list"])
		assert.Equal(t, map[string]any{"keys": []any{"x", "y", "z"}}, m["opts"])
		assert.NotContains(t, string(mustRestore(t, masked, stored)), ph)
	})

	t.Run("a different length takes nothing from the stored list", func(t *testing.T) {
		m := decodeMap(t, mustRestore(t, json.RawMessage(`{"token_list":["`+ph+`","`+ph+`","`+ph+`"]}`), stored))
		assert.Equal(t, []any{}, m["token_list"])
	})

	t.Run("a mixed list keeps the new entry and drops the placeholder", func(t *testing.T) {
		m := decodeMap(t, mustRestore(t, json.RawMessage(`{"token_list":["`+ph+`","fresh"]}`), stored))
		assert.Equal(t, []any{"fresh"}, m["token_list"])
	})

	t.Run("nothing stored drops every placeholder", func(t *testing.T) {
		out := mustRestore(t, json.RawMessage(`{"token_list":["`+ph+`"],"x":{"password":["`+ph+`"]}}`), nil)
		assert.NotContains(t, string(out), ph)
	})

	t.Run("a list that is not secret is left as sent", func(t *testing.T) {
		m := decodeMap(t, mustRestore(t, json.RawMessage(`{"recipients":["`+ph+`","b@example.com"]}`), stored))
		assert.Equal(t, []any{ph, "b@example.com"}, m["recipients"])
	})
}

// encoding/json folds U+017F (long s) to s and U+212A (Kelvin) to k when it matches a field name, so
// delivery reads these keys as "secret" and "..key".
func TestMaskConfig_FoldedUnicodeKeys(t *testing.T) {
	var cfg struct {
		Secret string `json:"secret"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"ſecret":"credential"}`), &cfg))
	require.Equal(t, "credential", cfg.Secret, "the premise: the decoder reads the key as secret")

	m := decodeMap(t, maskConfig(json.RawMessage(`{"url":"http://h.example","ſecret":"credential","apiKey":"k1"}`)))
	assert.Equal(t, ph, m["ſecret"])
	assert.Equal(t, ph, m["apiKey"])
}

// A JSON escape of the first star decodes to the placeholder, and a save of it must restore like the
// plain spelling. The backslash is joined at run time so the text is a real JSON escape.
func TestRestoreConfig_EscapedPlaceholder(t *testing.T) {
	escaped := `\` + `u002a*******`
	require.NotContains(t, escaped, ph, "the premise: the bytes do not hold the placeholder")
	stored := json.RawMessage(`{"url":"https://h.example/hook","secret":"s3"}`)
	m := decodeMap(t, mustRestore(t, json.RawMessage(`{"url":"`+escaped+`","secret":"`+escaped+`"}`), stored))
	assert.Equal(t, "https://h.example/hook", m["url"])
	assert.Equal(t, "s3", m["secret"])

	assert.Equal(t, string(stored), string(mustRestore(t, json.RawMessage(`"`+escaped+`"`), stored)), "the whole text, escaped")

	untouched := json.RawMessage(`{ "url": "u",  "secret": "x" }`)
	assert.Equal(t, string(untouched), string(mustRestore(t, untouched, stored)), "no placeholder: the client's bytes come back")
}

// Two stored keys that fold together name one decoded field by position, which a map does not keep.
// The placeholder takes neither, so a save cannot move the destination.
func TestRestoreConfig_AmbiguousFoldedKeys(t *testing.T) {
	stored := json.RawMessage(`{"URL":"https://a.example","url":"https://b.example"}`)
	out := mustRestore(t, json.RawMessage(`{"Url":"`+ph+`"}`), stored)
	assert.NotContains(t, decodeMap(t, out), "Url", "an ambiguous placeholder is dropped")
	assert.NotContains(t, string(out), "example")
}

// encoding/json reads a repeated member in text order (a later null is ignored, names fold together);
// the generic decode keeps the last value and loses the order, and re-encoding sorts the members. No
// restore can promise to keep the destination delivery reads, so a config that repeats a name is refused
// whether or not it carries the placeholder, for every spelling of the name and at any depth.
func TestRestoreConfig_RepeatedMembersAreRefused(t *testing.T) {
	stored := json.RawMessage(`{"url":"https://h.example/stored","headers":{"Authorization":"Bearer x"}}`)

	for name, incoming := range map[string]string{
		"plain":                `{"url":"old","url":"new"}`,
		"folded":               `{"url":"https://old.example/hook","URL":"https://new.example/hook","headers":{"Authorization":"Bearer x"}}`,
		"long s folds to s":    `{"ſecret":"a","secret":"b","url":"u"}`,
		"nested":               `{"opts":{"a":1,"a":2},"url":"u"}`,
		"in a list":            `{"opts":[{"x":1,"x":2}],"url":"u"}`,
		"with the placeholder": `{"url":"` + ph + `","url":null}`,
		"folded placeholder":   `{"URL":"` + ph + `","url":null}`,
		"after a huge number":  `{"n":1e1000,"url":"https://old.example","URL":"https://new.example","headers":{"Authorization":"` + ph + `"}}`,
	} {
		out, err := restoreConfig(json.RawMessage(incoming), stored)
		require.ErrorIs(t, err, errRepeatedMember, name)
		assert.Nil(t, out, name)
	}

	// Nothing is refused that repeats no name: a huge number, equal names in different objects, a list.
	for name, incoming := range map[string]string{
		"huge number":         `{"n":1e1000,"url":"u"}`,
		"same name elsewhere": `{"url":"u","headers":{"url":"a"},"opts":{"url":"b"}}`,
		"list of objects":     `{"url":"u","opts":[{"x":1},{"x":2}]}`,
	} {
		out, err := restoreConfig(json.RawMessage(incoming), stored)
		require.NoError(t, err, name)
		assert.Equal(t, incoming, string(out), name)
	}
}

func TestMaskedChannel_DoesNotChangeTheStoredChannel(t *testing.T) {
	stored := NotificationChannel{ID: 1, Type: "webhook", Config: json.RawMessage(`{"url":"https://h.example/x","secret":"s3"}`)}
	before := string(stored.Config)

	out := maskedChannel(stored)

	assert.Equal(t, before, string(stored.Config))
	assert.NotContains(t, string(out.Config), "s3")
	assert.Equal(t, uint(1), out.ID)
}

package notification

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
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
		m := decodeMap(t, restoreConfig(incoming, stored))
		assert.Equal(t, "https://h.example/hook?t=tok", m["url"])
		assert.Equal(t, "s3", m["secret"])
		assert.Equal(t, map[string]any{"Authorization": "Bearer b"}, m["headers"])
		assert.EqualValues(t, 9, m["timeout"])
	})

	t.Run("a new value replaces the stored one", func(t *testing.T) {
		m := decodeMap(t, restoreConfig(json.RawMessage(`{"url":"https://new.example","secret":"`+ph+`"}`), stored))
		assert.Equal(t, "https://new.example", m["url"])
		assert.Equal(t, "s3", m["secret"])
	})

	t.Run("a placeholder with nothing stored is dropped", func(t *testing.T) {
		m := decodeMap(t, restoreConfig(json.RawMessage(`{"url":"u","secret":"`+ph+`","headers":{"X":"`+ph+`"}}`), nil))
		assert.NotContains(t, m, "secret")
		assert.Equal(t, map[string]any{}, m["headers"])
	})

	t.Run("a secret left out stays out", func(t *testing.T) {
		m := decodeMap(t, restoreConfig(json.RawMessage(`{"url":"u"}`), stored))
		assert.NotContains(t, m, "secret")
	})

	t.Run("a config with no placeholder comes back untouched", func(t *testing.T) {
		incoming := json.RawMessage(`{"url": "u",   "secret":"x"}`)
		assert.Equal(t, string(incoming), string(restoreConfig(incoming, stored)))
	})

	t.Run("the masked whole text and text that is not ours", func(t *testing.T) {
		assert.Equal(t, string(stored), string(restoreConfig(json.RawMessage(`"`+ph+`"`), stored)))
		assert.Equal(t, `{"a":`, string(restoreConfig(json.RawMessage(`{"a":`), stored)))
		assert.Equal(t, `[1]`, string(restoreConfig(json.RawMessage(`[1]`), stored)))
	})
}

// closedPortURL returns a loopback URL with userinfo to a port nothing listens on.
func closedPortURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return "http://hook:pw620@" + addr + "/services/LEAKPATH620?t=LEAKQUERY620"
}

// What net/http writes into a failed send's error is the premise the scrub rests on: the password is
// "***" there, so the host, the path and the query token stay in the text unless the scrub matches
// that spelling too.
func TestScrubText_ARealFailedSendWithUserinfo(t *testing.T) {
	target := closedPortURL(t)
	_, err := http.Post(target, "application/json", nil) //nolint:gosec // loopback, closed port
	require.Error(t, err)

	assert.Contains(t, err.Error(), "hook:***@", "net/http writes the password as ***")
	assert.NotContains(t, err.Error(), "pw620")
	assert.Contains(t, err.Error(), "LEAKPATH620", "the path and query stay in the text")

	cfg := json.RawMessage(`{"url":"` + target + `"}`)
	text := scrubText("failed to send webhook: "+err.Error(), cfg)
	assert.NotContains(t, text, "LEAKPATH620")
	assert.NotContains(t, text, "LEAKQUERY620")
	assert.NotContains(t, text, "***@")
	assert.Contains(t, text, ph)
	// The dial address is the network error's own text, not the URL: it stays.
	assert.Contains(t, text, "dial tcp 127.0.0.1:")

	// Only the stored spelling is matched without the net/http one: the control for why both are.
	assert.Contains(t, err.Error(), "LEAKQUERY620")
	assert.NotContains(t, strings.ReplaceAll(err.Error(), target, ph), ph, "the stored spelling alone matches nothing")
}

func TestScrubText_OtherShapes(t *testing.T) {
	cfg := json.RawMessage(`{"url":"https://h.example/x?t=tok","secret":"s3cret","headers":{"Authorization":"Bearer bb"},"method":"POST"}`)

	// An unparseable URL quotes the raw string.
	assert.Equal(t, `parse "`+ph+`": bad`, scrubText(`parse "https://h.example/x?t=tok": bad`, cfg))
	// Several credentials in one text, and a plain field is left alone.
	assert.Equal(t, ph+" "+ph+" "+ph+" POST", scrubText("https://h.example/x?t=tok s3cret Bearer bb POST", cfg))
	// A text with nothing to cut, a config that is not an object, and an empty text.
	assert.Equal(t, "connection refused", scrubText("connection refused", cfg))
	assert.Equal(t, "see s3cret", scrubText("see s3cret", json.RawMessage(`not json`)))
	assert.Equal(t, "", scrubText("", cfg))
	// A longer credential goes before a shorter one it contains.
	nested := json.RawMessage(`{"secret":"abc","token":"abcdef"}`)
	assert.Equal(t, ph, scrubText("abcdef", nested))
}

func TestScrubError(t *testing.T) {
	cfg := json.RawMessage(`{"secret":"s3cret"}`)
	assert.NoError(t, scrubError(nil, cfg))

	clean := errors.New("connection refused")
	assert.Same(t, clean, scrubError(clean, cfg), "an error with nothing to cut is returned as it is")

	got := scrubError(errors.New("failed: s3cret"), cfg)
	assert.EqualError(t, got, "failed: "+ph)
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

	m := decodeMap(t, restoreConfig(incoming, stored))

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
		m := decodeMap(t, restoreConfig(masked, stored))
		assert.Equal(t, []any{"a", "b"}, m["token_list"])
		assert.Equal(t, map[string]any{"keys": []any{"x", "y", "z"}}, m["opts"])
		assert.NotContains(t, string(restoreConfig(masked, stored)), ph)
	})

	t.Run("a different length takes nothing from the stored list", func(t *testing.T) {
		m := decodeMap(t, restoreConfig(json.RawMessage(`{"token_list":["`+ph+`","`+ph+`","`+ph+`"]}`), stored))
		assert.Equal(t, []any{}, m["token_list"])
	})

	t.Run("a mixed list keeps the new entry and drops the placeholder", func(t *testing.T) {
		m := decodeMap(t, restoreConfig(json.RawMessage(`{"token_list":["`+ph+`","fresh"]}`), stored))
		assert.Equal(t, []any{"fresh"}, m["token_list"])
	})

	t.Run("nothing stored drops every placeholder", func(t *testing.T) {
		out := restoreConfig(json.RawMessage(`{"token_list":["`+ph+`"],"x":{"password":["`+ph+`"]}}`), nil)
		assert.NotContains(t, string(out), ph)
	})

	t.Run("a list that is not secret is left as sent", func(t *testing.T) {
		m := decodeMap(t, restoreConfig(json.RawMessage(`{"recipients":["`+ph+`","b@example.com"]}`), stored))
		assert.Equal(t, []any{ph, "b@example.com"}, m["recipients"])
	})
}

// The failed send quotes a URL the way net/url does, not the way it was stored.
func TestScrubText_QuotedAndNormalizedSpellings(t *testing.T) {
	t.Run("a control character is written as an escape", func(t *testing.T) {
		stored := "http://h.example/x?t=LEAKQUERY620\n"
		_, err := http.NewRequest(http.MethodPost, stored, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `LEAKQUERY620\n`, "net/url quotes the raw URL with %q")
		require.NotContains(t, err.Error(), stored)

		cfg, jerr := json.Marshal(map[string]any{"url": stored})
		require.NoError(t, jerr)
		text := scrubText(err.Error(), cfg)
		assert.NotContains(t, text, "LEAKQUERY620")
		assert.Contains(t, text, ph)
	})

	t.Run("a URL with a quote and a backslash", func(t *testing.T) {
		stored := `http://h.example/a"b\c?t=LEAKQUERY620`
		text := scrubText(`failed: Post `+strconv.Quote(stored)+`: boom`, json.RawMessage(`{"url":`+strconv.Quote(stored)+`}`))
		assert.NotContains(t, text, "LEAKQUERY620")
	})

	t.Run("a path net/http percent-encodes", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := l.Addr().String()
		require.NoError(t, l.Close())
		stored := "http://" + addr + "/p/Ünï/LEAKPATH620?t=LEAKQUERY620"

		_, perr := http.Post(stored, "application/json", nil) //nolint:gosec // loopback, closed port
		require.Error(t, perr)
		require.Contains(t, perr.Error(), "%C3%9C", "net/http writes the encoded path")
		require.NotContains(t, perr.Error(), "Ün", "so the stored spelling is not in the text")

		cfg, jerr := json.Marshal(map[string]any{"url": stored})
		require.NoError(t, jerr)
		text := scrubText(perr.Error(), cfg)
		assert.NotContains(t, text, "LEAKPATH620")
		assert.NotContains(t, text, "LEAKQUERY620")
		assert.NotContains(t, text, "%C3%9C")
	})
}

func TestScrubText_SecretInsideAList(t *testing.T) {
	cfg := json.RawMessage(`{"token_list":["tok-one","tok-two"],"recipients":["a@example.com"]}`)
	assert.Equal(t, ph+" "+ph+" a@example.com", scrubText("tok-one tok-two a@example.com", cfg))
}

func TestMaskedChannel_DoesNotChangeTheStoredChannel(t *testing.T) {
	stored := NotificationChannel{ID: 1, Type: "webhook", Config: json.RawMessage(`{"url":"https://h.example/x","secret":"s3"}`)}
	before := string(stored.Config)

	out := maskedChannel(stored)

	assert.Equal(t, before, string(stored.Config))
	assert.NotContains(t, string(out.Config), "s3")
	assert.Equal(t, uint(1), out.ID)
}

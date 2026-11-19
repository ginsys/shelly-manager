package redact

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// channelSlots are the names a notification channel adds at the document root.
var channelSlots = Slots{"url", "webhook_url", "headers"}

func decode(t *testing.T, text string) any {
	t.Helper()
	doc, ok := DecodeJSON([]byte(text))
	require.True(t, ok, text)
	return doc
}

func encoded(t *testing.T, doc any) string {
	t.Helper()
	out, err := EncodeJSON(doc)
	require.NoError(t, err)
	return string(out)
}

func masked(t *testing.T, text string, slots Slots) string {
	t.Helper()
	doc := decode(t, text)
	MaskTree(doc, slots)
	return encoded(t, doc)
}

func restored(t *testing.T, incoming, stored string, slots Slots) string {
	t.Helper()
	doc := decode(t, incoming)
	require.NoError(t, RestoreTree(doc, decode(t, stored), slots), incoming)
	return encoded(t, doc)
}

func TestMaskTree_ScalarsAndNumbers(t *testing.T) {
	got := masked(t, `{"pass":"p","api_key":12345,"name":"n","keep_alive":60,"empty_pass":""}`, nil)
	assert.JSONEq(t, `{"pass":"`+Placeholder+`","api_key":"`+Placeholder+`","name":"n","keep_alive":60,"empty_pass":""}`, got)
	assert.Equal(t, got, masked(t, got, nil), "masking a masked tree changes nothing")
}

// A secret key that holds an object or a list is one credential, and nothing inside it stays readable.
func TestMaskTree_ContainerUnderASecretKeyIsMaskedWhole(t *testing.T) {
	got := masked(t, `{"tokens":["a","b"],"password":{"x":"y"},"api_secrets":[{"v":1}],"empty":{},"token":[],`+
		`"opts":{"password":"pw","name":"n"},"recipients":["ops@example.com"]}`, nil)
	assert.JSONEq(t, `{"tokens":"`+Placeholder+`","password":"`+Placeholder+`","api_secrets":"`+Placeholder+`","empty":{},"token":[],`+
		`"opts":{"password":"`+Placeholder+`","name":"n"},"recipients":["ops@example.com"]}`, got)
}

func TestMaskTree_ListAtTheTopIsWalked(t *testing.T) {
	assert.JSONEq(t, `[{"pass":"`+Placeholder+`","n":1},"x"]`, masked(t, `[{"pass":"p","n":1},"x"]`, channelSlots))
}

// encoding/json folds U+017F (long s) to s and U+212A (Kelvin) to k when it matches a field name, so a
// reader of the document sees these keys as "secret" and "..key".
func TestMaskTree_FoldedUnicodeKeys(t *testing.T) {
	var cfg struct {
		Secret string `json:"secret"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"ſecret":"credential"}`), &cfg))
	require.Equal(t, "credential", cfg.Secret, "the premise: the decoder reads the key as secret")

	got := masked(t, `{"ſecret":"credential","apiKey":"k1","name":"n"}`, nil)
	assert.NotContains(t, got, "credential")
	assert.NotContains(t, got, "k1")
	assert.Contains(t, got, `"name":"n"`)
}

// The root slots are secret at the root only, whatever their casing; a nested one is a plain field.
func TestMaskTree_RootSlots(t *testing.T) {
	text := `{"URL":"https://h.example/S","Webhook_URL":"u2","Headers":{"A":"b"},"opts":{"url":"plain","headers":{"a":"b"}}}`
	assert.JSONEq(t, `{"URL":"`+Placeholder+`","Webhook_URL":"`+Placeholder+`","Headers":"`+Placeholder+`","opts":{"url":"plain","headers":{"a":"b"}}}`,
		masked(t, text, channelSlots))
	assert.JSONEq(t, text, masked(t, text, nil), "with no slots the same document holds no secret")
}

func TestRestoreTree_PlaceholderTakesTheStoredValueOrIsDropped(t *testing.T) {
	stored := `{"mqtt":{"pass":"kept","server":"old"},"list":[{"token":"t0"}]}`
	incoming := `{"mqtt":{"pass":"` + Placeholder + `","server":"new"},"list":[{"token":"` + Placeholder + `"}],"auth":{"pass":"` + Placeholder + `"}}`
	assert.JSONEq(t, `{"mqtt":{"pass":"kept","server":"new"},"list":[{"token":"t0"}],"auth":{}}`, restored(t, incoming, stored, nil))
}

func TestRestoreTree_NumberAndContainerComeBackWhole(t *testing.T) {
	assert.JSONEq(t, `{"auth_pass":12345678}`, restored(t, `{"auth_pass":"`+Placeholder+`"}`, `{"auth_pass":12345678}`, nil))

	stored := `{"tokens":["a","b"],"headers":{"Authorization":"Bearer b"},"opts":{"password":{"k":"v"}}}`
	assert.JSONEq(t, stored, restored(t, masked(t, stored, channelSlots), stored, channelSlots))
}

func TestRestoreTree_AnythingElseIsANewValueAsSent(t *testing.T) {
	stored := `{"tokens":["a","b"],"pass":"old","url":"https://old.example"}`
	incoming := `{"tokens":["fresh"],"pass":"new","url":{"a":1}}`
	assert.JSONEq(t, incoming, restored(t, incoming, stored, channelSlots))
}

func TestRestoreTree_NothingStoredAndAStoredPlaceholderDropTheKey(t *testing.T) {
	incoming := `{"pass":"` + Placeholder + `","name":"n"}`
	assert.JSONEq(t, `{"name":"n"}`, restored(t, incoming, `null`, nil))
	assert.JSONEq(t, `{"name":"n"}`, restored(t, incoming, `{"pass":"`+Placeholder+`"}`, nil), "a stored placeholder is never copied back")
}

// The stored value is found without regard to key case. Two stored keys that fold together name one
// decoded field by position, which a map does not keep, so the placeholder takes neither.
func TestRestoreTree_StoredKeyIsFoundWithoutRegardToCase(t *testing.T) {
	assert.JSONEq(t, `{"secret":"s3"}`, restored(t, `{"secret":"`+Placeholder+`"}`, `{"Secret":"s3"}`, nil))
	assert.JSONEq(t, `{}`, restored(t, `{"SECRET":"`+Placeholder+`"}`, `{"Secret":"a","secret":"b"}`, nil))
	assert.JSONEq(t, `{"opts":{"password":"kept"}}`, restored(t, `{"opts":{"password":"`+Placeholder+`"}}`, `{"OPTS":{"password":"kept"}}`, nil),
		"a container is followed by its folded name too")
}

// An entry of a list takes the secret of the stored entry at its position only when the two agree on
// everything but their secrets: a reordered or removed entry must not take another entry's secret.
func TestRestoreTree_ListEntriesPairByPositionAndContent(t *testing.T) {
	stored := `[{"name":"a","pass":"pa"},{"name":"b","pass":"pb"}]`
	assert.JSONEq(t, stored, restored(t, `[{"name":"a","pass":"`+Placeholder+`"},{"name":"b","pass":"`+Placeholder+`"}]`, stored, nil))
	assert.JSONEq(t, `[{"name":"b"},{"name":"a"}]`,
		restored(t, `[{"name":"b","pass":"`+Placeholder+`"},{"name":"a","pass":"`+Placeholder+`"}]`, stored, nil), "a reordered entry takes nothing")
	assert.JSONEq(t, `{"targets":[{"name":"n"}]}`,
		restored(t, `{"targets":[{"secret":"`+Placeholder+`","name":"n"}]}`, `{}`, channelSlots), "nothing stored: dropped")
	assert.JSONEq(t, `{"a":{"b":[{"name":"x","pass":"kept"}]}}`,
		restored(t, `{"a":{"b":[{"name":"x","pass":"`+Placeholder+`"}]}}`, `{"a":{"b":[{"name":"x","pass":"kept"}]}}`, nil))
}

// Entries that agree on everything but a secret they get back from the stored list cannot be told
// apart: the request is refused. Entries that differ only in a secret the entry sends anew are not.
func TestRestoreTree_AmbiguousListEntryIsRefused(t *testing.T) {
	stored := decode(t, `[{"name":"a","pass":"p1"},{"name":"a","pass":"p2"}]`)

	doc := decode(t, `[{"name":"a","pass":"`+Placeholder+`"}]`)
	err := RestoreTree(doc, stored, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAmbiguousEntry))

	doc = decode(t, `[{"name":"a","pass":"fresh"}]`)
	require.NoError(t, RestoreTree(doc, stored, nil))
	assert.JSONEq(t, `[{"name":"a","pass":"fresh"}]`, encoded(t, doc))
}

func TestSameWithoutSecrets(t *testing.T) {
	same := func(a, b string) bool { return SameWithoutSecrets(decode(t, a), decode(t, b)) }
	assert.True(t, same(`{"n":"a","pass":"x"}`, `{"n":"a","pass":"y"}`), "a secret may differ")
	assert.True(t, same(`{"n":"a","pass":""}`, `{"n":"a","pass":null}`), "a cleared secret is a secret too")
	assert.True(t, same(`{"n":"a","tokens":["x"]}`, `{"n":"a","tokens":{"k":"y"}}`), "a container under a secret key is a secret")
	assert.True(t, same(`{"n":1.0}`, `{"n":1e0}`), "numbers compare by value")
	assert.False(t, same(`{"n":"a","pass":"x"}`, `{"n":"b","pass":"x"}`))
	assert.False(t, same(`{"n":"a"}`, `{"n":"a","m":1}`))
}

func TestHoldsSecret(t *testing.T) {
	holds := func(key, text string) bool { return HoldsSecret(key, decode(t, text)) }
	assert.True(t, holds("pass", `"p"`))
	assert.True(t, holds("auth_pass", `12345`))
	assert.True(t, holds("x", `{"a":{"pass":"p"}}`), "a secret anywhere inside")
	assert.True(t, holds("x", `[{"token":"t"}]`))
	assert.True(t, holds("tokens", `["a"]`), "a non-empty container under a secret key is one secret")
	assert.False(t, holds("pass", `""`))
	assert.False(t, holds("pass", `"`+Placeholder+`"`), "a stored placeholder is not a saved secret")
	assert.False(t, holds("tokens", `[]`))
	assert.False(t, holds("name", `"n"`))
}

// encoding/json reads a repeated member in text order and the generic decode keeps the last value, so
// a text that repeats a name, in any spelling the decoder folds together and at any depth, is refused.
func TestHasRepeatedMember(t *testing.T) {
	for name, text := range map[string]string{
		"plain":               `{"url":"old","url":"new"}`,
		"folded":              `{"url":"a","URL":"b"}`,
		"long s folds to s":   `{"ſecret":"a","secret":"b"}`,
		"nested":              `{"opts":{"a":1,"a":2},"url":"u"}`,
		"in a list":           `{"opts":[{"x":1,"x":2}]}`,
		"after a huge number": `{"n":1e1000,"url":"a","URL":"b"}`,
		"unreadable text":     `{"a":`,
	} {
		assert.True(t, HasRepeatedMember([]byte(text)), name)
	}
	for name, text := range map[string]string{
		"huge number":         `{"n":1e1000,"url":"u"}`,
		"same name elsewhere": `{"url":"u","headers":{"url":"a"},"opts":{"url":"b"}}`,
		"list of objects":     `{"opts":[{"x":1},{"x":2}]}`,
		"dotless i":           "{\"id\":1,\"ıd\":2}",
		"dotted capital I":    "{\"id\":1,\"İd\":2}",
		"sharp s":             "{\"ss\":1,\"ß\":2}",
	} {
		assert.False(t, HasRepeatedMember([]byte(text)), name)
	}
}

func TestDecodeAndEncodeJSON(t *testing.T) {
	_, ok := DecodeJSON([]byte(`{"a":1} trailing`))
	assert.False(t, ok, "trailing data")
	_, ok = DecodeJSON([]byte(`{"a":`))
	assert.False(t, ok, "truncated")
	doc, ok := DecodeJSON([]byte(`{"a":12345678901234567890}`))
	require.True(t, ok)
	assert.Equal(t, `{"a":12345678901234567890}`, encoded(t, doc), "a number keeps every digit")
	assert.Equal(t, `{"t":"a&b<c>"}`, encoded(t, decode(t, `{"t":"a&b<c>"}`)), "a URL keeps its & and <")
}

// foldName is the fold encoding/json applies to member names: two names fold together exactly when
// strings.EqualFold says they are equal.
func TestFoldName_AgreesWithEqualFold(t *testing.T) {
	names := []string{"", "id", "ID", "Id", "ıd", "İd", "ss", "SS", "ß", "s", "S", "ſ", "k", "K", "K",
		"secret", "SECRET", "ſecret", "api_key", "API_KEY", "σ", "ς", "Σ", "straße", "STRASSE"}
	for _, a := range names {
		for _, b := range names {
			assert.Equal(t, strings.EqualFold(a, b), FoldName(a) == FoldName(b), "%q vs %q", a, b)
		}
	}
}

// A decoder reads the key the same way: the premise of the tests above.
func TestFoldName_MatchesWhatTheDecoderDoes(t *testing.T) {
	var v struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte("{\"ıd\":7}"), &v))
	assert.Equal(t, 0, v.ID, "the decoder does not read the dotless i as i")
	require.NoError(t, json.Unmarshal([]byte(`{"ID":7}`), &v))
	assert.Equal(t, 7, v.ID)
}

// CanonicalNumber agrees with exact arithmetic: over every small combination of sign, integer part,
// fraction and exponent, two spellings canonicalize alike exactly when their values are equal.
func TestCanonicalNumber_AgreesWithExactArithmetic(t *testing.T) {
	var spellings []string
	for _, sign := range []string{"", "-"} {
		for _, intPart := range []string{"0", "1", "10", "7", "100", "12"} {
			for _, frac := range []string{"", ".0", ".00", ".5", ".50", ".05", ".125"} {
				for _, exp := range []string{"", "e0", "E+0", "e-0", "e1", "e-1", "E+2", "e-2", "e03", "e-03", "e12", "e-12", "e30", "e-30"} {
					spellings = append(spellings, sign+intPart+frac+exp)
				}
			}
		}
	}
	value := func(s string) *big.Rat {
		r, ok := new(big.Rat).SetString(s)
		require.True(t, ok, s)
		return r
	}
	for _, a := range spellings {
		for _, b := range spellings {
			same := value(a).Cmp(value(b)) == 0
			if (CanonicalNumber(a) == CanonicalNumber(b)) != same {
				t.Fatalf("%s and %s: equal values %v, canonical %s and %s", a, b, same, CanonicalNumber(a), CanonicalNumber(b))
			}
		}
	}
}

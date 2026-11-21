package sync

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/redact"
)

const (
	storedActions = `{"actions":[` +
		`{"name":"btn_on_url","index":0,"secret_urls":["http://a:hunter2@h/1","http://a/2"]},` +
		`{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["http://b/1"]}]}`
	listShape = `["[REDACTED]","[REDACTED]"]`
	bareShape = `"[REDACTED]"`
)

func resolveActionsArchive(t *testing.T, archive, stored string) (string, int) {
	t.Helper()
	var a, s map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(archive), &a))
	require.NoError(t, json.Unmarshal([]byte(stored), &s))
	found := resolveConfiguration(a, s)
	raw, err := json.Marshal(a)
	require.NoError(t, err)
	return string(raw), found
}

func TestResolveRedactedActions(t *testing.T) {
	cases := []struct {
		name, archive, want string
		found               int
	}{
		{
			name: "the list shape of an export takes the stored lists",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":` + listShape + `},` +
				`{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["[REDACTED]"]}]}`,
			want:  storedActions,
			found: 3,
		},
		{
			name: "the bare shape of the SMA export takes the stored lists",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":` + bareShape + `},` +
				`{"name":"out_on_url","index":0,"enabled":true,"secret_urls":` + bareShape + `}]}`,
			want:  storedActions,
			found: 2,
		},
		{
			name: "a reordered archive restores nothing and never another entry's list",
			archive: `{"actions":[{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["[REDACTED]"]},` +
				`{"name":"btn_on_url","index":0,"secret_urls":` + listShape + `}]}`,
			want: `{"actions":[{"name":"out_on_url","index":0,"enabled":true},` +
				`{"name":"btn_on_url","index":0}]}`,
			found: 3,
		},
		{
			name:    "a shorter archive restores the entries that still line up and no other",
			archive: `{"actions":[{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["[REDACTED]"]}]}`,
			want:    `{"actions":[{"name":"out_on_url","index":0,"enabled":true}]}`,
			found:   1,
		},
		{
			name: "a longer archive gives the extra entry nothing",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":` + listShape + `},` +
				`{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["[REDACTED]"]},` +
				`{"name":"out_off_url","index":0,"secret_urls":["[REDACTED]"]}]}`,
			want: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["http://a:hunter2@h/1","http://a/2"]},` +
				`{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["http://b/1"]},` +
				`{"name":"out_off_url","index":0}]}`,
			found: 4,
		},
		{
			name: "a list of another length than the stored one is not restored",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["[REDACTED]"]},` +
				`{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["[REDACTED]"]}]}`,
			want: `{"actions":[{"name":"btn_on_url","index":0},` +
				`{"name":"out_on_url","index":0,"enabled":true,"secret_urls":["http://b/1"]}]}`,
			found: 2,
		},
		{
			name:    "an entry that differs from its stored twin in a plain field takes nothing",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"enabled":false,"secret_urls":` + listShape + `}]}`,
			want:    `{"actions":[{"name":"btn_on_url","index":0,"enabled":false}]}`,
			found:   2,
		},
		{
			name:    "a real list is left as it was written",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["http://new/1"]}]}`,
			want:    `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["http://new/1"]}]}`,
			found:   0,
		},
		{
			name:    "a real clear is left as it was written, never replaced by the stored list",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":[]}]}`,
			want:    `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":[]}]}`,
			found:   0,
		},
		{
			name:    "a list that mixes a placeholder and a real URL loses the key",
			archive: `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["[REDACTED]","http://new/2"]}]}`,
			want:    `{"actions":[{"name":"btn_on_url","index":0}]}`,
			found:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := resolveActionsArchive(t, tc.archive, storedActions)

			assert.JSONEq(t, tc.want, got)
			assert.Equal(t, tc.found, found)
			assert.NotContains(t, got, RedactedValue, "the placeholder was stored")
		})
	}
}

// The configuration decoder matches keys without regard to case, so an archive that spells the list
// key another way still states a list. It follows the same rule: a placeholder with nothing stored
// loses the key, and never becomes the explicit clear.
func TestResolveRedactedActions_AnotherSpellingOfTheKeyFollowsTheSameRule(t *testing.T) {
	for _, spelling := range []string{"SECRET_URLS", "Secret_Urls"} {
		t.Run(spelling, func(t *testing.T) {
			nothingStored, found := resolveActionsArchive(t,
				`{"actions":[{"name":"btn_on_url","index":0,"`+spelling+`":`+listShape+`}]}`, `{}`)
			assert.JSONEq(t, `{"actions":[{"name":"btn_on_url","index":0}]}`, nothingStored,
				"the placeholder list became a clear or kept a placeholder")
			assert.Equal(t, 2, found)

			restored, found := resolveActionsArchive(t,
				`{"actions":[{"name":"btn_on_url","index":0,"`+spelling+`":`+listShape+`}]}`,
				`{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["http://a:hunter2@h/1","http://a/2"]}]}`)
			assert.JSONEq(t, `{"actions":[{"name":"btn_on_url","index":0,"`+spelling+`":["http://a:hunter2@h/1","http://a/2"]}]}`,
				restored, "the stored list was not restored under the other spelling")
			assert.Equal(t, 2, found)

			kept, _ := resolveActionsArchive(t,
				`{"actions":[{"name":"btn_on_url","index":0,"`+spelling+`":["http://new/1"]}]}`, `{}`)
			assert.JSONEq(t, `{"actions":[{"name":"btn_on_url","index":0,"`+spelling+`":["http://new/1"]}]}`, kept,
				"a list the author wrote is kept as it is, under the spelling it came with")
		})
	}
}

// The rule is for an action entry, which holds a name and an index. Any other object that happens to
// carry a key like it is left exactly as the generic rule leaves it, whatever its casing.
func TestResolveRedactedActions_AnObjectThatIsNotAnActionIsLeftAlone(t *testing.T) {
	const variables = `{"targets":[{"SECRET_URLS":["http://a"],"Secret_Urls":[]},` +
		`{"name":"x","SECRET_URLS":["http://c"],"Secret_Urls":[]},` +
		`{"index":1,"SECRET_URLS":["http://d"],"Secret_Urls":[]}],"other":{"SECRET_URLS":["http://b"]}}`

	got, found := resolveActionsArchive(t, variables, `{}`)

	assert.JSONEq(t, variables, got)
	assert.Zero(t, found)
}

// An entry of the actions list is known by where it is, not by what it holds: an archive that leaves
// out the index (apply reads it as 0) still states an action, so a placeholder with nothing stored
// must not become the explicit clear there.
func TestResolveRedactedActions_AnEntryWithoutAnIndexNeverBecomesAClear(t *testing.T) {
	for name, archive := range map[string]string{
		"no index":           `{"actions":[{"name":"out_on_url","secret_urls":["[REDACTED]"]}]}`,
		"no name":            `{"actions":[{"index":0,"secret_urls":["[REDACTED]","[REDACTED]"]}]}`,
		"neither":            `{"actions":[{"secret_urls":"[REDACTED]"}]}`,
		"upper case, no idx": `{"ACTIONS":[{"NAME":"out_on_url","SECRET_URLS":["[REDACTED]"]}]}`,
		"no key but a mark":  `{"actions":[{"secret_urls":["[REDACTED]"],"Secret_Urls":["[REDACTED]"]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			got, found := resolveActionsArchive(t, archive, `{}`)

			assert.NotContains(t, got, "[]", "the placeholder became the explicit clear")
			assert.NotContains(t, strings.ToLower(got), "secret_urls", "a list with nothing stored must lose its key")
			assert.Positive(t, found)
		})
	}
}

// The action rule is for the list under the top-level actions key of a configuration, and nowhere
// else. A settings or variables object, and an actions list below the top level of a configuration,
// are resolved byte for byte as the generic rule resolves them.
func TestResolveRedactedActions_OnlyTheTopLevelListOfAConfigurationIsAnActionList(t *testing.T) {
	for name, archive := range map[string]string{
		"under a section":        `{"config":{"actions":[{"name":"x","secret_urls":["[REDACTED]"]}]}}`,
		"under a section, upper": `{"config":{"ACTIONS":[{"SECRET_URLS":["[REDACTED]"]}]}}`,
		"in a list":              `{"x":[{"actions":[{"secret_urls":["[REDACTED]"]}]}]}`,
		"a bare object":          `{"actions":{"secret_urls":["[REDACTED]"]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var stored, nested, generic map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(storedActions), &stored))
			require.NoError(t, json.Unmarshal([]byte(archive), &nested))
			require.NoError(t, json.Unmarshal([]byte(archive), &generic))

			found := resolveConfiguration(nested, stored)

			assert.Equal(t, resolveRedacted(generic, stored), found)
			assert.Equal(t, generic, nested)
		})
	}
}

// The action rule does not reach below an action entry, nor into a list inside the actions list: both
// are generic.
func TestResolveRedactedActions_TheRuleStopsAtTheEntry(t *testing.T) {
	got, _ := resolveActionsArchive(t,
		`{"actions":[{"name":"a","index":0,"extra":[{"secret_urls":["[REDACTED]"]}]},[{"secret_urls":["[REDACTED]"]}]]}`, `{}`)

	assert.JSONEq(t, `{"actions":[{"name":"a","index":0,"extra":[{"secret_urls":[]}]},[{"secret_urls":[]}]]}`, got)
}

// Two stored spellings of the actions key are no stored list: a placeholder takes nothing and loses
// its key, whichever spelling the archive uses.
func TestResolveRedactedActions_TwoStoredSpellingsOfTheListAreNoList(t *testing.T) {
	stored := `{"actions":[{"name":"a","index":0,"secret_urls":["http://a"]}],` +
		`"ACTIONS":[{"name":"a","index":0,"secret_urls":["http://b"]}]}`

	got, _ := resolveActionsArchive(t, `{"actions":[{"name":"a","index":0,"secret_urls":["[REDACTED]"]}]}`, stored)

	assert.JSONEq(t, `{"actions":[{"name":"a","index":0}]}`, got)
}

// The configuration route of the archive applies the rule end to end: an entry without an index, with
// nothing stored, ends with no URL key and never with the explicit clear.
func TestRestoreConfiguration_AnEntryWithoutAnIndexLosesItsKey(t *testing.T) {
	for _, key := range []string{"actions", "ACTIONS"} {
		var a map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(`{"`+key+`":[{"secret_urls":["[REDACTED]"]}]}`), &a))

		got, err := restoreConfiguration(a, `{}`, &ImportApplyResult{})
		require.NoError(t, err)

		assert.JSONEq(t, `{"`+key+`":[{}]}`, got)
	}
}

// A settings or a variables object goes through restoreObject, which never runs the action rule: the
// same actions list that loses its key in a configuration keeps the generic result there, an empty
// list for a placeholder with nothing stored.
func TestRestoreObject_NeverRunsTheActionRule(t *testing.T) {
	const archive = `{"actions":[{"name":"x","secret_urls":["[REDACTED]"]}]}`
	var a map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(archive), &a))

	got, err := restoreObject(a, `{}`, &ImportApplyResult{})
	require.NoError(t, err)

	assert.JSONEq(t, `{"actions":[{"name":"x","secret_urls":[]}]}`, got)
}

// A stored list under another spelling is the stored list, so a placeholder takes it, and a stored
// empty list under another spelling is kept as the clear it is. Two spellings in the stored entry are
// no list: nothing is taken and the key is lost.
func TestResolveRedactedActions_TheStoredKeyIsFoundInAnyCasing(t *testing.T) {
	cases := map[string]struct{ archive, stored, want string }{
		"a stored list": {
			`{"actions":[{"name":"a","index":0,"SECRET_URLS":["[REDACTED]"]}]}`,
			`{"actions":[{"name":"a","index":0,"SECRET_URLS":["http://a"]}]}`,
			`{"actions":[{"name":"a","index":0,"SECRET_URLS":["http://a"]}]}`,
		},
		"a stored clear under the bare placeholder": {
			`{"actions":[{"name":"a","index":0,"secret_urls":"[REDACTED]"}]}`,
			`{"actions":[{"name":"a","index":0,"Secret_Urls":[]}]}`,
			`{"actions":[{"name":"a","index":0,"secret_urls":[]}]}`,
		},
		"two stored spellings": {
			`{"actions":[{"name":"a","index":0,"secret_urls":["[REDACTED]"]}]}`,
			`{"actions":[{"name":"a","index":0,"secret_urls":["http://a"],"SECRET_URLS":["http://b"]}]}`,
			`{"actions":[{"name":"a","index":0}]}`,
		},
		"a fully upper case entry with nothing stored": {
			`{"actions":[{"NAME":"a","INDEX":0,"SECRET_URLS":["[REDACTED]","[REDACTED]"]}]}`,
			`{}`,
			`{"actions":[{"NAME":"a","INDEX":0}]}`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, _ := resolveActionsArchive(t, tc.archive, tc.stored)
			assert.JSONEq(t, tc.want, got)
		})
	}
}

// An entry that holds the key in two casings is one the decoder and this reading could not agree on,
// so it loses all of them and apply leaves the device's list alone.
func TestResolveRedactedActions_TheKeyInTwoCasingsIsDropped(t *testing.T) {
	stored := `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["http://a:hunter2@h/1"]}]}`
	for name, keys := range map[string]string{
		"exact and upper": `"secret_urls":["http://new/1"],"SECRET_URLS":` + listShape,
		"two variants":    `"Secret_urls":["http://new/1"],"SECRET_URLS":["http://new/2"]`,
		"exact and clear": `"secret_urls":[],"Secret_Urls":` + listShape,
	} {
		t.Run(name, func(t *testing.T) {
			got, _ := resolveActionsArchive(t, `{"actions":[{"name":"btn_on_url","index":0,`+keys+`}]}`, stored)
			assert.JSONEq(t, `{"actions":[{"name":"btn_on_url","index":0}]}`, got)
		})
	}
}

func TestResolveRedactedActions_NothingStoredLosesTheKeyAndNeverClears(t *testing.T) {
	for _, shape := range []string{listShape, bareShape} {
		got, found := resolveActionsArchive(t, `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":`+shape+`}]}`, `{}`)

		assert.JSONEq(t, `{"actions":[{"name":"btn_on_url","index":0}]}`, got, "an empty list would be the explicit clear")
		assert.Positive(t, found)
	}
}

func TestResolveRedactedActions_ExportedConfigRoundTrips(t *testing.T) {
	var exported, current map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(storedActions), &exported))
	require.NoError(t, json.Unmarshal([]byte(storedActions), &current))
	redact.Mask(exported, RedactedValue)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "hunter2", "the export must hide the URL")

	resolveConfiguration(exported, current)

	raw, err = json.Marshal(exported)
	require.NoError(t, err)
	assert.JSONEq(t, storedActions, string(raw))
}

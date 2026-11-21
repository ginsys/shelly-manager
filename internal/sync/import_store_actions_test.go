package sync

import (
	"encoding/json"
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
	found := resolveRedacted(a, s)
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

	resolveRedacted(exported, current)

	raw, err = json.Marshal(exported)
	require.NoError(t, err)
	assert.JSONEq(t, storedActions, string(raw))
}

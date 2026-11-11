package sync

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/redact"
)

// An archive written by hand can hold the placeholder as a bare list element. The exporter never
// writes one, but the import must still never store the placeholder.
// What the export writes for a stored configuration comes back as the stored one: the exporter
// masks with redact.Mask and the import resolves against the same stored object, numbers included.
func TestResolveRedacted_ExportedConfigRoundTrips(t *testing.T) {
	const stored = `{"psk":[12345678],"api_tokens":["one","two"],"mqtt":{"pass":"p","port":1883},"auth":{"key":7}}`
	var exported, current map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stored), &exported))
	require.NoError(t, json.Unmarshal([]byte(stored), &current))
	redact.Mask(exported, RedactedValue)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "12345678", "the export must hide the number")

	// psk (1), api_tokens (2), mqtt.pass (1) and auth.key (1).
	assert.Equal(t, 5, resolveRedacted(exported, current))
	raw, err = json.Marshal(exported)
	require.NoError(t, err)
	assert.JSONEq(t, stored, string(raw))
}

func TestResolveRedactedStringElementsInLists(t *testing.T) {
	cases := []struct {
		name, archive, stored, want string
		found                       int
	}{
		{
			name:    "a stored element at the same position is kept",
			archive: `{"tokens":["[REDACTED]","plain"]}`,
			stored:  `{"tokens":["secret","old"]}`,
			want:    `{"tokens":["secret","plain"]}`,
			found:   1,
		},
		{
			name:    "no stored element drops the placeholder",
			archive: `{"tokens":["[REDACTED]","plain"]}`,
			stored:  `{}`,
			want:    `{"tokens":["plain"]}`,
			found:   1,
		},
		{
			name:    "a stored placeholder is not restored",
			archive: `{"tokens":["[REDACTED]"]}`,
			stored:  `{"tokens":["[REDACTED]"]}`,
			want:    `{"tokens":[]}`,
			found:   1,
		},
		{
			name:    "a stored value of another kind is not restored",
			archive: `{"tokens":["[REDACTED]"]}`,
			stored:  `{"tokens":[{"a":1}]}`,
			want:    `{"tokens":[]}`,
			found:   1,
		},
		{
			name:    "a stored number at the same position is kept",
			archive: `{"psk":["[REDACTED]"]}`,
			stored:  `{"psk":[12345678]}`,
			want:    `{"psk":[12345678]}`,
			found:   1,
		},
		{
			name:    "a stored number is kept when the key holds one",
			archive: `{"psk":"[REDACTED]"}`,
			stored:  `{"psk":12345678}`,
			want:    `{"psk":12345678}`,
			found:   1,
		},
		{
			name:    "a nested list is resolved too",
			archive: `{"groups":[["[REDACTED]"]]}`,
			stored:  `{"groups":[["secret"]]}`,
			want:    `{"groups":[["secret"]]}`,
			found:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var archive, stored map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(tc.archive), &archive))
			require.NoError(t, json.Unmarshal([]byte(tc.stored), &stored))

			found := resolveRedacted(archive, stored)

			assert.Equal(t, tc.found, found)
			raw, err := json.Marshal(archive)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(raw))
		})
	}
}

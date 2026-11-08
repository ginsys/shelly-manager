package sync

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An archive written by hand can hold the placeholder as a bare list element. The exporter never
// writes one, but the import must still never store the placeholder.
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

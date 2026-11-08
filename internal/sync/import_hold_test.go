package sync

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReleasePlaceholderAvoidsEveryAddressTheIndexTreatsAsEqual(t *testing.T) {
	cases := map[string][]string{
		"an exact match":                    {"import-hold-1"},
		"a case-insensitive match":          {"IMPORT-HOLD-1"},
		"a match with trailing pad spaces":  {"import-hold-1  "},
		"several matches in a row":          {"import-hold-1", "import-hold-1-1", "IMPORT-HOLD-1-2"},
		"a match for a different device id": {"import-hold-2"},
	}
	for name, addresses := range cases {
		t.Run(name, func(t *testing.T) {
			taken := map[string]bool{}
			for _, address := range addresses {
				taken[addressKey(address)] = true
			}

			hold := releasePlaceholder(1, taken)

			assert.False(t, hasAddress(addresses, hold), "%q must differ from every taken address", hold)
			assert.True(t, taken[addressKey(hold)], "the placeholder is reserved")
			assert.LessOrEqual(t, len(hold), 191)
		})
	}
}

func TestReleasePlaceholderStaysShortHoweverManyAreTaken(t *testing.T) {
	taken := map[string]bool{}
	var hold string
	for i := 0; i < 500; i++ {
		hold = releasePlaceholder(1, taken)
	}
	assert.LessOrEqual(t, len(hold), 191)
	assert.True(t, strings.HasPrefix(hold, "import-hold-1"))
	assert.Len(t, taken, 500)
}

func hasAddress(addresses []string, candidate string) bool {
	for _, address := range addresses {
		if addressKey(address) == addressKey(candidate) {
			return true
		}
	}
	return false
}

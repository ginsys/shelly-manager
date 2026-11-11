package sync

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A placeholder that is a function of the device id can be spelled by a stored free-text address,
// and a collation the code does not model (accent-insensitive utf8mb4_general_ci) can then equate
// the two. A random suffix removes that whole class: a stored or archived value matches it only by
// a 2^-128 chance.
func TestReleasePlaceholderIsUnguessable(t *testing.T) {
	shape := regexp.MustCompile(`^import-hold-1-[0-9a-f]{32}$`)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		hold := releasePlaceholder(1)

		assert.Regexp(t, shape, hold)
		assert.LessOrEqual(t, len(hold), 191, "the ip column holds 191 characters")
		assert.False(t, seen[hold], "%q was drawn twice", hold)
		seen[hold] = true
	}
}

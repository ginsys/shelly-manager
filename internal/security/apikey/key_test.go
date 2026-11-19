package apikey

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKey_ZeroValueIsUnset(t *testing.T) {
	var k Key
	require.Equal(t, "", k.Get())
}

func TestKey_SetThenGet(t *testing.T) {
	var k Key
	k.Set("one")
	require.Equal(t, "one", k.Get())
	k.Set("two")
	require.Equal(t, "two", k.Get())
	k.Set("")
	require.Equal(t, "", k.Get())
}

// Share makes two handles one key: a Set through either is seen by both, the sharing handle's own
// earlier value is dropped and the source's value stays.
func TestKey_ShareMakesOneKey(t *testing.T) {
	var src, other Key
	src.Set("from-src")
	other.Set("own-earlier")

	other.Share(&src)

	require.Equal(t, "from-src", other.Get())
	src.Set("changed-on-src")
	require.Equal(t, "changed-on-src", other.Get())
	other.Set("changed-on-other")
	require.Equal(t, "changed-on-other", src.Get())
}

// A source that was never set still shares: the handle that shared with it sees the first Set.
func TestKey_ShareWithAnUnsetSource(t *testing.T) {
	var src, other Key
	other.Share(&src)
	require.Equal(t, "", other.Get())
	src.Set("later")
	require.Equal(t, "later", other.Get())
}

// Handles that start sharing and changing while others read must stay race-free (run with -race), and
// goroutines that all use a fresh Key first end up on one cell.
func TestKey_ConcurrentUse(t *testing.T) {
	var src Key
	handles := make([]Key, 8)
	var wg sync.WaitGroup
	for i := range handles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handles[i].Share(&src)
			for range 200 {
				handles[i].Get()
				src.Set("a")
				src.Set("b")
			}
		}()
	}
	wg.Wait()

	src.Set("final")
	for i := range handles {
		require.Equal(t, "final", handles[i].Get())
	}
}

func TestKey_FirstUseRacesOnOneCell(t *testing.T) {
	var k Key
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				k.Set("x")
			} else {
				k.Get()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, "x", k.Get())
}

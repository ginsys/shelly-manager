package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// database/sql starts one connectionOpener goroutine for every pool it opens, and
// only DB.Close ends it. A Connect that fails and leaves its pool open leaves that
// goroutine, and its idle connections, behind (ginsys/shelly-manager#602).
//
// The observer reads goroutine ids, never a count: a pool another test opened is in
// the baseline or leaves it, so only a pool created after the baseline and not
// closed can fail a check. This package runs no t.Parallel test, so no other test
// runs between the baseline and the check.
const openerFrame = "database/sql.(*DB).connectionOpener"

// openerGoroutineIDs returns the ids of the goroutines that run a pool's opener.
func openerGoroutineIDs(t *testing.T) map[int]bool {
	t.Helper()

	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}

	ids := map[int]bool{}
	for _, block := range strings.Split(string(buf), "\n\n") {
		if !strings.Contains(block, openerFrame) {
			continue
		}
		var id int
		_, err := fmt.Sscanf(block, "goroutine %d [", &id)
		require.NoError(t, err, "a goroutine header this reader cannot parse: %.60s", block)
		ids[id] = true
	}
	return ids
}

// newOpenerGoroutines returns the opener goroutines that are not in before.
func newOpenerGoroutines(t *testing.T, before map[int]bool) []int {
	t.Helper()

	var fresh []int
	for id := range openerGoroutineIDs(t) {
		if !before[id] {
			fresh = append(fresh, id)
		}
	}
	return fresh
}

// requirePoolClosed fails unless every pool opened since before has been closed.
// The wait is a bound on the opener's exit after Close, not a timing assertion.
func requirePoolClosed(t *testing.T, before map[int]bool) {
	t.Helper()

	require.Eventually(t, func() bool {
		return len(newOpenerGoroutines(t, before)) == 0
	}, 3*time.Second, 20*time.Millisecond, "a pool opened by Connect was left open: its opener goroutine is still running")
}

// refusedAddress returns a loopback host and a port nobody listens on.
func refusedAddress(t *testing.T) (host string, port int) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port = listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return "127.0.0.1", port
}

type refusingConnector struct{}

func (refusingConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("refused")
}

func (refusingConnector) Driver() driver.Driver { return nil }

// The control for the observer: it sees a pool when one is open and sees it go on
// Close. Without this an empty result would be what a broken reader also returns.
func TestPoolOpenerObserver_SeesAnOpenPoolAndItsClose(t *testing.T) {
	before := openerGoroutineIDs(t)

	db := sql.OpenDB(refusingConnector{})
	require.Eventually(t, func() bool {
		return len(newOpenerGoroutines(t, before)) == 1
	}, 3*time.Second, 20*time.Millisecond, "the observer does not see an open pool")

	require.NoError(t, db.Close())
	requirePoolClosed(t, before)
}

package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// gatedCleanupDB holds the expired-device cleanup open until the test releases it, so the test
// controls the ordering the race depends on.
type gatedCleanupDB struct {
	database.DatabaseInterface
	started     chan struct{}
	release     chan struct{}
	done        chan struct{}
	releaseOnce sync.Once
	finished    atomic.Bool
}

// open lets the held cleanup run. It is safe to call more than once.
func (g *gatedCleanupDB) open() {
	g.releaseOnce.Do(func() { close(g.release) })
}

func (g *gatedCleanupDB) CleanupExpiredDiscoveredDevices() (int64, error) {
	defer close(g.done)
	close(g.started)
	<-g.release
	n, err := g.DatabaseInterface.CleanupExpiredDiscoveredDevices()
	g.finished.Store(true)
	return n, err
}

// TestGetDiscoveredDevicesCleanupIsAwaited pins ginsys/shelly-manager#366: the cleanup that
// GetDiscoveredDevices starts in the background must be something the owner of the database can
// wait for, so closing the database never races with it.
func TestGetDiscoveredDevicesCleanupIsAwaited(t *testing.T) {
	dbManager, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	gated := &gatedCleanupDB{
		DatabaseInterface: dbManager,
		started:           make(chan struct{}),
		release:           make(chan struct{}),
		done:              make(chan struct{}),
	}
	handler := &Handler{DB: gated, logger: logger}
	// Teardown must not rely on WaitBackground, the code under test. If an assertion fails first,
	// release the held cleanup and wait for it on its own signal before the database closes. This
	// defer runs before the database cleanup above.
	defer func() {
		gated.open()
		select {
		case <-gated.started:
			<-gated.done
		default: // the cleanup never started, so there is nothing to wait for
		}
	}()

	w := httptest.NewRecorder()
	handler.GetDiscoveredDevices(w, httptest.NewRequest("GET", "/api/v1/provisioner/discovered-devices", nil))
	require.Equal(t, http.StatusOK, w.Code)
	<-gated.started // the cleanup is running and held open

	waited := make(chan struct{})
	go func() {
		handler.WaitBackground()
		close(waited)
	}()

	select {
	case <-waited:
		t.Fatal("WaitBackground returned while the cleanup was still running")
	case <-time.After(100 * time.Millisecond):
	}

	gated.open()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitBackground did not return after the cleanup finished")
	}
	require.True(t, gated.finished.Load(), "WaitBackground returned before the cleanup finished")
}

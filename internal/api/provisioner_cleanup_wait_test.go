package api

import (
	"net/http"
	"net/http/httptest"
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
	started  chan struct{}
	release  chan struct{}
	finished atomic.Bool
}

func (g *gatedCleanupDB) CleanupExpiredDiscoveredDevices() (int64, error) {
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
	}
	handler := &Handler{DB: gated, logger: logger}

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

	close(gated.release)
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitBackground did not return after the cleanup finished")
	}
	require.True(t, gated.finished.Load(), "WaitBackground returned before the cleanup finished")
}

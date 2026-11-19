package metrics

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// The admin key changes while requests are checked against it (run with -race): the guard and the
// WebSocket token check both read the key that RotateAdminKey replaces (ginsys/shelly-manager#668).
func TestAdminKey_ChangesWhileRequestsRun(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	h := NewHandler(NewService(nil, logger, prometheus.NewRegistry()), logger)
	h.SetAdminAPIKey("key-a")

	var stop atomic.Bool
	var reads atomic.Int64
	var wg sync.WaitGroup
	check := map[string]func(){
		"requireAdmin": func() {
			r := httptest.NewRequest(http.MethodGet, "/x", nil)
			r.Header.Set("Authorization", "Bearer key-a")
			h.requireAdmin(httptest.NewRecorder(), r)
		},
		// A wrong token stops at the key check, before any upgrade is tried.
		"websocket": func() {
			r := httptest.NewRequest(http.MethodGet, "/ws?token=wrong", nil)
			h.HandleWebSocket(httptest.NewRecorder(), r)
		},
	}
	for _, fn := range check {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fn()
				reads.Add(1)
			}
		}()
	}

	// Keep changing the key until both checks have run many times alongside it.
	for i := 0; i < 500 || reads.Load() < 2000; i++ {
		if i%2 == 0 {
			h.SetAdminAPIKey("key-b")
		} else {
			h.SetAdminAPIKey("key-a")
		}
	}
	stop.Store(true)
	wg.Wait()
}

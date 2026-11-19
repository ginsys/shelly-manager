package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
)

// shareAdminKey wires the sub-handlers to the main handler's admin key, as main.go does at start-up.
func shareAdminKey(h *Handler) {
	h.ShareAdminKey()
}

// keyGuards builds one main handler with the export, import and metrics handlers attached, the way
// startServer assembles them, and returns each handler's own guard (ginsys/shelly-manager#668).
func keyGuards(t *testing.T) (*Handler, map[string]func(http.ResponseWriter, *http.Request) bool) {
	t.Helper()
	logger := logging.GetDefault()
	exp := NewSyncHandlers(nil, logger)
	imp := NewImportHandlers(nil, logger)
	h := &Handler{
		logger:         logger,
		ExportHandlers: exp,
		ImportHandlers: imp,
		MetricsHandler: metrics.NewHandler(nil, logger),
	}
	shareAdminKey(h)

	return h, map[string]func(http.ResponseWriter, *http.Request) bool{
		"Handler":        h.requireAdmin,
		"AgentOrAdmin":   h.requireAgentOrAdmin,
		"ImportHandlers": imp.requireAdmin,
		"SyncHandlers":   exp.requireAdmin,
	}
}

func presents(key string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	return r
}

// One key for every guard: a rotation through the main handler is seen by all of them at once, and
// the old key is refused by all of them (before this, the export and import handlers kept their own
// copy and the main handler's change did not reach them until their own setter ran).
func TestAdminKey_OneKeyForEveryGuard(t *testing.T) {
	h, guards := keyGuards(t)
	h.SetAdminAPIKey("key-one")

	for name, guard := range guards {
		require.True(t, guard(httptest.NewRecorder(), presents("key-one")), name+": accepts the key")
		require.False(t, guard(httptest.NewRecorder(), presents("wrong")), name+": refuses another key")
		require.False(t, guard(httptest.NewRecorder(), presents("")), name+": refuses no key")
	}

	body, err := json.Marshal(map[string]string{"new_key": "key-two"})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/admin/rotate-admin-key", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer key-one")
	rr := httptest.NewRecorder()
	h.RotateAdminKey(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	for name, guard := range guards {
		require.True(t, guard(httptest.NewRecorder(), presents("key-two")), name+": accepts the rotated key")
		require.False(t, guard(httptest.NewRecorder(), presents("key-one")), name+": refuses the old key")
	}
}

// A rotation while requests run must be race-free (run with -race): the key was a plain string field in
// four handlers, written by RotateAdminKey and read by every guarded request.
func TestRotateAdminKey_WhileRequestsRun(t *testing.T) {
	h, guards := keyGuards(t)
	keys := [2]string{"key-a", "key-b"}
	h.SetAdminAPIKey(keys[0])

	var stop atomic.Bool
	var wg sync.WaitGroup
	for _, guard := range guards {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				guard(httptest.NewRecorder(), presents(keys[0]))
				guard(httptest.NewRecorder(), presents(keys[1]))
			}
		}()
	}

	current := 0
	for i := 0; i < 200; i++ {
		next := 1 - current
		body, err := json.Marshal(map[string]string{"new_key": keys[next]})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/admin/rotate-admin-key", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+keys[current])
		rr := httptest.NewRecorder()
		h.RotateAdminKey(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		current = next
	}
	stop.Store(true)
	wg.Wait()

	for name, guard := range guards {
		require.True(t, guard(httptest.NewRecorder(), presents(keys[current])), name+": accepts the last key")
		require.False(t, guard(httptest.NewRecorder(), presents(keys[1-current])), name+": refuses the previous key")
	}
}

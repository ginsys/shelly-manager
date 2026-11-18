package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/provisioning"
)

// The agent registers once at start-up. When that failed, or the manager lost the registration, the next poll
// must register first instead of failing for ever.
func TestPollForTasks_RegistersBeforePolling(t *testing.T) {
	var registers, polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/provisioner/agents/register":
			registers.Add(1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"data":{"agent_id":"a","status":"registered"}}`))
		case "/api/v1/provisioner/agents/a/tasks":
			polls.Add(1)
			_, _ = w.Write([]byte(`{"success":true,"data":{"agent_id":"a","tasks":[],"count":0}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	saved := apiClient
	t.Cleanup(func() { apiClient = saved })
	apiClient = provisioning.NewAPIClient(server.URL, "k", "a", logger)

	require.NoError(t, pollForTasks(context.Background()), "a client that never registered registers, then polls")
	assert.EqualValues(t, 1, registers.Load())
	assert.EqualValues(t, 1, polls.Load())

	require.NoError(t, pollForTasks(context.Background()))
	assert.EqualValues(t, 1, registers.Load(), "an agent that is registered does not register again")
	assert.EqualValues(t, 2, polls.Load())
}

package provisioning

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The manager issues an agent token at registration; the client keeps it in memory and sends it on every call.
func TestAPIClient_AgentToken(t *testing.T) {
	testutil.SkipIfNoSocketPermissions(t)
	logger, err := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	// manager answers register with the given token (none when empty) and records the token header of each other call.
	newManager := func(registerStatus int, token string) (*APIClient, *[]string) {
		seen := []string{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/provisioner/agents/register" {
				if registerStatus != http.StatusCreated {
					w.WriteHeader(registerStatus)
					_, _ = w.Write([]byte("in use"))
					return
				}
				data := map[string]interface{}{"agent_id": "a", "status": "registered"}
				if token != "" {
					data["agent_token"] = token
				}
				writeEnvelope(t, w, http.StatusCreated, data)
				return
			}
			seen = append(seen, r.Header.Get("X-Agent-Token"))
			if r.Method == http.MethodGet {
				writeEnvelope(t, w, http.StatusOK, map[string]interface{}{"agent_id": "a", "tasks": []interface{}{}, "count": 0})
				return
			}
			writeEnvelope(t, w, http.StatusOK, map[string]interface{}{"task_id": "t", "status": "completed"})
		}))
		t.Cleanup(server.Close)
		return NewAPIClient(server.URL, "k", "a", logger), &seen
	}

	t.Run("the token from registration is sent on poll, status and report", func(t *testing.T) {
		client, seen := newManager(http.StatusCreated, "tok-1")
		require.NoError(t, client.EnsureRegistered("h", nil, nil))
		_, err := client.PollTasks()
		require.NoError(t, err)
		require.NoError(t, client.UpdateTaskStatus("t", "completed", nil, ""))
		require.NoError(t, client.ReportDiscoveredDevices("", []*DiscoveredDevice{{MAC: "AA"}}))
		assert.Equal(t, []string{"tok-1", "tok-1", "tok-1"}, *seen)
	})

	t.Run("a manager that issues no token gets none: an older manager still works", func(t *testing.T) {
		client, seen := newManager(http.StatusCreated, "")
		require.NoError(t, client.EnsureRegistered("h", nil, nil))
		_, err := client.PollTasks()
		require.NoError(t, err)
		assert.Equal(t, []string{""}, *seen)
	})

	t.Run("a 401 drops the token with the registration, so the next registration starts clean", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/provisioner/agents/register" {
				writeEnvelope(t, w, http.StatusCreated, map[string]interface{}{"agent_id": "a", "agent_token": "tok-1"})
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()
		client := NewAPIClient(server.URL, "k", "a", logger)
		require.NoError(t, client.EnsureRegistered("h", nil, nil))
		_, err := client.PollTasks()
		require.Error(t, err)
		assert.False(t, client.IsRegistered())
		assert.Empty(t, client.agentToken, "the refused token is dropped")
	})

	t.Run("a 403 keeps the registration: it is a valid token for another agent, and registering again cannot fix it", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/provisioner/agents/register" {
				writeEnvelope(t, w, http.StatusCreated, map[string]interface{}{"agent_id": "a", "agent_token": "tok-1"})
				return
			}
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()
		client := NewAPIClient(server.URL, "k", "a", logger)
		require.NoError(t, client.EnsureRegistered("h", nil, nil))
		_, err := client.PollTasks()
		require.Error(t, err)
		assert.True(t, client.IsRegistered())
		assert.Equal(t, "tok-1", client.agentToken)
	})

	t.Run("a 409 on register leaves the client unregistered, and the next EnsureRegistered tries again", func(t *testing.T) {
		client, _ := newManager(http.StatusConflict, "")
		err := client.EnsureRegistered("h", nil, nil)
		require.Error(t, err)
		var httpErr *HTTPError
		require.ErrorAs(t, err, &httpErr)
		assert.Equal(t, http.StatusConflict, httpErr.StatusCode)
		assert.False(t, client.IsRegistered())
		require.Error(t, client.EnsureRegistered("h", nil, nil), "it tries again rather than giving up")
	})
}

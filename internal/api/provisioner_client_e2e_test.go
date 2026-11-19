package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/provisioning"
)

// The provisioner agent and the manager are two programs that share only a wire format. These tests run the
// real provisioning.APIClient against the real router, so a change to either side that the other cannot read
// fails here. The client's own tests use hand-written mocks and cannot catch that.

func resetProvisionerRegistry(t *testing.T) {
	t.Helper()
	clear := func() {
		registry.mu.Lock()
		registry.agents = make(map[string]*ProvisionerAgent)
		registry.tasks = make(map[string]*ProvisioningTask)
		registry.mu.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

// adminCall sends one operator request (the admin key when the manager has one) and returns the body.
func adminCall(t *testing.T, srv *httptest.Server, adminKey, method, path, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if adminKey != "" {
		req.Header.Set("Authorization", "Bearer "+adminKey)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func TestProvisionerClient_AgainstTheRealRouter(t *testing.T) {
	cases := []struct {
		name               string
		adminKey, agentKey string
		clientKey          string
	}{
		{"no keys configured", "", "", ""},
		{"admin and agent key, client sends the agent key", "admin-secret", "agent-secret", "agent-secret"},
		{"admin key only, client sends the admin key", "admin-secret", "", "admin-secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetProvisionerRegistry(t)
			srv := httptest.NewServer(configRoutesRouterKeys(t, tc.adminKey, tc.agentKey))
			defer srv.Close()

			client := provisioning.NewAPIClient(srv.URL, tc.clientKey, "e2e-agent", logging.GetDefault())

			require.NoError(t, client.TestConnectivity(), "health, before registering")

			require.NoError(t, client.RegisterAgent("host-1", []string{"wifi-provisioning"}, map[string]string{"platform": "golang"}))
			assert.True(t, client.IsRegistered())
			code, body := adminCall(t, srv, tc.adminKey, "GET", "/api/v1/provisioner/agents", "")
			require.Equal(t, http.StatusOK, code)
			assert.Contains(t, body, "e2e-agent", "the manager lists the agent that registered")

			code, body = adminCall(t, srv, tc.adminKey, "POST", "/api/v1/provisioner/tasks", `{"type":"provision","device_mac":"AA:BB:CC:DD:EE:01"}`)
			require.Equal(t, http.StatusCreated, code, body)

			tasks, err := client.PollTasks()
			require.NoError(t, err)
			require.Len(t, tasks, 1, "the task the operator created reaches the agent")
			assert.Equal(t, "provision", tasks[0].Type)
			assert.Equal(t, "AA:BB:CC:DD:EE:01", tasks[0].DeviceMAC)
			assert.Equal(t, "assigned", tasks[0].Status)

			require.NoError(t, client.UpdateTaskStatus(tasks[0].ID, "completed", map[string]interface{}{"ok": true}, ""))
			_, body = adminCall(t, srv, tc.adminKey, "GET", "/api/v1/provisioner/tasks", "")
			var listed struct {
				Success bool `json:"success"`
				Data    struct {
					Tasks []struct {
						ID     string `json:"id"`
						Status string `json:"status"`
					} `json:"tasks"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal([]byte(body), &listed))
			assert.True(t, listed.Success, "the list answers in the envelope")
			require.Len(t, listed.Data.Tasks, 1)
			assert.Equal(t, "completed", listed.Data.Tasks[0].Status, "the status the agent sent is stored")

			require.NoError(t, client.ReportDiscoveredDevices(tasks[0].ID, []*provisioning.DiscoveredDevice{{
				MAC: "AA:BB:CC:DD:EE:02", SSID: "shelly1-DDEE02", Model: "SHSW-1", Generation: 1,
				IP: "192.168.33.1", Signal: -50, Discovered: time.Now(),
			}}))
			code, body = adminCall(t, srv, tc.adminKey, "GET", "/api/v1/provisioner/discovered-devices", "")
			require.Equal(t, http.StatusOK, code)
			assert.Contains(t, body, "AA:BB:CC:DD:EE:02", "the reported device is stored")
		})
	}
}

// A manager restart empties its in-memory registry. The agent's next poll is refused, it notices, registers again
// and polls on.
func TestProvisionerClient_RegistersAgainAfterAManagerRestart(t *testing.T) {
	resetProvisionerRegistry(t)
	srv := httptest.NewServer(configRoutesRouterKeys(t, "admin-secret", "agent-secret"))
	defer srv.Close()
	client := provisioning.NewAPIClient(srv.URL, "agent-secret", "e2e-agent", logging.GetDefault())

	require.NoError(t, client.EnsureRegistered("host-1", nil, nil))
	_, err := client.PollTasks()
	require.NoError(t, err)

	resetProvisionerRegistry(t) // the restart

	_, err = client.PollTasks()
	require.Error(t, err, "the manager no longer knows the agent")
	assert.False(t, client.IsRegistered(), "the client noticed")

	require.NoError(t, client.EnsureRegistered("host-1", nil, nil))
	code, _ := adminCall(t, srv, "admin-secret", "POST", "/api/v1/provisioner/tasks", `{"type":"provision"}`)
	require.Equal(t, http.StatusCreated, code)
	tasks, err := client.PollTasks()
	require.NoError(t, err)
	assert.Len(t, tasks, 1, "polling works again after registering")
}

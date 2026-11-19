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

// An agent proves who it is with the token the manager issued when it registered. These tests walk the three
// routes that take an agent or task id from the caller (poll, task status, report) with two agents, and the
// register rules that keep one agent from taking another's id.

const (
	idAdminKey = "admin-secret"
	idAgentKey = "agent-secret"
)

// idCall sends one request with the given headers and returns the status and body.
func idCall(t *testing.T, srv *httptest.Server, method, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func idAgentHdr(token string) map[string]string {
	h := map[string]string{"Authorization": "Bearer " + idAgentKey}
	if token != "" {
		h["X-Agent-Token"] = token
	}
	return h
}

func idAdminHdr() map[string]string {
	return map[string]string{"Authorization": "Bearer " + idAdminKey}
}

// idRegister registers id with the shared agent key and returns the status, the token the manager issued and the body.
func idRegister(t *testing.T, srv *httptest.Server, id string, headers map[string]string) (int, string, string) {
	t.Helper()
	code, body := idCall(t, srv, "POST", "/api/v1/provisioner/agents/register", `{"id":"`+id+`","hostname":"h-`+id+`"}`, headers)
	var out struct {
		Data struct {
			Token string `json:"agent_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	return code, out.Data.Token, body
}

func idMustRegister(t *testing.T, srv *httptest.Server, id string) string {
	t.Helper()
	code, token, body := idRegister(t, srv, id, idAgentHdr(""))
	require.Equal(t, http.StatusCreated, code, body)
	require.NotEmpty(t, token, "the manager issues a token at registration")
	return token
}

// idTask has the operator create a task, assigned to agentID when it is not empty, and returns its id.
func idTask(t *testing.T, srv *httptest.Server, agentID string) string {
	t.Helper()
	body := `{"type":"provision","device_mac":"AA:BB:CC:DD:EE:01"`
	if agentID != "" {
		body += `,"agent_id":"` + agentID + `"`
	}
	code, raw := idCall(t, srv, "POST", "/api/v1/provisioner/tasks", body+`}`, idAdminHdr())
	require.Equal(t, http.StatusCreated, code, raw)
	var out struct {
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	require.NotEmpty(t, out.Data.TaskID)
	return out.Data.TaskID
}

func idTaskStatus(t *testing.T, id string) string {
	t.Helper()
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	task, ok := registry.tasks[id]
	require.True(t, ok)
	return task.Status
}

func idLastSeen(t *testing.T, id string) time.Time {
	t.Helper()
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	agent, ok := registry.agents[id]
	require.True(t, ok)
	return agent.LastSeen
}

func idSetLastSeen(t *testing.T, id string, at time.Time) {
	t.Helper()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	agent, ok := registry.agents[id]
	require.True(t, ok)
	agent.LastSeen = at
}

func idServer(t *testing.T) *httptest.Server {
	t.Helper()
	resetProvisionerRegistry(t)
	srv := httptest.NewServer(configRoutesRouterKeys(t, idAdminKey, idAgentKey))
	t.Cleanup(srv.Close)
	return srv
}

func TestAgentIdentity_PollReturnsOnlyTheCallersOwnTasks(t *testing.T) {
	srv := idServer(t)
	tokenA := idMustRegister(t, srv, "agent-a")
	tokenB := idMustRegister(t, srv, "agent-b")
	taskA := idTask(t, srv, "agent-a")

	pollA := "/api/v1/provisioner/agents/agent-a/tasks"

	code, _ := idCall(t, srv, "GET", pollA, "", idAgentHdr(tokenB))
	assert.Equal(t, http.StatusForbidden, code, "agent B's token on agent A's path")
	assert.Equal(t, "pending", idTaskStatus(t, taskA), "the refused poll did not hand A's task to anyone")

	code, _ = idCall(t, srv, "GET", pollA, "", idAgentHdr(""))
	assert.Equal(t, http.StatusUnauthorized, code, "the shared key alone names no agent")

	code, _ = idCall(t, srv, "GET", pollA, "", idAgentHdr("not-a-token"))
	assert.Equal(t, http.StatusUnauthorized, code, "an unknown token")

	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/nobody/tasks", "", idAgentHdr(tokenA))
	assert.Equal(t, http.StatusForbidden, code, "a valid token on a path that is not its own")

	code, body := idCall(t, srv, "GET", pollA, "", idAgentHdr(tokenA))
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, taskA, "A's own token gets A's task")
	assert.Equal(t, "assigned", idTaskStatus(t, taskA))

	code, _ = idCall(t, srv, "GET", pollA, "", idAdminHdr())
	assert.Equal(t, http.StatusOK, code, "the admin key still polls any agent")
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/nobody/tasks", "", idAdminHdr())
	assert.Equal(t, http.StatusNotFound, code, "the admin key still gets 404 for an unknown agent")
}

func TestAgentIdentity_StatusOnlyOnTheCallersOwnTasks(t *testing.T) {
	srv := idServer(t)
	tokenA := idMustRegister(t, srv, "agent-a")
	tokenB := idMustRegister(t, srv, "agent-b")
	taskA := idTask(t, srv, "agent-a")
	unassigned := idTask(t, srv, "")
	put := func(task string, h map[string]string) int {
		code, _ := idCall(t, srv, "PUT", "/api/v1/provisioner/tasks/"+task+"/status", `{"status":"completed","agent_id":"agent-a"}`, h)
		return code
	}

	assert.Equal(t, http.StatusForbidden, put(taskA, idAgentHdr(tokenB)), "B marks A's task, naming A in the body")
	assert.Equal(t, "pending", idTaskStatus(t, taskA))
	assert.Equal(t, http.StatusUnauthorized, put(taskA, idAgentHdr("")), "the shared key alone")
	assert.Equal(t, http.StatusForbidden, put(unassigned, idAgentHdr(tokenA)), "a task nobody was assigned")
	assert.Equal(t, "pending", idTaskStatus(t, unassigned))
	assert.Equal(t, http.StatusOK, put(taskA, idAgentHdr(tokenA)), "A marks its own task")
	assert.Equal(t, "completed", idTaskStatus(t, taskA))
	assert.Equal(t, http.StatusOK, put(unassigned, idAdminHdr()), "the admin key still marks any task")
}

func TestAgentIdentity_ReportIsFiledUnderTheCallersOwnId(t *testing.T) {
	srv := idServer(t)
	tokenA := idMustRegister(t, srv, "agent-a")
	tokenB := idMustRegister(t, srv, "agent-b")
	taskA := idTask(t, srv, "agent-a")
	report := func(agent, task string, h map[string]string) int {
		code, _ := idCall(t, srv, "POST", "/api/v1/provisioner/discovered-devices", idReportBodyFor(agent, task), h)
		return code
	}

	assert.Equal(t, http.StatusForbidden, report("agent-a", "", idAgentHdr(tokenB)), "B files devices under A")
	assert.Equal(t, http.StatusUnauthorized, report("agent-a", "", idAgentHdr("")), "the shared key alone")

	assert.Equal(t, http.StatusOK, report("agent-b", taskA, idAgentHdr(tokenB)), "B reports under its own id, naming A's task")
	assert.Equal(t, "pending", idTaskStatus(t, taskA), "that report did not complete A's task")

	assert.Equal(t, http.StatusOK, report("agent-a", taskA, idAgentHdr(tokenA)))
	assert.Equal(t, "completed", idTaskStatus(t, taskA), "A's own report completes A's task")
}

func idReportBodyFor(agent, task string) string {
	body := `{"agent_id":"` + agent + `"`
	if task != "" {
		body += `,"task_id":"` + task + `"`
	}
	return body + `,"devices":[{"mac":"AA:BB:CC:DD:EE:02","ssid":"s","model":"SHSW-1","generation":1,"ip":"192.168.33.1","signal":-50,"discovered":"2026-11-19T00:00:00Z"}]}`
}

func TestAgentIdentity_RegisterKeepsAnOnlineIdWithItsOwner(t *testing.T) {
	srv := idServer(t)
	tokenA := idMustRegister(t, srv, "agent-a")

	code, token, body := idRegister(t, srv, "agent-a", idAgentHdr(""))
	assert.Equal(t, http.StatusConflict, code, "a second registrant of an online id without its token: %s", body)
	assert.Empty(t, token)
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", idAgentHdr(tokenA))
	assert.Equal(t, http.StatusOK, code, "the refused register left A's token working")

	code, rotated, body := idRegister(t, srv, "agent-a", idAgentHdr(tokenA))
	require.Equal(t, http.StatusCreated, code, "the owner registers again with its token: %s", body)
	require.NotEmpty(t, rotated)
	assert.NotEqual(t, tokenA, rotated)
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", idAgentHdr(tokenA))
	assert.Equal(t, http.StatusUnauthorized, code, "the old token died")

	code, _, body = idRegister(t, srv, "agent-a", idAdminHdr())
	assert.Equal(t, http.StatusCreated, code, "the admin key may take an id over: %s", body)
}

func TestAgentIdentity_RegisterSucceedsOnceTheOldRecordIsOffline(t *testing.T) {
	srv := idServer(t)
	idMustRegister(t, srv, "agent-a")

	idSetLastSeen(t, "agent-a", time.Now().Add(-4*time.Minute))
	code, _, body := idRegister(t, srv, "agent-a", idAgentHdr(""))
	assert.Equal(t, http.StatusConflict, code, "silent for 4 minutes is still online: %s", body)

	idSetLastSeen(t, "agent-a", time.Now().Add(-6*time.Minute))
	code, token, body := idRegister(t, srv, "agent-a", idAgentHdr(""))
	require.Equal(t, http.StatusCreated, code, "silent for 6 minutes is offline: %s", body)
	assert.NotEmpty(t, token)
}

// A refused call proves nothing about the agent, so it must not keep the record online: otherwise a holder of the
// shared key could keep an id fresh with bad tokens and the real agent's register would be refused for good.
func TestAgentIdentity_ARefusedCallDoesNotMoveLastSeen(t *testing.T) {
	srv := idServer(t)
	tokenB := idMustRegister(t, srv, "agent-b")
	idMustRegister(t, srv, "agent-a")
	old := time.Now().Add(-10 * time.Minute)
	idSetLastSeen(t, "agent-a", old)

	code, _ := idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", idAgentHdr(""))
	require.Equal(t, http.StatusUnauthorized, code)
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", idAgentHdr("bad-token"))
	require.Equal(t, http.StatusUnauthorized, code)
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", idAgentHdr(tokenB))
	require.Equal(t, http.StatusForbidden, code)
	code, _ = idCall(t, srv, "POST", "/api/v1/provisioner/discovered-devices", idReportBodyFor("agent-a", ""), idAgentHdr(""))
	require.Equal(t, http.StatusUnauthorized, code)
	code, _ = idCall(t, srv, "POST", "/api/v1/provisioner/discovered-devices", idReportBodyFor("agent-a", ""), idAgentHdr(tokenB))
	require.Equal(t, http.StatusForbidden, code)

	assert.True(t, idLastSeen(t, "agent-a").Equal(old), "no refused call moved agent A's LastSeen")

	code, _, body := idRegister(t, srv, "agent-a", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, "so the real agent can still register once it is 5 minutes silent: %s", body)
}

// The real client holds the token the real manager issued, so two real agents on one manager cannot read each other's tasks.
func TestAgentIdentity_TwoRealClientsStayApart(t *testing.T) {
	srv := idServer(t)
	a := provisioning.NewAPIClient(srv.URL, idAgentKey, "agent-a", logging.GetDefault())
	b := provisioning.NewAPIClient(srv.URL, idAgentKey, "agent-b", logging.GetDefault())
	require.NoError(t, a.EnsureRegistered("host-a", nil, nil))
	require.NoError(t, b.EnsureRegistered("host-b", nil, nil))
	taskA := idTask(t, srv, "agent-a")

	tasksB, err := b.PollTasks()
	require.NoError(t, err)
	assert.Empty(t, tasksB, "B's own poll does not return A's task")
	assert.Equal(t, "pending", idTaskStatus(t, taskA))
	require.Error(t, b.UpdateTaskStatus(taskA, "completed", nil, ""), "B cannot mark A's task")
	assert.Equal(t, "pending", idTaskStatus(t, taskA))
	assert.True(t, b.IsRegistered(), "a 403 does not turn B into a re-registering loop")

	tasksA, err := a.PollTasks()
	require.NoError(t, err)
	require.Len(t, tasksA, 1)
	require.NoError(t, a.UpdateTaskStatus(taskA, "completed", nil, ""))
	assert.Equal(t, "completed", idTaskStatus(t, taskA))
}

// A restarted agent keeps its id inside a container. The manager still holds the old record, so the new process is
// refused until that record is 5 minutes silent, then registers and gets a new token.
func TestAgentIdentity_ARestartedRealAgentWaitsForItsOldRecord(t *testing.T) {
	srv := idServer(t)
	old := provisioning.NewAPIClient(srv.URL, idAgentKey, "agent-a", logging.GetDefault())
	require.NoError(t, old.EnsureRegistered("host-a", nil, nil))

	fresh := provisioning.NewAPIClient(srv.URL, idAgentKey, "agent-a", logging.GetDefault())
	err := fresh.EnsureRegistered("host-a", nil, nil)
	require.Error(t, err, "the old record is still online")
	assert.False(t, fresh.IsRegistered())

	idSetLastSeen(t, "agent-a", time.Now().Add(-6*time.Minute))
	require.NoError(t, fresh.EnsureRegistered("host-a", nil, nil))
	_, err = fresh.PollTasks()
	require.NoError(t, err)
	_, err = old.PollTasks()
	require.Error(t, err, "the old process's token died")
}

func TestAgentIdentity_AgentListNeverShowsTokens(t *testing.T) {
	srv := idServer(t)
	token := idMustRegister(t, srv, "agent-a")

	code, body := idCall(t, srv, "GET", "/api/v1/provisioner/agents", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "agent-a")
	assert.NotContains(t, body, token, "the token is not in the agent list")
	assert.NotContains(t, strings.ToLower(body), "token")
}

// With no admin key the manager is open by design (the default of every guarded route): nothing is enforced, but
// registration still hands out a token so an agent that sends it works the same on a keyed manager.
func TestAgentIdentity_NoAdminKeyKeepsTheRoutesOpen(t *testing.T) {
	resetProvisionerRegistry(t)
	srv := httptest.NewServer(configRoutesRouterKeys(t, "", ""))
	defer srv.Close()

	code, token, body := idRegister(t, srv, "agent-a", nil)
	require.Equal(t, http.StatusCreated, code, body)
	assert.NotEmpty(t, token)
	code, _, _ = idRegister(t, srv, "agent-a", nil)
	assert.Equal(t, http.StatusCreated, code, "no 409 rule without an admin key")

	taskID := idTask2(t, srv)
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", nil)
	assert.Equal(t, http.StatusOK, code, "poll needs no token")
	code, _ = idCall(t, srv, "PUT", "/api/v1/provisioner/tasks/"+taskID+"/status", `{"status":"completed"}`, nil)
	assert.Equal(t, http.StatusOK, code, "status needs no token")
	code, _ = idCall(t, srv, "POST", "/api/v1/provisioner/discovered-devices", idReportBodyFor("agent-a", ""), nil)
	assert.Equal(t, http.StatusOK, code, "report needs no token")
}

// idTask2 creates an unassigned task on a manager that has no admin key.
func idTask2(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	code, raw := idCall(t, srv, "POST", "/api/v1/provisioner/tasks", `{"type":"provision"}`, nil)
	require.Equal(t, http.StatusCreated, code, raw)
	var out struct {
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out.Data.TaskID
}

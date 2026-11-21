package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The provisioner registry lives in the database: a handler holds no agent, token or task of its own, so a
// second handler on the same database (a restart, or another replica) sees what the first one stored.

func registryDeadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// seedProvisionerAgent writes an agent row, as a register would have. Its token is "token-of-<id>".
func seedProvisionerAgent(t *testing.T, db *database.Manager, agentID string) {
	t.Helper()
	now := time.Now()
	require.NoError(t, db.GetDB().Create(&database.ProvisionerAgent{
		AgentID: agentID, TokenHash: database.HashAgentToken("token-of-" + agentID),
		Hostname: "host-" + agentID, IP: "127.0.0.1", Status: "online", LastSeen: now, RegisteredAt: now,
	}).Error)
}

// seedProvisionerTask writes a task row, as a create would have.
func seedProvisionerTask(t *testing.T, db *database.Manager, task database.ProvisioningTask) {
	t.Helper()
	require.NoError(t, db.GetDB().Create(&task).Error)
}

func TestProvisionerRegistry_LivesInTheDatabase(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	srv := httptest.NewServer(configRoutesRouterOn(t, db, idAdminKey, idAgentKey))
	defer srv.Close()

	token := idMustRegister(t, srv, "agent-a")
	taskID := idTask(t, srv, "agent-a")

	store := database.NewProvisionerStore(db.GetDB())
	agent, err := store.GetAgent(registryDeadline(t), "agent-a")
	require.NoError(t, err, "a register writes a row")
	assert.Equal(t, database.HashAgentToken(token), agent.TokenHash)
	assert.NotEqual(t, token, agent.TokenHash, "the table holds the hash of the token, never the token")
	assert.Equal(t, "online", agent.Status)

	task, err := store.GetTask(registryDeadline(t), taskID)
	require.NoError(t, err, "a created task writes a row")
	assert.Equal(t, "pending", task.Status)
	assert.Equal(t, "agent-a", task.AgentID)
}

func TestProvisionerRegistry_SurvivesAManagerRestart(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)

	first := httptest.NewServer(configRoutesRouterOn(t, db, idAdminKey, idAgentKey))
	token := idMustRegister(t, first, "agent-a")
	taskID := idTask(t, first, "agent-a")
	first.Close()

	// A new handler on the same database is a restarted manager.
	second := httptest.NewServer(configRoutesRouterOn(t, db, idAdminKey, idAgentKey))
	defer second.Close()

	code, body := idCall(t, second, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", idAgentHdr(token))
	require.Equal(t, http.StatusOK, code, "the agent's token still works: %s", body)
	assert.Contains(t, body, taskID, "the task created before the restart reaches the agent")

	code, _, body = idRegister(t, second, "agent-a", idAgentHdr(""))
	assert.Equal(t, http.StatusConflict, code, "an online id still belongs to the holder of its token: %s", body)
}

func TestProvisionerRegistry_RefusesValuesTheColumnsCannotHold(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	srv := httptest.NewServer(configRoutesRouterOn(t, db, idAdminKey, idAgentKey))
	defer srv.Close()

	long := strings.Repeat("a", 192)
	exact := strings.Repeat("a", 191)
	// 191 characters and 370 bytes. A new id of non-ASCII characters needs the "provisioner-" prefix.
	multibyte := "provisioner-" + strings.Repeat("é", 179)

	register := func(id string) (int, string) {
		code, _, body := idRegister(t, srv, id, idAgentHdr(""))
		return code, body
	}
	code, body := register(long)
	assert.Equal(t, http.StatusBadRequest, code, body)
	assert.Contains(t, body, "191 characters", "the answer names the limit")
	code, body = register(exact)
	assert.Equal(t, http.StatusCreated, code, body)
	code, body = register(multibyte)
	assert.Equal(t, http.StatusCreated, code, "the limit counts characters, not bytes: %s", body)

	create := func(body string) (int, string) {
		return idCall(t, srv, "POST", "/api/v1/provisioner/tasks", body, idAdminHdr())
	}
	code, body = create(`{"type":"` + long + `"}`)
	assert.Equal(t, http.StatusBadRequest, code, "a task type over the limit: %s", body)
	code, body = create(`{"type":"provision","agent_id":"` + long + `"}`)
	assert.Equal(t, http.StatusBadRequest, code, "a task agent id over the limit: %s", body)
	code, body = create(`{"type":"` + exact + `"}`)
	assert.Equal(t, http.StatusCreated, code, body)

	code, body = idCall(t, srv, "POST", "/api/v1/provisioning/tasks", `{"taskType":"`+long+`"}`, idAdminHdr())
	assert.Equal(t, http.StatusBadRequest, code, "the UI route has the same limit: %s", body)

	taskID := idTask(t, srv, "")
	code, body = idCall(t, srv, "PUT", "/api/v1/provisioner/tasks/"+taskID+"/status", `{"status":"`+long+`"}`, idAdminHdr())
	assert.Equal(t, http.StatusBadRequest, code, "a status over the limit: %s", body)
	assert.Equal(t, "pending", idTaskStatus(t, db, taskID), "the refused status was not stored")
}

// A caller who is refused gets its 401 or 403, never the 400 for a long status.
func TestProvisionerRegistry_LongStatusDoesNotReplaceAnIdentityRefusal(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	srv := httptest.NewServer(configRoutesRouterOn(t, db, idAdminKey, idAgentKey))
	defer srv.Close()

	idMustRegister(t, srv, "agent-a")
	other := idMustRegister(t, srv, "agent-b")
	taskID := idTask(t, srv, "agent-a")
	path := "/api/v1/provisioner/tasks/" + taskID + "/status"
	body := `{"status":"` + strings.Repeat("a", 192) + `"}`

	code, got := idCall(t, srv, "PUT", path, body, idAgentHdr(""))
	assert.Equal(t, http.StatusUnauthorized, code, "no agent token: %s", got)
	code, got = idCall(t, srv, "PUT", path, body, idAgentHdr(other))
	assert.Equal(t, http.StatusForbidden, code, "another agent's token: %s", got)
}

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The registry forgets in two ways: an admin deletes an agent, and a purge that a register starts removes
// agents silent for 30 days and tasks that finished 30 days ago.

func purgeRows(t *testing.T, db *database.Manager) (agents map[string]bool, tasks map[string]database.ProvisioningTask) {
	t.Helper()
	store := database.NewProvisionerStore(db.GetDB())
	list, err := store.ListAgents(registryDeadline(t))
	require.NoError(t, err)
	agents = map[string]bool{}
	for _, a := range list {
		agents[a.AgentID] = true
	}
	all, err := store.ListTasks(registryDeadline(t))
	require.NoError(t, err)
	tasks = map[string]database.ProvisioningTask{}
	for _, task := range all {
		tasks[task.TaskID] = task
	}
	return agents, tasks
}

func TestProvisionerRegistry_DeleteFreesTheIdAndRevokesTheToken(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	srv := httptest.NewServer(configRoutesRouterOn(t, db, idAdminKey, idAgentKey))
	defer srv.Close()

	token := idMustRegister(t, srv, "agent-a")
	other := idMustRegister(t, srv, "agent-b")
	pending := idTask(t, srv, "agent-a")
	seedProvisionerTask(t, db, database.ProvisioningTask{TaskID: "stranded", Type: "provision", Status: "assigned", AgentID: "agent-a"})
	seedProvisionerTask(t, db, database.ProvisioningTask{TaskID: "done", Type: "provision", Status: "completed", AgentID: "agent-a"})

	for _, h := range []map[string]string{nil, idAgentHdr(""), idAgentHdr(token)} {
		code, body := idCall(t, srv, "DELETE", "/api/v1/provisioner/agents/agent-a", "", h)
		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, code, "only the admin key deletes: %s", body)
	}
	code, body := idCall(t, srv, "DELETE", "/api/v1/provisioner/agents/nobody", "", idAdminHdr())
	assert.Equal(t, http.StatusNotFound, code, body)

	code, body = idCall(t, srv, "DELETE", "/api/v1/provisioner/agents/agent-a", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)

	code, body = idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-a/tasks", "", idAgentHdr(token))
	assert.Equal(t, http.StatusUnauthorized, code, "the deleted agent's token no longer works: %s", body)
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/agent-b/tasks", "", idAgentHdr(other))
	assert.Equal(t, http.StatusOK, code, "another agent is untouched")

	agents, tasks := purgeRows(t, db)
	assert.False(t, agents["agent-a"])
	assert.True(t, agents["agent-b"])
	assert.Equal(t, "failed", tasks["stranded"].Status, "an assigned task is failed, with the reason")
	assert.NotEmpty(t, tasks["stranded"].Config["_error"])
	assert.Equal(t, "completed", tasks["done"].Status)
	assert.Equal(t, "pending", tasks[pending].Status, "a pending task stays for whoever holds the id next")

	code, newToken, body := idRegister(t, srv, "agent-a", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, "the id is free at once, however recently it was seen: %s", body)
	assert.NotEqual(t, token, newToken)
}

func TestProvisionerRegistry_RegisterPurgesWhatHasGoneStale(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	router, handler := configRoutesHandlerOn(t, db, idAdminKey, idAgentKey)
	srv := httptest.NewServer(router)
	defer srv.Close()

	old := time.Now().Add(-31 * 24 * time.Hour)
	recent := time.Now().Add(-29 * 24 * time.Hour)
	for _, id := range []string{"silent", "silent-pending", "silent-assigned", "recent"} {
		seedProvisionerAgent(t, db, id)
	}
	for _, id := range []string{"silent", "silent-pending", "silent-assigned"} {
		idSetLastSeen(t, db, id, old)
	}
	idSetLastSeen(t, db, "recent", recent)
	for _, task := range []database.ProvisioningTask{
		{TaskID: "pending-of-silent", Status: "pending", AgentID: "silent-pending", UpdatedAt: old},
		{TaskID: "assigned-of-silent", Status: "assigned", AgentID: "silent-assigned", UpdatedAt: old},
		{TaskID: "old-completed", Status: "completed", UpdatedAt: old},
		{TaskID: "old-failed", Status: "failed", UpdatedAt: old},
		{TaskID: "recent-completed", Status: "completed", UpdatedAt: recent},
		{TaskID: "old-pending", Status: "pending", UpdatedAt: old},
	} {
		task := task
		task.Type = "provision"
		seedProvisionerTask(t, db, task)
	}

	idMustRegister(t, srv, "new")
	handler.WaitBackground()

	agents, tasks := purgeRows(t, db)
	assert.False(t, agents["silent"], "silent for 31 days and nothing pending")
	assert.False(t, agents["silent-assigned"], "an assigned task does not keep an agent")
	assert.True(t, agents["silent-pending"], "a pending task keeps its agent")
	assert.True(t, agents["recent"], "silent for 29 days")
	assert.True(t, agents["new"])
	assert.NotContains(t, tasks, "old-completed")
	assert.NotContains(t, tasks, "old-failed")
	assert.Contains(t, tasks, "recent-completed")
	assert.Contains(t, tasks, "old-pending", "only finished tasks are deleted by age")
	assert.Equal(t, "failed", tasks["assigned-of-silent"].Status, "the removed agent's assigned task is failed, with the reason")
	assert.NotEmpty(t, tasks["assigned-of-silent"].Config["_error"])

	// A second register within the hour does not purge again.
	seedProvisionerAgent(t, db, "silent-later")
	idSetLastSeen(t, db, "silent-later", old)
	idMustRegister(t, srv, "newer")
	handler.WaitBackground()
	agents, _ = purgeRows(t, db)
	assert.True(t, agents["silent-later"], "the purge runs at most once an hour")
}

func TestProvisionerRegistry_PurgeFailureNeverChangesTheRegisterAnswer(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	router, handler := configRoutesHandlerOn(t, db, idAdminKey, idAgentKey)
	srv := httptest.NewServer(router)
	defer srv.Close()

	require.NoError(t, db.GetDB().Migrator().DropTable(&database.ProvisioningTask{}))

	code, token, body := idRegister(t, srv, "agent-a", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, "a register answers 201 though the purge cannot run: %s", body)
	assert.NotEmpty(t, token)
	handler.WaitBackground()
}

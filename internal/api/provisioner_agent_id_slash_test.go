package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
)

// An agent id can hold a "/": a keyless id is any printable ASCII without a space, and an id that starts with
// "provisioner-" may hold any byte. Each of the five per-agent routes takes the id as one segment of the
// path, so each must reach an agent whose id holds a slash, sent as it is or escaped. The cases walk all five
// routes in the order an operator meets them: the agent polls, the page reads its status, the operator
// approves, rejects and deletes it.
func TestProvisionerAgentRoutes_ReachAnIdWithASlash(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		rawSent bool // the id can also be sent unescaped, as the shipped agent does
	}{
		{"one slash", "lab/pi-1", true},
		{"two slashes", "lab/rack-2/pi-3", true},
		{"a provisioner- id with a space and a slash", "provisioner-lab/pi 1", false},
		{"a slash and a percent sign", "lab/100%-pi", false},
	}
	for _, c := range cases {
		forms := map[string]func(string) string{"escaped": url.PathEscape}
		if c.rawSent {
			forms["raw"] = func(id string) string { return id }
		}
		for formName, form := range forms {
			t.Run(c.name+", "+formName, func(t *testing.T) {
				srv, db := idServer(t)
				token := idMustRegister(t, srv, c.id)
				seg := form(c.id)
				agents := "/api/v1/provisioner/agents/" + seg

				code, body := idCall(t, srv, "GET", agents+"/tasks", "", idAgentHdr(token))
				require.Equal(t, http.StatusOK, code, "poll: %s", body)

				code, body = idCall(t, srv, "GET", "/api/v1/provisioning/agents/"+seg+"/status", "", idAdminHdr())
				require.Equal(t, http.StatusOK, code, "status: %s", body)
				assert.Contains(t, body, `"id":`+jsonString(c.id), "the status route answers for this row")

				// A keyless row is legacy; the operator acts on one that waits for approval.
				res := db.GetDB().Model(&database.ProvisionerAgent{}).Where("agent_id = ?", c.id).Update("enrolment_state", "pending")
				require.NoError(t, res.Error)
				require.EqualValues(t, 1, res.RowsAffected)

				code, body = idCall(t, srv, "POST", agents+"/approve", "{}", idAdminHdr())
				require.Equal(t, http.StatusOK, code, "approve: %s", body)
				assert.NotContains(t, body, `"enrolment_state":"pending"`)

				code, body = idCall(t, srv, "POST", agents+"/reject", "{}", idAdminHdr())
				require.Equal(t, http.StatusOK, code, "reject: %s", body)
				assert.Contains(t, body, `"enrolment_state":"rejected"`)

				code, body = idCall(t, srv, "DELETE", agents, "", idAdminHdr())
				require.Equal(t, http.StatusOK, code, "delete: %s", body)
				_, err := database.NewProvisionerStore(db.GetDB()).GetAgent(registryDeadline(t), c.id)
				assert.ErrorIs(t, err, database.ErrAgentNotFound, "the row is gone")
			})
		}
	}
}

// Pin: ids that hold no slash keep reaching the same routes, and the suffix of one route is not read as
// part of an id: an agent named like a route suffix is a different agent from the one before the suffix.
func TestProvisionerAgentRoutes_PlainIdsAndSuffixLookalikes(t *testing.T) {
	srv, db := idServer(t)
	tokenA := idMustRegister(t, srv, "pi-a")
	tokenB := idMustRegister(t, srv, "pi-a/tasks")

	code, body := idCall(t, srv, "GET", "/api/v1/provisioner/agents/pi-a/tasks", "", idAgentHdr(tokenA))
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"agent_id":"pi-a"`)

	code, body = idCall(t, srv, "GET", "/api/v1/provisioner/agents/pi-a/tasks/tasks", "", idAgentHdr(tokenB))
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"agent_id":"pi-a/tasks"`, "the greedy id stops at the last /tasks")

	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/pi-a/tasks/tasks", "", idAgentHdr(tokenA))
	assert.Equal(t, http.StatusForbidden, code, "pi-a's token does not poll for pi-a/tasks")

	code, body = idCall(t, srv, "DELETE", "/api/v1/provisioner/agents/pi-a", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	_, err := database.NewProvisionerStore(db.GetDB()).GetAgent(registryDeadline(t), "pi-a/tasks")
	require.NoError(t, err, "deleting pi-a leaves pi-a/tasks")
}

// A new id that a per-agent route cannot reach cleanly is refused at register with a 400 that names the rule,
// and no row is created. An id with one inner slash still registers.
func TestProvisionerRegister_RefusesAnIdNoRouteCanReach(t *testing.T) {
	srv, db := idServer(t)
	for _, id := range []string{"a//b", "/a", "a/", "a/./b", "./a", "a/.", ".", "a/../b", "../a", "a/..", "..", "a..b"} {
		code, _, body := idRegister(t, srv, id, idAgentHdr(""))
		assert.Equal(t, http.StatusBadRequest, code, "%q: %s", id, body)
		assert.Contains(t, body, "empty segment", "%q: the refusal names the rule", id)
		_, err := database.NewProvisionerStore(db.GetDB()).GetAgent(registryDeadline(t), id)
		assert.ErrorIs(t, err, database.ErrAgentNotFound, "%q leaves no row", id)
	}
	code, _, body := idRegister(t, srv, "lab/pi-1", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, body)
	code, _, body = idRegister(t, srv, "v1.2/pi.local", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, body)
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}

// A "provisioner-" id may hold any byte, a newline included, with or without a slash. The five routes must
// reach such a row: the id pattern of a route matches every byte, not only the ones a dot matches.
func TestProvisionerAgentRoutes_ReachAnIdWithANewline(t *testing.T) {
	for _, id := range []string{"provisioner-pi\n2", "provisioner-lab/pi\n1"} {
		t.Run(strings.ReplaceAll(id, "\n", `\n`), func(t *testing.T) {
			srv, db := idServer(t)
			now := time.Now()
			pending := database.EnrolmentPending
			row := database.ProvisionerAgent{AgentID: id, TokenHash: database.HashAgentToken("t-" + id), Hostname: "h", Status: "pending",
				LastSeen: now, RegisteredAt: now, EnrolmentState: &pending}
			require.NoError(t, db.GetDB().Create(&row).Error)
			agents := "/api/v1/provisioner/agents/" + url.PathEscape(id)

			code, body := idCall(t, srv, "GET", agents+"/tasks", "", idAdminHdr())
			require.Equal(t, http.StatusOK, code, "poll: %s", body)
			code, body = idCall(t, srv, "GET", "/api/v1/provisioning/agents/"+url.PathEscape(id)+"/status", "", idAdminHdr())
			require.Equal(t, http.StatusOK, code, "status: %s", body)
			code, body = idCall(t, srv, "POST", agents+"/approve", "{}", idAdminHdr())
			require.Equal(t, http.StatusOK, code, "approve: %s", body)
			code, body = idCall(t, srv, "POST", agents+"/reject", "{}", idAdminHdr())
			require.Equal(t, http.StatusOK, code, "reject: %s", body)
			code, body = idCall(t, srv, "DELETE", agents, "", idAdminHdr())
			require.Equal(t, http.StatusOK, code, "delete: %s", body)
		})
	}
}

// A row from before the register rule can have an id with an empty segment. The path clean keeps a trailing
// slash, so DELETE reaches a row whose id ends in "/"; a row with "//" inside is reached by no route.
func TestProvisionerAgentRoutes_DeleteReachesAnExistingIdThatEndsInASlash(t *testing.T) {
	srv, db := idServer(t)
	now := time.Now()
	for _, id := range []string{"old-a/", "old-b//c"} {
		require.NoError(t, db.GetDB().Create(&database.ProvisionerAgent{
			AgentID: id, TokenHash: database.HashAgentToken("t-" + id), Hostname: "h", Status: "online", LastSeen: now, RegisteredAt: now,
		}).Error)
	}
	code, body := idCall(t, srv, "DELETE", "/api/v1/provisioner/agents/old-a/", "", idAdminHdr())
	assert.Equal(t, http.StatusOK, code, "a trailing slash is kept: %s", body)
	code, _ = idCall(t, srv, "DELETE", "/api/v1/provisioner/agents/old-b//c", "", idAdminHdr())
	assert.NotEqual(t, http.StatusOK, code, "the clean rewrites // before routing, so the row is not reached")
	_, err := database.NewProvisionerStore(db.GetDB()).GetAgent(registryDeadline(t), "old-b//c")
	assert.NoError(t, err, "the row with // inside is still there")
}

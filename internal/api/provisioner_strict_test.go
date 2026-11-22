package api

import (
	"crypto/ed25519"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/agentkey"
	"github.com/ginsys/shelly-manager/internal/database"
)

// strictServerOn serves the same database as idServer does, with security.agent_enrolment "strict": the
// manager restarted in the new mode over the rows the open one left.
func strictServerOn(t *testing.T, db *database.Manager) *httptest.Server {
	t.Helper()
	router, handler := configRoutesHandlerOn(t, db, idAdminKey, idAgentKey)
	handler.SetAgentEnrolment("strict")
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func TestStrictEnrolment_KeylessRegisterIs403ForEveryCaller(t *testing.T) {
	_, db := idServer(t)
	srv := strictServerOn(t, db)
	for name, hdr := range map[string]map[string]string{"the agent key": idAgentHdr(""), "the admin key": idAdminHdr()} {
		code, _, body := idRegister(t, srv, "plain-agent", hdr)
		assert.Equal(t, http.StatusForbidden, code, "%s: %s", name, body)
	}
	// A keyed id is refused the same way: the mode, not the id, is the answer.
	priv := kdKey(0xc1)
	code, _, body := idRegister(t, srv, kdID(priv), idAgentHdr(""))
	assert.Equal(t, http.StatusForbidden, code, body)
	assert.EqualValues(t, 0, kdRows(t, db), "a refused register writes no row")
}

func TestStrictEnrolment_OpenStillRegistersKeyless(t *testing.T) {
	srv, db := idServer(t)
	code, token, body := idRegister(t, srv, "plain-agent", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, body)
	assert.NotEmpty(t, token)
	assert.EqualValues(t, 1, kdRows(t, db))
}

func TestStrictEnrolment_AKeyedAgentEnrolsAsInOpen(t *testing.T) {
	_, db := idServer(t)
	srv := strictServerOn(t, db)
	priv := kdKey(0xc2)
	id := kdID(priv)

	code, data, body := kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "pending", data["status"])
	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/approve", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	code, data, body = kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	code, data, body = kdRegister(t, srv, priv, id, "", kdSign(t, priv, id, data["challenge"].(string)))
	require.Equal(t, http.StatusCreated, code, body)
	token, _ := data["agent_token"].(string)
	require.NotEmpty(t, token)

	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/"+id+"/tasks", "", idAgentHdr(token))
	assert.Equal(t, http.StatusOK, code, "an approved agent polls in strict mode")
}

func TestStrictEnrolment_ALegacyTokenAuthorizesNothing(t *testing.T) {
	open, db := idServer(t)
	token := idMustRegister(t, open, "old-agent")
	taskID := idTask(t, open, "old-agent")
	strict := strictServerOn(t, db)

	// Open: the token polls, updates its task and reports. The same token on the strict manager is a 401 on
	// each of the three routes, a NULL-state row included (a row from before the column is legacy too).
	for _, srv := range []*httptest.Server{open, strict} {
		code, _ := idCall(t, srv, "GET", "/api/v1/provisioner/agents/old-agent/tasks", "", idAgentHdr(token))
		if srv == open {
			assert.Equal(t, http.StatusOK, code, "open: the legacy token polls")
		} else {
			assert.Equal(t, http.StatusUnauthorized, code, "strict: poll")
		}
	}
	code, _ := idCall(t, strict, "PUT", "/api/v1/provisioner/tasks/"+taskID+"/status", `{"status":"running"}`, idAgentHdr(token))
	assert.Equal(t, http.StatusUnauthorized, code, "strict: task status")
	code, _ = idCall(t, strict, "POST", "/api/v1/provisioner/discovered-devices", `{"agent_id":"old-agent","task_id":"`+taskID+`","devices":[]}`, idAgentHdr(token))
	assert.Equal(t, http.StatusUnauthorized, code, "strict: report")

	require.NoError(t, db.GetDB().Model(&database.ProvisionerAgent{}).Where("agent_id = ?", "old-agent").UpdateColumn("enrolment_state", nil).Error)
	code, _ = idCall(t, strict, "GET", "/api/v1/provisioner/agents/old-agent/tasks", "", idAgentHdr(token))
	assert.Equal(t, http.StatusUnauthorized, code, "strict: a NULL state counts as legacy")

	// The admin key acts for any row, as before.
	code, _ = idCall(t, strict, "GET", "/api/v1/provisioner/agents/old-agent/tasks", "", idAdminHdr())
	assert.Equal(t, http.StatusOK, code, "strict: the admin key still acts for a legacy row")
}

func TestStrictEnrolment_NoOtherStateHoldsAUsableToken(t *testing.T) {
	_, db := idServer(t)
	strict := strictServerOn(t, db)
	for i, state := range []string{database.EnrolmentPending, database.EnrolmentPreapproved, database.EnrolmentRejected} {
		priv := kdKey(byte(0xc3 + i))
		id := kdID(priv)
		fp := id[len(agentkey.IDPrefix):]
		token := "tok-strict-" + state
		st := state
		row := database.ProvisionerAgent{
			AgentID: id, TokenHash: database.HashAgentToken(token), Status: state, Hostname: "h",
			EnrolmentState: &st, Fingerprint: &fp, LastSeen: time.Now(), RegisteredAt: time.Now(),
		}
		if state != database.EnrolmentPreapproved {
			pk := hex.EncodeToString(priv.Public().(ed25519.PublicKey))
			row.PublicKey = &pk
		}
		require.NoError(t, db.GetDB().Create(&row).Error)
		code, _ := idCall(t, strict, "GET", "/api/v1/provisioner/agents/"+id+"/tasks", "", idAgentHdr(token))
		assert.Equal(t, http.StatusUnauthorized, code, state)
	}
}

// Strict mode needs an admin key to approve anybody, and the start refuses it without one. The key cannot
// vanish at run time either: a rotation to an empty value is refused, and the old key keeps working.
func TestStrictEnrolment_TheAdminKeyCannotBeRotatedToEmpty(t *testing.T) {
	_, db := idServer(t)
	strict := strictServerOn(t, db)
	for _, newKey := range []string{``, `  `} {
		code, body := idCall(t, strict, "POST", "/api/v1/admin/rotate-admin-key", `{"new_key":"`+newKey+`"}`, idAdminHdr())
		assert.Equal(t, http.StatusBadRequest, code, "%q: %s", newKey, body)
	}
	code, _ := idCall(t, strict, "GET", "/api/v1/provisioner/agents", "", idAdminHdr())
	assert.Equal(t, http.StatusOK, code, "the admin key is unchanged")
	code, _ = idCall(t, strict, "GET", "/api/v1/provisioner/agents", "", nil)
	assert.Equal(t, http.StatusUnauthorized, code, "and the admin routes are still guarded")
}

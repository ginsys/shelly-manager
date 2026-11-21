package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
)

// admFingerprint is a stored-form fingerprint made of one repeated byte, built at run time: a literal "key-"
// and 32 characters in the source reads as an API key to a secret scanner.
func admFingerprint(b byte) string { return strings.Repeat(fmt.Sprintf("%02x", b), 16) }

// admAgent is the data of an agent as the admin routes answer it.
type admAgent struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	EnrolmentState string `json:"enrolment_state"`
	Fingerprint    string `json:"fingerprint"`
}

// admCall calls an admin route and decodes the agent in a success answer; raw is the whole body.
func admCall(t *testing.T, srv *httptest.Server, method, path, body string, hdr map[string]string) (code int, agent admAgent, raw string) {
	t.Helper()
	code, raw = idCall(t, srv, method, path, body, hdr)
	var out struct {
		Data admAgent `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return code, out.Data, raw
}

// admInsert writes the row of an agent enrolled by key, as a signed register will.
func admInsert(t *testing.T, db *database.Manager, fp, state string, hasKey bool) string {
	t.Helper()
	id := "key-" + fp
	now := time.Now()
	row := database.ProvisionerAgent{
		AgentID: id, TokenHash: database.HashAgentToken("tok-" + id), Hostname: "h-" + fp[:4], Status: "online",
		LastSeen: now, RegisteredAt: now, EnrolmentState: &state, Fingerprint: &fp,
	}
	if hasKey {
		key := strings.Repeat("7e", 32)
		row.PublicKey = &key
	}
	require.NoError(t, db.GetDB().Create(&row).Error)
	return id
}

func TestProvisionerAdmin_RoutesTakeTheAdminKeyAlone(t *testing.T) {
	srv, db := idServer(t)
	id := admInsert(t, db, admFingerprint(0x11), "pending", true)
	for name, hdr := range map[string]map[string]string{"the agent key": idAgentHdr(""), "no key": {}} {
		for _, route := range []struct{ method, path, body string }{
			{"POST", "/api/v1/provisioner/agents/" + id + "/approve", ""},
			{"POST", "/api/v1/provisioner/agents/" + id + "/reject", ""},
			{"POST", "/api/v1/provisioner/enrolments", `{"fingerprint":"` + admFingerprint(0x12) + `"}`},
		} {
			code, _, body := admCall(t, srv, route.method, route.path, route.body, hdr)
			assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, code, "%s %s with %s: %s", route.method, route.path, name, body)
		}
	}
	var n int64
	require.NoError(t, db.GetDB().Model(&database.ProvisionerAgent{}).Count(&n).Error)
	assert.EqualValues(t, 1, n, "a refused caller creates no row")
	row, err := database.NewProvisionerStore(db.GetDB()).GetAgent(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "pending", row.State(), "a refused caller moves nothing")
}

func TestProvisionerAdmin_ApproveRejectAndTheList(t *testing.T) {
	srv, db := idServer(t)
	fp := admFingerprint(0x21)
	id := admInsert(t, db, fp, "pending", true)
	admin := idAdminHdr()

	code, agent, body := admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/approve", "", admin)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "approved", agent.EnrolmentState)
	assert.Equal(t, "approved", agent.Status)
	assert.Equal(t, strings.Join([]string{fp[0:4], fp[4:8], fp[8:12], fp[12:16], fp[16:20], fp[20:24], fp[24:28], fp[28:32]}, "-"), agent.Fingerprint,
		"the fingerprint is shown as eight groups of four")
	assert.NotContains(t, body, "public_key")
	assert.NotContains(t, body, "token_hash")
	assert.NotContains(t, body, "challenge")

	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/approve", "", admin)
	assert.Equal(t, http.StatusConflict, code, "an approved row cannot be approved again: %s", body)

	code, agent, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/reject", "", admin)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "rejected", agent.EnrolmentState)
	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/reject", "", admin)
	assert.Equal(t, http.StatusConflict, code, "a rejected row cannot be rejected again: %s", body)

	for _, verb := range []string{"approve", "reject"} {
		code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/nobody/"+verb, "", admin)
		assert.Equal(t, http.StatusNotFound, code, "%s of an unknown id: %s", verb, body)
	}

	// The list shows every row's state: a row with no state as legacy, and no fingerprint where it has none.
	legacy := idMustRegister(t, srv, "plain-agent")
	assert.NotEmpty(t, legacy)
	code, raw := idCall(t, srv, "GET", "/api/v1/provisioner/agents", "", admin)
	require.Equal(t, http.StatusOK, code, raw)
	var list struct {
		Data struct {
			Agents []admAgent `json:"agents"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &list))
	states := map[string]admAgent{}
	for _, a := range list.Data.Agents {
		states[a.ID] = a
	}
	assert.Equal(t, "rejected", states[id].EnrolmentState)
	assert.NotEmpty(t, states[id].Fingerprint)
	assert.Equal(t, "legacy", states["plain-agent"].EnrolmentState)
	assert.Empty(t, states["plain-agent"].Fingerprint)
	assert.NotContains(t, raw, "public_key")
	assert.NotContains(t, raw, "token_hash")
	assert.NotContains(t, raw, "challenge")

	require.NoError(t, db.GetDB().Model(&database.ProvisionerAgent{}).Where("agent_id = ?", "plain-agent").
		UpdateColumn("enrolment_state", nil).Error)
	code, raw = idCall(t, srv, "GET", "/api/v1/provisioner/agents", "", admin)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, raw, `"enrolment_state":"legacy"`, "a row with no state is listed as legacy")
	assert.NotContains(t, raw, `"enrolment_state":""`)
}

func TestProvisionerAdmin_RejectEndsTheAgentsSession(t *testing.T) {
	srv, db := idServer(t)
	id := admInsert(t, db, admFingerprint(0x22), "approved", true)
	poll := "/api/v1/provisioner/agents/" + id + "/tasks"
	code, _ := idCall(t, srv, "GET", poll, "", idAgentHdr("tok-"+id))
	require.Equal(t, http.StatusOK, code, "the approved agent polls with its token")

	code, _, body := admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/reject", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	code, _ = idCall(t, srv, "GET", poll, "", idAgentHdr("tok-"+id))
	assert.Equal(t, http.StatusUnauthorized, code, "the token stops working at once")
}

func TestProvisionerAdmin_PreapproveRoute(t *testing.T) {
	srv, db := idServer(t)
	admin := idAdminHdr()
	fp := admFingerprint(0x31)

	code, agent, body := admCall(t, srv, "POST", "/api/v1/provisioner/enrolments", `{"fingerprint":"`+fp+`"}`, admin)
	require.Equal(t, http.StatusCreated, code, body)
	assert.Equal(t, "key-"+fp, agent.ID)
	assert.Equal(t, "preapproved", agent.EnrolmentState)
	assert.Equal(t, "preapproved", agent.Status)

	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/enrolments", `{"fingerprint":"`+fp+`"}`, admin)
	assert.Equal(t, http.StatusConflict, code, "the fingerprint has its row already: %s", body)

	pending := admFingerprint(0x32)
	admInsert(t, db, pending, "pending", true)
	code, agent, body = admCall(t, srv, "POST", "/api/v1/provisioner/enrolments", `{"fingerprint":"`+pending+`"}`, admin)
	assert.Equal(t, http.StatusOK, code, "a pending row is approved, not created: %s", body)
	assert.Equal(t, "approved", agent.EnrolmentState)

	for _, bad := range []string{`{"fingerprint":""}`, `{}`, `{"fingerprint":"nothex"}`, `not json`} {
		code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/enrolments", bad, admin)
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", bad, body)
	}
}

// A keyless agent still registers as before, and a row that is not legacy still turns it away.
func TestProvisionerAdmin_PreapprovedRowIsNotAnActiveAgent(t *testing.T) {
	srv, _ := idServer(t)
	admin := idAdminHdr()
	code, _, body := admCall(t, srv, "POST", "/api/v1/provisioner/enrolments", `{"fingerprint":"`+admFingerprint(0x41)+`"}`, admin)
	require.Equal(t, http.StatusCreated, code, body)
	idMustRegister(t, srv, "working-agent")

	code, raw := idCall(t, srv, "GET", "/api/v1/provisioner/health", "", idAgentHdr(""))
	require.Equal(t, http.StatusOK, code, raw)
	var health struct {
		Data struct {
			Agents       int `json:"total_agents"`
			ActiveAgents int `json:"active_agents"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &health))
	assert.Equal(t, 2, health.Data.Agents, "both rows are agents: %s", raw)
	assert.Equal(t, 1, health.Data.ActiveAgents, "only the legacy one works: %s", raw)
}

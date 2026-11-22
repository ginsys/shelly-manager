package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/agentkey"
	"github.com/ginsys/shelly-manager/internal/database"
)

// The agents page shows each agent's enrolment state and, for an agent enrolled by key, its fingerprint. The
// page reads the UI list and the UI status route, so both carry the two fields and nothing else about the key.
func TestProvisioningUI_AgentsListCarriesEnrolmentState(t *testing.T) {
	srv, db := idServer(t)

	now := time.Now()
	fingerprint := "0123456789abcdef0123456789abcdef"
	publicKey := strings.Repeat("ab", 32)
	challenge := strings.Repeat("cd", 32)
	tokenHash := database.HashAgentToken("a-secret-token")
	approved, pending := database.EnrolmentApproved, database.EnrolmentPending
	waitingFingerprint := strings.Repeat("ef", 16)
	rows := []database.ProvisionerAgent{
		{AgentID: agentkey.IDPrefix + fingerprint, TokenHash: tokenHash, Hostname: "keyed", Status: "online", LastSeen: now, RegisteredAt: now,
			EnrolmentState: &approved, Fingerprint: &fingerprint, PublicKey: &publicKey, Challenge: &challenge},
		{AgentID: "old-one", TokenHash: database.HashAgentToken("legacy-token"), Hostname: "legacy", Status: "online", LastSeen: now, RegisteredAt: now},
		{AgentID: agentkey.IDPrefix + waitingFingerprint, TokenHash: database.HashAgentToken("pending-token"), Hostname: "waiting", Status: "pending",
			LastSeen: now, RegisteredAt: now, EnrolmentState: &pending, Fingerprint: &waitingFingerprint},
	}
	for i := range rows {
		require.NoError(t, db.GetDB().Create(&rows[i]).Error)
	}

	code, body := idCall(t, srv, "GET", "/api/v1/provisioning/agents", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	var list struct {
		Data struct {
			Agents []map[string]any `json:"agents"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &list))
	byID := map[string]map[string]any{}
	for _, a := range list.Data.Agents {
		byID[a["id"].(string)] = a
	}
	require.Len(t, byID, 3)

	keyed := byID[agentkey.IDPrefix+fingerprint]
	assert.Equal(t, "approved", keyed["enrolmentState"])
	assert.Equal(t, agentkey.Readable(fingerprint), keyed["fingerprint"])
	assert.Equal(t, "legacy", byID["old-one"]["enrolmentState"], "a keyless row reads as legacy")
	assert.NotContains(t, byID["old-one"], "fingerprint", "a keyless row has no fingerprint")
	assert.Equal(t, "pending", byID[agentkey.IDPrefix+waitingFingerprint]["enrolmentState"])

	// The page never gets the key, the challenge or the token hash, under any name.
	for _, secret := range []string{publicKey, challenge, tokenHash} {
		assert.NotContains(t, body, secret)
	}
	for _, a := range list.Data.Agents {
		for key := range a {
			lower := strings.ToLower(key)
			assert.NotContains(t, lower, "key", "field %q", key)
			assert.NotContains(t, lower, "challenge", "field %q", key)
			assert.NotContains(t, lower, "token", "field %q", key)
		}
	}

	// A row that Refresh Status replaces keeps its state and fingerprint.
	code, body = idCall(t, srv, "GET", "/api/v1/provisioning/agents/"+agentkey.IDPrefix+fingerprint+"/status", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	var one struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &one))
	assert.Equal(t, "approved", one.Data["enrolmentState"])
	assert.Equal(t, agentkey.Readable(fingerprint), one.Data["fingerprint"])
}

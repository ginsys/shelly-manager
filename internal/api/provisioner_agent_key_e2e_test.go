package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/agentkey"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/provisioning"
)

// The keyed agent against the real router (ginsys/shelly-manager#638, slice 3). The agent and the manager share
// only a wire format, and the client's own tests answer from hand-written shapes. Here the real
// provisioning.APIClient registers against the real handler and store, so the id, the fingerprint, the two
// steps and the signed message are held to what the manager actually checks.

const agentKeyE2ESeed = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func keyedAgentClient(t *testing.T, baseURL string, strict bool) (*provisioning.APIClient, *agentkey.Key) {
	t.Helper()
	key, err := agentkey.FromSeed(agentKeyE2ESeed)
	require.NoError(t, err)
	client := provisioning.NewAPIClient(baseURL, idAgentKey, key.ID(), logging.GetDefault())
	client.SetKey(key, strict)
	return client, key
}

func TestKeyedAgent_EnrolsThroughTheRealRouter(t *testing.T) {
	srv, db := idServer(t)
	client, key := keyedAgentClient(t, srv.URL, false)

	// 1. An unknown key waits: a pending row under the id the key derives, listed with the fingerprint the agent prints.
	var notEnrolled *provisioning.NotEnrolledError
	require.ErrorAs(t, client.RegisterAgent("host-1", []string{"wifi-provisioning"}, nil), &notEnrolled)
	assert.Equal(t, "pending", notEnrolled.Status)
	assert.Equal(t, key.Readable(), notEnrolled.Fingerprint, "the fingerprint the manager shows is the one the agent prints")
	assert.False(t, client.IsRegistered())
	assert.EqualValues(t, 1, kdRows(t, db))
	code, body := idCall(t, srv, "GET", "/api/v1/provisioner/agents", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, key.ID())
	assert.Contains(t, body, key.Readable())
	assert.NotContains(t, body, agentKeyE2ESeed, "the seed never reaches the manager")

	// 2. The operator approves. The agent's next register is answered with a challenge, which it signs.
	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+key.ID()+"/approve", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	require.NoError(t, client.RegisterAgent("host-1", []string{"wifi-provisioning"}, nil))
	assert.True(t, client.IsRegistered())
	_, err := client.PollTasks()
	require.NoError(t, err, "the token from the signed register works on the next call")

	// 3. A restart needs no operator step: a new client with the same seed signs its way in again.
	again, _ := keyedAgentClient(t, srv.URL, true)
	require.NoError(t, again.RegisterAgent("host-1", []string{"wifi-provisioning"}, nil))
	_, err = again.PollTasks()
	require.NoError(t, err)
	assert.EqualValues(t, 1, kdRows(t, db), "the same key is the same row")

	// 4. A rejected key is an answer, not a failure, and gets no token.
	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+key.ID()+"/reject", "", idAdminHdr())
	require.Equal(t, http.StatusOK, code, body)
	restarted, _ := keyedAgentClient(t, srv.URL, true)
	require.ErrorAs(t, restarted.RegisterAgent("host-1", nil, nil), &notEnrolled)
	assert.Equal(t, "rejected", notEnrolled.Status)
	assert.False(t, restarted.IsRegistered())
}

func TestKeyedAgent_PreapprovedFingerprintNeedsNoWait(t *testing.T) {
	srv, _ := idServer(t)
	client, key := keyedAgentClient(t, srv.URL, true)
	code, _, body := admCall(t, srv, "POST", "/api/v1/provisioner/enrolments", `{"fingerprint":"`+key.Readable()+`"}`, idAdminHdr())
	require.Equal(t, http.StatusCreated, code, body)

	require.NoError(t, client.RegisterAgent("host-1", nil, nil), "the readable fingerprint the operator typed names this agent")
	assert.True(t, client.IsRegistered())
}

func TestKeyedAgent_StrictManagerRefusesAKeylessAgentAndKeepsAKeyedOne(t *testing.T) {
	_, db := idServer(t)
	srv := strictServerOn(t, db)

	keyless := provisioning.NewAPIClient(srv.URL, idAgentKey, "provisioner-h-1", logging.GetDefault())
	err := keyless.RegisterAgent("host-1", nil, nil)
	var httpErr *provisioning.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusForbidden, httpErr.StatusCode, "the shipped keyless agent is locked out of strict")

	keyed, _ := keyedAgentClient(t, srv.URL, true)
	var notEnrolled *provisioning.NotEnrolledError
	require.ErrorAs(t, keyed.RegisterAgent("host-1", nil, nil), &notEnrolled)
	assert.Equal(t, "pending", notEnrolled.Status)
}

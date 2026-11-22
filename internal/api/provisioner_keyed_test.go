package api

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/agentkey"
	"github.com/ginsys/shelly-manager/internal/database"
)

// kdKey makes the key of a test agent from one repeated byte, so no key appears in the source.
func kdKey(b byte) ed25519.PrivateKey { return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{b}, 32)) }

func kdID(priv ed25519.PrivateKey) string {
	return agentkey.AgentID(agentkey.Fingerprint(priv.Public().(ed25519.PublicKey)))
}

// kdRegister sends a register by key. sig is empty for the unsigned request; id is the agent's own derived id
// unless the caller names another.
func kdRegister(t *testing.T, srv *httptest.Server, priv ed25519.PrivateKey, id, publicKey, sig string) (int, map[string]any, string) {
	t.Helper()
	if publicKey == "" {
		publicKey = hex.EncodeToString(priv.Public().(ed25519.PublicKey))
	}
	body := map[string]any{"id": id, "hostname": "host-" + id[len(id)-4:], "public_key": publicKey}
	if sig != "" {
		body["signature"] = sig
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	code, resp := idCall(t, srv, "POST", "/api/v1/provisioner/agents/register", string(raw), idAgentHdr(""))
	var out struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal([]byte(resp), &out)
	return code, out.Data, resp
}

func kdSign(t *testing.T, priv ed25519.PrivateKey, id, challengeHex string) string {
	t.Helper()
	raw, err := hex.DecodeString(challengeHex)
	require.NoError(t, err)
	return hex.EncodeToString(ed25519.Sign(priv, agentkey.RegisterMessage(id, raw)))
}

func kdRows(t *testing.T, db *database.Manager) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.GetDB().Model(&database.ProvisionerAgent{}).Count(&n).Error)
	return n
}

func TestKeyedRegister_FormatIsCheckedBeforeAnyRow(t *testing.T) {
	srv, db := idServer(t)
	priv := kdKey(0xa1)
	id := kdID(priv)
	good := hex.EncodeToString(priv.Public().(ed25519.PublicKey))
	goodSig := strings.Repeat("ab", 64)
	for name, c := range map[string]struct{ id, key, sig string }{
		"a short public key":       {id, good[:62], ""},
		"a long public key":        {id, good + "00", ""},
		"an upper-case public key": {id, strings.ToUpper(good), ""},
		"a non-hex public key":     {id, strings.Repeat("zz", 32), ""},
		"a signature of 63 bytes":  {id, good, goodSig[:126]},
		"an upper-case signature":  {id, good, strings.ToUpper(goodSig)},
		"a non-hex signature":      {id, good, strings.Repeat("zz", 64)},
		"an id the key does not derive": {
			kdID(kdKey(0xa2)), good, "",
		},
		"an id that is the fingerprint with another prefix": {"KEYED-" + id[len(agentkey.IDPrefix):], good, ""},
	} {
		code, _, body := kdRegister(t, srv, priv, c.id, c.key, c.sig)
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", name, body)
	}
	assert.EqualValues(t, 0, kdRows(t, db), "a refused format writes no row")

	// A signature with no public key is a malformed keyed register too.
	code, _ := idCall(t, srv, "POST", "/api/v1/provisioner/agents/register",
		`{"id":"`+id+`","hostname":"h","signature":"`+goodSig+`"}`, idAgentHdr(""))
	assert.Equal(t, http.StatusBadRequest, code)
}

// PostgreSQL refuses a NUL in text, and a keyed register inserts before it reads the row's state, so a NUL in
// any reported value is a 400 at the handler for every state, never a 500.
func TestKeyedRegister_NULInAReportedValueIs400(t *testing.T) {
	srv, db := idServer(t)
	priv := kdKey(0xa7)
	id := kdID(priv)
	key := hex.EncodeToString(priv.Public().(ed25519.PublicKey))
	for name, fields := range map[string]string{
		"hostname":       `"hostname":"h\u0000"`,
		"version":        `"hostname":"h","version":"v\u0000"`,
		"a capability":   `"hostname":"h","capabilities":["a\u0000"]`,
		"a metadata key": `"hostname":"h","metadata":{"k\u0000":"v"}`,
		"a metadata val": `"hostname":"h","metadata":{"k":"v\u0000"}`,
	} {
		code, resp := idCall(t, srv, "POST", "/api/v1/provisioner/agents/register",
			`{"id":"`+id+`","public_key":"`+key+`",`+fields+`}`, idAgentHdr(""))
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", name, resp)
	}
	assert.EqualValues(t, 0, kdRows(t, db), "a refused value writes no row")
}

// The handler checks both lengths before it calls the store, so the store's own length error is never what a
// request meets: a wrong-size key or signature is the handler's 400, and no row is written.
func TestKeyedRegister_WrongSizeKeyOrSignatureIs400AtTheHandler(t *testing.T) {
	srv, db := idServer(t)
	priv := kdKey(0xa8)
	id := kdID(priv)
	key := hex.EncodeToString(priv.Public().(ed25519.PublicKey))
	code, _, resp := kdRegister(t, srv, priv, id, key[:len(key)-2], "")
	assert.Equal(t, http.StatusBadRequest, code, resp)
	assert.Contains(t, resp, "public_key must be 64 lowercase hexadecimal characters")
	code, _, resp = kdRegister(t, srv, priv, id, "", strings.Repeat("ab", 63))
	assert.Equal(t, http.StatusBadRequest, code, resp)
	assert.Contains(t, resp, "signature must be 128 lowercase hexadecimal characters")
	assert.EqualValues(t, 0, kdRows(t, db), "a refused size writes no row")
}

func TestKeyedRegister_TheWholeEnrolment(t *testing.T) {
	srv, db := idServer(t)
	admin := idAdminHdr()
	priv := kdKey(0xa3)
	id := kdID(priv)

	// 1. A new key is pending and gets no token, no key and no hash back.
	code, data, body := kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "pending", data["status"])
	assert.Equal(t, id, data["agent_id"])
	fp, _ := data["fingerprint"].(string)
	assert.Equal(t, agentkey.Readable(agentkey.Fingerprint(priv.Public().(ed25519.PublicKey))), fp, "the readable fingerprint")
	assert.NotContains(t, body, "agent_token")
	assert.NotContains(t, body, "public_key")
	assert.NotContains(t, body, "token_hash")

	// The operator sees it in the list with the same fingerprint, state pending.
	code, _, list := admCall(t, srv, "GET", "/api/v1/provisioner/agents", "", admin)
	require.Equal(t, http.StatusOK, code, list)
	assert.Contains(t, list, `"enrolment_state":"pending"`)
	assert.Contains(t, list, fp)

	// 2. Still pending after the next register; no challenge.
	code, data, body = kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "pending", data["status"])
	assert.NotContains(t, body, `"challenge"`)

	// 3. The operator approves; the agent is asked for a signature, and is given no token yet.
	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/approve", "", admin)
	require.Equal(t, http.StatusOK, code, body)
	code, data, body = kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "challenge", data["status"])
	ch, _ := data["challenge"].(string)
	require.Len(t, ch, 64)
	assert.NotContains(t, body, "agent_token")

	// 4. A wrong signature is a 401 that spends nothing; the right one is a 201 with a token that works.
	code, _, body = kdRegister(t, srv, priv, id, "", strings.Repeat("00", 64))
	assert.Equal(t, http.StatusUnauthorized, code, body)
	sig := kdSign(t, priv, id, ch)
	code, data, body = kdRegister(t, srv, priv, id, "", sig)
	require.Equal(t, http.StatusCreated, code, body)
	token, _ := data["agent_token"].(string)
	require.NotEmpty(t, token, body)
	assert.Equal(t, "registered", data["status"])
	assert.NotEmpty(t, data["registered_at"])
	fields := make([]string, 0, len(data))
	for k := range data {
		fields = append(fields, k)
	}
	assert.ElementsMatch(t, []string{"agent_id", "agent_token", "registered_at", "status", "message"}, fields,
		"the 201 has the fields of a keyless register's, no more")
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/"+id+"/tasks", "", idAgentHdr(token))
	assert.Equal(t, http.StatusOK, code, "the issued token polls")

	// 5. The same signature again finds no challenge, and the token is not replaced.
	code, _, body = kdRegister(t, srv, priv, id, "", sig)
	assert.Equal(t, http.StatusUnauthorized, code, body)
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/"+id+"/tasks", "", idAgentHdr(token))
	assert.Equal(t, http.StatusOK, code, "a replayed signature replaced no token")

	// 6. The agent restarts with no token: a new challenge, and a new token with no five-minute wait.
	code, data, body = kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	ch2, _ := data["challenge"].(string)
	code, data, body = kdRegister(t, srv, priv, id, "", kdSign(t, priv, id, ch2))
	require.Equal(t, http.StatusCreated, code, "the agent was seen a moment ago: %s", body)
	assert.NotEqual(t, token, data["agent_token"])
	code, _ = idCall(t, srv, "GET", "/api/v1/provisioner/agents/"+id+"/tasks", "", idAgentHdr(token))
	assert.Equal(t, http.StatusUnauthorized, code, "the new token ended the old one")

	// 7. A rejected key answers rejected and gets nothing.
	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/agents/"+id+"/reject", "", admin)
	require.Equal(t, http.StatusOK, code, body)
	code, data, body = kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "rejected", data["status"])
	assert.NotContains(t, body, "agent_token")
	assert.NotContains(t, body, `"challenge"`)
	assert.EqualValues(t, 1, kdRows(t, db), "one key, one row, through every step")
}

func TestKeyedRegister_PreapprovedKeySignsItsWayIn(t *testing.T) {
	srv, _ := idServer(t)
	priv := kdKey(0xa4)
	id := kdID(priv)
	fp := agentkey.Fingerprint(priv.Public().(ed25519.PublicKey))
	code, _, body := admCall(t, srv, "POST", "/api/v1/provisioner/enrolments", `{"fingerprint":"`+fp+`"}`, idAdminHdr())
	require.Equal(t, http.StatusCreated, code, body)

	code, data, body := kdRegister(t, srv, priv, id, "", "")
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "challenge", data["status"], "a preapproved key is asked for a signature at once")
	code, data, body = kdRegister(t, srv, priv, id, "", kdSign(t, priv, id, data["challenge"].(string)))
	require.Equal(t, http.StatusCreated, code, body)
	assert.NotEmpty(t, data["agent_token"])

	_, agent, raw := admCall(t, srv, "GET", "/api/v1/provisioner/agents", "", idAdminHdr())
	_ = agent
	assert.Contains(t, raw, `"enrolment_state":"approved"`, "the first good signature approved it")
}

func TestKeyedRegister_AKeylessRowAtTheIDIsAConflict(t *testing.T) {
	srv, db := idServer(t)
	priv := kdKey(0xa5)
	id := kdID(priv)
	now := time.Now()
	legacy := database.ProvisionerAgent{
		AgentID: id, TokenHash: database.HashAgentToken("tok-legacy-key"), Hostname: "old", Status: "online",
		LastSeen: now.Add(-time.Hour), RegisteredAt: now,
	}
	require.NoError(t, db.GetDB().Create(&legacy).Error)
	code, _, body := kdRegister(t, srv, priv, id, "", "")
	assert.Equal(t, http.StatusConflict, code, body)
	code, _, body = kdRegister(t, srv, priv, id, "", strings.Repeat("11", 64))
	assert.Equal(t, http.StatusConflict, code, body)
	row, err := database.NewProvisionerStore(db.GetDB()).GetAgent(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, database.HashAgentToken("tok-legacy-key"), row.TokenHash, "a keyed register replaces nothing of a legacy row")
}

func TestKeyedRegister_PendingCapIsA429(t *testing.T) {
	srv, db := idServer(t)
	for i := 0; i < registryPendingCap; i++ {
		priv := kdKey(byte(0xb0 + i))
		code, _, body := kdRegister(t, srv, priv, kdID(priv), "", "")
		require.Equal(t, http.StatusOK, code, "key %d: %s", i, body)
	}
	extra := kdKey(0xd0)
	code, _, body := kdRegister(t, srv, extra, kdID(extra), "", "")
	assert.Equal(t, http.StatusTooManyRequests, code, body)
	assert.EqualValues(t, registryPendingCap, kdRows(t, db), "the refused key left no row")

	first := kdKey(0xb0)
	code, _, body = kdRegister(t, srv, first, kdID(first), "", "")
	assert.Equal(t, http.StatusOK, code, "a key that waits already refreshes at the cap: %s", body)

	code, _, body = admCall(t, srv, "POST", "/api/v1/provisioner/enrolments",
		`{"fingerprint":"`+agentkey.Fingerprint(extra.Public().(ed25519.PublicKey))+`"}`, idAdminHdr())
	assert.Equal(t, http.StatusCreated, code, "the operator's pre-approval bypasses the cap: %s", body)
}

// The keyless path is untouched: no public key, and a keyed- id is still refused there.
func TestKeyedRegister_TheKeylessPathStaysAsItWas(t *testing.T) {
	srv, _ := idServer(t)
	code, token, body := idRegister(t, srv, "plain-agent", idAgentHdr(""))
	assert.Equal(t, http.StatusCreated, code, body)
	assert.NotEmpty(t, token)
	priv := kdKey(0xa6)
	code, _, body = idRegister2(t, srv, kdID(priv))
	assert.Equal(t, http.StatusBadRequest, code, "a keyless register of a keyed- id: %s", body)
}

// idRegister2 is a keyless register that returns the raw answer.
func idRegister2(t *testing.T, srv *httptest.Server, id string) (int, string, string) {
	t.Helper()
	return idRegister(t, srv, id, idAgentHdr(""))
}

package provisioning

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/agentkey"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// The register protocol of the design note, from the agent's side. The fake manager answers in the shapes the
// real handler writes (internal/api/provisioner_handlers.go, registerKeyed): `pending`, `challenge` and
// `rejected` are a 200 with status, agent_id, fingerprint and message (`challenge` adds the 64-hex challenge),
// and a token is only the 201 answer with agent_token and status "registered". The test against the real
// router lives in internal/api.

const testSeed = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

type registerBody struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

// fakeManager records every register body and answers with the given function; poll requests record their token.
type fakeManager struct {
	mu     sync.Mutex
	bodies []registerBody
	raw    []map[string]any
	tokens []string
}

func (f *fakeManager) register(t *testing.T, answer func(n int, b registerBody, w http.ResponseWriter)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/provisioner/agents/register" {
			f.mu.Lock()
			f.tokens = append(f.tokens, r.Header.Get("X-Agent-Token"))
			f.mu.Unlock()
			writeEnvelope(t, w, http.StatusOK, map[string]any{"agent_id": "x", "tasks": []any{}, "count": 0})
			return
		}
		var b registerBody
		var raw map[string]any
		body := json.NewDecoder(r.Body)
		require.NoError(t, body.Decode(&raw))
		enc, _ := json.Marshal(raw)
		require.NoError(t, json.Unmarshal(enc, &b))
		f.mu.Lock()
		f.bodies = append(f.bodies, b)
		f.raw = append(f.raw, raw)
		n := len(f.bodies)
		f.mu.Unlock()
		answer(n, b, w)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func keyedClient(t *testing.T, srvURL string, strict bool) (*APIClient, *agentkey.Key) {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	key, err := agentkey.FromSeed(testSeed)
	require.NoError(t, err)
	client := NewAPIClient(srvURL, "agent-key", key.ID(), logger)
	client.SetKey(key, strict)
	return client, key
}

func answerPending(t *testing.T, key *agentkey.Key, status, message string) func(int, registerBody, http.ResponseWriter) {
	return func(_ int, _ registerBody, w http.ResponseWriter) {
		writeEnvelope(t, w, http.StatusOK, map[string]any{
			"status": status, "agent_id": key.ID(), "fingerprint": key.Readable(), "message": message,
		})
	}
}

func TestKeyedRegister_PendingAndRejectedAreAnswersNotFailures(t *testing.T) {
	for _, status := range []string{"pending", "rejected"} {
		t.Run(status, func(t *testing.T) {
			key, err := agentkey.FromSeed(testSeed)
			require.NoError(t, err)
			f := &fakeManager{}
			srv := f.register(t, answerPending(t, key, status, "the operator decides"))
			client, _ := keyedClient(t, srv.URL, false)

			err = client.RegisterAgent("host", nil, nil)

			var notEnrolled *NotEnrolledError
			require.ErrorAs(t, err, &notEnrolled, "a typed answer, so the poll loop can wait instead of failing")
			assert.Equal(t, status, notEnrolled.Status)
			assert.Equal(t, key.Readable(), notEnrolled.Fingerprint)
			assert.False(t, client.IsRegistered())
			require.Len(t, f.bodies, 1, "one unsigned request, nothing signed")
			assert.Equal(t, key.PublicKeyHex(), f.bodies[0].PublicKey)
			assert.Equal(t, key.ID(), f.bodies[0].ID, "the id is the one the key derives")
			assert.Empty(t, f.bodies[0].Signature)
		})
	}
}

func TestKeyedRegister_ChallengeIsSignedAndTheTokenKept(t *testing.T) {
	key, err := agentkey.FromSeed(testSeed)
	require.NoError(t, err)
	challenge := make([]byte, 32)
	for i := range challenge {
		challenge[i] = byte(0xa0 + i)
	}
	f := &fakeManager{}
	srv := f.register(t, func(n int, b registerBody, w http.ResponseWriter) {
		if b.Signature == "" {
			writeEnvelope(t, w, http.StatusOK, map[string]any{
				"status": "challenge", "agent_id": key.ID(), "fingerprint": key.Readable(),
				"challenge": hex.EncodeToString(challenge), "message": "Sign the challenge",
			})
			return
		}
		writeEnvelope(t, w, http.StatusCreated, map[string]any{
			"agent_id": key.ID(), "agent_token": "tok-keyed", "registered_at": "2026-11-22T04:00:00Z",
			"status": "registered", "message": "Agent registered successfully",
		})
	})
	client, _ := keyedClient(t, srv.URL, true)

	require.NoError(t, client.RegisterAgent("host", nil, nil))

	require.Len(t, f.bodies, 2, "the unsigned request, then the signed one")
	assert.Empty(t, f.bodies[0].Signature)
	second := f.bodies[1]
	assert.Equal(t, key.PublicKeyHex(), second.PublicKey)
	sig, err := hex.DecodeString(second.Signature)
	require.NoError(t, err)
	// The message the note defines, written out by hand: text, 0, id, 0, the 32 decoded challenge bytes.
	msg := append([]byte("shelly-manager provisioner register v1\x00"+key.ID()+"\x00"), challenge...)
	pub, err := hex.DecodeString(key.PublicKeyHex())
	require.NoError(t, err)
	assert.True(t, ed25519.Verify(pub, msg, sig), "the signature covers the decoded challenge bytes, never the hex text")

	assert.True(t, client.IsRegistered())
	_, err = client.PollTasks()
	require.NoError(t, err)
	assert.Equal(t, []string{"tok-keyed"}, f.tokens, "the token is kept in memory and sent on the next call")
}

func TestKeyedRegister_RefusesABadChallengeBeforeSigning(t *testing.T) {
	for name, challenge := range map[string]string{
		"not hex": "zz" + hex64()[2:], "short": hex64()[:62], "long": hex64() + "00", "upper case": "AB" + hex64()[2:], "empty": "",
	} {
		t.Run(name, func(t *testing.T) {
			key, err := agentkey.FromSeed(testSeed)
			require.NoError(t, err)
			f := &fakeManager{}
			srv := f.register(t, func(_ int, _ registerBody, w http.ResponseWriter) {
				writeEnvelope(t, w, http.StatusOK, map[string]any{
					"status": "challenge", "agent_id": key.ID(), "fingerprint": key.Readable(), "challenge": challenge, "message": "m",
				})
			})
			client, _ := keyedClient(t, srv.URL, false)

			require.Error(t, client.RegisterAgent("host", nil, nil))
			assert.False(t, client.IsRegistered())
			assert.Len(t, f.bodies, 1, "nothing was signed or sent for a challenge that is not 64 lowercase hex")
		})
	}
}

func hex64() string { return "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" }

// The operator can reject or delete the row between the two steps: the signed request then draws the same
// typed answer as an unsigned one, and not a "no token" failure.
func TestKeyedRegister_AnAnswerToTheSignedStepIsStillAnEnrolmentAnswer(t *testing.T) {
	for _, status := range []string{"pending", "rejected"} {
		t.Run(status, func(t *testing.T) {
			key, err := agentkey.FromSeed(testSeed)
			require.NoError(t, err)
			f := &fakeManager{}
			srv := f.register(t, func(n int, b registerBody, w http.ResponseWriter) {
				if b.Signature == "" {
					writeEnvelope(t, w, http.StatusOK, map[string]any{
						"status": "challenge", "agent_id": key.ID(), "fingerprint": key.Readable(), "challenge": hex64(), "message": "m",
					})
					return
				}
				answerPending(t, key, status, "the operator changed their mind")(n, b, w)
			})
			client, _ := keyedClient(t, srv.URL, false)

			var notEnrolled *NotEnrolledError
			require.ErrorAs(t, client.RegisterAgent("host", nil, nil), &notEnrolled)
			assert.Equal(t, status, notEnrolled.Status)
			assert.Equal(t, key.Readable(), notEnrolled.Fingerprint)
			assert.False(t, client.IsRegistered())
		})
	}
}

func TestKeyedRegister_AFailedSignatureStartsAgainUnsigned(t *testing.T) {
	key, err := agentkey.FromSeed(testSeed)
	require.NoError(t, err)
	f := &fakeManager{}
	srv := f.register(t, func(n int, b registerBody, w http.ResponseWriter) {
		if b.Signature == "" {
			writeEnvelope(t, w, http.StatusOK, map[string]any{
				"status": "challenge", "agent_id": key.ID(), "fingerprint": key.Readable(), "challenge": hex64(), "message": "m",
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"error":{"message":"The challenge is missing or expired"}}`))
	})
	client, _ := keyedClient(t, srv.URL, false)

	var httpErr *HTTPError
	require.ErrorAs(t, client.RegisterAgent("host", nil, nil), &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.StatusCode)
	assert.False(t, client.IsRegistered())

	require.Error(t, client.EnsureRegistered("host", nil, nil))
	require.Len(t, f.bodies, 4, "the next try is a whole new round: unsigned, then signed")
	assert.Empty(t, f.bodies[2].Signature, "it starts from an unsigned request, which stores a fresh challenge")
}

// A manager without slice 2 reads the body as a keyless register and answers with a token at once.
func TestKeyedRegister_ATokenAtOnceIsAManagerWithoutEnrolment(t *testing.T) {
	answer := func(t *testing.T, key *agentkey.Key) func(int, registerBody, http.ResponseWriter) {
		return func(_ int, _ registerBody, w http.ResponseWriter) {
			writeEnvelope(t, w, http.StatusCreated, map[string]any{
				"agent_id": key.ID(), "agent_token": "tok-old", "registered_at": "2026-11-22T04:00:00Z",
				"status": "registered", "message": "Agent registered successfully",
			})
		}
	}
	t.Run("strict refuses it and keeps no token", func(t *testing.T) {
		key, err := agentkey.FromSeed(testSeed)
		require.NoError(t, err)
		f := &fakeManager{}
		srv := f.register(t, answer(t, key))
		client, _ := keyedClient(t, srv.URL, true)

		require.Error(t, client.RegisterAgent("host", nil, nil))
		assert.False(t, client.IsRegistered())
		assert.Empty(t, client.agentToken)
	})
	t.Run("open carries on under the new id", func(t *testing.T) {
		key, err := agentkey.FromSeed(testSeed)
		require.NoError(t, err)
		f := &fakeManager{}
		srv := f.register(t, answer(t, key))
		client, _ := keyedClient(t, srv.URL, false)

		require.NoError(t, client.RegisterAgent("host", nil, nil))
		assert.True(t, client.IsRegistered())
		assert.Equal(t, "tok-old", client.agentToken)
	})
}

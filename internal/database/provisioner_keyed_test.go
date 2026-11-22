package database

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/agentkey"
)

// keyedSeed makes the key of a test agent from one repeated byte, so no seed or key appears in the source.
func keyedSeed(b byte) ed25519.PrivateKey { return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{b}, 32)) }

func keyedPub(priv ed25519.PrivateKey) ed25519.PublicKey { return priv.Public().(ed25519.PublicKey) }

func keyedID(priv ed25519.PrivateKey) string {
	return agentkey.AgentID(agentkey.Fingerprint(keyedPub(priv)))
}

// keyedSign answers a challenge as an agent does: a signature over agentkey.RegisterMessage for its id and the decoded
// challenge bytes.
func keyedSign(t *testing.T, priv ed25519.PrivateKey, id, challengeHex string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(challengeHex)
	require.NoError(t, err)
	return ed25519.Sign(priv, agentkey.RegisterMessage(id, raw))
}

// keyedReg is one register by key: unsigned when sig is nil. The token it would issue is "issued-"+tag.
func keyedReg(priv ed25519.PrivateKey, sig []byte, tag string, cap int) KeyedRegistration {
	return KeyedRegistration{
		PublicKey: keyedPub(priv), Signature: sig, PendingCap: cap,
		Details: ProvisionerAgent{
			TokenHash: HashAgentToken("issued-" + tag), Hostname: "host-" + tag, IP: "192.0.2.9", Version: "v-" + tag,
			Capabilities: []string{"cap-" + tag}, Metadata: map[string]string{"tag": tag},
		},
	}
}

// checkProvisionerKeyed holds the cases of the keyed register that every provider must pass. Its keys are the
// seeds 0x71 to 0x9f, which no other case uses.
func checkProvisionerKeyed(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()
	row := func(id string) ProvisionerAgent {
		a, err := s.GetAgent(ctx, id)
		require.NoError(t, err, id)
		return a
	}
	// approve makes a key's row approved by the operator's way: unsigned register, then approve.
	approve := func(t *testing.T, priv ed25519.PrivateKey) string {
		t.Helper()
		_, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "first", 1000), now)
		require.NoError(t, err)
		id := keyedID(priv)
		_, err = s.ApproveAgent(ctx, id, now)
		require.NoError(t, err)
		return id
	}
	challenge := func(t *testing.T, priv ed25519.PrivateKey, at time.Time) string {
		t.Helper()
		res, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "ask", 1000), at)
		require.NoError(t, err)
		require.Equal(t, OutcomeChallenge, res.Outcome)
		return res.Challenge
	}

	t.Run("a new key is a pending row, with no token and nothing the agent chose but its details", func(t *testing.T) {
		priv := keyedSeed(0x71)
		res, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "a", 1000), now)
		require.NoError(t, err)
		assert.Equal(t, OutcomePending, res.Outcome)
		id := keyedID(priv)
		got := row(id)
		assert.Equal(t, EnrolmentPending, got.State())
		assert.Equal(t, EnrolmentPending, got.Status, "a pending row does not read online")
		require.NotNil(t, got.Fingerprint)
		assert.Equal(t, agentkey.Fingerprint(keyedPub(priv)), *got.Fingerprint, "the manager derives the fingerprint")
		require.NotNil(t, got.PublicKey)
		assert.Equal(t, hex.EncodeToString(keyedPub(priv)), *got.PublicKey)
		assert.Equal(t, "host-a", got.Hostname)
		assert.NotEqual(t, HashAgentToken("issued-a"), got.TokenHash, "the token it would be given is not stored")
		assert.Nil(t, got.Challenge)
		_, err = s.ClaimTasks(ctx, tokenActor("issued-a"), id, now)
		assert.ErrorIs(t, err, ErrUnauthorized, "no token was issued")

		// A signed register of a new key is the same answer: there is no row to hold a challenge.
		other := keyedSeed(0x72)
		res, err = s.RegisterKeyed(ctx, keyedReg(other, bytes.Repeat([]byte{1}, 64), "b", 1000), now)
		require.NoError(t, err)
		assert.Equal(t, OutcomePending, res.Outcome)
		assert.Equal(t, EnrolmentPending, row(keyedID(other)).State())
	})

	t.Run("a pending row is refreshed by the next register and stays pending", func(t *testing.T) {
		priv := keyedSeed(0x73)
		_, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "c1", 1000), now.Add(-time.Hour))
		require.NoError(t, err)
		res, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "c2", 1000), now)
		require.NoError(t, err)
		assert.Equal(t, OutcomePending, res.Outcome)
		got := row(keyedID(priv))
		assert.Equal(t, "host-c2", got.Hostname)
		assert.Equal(t, EnrolmentPending, got.Status)
		assert.WithinDuration(t, now, got.LastSeen, time.Second, "a refresh keeps the row from the 7-day purge")
		assert.Nil(t, got.Challenge, "a pending row is given no challenge")
	})

	t.Run("the pending cap refuses a new key and still lets a waiting one refresh", func(t *testing.T) {
		var waiting int64
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("enrolment_state = ?", EnrolmentPending).Count(&waiting).Error)
		limit := int(waiting) + 1
		first, second, third := keyedSeed(0x74), keyedSeed(0x75), keyedSeed(0x76)
		_, err := s.RegisterKeyed(ctx, keyedReg(first, nil, "d1", limit), now)
		require.NoError(t, err, "the row that fills the cap is written")
		_, err = s.RegisterKeyed(ctx, keyedReg(second, nil, "d2", limit), now)
		assert.ErrorIs(t, err, ErrPendingCapReached)
		_, err = s.GetAgent(ctx, keyedID(second))
		assert.ErrorIs(t, err, ErrAgentNotFound, "a refused register leaves no row")
		_, err = s.RegisterKeyed(ctx, keyedReg(first, nil, "d1b", limit), now)
		assert.NoError(t, err, "a key that waits already is not new")
		_, _, err = s.PreapproveFingerprint(ctx, agentkey.Fingerprint(keyedPub(third)), now)
		require.NoError(t, err, "the operator's pre-approval bypasses the cap")
		_, err = s.RegisterKeyed(ctx, keyedReg(third, nil, "d3", limit), now)
		assert.NoError(t, err, "a preapproved key is not a new one")
	})

	t.Run("a rejected row answers rejected and nothing is written", func(t *testing.T) {
		priv := keyedSeed(0x77)
		_, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "e1", 1000), now.Add(-time.Hour))
		require.NoError(t, err)
		id := keyedID(priv)
		_, err = s.RejectAgent(ctx, id, now.Add(-time.Minute))
		require.NoError(t, err)
		was := row(id)
		for _, sig := range [][]byte{nil, bytes.Repeat([]byte{2}, 64)} {
			res, err := s.RegisterKeyed(ctx, keyedReg(priv, sig, "e2", 1000), now)
			require.NoError(t, err)
			assert.Equal(t, OutcomeRejected, res.Outcome)
		}
		after := row(id)
		assert.Equal(t, was.Hostname, after.Hostname)
		assert.Equal(t, was.TokenHash, after.TokenHash)
		assert.Nil(t, after.Challenge, "a rejected key is given no challenge")
		assert.True(t, was.UpdatedAt.Equal(after.UpdatedAt), "nothing is written")
	})

	t.Run("a legacy row at the id, and a state this code does not know, are a 409", func(t *testing.T) {
		legacyKey, unknownKey := keyedSeed(0x78), keyedSeed(0x79)
		register(t, s, adminActor, keyedID(legacyKey), "tok-keyed-legacy", now.Add(-time.Hour))
		insertKeyedRow(t, manager, agentkey.Fingerprint(keyedPub(unknownKey)), "something-else", "tok-keyed-unknown", true, now.Add(-time.Hour))
		for name, priv := range map[string]ed25519.PrivateKey{"legacy": legacyKey, "unknown": unknownKey} {
			for _, sig := range [][]byte{nil, bytes.Repeat([]byte{3}, 64)} {
				_, err := s.RegisterKeyed(ctx, keyedReg(priv, sig, "f", 1000), now)
				assert.ErrorIs(t, err, ErrEnrolmentState, name)
			}
			assert.Equal(t, HashAgentToken("tok-keyed-"+name), row(keyedID(priv)).TokenHash, "%s: the token must not move", name)
		}
	})

	t.Run("an approved row answers an unsigned register with a challenge and writes nothing else", func(t *testing.T) {
		priv := keyedSeed(0x7a)
		id := approve(t, priv)
		was := row(id)
		res, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "g", 1000), now)
		require.NoError(t, err)
		assert.Equal(t, OutcomeChallenge, res.Outcome)
		assert.Len(t, res.Challenge, 64)
		got := row(id)
		require.NotNil(t, got.Challenge)
		assert.Equal(t, res.Challenge, *got.Challenge)
		require.NotNil(t, got.ChallengeExpiresAt)
		assert.WithinDuration(t, now.Add(60*time.Second), *got.ChallengeExpiresAt, time.Second)
		assert.Equal(t, was.Hostname, got.Hostname, "the details are not rewritten by anyone who knows the key")
		assert.Equal(t, was.IP, got.IP)
		assert.Equal(t, was.Version, got.Version)
		assert.Equal(t, was.Status, got.Status)
		assert.Equal(t, was.TokenHash, got.TokenHash)
		assert.True(t, was.LastSeen.Equal(got.LastSeen), "last_seen does not move, so a silent row still reaches the purge")
		assert.True(t, was.UpdatedAt.Equal(got.UpdatedAt), "updated_at does not move: the challenge columns are the only write")

		// Every column but the two challenge ones is as it was, for a preapproved row as for an approved one.
		pre := keyedSeed(0x9a)
		preFP := agentkey.Fingerprint(keyedPub(pre))
		_, _, err = s.PreapproveFingerprint(ctx, preFP, now.Add(-time.Hour))
		require.NoError(t, err)
		preWas := row(agentkey.AgentID(preFP))
		_, err = s.RegisterKeyed(ctx, keyedReg(pre, nil, "g3", 1000), now)
		require.NoError(t, err)
		preGot := row(agentkey.AgentID(preFP))
		require.NotNil(t, preGot.Challenge)
		preGot.Challenge, preGot.ChallengeExpiresAt = nil, nil
		assert.Equal(t, preWas, preGot, "an unsigned register of a preapproved row writes only the challenge")

		again, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "g2", 1000), now)
		require.NoError(t, err)
		assert.NotEqual(t, res.Challenge, again.Challenge, "a new request replaces the outstanding challenge")
	})

	t.Run("a good signature issues the token, ends the challenge and needs no silence", func(t *testing.T) {
		priv := keyedSeed(0x7b)
		id := approve(t, priv)
		// The agent was seen a second ago and a keyless register of this id would be ErrAgentInUse.
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", id).UpdateColumn("last_seen", now.Add(-time.Second)).Error)
		ch := challenge(t, priv, now)
		res, err := s.RegisterKeyed(ctx, keyedReg(priv, keyedSign(t, priv, id, ch), "h", 1000), now.Add(time.Second))
		require.NoError(t, err)
		assert.Equal(t, OutcomeRegistered, res.Outcome)
		got := row(id)
		assert.Equal(t, HashAgentToken("issued-h"), got.TokenHash)
		assert.Equal(t, "online", got.Status)
		assert.Equal(t, EnrolmentApproved, got.State())
		assert.Nil(t, got.Challenge)
		assert.Nil(t, got.ChallengeExpiresAt)
		assert.Equal(t, "host-h", got.Hostname)
		assert.Equal(t, "v-h", got.Version)
		assert.Equal(t, []string{"cap-h"}, got.Capabilities)
		assert.WithinDuration(t, now.Add(time.Second), got.LastSeen, time.Second)
		_, err = s.ClaimTasks(ctx, tokenActor("issued-h"), id, now)
		assert.NoError(t, err, "the issued token works")

		// The challenge is single use: the same signature again finds none.
		_, err = s.RegisterKeyed(ctx, keyedReg(priv, keyedSign(t, priv, id, ch), "h2", 1000), now.Add(2*time.Second))
		assert.ErrorIs(t, err, ErrChallengeFailed)
		assert.Equal(t, HashAgentToken("issued-h"), row(id).TokenHash, "a replayed signature replaces no token")
	})

	t.Run("a failed signed register writes nothing and does not spend the challenge", func(t *testing.T) {
		priv, stranger := keyedSeed(0x7c), keyedSeed(0x7d)
		id := approve(t, priv)
		ch := challenge(t, priv, now)
		was := row(id)
		raw, err := hex.DecodeString(ch)
		require.NoError(t, err)
		for name, sig := range map[string][]byte{
			"another key signed it":    keyedSign(t, stranger, id, ch),
			"for another agent id":     keyedSign(t, priv, "someone-else", ch),
			"the hex text, not bytes":  ed25519.Sign(priv, agentkey.RegisterMessage(id, []byte(ch))),
			"a bare challenge":         ed25519.Sign(priv, raw),
			"a different challenge":    keyedSign(t, priv, id, hex.EncodeToString(bytes.Repeat([]byte{9}, 32))),
			"a flipped signature byte": append(append([]byte{}, keyedSign(t, priv, id, ch)[:63]...), keyedSign(t, priv, id, ch)[63]^1),
		} {
			_, regErr := s.RegisterKeyed(ctx, keyedReg(priv, sig, "i", 1000), now.Add(time.Second))
			assert.ErrorIs(t, regErr, ErrChallengeFailed, name)
		}
		after := row(id)
		assert.Equal(t, was.TokenHash, after.TokenHash, "no token moved")
		require.NotNil(t, after.Challenge)
		assert.Equal(t, ch, *after.Challenge, "a bad signature does not consume the challenge")
		assert.Equal(t, was.Hostname, after.Hostname)

		// The right signature still works after the wrong ones, up to the last second of the 60.
		_, err = s.RegisterKeyed(ctx, keyedReg(priv, keyedSign(t, priv, id, ch), "i2", 1000), now.Add(59*time.Second))
		assert.NoError(t, err)
	})

	t.Run("a challenge that expired, and a signed register with no challenge, are refused", func(t *testing.T) {
		priv := keyedSeed(0x7e)
		id := approve(t, priv)
		_, err := s.RegisterKeyed(ctx, keyedReg(priv, bytes.Repeat([]byte{4}, 64), "j", 1000), now)
		assert.ErrorIs(t, err, ErrChallengeFailed, "no challenge was asked for")
		ch := challenge(t, priv, now)
		sig := keyedSign(t, priv, id, ch)
		for _, at := range []time.Duration{60 * time.Second, 61 * time.Second, time.Hour} {
			_, err = s.RegisterKeyed(ctx, keyedReg(priv, sig, "j2", 1000), now.Add(at))
			assert.ErrorIs(t, err, ErrChallengeFailed, "after %v", at)
		}
		assert.NotNil(t, row(id).Challenge, "a refused register does not clear an expired challenge either: the next unsigned one replaces it")
	})

	t.Run("a preapproved row becomes approved by its first good signature and stores the key then", func(t *testing.T) {
		priv := keyedSeed(0x7f)
		fp := agentkey.Fingerprint(keyedPub(priv))
		id := agentkey.AgentID(fp)
		pre, _, err := s.PreapproveFingerprint(ctx, fp, now)
		require.NoError(t, err)
		require.Nil(t, pre.PublicKey)
		_, err = s.RegisterKeyed(ctx, keyedReg(priv, bytes.Repeat([]byte{5}, 64), "k0", 1000), now)
		assert.ErrorIs(t, err, ErrChallengeFailed, "a signature with no challenge")

		ch := challenge(t, priv, now)
		was := row(id)
		assert.Equal(t, EnrolmentPreapproved, was.State(), "asking for a challenge does not approve")
		assert.Equal(t, EnrolmentPreapproved, was.Status)
		res, err := s.RegisterKeyed(ctx, keyedReg(priv, keyedSign(t, priv, id, ch), "k", 1000), now.Add(time.Second))
		require.NoError(t, err)
		assert.Equal(t, OutcomeRegistered, res.Outcome)
		got := row(id)
		assert.Equal(t, EnrolmentApproved, got.State())
		require.NotNil(t, got.PublicKey)
		assert.Equal(t, hex.EncodeToString(keyedPub(priv)), *got.PublicKey)
		require.NotNil(t, got.ApprovedAt)
		assert.Equal(t, HashAgentToken("issued-k"), got.TokenHash)
	})

	t.Run("an approved row whose stored key is not the presented one refuses the signature", func(t *testing.T) {
		priv := keyedSeed(0x80)
		id := approve(t, priv)
		other := hex.EncodeToString(keyedPub(keyedSeed(0x81)))
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", id).UpdateColumn("public_key", other).Error)
		ch := challenge(t, priv, now)
		_, err := s.RegisterKeyed(ctx, keyedReg(priv, keyedSign(t, priv, id, ch), "l", 1000), now)
		assert.ErrorIs(t, err, ErrChallengeFailed)
	})

	t.Run("a preapproved row whose fingerprint is not the key's refuses a valid signature", func(t *testing.T) {
		priv, other := keyedSeed(0x9b), keyedSeed(0x9c)
		id := keyedID(priv)
		// The row has the id derived from priv, but holds another key's fingerprint (or none): no register
		// makes that, so a signature must not turn it into an approved row.
		for name, fp := range map[string]any{"another fingerprint": agentkey.Fingerprint(keyedPub(other)), "no fingerprint": nil} {
			require.NoError(t, manager.GetDB().Where("agent_id = ?", id).Delete(&ProvisionerAgent{}).Error)
			state := EnrolmentPreapproved
			ch := hex.EncodeToString(bytes.Repeat([]byte{0x5c}, 32))
			exp := now.Add(30 * time.Second)
			bad := ProvisionerAgent{
				AgentID: id, TokenHash: HashAgentToken("tok-fpguard-" + name), Status: state, LastSeen: now, RegisteredAt: now,
				EnrolmentState: &state, Challenge: &ch, ChallengeExpiresAt: &exp,
			}
			if s, ok := fp.(string); ok {
				bad.Fingerprint = &s
			}
			require.NoError(t, manager.GetDB().Create(&bad).Error)
			was := row(id)
			_, err := s.RegisterKeyed(ctx, keyedReg(priv, keyedSign(t, priv, id, ch), "n", 1000), now)
			assert.ErrorIs(t, err, ErrChallengeFailed, name)
			assert.Equal(t, was, row(id), "%s: the row is unchanged", name)
		}
	})

	t.Run("a signed register of a pending row is answered pending and issues nothing", func(t *testing.T) {
		priv := keyedSeed(0x9d)
		_, err := s.RegisterKeyed(ctx, keyedReg(priv, nil, "o1", 1000), now)
		require.NoError(t, err)
		res, err := s.RegisterKeyed(ctx, keyedReg(priv, bytes.Repeat([]byte{7}, 64), "o2", 1000), now)
		require.NoError(t, err)
		assert.Equal(t, OutcomePending, res.Outcome)
		assert.NotEqual(t, HashAgentToken("issued-o2"), row(keyedID(priv)).TokenHash)
	})

	t.Run("a key or signature of the wrong length is refused before any row is read", func(t *testing.T) {
		priv := keyedSeed(0x82)
		short := keyedReg(priv, nil, "m", 1000)
		short.PublicKey = short.PublicKey[:31]
		_, err := s.RegisterKeyed(ctx, short, now)
		assert.ErrorIs(t, err, ErrKeyFormat)
		_, err = s.RegisterKeyed(ctx, keyedReg(priv, bytes.Repeat([]byte{6}, 63), "m", 1000), now)
		assert.ErrorIs(t, err, ErrKeyFormat)
		_, err = s.GetAgent(ctx, keyedID(priv))
		assert.ErrorIs(t, err, ErrAgentNotFound, "a refused format writes no row")
	})

	t.Run("two replicas answering one challenge: exactly one wins", func(t *testing.T) {
		priv := keyedSeed(0x83)
		id := approve(t, priv)
		ch := challenge(t, priv, now)
		sig := keyedSign(t, priv, id, ch)
		errs := make([]error, 4)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = s.RegisterKeyed(ctx, keyedReg(priv, sig, "race"+string(rune('a'+i)), 1000), now.Add(time.Second))
			}()
		}
		wg.Wait()
		wins := 0
		for _, err := range errs {
			if err == nil {
				wins++
			} else {
				assert.ErrorIs(t, err, ErrChallengeFailed)
			}
		}
		assert.Equal(t, 1, wins, "a challenge is spent once")
	})
}

func TestProvisionerKeyed_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkProvisionerKeyed(t, manager)
}

func TestKeyFingerprint(t *testing.T) {
	pub := keyedPub(keyedSeed(0x91))
	fp := agentkey.Fingerprint(pub)
	assert.Len(t, fp, 32)
	assert.Equal(t, fp, agentkey.Fingerprint(pub), "the same key has one fingerprint")
	assert.NotEqual(t, fp, agentkey.Fingerprint(keyedPub(keyedSeed(0x92))))
	assert.Equal(t, agentkey.IDPrefix+fp, agentkey.AgentID(fp))
	got, ok := NormalizeFingerprint(agentkey.Readable(fp))
	assert.True(t, ok)
	assert.Equal(t, fp, got, "the readable form round-trips")
}

func TestRegisterMessage(t *testing.T) {
	ch := bytes.Repeat([]byte{0xab}, 32)
	msg := agentkey.RegisterMessage("agent-x", ch)
	want := append([]byte("shelly-manager provisioner register v1\x00agent-x\x00"), ch...)
	assert.Equal(t, want, msg)
	assert.NotEqual(t, agentkey.RegisterMessage("agent-y", ch), msg, "the id is part of what is signed")
}

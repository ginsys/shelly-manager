package database

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFingerprint is a stored-form fingerprint (32 lowercase hex characters) made of one repeated byte. It is
// built at run time: the id of a keyed row is the "key-" prefix and 32 characters, and a literal of that
// shape in the source is what a secret scanner reads as an API key.
func testFingerprint(b byte) string { return strings.Repeat(fmt.Sprintf("%02x", b), 16) }

// insertKeyedRow writes the row of an agent enrolled by key as a signed register will: the id derived from
// the fingerprint, the state, a token and, unless the row never had a key, a public key.
func insertKeyedRow(t *testing.T, manager *Manager, fp, state, token string, hasKey bool, at time.Time) string {
	t.Helper()
	id := keyedAgentIDPrefix + fp
	row := agentRow(id, token, at)
	row.EnrolmentState, row.Fingerprint = strPtr(state), strPtr(fp)
	if hasKey {
		row.PublicKey = strPtr(testFingerprint(0x7e) + testFingerprint(0x7e))
	}
	require.NoError(t, manager.GetDB().Create(&row).Error, "insert %q", id)
	return id
}

// checkProvisionerAdmin holds the cases of the admin transitions (approve, reject, pre-approve) that every
// provider must pass. Its fingerprints are the repeated bytes 0x10 to 0x4f, which no other case uses.
func checkProvisionerAdmin(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()
	day := 24 * time.Hour
	row := func(id string) ProvisionerAgent {
		a, err := s.GetAgent(ctx, id)
		require.NoError(t, err, id)
		return a
	}

	t.Run("approve takes pending or rejected to approved and writes the state as the status", func(t *testing.T) {
		for i, from := range []string{EnrolmentPending, EnrolmentRejected} {
			id := insertKeyedRow(t, manager, testFingerprint(byte(0x10+i)), from, "tok-adm-approve-"+from, true, now.Add(-time.Hour))
			was := row(id)
			got, err := s.ApproveAgent(ctx, id, now)
			require.NoError(t, err, from)
			assert.Equal(t, EnrolmentApproved, got.State(), from)
			assert.Equal(t, EnrolmentApproved, got.Status, "%s: the status is the state", from)
			require.NotNil(t, got.ApprovedAt, from)
			assert.WithinDuration(t, now, *got.ApprovedAt, time.Second, from)
			assert.Equal(t, was.TokenHash, got.TokenHash, "%s: approving issues no token", from)
			assert.Equal(t, row(id).State(), EnrolmentApproved, "%s: the row holds it", from)
		}
	})

	t.Run("approve takes a rejected row that never had a key back to preapproved", func(t *testing.T) {
		id := insertKeyedRow(t, manager, testFingerprint(0x12), EnrolmentRejected, "tok-adm-keyless-rejected", false, now.Add(-time.Hour))
		got, err := s.ApproveAgent(ctx, id, now)
		require.NoError(t, err)
		assert.Equal(t, EnrolmentPreapproved, got.State())
		assert.Equal(t, EnrolmentPreapproved, got.Status)
		assert.Nil(t, got.ApprovedAt, "nobody has signed yet")
	})

	t.Run("approve refuses every other state and an unknown id, and writes nothing", func(t *testing.T) {
		legacy := "adm-legacy"
		register(t, s, adminActor, legacy, "tok-"+legacy, now.Add(-time.Hour))
		approved := insertKeyedRow(t, manager, testFingerprint(0x13), EnrolmentApproved, "tok-adm-approved", true, now.Add(-time.Hour))
		preapproved := insertKeyedRow(t, manager, testFingerprint(0x14), EnrolmentPreapproved, "tok-adm-preapproved", false, now.Add(-time.Hour))
		for name, id := range map[string]string{"legacy": legacy, "approved": approved, "preapproved": preapproved} {
			was := row(id)
			_, err := s.ApproveAgent(ctx, id, now)
			assert.ErrorIs(t, err, ErrEnrolmentState, name)
			after := row(id)
			assert.Equal(t, was.State(), after.State(), name)
			assert.Equal(t, was.Status, after.Status, name)
			assert.Equal(t, was.TokenHash, after.TokenHash, name)
			assert.True(t, was.UpdatedAt.Equal(after.UpdatedAt), "%s: nothing is written", name)
		}
		_, err := s.ApproveAgent(ctx, "adm-nobody", now)
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})

	t.Run("reject ends the session, clears the challenge and fails the stranded tasks", func(t *testing.T) {
		for i, from := range []string{EnrolmentPending, EnrolmentPreapproved, EnrolmentApproved} {
			id := insertKeyedRow(t, manager, testFingerprint(byte(0x20+i)), from, "tok-adm-reject-"+from, from != EnrolmentPreapproved, now.Add(-time.Minute))
			require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", id).
				Updates(map[string]any{"challenge": testFingerprint(0x55) + testFingerprint(0x55), "challenge_expires_at": now.Add(time.Minute)}).Error)
			for _, task := range []*ProvisioningTask{
				newTask("adm-assigned-"+from, "assigned", id, 0, now), newTask("adm-running-"+from, "in_progress", id, 0, now),
				newTask("adm-waiting-"+from, "pending", id, 0, now),
			} {
				require.NoError(t, s.CreateTask(ctx, task))
			}
			was := row(id)

			got, err := s.RejectAgent(ctx, id, now)
			require.NoError(t, err, from)
			assert.Equal(t, EnrolmentRejected, got.State(), from)
			assert.Equal(t, EnrolmentRejected, got.Status, "%s: the status is the state", from)
			assert.NotEqual(t, was.TokenHash, got.TokenHash, "%s: the token is replaced", from)
			assert.Nil(t, got.Challenge, from)
			assert.Nil(t, got.ChallengeExpiresAt, from)
			assert.Equal(t, was.Fingerprint, got.Fingerprint, "%s: the fingerprint stays, so the key does not return as pending", from)
			assert.Equal(t, was.PublicKey, got.PublicKey, from)
			_, err = s.ClaimTasks(ctx, tokenActor("tok-adm-reject-"+from), id, now)
			assert.ErrorIs(t, err, ErrUnauthorized, "%s: the old token stops working", from)
			for _, tid := range []string{"adm-assigned-", "adm-running-"} {
				stranded, taskErr := s.GetTask(ctx, tid+from)
				require.NoError(t, taskErr)
				assert.Equal(t, "failed", stranded.Status, tid+from)
				assert.Equal(t, "the agent was rejected", stranded.Config["_error"], tid+from)
			}
			task, err := s.GetTask(ctx, "adm-waiting-"+from)
			require.NoError(t, err)
			assert.Equal(t, "pending", task.Status, "%s: a pending task stays", from)
		}
	})

	t.Run("reject refuses a rejected row and a legacy row", func(t *testing.T) {
		rejected := insertKeyedRow(t, manager, testFingerprint(0x23), EnrolmentRejected, "tok-adm-rejected", true, now.Add(-time.Hour))
		legacy := "adm-legacy-reject"
		register(t, s, adminActor, legacy, "tok-"+legacy, now.Add(-time.Hour))
		require.NoError(t, s.CreateTask(ctx, newTask("adm-legacy-task", "assigned", legacy, 0, now)))
		for name, id := range map[string]string{"rejected": rejected, "legacy": legacy} {
			was := row(id)
			_, err := s.RejectAgent(ctx, id, now)
			assert.ErrorIs(t, err, ErrEnrolmentState, name)
			assert.Equal(t, was.TokenHash, row(id).TokenHash, "%s: the token must not move", name)
		}
		task, err := s.GetTask(ctx, "adm-legacy-task")
		require.NoError(t, err)
		assert.Equal(t, "assigned", task.Status, "a refused reject fails no task")
		_, err = s.RejectAgent(ctx, "adm-nobody", now)
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})

	t.Run("pre-approve creates a preapproved row, once, whatever form names the fingerprint", func(t *testing.T) {
		fp := testFingerprint(0x30)
		got, created, err := s.PreapproveFingerprint(ctx, fp, now)
		require.NoError(t, err)
		assert.True(t, created)
		assert.Equal(t, keyedAgentIDPrefix+fp, got.AgentID)
		assert.Equal(t, EnrolmentPreapproved, got.State())
		assert.Equal(t, EnrolmentPreapproved, got.Status)
		require.NotNil(t, got.Fingerprint)
		assert.Equal(t, fp, *got.Fingerprint)
		assert.Nil(t, got.PublicKey, "a preapproved row has no key yet")
		assert.Nil(t, got.ApprovedAt)
		assert.NotEmpty(t, got.TokenHash, "the token column is not null")

		for name, form := range map[string]string{"stored": fp, "upper case": strings.ToUpper(fp), "readable": FormatFingerprint(fp)} {
			_, created, err = s.PreapproveFingerprint(ctx, form, now)
			assert.ErrorIs(t, err, ErrEnrolmentState, name)
			assert.False(t, created, name)
		}

		other, _, err := s.PreapproveFingerprint(ctx, FormatFingerprint(strings.ToUpper(testFingerprint(0x31))), now)
		require.NoError(t, err, "a second preapproved row inserts: each holds its own never-issued token")
		assert.NotEqual(t, got.TokenHash, other.TokenHash)
		assert.Equal(t, keyedAgentIDPrefix+testFingerprint(0x31), other.AgentID, "the readable upper-case form stores as lower-case hex")
	})

	t.Run("pre-approve approves the pending row that has the fingerprint", func(t *testing.T) {
		fp := testFingerprint(0x32)
		id := insertKeyedRow(t, manager, fp, EnrolmentPending, "tok-adm-pre-pending", true, now.Add(-time.Hour))
		got, created, err := s.PreapproveFingerprint(ctx, fp, now)
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, id, got.AgentID)
		assert.Equal(t, EnrolmentApproved, got.State())
		assert.Equal(t, EnrolmentApproved, got.Status)
		require.NotNil(t, got.ApprovedAt)
	})

	t.Run("pre-approve refuses a fingerprint whose row is in any other state, and a bad fingerprint", func(t *testing.T) {
		approved := insertKeyedRow(t, manager, testFingerprint(0x33), EnrolmentApproved, "tok-adm-pre-approved", true, now.Add(-time.Hour))
		rejected := insertKeyedRow(t, manager, testFingerprint(0x34), EnrolmentRejected, "tok-adm-pre-rejected", true, now.Add(-time.Hour))
		for name, id := range map[string]string{"approved": approved, "rejected": rejected} {
			was := row(id)
			_, _, err := s.PreapproveFingerprint(ctx, strings.TrimPrefix(id, keyedAgentIDPrefix), now)
			assert.ErrorIs(t, err, ErrEnrolmentState, name)
			assert.Equal(t, was.State(), row(id).State(), name)
		}
		// A legacy row can sit on the id (an agent registered it before the prefix was refused): it has no
		// fingerprint, so it is not the pending row of this key.
		fp := testFingerprint(0x35)
		register(t, s, adminActor, keyedAgentIDPrefix+fp, "tok-adm-pre-legacy", now.Add(-time.Hour))
		_, _, err := s.PreapproveFingerprint(ctx, fp, now)
		assert.ErrorIs(t, err, ErrEnrolmentState, "legacy")

		var before int64
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Count(&before).Error)
		for _, bad := range []string{"", "abc", strings.Repeat("g", 32), testFingerprint(0x36) + "00", "ab-cd" + testFingerprint(0x36)[5:], strings.Repeat("ab", 16)[:30] + "-ab"} {
			_, _, err := s.PreapproveFingerprint(ctx, bad, now)
			assert.ErrorIs(t, err, ErrFingerprintInvalid, "%q", bad)
		}
		var after int64
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Count(&after).Error)
		assert.Equal(t, before, after, "a bad fingerprint writes no row")
	})

	t.Run("only a legacy or approved row that is fresh counts as an active agent", func(t *testing.T) {
		fresh := now.Add(-time.Minute)
		base, err := s.Counts(ctx, now, testOfflineAfter)
		require.NoError(t, err)
		legacy := "adm-active-legacy"
		register(t, s, adminActor, legacy, "tok-"+legacy, fresh)
		stateless := "adm-active-null"
		register(t, s, adminActor, stateless, "tok-"+stateless, fresh)
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", stateless).UpdateColumn("enrolment_state", nil).Error)
		insertKeyedRow(t, manager, testFingerprint(0x40), EnrolmentApproved, "tok-adm-active-approved", true, fresh)
		for i, state := range []string{EnrolmentPending, EnrolmentPreapproved, EnrolmentRejected} {
			insertKeyedRow(t, manager, testFingerprint(byte(0x41+i)), state, "tok-adm-active-"+state, state != EnrolmentPreapproved, fresh)
		}
		insertKeyedRow(t, manager, testFingerprint(0x44), EnrolmentApproved, "tok-adm-active-stale", true, now.Add(-time.Hour))

		got, err := s.Counts(ctx, now, testOfflineAfter)
		require.NoError(t, err)
		assert.EqualValues(t, 3, got.ActiveAgents-base.ActiveAgents, "legacy, no state and approved count; pending, preapproved, rejected and a silent row do not")
		assert.EqualValues(t, 7, got.Agents-base.Agents, "every row is an agent")
	})

	// The carried check: the purge ends an approved row's session once the row is old enough and its status is
	// not offline. A transition writes its state as the status, so none may write offline on a row that
	// still holds a usable token, or the purge would skip that row for good and its token would never end.
	t.Run("an approved row fresh from each transition still loses its session after the period", func(t *testing.T) {
		type arrival struct {
			name string
			make func() string
		}
		fp := byte(0x50)
		next := func() string { fp++; return testFingerprint(fp) }
		for _, a := range []arrival{
			{"approve of a pending row", func() string {
				id := insertKeyedRow(t, manager, next(), EnrolmentPending, "tok-adm-carry-"+fmt.Sprint(fp), true, now)
				_, err := s.ApproveAgent(ctx, id, now)
				require.NoError(t, err)
				return id
			}},
			{"approve of a rejected row with a key", func() string {
				id := insertKeyedRow(t, manager, next(), EnrolmentRejected, "tok-adm-carry-"+fmt.Sprint(fp), true, now)
				_, err := s.ApproveAgent(ctx, id, now)
				require.NoError(t, err)
				return id
			}},
			{"pre-approve of a pending row", func() string {
				f := next()
				id := insertKeyedRow(t, manager, f, EnrolmentPending, "tok-adm-carry-"+fmt.Sprint(fp), true, now)
				_, _, err := s.PreapproveFingerprint(ctx, f, now)
				require.NoError(t, err)
				return id
			}},
		} {
			id := a.make()
			fresh := row(id)
			require.Equal(t, EnrolmentApproved, fresh.State(), a.name)
			assert.NotEqual(t, "offline", fresh.Status, "%s: no transition writes offline", a.name)

			// The agent stays silent for the period.
			require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", id).
				UpdateColumn("last_seen", now.Add(-31*day)).Error)
			res, err := s.Purge(ctx, now, 30*day, 7*day, 30*day)
			require.NoError(t, err, a.name)
			assert.EqualValues(t, 1, res.Expired, "%s: the session ends once", a.name)
			ended := row(id)
			assert.Equal(t, "offline", ended.Status, a.name)
			assert.NotEqual(t, fresh.TokenHash, ended.TokenHash, "%s: the token is replaced", a.name)
			assert.Equal(t, EnrolmentApproved, ended.State(), "%s: the approval stays", a.name)
			assert.Equal(t, fresh.PublicKey, ended.PublicKey, a.name)
			assert.Equal(t, fresh.Fingerprint, ended.Fingerprint, a.name)
		}
		// The other transitions leave a row the purge never touches, and say so by their state.
		for i, from := range []string{EnrolmentPending, EnrolmentApproved} {
			id := insertKeyedRow(t, manager, testFingerprint(byte(0x60+i)), from, "tok-adm-carry-reject-"+from, true, now)
			rejected, err := s.RejectAgent(ctx, id, now)
			require.NoError(t, err)
			assert.NotEqual(t, "offline", rejected.Status, "reject of %s", from)
		}
		pre, _, err := s.PreapproveFingerprint(ctx, testFingerprint(0x62), now)
		require.NoError(t, err)
		assert.NotEqual(t, "offline", pre.Status, "pre-approve")
	})
}

func TestProvisionerAdmin_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkProvisionerAdmin(t, manager)
}

func TestNormalizeFingerprint(t *testing.T) {
	fp := testFingerprint(0xab)
	for name, in := range map[string]string{"stored": fp, "readable": FormatFingerprint(fp), "upper case": strings.ToUpper(fp)} {
		got, ok := NormalizeFingerprint(in)
		assert.True(t, ok, name)
		assert.Equal(t, fp, got, name)
	}
	assert.Equal(t, "abab-abab-abab-abab-abab-abab-abab-abab", FormatFingerprint(fp))
	for _, bad := range []string{"", fp[:31], fp + "a", strings.Replace(fp, "a", "g", 1), "abab-abab-abab-abab-abab-abab-abab-abab-", "aba-babab-abab-abab-abab-abab-abab-abab", strings.ReplaceAll(FormatFingerprint(fp), "-", "_")} {
		_, ok := NormalizeFingerprint(bad)
		assert.False(t, ok, "%q", bad)
	}
}

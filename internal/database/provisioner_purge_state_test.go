package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkProvisionerPurgeByState holds the cases of the purge by enrolment state that every provider must
// pass. Its ids carry the prefix "ps-", so the rows of the other cases stay out of what it asserts.
func checkProvisionerPurgeByState(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()
	day := 24 * time.Hour
	month, week := 30*day, 7*day
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	exists := func(id string) bool {
		_, err := s.GetAgent(ctx, id)
		return err == nil
	}
	insert := func(id, state, token string, at time.Time) {
		insertEnrolledRow(t, manager, id, state, token, at)
	}

	// A row of each state, silent for two ages: 8 days and 31 days. A row with no state is a legacy row.
	insert("ps-pending-8d", EnrolmentPending, "tok-ps-pending-8d", ago(8*day))
	insert("ps-pending-6d", EnrolmentPending, "tok-ps-pending-6d", ago(6*day))
	insert("ps-pending-8d-task", EnrolmentPending, "tok-ps-pending-8d-task", ago(8*day))
	insert("ps-preapproved-100d", EnrolmentPreapproved, "tok-ps-preapproved", ago(100*day))
	insert("ps-rejected-100d", EnrolmentRejected, "tok-ps-rejected", ago(100*day))
	register(t, s, adminActor, "ps-legacy-31d", "tok-ps-legacy-31d", ago(31*day))
	register(t, s, adminActor, "ps-legacy-29d", "tok-ps-legacy-29d", ago(29*day))
	register(t, s, adminActor, "ps-null-31d", "tok-ps-null-31d", ago(31*day))
	require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", "ps-null-31d").
		UpdateColumn("enrolment_state", nil).Error)
	register(t, s, adminActor, "ps-null-29d", "tok-ps-null-29d", ago(29*day))
	require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", "ps-null-29d").
		UpdateColumn("enrolment_state", nil).Error)
	// A state this code does not know, and the empty string, are never purged.
	insert("ps-unknown-100d", "something-else", "tok-ps-unknown", ago(100*day))
	insert("ps-empty-100d", "", "tok-ps-empty", ago(100*day))
	require.NoError(t, s.CreateTask(ctx, newTask("ps-pending-task", "pending", "ps-pending-8d-task", 0, ago(8*day))))

	// Three approved rows: past the cutoff and online, inside it, and past it but already ended.
	insert("ps-approved-31d", EnrolmentApproved, "tok-ps-approved-31d", ago(31*day))
	insert("ps-approved-29d", EnrolmentApproved, "tok-ps-approved-29d", ago(29*day))
	insert("ps-approved-31d-ended", EnrolmentApproved, "tok-ps-approved-ended", ago(31*day))
	require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", "ps-approved-31d-ended").
		UpdateColumn("status", "offline").Error)
	for _, task := range []*ProvisioningTask{
		newTask("ps-assigned", "assigned", "ps-approved-31d", 0, ago(31*day)),
		newTask("ps-in-progress", "in_progress", "ps-approved-31d", 0, ago(31*day)),
		newTask("ps-approved-pending", "pending", "ps-approved-31d", 0, ago(31*day)),
	} {
		require.NoError(t, s.CreateTask(ctx, task))
	}
	before := func(id string) ProvisionerAgent {
		a, err := s.GetAgent(ctx, id)
		require.NoError(t, err, id)
		return a
	}
	preapproved, rejected := before("ps-preapproved-100d"), before("ps-rejected-100d")
	unknown, empty := before("ps-unknown-100d"), before("ps-empty-100d")
	approvedEnded, approvedRecent := before("ps-approved-31d-ended"), before("ps-approved-29d")

	res, err := s.Purge(ctx, now, month, week, month)
	require.NoError(t, err)

	t.Run("legacy and stateless rows go after 30 days, pending rows after 7 and only with no pending task", func(t *testing.T) {
		assert.False(t, exists("ps-legacy-31d"), "a legacy row silent for 31 days is removed")
		assert.False(t, exists("ps-null-31d"), "a row with no state counts as legacy")
		assert.True(t, exists("ps-legacy-29d"), "a legacy row silent for 29 days stays")
		assert.True(t, exists("ps-null-29d"), "a row with no state silent for 29 days stays")
		assert.False(t, exists("ps-pending-8d"), "a pending row silent for 8 days is removed")
		assert.True(t, exists("ps-pending-6d"), "a pending row silent for 6 days stays")
		assert.True(t, exists("ps-pending-8d-task"), "a pending row with a pending task stays")
		assert.GreaterOrEqual(t, res.Agents, int64(3), "the legacy, stateless and pending rows removed")
	})

	t.Run("preapproved, rejected, unknown and empty states are never purged and never touched", func(t *testing.T) {
		for id, was := range map[string]ProvisionerAgent{
			"ps-preapproved-100d": preapproved, "ps-rejected-100d": rejected, "ps-unknown-100d": unknown, "ps-empty-100d": empty,
		} {
			now := before(id)
			assert.Equal(t, was.TokenHash, now.TokenHash, id)
			assert.Equal(t, was.Status, now.Status, id)
			assert.Equal(t, was.State(), now.State(), id)
			assert.True(t, was.LastSeen.Equal(now.LastSeen), id)
		}
	})

	t.Run("an approved row past the cutoff keeps its key and approval and loses its session", func(t *testing.T) {
		row := before("ps-approved-31d")
		assert.Equal(t, EnrolmentApproved, row.State())
		require.NotNil(t, row.PublicKey)
		require.NotNil(t, row.Fingerprint)
		assert.Equal(t, "a1ps-approved-31d", *row.PublicKey, "the key stays")
		assert.Equal(t, "f1ps-approved-31d", *row.Fingerprint, "the fingerprint stays")
		assert.Equal(t, "offline", row.Status)
		assert.NotEqual(t, HashAgentToken("tok-ps-approved-31d"), row.TokenHash, "the token is replaced")
		assert.Len(t, row.TokenHash, 64)
		assert.EqualValues(t, 1, res.Expired, "one approved row was expired by this pass")

		_, err := s.ClaimTasks(ctx, tokenActor("tok-ps-approved-31d"), "ps-approved-31d", now)
		assert.ErrorIs(t, err, ErrUnauthorized, "the old token no longer works")

		for _, id := range []string{"ps-assigned", "ps-in-progress"} {
			task, getErr := s.GetTask(ctx, id)
			require.NoError(t, getErr)
			assert.Equal(t, "failed", task.Status, id)
			assert.NotEmpty(t, task.Config["_error"], id)
		}
		pending, err := s.GetTask(ctx, "ps-approved-pending")
		require.NoError(t, err)
		assert.Equal(t, "pending", pending.Status, "a pending task stays, so the agent still receives it")
	})

	t.Run("an approved row inside the cutoff, or already ended, is not touched", func(t *testing.T) {
		for id, was := range map[string]ProvisionerAgent{"ps-approved-29d": approvedRecent, "ps-approved-31d-ended": approvedEnded} {
			now := before(id)
			assert.Equal(t, was.TokenHash, now.TokenHash, id)
			assert.Equal(t, was.Status, now.Status, id)
		}
	})

	t.Run("the next pass changes nothing and counts nothing", func(t *testing.T) {
		row := before("ps-approved-31d")
		again, err := s.Purge(ctx, now.Add(time.Hour), month, week, month)
		require.NoError(t, err)
		assert.EqualValues(t, 0, again.Expired, "the row is offline now, so it is not expired again")
		assert.EqualValues(t, 0, again.Agents)
		after := before("ps-approved-31d")
		assert.Equal(t, row.TokenHash, after.TokenHash, "the replacement token is written once")
		assert.True(t, row.UpdatedAt.Equal(after.UpdatedAt), "the row is not written again")
	})

	// The candidate query uses the shorter of the two periods, whichever it is: with the legacy period the
	// shorter one, a pending row silent for 8 days stays and a legacy row silent for 8 days goes.
	t.Run("the periods are read per state when the pending period is the longer", func(t *testing.T) {
		register(t, s, adminActor, "ps-rev-legacy-8d", "tok-ps-rev-legacy", ago(8*day))
		insert("ps-rev-pending-8d", EnrolmentPending, "tok-ps-rev-pending", ago(8*day))
		_, err := s.Purge(ctx, now, week, month, month)
		require.NoError(t, err)
		assert.False(t, exists("ps-rev-legacy-8d"), "a legacy row silent for 8 days goes when its period is 7")
		assert.True(t, exists("ps-rev-pending-8d"), "a pending row silent for 8 days stays when its period is 30")
	})
}

func TestProvisionerPurgeByState_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkProvisionerPurgeByState(t, manager)
}

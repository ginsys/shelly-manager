package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkProvisionerStrict holds the cases of the strict enrolment mode that every provider must pass: a token
// authorizes only on an approved row, wherever agentByToken is reached. Its ids are strict-* and its
// fingerprints the bytes 0xd1 to 0xd5, which no other case uses.
func checkProvisionerStrict(t *testing.T, manager *Manager) {
	t.Helper()
	open := NewProvisionerStore(manager.GetDB())
	strict := open.WithStrictEnrolment(true)
	ctx := provisionerCtx(t)
	now := time.Now()

	// One row per state, each with its own token; the NULL-state row is the keyless register's.
	register(t, open, adminActor, "strict-legacy", "tok-strict-legacy", now)
	require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", "strict-legacy").UpdateColumn("enrolment_state", nil).Error)
	register(t, open, adminActor, "strict-explicit-legacy", "tok-strict-explicit-legacy", now)
	states := map[string]string{}
	for i, st := range []string{EnrolmentPending, EnrolmentPreapproved, EnrolmentApproved, EnrolmentRejected} {
		states[st] = insertKeyedRow(t, manager, testFingerprint(byte(0xd1+i)), st, "tok-strict-"+st, st != EnrolmentPreapproved, now)
	}
	task := func(id, agent string) {
		require.NoError(t, open.CreateTask(ctx, newTask(id, "pending", agent, 1, now)))
	}

	t.Run("the token of a row that is not approved authorizes nothing in strict mode", func(t *testing.T) {
		ids := map[string]string{
			"a NULL state": "strict-legacy", "legacy": "strict-explicit-legacy", EnrolmentPending: states[EnrolmentPending],
			EnrolmentPreapproved: states[EnrolmentPreapproved], EnrolmentRejected: states[EnrolmentRejected],
		}
		for name, id := range ids {
			token := "tok-" + id
			if name != "a NULL state" && name != "legacy" {
				token = "tok-strict-" + name
			}
			_, err := strict.ClaimTasks(ctx, tokenActor(token), id, now)
			assert.ErrorIs(t, err, ErrUnauthorized, "%s: poll", name)
			_, err = strict.UpdateTaskStatus(ctx, tokenActor(token), "no-such-task", "running", now)
			assert.ErrorIs(t, err, ErrUnauthorized, "%s: task status", name)
			_, _, err = strict.ReportDiscovered(ctx, tokenActor(token), id, "no-such-task", nil, now)
			assert.ErrorIs(t, err, ErrUnauthorized, "%s: report", name)
		}
	})

	t.Run("the same tokens authorize in open mode, and an approved row's in both", func(t *testing.T) {
		for _, id := range []string{"strict-legacy", "strict-explicit-legacy"} {
			_, err := open.ClaimTasks(ctx, tokenActor("tok-"+id), id, now)
			assert.NoError(t, err, "open: %s polls", id)
		}
		for name, s := range map[string]*ProvisionerStore{"open": open, "strict": strict} {
			_, err := s.ClaimTasks(ctx, tokenActor("tok-strict-"+EnrolmentApproved), states[EnrolmentApproved], now)
			assert.NoError(t, err, "%s: an approved agent polls", name)
		}
	})

	t.Run("the admin actor acts for any row in strict mode", func(t *testing.T) {
		task("strict-task", "strict-legacy")
		for _, id := range []string{"strict-legacy", states[EnrolmentPending], states[EnrolmentRejected]} {
			_, err := strict.ClaimTasks(ctx, adminActor, id, now)
			assert.NoError(t, err, id)
		}
		_, err := strict.UpdateTaskStatus(ctx, adminActor, "strict-task", "running", now)
		assert.NoError(t, err)
	})

	t.Run("the mode does not move the original store", func(t *testing.T) {
		_, err := open.ClaimTasks(ctx, tokenActor("tok-strict-legacy"), "strict-legacy", now)
		assert.NoError(t, err, "WithStrictEnrolment returns a copy")
	})
}

func TestProvisionerStrict_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkProvisionerStrict(t, manager)
}

package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertEnrolledRow writes a row as a keyed enrolment will, bypassing the keyless register: a row in
// the given state with a token, last seen long ago so the 5-minute silence rule alone would let anyone
// take it over.
func insertEnrolledRow(t *testing.T, manager *Manager, id, state, token string, at time.Time) {
	t.Helper()
	row := agentRow(id, token, at)
	row.EnrolmentState = strPtr(state)
	if state != EnrolmentPreapproved {
		row.PublicKey = strPtr("a1" + id)
		row.Fingerprint = strPtr("f1" + id)
	}
	require.NoError(t, manager.GetDB().Create(&row).Error, "insert %q", id)
}

// checkProvisionerEnrolment holds the slice-2 cases of the keyless register that every provider must
// pass: SQLite runs them here, PostgreSQL and MySQL in the Database Provider Migration job.
func checkProvisionerEnrolment(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()
	silent := now.Add(-time.Hour)

	t.Run("a keyless register never replaces a row that is not legacy", func(t *testing.T) {
		for _, state := range []string{EnrolmentPending, EnrolmentPreapproved, EnrolmentApproved, EnrolmentRejected} {
			id := "enrolled-" + state
			insertEnrolledRow(t, manager, id, state, "tok-"+id, silent)
			for name, actor := range map[string]ProvisionerActor{
				"agent key, silent row": {},
				"admin key":             adminActor,
				"the row's own token":   tokenActor("tok-" + id),
			} {
				_, created, err := s.RegisterAgent(ctx, actor, agentRow(id, "intruder-"+state+name, now), testOfflineAfter)
				assert.ErrorIs(t, err, ErrAgentEnrolled, "%s, %s", state, name)
				assert.False(t, created)
			}
			row, err := s.GetAgent(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, HashAgentToken("tok-"+id), row.TokenHash, "%s: the token must not move", state)
			assert.Equal(t, state, row.State())
			assert.WithinDuration(t, silent, row.LastSeen, time.Second, "%s: a refused register writes nothing", state)
		}
	})

	t.Run("a legacy row registers as before, whether its state is legacy or NULL", func(t *testing.T) {
		for name, state := range map[string]*string{"legacy": strPtr(EnrolmentLegacy), "null": nil} {
			id := "old-" + name
			row := agentRow(id, "tok-"+id, silent)
			row.EnrolmentState = state
			require.NoError(t, manager.GetDB().Create(&row).Error)

			_, _, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow(id, "new-"+id, now), testOfflineAfter)
			require.NoError(t, err, "%s: a silent legacy row is taken over, as it is today", name)
			_, _, err = s.RegisterAgent(ctx, ProvisionerActor{}, agentRow(id, "again-"+id, now.Add(time.Second)), testOfflineAfter)
			assert.ErrorIs(t, err, ErrAgentInUse, "%s: the 5-minute rule still holds", name)
			_, _, err = s.RegisterAgent(ctx, adminActor, agentRow(id, "admin-"+id, now.Add(time.Second)), testOfflineAfter)
			assert.NoError(t, err, "%s: the admin key still replaces", name)
		}
	})

	t.Run("a new keyless row is written as legacy", func(t *testing.T) {
		_, created, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow("fresh", "tok-fresh", now), testOfflineAfter)
		require.NoError(t, err)
		require.True(t, created)
		row, err := s.GetAgent(ctx, "fresh")
		require.NoError(t, err)
		require.NotNil(t, row.EnrolmentState)
		assert.Equal(t, EnrolmentLegacy, *row.EnrolmentState)
		assert.Nil(t, row.PublicKey)
		assert.Nil(t, row.Fingerprint)
	})

	t.Run("many legacy rows carry no fingerprint and do not collide on its unique index", func(t *testing.T) {
		for _, id := range []string{"nofp-1", "nofp-2", "nofp-3"} {
			register(t, s, adminActor, id, "tok-"+id, now)
		}
	})

	t.Run("the backfill makes every NULL state legacy, twice over, and leaves the rest alone", func(t *testing.T) {
		db := manager.GetDB()
		stale := now.Add(-24 * time.Hour).Truncate(time.Second)
		insertEnrolledRow(t, manager, "bf-approved", EnrolmentApproved, "tok-bf-a", silent)
		old := agentRow("bf-null", "tok-bf-n", silent)
		old.UpdatedAt = stale
		require.NoError(t, db.Create(&old).Error)
		require.NoError(t, db.Model(&ProvisionerAgent{}).Where("agent_id = ?", "bf-null").
			UpdateColumns(map[string]any{"enrolment_state": nil, "updated_at": stale}).Error)

		for pass := 1; pass <= 2; pass++ {
			require.NoError(t, backfillEnrolmentState(db), "pass %d", pass)
			var nulls int64
			require.NoError(t, db.Model(&ProvisionerAgent{}).Where("enrolment_state IS NULL").Count(&nulls).Error)
			assert.EqualValues(t, 0, nulls, "pass %d", pass)
		}
		row, err := s.GetAgent(ctx, "bf-null")
		require.NoError(t, err)
		assert.Equal(t, EnrolmentLegacy, *row.EnrolmentState)
		assert.WithinDuration(t, stale, row.UpdatedAt, time.Second, "the backfill is not an edit of the row")
		kept, err := s.GetAgent(ctx, "bf-approved")
		require.NoError(t, err)
		assert.Equal(t, EnrolmentApproved, kept.State())
	})

	// Last, because it takes the enrolment columns off the table and puts them back.
	t.Run("an upgrade adds the columns to a populated table and the rows become legacy", func(t *testing.T) {
		db := manager.GetDB()
		migrator := db.Migrator()
		require.NoError(t, migrator.DropIndex(&ProvisionerAgent{}, "Fingerprint"))
		for _, column := range []string{"PublicKey", "Fingerprint", "EnrolmentState", "Challenge", "ChallengeExpiresAt", "ApprovedAt"} {
			require.NoError(t, migrator.DropColumn(&ProvisionerAgent{}, column), column)
		}
		for _, id := range []string{"pre-1", "pre-2"} {
			require.NoError(t, db.Exec("INSERT INTO provisioner_agents (agent_id, token_hash, hostname, ip, version, status, last_seen, registered_at, created_at, updated_at) "+
				"VALUES (?, ?, ?, '192.0.2.9', '1.0', 'online', ?, ?, ?, ?)", id, HashAgentToken("tok-"+id), id, silent, silent, silent, silent).Error)
		}

		require.NoError(t, manager.Migrate(&ProvisionerAgent{}), "AutoMigrate must add nullable columns to a table that holds rows")
		require.NoError(t, backfillEnrolmentState(db))

		for _, id := range []string{"pre-1", "pre-2"} {
			row, err := s.GetAgent(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, row.EnrolmentState, id)
			assert.Equal(t, EnrolmentLegacy, *row.EnrolmentState, id)
			assert.Nil(t, row.Fingerprint, id)
		}
		_, _, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow("pre-1", "after-upgrade", now), testOfflineAfter)
		assert.NoError(t, err, "a silent row from before the upgrade is taken over as before")
	})
}

func TestProvisionerEnrolment_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkProvisionerEnrolment(t, manager)
}

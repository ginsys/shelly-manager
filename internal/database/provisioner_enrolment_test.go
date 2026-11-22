package database

import (
	"errors"
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

// checkProvisionerIDRule holds the cases of the id rule for a new keyless row, which every provider must
// pass: a new id is printable ASCII without a space, unless it starts with the exact bytes
// "provisioner-" (the shipped agent's id, whatever hostname follows); a row that exists keeps registering.
func checkProvisionerIDRule(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()
	rows := func(id string) int64 {
		var n int64
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", id).Count(&n).Error)
		return n
	}

	t.Run("a new id with a space or a byte outside 0x21 to 0x7E is refused and leaves no row", func(t *testing.T) {
		for _, id := range []string{
			"a b", " lead", "trail ", "tab\tid", "new\nline", "del\x7fid", "ctl\x1fid",
			"café", "é", "日本", " nbsp", "kéy-abc", "Provisioner-a b", "provisioner_x é",
		} {
			for name, actor := range map[string]ProvisionerActor{"agent key": {}, "admin key": adminActor} {
				_, created, err := s.RegisterAgent(ctx, actor, agentRow(id, "tok-"+name+id, now), testOfflineAfter)
				assert.ErrorIs(t, err, ErrAgentIDInvalid, "%q with the %s", id, name)
				assert.False(t, created)
			}
			assert.EqualValues(t, 0, rows(id), "%q must leave no row", id)
		}
	})

	t.Run("an id that starts with provisioner- passes whatever follows", func(t *testing.T) {
		for _, id := range []string{"provisioner", "provisioner-", "provisioner-host-1", "provisioner-host with space-77", "provisioner-é日本", "provisioner-\ttab"} {
			_, created, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow(id, "tok-"+id, now), testOfflineAfter)
			require.NoError(t, err, "%q", id)
			assert.True(t, created, "%q", id)
		}
	})

	t.Run("every printable ASCII byte is a valid id, and so is a mix of them", func(t *testing.T) {
		for b := 0x21; b <= 0x7e; b++ {
			id := "p-" + string(rune(b)) + "-x"
			_, _, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow(id, "tok-ascii"+id, now), testOfflineAfter)
			assert.NoError(t, err, "%q", id)
		}
		_, _, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow(`!"#$%&'()*+,-./09:;<=>?@AZ[\]^_`+"`"+`az{|}~`, "tok-mix", now), testOfflineAfter)
		assert.NoError(t, err)
	})

	// The id is a path segment of every per-agent route, so a new id that the path clean rewrites (an empty
	// or a "." or ".." segment) or that the request validator refuses in the agent_id query value of the
	// discovered-devices route (".." anywhere) would be created and then be unreachable.
	t.Run("a new id with an empty, a . or a .. segment, or .. anywhere, is refused and leaves no row", func(t *testing.T) {
		for _, id := range []string{
			"a//b", "/a", "a/", "/", "a/./b", "./a", "a/.", ".", "a/../b", "../a", "a/..", "..",
			"a..b", "..a", "a..", "a../b", "...x", "provisioner-x//y", "provisioner-x/../y", "provisioner-a..b", "provisioner-x/",
		} {
			for name, actor := range map[string]ProvisionerActor{"agent key": {}, "admin key": adminActor} {
				_, created, err := s.RegisterAgent(ctx, actor, agentRow(id, "tok-"+name+id, now), testOfflineAfter)
				assert.ErrorIs(t, err, ErrAgentIDUnroutable, "%q with the %s", id, name)
				assert.False(t, created)
			}
			assert.EqualValues(t, 0, rows(id), "%q must leave no row", id)
		}
	})

	t.Run("an id with one inner slash, or dots that are not a . or .. segment, registers", func(t *testing.T) {
		for _, id := range []string{
			"lab/pi-1", "a/b", "a/b/c", "v1.2", "a.b", "a/b.c", "a/.b", "a/b.", ".a", "a.", "provisioner-lab/pi 1",
			"provisioner-host.example.com-42", "provisioner-/x",
		} {
			_, created, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow(id, "tok-pin"+id, now), testOfflineAfter)
			require.NoError(t, err, "%q", id)
			assert.True(t, created, "%q", id)
		}
	})

	t.Run("a row that exists keeps registering, whatever its id holds", func(t *testing.T) {
		silent := now.Add(-time.Hour)
		for _, id := range []string{"café ünï", "with space", "日本語"} {
			require.NoError(t, manager.GetDB().Create(&ProvisionerAgent{
				AgentID: id, TokenHash: HashAgentToken("old-" + id), Hostname: "h", Status: "online", LastSeen: silent, RegisteredAt: silent,
			}).Error)
			_, created, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow(id, "new-"+id, now), testOfflineAfter)
			require.NoError(t, err, "%q: a silent existing row is taken over as before", id)
			assert.False(t, created, "%q", id)
			_, _, err = s.RegisterAgent(ctx, adminActor, agentRow(id, "admin-"+id, now.Add(time.Second)), testOfflineAfter)
			assert.NoError(t, err, "%q: the admin key still replaces", id)
		}
	})

	t.Run("an id that a folding collation would equate with an enrolled key's is refused either way", func(t *testing.T) {
		// "kéyed-…" is the accent-folded spelling of "keyed-…". Under a binary collation it is a new id and the
		// id rule refuses it; under a folding collation it finds the keyed row and the state check refuses it.
		insertEnrolledRow(t, manager, "keyed-fold-demo", EnrolmentApproved, "tok-key-fold", now.Add(-time.Hour))
		_, _, err := s.RegisterAgent(ctx, adminActor, agentRow("kéyed-fold-demo", "intruder", now), testOfflineAfter)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAgentIDInvalid) || errors.Is(err, ErrAgentEnrolled), "got %v", err)
		row, getErr := s.GetAgent(ctx, "keyed-fold-demo")
		require.NoError(t, getErr)
		assert.Equal(t, HashAgentToken("tok-key-fold"), row.TokenHash, "the keyed row is untouched")
	})
}

func TestProvisionerEnrolment_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkProvisionerIDRule(t, manager)
	checkProvisionerEnrolment(t, manager)
}

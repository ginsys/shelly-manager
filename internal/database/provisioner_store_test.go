package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every store test runs under a context with a deadline. On SQLite the pool has one connection, so a
// store call made while a store transaction is open waits for ever; the deadline turns that into a
// failure of the test in seconds, with a message (ProvisionerStore doc comment).
func provisionerCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

const testOfflineAfter = 5 * time.Minute

var adminActor = ProvisionerActor{Privileged: true}

func tokenActor(token string) ProvisionerActor {
	return ProvisionerActor{TokenHash: HashAgentToken(token)}
}

func agentRow(id, token string, at time.Time) ProvisionerAgent {
	return ProvisionerAgent{
		AgentID: id, TokenHash: HashAgentToken(token), Hostname: id + ".local", IP: "192.0.2.7",
		Status: "online", LastSeen: at, RegisteredAt: at,
	}
}

func register(t *testing.T, s *ProvisionerStore, actor ProvisionerActor, id, token string, at time.Time) {
	t.Helper()
	_, _, err := s.RegisterAgent(provisionerCtx(t), actor, agentRow(id, token, at), testOfflineAfter)
	require.NoError(t, err, "register %q", id)
}

func newTask(id, status, agent string, priority int, at time.Time) *ProvisioningTask {
	return &ProvisioningTask{TaskID: id, Type: "discover", Status: status, AgentID: agent, Priority: priority, CreatedAt: at, UpdatedAt: at}
}

func taskIDs(tasks []ProvisioningTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.TaskID)
	}
	return ids
}

// checkProvisionerStore holds the cases every provider must pass, so SQLite runs them here and
// PostgreSQL and MySQL run them in the Database Provider Migration job.
func checkProvisionerStore(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()

	t.Run("ids are compared byte for byte", func(t *testing.T) {
		// "alpha " is a row from before the id rule (a new id with a space is refused now), so it is written
		// directly; the other two register after it and must not be read as it, whatever the collation pads.
		old := agentRow("alpha ", "tok-alpha ", now)
		require.NoError(t, manager.GetDB().Create(&old).Error, "insert the pre-rule row")
		for _, id := range []string{"alpha", "Alpha"} {
			_, created, err := s.RegisterAgent(ctx, adminActor, agentRow(id, "tok-"+id, now), testOfflineAfter)
			require.NoError(t, err, "register %q", id)
			assert.True(t, created, "%q must be a new agent, not a re-register of another id", id)
		}
		var n int64
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id IN ?", []string{"alpha", "Alpha", "alpha "}).Count(&n).Error)
		assert.EqualValues(t, 3, n)
	})

	t.Run("the token is stored hashed and never serialised", func(t *testing.T) {
		register(t, s, adminActor, "hashed", "tok-secret", now)
		var stored string
		require.NoError(t, manager.GetDB().Raw("SELECT token_hash FROM provisioner_agents WHERE agent_id = ?", "hashed").Scan(&stored).Error)
		assert.Equal(t, HashAgentToken("tok-secret"), stored)
		assert.Len(t, stored, 64)
		agent, err := s.GetAgent(ctx, "hashed")
		require.NoError(t, err)
		body, err := json.Marshal(agent)
		require.NoError(t, err)
		assert.NotContains(t, string(body), stored)
		assert.NotContains(t, string(body), "tok-secret")
		assert.Empty(t, HashAgentToken(""))
	})

	t.Run("a registered id is held for five minutes and a new token replaces the old", func(t *testing.T) {
		register(t, s, adminActor, "held", "tok-1", now)
		_, _, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow("held", "tok-x", now.Add(time.Minute)), testOfflineAfter)
		assert.ErrorIs(t, err, ErrAgentInUse, "no token")
		_, _, err = s.RegisterAgent(ctx, tokenActor("wrong"), agentRow("held", "tok-x", now.Add(time.Minute)), testOfflineAfter)
		assert.ErrorIs(t, err, ErrAgentInUse, "wrong token")

		_, created, err := s.RegisterAgent(ctx, tokenActor("tok-1"), agentRow("held", "tok-2", now.Add(time.Minute)), testOfflineAfter)
		require.NoError(t, err, "the holder re-registers")
		assert.False(t, created)

		_, err = s.ClaimTasks(ctx, tokenActor("tok-1"), "held", now)
		assert.ErrorIs(t, err, ErrUnauthorized, "the replaced token authorizes nothing")
		_, err = s.ClaimTasks(ctx, tokenActor("tok-2"), "held", now)
		assert.NoError(t, err)

		_, _, err = s.RegisterAgent(ctx, ProvisionerActor{}, agentRow("held", "tok-3", now.Add(2*time.Minute)), testOfflineAfter)
		assert.ErrorIs(t, err, ErrAgentInUse, "still online")
		_, _, err = s.RegisterAgent(ctx, adminActor, agentRow("held", "tok-3", now.Add(2*time.Minute)), testOfflineAfter)
		assert.NoError(t, err, "the admin key may take any id")
		_, _, err = s.RegisterAgent(ctx, ProvisionerActor{}, agentRow("held", "tok-4", now.Add(10*time.Minute)), testOfflineAfter)
		assert.NoError(t, err, "silent for more than five minutes")
	})

	t.Run("who may act for an agent", func(t *testing.T) {
		register(t, s, adminActor, "who-a", "tok-who-a", now)
		register(t, s, adminActor, "who-b", "tok-who-b", now)
		_, err := s.ClaimTasks(ctx, ProvisionerActor{}, "who-a", now)
		assert.ErrorIs(t, err, ErrUnauthorized, "an empty token never matches")
		_, err = s.ClaimTasks(ctx, tokenActor("nobody"), "who-a", now)
		assert.ErrorIs(t, err, ErrUnauthorized)
		_, err = s.ClaimTasks(ctx, tokenActor("tok-who-b"), "who-a", now)
		assert.ErrorIs(t, err, ErrForbidden)
		_, err = s.ClaimTasks(ctx, adminActor, "who-missing", now)
		assert.ErrorIs(t, err, ErrAgentNotFound)
		_, err = s.ClaimTasks(ctx, adminActor, "who-a", now)
		assert.NoError(t, err)
	})

	t.Run("a poll claims what is claimable, once, in priority order", func(t *testing.T) {
		register(t, s, adminActor, "claimer", "tok-claimer", now)
		for i, task := range []*ProvisioningTask{
			newTask("c-low", "pending", "", 0, now),
			newTask("c-high", "pending", "", 5, now.Add(time.Second)),
			newTask("c-mine", "pending", "claimer", 0, now.Add(2*time.Second)),
			newTask("c-other", "pending", "somebody-else", 9, now.Add(3*time.Second)),
			newTask("c-upper", "PENDING", "", 9, now.Add(4*time.Second)),
			newTask("c-done", "completed", "", 9, now.Add(5*time.Second)),
		} {
			require.NoError(t, s.CreateTask(ctx, task), "task %d", i)
		}

		claimed, err := s.ClaimTasks(ctx, tokenActor("tok-claimer"), "claimer", now)
		require.NoError(t, err)
		assert.Equal(t, []string{"c-high", "c-low", "c-mine"}, taskIDs(claimed))
		for _, task := range claimed {
			assert.Equal(t, "assigned", task.Status)
			assert.Equal(t, "claimer", task.AgentID)
		}
		again, err := s.ClaimTasks(ctx, tokenActor("tok-claimer"), "claimer", now)
		require.NoError(t, err)
		assert.Empty(t, again)
		for id, want := range map[string]string{"c-other": "pending", "c-upper": "PENDING", "c-done": "completed"} {
			got, getErr := s.GetTask(ctx, id)
			require.NoError(t, getErr)
			assert.Equal(t, want, got.Status, "task %s", id)
		}
	})

	t.Run("a task status is updated by its agent or the admin", func(t *testing.T) {
		register(t, s, adminActor, "st-a", "tok-st-a", now)
		register(t, s, adminActor, "st-b", "tok-st-b", now)
		require.NoError(t, s.CreateTask(ctx, newTask("st-1", "assigned", "st-a", 0, now)))
		require.NoError(t, s.CreateTask(ctx, newTask("st-free", "pending", "", 0, now)))

		_, err := s.UpdateTaskStatus(ctx, tokenActor("tok-st-b"), "st-1", "completed", now)
		assert.ErrorIs(t, err, ErrForbidden, "another agent's task")
		_, err = s.UpdateTaskStatus(ctx, tokenActor("tok-st-a"), "st-free", "completed", now)
		assert.ErrorIs(t, err, ErrForbidden, "an unassigned task is nobody's")
		_, err = s.UpdateTaskStatus(ctx, tokenActor("tok-st-a"), "st-missing", "completed", now)
		assert.ErrorIs(t, err, ErrForbidden, "an agent learns nothing about which ids exist")
		_, err = s.UpdateTaskStatus(ctx, tokenActor("nobody"), "st-1", "completed", now)
		assert.ErrorIs(t, err, ErrUnauthorized)
		_, err = s.UpdateTaskStatus(ctx, adminActor, "st-missing", "completed", now)
		assert.ErrorIs(t, err, ErrTaskNotFound)

		task, err := s.UpdateTaskStatus(ctx, tokenActor("tok-st-a"), "st-1", "in_progress", now.Add(time.Second))
		require.NoError(t, err)
		assert.Equal(t, "in_progress", task.Status)
		task, err = s.UpdateTaskStatus(ctx, adminActor, "st-free", "failed", now.Add(time.Second))
		require.NoError(t, err)
		assert.Equal(t, "failed", task.Status)
		// An update that changes no column reports 0 affected rows on MySQL: it is still an update of an existing task.
		task, err = s.UpdateTaskStatus(ctx, adminActor, "st-free", "failed", now.Add(time.Second))
		require.NoError(t, err)
		assert.Equal(t, "failed", task.Status)
	})

	t.Run("a report upserts devices and completes only its own task", func(t *testing.T) {
		register(t, s, adminActor, "rp-a", "tok-rp-a", now)
		register(t, s, adminActor, "rp-b", "tok-rp-b", now)
		require.NoError(t, s.CreateTask(ctx, newTask("rp-mine", "assigned", "rp-a", 0, now)))
		require.NoError(t, s.CreateTask(ctx, newTask("rp-theirs", "assigned", "rp-b", 0, now)))
		device := func(mac, ip string) DiscoveredDevice {
			return DiscoveredDevice{MAC: mac, IP: ip, Model: "SHSW-1", Discovered: now, ExpiresAt: now.Add(24 * time.Hour)}
		}

		_, _, err := s.ReportDiscovered(ctx, tokenActor("tok-rp-a"), "rp-a", "rp-theirs", []DiscoveredDevice{device("AA:01", "192.0.2.1")}, now)
		require.NoError(t, err)
		theirs, err := s.GetTask(ctx, "rp-theirs")
		require.NoError(t, err)
		assert.Equal(t, "assigned", theirs.Status, "a report must not complete another agent's task")

		persisted, failures, err := s.ReportDiscovered(ctx, tokenActor("tok-rp-a"), "rp-a", "rp-mine",
			[]DiscoveredDevice{device("AA:01", "192.0.2.2"), device(strings.Repeat("M", 300), "192.0.2.3"), device("AA:02", "192.0.2.4")}, now)
		require.NoError(t, err)
		assert.Equal(t, 3, persisted+len(failures), "every device is persisted or reported as failed")
		assert.LessOrEqual(t, len(failures), 1, "only the 300-character MAC may fail, and only where the column has a length: %v", failures)
		mine, err := s.GetTask(ctx, "rp-mine")
		require.NoError(t, err)
		assert.Equal(t, "completed", mine.Status)

		var rows []DiscoveredDevice
		require.NoError(t, manager.GetDB().Where("agent_id = ?", "rp-a").Order("mac").Find(&rows).Error)
		byMAC := map[string]string{}
		for _, row := range rows {
			byMAC[row.MAC] = row.IP
		}
		assert.Equal(t, "192.0.2.2", byMAC["AA:01"], "the second report updates the row, never adds one")
		assert.Equal(t, "192.0.2.4", byMAC["AA:02"], "devices after a failed one are still stored")

		_, _, err = s.ReportDiscovered(ctx, adminActor, "rp-a", "rp-theirs", []DiscoveredDevice{device("AA:03", "192.0.2.5")}, now)
		require.NoError(t, err)
		theirs, err = s.GetTask(ctx, "rp-theirs")
		require.NoError(t, err)
		assert.Equal(t, "completed", theirs.Status, "the admin key may complete any task")

		_, _, err = s.ReportDiscovered(ctx, tokenActor("tok-rp-a"), "rp-a", "", nil, now)
		assert.ErrorIs(t, err, ErrNoDevices)
		_, _, err = s.ReportDiscovered(ctx, adminActor, "", "", []DiscoveredDevice{device("AA:04", "192.0.2.6")}, now)
		assert.ErrorIs(t, err, ErrAgentIDRequired)
		_, _, err = s.ReportDiscovered(ctx, tokenActor("tok-rp-a"), "", "", []DiscoveredDevice{device("AA:04", "192.0.2.6")}, now)
		assert.ErrorIs(t, err, ErrForbidden)
		_, _, err = s.ReportDiscovered(ctx, adminActor, "rp-missing", "", []DiscoveredDevice{device("AA:04", "192.0.2.6")}, now)
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})

	t.Run("cancel, list and count", func(t *testing.T) {
		before, err := s.Counts(ctx, now, testOfflineAfter)
		require.NoError(t, err)
		require.NoError(t, s.CreateTask(ctx, newTask("cn-1", "pending", "", 0, now)))
		require.NoError(t, s.CreateTask(ctx, newTask("cn-2", "completed", "", 0, now)))
		after, err := s.Counts(ctx, now, testOfflineAfter)
		require.NoError(t, err)
		assert.EqualValues(t, before.Tasks+2, after.Tasks)
		assert.EqualValues(t, before.PendingTasks+1, after.PendingTasks)

		canceled, err := s.CancelTask(ctx, "cn-1", now)
		require.NoError(t, err)
		assert.Equal(t, "failed", canceled.Status)
		assert.Equal(t, "canceled", canceled.Config["_error"])
		finished, err := s.CancelTask(ctx, "cn-2", now)
		require.NoError(t, err)
		assert.Equal(t, "completed", finished.Status, "a finished task is not canceled")
		_, err = s.CancelTask(ctx, "cn-missing", now)
		assert.ErrorIs(t, err, ErrTaskNotFound)

		tasks, err := s.ListTasks(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(tasks), 2)
		_, err = s.GetAgent(ctx, "no-such-agent")
		assert.ErrorIs(t, err, ErrAgentNotFound)
	})

	t.Run("a long config round-trips and a 192-character id is refused", func(t *testing.T) {
		big := strings.Repeat("x", 8<<20)
		task := newTask("big", "pending", "", 0, now)
		task.Config = map[string]interface{}{"blob": big}
		require.NoError(t, s.CreateTask(ctx, task))
		got, err := s.GetTask(ctx, "big")
		require.NoError(t, err)
		assert.Equal(t, big, got.Config["blob"])

		// SQLite does not enforce a column's length; PostgreSQL and MySQL do, and the handler (PR B) refuses first.
		if manager.GetDB().Name() != "sqlite" {
			_, _, err = s.RegisterAgent(ctx, adminActor, agentRow(strings.Repeat("i", 192), "tok-long", now), testOfflineAfter)
			assert.Error(t, err)
		}
		_, _, err = s.RegisterAgent(ctx, adminActor, agentRow(strings.Repeat("i", 191), "tok-long-191", now), testOfflineAfter)
		assert.NoError(t, err)
	})

	t.Run("a replaced token cannot authorize a request that began before the replace", func(t *testing.T) {
		checkReplaceWaitsForRequestInFlight(t, manager, "flight-token", false)
	})

	t.Run("an admin poll takes the agent lock too", func(t *testing.T) {
		checkReplaceWaitsForRequestInFlight(t, manager, "flight-admin", true)
	})

	t.Run("twenty registers of one new id: one wins", func(t *testing.T) { checkRegisterRace(t, manager) })

	t.Run("delete and purge remove old rows and fail stranded tasks", func(t *testing.T) { checkDeleteAndPurge(t, manager) })

	t.Run("a create for an agent waits for a purge that holds the agent", func(t *testing.T) { checkCreateWaitsForPurge(t, manager) })

	// Last: see checkClaimOnce.
	t.Run("two agents poll one task: it is claimed once", func(t *testing.T) { checkClaimOnce(t, manager) })
}

// checkDeleteAndPurge runs the delete and the purge on the provider's own SQL. Its agent and task ids carry
// a prefix, so the rows of the other cases stay out of its counts.
func checkDeleteAndPurge(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()
	old, recent := now.Add(-31*24*time.Hour), now.Add(-29*24*time.Hour)
	month := 30 * 24 * time.Hour

	for _, id := range []string{"gone", "gone-assigned", "kept-pending", "kept-recent", "deleted"} {
		register(t, s, adminActor, "pg-"+id, "tok-pg-"+id, now)
	}
	for _, id := range []string{"gone", "gone-assigned", "kept-pending"} {
		require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", "pg-"+id).Update("last_seen", old).Error)
	}
	require.NoError(t, manager.GetDB().Model(&ProvisionerAgent{}).Where("agent_id = ?", "pg-kept-recent").Update("last_seen", recent).Error)
	for _, task := range []*ProvisioningTask{
		newTask("pg-stranded", "assigned", "pg-gone-assigned", 0, old),
		newTask("pg-in-progress", "in_progress", "pg-deleted", 0, old),
		newTask("pg-pending", "pending", "pg-kept-pending", 0, old),
		newTask("pg-old-completed", "completed", "", 0, old),
		newTask("pg-old-failed", "failed", "", 0, old),
		newTask("pg-recent-completed", "completed", "", 0, recent),
	} {
		require.NoError(t, s.CreateTask(ctx, task))
	}

	require.ErrorIs(t, s.DeleteAgent(ctx, "pg-nobody", now), ErrAgentNotFound)
	require.NoError(t, s.DeleteAgent(ctx, "pg-deleted", now))
	got, err := s.GetTask(ctx, "pg-in-progress")
	require.NoError(t, err)
	assert.Equal(t, "failed", got.Status)
	assert.NotEmpty(t, got.Config["_error"])

	res, err := s.Purge(ctx, now, month, 7*24*time.Hour, month)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Agents, int64(2), "pg-gone and pg-gone-assigned")
	assert.GreaterOrEqual(t, res.Tasks, int64(2), "pg-old-completed and pg-old-failed")

	_, err = s.GetAgent(ctx, "pg-gone")
	assert.ErrorIs(t, err, ErrAgentNotFound)
	_, err = s.GetAgent(ctx, "pg-gone-assigned")
	assert.ErrorIs(t, err, ErrAgentNotFound)
	for _, id := range []string{"pg-kept-pending", "pg-kept-recent"} {
		_, err = s.GetAgent(ctx, id)
		assert.NoError(t, err, id)
	}
	got, err = s.GetTask(ctx, "pg-stranded")
	require.NoError(t, err)
	assert.Equal(t, "failed", got.Status)
	assert.NotEmpty(t, got.Config["_error"])
	for _, id := range []string{"pg-old-completed", "pg-old-failed"} {
		_, err = s.GetTask(ctx, id)
		assert.ErrorIs(t, err, ErrTaskNotFound, id)
	}
	for _, id := range []string{"pg-pending", "pg-recent-completed"} {
		_, err = s.GetTask(ctx, id)
		assert.NoError(t, err, id)
	}
}

// checkReplaceWaitsForRequestInFlight holds a poll between "I hold the agent's lock" and "I read my
// tasks", starts a register that replaces the token, and asserts that the register does not finish
// while the poll is in flight. The register must then finish, and only the new token works.
func checkReplaceWaitsForRequestInFlight(t *testing.T, manager *Manager, id string, admin bool) {
	t.Helper()
	ctx := provisionerCtx(t)
	now := time.Now()
	oldToken, newToken := "tok-"+id+"-1", "tok-"+id+"-2"
	pollActor := tokenActor(oldToken)
	if admin {
		pollActor = adminActor
	}
	holder := NewProvisionerStore(manager.GetDB())
	register(t, holder, adminActor, id, oldToken, now)

	locked, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	holder.afterLock = func() {
		once.Do(func() {
			close(locked)
			<-release
		})
	}
	pollDone := make(chan error, 1)
	go func() {
		_, err := holder.ClaimTasks(ctx, pollActor, id, now)
		pollDone <- err
	}()
	<-locked

	replaceDone := make(chan error, 1)
	go func() {
		_, _, err := NewProvisionerStore(manager.GetDB()).RegisterAgent(ctx, tokenActor(oldToken), agentRow(id, newToken, now), testOfflineAfter)
		replaceDone <- err
	}()
	select {
	case err := <-replaceDone:
		t.Fatalf("the register finished while a request under the old token was in flight (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-pollDone)
	require.NoError(t, <-replaceDone)

	fresh := NewProvisionerStore(manager.GetDB())
	_, err := fresh.ClaimTasks(ctx, tokenActor(oldToken), id, now)
	assert.ErrorIs(t, err, ErrUnauthorized)
	_, err = fresh.ClaimTasks(ctx, tokenActor(newToken), id, now)
	assert.NoError(t, err)
}

// checkCreateWaitsForPurge holds a purge between "I hold the agent's lock" and "I look for a pending task",
// starts a task for that agent, and asserts that the create does not finish while the purge is in flight. A
// purge that did not lock out the create could delete the agent after the create's task was committed.
func checkCreateWaitsForPurge(t *testing.T, manager *Manager) {
	t.Helper()
	ctx := provisionerCtx(t)
	now := time.Now()
	month := 30 * 24 * time.Hour
	holder := NewProvisionerStore(manager.GetDB())
	// The purge must meet no other silent agent first, or its hook would fire on that one.
	require.NoError(t, manager.GetDB().Where("agent_id LIKE ?", "pg-%").Delete(&ProvisionerAgent{}).Error)
	register(t, holder, adminActor, "pg-purge-race", "tok-pg-race", now.Add(-31*24*time.Hour))

	locked, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	holder.afterLock = func() {
		once.Do(func() {
			close(locked)
			<-release
		})
	}
	purgeDone := make(chan error, 1)
	go func() {
		_, err := holder.Purge(ctx, now, month, 7*24*time.Hour, month)
		purgeDone <- err
	}()
	<-locked

	createDone := make(chan error, 1)
	go func() {
		createDone <- NewProvisionerStore(manager.GetDB()).CreateTask(ctx, newTask("pg-race-task", "pending", "pg-purge-race", 0, now))
	}()
	select {
	case err := <-createDone:
		t.Fatalf("the create finished while the purge held the agent's row (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-purgeDone)
	require.NoError(t, <-createDone)

	// Either order is consistent: the purge removed an agent with no pending task, and the task came after.
	_, agentErr := NewProvisionerStore(manager.GetDB()).GetAgent(ctx, "pg-purge-race")
	assert.ErrorIs(t, agentErr, ErrAgentNotFound)
	_, err := NewProvisionerStore(manager.GetDB()).GetTask(ctx, "pg-race-task")
	assert.NoError(t, err)
}

// checkReRegisterReplacesToken reads whether a register found the id from the stored row. A MySQL DSN with
// clientFoundRows reports 1 affected row for the no-op insert of an existing id, and a store that took
// that for an insert would answer the holder with a token that authorizes nothing and keep the old one.
func checkReRegisterReplacesToken(t *testing.T, manager *Manager) {
	t.Helper()
	s := NewProvisionerStore(manager.GetDB())
	ctx := provisionerCtx(t)
	now := time.Now()
	register(t, s, adminActor, "found-rows", "tok-fr-1", now)

	_, created, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow("found-rows", "tok-fr-x", now), testOfflineAfter)
	assert.ErrorIs(t, err, ErrAgentInUse, "an online id is not taken without its token")
	assert.False(t, created)
	_, created, err = s.RegisterAgent(ctx, tokenActor("tok-fr-1"), agentRow("found-rows", "tok-fr-2", now), testOfflineAfter)
	require.NoError(t, err)
	assert.False(t, created, "the id existed")
	_, err = s.ClaimTasks(ctx, tokenActor("tok-fr-1"), "found-rows", now)
	assert.ErrorIs(t, err, ErrUnauthorized, "the replaced token authorizes nothing")
	_, err = s.ClaimTasks(ctx, tokenActor("tok-fr-2"), "found-rows", now)
	assert.NoError(t, err, "the new token works")
}

func TestProvisionerStore_SQLite(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()
	checkProvisionerStore(t, manager)
}

// checkRegisterRace has twenty callers register one new id with no token: one gets the id and the others
// find it in use. Any other error, a database deadlock included, fails the test by name. On SQLite the
// pool has one connection and the callers run one after another; only PostgreSQL and MySQL prove that the
// insert-first register does not deadlock.
func checkRegisterRace(t *testing.T, manager *Manager) {
	t.Helper()
	ctx := provisionerCtx(t)
	now := time.Now()

	const callers = 20
	var wins, inUse atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := NewProvisionerStore(manager.GetDB())
			_, _, err := s.RegisterAgent(ctx, ProvisionerActor{}, agentRow("race", fmt.Sprintf("tok-race-%d", i), now), testOfflineAfter)
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, ErrAgentInUse):
				inUse.Add(1)
			default:
				t.Errorf("a concurrent register failed with a database error (a deadlock fails here): %v", err)
			}
		}()
	}
	wg.Wait()
	assert.EqualValues(t, 1, wins.Load(), "exactly one register gets the id")
	assert.EqualValues(t, callers-1, inUse.Load(), "every other register finds it in use")
}

// checkClaimOnce has two agents poll one unassigned task at once, twenty times: exactly one gets it each
// time. Run it last: every task it creates ends assigned, so it leaves nothing for another case to claim.
func checkClaimOnce(t *testing.T, manager *Manager) {
	t.Helper()
	ctx := provisionerCtx(t)
	now := time.Now()
	s := NewProvisionerStore(manager.GetDB())
	register(t, s, adminActor, "claim-a", "tok-claim-a", now)
	register(t, s, adminActor, "claim-b", "tok-claim-b", now)

	for round := 0; round < 20; round++ {
		taskID := fmt.Sprintf("once-%d", round)
		require.NoError(t, s.CreateTask(ctx, newTask(taskID, "pending", "", 0, now)))
		var claimed atomic.Int32
		var wg sync.WaitGroup
		for _, who := range []struct{ id, token string }{{"claim-a", "tok-claim-a"}, {"claim-b", "tok-claim-b"}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tasks, err := NewProvisionerStore(manager.GetDB()).ClaimTasks(ctx, tokenActor(who.token), who.id, now)
				if err != nil {
					t.Errorf("a concurrent claim failed with a database error: %v", err)
					return
				}
				for _, task := range tasks {
					if task.TaskID == taskID {
						claimed.Add(1)
					}
				}
			}()
		}
		wg.Wait()
		require.EqualValues(t, 1, claimed.Load(), "round %d", round)
	}
}

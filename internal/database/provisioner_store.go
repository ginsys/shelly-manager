package database

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Errors a handler maps to a response. None of them carries a token or a hash.
var (
	ErrAgentInUse = errors.New("agent id in use")
	// ErrAgentEnrolled is a keyless register of an id whose row is not legacy. Only an enrolled key may
	// replace such a row, so no actor can, the admin key included.
	ErrAgentEnrolled   = errors.New("agent id belongs to an enrolled key")
	ErrUnauthorized    = errors.New("agent token required")
	ErrForbidden       = errors.New("the agent token does not belong to this agent")
	ErrAgentNotFound   = errors.New("agent not found")
	ErrTaskNotFound    = errors.New("task not found")
	ErrAgentIDRequired = errors.New("agent id required")
	ErrNoDevices       = errors.New("at least one device is required")
	// ErrStatusTooLong is a status longer than the status column holds. It is returned after the caller is
	// authorized, so a refused caller never learns it from this answer.
	ErrStatusTooLong = errors.New("task status is too long")
)

// MaxProvisionerValueLen is the longest id, type or status, in characters, that the registry columns hold.
const MaxProvisionerValueLen = 191

// ProvisionerActor is who calls: the admin key (Privileged), or the holder of an agent token. TokenHash
// is HashAgentToken of the presented token, and is empty when none was presented.
type ProvisionerActor struct {
	Privileged bool
	TokenHash  string
}

// ProvisionerStore keeps the provisioner registry (agents, their hashed tokens and their tasks) in
// the database, so that every replica and every restart sees one registry.
//
// Locking: each operation that authorizes an agent runs in one transaction that locks the agent row
// (SELECT ... FOR UPDATE) before it reads or writes anything of that agent, so a register that replaces
// a token waits for the requests already in flight under the old one. Register is the one operation
// whose first statement is an INSERT, see RegisterAgent. A task row is locked after the agent row, never
// before it, so the two kinds of row cannot be locked in opposite orders. SQLite has one connection and
// drops the lock clause, so there a transaction is the lock.
//
// One cycle remains on MySQL: a register that replaces a token locks the agent row by its id, and a
// request that presents the old token locks the same row through the token's index entry, which the
// register then needs to replace. InnoDB detects the cycle and aborts one of the two with a deadlock
// error. The aborted transaction rolls back: nothing is authorized or written by it, so the outcome is a
// failed request and never a wrong answer. The caller gets the database error and decides whether to
// retry; the provisioning agent logs a failed task status update and does not retry it.
//
// Never call the database through another handle while a store transaction is open, and never from
// inside one: on SQLite that call waits for the one connection for ever. Resolve everything the store
// needs first, and pass a context with a deadline, so that a mistake fails instead of hanging.
type ProvisionerStore struct {
	db *gorm.DB
	// afterLock is a test hook, called once the agent row is locked.
	afterLock func()
}

// NewProvisionerStore returns a store over db.
func NewProvisionerStore(db *gorm.DB) *ProvisionerStore { return &ProvisionerStore{db: db} }

// HashAgentToken is the form in which a token is stored: lowercase hex SHA-256. An empty token has no hash.
func HashAgentToken(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *ProvisionerStore) tx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(fn)
}

func forUpdate() clause.Locking { return clause.Locking{Strength: "UPDATE"} }

// holdsToken compares two token hashes in constant time. An empty presented hash never matches.
func holdsToken(presented, stored string) bool {
	return presented != "" && subtle.ConstantTimeCompare([]byte(presented), []byte(stored)) == 1
}

// agentByToken locks the agent that holds the presented token. No agent holds it: ErrUnauthorized.
func (s *ProvisionerStore) agentByToken(tx *gorm.DB, actor ProvisionerActor) (*ProvisionerAgent, error) {
	if actor.TokenHash == "" {
		return nil, ErrUnauthorized
	}
	var agent ProvisionerAgent
	err := tx.Clauses(forUpdate()).Where("token_hash = ?", actor.TokenHash).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if s.afterLock != nil {
		s.afterLock()
	}
	return &agent, nil
}

// lockAgent decides whether the actor may act for the agent want and locks that agent's row. The admin
// key may act for any agent and gets the row, or nil when want is not registered. Anyone else needs
// the token issued to want: no agent holds it is ErrUnauthorized, another agent's token ErrForbidden.
func (s *ProvisionerStore) lockAgent(tx *gorm.DB, actor ProvisionerActor, want string) (*ProvisionerAgent, error) {
	if !actor.Privileged {
		agent, err := s.agentByToken(tx, actor)
		if err != nil {
			return nil, err
		}
		if agent.AgentID != want {
			return nil, ErrForbidden
		}
		return agent, nil
	}
	if want == "" {
		return nil, nil
	}
	var agent ProvisionerAgent
	err := tx.Clauses(forUpdate()).Where("agent_id = ?", want).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if s.afterLock != nil {
		s.afterLock()
	}
	return &agent, nil
}

// RegisterAgent stores agent (its AgentID, TokenHash, LastSeen and RegisteredAt set by the caller), or
// replaces the token and details of the agent that holds the id. An id that is online belongs to the
// agent that holds its token: unless the actor is privileged, or holds the current token, or the agent
// was last seen more than offlineAfter ago, the id is in use. A row whose state is not legacy is never
// replaced by this keyless register, by any actor (ErrAgentEnrolled), whatever its age. It reports
// whether the agent is new.
func (s *ProvisionerStore) RegisterAgent(ctx context.Context, actor ProvisionerActor, agent ProvisionerAgent, offlineAfter time.Duration) (ProvisionerAgent, bool, error) {
	var out ProvisionerAgent
	created := false
	err := s.tx(ctx, func(tx *gorm.DB) error {
		// A keyless register writes a legacy row, whatever the caller put in the state.
		legacy := EnrolmentLegacy
		agent.EnrolmentState = &legacy
		// Insert first: a SELECT ... FOR UPDATE of a missing key takes a gap lock on InnoDB, and two
		// replicas registering one new id would deadlock on it. Whether the insert took is read from the
		// locked row and not from the affected-row count, which MySQL reports as 1 for the no-op of an
		// existing id when the DSN sets clientFoundRows.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&agent).Error; err != nil {
			return err
		}
		cur := ProvisionerAgent{}
		if err := tx.Clauses(forUpdate()).Where("agent_id = ?", agent.AgentID).First(&cur).Error; err != nil {
			return err
		}
		if s.afterLock != nil {
			s.afterLock()
		}
		// The token is new and random, so the row holds its hash only if this insert wrote the row.
		if cur.TokenHash == agent.TokenHash {
			out, created = cur, true
			return nil
		}
		// A row with a key, or waiting for one, is replaced by a signed register and by nothing else: no
		// token, no silence and no admin key lets a keyless register take it.
		if cur.State() != EnrolmentLegacy {
			return ErrAgentEnrolled
		}
		if !actor.Privileged && agent.LastSeen.Sub(cur.LastSeen) <= offlineAfter && !holdsToken(actor.TokenHash, cur.TokenHash) {
			return ErrAgentInUse
		}
		cur.TokenHash, cur.Hostname, cur.IP, cur.Version = agent.TokenHash, agent.Hostname, agent.IP, agent.Version
		cur.Capabilities, cur.Metadata, cur.Status, cur.LastSeen = agent.Capabilities, agent.Metadata, agent.Status, agent.LastSeen
		out = cur
		return tx.Save(&cur).Error
	})
	return out, created, err
}

// ClaimTasks marks the agent online and claims every pending task that is unassigned or its own,
// highest priority first. Each claim is one conditional UPDATE, so two agents polling one unassigned
// task cannot both get it.
func (s *ProvisionerStore) ClaimTasks(ctx context.Context, actor ProvisionerActor, agentID string, now time.Time) ([]ProvisioningTask, error) {
	claimed := []ProvisioningTask{}
	err := s.tx(ctx, func(tx *gorm.DB) error {
		agent, err := s.lockAgent(tx, actor, agentID)
		if err != nil {
			return err
		}
		if agent == nil {
			return ErrAgentNotFound
		}
		if err := tx.Model(agent).Updates(map[string]interface{}{"last_seen": now, "status": "online"}).Error; err != nil {
			return err
		}
		var candidates []uint
		if err := tx.Model(&ProvisioningTask{}).Where("status = ? AND (agent_id = ? OR agent_id = ?)", "pending", "", agentID).
			Order("priority DESC, created_at, id").Pluck("id", &candidates).Error; err != nil {
			return err
		}
		var won []uint
		for _, id := range candidates {
			res := tx.Model(&ProvisioningTask{}).Where("id = ? AND status = ? AND (agent_id = ? OR agent_id = ?)", id, "pending", "", agentID).
				Updates(map[string]interface{}{"status": "assigned", "agent_id": agentID, "updated_at": now})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 1 {
				won = append(won, id)
			}
		}
		if len(won) == 0 {
			return nil
		}
		return tx.Where("id IN ?", won).Order("priority DESC, created_at, id").Find(&claimed).Error
	})
	return claimed, err
}

// UpdateTaskStatus sets a task's status. An agent may update only a task assigned to it: an unassigned
// or unknown task is nobody's, so an agent gets ErrForbidden for both and learns nothing about which
// ids exist. The admin key gets ErrTaskNotFound for an unknown task.
func (s *ProvisionerStore) UpdateTaskStatus(ctx context.Context, actor ProvisionerActor, taskID, status string, now time.Time) (ProvisioningTask, error) {
	var task ProvisioningTask
	err := s.tx(ctx, func(tx *gorm.DB) error {
		agentID := ""
		if actor.Privileged && status == "pending" {
			// A task the admin sets back to pending keeps its agent from a purge: lock that agent first, as Purge does.
			var owner string
			if err := tx.Model(&ProvisioningTask{}).Where("task_id = ?", taskID).Select("agent_id").Scan(&owner).Error; err != nil {
				return err
			}
			if owner != "" {
				if _, err := s.lockAgent(tx, actor, owner); err != nil {
					return err
				}
			}
		}
		if !actor.Privileged {
			agent, err := s.agentByToken(tx, actor)
			if err != nil {
				return err
			}
			agentID = agent.AgentID
		}
		// The task row is locked too, so the agent it belongs to cannot change between this read and the write.
		found := tx.Clauses(forUpdate()).Where("task_id = ?", taskID).First(&task)
		switch {
		case errors.Is(found.Error, gorm.ErrRecordNotFound) && actor.Privileged:
			return ErrTaskNotFound
		case errors.Is(found.Error, gorm.ErrRecordNotFound):
			return ErrForbidden
		case found.Error != nil:
			return found.Error
		case !actor.Privileged && task.AgentID != agentID:
			return ErrForbidden
		}
		if utf8.RuneCountInString(status) > MaxProvisionerValueLen {
			return ErrStatusTooLong
		}
		if err := tx.Model(&ProvisioningTask{}).Where("id = ?", task.ID).
			Updates(map[string]interface{}{"status": status, "updated_at": now}).Error; err != nil {
			return err
		}
		task.Status, task.UpdatedAt = status, now
		return nil
	})
	return task, err
}

// ReportDiscovered records the devices an agent found and completes the task it names, if that task is
// assigned to the reporting agent (the admin key may complete any). It returns how many devices were
// stored and why the others were not; a device that fails is rolled back alone.
func (s *ProvisionerStore) ReportDiscovered(ctx context.Context, actor ProvisionerActor, agentID, taskID string, devices []DiscoveredDevice, now time.Time) (int, []error, error) {
	persisted := 0
	var failures []error
	err := s.tx(ctx, func(tx *gorm.DB) error {
		// Identity first: a refused caller learns nothing from the validation answers below.
		agent, err := s.lockAgent(tx, actor, agentID)
		if err != nil {
			return err
		}
		switch {
		case agentID == "":
			return ErrAgentIDRequired
		case len(devices) == 0:
			return ErrNoDevices
		case agent == nil:
			return ErrAgentNotFound
		}
		if err := tx.Model(agent).Update("last_seen", now).Error; err != nil {
			return err
		}
		for i := range devices {
			devices[i].AgentID = agentID // a device belongs to the agent that reported it
			// A savepoint per device: on PostgreSQL a failed statement aborts the whole transaction.
			if err := tx.Transaction(func(inner *gorm.DB) error {
				_, upsertErr := upsertDiscoveredDevice(inner, &devices[i])
				return upsertErr
			}); err != nil {
				failures = append(failures, err)
				continue
			}
			persisted++
		}
		if taskID == "" {
			return nil
		}
		done := tx.Model(&ProvisioningTask{}).Where("task_id = ?", taskID)
		if !actor.Privileged {
			done = done.Where("agent_id = ?", agentID)
		}
		return done.Updates(map[string]interface{}{"status": "completed", "updated_at": now}).Error
	})
	return persisted, failures, err
}

// CreateTask stores a new task. A task for an agent locks that agent's row first, as Purge does, so a purge
// cannot delete the agent between its check for a pending task and this insert.
func (s *ProvisionerStore) CreateTask(ctx context.Context, task *ProvisioningTask) error {
	return s.tx(ctx, func(tx *gorm.DB) error {
		if task.AgentID != "" {
			if _, err := s.lockAgent(tx, ProvisionerActor{Privileged: true}, task.AgentID); err != nil {
				return err
			}
		}
		return tx.Create(task).Error
	})
}

// GetTask returns one task, or ErrTaskNotFound.
func (s *ProvisionerStore) GetTask(ctx context.Context, taskID string) (ProvisioningTask, error) {
	var task ProvisioningTask
	err := s.db.WithContext(ctx).Where("task_id = ?", taskID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task, ErrTaskNotFound
	}
	return task, err
}

// ListTasks returns every task, oldest first.
func (s *ProvisionerStore) ListTasks(ctx context.Context) ([]ProvisioningTask, error) {
	tasks := []ProvisioningTask{}
	err := s.db.WithContext(ctx).Order("created_at, id").Find(&tasks).Error
	return tasks, err
}

// CancelTask fails a task that is pending, assigned or in progress, with config["_error"] = "canceled".
// A task already finished is returned as it is.
func (s *ProvisionerStore) CancelTask(ctx context.Context, taskID string, now time.Time) (ProvisioningTask, error) {
	var task ProvisioningTask
	err := s.tx(ctx, func(tx *gorm.DB) error {
		err := tx.Clauses(forUpdate()).Where("task_id = ?", taskID).First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTaskNotFound
		}
		if err != nil {
			return err
		}
		switch task.Status {
		case "pending", "assigned", "in_progress":
		default:
			return nil
		}
		if task.Config == nil {
			task.Config = map[string]interface{}{}
		}
		task.Config["_error"] = "canceled"
		task.Status, task.UpdatedAt = "failed", now
		return tx.Model(&task).Select("Status", "Config", "UpdatedAt").Updates(&task).Error
	})
	return task, err
}

// removeAgent deletes the agent whose row tx holds locked, and fails its assigned and in-progress tasks
// with config["_error"] = reason: nobody will deliver them again, and the UI shows why. A pending task
// stays, so an agent that registers the id again still receives it.
func removeAgent(tx *gorm.DB, agent ProvisionerAgent, reason string, now time.Time) error {
	var stranded []ProvisioningTask
	if err := tx.Clauses(forUpdate()).Where("agent_id = ? AND status IN ?", agent.AgentID, []string{"assigned", "in_progress"}).
		Find(&stranded).Error; err != nil {
		return err
	}
	for i := range stranded {
		task := &stranded[i]
		if task.Config == nil {
			task.Config = map[string]interface{}{}
		}
		task.Config["_error"] = reason
		task.Status, task.UpdatedAt = "failed", now
		if err := tx.Model(task).Select("Status", "Config", "UpdatedAt").Updates(task).Error; err != nil {
			return err
		}
	}
	return tx.Delete(&ProvisionerAgent{}, agent.ID).Error
}

// DeleteAgent removes an agent, which frees its id and revokes its token (ErrAgentNotFound if there is
// none), and fails its assigned and in-progress tasks.
func (s *ProvisionerStore) DeleteAgent(ctx context.Context, agentID string, now time.Time) error {
	return s.tx(ctx, func(tx *gorm.DB) error {
		var agent ProvisionerAgent
		err := tx.Clauses(forUpdate()).Where("agent_id = ?", agentID).First(&agent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAgentNotFound
		}
		if err != nil {
			return err
		}
		return removeAgent(tx, agent, "the agent was deleted", now)
	})
}

// PurgeResult is what one Purge removed.
type PurgeResult struct {
	Agents, Tasks int64
}

// Purge removes the agents last seen before now-silentFor that have no pending task (their assigned and
// in-progress tasks are failed, as in DeleteAgent), and the completed and failed tasks last updated before
// now-keepFinished. Each agent goes in its own transaction, which locks the agent row first as every other
// operation does and checks the conditions again under that lock, so an agent that registered or polled
// meanwhile stays. It keeps going past a failure and returns the first error.
func (s *ProvisionerStore) Purge(ctx context.Context, now time.Time, silentFor, keepFinished time.Duration) (PurgeResult, error) {
	var res PurgeResult
	var first error
	note := func(err error) {
		if first == nil {
			first = err
		}
	}
	cutoff := now.Add(-silentFor)
	var silent []string
	if err := s.db.WithContext(ctx).Model(&ProvisionerAgent{}).Where("last_seen < ?", cutoff).Pluck("agent_id", &silent).Error; err != nil {
		note(err)
	}
	for _, id := range silent {
		err := s.tx(ctx, func(tx *gorm.DB) error {
			var agent ProvisionerAgent
			err := tx.Clauses(forUpdate()).Where("agent_id = ? AND last_seen < ?", id, cutoff).First(&agent).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			if s.afterLock != nil {
				s.afterLock()
			}
			// A locking read sees every pending task committed so far, on MySQL's repeatable read too. A writer of
			// a pending task for this agent locks the agent row first, so none can commit after this read.
			var pending []ProvisioningTask
			if err := tx.Clauses(forUpdate()).Where("agent_id = ? AND status = ?", id, "pending").Limit(1).Find(&pending).Error; err != nil || len(pending) > 0 {
				return err
			}
			if err := removeAgent(tx, agent, "the agent was removed after it stayed silent too long", now); err != nil {
				return err
			}
			res.Agents++
			return nil
		})
		if err != nil {
			note(err)
		}
	}
	done := s.db.WithContext(ctx).Where("status IN ? AND updated_at < ?", []string{"completed", "failed"}, now.Add(-keepFinished)).Delete(&ProvisioningTask{})
	res.Tasks = done.RowsAffected
	if done.Error != nil {
		note(done.Error)
	}
	return res, first
}

// ListAgents returns every agent, by hostname.
func (s *ProvisionerStore) ListAgents(ctx context.Context) ([]ProvisionerAgent, error) {
	agents := []ProvisionerAgent{}
	err := s.db.WithContext(ctx).Order("hostname, agent_id").Find(&agents).Error
	return agents, err
}

// GetAgent returns one agent, or ErrAgentNotFound.
func (s *ProvisionerStore) GetAgent(ctx context.Context, agentID string) (ProvisionerAgent, error) {
	var agent ProvisionerAgent
	err := s.db.WithContext(ctx).Where("agent_id = ?", agentID).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent, ErrAgentNotFound
	}
	return agent, err
}

// ProvisionerCounts are the figures of the health check.
type ProvisionerCounts struct {
	Agents, ActiveAgents, Tasks, PendingTasks int64
}

// Counts counts agents (active: seen within offlineAfter of now) and tasks (pending: pending or assigned).
func (s *ProvisionerStore) Counts(ctx context.Context, now time.Time, offlineAfter time.Duration) (ProvisionerCounts, error) {
	db := s.db.WithContext(ctx)
	var c ProvisionerCounts
	for _, q := range []struct {
		dst   *int64
		model interface{}
		where string
		args  []interface{}
	}{
		{&c.Agents, &ProvisionerAgent{}, "1 = 1", nil},
		{&c.ActiveAgents, &ProvisionerAgent{}, "last_seen >= ?", []interface{}{now.Add(-offlineAfter)}},
		{&c.Tasks, &ProvisioningTask{}, "1 = 1", nil},
		{&c.PendingTasks, &ProvisioningTask{}, "status IN ?", []interface{}{[]string{"pending", "assigned"}}},
	} {
		if err := db.Model(q.model).Where(q.where, q.args...).Count(q.dst).Error; err != nil {
			return c, err
		}
	}
	return c, nil
}

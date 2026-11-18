package configuration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"time"
)

// ErrApplyInProgress is returned when another manager replica holds the apply lock of the device
// for longer than applyLockWait. Nothing was read from or written to the device.
var ErrApplyInProgress = errors.New("another apply to this device is running; retry in a moment")

// ErrApplyLockLost is returned when the lock that keeps other replicas away from the device was lost
// while this apply ran. The device writes cannot be undone, so the device may hold a mix of two
// applies; the configuration stays pending.
var ErrApplyLockLost = errors.New("the lock that keeps other replicas from applying to this device was lost during the apply, " +
	"so the device may hold a mix of two applies; the configuration stays pending, apply it again")

// How long a second apply waits for the lock of another replica, and how often it asks again.
// Variables so a test can shorten them.
var (
	applyLockWait = 30 * time.Second
	applyLockPoll = 250 * time.Millisecond
)

// applyLock is a lock held in the database against the other manager replicas.
type applyLock interface {
	// StillHeld fails when the session that holds the lock is gone.
	StillHeld(ctx context.Context) error
	Unlock()
}

// applyLocker is the optional part of a ConfigRepository that takes the cross-replica lock. A
// repository without it serializes applies inside this process only.
type applyLocker interface {
	// LockDeviceApply returns a nil lock when the database needs none (SQLite runs in one process).
	LockDeviceApply(ctx context.Context, deviceID uint) (applyLock, error)
}

var _ applyLocker = (*GormConfigRepository)(nil)

// heldApply is what ApplyDesiredConfig holds: the mutex of this process and, where the database
// needs one, the lock against the other replicas.
type heldApply struct {
	local func()
	db    applyLock
}

// StillHeld reports whether the lock against the other replicas is still there. With no database
// lock, nothing can have been lost.
func (h *heldApply) StillHeld(ctx context.Context) error {
	if h.db == nil {
		return nil
	}
	return h.db.StillHeld(ctx)
}

// Unlock releases the database lock first, then the mutex.
func (h *heldApply) Unlock() {
	if h.db != nil {
		h.db.Unlock()
	}
	h.local()
}

// stillHeld asks the lock on its own short context, so a cancelled request still gets a true answer.
func stillHeld(ctx context.Context, lock *heldApply) error {
	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return lock.StillHeld(checkCtx)
}

// markPendingAfterLockLoss leaves the device pending and owing a reboot after a lock was lost during
// an apply: another replica may have set config_applied or cleared the owed reboot while this
// apply's older writes were still on their way to the device. A false flag is the true state of a
// device that may hold them, and the next apply repairs it. When the database is unreachable (the
// usual cause of a lost session) these writes fail too, and the log line is the only signal. Each
// write is tried twice, because the first query on a connection the server ended fails.
func (s *ConfigurationService) markPendingAfterLockLoss(deviceID uint) {
	for name, write := range map[string]func() error{
		"config_applied": func() error { return s.repo.SetDeviceConfigApplied(deviceID, false) },
		"pending_reboot": func() error { return s.repo.SetDevicePendingReboot(deviceID, true) },
	} {
		var err error
		for attempt := 0; attempt < 2; attempt++ {
			if err = write(); err == nil {
				break
			}
		}
		if err != nil {
			s.logger.WithFields(map[string]any{"error": err.Error(), "device_id": deviceID, "flag": name}).
				Error("The apply lock was lost and the device's flag could not be set to pending; verify the device or apply again")
		}
	}
}

// lockApplyAcrossReplicas serializes applies to one device in this process (the mutex, waited for
// without a bound) and then against the other manager replicas that share the database.
func (s *ConfigurationService) lockApplyAcrossReplicas(ctx context.Context, deviceID uint) (*heldApply, error) {
	held := &heldApply{local: s.lockApply(deviceID)}
	locker, ok := s.repo.(applyLocker)
	if !ok {
		s.noApplyLockOnce.Do(func() {
			s.logger.Info("The configuration repository offers no cross-replica apply lock; " +
				"applies to one device are serialized inside this process only")
		})
		return held, nil
	}
	lock, err := locker.LockDeviceApply(ctx, deviceID)
	if err != nil {
		held.local()
		return nil, err
	}
	held.db = lock
	return held, nil
}

// applyLockClass is the first key of the PostgreSQL advisory lock; the device ID is the second.
// It keeps this lock apart from any other advisory lock on the same database.
const applyLockClass int32 = 0x53484d41 // "SHMA"

// LockDeviceApply is applyLocker for GORM. PostgreSQL and MySQL hold a session lock on one pinned
// connection for the whole apply; the lock goes with the connection, so a replica that dies frees
// it. SQLite has one process and returns no lock. The whole acquisition (a pool slot, the
// connection, the polling) shares one deadline of applyLockWait; past it the apply is refused.
func (r *GormConfigRepository) LockDeviceApply(parent context.Context, deviceID uint) (applyLock, error) {
	dialect := r.db.Name()
	if dialect != "postgres" && dialect != "mysql" {
		return nil, nil
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		// A handle inside a transaction has no pool to pin a connection from; apply never runs there.
		return nil, nil
	}
	if dialect == "postgres" && uint64(deviceID) > math.MaxInt32 {
		return nil, fmt.Errorf("device id %d does not fit the apply lock key", deviceID)
	}

	ctx, cancel := context.WithTimeout(parent, applyLockWait)
	defer cancel()
	refused := func(err error) error {
		if parent.Err() == nil && ctx.Err() != nil {
			return ErrApplyInProgress
		}
		return fmt.Errorf("could not take the apply lock: %w", err)
	}

	releaseSlot, err := r.takeLockSlot(ctx, sqlDB)
	if err != nil {
		return nil, refused(err)
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		releaseSlot()
		return nil, refused(err)
	}
	lock := &sessionLock{conn: conn, releaseSlot: releaseSlot}
	fail := func(err error) (applyLock, error) {
		_ = conn.Close()
		releaseSlot()
		return nil, refused(err)
	}

	switch dialect {
	case "postgres":
		// The two-int4 form: pg_try_advisory_lock(int4, int4), so the device id must fit int4 (checked above).
		lock.try = "SELECT pg_try_advisory_lock($1::int4, $2::int4)"
		lock.release = "SELECT pg_advisory_unlock($1::int4, $2::int4)"
		lock.args = []any{applyLockClass, int32(deviceID)}
	default:
		// MySQL lock names belong to the server, not to a database, so the name carries the database.
		var database sql.NullString
		if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
			return fail(err)
		}
		name := fmt.Sprintf("sm-apply:%s:%d", database.String, deviceID)
		if len(name) > 64 { // the limit of a MySQL lock name
			sum := sha256.Sum256([]byte(database.String))
			name = fmt.Sprintf("sm-apply:%x:%d", sum[:8], deviceID)
		}
		lock.try = "SELECT GET_LOCK(?, 0)"
		lock.release = "SELECT RELEASE_LOCK(?)"
		lock.args = []any{name}
	}

	for {
		var got sql.NullBool
		if err := conn.QueryRowContext(ctx, lock.try, lock.args...).Scan(&got); err != nil {
			return fail(err)
		}
		if got.Valid && got.Bool {
			return lock, nil
		}
		select {
		case <-ctx.Done():
			return fail(ctx.Err())
		case <-time.After(applyLockPoll):
		}
	}
}

// takeLockSlot admits one more pinned lock connection. Each running apply pins one connection and
// still needs another for its own queries, so at most half of the pool may be pinned: with every
// connection pinned, every apply would wait for a free one for ever. An unbounded pool needs no
// limit; a pool of one cannot hold a lock and run the apply's queries at all.
func (r *GormConfigRepository) takeLockSlot(ctx context.Context, sqlDB *sql.DB) (func(), error) {
	r.lockSlotsOnce.Do(func() {
		switch maxOpen := sqlDB.Stats().MaxOpenConnections; {
		case maxOpen == 1:
			r.lockSlotsErr = errors.New("the database pool allows one connection, which the cross-replica apply lock " +
				"cannot share with the apply's own queries; set max_open_conns to at least 2")
		case maxOpen > 1:
			r.lockSlots = make(chan struct{}, maxOpen/2)
		}
	})
	if r.lockSlotsErr != nil {
		return nil, r.lockSlotsErr
	}
	if r.lockSlots == nil {
		return func() {}, nil
	}
	select {
	case r.lockSlots <- struct{}{}:
		return func() { <-r.lockSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sessionLock is a session-level lock held on one pinned connection. The connection is not used for
// anything else, and the unlock runs on the same connection: a pooled one would not hold the lock.
type sessionLock struct {
	conn        *sql.Conn
	try         string
	release     string
	args        []any
	releaseSlot func() // returns the pool slot this lock took
}

// StillHeld runs one statement on the pinned connection. A session that the server ended, or a
// connection that broke, fails here; database/sql never silently reconnects a pinned connection.
func (l *sessionLock) StillHeld(ctx context.Context) error {
	var one int
	return l.conn.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

// Unlock releases the lock on its own connection and returns the connection to the pool. When the
// release fails, the connection is discarded, so a lock never goes back into the pool with it. It
// uses its own context: a cancelled request still unlocks.
func (l *sessionLock) Unlock() {
	defer l.releaseSlot()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var released sql.NullBool
	err := l.conn.QueryRowContext(ctx, l.release, l.args...).Scan(&released)
	if err == nil && released.Valid && released.Bool {
		_ = l.conn.Close()
		return
	}
	// A broken connection already freed the lock with its session; a live one that would not
	// release it must not return to the pool holding it.
	_ = l.conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = l.conn.Close()
}

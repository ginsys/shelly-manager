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
// it. SQLite has one process and returns no lock.
func (r *GormConfigRepository) LockDeviceApply(ctx context.Context, deviceID uint) (applyLock, error) {
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

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not take a connection for the apply lock: %w", err)
	}
	lock := &sessionLock{conn: conn}
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
			_ = conn.Close()
			return nil, fmt.Errorf("could not read the database name for the apply lock: %w", err)
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

	deadline := time.Now().Add(applyLockWait)
	for {
		var got sql.NullBool
		if err := conn.QueryRowContext(ctx, lock.try, lock.args...).Scan(&got); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("could not ask for the apply lock: %w", err)
		}
		if got.Valid && got.Bool {
			return lock, nil
		}
		if time.Now().After(deadline) {
			_ = conn.Close()
			return nil, ErrApplyInProgress
		}
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return nil, ctx.Err()
		case <-time.After(applyLockPoll):
		}
	}
}

// sessionLock is a session-level lock held on one pinned connection. The connection is not used for
// anything else, and the unlock runs on the same connection: a pooled one would not hold the lock.
type sessionLock struct {
	conn    *sql.Conn
	try     string
	release string
	args    []any
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

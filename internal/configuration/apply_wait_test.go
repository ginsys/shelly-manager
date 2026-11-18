package configuration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A second apply to a device waits a bounded time, whether the apply it waits for runs in this
// process or on another replica, and is then refused with ErrApplyInProgress (409). The waiting
// itself holds no state: a waiter that gives up, or whose request ends, leaves the lock to the next
// caller.

// outcomeWithin returns the outcome of an apply that is expected to end within d. When it does not,
// the parked apply is released first, so the test fails instead of hanging the run.
func outcomeWithin(t *testing.T, ch <-chan applyOutcome, d time.Duration, release func(), what string) applyOutcome {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(d):
		release()
		t.Fatalf("%s did not end within %s: it is queued behind the running apply", what, d)
		return applyOutcome{}
	}
}

// applyAsyncWith is applyAsync with the caller's context.
func applyAsyncWith(ctx context.Context, svc *ConfigurationService, dev ShellyClient) <-chan applyOutcome {
	out := make(chan applyOutcome, 1)
	go func() {
		result, err := svc.ApplyDesiredConfig(ctx, 1, dev, "SHPLG-S")
		out <- applyOutcome{result, err}
	}()
	return out
}

// The case of ginsys/shelly-manager#573: with no other replica involved, a second apply used to wait
// on the in-process lock for as long as the first one ran.
func TestApplyLock_ASecondApplyOnTheSameReplicaIsRefusedAfterTheWait(t *testing.T) {
	shortLockWait(t, 300*time.Millisecond)
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	devA := newParkedDevice()
	devB := newFakeGen1Device()

	aDone := applyAsync(svc, devA)
	waitClosed(t, devA.parked, "apply A to park inside its write")
	appliedBefore, pendingBefore := repo.devices[1].ConfigApplied, repo.devices[1].PendingReboot

	start := time.Now()
	b := outcomeWithin(t, applyAsync(svc, devB), 3*time.Second, func() { close(devA.release) }, "apply B")
	elapsed := time.Since(start)

	require.ErrorIs(t, b.err, ErrApplyInProgress)
	assert.GreaterOrEqual(t, elapsed, 250*time.Millisecond, "B waited for the lock before it was refused")
	assert.Zero(t, devB.setCalls, "the refused apply wrote nothing to the device")
	assert.Zero(t, devB.reboots)
	assert.Equal(t, appliedBefore, repo.devices[1].ConfigApplied, "the refusal changed no flag")
	assert.Equal(t, pendingBefore, repo.devices[1].PendingReboot)

	close(devA.release)
	require.NoError(t, waitOutcome(t, aDone, "apply A").err)
}

// waitingLockRepo offers a cross-replica lock that another replica holds for good: every apply waits
// until its context ends, as the GORM repository does. The caller's deadline is the only budget.
type waitingLockRepo struct {
	*mockRepository
}

func (r *waitingLockRepo) LockDeviceApply(ctx context.Context, _ uint) (applyLock, error) {
	<-ctx.Done()
	return nil, fmt.Errorf("could not take the apply lock: %w", ctx.Err())
}

// One wait covers both locks. A second apply that spends half of it on the in-process lock has the
// other half left for the database lock, so the refusal comes at the wait and not at twice the wait.
func TestApplyLock_TheInProcessWaitAndTheDatabaseWaitShareOneDeadline(t *testing.T) {
	shortLockWait(t, time.Second)
	base, _ := newServiceWithDesiredName(t, "tv-office")
	repo := &waitingLockRepo{mockRepository: base.repo.(*mockRepository)}
	svc := NewConfigurationService(repo, nil, nil)
	// The in-process lock is held for half the wait and then freed.
	unlock, err := svc.lockApply(context.Background(), 1)
	require.NoError(t, err)
	time.AfterFunc(500*time.Millisecond, unlock)

	start := time.Now()
	b := outcomeWithin(t, applyAsync(svc, newFakeGen1Device()), 5*time.Second, func() {}, "the apply")
	elapsed := time.Since(start)

	require.ErrorIs(t, b.err, ErrApplyInProgress)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
	assert.Less(t, elapsed, 1300*time.Millisecond,
		"the two waits add up (500 ms for the in-process lock, then a full wait for the database lock): %s", elapsed)
}

// A waiter that gives up takes nothing with it: the lock goes to the next caller as soon as the
// holder is done.
func TestApplyLock_AWaiterThatGivesUpLeavesNoLockBehind(t *testing.T) {
	shortLockWait(t, 200*time.Millisecond)
	svc, _ := newServiceWithDesiredName(t, "tv-office")
	devA := newParkedDevice()

	aDone := applyAsync(svc, devA)
	waitClosed(t, devA.parked, "apply A to park inside its write")
	b := outcomeWithin(t, applyAsync(svc, newFakeGen1Device()), 3*time.Second, func() { close(devA.release) }, "apply B")
	require.ErrorIs(t, b.err, ErrApplyInProgress)

	close(devA.release)
	require.NoError(t, waitOutcome(t, aDone, "apply A").err)

	start := time.Now()
	c := waitOutcome(t, applyAsync(svc, newFakeGen1Device()), "a third apply after the holder finished")
	require.NoError(t, c.err)
	assert.Less(t, time.Since(start), 2*time.Second, "the third apply took the lock without waiting for anything")
}

// A request that ends while it waits is not a refusal: it gets its own context error, at once, and
// the lock stays with the holder.
func TestApplyLock_ACancelledWaiterReturnsItsOwnError(t *testing.T) {
	shortLockWait(t, 10*time.Second)
	svc, _ := newServiceWithDesiredName(t, "tv-office")
	devA := newParkedDevice()

	aDone := applyAsync(svc, devA)
	waitClosed(t, devA.parked, "apply A to park inside its write")
	ctx, cancel := context.WithCancel(context.Background())
	bDone := applyAsyncWith(ctx, svc, newFakeGen1Device())
	time.AfterFunc(100*time.Millisecond, cancel)

	b := outcomeWithin(t, bDone, 3*time.Second, func() { close(devA.release) }, "the cancelled apply B")

	require.Error(t, b.err)
	assert.True(t, errors.Is(b.err, context.Canceled), "got %v", b.err)
	assert.False(t, errors.Is(b.err, ErrApplyInProgress), "a request that ended is not a refusal")

	close(devA.release)
	require.NoError(t, waitOutcome(t, aDone, "apply A").err)
	require.NoError(t, waitOutcome(t, applyAsync(svc, newFakeGen1Device()), "an apply after the holder finished").err)
}

// The unlock function may be called twice (a deferred call after an explicit one): the second call
// must not release a lock a later holder took, nor block.
func TestApplyLock_TheUnlockIsSafeToCallTwice(t *testing.T) {
	svc, _ := newServiceWithDesiredName(t, "tv-office")

	first, err := svc.lockApply(context.Background(), 1)
	require.NoError(t, err)
	first()
	second, err := svc.lockApply(context.Background(), 1)
	require.NoError(t, err)
	first() // a second call of the first holder's unlock

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = svc.lockApply(ctx, 1)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "the second holder still holds the lock")
	second()
	third, err := svc.lockApply(context.Background(), 1)
	require.NoError(t, err)
	third()
}

// A lock that is free is taken even when the context has already ended, so a request that raced its
// own deadline never loses a lock nobody holds.
func TestApplyLock_AFreeLockIsTakenEvenWithAnEndedContext(t *testing.T) {
	svc, _ := newServiceWithDesiredName(t, "tv-office")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A select with both cases ready picks at random, so one call proves little: repeat it.
	for i := 0; i < 200; i++ {
		unlock, err := svc.lockApply(ctx, 1)
		require.NoError(t, err, "call %d", i)
		unlock()
	}
}

// A pool of one connection takes no database lock, so the in-process lock is the only one, and a
// second apply is refused after the wait there too (it used to queue, and then end in 408).
func TestApplyLockProviders_APoolOfOneSecondApplyIsRefusedAfterTheWait(t *testing.T) {
	for _, server := range lockServers(t) {
		t.Run(server.name, func(t *testing.T) {
			shortLockWait(t, 300*time.Millisecond)
			f := newLockFixture(t, server)
			svc := NewConfigurationService(NewGormConfigRepository(server.openSized(t, 1), nil), nil, nil)
			devA := newParkedDevice()
			devB := newFakeGen1Device()

			aDone := applyAsync(svc, devA)
			waitClosed(t, devA.parked, "apply A to park inside its write")

			b := outcomeWithin(t, applyAsync(svc, devB), 3*time.Second, func() { close(devA.release) }, "apply B")

			require.ErrorIs(t, b.err, ErrApplyInProgress)
			assert.Zero(t, devB.setCalls, "the refused apply wrote nothing to the device")

			close(devA.release)
			require.NoError(t, waitOutcome(t, aDone, "apply A").err)
			assert.True(t, f.stored(t).ConfigApplied)
		})
	}
}

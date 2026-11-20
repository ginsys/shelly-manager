package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// ErrSchedulingNotImplemented is the message of the 501 the schedule routes return until the write
// path lands (ginsys/shelly-manager#279, slice 2).
var ErrSchedulingNotImplemented = errors.New("drift schedule execution is not implemented in this release")

const (
	driftPollInterval = 30 * time.Second // how often the scheduler asks which schedules are due
	driftRunTimeout   = 10 * time.Minute // a run left running longer than this is interrupted
	driftRunsKept     = 100              // run rows kept per schedule
)

// driftCronParser reads exactly five fields: no seconds and no descriptors such as @hourly.
var driftCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// NextRun is the first instant after the given one that the spec names, in the server's time zone,
// returned in UTC. A spec that never fires (0 0 30 2 *) is an error.
func NextRun(spec string, after time.Time) (time.Time, error) {
	// The parser reads a zone prefix whatever its options say; the zone is the server's.
	if strings.HasPrefix(spec, "TZ=") || strings.HasPrefix(spec, "CRON_TZ=") {
		return time.Time{}, errors.New("a time zone prefix is not allowed: the schedule runs in the server's time zone")
	}
	schedule, err := driftCronParser.Parse(spec)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron spec (five fields: minute hour day-of-month month day-of-week): %w", err)
	}
	next := schedule.Next(after.In(time.Local))
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron spec %q never fires", spec)
	}
	return next.UTC(), nil
}

// ValidateCronSpec accepts exactly five-field standard cron that fires at some time.
func ValidateCronSpec(spec string) error {
	_, err := NextRun(spec, time.Now())
	return err
}

// Scheduler runs the drift detection schedules stored in the database. The rows are the only copy
// of the schedule: every poll reads which enabled schedules are due, so a replica that starts or
// stops loses nothing, one that crashes mid-run leaves a run row the next tick marks interrupted, and
// several replicas on one database share the work through claim.
type Scheduler struct {
	db     *gorm.DB
	svc    *Service
	logger *logging.Logger

	// ticks replaces the poll timer (tests); nil means a timer of driftPollInterval.
	ticks <-chan time.Time
	// clientFor builds the client a run reads a device through. It must only read: the default is
	// the service's own builder, never the one that saves device logins.
	clientFor func(uint) (shelly.Client, error)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewScheduler creates a scheduler over the schedule rows.
func NewScheduler(db *gorm.DB, svc *Service, logger *logging.Logger) *Scheduler {
	return &Scheduler{db: db, svc: svc, logger: logger, clientFor: svc.createClientForDevice}
}

// Start begins polling. It fails when the schedule table cannot be read, and when already started.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return errors.New("the drift scheduler is already running")
	}
	var enabled int64
	if err := s.db.Model(&DriftDetectionSchedule{}).Where("enabled = ?", true).Count(&enabled).Error; err != nil {
		return fmt.Errorf("read drift schedules: %w", err)
	}
	s.logger.WithFields(map[string]any{"enabled_schedules": enabled, "component": "configuration"}).
		Info("Starting drift detection scheduler")

	runCtx, cancel := context.WithCancel(ctx)
	ticks := s.ticks
	stopTimer := func() {}
	if ticks == nil {
		timer := time.NewTicker(driftPollInterval)
		ticks, stopTimer = timer.C, timer.Stop
	}
	done := make(chan struct{})
	s.cancel, s.done = cancel, done
	go func() {
		defer close(done)
		defer stopTimer()
		for {
			select {
			case <-runCtx.Done():
				return
			case now := <-ticks:
				s.tick(runCtx, now)
			}
		}
	}()
	return nil
}

// Stop ends polling and the run in flight: the devices it has not read yet are reported skipped. It
// waits for the loop, at most 30 seconds, and is a no-op when not running.
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return errors.New("the drift scheduler did not stop within 30 seconds")
	}
}

// IsRunning reports whether Start has been called and Stop has not.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel != nil
}

// tick handles every schedule that is due at now: it first ends runs another process left running.
func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	now = now.UTC()
	s.interruptStaleRuns(now)

	var due []DriftDetectionSchedule
	if err := s.db.Where("enabled = ? AND (next_run IS NULL OR next_run <= ?)", true, now).
		Order("id").Find(&due).Error; err != nil {
		s.logError("read due drift schedules", 0, err)
		return
	}
	for _, sc := range due {
		if ctx.Err() != nil {
			return
		}
		s.handle(ctx, sc, now)
	}
}

// interruptStaleRuns marks a run that has been running longer than the run timeout interrupted: its
// process ended without finishing it, and no other tick would ever close the row.
func (s *Scheduler) interruptStaleRuns(now time.Time) {
	err := s.db.Model(&DriftDetectionRun{}).
		Where("status = ? AND started_at < ?", DriftRunRunning, now.Add(-driftRunTimeout)).
		Updates(map[string]any{
			"status":       DriftRunInterrupted,
			"error":        "the process that started this run ended before the run finished",
			"completed_at": now,
		}).Error
	if err != nil {
		s.logError("interrupt stale drift runs", 0, err)
	}
}

func (s *Scheduler) handle(ctx context.Context, sc DriftDetectionSchedule, now time.Time) {
	next, err := NextRun(sc.CronSpec, now)
	if err != nil {
		s.failSchedule(sc, now, err)
		return
	}
	if sc.NextRun == nil {
		// Never armed (a row from before the scheduler ran): arm it, run it at its next instant.
		armErr := s.db.Model(&DriftDetectionSchedule{}).Where("id = ? AND next_run IS NULL", sc.ID).
			Update("next_run", next).Error
		if armErr != nil {
			s.logError("arm drift schedule", sc.ID, armErr)
		}
		return
	}
	run, err := s.claimAndRecord(sc, next, now)
	if err != nil {
		s.logError("claim drift schedule", sc.ID, err)
		return
	}
	if run != nil {
		s.runClaimed(ctx, sc, run, now)
	}
}

// claim takes one due tick of a schedule that was read earlier. It compares and sets the integer
// run_count, which counts claimed ticks (skipped ones too), so of several replicas that read the same
// row exactly one wins: a timestamp would be read back in another precision than it was written.
func (s *Scheduler) claim(db *gorm.DB, sc DriftDetectionSchedule, next time.Time) (bool, error) {
	res := db.Model(&DriftDetectionSchedule{}).
		Where("id = ? AND enabled = ? AND run_count = ?", sc.ID, true, sc.RunCount).
		Updates(map[string]any{"next_run": next, "run_count": gorm.Expr("run_count + 1")})
	return res.RowsAffected == 1, res.Error
}

// claimAndRecord claims the tick and writes its run row in one transaction, so a process that ends
// after the claim leaves a row the next tick marks interrupted, never a tick that nobody ran. The
// row is skipped when a run of the schedule is still running, otherwise running. It returns nil when
// another replica took the tick.
func (s *Scheduler) claimAndRecord(sc DriftDetectionSchedule, next, now time.Time) (*DriftDetectionRun, error) {
	var run *DriftDetectionRun
	err := s.db.Transaction(func(tx *gorm.DB) error {
		won, err := s.claim(tx, sc, next)
		if err != nil || !won {
			return err
		}
		var running int64
		if err := tx.Model(&DriftDetectionRun{}).
			Where("schedule_id = ? AND status = ?", sc.ID, DriftRunRunning).Count(&running).Error; err != nil {
			return err
		}
		row := DriftDetectionRun{ScheduleID: sc.ID, StartedAt: now, Status: DriftRunRunning}
		if running > 0 {
			row.Status, row.CompletedAt = DriftRunSkipped, &now
			row.Error = "the previous run of this schedule is still running"
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if row.Status == DriftRunRunning {
			if err := tx.Model(&DriftDetectionSchedule{}).Where("id = ?", sc.ID).Update("last_run", now).Error; err != nil {
				return err
			}
		}
		run = &row
		return nil
	})
	return run, err
}

// failSchedule records one failed run for a schedule that cannot run and disables it, so it is
// reported once and not on every tick. The disable is the arbiter: of several replicas that read the
// same row, only the one that turns enabled off records the failure.
func (s *Scheduler) failSchedule(sc DriftDetectionSchedule, now time.Time, cause error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&DriftDetectionSchedule{}).Where("id = ? AND enabled = ?", sc.ID, true).Update("enabled", false)
		if res.Error != nil || res.RowsAffected != 1 {
			return res.Error
		}
		return tx.Create(&DriftDetectionRun{
			ScheduleID: sc.ID, Status: DriftRunFailed, StartedAt: now, CompletedAt: &now, Error: cause.Error(),
		}).Error
	})
	if err != nil {
		s.logError("record a drift schedule that cannot run", sc.ID, err)
		return
	}
	s.logError("drift schedule cannot run", sc.ID, cause)
}

// runClaimed executes a claimed tick whose run row is open: a read-only drift check of the
// schedule's devices. A skipped row has nothing to run.
func (s *Scheduler) runClaimed(ctx context.Context, sc DriftDetectionSchedule, run *DriftDetectionRun, now time.Time) {
	defer s.pruneRuns(sc.ID)
	if run.Status != DriftRunRunning {
		return
	}

	started := time.Now()
	result, err := s.execute(ctx, sc)
	completed := now.Add(time.Since(started))
	run.CompletedAt = &completed
	duration := completed.Sub(now)
	run.Duration = &duration
	if err != nil {
		run.Status, run.Error = DriftRunFailed, err.Error()
	} else {
		run.Status = DriftRunCompleted
		run.Results, err = json.Marshal(result)
		if err != nil {
			run.Status, run.Error = DriftRunFailed, fmt.Sprintf("encode the run result: %v", err)
		}
	}
	if err := s.db.Save(run).Error; err != nil {
		s.logError("save drift run", sc.ID, err)
	}
}

// execute reads the drift of the schedule's devices. A panic in the read path fails the run and not
// the process.
func (s *Scheduler) execute(ctx context.Context, sc DriftDetectionSchedule) (result *BulkDriftResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, fmt.Errorf("drift check panicked: %v", r)
		}
	}()
	ids, err := s.resolveDevices(sc)
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, driftRunTimeout)
	defer cancel()
	return s.svc.BulkDetectDrift(runCtx, ids, s.clientFor, false)
}

// resolveDevices is the stored list when the schedule has one, else the devices its filter selects,
// else every device. A stored list is never widened: devices deleted since drop out of the check and
// show in it as not found.
func (s *Scheduler) resolveDevices(sc DriftDetectionSchedule) ([]uint, error) {
	var listed []uint
	if err := s.db.Model(&DriftScheduleDevice{}).Where("schedule_id = ?", sc.ID).
		Order("device_id").Pluck("device_id", &listed).Error; err != nil {
		return nil, fmt.Errorf("read the schedule's devices: %w", err)
	}
	if len(listed) > 0 {
		return listed, nil
	}

	query := s.db.Table("devices")
	if raw := bytes.TrimSpace(sc.DeviceFilter); len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var filter DriftDeviceFilter
		if err := decoder.Decode(&filter); err != nil {
			return nil, fmt.Errorf("device filter: %w", err)
		}
		if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
			return nil, errors.New("device filter: unexpected text after the filter")
		}
		if filter.Type != "" {
			query = query.Where("type = ?", filter.Type)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
	}
	var ids []uint
	if err := query.Order("id").Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("read the devices of the schedule: %w", err)
	}
	return ids, nil
}

// pruneRuns keeps the newest driftRunsKept runs of a schedule: each row holds a result document.
func (s *Scheduler) pruneRuns(scheduleID uint) {
	var keep []uint
	if err := s.db.Model(&DriftDetectionRun{}).Where("schedule_id = ?", scheduleID).
		Order("started_at DESC, id DESC").Limit(driftRunsKept).Pluck("id", &keep).Error; err != nil {
		s.logError("read drift runs to keep", scheduleID, err)
		return
	}
	if len(keep) < driftRunsKept {
		return
	}
	s.deleteRunsNotIn(scheduleID, keep)
}

// deleteRunsNotIn deletes the runs of a schedule that are not in keep and older than its newest row:
// a run another replica added after keep was read has a higher id and stays.
func (s *Scheduler) deleteRunsNotIn(scheduleID uint, keep []uint) {
	var newest uint
	for _, id := range keep {
		newest = max(newest, id)
	}
	if err := s.db.Where("schedule_id = ? AND id NOT IN ? AND id < ?", scheduleID, keep, newest).
		Delete(&DriftDetectionRun{}).Error; err != nil {
		s.logError("prune drift runs", scheduleID, err)
	}
}

func (s *Scheduler) logError(what string, scheduleID uint, err error) {
	s.logger.WithFields(map[string]any{
		"schedule_id": scheduleID, "error": err.Error(), "component": "configuration",
	}).Error("Drift scheduler: " + what)
}

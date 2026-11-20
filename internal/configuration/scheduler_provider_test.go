package configuration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Two manager replicas share one database, so the claim on a due schedule is tested against real
// PostgreSQL and MySQL servers, two handles on one database as two replicas have. The cases skip
// unless a server is named in the environment (lockServers), the variables the Database Provider
// Migration job sets; they run in that job and not on a developer host without a server.

func providerScheduler(t *testing.T, db *gorm.DB) *Scheduler {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	svc := NewService(db, logger)
	s := NewScheduler(db, svc, logger)
	s.clientFor = func(uint) (shelly.Client, error) { return nil, errors.New("no device is read in this case") }
	return s
}

func openSchedulerReplicas(t *testing.T, srv lockServer) (*Scheduler, *Scheduler, *gorm.DB) {
	t.Helper()
	a, b := srv.open(t), srv.open(t)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	NewService(a, logger) // the tables the rest of the package owns
	require.NoError(t, a.AutoMigrate(&Device{}, &DriftDetectionSchedule{}, &DriftDetectionRun{}, &DriftScheduleDevice{}))
	return providerScheduler(t, a), providerScheduler(t, b), a
}

func TestDriftSchedulerProviders_ClaimRace(t *testing.T) {
	for _, srv := range lockServers(t) {
		t.Run(srv.name, func(t *testing.T) {
			sa, sb, db := openSchedulerReplicas(t, srv)
			var ids []uint
			for i := 0; i < 5; i++ {
				sc := DriftDetectionSchedule{Name: "nightly", CronSpec: "*/5 * * * *", NextRun: ptr(at(0))}
				require.NoError(t, db.Create(&sc).Error)
				ids = append(ids, sc.ID)
			}

			start := make(chan struct{})
			var wg sync.WaitGroup
			for _, s := range []*Scheduler{sa, sb} {
				wg.Add(1)
				go func(s *Scheduler) {
					defer wg.Done()
					<-start
					s.tick(context.Background(), at(10*time.Second))
				}(s)
			}
			close(start)
			wg.Wait()

			for _, id := range ids {
				var runs int64
				require.NoError(t, db.Model(&DriftDetectionRun{}).Where("schedule_id = ?", id).Count(&runs).Error)
				assert.Equal(t, int64(1), runs, "schedule %d: two replicas ran one due tick", id)
				var sc DriftDetectionSchedule
				require.NoError(t, db.First(&sc, id).Error)
				assert.Equal(t, 1, sc.RunCount, "schedule %d", id)
				require.NotNil(t, sc.NextRun)
				assert.WithinDuration(t, at(5*time.Minute), *sc.NextRun, time.Second)
			}
		})
	}
}

// A run another replica started and has not finished makes this replica's due tick a skipped run,
// and the tick is still claimed.
func TestDriftSchedulerProviders_RunningOnAnotherReplicaIsSkipped(t *testing.T) {
	for _, srv := range lockServers(t) {
		t.Run(srv.name, func(t *testing.T) {
			sa, sb, db := openSchedulerReplicas(t, srv)
			sc := DriftDetectionSchedule{Name: "nightly", CronSpec: "*/5 * * * *", NextRun: ptr(at(0))}
			require.NoError(t, db.Create(&sc).Error)
			require.NoError(t, db.Create(&DriftDetectionRun{
				ScheduleID: sc.ID, Status: DriftRunRunning, StartedAt: at(-150 * time.Second),
			}).Error)

			sb.tick(context.Background(), at(10*time.Second))
			sa.tick(context.Background(), at(20*time.Second)) // the other replica finds it already claimed

			var runs []DriftDetectionRun
			require.NoError(t, db.Where("schedule_id = ?", sc.ID).Order("id").Find(&runs).Error)
			require.Len(t, runs, 2)
			assert.Equal(t, DriftRunRunning, runs[0].Status)
			assert.Equal(t, DriftRunSkipped, runs[1].Status)
		})
	}
}

// A run left running by a replica that died is marked interrupted by the next tick of any replica.
func TestDriftSchedulerProviders_StaleRunningRunIsInterrupted(t *testing.T) {
	for _, srv := range lockServers(t) {
		t.Run(srv.name, func(t *testing.T) {
			_, sb, db := openSchedulerReplicas(t, srv)
			sc := DriftDetectionSchedule{Name: "nightly", CronSpec: "*/5 * * * *", NextRun: ptr(at(time.Hour))}
			require.NoError(t, db.Create(&sc).Error)
			require.NoError(t, db.Create(&DriftDetectionRun{
				ScheduleID: sc.ID, Status: DriftRunRunning, StartedAt: at(-20 * time.Minute),
			}).Error)

			sb.tick(context.Background(), at(10*time.Second))

			var run DriftDetectionRun
			require.NoError(t, db.Where("schedule_id = ?", sc.ID).First(&run).Error)
			assert.Equal(t, DriftRunInterrupted, run.Status)
		})
	}
}

// The write path on the real servers: a device filter written by create and by update reads back
// (PostgreSQL returns a text column as a string, which a plain RawMessage cannot scan), and an
// update that changes nothing is not a missing schedule (MySQL reports 0 rows changed for it).
func TestDriftScheduleWriteProviders_FilterRoundTripsAndAnUnchangedUpdateIsFound(t *testing.T) {
	for _, srv := range lockServers(t) {
		t.Run(srv.name, func(t *testing.T) {
			s, _, db := openSchedulerReplicas(t, srv)
			svc := s.svc
			created, err := svc.CreateDriftSchedule(ScheduleInput{
				Name: "plugs", CronSpec: "*/5 * * * *", DeviceFilter: &DriftDeviceFilter{Type: "SHPLG-S"},
			})
			require.NoError(t, err)

			got, err := svc.GetDriftSchedule(created.ID)
			require.NoError(t, err)
			assert.JSONEq(t, `{"type":"SHPLG-S"}`, string(got.DeviceFilter))

			same := ScheduleInput{Name: "plugs", CronSpec: "*/5 * * * *", DeviceFilter: &DriftDeviceFilter{Type: "SHPLG-S"}}
			// The same body again, with a clock that stands still: no column changes, not even updated_at.
			db.NowFunc = func() time.Time { return at(0) }
			for i := 0; i < 2; i++ { // the first writes the fixed updated_at, the second changes nothing
				_, err = svc.UpdateDriftSchedule(created.ID, same)
				require.NoError(t, err)
			}
			changed, err := svc.UpdateDriftSchedule(created.ID, ScheduleInput{
				Name: "plugs", CronSpec: "*/5 * * * *", DeviceFilter: &DriftDeviceFilter{Status: "online"},
			})
			require.NoError(t, err)
			assert.JSONEq(t, `{"status":"online"}`, string(changed.DeviceFilter))

			list, err := svc.ListDriftSchedules()
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, []uint{}, list[0].DeviceIDs)

			var sc DriftDetectionSchedule
			require.NoError(t, db.First(&sc, created.ID).Error, "the scheduler reads the row the same way")
			require.NoError(t, svc.DeleteDriftSchedule(created.ID))
			assert.ErrorIs(t, svc.DeleteDriftSchedule(created.ID), gorm.ErrRecordNotFound)
		})
	}
}

// afterFirstScheduleRead runs fn once, right after the first read of the schedules table through db,
// then waits for it to end or for a second. A row lock held by that read makes fn wait for the commit,
// so the wait ends by the timeout; without the lock fn ends first.
func afterFirstScheduleRead(t *testing.T, db *gorm.DB, fn func()) {
	t.Helper()
	fired := false
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:after-schedule-read", func(tx *gorm.DB) {
		if fired || tx.Statement.Table != "drift_detection_schedules" {
			return
		}
		fired = true
		done := make(chan struct{})
		go func() { defer close(done); fn() }()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}))
}

// A delete that commits between an update's read and its write must leave no device rows. On MySQL a
// recount after the update still reads the old snapshot and finds the row, so only the row lock stops it.
func TestDriftScheduleWriteProviders_ADeleteBetweenReadAndUpdateLeavesNoDeviceRows(t *testing.T) {
	for _, srv := range lockServers(t) {
		t.Run(srv.name, func(t *testing.T) {
			sa, sb, db := openSchedulerReplicas(t, srv)
			require.NoError(t, db.Create(&Device{ID: 1, Name: "Device", Type: "SHSW-1", IP: "192.168.1.100", MAC: "AA:BB:CC:DD:EE:FF", LastSeen: at(0)}).Error)
			created, err := sa.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *"})
			require.NoError(t, err)
			deleted := make(chan struct{})
			afterFirstScheduleRead(t, db, func() {
				defer close(deleted)
				_ = sb.svc.DeleteDriftSchedule(created.ID)
			})

			_, err = sa.svc.UpdateDriftSchedule(created.ID, ScheduleInput{Name: "n", CronSpec: "0 3 * * *", DeviceIDs: []uint{1}})
			<-deleted

			assert.True(t, err == nil || errors.Is(err, gorm.ErrRecordNotFound), "update: %v", err)
			var rows, schedules int64
			require.NoError(t, db.Model(&DriftScheduleDevice{}).Where("schedule_id = ?", created.ID).Count(&rows).Error)
			require.NoError(t, db.Model(&DriftDetectionSchedule{}).Where("id = ?", created.ID).Count(&schedules).Error)
			assert.Zero(t, rows, "device rows of a deleted schedule")
			assert.Zero(t, schedules)
		})
	}
}

// Two updates that read the same spec must not leave a spec with the next run of the other: the second
// one waits for the first, reads its result and recomputes.
func TestDriftScheduleWriteProviders_ConcurrentUpdatesKeepTheSpecAndItsNextRunTogether(t *testing.T) {
	for _, srv := range lockServers(t) {
		t.Run(srv.name, func(t *testing.T) {
			sa, sb, db := openSchedulerReplicas(t, srv)
			created, err := sa.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "7 * * * *"})
			require.NoError(t, err)
			other := make(chan struct{})
			afterFirstScheduleRead(t, db, func() {
				defer close(other)
				_, uerr := sb.svc.UpdateDriftSchedule(created.ID, ScheduleInput{Name: "n", CronSpec: "0 3 * * *"})
				assert.NoError(t, uerr)
			})

			_, err = sa.svc.UpdateDriftSchedule(created.ID, ScheduleInput{Name: "renamed", CronSpec: "7 * * * *"})
			require.NoError(t, err)
			<-other

			var sc DriftDetectionSchedule
			require.NoError(t, db.First(&sc, created.ID).Error)
			require.NotNil(t, sc.NextRun)
			minute := map[string]int{"7 * * * *": 7, "0 3 * * *": 0}[sc.CronSpec]
			assert.Equal(t, minute, sc.NextRun.Minute(), "next run %s does not belong to spec %q", sc.NextRun, sc.CronSpec)
		})
	}
}

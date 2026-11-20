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

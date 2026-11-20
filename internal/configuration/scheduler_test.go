package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// The scheduler is driven by tick(ctx, now) with a fixed clock, so no case waits for a real timer
// and none touches a device: the devices are fakes.

var schedBase = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

type schedFixture struct {
	t       *testing.T
	db      *gorm.DB
	svc     *Service
	s       *Scheduler
	clients map[uint]*mockShellyClient
	asked   []uint
}

// newSchedFixture has devices 1..3: 1 and 3 are "SHSW-1", 2 is "SHPLG-S"; 1 and 2 are online and 3
// is offline; every device has a desired configuration and answers as in sync.
func newSchedFixture(t *testing.T) *schedFixture {
	t.Helper()
	svc, db := setupTestService(t)
	// The real devices table has a status column; this package's own model does not.
	require.NoError(t, db.Exec("ALTER TABLE devices ADD COLUMN status TEXT").Error)
	repo := newMockRepository()
	for id := uint(1); id <= 3; id++ {
		typ := "SHSW-1"
		if id == 2 {
			typ = "SHPLG-S"
		}
		createTestDevice(t, db, id, "Device", typ)
		repo.devices[id] = &ServiceDevice{ID: id, MAC: driftTestMAC, DesiredConfig: desiredNamed(t, "tv-office")}
	}
	require.NoError(t, db.Exec("UPDATE devices SET status = 'online' WHERE id IN (1, 2)").Error)
	require.NoError(t, db.Exec("UPDATE devices SET status = 'offline' WHERE id = 3").Error)
	svc.ConfigurationSvc = NewConfigurationService(repo, nil, nil)

	fx := &schedFixture{t: t, db: db, svc: svc, clients: map[uint]*mockShellyClient{}}
	fx.s = fx.newScheduler()
	return fx
}

func (fx *schedFixture) newScheduler() *Scheduler {
	s := NewScheduler(fx.db, fx.svc, fx.svc.logger)
	s.clientFor = func(id uint) (shelly.Client, error) {
		fx.asked = append(fx.asked, id)
		c, ok := fx.clients[id]
		if !ok {
			c = driftClient("tv-office", driftTestMAC)
			fx.clients[id] = c
		}
		return c, nil
	}
	return s
}

// add stores an enabled schedule due at next (nil: never armed) with the given device list.
func (fx *schedFixture) add(spec string, next *time.Time, filter string, ids ...uint) uint {
	fx.t.Helper()
	sc := DriftDetectionSchedule{Name: "nightly", CronSpec: spec, NextRun: next}
	if filter != "" {
		sc.DeviceFilter = json.RawMessage(filter)
	}
	require.NoError(fx.t, fx.db.Create(&sc).Error)
	for _, id := range ids {
		require.NoError(fx.t, fx.db.Create(&DriftScheduleDevice{ScheduleID: sc.ID, DeviceID: id}).Error)
	}
	return sc.ID
}

func (fx *schedFixture) schedule(id uint) DriftDetectionSchedule {
	fx.t.Helper()
	var sc DriftDetectionSchedule
	require.NoError(fx.t, fx.db.First(&sc, id).Error)
	return sc
}

func (fx *schedFixture) runs(scheduleID uint) []DriftDetectionRun {
	fx.t.Helper()
	var runs []DriftDetectionRun
	require.NoError(fx.t, fx.db.Where("schedule_id = ?", scheduleID).Order("id").Find(&runs).Error)
	return runs
}

func resultIDs(t *testing.T, run DriftDetectionRun) []uint {
	t.Helper()
	var res BulkDriftResult
	require.NoError(t, json.Unmarshal(run.Results, &res))
	ids := []uint{}
	for _, r := range res.Results {
		ids = append(ids, r.DeviceID)
	}
	return ids
}

func at(d time.Duration) time.Time { return schedBase.Add(d) }

func ptr(t time.Time) *time.Time { return &t }

func TestValidateCronSpec(t *testing.T) {
	for _, spec := range []string{"* * * * *", "*/5 * * * *", "0 3 * * 1-5", "30 2 1 * *"} {
		assert.NoError(t, ValidateCronSpec(spec), spec)
	}
	for _, spec := range []string{
		"",
		"* * * * * *",                  // six fields: the old parser's shape
		"*/5 * * *",                    // four
		"@every 10s",                   // descriptors are not five-field cron
		"@hourly",                      //
		"TZ=Europe/Brussels * * * * *", // the zone is the server's
		"CRON_TZ=UTC 0 3 * * *",        //
		"0 0 30 2 *",                   // parses, and never fires
		"61 * * * *",
	} {
		assert.Error(t, ValidateCronSpec(spec), spec)
	}
}

func TestNextRun(t *testing.T) {
	next, err := NextRun("*/5 * * * *", at(0))
	require.NoError(t, err)
	assert.True(t, next.Equal(at(5*time.Minute)), "next is %s", next)

	next, err = NextRun("*/5 * * * *", at(4*time.Minute+59*time.Second))
	require.NoError(t, err)
	assert.True(t, next.Equal(at(5*time.Minute)), "next is %s", next)

	_, err = NextRun("0 0 30 2 *", at(0))
	assert.Error(t, err, "a spec that never fires has no next run")
}

func TestSchedulerTick_ArmsThenRunsWhenDue(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", nil, "", 1)

	fx.s.tick(context.Background(), at(-2*time.Minute)) // 09:58: arms, never runs
	sc := fx.schedule(id)
	require.NotNil(t, sc.NextRun)
	assert.True(t, sc.NextRun.Equal(at(0)), "armed for %s", sc.NextRun)
	assert.Empty(t, fx.runs(id))
	assert.Zero(t, sc.RunCount)

	fx.s.tick(context.Background(), at(-time.Second)) // 09:59:59: not due
	assert.Empty(t, fx.runs(id))

	fx.s.tick(context.Background(), at(10*time.Second)) // 10:00:10: due
	runs := fx.runs(id)
	require.Len(t, runs, 1)
	assert.Equal(t, DriftRunCompleted, runs[0].Status)
	assert.NotNil(t, runs[0].CompletedAt)
	assert.Equal(t, []uint{1}, resultIDs(t, runs[0]))
	sc = fx.schedule(id)
	assert.True(t, sc.NextRun.Equal(at(5*time.Minute)), "next run %s", sc.NextRun)
	require.NotNil(t, sc.LastRun)
	assert.True(t, sc.LastRun.Equal(at(10*time.Second)))
	assert.Equal(t, 1, sc.RunCount)

	fx.s.tick(context.Background(), at(20*time.Second)) // the same tick again: nothing due
	assert.Len(t, fx.runs(id), 1)
}

func TestSchedulerTick_ALongGapRunsOnceNotOncePerMissedTick(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(5*time.Minute)), "", 1)

	fx.s.tick(context.Background(), at(time.Hour+10*time.Second)) // 11:00:10, the process was down

	assert.Len(t, fx.runs(id), 1)
	assert.True(t, fx.schedule(id).NextRun.Equal(at(time.Hour+5*time.Minute)))
}

func TestSchedulerTick_SelectsExactlyTheConfiguredDevices(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		ids    []uint
		want   []uint
		status string
		errHas string
	}{
		{name: "neither list nor filter: every device", want: []uint{1, 2, 3}},
		{name: "a stored list", ids: []uint{1, 3}, want: []uint{1, 3}},
		{name: "a filter on type", filter: `{"type":"SHSW-1"}`, want: []uint{1, 3}},
		{name: "a filter on status", filter: `{"status":"online"}`, want: []uint{1, 2}},
		{name: "a filter on both is an AND", filter: `{"type":"SHSW-1","status":"online"}`, want: []uint{1}},
		{name: "a filter nothing matches runs no device", filter: `{"type":"NO-SUCH"}`, want: []uint{}},
		{name: "a list of deleted devices never widens to all", ids: []uint{90, 91}, want: []uint{}},
		{name: "an unknown filter key fails the run and reads no device", filter: `{"device_type":"SHSW-1"}`,
			want: []uint{}, status: DriftRunFailed, errHas: "device_type"},
		{name: "text after the filter fails the run, it is never read as an empty filter", filter: `{} {"type":"SHSW-1"}`,
			want: []uint{}, status: DriftRunFailed, errHas: "device filter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newSchedFixture(t)
			id := fx.add("*/5 * * * *", ptr(at(0)), tc.filter, tc.ids...)

			fx.s.tick(context.Background(), at(10*time.Second))

			runs := fx.runs(id)
			require.Len(t, runs, 1)
			wantStatus := tc.status
			if wantStatus == "" {
				wantStatus = DriftRunCompleted
			}
			assert.Equal(t, wantStatus, runs[0].Status, runs[0].Error)
			assert.Contains(t, runs[0].Error, tc.errHas)
			assert.Equal(t, append([]uint{}, tc.want...), append([]uint{}, fx.asked...), "devices contacted")
			if wantStatus == DriftRunCompleted && len(tc.ids) == 0 {
				assert.Equal(t, tc.want, resultIDs(t, runs[0]))
			}
			if tc.name == "a list of deleted devices never widens to all" {
				for _, id := range resultIDs(t, runs[0]) {
					assert.NotContains(t, []uint{1, 2, 3}, id)
				}
			}
		})
	}
}

// A run only reads: the fake device answers GetInfo and GetConfig and nothing else, and a device
// that rejects the read leaves its stored row as it was.
func TestSchedulerTick_RunOnlyReadsDevices(t *testing.T) {
	fx := newSchedFixture(t)
	rejecting := new(mockShellyClient)
	rejecting.On("GetInfo", mock.Anything).Return(&shelly.DeviceInfo{ID: "d", MAC: driftTestMAC, Generation: 1, Model: "SHSW-1"}, nil).Maybe()
	rejecting.On("GetConfig", mock.Anything).Return(nil, errors.New("401 Unauthorized")).Maybe()
	rejecting.On("GetGeneration").Return(1).Maybe()
	rejecting.On("GetIP").Return("192.168.1.100").Maybe()
	fx.clients[2] = rejecting
	var before []map[string]any
	require.NoError(t, fx.db.Table("devices").Order("id").Find(&before).Error)
	id := fx.add("*/5 * * * *", ptr(at(0)), "")

	fx.s.tick(context.Background(), at(10*time.Second))

	runs := fx.runs(id)
	require.Len(t, runs, 1)
	assert.Equal(t, DriftRunCompleted, runs[0].Status)
	var res BulkDriftResult
	require.NoError(t, json.Unmarshal(runs[0].Results, &res))
	assert.Equal(t, 1, res.Errors, "the rejecting device is one error of the run")
	read := map[string]bool{"GetInfo": true, "GetConfig": true, "GetGeneration": true, "GetIP": true}
	for did, c := range fx.clients {
		for _, call := range c.Calls {
			assert.True(t, read[call.Method], "device %d: the run called %s", did, call.Method)
		}
	}
	var after []map[string]any
	require.NoError(t, fx.db.Table("devices").Order("id").Find(&after).Error)
	assert.Equal(t, before, after, "a run changed a device row")
}

func TestSchedulerTick_SkipsWhileThePreviousRunIsRunning(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	require.NoError(t, fx.db.Create(&DriftDetectionRun{
		ScheduleID: id, Status: DriftRunRunning, StartedAt: at(-150 * time.Second), // 09:57:30
	}).Error)

	fx.s.tick(context.Background(), at(10*time.Second))

	runs := fx.runs(id)
	require.Len(t, runs, 2)
	assert.Equal(t, DriftRunRunning, runs[0].Status, "the running row is left as it was")
	assert.Equal(t, DriftRunSkipped, runs[1].Status)
	assert.Contains(t, runs[1].Error, "still running")
	assert.Empty(t, fx.asked, "no device is contacted by a skipped run")
	sc := fx.schedule(id)
	assert.True(t, sc.NextRun.Equal(at(5*time.Minute)), "a skipped tick still moves next_run")
	assert.Equal(t, 1, sc.RunCount, "run_count counts claimed ticks, skipped ones included")
}

func TestSchedulerTick_MarksStaleRunningRunsInterruptedAndRuns(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	require.NoError(t, fx.db.Create(&DriftDetectionRun{
		ScheduleID: id, Status: DriftRunRunning, StartedAt: at(-20 * time.Minute),
	}).Error)

	fx.s.tick(context.Background(), at(10*time.Second))

	runs := fx.runs(id)
	require.Len(t, runs, 2)
	assert.Equal(t, DriftRunInterrupted, runs[0].Status)
	assert.NotEmpty(t, runs[0].Error)
	assert.NotNil(t, runs[0].CompletedAt)
	assert.Equal(t, DriftRunCompleted, runs[1].Status)
}

// A schedule that is not due is swept too: a crashed replica's row must not stay running while the
// others tick.
func TestSchedulerTick_SweepsStaleRunsOfSchedulesThatAreNotDue(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(time.Hour)), "", 1)
	require.NoError(t, fx.db.Create(&DriftDetectionRun{
		ScheduleID: id, Status: DriftRunRunning, StartedAt: at(-20 * time.Minute),
	}).Error)

	fx.s.tick(context.Background(), at(10*time.Second))

	runs := fx.runs(id)
	require.Len(t, runs, 1)
	assert.Equal(t, DriftRunInterrupted, runs[0].Status)
}

func TestSchedulerTick_ASpecThatDoesNotParseFailsOnceAndDisablesTheSchedule(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("0 */5 * * * *", nil, "", 1) // six fields: a row the old parser accepted

	fx.s.tick(context.Background(), at(0))
	fx.s.tick(context.Background(), at(time.Hour))

	runs := fx.runs(id)
	require.Len(t, runs, 1)
	assert.Equal(t, DriftRunFailed, runs[0].Status)
	assert.Contains(t, runs[0].Error, "cron")
	assert.False(t, fx.schedule(id).Enabled)
	assert.Empty(t, fx.asked)
}

func TestSchedulerTick_IgnoresADisabledSchedule(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	require.NoError(t, fx.db.Model(&DriftDetectionSchedule{}).Where("id = ?", id).Update("enabled", false).Error)

	fx.s.tick(context.Background(), at(10*time.Second))

	assert.Empty(t, fx.runs(id))
	assert.Empty(t, fx.asked)
}

// Nothing lives in memory: a new scheduler on the same rows neither repeats a run nor loses the
// next one, and the history stays.
func TestSchedulerTick_ResumesFromTheRowsAfterARestart(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	fx.s.tick(context.Background(), at(10*time.Second))

	restarted := fx.newScheduler()
	restarted.tick(context.Background(), at(2*time.Minute))
	assert.Len(t, fx.runs(id), 1, "the restart repeated a run")

	restarted.tick(context.Background(), at(5*time.Minute+5*time.Second))
	runs := fx.runs(id)
	require.Len(t, runs, 2)
	assert.Equal(t, DriftRunCompleted, runs[1].Status)
	assert.Equal(t, 2, fx.schedule(id).RunCount)
}

func TestSchedulerTick_KeepsTheNewestHundredRuns(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	for i := 0; i < 104; i++ {
		started := at(-time.Duration(104-i) * time.Hour)
		require.NoError(t, fx.db.Create(&DriftDetectionRun{ScheduleID: id, Status: DriftRunCompleted, StartedAt: started}).Error)
	}

	fx.s.tick(context.Background(), at(10*time.Second))

	runs := fx.runs(id)
	require.Len(t, runs, 100)
	assert.Equal(t, uint(6), runs[0].ID, "the five oldest rows go")
	assert.Equal(t, at(10*time.Second).UTC(), runs[99].StartedAt.UTC(), "the run just made stays")
}

// Stop, or the run timeout, ends the call: the devices not yet read are reported skipped and the
// run is still recorded.
func TestSchedulerTick_ACallThatEndsSkipsTheRestOfTheDevices(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := fx.newScheduler().clientFor
	fx.s.clientFor = func(did uint) (shelly.Client, error) {
		if did == 1 {
			cancel() // the stop arrives while device 1 is being read
		}
		return base(did)
	}

	fx.s.tick(ctx, at(10*time.Second))

	runs := fx.runs(id)
	require.Len(t, runs, 1)
	assert.Equal(t, DriftRunCompleted, runs[0].Status)
	var res BulkDriftResult
	require.NoError(t, json.Unmarshal(runs[0].Results, &res))
	require.Len(t, res.Results, 3)
	assert.Equal(t, "synced", res.Results[0].Status)
	for _, r := range res.Results[1:] {
		assert.Equal(t, BulkStatusSkipped, r.Status)
		assert.Equal(t, ReasonCallEnded, r.Error)
	}
	assert.Equal(t, 1, fx.schedule(id).RunCount)
}

// Two replicas that read the same due row: one claim wins, the other finds the row already moved.
func TestSchedulerClaim_OneWinnerPerTick(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	read := fx.schedule(id)
	next := at(5 * time.Minute)

	won, err := fx.s.claim(fx.db, read, next)
	require.NoError(t, err)
	assert.True(t, won)
	again, err := fx.s.claim(fx.db, read, next)
	require.NoError(t, err)
	assert.False(t, again, "the same stale read claimed twice")

	sc := fx.schedule(id)
	assert.Equal(t, 1, sc.RunCount)
	assert.True(t, sc.NextRun.Equal(next))
}

// The claim and the run row are one commit: a tick whose run cannot be recorded is not claimed, so
// it is still due and a crash between the two never loses an occurrence.
func TestSchedulerTick_ATickThatCannotRecordItsRunIsNotClaimed(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	require.NoError(t, fx.db.Migrator().DropTable(&DriftDetectionRun{}))

	fx.s.tick(context.Background(), at(10*time.Second))

	sc := fx.schedule(id)
	assert.Zero(t, sc.RunCount, "the claim committed without its run row")
	assert.True(t, sc.NextRun.Equal(at(0)), "next_run moved without a run row: %s", sc.NextRun)
}

// A schedule that waits behind a long run starts later than the poll that found it due: its run
// row says when it really started, or another replica would take the row for a dead run.
func TestSchedulerTick_ALaterScheduleRecordsWhenItReallyStarted(t *testing.T) {
	fx := newSchedFixture(t)
	first := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	second := fx.add("*/5 * * * *", ptr(at(0)), "", 2)
	fx.s.since = func(time.Time) time.Duration { return 9*time.Minute + 400*time.Millisecond }

	fx.s.tick(context.Background(), at(10*time.Second))

	for _, id := range []uint{first, second} {
		runs := fx.runs(id)
		require.Len(t, runs, 1)
		assert.True(t, runs[0].StartedAt.Equal(at(10*time.Second+9*time.Minute)), "started_at %s", runs[0].StartedAt)
	}
}

// A failure recorded for a schedule that cannot run counts toward the history cap too.
func TestSchedulerFailSchedule_KeepsTheHistoryCap(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("0 */5 * * * *", nil, "", 1)
	for i := 0; i < driftRunsKept; i++ {
		require.NoError(t, fx.db.Create(&DriftDetectionRun{
			ScheduleID: id, Status: DriftRunCompleted, StartedAt: at(-time.Duration(i+1) * time.Hour),
		}).Error)
	}

	fx.s.tick(context.Background(), at(0))

	assert.Len(t, fx.runs(id), driftRunsKept)
}

// Two replicas that read the same due schedule with a spec that cannot run record one failure.
func TestSchedulerFailSchedule_TwoReplicasRecordOneFailure(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("0 */5 * * * *", nil, "", 1)
	read := fx.schedule(id)

	fx.s.failSchedule(read, at(0), errors.New("invalid cron spec"))
	fx.s.failSchedule(read, at(0), errors.New("invalid cron spec"))

	assert.Len(t, fx.runs(id), 1)
	assert.False(t, fx.schedule(id).Enabled)
}

// A prune that read its list of rows to keep before another replica added a run must not delete
// that run.
func TestSchedulerPrune_NeverDeletesARunNewerThanItsList(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(time.Hour)), "", 1)
	var keep []uint
	for i := 0; i < driftRunsKept+1; i++ {
		run := DriftDetectionRun{ScheduleID: id, Status: DriftRunCompleted, StartedAt: at(time.Duration(i) * time.Minute)}
		require.NoError(t, fx.db.Create(&run).Error)
		if i > 0 {
			keep = append(keep, run.ID) // the newest 100 of the 101 rows at the time of the read
		}
	}
	newer := DriftDetectionRun{ScheduleID: id, Status: DriftRunCompleted, StartedAt: at(time.Hour)}
	require.NoError(t, fx.db.Create(&newer).Error) // another replica's run, after the read

	fx.s.deleteRunsNotIn(id, keep)

	runs := fx.runs(id)
	assert.Len(t, runs, driftRunsKept+1, "the oldest row goes and the newer run stays")
	assert.Equal(t, newer.ID, runs[len(runs)-1].ID)
	assert.NotEqual(t, uint(1), runs[0].ID)
}

func TestSchedulerClaim_ADisabledScheduleIsNotClaimed(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	read := fx.schedule(id)
	require.NoError(t, fx.db.Model(&DriftDetectionSchedule{}).Where("id = ?", id).Update("enabled", false).Error)

	won, err := fx.s.claim(fx.db, read, at(5*time.Minute))

	require.NoError(t, err)
	assert.False(t, won)
	assert.Zero(t, fx.schedule(id).RunCount)
}

func TestSchedulerLifecycle(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	ticks := make(chan time.Time)
	fx.s.ticks = ticks
	fx.s.now = func() time.Time { return at(10 * time.Second) }
	assert.False(t, fx.s.IsRunning())

	require.NoError(t, fx.s.Start(context.Background()))
	assert.True(t, fx.s.IsRunning())
	assert.Error(t, fx.s.Start(context.Background()), "a second Start is refused")

	ticks <- at(10 * time.Second)
	ticks <- at(20 * time.Second) // taken only once the loop has finished the first tick

	require.NoError(t, fx.s.Stop())
	assert.False(t, fx.s.IsRunning())
	assert.NoError(t, fx.s.Stop(), "a second Stop is a no-op")
	assert.Len(t, fx.runs(id), 1)
}

// A timer value that was delivered late carries the instant it was due, not the instant the loop
// took it: the poll runs at the clock read when it is received, so a run recorded after a long
// previous poll is not backdated.
func TestSchedulerLifecycle_ADelayedTickRunsAtTheClockNotTheTimerValue(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	ticks := make(chan time.Time)
	fx.s.ticks = ticks
	fx.s.now = func() time.Time { return at(17 * time.Minute) }

	require.NoError(t, fx.s.Start(context.Background()))
	ticks <- at(-time.Minute) // due a minute before the schedule, delivered at 17 minutes
	ticks <- at(-time.Minute) // taken only once the loop has finished the first tick
	require.NoError(t, fx.s.Stop())

	runs := fx.runs(id)
	require.Len(t, runs, 1, "the poll ran at the clock, when the schedule was due")
	assert.True(t, runs[0].StartedAt.Equal(at(17*time.Minute)), "the run starts at the clock read, got %s", runs[0].StartedAt)
}

func TestSchedulerStart_FailsWhenTheScheduleTableIsUnreadable(t *testing.T) {
	fx := newSchedFixture(t)
	require.NoError(t, fx.db.Migrator().DropTable(&DriftDetectionSchedule{}))
	fx.s.ticks = make(chan time.Time)

	assert.Error(t, fx.s.Start(context.Background()))
	assert.False(t, fx.s.IsRunning())
}

// The results column holds JSON text: backslashes and a large document come back unchanged.
func TestDriftRunResultsRoundTrip(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", nil, "")
	doc := json.RawMessage(`{"path":"C:\\temp\\x","note":"a\"b"}`)
	require.NoError(t, fx.db.Create(&DriftDetectionRun{ScheduleID: id, Status: DriftRunCompleted, StartedAt: at(0), Results: doc}).Error)

	runs := fx.runs(id)
	require.Len(t, runs, 1)
	assert.JSONEq(t, string(doc), string(runs[0].Results))
}

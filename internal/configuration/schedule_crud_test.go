package configuration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The write path is the Service's, over the same rows the scheduler reads. The fixture's devices
// 1..3 exist; the clock is the real one, so a next_run is checked against the spec at both ends of
// the call.

func boolPtr(b bool) *bool { return &b }

func (fx *schedFixture) count(model any, query string, args ...any) int64 {
	fx.t.Helper()
	var n int64
	require.NoError(fx.t, fx.db.Model(model).Where(query, args...).Count(&n).Error)
	return n
}

// nextBetween asserts that next is the spec's next instant as seen at some time between lo and hi.
func nextBetween(t *testing.T, spec string, next *time.Time, lo, hi time.Time) {
	t.Helper()
	require.NotNil(t, next)
	first, err := NextRun(spec, lo)
	require.NoError(t, err)
	last, err := NextRun(spec, hi)
	require.NoError(t, err)
	assert.False(t, next.Before(first) || next.After(last), "next_run %s is not in [%s, %s]", next, first, last)
}

func TestCreateDriftSchedule_StoresAnArmedScheduleWithItsDevices(t *testing.T) {
	fx := newSchedFixture(t)
	lo := time.Now()

	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "nightly", CronSpec: "*/5 * * * *", DeviceIDs: []uint{2, 1, 2}})

	require.NoError(t, err)
	nextBetween(t, "*/5 * * * *", sc.NextRun, lo, time.Now())
	assert.True(t, sc.Enabled, "a schedule is enabled unless the input says otherwise")
	assert.Equal(t, []uint{1, 2}, sc.DeviceIDs, "ids are sorted and a repeated id is dropped")
	stored, err := fx.svc.GetDriftSchedule(sc.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint{1, 2}, stored.DeviceIDs)
	assert.EqualValues(t, 2, fx.count(&DriftScheduleDevice{}, "schedule_id = ?", sc.ID))
}

func TestCreateDriftSchedule_ADisabledScheduleHasNoNextRun(t *testing.T) {
	fx := newSchedFixture(t)

	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "later", CronSpec: "0 3 * * *", Enabled: boolPtr(false)})

	require.NoError(t, err)
	assert.False(t, sc.Enabled)
	assert.Nil(t, sc.NextRun)
	assert.False(t, fx.schedule(sc.ID).Enabled, "the stored row is disabled too")
}

func TestCreateDriftSchedule_StoresTheFilterAndTheScheduleRunsOnIt(t *testing.T) {
	fx := newSchedFixture(t)

	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{
		Name: "plugs", CronSpec: "*/5 * * * *", DeviceFilter: &DriftDeviceFilter{Type: "SHPLG-S"},
	})
	require.NoError(t, err)

	require.NoError(t, fx.db.Model(&DriftDetectionSchedule{}).Where("id = ?", sc.ID).
		Update("next_run", at(0)).Error)
	fx.s.tick(context.Background(), at(time.Minute))
	runs := fx.runs(sc.ID)
	require.Len(t, runs, 1)
	assert.Equal(t, DriftRunCompleted, runs[0].Status)
	assert.Equal(t, []uint{2}, resultIDs(t, runs[0]))
}

func TestCreateDriftSchedule_RefusesWhatItCannotRunAndWritesNothing(t *testing.T) {
	cases := []struct {
		name string
		in   ScheduleInput
		want string
	}{
		{"no name", ScheduleInput{CronSpec: "*/5 * * * *"}, "name"},
		{"six fields", ScheduleInput{Name: "n", CronSpec: "0 */5 * * * *"}, "cron"},
		{"descriptor", ScheduleInput{Name: "n", CronSpec: "@hourly"}, "cron"},
		{"zone prefix", ScheduleInput{Name: "n", CronSpec: "TZ=Europe/Brussels * * * * *"}, "zone"},
		{"never fires", ScheduleInput{Name: "n", CronSpec: "0 0 30 2 *"}, "never fires"},
		{"empty spec", ScheduleInput{Name: "n"}, "cron"},
		{"list and filter", ScheduleInput{Name: "n", CronSpec: "* * * * *", DeviceIDs: []uint{1}, DeviceFilter: &DriftDeviceFilter{Type: "x"}}, "device_ids"},
		{"unknown device", ScheduleInput{Name: "n", CronSpec: "* * * * *", DeviceIDs: []uint{1, 99}}, "99"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newSchedFixture(t)

			sc, err := fx.svc.CreateDriftSchedule(c.in)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidDriftSchedule)
			assert.Contains(t, err.Error(), c.want)
			assert.Nil(t, sc)
			assert.Zero(t, fx.count(&DriftDetectionSchedule{}, "1 = 1"))
			assert.Zero(t, fx.count(&DriftScheduleDevice{}, "1 = 1"))
		})
	}
}

func TestUpdateDriftSchedule_ASpecChangeRecomputesNextRun(t *testing.T) {
	fx := newSchedFixture(t)
	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *"})
	require.NoError(t, err)
	lo := time.Now()

	updated, err := fx.svc.UpdateDriftSchedule(sc.ID, ScheduleInput{Name: "renamed", CronSpec: "0 3 * * *"})

	require.NoError(t, err)
	assert.Equal(t, "renamed", updated.Name)
	nextBetween(t, "0 3 * * *", updated.NextRun, lo, time.Now())
	nextBetween(t, "0 3 * * *", fx.schedule(sc.ID).NextRun, lo, time.Now())
}

func TestUpdateDriftSchedule_KeepsTheEnabledFlagWhenTheInputHasNone(t *testing.T) {
	fx := newSchedFixture(t)
	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", Enabled: boolPtr(false)})
	require.NoError(t, err)

	updated, err := fx.svc.UpdateDriftSchedule(sc.ID, ScheduleInput{Name: "n", CronSpec: "*/10 * * * *"})

	require.NoError(t, err)
	assert.False(t, updated.Enabled)
	assert.Nil(t, updated.NextRun, "a disabled schedule stays unarmed")
}

func TestUpdateDriftSchedule_ReplacesTheListOrTheFilterNeverBoth(t *testing.T) {
	fx := newSchedFixture(t)
	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", DeviceIDs: []uint{1, 2}})
	require.NoError(t, err)

	updated, err := fx.svc.UpdateDriftSchedule(sc.ID, ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", DeviceFilter: &DriftDeviceFilter{Status: "online"}})
	require.NoError(t, err)
	assert.Empty(t, updated.DeviceIDs)
	assert.Zero(t, fx.count(&DriftScheduleDevice{}, "schedule_id = ?", sc.ID), "the old list is gone")
	assert.JSONEq(t, `{"status":"online"}`, string(fx.schedule(sc.ID).DeviceFilter))

	back, err := fx.svc.UpdateDriftSchedule(sc.ID, ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", DeviceIDs: []uint{3}})
	require.NoError(t, err)
	assert.Equal(t, []uint{3}, back.DeviceIDs)
	assert.True(t, len(fx.schedule(sc.ID).DeviceFilter) == 0 || string(fx.schedule(sc.ID).DeviceFilter) == "null",
		"the old filter is gone: %q", fx.schedule(sc.ID).DeviceFilter)
}

func TestUpdateDriftSchedule_ABadInputLeavesTheStoredScheduleAsItWas(t *testing.T) {
	fx := newSchedFixture(t)
	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", DeviceIDs: []uint{1, 2}})
	require.NoError(t, err)

	_, err = fx.svc.UpdateDriftSchedule(sc.ID, ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", DeviceIDs: []uint{3, 99}})

	require.ErrorIs(t, err, ErrInvalidDriftSchedule)
	stored, err := fx.svc.GetDriftSchedule(sc.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint{1, 2}, stored.DeviceIDs)
}

func TestUpdateDriftSchedule_AMissingScheduleIsNotFound(t *testing.T) {
	fx := newSchedFixture(t)

	_, err := fx.svc.UpdateDriftSchedule(42, ScheduleInput{Name: "n", CronSpec: "*/5 * * * *"})

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Zero(t, fx.count(&DriftDetectionSchedule{}, "1 = 1"), "an update never creates")
}

func TestToggleDriftSchedule_OnArmsFromNowOffClearsTheNextRun(t *testing.T) {
	fx := newSchedFixture(t)
	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", Enabled: boolPtr(false)})
	require.NoError(t, err)
	lo := time.Now()

	on, err := fx.svc.ToggleDriftSchedule(sc.ID)
	require.NoError(t, err)
	assert.True(t, on.Enabled)
	nextBetween(t, "*/5 * * * *", on.NextRun, lo, time.Now())

	off, err := fx.svc.ToggleDriftSchedule(sc.ID)
	require.NoError(t, err)
	assert.False(t, off.Enabled)
	assert.Nil(t, off.NextRun)
	assert.Nil(t, fx.schedule(sc.ID).NextRun)
}

func TestToggleDriftSchedule_ASpecThatDoesNotParseCannotBeEnabled(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("0 */5 * * * *", nil, "", 1) // six fields, stored by an older release
	require.NoError(t, fx.db.Model(&DriftDetectionSchedule{}).Where("id = ?", id).Update("enabled", false).Error)

	_, err := fx.svc.ToggleDriftSchedule(id)

	assert.ErrorIs(t, err, ErrInvalidDriftSchedule)
	assert.False(t, fx.schedule(id).Enabled)
}

func TestToggleDriftSchedule_AMissingScheduleIsNotFound(t *testing.T) {
	fx := newSchedFixture(t)

	_, err := fx.svc.ToggleDriftSchedule(42)

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestDeleteDriftSchedule_RemovesTheDevicesAndTheRunsToo(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1, 2)
	other := fx.add("*/5 * * * *", ptr(at(0)), "", 3)
	fx.s.tick(context.Background(), at(time.Minute))
	require.NotEmpty(t, fx.runs(id))

	require.NoError(t, fx.svc.DeleteDriftSchedule(id))

	assert.Zero(t, fx.count(&DriftDetectionSchedule{}, "id = ?", id))
	assert.Zero(t, fx.count(&DriftScheduleDevice{}, "schedule_id = ?", id))
	assert.Zero(t, fx.count(&DriftDetectionRun{}, "schedule_id = ?", id))
	assert.EqualValues(t, 1, fx.count(&DriftScheduleDevice{}, "schedule_id = ?", other), "another schedule's list stays")
}

func TestDeleteDriftSchedule_AMissingScheduleIsNotFound(t *testing.T) {
	fx := newSchedFixture(t)

	err := fx.svc.DeleteDriftSchedule(42)

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestListDriftSchedules_FillsTheDeviceListsOfEverySchedule(t *testing.T) {
	fx := newSchedFixture(t)
	a := fx.add("*/5 * * * *", nil, "", 3, 1)
	b := fx.add("*/5 * * * *", nil, "")

	list, err := fx.svc.ListDriftSchedules()

	require.NoError(t, err)
	require.Len(t, list, 2)
	byID := map[uint][]uint{}
	for _, sc := range list {
		byID[sc.ID] = sc.DeviceIDs
	}
	assert.Equal(t, []uint{1, 3}, byID[a])
	assert.Empty(t, byID[b])
}

// A claim that read the row before an Update must not undo it: it would put the old spec's next
// instant over the new one, and run a tick of a spec that no longer exists.
func TestSchedulerClaim_AReplacedSpecIsNotClaimedFromAnEarlierRead(t *testing.T) {
	fx := newSchedFixture(t)
	sc, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *", DeviceIDs: []uint{1}})
	require.NoError(t, err)
	require.NoError(t, fx.db.Model(&DriftDetectionSchedule{}).Where("id = ?", sc.ID).Update("next_run", at(0)).Error)
	read := fx.schedule(sc.ID)

	updated, err := fx.svc.UpdateDriftSchedule(sc.ID, ScheduleInput{Name: "n", CronSpec: "0 3 * * *", DeviceIDs: []uint{1}})
	require.NoError(t, err)

	won, err := fx.s.claim(fx.db, read, at(5*time.Minute))

	require.NoError(t, err)
	assert.False(t, won)
	assert.Zero(t, fx.schedule(sc.ID).RunCount)
	assert.True(t, fx.schedule(sc.ID).NextRun.Equal(*updated.NextRun), "the edit's next_run stands")
}

// The same race, on the path that disables a schedule: a row whose spec did not parse is read, the
// user fixes the spec, and the stale read must not disable the fixed schedule.
func TestSchedulerFailSchedule_AReplacedSpecIsNotDisabledFromAnEarlierRead(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("0 */5 * * * *", nil, "", 1) // six fields: stored by an older release
	read := fx.schedule(id)

	updated, err := fx.svc.UpdateDriftSchedule(id, ScheduleInput{Name: "nightly", CronSpec: "*/5 * * * *", DeviceIDs: []uint{1}})
	require.NoError(t, err)
	require.True(t, updated.Enabled)

	fx.s.failSchedule(read, at(0), assert.AnError)

	assert.True(t, fx.schedule(id).Enabled, "the fixed schedule stays enabled")
	assert.Empty(t, fx.runs(id), "and no failed run is written for a spec that is gone")
}

// A poll that read an enabled, unarmed row must not arm it after the user turned it off: arming
// would put a next_run on a disabled schedule, which a toggle on is the only thing meant to do.
func TestSchedulerArm_AStaleReadDoesNotArmADisabledSchedule(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", nil, "", 1) // enabled, never armed
	read := fx.schedule(id)
	_, err := fx.svc.ToggleDriftSchedule(id) // off
	require.NoError(t, err)

	fx.s.handle(context.Background(), read, at(0))

	assert.False(t, fx.schedule(id).Enabled)
	assert.Nil(t, fx.schedule(id).NextRun, "a disabled schedule has no next_run")
}

func TestSchedulerArm_AStaleReadDoesNotArmAReplacedSpec(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", nil, "", 1)
	read := fx.schedule(id)
	updated, err := fx.svc.UpdateDriftSchedule(id, ScheduleInput{Name: "n", CronSpec: "0 3 * * *", DeviceIDs: []uint{1}})
	require.NoError(t, err)

	fx.s.handle(context.Background(), read, at(0))

	assert.True(t, fx.schedule(id).NextRun.Equal(*updated.NextRun), "the edit's next_run stands")
}

// A schedule that cannot run is disabled with no next_run, so a disabled schedule never shows one.
func TestSchedulerFailSchedule_ClearsTheNextRunItDisables(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("0 */5 * * * *", ptr(at(0)), "", 1) // six fields, due
	fx.s.tick(context.Background(), at(time.Minute))

	assert.False(t, fx.schedule(id).Enabled)
	assert.Nil(t, fx.schedule(id).NextRun)
}

// The row of a run whose schedule was deleted while it ran must not come back when the run ends.
func TestSchedulerRun_ADeletedSchedulesRunIsNotRecreated(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "", 1)
	sc := fx.schedule(id)
	run, err := fx.s.claimAndRecord(sc, at(5*time.Minute), at(time.Minute))
	require.NoError(t, err)
	require.NotNil(t, run)
	require.NoError(t, fx.svc.DeleteDriftSchedule(id))

	fx.s.runClaimed(context.Background(), sc, run, at(time.Minute))

	assert.Zero(t, fx.count(&DriftDetectionRun{}, "schedule_id = ?", id))
}

// An update whose schedule is deleted between its read and its write must leave no device rows.
func TestUpdateDriftSchedule_ADeleteInTheMiddleWritesNoDeviceRows(t *testing.T) {
	fx := newSchedFixture(t)
	id := fx.add("*/5 * * * *", ptr(at(0)), "") // no device list yet
	fired := false
	require.NoError(t, fx.db.Callback().Query().After("gorm:query").Register("test:delete-after-read", func(tx *gorm.DB) {
		if fired || tx.Statement.Table != "drift_detection_schedules" {
			return
		}
		fired = true
		_, err := tx.Statement.ConnPool.ExecContext(tx.Statement.Context, "DELETE FROM drift_detection_schedules")
		require.NoError(t, err)
	}))

	_, err := fx.svc.UpdateDriftSchedule(id, ScheduleInput{Name: "n", CronSpec: "0 3 * * *", DeviceIDs: []uint{2, 3}})

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.True(t, fired)
	assert.Zero(t, fx.count(&DriftScheduleDevice{}, "schedule_id = ?", id))
}

func TestDriftSchedule_ANoDeviceListIsAnEmptyArrayInJSON(t *testing.T) {
	fx := newSchedFixture(t)
	created, err := fx.svc.CreateDriftSchedule(ScheduleInput{Name: "n", CronSpec: "*/5 * * * *"})
	require.NoError(t, err)
	got, err := fx.svc.GetDriftSchedule(created.ID)
	require.NoError(t, err)
	list, err := fx.svc.ListDriftSchedules()
	require.NoError(t, err)

	for name, v := range map[string]any{"create": created, "get": got, "list": list[0]} {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"device_ids":[]`, name)
	}
}

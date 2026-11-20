package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// seedRuns stores n runs of a schedule, run i started i minutes after the first, each with a result.
func (a *scheduleAPI) seedRuns(scheduleID uint, n int) {
	a.t.Helper()
	first := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		run := configuration.DriftDetectionRun{
			ScheduleID: scheduleID, Status: "completed", StartedAt: first.Add(time.Duration(i) * time.Minute),
			Results: json.RawMessage(`{"total":1}`),
		}
		require.NoError(a.t, a.db.GetDB().Create(&run).Error)
	}
}

func (a *scheduleAPI) runs(id, query string) (*httptest.ResponseRecorder, []map[string]any) {
	a.t.Helper()
	w := a.do(http.MethodGet, schedulesURL+"/"+id+"/runs"+query, a.handler.GetDriftScheduleRuns, "", id)
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if w.Code == http.StatusOK {
		require.NoError(a.t, json.Unmarshal(w.Body.Bytes(), &resp))
	}
	return w, resp.Data
}

func TestDriftScheduleRunsRoute_ListsTheNewestRunsFirstWithoutTheirResults(t *testing.T) {
	a := newScheduleAPI(t)
	sc := decodeSchedule(t, a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"nightly","cron_spec":"0 3 * * *"}`, "")).Data
	a.seedRuns(sc.ID, 3)

	w, runs := a.runs(strconv.Itoa(int(sc.ID)), "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, runs, 3)
	assert.Equal(t, "2026-11-20T08:02:00Z", runs[0]["started_at"], "newest first")
	assert.Equal(t, "2026-11-20T08:00:00Z", runs[2]["started_at"])
	for _, run := range runs {
		assert.NotContains(t, run, "results", "the list leaves the result document out")
		assert.Equal(t, "completed", run["status"])
	}
}

func TestDriftScheduleRunsRoute_LimitDefaultsToTwentyAndIsBounded(t *testing.T) {
	a := newScheduleAPI(t)
	sc := decodeSchedule(t, a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"nightly","cron_spec":"0 3 * * *"}`, "")).Data
	a.seedRuns(sc.ID, 25)
	id := strconv.Itoa(int(sc.ID))

	_, byDefault := a.runs(id, "")
	_, three := a.runs(id, "?limit=3")
	_, all := a.runs(id, "?limit=100")

	assert.Len(t, byDefault, 20)
	assert.Len(t, three, 3)
	assert.Len(t, all, 25)
	for _, bad := range []string{"0", "-1", "101", "abc", "1.5"} {
		w, _ := a.runs(id, "?limit="+bad)
		assert.Equal(t, http.StatusBadRequest, w.Code, "limit=%s", bad)
	}
}

func TestDriftScheduleRunsRoute_AScheduleWithNoRunIsAnEmptyListAndAMissingOneIs404(t *testing.T) {
	a := newScheduleAPI(t)
	sc := decodeSchedule(t, a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"nightly","cron_spec":"0 3 * * *"}`, "")).Data

	w, runs := a.runs(strconv.Itoa(int(sc.ID)), "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, runs)
	assert.Empty(t, runs)
	assert.True(t, strings.Contains(w.Body.String(), `"data":[]`), w.Body.String())

	missing, _ := a.runs("999", "")
	assert.Equal(t, http.StatusNotFound, missing.Code)
	notANumber, _ := a.runs("abc", "")
	assert.Equal(t, http.StatusBadRequest, notANumber.Code)
}

func TestDriftSchedulerStatusRoute_ReportsWhetherThisProcessRunsTheScheduler(t *testing.T) {
	a := newScheduleAPI(t)
	read := func() bool {
		w := a.do(http.MethodGet, "/api/v1/config/drift-scheduler", a.handler.GetDriftSchedulerStatus, "", "")
		require.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Data struct {
				Enabled bool `json:"enabled"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		return resp.Data.Enabled
	}

	assert.False(t, read(), "a handler never told answers false")
	a.handler.SetDriftSchedulerEnabled(true)
	assert.True(t, read())
}

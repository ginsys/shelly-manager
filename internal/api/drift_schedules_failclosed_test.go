package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The run-history route stays closed until it reads the runs the scheduler records (#279): an empty
// list would assert history that was never read. The other schedule routes are tested in
// drift_schedules_write_test.go.

// TestDriftScheduleRunsRoute_FailsClosed_NoServiceAccess proves the handler returns 501 before parsing
// the id or reaching the service: it is built with a nil Service and nil DB, so any access would
// panic, and a clean 501 is the assertion.
func TestDriftScheduleRunsRoute_FailsClosed_NoServiceAccess(t *testing.T) {
	handler := &Handler{logger: logging.GetDefault()} // Service and DB deliberately nil
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/drift-schedules/not-a-number/runs?limit=oops", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "not-a-number"})
	w := httptest.NewRecorder()

	handler.GetDriftScheduleRuns(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.False(t, resp.Success)
	assert.Equal(t, apiresp.ErrCodeNotImplemented, resp.Error.Code)
	assert.Equal(t, configuration.ErrSchedulingNotImplemented.Error(), resp.Error.Message)
}

// TestDriftScheduleRunsRoute_NoDatabaseSideEffects runs the closed route against a real database with
// a valid id and asserts nothing was written, updated or deleted.
func TestDriftScheduleRunsRoute_NoDatabaseSideEffects(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger := logging.GetDefault()

	// The database Manager above creates the schedule and run tables; NewService migrates the
	// device config, config history, drift report and drift trend tables.
	configuration.NewService(db.GetDB(), logger)
	handler := &Handler{DB: db, Service: testShellyService(t, db), logger: logger}

	seed := configuration.DriftDetectionSchedule{Name: "stored", Enabled: true, CronSpec: "0 0 * * *"}
	require.NoError(t, db.GetDB().Create(&seed).Error)
	run := configuration.DriftDetectionRun{ScheduleID: seed.ID, Status: "completed", StartedAt: time.Now().UTC()}
	require.NoError(t, db.GetDB().Create(&run).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/drift-schedules/1/runs", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()
	handler.GetDriftScheduleRuns(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	var schedules, runs int64
	require.NoError(t, db.GetDB().Model(&configuration.DriftDetectionSchedule{}).Count(&schedules).Error)
	require.NoError(t, db.GetDB().Model(&configuration.DriftDetectionRun{}).Count(&runs).Error)
	assert.EqualValues(t, 1, schedules)
	assert.EqualValues(t, 1, runs)
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// scheduleAPI is a handler over a real database with devices 1 and 2.
type scheduleAPI struct {
	t       *testing.T
	db      database.DatabaseInterface
	handler *Handler
}

func newScheduleAPI(t *testing.T) *scheduleAPI {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	logger := logging.GetDefault()
	configuration.NewService(db.GetDB(), logger)
	for i := 1; i <= 2; i++ { // devices 1 and 2
		n := strconv.Itoa(i)
		require.NoError(t, db.AddDevice(&database.Device{
			IP: "192.168.1." + n, MAC: "AA:BB:CC:DD:EE:0" + n, Type: "SHSW-1", Name: "device " + n, Status: "online",
		}))
	}
	return &scheduleAPI{t: t, db: db, handler: &Handler{DB: db, Service: testShellyService(t, db), logger: logger}}
}

type scheduleResponse struct {
	Success bool                                 `json:"success"`
	Data    configuration.DriftDetectionSchedule `json:"data"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// do calls a handler with a body and mux vars and returns the recorder.
func (a *scheduleAPI) do(method, target string, h http.HandlerFunc, body string, id string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if id != "" {
		req = mux.SetURLVars(req, map[string]string{"id": id})
	}
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func (a *scheduleAPI) count(model any) int64 {
	a.t.Helper()
	var n int64
	require.NoError(a.t, a.db.GetDB().Model(model).Count(&n).Error)
	return n
}

func decodeSchedule(t *testing.T, w *httptest.ResponseRecorder) scheduleResponse {
	t.Helper()
	var resp scheduleResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	return resp
}

const schedulesURL = "/api/v1/config/drift-schedules"

func TestCreateDriftScheduleRoute_StoresARunnableSchedule(t *testing.T) {
	a := newScheduleAPI(t)

	w := a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"nightly","cron_spec":"*/5 * * * *","device_ids":[2,1]}`, "")

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	resp := decodeSchedule(t, w)
	assert.True(t, resp.Data.Enabled)
	assert.NotNil(t, resp.Data.NextRun)
	assert.Equal(t, []uint{1, 2}, resp.Data.DeviceIDs)
	assert.EqualValues(t, 1, a.count(&configuration.DriftDetectionSchedule{}))
}

func TestCreateDriftScheduleRoute_RefusesABadBodyAndWritesNothing(t *testing.T) {
	cases := map[string]string{
		"not json":            `{not json`,
		"a six-field spec":    `{"name":"n","cron_spec":"0 */5 * * * *"}`,
		"a descriptor":        `{"name":"n","cron_spec":"@hourly"}`,
		"no name":             `{"cron_spec":"* * * * *"}`,
		"a legacy filter key": `{"name":"n","cron_spec":"* * * * *","device_filter":{"device_type":"SHSW-1"}}`,
		"an unknown field":    `{"name":"n","cron_spec":"* * * * *","schedule":"x"}`,
		"text after the body": `{"name":"n","cron_spec":"* * * * *"} garbage`,
		"list and filter":     `{"name":"n","cron_spec":"* * * * *","device_ids":[1],"device_filter":{"type":"SHSW-1"}}`,
		"an unknown device":   `{"name":"n","cron_spec":"* * * * *","device_ids":[1,99]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			a := newScheduleAPI(t)

			w := a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule, body, "")

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Zero(t, a.count(&configuration.DriftDetectionSchedule{}))
			assert.Zero(t, a.count(&configuration.DriftScheduleDevice{}))
		})
	}
}

func TestUpdateAndToggleDriftScheduleRoutes(t *testing.T) {
	a := newScheduleAPI(t)
	created := decodeSchedule(t, a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"nightly","cron_spec":"*/5 * * * *"}`, "")).Data
	id := strconv.Itoa(int(created.ID))

	w := a.do(http.MethodPut, schedulesURL+"/"+id, a.handler.UpdateDriftSchedule,
		`{"name":"renamed","cron_spec":"0 3 * * *","device_filter":{"status":"online"},"revision":`+strconv.Itoa(created.Revision)+`}`, id)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	updated := decodeSchedule(t, w).Data
	assert.Equal(t, "renamed", updated.Name)
	assert.Equal(t, "0 3 * * *", updated.CronSpec)
	assert.JSONEq(t, `{"status":"online"}`, string(updated.DeviceFilter))
	assert.Equal(t, created.Revision+1, updated.Revision, "a write bumps the revision the answer carries")

	w = a.do(http.MethodPost, schedulesURL+"/"+id+"/toggle", a.handler.ToggleDriftSchedule, "", id)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	off := decodeSchedule(t, w).Data
	assert.False(t, off.Enabled)
	assert.Nil(t, off.NextRun)
	assert.Equal(t, updated.Revision+1, off.Revision, "a toggle answers with its own revision")
}

// An update names the revision it read. The one that is stored writes; any other is a 409 that carries the
// current schedule and changes nothing; none is a 400, so no caller can skip the check.
func TestUpdateDriftScheduleRoute_ChecksTheRevisionTheCallerRead(t *testing.T) {
	a := newScheduleAPI(t)
	created := decodeSchedule(t, a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"nightly","cron_spec":"*/5 * * * *"}`, "")).Data
	id := strconv.Itoa(int(created.ID))
	put := func(body string) *httptest.ResponseRecorder {
		return a.do(http.MethodPut, schedulesURL+"/"+id, a.handler.UpdateDriftSchedule, body, id)
	}
	rev := strconv.Itoa(created.Revision)

	// Another writer saved first: the stored revision moved on.
	require.Equal(t, http.StatusOK, put(`{"name":"first","cron_spec":"*/5 * * * *","revision":`+rev+`}`).Code)

	w := put(`{"name":"second","cron_spec":"*/5 * * * *","revision":` + rev + `}`)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var conflict struct {
		Error struct {
			Code    string                               `json:"code"`
			Details configuration.DriftDetectionSchedule `json:"details"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&conflict))
	assert.Equal(t, "CONFLICT", conflict.Error.Code)
	assert.Equal(t, "first", conflict.Error.Details.Name, "the answer carries the current schedule")
	assert.Equal(t, created.Revision+1, conflict.Error.Details.Revision)
	var stored configuration.DriftDetectionSchedule
	require.NoError(t, a.db.GetDB().First(&stored, created.ID).Error)
	assert.Equal(t, "first", stored.Name, "a refused write changes nothing")
	assert.Equal(t, created.Revision+1, stored.Revision, "and does not bump the revision")

	for name, body := range map[string]string{
		"no revision":     `{"name":"third","cron_spec":"*/5 * * * *"}`,
		"a null revision": `{"name":"third","cron_spec":"*/5 * * * *","revision":null}`,
	} {
		w = put(body)
		assert.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
		assert.Contains(t, w.Body.String(), "revision is required", name)
	}
	require.NoError(t, a.db.GetDB().First(&stored, created.ID).Error)
	assert.Equal(t, "first", stored.Name)

	// Create has no revision to send: it is an unknown field there, as before.
	w = a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"n","cron_spec":"* * * * *","revision":0}`, "")
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestScheduleWriteRoutes_ABadOrMissingIDIsRefused(t *testing.T) {
	a := newScheduleAPI(t)
	body := `{"name":"n","cron_spec":"* * * * *","revision":0}`

	assert.Equal(t, http.StatusBadRequest,
		a.do(http.MethodPut, schedulesURL+"/x", a.handler.UpdateDriftSchedule, body, "x").Code)
	assert.Equal(t, http.StatusBadRequest,
		a.do(http.MethodPost, schedulesURL+"/x/toggle", a.handler.ToggleDriftSchedule, "", "x").Code)
	assert.Equal(t, http.StatusNotFound,
		a.do(http.MethodPut, schedulesURL+"/999", a.handler.UpdateDriftSchedule, body, "999").Code)
	assert.Equal(t, http.StatusNotFound,
		a.do(http.MethodPost, schedulesURL+"/999/toggle", a.handler.ToggleDriftSchedule, "", "999").Code)
	assert.Equal(t, http.StatusNotFound,
		a.do(http.MethodDelete, schedulesURL+"/999", a.handler.DeleteDriftSchedule, "", "999").Code,
		"a missing schedule is a 404, not a 200")
	assert.Zero(t, a.count(&configuration.DriftDetectionSchedule{}), "an update never creates")
}

func TestDriftScheduleReadAndDeleteRoutes_StayTruthful(t *testing.T) {
	a := newScheduleAPI(t)
	created := decodeSchedule(t, a.do(http.MethodPost, schedulesURL, a.handler.CreateDriftSchedule,
		`{"name":"keepme","cron_spec":"0 0 * * *","device_ids":[1,2]}`, "")).Data
	id := strconv.Itoa(int(created.ID))

	w := a.do(http.MethodGet, schedulesURL, a.handler.GetDriftSchedules, "", "")
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Data []configuration.DriftDetectionSchedule `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&list))
	require.Len(t, list.Data, 1)
	assert.Equal(t, []uint{1, 2}, list.Data[0].DeviceIDs, "the list carries each schedule's devices")

	w = a.do(http.MethodGet, schedulesURL+"/"+id, a.handler.GetDriftSchedule, "", id)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []uint{1, 2}, decodeSchedule(t, w).Data.DeviceIDs)
	assert.Equal(t, http.StatusNotFound,
		a.do(http.MethodGet, schedulesURL+"/999", a.handler.GetDriftSchedule, "", "999").Code)

	require.Equal(t, http.StatusOK, a.do(http.MethodDelete, schedulesURL+"/"+id, a.handler.DeleteDriftSchedule, "", id).Code)
	assert.Zero(t, a.count(&configuration.DriftDetectionSchedule{}))
	assert.Zero(t, a.count(&configuration.DriftScheduleDevice{}), "the device list goes with the schedule")
}

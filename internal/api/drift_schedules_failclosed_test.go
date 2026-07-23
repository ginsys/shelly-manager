package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// failClosedRoute describes one drift-schedule endpoint that must fail closed.
type failClosedRoute struct {
	name    string
	method  string
	target  string
	handler func(*Handler) http.HandlerFunc
	// body is sent as-is; a deliberately malformed payload proves the handler
	// returns 501 before it would ever decode input.
	body string
	// vars are the mux path variables; a non-numeric id proves the handler
	// returns 501 before it would parse the id.
	vars map[string]string
}

func failClosedRoutes() []failClosedRoute {
	return []failClosedRoute{
		{
			name:    "create",
			method:  http.MethodPost,
			target:  "/api/v1/config/drift-schedules",
			handler: func(h *Handler) http.HandlerFunc { return h.CreateDriftSchedule },
			body:    "{not valid json",
		},
		{
			name:    "update",
			method:  http.MethodPut,
			target:  "/api/v1/config/drift-schedules/not-a-number",
			handler: func(h *Handler) http.HandlerFunc { return h.UpdateDriftSchedule },
			body:    "{not valid json",
			vars:    map[string]string{"id": "not-a-number"},
		},
		{
			name:    "toggle",
			method:  http.MethodPost,
			target:  "/api/v1/config/drift-schedules/not-a-number/toggle",
			handler: func(h *Handler) http.HandlerFunc { return h.ToggleDriftSchedule },
			vars:    map[string]string{"id": "not-a-number"},
		},
		{
			name:    "runs",
			method:  http.MethodGet,
			target:  "/api/v1/config/drift-schedules/not-a-number/runs?limit=oops",
			handler: func(h *Handler) http.HandlerFunc { return h.GetDriftScheduleRuns },
			vars:    map[string]string{"id": "not-a-number"},
		},
	}
}

// TestDriftScheduleWriteRoutes_FailClosed_NoServiceAccess proves the four
// fail-closed handlers return 501 before parsing input or reaching the service.
// The handler is built with a nil Service and nil DB, so any parse/service
// access would panic; a clean 501 is the assertion.
func TestDriftScheduleWriteRoutes_FailClosed_NoServiceAccess(t *testing.T) {
	handler := &Handler{logger: logging.GetDefault()} // Service and DB deliberately nil

	for _, route := range failClosedRoutes() {
		t.Run(route.name, func(t *testing.T) {
			var body *strings.Reader
			if route.body != "" {
				body = strings.NewReader(route.body)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(route.method, route.target, body)
			if route.vars != nil {
				req = mux.SetURLVars(req, route.vars)
			}
			w := httptest.NewRecorder()

			route.handler(handler)(w, req)

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
		})
	}
}

// TestDriftScheduleWriteRoutes_NoDatabaseSideEffects seeds a schedule and a run
// row, exercises every fail-closed route against a real database, and asserts
// nothing was written, updated or deleted. This is the integration backstop
// behind the nil-Service unit proof above.
func TestDriftScheduleWriteRoutes_NoDatabaseSideEffects(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger := logging.GetDefault()

	// NewService runs the AutoMigrate that creates the schedule/run tables.
	configuration.NewService(db.GetDB(), logger)

	handler := &Handler{
		DB:      db,
		Service: testShellyService(t, db),
		logger:  logger,
	}

	seed := configuration.DriftDetectionSchedule{
		Name:     "stored-only",
		Enabled:  true,
		CronSpec: "0 0 * * *",
	}
	require.NoError(t, db.GetDB().Create(&seed).Error)
	run := configuration.DriftDetectionRun{ScheduleID: seed.ID, Status: "completed"}
	require.NoError(t, db.GetDB().Create(&run).Error)

	snapshot := func() (schedules, runs int64, first configuration.DriftDetectionSchedule) {
		require.NoError(t, db.GetDB().Model(&configuration.DriftDetectionSchedule{}).Count(&schedules).Error)
		require.NoError(t, db.GetDB().Model(&configuration.DriftDetectionRun{}).Count(&runs).Error)
		require.NoError(t, db.GetDB().First(&first, seed.ID).Error)
		return
	}

	beforeSchedules, beforeRuns, beforeRow := snapshot()

	// Use valid ids and bodies here so that, were the handlers not failing
	// closed, they would actually mutate the database.
	valid := `{"name":"mutated","enabled":false,"cron_spec":"0 */6 * * *"}`
	cases := []struct {
		name    string
		method  string
		target  string
		handler http.HandlerFunc
		body    string
		vars    map[string]string
	}{
		{"create", http.MethodPost, "/api/v1/config/drift-schedules", handler.CreateDriftSchedule, valid, nil},
		{"update", http.MethodPut, "/api/v1/config/drift-schedules/1", handler.UpdateDriftSchedule, valid, map[string]string{"id": "1"}},
		{"toggle", http.MethodPost, "/api/v1/config/drift-schedules/1/toggle", handler.ToggleDriftSchedule, "{}", map[string]string{"id": "1"}},
		{"runs", http.MethodGet, "/api/v1/config/drift-schedules/1/runs", handler.GetDriftScheduleRuns, "", map[string]string{"id": "1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.target, strings.NewReader(c.body))
			if c.vars != nil {
				req = mux.SetURLVars(req, c.vars)
			}
			w := httptest.NewRecorder()
			c.handler(w, req)
			assert.Equal(t, http.StatusNotImplemented, w.Code)
		})
	}

	afterSchedules, afterRuns, afterRow := snapshot()
	assert.Equal(t, beforeSchedules, afterSchedules, "schedule count changed")
	assert.Equal(t, beforeRuns, afterRuns, "run count changed")
	assert.Equal(t, beforeRow.Name, afterRow.Name, "schedule name changed")
	assert.Equal(t, beforeRow.Enabled, afterRow.Enabled, "schedule enabled flag changed")
	assert.Equal(t, beforeRow.CronSpec, afterRow.CronSpec, "schedule cron_spec changed")
}

// TestDriftScheduleReadRoutes_StayTruthful confirms the retained inspection and
// deletion routes keep working against real data — list, detail, a real delete,
// and a genuine 404 for a missing schedule (proving the errors.Is not-found fix).
func TestDriftScheduleReadRoutes_StayTruthful(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	logger := logging.GetDefault()
	configuration.NewService(db.GetDB(), logger)

	handler := &Handler{
		DB:      db,
		Service: testShellyService(t, db),
		logger:  logger,
	}

	seed := configuration.DriftDetectionSchedule{Name: "keepme", Enabled: true, CronSpec: "0 0 * * *"}
	require.NoError(t, db.GetDB().Create(&seed).Error)

	t.Run("list returns stored schedules", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/config/drift-schedules", nil)
		w := httptest.NewRecorder()
		handler.GetDriftSchedules(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Success bool                                   `json:"success"`
			Data    []configuration.DriftDetectionSchedule `json:"data"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.True(t, resp.Success)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "keepme", resp.Data[0].Name)
	})

	t.Run("detail returns stored schedule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/config/drift-schedules/1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "1"})
		w := httptest.NewRecorder()
		handler.GetDriftSchedule(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("detail 404 for missing schedule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/config/drift-schedules/999", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "999"})
		w := httptest.NewRecorder()
		handler.GetDriftSchedule(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("delete removes the schedule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/config/drift-schedules/1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "1"})
		w := httptest.NewRecorder()
		handler.DeleteDriftSchedule(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var count int64
		require.NoError(t, db.GetDB().Model(&configuration.DriftDetectionSchedule{}).Count(&count).Error)
		assert.Equal(t, int64(0), count)
	})
}

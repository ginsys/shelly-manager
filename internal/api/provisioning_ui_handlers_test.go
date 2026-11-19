package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func newTestHandler(t *testing.T) (*Handler, *database.Manager) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	svc := testShellyService(t, db)
	nh := testNotificationHandler(t, db)
	return NewHandlerWithLogger(db, svc, nh, nil, logging.GetDefault()), db
}

func seedDevice(t *testing.T, db *database.Manager, ip, mac, name string) *database.Device {
	t.Helper()
	d := &database.Device{IP: ip, MAC: mac, Name: name, Type: "Shelly1"}
	require.NoError(t, db.AddDevice(d))
	return d
}

func TestListProvisioningTasksUI_EmptyThenPopulated(t *testing.T) {
	h, db := newTestHandler(t)

	// Empty
	req := httptest.NewRequest("GET", "/api/v1/provisioning/tasks", nil)
	w := httptest.NewRecorder()
	h.ListProvisioningTasksUI(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			Tasks []uiProvisioningTask `json:"tasks"`
			Count int                  `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.True(t, env.Success)
	assert.Equal(t, 0, env.Data.Count)
	assert.Empty(t, env.Data.Tasks)

	// Seed a task row
	seedProvisionerTask(t, db, database.ProvisioningTask{
		TaskID: "task_abc", Type: "configure", Status: "pending",
		Config: map[string]interface{}{}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	w = httptest.NewRecorder()
	h.ListProvisioningTasksUI(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	env.Data.Tasks = nil
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Len(t, env.Data.Tasks, 1)
	assert.Equal(t, "task_abc", env.Data.Tasks[0].ID)
	assert.Equal(t, "pending", env.Data.Tasks[0].Status)
}

func TestListProvisioningTasksUI_StatusFilterAndMapping(t *testing.T) {
	h, db := newTestHandler(t)

	now := time.Now()
	for id, status := range map[string]string{"t1": "pending", "t2": "assigned", "t3": "in_progress", "t4": "completed"} {
		seedProvisionerTask(t, db, database.ProvisioningTask{TaskID: id, Type: "configure", Status: status, CreatedAt: now})
	}

	req := httptest.NewRequest("GET", "/api/v1/provisioning/tasks?status=running", nil)
	w := httptest.NewRecorder()
	h.ListProvisioningTasksUI(w, req)

	var env struct {
		Data struct {
			Tasks []uiProvisioningTask `json:"tasks"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	// assigned + in_progress both map to running
	ids := []string{}
	for _, task := range env.Data.Tasks {
		assert.Equal(t, "running", task.Status)
		ids = append(ids, task.ID)
	}
	assert.ElementsMatch(t, []string{"t2", "t3"}, ids)
}

func TestGetProvisioningTaskUI_FoundAndMissing(t *testing.T) {
	h, db := newTestHandler(t)

	seedProvisionerTask(t, db, database.ProvisioningTask{
		TaskID: "task_xyz", Type: "update", Status: "completed",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/v1/provisioning/tasks/task_xyz", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "task_xyz"})
	w := httptest.NewRecorder()
	h.GetProvisioningTaskUI(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Missing
	req = httptest.NewRequest("GET", "/api/v1/provisioning/tasks/nope", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nope"})
	w = httptest.NewRecorder()
	h.GetProvisioningTaskUI(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateProvisioningTaskUI_ResolvesDeviceMAC(t *testing.T) {
	h, db := newTestHandler(t)

	dev := seedDevice(t, db, "10.0.0.1", "AA:BB:CC:DD:EE:01", "Living Room")

	body := map[string]interface{}{
		"deviceId": strconv.FormatUint(uint64(dev.ID), 10),
		"taskType": "configure",
		"config":   map[string]interface{}{"ssid": "Home"},
	}
	bodyJSON, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/provisioning/tasks", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateProvisioningTaskUI(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var env struct {
		Data uiProvisioningTask `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, "configure", env.Data.TaskType)
	assert.Equal(t, "pending", env.Data.Status)
	assert.Equal(t, strconv.FormatUint(uint64(dev.ID), 10), env.Data.DeviceID)
	assert.Equal(t, "Living Room", env.Data.DeviceName)

	// Confirm task got inserted with the MAC resolved from deviceId
	tasks, err := database.NewProvisionerStore(db.GetDB()).ListTasks(registryDeadline(t))
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, dev.MAC, tasks[0].DeviceMAC)
}

func TestCreateProvisioningTaskUI_ValidationErrors(t *testing.T) {
	h, _ := newTestHandler(t)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"invalid json", "not-json", http.StatusBadRequest},
		{"missing taskType", `{"deviceId":"1"}`, http.StatusBadRequest},
		{"unknown device", `{"deviceId":"9999","taskType":"configure"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/provisioning/tasks", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CreateProvisioningTaskUI(w, req)
			assert.Equal(t, tc.want, w.Code)
		})
	}
}

func TestCancelProvisioningTaskUI(t *testing.T) {
	h, db := newTestHandler(t)

	seedProvisionerTask(t, db, database.ProvisioningTask{TaskID: "t_pending", Type: "configure", Status: "pending", Config: map[string]interface{}{}})
	seedProvisionerTask(t, db, database.ProvisioningTask{TaskID: "t_done", Type: "configure", Status: "completed", Config: map[string]interface{}{}})

	for _, id := range []string{"t_pending", "t_done"} {
		req := httptest.NewRequest("POST", "/api/v1/provisioning/tasks/"+id+"/cancel", nil)
		req = mux.SetURLVars(req, map[string]string{"id": id})
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.CancelProvisioningTaskUI(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	store := database.NewProvisionerStore(db.GetDB())
	pending, err := store.GetTask(registryDeadline(t), "t_pending")
	require.NoError(t, err)
	assert.Equal(t, "failed", pending.Status)
	assert.Equal(t, "canceled", pending.Config["_error"])
	// Completed task untouched
	done, err := store.GetTask(registryDeadline(t), "t_done")
	require.NoError(t, err)
	assert.Equal(t, "completed", done.Status)
}

func TestCancelProvisioningTaskUI_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest("POST", "/api/v1/provisioning/tasks/nope/cancel", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nope"})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CancelProvisioningTaskUI(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBulkProvisionUI(t *testing.T) {
	h, db := newTestHandler(t)

	d1 := seedDevice(t, db, "10.0.0.1", "AA:BB:CC:DD:EE:01", "dev1")
	d2 := seedDevice(t, db, "10.0.0.2", "AA:BB:CC:DD:EE:02", "dev2")

	body, _ := json.Marshal(map[string]interface{}{
		"deviceIds": []string{
			strconv.FormatUint(uint64(d1.ID), 10),
			strconv.FormatUint(uint64(d2.ID), 10),
		},
		"config": map[string]interface{}{"ssid": "HomeNet"},
	})
	req := httptest.NewRequest("POST", "/api/v1/provisioning/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.BulkProvisionUI(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var env struct {
		Data struct {
			Tasks []uiProvisioningTask `json:"tasks"`
			Count int                  `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, 2, env.Data.Count)
	assert.Len(t, env.Data.Tasks, 2)
}

func TestBulkProvisionUI_EmptyIDs(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest("POST", "/api/v1/provisioning/bulk",
		bytes.NewReader([]byte(`{"deviceIds":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.BulkProvisionUI(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListProvisioningAgentsUI_FieldTranslation(t *testing.T) {
	h, db := newTestHandler(t)

	now := time.Now()
	require.NoError(t, db.GetDB().Create(&database.ProvisionerAgent{
		AgentID: "a1", TokenHash: database.HashAgentToken("t1"), Hostname: "alpha", IP: "10.0.0.5", Version: "v1.2.3",
		Capabilities: []string{"wifi"}, Status: "online", LastSeen: now, RegisteredAt: now,
	}).Error)
	require.NoError(t, db.GetDB().Create(&database.ProvisionerAgent{
		AgentID: "a2", TokenHash: database.HashAgentToken("t2"), Hostname: "bravo", Status: "online",
		LastSeen: now.Add(-10 * time.Minute), RegisteredAt: now.Add(-time.Hour),
	}).Error)

	req := httptest.NewRequest("GET", "/api/v1/provisioning/agents", nil)
	w := httptest.NewRecorder()
	h.ListProvisioningAgentsUI(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var env struct {
		Data struct {
			Agents []uiProvisioningAgent `json:"agents"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Len(t, env.Data.Agents, 2)

	byID := map[string]uiProvisioningAgent{}
	for _, a := range env.Data.Agents {
		byID[a.ID] = a
	}
	assert.Equal(t, "alpha", byID["a1"].Name)
	assert.Equal(t, "online", byID["a1"].Status)
	assert.Equal(t, "offline", byID["a2"].Status) // derived from stale LastSeen
}

func TestGetProvisioningAgentStatusUI(t *testing.T) {
	h, db := newTestHandler(t)

	seedProvisionerAgent(t, db, "a1")

	req := httptest.NewRequest("GET", "/api/v1/provisioning/agents/a1/status", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "a1"})
	w := httptest.NewRecorder()
	h.GetProvisioningAgentStatusUI(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest("GET", "/api/v1/provisioning/agents/missing/status", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "missing"})
	w = httptest.NewRecorder()
	h.GetProvisioningAgentStatusUI(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestProvisioningUI_RequiresAdminWhenKeyConfigured(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetAdminAPIKey("secret")
	t.Cleanup(func() { h.SetAdminAPIKey("") })

	req := httptest.NewRequest("GET", "/api/v1/provisioning/tasks", nil)
	w := httptest.NewRecorder()
	h.ListProvisioningTasksUI(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// With valid bearer
	req = httptest.NewRequest("GET", "/api/v1/provisioning/tasks", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	h.ListProvisioningTasksUI(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMapInternalToUIStatus(t *testing.T) {
	cases := map[string]string{
		"pending":     "pending",
		"assigned":    "running",
		"in_progress": "running",
		"completed":   "completed",
		"failed":      "failed",
		"other":       "other",
	}
	for in, want := range cases {
		assert.Equal(t, want, mapInternalToUIStatus(in), "input=%s", in)
	}
}

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A report from an agent stores its devices, marks the agent seen and completes the task it names, all from the
// rows in the registry tables.
func TestReportDiscoveredDevices_StoresDevicesAndCompletesTheNamedTask(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	handler := &Handler{DB: db, logger: logging.GetDefault()}
	t.Cleanup(handler.WaitBackground) // runs before the database cleanup above

	seedProvisionerAgent(t, db, "test-agent-1")
	old := time.Now().Add(-time.Hour)
	require.NoError(t, db.GetDB().Model(&database.ProvisionerAgent{}).Where("agent_id = ?", "test-agent-1").Update("last_seen", old).Error)
	seedProvisionerTask(t, db, database.ProvisioningTask{
		TaskID: "test-task-1", Type: "discover_devices", Status: "in_progress", AgentID: "test-agent-1",
		CreatedAt: old, UpdatedAt: old,
	})

	body, err := json.Marshal(map[string]interface{}{
		"agent_id": "test-agent-1",
		"task_id":  "test-task-1",
		"devices": []map[string]interface{}{
			{"mac": "AA:BB:CC:DD:EE:FF", "ssid": "shelly1-AABBCC", "model": "SHSW-1", "generation": 1, "ip": "192.168.33.1", "signal": -45, "discovered": time.Now().Format(time.RFC3339)},
			{"mac": "11:22:33:44:55:66", "ssid": "shellyplus1-112233", "model": "SNSW-001X16EU", "generation": 2, "ip": "192.168.33.2", "signal": -52, "discovered": time.Now().Format(time.RFC3339)},
		},
	})
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.ReportDiscoveredDevices(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var out struct {
		Data struct {
			Received  int `json:"devices_received"`
			Processed int `json:"devices_processed"`
			Persisted int `json:"devices_persisted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, 2, out.Data.Received)
	assert.Equal(t, 2, out.Data.Processed)
	assert.Equal(t, 2, out.Data.Persisted)

	devices, err := db.GetDiscoveredDevices("test-agent-1")
	require.NoError(t, err)
	assert.Len(t, devices, 2, "both devices are filed under the reporting agent")

	store := database.NewProvisionerStore(db.GetDB())
	task, err := store.GetTask(registryDeadline(t), "test-task-1")
	require.NoError(t, err)
	assert.Equal(t, "completed", task.Status)
	agent, err := store.GetAgent(registryDeadline(t), "test-agent-1")
	require.NoError(t, err)
	assert.True(t, agent.LastSeen.After(old), "the report marks the agent seen")
}

// The report route answers the same four ways it did when the registry was in memory.
func TestReportDiscoveredDevices_Refusals(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	handler := &Handler{DB: db, logger: logging.GetDefault()}
	t.Cleanup(handler.WaitBackground)
	seedProvisionerAgent(t, db, "known")

	device := `{"mac":"AA:BB:CC:DD:EE:FF","ssid":"s","model":"SHSW-1","generation":1,"ip":"192.168.33.1","signal":-45,"discovered":"2026-11-19T00:00:00Z"}`
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"no agent id", `{"devices":[` + device + `]}`, http.StatusBadRequest},
		{"no devices", `{"agent_id":"known","devices":[]}`, http.StatusBadRequest},
		{"an agent that never registered", `{"agent_id":"stranger","devices":[` + device + `]}`, http.StatusUnauthorized},
		{"not JSON", `nope`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewReader([]byte(tc.body)))
			w := httptest.NewRecorder()
			handler.ReportDiscoveredDevices(w, req)
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

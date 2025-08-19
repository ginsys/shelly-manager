package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestReportDiscoveredDevicesDatabase(t *testing.T) {
	// Setup test database
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	dbManager, err := database.NewManagerWithLogger(":memory:", logger)
	require.NoError(t, err)
	defer dbManager.Close()

	// Create handler
	handler := &Handler{DB: dbManager, logger: logger}

	t.Run("ValidDiscoveryReport", func(t *testing.T) {
		// First register the agent
		registry.mu.Lock()
		registry.agents["test-agent-1"] = &ProvisionerAgent{
			ID:           "test-agent-1",
			Hostname:     "test-host",
			IP:           "127.0.0.1",
			Status:       "active",
			LastSeen:     time.Now(),
			RegisteredAt: time.Now(),
		}
		registry.mu.Unlock()

		requestBody := map[string]interface{}{
			"agent_id": "test-agent-1",
			"task_id":  "task-123",
			"devices": []map[string]interface{}{
				{
					"mac":        "aa:bb:cc:dd:ee:ff",
					"ssid":       "shellyplus1-112233",
					"model":      "SNSW-001X16EU",
					"generation": 2,
					"ip":         "192.168.33.1",
					"signal":     -45,
					"discovered": time.Now().Format(time.RFC3339),
				},
				{
					"mac":        "11:22:33:44:55:66",
					"ssid":       "shelly1-aabbcc",
					"model":      "SHSW-1",
					"generation": 1,
					"ip":         "192.168.33.2",
					"signal":     -55,
					"discovered": time.Now().Format(time.RFC3339),
				},
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ReportDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(2), response["devices_received"].(float64))
		assert.Equal(t, float64(2), response["devices_processed"].(float64))
		assert.Equal(t, float64(2), response["devices_persisted"].(float64))

		// Verify devices were persisted in database
		devices, err := dbManager.GetDiscoveredDevices("test-agent-1")
		assert.NoError(t, err)
		assert.Len(t, devices, 2)

		// Verify device details
		assert.Contains(t, []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"}, devices[0].MAC)
		assert.Contains(t, []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"}, devices[1].MAC)
		assert.Equal(t, "test-agent-1", devices[0].AgentID)
		assert.Equal(t, "task-123", devices[0].TaskID)
	})

	t.Run("UnregisteredAgent", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"agent_id": "unknown-agent",
			"devices": []map[string]interface{}{
				{
					"mac":        "test:device:mac",
					"discovered": time.Now().Format(time.RFC3339),
				},
			},
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ReportDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ReportDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("MissingAgentID", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"devices": []map[string]interface{}{
				{
					"mac":        "test:device:mac",
					"discovered": time.Now().Format(time.RFC3339),
				},
			},
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ReportDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("EmptyDevicesArray", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"agent_id": "test-agent",
			"devices":  []map[string]interface{}{},
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ReportDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// Clean up registry
	t.Cleanup(func() {
		registry.mu.Lock()
		delete(registry.agents, "test-agent-1")
		registry.mu.Unlock()
	})
}

func TestGetDiscoveredDevicesDatabase(t *testing.T) {
	// Setup test database
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	dbManager, err := database.NewManagerWithLogger(":memory:", logger)
	require.NoError(t, err)
	defer dbManager.Close()

	// Create handler
	handler := &Handler{DB: dbManager, logger: logger}

	// Seed test data
	testDevices := []*database.DiscoveredDevice{
		{
			MAC:        "aa:bb:cc:dd:ee:ff",
			SSID:       "shellyplus1-112233",
			Model:      "SNSW-001X16EU",
			Generation: 2,
			IP:         "192.168.33.1",
			Signal:     -45,
			AgentID:    "agent-1",
			TaskID:     "task-123",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		},
		{
			MAC:        "11:22:33:44:55:66",
			SSID:       "shelly1-aabbcc",
			Model:      "SHSW-1",
			Generation: 1,
			IP:         "192.168.33.2",
			Signal:     -55,
			AgentID:    "agent-2",
			TaskID:     "task-456",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		},
		{
			MAC:        "99:88:77:66:55:44",
			SSID:       "shellyplus1-ddeeff",
			Model:      "SNSW-001X16EU",
			Generation: 2,
			IP:         "192.168.33.3",
			Signal:     -40,
			AgentID:    "agent-1",
			TaskID:     "task-789",
			Discovered: time.Now(),
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		},
	}

	for _, device := range testDevices {
		err := dbManager.AddDiscoveredDevice(device)
		require.NoError(t, err)
	}

	t.Run("GetAllDiscoveredDevices", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/provisioner/discovered-devices", nil)
		w := httptest.NewRecorder()

		handler.GetDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(3), response["device_count"].(float64))
		assert.Equal(t, "", response["filtered_by"].(string))

		devices := response["devices"].([]interface{})
		assert.Len(t, devices, 3)
	})

	t.Run("GetDiscoveredDevicesFilteredByAgent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/provisioner/discovered-devices?agent_id=agent-1", nil)
		w := httptest.NewRecorder()

		handler.GetDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(2), response["device_count"].(float64))
		assert.Equal(t, "agent-1", response["filtered_by"].(string))

		devices := response["devices"].([]interface{})
		assert.Len(t, devices, 2)

		// Verify all devices belong to agent-1
		for _, device := range devices {
			deviceMap := device.(map[string]interface{})
			assert.Equal(t, "agent-1", deviceMap["agent_id"])
		}
	})

	t.Run("GetDiscoveredDevicesEmptyResult", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/provisioner/discovered-devices?agent_id=non-existent-agent", nil)
		w := httptest.NewRecorder()

		handler.GetDiscoveredDevices(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(0), response["device_count"].(float64))
		assert.Equal(t, "non-existent-agent", response["filtered_by"].(string))

		devices := response["devices"].([]interface{})
		assert.Len(t, devices, 0)
	})
}

func TestDiscoveredDevicesDatabaseIntegration(t *testing.T) {
	// Setup test database
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	dbManager, err := database.NewManagerWithLogger(":memory:", logger)
	require.NoError(t, err)
	defer dbManager.Close()

	// Create handler and router
	handler := &Handler{DB: dbManager, logger: logger}
	router := mux.NewRouter()
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/provisioner/discovered-devices", handler.ReportDiscoveredDevices).Methods("POST")
	api.HandleFunc("/provisioner/discovered-devices", handler.GetDiscoveredDevices).Methods("GET")

	// Register agent
	registry.mu.Lock()
	registry.agents["integration-agent"] = &ProvisionerAgent{
		ID:           "integration-agent",
		Hostname:     "integration-host",
		IP:           "127.0.0.1",
		Status:       "active",
		LastSeen:     time.Now(),
		RegisteredAt: time.Now(),
	}
	registry.mu.Unlock()

	t.Run("FullWorkflow", func(t *testing.T) {
		// Step 1: Report discovered devices
		reportBody := map[string]interface{}{
			"agent_id": "integration-agent",
			"task_id":  "integration-task",
			"devices": []map[string]interface{}{
				{
					"mac":        "integration:device:1",
					"ssid":       "shellyplus1-integration",
					"model":      "SNSW-001X16EU",
					"generation": 2,
					"ip":         "192.168.33.10",
					"signal":     -50,
					"discovered": time.Now().Format(time.RFC3339),
				},
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}

		body, _ := json.Marshal(reportBody)
		req := httptest.NewRequest("POST", "/api/v1/provisioner/discovered-devices", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Step 2: Retrieve discovered devices
		req = httptest.NewRequest("GET", "/api/v1/provisioner/discovered-devices?agent_id=integration-agent", nil)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(1), response["device_count"].(float64))

		devices := response["devices"].([]interface{})
		assert.Len(t, devices, 1)

		device := devices[0].(map[string]interface{})
		assert.Equal(t, "integration:device:1", device["mac"])
		assert.Equal(t, "integration-agent", device["agent_id"])
		assert.Equal(t, "integration-task", device["task_id"])
	})

	// Clean up registry
	t.Cleanup(func() {
		registry.mu.Lock()
		delete(registry.agents, "integration-agent")
		registry.mu.Unlock()
	})
}

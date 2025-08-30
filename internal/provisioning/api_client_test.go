package provisioning

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func TestAPIClient(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	if err != nil {
		t.Fatal("Failed to create logger:", err)
	}

	t.Run("NewAPIClient", func(t *testing.T) {
		client := NewAPIClient("http://localhost:8080", "test-key", "agent-1", logger)

		assert.NotNil(t, client)
		assert.Equal(t, "http://localhost:8080", client.baseURL)
		assert.Equal(t, "test-key", client.apiKey)
		assert.Equal(t, "agent-1", client.agentID)
		assert.Equal(t, 30*time.Second, client.client.Timeout)
	})

	t.Run("RegisterAgent_Success", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/provisioner/agents/register", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			// Decode and validate request body
			var agent struct {
				ID           string            `json:"id"`
				Hostname     string            `json:"hostname"`
				IP           string            `json:"ip"`
				Version      string            `json:"version"`
				Capabilities []string          `json:"capabilities"`
				Status       string            `json:"status"`
				Metadata     map[string]string `json:"metadata"`
			}

			err := json.NewDecoder(r.Body).Decode(&agent)
			require.NoError(t, err)

			assert.Equal(t, "test-agent", agent.ID)
			assert.Equal(t, "test-host", agent.Hostname)
			// Note: status is not part of the registration request

			// Send success response
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"success":       true,
				"agent_id":      "test-agent",
				"registered_at": time.Now(),
				"status":        "registered",
				"message":       "Agent registered successfully",
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		err := client.RegisterAgent("test-host", []string{"wifi"}, map[string]string{"region": "us"})
		assert.NoError(t, err)
	})

	t.Run("RegisterAgent_ServerError", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte("Internal server error")); err != nil {
				t.Logf("Failed to write response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		err := client.RegisterAgent("test-host", []string{"wifi"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("PollTasks_Success", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		expectedTasks := []ProvisioningTask{
			{
				ID:     "task-1",
				Type:   "device_provisioning",
				Status: "pending",
				Config: map[string]interface{}{
					"device_mac": "AA:BB:CC:DD:EE:FF",
					"ssid":       "HomeWiFi",
				},
			},
			{
				ID:     "task-2",
				Type:   "network_scan",
				Status: "pending",
				Config: map[string]interface{}{"timeout": 60},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/provisioner/agents/test-agent/tasks", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"tasks": expectedTasks,
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		// Need to register first
		client.registered = true
		tasks, err := client.PollTasks()
		assert.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Equal(t, "task-1", tasks[0].ID)
		assert.Equal(t, "device_provisioning", tasks[0].Type)
		assert.Equal(t, "task-2", tasks[1].ID)
		assert.Equal(t, "network_scan", tasks[1].Type)
	})

	t.Run("PollTasks_EmptyResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"tasks": []interface{}{},
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		// Need to register first
		client.registered = true
		tasks, err := client.PollTasks()
		assert.NoError(t, err)
		assert.Len(t, tasks, 0)
	})

	t.Run("UpdateTaskStatus_Success", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			assert.Equal(t, "/api/v1/provisioner/tasks/task-123/status", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			// Decode and validate request body
			var statusUpdate struct {
				Status string                 `json:"status"`
				Result map[string]interface{} `json:"result"`
				Error  string                 `json:"error"`
			}

			err := json.NewDecoder(r.Body).Decode(&statusUpdate)
			require.NoError(t, err)

			assert.Equal(t, "completed", statusUpdate.Status)
			assert.Equal(t, "Device provisioned successfully", statusUpdate.Result["message"])

			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "updated"}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		// Need to register first
		client.registered = true
		err := client.UpdateTaskStatus("task-123", "completed", map[string]interface{}{"message": "Device provisioned successfully"}, "")
		assert.NoError(t, err)
	})

	t.Run("UpdateTaskStatus_TaskNotFound", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte("Task not found")); err != nil {
				t.Logf("Failed to write response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		// Need to register first
		client.registered = true
		err := client.UpdateTaskStatus("non-existent", "completed", map[string]interface{}{}, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("TestConnectivity_Success", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/provisioner/health", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		err := client.TestConnectivity()
		assert.NoError(t, err)
	})

	t.Run("TestConnectivity_ServerDown", func(t *testing.T) {
		client := NewAPIClient("http://localhost:99999", "test-key", "test-agent", logger)

		err := client.TestConnectivity()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("SlowServer", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		err := client.TestConnectivity()
		assert.NoError(t, err) // Should succeed with default timeout
	})

	t.Run("Invalid_BaseURL", func(t *testing.T) {
		client := NewAPIClient("not-a-valid-url", "test-key", "test-agent", logger)

		err := client.TestConnectivity()
		assert.Error(t, err)
	})

	t.Run("Authentication_Missing", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for missing or invalid auth
			auth := r.Header.Get("Authorization")
			if auth != "Bearer valid-key" {
				w.WriteHeader(http.StatusUnauthorized)
				if _, err := w.Write([]byte("Unauthorized")); err != nil {
					t.Logf("Failed to write response: %v", err)
				}
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Test with invalid key
		client := NewAPIClient(server.URL, "invalid-key", "test-agent", logger)

		err := client.TestConnectivity()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("ReportDiscoveredDevices_Success", func(t *testing.T) {
		testutil.SkipIfNoSocketPermissions(t)
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/provisioner/discovered-devices", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			// Decode and validate request body
			var req DeviceDiscoveryRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			assert.Equal(t, "agent-1", req.AgentID)
			assert.Equal(t, "task-123", req.TaskID)
			assert.Len(t, req.Devices, 2)

			// Validate first device
			assert.Equal(t, "AA:BB:CC:DD:EE:FF", req.Devices[0].MAC)
			assert.Equal(t, "shelly1-AABBCC", req.Devices[0].SSID)
			assert.Equal(t, "SHSW-1", req.Devices[0].Model)
			assert.Equal(t, 1, req.Devices[0].Generation)
			assert.Equal(t, "192.168.33.1", req.Devices[0].IP)
			assert.Equal(t, -45, req.Devices[0].Signal)

			// Return success response
			response := DeviceDiscoveryResponse{
				Success:          true,
				DevicesReceived:  2,
				DevicesProcessed: 2,
				Timestamp:        time.Now(),
				Message:          "Successfully processed devices",
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}))
		defer server.Close()

		// Create client and register it first
		client := NewAPIClient(server.URL, "test-key", "agent-1", logger)
		client.registered = true // Mock registration

		// Create test devices
		devices := []*DiscoveredDevice{
			{
				MAC:        "AA:BB:CC:DD:EE:FF",
				SSID:       "shelly1-AABBCC",
				Model:      "SHSW-1",
				Generation: 1,
				IP:         "192.168.33.1",
				Signal:     -45,
				Discovered: time.Now(),
			},
			{
				MAC:        "11:22:33:44:55:66",
				SSID:       "shellyplus1-112233",
				Model:      "SNSW-001X16EU",
				Generation: 2,
				IP:         "192.168.33.1",
				Signal:     -52,
				Discovered: time.Now(),
			},
		}

		// Test reporting discovered devices
		err := client.ReportDiscoveredDevices("task-123", devices)
		assert.NoError(t, err)
	})

	t.Run("ReportDiscoveredDevices_NotRegistered", func(t *testing.T) {
		client := NewAPIClient("http://localhost:8080", "test-key", "agent-1", logger)

		devices := []*DiscoveredDevice{
			{
				MAC:        "AA:BB:CC:DD:EE:FF",
				SSID:       "shelly1-AABBCC",
				Model:      "SHSW-1",
				Generation: 1,
				IP:         "192.168.33.1",
				Signal:     -45,
				Discovered: time.Now(),
			},
		}

		err := client.ReportDiscoveredDevices("task-123", devices)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent not registered")
	})
}

func TestProvisioningTask_JSONSerialization(t *testing.T) {
	t.Run("JSON_Serialization", func(t *testing.T) {
		task := ProvisioningTask{
			ID:     "test-task",
			Type:   "device_provisioning",
			Status: "pending",
			Config: map[string]interface{}{
				"device_mac": "AA:BB:CC:DD:EE:FF",
				"ssid":       "TestNetwork",
				"timeout":    300,
			},
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(task)
		require.NoError(t, err)

		// Unmarshal back
		var unmarshaled ProvisioningTask
		err = json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, task.ID, unmarshaled.ID)
		assert.Equal(t, task.Type, unmarshaled.Type)
		assert.Equal(t, task.Status, unmarshaled.Status)
		assert.Equal(t, task.Config["device_mac"], unmarshaled.Config["device_mac"])
		assert.Equal(t, task.Config["ssid"], unmarshaled.Config["ssid"])
		// JSON unmarshaling converts numbers to float64
		assert.Equal(t, float64(300), unmarshaled.Config["timeout"])
	})

}

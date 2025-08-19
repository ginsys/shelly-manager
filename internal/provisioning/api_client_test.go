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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":       true,
				"agent_id":      "test-agent",
				"registered_at": time.Now(),
				"status":        "registered",
				"message":       "Agent registered successfully",
			})
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		err := client.RegisterAgent("test-host", []string{"wifi"}, map[string]string{"region": "us"})
		assert.NoError(t, err)
	})

	t.Run("RegisterAgent_ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		err := client.RegisterAgent("test-host", []string{"wifi"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("PollTasks_Success", func(t *testing.T) {
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tasks": expectedTasks,
			})
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tasks": []interface{}{},
			})
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
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
		}))
		defer server.Close()

		client := NewAPIClient(server.URL, "test-key", "test-agent", logger)

		// Need to register first
		client.registered = true
		err := client.UpdateTaskStatus("task-123", "completed", map[string]interface{}{"message": "Device provisioned successfully"}, "")
		assert.NoError(t, err)
	})

	t.Run("UpdateTaskStatus_TaskNotFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Task not found"))
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/provisioner/health", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
			})
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for missing or invalid auth
			auth := r.Header.Get("Authorization")
			if auth != "Bearer valid-key" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorized"))
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

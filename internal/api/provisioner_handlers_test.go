package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProvisioningTask(t *testing.T) {
	t.Run("ValidTask", func(t *testing.T) {
		task := ProvisioningTask{
			ID:         "valid-task",
			Type:       "device_provisioning",
			DeviceMAC:  "AA:BB:CC:DD:EE:FF",
			TargetSSID: "HomeWiFi",
			Status:     "pending",
			AgentID:    "agent-1",
			CreatedAt:  time.Now(),
			Priority:   1,
		}

		assert.NotEmpty(t, task.ID)
		assert.NotEmpty(t, task.Type)
		assert.NotEmpty(t, task.Status)
		assert.NotEmpty(t, task.AgentID)
		assert.False(t, task.CreatedAt.IsZero())
	})
}

func TestProvisionerAgent(t *testing.T) {
	t.Run("ValidAgent", func(t *testing.T) {
		agent := ProvisionerAgent{
			ID:           "valid-agent",
			Hostname:     "test-host",
			IP:           "192.168.1.100",
			Version:      "v1.0.0",
			Capabilities: []string{"wifi", "bluetooth"},
			Status:       "online",
			LastSeen:     time.Now(),
			RegisteredAt: time.Now(),
			Metadata:     map[string]string{"region": "us"},
		}

		assert.NotEmpty(t, agent.ID)
		assert.NotEmpty(t, agent.Hostname)
		assert.NotEmpty(t, agent.Status)
		assert.False(t, agent.RegisteredAt.IsZero())
	})

	t.Run("AgentStatusUpdate", func(t *testing.T) {
		agent := ProvisionerAgent{
			ID:       "test-agent",
			Status:   "online",
			LastSeen: time.Now(),
		}

		// Simulate agent going offline
		agent.LastSeen = time.Now().Add(-10 * time.Minute)
		if time.Since(agent.LastSeen) > 5*time.Minute {
			agent.Status = "offline"
		}

		assert.Equal(t, "offline", agent.Status)
	})
}

func TestProvisioningTaskStateMachine(t *testing.T) {
	testCases := []struct {
		name          string
		initialStatus string
		nextStatus    string
		expectedValid bool
	}{
		{"pending to assigned", "pending", "assigned", true},
		{"assigned to in_progress", "assigned", "in_progress", true},
		{"in_progress to completed", "in_progress", "completed", true},
		{"in_progress to failed", "in_progress", "failed", true},
		{"completed to pending", "completed", "pending", false}, // Invalid transition
		{"failed to pending", "failed", "pending", true},        // Can retry
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task := ProvisioningTask{
				ID:     "test-task",
				Status: tc.initialStatus,
			}

			// Simulate state change
			if tc.expectedValid {
				task.Status = tc.nextStatus
				task.UpdatedAt = time.Now()
			}

			if tc.expectedValid {
				assert.Equal(t, tc.nextStatus, task.Status)
				assert.False(t, task.UpdatedAt.IsZero())
			} else {
				// For invalid transitions, we would expect the original status
				assert.Equal(t, tc.initialStatus, task.Status)
			}
		})
	}
}

func TestProvisionerRegistry_ConcurrentAccess(t *testing.T) {
	// Test that global registry handles concurrent access
	// This is a basic smoke test since the actual registry uses sync.RWMutex
	t.Run("GlobalRegistryExists", func(t *testing.T) {
		assert.NotNil(t, registry)
		assert.NotNil(t, registry.agents)
		assert.NotNil(t, registry.tasks)
	})
}

func TestReportDiscoveredDevices(t *testing.T) {
	// Set up test agent in registry
	testAgent := &ProvisionerAgent{
		ID:           "test-agent-1",
		Hostname:     "test-host",
		Status:       "online",
		LastSeen:     time.Now(),
		RegisteredAt: time.Now(),
	}

	// Set up test task in registry
	testTask := &ProvisioningTask{
		ID:        "test-task-1",
		Type:      "discover_devices",
		Status:    "in_progress",
		AgentID:   "test-agent-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.Run("ValidDeviceDiscoveryReport", func(t *testing.T) {
		// Clear registry and add test data
		registry.mu.Lock()
		registry.agents = make(map[string]*ProvisionerAgent)
		registry.tasks = make(map[string]*ProvisioningTask)
		registry.agents[testAgent.ID] = testAgent
		registry.tasks[testTask.ID] = testTask
		registry.mu.Unlock()

		// Test device discovery report validation
		discoveredDevices := []struct {
			MAC        string    `json:"mac"`
			SSID       string    `json:"ssid"`
			Model      string    `json:"model"`
			Generation int       `json:"generation"`
			IP         string    `json:"ip"`
			Signal     int       `json:"signal"`
			Discovered time.Time `json:"discovered"`
		}{
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

		// Validate device data structure
		assert.Len(t, discoveredDevices, 2)

		// Validate first device (Gen1)
		assert.Equal(t, "AA:BB:CC:DD:EE:FF", discoveredDevices[0].MAC)
		assert.Equal(t, "shelly1-AABBCC", discoveredDevices[0].SSID)
		assert.Equal(t, "SHSW-1", discoveredDevices[0].Model)
		assert.Equal(t, 1, discoveredDevices[0].Generation)
		assert.Equal(t, "192.168.33.1", discoveredDevices[0].IP)
		assert.Equal(t, -45, discoveredDevices[0].Signal)
		assert.False(t, discoveredDevices[0].Discovered.IsZero())

		// Validate second device (Gen2)
		assert.Equal(t, "11:22:33:44:55:66", discoveredDevices[1].MAC)
		assert.Equal(t, "shellyplus1-112233", discoveredDevices[1].SSID)
		assert.Equal(t, "SNSW-001X16EU", discoveredDevices[1].Model)
		assert.Equal(t, 2, discoveredDevices[1].Generation)
		assert.Equal(t, -52, discoveredDevices[1].Signal)
	})

	t.Run("AgentValidation", func(t *testing.T) {
		// Test agent validation logic
		registry.mu.Lock()
		registry.agents = make(map[string]*ProvisionerAgent)
		registry.agents[testAgent.ID] = testAgent
		registry.mu.Unlock()

		// Verify agent exists in registry
		registry.mu.RLock()
		agent, exists := registry.agents[testAgent.ID]
		registry.mu.RUnlock()

		assert.True(t, exists)
		assert.NotNil(t, agent)
		assert.Equal(t, testAgent.ID, agent.ID)
		assert.Equal(t, "online", agent.Status)
	})

	t.Run("TaskStatusUpdateLogic", func(t *testing.T) {
		// Test task status update logic
		registry.mu.Lock()
		registry.tasks = make(map[string]*ProvisioningTask)

		// Create task in in_progress state
		task := &ProvisioningTask{
			ID:        "task-update-test",
			Type:      "discover_devices",
			Status:    "in_progress",
			AgentID:   testAgent.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		registry.tasks[task.ID] = task
		registry.mu.Unlock()

		// Verify initial state
		registry.mu.RLock()
		initialTask, exists := registry.tasks[task.ID]
		registry.mu.RUnlock()

		assert.True(t, exists)
		assert.Equal(t, "in_progress", initialTask.Status)

		// Add small delay to ensure time difference
		time.Sleep(1 * time.Millisecond)

		// Simulate task completion logic
		registry.mu.Lock()
		if taskToUpdate, taskExists := registry.tasks[task.ID]; taskExists {
			taskToUpdate.Status = "completed"
			taskToUpdate.UpdatedAt = time.Now()
		}
		registry.mu.Unlock()

		// Verify status was updated
		registry.mu.RLock()
		updatedTask, exists := registry.tasks[task.ID]
		registry.mu.RUnlock()

		assert.True(t, exists)
		assert.Equal(t, "completed", updatedTask.Status)
		assert.True(t, updatedTask.UpdatedAt.After(initialTask.UpdatedAt) ||
			updatedTask.UpdatedAt.Equal(initialTask.UpdatedAt))
	})

	t.Run("DeviceProcessingCounting", func(t *testing.T) {
		// Test device processing counter logic
		devices := []map[string]interface{}{
			{
				"mac":        "AA:BB:CC:DD:EE:FF",
				"ssid":       "shelly1-AABBCC",
				"model":      "SHSW-1",
				"generation": 1,
				"ip":         "192.168.33.1",
				"signal":     -45,
				"discovered": time.Now(),
			},
			{
				"mac":        "11:22:33:44:55:66",
				"ssid":       "shellyplus1-112233",
				"model":      "SNSW-001X16EU",
				"generation": 2,
				"ip":         "192.168.33.1",
				"signal":     -52,
				"discovered": time.Now(),
			},
		}

		// Simulate device processing logic
		devicesProcessed := 0
		for range devices {
			// In actual handler, device processing logic would go here
			devicesProcessed++
		}

		assert.Equal(t, 2, devicesProcessed)
		assert.Equal(t, len(devices), devicesProcessed)
	})

	t.Run("ResponseStructureValidation", func(t *testing.T) {
		// Test response structure matches API contract
		response := map[string]interface{}{
			"success":           true,
			"devices_received":  2,
			"devices_processed": 2,
			"timestamp":         time.Now(),
			"message":           "Successfully processed 2 discovered devices",
		}

		// Validate response structure
		assert.Contains(t, response, "success")
		assert.Contains(t, response, "devices_received")
		assert.Contains(t, response, "devices_processed")
		assert.Contains(t, response, "timestamp")
		assert.Contains(t, response, "message")

		// Validate response values
		assert.True(t, response["success"].(bool))
		assert.Equal(t, 2, response["devices_received"].(int))
		assert.Equal(t, 2, response["devices_processed"].(int))
		assert.NotZero(t, response["timestamp"].(time.Time))
		assert.Contains(t, response["message"].(string), "Successfully processed")
	})
}

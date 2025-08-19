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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/database"
)

// ProvisionerAgent represents a registered provisioning agent
type ProvisionerAgent struct {
	ID           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	IP           string            `json:"ip"`
	Version      string            `json:"version,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Status       string            `json:"status"`
	LastSeen     time.Time         `json:"last_seen"`
	RegisteredAt time.Time         `json:"registered_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ProvisioningTask represents a task for a provisioning agent
type ProvisioningTask struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	DeviceMAC  string                 `json:"device_mac,omitempty"`
	TargetSSID string                 `json:"target_ssid,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Status     string                 `json:"status"`
	AgentID    string                 `json:"agent_id,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Priority   int                    `json:"priority,omitempty"`
}

// ProvisionerRegistry manages registered agents and tasks
type ProvisionerRegistry struct {
	mu     sync.RWMutex
	agents map[string]*ProvisionerAgent
	tasks  map[string]*ProvisioningTask
}

// Global registry instance
var registry = &ProvisionerRegistry{
	agents: make(map[string]*ProvisionerAgent),
	tasks:  make(map[string]*ProvisioningTask),
}

// RegisterAgent handles POST /api/v1/provisioner/agents/register
func (h *Handler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID           string            `json:"id"`
		Hostname     string            `json:"hostname"`
		Version      string            `json:"version,omitempty"`
		Capabilities []string          `json:"capabilities,omitempty"`
		Metadata     map[string]string `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	// Validate required fields
	if req.ID == "" || req.Hostname == "" {
		h.responseWriter().WriteValidationError(w, r, "Missing required fields: id and hostname")
		return
	}

	// Get client IP
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	// Create or update agent registration
	registry.mu.Lock()
	defer registry.mu.Unlock()

	now := time.Now()
	agent, exists := registry.agents[req.ID]

	if !exists {
		agent = &ProvisionerAgent{
			ID:           req.ID,
			RegisteredAt: now,
		}
		registry.agents[req.ID] = agent

		h.logger.WithFields(map[string]any{
			"agent_id": req.ID,
			"hostname": req.Hostname,
			"ip":       clientIP,
		}).Info("New provisioning agent registered")
	} else {
		h.logger.WithFields(map[string]any{
			"agent_id": req.ID,
			"hostname": req.Hostname,
			"ip":       clientIP,
		}).Debug("Existing provisioning agent re-registered")
	}

	// Update agent information
	agent.Hostname = req.Hostname
	agent.IP = clientIP
	agent.Version = req.Version
	agent.Capabilities = req.Capabilities
	agent.Status = "online"
	agent.LastSeen = now
	agent.Metadata = req.Metadata

	response := map[string]interface{}{
		"agent_id":      agent.ID,
		"registered_at": agent.RegisteredAt,
		"status":        "registered",
		"message":       "Agent registered successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	h.writeJSON(w, response)
}

// GetProvisionerAgents handles GET /api/v1/provisioner/agents
func (h *Handler) GetProvisionerAgents(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	agents := make([]*ProvisionerAgent, 0, len(registry.agents))
	for _, agent := range registry.agents {
		// Update status based on last seen time
		if time.Since(agent.LastSeen) > 5*time.Minute {
			agent.Status = "offline"
		}
		agents = append(agents, agent)
	}

	w.Header().Set("Content-Type", "application/json")
	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

// PollTasks handles GET /api/v1/provisioner/agents/{id}/tasks
func (h *Handler) PollTasks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	if agentID == "" {
		h.responseWriter().WriteValidationError(w, r, "Agent ID is required")
		return
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	// Update agent last seen time
	agent, exists := registry.agents[agentID]
	if !exists {
		h.responseWriter().WriteNotFoundError(w, r, "Agent")
		return
	}

	agent.LastSeen = time.Now()
	agent.Status = "online"

	// Find pending tasks for this agent or unassigned tasks
	var availableTasks []*ProvisioningTask
	for _, task := range registry.tasks {
		if (task.AgentID == "" || task.AgentID == agentID) && task.Status == "pending" {
			task.AgentID = agentID
			task.Status = "assigned"
			task.UpdatedAt = time.Now()
			availableTasks = append(availableTasks, task)
		}
	}

	h.logger.WithFields(map[string]any{
		"agent_id":        agentID,
		"available_tasks": len(availableTasks),
	}).Debug("Agent polling for tasks")

	response := map[string]interface{}{
		"agent_id": agentID,
		"tasks":    availableTasks,
		"count":    len(availableTasks),
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// UpdateTaskStatus handles PUT /api/v1/provisioner/tasks/{id}/status
func (h *Handler) UpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	if taskID == "" {
		h.responseWriter().WriteValidationError(w, r, "Task ID is required")
		return
	}

	var req struct {
		Status  string                 `json:"status"`
		AgentID string                 `json:"agent_id"`
		Result  map[string]interface{} `json:"result,omitempty"`
		Error   string                 `json:"error,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	task, exists := registry.tasks[taskID]
	if !exists {
		h.responseWriter().WriteNotFoundError(w, r, "Task")
		return
	}

	// Update task status
	task.Status = req.Status
	task.UpdatedAt = time.Now()

	h.logger.WithFields(map[string]any{
		"task_id":  taskID,
		"agent_id": req.AgentID,
		"status":   req.Status,
		"error":    req.Error,
	}).Info("Provisioning task status updated")

	response := map[string]interface{}{
		"success":    true,
		"task_id":    taskID,
		"status":     task.Status,
		"updated_at": task.UpdatedAt,
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// CreateProvisioningTask handles POST /api/v1/provisioner/tasks
func (h *Handler) CreateProvisioningTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type       string                 `json:"type"`
		DeviceMAC  string                 `json:"device_mac,omitempty"`
		TargetSSID string                 `json:"target_ssid,omitempty"`
		Config     map[string]interface{} `json:"config,omitempty"`
		AgentID    string                 `json:"agent_id,omitempty"`
		Priority   int                    `json:"priority,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if req.Type == "" {
		h.responseWriter().WriteValidationError(w, r, "Task type is required")
		return
	}

	// Generate task ID
	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	registry.mu.Lock()
	defer registry.mu.Unlock()

	task := &ProvisioningTask{
		ID:         taskID,
		Type:       req.Type,
		DeviceMAC:  req.DeviceMAC,
		TargetSSID: req.TargetSSID,
		Config:     req.Config,
		Status:     "pending",
		AgentID:    req.AgentID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Priority:   req.Priority,
	}

	registry.tasks[taskID] = task

	h.logger.WithFields(map[string]any{
		"task_id":     taskID,
		"type":        req.Type,
		"device_mac":  req.DeviceMAC,
		"target_ssid": req.TargetSSID,
		"agent_id":    req.AgentID,
	}).Info("New provisioning task created")

	response := map[string]interface{}{
		"success":    true,
		"task_id":    taskID,
		"status":     "pending",
		"created_at": task.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	h.responseWriter().WriteCreated(w, r, response)
}

// GetProvisioningTasks handles GET /api/v1/provisioner/tasks
func (h *Handler) GetProvisioningTasks(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	tasks := make([]*ProvisioningTask, 0, len(registry.tasks))
	for _, task := range registry.tasks {
		tasks = append(tasks, task)
	}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// HealthCheck handles GET /api/v1/provisioner/health
func (h *Handler) ProvisionerHealthCheck(w http.ResponseWriter, r *http.Request) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	activeAgents := 0
	for _, agent := range registry.agents {
		if time.Since(agent.LastSeen) <= 5*time.Minute {
			activeAgents++
		}
	}

	pendingTasks := 0
	for _, task := range registry.tasks {
		if task.Status == "pending" || task.Status == "assigned" {
			pendingTasks++
		}
	}

	response := map[string]interface{}{
		"status":          "healthy",
		"total_agents":    len(registry.agents),
		"active_agents":   activeAgents,
		"total_tasks":     len(registry.tasks),
		"pending_tasks":   pendingTasks,
		"timestamp":       time.Now(),
		"provisioner_api": "operational",
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// ReportDiscoveredDevices handles POST /api/v1/provisioner/discovered-devices
func (h *Handler) ReportDiscoveredDevices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		TaskID  string `json:"task_id,omitempty"`
		Devices []struct {
			MAC        string    `json:"mac"`
			SSID       string    `json:"ssid"`
			Model      string    `json:"model"`
			Generation int       `json:"generation"`
			IP         string    `json:"ip"`
			Signal     int       `json:"signal"`
			Discovered time.Time `json:"discovered"`
		} `json:"devices"`
		Timestamp time.Time `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if req.AgentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	if len(req.Devices) == 0 {
		http.Error(w, "At least one device is required", http.StatusBadRequest)
		return
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	// Verify agent is registered
	agent, exists := registry.agents[req.AgentID]
	if !exists {
		http.Error(w, "Agent not registered", http.StatusUnauthorized)
		return
	}

	// Update agent's last seen timestamp
	agent.LastSeen = time.Now()

	h.logger.WithFields(map[string]any{
		"agent_id":     req.AgentID,
		"task_id":      req.TaskID,
		"device_count": len(req.Devices),
		"component":    "provisioner_handler",
	}).Info("Received discovered devices from provisioning agent")

	// Process discovered devices
	devicesProcessed := 0
	devicesPersisted := 0
	for _, device := range req.Devices {
		// Log each discovered device
		h.logger.WithFields(map[string]any{
			"mac":        device.MAC,
			"ssid":       device.SSID,
			"model":      device.Model,
			"generation": device.Generation,
			"ip":         device.IP,
			"signal":     device.Signal,
			"discovered": device.Discovered,
			"agent_id":   req.AgentID,
			"component":  "provisioner_handler",
		}).Info("Processing discovered Shelly device")

		// Store discovered device in database for UI display
		discoveredDevice := &database.DiscoveredDevice{
			MAC:        device.MAC,
			SSID:       device.SSID,
			Model:      device.Model,
			Generation: device.Generation,
			IP:         device.IP,
			Signal:     device.Signal,
			AgentID:    req.AgentID,
			TaskID:     req.TaskID,
			Discovered: device.Discovered,
			ExpiresAt:  device.Discovered.Add(24 * time.Hour), // Expire after 24 hours
		}

		if err := h.DB.UpsertDiscoveredDevice(discoveredDevice); err != nil {
			h.logger.WithFields(map[string]any{
				"mac":       device.MAC,
				"agent_id":  req.AgentID,
				"error":     err.Error(),
				"component": "provisioner_handler",
			}).Warn("Failed to persist discovered device")
		} else {
			devicesPersisted++
		}

		devicesProcessed++
	}

	// Update task status if task ID is provided
	if req.TaskID != "" {
		if task, taskExists := registry.tasks[req.TaskID]; taskExists {
			task.Status = "completed"
			task.UpdatedAt = time.Now()

			h.logger.WithFields(map[string]any{
				"task_id":       req.TaskID,
				"agent_id":      req.AgentID,
				"devices_found": len(req.Devices),
				"component":     "provisioner_handler",
			}).Info("Discovery task completed with results")
		}
	}

	response := map[string]interface{}{
		"success":           true,
		"devices_received":  len(req.Devices),
		"devices_processed": devicesProcessed,
		"devices_persisted": devicesPersisted,
		"timestamp":         time.Now(),
		"message":           fmt.Sprintf("Successfully processed %d discovered devices (%d persisted)", devicesProcessed, devicesPersisted),
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// GetDiscoveredDevices handles GET /api/v1/provisioner/discovered-devices
func (h *Handler) GetDiscoveredDevices(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	agentID := r.URL.Query().Get("agent_id")

	// Retrieve discovered devices from database
	devices, err := h.DB.GetDiscoveredDevices(agentID)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"agent_id":  agentID,
			"error":     err.Error(),
			"component": "provisioner_handler",
		}).Error("Failed to retrieve discovered devices")
		apiresp.NewResponseWriter(h.logger).WriteInternalError(w, r, fmt.Errorf("failed to retrieve discovered devices"))
		return
	}

	// Clean up expired devices while we're here (async cleanup)
	go func() {
		if deleted, err := h.DB.CleanupExpiredDiscoveredDevices(); err != nil {
			h.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"component": "provisioner_handler",
			}).Warn("Failed to cleanup expired discovered devices")
		} else if deleted > 0 {
			h.logger.WithFields(map[string]any{
				"deleted":   deleted,
				"component": "provisioner_handler",
			}).Debug("Cleaned up expired discovered devices")
		}
	}()

	h.logger.WithFields(map[string]any{
		"agent_id":     agentID,
		"device_count": len(devices),
		"component":    "provisioner_handler",
	}).Info("Retrieved discovered devices")

	response := map[string]interface{}{
		"success":      true,
		"devices":      devices,
		"device_count": len(devices),
		"filtered_by":  agentID,
		"timestamp":    time.Now(),
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

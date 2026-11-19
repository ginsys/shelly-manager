package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/security/apikey"
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

	// token is the identity the manager issued at registration. It is never serialised.
	token string
}

// agentOfflineAfter is how long an agent may stay silent before it counts as offline and
// another caller may register under its id.
const agentOfflineAfter = 5 * time.Minute

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

const agentTokenHeader = "X-Agent-Token"

func newAgentToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// holdsAdminKey reports whether the request may act for any agent: the admin key, or no admin key configured.
func (h *Handler) holdsAdminKey(r *http.Request) bool {
	return h.AdminAPIKey == "" || apikey.Authorized(r, h.AdminAPIKey)
}

// agentOf returns the registered agent whose token the request presents, or nil. The caller holds registry.mu.
func agentOf(r *http.Request) *ProvisionerAgent {
	presented := r.Header.Get(agentTokenHeader)
	for _, agent := range registry.agents {
		if apikey.Equal(presented, agent.token) {
			return agent
		}
	}
	return nil
}

// authorizeAgent decides whether the caller may act for the agent want. The admin key, or no admin key at
// all, may act for any agent. Anyone else needs the token issued to want at registration: no valid token is
// a 401, which tells a real agent to register again, and a valid token of another agent is a 403. It writes
// the refusal and returns false. Call it before anything that records activity for the agent, so a refused
// caller cannot keep an agent id looking online. The caller holds registry.mu.
func (h *Handler) authorizeAgent(w http.ResponseWriter, r *http.Request, want string) bool {
	if h.holdsAdminKey(r) {
		return true
	}
	agent := agentOf(r)
	if agent == nil {
		h.responseWriter().WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Agent token required: register the agent", nil)
		return false
	}
	if agent.ID != want {
		h.responseWriter().WriteError(w, r, http.StatusForbidden, apiresp.ErrCodeForbidden, "The agent token does not belong to this agent", nil)
		return false
	}
	return true
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

	// An id that is online belongs to the agent that holds its token. Without this, anyone with the shared key
	// could register the id again and receive the tasks assigned to it.
	if exists && !h.holdsAdminKey(r) && now.Sub(agent.LastSeen) <= agentOfflineAfter &&
		!apikey.Equal(r.Header.Get(agentTokenHeader), agent.token) {
		h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "Agent id in use", nil)
		return
	}

	token, err := newAgentToken()
	if err != nil {
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

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
	agent.token = token // a register of an existing id replaces the old token

	response := map[string]interface{}{
		"agent_id":      agent.ID,
		"agent_token":   token,
		"registered_at": agent.RegisteredAt,
		"status":        "registered",
		"message":       "Agent registered successfully",
	}
	h.responseWriter().WriteCreated(w, r, response)
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

	if !h.authorizeAgent(w, r, agentID) {
		return
	}

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
		Status string                 `json:"status"`
		Result map[string]interface{} `json:"result,omitempty"`
		Error  string                 `json:"error,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	// An agent may update only a task assigned to it. An unassigned or unknown task is nobody's (want is ""),
	// so an agent gets the same 403 for both and learns nothing about which task ids exist.
	task, exists := registry.tasks[taskID]
	want := ""
	if exists {
		want = task.AgentID
	}
	if !h.authorizeAgent(w, r, want) {
		return
	}
	if !exists {
		h.responseWriter().WriteNotFoundError(w, r, "Task")
		return
	}

	// Update task status
	task.Status = req.Status
	task.UpdatedAt = time.Now()

	h.logger.WithFields(map[string]any{
		"task_id":  taskID,
		"agent_id": task.AgentID,
		"status":   req.Status,
		"error":    req.Error,
	}).Info("Provisioning task status updated")

	response := map[string]interface{}{
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
		"task_id":    taskID,
		"status":     "pending",
		"created_at": task.CreatedAt,
	}

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

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{
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
		if time.Since(agent.LastSeen) <= agentOfflineAfter {
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

	registry.mu.Lock()
	defer registry.mu.Unlock()

	// Identity first: a refused caller learns nothing from the validation answers below.
	if !h.authorizeAgent(w, r, req.AgentID) {
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

	// Complete the task if one is named, and only if it is assigned to the reporting agent: the report
	// route must not become a way to complete another agent's task.
	if req.TaskID != "" {
		if task, taskExists := registry.tasks[req.TaskID]; taskExists && (h.holdsAdminKey(r) || task.AgentID == req.AgentID) {
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
	h.background.Add(1)
	go func() {
		defer h.background.Done()
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
		"devices":      devices,
		"device_count": len(devices),
		"filtered_by":  agentID,
		"timestamp":    time.Now(),
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

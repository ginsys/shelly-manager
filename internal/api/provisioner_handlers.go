package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/security/apikey"
)

// The provisioner registry (agents, their hashed tokens and their tasks) lives in the database, behind
// database.ProvisionerStore. A handler decides who the caller is, calls the store and maps its errors to a
// response; it keeps no registry state of its own.

// agentOfflineAfter is how long an agent may stay silent before it counts as offline and
// another caller may register under its id.
const agentOfflineAfter = 5 * time.Minute

// maxProvisionerValueLen is the longest agent id, task type, task status or task agent id the registry stores,
// in characters: the indexed columns that hold them are 191 characters wide.
const maxProvisionerValueLen = database.MaxProvisionerValueLen

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
	key := h.adminKey.Get()
	return key == "" || apikey.Authorized(r, key)
}

// provisionerStore returns the registry store over the handler's database.
func (h *Handler) provisionerStore() *database.ProvisionerStore {
	return database.NewProvisionerStore(h.DB.GetDB())
}

// provisionerActor says who calls: the admin key (or no admin key at all) may act for any agent, anyone
// else is the holder of the token they present. Only the hash of the token reaches the store.
func (h *Handler) provisionerActor(r *http.Request) database.ProvisionerActor {
	return database.ProvisionerActor{
		Privileged: h.holdsAdminKey(r),
		TokenHash:  database.HashAgentToken(r.Header.Get(agentTokenHeader)),
	}
}

// tooLongForRegistry reports whether any value is over the longest the registry stores, and writes the
// 400 for it.
func (h *Handler) tooLongForRegistry(w http.ResponseWriter, r *http.Request, name, value string) bool {
	if utf8.RuneCountInString(value) <= maxProvisionerValueLen {
		return false
	}
	h.responseWriter().WriteValidationError(w, r, fmt.Sprintf("%s must be at most %d characters", name, maxProvisionerValueLen))
	return true
}

// writeProvisionerError maps a store error to its response. A refused agent is a 401, which tells a real
// agent to register again, or a 403 when the token belongs to another agent. An id in use is a 409, an
// unknown agent or task a 404, a status over the column length a 400, and any other error a 500.
func (h *Handler) writeProvisionerError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, database.ErrUnauthorized):
		h.responseWriter().WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Agent token required: register the agent", nil)
	case errors.Is(err, database.ErrForbidden):
		h.responseWriter().WriteError(w, r, http.StatusForbidden, apiresp.ErrCodeForbidden, "The agent token does not belong to this agent", nil)
	case errors.Is(err, database.ErrAgentInUse):
		h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "Agent id in use", nil)
	case errors.Is(err, database.ErrAgentNotFound):
		h.responseWriter().WriteNotFoundError(w, r, "Agent")
	case errors.Is(err, database.ErrTaskNotFound):
		h.responseWriter().WriteNotFoundError(w, r, "Task")
	case errors.Is(err, database.ErrStatusTooLong):
		h.responseWriter().WriteValidationError(w, r, fmt.Sprintf("Task status must be at most %d characters", maxProvisionerValueLen))
	default:
		h.responseWriter().WriteInternalError(w, r, err)
	}
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
	if h.tooLongForRegistry(w, r, "Agent id", req.ID) {
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

	token, err := newAgentToken()
	if err != nil {
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	// An id that is online belongs to the agent that holds its token. Without this, anyone with the shared key
	// could register the id again and receive the tasks assigned to it. The store applies the rule, and a
	// register of an existing id replaces the old token.
	now := time.Now()
	agent, created, err := h.provisionerStore().RegisterAgent(r.Context(), h.provisionerActor(r), database.ProvisionerAgent{
		AgentID:      req.ID,
		TokenHash:    database.HashAgentToken(token),
		Hostname:     req.Hostname,
		IP:           clientIP,
		Version:      req.Version,
		Capabilities: req.Capabilities,
		Status:       "online",
		Metadata:     req.Metadata,
		LastSeen:     now,
		RegisteredAt: now,
	}, agentOfflineAfter)
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	fields := map[string]any{"agent_id": req.ID, "hostname": req.Hostname, "ip": clientIP}
	if created {
		h.logger.WithFields(fields).Info("New provisioning agent registered")
	} else {
		h.logger.WithFields(fields).Debug("Existing provisioning agent re-registered")
	}

	response := map[string]interface{}{
		"agent_id":      agent.AgentID,
		"agent_token":   token,
		"registered_at": agent.RegisteredAt,
		"status":        "registered",
		"message":       "Agent registered successfully",
	}
	h.responseWriter().WriteCreated(w, r, response)

	// The register has committed and answered; the purge is its own transaction and never changes the answer.
	h.purgeRegistryIfDue(r.Context())
}

// How the registry forgets: an agent silent this long with no pending task is removed, and a task that
// finished (completed or failed) this long ago is deleted. The purge runs at most once an hour per process.
const (
	registryAgentSilentFor = 30 * 24 * time.Hour
	registryKeepFinished   = 30 * 24 * time.Hour
	registryPurgeEvery     = time.Hour
	registryPurgeTimeout   = time.Minute
)

// purgeRegistryIfDue starts the purge in the background group when the last one began over an hour ago. It
// outlives the request that started it (context.WithoutCancel) and logs a failure without reporting it.
func (h *Handler) purgeRegistryIfDue(ctx context.Context) {
	now := time.Now()
	last := h.registryPurgedAt.Load()
	if now.UnixNano()-last < int64(registryPurgeEvery) || !h.registryPurgedAt.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	store := h.provisionerStore()
	detached := context.WithoutCancel(ctx)
	h.background.Add(1)
	go func() {
		defer h.background.Done()
		purgeCtx, cancel := context.WithTimeout(detached, registryPurgeTimeout)
		defer cancel()
		res, err := store.Purge(purgeCtx, now, registryAgentSilentFor, registryKeepFinished)
		fields := map[string]any{"agents": res.Agents, "tasks": res.Tasks, "component": "provisioner_handler"}
		if err != nil {
			fields["error"] = err.Error()
			h.logger.WithFields(fields).Warn("Provisioner registry purge failed")
		} else if res.Agents > 0 || res.Tasks > 0 {
			h.logger.WithFields(fields).Info("Provisioner registry purge removed old rows")
		}
	}()
}

// DeleteProvisionerAgent handles DELETE /api/v1/provisioner/agents/{id}: it frees the id, revokes the
// agent's token and fails its assigned tasks. A token held by a running agent stops working at once, and
// that agent registers again.
func (h *Handler) DeleteProvisionerAgent(w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["id"]
	if err := h.provisionerStore().DeleteAgent(r.Context(), agentID, time.Now()); err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}
	h.logger.WithFields(map[string]any{"agent_id": agentID}).Info("Provisioning agent deleted")
	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{"agent_id": agentID, "status": "deleted"})
}

// GetProvisionerAgents handles GET /api/v1/provisioner/agents
func (h *Handler) GetProvisionerAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.provisionerStore().ListAgents(r.Context())
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}
	for i := range agents {
		// Update status based on last seen time
		if time.Since(agents[i].LastSeen) > agentOfflineAfter {
			agents[i].Status = "offline"
		}
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

	// The store authorizes the caller before it records any activity for the agent, so a refused caller
	// cannot keep an agent id looking online, then claims every pending task that is unassigned or the agent's.
	availableTasks, err := h.provisionerStore().ClaimTasks(r.Context(), h.provisionerActor(r), agentID, time.Now())
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
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

	// The status length is checked by the store after it authorizes the caller, so a refused caller gets
	// its 401 or 403 and never the 400.
	// An agent may update only a task assigned to it. An unassigned or unknown task is nobody's, so an agent
	// gets the same 403 for both and learns nothing about which task ids exist.
	task, err := h.provisionerStore().UpdateTaskStatus(r.Context(), h.provisionerActor(r), taskID, req.Status, time.Now())
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

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
	if h.tooLongForRegistry(w, r, "Task type", req.Type) || h.tooLongForRegistry(w, r, "Task agent id", req.AgentID) {
		return
	}

	now := time.Now()
	task := &database.ProvisioningTask{
		TaskID:     fmt.Sprintf("task_%d", now.UnixNano()),
		Type:       req.Type,
		DeviceMAC:  req.DeviceMAC,
		TargetSSID: req.TargetSSID,
		Config:     req.Config,
		Status:     "pending",
		AgentID:    req.AgentID,
		CreatedAt:  now,
		UpdatedAt:  now,
		Priority:   req.Priority,
	}
	if err := h.provisionerStore().CreateTask(r.Context(), task); err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	h.logger.WithFields(map[string]any{
		"task_id":     task.TaskID,
		"type":        req.Type,
		"device_mac":  req.DeviceMAC,
		"target_ssid": req.TargetSSID,
		"agent_id":    req.AgentID,
	}).Info("New provisioning task created")

	response := map[string]interface{}{
		"task_id":    task.TaskID,
		"status":     "pending",
		"created_at": task.CreatedAt,
	}

	h.responseWriter().WriteCreated(w, r, response)
}

// GetProvisioningTasks handles GET /api/v1/provisioner/tasks
func (h *Handler) GetProvisioningTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.provisionerStore().ListTasks(r.Context())
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// HealthCheck handles GET /api/v1/provisioner/health
func (h *Handler) ProvisionerHealthCheck(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	counts, err := h.provisionerStore().Counts(r.Context(), now, agentOfflineAfter)
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	response := map[string]interface{}{
		"status":          "healthy",
		"total_agents":    counts.Agents,
		"active_agents":   counts.ActiveAgents,
		"total_tasks":     counts.Tasks,
		"pending_tasks":   counts.PendingTasks,
		"timestamp":       now,
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

	devices := make([]database.DiscoveredDevice, len(req.Devices))
	for i, device := range req.Devices {
		// Stored for UI display, and expired after 24 hours.
		devices[i] = database.DiscoveredDevice{
			MAC:        device.MAC,
			SSID:       device.SSID,
			Model:      device.Model,
			Generation: device.Generation,
			IP:         device.IP,
			Signal:     device.Signal,
			AgentID:    req.AgentID,
			TaskID:     req.TaskID,
			Discovered: device.Discovered,
			ExpiresAt:  device.Discovered.Add(24 * time.Hour),
		}
	}

	// Identity first: a refused caller learns nothing from the validation answers. The store then records
	// the devices, marks the agent seen, and completes the task if one is named and it is assigned to the
	// reporting agent: the report route must not become a way to complete another agent's task.
	devicesPersisted, failures, err := h.provisionerStore().ReportDiscovered(r.Context(), h.provisionerActor(r), req.AgentID, req.TaskID, devices, time.Now())
	switch {
	case errors.Is(err, database.ErrAgentIDRequired):
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	case errors.Is(err, database.ErrNoDevices):
		http.Error(w, "At least one device is required", http.StatusBadRequest)
		return
	case errors.Is(err, database.ErrAgentNotFound):
		http.Error(w, "Agent not registered", http.StatusUnauthorized)
		return
	case err != nil:
		h.writeProvisionerError(w, r, err)
		return
	}
	h.logger.WithFields(map[string]any{
		"agent_id":     req.AgentID,
		"task_id":      req.TaskID,
		"device_count": len(req.Devices),
		"component":    "provisioner_handler",
	}).Info("Received discovered devices from provisioning agent")
	for _, device := range req.Devices {
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
		}).Info("Processed discovered Shelly device")
	}
	for _, failure := range failures {
		h.logger.WithFields(map[string]any{
			"agent_id":  req.AgentID,
			"error":     failure.Error(),
			"component": "provisioner_handler",
		}).Warn("Failed to persist discovered device")
	}

	devicesProcessed := len(req.Devices)
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

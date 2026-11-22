package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/database"
)

// UI-facing DTOs for the `/api/v1/provisioning/*` endpoints. The backend keeps
// tasks/agents in the database (see provisioner_handlers.go) with snake_case
// field names and agent-oriented status vocabulary; the UI expects camelCase
// fields and a coarser status set. These DTOs + translators bridge the two so
// the frontend contract stays stable.

type uiProvisioningTask struct {
	ID            string                 `json:"id"`
	DeviceID      string                 `json:"deviceId,omitempty"`
	DeviceName    string                 `json:"deviceName,omitempty"`
	Status        string                 `json:"status"`
	TaskType      string                 `json:"taskType"`
	Configuration map[string]interface{} `json:"config,omitempty"`
	Result        map[string]interface{} `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
}

// uiProvisioningAgent is an agent as the page shows it. EnrolmentState is the row's state (a keyless row is
// legacy) and Fingerprint is the key's fingerprint in the readable form, empty for a keyless row. Never the
// key, the challenge or the token hash.
type uiProvisioningAgent struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Version        string    `json:"version"`
	Capabilities   []string  `json:"capabilities"`
	LastSeen       time.Time `json:"lastSeen"`
	EnrolmentState string    `json:"enrolmentState"`
	Fingerprint    string    `json:"fingerprint,omitempty"`
}

type uiCreateTaskRequest struct {
	DeviceID      string                 `json:"deviceId"`
	TaskType      string                 `json:"taskType"`
	Configuration map[string]interface{} `json:"config,omitempty"`
}

type uiBulkProvisionRequest struct {
	DeviceIDs     []string               `json:"deviceIds"`
	Configuration map[string]interface{} `json:"config,omitempty"`
}

// mapInternalToUIStatus collapses internal task statuses into the four
// values the UI filter/state machine expects.
func mapInternalToUIStatus(internal string) string {
	switch internal {
	case "assigned", "in_progress":
		return "running"
	case "pending", "completed", "failed":
		return internal
	default:
		return internal
	}
}

// agentStatus derives the UI-facing agent status from LastSeen timestamp,
// using the registry's freshness threshold, agentOfflineAfter.
func agentStatus(agent *database.ProvisionerAgent) string {
	if time.Since(agent.LastSeen) > agentOfflineAfter {
		return "offline"
	}
	if agent.Status != "" {
		return agent.Status
	}
	return "online"
}

// toUITask translates an internal ProvisioningTask to its UI representation.
// deviceName is resolved by MAC lookup when the task carries DeviceMAC.
func (h *Handler) toUITask(task *database.ProvisioningTask) uiProvisioningTask {
	out := uiProvisioningTask{
		ID:            task.TaskID,
		Status:        mapInternalToUIStatus(task.Status),
		TaskType:      task.Type,
		Configuration: task.Config,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
	}

	if task.DeviceMAC != "" && h.DB != nil {
		if dev, err := h.DB.GetDeviceByMAC(task.DeviceMAC); err == nil && dev != nil {
			out.DeviceID = strconv.FormatUint(uint64(dev.ID), 10)
			out.DeviceName = dev.Name
		}
	}

	if result, ok := task.Config["_result"].(map[string]interface{}); ok {
		out.Result = result
	}
	if errMsg, ok := task.Config["_error"].(string); ok {
		out.Error = errMsg
	}

	return out
}

func toUIAgent(agent *database.ProvisionerAgent) uiProvisioningAgent {
	capabilities := agent.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	fingerprint := ""
	if agent.Fingerprint != nil {
		fingerprint = database.FormatFingerprint(*agent.Fingerprint)
	}
	return uiProvisioningAgent{
		ID:             agent.AgentID,
		Name:           agent.Hostname,
		Status:         agentStatus(agent),
		Version:        agent.Version,
		Capabilities:   capabilities,
		LastSeen:       agent.LastSeen,
		EnrolmentState: agent.State(),
		Fingerprint:    fingerprint,
	}
}

// resolveDeviceMAC looks up the MAC address for a numeric device ID carried
// in a UI request. Empty deviceId is allowed (task may target any agent).
func (h *Handler) resolveDeviceMAC(deviceID string) (string, error) {
	if deviceID == "" {
		return "", nil
	}
	id, err := strconv.ParseUint(deviceID, 10, 32)
	if err != nil {
		return "", fmt.Errorf("deviceId must be a numeric device id")
	}
	dev, err := h.DB.GetDevice(uint(id))
	if err != nil {
		return "", fmt.Errorf("device not found")
	}
	return dev.MAC, nil
}

// ListProvisioningTasksUI handles GET /api/v1/provisioning/tasks
func (h *Handler) ListProvisioningTasksUI(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	statusFilter := r.URL.Query().Get("status")

	tasks, err := h.provisionerStore().ListTasks(r.Context())
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	uiTasks := make([]uiProvisioningTask, 0, len(tasks))
	for i := range tasks {
		ui := h.toUITask(&tasks[i])
		if statusFilter != "" && ui.Status != statusFilter {
			continue
		}
		uiTasks = append(uiTasks, ui)
	}

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{
		"tasks": uiTasks,
		"count": len(uiTasks),
	})
}

// GetProvisioningTaskUI handles GET /api/v1/provisioning/tasks/{id}
func (h *Handler) GetProvisioningTaskUI(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	task, err := h.provisionerStore().GetTask(r.Context(), mux.Vars(r)["id"])
	if errors.Is(err, database.ErrTaskNotFound) {
		h.responseWriter().WriteNotFoundError(w, r, "Provisioning task")
		return
	}
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, h.toUITask(&task))
}

// CreateProvisioningTaskUI handles POST /api/v1/provisioning/tasks
func (h *Handler) CreateProvisioningTaskUI(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var req uiCreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}
	if req.TaskType == "" {
		h.responseWriter().WriteValidationError(w, r, "taskType is required")
		return
	}
	if h.tooLongForRegistry(w, r, "taskType", req.TaskType) {
		return
	}

	mac, err := h.resolveDeviceMAC(req.DeviceID)
	if err != nil {
		h.responseWriter().WriteValidationError(w, r, err.Error())
		return
	}

	task, err := h.createTask(r.Context(), req.TaskType, mac, req.Configuration)
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}
	h.responseWriter().WriteCreated(w, r, h.toUITask(task))
}

// createTask builds and stores a pending task. Separated so BulkProvisionUI can reuse it.
func (h *Handler) createTask(ctx context.Context, taskType, deviceMAC string, config map[string]interface{}) (*database.ProvisioningTask, error) {
	if config == nil {
		config = map[string]interface{}{}
	}
	now := time.Now()
	task := &database.ProvisioningTask{
		TaskID:    fmt.Sprintf("task_%d", now.UnixNano()),
		Type:      taskType,
		DeviceMAC: deviceMAC,
		Config:    config,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.provisionerStore().CreateTask(ctx, task); err != nil {
		return nil, err
	}

	h.logger.WithFields(map[string]any{
		"task_id":    task.TaskID,
		"task_type":  taskType,
		"device_mac": deviceMAC,
		"component":  "provisioning_ui",
	}).Info("UI-initiated provisioning task created")

	return task, nil
}

// CancelProvisioningTaskUI handles POST /api/v1/provisioning/tasks/{id}/cancel
//
// For unassigned tasks this is a clean flip to "failed" with error="canceled".
// Tasks already picked up by an agent are cancelled best-effort — the agent
// will push its final status on completion and overwrite ours. Changing that
// behaviour would require extending the agent protocol (out of scope here).
func (h *Handler) CancelProvisioningTaskUI(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	task, err := h.provisionerStore().CancelTask(r.Context(), mux.Vars(r)["id"], time.Now())
	if errors.Is(err, database.ErrTaskNotFound) {
		h.responseWriter().WriteNotFoundError(w, r, "Provisioning task")
		return
	}
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, h.toUITask(&task))
}

// BulkProvisionUI handles POST /api/v1/provisioning/bulk
func (h *Handler) BulkProvisionUI(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var req uiBulkProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}
	if len(req.DeviceIDs) == 0 {
		h.responseWriter().WriteValidationError(w, r, "deviceIds must contain at least one device id")
		return
	}

	uiTasks := make([]uiProvisioningTask, 0, len(req.DeviceIDs))
	for _, devID := range req.DeviceIDs {
		mac, err := h.resolveDeviceMAC(devID)
		if err != nil {
			h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest,
				fmt.Sprintf("device %q: %s", devID, err.Error()), nil)
			return
		}
		task, err := h.createTask(r.Context(), "configure", mac, req.Configuration)
		if err != nil {
			h.writeProvisionerError(w, r, err)
			return
		}
		uiTasks = append(uiTasks, h.toUITask(task))
	}

	h.responseWriter().WriteCreated(w, r, map[string]interface{}{
		"tasks": uiTasks,
		"count": len(uiTasks),
	})
}

// ListProvisioningAgentsUI handles GET /api/v1/provisioning/agents
func (h *Handler) ListProvisioningAgentsUI(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	agents, err := h.provisionerStore().ListAgents(r.Context()) // by hostname
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	uiAgents := make([]uiProvisioningAgent, 0, len(agents))
	for i := range agents {
		uiAgents = append(uiAgents, toUIAgent(&agents[i]))
	}

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{
		"agents": uiAgents,
		"count":  len(uiAgents),
	})
}

// GetProvisioningAgentStatusUI handles GET /api/v1/provisioning/agents/{id}/status
func (h *Handler) GetProvisioningAgentStatusUI(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	agent, err := h.provisionerStore().GetAgent(r.Context(), mux.Vars(r)["id"])
	if errors.Is(err, database.ErrAgentNotFound) {
		h.responseWriter().WriteNotFoundError(w, r, "Provisioning agent")
		return
	}
	if err != nil {
		h.writeProvisionerError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, toUIAgent(&agent))
}

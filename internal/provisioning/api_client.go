package provisioning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// APIClient handles communication with the main shelly-manager API server
type APIClient struct {
	baseURL    string
	apiKey     string
	client     *http.Client
	logger     *logging.Logger
	agentID    string
	registered bool
}

// AgentRegistrationRequest represents the agent registration payload
type AgentRegistrationRequest struct {
	ID           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	Version      string            `json:"version,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// AgentRegistrationResponse represents the API response for agent registration
type AgentRegistrationResponse struct {
	Success      bool      `json:"success"`
	AgentID      string    `json:"agent_id"`
	RegisteredAt time.Time `json:"registered_at"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
}

// ProvisioningTask represents a task from the API server
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

// TaskPollResponse represents the response from task polling
type TaskPollResponse struct {
	AgentID string              `json:"agent_id"`
	Tasks   []*ProvisioningTask `json:"tasks"`
	Count   int                 `json:"count"`
}

// TaskStatusUpdateRequest represents a task status update payload
type TaskStatusUpdateRequest struct {
	Status  string                 `json:"status"`
	AgentID string                 `json:"agent_id"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// HealthCheckResponse represents the API health check response
type HealthCheckResponse struct {
	Status         string    `json:"status"`
	TotalAgents    int       `json:"total_agents"`
	ActiveAgents   int       `json:"active_agents"`
	TotalTasks     int       `json:"total_tasks"`
	PendingTasks   int       `json:"pending_tasks"`
	Timestamp      time.Time `json:"timestamp"`
	ProvisionerAPI string    `json:"provisioner_api"`
}

// DiscoveredDevice represents a device discovered during provisioning scan
type DiscoveredDevice struct {
	MAC        string    `json:"mac"`
	SSID       string    `json:"ssid"`       // AP SSID (e.g., shelly1-AABBCC)
	Model      string    `json:"model"`      // Device model
	Generation int       `json:"generation"` // Gen1 or Gen2+
	IP         string    `json:"ip"`         // IP in AP mode (usually 192.168.33.1)
	Signal     int       `json:"signal"`     // WiFi signal strength
	Discovered time.Time `json:"discovered"`
}

// DeviceDiscoveryRequest represents the request to report discovered devices
type DeviceDiscoveryRequest struct {
	AgentID   string              `json:"agent_id"`
	TaskID    string              `json:"task_id,omitempty"`
	Devices   []*DiscoveredDevice `json:"devices"`
	Timestamp time.Time           `json:"timestamp"`
}

// DeviceDiscoveryResponse represents the API response for device discovery reporting
type DeviceDiscoveryResponse struct {
	Success          bool      `json:"success"`
	DevicesReceived  int       `json:"devices_received"`
	DevicesProcessed int       `json:"devices_processed"`
	Timestamp        time.Time `json:"timestamp"`
	Message          string    `json:"message,omitempty"`
}

// NewAPIClient creates a new API client for provisioner communication
func NewAPIClient(baseURL, apiKey, agentID string, logger *logging.Logger) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:     logger,
		agentID:    agentID,
		registered: false,
	}
}

// RegisterAgent registers this provisioning agent with the API server
func (c *APIClient) RegisterAgent(hostname string, capabilities []string, metadata map[string]string) error {
	req := AgentRegistrationRequest{
		ID:           c.agentID,
		Hostname:     hostname,
		Version:      "0.5.0-alpha",
		Capabilities: capabilities,
		Metadata:     metadata,
	}

	var response AgentRegistrationResponse
	if err := c.makeRequest("POST", "/api/v1/provisioner/agents/register", req, &response); err != nil {
		c.logger.WithFields(map[string]any{
			"agent_id": c.agentID,
			"hostname": hostname,
			"error":    err.Error(),
		}).Error("Failed to register agent with API server")
		return fmt.Errorf("failed to register agent: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("agent registration failed: %s", response.Message)
	}

	c.registered = true
	c.logger.WithFields(map[string]any{
		"agent_id":      response.AgentID,
		"status":        response.Status,
		"registered_at": response.RegisteredAt,
	}).Info("Agent successfully registered with API server")

	return nil
}

// PollTasks polls the API server for available provisioning tasks
func (c *APIClient) PollTasks() ([]*ProvisioningTask, error) {
	if !c.registered {
		return nil, fmt.Errorf("agent not registered - call RegisterAgent first")
	}

	endpoint := fmt.Sprintf("/api/v1/provisioner/agents/%s/tasks", c.agentID)

	var response TaskPollResponse
	if err := c.makeRequest("GET", endpoint, nil, &response); err != nil {
		c.logger.WithFields(map[string]any{
			"agent_id": c.agentID,
			"error":    err.Error(),
		}).Error("Failed to poll for tasks from API server")
		return nil, fmt.Errorf("failed to poll tasks: %w", err)
	}

	c.logger.WithFields(map[string]any{
		"agent_id":   c.agentID,
		"task_count": response.Count,
	}).Debug("Polled tasks from API server")

	return response.Tasks, nil
}

// UpdateTaskStatus updates the status of a specific task
func (c *APIClient) UpdateTaskStatus(taskID, status string, result map[string]interface{}, errorMsg string) error {
	if !c.registered {
		return fmt.Errorf("agent not registered - call RegisterAgent first")
	}

	endpoint := fmt.Sprintf("/api/v1/provisioner/tasks/%s/status", taskID)

	req := TaskStatusUpdateRequest{
		Status:  status,
		AgentID: c.agentID,
		Result:  result,
		Error:   errorMsg,
	}

	var response map[string]interface{}
	if err := c.makeRequest("PUT", endpoint, req, &response); err != nil {
		c.logger.WithFields(map[string]any{
			"task_id":  taskID,
			"agent_id": c.agentID,
			"status":   status,
			"error":    err.Error(),
		}).Error("Failed to update task status")
		return fmt.Errorf("failed to update task status: %w", err)
	}

	c.logger.WithFields(map[string]any{
		"task_id":  taskID,
		"agent_id": c.agentID,
		"status":   status,
	}).Debug("Task status updated successfully")

	return nil
}

// TestConnectivity tests connectivity to the API server
func (c *APIClient) TestConnectivity() error {
	var response HealthCheckResponse
	if err := c.makeRequest("GET", "/api/v1/provisioner/health", nil, &response); err != nil {
		c.logger.WithFields(map[string]any{
			"base_url": c.baseURL,
			"error":    err.Error(),
		}).Error("API connectivity test failed")
		return fmt.Errorf("connectivity test failed: %w", err)
	}

	if response.Status != "healthy" {
		return fmt.Errorf("API server reports unhealthy status: %s", response.Status)
	}

	c.logger.WithFields(map[string]any{
		"base_url":        c.baseURL,
		"server_status":   response.Status,
		"total_agents":    response.TotalAgents,
		"active_agents":   response.ActiveAgents,
		"pending_tasks":   response.PendingTasks,
		"provisioner_api": response.ProvisionerAPI,
	}).Info("API connectivity test successful")

	return nil
}

// IsRegistered returns whether the agent is registered with the API server
func (c *APIClient) IsRegistered() bool {
	return c.registered
}

// GetAgentID returns the agent ID
func (c *APIClient) GetAgentID() string {
	return c.agentID
}

// ReportDiscoveredDevices reports discovered devices to the API server
func (c *APIClient) ReportDiscoveredDevices(taskID string, devices []*DiscoveredDevice) error {
	if !c.registered {
		return fmt.Errorf("agent not registered with API server")
	}

	req := DeviceDiscoveryRequest{
		AgentID:   c.agentID,
		TaskID:    taskID,
		Devices:   devices,
		Timestamp: time.Now(),
	}

	var resp DeviceDiscoveryResponse
	err := c.makeRequest("POST", "/api/v1/provisioner/discovered-devices", req, &resp)
	if err != nil {
		c.logger.WithFields(map[string]any{
			"error":        err.Error(),
			"agent_id":     c.agentID,
			"task_id":      taskID,
			"device_count": len(devices),
			"component":    "api_client",
		}).Error("Failed to report discovered devices")
		return fmt.Errorf("failed to report discovered devices: %w", err)
	}

	c.logger.WithFields(map[string]any{
		"agent_id":          c.agentID,
		"task_id":           taskID,
		"devices_sent":      len(devices),
		"devices_received":  resp.DevicesReceived,
		"devices_processed": resp.DevicesProcessed,
		"success":           resp.Success,
		"component":         "api_client",
	}).Info("Successfully reported discovered devices")

	if !resp.Success {
		return fmt.Errorf("API server failed to process discovered devices: %s", resp.Message)
	}

	return nil
}

// makeRequest is a helper method to make HTTP requests to the API server
func (c *APIClient) makeRequest(method, endpoint string, requestBody interface{}, responseBody interface{}) error {
	url := c.baseURL + endpoint

	// Enhanced logging to show exact URL being called
	c.logger.WithFields(map[string]any{
		"component": "api_client",
		"method":    method,
		"base_url":  c.baseURL,
		"endpoint":  endpoint,
		"full_url":  url,
		"agent_id":  c.agentID,
	}).Debug("Making API request")

	var body io.Reader
	if requestBody != nil {
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("shelly-provisioner/%s", c.agentID))

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Make the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error if possible but continue
			_ = err
		}
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.WithFields(map[string]any{
			"component":     "api_client",
			"method":        method,
			"full_url":      url,
			"status_code":   resp.StatusCode,
			"response_body": string(respBody),
			"agent_id":      c.agentID,
		}).Error("API request failed with HTTP error")
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response if responseBody is provided
	if responseBody != nil {
		if err := json.Unmarshal(respBody, responseBody); err != nil {
			c.logger.WithFields(map[string]any{
				"component":     "api_client",
				"method":        method,
				"full_url":      url,
				"status_code":   resp.StatusCode,
				"response_body": string(respBody),
				"error":         err.Error(),
			}).Error("Failed to parse JSON response")
			return fmt.Errorf("failed to unmarshal response body: %w", err)
		}
	}

	c.logger.WithFields(map[string]any{
		"component":   "api_client",
		"method":      method,
		"full_url":    url,
		"status_code": resp.StatusCode,
		"agent_id":    c.agentID,
	}).Debug("API request completed successfully")

	return nil
}

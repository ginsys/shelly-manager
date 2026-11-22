package provisioning

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/ginsys/shelly-manager/internal/agentkey"
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
	// agentToken is the identity the manager issued at registration. It is kept in memory only: a restarted agent
	// registers again and gets a new one. An older manager issues none, and nothing is sent then.
	agentToken string
	// key is the agent's signing key, or nil for an agent that registers with the shared key alone. strict is
	// provisioning.enrolment "strict": a manager that answers a keyed register with a token at once is refused.
	key    *agentkey.Key
	strict bool
	// lastEnrolStatus is the last `pending` or `rejected` answer, so each change is logged once, not every retry.
	lastEnrolStatus string
}

// AgentRegistrationRequest represents the agent registration payload. PublicKey and Signature are the two steps
// of an enrolment by key: the first request has the key only, the second adds the signed challenge.
type AgentRegistrationRequest struct {
	ID           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	Version      string            `json:"version,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	PublicKey    string            `json:"public_key,omitempty"`
	Signature    string            `json:"signature,omitempty"`
}

// NotEnrolledError is the manager's answer `pending` or `rejected` to a keyed register: an answer, not a
// failure. The agent waits for the operator and asks again.
type NotEnrolledError struct {
	Status, Fingerprint, Message string
}

func (e *NotEnrolledError) Error() string {
	return fmt.Sprintf("agent not enrolled yet (%s): fingerprint %s", e.Status, e.Fingerprint)
}

// challengePattern is what a challenge is: 32 bytes as lowercase hexadecimal. Anything else is not signed.
var challengePattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// HTTPError is a non-2xx answer from the API server. Callers read StatusCode to tell a lost registration (401, 404)
// from any other failure.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("API request failed with status %d: %s", e.StatusCode, e.Body)
}

// AgentRegistrationResponse is the `data` of the API response for agent registration
type AgentRegistrationResponse struct {
	AgentID      string    `json:"agent_id"`
	AgentToken   string    `json:"agent_token,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
	// Fingerprint and Challenge come with a keyed register's `pending`, `rejected` and `challenge` answers.
	Fingerprint string `json:"fingerprint,omitempty"`
	Challenge   string `json:"challenge,omitempty"`
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

// DeviceDiscoveryResponse is the `data` of the API response for device discovery reporting
type DeviceDiscoveryResponse struct {
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

// SetKey makes the agent register by key. The id the agent was made with must be key.ID().
func (c *APIClient) SetKey(key *agentkey.Key, strict bool) {
	c.key, c.strict = key, strict
}

// RegisterAgent registers this provisioning agent with the API server. A keyed agent does it in two steps: an
// unsigned request that names its key gets `pending`, `rejected` (a *NotEnrolledError) or `challenge`, and the
// signed answer to a challenge gets the token. Every call starts again from the unsigned request.
func (c *APIClient) RegisterAgent(hostname string, capabilities []string, metadata map[string]string) error {
	req := AgentRegistrationRequest{
		ID:           c.agentID,
		Hostname:     hostname,
		Version:      "0.5.0-alpha",
		Capabilities: capabilities,
		Metadata:     metadata,
	}
	if c.key != nil {
		req.PublicKey = c.key.PublicKeyHex()
	}

	response, err := c.postRegister(req)
	if err != nil {
		return err
	}
	if c.key != nil {
		switch response.Status {
		case "pending", "rejected":
			return c.notEnrolled(response)
		case "challenge":
			return c.answerChallenge(req, response)
		}
		if c.strict {
			return errors.New("the manager answered a keyed register with a token and no signed challenge, so it does not enrol by key, and provisioning.enrolment is strict")
		}
	}
	c.accept(response)
	return nil
}

func (c *APIClient) postRegister(req AgentRegistrationRequest) (AgentRegistrationResponse, error) {
	var response AgentRegistrationResponse
	if err := c.makeRequest("POST", "/api/v1/provisioner/agents/register", req, &response); err != nil {
		c.logger.WithFields(map[string]any{
			"agent_id": c.agentID,
			"hostname": req.Hostname,
			"error":    err.Error(),
		}).Error("Failed to register agent with API server")
		return response, fmt.Errorf("failed to register agent: %w", err)
	}
	return response, nil
}

// answerChallenge signs the manager's challenge and sends the second step. A challenge that is not 64 lowercase
// hexadecimal characters is not signed.
func (c *APIClient) answerChallenge(req AgentRegistrationRequest, challenge AgentRegistrationResponse) error {
	if !challengePattern.MatchString(challenge.Challenge) {
		return errors.New("the manager sent a challenge that is not 64 lowercase hexadecimal characters: not signing it")
	}
	raw, _ := hex.DecodeString(challenge.Challenge) // the pattern above guarantees valid hexadecimal
	req.Signature = c.key.Sign(c.agentID, raw)
	response, err := c.postRegister(req)
	if err != nil {
		return err
	}
	if response.Status == "pending" || response.Status == "rejected" {
		return c.notEnrolled(response) // the operator changed the row between the two steps
	}
	if response.AgentToken == "" {
		return fmt.Errorf("the manager accepted the signature but sent no token (status %q)", response.Status)
	}
	c.accept(response)
	return nil
}

// notEnrolled returns the typed answer and logs each change of it once: the retries repeat it, the log does not.
func (c *APIClient) notEnrolled(response AgentRegistrationResponse) error {
	fingerprint := response.Fingerprint
	if fingerprint == "" {
		fingerprint = c.key.Readable()
	}
	if response.Status != c.lastEnrolStatus {
		c.lastEnrolStatus = response.Status
		fields := map[string]any{"agent_id": c.agentID, "fingerprint": fingerprint, "message": response.Message}
		if response.Status == "rejected" {
			c.logger.WithFields(fields).Error("The operator rejected this agent: it asks again until the operator approves it. Check the fingerprint in the manager")
		} else {
			c.logger.WithFields(fields).Info("Waiting for the operator to approve this agent: compare this fingerprint in the manager, then approve it")
		}
	}
	return &NotEnrolledError{Status: response.Status, Fingerprint: fingerprint, Message: response.Message}
}

// accept keeps the token of a successful register in memory.
func (c *APIClient) accept(response AgentRegistrationResponse) {
	c.registered = true
	c.agentToken = response.AgentToken
	c.lastEnrolStatus = ""
	c.logger.WithFields(map[string]any{
		"agent_id":      response.AgentID,
		"status":        response.Status,
		"registered_at": response.RegisteredAt,
	}).Info("Agent successfully registered with API server")
}

// EnsureRegistered registers the agent unless it already is. The poll loop calls it before every poll, so a
// registration that failed at start-up, or that the manager lost (its database was restored from an older
// backup, or an operator removed the agent), is retried.
func (c *APIClient) EnsureRegistered(hostname string, capabilities []string, metadata map[string]string) error {
	if c.registered {
		return nil
	}
	return c.RegisterAgent(hostname, capabilities, metadata)
}

// dropRegistration marks the agent unregistered when err is an HTTP answer with one of the given statuses, so the
// next EnsureRegistered registers again.
func (c *APIClient) dropRegistration(err error, statuses ...int) {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return
	}
	for _, s := range statuses {
		if httpErr.StatusCode == s {
			c.registered = false
			c.agentToken = ""
			c.logger.WithFields(map[string]any{
				"agent_id":    c.agentID,
				"status_code": s,
			}).Warn("Manager no longer knows this agent, registering again on the next poll")
			return
		}
	}
}

// PollTasks polls the API server for available provisioning tasks
func (c *APIClient) PollTasks() ([]*ProvisioningTask, error) {
	if !c.registered {
		return nil, fmt.Errorf("agent not registered - call RegisterAgent first")
	}

	endpoint := fmt.Sprintf("/api/v1/provisioner/agents/%s/tasks", c.agentID)

	var response TaskPollResponse
	if err := c.makeRequest("GET", endpoint, nil, &response); err != nil {
		c.dropRegistration(err, http.StatusUnauthorized, http.StatusNotFound)
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
		c.dropRegistration(err, http.StatusUnauthorized)
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
		c.dropRegistration(err, http.StatusUnauthorized)
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
		"component":         "api_client",
	}).Info("Successfully reported discovered devices")

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
	if c.agentToken != "" {
		req.Header.Set("X-Agent-Token", c.agentToken)
	}

	// Make the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error if possible but continue
			_ = closeErr
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
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	// Parse response if responseBody is provided. Every manager route answers in the {success, data} envelope; the
	// caller's struct is the `data` of it, and the envelope's `success` is the only success flag.
	if responseBody != nil {
		var envelope struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
			Error   *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err := json.Unmarshal(respBody, &envelope)
		if err == nil && !envelope.Success {
			msg := "no reason given"
			if envelope.Error != nil {
				msg = envelope.Error.Message
			}
			return fmt.Errorf("API server reported failure: %s", msg)
		}
		if err == nil && len(envelope.Data) > 0 {
			err = json.Unmarshal(envelope.Data, responseBody)
		}
		if err != nil {
			c.logger.WithFields(map[string]any{
				"component":     "api_client",
				"method":        method,
				"full_url":      url,
				"status_code":   resp.StatusCode,
				"response_size": len(respBody), // size only: a success answer can carry the agent token
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

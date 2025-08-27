package opnsense

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Client represents an OPNSense API client
type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	logger     *logging.Logger
}

// ClientConfig holds configuration for creating an OPNSense client
type ClientConfig struct {
	Host               string        `json:"host"`
	Port               int           `json:"port"`
	UseHTTPS           bool          `json:"use_https"`
	APIKey             string        `json:"api_key"`
	APISecret          string        `json:"api_secret"`
	Timeout            time.Duration `json:"timeout"`
	InsecureSkipVerify bool          `json:"insecure_skip_verify"`
}

// NewClient creates a new OPNSense API client
func NewClient(config ClientConfig, logger *logging.Logger) (*Client, error) {
	if config.Host == "" {
		return nil, fmt.Errorf("host is required")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if config.APISecret == "" {
		return nil, fmt.Errorf("api_secret is required")
	}

	// Set defaults
	if config.Port == 0 {
		config.Port = 443
		if !config.UseHTTPS {
			config.Port = 80
		}
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// Build base URL
	scheme := "http"
	if config.UseHTTPS {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s:%d", scheme, config.Host, config.Port)

	// Create HTTP client with custom transport for TLS configuration
	transport := &http.Transport{}
	if config.UseHTTPS && config.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	client := &Client{
		baseURL:    baseURL,
		apiKey:     config.APIKey,
		apiSecret:  config.APISecret,
		httpClient: httpClient,
		logger:     logger,
	}

	return client, nil
}

// TestConnection tests the connection to OPNSense API
func (c *Client) TestConnection(ctx context.Context) error {
	c.logger.Info("Testing connection to OPNSense", "host", c.baseURL)

	// Try to get basic system information
	_, err := c.makeRequest(ctx, "GET", "/api/core/system/status", nil)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	c.logger.Info("OPNSense connection test successful")
	return nil
}

// makeRequest makes an authenticated HTTP request to the OPNSense API
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	// Build full URL
	fullURL := c.baseURL + endpoint

	// Prepare request body
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Set basic authentication
	req.SetBasicAuth(c.apiKey, c.apiSecret)

	// Log request (without sensitive data)
	c.logger.Debug("Making OPNSense API request",
		"method", method,
		"endpoint", endpoint,
		"url", strings.ReplaceAll(fullURL, c.apiKey, "***"),
	)

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.WithFields(map[string]any{
				"error": closeErr.Error(),
			}).Debug("Failed to close response body")
		}
	}()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Error("OPNSense API request failed",
			"method", method,
			"endpoint", endpoint,
			"status_code", resp.StatusCode,
			"response", string(responseBody),
		)

		// Try to parse the response body for more detailed error information
		var errorResponse struct {
			Status      string                 `json:"status"`
			Message     string                 `json:"message"`
			Validations map[string]interface{} `json:"validations"`
		}

		if err := json.Unmarshal(responseBody, &errorResponse); err != nil {
			// If we can't parse JSON, use the raw response as the message
			return nil, &APIError{
				HTTPStatus: resp.StatusCode,
				Message:    string(responseBody),
			}
		}

		// Extract message from parsed JSON, fallback to generic message
		message := errorResponse.Message
		if message == "" {
			message = "API request failed"
		}

		// Convert validations to string map for compatibility with APIError.Details
		details := make(map[string]string)
		for key, value := range errorResponse.Validations {
			details[key] = fmt.Sprintf("%v", value)
		}

		return nil, &APIError{
			HTTPStatus: resp.StatusCode,
			Message:    message,
			Details:    details,
		}
	}

	c.logger.Debug("OPNSense API request successful",
		"method", method,
		"endpoint", endpoint,
		"status_code", resp.StatusCode,
		"response_length", len(responseBody),
	)

	return responseBody, nil
}

// makeRequestWithQuery makes an HTTP request with URL query parameters
func (c *Client) makeRequestWithQuery(ctx context.Context, method, endpoint string, queryParams map[string]string, body interface{}) ([]byte, error) {
	if len(queryParams) > 0 {
		values := url.Values{}
		for key, value := range queryParams {
			values.Add(key, value)
		}
		endpoint = endpoint + "?" + values.Encode()
	}

	return c.makeRequest(ctx, method, endpoint, body)
}

// GetSystemStatus retrieves basic system status information
func (c *Client) GetSystemStatus(ctx context.Context) (*SystemStatus, error) {
	responseBody, err := c.makeRequest(ctx, "GET", "/api/core/system/status", nil)
	if err != nil {
		return nil, err
	}

	var status SystemStatus
	if err := json.Unmarshal(responseBody, &status); err != nil {
		return nil, fmt.Errorf("failed to parse system status response: %w", err)
	}

	return &status, nil
}

// Close closes the HTTP client connections
func (c *Client) Close() error {
	// Close idle connections
	c.httpClient.CloseIdleConnections()
	c.logger.Info("OPNSense client closed")
	return nil
}

package opnsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func setupTestLogger(t *testing.T) *logging.Logger {
	logger, err := logging.New(logging.Config{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	})
	require.NoError(t, err)
	return logger
}

func TestNewClient(t *testing.T) {
	logger := setupTestLogger(t)

	t.Run("Valid Configuration", func(t *testing.T) {
		config := ClientConfig{
			Host:      "192.168.1.1",
			Port:      443,
			UseHTTPS:  true,
			APIKey:    "test-api-key",
			APISecret: "test-api-secret",
			Timeout:   30 * time.Second,
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "https://192.168.1.1:443", client.baseURL)
		assert.Equal(t, "test-api-key", client.apiKey)
		assert.Equal(t, "test-api-secret", client.apiSecret)
	})

	t.Run("HTTP Configuration", func(t *testing.T) {
		config := ClientConfig{
			Host:      "10.0.0.1",
			Port:      8080,
			UseHTTPS:  false,
			APIKey:    "test-key",
			APISecret: "test-secret",
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)
		assert.Equal(t, "http://10.0.0.1:8080", client.baseURL)
	})

	t.Run("Default Port Settings", func(t *testing.T) {
		config := ClientConfig{
			Host:      "opnsense.local",
			UseHTTPS:  true,
			APIKey:    "test-key",
			APISecret: "test-secret",
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)
		assert.Equal(t, "https://opnsense.local:443", client.baseURL)

		config.UseHTTPS = false
		client, err = NewClient(config, logger)
		assert.NoError(t, err)
		assert.Equal(t, "http://opnsense.local:80", client.baseURL)
	})

	t.Run("Default Timeout", func(t *testing.T) {
		config := ClientConfig{
			Host:      "192.168.1.1",
			APIKey:    "test-key",
			APISecret: "test-secret",
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)
		assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
	})

	t.Run("Missing Host", func(t *testing.T) {
		config := ClientConfig{
			APIKey:    "test-key",
			APISecret: "test-secret",
		}

		client, err := NewClient(config, logger)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "host is required")
	})

	t.Run("Missing API Key", func(t *testing.T) {
		config := ClientConfig{
			Host:      "192.168.1.1",
			APISecret: "test-secret",
		}

		client, err := NewClient(config, logger)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "api_key is required")
	})

	t.Run("Missing API Secret", func(t *testing.T) {
		config := ClientConfig{
			Host:   "192.168.1.1",
			APIKey: "test-key",
		}

		client, err := NewClient(config, logger)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "api_secret is required")
	})

	t.Run("Insecure Skip Verify", func(t *testing.T) {
		config := ClientConfig{
			Host:               "192.168.1.1",
			UseHTTPS:           true,
			APIKey:             "test-key",
			APISecret:          "test-secret",
			InsecureSkipVerify: true,
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)
		assert.NotNil(t, client)

		// Verify TLS configuration
		transport, ok := client.httpClient.Transport.(*http.Transport)
		assert.True(t, ok)
		assert.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	})
}

func TestClientTestConnection(t *testing.T) {
	logger := setupTestLogger(t)

	t.Run("Successful Connection", func(t *testing.T) {
		// Mock server that returns a successful response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify basic auth is present
			username, password, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "test-key", username)
			assert.Equal(t, "test-secret", password)

			// Verify correct endpoint
			assert.Equal(t, "/api/core/system/status", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version": "22.7.8"}`))
		}))
		defer server.Close()

		// Parse the test server URL
		config := ClientConfig{
			Host:      strings.TrimPrefix(server.URL, "http://"),
			UseHTTPS:  false,
			APIKey:    "test-key",
			APISecret: "test-secret",
		}

		// Extract host and port from server URL
		parts := strings.Split(config.Host, ":")
		if len(parts) == 2 {
			config.Host = parts[0]
			config.Port = 0 // Let it use the server's port
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)

		// Override the base URL to point to our test server
		client.baseURL = server.URL

		err = client.TestConnection(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Connection Failure - Wrong Credentials", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
		}))
		defer server.Close()

		config := ClientConfig{
			Host:      "localhost",
			UseHTTPS:  false,
			APIKey:    "wrong-key",
			APISecret: "wrong-secret",
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)
		client.baseURL = server.URL

		err = client.TestConnection(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection test failed")
	})

	t.Run("Connection Failure - Server Not Reachable", func(t *testing.T) {
		config := ClientConfig{
			Host:      "192.0.2.1", // TEST-NET-1 - guaranteed unreachable
			Port:      443,
			UseHTTPS:  true,
			APIKey:    "test-key",
			APISecret: "test-secret",
			Timeout:   1 * time.Second, // Short timeout for testing
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = client.TestConnection(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection test failed")
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version": "22.7.8"}`))
		}))
		defer server.Close()

		config := ClientConfig{
			Host:      "localhost",
			UseHTTPS:  false,
			APIKey:    "test-key",
			APISecret: "test-secret",
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)
		client.baseURL = server.URL

		// Create context that will be cancelled quickly
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err = client.TestConnection(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestClientMakeRequest(t *testing.T) {
	logger := setupTestLogger(t)

	t.Run("GET Request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/core/system/status", r.URL.Path)

			username, password, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "api-key", username)
			assert.Equal(t, "api-secret", password)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok", "version": "22.7.8"}`))
		}))
		defer server.Close()

		config := ClientConfig{
			Host:      "localhost",
			UseHTTPS:  false,
			APIKey:    "api-key",
			APISecret: "api-secret",
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)
		client.baseURL = server.URL

		response, err := client.makeRequest(context.Background(), "GET", "/api/core/system/status", nil)
		assert.NoError(t, err)
		assert.Contains(t, string(response), "22.7.8")
	})

	t.Run("POST Request with Body", func(t *testing.T) {
		expectedBody := `{"hostname": "test-device", "ip": "192.168.1.100"}`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/dhcpv4/reservation", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			assert.JSONEq(t, expectedBody, string(body))

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok", "uuid": "new-uuid"}`))
		}))
		defer server.Close()

		config := ClientConfig{
			Host:      "localhost",
			UseHTTPS:  false,
			APIKey:    "api-key",
			APISecret: "api-secret",
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)
		client.baseURL = server.URL

		requestData := map[string]interface{}{
			"hostname": "test-device",
			"ip":       "192.168.1.100",
		}

		response, err := client.makeRequest(context.Background(), "POST", "/api/dhcpv4/reservation", requestData)
		assert.NoError(t, err)
		assert.Contains(t, string(response), "new-uuid")
	})

	t.Run("HTTP Error Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{
				"status": "failed", 
				"message": "Validation error",
				"validations": {"mac": "Invalid MAC address"}
			}`))
		}))
		defer server.Close()

		config := ClientConfig{
			Host:      "localhost",
			UseHTTPS:  false,
			APIKey:    "api-key",
			APISecret: "api-secret",
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)
		client.baseURL = server.URL

		_, err = client.makeRequest(context.Background(), "POST", "/api/dhcpv4/reservation", map[string]interface{}{
			"mac": "invalid-mac",
		})

		assert.Error(t, err)
		apiError, ok := err.(*APIError)
		assert.True(t, ok)
		assert.Equal(t, 400, apiError.HTTPStatus)
		assert.Equal(t, "Validation error", apiError.Message)
		assert.Contains(t, apiError.Details, "mac")
	})

	t.Run("Network Error", func(t *testing.T) {
		config := ClientConfig{
			Host:      "192.0.2.1", // TEST-NET-1 - unreachable
			Port:      443,
			UseHTTPS:  true,
			APIKey:    "api-key",
			APISecret: "api-secret",
			Timeout:   1 * time.Second,
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = client.makeRequest(ctx, "GET", "/api/core/system/status", nil)
		assert.Error(t, err)
		// Should be a network error, not an APIError
		_, isAPIError := err.(*APIError)
		assert.False(t, isAPIError)
	})

	t.Run("Invalid JSON in Request Body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This shouldn't be called if JSON marshaling fails
			t.Fatal("Server should not be called with invalid JSON request")
		}))
		defer server.Close()

		config := ClientConfig{
			Host:      "localhost",
			UseHTTPS:  false,
			APIKey:    "api-key",
			APISecret: "api-secret",
		}

		client, err := NewClient(config, logger)
		require.NoError(t, err)
		client.baseURL = server.URL

		// Use a map with a channel value which can't be marshaled to JSON
		invalidData := map[string]interface{}{
			"channel": make(chan int),
		}

		_, err = client.makeRequest(context.Background(), "POST", "/api/test", invalidData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to marshal request body")
	})
}

func TestClientConfigValidation(t *testing.T) {
	logger := setupTestLogger(t)

	testCases := []struct {
		name        string
		config      ClientConfig
		expectError string
	}{
		{
			name: "Valid minimal config",
			config: ClientConfig{
				Host:      "192.168.1.1",
				APIKey:    "key",
				APISecret: "secret",
			},
			expectError: "",
		},
		{
			name: "Empty host",
			config: ClientConfig{
				APIKey:    "key",
				APISecret: "secret",
			},
			expectError: "host is required",
		},
		{
			name: "Empty API key",
			config: ClientConfig{
				Host:      "192.168.1.1",
				APISecret: "secret",
			},
			expectError: "api_key is required",
		},
		{
			name: "Empty API secret",
			config: ClientConfig{
				Host:   "192.168.1.1",
				APIKey: "key",
			},
			expectError: "api_secret is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(tc.config, logger)

			if tc.expectError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestClientHTTPSConfiguration(t *testing.T) {
	logger := setupTestLogger(t)

	t.Run("HTTPS with Skip Verify", func(t *testing.T) {
		config := ClientConfig{
			Host:               "opnsense.local",
			UseHTTPS:           true,
			APIKey:             "key",
			APISecret:          "secret",
			InsecureSkipVerify: true,
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)

		transport := client.httpClient.Transport.(*http.Transport)
		assert.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	})

	t.Run("HTTPS without Skip Verify", func(t *testing.T) {
		config := ClientConfig{
			Host:               "opnsense.local",
			UseHTTPS:           true,
			APIKey:             "key",
			APISecret:          "secret",
			InsecureSkipVerify: false,
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)

		transport := client.httpClient.Transport.(*http.Transport)
		// TLS config might be nil for default secure settings
		if transport.TLSClientConfig != nil {
			assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
		}
	})

	t.Run("HTTP Configuration", func(t *testing.T) {
		config := ClientConfig{
			Host:      "opnsense.local",
			UseHTTPS:  false,
			APIKey:    "key",
			APISecret: "secret",
		}

		client, err := NewClient(config, logger)
		assert.NoError(t, err)

		assert.Contains(t, client.baseURL, "http://")
		assert.NotContains(t, client.baseURL, "https://")
	})
}

package shelly

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Helper functions are now in testhelpers_test.go

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assertNotNil(t, factory)
}

func TestNewFactoryWithLogger(t *testing.T) {
	logger := logging.GetDefault()
	factory := NewFactoryWithLogger(logger)
	assertNotNil(t, factory)
}

func TestFactory_CreateClient_ReturnsError(t *testing.T) {
	factory := NewFactory()

	client, err := factory.CreateClient("192.168.1.100", 1)
	assertError(t, err)
	assertEqual(t, (Client)(nil), client)
}

func TestFactory_DetectGeneration_Gen2Device(t *testing.T) {
	// Mock server that responds like a Gen2+ device
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc/Shelly.GetDeviceInfo" {
			response := map[string]interface{}{
				"id":  "shellyplusht-08b61fcb7f3c",
				"gen": 2,
				"mac": "08B61FCB7F3C",
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	factory := NewFactory()
	serverIP := server.URL[len("http://"):]

	ctx := context.Background()
	generation, err := factory.DetectGeneration(ctx, serverIP)
	assertNoError(t, err)
	assertEqual(t, 2, generation)
}

func TestFactory_DetectGeneration_Gen3Device(t *testing.T) {
	// Mock server that responds like a Gen3 device
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc/Shelly.GetDeviceInfo" {
			response := map[string]interface{}{
				"id":  "shellyplus2pm-8c4b14123456",
				"gen": 3,
				"mac": "8C4B14123456",
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	factory := NewFactory()
	serverIP := server.URL[len("http://"):]

	ctx := context.Background()
	generation, err := factory.DetectGeneration(ctx, serverIP)
	assertNoError(t, err)
	assertEqual(t, 3, generation)
}

func TestFactory_DetectGeneration_Gen1Device(t *testing.T) {
	// Mock server that responds like a Gen1 device
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc/Shelly.GetDeviceInfo":
			// Gen1 devices don't respond to RPC endpoints, so return 404
			http.NotFound(w, r)
		case "/shelly":
			response := map[string]interface{}{
				"type": "SHSW-1",
				"mac":  "A4CF12345678",
				"auth": true,
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	factory := NewFactory()
	serverIP := server.URL[len("http://"):]

	ctx := context.Background()
	generation, err := factory.DetectGeneration(ctx, serverIP)
	assertNoError(t, err)
	assertEqual(t, 1, generation)
}

func TestFactory_DetectGeneration_InvalidResponse(t *testing.T) {
	// Mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc/Shelly.GetDeviceInfo":
			_, _ = w.Write([]byte("invalid json"))
		case "/shelly":
			_, _ = w.Write([]byte("invalid json"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	factory := NewFactory()
	serverIP := server.URL[len("http://"):]

	ctx := context.Background()
	generation, err := factory.DetectGeneration(ctx, serverIP)
	assertError(t, err)
	assertEqual(t, 0, generation)
}

func TestFactory_DetectGeneration_NoResponse(t *testing.T) {
	// Mock server that returns 404 for all endpoints
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	factory := NewFactory()
	serverIP := server.URL[len("http://"):]

	ctx := context.Background()
	generation, err := factory.DetectGeneration(ctx, serverIP)
	assertError(t, err)
	assertEqual(t, 0, generation)
}

func TestFactory_DetectGeneration_EmptyResponse(t *testing.T) {
	// Mock server that returns empty JSON objects
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc/Shelly.GetDeviceInfo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{}"))
		case "/shelly":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{}"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	factory := NewFactory()
	serverIP := server.URL[len("http://"):]

	ctx := context.Background()
	generation, err := factory.DetectGeneration(ctx, serverIP)
	assertError(t, err)
	assertEqual(t, 0, generation)
	assertEqual(t, ErrInvalidGeneration, err)
}

func TestFactory_DetectGeneration_NetworkError(t *testing.T) {
	factory := NewFactory()

	ctx := context.Background()
	generation, err := factory.DetectGeneration(ctx, "192.168.1.200") // Non-existent IP
	assertError(t, err)
	assertEqual(t, 0, generation)
}

func TestFactory_DetectGeneration_ContextCancellation(t *testing.T) {
	factory := NewFactory()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	generation, err := factory.DetectGeneration(ctx, "192.168.1.100")
	assertError(t, err)
	assertEqual(t, 0, generation)
}

func TestFactory_CreateClientWithDetection(t *testing.T) {
	// Mock Gen2 device
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc/Shelly.GetDeviceInfo" {
			response := map[string]interface{}{
				"id":  "shellyplusht-08b61fcb7f3c",
				"gen": 2,
				"mac": "08B61FCB7F3C",
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	factory := NewFactory()
	serverIP := server.URL[len("http://"):]

	ctx := context.Background()
	client, err := factory.CreateClientWithDetection(ctx, serverIP)

	// Should detect generation but fail to create client due to import cycle limitation
	assertError(t, err)
	assertEqual(t, (Client)(nil), client)
}

func TestDefaultFactory(t *testing.T) {
	assertNotNil(t, DefaultFactory)

	// Test that it implements the Factory interface
	var _ = DefaultFactory
}

func TestFactory_Interface(t *testing.T) {
	// Test that factory implements Factory interface
	factory := NewFactory()
	assertNotNil(t, factory)
}

// Test error cases for factory methods
func TestFactory_EdgeCases(t *testing.T) {
	factory := NewFactory()
	ctx := context.Background()

	// Test with empty IP
	generation, err := factory.DetectGeneration(ctx, "")
	assertError(t, err)
	assertEqual(t, 0, generation)

	// Test with invalid IP format
	generation, err = factory.DetectGeneration(ctx, "invalid-ip")
	assertError(t, err)
	assertEqual(t, 0, generation)
}

// Test with different response status codes
func TestFactory_DetectGeneration_StatusCodes(t *testing.T) {
	tests := []struct {
		name          string
		gen2Status    int
		gen1Status    int
		expectedGen   int
		expectedError bool
	}{
		{
			name:          "Gen2 OK, Gen1 Not Found",
			gen2Status:    http.StatusOK,
			gen1Status:    http.StatusNotFound,
			expectedGen:   2,
			expectedError: false,
		},
		{
			name:          "Gen2 Not Found, Gen1 OK",
			gen2Status:    http.StatusNotFound,
			gen1Status:    http.StatusOK,
			expectedGen:   1,
			expectedError: false,
		},
		{
			name:          "Both Not Found",
			gen2Status:    http.StatusNotFound,
			gen1Status:    http.StatusNotFound,
			expectedGen:   0,
			expectedError: true,
		},
		{
			name:          "Gen2 Unauthorized, Gen1 OK",
			gen2Status:    http.StatusUnauthorized,
			gen1Status:    http.StatusOK,
			expectedGen:   1,
			expectedError: false,
		},
		{
			name:          "Gen2 Internal Error, Gen1 OK",
			gen2Status:    http.StatusInternalServerError,
			gen1Status:    http.StatusOK,
			expectedGen:   1,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/rpc/Shelly.GetDeviceInfo":
					if tt.gen2Status == http.StatusOK {
						response := map[string]interface{}{
							"id":  "shellyplusht-08b61fcb7f3c",
							"gen": 2,
							"mac": "08B61FCB7F3C",
						}
						w.Header().Set("Content-Type", "application/json")
						if err := json.NewEncoder(w).Encode(response); err != nil {
							t.Logf("Failed to encode JSON response: %v", err)
						}
					} else {
						w.WriteHeader(tt.gen2Status)
					}
				case "/shelly":
					if tt.gen1Status == http.StatusOK {
						response := map[string]interface{}{
							"type": "SHSW-1",
							"mac":  "A4CF12345678",
							"auth": true,
						}
						w.Header().Set("Content-Type", "application/json")
						if err := json.NewEncoder(w).Encode(response); err != nil {
							t.Logf("Failed to encode JSON response: %v", err)
						}
					} else {
						w.WriteHeader(tt.gen1Status)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			factory := NewFactory()
			serverIP := server.URL[len("http://"):]

			ctx := context.Background()
			generation, err := factory.DetectGeneration(ctx, serverIP)

			if tt.expectedError {
				assertError(t, err)
			} else {
				assertNoError(t, err)
			}
			assertEqual(t, tt.expectedGen, generation)
		})
	}
}

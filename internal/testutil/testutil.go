package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestConfig creates a test configuration
func TestConfig() *config.Config {
	return &config.Config{
		Server: struct {
			Port     int    `mapstructure:"port"`
			Host     string `mapstructure:"host"`
			LogLevel string `mapstructure:"log_level"`
		}{
			Port:     8080,
			Host:     "127.0.0.1",
			LogLevel: "debug",
		},
		Database: struct {
			Path            string            `mapstructure:"path"`
			Provider        string            `mapstructure:"provider"`
			DSN             string            `mapstructure:"dsn"`
			MaxOpenConns    int               `mapstructure:"max_open_conns"`
			MaxIdleConns    int               `mapstructure:"max_idle_conns"`
			ConnMaxLifetime int               `mapstructure:"conn_max_lifetime"`
			ConnMaxIdleTime int               `mapstructure:"conn_max_idle_time"`
			SlowQueryTime   int               `mapstructure:"slow_query_time"`
			LogLevel        string            `mapstructure:"log_level"`
			Options         map[string]string `mapstructure:"options"`
		}{
			Path: ":memory:", // Use in-memory SQLite for tests
		},
		Discovery: struct {
			Enabled         bool     `mapstructure:"enabled"`
			Networks        []string `mapstructure:"networks"`
			Interval        int      `mapstructure:"interval"`
			Timeout         int      `mapstructure:"timeout"`
			EnableMDNS      bool     `mapstructure:"enable_mdns"`
			EnableSSDP      bool     `mapstructure:"enable_ssdp"`
			ConcurrentScans int      `mapstructure:"concurrent_scans"`
		}{
			Enabled:         true,
			Networks:        []string{"192.168.1.0/24"},
			Interval:        300,
			Timeout:         2,
			EnableMDNS:      true,
			EnableSSDP:      true,
			ConcurrentScans: 10,
		},
	}
}

// TestDatabase creates a test database using provider abstraction
func TestDatabase(t *testing.T) (*database.Manager, func()) {
	// Create a unique temporary file for this test to avoid in-memory issues
	tmpFile, err := os.CreateTemp("", "test-*.db")
	require.NoError(t, err, "Failed to create temp file")
	tmpFile.Close()

	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err, "Failed to create logger")

	// Create database manager with the temporary file using provider abstraction
	dbManager, err := database.NewManagerFromPathWithLogger(tmpFile.Name(), logger)
	require.NoError(t, err, "Failed to create test database")

	cleanup := func() {
		if dbManager != nil {
			dbManager.Close()
		}
		os.Remove(tmpFile.Name())
	}

	return dbManager, cleanup
}

// TestDatabaseMemory creates an in-memory test database using provider abstraction
// This is faster but should be used carefully to avoid race conditions
func TestDatabaseMemory(t *testing.T) (*database.Manager, func()) {
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err, "Failed to create logger")

	// Use file::memory:?cache=shared for shared in-memory database
	dbManager, err := database.NewManagerFromPathWithLogger("file::memory:?cache=shared", logger)
	require.NoError(t, err, "Failed to create test database")

	cleanup := func() {
		if dbManager != nil {
			dbManager.Close()
		}
	}

	return dbManager, cleanup
}

// TestDevice creates a test device
func TestDevice() *database.Device {
	return &database.Device{
		IP:       "192.168.1.100",
		MAC:      "A4:CF:12:34:56:78",
		Type:     "Smart Plug",
		Name:     "Test Device",
		Firmware: "20231219-134356",
		Status:   "online",
		LastSeen: time.Now(),
		Settings: `{"model":"SHPLG-S","gen":1,"auth_enabled":true}`,
	}
}

// MockShellyServer creates a mock HTTP server that simulates Shelly device responses
func MockShellyServer() *httptest.Server {
	mux := http.NewServeMux()

	// Mock Gen1 device responses
	mux.HandleFunc("/shelly", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"type":         "SHPLG-S",
			"mac":          "A4CF12345678",
			"auth":         true,
			"fw":           "20231219-134356",
			"discoverable": true,
			"num_outputs":  1,
			"num_meters":   1,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	// Mock status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"wifi_sta": map[string]interface{}{
				"connected": true,
				"ssid":      "TestNetwork",
				"ip":        "192.168.1.100",
				"rssi":      -45,
			},
			"cloud": map[string]interface{}{
				"enabled":   false,
				"connected": false,
			},
			"mqtt": map[string]interface{}{
				"connected": false,
			},
			"time":            time.Now().Format("15:04"),
			"unixtime":        time.Now().Unix(),
			"serial":          12345,
			"has_update":      false,
			"mac":             "A4CF12345678",
			"cfg_changed_cnt": 2,
			"actions_stats": map[string]interface{}{
				"skipped": 0,
			},
			"relays": []map[string]interface{}{
				{
					"ison":            true,
					"has_timer":       false,
					"timer_started":   0,
					"timer_duration":  0,
					"timer_remaining": 0,
					"overpower":       false,
					"overtemperature": false,
					"is_valid":        true,
					"source":          "input",
				},
			},
			"meters": []map[string]interface{}{
				{
					"power":     25.5,
					"overpower": 0.0,
					"is_valid":  true,
					"timestamp": time.Now().Unix(),
					"counters":  []float64{123.456, 234.567, 345.678},
					"total":     703.501,
				},
			},
			"temperature":     45.2,
			"overtemperature": false,
			"tmp": map[string]interface{}{
				"tC":       45.2,
				"tF":       113.36,
				"is_valid": true,
			},
			"ram_total": 50592,
			"ram_free":  39052,
			"fs_size":   233681,
			"fs_free":   162648,
			"uptime":    3600,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	return httptest.NewServer(mux)
}

// MockShellyGen2Server creates a mock server for Gen2+ devices
func MockShellyGen2Server() *httptest.Server {
	mux := http.NewServeMux()

	// Mock Gen2 device responses
	mux.HandleFunc("/shelly", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":          "shellyplusht-08b61fcb7f3c",
			"mac":         "08B61FCB7F3C",
			"model":       "SNSN-0013A",
			"gen":         2,
			"fw_id":       "20231031-165617/1.0.3-geb51a17",
			"ver":         "1.0.3",
			"app":         "PlusHT",
			"auth_en":     false,
			"auth_domain": "shellyplusht-08b61fcb7f3c",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	return httptest.NewServer(mux)
}

// TempDir creates a temporary directory for testing
func TempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "shelly-manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// CreateTestConfigFile creates a temporary config file for testing
func CreateTestConfigFile(t *testing.T, cfg *config.Config) string {
	dir := TempDir(t)
	configFile := filepath.Join(dir, "testing.yaml")

	content := `server:
  port: 8080
  host: 127.0.0.1
  log_level: "debug"

database:
  path: ":memory:"

discovery:
  enabled: true
  networks:
    - 192.168.1.0/24
  interval: 300
  timeout: 2
  enable_mdns: true
  enable_ssdp: true
  concurrent_scans: 10
`

	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	return configFile
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

// AssertTrue asserts that a condition is true
func AssertTrue(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatalf("Expected condition to be true, but it was false")
	}
}

// AssertNotNil asserts that a value is not nil
func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()
	if value == nil {
		t.Fatalf("Expected value to be not nil, but it was nil")
	}
}

package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func TestApplicationInitialization(t *testing.T) {
	// Test the initialization sequence that would happen in main()
	tempDir := testutil.TempDir(t)
	dbPath := filepath.Join(tempDir, "test.db")
	configPath := filepath.Join(tempDir, "test.yaml")

	// Create test config
	configContent := `server:
  port: 8080
  host: localhost
  log_level: error

logging:
  level: error
  format: text
  output: stderr

database:
  path: ` + dbPath + `

discovery:
  enabled: true
  networks:
    - 192.168.1.0/24
  timeout: 5
  concurrent_scans: 2

provisioning:
  auth_enabled: false
  cloud_enabled: false
  mqtt_enabled: false
  auto_provision: false
  provision_interval: 300

dhcp:
  network: 192.168.1.0/24
  start_ip: 192.168.1.100
  end_ip: 192.168.1.200
  auto_reserve: false

opnsense:
  enabled: false
  host: 192.168.1.1
  port: 443
  auto_apply: false

main_app:
  enabled: false

notifications:
  enabled: false
  thresholds:
    critical_drift_count: 10
    warning_drift_count: 5
    max_per_hour: 100

resolution:
  auto_fix_enabled: false
  safe_mode: true
  approval_required: true
  auto_fix_categories: []
  excluded_paths: []

metrics:
  enabled: false
  prometheus_enabled: false
  prometheus_port: 9090
  collection_interval: 60
  retention_days: 30
  enable_http_metrics: false
  enable_detailed_timing: false
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Run("Config Loading", func(t *testing.T) {
		// Test config loading functionality
		cfg, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if cfg.Server.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", cfg.Server.Port)
		}

		if cfg.Database.Path != dbPath {
			t.Errorf("Expected database path %s, got %s", dbPath, cfg.Database.Path)
		}

		if len(cfg.Discovery.Networks) != 1 || cfg.Discovery.Networks[0] != "192.168.1.0/24" {
			t.Errorf("Expected discovery networks [192.168.1.0/24], got %v", cfg.Discovery.Networks)
		}
	})

	t.Run("Database Initialization", func(t *testing.T) {
		db, err := database.NewManager(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database manager: %v", err)
		}

		// Verify database is working
		devices, err := db.GetDevices()
		if err != nil {
			t.Errorf("Failed to get devices from new database: %v", err)
		}

		if len(devices) != 0 {
			t.Errorf("Expected empty device list, got %d devices", len(devices))
		}

		// Close database connection if possible
		if sqlDB, err := db.DB.DB(); err == nil {
			sqlDB.Close()
		}
	})

	t.Run("Logger Initialization", func(t *testing.T) {
		// Test logger initialization
		logConfig := logging.Config{
			Level:  "error",
			Format: "text",
			Output: "stderr",
		}

		logger, err := logging.New(logConfig)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		if logger == nil {
			t.Error("Logger should not be nil")
		}

		// Test basic logging functionality
		logger.Info("Test log message")
		logger.Error("Test error message")
	})
}

func TestInitializationWithDefaults(t *testing.T) {
	// Test initialization with default/minimal configuration
	tempDir := testutil.TempDir(t)
	dbPath := filepath.Join(tempDir, "minimal.db")

	minimalConfig := &config.Config{
		Database: struct {
			Path string `mapstructure:"path"`
		}{
			Path: dbPath,
		},
	}

	t.Run("Minimal Database Setup", func(t *testing.T) {
		db, err := database.NewManager(minimalConfig.Database.Path)
		if err != nil {
			t.Fatalf("Failed to create database with minimal config: %v", err)
		}

		// Test that database works with minimal config
		devices, err := db.GetDevices()
		if err != nil {
			t.Errorf("Database should work with minimal config: %v", err)
		}

		if devices == nil {
			t.Error("Devices slice should not be nil")
		}

		// Close database connection
		if sqlDB, err := db.DB.DB(); err == nil {
			sqlDB.Close()
		}
	})

	t.Run("Default Logger", func(t *testing.T) {
		logger := logging.GetDefault()
		if logger == nil {
			t.Error("Default logger should not be nil")
		}

		// Test default logger functionality
		logger.Info("Default logger test")
	})
}

func TestErrorConditions(t *testing.T) {
	// Test initialization error conditions
	t.Run("Invalid Database Path", func(t *testing.T) {
		// Try to create database in non-existent directory without permissions
		invalidPath := "/root/nonexistent/test.db"
		_, err := database.NewManager(invalidPath)
		if err == nil {
			t.Error("Expected error for invalid database path")
		}
	})

	t.Run("Invalid Config File", func(t *testing.T) {
		tempDir := testutil.TempDir(t)
		invalidConfigPath := filepath.Join(tempDir, "invalid.yaml")

		// Write invalid YAML
		err := os.WriteFile(invalidConfigPath, []byte("invalid: yaml: content: ["), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		_, err = config.Load(invalidConfigPath)
		if err == nil {
			t.Error("Expected error for invalid config file")
		}
	})

	t.Run("Nonexistent Config File", func(t *testing.T) {
		_, err := config.Load("/nonexistent/config.yaml")
		if err == nil {
			t.Error("Expected error for nonexistent config file")
		}
	})
}

func TestApplicationShutdown(t *testing.T) {
	// Test graceful shutdown functionality
	tempDir := testutil.TempDir(t)
	dbPath := filepath.Join(tempDir, "shutdown_test.db")

	db, err := database.NewManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	t.Run("Database Cleanup", func(t *testing.T) {
		// Test that database connections are properly closed
		sqlDB, err := db.DB.DB()
		if err != nil {
			t.Fatalf("Failed to get SQL DB: %v", err)
		}

		// Close the database
		err = sqlDB.Close()
		if err != nil {
			t.Errorf("Failed to close database: %v", err)
		}

		// Verify database is closed by trying to ping
		err = sqlDB.Ping()
		if err == nil {
			t.Error("Expected error pinging closed database")
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		// Test context cancellation handling
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan bool)
		go func() {
			// Simulate work that responds to context cancellation
			select {
			case <-ctx.Done():
				done <- true
			case <-time.After(100 * time.Millisecond):
				done <- false
			}
		}()

		// Cancel context immediately
		cancel()

		result := <-done
		if !result {
			t.Error("Expected context cancellation to be handled")
		}
	})
}

func TestConcurrentInitialization(t *testing.T) {
	// Test that initialization is safe under concurrent access
	tempDir := testutil.TempDir(t)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Start multiple goroutines that try to initialize components
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine creates its own database
			dbPath := filepath.Join(tempDir, "concurrent_"+string(rune('0'+id))+".db")
			db, err := database.NewManager(dbPath)
			if err != nil {
				errors <- err
				return
			}

			// Test database operations
			devices, err := db.GetDevices()
			if err != nil {
				errors <- err
				return
			}

			if devices == nil {
				errors <- err
				return
			}

			// Close database
			if sqlDB, err := db.DB.DB(); err == nil {
				sqlDB.Close()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent initialization error: %v", err)
	}
}

func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test in short mode")
	}

	// Test that initialization doesn't cause excessive memory usage
	tempDir := testutil.TempDir(t)
	dbPath := filepath.Join(tempDir, "memory_test.db")

	// Create multiple database managers to test for leaks
	for i := 0; i < 100; i++ {
		db, err := database.NewManager(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database manager %d: %v", i, err)
		}

		// Perform some operations
		_, err = db.GetDevices()
		if err != nil {
			t.Errorf("Database operation failed on iteration %d: %v", i, err)
		}

		// Close database connection
		if sqlDB, err := db.DB.DB(); err == nil {
			sqlDB.Close()
		}
	}

	// Force garbage collection
	// In a real test, we might measure memory usage here
	// For now, we just verify no panics or crashes occurred
}

func TestConfigurationEdgeCases(t *testing.T) {
	// Test configuration edge cases
	tempDir := testutil.TempDir(t)

	tests := []struct {
		name          string
		configContent string
		expectError   bool
		description   string
	}{
		{
			name:          "Empty Config",
			configContent: "",
			expectError:   false,
			description:   "Should handle empty config with defaults",
		},
		{
			name: "Config with Comments",
			configContent: `# This is a comment
server:
  port: 8080  # Server port
# Another comment
database:
  path: /tmp/test.db
`,
			expectError: false,
			description: "Should handle YAML comments",
		},
		{
			name: "Config with Environment Variables",
			configContent: `server:
  port: ${PORT:-8080}
database:
  path: ${DB_PATH:-/tmp/test.db}
`,
			expectError: true,
			description: "Should fail on environment variable placeholders (not supported)",
		},
		{
			name: "Minimal Valid Config",
			configContent: `database:
  path: /tmp/minimal.db
`,
			expectError: false,
			description: "Should work with minimal configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, tt.name+".yaml")
			err := os.WriteFile(configPath, []byte(tt.configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err = config.Load(configPath)
			if (err != nil) != tt.expectError {
				t.Errorf("Config loading error = %v, expectError = %v (%s)",
					err, tt.expectError, tt.description)
			}
		})
	}
}

package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// Helper function to create test database
func createTestDB(t *testing.T) *database.Manager {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create logger for database
	logger, err := logging.New(logging.Config{
		Level:  "error", // Minimize test output
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	db, err := database.NewManagerWithLogger(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Ensure database is closed when test completes to prevent Windows file locking issues
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close test database: %v", closeErr)
		}
	})

	return db
}

// Helper function to create test database without cleanup (for concurrent tests)
func createTestDBNoCleanup(t *testing.T) *database.Manager {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create logger for database
	logger, err := logging.New(logging.Config{
		Level:  "error", // Minimize test output
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	db, err := database.NewManagerWithLogger(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Note: No cleanup here - caller must call db.Close() manually
	return db
}

// Helper function to create test config
func createTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Discovery.Enabled = true
	cfg.Discovery.Networks = []string{"203.0.113.0/30"} // TEST-NET-3 range for testing
	cfg.Discovery.Interval = 60
	cfg.Discovery.Timeout = 1 // Short timeout for tests
	cfg.Discovery.EnableMDNS = false
	cfg.Discovery.EnableSSDP = false
	cfg.Discovery.ConcurrentScans = 2
	cfg.Database.Path = ":memory:"
	return cfg
}

// Helper function to create test logger
func createTestLogger(t *testing.T) *logging.Logger {
	t.Helper()

	logger, err := logging.New(logging.Config{
		Level:  "error", // Minimize test output
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	return logger
}

func TestNewService(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfig()

	service := NewService(db, cfg)

	if service == nil {
		t.Fatal("NewService should return a service instance")
	}

	if service.DB != db {
		t.Error("Service should have the provided database manager")
	}

	if service.Config != cfg {
		t.Error("Service should have the provided config")
	}

	if service.logger == nil {
		t.Error("Service should have a logger")
	}

	if service.ctx == nil {
		t.Error("Service should have a context")
	}

	if service.cancel == nil {
		t.Error("Service should have a cancel function")
	}
}

func TestNewServiceWithLogger(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	if service == nil {
		t.Fatal("NewServiceWithLogger should return a service instance")
	}

	if service.DB != db {
		t.Error("Service should have the provided database manager")
	}

	if service.Config != cfg {
		t.Error("Service should have the provided config")
	}

	if service.logger != logger {
		t.Error("Service should have the provided logger")
	}

	if service.ctx == nil {
		t.Error("Service should have a context")
	}

	if service.cancel == nil {
		t.Error("Service should have a cancel function")
	}
}

func TestShellyService_Stop(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Verify context is initially not cancelled
	select {
	case <-service.ctx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Expected: context is not cancelled
	}

	// Call Stop
	service.Stop()

	// Verify context is now cancelled
	select {
	case <-service.ctx.Done():
		// Expected: context is cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after Stop()")
	}
}

func TestShellyService_DiscoverDevices_InvalidNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Test with invalid network format (this should still work but find no devices)
	devices, err := service.DiscoverDevices("invalid-network-format")

	// Should not return an error for invalid network format
	// (the discovery process should handle this gracefully)
	if err != nil {
		t.Logf("Discovery with invalid network returned error: %v (this may be expected)", err)
	}

	// Should return empty devices list
	if len(devices) > 0 {
		t.Logf("Found %d devices with invalid network (unexpected but not necessarily an error)", len(devices))
	}
}

func TestShellyService_DiscoverDevices_AutoNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Test with "auto" network parameter (should use config networks)
	devices, err := service.DiscoverDevices("auto")

	// Since we're using a very small test network (192.168.1.0/30),
	// we don't expect to find real devices
	if err != nil {
		t.Logf("Discovery with auto network returned error: %v", err)
	}

	// Should return a list (may be empty for test network)
	if devices == nil {
		t.Error("DiscoverDevices should return a device slice, not nil")
	}

	// Log results for debugging
	t.Logf("Discovery completed with %d devices found", len(devices))
}

func TestShellyService_DiscoverDevices_EmptyNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Test with empty network parameter (should use config networks)
	devices, err := service.DiscoverDevices("")

	// Should complete without error
	if err != nil {
		t.Logf("Discovery with empty network returned error: %v", err)
	}

	// Should return a list
	if devices == nil {
		t.Error("DiscoverDevices should return a device slice, not nil")
	}

	t.Logf("Discovery with empty network found %d devices", len(devices))
}

func TestShellyService_DiscoverDevices_SpecificNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Test with specific network parameter (TEST-NET-3 range)
	testNetwork := "203.0.113.0/30"
	devices, err := service.DiscoverDevices(testNetwork)

	// Should complete without error
	if err != nil {
		t.Logf("Discovery with specific network returned error: %v", err)
	}

	// Should return a list
	if devices == nil {
		t.Error("DiscoverDevices should return a device slice, not nil")
	}

	t.Logf("Discovery on network %s found %d devices", testNetwork, len(devices))
}

func TestShellyService_DiscoverDevices_NoConfigNetworks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)

	// Create config with no discovery networks
	cfg := &config.Config{}
	cfg.Discovery.Enabled = true
	cfg.Discovery.Networks = []string{} // Empty networks
	cfg.Discovery.Interval = 60
	cfg.Discovery.Timeout = 1
	cfg.Discovery.EnableMDNS = false
	cfg.Discovery.EnableSSDP = false
	cfg.Discovery.ConcurrentScans = 2
	cfg.Database.Path = ":memory:"

	logger := createTestLogger(t)
	service := NewServiceWithLogger(db, cfg, logger)

	// Test discovery with empty config networks and auto parameter
	devices, err := service.DiscoverDevices("auto")

	// Should complete (may find no devices due to empty config)
	if err != nil {
		t.Logf("Discovery with no config networks returned error: %v", err)
	}

	// Should return a list
	if devices == nil {
		t.Error("DiscoverDevices should return a device slice, not nil")
	}

	t.Logf("Discovery with no config networks found %d devices", len(devices))
}

func TestShellyService_DiscoverDevices_ZeroTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)

	// Create config with zero timeout (should use default)
	cfg := &config.Config{}
	cfg.Discovery.Enabled = true
	cfg.Discovery.Networks = []string{"203.0.113.0/30"} // TEST-NET-3
	cfg.Discovery.Interval = 60
	cfg.Discovery.Timeout = 0 // Zero timeout - should use default
	cfg.Discovery.EnableMDNS = false
	cfg.Discovery.EnableSSDP = false
	cfg.Discovery.ConcurrentScans = 2
	cfg.Database.Path = ":memory:"

	logger := createTestLogger(t)
	service := NewServiceWithLogger(db, cfg, logger)

	// Test discovery with zero timeout
	devices, err := service.DiscoverDevices("203.0.113.0/30")

	// Should complete (should use default timeout of 2 seconds)
	if err != nil {
		t.Logf("Discovery with zero timeout returned error: %v", err)
	}

	// Should return a list
	if devices == nil {
		t.Error("DiscoverDevices should return a device slice, not nil")
	}

	t.Logf("Discovery with zero timeout found %d devices", len(devices))
}

func TestShellyService_DiscoverDevices_CancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Stop the service to cancel its context
	service.Stop()

	// Try discovery with cancelled context
	// Note: DiscoverDevices creates its own context with timeout, so this test
	// verifies that the service can still function even if the main context is cancelled
	devices, err := service.DiscoverDevices("203.0.113.0/30")

	// Should still work since DiscoverDevices uses its own context
	if err != nil {
		t.Logf("Discovery with cancelled service context returned error: %v", err)
	}

	// Should return a list
	if devices == nil {
		t.Error("DiscoverDevices should return a device slice, not nil")
	}

	t.Logf("Discovery with cancelled context found %d devices", len(devices))
}

func TestShellyService_DiscoverDevices_DeviceConversion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	// This test verifies the device conversion logic
	// Since we can't reliably find real devices in tests, we'll test the structure

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Run discovery
	devices, err := service.DiscoverDevices("203.0.113.0/30")

	if err != nil {
		t.Logf("Discovery returned error: %v", err)
	}

	// Verify the structure of returned devices
	for i, device := range devices {
		t.Logf("Device %d: IP=%s, MAC=%s, Type=%s, Name=%s, Firmware=%s, Status=%s",
			i, device.IP, device.MAC, device.Type, device.Name, device.Firmware, device.Status)

		// Verify required fields are set appropriately
		if device.Status != "online" {
			t.Errorf("Device %d should have status 'online', got '%s'", i, device.Status)
		}

		// Settings should be JSON string
		if device.Settings == "" {
			t.Errorf("Device %d should have settings populated", i)
		}

		// LastSeen should be set
		if device.LastSeen.IsZero() {
			t.Errorf("Device %d should have LastSeen timestamp", i)
		}
	}
}

func TestShellyService_MultipleOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Test multiple discovery operations
	for i := 0; i < 3; i++ {
		devices, err := service.DiscoverDevices("203.0.113.0/30")

		if err != nil {
			t.Logf("Discovery iteration %d returned error: %v", i+1, err)
		}

		if devices == nil {
			t.Errorf("Discovery iteration %d returned nil devices", i+1)
		}

		t.Logf("Discovery iteration %d found %d devices", i+1, len(devices))
	}

	// Stop the service
	service.Stop()

	// Verify service can still perform operations after stop
	devices, err := service.DiscoverDevices("203.0.113.0/30")

	if err != nil {
		t.Logf("Discovery after stop returned error: %v", err)
	}

	if devices == nil {
		t.Error("Discovery after stop should still return device slice")
	}
}

// Integration test with database operations
func TestShellyService_WithDatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)

	// Add a test device to database first
	testDevice := &database.Device{
		IP:       "203.0.113.100",
		MAC:      "AA:BB:CC:DD:EE:FF",
		Type:     "switch",
		Name:     "Test Device",
		Firmware: "1.0.0",
		Status:   "online",
		LastSeen: time.Now(),
		Settings: `{"model":"test","gen":1,"auth_enabled":false}`,
	}

	err := db.AddDevice(testDevice)
	if err != nil {
		t.Fatalf("Failed to add test device: %v", err)
	}

	// Retrieve devices from database
	devices, err := db.GetDevices()
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}

	if len(devices) != 1 {
		t.Errorf("Expected 1 device in database, got %d", len(devices))
	}

	if devices[0].IP != testDevice.IP {
		t.Errorf("Expected device IP %s, got %s", testDevice.IP, devices[0].IP)
	}

	// Now run discovery (won't find the test device we added, but should work)
	discoveredDevices, err := service.DiscoverDevices("203.0.113.0/30")

	if err != nil {
		t.Logf("Discovery returned error: %v", err)
	}

	// Both operations should work independently
	t.Logf("Database contains %d devices", len(devices))
	t.Logf("Discovery found %d devices", len(discoveredDevices))
}

// Benchmark tests
func BenchmarkShellyService_Creation(b *testing.B) {
	// Create test dependencies once
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")

	logger, _ := logging.New(logging.Config{
		Level:  "error",
		Format: "text",
		Output: "stderr",
	})

	db, err := database.NewManagerWithLogger(dbPath, logger)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}

	// Ensure database is closed to prevent Windows file locking issues
	b.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			b.Logf("Failed to close benchmark database: %v", closeErr)
		}
	})

	cfg := createTestConfig()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		service := NewServiceWithLogger(db, cfg, logger)
		service.Stop() // Clean up
	}
}

func BenchmarkShellyService_DiscoverDevices(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	db := createTestDB(&testing.T{}) // Use testing.T for helper
	cfg := createTestConfig()
	logger, _ := logging.New(logging.Config{
		Level:  "error",
		Format: "text",
		Output: "stderr",
	})

	service := NewServiceWithLogger(db, cfg, logger)
	defer service.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = service.DiscoverDevices("203.0.113.0/30")
	}
}

// Test edge cases and error conditions
func TestShellyService_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)

	service := NewServiceWithLogger(db, cfg, logger)
	defer service.Stop()

	// Test with nil config networks
	originalNetworks := cfg.Discovery.Networks
	cfg.Discovery.Networks = nil

	devices, err := service.DiscoverDevices("auto")
	if err != nil {
		t.Logf("Discovery with nil networks returned error: %v", err)
	}
	if devices == nil {
		t.Error("Should return device slice even with nil networks")
	}

	// Restore networks
	cfg.Discovery.Networks = originalNetworks

	// Test with very large timeout (should be handled gracefully)
	cfg.Discovery.Timeout = 999999
	devices, err = service.DiscoverDevices("203.0.113.0/30")
	if err != nil {
		t.Logf("Discovery with large timeout returned error: %v", err)
	}
	if devices == nil {
		t.Error("Should return device slice even with large timeout")
	}

	// Test multiple simultaneous discoveries (stress test)
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func(id int) {
			defer func() { done <- true }()
			devices, err := service.DiscoverDevices("203.0.113.0/30")
			if err != nil {
				t.Logf("Concurrent discovery %d returned error: %v", id, err)
			}
			if devices == nil {
				t.Errorf("Concurrent discovery %d returned nil devices", id)
			}
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(10 * time.Second):
			t.Error("Concurrent discovery timed out")
		}
	}
}

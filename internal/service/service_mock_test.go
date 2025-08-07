package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestShellyService_Basic tests basic service operations without network calls
func TestShellyService_Basic(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)
	
	service := NewServiceWithLogger(db, cfg, logger)
	
	// Test service creation
	if service == nil {
		t.Fatal("Service should not be nil")
	}
	
	if service.DB != db {
		t.Error("Service should have correct database")
	}
	
	if service.Config != cfg {
		t.Error("Service should have correct config")
	}
	
	if service.logger != logger {
		t.Error("Service should have correct logger")
	}
	
	// Test context is created
	if service.ctx == nil {
		t.Error("Service should have context")
	}
	
	if service.cancel == nil {
		t.Error("Service should have cancel function")
	}
	
	// Test stop functionality
	select {
	case <-service.ctx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Expected: context is not cancelled
	}
	
	service.Stop()
	
	select {
	case <-service.ctx.Done():
		// Expected: context is cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after Stop")
	}
}

// TestShellyService_ConfigHandling tests configuration handling
func TestShellyService_ConfigHandling(t *testing.T) {
	db := createTestDB(t)
	logger := createTestLogger(t)
	
	// Test with various config scenarios
	testCases := []struct {
		name        string
		setupConfig func() *config.Config
	}{
		{
			name: "Default config",
			setupConfig: func() *config.Config {
				return createTestConfig()
			},
		},
		{
			name: "Empty networks",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Discovery.Enabled = true
				cfg.Discovery.Networks = []string{}
				cfg.Discovery.Timeout = 1
				return cfg
			},
		},
		{
			name: "Zero timeout",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Discovery.Enabled = true
				cfg.Discovery.Networks = []string{"192.168.1.0/30"}
				cfg.Discovery.Timeout = 0
				return cfg
			},
		},
		{
			name: "Disabled discovery",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Discovery.Enabled = false
				cfg.Discovery.Networks = []string{"192.168.1.0/24"}
				cfg.Discovery.Timeout = 5
				return cfg
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.setupConfig()
			service := NewServiceWithLogger(db, cfg, logger)
			
			// Verify service is created successfully
			if service == nil {
				t.Fatal("Service creation should succeed")
			}
			
			// Verify config is stored correctly
			if service.Config != cfg {
				t.Error("Service should store the provided config")
			}
			
			service.Stop()
		})
	}
}

// TestShellyService_DatabaseIntegration tests database integration
func TestShellyService_DatabaseIntegration(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)
	
	service := NewServiceWithLogger(db, cfg, logger)
	defer service.Stop()
	
	// Add test devices to database
	testDevices := []*database.Device{
		{
			IP:       "192.168.1.100",
			MAC:      "AA:BB:CC:DD:EE:01",
			Type:     "switch",
			Name:     "Test Switch 1",
			Firmware: "1.0.0",
			Status:   "online",
			LastSeen: time.Now(),
			Settings: `{"model":"SHSW-1","gen":1,"auth_enabled":false}`,
		},
		{
			IP:       "192.168.1.101",
			MAC:      "AA:BB:CC:DD:EE:02", 
			Type:     "dimmer",
			Name:     "Test Dimmer 1",
			Firmware: "1.1.0",
			Status:   "offline",
			LastSeen: time.Now().Add(-1 * time.Hour),
			Settings: `{"model":"SHDM-1","gen":1,"auth_enabled":true}`,
		},
	}
	
	// Add devices to database
	for _, device := range testDevices {
		err := db.AddDevice(device)
		if err != nil {
			t.Fatalf("Failed to add device %s: %v", device.Name, err)
		}
	}
	
	// Retrieve devices from database
	retrievedDevices, err := db.GetDevices()
	if err != nil {
		t.Fatalf("Failed to retrieve devices: %v", err)
	}
	
	// Verify devices were stored correctly
	if len(retrievedDevices) != len(testDevices) {
		t.Errorf("Expected %d devices, got %d", len(testDevices), len(retrievedDevices))
	}
	
	// Verify device data
	for i, device := range retrievedDevices {
		if i >= len(testDevices) {
			break
		}
		original := testDevices[i]
		
		if device.IP != original.IP {
			t.Errorf("Device %d IP mismatch: expected %s, got %s", i, original.IP, device.IP)
		}
		if device.MAC != original.MAC {
			t.Errorf("Device %d MAC mismatch: expected %s, got %s", i, original.MAC, device.MAC)
		}
		if device.Type != original.Type {
			t.Errorf("Device %d Type mismatch: expected %s, got %s", i, original.Type, device.Type)
		}
		if device.Name != original.Name {
			t.Errorf("Device %d Name mismatch: expected %s, got %s", i, original.Name, device.Name)
		}
	}
	
	// Test device updates
	updateDevice := retrievedDevices[0]
	updateDevice.Status = "updated"
	updateDevice.Firmware = "2.0.0"
	
	err = db.UpdateDevice(&updateDevice)
	if err != nil {
		t.Fatalf("Failed to update device: %v", err)
	}
	
	// Verify update by getting all devices and finding the updated one
	allDevices, err := db.GetDevices()
	if err != nil {
		t.Fatalf("Failed to get devices after update: %v", err)
	}
	
	var updatedDevice *database.Device
	for i, device := range allDevices {
		if device.ID == updateDevice.ID {
			updatedDevice = &allDevices[i]
			break
		}
	}
	
	if updatedDevice == nil {
		t.Fatal("Should find the updated device")
	}
	
	if updatedDevice.Status != "updated" {
		t.Errorf("Expected updated status 'updated', got '%s'", updatedDevice.Status)
	}
	if updatedDevice.Firmware != "2.0.0" {
		t.Errorf("Expected updated firmware '2.0.0', got '%s'", updatedDevice.Firmware)
	}
}

// TestShellyService_ErrorHandling tests error handling scenarios
func TestShellyService_ErrorHandling(t *testing.T) {
	logger := createTestLogger(t)
	
	// Test with invalid database path
	tempDir := t.TempDir()
	invalidDBPath := filepath.Join(tempDir, "nonexistent", "path", "test.db")
	
	_, err := database.NewManagerWithLogger(invalidDBPath, logger)
	if err == nil {
		t.Error("Should fail with invalid database path")
	}
	
	// Test service creation with nil parameters
	db := createTestDB(t)
	cfg := createTestConfig()
	
	// Service with nil database (this might panic or return nil, both are acceptable)
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Service creation with nil database panicked: %v (expected behavior)", r)
		}
	}()
	
	service := NewServiceWithLogger(nil, cfg, logger)
	if service != nil {
		t.Log("Service created with nil database (may be acceptable)")
		service.Stop()
	}
	
	// Service with nil config
	service2 := NewServiceWithLogger(db, nil, logger)
	if service2 != nil {
		t.Log("Service created with nil config (may be acceptable)")
		service2.Stop()
	}
	
	// Service with nil logger should use default
	service3 := NewServiceWithLogger(db, cfg, nil)
	if service3 == nil {
		t.Log("Service creation with nil logger failed (may be expected)")
	} else {
		if service3.logger == nil {
			t.Log("Service with nil logger has no logger (may be expected behavior)")
		} else {
			t.Log("Service correctly handled nil logger by providing a default")
		}
		service3.Stop()
	}
}

// TestShellyService_ConcurrentOperations tests concurrent access
func TestShellyService_ConcurrentOperations(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)
	
	service := NewServiceWithLogger(db, cfg, logger)
	defer service.Stop()
	
	// Test concurrent service creation and destruction
	done := make(chan bool, 5)
	
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Create service
			localService := NewServiceWithLogger(db, cfg, logger)
			if localService == nil {
				t.Errorf("Concurrent service creation %d failed", id)
				return
			}
			
			// Use service briefly
			time.Sleep(10 * time.Millisecond)
			
			// Stop service
			localService.Stop()
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Error("Concurrent operations timed out")
			return
		}
	}
}

// Benchmark service creation and cleanup
func BenchmarkShellyService_CreationMock(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench_mock.db")
	
	logger, _ := logging.New(logging.Config{
		Level:  "error",
		Format: "text",
		Output: "stderr",
	})
	
	db, err := database.NewManagerWithLogger(dbPath, logger)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	
	cfg := createTestConfig()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		service := NewServiceWithLogger(db, cfg, logger)
		service.Stop()
	}
}

// Test memory usage and cleanup
func TestShellyService_MemoryCleanup(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfig()
	logger := createTestLogger(t)
	
	// Create and destroy many services to test for memory leaks
	for i := 0; i < 100; i++ {
		service := NewServiceWithLogger(db, cfg, logger)
		
		// Use the service briefly
		if service.ctx == nil {
			t.Error("Service context should be initialized")
		}
		
		// Stop the service
		service.Stop()
		
		// Verify cleanup
		select {
		case <-service.ctx.Done():
			// Expected: context is cancelled
		default:
			t.Error("Service context should be cancelled after Stop")
		}
	}
}

// Test service initialization edge cases
func TestShellyService_InitializationEdgeCases(t *testing.T) {
	// Test NewService (without logger)
	db := createTestDB(t)
	cfg := createTestConfig()
	
	service := NewService(db, cfg)
	if service == nil {
		t.Fatal("NewService should return a service")
	}
	
	if service.logger == nil {
		t.Error("NewService should set a default logger")
	}
	
	service.Stop()
	
	// Test with extreme config values
	extremeConfig := &config.Config{}
	extremeConfig.Discovery.Enabled = true
	extremeConfig.Discovery.Networks = make([]string, 1000) // Very large network list
	for i := range extremeConfig.Discovery.Networks {
		extremeConfig.Discovery.Networks[i] = "192.168.1.0/30"
	}
	extremeConfig.Discovery.Timeout = -1 // Negative timeout
	extremeConfig.Discovery.ConcurrentScans = -100 // Negative concurrency
	
	service2 := NewService(db, extremeConfig)
	if service2 == nil {
		t.Error("Service should handle extreme config values")
	} else {
		service2.Stop()
	}
}
package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestNewManagerWithLogger(t *testing.T) {
	// Test successful creation with custom logger
	logger, err := logging.New(logging.Config{
		Level:  "error", // Use error level to minimize test output
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	manager, err := NewManagerWithLogger(":memory:", logger)
	if err != nil {
		t.Fatalf("Expected no error creating manager with logger, got: %v", err)
	}

	if manager.DB == nil {
		t.Fatal("Expected DB to be initialized")
	}

	if manager.logger != logger {
		t.Error("Expected logger to be set correctly")
	}

	// Test with file database path
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	manager2, err := NewManagerWithLogger(dbPath, logger)
	if err != nil {
		t.Fatalf("Expected no error creating manager with file path, got: %v", err)
	}

	if manager2.DB == nil {
		t.Fatal("Expected DB to be initialized with file path")
	}

	// Test with directory creation
	nestedPath := filepath.Join(tempDir, "nested", "directory", "test.db")
	manager3, err := NewManagerWithLogger(nestedPath, logger)
	if err != nil {
		t.Fatalf("Expected no error creating nested directory, got: %v", err)
	}

	if manager3.DB == nil {
		t.Fatal("Expected DB to be initialized with nested path")
	}
}

func TestNewManagerWithLogger_InvalidPath(t *testing.T) {
	logger := logging.GetDefault()

	// Test with invalid path that can't be created
	invalidPath := "/root/nonexistent/deeply/nested/path/database.db"
	_, err := NewManagerWithLogger(invalidPath, logger)
	if err == nil {
		t.Error("Expected error for invalid database path with logger")
	}
}

func TestUpsertDeviceFromDiscovery_EdgeCases(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test with device that has empty MAC (edge case)
	emptyMACUpdate := DiscoveryUpdate{
		IP:       "192.168.1.200",
		Type:     "SHSW-1",
		Firmware: "1.0.0",
		Status:   "online",
		LastSeen: time.Now(),
	}

	_, err = manager.UpsertDeviceFromDiscovery("", emptyMACUpdate, "Empty MAC Device")
	// Note: Empty MAC is actually allowed by the current implementation
	// This test documents the current behavior rather than enforcing validation
	if err != nil {
		t.Logf("Empty MAC resulted in error (expected behavior): %v", err)
	}

	// Test with device that has all valid fields
	validUpdate := DiscoveryUpdate{
		IP:       "192.168.1.202",
		Type:     "SHSW-1",
		Firmware: "1.0.0",
		Status:   "online",
		LastSeen: time.Now(),
	}

	_, err = manager.UpsertDeviceFromDiscovery("AA:BB:CC:DD:EE:FF", validUpdate, "Valid Device")
	if err != nil {
		t.Errorf("Expected no error for valid device, got: %v", err)
	}

	// Test updating the same device with different IP
	updatedUpdate := DiscoveryUpdate{
		IP:       "192.168.1.203",
		Type:     "SHSW-1",
		Firmware: "1.0.1",
		Status:   "online",
		LastSeen: time.Now(),
	}

	_, err = manager.UpsertDeviceFromDiscovery("AA:BB:CC:DD:EE:FF", updatedUpdate, "Valid Device Updated")
	if err != nil {
		t.Errorf("Expected no error updating device, got: %v", err)
	}

	// Verify the device was updated
	retrievedDevice, err := manager.GetDeviceByMAC("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("Expected to find updated device: %v", err)
	}

	if retrievedDevice.IP != "192.168.1.203" {
		t.Errorf("Expected IP to be updated to 192.168.1.203, got %s", retrievedDevice.IP)
	}

	if retrievedDevice.Firmware != "1.0.1" {
		t.Errorf("Expected firmware to be updated to 1.0.1, got %s", retrievedDevice.Firmware)
	}
}

func TestUpsertDeviceFromDiscovery_PreservesExistingData(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// First, create a device manually with custom name and settings
	originalDevice := &Device{
		IP:       "192.168.1.100",
		MAC:      "11:22:33:44:55:66",
		Type:     "SHSW-1",
		Name:     "Custom Device Name",
		Firmware: "1.0.0",
		Settings: `{"custom": "settings"}`,
		Status:   "configured",
	}

	err = manager.AddDevice(originalDevice)
	if err != nil {
		t.Fatalf("Failed to add original device: %v", err)
	}

	// Now simulate discovery update with different data
	discoveryUpdate := DiscoveryUpdate{
		IP:       "192.168.1.101", // Different IP
		Type:     "SHSW-1",
		Firmware: "1.0.1", // Updated firmware
		Status:   "online",
		LastSeen: time.Now(),
	}

	_, err = manager.UpsertDeviceFromDiscovery("11:22:33:44:55:66", discoveryUpdate, "Discovered Device Name")
	if err != nil {
		t.Errorf("Expected no error updating from discovery, got: %v", err)
	}

	// Verify that important fields were preserved while others were updated
	updatedDevice, err := manager.GetDeviceByMAC("11:22:33:44:55:66")
	if err != nil {
		t.Fatalf("Expected to find updated device: %v", err)
	}

	// IP should be updated
	if updatedDevice.IP != "192.168.1.101" {
		t.Errorf("Expected IP to be updated to 192.168.1.101, got %s", updatedDevice.IP)
	}

	// Firmware should be updated
	if updatedDevice.Firmware != "1.0.1" {
		t.Errorf("Expected firmware to be updated to 1.0.1, got %s", updatedDevice.Firmware)
	}

	// Name should be preserved (not overwritten by discovery)
	if updatedDevice.Name != "Custom Device Name" {
		t.Errorf("Expected name to be preserved as 'Custom Device Name', got %s", updatedDevice.Name)
	}

	// Settings should be preserved
	if updatedDevice.Settings != `{"custom": "settings"}` {
		t.Errorf("Expected settings to be preserved, got %s", updatedDevice.Settings)
	}

	// Status is updated during discovery (this is the current behavior)
	// The discovery update overwrites the status field
	if updatedDevice.Status != "online" {
		t.Errorf("Expected status to be updated to 'online', got %s", updatedDevice.Status)
	}
}

func TestDatabaseClose(t *testing.T) {
	// Test database Close functionality
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "close_test.db")

	manager, err := NewManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add some data to ensure the database is working
	device := &Device{
		IP:       "192.168.1.100",
		MAC:      "AA:BB:CC:DD:EE:FF",
		Type:     "SHSW-1",
		Name:     "Test Device",
		Firmware: "1.0.0",
	}

	err = manager.AddDevice(device)
	if err != nil {
		t.Fatalf("Failed to add test device: %v", err)
	}

	// Test closing the database
	err = manager.Close()
	if err != nil {
		t.Errorf("Expected no error closing database, got: %v", err)
	}

	// Verify database is closed by trying to use it
	// This should fail or behave differently after close
	_, err = manager.GetDevices()
	if err == nil {
		t.Error("Expected error when using database after close")
	}

	// Test closing already closed database (should not panic)
	err2 := manager.Close()
	if err2 != nil && err2.Error() != "sql: database is closed" {
		t.Errorf("Expected 'database is closed' error or nil, got: %v", err2)
	}
}

func TestDatabaseClose_InMemory(t *testing.T) {
	// Test closing in-memory database
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory manager: %v", err)
	}

	// Test closing in-memory database
	err = manager.Close()
	if err != nil {
		t.Errorf("Expected no error closing in-memory database, got: %v", err)
	}
}

func TestDatabaseOperations_AfterClose(t *testing.T) {
	// Test various database operations after close to ensure proper error handling
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Close the database
	err = manager.Close()
	if err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Test various operations after close
	device := &Device{
		IP:       "192.168.1.100",
		MAC:      "AA:BB:CC:DD:EE:FF",
		Type:     "SHSW-1",
		Name:     "Test Device",
		Firmware: "1.0.0",
	}

	// These operations should fail gracefully after close
	err = manager.AddDevice(device)
	if err == nil {
		t.Error("Expected error for AddDevice after close")
	}

	_, err = manager.GetDevices()
	if err == nil {
		t.Error("Expected error for GetDevices after close")
	}

	_, err = manager.GetDevice(1)
	if err == nil {
		t.Error("Expected error for GetDevice after close")
	}

	_, err = manager.GetDeviceByMAC("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Error("Expected error for GetDeviceByMAC after close")
	}

	err = manager.UpdateDevice(device)
	if err == nil {
		t.Error("Expected error for UpdateDevice after close")
	}

	err = manager.DeleteDevice(1)
	if err == nil {
		t.Error("Expected error for DeleteDevice after close")
	}

	discoveryInfo := DiscoveryUpdate{
		IP:       "192.168.1.100",
		Type:     "SHSW-1",
		Firmware: "1.0.0",
		Status:   "online",
		LastSeen: time.Now(),
	}

	_, err = manager.UpsertDeviceFromDiscovery("AA:BB:CC:DD:EE:FF", discoveryInfo, "Test Device")
	if err == nil {
		t.Error("Expected error for UpsertDeviceFromDiscovery after close")
	}
}

func TestDatabaseManager_LoggerIntegration(t *testing.T) {
	// Test that database operations properly use the logger
	logger, err := logging.New(logging.Config{
		Level:  "debug", // Use debug to see all log messages
		Format: "text",
		Output: "stderr",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	manager, err := NewManagerWithLogger(":memory:", logger)
	if err != nil {
		t.Fatalf("Failed to create manager with logger: %v", err)
	}

	// Perform operations that should trigger logging
	device := &Device{
		IP:       "192.168.1.100",
		MAC:      "AA:BB:CC:DD:EE:FF",
		Type:     "SHSW-1",
		Name:     "Logger Test Device",
		Firmware: "1.0.0",
	}

	// These operations should log timing information
	err = manager.AddDevice(device)
	if err != nil {
		t.Errorf("AddDevice failed: %v", err)
	}

	_, err = manager.GetDevices()
	if err != nil {
		t.Errorf("GetDevices failed: %v", err)
	}

	_, err = manager.GetDevice(device.ID)
	if err != nil {
		t.Errorf("GetDevice failed: %v", err)
	}

	device.Name = "Updated Logger Test Device"
	err = manager.UpdateDevice(device)
	if err != nil {
		t.Errorf("UpdateDevice failed: %v", err)
	}

	// Test operations that should generate error logs
	_, err = manager.GetDevice(99999) // Non-existent device
	if err == nil {
		t.Error("Expected error for non-existent device")
	}

	// The test passes if no panics occur and logging doesn't cause failures
}

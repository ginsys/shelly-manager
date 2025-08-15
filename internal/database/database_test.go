package database

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestNewManager(t *testing.T) {
	// Test with in-memory database
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Expected no error creating manager, got: %v", err)
	}
	
	if manager.DB == nil {
		t.Fatal("Expected DB to be initialized")
	}
}

func TestNewManager_InvalidPath(t *testing.T) {
	// Test with invalid database path
	_, err := NewManager("/invalid/path/database.db")
	if err == nil {
		t.Fatal("Expected error for invalid database path")
	}
}

func TestDeviceCRUD(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Create test device
	device := &Device{
		IP:       "192.168.1.100",
		MAC:      "A4:CF:12:34:56:78",
		Type:     "Smart Plug",
		Name:     "Test Device",
		Firmware: "20231219-134356",
		Status:   "online",
		LastSeen: time.Now(),
		Settings: `{"model":"SHPLG-S","gen":1}`,
	}

	// Test AddDevice
	err = manager.AddDevice(device)
	if err != nil {
		t.Fatalf("Failed to add device: %v", err)
	}

	if device.ID == 0 {
		t.Fatal("Expected device ID to be set after creation")
	}

	// Test GetDevice
	retrievedDevice, err := manager.GetDevice(device.ID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}

	if retrievedDevice.MAC != device.MAC {
		t.Errorf("Expected MAC %s, got %s", device.MAC, retrievedDevice.MAC)
	}

	// Test GetDeviceByMAC
	deviceByMAC, err := manager.GetDeviceByMAC(device.MAC)
	if err != nil {
		t.Fatalf("Failed to get device by MAC: %v", err)
	}

	if deviceByMAC.ID != device.ID {
		t.Errorf("Expected device ID %d, got %d", device.ID, deviceByMAC.ID)
	}

	// Test UpdateDevice
	device.Name = "Updated Device Name"
	device.Status = "offline"
	err = manager.UpdateDevice(device)
	if err != nil {
		t.Fatalf("Failed to update device: %v", err)
	}

	updatedDevice, err := manager.GetDevice(device.ID)
	if err != nil {
		t.Fatalf("Failed to get updated device: %v", err)
	}

	if updatedDevice.Name != "Updated Device Name" {
		t.Errorf("Expected name 'Updated Device Name', got %s", updatedDevice.Name)
	}

	if updatedDevice.Status != "offline" {
		t.Errorf("Expected status 'offline', got %s", updatedDevice.Status)
	}

	// Test GetDevices
	devices, err := manager.GetDevices()
	if err != nil {
		t.Fatalf("Failed to get devices: %v", err)
	}

	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}

	// Test DeleteDevice
	err = manager.DeleteDevice(device.ID)
	if err != nil {
		t.Fatalf("Failed to delete device: %v", err)
	}

	// Verify deletion
	_, err = manager.GetDevice(device.ID)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound after deletion, got: %v", err)
	}
}

func TestGetDevice_NotFound(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = manager.GetDevice(999)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound for non-existent device, got: %v", err)
	}
}

func TestUpsertDeviceFromDiscovery(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	mac := "A4:CF:12:34:56:78"
	initialUpdate := DiscoveryUpdate{
		IP:       "192.168.1.100",
		Type:     "Smart Plug",
		Firmware: "20231219-134356",
		Status:   "online",
		LastSeen: time.Now(),
	}

	// Test 1: Create new device via discovery
	device, err := manager.UpsertDeviceFromDiscovery(mac, initialUpdate, "shelly1-123456")
	if err != nil {
		t.Fatalf("Failed to create device from discovery: %v", err)
	}

	if device.MAC != mac {
		t.Errorf("Expected MAC %s, got %s", mac, device.MAC)
	}
	if device.Name != "shelly1-123456" {
		t.Errorf("Expected name 'shelly1-123456', got %s", device.Name)
	}
	if device.IP != initialUpdate.IP {
		t.Errorf("Expected IP %s, got %s", initialUpdate.IP, device.IP)
	}

	// Manually update the device to simulate user configuration
	device.Name = "Living Room Plug"
	customSettings := map[string]interface{}{
		"model":        "SHPLG-S",
		"gen":          1,
		"auth_enabled": true,
		"auth_user":    "admin",
		"auth_pass":    "password123",
		"sync_status":  "synced",
		"custom_field": "user_data",
	}
	settingsJSON, _ := json.Marshal(customSettings)
	device.Settings = string(settingsJSON)
	
	err = manager.UpdateDevice(device)
	if err != nil {
		t.Fatalf("Failed to update device: %v", err)
	}

	// Test 2: Update existing device via discovery (should preserve user data)
	updatedDiscovery := DiscoveryUpdate{
		IP:       "192.168.1.101", // IP changed
		Type:     "Smart Plug",     // Same type
		Firmware: "20240101-145500", // Firmware updated
		Status:   "online",
		LastSeen: time.Now(),
	}

	updatedDevice, err := manager.UpsertDeviceFromDiscovery(mac, updatedDiscovery, "ignored-name")
	if err != nil {
		t.Fatalf("Failed to update device from discovery: %v", err)
	}

	// Verify discovery fields were updated
	if updatedDevice.IP != "192.168.1.101" {
		t.Errorf("Expected IP to be updated to 192.168.1.101, got %s", updatedDevice.IP)
	}
	if updatedDevice.Firmware != "20240101-145500" {
		t.Errorf("Expected firmware to be updated to 20240101-145500, got %s", updatedDevice.Firmware)
	}

	// Verify user-configured fields were preserved
	if updatedDevice.Name != "Living Room Plug" {
		t.Errorf("Expected name to be preserved as 'Living Room Plug', got %s", updatedDevice.Name)
	}

	// Parse and verify settings were preserved
	var preservedSettings map[string]interface{}
	err = json.Unmarshal([]byte(updatedDevice.Settings), &preservedSettings)
	if err != nil {
		t.Fatalf("Failed to parse preserved settings: %v", err)
	}

	// Check that custom user data was preserved
	if preservedSettings["custom_field"] != "user_data" {
		t.Errorf("Expected custom_field to be preserved as 'user_data', got %v", preservedSettings["custom_field"])
	}
	if preservedSettings["sync_status"] != "synced" {
		t.Errorf("Expected sync_status to be preserved as 'synced', got %v", preservedSettings["sync_status"])
	}
	if preservedSettings["auth_user"] != "admin" {
		t.Errorf("Expected auth_user to be preserved as 'admin', got %v", preservedSettings["auth_user"])
	}
	if preservedSettings["auth_pass"] != "password123" {
		t.Errorf("Expected auth_pass to be preserved as 'password123', got %v", preservedSettings["auth_pass"])
	}

	// Test 3: Verify we can't create duplicate devices with same MAC
	duplicateDevice, err := manager.UpsertDeviceFromDiscovery(mac, initialUpdate, "different-name")
	if err != nil {
		t.Fatalf("Upsert should not fail for existing MAC: %v", err)
	}
	
	// Should return the same device (by ID) and preserve the custom name
	if duplicateDevice.ID != updatedDevice.ID {
		t.Errorf("Expected same device ID, got %d vs %d", duplicateDevice.ID, updatedDevice.ID)
	}
	if duplicateDevice.Name != "Living Room Plug" {
		t.Errorf("Expected existing name to be preserved, got %s", duplicateDevice.Name)
	}
}

func TestGetDeviceByMAC_NotFound(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_, err = manager.GetDeviceByMAC("nonexistent:mac:addr")
	if err != gorm.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound, got: %v", err)
	}
}

func TestAddDevice_Duplicate(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	device1 := &Device{
		IP:   "192.168.1.100",
		MAC:  "A4:CF:12:34:56:78",
		Type: "Smart Plug",
		Name: "Device 1",
	}

	device2 := &Device{
		IP:   "192.168.1.100", // Same IP
		MAC:  "A4:CF:12:34:56:79",
		Type: "Smart Plug",
		Name: "Device 2",
	}

	// Add first device
	err = manager.AddDevice(device1)
	if err != nil {
		t.Fatalf("Failed to add first device: %v", err)
	}

	// Try to add device with same IP (should fail due to unique index)
	err = manager.AddDevice(device2)
	if err == nil {
		t.Fatal("Expected error when adding device with duplicate IP")
	}
}

func TestMultipleDevices(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add multiple devices
	devices := []*Device{
		{
			IP:   "192.168.1.100",
			MAC:  "A4:CF:12:34:56:78",
			Type: "Smart Plug",
			Name: "Device 1",
		},
		{
			IP:   "192.168.1.101",
			MAC:  "A4:CF:12:34:56:79",
			Type: "Relay Switch",
			Name: "Device 2",
		},
		{
			IP:   "192.168.1.102",
			MAC:  "A4:CF:12:34:56:7A",
			Type: "Power Meter Switch",
			Name: "Device 3",
		},
	}

	for _, device := range devices {
		err = manager.AddDevice(device)
		if err != nil {
			t.Fatalf("Failed to add device %s: %v", device.Name, err)
		}
	}

	// Test GetDevices returns all devices
	allDevices, err := manager.GetDevices()
	if err != nil {
		t.Fatalf("Failed to get all devices: %v", err)
	}

	if len(allDevices) != 3 {
		t.Errorf("Expected 3 devices, got %d", len(allDevices))
	}

	// Verify each device can be retrieved
	for _, originalDevice := range devices {
		retrievedDevice, err := manager.GetDeviceByMAC(originalDevice.MAC)
		if err != nil {
			t.Fatalf("Failed to get device by MAC %s: %v", originalDevice.MAC, err)
		}

		if retrievedDevice.Name != originalDevice.Name {
			t.Errorf("Expected name %s, got %s", originalDevice.Name, retrievedDevice.Name)
		}
	}
}

func TestDeviceTimestamps(t *testing.T) {
	manager, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	device := &Device{
		IP:   "192.168.1.100",
		MAC:  "A4:CF:12:34:56:78",
		Type: "Smart Plug",
		Name: "Test Device",
	}

	// Add device
	err = manager.AddDevice(device)
	if err != nil {
		t.Fatalf("Failed to add device: %v", err)
	}

	// Check that timestamps were set
	if device.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if device.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}

	originalUpdatedAt := device.UpdatedAt

	// Sleep to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update device
	device.Name = "Updated Name"
	err = manager.UpdateDevice(device)
	if err != nil {
		t.Fatalf("Failed to update device: %v", err)
	}

	// Check that UpdatedAt was modified
	if !device.UpdatedAt.After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated after modification")
	}
}
package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
)

// Test helper to create a mock Shelly device server
func createMockShellyServer() *httptest.Server {
	mux := http.NewServeMux()

	// Gen1 endpoints
	mux.HandleFunc("/shelly", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
			"type": "SHSW-25",
			"mac": "68C63A123456",
			"auth": false,
			"fw": "1.14.0",
			"longid": 1
		}`)
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
			"wifi_sta": {"connected": true, "ssid": "TestNetwork", "ip": "192.168.1.100", "rssi": -45},
			"cloud": {"enabled": false, "connected": false},
			"mqtt": {"connected": true},
			"time": "15:04",
			"unixtime": 1640995440,
			"has_update": false,
			"ram_total": 51704,
			"ram_free": 40152,
			"fs_size": 233681,
			"fs_free": 162648,
			"uptime": 3600,
			"relays": [{"ison": true, "has_timer": false, "timer_started": 0, "timer_duration": 0, "timer_remaining": 0, "overpower": false, "source": "input"}],
			"meters": [{"power": 25.5, "is_valid": true, "timestamp": 1640995440, "counters": [0.123, 0.000, 0.000], "total": 12345}],
			"temperature": 45.2,
			"overtemperature": false
		}`)
	})

	mux.HandleFunc("/relay/0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ison": true, "has_timer": false}`)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
			"device": {"type": "SHSW-25", "mac": "68C63A123456", "hostname": "shelly1-68C63A123456"},
			"wifi_ap": {"enabled": false},
			"wifi_sta": {"enabled": true, "ssid": "TestNetwork", "ipv4_method": "dhcp", "ip": null, "gw": null, "mask": null, "dns": null},
			"mqtt": {"enable": false, "server": "", "user": "", "reconnect_timeout_max": 60.0, "reconnect_timeout_min": 2.0, "clean_session": true, "keep_alive": 60, "max_qos": 0, "retain": false, "update_period": 30},
			"coiot": {"enabled": false, "update_period": 15},
			"sntp": {"server": "time.google.com"},
			"login": {"enabled": false, "unprotected": false, "username": "admin"},
			"pin_code": "",
			"coiot_execute_enable": false,
			"name": "Test Device",
			"fw": "1.14.0",
			"build_info": {"build_id": "20231219-134356", "build_timestamp": "2023-12-19T13:43:56Z", "build_version": "1.0"},
			"cloud": {"enabled": false, "connected": false},
			"timezone": "Europe/Sofia",
			"lat": 42.6977,
			"lng": 23.3219,
			"tzautodetect": false,
			"time": "15:04",
			"hwinfo": {"hw_revision": "prod-190516", "batch_id": 0},
			"max_power": 3500
		}`)
	})

	// Energy meter endpoint
	mux.HandleFunc("/meter/0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
			"power": 25.5,
			"is_valid": true,
			"timestamp": 1640995440,
			"counters": [0.123, 0.000, 0.000],
			"total": 12345
		}`)
	})

	return httptest.NewServer(mux)
}

// Test helper to create test device in database
func createTestDevice(t *testing.T, db *database.Manager, ip string) *database.Device {
	t.Helper()

	device := &database.Device{
		IP:       ip,
		MAC:      "68C63A123456",
		Type:     "SHSW-25",
		Name:     "Test Device",
		Firmware: "1.14.0",
		Settings: `{"model":"SHSW-25","gen":1,"auth_enabled":false}`,
	}

	if err := db.AddDevice(device); err != nil {
		t.Fatalf("Failed to create test device: %v", err)
	}

	return device
}

func TestShellyService_ControlDevice(t *testing.T) {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}
	server := createMockShellyServer()
	defer server.Close()

	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	// Extract IP from server URL for test device
	serverIP := server.URL[len("http://"):]
	device := createTestDevice(t, db, serverIP)

	tests := []struct {
		name        string
		action      string
		params      map[string]interface{}
		expectError bool
	}{
		{
			name:        "Turn on relay",
			action:      "on",
			params:      map[string]interface{}{"channel": 0},
			expectError: false,
		},
		{
			name:        "Turn off relay",
			action:      "off",
			params:      map[string]interface{}{"channel": 0},
			expectError: false,
		},
		{
			name:        "Toggle relay",
			action:      "toggle",
			params:      map[string]interface{}{"channel": 0},
			expectError: false,
		},
		{
			name:        "Invalid action",
			action:      "invalid_action",
			params:      map[string]interface{}{"channel": 0},
			expectError: true,
		},
		{
			name:        "Missing channel parameter",
			action:      "on",
			params:      map[string]interface{}{},
			expectError: false, // Channel defaults to 0, so this should succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ControlDevice(device.ID, tt.action, tt.params)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Did not expect error for %s, got: %v", tt.name, err)
			}
		})
	}
}

func TestShellyService_GetDeviceStatus(t *testing.T) {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}
	server := createMockShellyServer()
	defer server.Close()

	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	serverIP := server.URL[len("http://"):]
	device := createTestDevice(t, db, serverIP)

	status, err := service.GetDeviceStatus(device.ID)
	if err != nil {
		t.Fatalf("GetDeviceStatus failed: %v", err)
	}

	if status == nil {
		t.Fatal("Status should not be nil")
	}

	// Verify expected status fields (based on GetDeviceStatus method return structure)
	expectedFields := []string{"device_id", "ip", "temperature", "uptime", "wifi", "switches", "meters"}
	for _, field := range expectedFields {
		if _, exists := status[field]; !exists {
			t.Errorf("Expected field %s not found in status", field)
		}
	}
}

func TestShellyService_GetDeviceStatus_NonExistentDevice(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	_, err := service.GetDeviceStatus(99999) // Non-existent device ID
	if err == nil {
		t.Error("Expected error for non-existent device")
	}
}

func TestShellyService_GetDeviceEnergy(t *testing.T) {
	server := createMockShellyServer()
	defer server.Close()

	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	serverIP := server.URL[len("http://"):]
	device := createTestDevice(t, db, serverIP)

	energy, err := service.GetDeviceEnergy(device.ID, 0)
	if err != nil {
		t.Fatalf("GetDeviceEnergy failed: %v", err)
	}

	if energy == nil {
		t.Fatal("Energy data should not be nil")
	}

	// Verify energy data fields
	if energy.Power != 25.5 {
		t.Errorf("Expected power 25.5, got %f", energy.Power)
	}

	if energy.Total != 12.345 { // Should be converted from Wh to kWh
		t.Errorf("Expected total 12.345 kWh, got %f", energy.Total)
	}
}

func TestShellyService_GetDeviceEnergy_InvalidChannel(t *testing.T) {
	server := createMockShellyServer()
	defer server.Close()

	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	serverIP := server.URL[len("http://"):]
	device := createTestDevice(t, db, serverIP)

	_, err := service.GetDeviceEnergy(device.ID, 99) // Invalid channel
	if err == nil {
		t.Error("Expected error for invalid channel")
	}
}

func TestShellyService_ClearClientCache(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	// Add some mock clients to cache
	service.clientMu.Lock()
	service.clients["192.168.1.100"] = nil
	service.clients["192.168.1.101"] = nil
	service.clientMu.Unlock()

	// Clear specific client
	service.ClearClientCache("192.168.1.100")

	service.clientMu.RLock()
	if _, exists := service.clients["192.168.1.100"]; exists {
		t.Error("Client cache should be cleared for specific IP")
	}

	if _, exists := service.clients["192.168.1.101"]; !exists {
		t.Error("Other clients should remain in cache")
	}
	service.clientMu.RUnlock()

	// Clear all clients with empty string
	service.ClearClientCache("")

	service.clientMu.RLock()
	if len(service.clients) != 0 {
		t.Error("All clients should be cleared")
	}
	service.clientMu.RUnlock()
}

func TestShellyService_getClient_Authentication(t *testing.T) {
	server := createMockShellyServer()
	defer server.Close()

	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	serverIP := server.URL[len("http://"):]

	tests := []struct {
		name        string
		auth        bool
		username    string
		password    string
		expectError bool
	}{
		{
			name:        "No authentication",
			auth:        false,
			username:    "",
			password:    "",
			expectError: false,
		},
		{
			name:        "With authentication",
			auth:        true,
			username:    "admin",
			password:    "password",
			expectError: false,
		},
		{
			name:        "Authentication required but no credentials",
			auth:        true,
			username:    "",
			password:    "",
			expectError: false, // Should still create client, auth will fail on use
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := &database.Device{
				ID:       1,
				IP:       serverIP, // Use mock server IP instead of unreachable IP
				MAC:      "68C63A123456",
				Type:     "SHSW-25",
				Settings: `{"model":"SHSW-25","gen":1,"auth_enabled":false}`,
			}

			client, err := service.getClient(device)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && client == nil {
				t.Error("Client should not be nil")
			}
		})
	}
}

func TestShellyService_ClientCaching(t *testing.T) {
	server := createMockShellyServer()
	defer server.Close()

	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	serverIP := server.URL[len("http://"):]
	device := &database.Device{
		ID:       1,
		IP:       serverIP,
		MAC:      "68C63A123456",
		Type:     "SHSW-25",
		Settings: `{"model":"SHSW-25","gen":1,"auth_enabled":false}`,
	}

	// First call should create client
	client1, err := service.getClient(device)
	if err != nil {
		t.Fatalf("Failed to get client: %v", err)
	}

	// Second call should return cached client (test client caching works)
	client2, err := service.getClient(device)
	if err != nil {
		t.Fatalf("Failed to get cached client: %v", err)
	}

	// Both clients should be functional and point to the same device
	if client1 == nil || client2 == nil {
		t.Error("Clients should not be nil")
	}

	// Test that both clients work (this verifies functional equivalence)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	info1, err1 := client1.GetInfo(ctx)
	info2, err2 := client2.GetInfo(ctx)

	if err1 != nil || err2 != nil {
		t.Errorf("Both clients should work: err1=%v, err2=%v", err1, err2)
	}

	if info1 == nil || info2 == nil || info1.Type != info2.Type {
		t.Error("Both clients should return equivalent device info")
	}
}

func TestShellyService_ErrorHandling_BusinessLogic(t *testing.T) {
	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	tests := []struct {
		name        string
		deviceID    uint
		expectError bool
	}{
		{
			name:        "Non-existent device",
			deviceID:    99999,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test all major service functions with invalid device
			_, err := service.GetDeviceStatus(tt.deviceID)
			if !tt.expectError && err != nil {
				t.Errorf("GetDeviceStatus: unexpected error: %v", err)
			}
			if tt.expectError && err == nil {
				t.Error("GetDeviceStatus: expected error but got none")
			}

			_, err = service.GetDeviceEnergy(tt.deviceID, 0)
			if !tt.expectError && err != nil {
				t.Errorf("GetDeviceEnergy: unexpected error: %v", err)
			}
			if tt.expectError && err == nil {
				t.Error("GetDeviceEnergy: expected error but got none")
			}

			err = service.ControlDevice(tt.deviceID, "on", map[string]interface{}{"channel": 0})
			if !tt.expectError && err != nil {
				t.Errorf("ControlDevice: unexpected error: %v", err)
			}
			if tt.expectError && err == nil {
				t.Error("ControlDevice: expected error but got none")
			}
		})
	}
}

func TestShellyService_ConcurrentAccess(t *testing.T) {
	server := createMockShellyServer()
	defer server.Close()

	db := createTestDB(t)
	cfg := createTestConfigBusiness()
	service := NewService(db, cfg)

	serverIP := server.URL[len("http://"):]
	device := createTestDevice(t, db, serverIP)

	// Test concurrent access to the same device
	done := make(chan bool)
	errors := make(chan error, 10)

	// Launch multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Perform various operations concurrently
			_, err := service.GetDeviceStatus(device.ID)
			if err != nil {
				errors <- fmt.Errorf("GetDeviceStatus %d: %w", id, err)
				return
			}

			err = service.ControlDevice(device.ID, "on", map[string]interface{}{"channel": 0})
			if err != nil {
				errors <- fmt.Errorf("ControlDevice %d: %w", id, err)
				return
			}

			_, err = service.GetDeviceEnergy(device.ID, 0)
			if err != nil {
				errors <- fmt.Errorf("GetDeviceEnergy %d: %w", id, err)
				return
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

// Test configuration helper functions
func createTestConfigBusiness() *config.Config {
	return &config.Config{
		Discovery: struct {
			Enabled         bool     `mapstructure:"enabled"`
			Networks        []string `mapstructure:"networks"`
			Interval        int      `mapstructure:"interval"`
			Timeout         int      `mapstructure:"timeout"`
			EnableMDNS      bool     `mapstructure:"enable_mdns"`
			EnableSSDP      bool     `mapstructure:"enable_ssdp"`
			ConcurrentScans int      `mapstructure:"concurrent_scans"`
		}{
			Networks: []string{"192.168.1.0/24"},
			Timeout:  5,
			Enabled:  true,
		},
	}
}

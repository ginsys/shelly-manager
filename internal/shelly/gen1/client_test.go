package gen1

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockGen1Server creates a test server that mimics a Gen1 Shelly device
func mockGen1Server(t *testing.T) *httptest.Server {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			// Device info endpoint
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"type":        "SHSW-25",
				"mac":         "A4CF12F45678",
				"auth":        false,
				"fw":          "20230913-114008/v1.14.0-gcb84623",
				"num_outputs": 2,
				"num_meters":  2,
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}

		case "/status":
			// Status endpoint
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"wifi_sta": map[string]interface{}{
					"connected": true,
					"ssid":      "TestNetwork",
					"ip":        "192.168.1.100",
					"rssi":      -65,
				},
				"temperature":     35.5,
				"overtemperature": false,
				"uptime":          12345,
				"has_update":      false,
				"ram_total":       50592,
				"ram_free":        35640,
				"fs_size":         233681,
				"fs_free":         162648,
				"relays": []map[string]interface{}{
					{
						"ison":            true,
						"has_timer":       false,
						"timer_started":   0,
						"timer_duration":  0,
						"timer_remaining": 0,
						"source":          "http",
					},
					{
						"ison":            false,
						"has_timer":       false,
						"timer_started":   0,
						"timer_duration":  0,
						"timer_remaining": 0,
						"source":          "input",
					},
				},
				"meters": []map[string]interface{}{
					{
						"power":     15.32,
						"is_valid":  true,
						"timestamp": time.Now().Unix(),
						"counters":  []float64{0, 0, 0},
						"total":     1234,
					},
					{
						"power":     0.0,
						"is_valid":  true,
						"timestamp": time.Now().Unix(),
						"counters":  []float64{0, 0, 0},
						"total":     0,
					},
				},
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}

		case "/settings":
			switch r.Method {
			case "GET":
				// Get settings
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"name":     "Test Shelly",
					"timezone": "Europe/Amsterdam",
					"lat":      52.3676,
					"lng":      4.9041,
					"wifi_sta": map[string]interface{}{
						"enabled":     true,
						"ssid":        "TestNetwork",
						"ipv4_method": "dhcp",
					},
					"cloud": map[string]interface{}{
						"enabled": false,
					},
					"relays": []map[string]interface{}{
						{
							"name":     "Relay 1",
							"ison":     true,
							"auto_on":  0,
							"auto_off": 0,
						},
						{
							"name":     "Relay 2",
							"ison":     false,
							"auto_on":  0,
							"auto_off": 0,
						},
					},
				}); err != nil {
					t.Logf("Failed to encode JSON response: %v", err)
				}
			case "POST":
				// Set settings
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
				}); err != nil {
					t.Logf("Failed to encode JSON response: %v", err)
				}
			}

		case "/relay/0", "/relay/1":
			if r.Method == "POST" {
				// Control relay
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"ison":            r.FormValue("turn") == "on",
					"has_timer":       false,
					"timer_started":   0,
					"timer_duration":  0,
					"timer_remaining": 0,
					"source":          "http",
				}); err != nil {
					t.Logf("Failed to encode JSON response: %v", err)
				}
			}

		case "/meter/0", "/meter/1":
			// Meter endpoint
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"power":     15.32,
				"is_valid":  true,
				"timestamp": time.Now().Unix(),
				"counters":  []float64{0, 0, 0},
				"total":     1234,
				"voltage":   230.5,
				"current":   0.067,
				"pf":        0.99,
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}

		case "/ota":
			switch r.Method {
			case "GET":
				// Check for updates
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"has_update":    true,
					"new_version":   "20231015-120000/v1.14.1",
					"old_version":   "20230913-114008/v1.14.0-gcb84623",
					"release_notes": "Bug fixes and improvements",
				}); err != nil {
					t.Logf("Failed to encode JSON response: %v", err)
				}
			case "POST":
				// Trigger update
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "updating",
				}); err != nil {
					t.Logf("Failed to encode JSON response: %v", err)
				}
			}

		case "/reboot":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))

		case "/settings/factory_reset":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))

		default:
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Not found",
			}); err != nil {
				t.Logf("Failed to encode JSON response: %v", err)
			}
		}
	}))
}

func TestGen1Client_GetInfo(t *testing.T) {
	server := mockGen1Server(t)
	defer server.Close()

	// Extract IP from server URL
	ip := server.URL[7:] // Remove "http://"

	client := NewClient(ip)
	ctx := context.Background()

	info, err := client.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}

	if info.Type != "SHSW-25" {
		t.Errorf("Expected type SHSW-25, got %s", info.Type)
	}

	if info.MAC != "A4CF12F45678" {
		t.Errorf("Expected MAC A4CF12F45678, got %s", info.MAC)
	}

	if info.Generation != 1 {
		t.Errorf("Expected generation 1, got %d", info.Generation)
	}
}

func TestGen1Client_GetStatus(t *testing.T) {
	server := mockGen1Server(t)
	defer server.Close()

	ip := server.URL[7:]
	client := NewClient(ip)
	ctx := context.Background()

	status, err := client.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Temperature != 35.5 {
		t.Errorf("Expected temperature 35.5, got %f", status.Temperature)
	}

	if status.WiFiStatus == nil {
		t.Fatal("Expected WiFi status, got nil")
	}

	if !status.WiFiStatus.Connected {
		t.Error("Expected WiFi to be connected")
	}

	if len(status.Switches) != 2 {
		t.Errorf("Expected 2 switches, got %d", len(status.Switches))
	}

	if !status.Switches[0].Output {
		t.Error("Expected first switch to be on")
	}

	if status.Switches[1].Output {
		t.Error("Expected second switch to be off")
	}
}

func TestGen1Client_SetSwitch(t *testing.T) {
	server := mockGen1Server(t)
	defer server.Close()

	ip := server.URL[7:]
	client := NewClient(ip)
	ctx := context.Background()

	// Turn on relay 0
	err := client.SetSwitch(ctx, 0, true)
	if err != nil {
		t.Fatalf("SetSwitch(0, true) failed: %v", err)
	}

	// Turn off relay 1
	err = client.SetSwitch(ctx, 1, false)
	if err != nil {
		t.Fatalf("SetSwitch(1, false) failed: %v", err)
	}
}

func TestGen1Client_GetConfig(t *testing.T) {
	server := mockGen1Server(t)
	defer server.Close()

	ip := server.URL[7:]
	client := NewClient(ip)
	ctx := context.Background()

	config, err := client.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if config.Name != "Test Shelly" {
		t.Errorf("Expected name 'Test Shelly', got %s", config.Name)
	}

	if config.Timezone != "Europe/Amsterdam" {
		t.Errorf("Expected timezone 'Europe/Amsterdam', got %s", config.Timezone)
	}

	if config.WiFi == nil {
		t.Fatal("Expected WiFi config, got nil")
	}

	if !config.WiFi.Enable {
		t.Error("Expected WiFi to be enabled")
	}

	if len(config.Switches) != 2 {
		t.Errorf("Expected 2 switch configs, got %d", len(config.Switches))
	}
}

func TestGen1Client_CheckUpdate(t *testing.T) {
	server := mockGen1Server(t)
	defer server.Close()

	ip := server.URL[7:]
	client := NewClient(ip)
	ctx := context.Background()

	update, err := client.CheckUpdate(ctx)
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}

	if !update.HasUpdate {
		t.Error("Expected update to be available")
	}

	if update.NewVersion != "20231015-120000/v1.14.1" {
		t.Errorf("Expected new version '20231015-120000/v1.14.1', got %s", update.NewVersion)
	}
}

func TestGen1Client_GetEnergyData(t *testing.T) {
	server := mockGen1Server(t)
	defer server.Close()

	ip := server.URL[7:]
	client := NewClient(ip)
	ctx := context.Background()

	energy, err := client.GetEnergyData(ctx, 0)
	if err != nil {
		t.Fatalf("GetEnergyData failed: %v", err)
	}

	if energy.Power != 15.32 {
		t.Errorf("Expected power 15.32W, got %f", energy.Power)
	}

	if energy.Voltage != 230.5 {
		t.Errorf("Expected voltage 230.5V, got %f", energy.Voltage)
	}

	if energy.Total != 1.234 { // 1234 Wh = 1.234 kWh
		t.Errorf("Expected total 1.234 kWh, got %f", energy.Total)
	}
}

func TestGen1Client_AuthRequired(t *testing.T) {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return success if auth is correct
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "SHSW-1",
			"mac":  "A4CF12F45678",
			"auth": true,
		}); err != nil {
			t.Logf("Failed to encode JSON response: %v", err)
		}
	}))
	defer server.Close()

	ip := server.URL[7:]

	// Test without auth - should fail
	client := NewClient(ip)
	ctx := context.Background()

	_, err := client.GetInfo(ctx)
	if err == nil || err.Error() != "authentication required" {
		t.Errorf("Expected authentication required, got %v", err)
	}

	// Test with auth - should succeed
	clientWithAuth := NewClient(ip, WithAuth("admin", "secret"))
	info, err := clientWithAuth.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo with auth failed: %v", err)
	}

	if info.Type != "SHSW-1" {
		t.Errorf("Expected type SHSW-1, got %s", info.Type)
	}
}

func TestGen1Client_Retry(t *testing.T) {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on 3rd attempt
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "SHSW-1",
			"mac":  "A4CF12F45678",
		}); err != nil {
			t.Logf("Failed to encode JSON response: %v", err)
		}
	}))
	defer server.Close()

	ip := server.URL[7:]
	client := NewClient(ip, WithRetry(3, 10*time.Millisecond))
	ctx := context.Background()

	info, err := client.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo failed after retries: %v", err)
	}

	if info.Type != "SHSW-1" {
		t.Errorf("Expected type SHSW-1, got %s", info.Type)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

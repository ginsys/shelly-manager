package gen2

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Helper functions are in ../testhelpers_test.go

// isTestNetAddress validates that an address is in a safe TEST-NET range (RFC 5737)
func isTestNetAddress(addr string) bool {
	testNetPrefixes := []string{"192.0.2.", "198.51.100.", "203.0.113."}
	for _, prefix := range testNetPrefixes {
		if strings.HasPrefix(addr, prefix) {
			return true
		}
	}
	return false
}

// assertTestNetAddress fails the test if the address is not in TEST-NET range
func assertTestNetAddress(t *testing.T, addr string) {
	t.Helper()
	if !isTestNetAddress(addr) {
		t.Fatalf("Address %s is not in TEST-NET range - tests should only use RFC 5737 TEST-NET addresses", addr)
	}
}

func TestNewClient(t *testing.T) {
	// Test with default options
	client := NewClient("192.168.1.100")
	assertNotNil(t, client)
	assertEqual(t, "192.168.1.100", client.ip)
	assertEqual(t, 2, client.generation)
	assertEqual(t, 10*time.Second, client.config.timeout)
	assertEqual(t, 3, client.config.retryAttempts)
	assertEqual(t, 1*time.Second, client.config.retryDelay)
	assertEqual(t, "shelly-manager/1.0", client.config.userAgent)

	// Test with custom options
	client = NewClient("192.168.1.101",
		WithAuth("admin", "password"),
		WithTimeout(5*time.Second),
		WithRetry(2, 2*time.Second),
		WithSkipTLSVerify(true),
		WithUserAgent("test-agent"))

	assertEqual(t, "admin", client.config.username)
	assertEqual(t, "password", client.config.password)
	assertEqual(t, 5*time.Second, client.config.timeout)
	assertEqual(t, 2, client.config.retryAttempts)
	assertEqual(t, 2*time.Second, client.config.retryDelay)
	assertTrue(t, client.config.skipTLSVerify)
	assertEqual(t, "test-agent", client.config.userAgent)
}

func TestClientOption_Functions(t *testing.T) {
	cfg := &clientConfig{}

	// Test WithAuth
	WithAuth("testuser", "testpass")(cfg)
	assertEqual(t, "testuser", cfg.username)
	assertEqual(t, "testpass", cfg.password)

	// Test WithTimeout
	WithTimeout(30 * time.Second)(cfg)
	assertEqual(t, 30*time.Second, cfg.timeout)

	// Test WithRetry
	WithRetry(5, 3*time.Second)(cfg)
	assertEqual(t, 5, cfg.retryAttempts)
	assertEqual(t, 3*time.Second, cfg.retryDelay)

	// Test WithSkipTLSVerify
	WithSkipTLSVerify(true)(cfg)
	assertTrue(t, cfg.skipTLSVerify)

	// Test WithUserAgent
	WithUserAgent("custom-agent")(cfg)
	assertEqual(t, "custom-agent", cfg.userAgent)
}

func TestClient_GetGeneration(t *testing.T) {
	client := NewClient("192.168.1.100")
	assertEqual(t, 2, client.GetGeneration())
}

func TestClient_GetIP(t *testing.T) {
	client := NewClient("192.168.1.100")
	assertEqual(t, "192.168.1.100", client.GetIP())
}

// mockGen2Server creates a mock Gen2+ device server
func mockGen2Server() *httptest.Server {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		panic(err)
	} else {
		_ = ln.Close()
	}
	mux := http.NewServeMux()

	// Mock Shelly.GetDeviceInfo RPC call
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		var result interface{}
		var rpcError *shelly.RPCError

		switch req.Method {
		case "Shelly.GetDeviceInfo":
			result = map[string]interface{}{
				"id":          "shellyplusht-08b61fcb7f3c",
				"mac":         "08B61FCB7F3C",
				"model":       "SNSN-0013A",
				"gen":         2,
				"fw_id":       "20231031-165617/1.0.3-geb51a17",
				"ver":         "1.0.3",
				"fw":          "1.0.3",
				"app":         "PlusHT",
				"auth_en":     false,
				"auth_domain": "shellyplusht-08b61fcb7f3c",
			}
		case "Shelly.GetStatus":
			result = map[string]interface{}{
				"sys": map[string]interface{}{
					"temp":      45.2,
					"overtemp":  false,
					"uptime":    3600,
					"ram_total": 50592,
					"ram_free":  39052,
					"fs_size":   233681,
					"fs_free":   162648,
				},
				"wifi": map[string]interface{}{
					"sta_ip":    "192.168.1.100",
					"ssid":      "TestNetwork",
					"rssi":      -45,
					"connected": true,
				},
				"switch:0": map[string]interface{}{
					"output":  true,
					"apower":  25.5,
					"voltage": 230.0,
					"current": 0.11,
					"source":  "input",
					"temperature": map[string]interface{}{
						"tC": 45.2,
					},
				},
			}
		case "Shelly.GetConfig":
			result = map[string]interface{}{
				"sys": map[string]interface{}{
					"device": map[string]interface{}{
						"name": "Test Device",
					},
					"location": map[string]interface{}{
						"tz":  "Europe/Sofia",
						"lat": 42.6977,
						"lon": 23.3219,
					},
					"debug": map[string]interface{}{
						"enable": false,
					},
				},
				"wifi": map[string]interface{}{
					"sta": map[string]interface{}{
						"enable":     true,
						"ssid":       "TestNetwork",
						"pass":       "password123",
						"ipv4mode":   "dhcp",
						"ip":         "",
						"netmask":    "",
						"gw":         "",
						"nameserver": "",
					},
				},
				"cloud": map[string]interface{}{
					"enable": false,
					"server": "shelly-103-eu.shelly.cloud:6022/jrpc",
				},
				"switch:0": map[string]interface{}{
					"name":          "Switch 0",
					"in_mode":       "momentary",
					"initial_state": "restore_last",
					"auto_on":       0.0,
					"auto_off":      0.0,
				},
			}
		case "Shelly.SetAuth":
			result = map[string]interface{}{
				"restart_required": true,
			}
		case "Switch.Set":
			result = map[string]interface{}{
				"was_on": false,
			}
		case "Switch.SetConfig":
			result = map[string]interface{}{
				"restart_required": false,
			}
		case "Light.Set":
			result = map[string]interface{}{
				"was_on": true,
			}
		case "Light.SetConfig":
			result = map[string]interface{}{
				"restart_required": false,
			}
		case "Input.SetConfig":
			result = map[string]interface{}{
				"restart_required": false,
			}
		case "Shelly.SetConfig":
			result = map[string]interface{}{
				"restart_required": false,
			}
		case "Sys.SetConfig":
			result = map[string]interface{}{
				"restart_required": false,
			}
		case "Cover.GoToPosition", "Cover.Open", "Cover.Close", "Cover.Stop":
			result = map[string]interface{}{
				"pos": 50,
			}
		case "Shelly.Reboot":
			result = map[string]interface{}{
				"restart_required": true,
			}
		case "Shelly.FactoryReset":
			result = map[string]interface{}{
				"restart_required": true,
			}
		case "Shelly.CheckForUpdate":
			result = map[string]interface{}{
				"stable": map[string]interface{}{
					"version":  "1.0.4",
					"build_id": "20231101-162345",
				},
				"beta": map[string]interface{}{
					"version":  "1.0.5-beta",
					"build_id": "20231115-123456",
				},
			}
		case "Shelly.Update":
			result = map[string]interface{}{
				"restart_required": true,
			}
		case "Switch.GetStatus":
			result = map[string]interface{}{
				"total":   12345.0,
				"current": 0.11,
				"voltage": 230.0,
				"apower":  25.5,
			}
		default:
			rpcError = &shelly.RPCError{
				Code:    -32601,
				Message: "Method not found",
			}
		}

		resp := RPCResponse{
			ID:     req.ID,
			Result: nil,
			Error:  rpcError,
		}

		if result != nil {
			resultJSON, _ := json.Marshal(result)
			resp.Result = json.RawMessage(resultJSON)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp) // Error ignored in test mock
	})

	return httptest.NewServer(mux)
}

func TestClient_GetInfo(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	// Extract IP from server URL
	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()
	info, err := client.GetInfo(ctx)
	assertNoError(t, err)
	assertNotNil(t, info)

	assertEqual(t, "shellyplusht-08b61fcb7f3c", info.ID)
	assertEqual(t, "08B61FCB7F3C", info.MAC)
	assertEqual(t, "SNSN-0013A", info.Model)
	assertEqual(t, 2, info.Generation)
	assertEqual(t, "20231031-165617/1.0.3-geb51a17", info.FirmwareID)
	assertEqual(t, "1.0.3", info.Version)
	assertEqual(t, "PlusHT", info.App)
	assertEqual(t, false, info.AuthEn)
	assertEqual(t, "shellyplusht-08b61fcb7f3c", info.AuthDomain)
	assertEqual(t, serverIP, info.IP)
}

func TestClient_GetStatus(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()
	status, err := client.GetStatus(ctx)
	assertNoError(t, err)
	assertNotNil(t, status)

	// Test system status parsing
	assertEqual(t, 45.2, status.Temperature)
	assertEqual(t, false, status.Overtemperature)
	assertEqual(t, 3600, status.Uptime)
	assertEqual(t, 50592, status.RAMTotal)
	assertEqual(t, 39052, status.RAMFree)
	assertEqual(t, 233681, status.FSSize)
	assertEqual(t, 162648, status.FSFree)

	// Test WiFi status parsing
	assertNotNil(t, status.WiFiStatus)
	assertTrue(t, status.WiFiStatus.Connected)
	assertEqual(t, "192.168.1.100", status.WiFiStatus.IP)
	assertEqual(t, "TestNetwork", status.WiFiStatus.SSID)
	assertEqual(t, -45, status.WiFiStatus.RSSI)

	// Test switch status parsing
	assertEqual(t, 1, len(status.Switches))
	assertEqual(t, 0, status.Switches[0].ID)
	assertTrue(t, status.Switches[0].Output)
	assertEqual(t, 25.5, status.Switches[0].APower)
	assertEqual(t, 230.0, status.Switches[0].Voltage)
	assertEqual(t, 0.11, status.Switches[0].Current)
	assertEqual(t, 45.2, status.Switches[0].Temperature)
	assertEqual(t, "input", status.Switches[0].Source)
}

func TestClient_GetConfig(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()
	config, err := client.GetConfig(ctx)
	assertNoError(t, err)
	assertNotNil(t, config)

	// Test system config parsing
	assertEqual(t, "Test Device", config.Name)
	assertEqual(t, "Europe/Sofia", config.Timezone)
	assertEqual(t, 42.6977, config.Lat)
	assertEqual(t, 23.3219, config.Lng)
	assertEqual(t, false, config.Debug)

	// Test WiFi config parsing
	assertNotNil(t, config.WiFi)
	assertTrue(t, config.WiFi.Enable)
	assertEqual(t, "TestNetwork", config.WiFi.SSID)
	assertEqual(t, "password123", config.WiFi.Password)
	assertEqual(t, "dhcp", config.WiFi.IPV4Mode)

	// Test cloud config parsing
	assertNotNil(t, config.Cloud)
	assertEqual(t, false, config.Cloud.Enable)
	assertEqual(t, "shelly-103-eu.shelly.cloud:6022/jrpc", config.Cloud.Server)

	// Test switch config parsing
	assertEqual(t, 1, len(config.Switches))
	assertEqual(t, 0, config.Switches[0].ID)
	assertEqual(t, "Switch 0", config.Switches[0].Name)
	assertEqual(t, "momentary", config.Switches[0].InMode)
	assertEqual(t, "restore_last", config.Switches[0].InitialState)
	assertEqual(t, 0, config.Switches[0].AutoOn)
	assertEqual(t, 0, config.Switches[0].AutoOff)
}

func TestClient_SetConfig(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()
	config := map[string]interface{}{
		"name": "Updated Device",
	}

	err := client.SetConfig(ctx, config)
	assertNoError(t, err)
}

func TestClient_SetAuth(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	err := client.SetAuth(ctx, "admin", "newpassword")
	assertNoError(t, err)
}

func TestClient_ResetAuth(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	err := client.ResetAuth(ctx)
	assertNoError(t, err)
}

func TestClient_SetSwitch(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	err := client.SetSwitch(ctx, 0, true)
	assertNoError(t, err)

	err = client.SetSwitch(ctx, 0, false)
	assertNoError(t, err)
}

func TestClient_TestConnection(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	err := client.TestConnection(ctx)
	assertNoError(t, err)
}

func TestClient_LightOperations(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	// Test SetBrightness
	err := client.SetBrightness(ctx, 0, 75)
	assertNoError(t, err)

	// Test SetColorRGB
	err = client.SetColorRGB(ctx, 0, 255, 128, 64)
	assertNoError(t, err)

	// Test SetColorTemp
	err = client.SetColorTemp(ctx, 0, 4000)
	assertNoError(t, err)

	// Test SetWhiteChannel
	err = client.SetWhiteChannel(ctx, 0, 50, 3000)
	assertNoError(t, err)

	// Test SetColorMode (no-op for Gen2+)
	err = client.SetColorMode(ctx, "color")
	assertNoError(t, err)
}

func TestClient_RollerOperations(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	// Test SetRollerPosition
	err := client.SetRollerPosition(ctx, 0, 50)
	assertNoError(t, err)

	// Test position bounds
	err = client.SetRollerPosition(ctx, 0, -10) // Should clamp to 0
	assertNoError(t, err)

	err = client.SetRollerPosition(ctx, 0, 150) // Should clamp to 100
	assertNoError(t, err)

	// Test OpenRoller
	err = client.OpenRoller(ctx, 0)
	assertNoError(t, err)

	// Test CloseRoller
	err = client.CloseRoller(ctx, 0)
	assertNoError(t, err)

	// Test StopRoller
	err = client.StopRoller(ctx, 0)
	assertNoError(t, err)
}

func TestClient_AdvancedSettings(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	// Test SetRelaySettings
	relaySettings := map[string]interface{}{
		"initial_state": "on",
		"auto_off":      10,
	}
	err := client.SetRelaySettings(ctx, 0, relaySettings)
	assertNoError(t, err)

	// Test SetLightSettings
	lightSettings := map[string]interface{}{
		"default_brightness": 75,
		"night_mode":         true,
	}
	err = client.SetLightSettings(ctx, 0, lightSettings)
	assertNoError(t, err)

	// Test SetInputSettings
	inputSettings := map[string]interface{}{
		"type":   "momentary",
		"invert": false,
	}
	err = client.SetInputSettings(ctx, 0, inputSettings)
	assertNoError(t, err)

	// Test SetLEDSettings
	ledSettings := map[string]interface{}{
		"power_led":  true,
		"status_led": false,
	}
	err = client.SetLEDSettings(ctx, ledSettings)
	assertNoError(t, err)
}

func TestClient_UpdateOperations(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	// Test CheckUpdate
	updateInfo, err := client.CheckUpdate(ctx)
	assertNoError(t, err)
	assertNotNil(t, updateInfo)
	assertEqual(t, "1.0.3", updateInfo.OldVersion)
	assertTrue(t, updateInfo.HasUpdate)
	assertEqual(t, "1.0.4", updateInfo.NewVersion)

	// Test PerformUpdate
	err = client.PerformUpdate(ctx)
	assertNoError(t, err)
}

func TestClient_SystemOperations(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	// Test Reboot
	err := client.Reboot(ctx)
	assertNoError(t, err)

	// Test FactoryReset
	err = client.FactoryReset(ctx)
	assertNoError(t, err)
}

func TestClient_GetMetrics(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	metrics, err := client.GetMetrics(ctx)
	assertNoError(t, err)
	assertNotNil(t, metrics)

	assertEqual(t, 45.2, metrics.Temperature)
	assertEqual(t, 3600, metrics.Uptime)
	assertEqual(t, -45, metrics.WiFiRSSI)
}

func TestClient_GetEnergyData(t *testing.T) {
	server := mockGen2Server()
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()

	energyData, err := client.GetEnergyData(ctx, 0)
	assertNoError(t, err)
	assertNotNil(t, energyData)

	assertEqual(t, 25.5, energyData.Power)
	assertEqual(t, 12.345, energyData.Total) // Converted from Wh to kWh
	assertEqual(t, 230.0, energyData.Voltage)
	assertEqual(t, 0.11, energyData.Current)
}

// Test error cases
func TestClient_ErrorHandling(t *testing.T) {
	// Test with non-existent server using TEST-NET address
	testAddr := "192.0.2.200"
	assertTestNetAddress(t, testAddr)
	t.Logf("Using safe TEST-NET address: %s (RFC 5737 - no real network traffic)", testAddr)

	client := NewClient(testAddr, WithTimeout(50*time.Millisecond))
	ctx := context.Background()

	_, err := client.GetInfo(ctx)
	assertError(t, err)

	_, err = client.GetStatus(ctx)
	assertError(t, err)

	_, err = client.GetConfig(ctx)
	assertError(t, err)
}

func TestClient_ContextCancellation(t *testing.T) {
	testAddr := "192.0.2.200"
	assertTestNetAddress(t, testAddr)
	t.Logf("Using safe TEST-NET address: %s (RFC 5737 - no real network traffic)", testAddr)

	client := NewClient(testAddr, WithTimeout(50*time.Millisecond))

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetInfo(ctx)
	assertError(t, err)
}

func TestClient_AuthRequired(t *testing.T) {
	// Create mock server that returns 401 for unauthorized requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	serverIP := server.URL[len("http://"):]
	client := NewClient(serverIP)

	ctx := context.Background()
	_, err := client.GetInfo(ctx)
	assertError(t, err)
	assertEqual(t, shelly.ErrAuthRequired, err)
}

func TestClient_WithLogger(t *testing.T) {
	// Test that client accepts custom logger
	logger := logging.GetDefault()
	client := NewClient("192.168.1.100")
	client.logger = logger
	assertNotNil(t, client.logger)
}

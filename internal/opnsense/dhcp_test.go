package opnsense

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func setupDHCPTestService(t *testing.T) (*DHCPManager, func()) {
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	// Mock server will be set up per test
	config := ClientConfig{
		Host:      "localhost",
		UseHTTPS:  false,
		APIKey:    "test-api-key",
		APISecret: "test-api-secret",
		Timeout:   10 * time.Second,
	}

	client, err := NewClient(config, logger)
	require.NoError(t, err)

	dhcpManager := NewDHCPManager(client)

	cleanup := func() {
		// Cleanup resources if needed
	}

	return dhcpManager, cleanup
}

func TestNewDHCPManager(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	config := ClientConfig{
		Host:      "localhost",
		APIKey:    "test-key",
		APISecret: "test-secret",
	}

	client, err := NewClient(config, logger)
	require.NoError(t, err)

	manager := NewDHCPManager(client)
	assert.NotNil(t, manager)
	assert.Equal(t, client, manager.client)
}

func TestDHCPManager_GetReservations(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Successful Reservations Retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/api/dhcp/leases/searchReservations")

			response := DHCPReservationList{
				Reservations: map[string]DHCPReservation{
					"uuid-1": {
						MAC:         "aa:bb:cc:dd:ee:ff",
						IP:          "192.168.1.100",
						Hostname:    "shelly-device-1",
						Description: "Test device 1",
					},
					"uuid-2": {
						MAC:         "11:22:33:44:55:66",
						IP:          "192.168.1.101",
						Hostname:    "shelly-device-2",
						Description: "Test device 2",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservations, err := dhcpManager.GetReservations(context.Background(), "")
		assert.NoError(t, err)
		assert.Len(t, reservations, 2)

		// Check that UUIDs are set
		for _, reservation := range reservations {
			assert.NotEmpty(t, reservation.UUID)
		}
	})

	t.Run("Reservations with Interface Filter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check that interface parameter is passed
			assert.Equal(t, "lan", r.URL.Query().Get("interface"))

			response := DHCPReservationList{
				Reservations: map[string]DHCPReservation{
					"uuid-1": {
						MAC:       "aa:bb:cc:dd:ee:ff",
						IP:        "192.168.1.100",
						Hostname:  "device-1",
						Interface: "lan",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservations, err := dhcpManager.GetReservations(context.Background(), "lan")
		assert.NoError(t, err)
		assert.Len(t, reservations, 1)
		assert.Equal(t, "lan", reservations[0].Interface)
	})

	t.Run("API Error Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message": "Internal server error"}`))
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservations, err := dhcpManager.GetReservations(context.Background(), "")
		assert.Error(t, err)
		assert.Nil(t, reservations)
		assert.Contains(t, err.Error(), "failed to fetch DHCP reservations")
	})
}

func TestDHCPManager_GetReservation(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Successful Single Reservation Retrieval", func(t *testing.T) {
		testUUID := "test-uuid-123"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, fmt.Sprintf("/api/dhcp/leases/getReservation/%s", testUUID))

			reservation := DHCPReservation{
				MAC:         "aa:bb:cc:dd:ee:ff",
				IP:          "192.168.1.100",
				Hostname:    "test-device",
				Description: "Test reservation",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(reservation)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservation, err := dhcpManager.GetReservation(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.NotNil(t, reservation)
		assert.Equal(t, testUUID, reservation.UUID)
		assert.Equal(t, "aa:bb:cc:dd:ee:ff", reservation.MAC)
		assert.Equal(t, "192.168.1.100", reservation.IP)
	})

	t.Run("Reservation Not Found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Reservation not found"}`))
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservation, err := dhcpManager.GetReservation(context.Background(), "non-existent-uuid")
		assert.Error(t, err)
		assert.Nil(t, reservation)
	})
}

func TestDHCPManager_CreateReservation(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Successful Reservation Creation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/api/dhcp/leases/addReservation")

			// Verify request body
			var requestReservation DHCPReservation
			err := json.NewDecoder(r.Body).Decode(&requestReservation)
			require.NoError(t, err)
			assert.Equal(t, "aa:bb:cc:dd:ee:ff", requestReservation.MAC)
			assert.Equal(t, "192.168.1.100", requestReservation.IP)
			assert.Equal(t, "test-device", requestReservation.Hostname)

			response := DHCPReservationResponse{
				Status:  "ok",
				UUID:    "new-uuid-123",
				Message: "Reservation created successfully",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservation := DHCPReservation{
			MAC:         "aa:bb:cc:dd:ee:ff",
			IP:          "192.168.1.100",
			Hostname:    "test-device",
			Description: "Test device",
		}

		response, err := dhcpManager.CreateReservation(context.Background(), reservation)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, "new-uuid-123", response.UUID)
	})

	t.Run("Invalid Reservation Data", func(t *testing.T) {
		reservation := DHCPReservation{
			MAC:      "", // Missing MAC
			IP:       "192.168.1.100",
			Hostname: "test-device",
		}

		response, err := dhcpManager.CreateReservation(context.Background(), reservation)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid reservation data")
	})

	t.Run("API Creation Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := DHCPReservationResponse{
				Status:  "failed",
				Message: "MAC address already exists",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservation := DHCPReservation{
			MAC:      "aa:bb:cc:dd:ee:ff",
			IP:       "192.168.1.100",
			Hostname: "test-device",
		}

		response, err := dhcpManager.CreateReservation(context.Background(), reservation)
		assert.Error(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "failed", response.Status)
		assert.Contains(t, err.Error(), "failed to create reservation")
	})
}

func TestDHCPManager_UpdateReservation(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Successful Reservation Update", func(t *testing.T) {
		testUUID := "existing-uuid-123"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, fmt.Sprintf("/api/dhcp/leases/setReservation/%s", testUUID))

			response := DHCPReservationResponse{
				Status:  "ok",
				Message: "Reservation updated successfully",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservation := DHCPReservation{
			MAC:      "aa:bb:cc:dd:ee:ff",
			IP:       "192.168.1.101", // Changed IP
			Hostname: "updated-device",
		}

		response, err := dhcpManager.UpdateReservation(context.Background(), testUUID, reservation)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
	})

	t.Run("Update with Invalid Data", func(t *testing.T) {
		reservation := DHCPReservation{
			MAC:      "invalid-mac",
			IP:       "192.168.1.100",
			Hostname: "test-device",
		}

		response, err := dhcpManager.UpdateReservation(context.Background(), "test-uuid", reservation)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid reservation data")
	})
}

func TestDHCPManager_DeleteReservation(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Successful Reservation Deletion", func(t *testing.T) {
		testUUID := "delete-uuid-123"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, fmt.Sprintf("/api/dhcp/leases/delReservation/%s", testUUID))

			response := DHCPReservationResponse{
				Status:  "ok",
				Message: "Reservation deleted successfully",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		response, err := dhcpManager.DeleteReservation(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
	})

	t.Run("Delete Non-Existent Reservation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := DHCPReservationResponse{
				Status:  "failed",
				Message: "Reservation not found",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		response, err := dhcpManager.DeleteReservation(context.Background(), "non-existent-uuid")
		assert.Error(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "failed", response.Status)
	})
}

func TestDHCPManager_FindReservationByMAC(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Find Existing Reservation by MAC", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := DHCPReservationList{
				Reservations: map[string]DHCPReservation{
					"uuid-1": {
						MAC:      "aa:bb:cc:dd:ee:ff",
						IP:       "192.168.1.100",
						Hostname: "device-1",
					},
					"uuid-2": {
						MAC:      "11:22:33:44:55:66",
						IP:       "192.168.1.101",
						Hostname: "device-2",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		// Test different MAC formats (should normalize)
		testCases := []string{
			"aa:bb:cc:dd:ee:ff",
			"AA:BB:CC:DD:EE:FF",
			"aa-bb-cc-dd-ee-ff",
			"aabbccddeeff",
		}

		for _, mac := range testCases {
			reservation, err := dhcpManager.FindReservationByMAC(context.Background(), mac, "")
			assert.NoError(t, err)
			assert.NotNil(t, reservation)
			assert.Equal(t, "aa:bb:cc:dd:ee:ff", reservation.MAC)
			assert.Equal(t, "uuid-1", reservation.UUID)
		}
	})

	t.Run("MAC Not Found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := DHCPReservationList{
				Reservations: map[string]DHCPReservation{},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		reservation, err := dhcpManager.FindReservationByMAC(context.Background(), "99:99:99:99:99:99", "")
		assert.Error(t, err)
		assert.Nil(t, reservation)
		assert.Contains(t, err.Error(), "no reservation found for MAC address")
	})
}

func TestDHCPManager_FindReservationByIP(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := DHCPReservationList{
			Reservations: map[string]DHCPReservation{
				"uuid-1": {
					MAC:      "aa:bb:cc:dd:ee:ff",
					IP:       "192.168.1.100",
					Hostname: "device-1",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	dhcpManager.client.baseURL = server.URL

	t.Run("Find Existing Reservation by IP", func(t *testing.T) {
		reservation, err := dhcpManager.FindReservationByIP(context.Background(), "192.168.1.100", "")
		assert.NoError(t, err)
		assert.NotNil(t, reservation)
		assert.Equal(t, "192.168.1.100", reservation.IP)
		assert.Equal(t, "uuid-1", reservation.UUID)
	})

	t.Run("IP Not Found", func(t *testing.T) {
		reservation, err := dhcpManager.FindReservationByIP(context.Background(), "192.168.1.200", "")
		assert.Error(t, err)
		assert.Nil(t, reservation)
		assert.Contains(t, err.Error(), "no reservation found for IP address")
	})
}

func TestDHCPManager_ValidateReservation(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Valid Reservation", func(t *testing.T) {
		reservation := DHCPReservation{
			MAC:      "aa:bb:cc:dd:ee:ff",
			IP:       "192.168.1.100",
			Hostname: "test-device",
		}

		err := dhcpManager.validateReservation(reservation)
		assert.NoError(t, err)
	})

	t.Run("Invalid MAC Address", func(t *testing.T) {
		testCases := []struct {
			name string
			mac  string
		}{
			{"Empty MAC", ""},
			{"Invalid Format", "invalid-mac"},
			{"Too Short", "aa:bb:cc"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				reservation := DHCPReservation{
					MAC:      tc.mac,
					IP:       "192.168.1.100",
					Hostname: "test-device",
				}

				err := dhcpManager.validateReservation(reservation)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "MAC address")
			})
		}
	})

	t.Run("Invalid IP Address", func(t *testing.T) {
		testCases := []struct {
			name string
			ip   string
		}{
			{"Empty IP", ""},
			{"Invalid Format", "invalid-ip"},
			{"Out of Range", "999.999.999.999"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				reservation := DHCPReservation{
					MAC:      "aa:bb:cc:dd:ee:ff",
					IP:       tc.ip,
					Hostname: "test-device",
				}

				err := dhcpManager.validateReservation(reservation)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "IP address")
			})
		}
	})

	t.Run("Invalid Hostname", func(t *testing.T) {
		testCases := []struct {
			name     string
			hostname string
		}{
			{"Empty Hostname", ""},
			{"Too Long Hostname", "this-is-a-very-long-hostname-that-exceeds-the-maximum-allowed-length-for-dns-hostnames"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				reservation := DHCPReservation{
					MAC:      "aa:bb:cc:dd:ee:ff",
					IP:       "192.168.1.100",
					Hostname: tc.hostname,
				}

				err := dhcpManager.validateReservation(reservation)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "hostname")
			})
		}
	})
}

func TestDHCPManager_NormalizeMAC(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	testCases := []struct {
		input    string
		expected string
	}{
		{"AA:BB:CC:DD:EE:FF", "aabbccddeeff"},
		{"aa:bb:cc:dd:ee:ff", "aabbccddeeff"},
		{"AA-BB-CC-DD-EE-FF", "aabbccddeeff"},
		{"aa-bb-cc-dd-ee-ff", "aabbccddeeff"},
		{"aabbccddeeff", "aabbccddeeff"},
		{"AABBCCDDEEFF", "aabbccddeeff"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Normalize_%s", tc.input), func(t *testing.T) {
			result := dhcpManager.normalizeMAC(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDHCPManager_GenerateHostname(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	device := DeviceMapping{
		ShellyMAC:  "aa:bb:cc:dd:ee:ff",
		ShellyName: "Shelly1PM",
	}

	t.Run("Default Template", func(t *testing.T) {
		hostname := dhcpManager.GenerateHostname(device, "")
		assert.NotEmpty(t, hostname)
		assert.Contains(t, hostname, "shelly1pm")
		assert.Contains(t, hostname, "eeff") // last 4 chars of MAC
	})

	t.Run("Custom Template", func(t *testing.T) {
		template := "device-{{.Name}}-{{.MAC | last4}}"
		hostname := dhcpManager.GenerateHostname(device, template)
		assert.Equal(t, "device-shelly1pm-eeff", hostname)
	})

	t.Run("Hostname Sanitization", func(t *testing.T) {
		device := DeviceMapping{
			ShellyMAC:  "aa:bb:cc:dd:ee:ff",
			ShellyName: "Shelly Device With Spaces!@#",
		}

		template := "{{.Name}}"
		hostname := dhcpManager.GenerateHostname(device, template)

		// Should be sanitized (lowercase, alphanumeric + hyphens only)
		assert.Regexp(t, "^[a-z0-9-]+$", hostname)
		assert.NotContains(t, hostname, " ")
		assert.NotContains(t, hostname, "!")
	})
}

func TestDHCPManager_SanitizeHostname(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	testCases := []struct {
		input    string
		expected string
	}{
		{"Valid-Hostname", "valid-hostname"},
		{"Invalid Characters!@#", "invalid-characters---"},
		{"Multiple---Hyphens", "multiple---hyphens"},
		{"   Leading-And-Trailing   ", "leading-and-trailing"},
		{"", "shelly-device"}, // Empty should get default
		{"a-very-long-hostname-that-exceeds-sixty-three-characters-limit-for-sure", "a-very-long-hostname-that-exceeds-sixty-three-characters-limit"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Sanitize_%s", tc.input), func(t *testing.T) {
			result := dhcpManager.sanitizeHostname(tc.input)
			assert.Equal(t, tc.expected, result)
			assert.True(t, len(result) <= 63, "Hostname should be <= 63 characters")
		})
	}
}

func TestDHCPManager_ApplyConfiguration(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Successful Configuration Apply", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/api/dhcp/service/reconfigure")

			response := ConfigurationStatus{
				Status: "ok",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		err := dhcpManager.ApplyConfiguration(context.Background())
		assert.NoError(t, err)
	})

	t.Run("Configuration Apply Failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := ConfigurationStatus{
				Status:  "failed",
				Message: "Configuration validation failed",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		err := dhcpManager.ApplyConfiguration(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to apply configuration")
	})
}

func TestDHCPManager_SyncReservations(t *testing.T) {
	dhcpManager, cleanup := setupDHCPTestService(t)
	defer cleanup()

	t.Run("Successful Sync - Create New Reservations", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/dhcp/leases/searchReservations":
				// Return empty existing reservations
				response := DHCPReservationList{
					Reservations: map[string]DHCPReservation{},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			case "/api/dhcp/leases/addReservation":
				response := DHCPReservationResponse{
					Status: "ok",
					UUID:   "new-uuid",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			case "/api/dhcp/service/reconfigure":
				response := ConfigurationStatus{
					Status: "ok",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		devices := []DeviceMapping{
			{
				ShellyMAC:        "aa:bb:cc:dd:ee:ff",
				ShellyIP:         "192.168.1.100",
				ShellyName:       "Shelly1PM",
				OPNSenseHostname: "shelly-device-1",
				Interface:        "lan",
			},
		}

		options := SyncOptions{
			DryRun:             false,
			ConflictResolution: ConflictResolutionManagerWins,
			ApplyChanges:       true,
		}

		result, err := dhcpManager.SyncReservations(context.Background(), devices, options)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, 1, result.ReservationsAdded)
		assert.Equal(t, 0, result.ReservationsUpdated)
	})

	t.Run("Sync with Conflicts - Manager Wins", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/dhcp/leases/searchReservations":
				// Return existing reservation with different IP
				response := DHCPReservationList{
					Reservations: map[string]DHCPReservation{
						"existing-uuid": {
							MAC:      "aa:bb:cc:dd:ee:ff",
							IP:       "192.168.1.200", // Different IP
							Hostname: "old-hostname",
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			case "/api/dhcp/leases/setReservation/existing-uuid":
				response := DHCPReservationResponse{
					Status: "ok",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		devices := []DeviceMapping{
			{
				ShellyMAC:        "aa:bb:cc:dd:ee:ff",
				ShellyIP:         "192.168.1.100",
				OPNSenseHostname: "new-hostname",
			},
		}

		options := SyncOptions{
			DryRun:             false,
			ConflictResolution: ConflictResolutionManagerWins,
		}

		result, err := dhcpManager.SyncReservations(context.Background(), devices, options)
		assert.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 0, result.ReservationsAdded)
		assert.Equal(t, 1, result.ReservationsUpdated)
	})

	t.Run("Dry Run Mode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/dhcp/leases/searchReservations":
				response := DHCPReservationList{
					Reservations: map[string]DHCPReservation{},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			default:
				// In dry run mode, no other API calls should be made
				t.Errorf("Unexpected API call in dry run mode: %s", r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()
		dhcpManager.client.baseURL = server.URL

		devices := []DeviceMapping{
			{
				ShellyMAC:        "aa:bb:cc:dd:ee:ff",
				ShellyIP:         "192.168.1.100",
				OPNSenseHostname: "test-device",
			},
		}

		options := SyncOptions{
			DryRun: true,
		}

		result, err := dhcpManager.SyncReservations(context.Background(), devices, options)
		assert.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 1, result.ReservationsAdded) // Should count what would be added
	})
}

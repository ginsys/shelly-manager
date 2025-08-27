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

func setupFirewallTestService(t *testing.T) (*FirewallManager, func()) {
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	config := ClientConfig{
		Host:      "localhost",
		UseHTTPS:  false,
		APIKey:    "test-api-key",
		APISecret: "test-api-secret",
		Timeout:   10 * time.Second,
	}

	client, err := NewClient(config, logger)
	require.NoError(t, err)

	firewallManager := NewFirewallManager(client)

	cleanup := func() {
		// Cleanup resources if needed
	}

	return firewallManager, cleanup
}

func TestNewFirewallManager(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	require.NoError(t, err)

	config := ClientConfig{
		Host:      "localhost",
		APIKey:    "test-key",
		APISecret: "test-secret",
	}

	client, err := NewClient(config, logger)
	require.NoError(t, err)

	manager := NewFirewallManager(client)
	assert.NotNil(t, manager)
	assert.Equal(t, client, manager.client)
}

func TestFirewallManager_GetAliases(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Successful Aliases Retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/api/firewall/alias/searchItem")

			response := FirewallAliasList{
				Aliases: map[string]FirewallAlias{
					"uuid-1": {
						Name:        "SHELLY_DEVICES",
						Type:        "host",
						Content:     []string{"192.168.1.100", "192.168.1.101"},
						Description: "Shelly devices",
						Enabled:     true,
					},
					"uuid-2": {
						Name:        "WEB_SERVERS",
						Type:        "network",
						Content:     []string{"10.0.1.0/24"},
						Description: "Web server network",
						Enabled:     true,
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		aliases, err := firewallManager.GetAliases(context.Background())
		assert.NoError(t, err)
		assert.Len(t, aliases, 2)

		// Check that UUIDs are set
		for _, alias := range aliases {
			assert.NotEmpty(t, alias.UUID)
		}
	})

	t.Run("API Error Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message": "Internal server error"}`))
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		aliases, err := firewallManager.GetAliases(context.Background())
		assert.Error(t, err)
		assert.Nil(t, aliases)
		assert.Contains(t, err.Error(), "failed to fetch firewall aliases")
	})
}

func TestFirewallManager_GetAlias(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Successful Single Alias Retrieval", func(t *testing.T) {
		testUUID := "test-uuid-123"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, fmt.Sprintf("/api/firewall/alias/getItem/%s", testUUID))

			alias := FirewallAlias{
				Name:        "SHELLY_DEVICES",
				Type:        "host",
				Content:     []string{"192.168.1.100", "192.168.1.101"},
				Description: "Shelly devices",
				Enabled:     true,
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(alias)
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		alias, err := firewallManager.GetAlias(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.NotNil(t, alias)
		assert.Equal(t, testUUID, alias.UUID)
		assert.Equal(t, "SHELLY_DEVICES", alias.Name)
		assert.Equal(t, "host", alias.Type)
	})

	t.Run("Alias Not Found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Alias not found"}`))
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		alias, err := firewallManager.GetAlias(context.Background(), "non-existent-uuid")
		assert.Error(t, err)
		assert.Nil(t, alias)
	})
}

func TestFirewallManager_FindAliasByName(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := FirewallAliasList{
			Aliases: map[string]FirewallAlias{
				"uuid-1": {
					Name:    "SHELLY_DEVICES",
					Type:    "host",
					Content: []string{"192.168.1.100"},
				},
				"uuid-2": {
					Name:    "WEB_SERVERS",
					Type:    "network",
					Content: []string{"10.0.1.0/24"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	firewallManager.client.baseURL = server.URL

	t.Run("Find Existing Alias by Name", func(t *testing.T) {
		alias, err := firewallManager.FindAliasByName(context.Background(), "SHELLY_DEVICES")
		assert.NoError(t, err)
		assert.NotNil(t, alias)
		assert.Equal(t, "SHELLY_DEVICES", alias.Name)
		assert.Equal(t, "uuid-1", alias.UUID)
	})

	t.Run("Find Alias by Name Case Insensitive", func(t *testing.T) {
		alias, err := firewallManager.FindAliasByName(context.Background(), "shelly_devices")
		assert.NoError(t, err)
		assert.NotNil(t, alias)
		assert.Equal(t, "SHELLY_DEVICES", alias.Name)
	})

	t.Run("Alias Name Not Found", func(t *testing.T) {
		alias, err := firewallManager.FindAliasByName(context.Background(), "NONEXISTENT_ALIAS")
		assert.Error(t, err)
		assert.Nil(t, alias)
		assert.Contains(t, err.Error(), "no firewall alias found with name")
	})
}

func TestFirewallManager_CreateAlias(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Successful Alias Creation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/api/firewall/alias/addItem")

			// Verify request body
			var requestAlias FirewallAlias
			err := json.NewDecoder(r.Body).Decode(&requestAlias)
			require.NoError(t, err)
			assert.Equal(t, "TEST_ALIAS", requestAlias.Name)
			assert.Equal(t, "host", requestAlias.Type)

			response := FirewallAliasResponse{
				Status:  "ok",
				UUID:    "new-uuid-123",
				Message: "Alias created successfully",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		alias := FirewallAlias{
			Name:        "TEST_ALIAS",
			Type:        "host",
			Content:     []string{"192.168.1.100"},
			Description: "Test alias",
			Enabled:     true,
		}

		response, err := firewallManager.CreateAlias(context.Background(), alias)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, "new-uuid-123", response.UUID)
	})

	t.Run("Invalid Alias Data", func(t *testing.T) {
		alias := FirewallAlias{
			Name: "", // Missing name
			Type: "host",
		}

		response, err := firewallManager.CreateAlias(context.Background(), alias)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid alias data")
	})
}

func TestFirewallManager_UpdateAlias(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Successful Alias Update", func(t *testing.T) {
		testUUID := "existing-uuid-123"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, fmt.Sprintf("/api/firewall/alias/setItem/%s", testUUID))

			response := FirewallAliasResponse{
				Status:  "ok",
				Message: "Alias updated successfully",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		alias := FirewallAlias{
			Name:    "UPDATED_ALIAS",
			Type:    "host",
			Content: []string{"192.168.1.101", "192.168.1.102"}, // Updated content
		}

		response, err := firewallManager.UpdateAlias(context.Background(), testUUID, alias)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
	})

	t.Run("Update with Invalid Data", func(t *testing.T) {
		alias := FirewallAlias{
			Name: "Invalid@Name", // Invalid characters
			Type: "host",
		}

		response, err := firewallManager.UpdateAlias(context.Background(), "test-uuid", alias)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid alias data")
	})
}

func TestFirewallManager_DeleteAlias(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Successful Alias Deletion", func(t *testing.T) {
		testUUID := "delete-uuid-123"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, fmt.Sprintf("/api/firewall/alias/delItem/%s", testUUID))

			response := FirewallAliasResponse{
				Status:  "ok",
				Message: "Alias deleted successfully",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		response, err := firewallManager.DeleteAlias(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
	})
}

func TestFirewallManager_ValidateAlias(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Valid Host Alias", func(t *testing.T) {
		alias := FirewallAlias{
			Name:    "VALID_ALIAS",
			Type:    "host",
			Content: []string{"192.168.1.100", "10.0.0.1"},
		}

		err := firewallManager.validateAlias(alias)
		assert.NoError(t, err)
	})

	t.Run("Valid Network Alias", func(t *testing.T) {
		alias := FirewallAlias{
			Name:    "NETWORK_ALIAS",
			Type:    "network",
			Content: []string{"192.168.1.0/24", "10.0.0.0/16"},
		}

		err := firewallManager.validateAlias(alias)
		assert.NoError(t, err)
	})

	t.Run("Valid Port Alias", func(t *testing.T) {
		alias := FirewallAlias{
			Name:    "PORT_ALIAS",
			Type:    "port",
			Content: []string{"80", "443", "8080-8090"},
		}

		err := firewallManager.validateAlias(alias)
		assert.NoError(t, err)
	})

	t.Run("Invalid Alias Name", func(t *testing.T) {
		testCases := []struct {
			name      string
			aliasName string
		}{
			{"Empty Name", ""},
			{"Too Long Name", "this_is_a_very_long_alias_name_that_exceeds_the_maximum_allowed_length"},
			{"Invalid Characters", "alias@name"},
			{"Spaces", "alias name"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				alias := FirewallAlias{
					Name:    tc.aliasName,
					Type:    "host",
					Content: []string{"192.168.1.100"},
				}

				err := firewallManager.validateAlias(alias)
				assert.Error(t, err)
			})
		}
	})

	t.Run("Invalid Alias Type", func(t *testing.T) {
		alias := FirewallAlias{
			Name:    "VALID_NAME",
			Type:    "invalid_type",
			Content: []string{"192.168.1.100"},
		}

		err := firewallManager.validateAlias(alias)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid alias type")
	})

	t.Run("Empty Content", func(t *testing.T) {
		alias := FirewallAlias{
			Name:    "VALID_NAME",
			Type:    "host",
			Content: []string{},
		}

		err := firewallManager.validateAlias(alias)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alias content is required")
	})

	t.Run("Invalid Host Content", func(t *testing.T) {
		alias := FirewallAlias{
			Name:    "INVALID_HOST",
			Type:    "host",
			Content: []string{"invalid-ip", "999.999.999.999"},
		}

		err := firewallManager.validateAlias(alias)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid host content")
	})

	t.Run("Invalid Network Content", func(t *testing.T) {
		alias := FirewallAlias{
			Name:    "INVALID_NETWORK",
			Type:    "network",
			Content: []string{"192.168.1.0/99", "invalid-network"},
		}

		err := firewallManager.validateAlias(alias)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid network content")
	})
}

func TestFirewallManager_ValidateIPOrNetwork(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	validCases := []string{
		"192.168.1.1",
		"10.0.0.1",
		"127.0.0.1",
		"192.168.1.0/24",
		"10.0.0.0/16",
		"::1",
		"2001:db8::/32",
	}

	for _, validCase := range validCases {
		t.Run(fmt.Sprintf("Valid_%s", validCase), func(t *testing.T) {
			err := firewallManager.validateIPOrNetwork(validCase)
			assert.NoError(t, err)
		})
	}

	invalidCases := []string{
		"invalid-ip",
		"999.999.999.999",
		"192.168.1.0/99",
		"not-an-ip",
	}

	for _, invalidCase := range invalidCases {
		t.Run(fmt.Sprintf("Invalid_%s", invalidCase), func(t *testing.T) {
			err := firewallManager.validateIPOrNetwork(invalidCase)
			assert.Error(t, err)
		})
	}
}

func TestFirewallManager_ValidatePortOrRange(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	validCases := []string{
		"80",
		"443",
		"8080-8090",
		"1000-2000",
		"ssh",
		"http",
	}

	for _, validCase := range validCases {
		t.Run(fmt.Sprintf("Valid_%s", validCase), func(t *testing.T) {
			err := firewallManager.validatePortOrRange(validCase)
			assert.NoError(t, err)
		})
	}

	invalidCases := []string{
		"", // Empty
		"80-",
		"-80",
	}

	for _, invalidCase := range invalidCases {
		t.Run(fmt.Sprintf("Invalid_%s", invalidCase), func(t *testing.T) {
			err := firewallManager.validatePortOrRange(invalidCase)
			assert.Error(t, err)
		})
	}
}

func TestFirewallManager_UpdateShellyDeviceAlias(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Update Existing Alias", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/firewall/alias/searchItem":
				// Return existing alias
				response := FirewallAliasList{
					Aliases: map[string]FirewallAlias{
						"existing-uuid": {
							Name:    "SHELLY_DEVICES",
							Type:    "host",
							Content: []string{"192.168.1.50"},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			case "/api/firewall/alias/setItem/existing-uuid":
				// Update alias
				response := FirewallAliasResponse{
					Status: "ok",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		devices := []DeviceMapping{
			{
				ShellyIP:   "192.168.1.100",
				ShellyName: "Shelly1PM-1",
			},
			{
				ShellyIP:   "192.168.1.101",
				ShellyName: "Shelly1PM-2",
			},
		}

		response, err := firewallManager.UpdateShellyDeviceAlias(context.Background(), "SHELLY_DEVICES", devices, false)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
	})

	t.Run("Create New Alias", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/firewall/alias/searchItem":
				// Return empty aliases (alias doesn't exist)
				response := FirewallAliasList{
					Aliases: map[string]FirewallAlias{},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			case "/api/firewall/alias/addItem":
				// Create new alias
				response := FirewallAliasResponse{
					Status: "ok",
					UUID:   "new-uuid",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		devices := []DeviceMapping{
			{
				ShellyIP:   "192.168.1.100",
				ShellyName: "Shelly1PM-1",
			},
		}

		response, err := firewallManager.UpdateShellyDeviceAlias(context.Background(), "NEW_SHELLY_ALIAS", devices, true)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, "new-uuid", response.UUID)
	})

	t.Run("No Valid IPs", func(t *testing.T) {
		devices := []DeviceMapping{
			{
				ShellyIP:   "invalid-ip",
				ShellyName: "Invalid Device",
			},
			{
				ShellyIP:   "", // Empty IP
				ShellyName: "Empty IP Device",
			},
		}

		response, err := firewallManager.UpdateShellyDeviceAlias(context.Background(), "EMPTY_ALIAS", devices, true)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "no valid IP addresses found")
	})
}

func TestFirewallManager_ApplyConfiguration(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Successful Configuration Apply", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/api/firewall/alias/reconfigure")

			response := ConfigurationStatus{
				Status: "ok",
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		firewallManager.client.baseURL = server.URL

		err := firewallManager.ApplyConfiguration(context.Background())
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
		firewallManager.client.baseURL = server.URL

		err := firewallManager.ApplyConfiguration(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to apply configuration")
	})
}

func TestFirewallManager_SyncShellyDeviceAliases(t *testing.T) {
	firewallManager, cleanup := setupFirewallTestService(t)
	defer cleanup()

	t.Run("Successful Sync", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/firewall/alias/searchItem":
				response := FirewallAliasList{
					Aliases: map[string]FirewallAlias{},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			case "/api/firewall/alias/addItem":
				response := FirewallAliasResponse{
					Status: "ok",
					UUID:   "new-uuid",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)

			case "/api/firewall/alias/reconfigure":
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
		firewallManager.client.baseURL = server.URL

		aliasConfigs := map[string][]DeviceMapping{
			"SHELLY_LIVING_ROOM": {
				{ShellyIP: "192.168.1.100", ShellyName: "Living Room Light"},
			},
			"SHELLY_KITCHEN": {
				{ShellyIP: "192.168.1.101", ShellyName: "Kitchen Switch"},
			},
		}

		options := SyncOptions{
			DryRun:       false,
			ApplyChanges: true,
		}

		result, err := firewallManager.SyncShellyDeviceAliases(context.Background(), aliasConfigs, options)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, 2, result.AliasesUpdated)
	})

	t.Run("Dry Run Mode", func(t *testing.T) {
		// In dry run mode, no API calls should be made except for logging
		aliasConfigs := map[string][]DeviceMapping{
			"TEST_ALIAS": {
				{ShellyIP: "192.168.1.100", ShellyName: "Test Device"},
			},
		}

		options := SyncOptions{
			DryRun: true,
		}

		result, err := firewallManager.SyncShellyDeviceAliases(context.Background(), aliasConfigs, options)
		assert.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 1, result.AliasesUpdated) // Should count what would be updated
	})
}

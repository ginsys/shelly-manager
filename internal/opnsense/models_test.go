package opnsense

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemStatus(t *testing.T) {
	t.Run("Valid SystemStatus", func(t *testing.T) {
		status := &SystemStatus{
			Version:     "22.7.8",
			ConfigDate:  time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC),
			Uptime:      "15 days, 3 hours, 42 minutes",
			LoadAverage: []float64{0.15, 0.12, 0.08},
			CPUUsage:    25.5,
		}

		status.MemoryUsage.Used = 2147483648  // 2GB
		status.MemoryUsage.Total = 8589934592 // 8GB

		assert.Equal(t, "22.7.8", status.Version)
		assert.Equal(t, 25.5, status.CPUUsage)
		assert.Len(t, status.LoadAverage, 3)
		assert.Equal(t, uint64(2147483648), status.MemoryUsage.Used)
		assert.Equal(t, uint64(8589934592), status.MemoryUsage.Total)
	})

	t.Run("SystemStatus JSON Serialization", func(t *testing.T) {
		status := &SystemStatus{
			Version:     "23.1.0",
			ConfigDate:  time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			Uptime:      "7 days",
			LoadAverage: []float64{0.5},
			CPUUsage:    50.0,
		}
		status.MemoryUsage.Used = 1073741824
		status.MemoryUsage.Total = 4294967296

		data, err := json.Marshal(status)
		require.NoError(t, err)

		var unmarshaled SystemStatus
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, status.Version, unmarshaled.Version)
		assert.Equal(t, status.CPUUsage, unmarshaled.CPUUsage)
		assert.Equal(t, status.MemoryUsage.Used, unmarshaled.MemoryUsage.Used)
	})
}

func TestDHCPReservation(t *testing.T) {
	t.Run("Valid DHCP Reservation", func(t *testing.T) {
		reservation := &DHCPReservation{
			UUID:        "12345-67890-abcdef",
			MAC:         "68:C6:3A:12:34:56",
			IP:          "192.168.1.100",
			Hostname:    "shelly-switch-01",
			Description: "Living room light switch",
			Disabled:    false,
			Interface:   "lan",
		}

		assert.Equal(t, "68:C6:3A:12:34:56", reservation.MAC)
		assert.Equal(t, "192.168.1.100", reservation.IP)
		assert.Equal(t, "shelly-switch-01", reservation.Hostname)
		assert.False(t, reservation.Disabled)
	})

	t.Run("DHCP Reservation without optional fields", func(t *testing.T) {
		reservation := &DHCPReservation{
			MAC:      "68:C6:3A:AB:CD:EF",
			IP:       "192.168.1.101",
			Hostname: "device-test",
		}

		assert.Empty(t, reservation.UUID)
		assert.Empty(t, reservation.Description)
		assert.False(t, reservation.Disabled) // Default value
		assert.Empty(t, reservation.Interface)
	})

	t.Run("DHCP Reservation JSON Serialization", func(t *testing.T) {
		reservation := &DHCPReservation{
			UUID:        "test-uuid-123",
			MAC:         "AA:BB:CC:DD:EE:FF",
			IP:          "10.0.0.50",
			Hostname:    "test-device",
			Description: "Test device description",
			Disabled:    true,
			Interface:   "opt1",
		}

		data, err := json.Marshal(reservation)
		require.NoError(t, err)

		var unmarshaled DHCPReservation
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, reservation.UUID, unmarshaled.UUID)
		assert.Equal(t, reservation.MAC, unmarshaled.MAC)
		assert.Equal(t, reservation.IP, unmarshaled.IP)
		assert.Equal(t, reservation.Hostname, unmarshaled.Hostname)
		assert.Equal(t, reservation.Description, unmarshaled.Description)
		assert.Equal(t, reservation.Disabled, unmarshaled.Disabled)
		assert.Equal(t, reservation.Interface, unmarshaled.Interface)
	})
}

func TestDHCPReservationResponse(t *testing.T) {
	t.Run("Successful Response", func(t *testing.T) {
		response := &DHCPReservationResponse{
			Status:  "ok",
			Message: "Reservation created successfully",
			UUID:    "new-uuid-456",
		}

		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, "new-uuid-456", response.UUID)
		assert.Nil(t, response.Validations)
	})

	t.Run("Error Response with Validations", func(t *testing.T) {
		response := &DHCPReservationResponse{
			Status:  "failed",
			Message: "Validation errors occurred",
			Validations: map[string]string{
				"mac": "Invalid MAC address format",
				"ip":  "IP address already in use",
			},
		}

		assert.Equal(t, "failed", response.Status)
		assert.Contains(t, response.Validations, "mac")
		assert.Contains(t, response.Validations, "ip")
		assert.Equal(t, "Invalid MAC address format", response.Validations["mac"])
	})
}

func TestFirewallAlias(t *testing.T) {
	t.Run("Host Alias", func(t *testing.T) {
		alias := &FirewallAlias{
			UUID:        "alias-uuid-123",
			Name:        "ShellyDevices",
			Type:        "host",
			Content:     []string{"192.168.1.100", "192.168.1.101", "192.168.1.102"},
			Description: "All Shelly devices in the network",
			Enabled:     true,
			UpdateFreq:  "daily",
		}

		assert.Equal(t, "ShellyDevices", alias.Name)
		assert.Equal(t, "host", alias.Type)
		assert.Len(t, alias.Content, 3)
		assert.Contains(t, alias.Content, "192.168.1.100")
		assert.True(t, alias.Enabled)
	})

	t.Run("Network Alias", func(t *testing.T) {
		alias := &FirewallAlias{
			Name:        "IoTNetworks",
			Type:        "network",
			Content:     []string{"192.168.100.0/24", "10.0.10.0/24"},
			Description: "IoT device networks",
			Enabled:     true,
		}

		assert.Equal(t, "network", alias.Type)
		assert.Contains(t, alias.Content, "192.168.100.0/24")
		assert.Empty(t, alias.UpdateFreq)
	})

	t.Run("Port Alias", func(t *testing.T) {
		alias := &FirewallAlias{
			Name:    "ShellyPorts",
			Type:    "port",
			Content: []string{"80", "443", "8080"},
			Enabled: false,
		}

		assert.Equal(t, "port", alias.Type)
		assert.Len(t, alias.Content, 3)
		assert.False(t, alias.Enabled)
	})

	t.Run("URL Alias", func(t *testing.T) {
		alias := &FirewallAlias{
			Name:       "ThreatFeeds",
			Type:       "url",
			Content:    []string{"https://example.com/blocklist.txt"},
			UpdateFreq: "hourly",
			Enabled:    true,
		}

		assert.Equal(t, "url", alias.Type)
		assert.Equal(t, "hourly", alias.UpdateFreq)
		assert.Contains(t, alias.Content, "https://example.com/blocklist.txt")
	})
}

func TestFirewallAliasResponse(t *testing.T) {
	t.Run("Success Response", func(t *testing.T) {
		response := &FirewallAliasResponse{
			Status: "ok",
			UUID:   "firewall-alias-789",
		}

		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, "firewall-alias-789", response.UUID)
		assert.Empty(t, response.Message)
	})

	t.Run("Validation Error Response", func(t *testing.T) {
		response := &FirewallAliasResponse{
			Status:  "failed",
			Message: "Alias validation failed",
			Validations: map[string]string{
				"name":    "Alias name already exists",
				"content": "Invalid IP address in content",
			},
		}

		assert.Equal(t, "failed", response.Status)
		assert.Equal(t, "Alias validation failed", response.Message)
		assert.Len(t, response.Validations, 2)
	})
}

func TestConfigurationStatus(t *testing.T) {
	t.Run("Configuration Changed", func(t *testing.T) {
		status := &ConfigurationStatus{
			Status:  "ok",
			Message: "Configuration updated successfully",
			Changed: true,
		}

		assert.Equal(t, "ok", status.Status)
		assert.True(t, status.Changed)
		assert.Equal(t, "Configuration updated successfully", status.Message)
	})

	t.Run("No Configuration Changes", func(t *testing.T) {
		status := &ConfigurationStatus{
			Status:  "ok",
			Message: "No changes required",
			Changed: false,
		}

		assert.False(t, status.Changed)
		assert.Equal(t, "No changes required", status.Message)
	})
}

func TestSyncResult(t *testing.T) {
	t.Run("Successful Sync", func(t *testing.T) {
		result := &SyncResult{
			Success:             true,
			ReservationsAdded:   3,
			ReservationsUpdated: 2,
			ReservationsDeleted: 1,
			AliasesUpdated:      1,
			Duration:            time.Second * 30,
		}

		assert.True(t, result.Success)
		assert.Equal(t, 3, result.ReservationsAdded)
		assert.Equal(t, 2, result.ReservationsUpdated)
		assert.Equal(t, 1, result.ReservationsDeleted)
		assert.Equal(t, 1, result.AliasesUpdated)
		assert.Equal(t, time.Second*30, result.Duration)
		assert.Empty(t, result.Errors)
		assert.Empty(t, result.Warnings)
	})

	t.Run("Sync with Warnings and Errors", func(t *testing.T) {
		result := &SyncResult{
			Success:             false,
			ReservationsAdded:   1,
			ReservationsUpdated: 0,
			ReservationsDeleted: 0,
			AliasesUpdated:      0,
			Errors:              []string{"Failed to connect to device", "Invalid MAC address"},
			Warnings:            []string{"Device not responding", "Hostname conflict"},
			Duration:            time.Second * 45,
		}

		assert.False(t, result.Success)
		assert.Len(t, result.Errors, 2)
		assert.Len(t, result.Warnings, 2)
		assert.Contains(t, result.Errors, "Failed to connect to device")
		assert.Contains(t, result.Warnings, "Device not responding")
	})
}

func TestConflictResolution(t *testing.T) {
	t.Run("Conflict Resolution Constants", func(t *testing.T) {
		assert.Equal(t, ConflictResolution("opnsense_wins"), ConflictResolutionOPNSenseWins)
		assert.Equal(t, ConflictResolution("manager_wins"), ConflictResolutionManagerWins)
		assert.Equal(t, ConflictResolution("manual"), ConflictResolutionManual)
		assert.Equal(t, ConflictResolution("skip"), ConflictResolutionSkip)
	})
}

func TestSyncOptions(t *testing.T) {
	t.Run("Default Sync Options", func(t *testing.T) {
		options := &SyncOptions{
			ConflictResolution: ConflictResolutionManagerWins,
			DryRun:             false,
			UpdateFirewall:     true,
			BackupBefore:       true,
			ApplyChanges:       true,
		}

		assert.Equal(t, ConflictResolutionManagerWins, options.ConflictResolution)
		assert.False(t, options.DryRun)
		assert.True(t, options.UpdateFirewall)
		assert.True(t, options.BackupBefore)
		assert.True(t, options.ApplyChanges)
	})

	t.Run("Conservative Sync Options", func(t *testing.T) {
		options := &SyncOptions{
			ConflictResolution: ConflictResolutionManual,
			DryRun:             true,
			UpdateFirewall:     false,
			BackupBefore:       true,
			ApplyChanges:       false,
		}

		assert.Equal(t, ConflictResolutionManual, options.ConflictResolution)
		assert.True(t, options.DryRun)
		assert.False(t, options.UpdateFirewall)
		assert.False(t, options.ApplyChanges)
	})
}

func TestDeviceMapping(t *testing.T) {
	t.Run("Complete Device Mapping", func(t *testing.T) {
		syncTime := time.Now()
		mapping := &DeviceMapping{
			ShellyMAC:        "68:C6:3A:12:34:56",
			ShellyIP:         "192.168.1.100",
			ShellyName:       "shelly-switch-living-room",
			OPNSenseHostname: "switch-lr",
			Interface:        "lan",
			LastSync:         &syncTime,
			SyncStatus:       "success",
		}

		assert.Equal(t, "68:C6:3A:12:34:56", mapping.ShellyMAC)
		assert.Equal(t, "192.168.1.100", mapping.ShellyIP)
		assert.Equal(t, "switch-lr", mapping.OPNSenseHostname)
		assert.Equal(t, "lan", mapping.Interface)
		assert.NotNil(t, mapping.LastSync)
		assert.Equal(t, "success", mapping.SyncStatus)
	})

	t.Run("Minimal Device Mapping", func(t *testing.T) {
		mapping := &DeviceMapping{
			ShellyMAC:  "AA:BB:CC:DD:EE:FF",
			ShellyIP:   "10.0.0.100",
			ShellyName: "test-device",
			SyncStatus: "pending",
		}

		assert.Equal(t, "AA:BB:CC:DD:EE:FF", mapping.ShellyMAC)
		assert.Equal(t, "pending", mapping.SyncStatus)
		assert.Nil(t, mapping.LastSync)
		assert.Empty(t, mapping.OPNSenseHostname)
		assert.Empty(t, mapping.Interface)
	})
}

func TestAPIError(t *testing.T) {
	t.Run("Simple API Error", func(t *testing.T) {
		err := &APIError{
			HTTPStatus: 404,
			Message:    "Resource not found",
		}

		assert.Equal(t, 404, err.HTTPStatus)
		assert.Equal(t, "Resource not found", err.Message)
		assert.Equal(t, "OPNSense API error (HTTP 404): Resource not found", err.Error())
	})

	t.Run("API Error with Details", func(t *testing.T) {
		err := &APIError{
			HTTPStatus: 400,
			Message:    "Validation failed",
			Details: map[string]string{
				"mac": "Invalid MAC address format",
				"ip":  "IP address out of range",
			},
		}

		errorStr := err.Error()
		assert.Contains(t, errorStr, "HTTP 400")
		assert.Contains(t, errorStr, "Validation failed")
		assert.Contains(t, errorStr, "mac")
		assert.Contains(t, errorStr, "Invalid MAC address format")
	})

	t.Run("API Error Empty Details", func(t *testing.T) {
		err := &APIError{
			HTTPStatus: 500,
			Message:    "Internal server error",
			Details:    map[string]string{},
		}

		// Empty details map should not show details in error message
		errorStr := err.Error()
		assert.Equal(t, "OPNSense API error (HTTP 500): Internal server error", errorStr)
	})
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	t.Run("DHCPReservationList JSON", func(t *testing.T) {
		reservationList := &DHCPReservationList{
			Reservations: map[string]DHCPReservation{
				"uuid-1": {
					UUID:     "uuid-1",
					MAC:      "68:C6:3A:11:11:11",
					IP:       "192.168.1.10",
					Hostname: "device-1",
				},
				"uuid-2": {
					UUID:     "uuid-2",
					MAC:      "68:C6:3A:22:22:22",
					IP:       "192.168.1.20",
					Hostname: "device-2",
				},
			},
		}

		data, err := json.Marshal(reservationList)
		require.NoError(t, err)

		var unmarshaled DHCPReservationList
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Len(t, unmarshaled.Reservations, 2)
		assert.Contains(t, unmarshaled.Reservations, "uuid-1")
		assert.Equal(t, "device-1", unmarshaled.Reservations["uuid-1"].Hostname)
	})

	t.Run("FirewallAliasList JSON", func(t *testing.T) {
		aliasList := &FirewallAliasList{
			Aliases: map[string]FirewallAlias{
				"alias-1": {
					UUID:    "alias-1",
					Name:    "ShellyDevices",
					Type:    "host",
					Content: []string{"192.168.1.100", "192.168.1.101"},
					Enabled: true,
				},
			},
		}

		data, err := json.Marshal(aliasList)
		require.NoError(t, err)

		var unmarshaled FirewallAliasList
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Len(t, unmarshaled.Aliases, 1)
		alias := unmarshaled.Aliases["alias-1"]
		assert.Equal(t, "ShellyDevices", alias.Name)
		assert.Equal(t, "host", alias.Type)
		assert.Len(t, alias.Content, 2)
	})

	t.Run("SyncOptions JSON", func(t *testing.T) {
		options := &SyncOptions{
			ConflictResolution: ConflictResolutionOPNSenseWins,
			DryRun:             true,
			UpdateFirewall:     false,
			BackupBefore:       true,
			ApplyChanges:       false,
		}

		data, err := json.Marshal(options)
		require.NoError(t, err)

		var unmarshaled SyncOptions
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, ConflictResolutionOPNSenseWins, unmarshaled.ConflictResolution)
		assert.True(t, unmarshaled.DryRun)
		assert.False(t, unmarshaled.UpdateFirewall)
		assert.True(t, unmarshaled.BackupBefore)
		assert.False(t, unmarshaled.ApplyChanges)
	})
}

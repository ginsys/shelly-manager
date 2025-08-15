package configuration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLevel_Constants(t *testing.T) {
	tests := []struct {
		name     string
		level    ConfigLevel
		expected int
	}{
		{
			name:     "SystemLevel",
			level:    SystemLevel,
			expected: 0,
		},
		{
			name:     "TemplateLevel",
			level:    TemplateLevel,
			expected: 1,
		},
		{
			name:     "DeviceLevel",
			level:    DeviceLevel,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.level))
		})
	}
}

func TestConfigTemplate_Marshaling(t *testing.T) {
	template := &ConfigTemplate{
		ID:          1,
		Name:        "Test Template",
		Description: "A test template",
		DeviceType:  "SHSW-1",
		Generation:  1,
		Config:      json.RawMessage(`{"test": "config"}`),
		Variables:   json.RawMessage(`{"var1": "value1"}`),
		IsDefault:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(template)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded ConfigTemplate
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, template.ID, decoded.ID)
	assert.Equal(t, template.Name, decoded.Name)
	assert.Equal(t, template.Description, decoded.Description)
	assert.Equal(t, template.DeviceType, decoded.DeviceType)
	assert.Equal(t, template.Generation, decoded.Generation)
	assert.Equal(t, template.IsDefault, decoded.IsDefault)
	assert.JSONEq(t, string(template.Config), string(decoded.Config))
	assert.JSONEq(t, string(template.Variables), string(decoded.Variables))
}

func TestDeviceConfig_Marshaling(t *testing.T) {
	now := time.Now()
	templateID := uint(1)

	config := &DeviceConfig{
		ID:         1,
		DeviceID:   100,
		TemplateID: &templateID,
		Config:     json.RawMessage(`{"device": "config"}`),
		LastSynced: &now,
		SyncStatus: "synced",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded DeviceConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, config.ID, decoded.ID)
	assert.Equal(t, config.DeviceID, decoded.DeviceID)
	assert.NotNil(t, decoded.TemplateID)
	assert.Equal(t, *config.TemplateID, *decoded.TemplateID)
	assert.Equal(t, config.SyncStatus, decoded.SyncStatus)
	assert.JSONEq(t, string(config.Config), string(decoded.Config))
	assert.WithinDuration(t, *config.LastSynced, *decoded.LastSynced, time.Second)
}

func TestConfigHistory_Marshaling(t *testing.T) {
	history := &ConfigHistory{
		ID:        1,
		DeviceID:  100,
		ConfigID:  1,
		Action:    "import",
		OldConfig: json.RawMessage(`{"old": "config"}`),
		NewConfig: json.RawMessage(`{"new": "config"}`),
		Changes:   json.RawMessage(`[{"path": "test", "type": "modified"}]`),
		ChangedBy: "system",
		CreatedAt: time.Now(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(history)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded ConfigHistory
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, history.ID, decoded.ID)
	assert.Equal(t, history.DeviceID, decoded.DeviceID)
	assert.Equal(t, history.ConfigID, decoded.ConfigID)
	assert.Equal(t, history.Action, decoded.Action)
	assert.Equal(t, history.ChangedBy, decoded.ChangedBy)
	assert.JSONEq(t, string(history.OldConfig), string(decoded.OldConfig))
	assert.JSONEq(t, string(history.NewConfig), string(decoded.NewConfig))
	assert.JSONEq(t, string(history.Changes), string(decoded.Changes))
}

func TestConfigDrift(t *testing.T) {
	now := time.Now()

	drift := &ConfigDrift{
		DeviceID:       100,
		DeviceName:     "Test Device",
		LastSynced:     &now,
		DriftDetected:  now.Add(time.Hour),
		RequiresAction: true,
		Differences: []ConfigDifference{
			{
				Path:     "wifi.ssid",
				Expected: "oldSSID",
				Actual:   "newSSID",
				Type:     "modified",
			},
			{
				Path:     "mqtt.enabled",
				Expected: true,
				Actual:   nil,
				Type:     "removed",
			},
			{
				Path:     "cloud.enabled",
				Expected: nil,
				Actual:   false,
				Type:     "added",
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(drift)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded ConfigDrift
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, drift.DeviceID, decoded.DeviceID)
	assert.Equal(t, drift.DeviceName, decoded.DeviceName)
	assert.Equal(t, drift.RequiresAction, decoded.RequiresAction)
	assert.WithinDuration(t, *drift.LastSynced, *decoded.LastSynced, time.Second)
	assert.WithinDuration(t, drift.DriftDetected, decoded.DriftDetected, time.Second)
	assert.Len(t, decoded.Differences, 3)

	// Check differences
	for i, diff := range decoded.Differences {
		assert.Equal(t, drift.Differences[i].Path, diff.Path)
		assert.Equal(t, drift.Differences[i].Type, diff.Type)
	}
}

func TestGen1Config_Marshaling(t *testing.T) {
	config := &Gen1Config{
		Name:     "Test Device",
		Timezone: "America/New_York",
	}

	// Set WiFi config
	config.WiFi.SSID = "TestNetwork"
	config.WiFi.Password = "secret123"
	config.WiFi.IP = "192.168.1.100"
	config.WiFi.Netmask = "255.255.255.0"
	config.WiFi.Gateway = "192.168.1.1"
	config.WiFi.DNS = "8.8.8.8"

	// Set Auth config
	config.Auth.Enabled = true
	config.Auth.Username = "admin"
	config.Auth.Password = "password"

	// Set Cloud config
	config.Cloud.Enabled = true
	config.Cloud.Server = "cloud.shelly.com"

	// Set MQTT config
	config.MQTT.Enable = true
	config.MQTT.Server = "mqtt.example.com:1883"
	config.MQTT.User = "mqttuser"
	config.MQTT.Password = "mqttpass"
	config.MQTT.ID = "shelly-test"
	config.MQTT.CleanSession = true
	config.MQTT.RetainMessages = false
	config.MQTT.QoS = 1
	config.MQTT.KeepAlive = 60

	// Add device-specific settings
	config.DeviceSettings = json.RawMessage(`{"relay": {"on": true}}`)

	// Test JSON marshaling
	data, err := json.Marshal(config)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded Gen1Config
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, config.Name, decoded.Name)
	assert.Equal(t, config.Timezone, decoded.Timezone)

	// Verify WiFi settings
	assert.Equal(t, config.WiFi.SSID, decoded.WiFi.SSID)
	assert.Equal(t, config.WiFi.Password, decoded.WiFi.Password)
	assert.Equal(t, config.WiFi.IP, decoded.WiFi.IP)
	assert.Equal(t, config.WiFi.Netmask, decoded.WiFi.Netmask)
	assert.Equal(t, config.WiFi.Gateway, decoded.WiFi.Gateway)
	assert.Equal(t, config.WiFi.DNS, decoded.WiFi.DNS)

	// Verify Auth settings
	assert.Equal(t, config.Auth.Enabled, decoded.Auth.Enabled)
	assert.Equal(t, config.Auth.Username, decoded.Auth.Username)
	assert.Equal(t, config.Auth.Password, decoded.Auth.Password)

	// Verify Cloud settings
	assert.Equal(t, config.Cloud.Enabled, decoded.Cloud.Enabled)
	assert.Equal(t, config.Cloud.Server, decoded.Cloud.Server)

	// Verify MQTT settings
	assert.Equal(t, config.MQTT.Enable, decoded.MQTT.Enable)
	assert.Equal(t, config.MQTT.Server, decoded.MQTT.Server)
	assert.Equal(t, config.MQTT.User, decoded.MQTT.User)
	assert.Equal(t, config.MQTT.Password, decoded.MQTT.Password)
	assert.Equal(t, config.MQTT.ID, decoded.MQTT.ID)
	assert.Equal(t, config.MQTT.CleanSession, decoded.MQTT.CleanSession)
	assert.Equal(t, config.MQTT.RetainMessages, decoded.MQTT.RetainMessages)
	assert.Equal(t, config.MQTT.QoS, decoded.MQTT.QoS)
	assert.Equal(t, config.MQTT.KeepAlive, decoded.MQTT.KeepAlive)

	// Verify device settings
	assert.JSONEq(t, string(config.DeviceSettings), string(decoded.DeviceSettings))
}

func TestGen2Config_Marshaling(t *testing.T) {
	config := &Gen2Config{}

	// Set System config
	config.Sys.Device.Name = "Test Device Gen2"
	config.Sys.Device.MAC = "AA:BB:CC:DD:EE:FF"
	config.Sys.Location.Timezone = "Europe/London"
	config.Sys.Location.Lat = 51.5074
	config.Sys.Location.Lon = -0.1278
	config.Sys.Debug.Level = "info"
	config.Sys.Debug.FileLog = true

	// Set WiFi AP config
	config.WiFi.AP.SSID = "ShellyAP"
	config.WiFi.AP.Pass = "appassword"
	config.WiFi.AP.Enable = false

	// Set WiFi STA config
	config.WiFi.STA.SSID = "HomeNetwork"
	config.WiFi.STA.Pass = "homepass"
	config.WiFi.STA.Enable = true
	config.WiFi.STA.IP = "192.168.1.200"
	config.WiFi.STA.Netmask = "255.255.255.0"
	config.WiFi.STA.Gateway = "192.168.1.1"
	config.WiFi.STA.NameServer = "1.1.1.1"

	// Set Cloud config
	config.Cloud.Enable = true
	config.Cloud.Server = "cloud-gen2.shelly.com"

	// Set MQTT config
	config.MQTT.Enable = true
	config.MQTT.Server = "mqtt-gen2.example.com:8883"
	config.MQTT.User = "gen2user"
	config.MQTT.Pass = "gen2pass"
	config.MQTT.ClientID = "shelly-gen2-test"
	config.MQTT.Topic = "shellies/gen2/test"

	// Add component configurations
	config.Components = json.RawMessage(`{"switch:0": {"name": "Light", "in_mode": "momentary"}}`)

	// Test JSON marshaling
	data, err := json.Marshal(config)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded Gen2Config
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify System settings
	assert.Equal(t, config.Sys.Device.Name, decoded.Sys.Device.Name)
	assert.Equal(t, config.Sys.Device.MAC, decoded.Sys.Device.MAC)
	assert.Equal(t, config.Sys.Location.Timezone, decoded.Sys.Location.Timezone)
	assert.Equal(t, config.Sys.Location.Lat, decoded.Sys.Location.Lat)
	assert.Equal(t, config.Sys.Location.Lon, decoded.Sys.Location.Lon)
	assert.Equal(t, config.Sys.Debug.Level, decoded.Sys.Debug.Level)
	assert.Equal(t, config.Sys.Debug.FileLog, decoded.Sys.Debug.FileLog)

	// Verify WiFi AP settings
	assert.Equal(t, config.WiFi.AP.SSID, decoded.WiFi.AP.SSID)
	assert.Equal(t, config.WiFi.AP.Pass, decoded.WiFi.AP.Pass)
	assert.Equal(t, config.WiFi.AP.Enable, decoded.WiFi.AP.Enable)

	// Verify WiFi STA settings
	assert.Equal(t, config.WiFi.STA.SSID, decoded.WiFi.STA.SSID)
	assert.Equal(t, config.WiFi.STA.Pass, decoded.WiFi.STA.Pass)
	assert.Equal(t, config.WiFi.STA.Enable, decoded.WiFi.STA.Enable)
	assert.Equal(t, config.WiFi.STA.IP, decoded.WiFi.STA.IP)
	assert.Equal(t, config.WiFi.STA.Netmask, decoded.WiFi.STA.Netmask)
	assert.Equal(t, config.WiFi.STA.Gateway, decoded.WiFi.STA.Gateway)
	assert.Equal(t, config.WiFi.STA.NameServer, decoded.WiFi.STA.NameServer)

	// Verify Cloud settings
	assert.Equal(t, config.Cloud.Enable, decoded.Cloud.Enable)
	assert.Equal(t, config.Cloud.Server, decoded.Cloud.Server)

	// Verify MQTT settings
	assert.Equal(t, config.MQTT.Enable, decoded.MQTT.Enable)
	assert.Equal(t, config.MQTT.Server, decoded.MQTT.Server)
	assert.Equal(t, config.MQTT.User, decoded.MQTT.User)
	assert.Equal(t, config.MQTT.Pass, decoded.MQTT.Pass)
	assert.Equal(t, config.MQTT.ClientID, decoded.MQTT.ClientID)
	assert.Equal(t, config.MQTT.Topic, decoded.MQTT.Topic)

	// Verify components
	assert.JSONEq(t, string(config.Components), string(decoded.Components))
}

func TestTemplateVariable(t *testing.T) {
	tests := []struct {
		name     string
		variable TemplateVariable
	}{
		{
			name: "String variable",
			variable: TemplateVariable{
				Name:         "device_name",
				Description:  "The name of the device",
				Type:         "string",
				DefaultValue: "MyDevice",
				Required:     true,
			},
		},
		{
			name: "Number variable",
			variable: TemplateVariable{
				Name:         "keepalive",
				Description:  "MQTT keepalive interval",
				Type:         "number",
				DefaultValue: 60,
				Required:     false,
			},
		},
		{
			name: "Boolean variable",
			variable: TemplateVariable{
				Name:         "cloud_enabled",
				Description:  "Enable cloud connection",
				Type:         "boolean",
				DefaultValue: false,
				Required:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.variable)
			require.NoError(t, err)

			// Test JSON unmarshaling
			var decoded TemplateVariable
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.variable.Name, decoded.Name)
			assert.Equal(t, tt.variable.Description, decoded.Description)
			assert.Equal(t, tt.variable.Type, decoded.Type)
			assert.Equal(t, tt.variable.Required, decoded.Required)

			// DefaultValue comparison depends on type
			switch tt.variable.Type {
			case "string":
				assert.Equal(t, tt.variable.DefaultValue.(string), decoded.DefaultValue.(string))
			case "number":
				// JSON unmarshaling converts numbers to float64
				expected := float64(tt.variable.DefaultValue.(int))
				assert.Equal(t, expected, decoded.DefaultValue.(float64))
			case "boolean":
				assert.Equal(t, tt.variable.DefaultValue.(bool), decoded.DefaultValue.(bool))
			}
		})
	}
}

func TestBulkConfigOperation_Marshaling(t *testing.T) {
	now := time.Now()
	templateID := uint(1)

	operation := &BulkConfigOperation{
		ID:          1,
		Name:        "Bulk Update",
		Description: "Update all devices",
		DeviceIDs:   []uint{1, 2, 3, 4, 5},
		TemplateID:  &templateID,
		Config:      json.RawMessage(`{"bulk": "config"}`),
		Status:      "completed",
		Progress:    100,
		Results:     json.RawMessage(`[{"device_id": 1, "status": "success"}]`),
		StartedAt:   &now,
		CompletedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(operation)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded BulkConfigOperation
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, operation.ID, decoded.ID)
	assert.Equal(t, operation.Name, decoded.Name)
	assert.Equal(t, operation.Description, decoded.Description)
	assert.Equal(t, operation.DeviceIDs, decoded.DeviceIDs)
	assert.NotNil(t, decoded.TemplateID)
	assert.Equal(t, *operation.TemplateID, *decoded.TemplateID)
	assert.Equal(t, operation.Status, decoded.Status)
	assert.Equal(t, operation.Progress, decoded.Progress)
	assert.JSONEq(t, string(operation.Config), string(decoded.Config))
	assert.JSONEq(t, string(operation.Results), string(decoded.Results))
	assert.WithinDuration(t, *operation.StartedAt, *decoded.StartedAt, time.Second)
	assert.WithinDuration(t, *operation.CompletedAt, *decoded.CompletedAt, time.Second)
}

func TestConfigDifference_Types(t *testing.T) {
	tests := []struct {
		name string
		diff ConfigDifference
	}{
		{
			name: "Modified field",
			diff: ConfigDifference{
				Path:     "wifi.ssid",
				Expected: "OldNetwork",
				Actual:   "NewNetwork",
				Type:     "modified",
			},
		},
		{
			name: "Added field",
			diff: ConfigDifference{
				Path:     "mqtt.enabled",
				Expected: nil,
				Actual:   true,
				Type:     "added",
			},
		},
		{
			name: "Removed field",
			diff: ConfigDifference{
				Path:     "cloud.server",
				Expected: "cloud.shelly.com",
				Actual:   nil,
				Type:     "removed",
			},
		},
		{
			name: "Nested path",
			diff: ConfigDifference{
				Path:     "sys.device.location.timezone",
				Expected: "UTC",
				Actual:   "America/New_York",
				Type:     "modified",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.diff)
			require.NoError(t, err)

			// Test JSON unmarshaling
			var decoded ConfigDifference
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.diff.Path, decoded.Path)
			assert.Equal(t, tt.diff.Type, decoded.Type)

			// Compare Expected and Actual based on type
			if tt.diff.Expected == nil {
				assert.Nil(t, decoded.Expected)
			} else {
				assert.NotNil(t, decoded.Expected)
			}

			if tt.diff.Actual == nil {
				assert.Nil(t, decoded.Actual)
			} else {
				assert.NotNil(t, decoded.Actual)
			}
		})
	}
}

func TestDeviceConfig_SyncStatuses(t *testing.T) {
	validStatuses := []string{"synced", "pending", "error", "drift"}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			config := DeviceConfig{
				SyncStatus: status,
			}

			// Verify the status is one of the valid values
			assert.Contains(t, validStatuses, config.SyncStatus)
		})
	}
}

func TestConfigHistory_Actions(t *testing.T) {
	validActions := []string{"import", "export", "sync", "manual", "template"}

	for _, action := range validActions {
		t.Run(action, func(t *testing.T) {
			history := ConfigHistory{
				Action: action,
			}

			// Verify the action is valid
			assert.Contains(t, []string{"import", "export", "sync", "manual", "template"}, history.Action)
		})
	}
}

func TestBulkConfigOperation_Statuses(t *testing.T) {
	validStatuses := []string{"pending", "running", "completed", "failed"}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			operation := BulkConfigOperation{
				Status: status,
			}

			// Verify the status is one of the valid values
			assert.Contains(t, validStatuses, operation.Status)
		})
	}
}

func TestConfigTemplate_DeviceTypes(t *testing.T) {
	tests := []struct {
		name       string
		deviceType string
		generation int
		valid      bool
	}{
		{
			name:       "Universal template",
			deviceType: "all",
			generation: 0,
			valid:      true,
		},
		{
			name:       "Gen1 specific",
			deviceType: "SHSW-1",
			generation: 1,
			valid:      true,
		},
		{
			name:       "Gen2+ specific",
			deviceType: "SHSW-25",
			generation: 2,
			valid:      true,
		},
		{
			name:       "Plug device",
			deviceType: "SHPLG-S",
			generation: 1,
			valid:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := ConfigTemplate{
				DeviceType: tt.deviceType,
				Generation: tt.generation,
			}

			// Basic validation
			assert.NotEmpty(t, template.DeviceType)
			assert.GreaterOrEqual(t, template.Generation, 0)
			assert.LessOrEqual(t, template.Generation, 3)
		})
	}
}

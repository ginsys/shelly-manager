package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
)

func TestValidateDeviceSettings(t *testing.T) {
	tests := []struct {
		name           string
		inputSettings  string
		expectedValid  bool
		expectedOutput string
	}{
		{
			name:           "Empty settings should get defaults",
			inputSettings:  "",
			expectedValid:  true,
			expectedOutput: `{"auth_enabled":false,"gen":1,"model":"Unknown"}`,
		},
		{
			name:           "Valid JSON settings should be normalized",
			inputSettings:  `{"model":"SHPLG-S","gen":1}`,
			expectedValid:  true,
			expectedOutput: `{"auth_enabled":false,"gen":1,"model":"SHPLG-S"}`,
		},
		{
			name:          "Invalid JSON should be rejected",
			inputSettings: `{"model":"SHPLG-S","gen":1,}`, // trailing comma
			expectedValid: false,
		},
		{
			name:           "Settings with all fields should remain unchanged",
			inputSettings:  `{"model":"SHPLG-S","gen":1,"auth_enabled":true}`,
			expectedValid:  true,
			expectedOutput: `{"auth_enabled":true,"gen":1,"model":"SHPLG-S"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{}
			device := &database.Device{
				Settings: tt.inputSettings,
			}

			err := handler.validateDeviceSettings(device)

			if tt.expectedValid {
				require.NoError(t, err, "Expected validation to succeed")
				if tt.expectedOutput != "" {
					assert.JSONEq(t, tt.expectedOutput, device.Settings, "Settings should be normalized correctly")
				}
			} else {
				require.Error(t, err, "Expected validation to fail")
			}
		})
	}
}

func TestAddDeviceWithValidation(t *testing.T) {
	// Create test database
	db, err := database.NewManagerFromPath(":memory:")
	require.NoError(t, err)

	// Create handler
	handler := NewHandler(db, nil, nil)

	tests := []struct {
		name           string
		inputDevice    database.Device
		expectedStatus int
	}{
		{
			name: "Device with empty settings should be created with defaults",
			inputDevice: database.Device{
				IP:       "192.168.1.100",
				MAC:      "AA:BB:CC:DD:EE:FF",
				Type:     "Test Device",
				Name:     "Test",
				Settings: "",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Device with valid settings should be created",
			inputDevice: database.Device{
				IP:       "192.168.1.101",
				MAC:      "AA:BB:CC:DD:EE:F0",
				Type:     "Test Device",
				Name:     "Test",
				Settings: `{"model":"SHPLG-S","gen":1,"auth_enabled":true}`,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "Device with invalid JSON settings should be rejected",
			inputDevice: database.Device{
				IP:       "192.168.1.102",
				MAC:      "AA:BB:CC:DD:EE:F1",
				Type:     "Test Device",
				Name:     "Test",
				Settings: `{"model":"SHPLG-S","gen":1,}`, // invalid JSON
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body
			body, err := json.Marshal(tt.inputDevice)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/api/v1/devices", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create router and add route
			router := mux.NewRouter()
			router.HandleFunc("/api/v1/devices", handler.AddDevice).Methods("POST")

			// Serve the request
			router.ServeHTTP(rr, req)

			// Check response status
			assert.Equal(t, tt.expectedStatus, rr.Code, "Expected status %d, got %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())

			// If creation was successful, verify the device was created with proper settings
			if tt.expectedStatus == http.StatusCreated {
				var responseDevice database.Device
				err := json.Unmarshal(rr.Body.Bytes(), &responseDevice)
				require.NoError(t, err)

				// Verify settings are valid JSON
				var settings map[string]interface{}
				err = json.Unmarshal([]byte(responseDevice.Settings), &settings)
				require.NoError(t, err, "Response device settings should be valid JSON")

				// Verify required fields exist
				assert.Contains(t, settings, "model", "Settings should contain model")
				assert.Contains(t, settings, "gen", "Settings should contain gen")
				assert.Contains(t, settings, "auth_enabled", "Settings should contain auth_enabled")
			}
		})
	}
}

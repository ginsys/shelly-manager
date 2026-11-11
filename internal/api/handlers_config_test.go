package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func setupTestHandler(t *testing.T) (*Handler, func()) {
	t.Helper()

	db, cleanup := testutil.TestDatabase(t)
	logger := logging.GetDefault()

	handler := &Handler{
		DB:            db,
		logger:        logger,
		ConfigService: configuration.NewService(db.GetDB(), logger),
	}

	return handler, cleanup
}

func TestGetNewConfigTemplates(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	template := &configuration.ServiceConfigTemplate{
		Name:   "test-template",
		Scope:  "global",
		Config: json.RawMessage(`{"mqtt":{"enable":true}}`),
	}
	err := handler.ConfigService.ConfigurationSvc.CreateTemplate(template)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/config/templates/new", nil)
	w := httptest.NewRecorder()

	handler.GetNewConfigTemplates(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Templates []TemplateResponse `json:"templates"`
		} `json:"data"`
	}
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Len(t, response.Data.Templates, 1)
	assert.Equal(t, "test-template", response.Data.Templates[0].Name)
}

func TestCreateNewConfigTemplate(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	reqBody := CreateTemplateRequest{
		Name:   "new-template",
		Scope:  "global",
		Config: &configuration.DeviceConfiguration{},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/config/templates/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateNewConfigTemplate(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Template TemplateResponse `json:"template"`
		} `json:"data"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "new-template", response.Data.Template.Name)
	assert.NotZero(t, response.Data.Template.ID)
}

func TestCreateNewConfigTemplate_ValidationErrors(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name    string
		reqBody CreateTemplateRequest
	}{
		{
			name:    "missing name",
			reqBody: CreateTemplateRequest{Scope: "global", Config: &configuration.DeviceConfiguration{}},
		},
		{
			name:    "missing scope",
			reqBody: CreateTemplateRequest{Name: "test", Config: &configuration.DeviceConfiguration{}},
		},
		{
			name:    "missing config",
			reqBody: CreateTemplateRequest{Name: "test", Scope: "global"},
		},
		{
			name:    "invalid scope",
			reqBody: CreateTemplateRequest{Name: "test", Scope: "invalid", Config: &configuration.DeviceConfiguration{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest("POST", "/api/v1/config/templates/new", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateNewConfigTemplate(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestGetNewConfigTemplate(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	template := &configuration.ServiceConfigTemplate{
		Name:   "test-template",
		Scope:  "global",
		Config: json.RawMessage(`{}`),
	}
	err := handler.ConfigService.ConfigurationSvc.CreateTemplate(template)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/config/templates/new/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler.GetNewConfigTemplate(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetNewConfigTemplate_NotFound(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/config/templates/new/999", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	w := httptest.NewRecorder()

	handler.GetNewConfigTemplate(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteNewConfigTemplate(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	template := &configuration.ServiceConfigTemplate{
		Name:   "test-template",
		Scope:  "global",
		Config: json.RawMessage(`{}`),
	}
	err := handler.ConfigService.ConfigurationSvc.CreateTemplate(template)
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/config/templates/new/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	handler.DeleteNewConfigTemplate(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	_, err = handler.ConfigService.ConfigurationSvc.GetTemplate(1)
	assert.ErrorIs(t, err, configuration.ErrTemplateNotFound)
}

func TestListAllNewTags(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/tags/new", nil)
	w := httptest.NewRecorder()

	handler.ListAllNewTags(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Tags   []string       `json:"tags"`
			Counts map[string]int `json:"counts"`
		} `json:"data"`
	}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response.Success)
}

func TestSecretRedaction(t *testing.T) {
	wifiPass := "secret-wifi"
	mqttPass := "secret-mqtt"
	authPass := "secret-auth"

	config := &configuration.DeviceConfiguration{
		WiFi: &configuration.WiFiConfiguration{
			Password: &wifiPass,
		},
		MQTT: &configuration.MQTTConfiguration{
			Password: &mqttPass,
		},
		Auth: &configuration.AuthConfiguration{
			Password: &authPass,
		},
	}

	assert.True(t, hasWiFiPassword(config))
	assert.True(t, hasMQTTPassword(config))
	assert.True(t, hasAuthPassword(config))

	redactSecrets(config)

	assert.Nil(t, config.WiFi.Password)
	assert.Nil(t, config.MQTT.Password)
	assert.Nil(t, config.Auth.Password)
}

func TestSecretRedaction_NoSecrets(t *testing.T) {
	config := &configuration.DeviceConfiguration{
		WiFi: &configuration.WiFiConfiguration{
			Enable: boolPtr(true),
		},
		MQTT: &configuration.MQTTConfiguration{
			Enable: boolPtr(true),
		},
	}

	assert.False(t, hasWiFiPassword(config))
	assert.False(t, hasMQTTPassword(config))
	assert.False(t, hasAuthPassword(config))
}

func TestMaskSecrets(t *testing.T) {
	wifiPass, empty := "secret-wifi", ""
	config := &configuration.DeviceConfiguration{
		WiFi: &configuration.WiFiConfiguration{Password: &wifiPass},
		Auth: &configuration.AuthConfiguration{Password: &empty},
		MQTT: &configuration.MQTTConfiguration{},
	}

	maskSecrets(config)

	// A set secret shows that it is set, never its value. An empty or absent one stays as it was.
	require.NotNil(t, config.WiFi.Password)
	assert.Equal(t, secretMask, *config.WiFi.Password)
	assert.Equal(t, "", *config.Auth.Password)
	assert.Nil(t, config.MQTT.Password)

	assert.NotPanics(t, func() { maskSecrets(nil) })
}

// The desired configuration keeps only the winning layer of a field, so a device override hides
// the template value under it. The response also carries the template-only configuration, which
// is what a reset field falls back to, with a set template secret shown as set and never as its value.
func TestGetDeviceDesiredConfig_CarriesTheInheritedConfiguration(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	device := testutil.TestDevice()
	require.NoError(t, handler.DB.AddDevice(device))

	svc := handler.ConfigService.ConfigurationSvc
	template := &configuration.ServiceConfigTemplate{
		Name:   "Global",
		Scope:  "global",
		Config: json.RawMessage(`{"mqtt":{"server":"global.broker","port":1883},"auth":{"enable":true,"user":"admin","pass":"template-secret"}}`),
	}
	require.NoError(t, svc.CreateTemplate(template))
	require.NoError(t, svc.SetDeviceTemplates(device.ID, []uint{template.ID}))
	port := 8883
	require.NoError(t, svc.SetDeviceOverrides(device.ID, &configuration.DeviceConfiguration{
		MQTT: &configuration.MQTTConfiguration{Port: &port},
	}))

	req := httptest.NewRequest("GET", "/api/v1/devices/1/desired-config", nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.FormatUint(uint64(device.ID), 10)})
	w := httptest.NewRecorder()
	handler.GetDeviceDesiredConfig(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data struct {
			Config           map[string]any    `json:"config"`
			Sources          map[string]string `json:"sources"`
			Inherited        map[string]any    `json:"inherited"`
			InheritedSources map[string]string `json:"inherited_sources"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response))

	assert.EqualValues(t, 8883, response.Data.Config["mqtt"].(map[string]any)["port"])
	assert.Equal(t, "device-override", response.Data.Sources["mqtt.port"])

	inheritedMQTT := response.Data.Inherited["mqtt"].(map[string]any)
	assert.EqualValues(t, 1883, inheritedMQTT["port"])
	assert.Equal(t, "global.broker", inheritedMQTT["server"])
	assert.Equal(t, "Global", response.Data.InheritedSources["mqtt.port"])

	inheritedAuth := response.Data.Inherited["auth"].(map[string]any)
	assert.Equal(t, secretMask, inheritedAuth["pass"])
	assert.NotContains(t, w.Body.String(), "template-secret")
}

func boolPtr(b bool) *bool {
	return &b
}

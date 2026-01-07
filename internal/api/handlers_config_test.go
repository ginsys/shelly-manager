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

func boolPtr(b bool) *bool {
	return &b
}

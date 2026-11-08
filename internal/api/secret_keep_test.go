package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

func putTemplate(t *testing.T, handler *Handler, id uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/config/templates/new/%d", id), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	handler.UpdateNewConfigTemplate(w, req)
	return w
}

func TestUpdateTemplateKeepsAnOmittedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	tmpl := &configuration.ServiceConfigTemplate{
		Name: "mqtt", Scope: configuration.ScopeGlobal,
		Config: json.RawMessage(`{"mqtt":{"user":"iot","pass":"x"},"auth":{"user":"admin","pass":"y"}}`),
	}
	require.NoError(t, handler.ConfigService.ConfigurationSvc.CreateTemplate(tmpl))

	// The body a client read back: secrets redacted, one other field changed.
	w := putTemplate(t, handler, tmpl.ID, `{"config":{"mqtt":{"user":"iot2"},"auth":{"user":"admin"}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetTemplate(tmpl.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"mqtt":{"user":"iot2","pass":"x"},"auth":{"user":"admin","pass":"y"}}`, string(stored.Config))

	var resp struct {
		Data struct {
			Template TemplateResponse `json:"template"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{"mqtt.pass", "auth.pass"}, resp.Data.Template.StoredSecrets)
	assert.NotContains(t, w.Body.String(), `"x"`, "the response must not carry the secret")
}

func TestUpdateTemplateClearsASecretOnEmptyString(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	tmpl := &configuration.ServiceConfigTemplate{
		Name: "mqtt", Scope: configuration.ScopeGlobal,
		Config: json.RawMessage(`{"mqtt":{"user":"iot","pass":"x"}}`),
	}
	require.NoError(t, handler.ConfigService.ConfigurationSvc.CreateTemplate(tmpl))

	w := putTemplate(t, handler, tmpl.ID, `{"config":{"mqtt":{"user":"iot","pass":""}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetTemplate(tmpl.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"mqtt":{"user":"iot"}}`, string(stored.Config))
}

func overrideDevice(t *testing.T, handler *Handler, overrides string) uint {
	t.Helper()
	device := &database.Device{IP: "192.0.2.40", MAC: "AA:BB:CC:DD:EE:40", Type: "SHPLG-S", Name: "plug"}
	require.NoError(t, handler.DB.AddDevice(device))
	var cfg configuration.DeviceConfiguration
	require.NoError(t, json.Unmarshal([]byte(overrides), &cfg))
	require.NoError(t, handler.ConfigService.ConfigurationSvc.SetDeviceOverrides(device.ID, &cfg))
	return device.ID
}

func putOverrides(t *testing.T, handler *Handler, id uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/devices/%d/overrides/new", id), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	handler.SetDeviceNewOverrides(w, req)
	return w
}

func TestSetOverridesKeepsAnOmittedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := overrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := putOverrides(t, handler, id, `{"mqtt":{"user":"iot2"},"led":{"power_indication":false}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT)
	require.NotNil(t, stored.MQTT.Password, "the stored MQTT password must be kept")
	assert.Equal(t, "x", *stored.MQTT.Password)
	assert.Equal(t, "iot2", *stored.MQTT.User)

	var resp struct {
		Data struct {
			StoredSecrets []string `json:"stored_secrets"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{"mqtt.pass"}, resp.Data.StoredSecrets)
	assert.NotContains(t, w.Body.String(), `"x"`, "the response must not carry the secret")
}

func TestSetOverridesClearsASecretOnEmptyString(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := overrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := putOverrides(t, handler, id, `{"mqtt":{"user":"iot","pass":""}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT)
	assert.Nil(t, stored.MQTT.Password)
}

func TestGetOverridesListsStoredSecrets(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := overrideDevice(t, handler, `{"auth":{"user":"admin","pass":"y"}}`)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/overrides/new", id), nil)
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	handler.GetDeviceNewOverrides(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Data struct {
			StoredSecrets []string `json:"stored_secrets"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []string{"auth.pass"}, resp.Data.StoredSecrets)
	assert.NotContains(t, w.Body.String(), `"y"`)
}

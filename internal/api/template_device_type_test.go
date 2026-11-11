package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

func createTemplateBody(t *testing.T, handler *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/config/templates/new", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateNewConfigTemplate(w, req)
	return w
}

func updateTemplateBody(t *testing.T, handler *Handler, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("PUT", "/api/v1/config/templates/new/"+id, strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": id})
	w := httptest.NewRecorder()
	handler.UpdateNewConfigTemplate(w, req)
	return w
}

// A device-type template names a model the server supports. An unknown type used to get a form
// with every section and nothing applicable, and no hint that the type was wrong.
func TestCreateTemplateRefusesAnUnknownDeviceType(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	w := createTemplateBody(t, handler,
		`{"name":"x","scope":"device_type","device_type":"SHXX-NOPE","config":{}}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	fields := decodeFieldErrors(t, w)
	require.Contains(t, fields, "device_type")
	assert.Contains(t, fields["device_type"], "SHXX-NOPE")

	templates, err := handler.ConfigService.ConfigurationSvc.ListTemplates("")
	require.NoError(t, err)
	assert.Empty(t, templates, "a refused type must not be stored")
}

func TestCreateTemplateAcceptsEverySupportedDeviceType(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	for i, deviceType := range configuration.NewGen1Converter(nil).SupportedDeviceTypes() {
		body := `{"name":"t` + string(rune('a'+i)) + `","scope":"device_type","device_type":"` + deviceType + `","config":{}}`
		w := createTemplateBody(t, handler, body)
		require.Equal(t, http.StatusCreated, w.Code, deviceType+": "+w.Body.String())
	}
}

// The type belongs to a device-type template only. A group or global template never reads it.
func TestCreateTemplateIgnoresTheTypeOfAGlobalTemplate(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	w := createTemplateBody(t, handler,
		`{"name":"g","scope":"global","device_type":"SHXX-NOPE","config":{}}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// An empty type keeps its own refusal from the service.
func TestCreateTemplateStillRequiresADeviceType(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	w := createTemplateBody(t, handler, `{"name":"x","scope":"device_type","config":{}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// A template stored before the check may carry a type outside the list. It stays editable, and a
// body that repeats its own stored type is not a change.
func TestUpdateTemplateKeepsALegacyDeviceType(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	legacy := &configuration.ServiceConfigTemplate{
		Name: "legacy", Scope: configuration.ScopeDeviceType, DeviceType: "SHXX-OLD",
		Config: json.RawMessage(`{}`),
	}
	require.NoError(t, handler.ConfigService.ConfigurationSvc.CreateTemplate(legacy))

	w := updateTemplateBody(t, handler, "1", `{"name":"renamed"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = updateTemplateBody(t, handler, "1", `{"name":"again","device_type":"SHXX-OLD"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// A body that names a different type than the stored one is checked. The update API keeps the
// type, so a different unknown one is refused instead of being silently ignored.
func TestUpdateTemplateRefusesADifferentUnknownDeviceType(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	plug := &configuration.ServiceConfigTemplate{
		Name: "plug", Scope: configuration.ScopeDeviceType, DeviceType: "SHPLG-S",
		Config: json.RawMessage(`{}`),
	}
	require.NoError(t, handler.ConfigService.ConfigurationSvc.CreateTemplate(plug))

	w := updateTemplateBody(t, handler, "1", `{"name":"plug2","device_type":"SHXX-NOPE"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, decodeFieldErrors(t, w), "device_type")

	stored, err := handler.ConfigService.ConfigurationSvc.GetTemplate(1)
	require.NoError(t, err)
	assert.Equal(t, "plug", stored.Name, "a refused update must not be stored")
}

// GET /api/v1/config/device-types fills the picker of the create dialog.
func TestGetDeviceTypesListsTheSupportedModels(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/config/device-types", nil)
	w := httptest.NewRecorder()
	handler.GetDeviceTypes(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			DeviceTypes []struct {
				DeviceType string `json:"device_type"`
				Name       string `json:"name"`
				Generation int    `json:"generation"`
			} `json:"device_types"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.True(t, body.Success)

	var codes []string
	for _, info := range body.Data.DeviceTypes {
		codes = append(codes, info.DeviceType)
		assert.NotEmpty(t, info.Name, info.DeviceType)
		assert.Equal(t, 1, info.Generation, info.DeviceType)
	}
	assert.Equal(t, configuration.NewGen1Converter(nil).SupportedDeviceTypes(), codes)
}

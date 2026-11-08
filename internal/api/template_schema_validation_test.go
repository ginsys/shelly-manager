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

type fieldErrorBody struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string `json:"code"`
		Details struct {
			Fields []configuration.FieldError `json:"fields"`
		} `json:"details"`
	} `json:"error"`
}

func decodeFieldErrors(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body fieldErrorBody
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.False(t, body.Success)
	assert.Equal(t, "VALIDATION_FAILED", body.Error.Code)
	out := map[string]string{}
	for _, f := range body.Error.Details.Fields {
		out[f.Path] = f.Message
	}
	return out
}

func TestCreateTemplateRejectsFieldsAgainstTheModelSchema(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	body := `{"name":"plug","scope":"device_type","device_type":"SHPLG-S",
		"config":{"power_metering":{"max_power":-1},"relay":{"default_state":"sometimes"},"roller":{"max_time":10}}}`
	req := httptest.NewRequest("POST", "/api/v1/config/templates/new", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateNewConfigTemplate(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, map[string]string{
		"power_metering.max_power": "Must be at least 0",
		"relay.default_state":      "Must be one of: off, on, last, switch",
		"roller":                   "SHPLG-S has no roller settings",
	}, decodeFieldErrors(t, w))

	templates, err := handler.ConfigService.ConfigurationSvc.ListTemplates("")
	require.NoError(t, err)
	assert.Empty(t, templates, "a rejected body must not be stored")
}

func TestCreateTemplateReportsATypeErrorAsAField(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// DeviceConfiguration decoding would fail on this with a generic message.
	body := `{"name":"g","scope":"global","config":{"relay":{"auto_off":"soon"}}}`
	req := httptest.NewRequest("POST", "/api/v1/config/templates/new", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateNewConfigTemplate(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, map[string]string{"relay.auto_off": "Must be a whole number"}, decodeFieldErrors(t, w))
}

func TestCreateTemplateAcceptsAValidPlugBody(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	body := `{"name":"plug","scope":"device_type","device_type":"SHPLG-S",
		"config":{"power_metering":{"max_power":1400},"relay":{"auto_off":3600}}}`
	req := httptest.NewRequest("POST", "/api/v1/config/templates/new", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.CreateNewConfigTemplate(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestUpdateTemplateValidatesAgainstTheStoredScope(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	tmpl := &configuration.ServiceConfigTemplate{
		Name: "plug", Scope: configuration.ScopeDeviceType, DeviceType: "SHPLG-S",
		Config: json.RawMessage(`{}`),
	}
	require.NoError(t, handler.ConfigService.ConfigurationSvc.CreateTemplate(tmpl))

	// The request carries no scope; the stored template's model decides the schema.
	req := httptest.NewRequest("PUT", "/api/v1/config/templates/new/1",
		strings.NewReader(`{"config":{"dimming":{"default_state":true}}}`))
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()
	handler.UpdateNewConfigTemplate(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, map[string]string{"dimming": "SHPLG-S has no dimming settings"}, decodeFieldErrors(t, w))

	stored, err := handler.ConfigService.ConfigurationSvc.GetTemplate(1)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(stored.Config))
}

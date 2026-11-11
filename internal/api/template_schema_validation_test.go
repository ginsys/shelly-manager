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
		"config":{"power_metering":{"max_power":-1},"relay":{"default_state":"sometimes"},"roller":{"max_open_time":10}}}`
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

// The decoder that stores a body matches a key without regard to case and merges a repeated
// section, so validation must refuse those shapes instead of checking a different body than the
// one that is stored. Each refusal is a 400 that names the key, and nothing is stored.
func TestTemplateBodyKeysAreRefusedWhenStoredDifferently(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   map[string]string
	}{
		{"differently cased section", `{"Relay":{"auto_off":-1}}`,
			map[string]string{"Relay": `Unknown key "Relay"; the key is "relay"`}},
		{"differently cased field", `{"relay":{"AUTO_OFF":-1}}`,
			map[string]string{"relay.AUTO_OFF": `Unknown key "AUTO_OFF"; the key is "auto_off"`}},
		{"unknown section", `{"relays":{"auto_off":5}}`,
			map[string]string{"relays": `Unknown key "relays"`}},
		{"unknown field", `{"relay":{"auto_of":5}}`,
			map[string]string{"relay.auto_of": `Unknown key "auto_of"`}},
		{"duplicate section", `{"relay":{"auto_off":-1},"relay":{}}`,
			map[string]string{"relay": `Duplicate key "relay"`}},
		{"duplicate field", `{"relay":{"auto_off":5,"auto_off":6}}`,
			map[string]string{"relay.auto_off": `Duplicate key "auto_off"`}},
	}
	for _, tc := range cases {
		t.Run("create "+tc.name, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()

			body := `{"name":"plug","scope":"device_type","device_type":"SHPLG-S","config":` + tc.config + `}`
			req := httptest.NewRequest("POST", "/api/v1/config/templates/new", strings.NewReader(body))
			w := httptest.NewRecorder()
			handler.CreateNewConfigTemplate(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Equal(t, tc.want, decodeFieldErrors(t, w))
			templates, err := handler.ConfigService.ConfigurationSvc.ListTemplates("")
			require.NoError(t, err)
			assert.Empty(t, templates, "a rejected body must not be stored")
		})
		t.Run("update "+tc.name, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()

			tmpl := &configuration.ServiceConfigTemplate{
				Name: "plug", Scope: configuration.ScopeDeviceType, DeviceType: "SHPLG-S",
				Config: json.RawMessage(`{}`),
			}
			require.NoError(t, handler.ConfigService.ConfigurationSvc.CreateTemplate(tmpl))

			req := httptest.NewRequest("PUT", "/api/v1/config/templates/new/1",
				strings.NewReader(`{"config":`+tc.config+`}`))
			req = mux.SetURLVars(req, map[string]string{"id": "1"})
			w := httptest.NewRecorder()
			handler.UpdateNewConfigTemplate(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Equal(t, tc.want, decodeFieldErrors(t, w))
			stored, err := handler.ConfigService.ConfigurationSvc.GetTemplate(1)
			require.NoError(t, err)
			assert.JSONEq(t, `{}`, string(stored.Config))
		})
	}
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

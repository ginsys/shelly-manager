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

// A client reads a template or the overrides of a device without the secret values, edits the
// body and sends it back. These tests pin what a save does with a secret the body leaves out.

func newKeepTemplate(t *testing.T, handler *Handler, config string) uint {
	t.Helper()
	tmpl := &configuration.ServiceConfigTemplate{
		Name: "keep", Scope: configuration.ScopeGlobal, Config: json.RawMessage(config),
	}
	require.NoError(t, handler.ConfigService.ConfigurationSvc.CreateTemplate(tmpl))
	return tmpl.ID
}

func putKeepTemplate(t *testing.T, handler *Handler, id uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/config/templates/new/%d", id), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	handler.UpdateNewConfigTemplate(w, req)
	return w
}

func storedTemplateConfig(t *testing.T, handler *Handler, id uint) string {
	t.Helper()
	stored, err := handler.ConfigService.ConfigurationSvc.GetTemplate(id)
	require.NoError(t, err)
	return string(stored.Config)
}

func templateStoredSecrets(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var resp struct {
		Data struct {
			Template TemplateResponse `json:"template"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.Data.Template.StoredSecrets
}

func TestUpdateTemplateKeepsAnOmittedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"x"},"auth":{"user":"admin","pass":"y"}}`)

	// The body a client read back: secrets left out, one other field changed.
	w := putKeepTemplate(t, handler, id, `{"config":{"mqtt":{"user":"iot2"},"auth":{"user":"admin"}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.JSONEq(t, `{"mqtt":{"user":"iot2","pass":"x"},"auth":{"user":"admin","pass":"y"}}`,
		storedTemplateConfig(t, handler, id))
	assert.ElementsMatch(t, []string{"mqtt.pass", "auth.pass"}, templateStoredSecrets(t, w))
	assert.NotContains(t, w.Body.String(), `"x"`, "the response must not carry the secret")
}

func TestUpdateTemplateKeepsASecretWhoseSectionIsOmitted(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler,
		`{"mqtt":{"user":"iot","pass":"x"},"network":{"wifi":{"sta":{"ssid":"home","pass":"net-pass"}}}}`)

	w := putKeepTemplate(t, handler, id, `{"config":{"led":{"power_indication":false}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got configuration.DeviceConfiguration
	require.NoError(t, json.Unmarshal([]byte(storedTemplateConfig(t, handler, id)), &got))
	require.NotNil(t, got.MQTT)
	require.NotNil(t, got.MQTT.Password)
	assert.Equal(t, "x", *got.MQTT.Password)
	require.NotNil(t, got.Network.WiFi.STA)
	assert.Equal(t, "net-pass", got.Network.WiFi.STA.Password)
	assert.NotNil(t, got.LED, "the field the client sent is still saved")
}

// The has_wifi_password flag is one flag for two passwords. stored_secrets names the path, so a
// client can tell an access point password from a station password.
func TestUpdateTemplateKeepsAnAccessPointPasswordAndNamesIt(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler, `{"wifi":{"ssid":"home","ap":{"pass":"ap-secret"}}}`)

	w := putKeepTemplate(t, handler, id, `{"config":{"wifi":{"ssid":"home2"}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.JSONEq(t, `{"wifi":{"ssid":"home2","ap":{"pass":"ap-secret"}}}`, storedTemplateConfig(t, handler, id))
	assert.Equal(t, []string{"wifi.ap.pass"}, templateStoredSecrets(t, w))
	assert.NotContains(t, w.Body.String(), "ap-secret")
}

func TestUpdateTemplateClearsASecretOnEmptyString(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := putKeepTemplate(t, handler, id, `{"config":{"mqtt":{"user":"iot","pass":""}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.JSONEq(t, `{"mqtt":{"user":"iot","pass":""}}`, storedTemplateConfig(t, handler, id))
	assert.Empty(t, templateStoredSecrets(t, w))
}

// A plain string cannot tell an omitted value from an empty one, so "" keeps the stored password.
// This is the named limit: such a password cannot be cleared through this path.
func TestUpdateTemplateKeepsANetworkPasswordSentAsEmpty(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler, `{"network":{"wifi":{"sta":{"ssid":"home","pass":"net-pass"}}}}`)

	w := putKeepTemplate(t, handler, id, `{"config":{"network":{"wifi":{"sta":{"ssid":"home","pass":""}}}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got configuration.DeviceConfiguration
	require.NoError(t, json.Unmarshal([]byte(storedTemplateConfig(t, handler, id)), &got))
	assert.Equal(t, "net-pass", got.Network.WiFi.STA.Password)
}

func TestUpdateTemplateRestoresAMaskedSecretAndNeverStoresTheMask(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := putKeepTemplate(t, handler, id, `{"config":{"mqtt":{"user":"iot","pass":"********"},"auth":{"pass":"********"}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored := storedTemplateConfig(t, handler, id)
	assert.NotContains(t, stored, secretMask)
	assert.Contains(t, stored, `"pass":"x"`)
}

func TestCreateTemplateNeverStoresTheMask(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/v1/config/templates/new",
		strings.NewReader(`{"name":"m","scope":"global","config":{"mqtt":{"user":"iot","pass":"********"}}}`))
	w := httptest.NewRecorder()
	handler.CreateNewConfigTemplate(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp struct {
		Data struct {
			Template TemplateResponse `json:"template"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotContains(t, storedTemplateConfig(t, handler, resp.Data.Template.ID), secretMask)
}

func TestGetTemplateListsStoredSecrets(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler, `{"wifi":{"password":"w","ap":{"key":"k"}},"auth":{"pass":"y"}}`)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/config/templates/new/%d", id), nil)
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	handler.GetNewConfigTemplate(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Data struct {
			Template TemplateResponse `json:"template"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.ElementsMatch(t, []string{"wifi.password", "wifi.ap.key", "auth.pass"}, resp.Data.Template.StoredSecrets)
	assert.NotContains(t, w.Body.String(), `"y"`)
}

func keepOverrideDevice(t *testing.T, handler *Handler, overrides string) uint {
	t.Helper()
	device := &database.Device{IP: "192.0.2.40", MAC: "AA:BB:CC:DD:EE:40", Type: "SHPLG-S", Name: "plug"}
	require.NoError(t, handler.DB.AddDevice(device))
	var cfg configuration.DeviceConfiguration
	require.NoError(t, json.Unmarshal([]byte(overrides), &cfg))
	require.NoError(t, handler.ConfigService.ConfigurationSvc.SetDeviceOverrides(device.ID, &cfg))
	return device.ID
}

func sendOverrides(t *testing.T, handler *Handler, method string, id uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, fmt.Sprintf("/api/v1/devices/%d/overrides/new", id), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	if method == "PATCH" {
		handler.PatchDeviceNewOverrides(w, req)
	} else {
		handler.SetDeviceNewOverrides(w, req)
	}
	return w
}

func TestSetOverridesKeepsAnOmittedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"},"wifi":{"ap":{"pass":"ap-secret"}}}`)

	w := sendOverrides(t, handler, "PUT", id, `{"mqtt":{"user":"iot2"},"led":{"power_indication":false}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT.Password)
	assert.Equal(t, "x", *stored.MQTT.Password)
	assert.Equal(t, "iot2", *stored.MQTT.User)
	require.NotNil(t, stored.WiFi.AccessPoint.Password, "a secret in an omitted section is kept too")
	assert.Equal(t, "ap-secret", *stored.WiFi.AccessPoint.Password)
	assert.NotContains(t, w.Body.String(), `"x"`)
	assert.NotContains(t, w.Body.String(), "ap-secret")
}

func TestSetOverridesClearsASecretOnEmptyString(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := sendOverrides(t, handler, "PUT", id, `{"mqtt":{"user":"iot","pass":""}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT.Password, "the empty value is stored, not dropped")
	assert.Equal(t, "", *stored.MQTT.Password)
}

// An empty override password beats the password of an assigned template, and it still does after a
// later save that leaves the key out.
func TestEmptyOverrideSecretBeatsTheTemplateAcrossSaves(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	svc := handler.ConfigService.ConfigurationSvc
	tmplID := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"template-secret"}}`)
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)
	require.NoError(t, svc.SetDeviceTemplates(id, []uint{tmplID}))

	for _, body := range []string{
		`{"mqtt":{"user":"iot","pass":""}}`, // the user clears the password on purpose
		`{"mqtt":{"user":"iot2"}}`,          // a later save: the key is left out
	} {
		w := sendOverrides(t, handler, "PUT", id, body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		desired, _, err := svc.GetDesiredConfig(id)
		require.NoError(t, err)
		require.NotNil(t, desired.MQTT.Password, "after %s", body)
		assert.Equal(t, "", *desired.MQTT.Password, "after %s", body)
	}
}

func TestPatchOverridesKeepsAnOmittedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":{"user":"iot2"}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT.Password)
	assert.Equal(t, "x", *stored.MQTT.Password)
}

// JSON null on a secret path removes the override, so a device inherits the template secret again.
// It is the fourth value beside omitted (keep), "" (clear) and the mask (restore).
func TestSetOverridesRemovesASecretOnNull(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	svc := handler.ConfigService.ConfigurationSvc
	tmplID := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"template-secret"}}`)
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"},"wifi":{"ap":{"pass":"ap-secret"}}}`)
	require.NoError(t, svc.SetDeviceTemplates(id, []uint{tmplID}))

	w := sendOverrides(t, handler, "PUT", id, `{"mqtt":{"user":"iot","pass":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := svc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.Nil(t, stored.MQTT.Password, "null removes the override, it is not stored as empty")
	require.NotNil(t, stored.WiFi.AccessPoint.Password, "a secret the body omits is still kept")
	assert.Equal(t, "ap-secret", *stored.WiFi.AccessPoint.Password)
	desired, _, err := svc.GetDesiredConfig(id)
	require.NoError(t, err)
	require.NotNil(t, desired.MQTT.Password)
	assert.Equal(t, "template-secret", *desired.MQTT.Password, "the device inherits the template password again")
}

func TestSetOverridesRemovesANetworkPasswordOnNull(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"network":{"wifi":{"sta":{"ssid":"home","pass":"net-pass"}}}}`)

	w := sendOverrides(t, handler, "PUT", id, `{"network":{"wifi":{"sta":{"ssid":"home","pass":null}}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.Equal(t, "", stored.Network.WiFi.STA.Password, "null clears a plain-string password that \"\" cannot")
}

// Each secret path is a JSON key path, so a raw body can be read for null at it. This fails when a
// slot is listed under a name that is not its JSON key, which would make null silently do nothing.
func TestNullSecretPathsReadEverySecretSlot(t *testing.T) {
	paths := allSecretPaths()
	require.Len(t, paths, 7)
	nest := func(path string, leaf any) []byte {
		keys := strings.Split(path, ".")
		node := leaf
		for i := len(keys) - 1; i >= 0; i-- {
			node = map[string]any{keys[i]: node}
		}
		raw, err := json.Marshal(node)
		require.NoError(t, err)
		return raw
	}
	for _, path := range paths {
		assert.Equal(t, map[string]bool{path: true}, nullSecretPaths(nest(path, nil)), "null at %s", path)
		assert.Empty(t, nullSecretPaths(nest(path, "")), "an empty string at %s is not null", path)
		assert.Empty(t, nullSecretPaths(nest(path, "x")), "a value at %s is not null", path)
	}
	assert.Empty(t, nullSecretPaths([]byte(`{"mqtt":{"user":null}}`)), "null elsewhere is not a secret reset")
	assert.Empty(t, nullSecretPaths([]byte(`{"mqtt":null}`)), "a null section is not a secret reset")
}

func TestUpdateTemplateRemovesASecretOnNull(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := putKeepTemplate(t, handler, id, `{"config":{"mqtt":{"user":"iot","pass":null}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.JSONEq(t, `{"mqtt":{"user":"iot"}}`, storedTemplateConfig(t, handler, id))
	assert.Empty(t, templateStoredSecrets(t, w))
}

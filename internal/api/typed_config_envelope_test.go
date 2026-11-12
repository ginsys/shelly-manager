package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/testutil"
)

// These tests pin the contract of the typed-config routes: every answer, success or failure, is
// the {success, data | error} envelope, and the request bodies use the keys the Go types declare
// (configuration, device_model, configurations). GET /devices/{id}/capabilities keeps its bare
// success body on purpose, and only its failures are enveloped.

// envelope is the standard answer body.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	} `json:"error"`
}

// decodeEnvelope reads the body as an envelope and checks it is JSON.
func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json", w.Body.String())
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "the body is not JSON: %q", w.Body.String())
	return env
}

// requireData checks a success envelope and returns its data member as a map.
func requireData(t *testing.T, w *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	env := decodeEnvelope(t, w)
	require.True(t, env.Success, "success must be true: %s", w.Body.String())
	require.Nil(t, env.Error, w.Body.String())
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(env.Data, &data), "data is not an object: %s", w.Body.String())
	return data
}

// requireError checks an error envelope with the given status and code.
func requireError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) envelope {
	t.Helper()
	require.Equal(t, status, w.Code, w.Body.String())
	env := decodeEnvelope(t, w)
	require.False(t, env.Success, w.Body.String())
	require.NotNil(t, env.Error, "no error member: %s", w.Body.String())
	assert.Equal(t, code, env.Error.Code, w.Body.String())
	assert.NotEmpty(t, env.Error.Message, w.Body.String())
	return env
}

func TestTypedConfigRoutes_SuccessAnswersAreEnveloped(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"}}`)

	t.Run("normalized read", func(t *testing.T) {
		w := serve(h.GetTypedDeviceConfigNormalized, "GET", "/api/v1/devices/1/config/typed/normalized", id, "")
		assert.Contains(t, requireData(t, w), "configuration")
	})

	t.Run("save", func(t *testing.T) {
		body := `{"configuration":{"wifi":{"enable":true,"ssid":"office"}}}`
		w := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
		data := requireData(t, w)
		assert.Contains(t, data, "configuration")
		assert.Contains(t, string(data["validation"]), `"valid":true`)
	})

	t.Run("validate", func(t *testing.T) {
		body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"}},"device_model":"SHPLG-S","generation":1}`
		w := serve(h.ValidateTypedConfig, "POST", "/api/v1/config/validate-typed", "", body)
		assert.Equal(t, "true", string(requireData(t, w)["valid"]))
	})

	t.Run("convert to typed", func(t *testing.T) {
		body := `{"configuration":{"wifi_sta":{"enabled":true,"ssid":"home"}},"device_model":"SHPLG-S","generation":1}`
		w := serve(h.ConvertConfigToTyped, "POST", "/api/v1/config/convert-to-typed", "", body)
		assert.Contains(t, requireData(t, w), "configuration")
	})

	t.Run("convert to raw", func(t *testing.T) {
		w := serve(h.ConvertTypedToRaw, "POST", "/api/v1/config/convert-to-raw", "",
			`{"configuration":{"wifi":{"enable":true,"ssid":"home"}}}`)
		assert.Contains(t, requireData(t, w), "configuration")
	})

	t.Run("bulk validate", func(t *testing.T) {
		body := `{"configurations":[{"configuration":{"wifi":{"enable":true,"ssid":"home"}}},{"device_model":"SHPLG-S"}]}`
		w := serve(h.BulkValidateConfigs, "POST", "/api/v1/config/bulk-validate", "", body)
		data := requireData(t, w)
		assert.JSONEq(t, `{"total":2,"valid":1,"invalid":1}`, string(data["summary"]))
		var results []json.RawMessage
		require.NoError(t, json.Unmarshal(data["results"], &results))
		assert.Len(t, results, 2)
	})
}

// The capabilities answer is bare: the device page reads it that way and the API docs say so.
func TestGetDeviceCapabilities_SuccessStaysBare(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	device := testutil.TestDevice()
	require.NoError(t, h.DB.AddDevice(device))

	w := serve(h.GetDeviceCapabilities, "GET", "/api/v1/devices/1/capabilities", idString(device), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "capabilities")
	assert.Contains(t, body, "device_model")
	assert.NotContains(t, body, "success")
}

// A stored configuration that is not an object cannot be converted. Both typed reads answer 500
// with the generic message and say nothing of the conversion error. Before this change the typed
// read answered {success:false, error:"<string>"} with the conversion error in the string.
func TestTypedConfigReads_ConversionFailureIsAGeneric500(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `5`)

	reads := map[string]http.HandlerFunc{
		"typed read":      h.GetTypedDeviceConfig,
		"normalized read": h.GetTypedDeviceConfigNormalized,
	}
	for name, handler := range reads {
		t.Run(name, func(t *testing.T) {
			w := serve(handler, "GET", "/api/v1/x", id, "")
			env := requireError(t, w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
			assert.Equal(t, "Internal server error", env.Error.Message, w.Body.String())
			assert.NotContains(t, w.Body.String(), "convert")
			assert.Empty(t, env.Error.Details, w.Body.String())
		})
	}
}

// Each failure answers with the error envelope, a status, a code and a message the web client can
// show. The refusals that carry a stored secret keep their own tests. Some cases (the bad id and
// no-device answers of the reads, the not-an-object conversion) already answered as they do now
// and pin that answer; the rest answered plain text or another shape before this change.
func TestTypedConfigRoutes_FailuresAreEnveloped(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"}}`)

	badSettings := testutil.TestDevice()
	badSettings.MAC = "AA:BB:CC:00:00:01"
	badSettings.IP = "192.168.1.201"
	badSettings.Settings = "not json"
	require.NoError(t, h.DB.AddDevice(badSettings))

	const (
		badReq   = "BAD_REQUEST"
		notFound = "NOT_FOUND"
		internal = "INTERNAL_SERVER_ERROR"
	)
	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		id      string
		body    string
		status  int
		code    string
	}{
		{"read: bad id", h.GetTypedDeviceConfig, "GET", "abc", "", 400, badReq},
		{"read: no device", h.GetTypedDeviceConfig, "GET", "9999", "", 404, notFound},
		{"normalized: bad id", h.GetTypedDeviceConfigNormalized, "GET", "abc", "", 400, badReq},
		{"normalized: no device", h.GetTypedDeviceConfigNormalized, "GET", "9999", "", 404, notFound},
		{"save: bad id", h.UpdateTypedDeviceConfig, "PUT", "abc", `{"configuration":{}}`, 400, badReq},
		{"save: no device", h.UpdateTypedDeviceConfig, "PUT", "9999", `{"configuration":{}}`, 404, notFound},
		{"save: invalid JSON", h.UpdateTypedDeviceConfig, "PUT", id, `{`, 400, badReq},
		{"save: no configuration", h.UpdateTypedDeviceConfig, "PUT", id, `{}`, 400, badReq},
		{"save: old key config", h.UpdateTypedDeviceConfig, "PUT", id, `{"config":{"wifi":{"enable":true}}}`, 400, badReq},
		{"validate: invalid JSON", h.ValidateTypedConfig, "POST", "", `{`, 400, badReq},
		{"validate: no configuration", h.ValidateTypedConfig, "POST", "", `{}`, 400, badReq},
		{"validate: old keys", h.ValidateTypedConfig, "POST", "", `{"config":{"wifi":{"enable":true}},"deviceType":"SHPLG-S"}`, 400, badReq},
		{"convert to typed: invalid JSON", h.ConvertConfigToTyped, "POST", "", `{`, 400, badReq},
		{"convert to typed: no configuration", h.ConvertConfigToTyped, "POST", "", `{}`, 400, badReq},
		{"convert to typed: old key", h.ConvertConfigToTyped, "POST", "", `{"config":{"wifi_sta":{"enabled":true}}}`, 400, badReq},
		{"convert to typed: not an object", h.ConvertConfigToTyped, "POST", "", `{"configuration":5}`, 500, internal},
		{"convert to raw: invalid JSON", h.ConvertTypedToRaw, "POST", "", `{`, 400, badReq},
		{"convert to raw: no configuration", h.ConvertTypedToRaw, "POST", "", `{}`, 400, badReq},
		{"convert to raw: old key", h.ConvertTypedToRaw, "POST", "", `{"config":{"wifi":{"enable":true}}}`, 400, badReq},
		{"bulk: invalid JSON", h.BulkValidateConfigs, "POST", "", `{`, 400, badReq},
		{"bulk: empty list", h.BulkValidateConfigs, "POST", "", `{"configurations":[]}`, 400, badReq},
		{"bulk: old key", h.BulkValidateConfigs, "POST", "", `{"configs":[{"config":{"wifi":{"enable":true}}}]}`, 400, badReq},
		{"capabilities: bad id", h.GetDeviceCapabilities, "GET", "abc", "", 400, badReq},
		{"capabilities: no device", h.GetDeviceCapabilities, "GET", "9999", "", 404, notFound},
		{"capabilities: settings not JSON", h.GetDeviceCapabilities, "GET", idString(badSettings), "", 500, internal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := serve(c.handler, c.method, "/api/v1/x", c.id, c.body)
			requireError(t, w, c.status, c.code)
		})
	}

	// The schema route is the ninth handler of the file; its failures are read from the query.
	schema := []struct {
		name   string
		query  string
		status int
		code   string
	}{
		{"schema: empty device_type", "device_type=", 400, badReq},
		{"schema: bad device_id", "device_id=abc", 400, badReq},
		{"schema: no device", "device_id=9999", 404, notFound},
		{"schema: two selectors", "device_type=SHPLG-S&device_id=1", 400, badReq},
		{"schema: bad scope", "scope=nope", 400, badReq},
	}
	for _, c := range schema {
		t.Run(c.name, func(t *testing.T) {
			w := serve(h.GetConfigurationSchema, "GET", "/api/v1/configuration/schema?"+c.query, "", "")
			requireError(t, w, c.status, c.code)
		})
	}
}

// A refused save answers 400 VALIDATION_FAILED, with the validation result in the details.
func TestUpdateTypedDeviceConfig_ValidationFailureCarriesTheResult(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"}}`)

	body := `{"configuration":{"wifi":{"enable":true,"ssid":"home","password":"short"}},"validation_level":"strict"}`
	w := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)

	env := requireError(t, w, http.StatusBadRequest, "VALIDATION_FAILED")
	assert.Contains(t, string(env.Error.Details), "WEAK_WIFI_PASSWORD")
	assert.Contains(t, string(env.Error.Details), `"valid":false`)
	assert.NotContains(t, storedConfig(t, h, id), "short", "a refused save must store nothing")
}

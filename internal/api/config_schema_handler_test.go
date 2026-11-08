package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

// getSchema calls GET /api/v1/config/schema with query and returns the status and the data.
func getSchema(t *testing.T, handler *Handler, query string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/config/schema"+query, nil)
	w := httptest.NewRecorder()
	handler.GetConfigurationSchema(w, req)

	var body struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	return w.Code, body.Data
}

func schemaSections(data map[string]interface{}) []string {
	props, _ := data["properties"].(map[string]interface{})
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestConfigSchemaWithoutDeviceIsGeneric(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	code, data := getSchema(t, handler, "")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, schemaSections(data), "energy_meter", "the generic schema keeps every section")
	assert.NotContains(t, data, configuration.SchemaKeyDeviceType)
}

func TestConfigSchemaForPlugModel(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	code, data := getSchema(t, handler, "?device_type=SHPLG-S")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"auth", "cloud", "coiot", "led", "location", "mqtt", "network", "power_metering", "relay", "system", "wifi"},
		schemaSections(data))
	assert.Equal(t, "SHPLG-S", data[configuration.SchemaKeyDeviceType])
	assert.Equal(t, true, data[configuration.SchemaKeyApplySupported])

	maxPower := data["properties"].(map[string]interface{})["power_metering"].(map[string]interface{})["properties"].(map[string]interface{})["max_power"].(map[string]interface{})
	assert.Equal(t, true, maxPower[configuration.SchemaKeyApplicable])
	defaultState := data["properties"].(map[string]interface{})["relay"].(map[string]interface{})["properties"].(map[string]interface{})["default_state"].(map[string]interface{})
	assert.Equal(t, false, defaultState[configuration.SchemaKeyApplicable])
}

func TestConfigSchemaForDeviceIDUsesStoredModel(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	// The display type is "Smart Plug"; the model the converters match on is in the settings.
	device := &database.Device{
		IP: "192.0.2.10", MAC: "AA:BB:CC:DD:EE:01", Type: "Smart Plug", Name: "plug",
		Settings: `{"model":"SHPLG-S"}`,
	}
	require.NoError(t, handler.DB.AddDevice(device))

	code, data := getSchema(t, handler, fmt.Sprintf("?device_id=%d", device.ID))
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "SHPLG-S", data[configuration.SchemaKeyDeviceType])
	assert.NotContains(t, schemaSections(data), "dimming")
}

func TestConfigSchemaForGlobalTemplate(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	code, data := getSchema(t, handler, "?scope=global")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"auth", "cloud", "coiot", "dimming", "input", "led", "location", "mqtt", "network", "power_metering", "relay", "roller", "system", "wifi"},
		schemaSections(data), "a global template holds every stored section, and no generic-only section")
	assert.Equal(t, true, data[configuration.SchemaKeyApplySupported])

	// A field apply writes on some model stays editable in a global template.
	maxPower := data["properties"].(map[string]interface{})["power_metering"].(map[string]interface{})["properties"].(map[string]interface{})["max_power"].(map[string]interface{})
	assert.Equal(t, true, maxPower[configuration.SchemaKeyApplicable])

	code, _ = getSchema(t, handler, "?scope=device_type")
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = getSchema(t, handler, "?scope=global&device_type=SHPLG-S")
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestConfigSchemaParameterErrors(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	code, _ := getSchema(t, handler, "?device_id=abc")
	assert.Equal(t, http.StatusBadRequest, code)

	code, _ = getSchema(t, handler, "?device_id=1&device_type=SHPLG-S")
	assert.Equal(t, http.StatusBadRequest, code)

	code, _ = getSchema(t, handler, "?device_id=999")
	assert.Equal(t, http.StatusNotFound, code)

	// A present but empty parameter is a 400, never the generic schema.
	for _, q := range []string{"?device_id=", "?device_id=&device_type=SHPLG-S", "?device_type=", "?device_type=&device_id=1"} {
		code, _ = getSchema(t, handler, q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}

	// A malformed pair is a 400, never the generic schema, and it cannot hide a second parameter.
	for _, q := range []string{"?device_id=%ZZ", "?device_type=SHPLG-S&device_id=%ZZ", "?device_type=%ZZ", "?device_type=SHPLG-S&x=%ZZ"} {
		code, _ = getSchema(t, handler, q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

// The device override routes check values against the device schema, as the template routes do:
// 400 VALIDATION_FAILED with the field list in error.details.fields, and nothing stored.

type refusedFields struct {
	Error struct {
		Code    string `json:"code"`
		Details struct {
			Fields []configuration.FieldError `json:"fields"`
		} `json:"details"`
	} `json:"error"`
}

func refusedFieldsOf(t *testing.T, w *httptest.ResponseRecorder) refusedFields {
	t.Helper()
	var resp refusedFields
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	return resp
}

func fieldPaths(fields []configuration.FieldError) []string {
	paths := make([]string, len(fields))
	for i, f := range fields {
		paths[i] = f.Path
	}
	return paths
}

func TestSetOverridesRefusesAnOutOfRangeValueAndStoresNothing(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot"}}`)

	w := sendOverrides(t, handler, "PUT", id, `{"mqtt":{"user":"changed","port":99999}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	resp := refusedFieldsOf(t, w)
	assert.Equal(t, "VALIDATION_FAILED", resp.Error.Code)
	assert.Equal(t, []string{"mqtt.port"}, fieldPaths(resp.Error.Details.Fields))

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT.User)
	assert.Equal(t, "iot", *stored.MQTT.User, "a refused body changes nothing")
	assert.Nil(t, stored.MQTT.Port)
}

func TestPatchOverridesRefusesAnOutOfRangeValueAndStoresNothing(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"location":{"lat":10}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"location":{"lat":500}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	resp := refusedFieldsOf(t, w)
	assert.Equal(t, "VALIDATION_FAILED", resp.Error.Code)
	assert.Equal(t, []string{"location.lat"}, fieldPaths(resp.Error.Details.Fields))

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.Location.Latitude)
	assert.Equal(t, 10.0, *stored.Location.Latitude)
}

func TestOverridesValidationListsEveryRefusedField(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{}`)

	w := sendOverrides(t, handler, "PUT", id, `{"mqtt":{"port":0,"keep_alive":99999},"location":{"lng":-181}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, []string{"location.lng", "mqtt.keep_alive", "mqtt.port"},
		fieldPaths(refusedFieldsOf(t, w).Error.Details.Fields))
}

func TestOverridesValidationReadsAMiscasedKeyAsTheFieldItStores(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{}`)

	// Go's decoder matches keys without regard to case, so this body would store mqtt.port.
	for _, method := range []string{"PUT", "PATCH"} {
		w := sendOverrides(t, handler, method, id, `{"MQTT":{"PORT":99999}}`)
		require.Equal(t, http.StatusBadRequest, w.Code, method+": "+w.Body.String())
		assert.Equal(t, []string{"mqtt.port"}, fieldPaths(refusedFieldsOf(t, w).Error.Details.Fields), method)
	}
}

func TestPatchOverridesNullRemovesAStoredBadValue(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","port":99999}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":{"port":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT)
	assert.Nil(t, stored.MQTT.Port, "the way out of a stored bad value stays open")
	require.NotNil(t, stored.MQTT.User)
}

// A section the model lacks can still be stored (a template or an older model change put it
// there). Removing its last value with a null must stay possible: the decoded body keeps an empty
// section, which holds nothing to check and must not read as the section being sent.
func TestPatchOverridesNullRemovesAStoredValueOfASectionTheModelLacks(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"roller":{"max_open_time":30},"mqtt":{"user":"iot"}}`) // a SHPLG-S plug has no roller mode

	w := sendOverrides(t, handler, "PATCH", id, `{"roller":{"max_open_time":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.True(t, stored.Roller == nil || stored.Roller.MaxOpenTime == nil, "the roller value is removed")
	require.NotNil(t, stored.MQTT, "another section is kept")

	// A value in that section is still refused.
	w = sendOverrides(t, handler, "PATCH", id, `{"roller":{"max_open_time":40}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestGetOverridesStillAnswersForAStoredBadValue(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"port":99999}}`)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/overrides/new", id), nil)
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	handler.GetDeviceNewOverrides(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "99999")
}

func TestSetOverridesRefusesASectionTheModelDoesNotHave(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{}`) // a SHPLG-S plug has no roller mode

	w := sendOverrides(t, handler, "PUT", id, `{"roller":{"max_open_time":30}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, []string{"roller"}, fieldPaths(refusedFieldsOf(t, w).Error.Details.Fields))
}

func TestOverridesValidationUsesEverySectionForADeviceWithoutAModel(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	device := &database.Device{IP: "192.0.2.41", MAC: "AA:BB:CC:DD:EE:41", Type: "", Name: "unknown"}
	require.NoError(t, handler.DB.AddDevice(device))

	w := sendOverrides(t, handler, "PUT", device.ID, `{"roller":{"max_open_time":30}}`)
	assert.Equal(t, http.StatusOK, w.Code, "a device with no model is checked against the unsupported-model schema: "+w.Body.String())

	w = sendOverrides(t, handler, "PUT", device.ID, `{"mqtt":{"port":99999}}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestOverridesValidationRefusesABadIPv4Address(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{}`)

	// The Wi-Fi apply writes wifi.static_ip.* to the device, so a bad address must stop here.
	w := sendOverrides(t, handler, "PUT", id,
		`{"wifi":{"ipv4mode":"static","static_ip":{"ip":"300.1.1.1","netmask":"255.255.255.0","gw":"192.0.2.1"}}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, []string{"wifi.static_ip.ip"}, fieldPaths(refusedFieldsOf(t, w).Error.Details.Fields))

	w = sendOverrides(t, handler, "PUT", id,
		`{"wifi":{"ipv4mode":"static","static_ip":{"ip":"192.0.2.50","netmask":"255.255.255.0","gw":"192.0.2.1","nameserver":""}}}`)
	assert.Equal(t, http.StatusOK, w.Code, "a valid address and an empty one are accepted: "+w.Body.String())
}

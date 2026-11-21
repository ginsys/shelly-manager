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

// Every secret slot has a binding row in the configuration package, and every row names a slot: a new
// secret field cannot be added with no decision about where its secret may be kept.
func TestSecretBindings_NameExactlyTheSlotsOfSecretSlots(t *testing.T) {
	assert.Equal(t, allSecretPaths(), configuration.SecretBindingPaths())
}

func rollbackDeviceWith(t *testing.T, handler *Handler) uint {
	t.Helper()
	device := &database.Device{IP: "192.0.2.50", MAC: "AA:BB:CC:DD:EE:50", Type: "SHPLG-S", Name: "rollback"}
	require.NoError(t, handler.DB.AddDevice(device))
	return device.ID
}

func putOverrides(t *testing.T, handler *Handler, id uint, overrides string) {
	t.Helper()
	w := sendOverrides(t, handler, "PUT", id, overrides)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func callHistory(t *testing.T, handler *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/devices/"+id+"/desired-config/history", nil)
	req = mux.SetURLVars(req, map[string]string{"id": id})
	w := httptest.NewRecorder()
	handler.GetDeviceDesiredHistory(w, req)
	return w
}

func callRollback(t *testing.T, handler *Handler, id uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d/config/new/rollback", id), strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprint(id)})
	w := httptest.NewRecorder()
	handler.RollbackDeviceDesiredConfig(w, req)
	return w
}

type historyAnswer struct {
	Data struct {
		History []DesiredHistoryEntry `json:"history"`
	} `json:"data"`
}

// versionHolding finds the id of the newest version whose text holds needle.
func versionHolding(t *testing.T, handler *Handler, id uint, needle string) uint {
	t.Helper()
	w := callHistory(t, handler, fmt.Sprint(id))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var answer historyAnswer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	for _, entry := range answer.Data.History {
		if entry.NewDesired != nil && strings.Contains(*entry.NewDesired, needle) {
			return entry.ID
		}
	}
	require.Failf(t, "no version", "no version holds %q in %s", needle, w.Body.String())
	return 0
}

type errorAnswer struct {
	Error struct {
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func TestDesiredHistoryList_NewestFirstAndMasked(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	putOverrides(t, handler, id, `{"mqtt":{"server":"a.example","user":"iot","pass":"sentinel-one"}}`)
	putOverrides(t, handler, id, `{"mqtt":{"server":"b.example","user":"iot","pass":"sentinel-two"}}`)

	w := callHistory(t, handler, fmt.Sprint(id))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "sentinel-one")
	assert.NotContains(t, w.Body.String(), "sentinel-two")

	var answer historyAnswer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	require.GreaterOrEqual(t, len(answer.Data.History), 2)
	assert.Contains(t, *answer.Data.History[0].NewDesired, "b.example", "newest first")
	assert.Contains(t, *answer.Data.History[0].NewDesired, secretMask)
	assert.Equal(t, "recompute", answer.Data.History[0].Action)
}

// A row written before a secret was masked at storage still holds it: the list masks it again.
func TestDesiredHistoryList_MasksAStoredSecretAgain(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	text := `{"mqtt":{"server":"a.example","pass":"raw-secret"}}`
	row := configuration.DesiredConfigHistory{DeviceID: id, Action: "recompute", NewDesired: &text, ChangedBy: "system"}
	require.NoError(t, handler.DB.GetDB().Create(&row).Error)

	w := callHistory(t, handler, fmt.Sprint(id))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "raw-secret")
	assert.Contains(t, w.Body.String(), "a.example")
}

func TestDesiredHistoryList_ABadOrMissingDevice(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	assert.Equal(t, http.StatusBadRequest, callHistory(t, handler, "x").Code)
	assert.Equal(t, http.StatusNotFound, callHistory(t, handler, "9999").Code)
}

func TestRollbackRoute_SetsTheVersionAndSaysTheDeviceIsUnchanged(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	putOverrides(t, handler, id, `{"mqtt":{"server":"a.example"}}`)
	first := versionHolding(t, handler, id, "a.example")
	putOverrides(t, handler, id, `{"mqtt":{"server":"b.example"}}`)

	w := callRollback(t, handler, id, fmt.Sprintf(`{"history_id":%d}`, first))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), fmt.Sprintf("Desired configuration set to version %d. The device is unchanged until you apply it.", first))

	device, err := handler.DB.GetDevice(id)
	require.NoError(t, err)
	assert.Contains(t, device.DesiredConfig, "a.example")
	assert.NotContains(t, device.DesiredConfig, "b.example")
	assert.False(t, device.ConfigApplied, "the device is pending until an apply")

	list := callHistory(t, handler, fmt.Sprint(id))
	var answer historyAnswer
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &answer))
	assert.Equal(t, "rollback", answer.Data.History[0].Action)
	require.NotNil(t, answer.Data.History[0].RestoredFrom)
	assert.Equal(t, first, *answer.Data.History[0].RestoredFrom)
}

func TestRollbackRoute_AVersionOfAnotherDeviceOrNoneIs404(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	putOverrides(t, handler, id, `{"mqtt":{"server":"a.example"}}`)
	version := versionHolding(t, handler, id, "a.example")

	other := &database.Device{IP: "192.0.2.51", MAC: "AA:BB:CC:DD:EE:51", Type: "SHPLG-S", Name: "other"}
	require.NoError(t, handler.DB.AddDevice(other))

	assert.Equal(t, http.StatusNotFound, callRollback(t, handler, other.ID, fmt.Sprintf(`{"history_id":%d}`, version)).Code)
	assert.Equal(t, http.StatusNotFound, callRollback(t, handler, id, `{"history_id":99999}`).Code)
	assert.Equal(t, http.StatusNotFound, callRollback(t, handler, 99999, fmt.Sprintf(`{"history_id":%d}`, version)).Code)
}

func TestRollbackRoute_ASecretIsAskedForThenAccepted(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	putOverrides(t, handler, id, `{"mqtt":{"server":"a.example","pass":"old-secret"}}`)
	first := versionHolding(t, handler, id, "a.example")
	putOverrides(t, handler, id, `{"mqtt":{"server":"b.example","pass":"cur-secret"}}`)
	before, err := handler.DB.GetDevice(id)
	require.NoError(t, err)

	w := callRollback(t, handler, id, fmt.Sprintf(`{"history_id":%d}`, first))
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var refusal errorAnswer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &refusal))
	assert.Equal(t, []any{"mqtt.pass"}, refusal.Error.Details["paths"])
	for _, secret := range []string{"old-secret", "cur-secret"} {
		assert.NotContains(t, w.Body.String(), secret)
	}
	after, err := handler.DB.GetDevice(id)
	require.NoError(t, err)
	assert.Equal(t, before.DesiredConfig, after.DesiredConfig, "a refusal writes nothing")

	w = callRollback(t, handler, id, fmt.Sprintf(`{"history_id":%d,"secrets":{"mqtt.pass":"entered-secret"}}`, first))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "entered-secret", "the answer never repeats a secret")
	done, err := handler.DB.GetDevice(id)
	require.NoError(t, err)
	assert.Contains(t, done.Overrides, "entered-secret")
	assert.NotContains(t, callHistory(t, handler, fmt.Sprint(id)).Body.String(), "entered-secret")
}

func TestRollbackRoute_TheCurrentSecretIsKeptWhereTheBrokerIsTheSame(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	putOverrides(t, handler, id, `{"mqtt":{"server":"a.example","user":"iot","pass":"cur-secret"},"led":{"power_indication":true}}`)
	first := versionHolding(t, handler, id, "power_indication")
	putOverrides(t, handler, id, `{"mqtt":{"server":"a.example","user":"iot","pass":"cur-secret"},"led":{"power_indication":false}}`)

	w := callRollback(t, handler, id, fmt.Sprintf(`{"history_id":%d}`, first))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	device, err := handler.DB.GetDevice(id)
	require.NoError(t, err)
	assert.Contains(t, device.Overrides, "cur-secret")
	assert.NotContains(t, w.Body.String(), "cur-secret")
}

func TestRollbackRoute_ABadBodyIsRefusedWithoutEchoingIt(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	putOverrides(t, handler, id, `{"mqtt":{"server":"a.example","pass":"old-secret"}}`)
	first := versionHolding(t, handler, id, "a.example")
	putOverrides(t, handler, id, `{"mqtt":{"server":"b.example","pass":"cur-secret"}}`)

	bodies := map[string]string{
		"truncated json":          fmt.Sprintf(`{"history_id":%d,"secrets":{"mqtt.pass":"sentinel-body`, first),
		"no history id":           `{"secrets":{"mqtt.pass":"sentinel-body"}}`,
		"a repeated member":       fmt.Sprintf(`{"history_id":%d,"secrets":{"mqtt.pass":"sentinel-body"},"secrets":{}}`, first),
		"a key off the list":      fmt.Sprintf(`{"history_id":%d,"secrets":{"mqtt.pass":"sentinel-body","auth.pass":"sentinel-body"}}`, first),
		"an empty value":          fmt.Sprintf(`{"history_id":%d,"secrets":{"mqtt.pass":""}}`, first),
		"the mask":                fmt.Sprintf(`{"history_id":%d,"secrets":{"mqtt.pass":"%s"}}`, first, secretMask),
		"an empty list":           fmt.Sprintf(`{"history_id":%d,"secrets":{"mqtt.pass":[]}}`, first),
		"a misspelt member":       fmt.Sprintf(`{"history_id":%d,"secret":{"mqtt.pass":"sentinel-body"}}`, first),
		"data after the body":     fmt.Sprintf(`{"history_id":%d} {"history_id":1}`, first),
		"a closing bracket after": fmt.Sprintf(`{"history_id":%d}]`, first),
		"a closing brace after":   fmt.Sprintf(`{"history_id":%d}}`, first),
		"a null secrets":          fmt.Sprintf(`{"history_id":%d,"secrets":null}`, first),
		"a list for secrets":      fmt.Sprintf(`{"history_id":%d,"secrets":["x"]}`, first),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			w := callRollback(t, handler, id, body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.NotContains(t, w.Body.String(), "sentinel-body")
			assert.NotContains(t, w.Body.String(), "cur-secret")
		})
	}
}

func TestRollbackRoute_AVersionWithNoStoredTextIs422(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	row := configuration.DesiredConfigHistory{DeviceID: id, Action: "recompute", ChangedBy: "system"}
	require.NoError(t, handler.DB.GetDB().Create(&row).Error)

	w := callRollback(t, handler, id, fmt.Sprintf(`{"history_id":%d}`, row.ID))
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
}

// Retention may leave more rows than it keeps for a while; the list still answers the last 50.
func TestDesiredHistoryList_AnswersAtMostFiftyVersions(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	for i := 0; i < 55; i++ {
		text := fmt.Sprintf(`{"led":{"n":%d}}`, i)
		row := configuration.DesiredConfigHistory{DeviceID: id, Action: "recompute", NewDesired: &text, ChangedBy: "system"}
		require.NoError(t, handler.DB.GetDB().Create(&row).Error)
	}

	var answer historyAnswer
	require.NoError(t, json.Unmarshal(callHistory(t, handler, fmt.Sprint(id)).Body.Bytes(), &answer))
	require.Len(t, answer.Data.History, 50)
	assert.Contains(t, *answer.Data.History[0].NewDesired, `"n":54`, "the newest first")
}

// A mask at a path that is no secret slot is not restorable: 422 with the paths, never a 409 that asks
// the operator to enter something that the next call would refuse.
func TestRollbackRoute_AMaskWhereNoSlotIsIs422WithThePaths(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := rollbackDeviceWith(t, handler)
	text := `{"custom":{"token":"` + secretMask + `"}}`
	row := configuration.DesiredConfigHistory{DeviceID: id, Action: "recompute", NewDesired: &text, ChangedBy: "system"}
	require.NoError(t, handler.DB.GetDB().Create(&row).Error)

	w := callRollback(t, handler, id, fmt.Sprintf(`{"history_id":%d,"secrets":{"custom.token":"sentinel-body"}}`, row.ID))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	var refusal errorAnswer
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &refusal))
	assert.Equal(t, []any{"custom.token"}, refusal.Error.Details["paths"])
	assert.NotContains(t, w.Body.String(), "sentinel-body")
}

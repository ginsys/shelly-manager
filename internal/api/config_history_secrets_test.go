package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// The history answer carries the stored configuration of a device, so it follows the rules of
// GET /devices/{id}/config: a saved secret is never in the answer, only the mask.

const (
	histOldSecret = "h1st-old-s3cret"
	histNewSecret = "h1st-new-s3cret"
	histRemoved   = "h1st-removed-s3cret"
)

func historyRequest(t *testing.T, h *Handler, deviceID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%s/config/history", deviceID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": deviceID})
	w := httptest.NewRecorder()
	h.GetConfigHistory(w, req)
	return w
}

// historyRows decodes the answer's data member as rows with the three config fields kept as raw
// JSON, so a test can read them as the client does.
func historyRows(t *testing.T, w *httptest.ResponseRecorder) []map[string]json.RawMessage {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var env struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	return env.Data
}

func seedHistory(t *testing.T, h *Handler, deviceID uint, row configuration.ConfigHistory) {
	t.Helper()
	row.DeviceID = deviceID
	row.ConfigID = 1
	require.NoError(t, h.DB.GetDB().Create(&row).Error)
}

func TestGetConfigHistory_MasksSavedSecrets(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"mqtt":{"user":"iot","pass":"`+histNewSecret+`"}}`)
	devID, err := strconv.ParseUint(id, 10, 32)
	require.NoError(t, err)

	changes := `[` +
		`{"path":"mqtt.pass","expected":"` + histOldSecret + `","actual":"` + histNewSecret + `","type":"modified"},` +
		`{"path":"mqtt.user","expected":"iot","actual":"iot2","type":"modified"},` +
		`{"path":"auth","expected":{"user":"admin","password":"` + histRemoved + `"},"actual":null,"type":"removed"}]`
	seedHistory(t, h, uint(devID), configuration.ConfigHistory{
		Action:    "manual",
		OldConfig: json.RawMessage(`{"mqtt":{"user":"iot","pass":"` + histOldSecret + `"}}`),
		NewConfig: json.RawMessage(`{"mqtt":{"user":"iot2","pass":"` + histNewSecret + `"}}`),
		Changes:   json.RawMessage(changes),
		ChangedBy: "user",
	})

	w := historyRequest(t, h, id)
	body := w.Body.String()
	for _, s := range []string{histOldSecret, histNewSecret, histRemoved} {
		assert.NotContains(t, body, s, "the history answer returns a saved secret in clear")
	}

	// The mask replaces the secret and nothing else: the user names and the paths stay readable.
	var seeded map[string]json.RawMessage
	for _, row := range historyRows(t, w) {
		if string(row["action"]) == `"manual"` {
			seeded = row
		}
	}
	require.NotNil(t, seeded, "the seeded row is in the answer")
	assert.JSONEq(t, `{"mqtt":{"user":"iot","pass":"********"}}`, string(seeded["old_config"]))
	assert.JSONEq(t, `{"mqtt":{"user":"iot2","pass":"********"}}`, string(seeded["new_config"]))
	assert.JSONEq(t, `[`+
		`{"path":"mqtt.pass","expected":"********","actual":"********","type":"modified"},`+
		`{"path":"mqtt.user","expected":"iot","actual":"iot2","type":"modified"},`+
		`{"path":"auth","expected":{"user":"admin","password":"********"},"actual":null,"type":"removed"}]`,
		string(seeded["changes"]))
}

// Masking is a property of the answer only: the stored rows keep the secrets, so a later
// compare or export still reads them.
func TestGetConfigHistory_LeavesStoredRowsIntact(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"mqtt":{"pass":"`+histNewSecret+`"}}`)
	devID, err := strconv.ParseUint(id, 10, 32)
	require.NoError(t, err)
	seedHistory(t, h, uint(devID), configuration.ConfigHistory{
		Action:    "manual",
		NewConfig: json.RawMessage(`{"mqtt":{"pass":"` + histNewSecret + `"}}`),
		Changes:   json.RawMessage(`[{"path":"mqtt.pass","expected":"a","actual":"` + histNewSecret + `","type":"modified"}]`),
	})

	historyRequest(t, h, id)

	var stored []configuration.ConfigHistory
	require.NoError(t, h.DB.GetDB().Where("device_id = ?", devID).Find(&stored).Error)
	require.NotEmpty(t, stored)
	for _, row := range stored {
		if row.Action != "manual" {
			continue
		}
		assert.Contains(t, string(row.NewConfig), histNewSecret)
		assert.Contains(t, string(row.Changes), histNewSecret)
	}
}

// A text that is not JSON cannot be searched for a secret, so it is masked whole, as
// maskJSONText does for a stored configuration.
func TestGetConfigHistory_MasksATextThatIsNotJSON(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"mqtt":{"user":"iot"}}`)
	devID, err := strconv.ParseUint(id, 10, 32)
	require.NoError(t, err)
	seedHistory(t, h, uint(devID), configuration.ConfigHistory{
		Action:    "manual",
		NewConfig: json.RawMessage(`pass=` + histNewSecret),
		Changes:   json.RawMessage(`pass=` + histNewSecret),
	})

	w := historyRequest(t, h, id)
	assert.NotContains(t, w.Body.String(), histNewSecret)
}

// maskChanges reads a difference the way the configuration is read: by every key on its path, and
// anything that is not a list of difference objects is masked whole.
func TestMaskChanges(t *testing.T) {
	const m = `"********"`
	cases := []struct {
		name, in, want string
	}{
		{"secret leaf", `[{"path":"mqtt.pass","expected":"a","actual":"b"}]`,
			`[{"path":"mqtt.pass","expected":"********","actual":"********"}]`},
		{"secret ancestor, plain leaf", `[{"path":"api_key.value","expected":"old-secret","actual":"new-secret","type":"modified"}]`,
			`[{"path":"api_key.value","expected":"********","actual":"********","type":"modified"}]`},
		{"secret list entry", `[{"path":"relays[0].pass","expected":"a","actual":"b"}]`,
			`[{"path":"relays[0].pass","expected":"********","actual":"********"}]`},
		{"secret under a list key", `[{"path":"tokens[2]","expected":"a","actual":"b"}]`,
			`[{"path":"tokens[2]","expected":"********","actual":"********"}]`},
		{"section that holds a secret", `[{"path":"mqtt","expected":{"user":"u","pass":"p"},"actual":null}]`,
			`[{"path":"mqtt","expected":{"user":"u","pass":"********"},"actual":null}]`},
		{"plain values stay", `[{"path":"mqtt.user","expected":"a","actual":"b"}]`,
			`[{"path":"mqtt.user","expected":"a","actual":"b"}]`},
		{"a side that is absent", `[{"path":"mqtt.pass","actual":"b"}]`,
			`[{"path":"mqtt.pass","actual":"********"}]`},
		{"no path", `[{"expected":{"pass":"p"},"actual":"x"}]`, m},
		{"an empty path", `[{"path":"","expected":{"pass":"p"},"actual":"x"}]`,
			`[{"path":"","expected":{"pass":"********"},"actual":"x"}]`},
		{"a key with literal brackets", `[{"path":"mqtt.credentials[password]","expected":"a","actual":"s3cr3t-secret"}]`,
			`[{"path":"mqtt.credentials[password]","expected":"********","actual":"********"}]`},
		{"a secret field outside expected and actual", `[{"path":"mqtt.user","expected":"a","actual":"b","password":"secret"}]`,
			`[{"path":"mqtt.user","expected":"a","actual":"b","password":"********"}]`},
		{"an object that is no difference", `[{"password":"secret"}]`, m},
		{"a path that is not text", `[{"path":7,"expected":"a","actual":"secret"}]`, m},
		{"an item that is not an object", `[{"path":"mqtt.pass","actual":"secret"},null]`, m},
		{"an object instead of a list", `{"path":"mqtt.pass","actual":"secret"}`, m},
		{"a string instead of a list", `"secret"`, m},
		{"not JSON", `pass=secret`, m},
		{"the JSON null", `null`, `null`},
		{"an empty list", `[]`, `[]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := maskChanges(json.RawMessage(c.in))
			assert.JSONEq(t, c.want, string(got))
			assert.NotContains(t, string(got), "secret", "a secret survived")
		})
	}
	assert.Empty(t, maskChanges(nil))
}

// The empty and the missing fields keep their meaning: a row written by the "updated" path has
// no old config and no changes, and the answer does not invent either.
func TestGetConfigHistory_KeepsEmptyFieldsEmpty(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"mqtt":{"user":"iot"}}`)

	rows := historyRows(t, historyRequest(t, h, id))
	require.NotEmpty(t, rows)
	for _, row := range rows {
		assert.Contains(t, []string{"", "null", `""`}, string(row["old_config"]), "no old config is invented")
		assert.Contains(t, []string{"", "null", `""`}, string(row["changes"]), "no changes are invented")
	}
}

// A read that fails for any reason other than "no import" stays a server failure: the empty
// state of the page must not hide a broken store.
func TestGetDeviceConfig_OtherFailureStaysA500(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"mqtt":{"user":"iot"}}`)
	require.NoError(t, h.DB.GetDB().Migrator().DropTable(&configuration.DeviceConfig{}))

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%s/config", id), nil)
	req = mux.SetURLVars(req, map[string]string{"id": id})
	w := httptest.NewRecorder()
	h.GetDeviceConfig(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

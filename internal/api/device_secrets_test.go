package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// These tests pin that no device route returns a saved secret. The secrets sit inside JSON text
// (Overrides, DesiredConfig and Settings are strings that hold a document), so a scan of the
// response body finds them only as escaped text: the tests look for the value, never for a key.

const (
	secWiFi   = "s3cr3t-wifi"
	secAP     = "s3cr3t-ap"
	secAPKey  = "s3cr3t-apkey"
	secMQTT   = "s3cr3t-mqtt"
	secAuth   = "s3cr3t-auth"
	secSTA    = "s3cr3t-sta"
	secDesire = "s3cr3t-desired"
	secFlat   = "s3cr3t-flat"
	secLegacy = "s3cr3t-legacy"
)

var allDeviceSecrets = []string{secWiFi, secAP, secAPKey, secMQTT, secAuth, secSTA, secDesire, secFlat, secLegacy}

const secretOverrides = `{"wifi":{"password":"` + secWiFi + `","ap":{"pass":"` + secAP + `","key":"` + secAPKey + `"}},` +
	`"mqtt":{"pass":"` + secMQTT + `","server":"mqtt.example.net"},"auth":{"pass":"` + secAuth + `","user":"admin"},` +
	`"network":{"wifi":{"sta":{"pass":"` + secSTA + `"}}},"future":{"unknown_setting":"keep-me"}}`

const secretDesired = `{"mqtt":{"pass":"` + secDesire + `","server":"mqtt.example.net"}}`

const secretSettings = `{"model":"SHPLG-S","gen":1,"auth_enabled":true,"auth_user":"admin","auth_pass":"` + secFlat + `",` +
	`"auth":{"username":"admin","password":"` + secLegacy + `"}}`

func secretDevice() *database.Device {
	d := testutil.TestDevice()
	d.Settings = secretSettings
	d.Overrides = secretOverrides
	d.DesiredConfig = secretDesired
	return d
}

func assertNoDeviceSecret(t *testing.T, where, body string) {
	t.Helper()
	for _, s := range allDeviceSecrets {
		assert.NotContains(t, body, s, "%s returns the saved secret %s in clear", where, s)
	}
}

// dataOf returns the "data" member of a success envelope.
func dataOf(t *testing.T, body []byte) json.RawMessage {
	t.Helper()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env), string(body))
	return env.Data
}

func serve(h http.HandlerFunc, method, target, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if id != "" {
		req = mux.SetURLVars(req, map[string]string{"id": id})
	}
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func idString(d *database.Device) string { return strconv.FormatUint(uint64(d.ID), 10) }

func TestDeviceRoutes_ReadsReturnNoSavedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	device := secretDevice()
	require.NoError(t, handler.DB.AddDevice(device))

	list := serve(handler.GetDevices, "GET", "/api/v1/devices", "", "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	assertNoDeviceSecret(t, "list devices", list.Body.String())

	get := serve(handler.GetDevice, "GET", "/api/v1/devices/1", idString(device), "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	body := get.Body.String()
	assertNoDeviceSecret(t, "get device", body)

	// A secret that is set shows as the mask, and the fields around it stay.
	assert.Contains(t, body, secretMask)
	assert.Contains(t, body, "mqtt.example.net")
	assert.Contains(t, body, "keep-me", "a key the typed configuration does not know must survive the mask")
	assert.Contains(t, body, `admin`)
}

func TestDeviceRoutes_WritesAnswerWithNoSavedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	body, err := json.Marshal(secretDevice())
	require.NoError(t, err)

	add := serve(handler.AddDevice, "POST", "/api/v1/devices", "", string(body))
	require.Equal(t, http.StatusCreated, add.Code, add.Body.String())
	assertNoDeviceSecret(t, "add device", add.Body.String())

	var created database.Device
	require.NoError(t, json.Unmarshal(dataOf(t, add.Body.Bytes()), &created))
	stored, err := handler.DB.GetDevice(created.ID)
	require.NoError(t, err)
	assert.Contains(t, stored.Overrides, secMQTT, "the secret must still be stored")
	assert.Contains(t, stored.Settings, secFlat)

	update := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", idString(&created), string(body))
	require.Equal(t, http.StatusOK, update.Code, update.Body.String())
	assertNoDeviceSecret(t, "update device", update.Body.String())
}

// A client that reads a device and saves it again sends the mask back. It keeps every stored
// secret, stores the mask nowhere, and a secret sent as a new value replaces the stored one.
func TestDeviceRoutes_UpdateWithTheMaskKeepsStoredSecrets(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	device := secretDevice()
	require.NoError(t, handler.DB.AddDevice(device))
	id := idString(device)

	get := serve(handler.GetDevice, "GET", "/api/v1/devices/1", id, "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	var read map[string]any
	require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))
	read["name"] = "Renamed"
	back, err := json.Marshal(read)
	require.NoError(t, err)

	put := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", id, string(back))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assertNoDeviceSecret(t, "update device", put.Body.String())

	stored, err := handler.DB.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", stored.Name)
	for _, text := range []string{stored.Overrides, stored.DesiredConfig, stored.Settings} {
		assert.NotContains(t, text, secretMask, "the mask was stored as a value")
	}
	for _, s := range []string{secWiFi, secAP, secAPKey, secMQTT, secAuth, secSTA} {
		assert.Contains(t, stored.Overrides, s, "a PUT that sent the mask lost the stored secret %s", s)
	}
	assert.Contains(t, stored.DesiredConfig, secDesire)
	assert.Contains(t, stored.Settings, secFlat)
	assert.Contains(t, stored.Settings, secLegacy)
	assert.Contains(t, stored.Overrides, "keep-me")

	// The first PUT moved updated_at. The next PUT sends the new version that the answer carries.
	var saved map[string]any
	require.NoError(t, json.Unmarshal(dataOf(t, put.Body.Bytes()), &saved))
	read["updated_at"] = saved["updated_at"]

	// A new value is a new secret: it is stored, and the answer masks it.
	read["overrides"] = strings.Replace(secretOverrides, secMQTT, "brand-new-mqtt", 1)
	fresh, err := json.Marshal(read)
	require.NoError(t, err)
	put = serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", id, string(fresh))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.NotContains(t, put.Body.String(), "brand-new-mqtt")
	stored, err = handler.DB.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Contains(t, stored.Overrides, "brand-new-mqtt")
	assert.NotContains(t, stored.Overrides, secMQTT)
}

// A mask in a new device has nothing to restore from: it is stored as no secret, never as the mask.
func TestDeviceRoutes_AddWithTheMaskStoresNoMask(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	d := testutil.TestDevice()
	d.Overrides = `{"mqtt":{"pass":"` + secretMask + `","server":"mqtt.example.net"}}`
	d.Settings = `{"model":"SHPLG-S","gen":1,"auth_enabled":true,"auth_pass":"` + secretMask + `"}`
	body, err := json.Marshal(d)
	require.NoError(t, err)

	add := serve(handler.AddDevice, "POST", "/api/v1/devices", "", string(body))
	require.Equal(t, http.StatusCreated, add.Code, add.Body.String())

	var created database.Device
	require.NoError(t, json.Unmarshal(dataOf(t, add.Body.Bytes()), &created))
	stored, err := handler.DB.GetDevice(created.ID)
	require.NoError(t, err)
	assert.NotContains(t, stored.Overrides, secretMask)
	assert.NotContains(t, stored.Settings, secretMask)
	assert.Contains(t, stored.Overrides, "mqtt.example.net")
}

// Text that is not valid JSON cannot be walked. It must not leak: it is replaced by the mask whole,
// whether or not it names a secret key.
func TestDeviceRoutes_MalformedJSONTextDoesNotLeak(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	leaky := testutil.TestDevice()
	leaky.Overrides = `{"mqtt":{"pass":"` + secMQTT + `"` // truncated
	require.NoError(t, handler.DB.AddDevice(leaky))

	quiet := testutil.TestDevice()
	quiet.IP, quiet.MAC = "192.168.1.101", "A4:CF:12:34:56:79"
	quiet.Overrides = `{"name": "no secret here"` // truncated, no secret key
	require.NoError(t, handler.DB.AddDevice(quiet))

	got := serve(handler.GetDevice, "GET", "/api/v1/devices/1", idString(leaky), "")
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())
	assert.NotContains(t, got.Body.String(), secMQTT, "malformed text leaked a secret")

	got = serve(handler.GetDevice, "GET", "/api/v1/devices/2", idString(quiet), "")
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())
	assert.NotContains(t, got.Body.String(), "no secret here", "a malformed text is masked whole")
	assert.Contains(t, got.Body.String(), `"overrides":"`+secretMask+`"`)
}

// The live reads return what the device reports, in the device's own shape (Gen1 here), and the
// stored baseline that drift compares with must keep the real values.
func TestDeviceRoutes_LiveConfigReadsReturnNoSecret(t *testing.T) {
	// The live reads need a service that can reach a device, which setupTestHandler does not build.
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := NewHandlerWithLogger(db, testShellyService(t, db), testNotificationHandler(t, db), nil, logging.GetDefault())

	const (
		liveSTA   = "s3cr3t-live-sta"
		liveAP    = "s3cr3t-live-ap"
		liveLogin = "s3cr3t-live-login"
		liveMQTT  = "s3cr3t-live-mqtt"
	)
	deviceMux := http.NewServeMux()
	deviceMux.HandleFunc("/shelly", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"type":"SHSW-25","mac":"68C63A123456","auth":false,"fw":"1.14.0"}`)
	})
	deviceMux.HandleFunc("/settings", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{
			"device": {"type": "SHSW-25", "mac": "68C63A123456", "hostname": "shelly1-68C63A123456"},
			"wifi_sta": {"enabled": true, "ssid": "TestNetwork", "key": %q},
			"wifi_ap": {"enabled": false, "ssid": "shelly1", "key": %q},
			"login": {"enabled": true, "username": "admin", "password": %q},
			"mqtt": {"enable": true, "server": "mqtt.example.net", "pass": %q},
			"name": "Live Name", "fw": "1.14.0"
		}`, liveSTA, liveAP, liveLogin, liveMQTT)
	})
	srv := httptest.NewServer(deviceMux)
	defer srv.Close()

	device := testutil.TestDevice()
	device.IP = strings.TrimPrefix(srv.URL, "http://")
	device.MAC = "68C63A123456"
	device.Type = "SHSW-25"
	device.Settings = `{"model":"SHSW-25","gen":1,"auth_enabled":false}`
	require.NoError(t, handler.DB.AddDevice(device))
	id := idString(device)

	live := []string{liveSTA, liveAP, liveLogin, liveMQTT}
	check := func(name string, w *httptest.ResponseRecorder) {
		t.Helper()
		require.Equal(t, http.StatusOK, w.Code, "%s: %s", name, w.Body.String())
		for _, s := range live {
			assert.NotContains(t, w.Body.String(), s, "%s returns the device secret %s in clear", name, s)
		}
		assert.Contains(t, w.Body.String(), "TestNetwork", "%s lost a field that is not a secret", name)
	}

	check("current config", serve(handler.GetCurrentDeviceConfig, "GET", "/api/v1/devices/1/config/current", id, ""))
	check("import config", serve(handler.ImportDeviceConfig, "POST", "/api/v1/devices/1/config/import", id, ""))
	check("stored config", serve(handler.GetDeviceConfig, "GET", "/api/v1/devices/1/config", id, ""))

	norm := serve(handler.GetCurrentDeviceConfigNormalized, "GET", "/api/v1/devices/1/config/current/normalized", id, "")
	require.Equal(t, http.StatusOK, norm.Code, norm.Body.String())
	for _, s := range live {
		assert.NotContains(t, norm.Body.String(), s, "normalized config returns the device secret %s", s)
	}

	// The mask is for the response only: the stored record keeps the real values.
	stored, err := handler.Service.GetDeviceConfig(device.ID)
	require.NoError(t, err)
	for _, s := range live {
		assert.Contains(t, string(stored.Config), s, "masking changed the stored configuration")
	}
}

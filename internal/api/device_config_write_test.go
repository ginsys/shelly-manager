package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A client that reads GET /devices/{id}/config and sends the config member back to
// PUT /devices/{id}/config sends the mask. The stored credentials must survive that round trip,
// and the answer to the PUT must not return them either.
func TestDeviceRoutes_ConfigPutKeepsStoredSecretsAndAnswersMasked(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := NewHandlerWithLogger(db, testShellyService(t, db), testNotificationHandler(t, db), nil, logging.GetDefault())

	const (
		liveSTA   = "s3cr3t-put-sta"
		liveLogin = "s3cr3t-put-login"
		liveMQTT  = "s3cr3t-put-mqtt"
		newMQTT   = "s3cr3t-put-new-mqtt"
	)
	deviceMux := http.NewServeMux()
	deviceMux.HandleFunc("/shelly", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"type":"SHSW-25","mac":"68C63A123456","auth":false,"fw":"1.14.0"}`)
	})
	deviceMux.HandleFunc("/settings", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{
			"device": {"type": "SHSW-25", "mac": "68C63A123456", "hostname": "shelly1-68C63A123456"},
			"wifi_sta": {"enabled": true, "ssid": "TestNetwork", "key": %q},
			"login": {"enabled": true, "username": "admin", "password": %q},
			"mqtt": {"enable": true, "server": "mqtt.example.net", "pass": %q},
			"name": "Live Name", "fw": "1.14.0"
		}`, liveSTA, liveLogin, liveMQTT)
	})
	srv := httptest.NewServer(deviceMux)
	defer srv.Close()

	device := testutil.TestDevice()
	device.IP = strings.TrimPrefix(srv.URL, "http://")
	device.MAC = "68C63A123456"
	device.Type = "SHSW-25"
	device.Settings = `{"model":"SHSW-25","gen":1,"auth_enabled":false}`
	require.NoError(t, db.AddDevice(device))
	id := idString(device)

	// Seed the stored configuration from the device.
	seed := serve(handler.ImportDeviceConfig, "POST", "/api/v1/devices/1/config/import", id, "")
	require.Equal(t, http.StatusOK, seed.Code, seed.Body.String())

	// Read it, edit an unrelated setting, send the config member back.
	read := serve(handler.GetDeviceConfig, "GET", "/api/v1/devices/1/config", id, "")
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	var shown struct {
		Config map[string]any `json:"config"`
	}
	require.NoError(t, json.Unmarshal(dataOf(t, read.Body.Bytes()), &shown))
	require.NotEmpty(t, shown.Config)
	shown.Config["name"] = "Renamed"
	body, err := json.Marshal(shown.Config)
	require.NoError(t, err)
	require.Contains(t, string(body), secretMask, "the read must have masked something for this test to mean anything")

	put := serve(handler.UpdateDeviceConfig, "PUT", "/api/v1/devices/1/config", id, string(body))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	for _, s := range []string{liveSTA, liveLogin, liveMQTT} {
		assert.NotContains(t, put.Body.String(), s, "the answer to the PUT returns the saved secret %s", s)
	}

	stored, err := handler.Service.GetDeviceConfig(device.ID)
	require.NoError(t, err)
	for _, s := range []string{liveSTA, liveLogin, liveMQTT} {
		assert.Contains(t, string(stored.Config), s, "the PUT overwrote the stored secret %s", s)
	}
	assert.NotContains(t, string(stored.Config), secretMask, "the mask was stored")
	assert.Contains(t, string(stored.Config), "Renamed", "the unrelated edit was lost")

	// A secret sent as a new value replaces the stored one.
	shown.Config["mqtt"] = map[string]any{"enable": true, "server": "mqtt.example.net", "pass": newMQTT}
	body, err = json.Marshal(shown.Config)
	require.NoError(t, err)
	put = serve(handler.UpdateDeviceConfig, "PUT", "/api/v1/devices/1/config", id, string(body))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.NotContains(t, put.Body.String(), newMQTT, "the answer to the PUT returns the new secret")
	stored, err = handler.Service.GetDeviceConfig(device.ID)
	require.NoError(t, err)
	assert.Contains(t, string(stored.Config), newMQTT)
	assert.NotContains(t, string(stored.Config), liveMQTT)
	assert.Contains(t, string(stored.Config), liveSTA, "the secrets the client left as the mask must stay")
}

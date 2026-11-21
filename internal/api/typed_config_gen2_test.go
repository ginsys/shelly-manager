package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The typed read of a second-generation import (ginsys/shelly-manager#789). The input is a real-shaped
// Shelly.GetConfig document of a Plus 1PM. Its sections get their typed homes, the raw part holds the
// whole document, and a warning names each top-level key that nothing is read from.

func gen2Fixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "typed_config", "plus1pm.config.json"))
	require.NoError(t, err)
	return data
}

// typedSections marshals the typed answer to a map, so each section can be compared as JSON.
func typedSections(t *testing.T, typed any) map[string]json.RawMessage {
	t.Helper()
	data, err := json.Marshal(typed)
	require.NoError(t, err)
	var sections map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &sections))
	return sections
}

// rawKeyWarning returns the keys the "stored in raw field" warning names, sorted.
func rawKeyWarning(t *testing.T, info *ConfigurationConversionInfo) []string {
	t.Helper()
	const prefix = "Unconverted settings stored in raw field: "
	for _, w := range info.Warnings {
		if rest, ok := strings.CutPrefix(w, prefix); ok {
			keys := strings.Split(rest, ", ")
			sort.Strings(keys)
			return keys
		}
	}
	return nil
}

var plus1PMSections = map[string]string{
	"wifi": `{"enable":true,"ssid":"HomeNet","ipv4mode":"dhcp","roam_threshold":-80,
		"ap":{"enable":true,"ssid":"ShellyPlus1PM-AABBCC"}}`,
	"mqtt": `{"enable":true,"server":"broker.example.net","port":1883,"user":"shelly","id":"shellyplus1pm-AABBCC",
		"topic_prefix":"shellyplus1pm-AABBCC","rpc_ntf":true,"status_ntf":false}`,
	"system":   `{"device":{"name":"Porch","eco_mode":false,"discoverable":true},"sntp":{"server":"time.cloudflare.com"}}`,
	"location": `{"tz":"Europe/Brussels","lat":50.85,"lng":4.35}`,
	"cloud":    `{"enable":true,"server":"iot.shelly.cloud:6012/jrpc"}`,
	"relay":    `{"relays":[{"id":0,"name":"Pump","default_state":"restore_last","auto_on":0,"auto_off":300}]}`,
	"input":    `{"inputs":[{"id":0,"type":"switch","mode":"follow","inverted":false}]}`,
}

// The top-level keys of the Plus 1PM document that nothing is read from, sorted.
var plus1PMUnread = []string{"ble", "modbus", "script:1", "ws"}

func TestConvertToTypedConfig_ByGeneration(t *testing.T) {
	h := &Handler{}
	doc := gen2Fixture(t)

	// The document decides, whatever the device row says: the firmware of a Gen2 device starts with 1.,
	// the manual-add default for gen is 1, and the convert route builds a row with neither.
	rows := map[string]*database.Device{
		"gen 2 row, firmware 1.4.4": {Type: "SNSW-001P16EU", Firmware: "1.4.4", Settings: `{"model":"SNSW-001P16EU","gen":2}`},
		"no gen and firmware 1.4.4": {Type: "SNSW-001P16EU", Firmware: "1.4.4", Settings: `{}`},
		"gen 1 row":                 {Type: "SNSW-001P16EU", Firmware: "1.4.4", Settings: `{"model":"SNSW-001P16EU","gen":1}`},
		"no settings, no firmware":  {},
		"gen 3 row":                 {Type: "S3SW-001P16EU", Firmware: "1.2.0", Settings: `{"model":"S3SW-001P16EU","gen":3}`},
	}
	for name, device := range rows {
		t.Run(name, func(t *testing.T) {
			typed, info, err := h.convertToTypedConfig(doc, device)
			require.NoError(t, err)

			sections := typedSections(t, typed)
			for section, want := range plus1PMSections {
				assert.JSONEq(t, want, string(sections[section]), "section %s", section)
			}

			assert.Equal(t, plus1PMUnread, rawKeyWarning(t, info))
			assert.JSONEq(t, string(doc), string(typed.Raw), "the raw part is the whole document")
			assert.Equal(t, "raw", info.ConvertedFrom)
			assert.True(t, info.HasTypedConfig)
		})
	}
}

// The warning names top-level keys only: a key that is read is not named, however many of its
// sub-keys the typed form has no home for (sys.cfg_rev, mqtt.ssl_ca). They are in the raw part.
func TestConvertToTypedConfig_Gen2WarnsOnTopLevelKeysOnly(t *testing.T) {
	typed, info, err := (&Handler{}).convertToTypedConfig(gen2Fixture(t), &database.Device{})
	require.NoError(t, err)
	for _, key := range rawKeyWarning(t, info) {
		assert.NotContains(t, key, ".")
	}
	assert.Contains(t, string(typed.Raw), `"cfg_rev"`)
	assert.Contains(t, string(typed.Raw), `"ssl_ca"`)
}

func TestConvertToTypedConfig_Gen2TwoChannelsAndNulls(t *testing.T) {
	h := &Handler{}

	t.Run("two switches and two inputs", func(t *testing.T) {
		var doc map[string]any
		require.NoError(t, json.Unmarshal(gen2Fixture(t), &doc))
		second := map[string]any{"id": 1, "name": "Lamp", "in_mode": "flip", "initial_state": "on",
			"auto_on": true, "auto_on_delay": 45, "auto_off": false, "auto_off_delay": 60, "input_id": 1}
		doc["switch:1"] = second
		doc["input:1"] = map[string]any{"id": 1, "name": "Wall", "type": "button", "enable": true, "invert": true}
		in, err := json.Marshal(doc)
		require.NoError(t, err)

		typed, _, err := h.convertToTypedConfig(in, &database.Device{})
		require.NoError(t, err)
		sections := typedSections(t, typed)
		assert.JSONEq(t, `{"relays":[
			{"id":0,"name":"Pump","default_state":"restore_last","auto_on":0,"auto_off":300},
			{"id":1,"name":"Lamp","default_state":"on","auto_on":45,"auto_off":0}]}`, string(sections["relay"]))
		assert.JSONEq(t, `{"inputs":[
			{"id":0,"type":"switch","mode":"follow","inverted":false},
			{"id":1,"name":"Wall","type":"button","mode":"flip","inverted":true}]}`, string(sections["input"]))
	})

	t.Run("null settings are unset", func(t *testing.T) {
		in := `{"sys":{"device":{"name":null},"location":{"tz":null}},"switch:0":{"name":null,"initial_state":null},` +
			`"wifi":{"sta":{"ssid":null},"ap":{"ssid":null}},"mqtt":{"server":null},"cloud":{"server":null}}`
		typed, info, err := h.convertToTypedConfig(json.RawMessage(in), &database.Device{})
		require.NoError(t, err)
		require.NotNil(t, typed.Relay)
		require.Len(t, typed.Relay.Relays, 1)
		assert.Nil(t, typed.Relay.Relays[0].Name)
		assert.Nil(t, typed.Relay.Relays[0].DefaultState)
		assert.Nil(t, typed.System)
		assert.Nil(t, typed.Location)
		assert.Nil(t, typed.WiFi)
		assert.Equal(t, "raw", info.ConvertedFrom)
	})

	t.Run("a document with an empty sys object is still a Gen2 document", func(t *testing.T) {
		typed, info, err := h.convertToTypedConfig(json.RawMessage(`{"sys":{},"switch:0":{"name":"x"}}`), &database.Device{})
		require.NoError(t, err)
		require.NotNil(t, typed.Relay)
		assert.Equal(t, "x", *typed.Relay.Relays[0].Name)
		assert.Empty(t, rawKeyWarning(t, info))
	})
}

const (
	gen2STA    = "gen2-sta-pass"
	gen2STA1   = "gen2-sta1-pass"
	gen2Modbus = "gen2-modbus-pass"
)

// A secret in a Gen2 import has no typed slot. It is read masked, inside the raw part, and a typed
// save keeps it and stores no mask.
func TestTypedConfigRoutes_Gen2SecretsAreMaskedAndSurviveASave(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	device := testutil.TestDevice()
	device.Type, device.Firmware = "SNSW-001P16EU", "1.4.4"
	device.Settings = `{"model":"SNSW-001P16EU","gen":2,"auth_enabled":false}`
	require.NoError(t, h.DB.AddDevice(device))
	require.NoError(t, h.Service.ConfigSvc.UpdateDeviceConfigFromJSON(device.ID, json.RawMessage(gen2Fixture(t))))
	id := idString(device)

	get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	body := get.Body.String()
	for _, secret := range []string{gen2STA, gen2STA1, gen2Modbus} {
		assert.NotContains(t, body, secret, "the read returns the saved secret %s", secret)
	}

	var read struct {
		Configuration  map[string]json.RawMessage  `json:"configuration"`
		ConversionInfo ConfigurationConversionInfo `json:"conversion_info"`
	}
	require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))
	assert.NotContains(t, string(read.Configuration["wifi"]), `"password"`, "no typed Wi-Fi slot is set")
	assert.NotContains(t, string(read.Configuration["wifi"]), `"pass"`)
	var raw struct {
		WiFi struct {
			STA struct {
				Pass string `json:"pass"`
			} `json:"sta"`
		} `json:"wifi"`
		Modbus struct {
			Pass string `json:"pass"`
		} `json:"modbus"`
	}
	require.NoError(t, json.Unmarshal(read.Configuration["raw"], &raw))
	assert.Equal(t, secretMask, raw.WiFi.STA.Pass)
	assert.Equal(t, secretMask, raw.Modbus.Pass)
	for _, w := range read.ConversionInfo.Warnings {
		for _, secret := range []string{gen2STA, gen2STA1, gen2Modbus} {
			assert.NotContains(t, w, secret, "a warning names paths and never values")
		}
	}
	assert.Contains(t, strings.Join(read.ConversionInfo.Warnings, " "), "modbus")

	// Read, then save the read back, twice: the first save replaces the imported document with the typed
	// one, the second reads that typed document as the stored one.
	for round := 1; round <= 2; round++ {
		putBody, err := json.Marshal(map[string]any{"configuration": read.Configuration})
		require.NoError(t, err)
		put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, string(putBody))
		require.Equal(t, http.StatusOK, put.Code, "save %d: %s", round, put.Body.String())

		stored := storedConfig(t, h, id)
		for _, secret := range []string{gen2STA, gen2STA1, gen2Modbus} {
			assert.Contains(t, stored, secret, "save %d must keep the stored secret %s", round, secret)
		}
		assert.NotContains(t, stored, secretMask, "save %d must never store the mask", round)
		for _, key := range []string{"power_limit", "cfg_rev", "ssl_ca", "script:1", `"ws"`, `"ble"`} {
			assert.Contains(t, stored, key, "save %d loses the setting %s", round, key)
		}

		// The read after the save still masks the raw part.
		again := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
		require.Equal(t, http.StatusOK, again.Code, again.Body.String())
		for _, secret := range []string{gen2STA, gen2STA1, gen2Modbus} {
			assert.NotContains(t, again.Body.String(), secret, "the read after save %d returns %s", round, secret)
		}
		require.NoError(t, json.Unmarshal(dataOf(t, again.Body.Bytes()), &read))
		require.NoError(t, json.Unmarshal(read.Configuration["raw"], &raw))
		assert.Equal(t, secretMask, raw.WiFi.STA.Pass, "round %d", round)
	}
}

// A typed field the form clears stays cleared: the raw part is a copy of the import and never wins over
// the typed value, and the secrets in it still survive the save.
func TestTypedConfigRoutes_Gen2ClearedTypedFieldStaysCleared(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	device := testutil.TestDevice()
	device.Type, device.Firmware = "SNSW-001P16EU", "1.4.4"
	device.Settings = `{"model":"SNSW-001P16EU","gen":2,"auth_enabled":false}`
	require.NoError(t, h.DB.AddDevice(device))
	require.NoError(t, h.Service.ConfigSvc.UpdateDeviceConfigFromJSON(device.ID, json.RawMessage(gen2Fixture(t))))
	id := idString(device)

	get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	var read struct {
		Configuration map[string]json.RawMessage `json:"configuration"`
	}
	require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))
	read.Configuration["cloud"] = json.RawMessage(`{"enable":false}`)
	putBody, err := json.Marshal(map[string]any{"configuration": read.Configuration})
	require.NoError(t, err)
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, string(putBody))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())

	after := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
	require.Equal(t, http.StatusOK, after.Code, after.Body.String())
	require.NoError(t, json.Unmarshal(dataOf(t, after.Body.Bytes()), &read))
	assert.JSONEq(t, `{"enable":false}`, string(read.Configuration["cloud"]))
	assert.Contains(t, storedConfig(t, h, id), gen2Modbus)
}

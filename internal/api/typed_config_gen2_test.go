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
// Shelly.GetConfig document of a Plus 1PM. Its sections get their typed homes, and every setting the
// typed model has no home for is named in a warning and kept in the raw part.

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

// Every setting of the Plus 1PM document that has no typed home. A top-level key that nothing in it is
// read from is named as the key.
var plus1PMUnread = []string{
	"ble", "ws", "script:1", "modbus",
	"mqtt.ssl_ca", "mqtt.use_client_cert", "mqtt.enable_control",
	"wifi.ap.is_open", "wifi.ap.range_extender.enable", "wifi.sta.is_open", "wifi.sta.pass", "wifi.roam.interval",
	"wifi.sta1.ssid", "wifi.sta1.pass", "wifi.sta1.is_open", "wifi.sta1.enable", "wifi.sta1.ipv4mode",
	"wifi.sta1.ip", "wifi.sta1.netmask", "wifi.sta1.gw", "wifi.sta1.nameserver",
	"sys.cfg_rev", "sys.device.fw_id", "sys.device.mac",
	"sys.debug.level", "sys.debug.file_level", "sys.debug.mqtt.enable", "sys.debug.websocket.enable", "sys.debug.udp.addr",
	"sys.rpc_udp.dst_addr", "sys.rpc_udp.listen_port",
	"switch:0.id", "switch:0.input_id", "switch:0.auto_on_delay", "switch:0.autorecover_voltage_errors", "switch:0.power_limit",
	"switch:0.voltage_limit", "switch:0.undervoltage_limit", "switch:0.current_limit",
	"input:0.id", "input:0.enable", "input:0.factory_reset",
}

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

			want := append([]string(nil), plus1PMUnread...)
			sort.Strings(want)
			assert.Equal(t, want, rawKeyWarning(t, info))
			assert.Equal(t, "raw", info.ConvertedFrom)
			assert.True(t, info.HasTypedConfig)
		})
	}
}

// A setting the typed model reads, at its zero value, is typed and is not also kept in the raw part:
// the two would disagree after the typed value is edited.
func TestConvertToTypedConfig_Gen2ZeroValueIsNotKeptTwice(t *testing.T) {
	h := &Handler{}
	typed, info, err := h.convertToTypedConfig(gen2Fixture(t), &database.Device{})
	require.NoError(t, err)

	// sys.device.eco_mode, mqtt.status_ntf, input:0.invert and switch:0.auto_on are all false in the
	// fixture, and each is read.
	for _, key := range []string{"eco_mode", "status_ntf", "invert", "auto_on"} {
		assert.NotContains(t, string(typed.Raw), `"`+key+`":`)
	}
	warned := rawKeyWarning(t, info)
	for _, path := range []string{"sys.device.eco_mode", "mqtt.status_ntf", "input:0.invert", "switch:0.auto_on"} {
		assert.NotContains(t, warned, path)
	}
	assert.Contains(t, string(typed.Raw), `"ws"`, "a component nothing reads stays")
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
	assert.Contains(t, strings.Join(read.ConversionInfo.Warnings, " "), "wifi.sta.pass")

	// Read, then save the read back unchanged.
	putBody, err := json.Marshal(map[string]any{"configuration": read.Configuration})
	require.NoError(t, err)
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, string(putBody))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())

	stored := storedConfig(t, h, id)
	for _, secret := range []string{gen2STA, gen2STA1, gen2Modbus} {
		assert.Contains(t, stored, secret, "a saved read must keep the stored secret %s", secret)
	}
	assert.NotContains(t, stored, secretMask, "the mask must never be stored")
	for _, key := range []string{"power_limit", "voltage_limit", "current_limit", "script:1", `"ws"`} {
		assert.Contains(t, stored, key, "a typed save loses the unread setting %s", key)
	}
}

// leafPaths lists the dotted path of every leaf of a decoded document. An empty object is no leaf.
func leafPaths(prefix string, v any, out *[]string) {
	if m, ok := v.(map[string]any); ok {
		for k, child := range m {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			leafPaths(p, child, out)
		}
		return
	}
	*out = append(*out, prefix)
}

// Every setting of the document is either read into a typed field or kept in the raw part. The settings
// that count as read are listed, so a reader that starts or stops reading one changes this test, and a
// setting can never fall between the two.
func TestConvertToTypedConfig_Gen2EverySettingIsReadOrKept(t *testing.T) {
	var doc map[string]any
	require.NoError(t, json.Unmarshal(gen2Fixture(t), &doc))
	var all []string
	leafPaths("", doc, &all)

	kept := map[string]bool{}
	for _, p := range plus1PMUnread {
		kept[p] = true
	}
	var read []string
	for _, p := range all {
		covered := false
		for k := range kept {
			if p == k || strings.HasPrefix(p, k+".") {
				covered = true
			}
		}
		if !covered {
			read = append(read, p)
		}
	}
	sort.Strings(read)
	want := []string{
		"cloud.enable", "cloud.server",
		"input:0.invert", "input:0.name", "input:0.type",
		"mqtt.client_id", "mqtt.enable", "mqtt.server", "mqtt.status_ntf", "mqtt.rpc_ntf", "mqtt.topic_prefix", "mqtt.user",
		"switch:0.auto_off", "switch:0.auto_off_delay", "switch:0.auto_on", "switch:0.in_mode", "switch:0.initial_state", "switch:0.name",
		"sys.device.discoverable", "sys.device.eco_mode", "sys.device.name",
		"sys.location.lat", "sys.location.lon", "sys.location.tz", "sys.sntp.server",
		"wifi.ap.enable", "wifi.ap.ssid", "wifi.roam.rssi_thr", "wifi.sta.enable", "wifi.sta.ip", "wifi.sta.ipv4mode",
		"wifi.sta.gw", "wifi.sta.netmask", "wifi.sta.nameserver", "wifi.sta.ssid",
	}
	sort.Strings(want)
	assert.Equal(t, want, read)
}

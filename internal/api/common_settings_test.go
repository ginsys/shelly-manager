package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The common-settings route compares the stored imports of the devices a request names and answers the
// settings that are equal on all of them. These tests pin what it must never answer: a saved secret (a
// Wi-Fi password equal on every device above all), a value that is one device's own, and a device that
// has no import counted as agreeing (ginsys/shelly-manager#747 slice 3).

const (
	comWiFi   = "c0mm-wifi-s3cret"
	comAP     = "c0mm-ap-s3cret"
	comAPSlot = "c0mm-apslot-s3cret"
	comMQTT   = "c0mm-mqtt-s3cret"
	comAuth   = "c0mm-auth-s3cret"
	comSTA    = "c0mm-sta-s3cret"
	comRaw    = "c0mm-raw-s3cret"
)

var commonSecrets = []string{comWiFi, comAP, comAPSlot, comMQTT, comAuth, comSTA, comRaw}

// commonSite is the configuration of a device at one site, in the typed shape, with every typed secret
// set to the same value on every device. name and mqttID are the device's own.
func commonSite(name, mqttID string) string {
	return `{"wifi":{"enable":true,"ssid":"site-net","password":"` + comWiFi + `",` +
		`"ap":{"enable":false,"ssid":"ap-` + name + `","pass":"` + comAP + `","key":"` + comAPSlot + `"}},` +
		`"mqtt":{"enable":true,"server":"mqtt.example.net:1883","pass":"` + comMQTT + `","id":"` + mqttID + `"},` +
		`"auth":{"enable":true,"user":"admin","pass":"` + comAuth + `"},` +
		`"network":{"wifi":{"sta":{"ssid":"site-net","pass":"` + comSTA + `"}}},` +
		`"system":{"device":{"name":"` + name + `","mac":"AA:BB:CC:00:00:` + name + `"}},` +
		`"relay":{"relays":[{"id":0,"name":"` + name + ` relay","default_state":"off"}]},` +
		`"raw":{"future":{"api_token":"` + comRaw + `","label":"same"}}}`
}

// commonDevice adds a device of the given model with its own address and stores config as its import. An
// empty config adds a device with no import.
func commonDevice(t *testing.T, h *Handler, n int, model, config string) uint {
	t.Helper()
	device := testutil.TestDevice()
	device.MAC = fmt.Sprintf("A4:CF:12:34:56:%02X", n)
	device.IP = fmt.Sprintf("127.0.0.1:%d", 1000+n)
	device.Name = fmt.Sprintf("dev-%d", n)
	device.Settings = `{"model":"` + model + `","gen":1}`
	require.NoError(t, h.DB.AddDevice(device))
	if config != "" {
		require.NoError(t, h.Service.ConfigSvc.UpdateDeviceConfigFromJSON(device.ID, json.RawMessage(config)))
	}
	return device.ID
}

type commonAnswer struct {
	Compared []uint `json:"compared"`
	Skipped  []struct {
		DeviceID   uint   `json:"device_id"`
		DeviceName string `json:"device_name"`
		Reason     string `json:"reason"`
	} `json:"skipped"`
	DeviceType string         `json:"device_type"`
	Common     map[string]any `json:"common"`
	Differing  map[string]int `json:"differing"`
}

func askCommon(h *Handler, body string) *httptest.ResponseRecorder {
	return serve(h.CommonSettings, "POST", "/api/v1/config/common-settings", "", body)
}

// commonFor asks for the common settings of the devices and decodes the answer.
func commonFor(t *testing.T, h *Handler, ids ...uint) (commonAnswer, string) {
	t.Helper()
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	w := askCommon(h, `{"device_ids":[`+strings.Join(parts, ",")+`]}`)
	body := w.Body.String()
	require.Equal(t, http.StatusOK, w.Code, body)
	var answer commonAnswer
	require.NoError(t, json.Unmarshal(dataOf(t, []byte(body)), &answer), body)
	return answer, body
}

func commonAt(m map[string]any, path ...string) (any, bool) {
	var cur any = m
	for _, key := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = next[key]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// The trap: a Wi-Fi password, an MQTT password or a login password that is the same on every device is
// "common", and must never come back, nor the mask that stands for it.
func TestCommonSettings_NeverReturnsASecretEqualOnEveryDevice(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	ids := []uint{
		commonDevice(t, h, 1, "SHPLG-S", commonSite("01", "shelly-01")),
		commonDevice(t, h, 2, "SHPLG-S", commonSite("02", "shelly-02")),
		commonDevice(t, h, 3, "SHPLG-S", commonSite("03", "shelly-03")),
	}

	answer, body := commonFor(t, h, ids...)

	for _, secret := range commonSecrets {
		assert.NotContains(t, body, secret, "the answer returns the saved secret %s", secret)
	}
	assert.NotContains(t, body, "********", "the answer returns the mask of a secret")
	// Control: the answer is the real one, and what is not secret and is equal does come back.
	server, ok := commonAt(answer.Common, "mqtt", "server")
	require.True(t, ok, body)
	assert.Equal(t, "mqtt.example.net:1883", server)
	ssid, ok := commonAt(answer.Common, "wifi", "ssid")
	require.True(t, ok, body)
	assert.Equal(t, "site-net", ssid)
}

// A secret slot that is set to an empty value on every device is stripped too: the template would clear
// the password of every device it is applied to.
func TestCommonSettings_StripsAnEmptySecretSlotEqualEverywhere(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	config := `{"mqtt":{"enable":true,"server":"mqtt.example.net:1883","pass":""},"auth":{"enable":false,"pass":""}}`
	a := commonDevice(t, h, 1, "SHPLG-S", config)
	b := commonDevice(t, h, 2, "SHPLG-S", config)

	answer, body := commonFor(t, h, a, b)

	_, hasPass := commonAt(answer.Common, "mqtt", "pass")
	assert.False(t, hasPass, body)
	_, hasAuthPass := commonAt(answer.Common, "auth", "pass")
	assert.False(t, hasAuthPass, body)
	_, hasServer := commonAt(answer.Common, "mqtt", "server")
	assert.True(t, hasServer, body)
}

func TestCommonSettings_AnswersEqualValuesOnly(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	a := commonDevice(t, h, 1, "SHPLG-S", `{"mqtt":{"enable":true,"server":"one.example.net"},"wifi":{"ssid":"net"},"cloud":{"enable":false}}`)
	b := commonDevice(t, h, 2, "SHPLG-S", `{"mqtt":{"enable":true,"server":"two.example.net"},"wifi":{"ssid":"net"}}`)
	c := commonDevice(t, h, 3, "SHPLG-S", `{"mqtt":{"enable":true,"server":"one.example.net"},"wifi":{"ssid":"net"},"cloud":{"enable":false}}`)

	answer, body := commonFor(t, h, a, b, c)

	enable, ok := commonAt(answer.Common, "mqtt", "enable")
	assert.True(t, ok && enable == true, body)
	_, hasServer := commonAt(answer.Common, "mqtt", "server")
	assert.False(t, hasServer, "a value that differs is not common: %s", body)
	_, hasCloud := commonAt(answer.Common, "cloud")
	assert.False(t, hasCloud, "a section one device lacks is not common: %s", body)
	ssid, _ := commonAt(answer.Common, "wifi", "ssid")
	assert.Equal(t, "net", ssid, body)
	// A section that is partly common says how much of it differs.
	assert.Equal(t, 1, answer.Differing["mqtt"], body)
	assert.Equal(t, 1, answer.Differing["cloud"], body)
}

func TestCommonSettings_StripsWhatIsOneDevicesOwn(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	same := `{"system":{"device":{"name":"same","hostname":"same-host","mac":"AA:BB:CC:00:00:01","eco_mode":true}},` +
		`"mqtt":{"enable":true,"id":"same-id","topic_prefix":"same/prefix"},` +
		`"auth":{"enable":true,"realm":"same-realm"},` +
		`"wifi":{"enable":true,"ipv4mode":"static","static_ip":{"ip":"192.0.2.9"}},` +
		`"relay":{"relays":[{"id":0,"name":"same relay","default_state":"off"}]}}`
	a := commonDevice(t, h, 1, "SHPLG-S", same)
	b := commonDevice(t, h, 2, "SHPLG-S", same)

	answer, body := commonFor(t, h, a, b)

	for _, path := range [][]string{
		{"system", "device", "name"}, {"system", "device", "hostname"}, {"system", "device", "mac"},
		{"mqtt", "id"}, {"mqtt", "topic_prefix"}, {"auth", "realm"},
		{"wifi", "ipv4mode"}, {"wifi", "static_ip"},
	} {
		_, found := commonAt(answer.Common, path...)
		assert.False(t, found, "%s is one device's own: %s", strings.Join(path, "."), body)
	}
	assert.NotContains(t, body, "same relay", body)
	// Control: what is not one device's own and is equal stays.
	eco, ok := commonAt(answer.Common, "system", "device", "eco_mode")
	assert.True(t, ok && eco == true, body)
}

func TestCommonSettings_NeverReturnsASecretInTheRawPart(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	config := `{"mqtt":{"enable":true},"raw":{"future":{"api_token":"` + comRaw + `","label":"same"}}}`
	a := commonDevice(t, h, 1, "SHPLG-S", config)
	b := commonDevice(t, h, 2, "SHPLG-S", config)

	answer, body := commonFor(t, h, a, b)

	assert.NotContains(t, body, comRaw)
	_, hasRaw := commonAt(answer.Common, "raw")
	assert.False(t, hasRaw, "the raw part is never common: %s", body)
}

func TestCommonSettings_NamesTheDevicesItSkipped(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	config := `{"mqtt":{"enable":true,"server":"mqtt.example.net"}}`
	a := commonDevice(t, h, 1, "SHPLG-S", config)
	b := commonDevice(t, h, 2, "SHPLG-S", config)
	noImport := commonDevice(t, h, 3, "SHPLG-S", "")
	const unknown = uint(99999)

	answer, body := commonFor(t, h, a, noImport, unknown, b)

	assert.Equal(t, []uint{a, b}, answer.Compared, body)
	require.Len(t, answer.Skipped, 2, body)
	assert.Equal(t, noImport, answer.Skipped[0].DeviceID)
	assert.Equal(t, "dev-3", answer.Skipped[0].DeviceName)
	assert.Equal(t, "no stored import", answer.Skipped[0].Reason)
	assert.Equal(t, unknown, answer.Skipped[1].DeviceID)
	assert.Equal(t, "device not found", answer.Skipped[1].Reason)
	// The device with no import does not shrink the agreement of the others.
	server, _ := commonAt(answer.Common, "mqtt", "server")
	assert.Equal(t, "mqtt.example.net", server, body)
}

func TestCommonSettings_PairsListElementsByPositionWhenTheLengthsAgree(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	a := commonDevice(t, h, 1, "SHSW-25", `{"relay":{"relays":[{"id":0,"name":"a","default_state":"off","auto_on":5},{"id":1,"name":"b","default_state":"on"}]}}`)
	b := commonDevice(t, h, 2, "SHSW-25", `{"relay":{"relays":[{"id":0,"name":"c","default_state":"off","auto_on":9},{"id":1,"name":"d","default_state":"on"}]}}`)

	answer, body := commonFor(t, h, a, b)

	relays, ok := commonAt(answer.Common, "relay", "relays")
	require.True(t, ok, body)
	list := relays.([]any)
	require.Len(t, list, 2, body)
	first := list[0].(map[string]any)
	// The element keeps its id when it is equal on every device, and loses the value that differs.
	assert.EqualValues(t, 0, first["id"], body)
	assert.Equal(t, "off", first["default_state"], body)
	_, hasAutoOn := first["auto_on"]
	assert.False(t, hasAutoOn, body)
	assert.EqualValues(t, 1, list[1].(map[string]any)["id"], body)
}

func TestCommonSettings_OmitsAListWhoseLengthDiffers(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	a := commonDevice(t, h, 1, "SHSW-25", `{"relay":{"auto_on":5,"relays":[{"id":0,"default_state":"off"}]}}`)
	b := commonDevice(t, h, 2, "SHSW-25", `{"relay":{"auto_on":5,"relays":[{"id":0,"default_state":"off"},{"id":1,"default_state":"off"}]}}`)

	answer, body := commonFor(t, h, a, b)

	_, hasRelays := commonAt(answer.Common, "relay", "relays")
	assert.False(t, hasRelays, body)
	autoOn, ok := commonAt(answer.Common, "relay", "auto_on")
	assert.True(t, ok && autoOn == float64(5), body)
}

func TestCommonSettings_NamesTheModelOnlyWhenAllDevicesShareIt(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	config := `{"mqtt":{"enable":true}}`
	a := commonDevice(t, h, 1, "SHPLG-S", config)
	b := commonDevice(t, h, 2, "SHPLG-S", config)
	c := commonDevice(t, h, 3, "SHSW-25", config)

	same, _ := commonFor(t, h, a, b)
	mixed, _ := commonFor(t, h, a, b, c)

	assert.Equal(t, "SHPLG-S", same.DeviceType)
	assert.Equal(t, "", mixed.DeviceType)
}

func TestCommonSettings_RefusesABadRequest(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	tooMany := make([]string, maxCommonSettingsDevices+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprint(i + 1)
	}
	for name, body := range map[string]string{
		"no ids":        `{"device_ids":[]}`,
		"no body":       ``,
		"unknown field": `{"device_ids":[1],"model":"SHPLG-S"}`,
		"not json":      `device_ids`,
		"too many":      `{"device_ids":[` + strings.Join(tooMany, ",") + `]}`,
	} {
		w := askCommon(h, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", name, w.Body.String())
	}
}

// The UI drops the same per-device values before it builds a template, from its own copy of the list.
func TestUIPerDevicePathsAreTheServerList(t *testing.T) {
	raw, err := os.ReadFile("../../ui/src/utils/perDevicePaths.json")
	require.NoError(t, err, "read the UI's per-device path list")
	var ui []string
	require.NoError(t, json.Unmarshal(raw, &ui))
	sort.Strings(ui)
	want := append([]string(nil), perDevicePaths...)
	sort.Strings(want)
	if !reflect.DeepEqual(ui, want) {
		t.Errorf("ui/src/utils/perDevicePaths.json = %v, the server's per-device paths are %v", ui, want)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// These tests pin that the typed configuration routes never return a saved secret, and that a
// client that reads a configuration and saves it back keeps every stored secret and stores no mask.

const (
	typWiFi  = "t7pe-wifi-s3cret"
	typAP    = "t7pe-ap-s3cret"
	typAPKey = "t7pe-apkey-s3cret"
	typMQTT  = "t7pe-mqtt-s3cret"
	typAuth  = "t7pe-auth-s3cret"
	typSTA   = "t7pe-sta-s3cret"
	typRaw   = "t7pe-raw-s3cret"
	typGen1  = "t7pe-gen1-s3cret"
)

var allTypedSecrets = []string{typWiFi, typAP, typAPKey, typMQTT, typAuth, typSTA, typRaw, typGen1}

// typedStored is a configuration already in the typed shape, with every typed secret set and one
// secret inside the raw part, which holds the settings the typed model does not know.
const typedStored = `{"wifi":{"enable":true,"ssid":"home","password":"` + typWiFi + `",` +
	`"ap":{"enable":false,"pass":"` + typAP + `","key":"` + typAPKey + `"}},` +
	`"mqtt":{"enable":true,"server":"mqtt.example.net:1883","pass":"` + typMQTT + `"},` +
	`"auth":{"enable":true,"user":"admin","pass":"` + typAuth + `"},` +
	`"network":{"wifi":{"sta":{"pass":"` + typSTA + `"}}},` +
	`"raw":{"future":{"api_token":"` + typRaw + `","label":"keep-me"}}}`

// gen1Stored is a configuration in the device's own shape. The typed routes convert it, and what
// they cannot place lands in the raw part.
const gen1Stored = `{"mqtt":{"enable":true,"server":"mqtt.example.net:1883","pass":"` + typMQTT + `"},` +
	`"login":{"enabled":true,"username":"admin","password":"` + typAuth + `"},` +
	`"vendor_blob":{"api_key":"` + typGen1 + `","label":"keep-me"}}`

// gen1Credentials is a configuration in the device's own shape, whose Wi-Fi and login secrets use
// the device's spelling: "key" for the Wi-Fi passwords and "password" for the login.
const (
	gen1STAPwd   = "t7pe-gen1sta-s3cret"
	gen1APPwd    = "t7pe-gen1ap-s3cret"
	gen1LoginPwd = "t7pe-gen1login-s3cret"
)

const gen1Credentials = `{"wifi_sta":{"enabled":true,"ssid":"home","key":"` + gen1STAPwd + `"},` +
	`"wifi_ap":{"enabled":false,"ssid":"shelly","key":"` + gen1APPwd + `"},` +
	`"login":{"enabled":true,"username":"admin","password":"` + gen1LoginPwd + `"}}`

var gen1CredentialSecrets = []string{gen1STAPwd, gen1APPwd, gen1LoginPwd}

// A configuration in the device's own shape keeps its Wi-Fi key and login password through a typed
// save: the converted view holds them, the read shows the mask, and the save restores them.
func TestTypedConfigRoutes_Gen1CredentialsSurviveASave(t *testing.T) {
	bodies := map[string]func(t *testing.T, h *Handler, id string) string{
		"read then save": func(t *testing.T, h *Handler, id string) string {
			get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
			require.Equal(t, http.StatusOK, get.Code, get.Body.String())
			for _, s := range gen1CredentialSecrets {
				assert.NotContains(t, get.Body.String(), s, "the read returns the saved secret %s", s)
			}
			var read struct {
				Configuration json.RawMessage `json:"configuration"`
			}
			require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))
			return `{"configuration":` + string(read.Configuration) + `}`
		},
		"sections sent without the secrets": func(*testing.T, *Handler, string) string {
			return `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"auth":{"enable":true,"user":"admin"}}}`
		},
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, gen1Credentials)

			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body(t, h, id))
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())

			stored := storedConfig(t, h, id)
			assert.Contains(t, stored, gen1STAPwd, "the Wi-Fi key is lost by a typed save")
			assert.Contains(t, stored, gen1LoginPwd, "the login password is lost by a typed save")
			assert.NotContains(t, stored, secretMask, "the mask must never be stored")
		})
	}
}

// The typed name wins where a configuration has both spellings, even when its value is null: the
// device spelling then never takes its place, in the typed read or in a save.
func TestConvertToTypedConfig_PassWinsOverTheDeviceSpelling(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	device := testutil.TestDevice()
	require.NoError(t, h.DB.AddDevice(device))

	cases := map[string]struct {
		stored string
		wifi   *string
		auth   *string
	}{
		"device spelling alone": {
			`{"wifi_sta":{"key":"dev-key"},"login":{"password":"dev-pwd"}}`, strPtr("dev-key"), strPtr("dev-pwd")},
		"both spellings: pass wins": {
			`{"wifi_sta":{"pass":"typed-key","key":"dev-key"},"login":{"pass":"typed-pwd","password":"dev-pwd"}}`, strPtr("typed-key"), strPtr("typed-pwd")},
		"pass null: no fallback": {
			`{"wifi_sta":{"pass":null,"key":"dev-key"},"login":{"pass":null,"password":"dev-pwd"}}`, nil, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			device.Settings = "{}"
			typed, _, err := h.convertToTypedConfig(json.RawMessage(c.stored), device)
			require.NoError(t, err)
			require.NotNil(t, typed.WiFi)
			require.NotNil(t, typed.Auth)
			assert.Equal(t, c.wifi, typed.WiFi.Password)
			assert.Equal(t, c.auth, typed.Auth.Password)
		})
	}
}

func strPtr(s string) *string { return &s }

func typedHandler(t *testing.T) (*Handler, func()) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	logger := logging.GetDefault()
	return &Handler{
		DB:            db,
		Service:       testShellyService(t, db),
		logger:        logger,
		ConfigService: configuration.NewService(db.GetDB(), logger),
	}, cleanup
}

// typedDevice adds a device and stores stored as its configuration.
func typedDevice(t *testing.T, h *Handler, stored string) string {
	t.Helper()
	device := testutil.TestDevice()
	require.NoError(t, h.DB.AddDevice(device))
	require.NoError(t, h.Service.ConfigSvc.UpdateDeviceConfigFromJSON(device.ID, json.RawMessage(stored)))
	return idString(device)
}

func assertNoTypedSecret(t *testing.T, where, body string) {
	t.Helper()
	for _, s := range allTypedSecrets {
		assert.NotContains(t, body, s, "%s returns the saved secret %s in clear", where, s)
	}
}

// storedConfig returns the stored configuration text of a device.
func storedConfig(t *testing.T, h *Handler, id string) string {
	t.Helper()
	var n uint
	for _, c := range id {
		n = n*10 + uint(c-'0')
	}
	cfg, err := h.Service.GetDeviceConfig(n)
	require.NoError(t, err)
	return string(cfg.Config)
}

func TestTypedConfigRoutes_ReadsReturnNoSavedSecret(t *testing.T) {
	for name, stored := range map[string]string{"typed": typedStored, "device shape": gen1Stored} {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, stored)

			get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
			require.Equal(t, http.StatusOK, get.Code, get.Body.String())
			assertNoTypedSecret(t, "GET config/typed", get.Body.String())
			assert.Contains(t, get.Body.String(), secretMask, "a secret that is set shows as the mask")
			assert.Contains(t, get.Body.String(), "keep-me", "a setting around a secret must survive the mask")

			norm := serve(h.GetTypedDeviceConfigNormalized, "GET", "/api/v1/devices/1/config/typed/normalized", id, "")
			require.Equal(t, http.StatusOK, norm.Code, norm.Body.String())
			assertNoTypedSecret(t, "GET config/typed/normalized", norm.Body.String())
		})
	}
}

// A client reads the typed configuration and saves it back unchanged: every stored secret stays,
// the mask is stored nowhere, and the answer carries no secret either.
func TestTypedConfigRoutes_ReadEditSaveKeepsStoredSecrets(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, typedStored)

	get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	var read struct {
		Configuration json.RawMessage `json:"configuration"`
	}
	require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))

	body := `{"configuration":` + string(read.Configuration) + `}`
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assertNoTypedSecret(t, "PUT config/typed", put.Body.String())

	stored := storedConfig(t, h, id)
	for _, s := range []string{typWiFi, typAP, typAPKey, typMQTT, typAuth, typSTA, typRaw} {
		assert.Contains(t, stored, s, "a saved read must keep the stored secret %s", s)
	}
	assert.NotContains(t, stored, secretMask, "the mask must never be stored")
}

// A secret sent as a new value replaces the stored one, and the others are kept.
func TestTypedConfigRoutes_NewSecretReplacesStoredOne(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, typedStored)

	get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	var read struct {
		Configuration map[string]any `json:"configuration"`
	}
	require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))
	read.Configuration["mqtt"].(map[string]any)["pass"] = "n3w-mqtt-pass"

	payload, err := json.Marshal(map[string]any{"configuration": read.Configuration})
	require.NoError(t, err)
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, string(payload))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.NotContains(t, put.Body.String(), "n3w-mqtt-pass", "the answer must not echo the new secret")

	stored := storedConfig(t, h, id)
	assert.Contains(t, stored, "n3w-mqtt-pass")
	assert.NotContains(t, stored, typMQTT)
	assert.Contains(t, stored, typAuth)
	assert.NotContains(t, stored, secretMask)
}

// The save rules are those of the overrides routes: a secret the body leaves out keeps its stored
// value, an explicit "" or a null removes it, and the other secrets stay.
func TestTypedConfigRoutes_OmittedKeepsEmptyAndNullRemove(t *testing.T) {
	cases := map[string]struct {
		mqtt string
		kept bool
	}{
		"omitted keeps": {`{"enable":true,"server":"mqtt.example.net:1883"}`, true},
		"empty clears":  {`{"enable":true,"server":"mqtt.example.net:1883","pass":""}`, false},
		"null removes":  {`{"enable":true,"server":"mqtt.example.net:1883","pass":null}`, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, typedStored)

			body := `{"configuration":{"mqtt":` + c.mqtt + `,"auth":{"enable":true,"user":"admin","pass":"` + secretMask + `"}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())

			stored := storedConfig(t, h, id)
			assert.Equal(t, c.kept, strings.Contains(stored, typMQTT), "stored configuration: %s", stored)
			assert.Contains(t, stored, typAuth, "the mask keeps the auth secret")
			assert.NotContains(t, stored, secretMask)
		})
	}
}

// The network passwords follow the same rules, since the Network fields are pointers: omitted keeps,
// "" clears and null removes.
func TestTypedConfigRoutes_NetworkPasswordOmittedKeepsEmptyAndNullRemove(t *testing.T) {
	cases := map[string]struct {
		sta  string
		kept bool
	}{
		"omitted keeps": {`{"ssid":"office"}`, true},
		"empty clears":  {`{"ssid":"office","pass":""}`, false},
		"null removes":  {`{"ssid":"office","pass":null}`, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, typedStored)

			body := `{"configuration":{"network":{"wifi":{"sta":` + c.sta + `}}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())

			stored := storedConfig(t, h, id)
			assert.Equal(t, c.kept, strings.Contains(stored, typSTA), "stored configuration: %s", stored)
			assert.NotContains(t, stored, secretMask)
		})
	}
}

// A raw secret the body leaves out keeps its stored value, whether the raw part is edited or
// omitted, and one sent as "" or null is removed.
func TestTypedConfigRoutes_RawSecretOmittedKeepsEmptyAndNullRemove(t *testing.T) {
	cases := map[string]struct {
		raw  string
		kept bool
	}{
		"edited, secret left out": {`"raw":{"future":{"label":"edited"}},`, true},
		"raw omitted":             {``, true},
		"empty clears":            {`"raw":{"future":{"api_token":"","label":"edited"}},`, false},
		"null removes":            {`"raw":{"future":{"api_token":null,"label":"edited"}},`, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, typedStored)

			body := `{"configuration":{` + c.raw + `"wifi":{"enable":true,"ssid":"home"}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			assertNoTypedSecret(t, "PUT config/typed", put.Body.String())
			assert.Equal(t, c.kept, strings.Contains(storedConfig(t, h, id), typRaw), "stored configuration: %s", storedConfig(t, h, id))
		})
	}
}

// The validator reads the real configuration and a message can quote a value from it, so the
// validation part of an answer is scrubbed too.
func TestTypedConfigRoutes_ValidationMessagesCarryNoSavedSecret(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	const tmpl = `{{envOr \"T7PE_UNSET\" \"t7pe-tmpl-s3cret\"}}`
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"},"raw":{"future":{"api_token":"`+tmpl+`"}}}`)

	get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	assert.NotContains(t, get.Body.String(), "t7pe-tmpl-s3cret")
}

// The messages that quote a template fragment are masked after the colon, whatever the fragment
// holds (a token with "&", or a secret that is only part of an expression). Others stay.
func TestScrubValidation(t *testing.T) {
	result := &configuration.ValidationResult{
		Errors: []configuration.ValidationError{
			{Code: "TEMPLATE_SYNTAX_ERROR", Message: "Invalid template syntax: bad a&b here"},
			{Code: "WIFI_PASSWORD_SHORT", Message: "WiFi password must be at least 8 characters"},
		},
		Warnings: []configuration.ValidationWarning{
			{Code: "DANGEROUS_TEMPLATE_FUNCTION", Message: "Template contains potentially dangerous function: {{exec \"x\"}}"},
			{Code: "CUSTOM_VARIABLE_REFERENCE", Message: "Custom variable referenced: t7pe-secret - ensure it's provided"},
		},
		Info: []configuration.ValidationInfo{{Code: "ENV_VARIABLE_REFERENCE", Message: "Template references environment variables: prefix-{{envOr \"T\" \"d\"}}"}},
	}
	scrubValidation(result)
	assert.Equal(t, "Invalid template syntax: "+secretMask, result.Errors[0].Message)
	assert.Equal(t, "WiFi password must be at least 8 characters", result.Errors[1].Message)
	assert.Equal(t, "Template contains potentially dangerous function: "+secretMask, result.Warnings[0].Message)
	assert.NotContains(t, result.Warnings[1].Message, "t7pe-secret")
	assert.Equal(t, "Template references environment variables: "+secretMask, result.Info[0].Message)
}

// An edited raw part that leaves out a key holding an array secret keeps that key.
func TestTypedConfigRoutes_EditedRawKeepsOmittedArraySecrets(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[{"password":"`+typRaw+`"}],"other":"x"}}`)

	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, `{"configuration":{"raw":{}}}`)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	stored := storedConfig(t, h, id)
	assert.Contains(t, stored, typRaw)
	assert.NotContains(t, stored, `"other"`, "a stored key with no secret is not restored")
}

// A masked secret in an array entry returns to its own entry only: a reordered or removed entry
// takes no other entry's password, and an unchanged array keeps every one.
func TestTypedConfigRoutes_ArrayEntriesNeverTakeAnotherEntrysSecret(t *testing.T) {
	const stored = `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` +
		`{"name":"alice","password":"t7pe-alice-s3cret"},{"name":"bob","password":"t7pe-bob-s3cret"}]}}`
	cases := map[string]struct {
		accounts string
		alice    bool
		bob      bool
	}{
		"unchanged":     {`[{"name":"alice","password":"` + secretMask + `"},{"name":"bob","password":"` + secretMask + `"}]`, true, true},
		"swapped":       {`[{"name":"bob","password":"` + secretMask + `"},{"name":"alice","password":"` + secretMask + `"}]`, false, false},
		"first removed": {`[{"name":"bob","password":"` + secretMask + `"}]`, false, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, stored)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":` + c.accounts + `}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			got := storedConfig(t, h, id)
			assert.Equal(t, c.alice, strings.Contains(got, "t7pe-alice-s3cret"), "stored: %s", got)
			assert.Equal(t, c.bob, strings.Contains(got, "t7pe-bob-s3cret"), "stored: %s", got)
			assert.NotContains(t, got, secretMask)
		})
	}
}

// Clearing one secret of an array entry leaves the entry's other masked secret in place.
func TestTypedConfigRoutes_ClearingOneArraySecretKeepsTheOther(t *testing.T) {
	for name, cleared := range map[string]string{"empty": `""`, "null": `null`} {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[`+
				`{"name":"alice","password":"t7pe-alice-s3cret","api_token":"t7pe-token-s3cret"}]}}`)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` +
				`{"name":"alice","password":` + cleared + `,"api_token":"` + secretMask + `"}]}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			got := storedConfig(t, h, id)
			assert.NotContains(t, got, "t7pe-alice-s3cret")
			assert.Contains(t, got, "t7pe-token-s3cret", "stored: %s", got)
		})
	}
}

// An array entry that leaves out its secret keeps it when it is the stored entry at its position
// (compared without secrets), and takes nothing when it is not. An explicit "" removes it.
func TestTypedConfigRoutes_ArrayEntryOmittingItsSecretKeepsIt(t *testing.T) {
	const stored = `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` +
		`{"name":"alice","password":"t7pe-alice-s3cret"},{"name":"bob","password":"t7pe-bob-s3cret"}]}}`
	cases := map[string]struct {
		accounts string
		alice    bool
		bob      bool
	}{
		"both omit":     {`[{"name":"alice"},{"name":"bob"}]`, true, true},
		"swapped":       {`[{"name":"bob"},{"name":"alice"}]`, false, false},
		"first removed": {`[{"name":"bob"}]`, false, false},
		"empty clears":  {`[{"name":"alice","password":""},{"name":"bob"}]`, false, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, stored)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":` + c.accounts + `}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			got := storedConfig(t, h, id)
			assert.Equal(t, c.alice, strings.Contains(got, "t7pe-alice-s3cret"), "stored: %s", got)
			assert.Equal(t, c.bob, strings.Contains(got, "t7pe-bob-s3cret"), "stored: %s", got)
		})
	}
}

// An array entry that matches more than one stored entry, because they differ only in secrets,
// cannot be told apart: a mask or an omitted secret is refused with a 400 and nothing is stored,
// and a new secret value is accepted.
func TestTypedConfigRoutes_AmbiguousArrayEntryIsRefused(t *testing.T) {
	const stored = `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` +
		`{"token":"alice","password":"t7pe-alice-s3cret"},{"token":"bob","password":"t7pe-bob-s3cret"}]}}`
	cases := map[string]struct {
		accounts string
		status   int
	}{
		"omitted":   {`[{"token":"bob"}]`, http.StatusBadRequest},
		"masked":    {`[{"token":"bob","password":"` + secretMask + `"}]`, http.StatusBadRequest},
		"new value": {`[{"token":"bob","password":"t7pe-new-s3cret"}]`, http.StatusOK},
		"cleared":   {`[{"token":"bob","password":""}]`, http.StatusOK},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, stored)

			body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":` + c.accounts + `}}}`
			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
			require.Equal(t, c.status, put.Code, put.Body.String())
			got := storedConfig(t, h, id)
			if c.status == http.StatusBadRequest {
				assert.Contains(t, put.Body.String(), "more than one stored entry")
				assert.NotContains(t, put.Body.String(), "t7pe-")
				assert.Equal(t, stored, got, "a refused PUT stores nothing")
				return
			}
			assert.NotContains(t, got, "t7pe-alice-s3cret")
			assert.NotContains(t, got, secretMask)
		})
	}
}

// A mask that no stored entry holds a secret for needs no choosing, so it is dropped as before and
// the explicit new password is kept, even when the entries look alike.
func TestTypedConfigRoutes_UnmatchedMaskInAmbiguousEntryIsDropped(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[`+
		`{"name":"x","password":"t7pe-a-s3cret"},{"name":"x","password":"t7pe-b-s3cret"}]}}`)

	body := `{"configuration":{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[` +
		`{"name":"x","password":"t7pe-new-s3cret","api_token":"` + secretMask + `"}]}}}`
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	got := storedConfig(t, h, id)
	assert.Contains(t, got, "t7pe-new-s3cret")
	assert.NotContains(t, got, secretMask)
	assert.NotContains(t, got, "api_token")
}

// The text helper reports the same ambiguity, which the device routes share.
func TestRestoreMaskedText_AmbiguousEntryIsAnError(t *testing.T) {
	stored := `{"accounts":[{"token":"alice","password":"A"},{"token":"bob","password":"B"}]}`
	_, err := restoreMaskedText(`{"accounts":[{"token":"bob","password":"`+secretMask+`"}]}`, stored)
	assert.ErrorIs(t, err, errAmbiguousEntry)

	// Identical stored entries cannot swap anything, so they are not ambiguous.
	same := `{"accounts":[{"token":"bob","password":"A"},{"token":"bob","password":"A"}]}`
	got, err := restoreMaskedText(`{"accounts":[{"token":"bob","password":"`+secretMask+`"}]}`, same)
	assert.NoError(t, err)
	assert.Contains(t, got, `"password":"A"`)
}

// A raw part the body omits is the stored one, an array secret included.
func TestTypedConfigRoutes_OmittedRawKeepsArraySecrets(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"},"raw":{"accounts":[{"password":"`+typRaw+`"}]}}`)

	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, `{"configuration":{}}`)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assertNoTypedSecret(t, "PUT config/typed", put.Body.String())
	assert.Contains(t, storedConfig(t, h, id), typRaw)
}

// A mask with nothing stored behind it is dropped, never stored.
func TestTypedConfigRoutes_MaskWithNothingStoredStoresNoMask(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	id := typedDevice(t, h, `{"wifi":{"enable":true,"ssid":"home"}}`)

	body := `{"configuration":{"wifi":{"enable":true,"ssid":"home","password":"` + secretMask + `"}}}`
	put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, body)
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())
	assert.NotContains(t, storedConfig(t, h, id), secretMask)
}

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

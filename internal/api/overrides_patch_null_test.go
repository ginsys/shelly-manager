package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PATCH on the device overrides follows the JSON merge patch rule: null removes the field.
// These tests cover a plain field, a secret, a nested secret, a whole section and the casing.

func TestPatchOverridesNullRemovesAPlainField(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"server":"mqtt.example:1883","user":"iot","pass":"x"}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":{"server":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT)
	assert.Nil(t, stored.MQTT.Server, "null removes the field")
	require.NotNil(t, stored.MQTT.User, "a field the body leaves out stays")
	assert.Equal(t, "iot", *stored.MQTT.User)
	require.NotNil(t, stored.MQTT.Password)
	assert.Equal(t, "x", *stored.MQTT.Password)
}

func TestPatchOverridesNullRemovesASecretSoTheTemplateShowsThrough(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	svc := handler.ConfigService.ConfigurationSvc
	tmplID := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"template-secret"}}`)
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"device-secret"}}`)
	require.NoError(t, svc.SetDeviceTemplates(id, []uint{tmplID}))

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":{"pass":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := svc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT)
	assert.Nil(t, stored.MQTT.Password, "the override no longer pins the secret")

	desired, _, err := svc.GetDesiredConfig(id)
	require.NoError(t, err)
	require.NotNil(t, desired.MQTT.Password)
	assert.Equal(t, "template-secret", *desired.MQTT.Password)
}

func TestPatchOverridesNullRemovesANestedSecret(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"network":{"wifi":{"sta":{"ssid":"home","pass":"device-secret"}}}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"network":{"wifi":{"sta":{"pass":null}}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.Network.WiFi.STA)
	assert.Nil(t, stored.Network.WiFi.STA.Password)
	require.NotNil(t, stored.Network.WiFi.STA.SSID)
	assert.Equal(t, "home", *stored.Network.WiFi.STA.SSID)
}

func TestPatchOverridesNullRemovesAWholeSection(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"},"led":{"power_indication":false}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":null}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.Nil(t, stored.MQTT, "the whole section is removed")
	assert.NotNil(t, stored.LED, "another section stays")
}

func TestPatchOverridesNullIsReadWithoutRegardToCase(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"MQTT":{"PASS":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.Nil(t, stored.MQTT.Password)
}

func TestPatchOverridesNullForAFieldThatIsNotStoredChangesNothing(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot"}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":{"pass":null,"nonsense":null},"auth":{"pass":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT.User)
	assert.Equal(t, "iot", *stored.MQTT.User)
	assert.Nil(t, stored.MQTT.Password)
}

func TestPatchOverridesNullRemovesAFieldInsideAnArrayElement(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"relay":{"relays":[{"id":0,"name":"old"},{"id":1,"name":"keep"}]}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"relay":{"relays":[{"id":0,"name":null}]}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.Len(t, stored.Relay.Relays, 2)
	assert.Nil(t, stored.Relay.Relays[0].Name, "null removes the field of that element")
	require.NotNil(t, stored.Relay.Relays[1].Name)
	assert.Equal(t, "keep", *stored.Relay.Relays[1].Name)
}

// A null for another field must not change a stored number that a float cannot hold.
func TestPatchOverridesNullKeepsAStoredNumberExact(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"relay":{"auto_off":9007199254740993},"mqtt":{"pass":"x"}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":{"pass":null},"unknown":null}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.Relay.AutoOff)
	assert.Equal(t, 9007199254740993, *stored.Relay.AutoOff)
}

// The struct decode ignores a number it cannot hold in an unknown field, so the removal must too.
func TestPatchOverridesNullStillRemovesWhenAnUnknownFieldHoldsAHugeNumber(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"secret"}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"mqtt":{"pass":null},"unknown":1e400}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.Nil(t, stored.MQTT.Password)
}

// A key that appears twice leaves two readings of the body, so the request is refused.
func TestPatchOverridesRefusesADuplicateKeyAndChangesNothing(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"secret"}}`)

	w := sendOverrides(t, handler, "PATCH", id,
		`{"mqtt":{"pass":"********"},"mqtt":{"pass":null},"mqtt":{"user":"iot2"}}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.MQTT.Password)
	assert.Equal(t, "secret", *stored.MQTT.Password)
	assert.Equal(t, "iot", *stored.MQTT.User)
}

// A plain field with no omitempty is written even at its zero value, so null cannot make it
// absent. It returns to the zero value, and a null for the whole section removes it.
func TestPatchOverridesNullOnAPlainFieldReturnsItToTheZeroValue(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"system":{"device":{"ble":{"enable":true,"rpc":{}}}}}`)

	w := sendOverrides(t, handler, "PATCH", id, `{"system":{"device":{"ble":{"enable":null}}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.False(t, stored.System.Device.BleConfig.Enable)

	w = sendOverrides(t, handler, "PATCH", id, `{"system":{"device":{"ble":null}}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	stored, err = handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.Nil(t, stored.System)
}

func TestPatchOverridesNullOnAZeroValuedPlainFieldKeepsItsSection(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"system":{"sleep_mode":{"enable":false}}}`)

	for _, body := range []string{
		`{"system":{"sleep_mode":{"enable":null}}}`,
		`{"system":{"sleep_mode":{"unknown":null}}}`,
	} {
		w := sendOverrides(t, handler, "PATCH", id, body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
		require.NoError(t, err)
		require.NotNil(t, stored.System, body)
		require.NotNil(t, stored.System.Sleep, body)
		assert.False(t, stored.System.Sleep.Enable, body)
	}
}

func TestPatchOverridesKeysAreMatchedLikeTheDecoder(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"wifi":{"password":"secret"},"mqtt":{"id":"client"}}`)

	w := sendOverrides(t, handler, "PATCH", id, "{\"wİfİ\":null}")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = sendOverrides(t, handler, "PATCH", id, "{\"mqtt\":{\"id\":\"client\",\"İd\":1}}")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	require.NotNil(t, stored.WiFi, "a key the decoder does not match removes nothing")
	require.NotNil(t, stored.MQTT.ClientID)
	assert.Equal(t, "client", *stored.MQTT.ClientID)
}

// A secret path takes four values on PATCH, as on PUT: omitted keeps the stored value, "" clears
// it, the mask restores it and null removes it.
func TestPatchOverridesSecretTakesFourValues(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantSet  bool
		wantPass string
	}{
		{"omitted keeps", `{"mqtt":{"user":"iot2"}}`, true, "x"},
		{"empty string clears", `{"mqtt":{"pass":""}}`, true, ""},
		{"mask restores", `{"mqtt":{"pass":"********"}}`, true, "x"},
		{"null removes", `{"mqtt":{"pass":null}}`, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

			w := sendOverrides(t, handler, "PATCH", id, tc.body)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.NotContains(t, w.Body.String(), `"x"`, "a stored secret never leaves the API")

			stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
			require.NoError(t, err)
			require.NotNil(t, stored.MQTT)
			if !tc.wantSet {
				assert.Nil(t, stored.MQTT.Password)
				return
			}
			require.NotNil(t, stored.MQTT.Password)
			assert.Equal(t, tc.wantPass, *stored.MQTT.Password)
		})
	}
}

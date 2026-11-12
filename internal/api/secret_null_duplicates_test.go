package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The decoder and the reading of null must agree on which member of a repeated key counts. They
// cannot, so a configuration body with a repeated key is refused with 400 and nothing is saved.
// Before, the reading kept one entry by map order and the null could be missed, so the old
// password came back.
var repeatedKeyBodies = map[string]string{
	"other case, null last":  `{"mqtt":{"user":"iot","pass":"new","PASS":null}}`,
	"other case, null first": `{"mqtt":{"user":"iot","PASS":null,"pass":"new"}}`,
	"repeated section":       `{"mqtt":{"pass":"a"},"mqtt":{"pass":null}}`,
	"repeated user":          `{"mqtt":{"user":"a","user":"b","pass":null}}`,
}

func TestSetOverridesRefusesARepeatedKeyAndChangesNothing(t *testing.T) {
	for name, body := range repeatedKeyBodies {
		t.Run(name, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

			w := sendOverrides(t, handler, "PUT", id, body)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

			stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
			require.NoError(t, err)
			require.NotNil(t, stored.MQTT)
			require.NotNil(t, stored.MQTT.Password)
			assert.Equal(t, "x", *stored.MQTT.Password)
			assert.Equal(t, "iot", *stored.MQTT.User)
		})
	}
}

func TestUpdateTemplateRefusesARepeatedKeyAndChangesNothing(t *testing.T) {
	for name, body := range repeatedKeyBodies {
		t.Run(name, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			id := newKeepTemplate(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

			w := putKeepTemplate(t, handler, id, `{"config":`+body+`}`)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

			assert.JSONEq(t, `{"mqtt":{"user":"iot","pass":"x"}}`, storedTemplateConfig(t, handler, id))
		})
	}
}

func TestTypedConfigPutRefusesARepeatedKeyAndChangesNothing(t *testing.T) {
	for name, body := range repeatedKeyBodies {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			id := typedDevice(t, h, typedStored)
			before := storedConfig(t, h, id)

			put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, `{"configuration":`+body+`}`)
			require.Equal(t, http.StatusBadRequest, put.Code, put.Body.String())

			assert.Equal(t, before, storedConfig(t, h, id))
		})
	}
}

// The control: the same bodies without the repeat are saved, so the 400 above comes from the repeat.
func TestSetOverridesStillRemovesASecretOnNullWithoutARepeat(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()
	id := keepOverrideDevice(t, handler, `{"mqtt":{"user":"iot","pass":"x"}}`)

	w := sendOverrides(t, handler, "PUT", id, `{"mqtt":{"user":"iot","PASS":null}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	stored, err := handler.ConfigService.ConfigurationSvc.GetDeviceOverrides(id)
	require.NoError(t, err)
	assert.Nil(t, stored.MQTT.Password)
}

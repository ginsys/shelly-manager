package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

func TestRedactDifference_SecretPathIsMasked(t *testing.T) {
	path, expected, actual := redactDifference(configuration.ConfigDifference{
		Path: "wifi.password", Expected: "hunter2", Actual: "old-secret",
	})

	assert.Equal(t, "wifi.password", path)
	assert.Equal(t, redactedValue, expected)
	assert.Equal(t, redactedValue, actual)
}

func TestRedactDifference_PlainValueIsKept(t *testing.T) {
	_, expected, actual := redactDifference(configuration.ConfigDifference{
		Path: "system.device.name", Expected: "tv-office", Actual: "",
	})

	assert.Equal(t, "tv-office", expected)
	assert.Equal(t, "", actual)
}

func TestRedactDifference_WholeSectionIsMaskedKeyByKey(t *testing.T) {
	_, expected, actual := redactDifference(configuration.ConfigDifference{
		Path:     "mqtt",
		Expected: map[string]interface{}{"server": "broker:1883", "pass": "hunter2", "user": "iot"},
		Actual:   &configuration.MQTTConfiguration{Server: configuration.StringPtr("broker"), Password: configuration.StringPtr("old")},
	})

	raw, err := json.Marshal([]any{expected, actual})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "hunter2")
	assert.NotContains(t, string(raw), `"old"`)
	assert.Contains(t, string(raw), "broker:1883")
	assert.Contains(t, string(raw), redactedValue)
}

func TestRedactDifference_NilStaysNil(t *testing.T) {
	_, expected, actual := redactDifference(configuration.ConfigDifference{Path: "wifi.password"})

	assert.Nil(t, expected)
	assert.Nil(t, actual)
}

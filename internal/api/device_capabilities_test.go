package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ginsys/shelly-manager/internal/database"
)

// The capabilities route lists roller for the model the device schema offers a roller section
// for, and for no other: both read configuration.HasRollerMode.
func TestGetDeviceCapabilities_RollerFollowsTheDeviceSchema(t *testing.T) {
	h := &Handler{}

	assert.Contains(t, h.getDeviceCapabilities("SHSW-25", 1), "roller")
	assert.NotContains(t, h.getDeviceCapabilities("SHPLG-S", 1), "roller")
	assert.NotContains(t, h.getDeviceCapabilities("SHSW-1", 1), "roller")
}

// A Gen1 device has MQTT, cloud and login settings (the app already writes the first two), so the
// route lists them for every generation. Bluetooth and Ethernet stay Gen2 and later.
func TestGetDeviceCapabilities_Gen1HasMQTTCloudAndAuth(t *testing.T) {
	h := &Handler{}

	for _, model := range []string{"SHSW-1", "SHPLG-S", "SHDM-1", "SHSW-25"} {
		caps := h.getDeviceCapabilities(model, 1)
		assert.Subset(t, caps, []string{"wifi", "mqtt", "cloud", "auth"}, model)
		assert.NotContains(t, caps, "ble", model)
		assert.NotContains(t, caps, "ethernet", model)
	}

	gen2 := h.getDeviceCapabilities("SHSW-1PM", 2)
	assert.Subset(t, gen2, []string{"wifi", "mqtt", "cloud", "auth", "ble", "ethernet"})
	assert.Equal(t, 1, countOf(gen2, "mqtt"), "a Gen2 device lists mqtt once")
}

// The validator warns "device may not support MQTT" when the capability list lacks mqtt, so a
// Gen1 device with MQTT enabled used to draw that warning.
func TestCreateValidator_Gen1MQTTDrawsNoUnsupportedWarning(t *testing.T) {
	h := &Handler{}
	device := &database.Device{Type: "SHSW-1", Firmware: "20230913-114008/v1.14.0-gcb84623"}

	validator := h.createValidator("basic", device)
	result := validator.ValidateConfiguration(json.RawMessage(`{"mqtt":{"enable":true,"server":"broker.local:1883"}}`))

	for _, w := range result.Warnings {
		assert.NotEqual(t, "MQTT_NOT_SUPPORTED", w.Code)
	}
}

func countOf(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}
	return n
}

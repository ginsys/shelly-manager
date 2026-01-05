package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceConfiguration_JSONMarshaling(t *testing.T) {
	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable: BoolPtr(true),
			SSID:   StringPtr("TestNetwork"),
		},
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("mqtt.example.com"),
			Port:   IntPtr(1883),
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var decoded DeviceConfiguration
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.WiFi)
	assert.NotNil(t, decoded.WiFi.Enable)
	assert.True(t, *decoded.WiFi.Enable)
	assert.NotNil(t, decoded.WiFi.SSID)
	assert.Equal(t, "TestNetwork", *decoded.WiFi.SSID)

	assert.NotNil(t, decoded.MQTT)
	assert.NotNil(t, decoded.MQTT.Enable)
	assert.True(t, *decoded.MQTT.Enable)
	assert.NotNil(t, decoded.MQTT.Server)
	assert.Equal(t, "mqtt.example.com", *decoded.MQTT.Server)
}

func TestDeviceConfiguration_NilFieldsOmitted(t *testing.T) {
	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable: BoolPtr(true),
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	wifi, ok := raw["wifi"].(map[string]interface{})
	require.True(t, ok, "wifi should be present")

	_, hasSSID := wifi["ssid"]
	assert.False(t, hasSSID, "nil SSID should be omitted from JSON")

	_, hasMQTT := raw["mqtt"]
	assert.False(t, hasMQTT, "nil MQTT should be omitted from JSON")
}

func TestDeviceConfiguration_SHPLGS_Example(t *testing.T) {
	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable: BoolPtr(true),
			SSID:   StringPtr("HomeNetwork"),
		},
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("192.168.1.50"),
			Port:   IntPtr(1883),
		},
		Relay: &RelayConfig{
			DefaultState: StringPtr("last"),
			AutoOff:      IntPtr(0),
			Relays: []SingleRelayConfig{
				{
					ID:           0,
					Name:         StringPtr("Plug 1"),
					DefaultState: StringPtr("off"),
				},
			},
		},
		PowerMetering: &PowerMeteringConfig{
			MaxPower: IntPtr(2500),
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var decoded DeviceConfiguration
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.Relay)
	assert.NotNil(t, decoded.Relay.DefaultState)
	assert.Equal(t, "last", *decoded.Relay.DefaultState)

	assert.NotNil(t, decoded.PowerMetering)
	assert.NotNil(t, decoded.PowerMetering.MaxPower)
	assert.Equal(t, 2500, *decoded.PowerMetering.MaxPower)
}

func TestDeviceConfiguration_SHSW1_Example(t *testing.T) {
	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable: BoolPtr(true),
			SSID:   StringPtr("HomeNetwork"),
		},
		Relay: &RelayConfig{
			DefaultState: StringPtr("off"),
			Relays: []SingleRelayConfig{
				{
					ID:   0,
					Name: StringPtr("Switch 1"),
				},
			},
		},
		Input: &InputConfig{
			Inputs: []SingleInputConfig{
				{
					ID:   0,
					Type: StringPtr("button"),
				},
			},
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var decoded DeviceConfiguration
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.Relay)
	assert.Len(t, decoded.Relay.Relays, 1)

	assert.NotNil(t, decoded.Input)
	assert.Len(t, decoded.Input.Inputs, 1)

	assert.Nil(t, decoded.PowerMetering, "SHSW-1 has no power metering")
}

func TestDeviceConfiguration_SHIX31_Example(t *testing.T) {
	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable: BoolPtr(true),
			SSID:   StringPtr("HomeNetwork"),
		},
		Input: &InputConfig{
			Inputs: []SingleInputConfig{
				{ID: 0, Type: StringPtr("button")},
				{ID: 1, Type: StringPtr("button")},
				{ID: 2, Type: StringPtr("button")},
			},
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var decoded DeviceConfiguration
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.Input)
	assert.Len(t, decoded.Input.Inputs, 3)

	assert.Nil(t, decoded.Relay, "SHIX3-1 has no relay")
	assert.Nil(t, decoded.PowerMetering, "SHIX3-1 has no power metering")
}

func TestDeviceConfiguration_TemplateInheritance(t *testing.T) {
	globalTemplate := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			Enable: BoolPtr(true),
			SSID:   StringPtr("GlobalSSID"),
		},
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("mqtt.global.com"),
		},
	}

	deviceOverride := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			SSID: StringPtr("DeviceSSID"),
		},
	}

	assert.NotNil(t, deviceOverride.WiFi)
	assert.NotNil(t, deviceOverride.WiFi.SSID)
	assert.Equal(t, "DeviceSSID", *deviceOverride.WiFi.SSID)
	assert.Nil(t, deviceOverride.WiFi.Enable)

	assert.NotNil(t, globalTemplate.WiFi)
	assert.NotNil(t, globalTemplate.WiFi.Enable)
	assert.True(t, *globalTemplate.WiFi.Enable)
}

func TestCoIoTConfiguration_JSONMarshaling(t *testing.T) {
	config := &DeviceConfiguration{
		CoIoT: &CoIoTConfiguration{
			Enable:       BoolPtr(true),
			UpdatePeriod: IntPtr(30),
			Peer:         StringPtr("224.0.1.187:5683"),
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var decoded DeviceConfiguration
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.NotNil(t, decoded.CoIoT)
	assert.NotNil(t, decoded.CoIoT.Enable)
	assert.True(t, *decoded.CoIoT.Enable)
	assert.NotNil(t, decoded.CoIoT.UpdatePeriod)
	assert.Equal(t, 30, *decoded.CoIoT.UpdatePeriod)
}

func TestDeviceConfiguration_EmptyConfig(t *testing.T) {
	config := &DeviceConfiguration{}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	assert.Equal(t, "{}", string(data), "empty config should marshal to {}")
}

func TestDeviceConfiguration_PartialConfig(t *testing.T) {
	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	_, hasWiFi := raw["wifi"]
	assert.False(t, hasWiFi, "nil WiFi should not be in JSON")

	mqtt, hasMQTT := raw["mqtt"]
	assert.True(t, hasMQTT, "MQTT should be present")

	mqttMap := mqtt.(map[string]interface{})
	_, hasServer := mqttMap["server"]
	assert.False(t, hasServer, "nil Server should not be in JSON")
}

package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeConfigurations_EmptyLayers(t *testing.T) {
	result, err := MergeConfigurations([]ConfigLayer{})
	require.NoError(t, err)
	assert.NotNil(t, result.Config)
	assert.Empty(t, result.Sources)
}

func TestMergeConfigurations_SingleLayer(t *testing.T) {
	layer := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server: StringPtr("broker.local"),
				Port:   IntPtr(1883),
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer})
	require.NoError(t, err)
	assert.NotNil(t, result.Config.MQTT)
	assert.Equal(t, "broker.local", *result.Config.MQTT.Server)
	assert.Equal(t, 1883, *result.Config.MQTT.Port)

	assert.Equal(t, "global", result.Sources["mqtt.server"])
	assert.Equal(t, "global", result.Sources["mqtt.port"])
}

func TestMergeConfigurations_TwoLayersNoOverlap(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server: StringPtr("broker.local"),
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			WiFi: &WiFiConfiguration{
				SSID: StringPtr("MyNetwork"),
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2})
	require.NoError(t, err)

	assert.NotNil(t, result.Config.MQTT)
	assert.Equal(t, "broker.local", *result.Config.MQTT.Server)

	assert.NotNil(t, result.Config.WiFi)
	assert.Equal(t, "MyNetwork", *result.Config.WiFi.SSID)

	assert.Equal(t, "global", result.Sources["mqtt.server"])
	assert.Equal(t, "device", result.Sources["wifi.ssid"])
}

func TestMergeConfigurations_TwoLayersOverlap(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server: StringPtr("old.broker.local"),
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server: StringPtr("new.broker.local"),
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2})
	require.NoError(t, err)

	assert.NotNil(t, result.Config.MQTT)
	assert.Equal(t, "new.broker.local", *result.Config.MQTT.Server)
	assert.Equal(t, "device", result.Sources["mqtt.server"])
}

func TestMergeConfigurations_ThreeLayers(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server: StringPtr("global.broker"),
				Port:   IntPtr(1883),
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "group",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server: StringPtr("group.broker"),
			},
		},
	}

	layer3 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Port: IntPtr(8883),
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2, layer3})
	require.NoError(t, err)

	assert.NotNil(t, result.Config.MQTT)
	assert.Equal(t, "group.broker", *result.Config.MQTT.Server)
	assert.Equal(t, 8883, *result.Config.MQTT.Port)

	assert.Equal(t, "group", result.Sources["mqtt.server"])
	assert.Equal(t, "device", result.Sources["mqtt.port"])
}

func TestMergeConfigurations_NilVsZeroValue(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			WiFi: &WiFiConfiguration{
				Enable: BoolPtr(true),
			},
			MQTT: &MQTTConfiguration{
				Enable: BoolPtr(true),
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Enable: BoolPtr(false),
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2})
	require.NoError(t, err)

	assert.True(t, *result.Config.WiFi.Enable)
	assert.False(t, *result.Config.MQTT.Enable)

	assert.Equal(t, "global", result.Sources["wifi.enable"])
	assert.Equal(t, "device", result.Sources["mqtt.enable"])
}

func TestMergeConfigurations_NestedStructMerge(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server:    StringPtr("broker.local"),
				Port:      IntPtr(1883),
				KeepAlive: IntPtr(60),
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			MQTT: &MQTTConfiguration{
				Server: StringPtr("device.broker"),
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2})
	require.NoError(t, err)

	assert.NotNil(t, result.Config.MQTT)
	assert.Equal(t, "device.broker", *result.Config.MQTT.Server)
	assert.Equal(t, 1883, *result.Config.MQTT.Port)
	assert.Equal(t, 60, *result.Config.MQTT.KeepAlive)

	assert.Equal(t, "device", result.Sources["mqtt.server"])
	assert.Equal(t, "global", result.Sources["mqtt.port"])
	assert.Equal(t, "global", result.Sources["mqtt.keep_alive"])
}

func TestMergeConfigurations_SliceMerge(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			Relay: &RelayConfig{
				Relays: []SingleRelayConfig{
					{
						ID:      0,
						AutoOff: IntPtr(3600),
					},
					{
						ID:      1,
						AutoOff: IntPtr(7200),
					},
				},
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			Relay: &RelayConfig{
				Relays: []SingleRelayConfig{
					{
						ID:   0,
						Name: StringPtr("Kitchen Light"),
					},
				},
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2})
	require.NoError(t, err)

	assert.NotNil(t, result.Config.Relay)
	assert.Len(t, result.Config.Relay.Relays, 2)

	assert.Equal(t, "Kitchen Light", *result.Config.Relay.Relays[0].Name)
	assert.Equal(t, 3600, *result.Config.Relay.Relays[0].AutoOff)
	assert.Equal(t, 7200, *result.Config.Relay.Relays[1].AutoOff)

	assert.Equal(t, "device", result.Sources["relay.relays.0.name"])
	assert.Equal(t, "global", result.Sources["relay.relays.0.auto_off"])
	assert.Equal(t, "global", result.Sources["relay.relays.1.auto_off"])
}

func TestMergeConfigurations_SourceTracking(t *testing.T) {
	layers := []ConfigLayer{
		{
			Name: "global",
			Config: &DeviceConfiguration{
				MQTT: &MQTTConfiguration{
					Server: StringPtr("global.broker"),
					Port:   IntPtr(1883),
				},
				Location: &LocationConfiguration{
					Timezone: StringPtr("UTC"),
				},
			},
		},
		{
			Name: "group",
			Config: &DeviceConfiguration{
				MQTT: &MQTTConfiguration{
					Port: IntPtr(8883),
				},
			},
		},
		{
			Name: "device",
			Config: &DeviceConfiguration{
				Location: &LocationConfiguration{
					Latitude: Float64Ptr(40.7128),
				},
			},
		},
	}

	result, err := MergeConfigurations(layers)
	require.NoError(t, err)

	assert.Equal(t, "global", result.Sources["mqtt.server"])
	assert.Equal(t, "group", result.Sources["mqtt.port"])
	assert.Equal(t, "global", result.Sources["location.tz"])
	assert.Equal(t, "device", result.Sources["location.lat"])
}

func TestMergeConfigurations_DeepNesting(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "global",
		Config: &DeviceConfiguration{
			WiFi: &WiFiConfiguration{
				StaticIP: &StaticIPConfig{
					IP:      StringPtr("192.168.1.100"),
					Netmask: StringPtr("255.255.255.0"),
					Gateway: StringPtr("192.168.1.1"),
				},
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			WiFi: &WiFiConfiguration{
				StaticIP: &StaticIPConfig{
					Gateway: StringPtr("192.168.1.254"),
				},
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2})
	require.NoError(t, err)

	assert.NotNil(t, result.Config.WiFi)
	assert.NotNil(t, result.Config.WiFi.StaticIP)
	assert.Equal(t, "192.168.1.100", *result.Config.WiFi.StaticIP.IP)
	assert.Equal(t, "255.255.255.0", *result.Config.WiFi.StaticIP.Netmask)
	assert.Equal(t, "192.168.1.254", *result.Config.WiFi.StaticIP.Gateway)

	assert.Equal(t, "global", result.Sources["wifi.static_ip.ip"])
	assert.Equal(t, "global", result.Sources["wifi.static_ip.netmask"])
	assert.Equal(t, "device", result.Sources["wifi.static_ip.gw"])
}

func TestMergeConfigurations_NilLayerConfig(t *testing.T) {
	layers := []ConfigLayer{
		{
			Name: "layer1",
			Config: &DeviceConfiguration{
				MQTT: &MQTTConfiguration{
					Server: StringPtr("broker1"),
				},
			},
		},
		{
			Name:   "nil-layer",
			Config: nil,
		},
		{
			Name: "layer2",
			Config: &DeviceConfiguration{
				MQTT: &MQTTConfiguration{
					Port: IntPtr(1883),
				},
			},
		},
	}

	result, err := MergeConfigurations(layers)
	require.NoError(t, err)

	assert.NotNil(t, result.Config.MQTT)
	assert.Equal(t, "broker1", *result.Config.MQTT.Server)
	assert.Equal(t, 1883, *result.Config.MQTT.Port)
}

func TestMergeConfigurations_InputSliceMerge(t *testing.T) {
	layer1 := ConfigLayer{
		Name: "device-type",
		Config: &DeviceConfiguration{
			Input: &InputConfig{
				Type: StringPtr("button"),
				Inputs: []SingleInputConfig{
					{
						ID:   0,
						Type: StringPtr("switch"),
					},
					{
						ID:   1,
						Type: StringPtr("momentary"),
					},
					{
						ID:   2,
						Type: StringPtr("switch"),
					},
				},
			},
		},
	}

	layer2 := ConfigLayer{
		Name: "device",
		Config: &DeviceConfiguration{
			Input: &InputConfig{
				Inputs: []SingleInputConfig{
					{
						ID:   0,
						Name: StringPtr("Door Sensor"),
					},
					{
						ID:   1,
						Name: StringPtr("Motion Sensor"),
					},
				},
			},
		},
	}

	result, err := MergeConfigurations([]ConfigLayer{layer1, layer2})
	require.NoError(t, err)

	assert.NotNil(t, result.Config.Input)
	assert.Equal(t, "button", *result.Config.Input.Type)
	assert.Len(t, result.Config.Input.Inputs, 3)

	assert.Equal(t, "Door Sensor", *result.Config.Input.Inputs[0].Name)
	assert.Equal(t, "switch", *result.Config.Input.Inputs[0].Type)

	assert.Equal(t, "Motion Sensor", *result.Config.Input.Inputs[1].Name)
	assert.Equal(t, "momentary", *result.Config.Input.Inputs[1].Type)

	assert.Equal(t, "switch", *result.Config.Input.Inputs[2].Type)
	assert.Nil(t, result.Config.Input.Inputs[2].Name)

	assert.Equal(t, "device-type", result.Sources["input.type"])
	assert.Equal(t, "device", result.Sources["input.inputs.0.name"])
	assert.Equal(t, "device-type", result.Sources["input.inputs.0.type"])
}

func TestEngine_Merge(t *testing.T) {
	engine := Engine{}

	layers := []ConfigLayer{
		{
			Name: "global",
			Config: &DeviceConfiguration{
				MQTT: &MQTTConfiguration{
					Server: StringPtr("broker.local"),
				},
			},
		},
	}

	result, err := engine.Merge(layers)
	require.NoError(t, err)
	assert.NotNil(t, result.Config)
	assert.Equal(t, "broker.local", *result.Config.MQTT.Server)
}

func TestGetFieldSource(t *testing.T) {
	sources := map[string]string{
		"mqtt.server": "global",
		"wifi.ssid":   "device",
	}

	source, err := GetFieldSource(sources, "mqtt.server")
	require.NoError(t, err)
	assert.Equal(t, "global", source)

	_, err = GetFieldSource(sources, "nonexistent")
	assert.Error(t, err)
}

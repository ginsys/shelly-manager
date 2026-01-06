package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

type applyTestClient struct {
	ip          string
	generation  int
	configErr   error
	setErr      error
	rebootErr   error
	testConnErr error
	configs     []map[string]interface{}
	currentRaw  json.RawMessage
}

func newApplyTestClient() *applyTestClient {
	return &applyTestClient{
		ip:         "192.168.1.100",
		generation: 1,
		configs:    []map[string]interface{}{},
		currentRaw: json.RawMessage(`{"name":"test"}`),
	}
}

func (m *applyTestClient) SetConfig(ctx context.Context, config map[string]interface{}) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.configs = append(m.configs, config)
	return nil
}

func (m *applyTestClient) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	if m.configErr != nil {
		return nil, m.configErr
	}
	return &shelly.DeviceConfig{
		Raw: m.currentRaw,
	}, nil
}

func (m *applyTestClient) GetInfo(ctx context.Context) (*shelly.DeviceInfo, error) {
	return &shelly.DeviceInfo{
		ID:         "test-device",
		Generation: m.generation,
	}, nil
}

func (m *applyTestClient) Reboot(ctx context.Context) error {
	return m.rebootErr
}

func (m *applyTestClient) TestConnection(ctx context.Context) error {
	return m.testConnErr
}

func (m *applyTestClient) GetGeneration() int {
	return m.generation
}

func (m *applyTestClient) GetIP() string {
	return m.ip
}

func TestNewConfigApplier(t *testing.T) {
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)
	require.NotNil(t, applier)
	assert.NotNil(t, applier.converter)
	assert.NotNil(t, applier.comparator)
	assert.NotNil(t, applier.logger)
}

func TestApplyConfig_EmptyConfig(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{}
	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.FailedCount)
}

func TestApplyConfig_MQTTSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("broker.local"),
			Port:   IntPtr(1883),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.FailedCount)
	assert.Greater(t, result.AppliedCount, 0)

	foundMQTT := false
	for _, cfg := range client.configs {
		if _, ok := cfg["mqtt"]; ok {
			foundMQTT = true
		}
	}
	assert.True(t, foundMQTT, "MQTT settings should have been sent")
}

func TestApplyConfig_WiFiSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			SSID:     StringPtr("MyNetwork"),
			Password: StringPtr("secret"),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)

	foundWiFi := false
	for _, cfg := range client.configs {
		if _, ok := cfg["wifi_sta"]; ok {
			foundWiFi = true
		}
	}
	assert.True(t, foundWiFi, "WiFi settings should have been sent")
}

func TestApplyConfig_RelaySettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("Kitchen"), DefaultState: StringPtr("off")},
			},
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestApplyConfig_SetConfigError(t *testing.T) {
	client := newApplyTestClient()
	client.setErr = errors.New("device rejected settings")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Greater(t, result.FailedCount, 0)
	assert.NotEmpty(t, result.Failures)
}

func TestApplyConfig_RequiresReboot(t *testing.T) {
	client := newApplyTestClient()
	client.currentRaw = json.RawMessage(`{"wifi_sta":{"ssid":"OldNetwork"}}`)

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			SSID: StringPtr("NewNetwork"),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.RequiresReboot)
	assert.Contains(t, result.Warnings[0], "reboot")
}

func TestApplyConfig_AuthChangeRequiresReboot(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Auth: &AuthConfiguration{
			Enable: BoolPtr(true),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.RequiresReboot)
}

func TestApplyConfig_MeasuresDuration(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{}
	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestApplyConfig_GetConfigError(t *testing.T) {
	client := newApplyTestClient()
	client.configErr = errors.New("cannot get config")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.RequiresReboot)
}

func TestApplyConfig_CloudSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Cloud: &CloudConfiguration{
			Enable: BoolPtr(false),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestApplyConfig_CoIoTSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		CoIoT: &CoIoTConfiguration{
			Enable:       BoolPtr(true),
			UpdatePeriod: IntPtr(60),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestApplyConfig_SystemSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{
				Name:    StringPtr("MyDevice"),
				EcoMode: BoolPtr(true),
			},
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestApplyConfig_LocationSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		Location: &LocationConfiguration{
			Timezone:  StringPtr("Europe/Brussels"),
			Latitude:  Float64Ptr(50.85),
			Longitude: Float64Ptr(4.35),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestApplyConfig_LEDSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		LED: &LEDConfig{
			PowerIndication:   BoolPtr(false),
			NetworkIndication: BoolPtr(true),
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestRebootAndWait_Success(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	err := applier.RebootAndWait(context.Background(), client, 5*time.Second)
	assert.NoError(t, err)
}

func TestRebootAndWait_RebootError(t *testing.T) {
	client := newApplyTestClient()
	client.rebootErr = errors.New("reboot failed")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	err := applier.RebootAndWait(context.Background(), client, 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reboot")
}

func TestRebootAndWait_Timeout(t *testing.T) {
	client := newApplyTestClient()
	client.testConnErr = errors.New("device offline")

	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	err := applier.RebootAndWait(context.Background(), client, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "did not come back")
}

func TestApplyConfig_MultipleSettings(t *testing.T) {
	client := newApplyTestClient()
	converter := NewGen1Converter(nil)
	applier := NewConfigApplier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("broker.local"),
		},
		Cloud: &CloudConfiguration{
			Enable: BoolPtr(false),
		},
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{
				Name: StringPtr("TestDevice"),
			},
		},
	}

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, result.AppliedCount, 1)
}

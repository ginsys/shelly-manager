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

type verifyTestClient struct {
	ip          string
	generation  int
	configErr   error
	setErr      error
	rebootErr   error
	testConnErr error
	configs     []map[string]interface{}
	currentRaw  json.RawMessage
}

func newVerifyTestClient() *verifyTestClient {
	return &verifyTestClient{
		ip:         "192.168.1.100",
		generation: 1,
		configs:    []map[string]interface{}{},
		currentRaw: json.RawMessage(`{}`),
	}
}

func (m *verifyTestClient) SetConfig(ctx context.Context, config map[string]interface{}) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.configs = append(m.configs, config)
	return nil
}

func (m *verifyTestClient) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	if m.configErr != nil {
		return nil, m.configErr
	}
	return &shelly.DeviceConfig{
		Raw: m.currentRaw,
	}, nil
}

func (m *verifyTestClient) GetInfo(ctx context.Context) (*shelly.DeviceInfo, error) {
	return &shelly.DeviceInfo{
		ID:         "test-device",
		Generation: m.generation,
	}, nil
}

func (m *verifyTestClient) Reboot(ctx context.Context) error {
	return m.rebootErr
}

func (m *verifyTestClient) TestConnection(ctx context.Context) error {
	return m.testConnErr
}

func (m *verifyTestClient) GetGeneration() int {
	return m.generation
}

func (m *verifyTestClient) GetIP() string {
	return m.ip
}

func TestNewConfigVerifier(t *testing.T) {
	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)
	require.NotNil(t, verifier)
	assert.NotNil(t, verifier.converter)
	assert.NotNil(t, verifier.comparator)
	assert.NotNil(t, verifier.applier)
	assert.NotNil(t, verifier.logger)
}

func TestVerifyConfig_Match(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{
		"mqtt": {"enable": true, "server": "broker.local:1883"}
	}`)

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	desired := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("broker.local"),
			Port:   IntPtr(1883),
		},
	}

	result, err := verifier.VerifyConfig(context.Background(), client, desired, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
	assert.NotNil(t, result.Imported)
	assert.NotNil(t, result.Desired)
}

func TestVerifyConfig_Mismatch(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{
		"mqtt": {"enable": true, "server": "device.broker:1883"}
	}`)

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	desired := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("expected.broker"),
			Port:   IntPtr(1883),
		},
	}

	result, err := verifier.VerifyConfig(context.Background(), client, desired, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Match)
	assert.NotEmpty(t, result.Differences)
}

func TestVerifyConfig_GetConfigError(t *testing.T) {
	client := newVerifyTestClient()
	client.configErr = errors.New("device unreachable")

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	desired := &DeviceConfiguration{}

	_, err := verifier.VerifyConfig(context.Background(), client, desired, "SHPLG-S")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get device config")
}

func TestApplyAndVerify_Success(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{
		"name": "TestDevice"
	}`)

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	config := &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{
				Name: StringPtr("TestDevice"),
			},
		},
	}

	result, err := verifier.ApplyAndVerify(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.NotNil(t, result.ApplyResult)
	assert.NotNil(t, result.VerifyResult)
	assert.True(t, result.ApplyResult.Success)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestApplyAndVerify_ApplyFails(t *testing.T) {
	client := newVerifyTestClient()
	client.setErr = errors.New("device rejected settings")

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	config := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
		},
	}

	result, err := verifier.ApplyAndVerify(context.Background(), client, config, "SHPLG-S")

	require.NoError(t, err)
	assert.NotNil(t, result.ApplyResult)
	assert.False(t, result.ApplyResult.Success)
	assert.False(t, result.ConfigApplied)
}

func TestVerifyOnly(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{"name": "Test"}`)

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	desired := &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{
				Name: StringPtr("Test"),
			},
		},
	}

	result, err := verifier.VerifyOnly(context.Background(), client, desired, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.Match)
}

func TestImportConfig(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{
		"mqtt": {"enable": true, "server": "broker:1883"},
		"name": "ImportedDevice"
	}`)

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	config, raw, err := verifier.ImportConfig(context.Background(), client, "SHPLG-S")

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, raw)
	assert.NotNil(t, config.MQTT)
	assert.Equal(t, "broker", *config.MQTT.Server)
}

func TestImportConfig_Error(t *testing.T) {
	client := newVerifyTestClient()
	client.configErr = errors.New("device error")

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	_, _, err := verifier.ImportConfig(context.Background(), client, "SHPLG-S")
	assert.Error(t, err)
}

func TestGetDiffReport_Match(t *testing.T) {
	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	result := &VerifyResult{
		Match:       true,
		Differences: []ConfigDifference{},
	}

	report := verifier.GetDiffReport(result)
	assert.Contains(t, report, "no differences found")
}

func TestGetDiffReport_WithDifferences(t *testing.T) {
	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	result := &VerifyResult{
		Match: false,
		Differences: []ConfigDifference{
			{
				Path:        "mqtt.server",
				Expected:    "expected.broker",
				Actual:      "actual.broker",
				Severity:    "critical",
				Description: "MQTT server mismatch",
			},
		},
	}

	report := verifier.GetDiffReport(result)
	assert.Contains(t, report, "1 difference")
	assert.Contains(t, report, "mqtt.server")
	assert.Contains(t, report, "expected.broker")
	assert.Contains(t, report, "actual.broker")
}

func TestGetApplyVerifyReport(t *testing.T) {
	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	result := &ApplyVerifyResult{
		ApplyResult: &ApplyResult{
			Success:        true,
			SettingsCount:  5,
			AppliedCount:   5,
			FailedCount:    0,
			RequiresReboot: false,
		},
		VerifyResult: &VerifyResult{
			Match:       true,
			Differences: []ConfigDifference{},
		},
		ConfigApplied: true,
		Duration:      100 * time.Millisecond,
	}

	report := verifier.GetApplyVerifyReport(result)
	assert.Contains(t, report, "Apply Phase")
	assert.Contains(t, report, "Verify Phase")
	assert.Contains(t, report, "config_applied=true")
}

func TestGetApplyVerifyReport_WithFailures(t *testing.T) {
	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	result := &ApplyVerifyResult{
		ApplyResult: &ApplyResult{
			Success:       false,
			SettingsCount: 3,
			AppliedCount:  1,
			FailedCount:   2,
			Failures: []ApplyFailure{
				{Path: "mqtt.server", Error: "connection refused"},
			},
		},
		ConfigApplied: false,
		Duration:      50 * time.Millisecond,
	}

	report := verifier.GetApplyVerifyReport(result)
	assert.Contains(t, report, "Failures")
	assert.Contains(t, report, "mqtt.server")
	assert.Contains(t, report, "connection refused")
	assert.Contains(t, report, "config_applied=false")
}

func TestVerifyConfig_Duration(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{}`)

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	desired := &DeviceConfiguration{}

	result, err := verifier.VerifyConfig(context.Background(), client, desired, "SHPLG-S")

	require.NoError(t, err)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestVerifyConfig_MultipleFields(t *testing.T) {
	client := newVerifyTestClient()
	client.currentRaw = json.RawMessage(`{
		"mqtt": {"enable": false, "server": "other:1883"},
		"cloud": {"enabled": true}
	}`)

	converter := NewGen1Converter(nil)
	verifier := NewConfigVerifier(converter, nil)

	desired := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Enable: BoolPtr(true),
			Server: StringPtr("expected"),
		},
		Cloud: &CloudConfiguration{
			Enable: BoolPtr(false),
		},
	}

	result, err := verifier.VerifyConfig(context.Background(), client, desired, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Match)
	assert.GreaterOrEqual(t, len(result.Differences), 2)
}

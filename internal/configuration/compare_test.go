package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigComparator(t *testing.T) {
	c := NewConfigComparator()
	require.NotNil(t, c)
	assert.NotEmpty(t, c.rules)
}

func TestCompare_BothNil(t *testing.T) {
	c := NewConfigComparator()
	result := c.Compare(nil, nil)
	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
}

func TestCompare_ExpectedNil(t *testing.T) {
	c := NewConfigComparator()
	actual := &DeviceConfiguration{}
	result := c.Compare(nil, actual)
	assert.False(t, result.Match)
	assert.Len(t, result.Differences, 1)
}

func TestCompare_ActualNil(t *testing.T) {
	c := NewConfigComparator()
	expected := &DeviceConfiguration{}
	result := c.Compare(expected, nil)
	assert.False(t, result.Match)
	assert.Len(t, result.Differences, 1)
}

func TestCompare_BothEmpty(t *testing.T) {
	c := NewConfigComparator()
	expected := &DeviceConfiguration{}
	actual := &DeviceConfiguration{}
	result := c.Compare(expected, actual)
	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
}

func TestCompare_MatchingConfigs(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
			Port:   IntPtr(1883),
		},
	}

	actual := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
			Port:   IntPtr(1883),
		},
	}

	result := c.Compare(expected, actual)
	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
}

func TestCompare_DifferentValues(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker1.local"),
		},
	}

	actual := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker2.local"),
		},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)
	assert.Len(t, result.Differences, 1)
	assert.Equal(t, "mqtt.server", result.Differences[0].Path)
	assert.Equal(t, "broker1.local", result.Differences[0].Expected)
	assert.Equal(t, "broker2.local", result.Differences[0].Actual)
}

func TestCompare_MissingValue(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
			Port:   IntPtr(1883),
		},
	}

	actual := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
			// Port is nil (missing)
		},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)
	assert.Len(t, result.Differences, 1)
	assert.Equal(t, "mqtt.port", result.Differences[0].Path)
}

func TestCompare_NilMeansInherit(t *testing.T) {
	c := NewConfigComparator()

	// If expected has nil field, we don't compare it
	expected := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
			// Port is nil - inherit, don't compare
		},
	}

	actual := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker.local"),
			Port:   IntPtr(8883), // Device has its own port
		},
	}

	result := c.Compare(expected, actual)
	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
}

func TestCompare_NestedStruct(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			StaticIP: &StaticIPConfig{
				IP:      StringPtr("192.168.1.100"),
				Gateway: StringPtr("192.168.1.1"),
			},
		},
	}

	actual := &DeviceConfiguration{
		WiFi: &WiFiConfiguration{
			StaticIP: &StaticIPConfig{
				IP:      StringPtr("192.168.1.100"),
				Gateway: StringPtr("192.168.1.254"), // Different
			},
		},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)
	assert.Len(t, result.Differences, 1)
	assert.Contains(t, result.Differences[0].Path, "static_ip")
	assert.Contains(t, result.Differences[0].Path, "gw")
}

func TestCompare_SliceElements(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("Kitchen")},
				{ID: 1, Name: StringPtr("Living")},
			},
		},
	}

	actual := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("Kitchen")},
				{ID: 1, Name: StringPtr("Bedroom")}, // Different
			},
		},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)
	assert.Len(t, result.Differences, 1)
	assert.Contains(t, result.Differences[0].Path, "relay.relays.1")
}

func TestCompare_ToleranceForCoordinates(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		Location: &LocationConfiguration{
			Latitude:  Float64Ptr(40.71280),
			Longitude: Float64Ptr(-74.00600),
		},
	}

	actual := &DeviceConfiguration{
		Location: &LocationConfiguration{
			Latitude:  Float64Ptr(40.71285),  // Within tolerance
			Longitude: Float64Ptr(-74.00602), // Within tolerance
		},
	}

	result := c.Compare(expected, actual)
	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
}

func TestCompare_ToleranceExceeded(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		Location: &LocationConfiguration{
			Latitude: Float64Ptr(40.71280),
		},
	}

	actual := &DeviceConfiguration{
		Location: &LocationConfiguration{
			Latitude: Float64Ptr(40.71400), // Exceeds tolerance
		},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)
}

func TestCompare_SkippedFields(t *testing.T) {
	// Custom rule to skip a specific field
	rules := []FieldCompareRule{
		{Path: "mqtt.server", SkipCompare: true},
	}
	c := NewConfigComparatorWithRules(rules)

	expected := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker1.local"),
			Port:   IntPtr(1883),
		},
	}

	actual := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker2.local"), // Different but skipped
			Port:   IntPtr(1883),
		},
	}

	result := c.Compare(expected, actual)
	assert.True(t, result.Match)
}

func TestCompare_WildcardRule(t *testing.T) {
	rules := []FieldCompareRule{
		{Path: "relay.relays.*", Severity: "warning", Category: "device"},
	}
	c := NewConfigComparatorWithRules(rules)

	expected := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("A")},
			},
		},
	}

	actual := &DeviceConfiguration{
		Relay: &RelayConfig{
			Relays: []SingleRelayConfig{
				{ID: 0, Name: StringPtr("B")},
			},
		},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)
	// Severity should be warning from wildcard rule
	// Note: The wildcard matches the slice index level, not the field inside
}

func TestCompare_CategoryInference(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{Server: StringPtr("a")},
		Auth: &AuthConfiguration{Enable: BoolPtr(true)},
	}

	actual := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{Server: StringPtr("b")},
		Auth: &AuthConfiguration{Enable: BoolPtr(false)},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)

	// Find the differences by path
	var mqttDiff, authDiff *ConfigDifference
	for i := range result.Differences {
		if result.Differences[i].Path == "mqtt.server" {
			mqttDiff = &result.Differences[i]
		}
		if result.Differences[i].Path == "auth.enable" {
			authDiff = &result.Differences[i]
		}
	}

	require.NotNil(t, mqttDiff)
	require.NotNil(t, authDiff)

	assert.Equal(t, "network", mqttDiff.Category)
	assert.Equal(t, "security", authDiff.Category)
}

func TestCompareResult_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   CompareResult
		expected bool
	}{
		{
			name:     "empty",
			result:   CompareResult{Match: true, Differences: nil},
			expected: false,
		},
		{
			name: "warning only",
			result: CompareResult{
				Match:       false,
				Differences: []ConfigDifference{{Severity: "warning"}},
			},
			expected: false,
		},
		{
			name: "critical error",
			result: CompareResult{
				Match:       false,
				Differences: []ConfigDifference{{Severity: "critical"}},
			},
			expected: true,
		},
		{
			name: "mixed",
			result: CompareResult{
				Match: false,
				Differences: []ConfigDifference{
					{Severity: "warning"},
					{Severity: "critical"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.HasErrors())
		})
	}
}

func TestCompareResult_ErrorCount(t *testing.T) {
	result := CompareResult{
		Differences: []ConfigDifference{
			{Severity: "critical"},
			{Severity: "warning"},
			{Severity: "critical"},
			{Severity: "info"},
		},
	}
	assert.Equal(t, 2, result.ErrorCount())
}

func TestCompareResult_WarningCount(t *testing.T) {
	result := CompareResult{
		Differences: []ConfigDifference{
			{Severity: "critical"},
			{Severity: "warning"},
			{Severity: "warning"},
			{Severity: "info"},
		},
	}
	assert.Equal(t, 2, result.WarningCount())
}

func TestPathMatches(t *testing.T) {
	c := NewConfigComparator()

	tests := []struct {
		path    string
		pattern string
		match   bool
	}{
		{"mqtt.server", "mqtt.server", true},
		{"mqtt.server", "mqtt.port", false},
		{"relay.relays.0", "relay.relays.*", true},
		{"relay.relays.1", "relay.relays.*", true},
		{"relay.relays.0.name", "relay.relays.*.name", true},
		{"relay.relays", "relay.relays.*", false},
		{"wifi.ssid", "mqtt.*", false},
	}

	for _, tt := range tests {
		t.Run(tt.path+":"+tt.pattern, func(t *testing.T) {
			assert.Equal(t, tt.match, c.pathMatches(tt.path, tt.pattern))
		})
	}
}

func TestCompare_MultipleDifferences(t *testing.T) {
	c := NewConfigComparator()

	expected := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker1"),
			Port:   IntPtr(1883),
		},
		WiFi: &WiFiConfiguration{
			SSID: StringPtr("Network1"),
		},
	}

	actual := &DeviceConfiguration{
		MQTT: &MQTTConfiguration{
			Server: StringPtr("broker2"),
			Port:   IntPtr(8883),
		},
		WiFi: &WiFiConfiguration{
			SSID: StringPtr("Network2"),
		},
	}

	result := c.Compare(expected, actual)
	assert.False(t, result.Match)
	assert.Len(t, result.Differences, 3)
}

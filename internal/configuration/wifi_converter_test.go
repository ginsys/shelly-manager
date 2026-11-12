package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The device reports the static netmask as `mask` (GET /settings, wifi_sta) and takes it as
// `netmask`. The converter models the GET document, so it reads `mask`.
func TestGen1Converter_ReadsTheDeviceNetmaskKey(t *testing.T) {
	raw := []byte(`{"wifi_sta":{"enabled":true,"ssid":"office","ipv4_method":"static",` +
		`"ip":"192.168.1.50","gw":"192.168.1.1","mask":"255.255.255.0","dns":"8.8.8.8"}}`)

	config, err := NewGen1Converter(nil).FromAPIConfig(raw, "SHSW-1")

	require.NoError(t, err)
	require.NotNil(t, config.WiFi)
	require.NotNil(t, config.WiFi.StaticIP)
	assert.Equal(t, "255.255.255.0", StringVal(config.WiFi.StaticIP.Netmask, ""))
}

func TestGen1Converter_StillReadsTheOlderNetmaskKey(t *testing.T) {
	raw := []byte(`{"wifi_sta":{"enabled":true,"ssid":"office","ipv4_method":"static",` +
		`"ip":"192.168.1.50","gw":"192.168.1.1","netmask":"255.255.0.0","dns":"8.8.8.8"}}`)

	config, err := NewGen1Converter(nil).FromAPIConfig(raw, "SHSW-1")

	require.NoError(t, err)
	require.NotNil(t, config.WiFi.StaticIP)
	assert.Equal(t, "255.255.0.0", StringVal(config.WiFi.StaticIP.Netmask, ""))
}

func TestGen1Converter_TheDeviceNetmaskKeyWinsOverTheOlderOne(t *testing.T) {
	raw := []byte(`{"wifi_sta":{"ip":"192.168.1.50","mask":"255.255.255.0","netmask":"255.255.0.0"}}`)

	config, err := NewGen1Converter(nil).FromAPIConfig(raw, "SHSW-1")

	require.NoError(t, err)
	assert.Equal(t, "255.255.255.0", StringVal(config.WiFi.StaticIP.Netmask, ""))
}

func TestGen1Converter_ExportsTheNetmaskUnderTheDeviceKey(t *testing.T) {
	desired := &DeviceConfiguration{WiFi: &WiFiConfiguration{
		StaticIP: &StaticIPConfig{IP: StringPtr("192.168.1.50"), Netmask: StringPtr("255.255.255.0"), Gateway: StringPtr("192.168.1.1")},
	}}

	raw, err := NewGen1Converter(nil).ToAPIConfig(desired, "SHSW-1")
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	sta := got["wifi_sta"].(map[string]interface{})
	assert.Equal(t, "255.255.255.0", sta["mask"])
	assert.NotContains(t, sta, "netmask")
}

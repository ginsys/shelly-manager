package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// networkLayer reads a layer from JSON, the way a template or an override reaches the merge.
func networkLayer(t *testing.T, name, body string) ConfigLayer {
	t.Helper()
	var cfg DeviceConfiguration
	require.NoError(t, json.Unmarshal([]byte(body), &cfg))
	return ConfigLayer{Name: name, Config: &cfg}
}

// mergedNetwork returns the Network part of a merge as a JSON map, so the test reads the values
// whatever Go type holds them.
func mergedNetwork(t *testing.T, layers ...ConfigLayer) (map[string]any, map[string]string) {
	t.Helper()
	result, err := MergeConfigurations(layers)
	require.NoError(t, err)
	require.NotNil(t, result.Config.Network)
	raw, err := json.Marshal(result.Config.Network)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out, result.Sources
}

func TestMergeConfigurations_KeepsEveryNetworkField(t *testing.T) {
	template := networkLayer(t, "template", `{"network":{
		"eth":{"enable":true,"ipv4mode":"static","ip":{"ip":"10.0.0.5","netmask":"255.255.255.0"}},
		"wifi":{
			"sta":{"enable":true,"ssid":"home","pass":"sta-secret","is_open":false,"ipv4mode":"dhcp","nameserver":"10.0.0.1"},
			"ap":{"enable":true,"ssid":"setup","pass":"ap-secret-1","is_open":true,"max_sta":4,"range_start":"192.168.33.2","range_end":"192.168.33.9"}}}}`)
	override := networkLayer(t, "device", `{"network":{
		"eth":{"enable":false},
		"wifi":{"sta":{"ssid":"office","enable":false},"ap":{"max_sta":2,"is_open":false}}}}`)

	got, sources := mergedNetwork(t, template, override)

	assert.Equal(t, map[string]any{
		"eth": map[string]any{
			"enable": false, "ipv4mode": "static",
			"ip": map[string]any{"ip": "10.0.0.5", "netmask": "255.255.255.0"},
		},
		"wifi": map[string]any{
			"sta": map[string]any{
				"enable": false, "ssid": "office", "pass": "sta-secret", "is_open": false,
				"ipv4mode": "dhcp", "nameserver": "10.0.0.1",
			},
			"ap": map[string]any{
				"enable": true, "ssid": "setup", "pass": "ap-secret-1", "is_open": false,
				"max_sta": float64(2), "range_start": "192.168.33.2", "range_end": "192.168.33.9",
			},
		},
	}, got)

	assert.Equal(t, "device", sources["network.eth.enable"])
	assert.Equal(t, "template", sources["network.eth.ipv4mode"])
	assert.Equal(t, "template", sources["network.wifi.sta.pass"])
	assert.Equal(t, "device", sources["network.wifi.sta.ssid"])
	assert.Equal(t, "device", sources["network.wifi.sta.enable"])
	assert.Equal(t, "device", sources["network.wifi.ap.is_open"])
	assert.Equal(t, "template", sources["network.wifi.ap.range_end"])
}

func TestMergeConfigurations_NetworkEmptyStringOverridesALowerLayer(t *testing.T) {
	template := networkLayer(t, "template", `{"network":{"wifi":{"sta":{"ssid":"home","pass":"sta-secret"}}}}`)
	override := networkLayer(t, "device", `{"network":{"wifi":{"sta":{"pass":""}}}}`)

	got, sources := mergedNetwork(t, template, override)

	sta := got["wifi"].(map[string]any)["sta"].(map[string]any)
	assert.Equal(t, "home", sta["ssid"])
	assert.Equal(t, "", sta["pass"])
	assert.Equal(t, "device", sources["network.wifi.sta.pass"])
}

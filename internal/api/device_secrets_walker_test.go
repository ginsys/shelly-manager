package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maskedPaths lists the dotted path of every string that holds the mask in a JSON text.
func maskedPaths(t *testing.T, text string) []string {
	t.Helper()
	var doc any
	require.NoError(t, json.Unmarshal([]byte(text), &doc), text)
	var out []string
	var walk func(v any, path string)
	walk = func(v any, path string) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				p := k
				if path != "" {
					p = path + "." + k
				}
				walk(child, p)
			}
		case []any:
			for i, child := range x {
				walk(child, fmt.Sprintf("%s[%d]", path, i))
			}
		case string:
			if x == secretMask {
				out = append(out, path)
			}
		}
	}
	walk(doc, "")
	sort.Strings(out)
	return out
}

// These samples are what a user stops seeing in clear. The lists are the keys the walker masks, and
// the pull request names them so a reviewer can read the effect of the key rule.
const gen1SettingsSample = `{
	"device": {"type": "SHSW-25", "mac": "68C63A123456", "hostname": "shelly1-68C63A123456"},
	"wifi_ap": {"enabled": false, "ssid": "shelly1-68C63A123456", "key": "ap-secret"},
	"wifi_sta": {"enabled": true, "ssid": "HomeNet", "key": "sta-secret", "ipv4_method": "dhcp"},
	"wifi_sta1": {"enabled": false, "ssid": "Other", "key": "sta1-secret"},
	"mqtt": {"enable": true, "server": "mqtt.example.net:1883", "user": "shelly", "pass": "mqtt-secret", "keep_alive": 60},
	"login": {"enabled": true, "unprotected": false, "username": "admin", "password": "login-secret"},
	"name": "Garage", "fw": "20230913-114010/v1.14.0-gcb84623", "max_power": 0,
	"relays": [{"name": "Relay 0", "default_state": "off", "auto_on": 0}]
}`

const gen2ConfigSample = `{
	"sys": {"device": {"name": "Porch", "mac": "AABBCCDDEEFF"}, "cfg_rev": 12},
	"wifi": {"sta": {"ssid": "HomeNet", "pass": "sta-secret", "enable": true}, "ap": {"ssid": "ShellyPlus", "pass": "ap-secret"}},
	"mqtt": {"enable": true, "server": "mqtt.example.net:1883", "user": "shelly", "pass": "mqtt-secret"},
	"cloud": {"enable": false, "server": "shelly-1-eu.shelly.cloud:6022/jrpc"}
}`

func TestMaskJSONText_NamesTheKeysItMasksInDeviceSamples(t *testing.T) {
	assert.Equal(t,
		[]string{"login.password", "mqtt.pass", "wifi_ap.key", "wifi_sta.key", "wifi_sta1.key"},
		maskedPaths(t, maskJSONText(gen1SettingsSample)), "Gen1 /settings")
	assert.Equal(t,
		[]string{"mqtt.pass", "wifi.ap.pass", "wifi.sta.pass"},
		maskedPaths(t, maskJSONText(gen2ConfigSample)), "Gen2 config")

	// What is not a secret is untouched.
	got := maskJSONText(gen1SettingsSample)
	for _, keep := range []string{"HomeNet", "shelly1-68C63A123456", `"keep_alive":60`, "mqtt.example.net:1883", "admin", "Relay 0"} {
		assert.Contains(t, got, keep)
	}
	for _, secret := range []string{"ap-secret", "sta-secret", "sta1-secret", "mqtt-secret", "login-secret"} {
		assert.NotContains(t, got, secret)
	}
}

func TestMaskJSONText_EmptySecretStaysEmptyAndMaskingIsStable(t *testing.T) {
	text := `{"mqtt":{"pass":"","server":"x"},"auth":{"pass":"p"}}`
	once := maskJSONText(text)
	assert.JSONEq(t, `{"mqtt":{"pass":"","server":"x"},"auth":{"pass":"`+secretMask+`"}}`, once)
	assert.Equal(t, once, maskJSONText(once), "masking a masked text changes nothing")
	assert.Equal(t, "", maskJSONText(""))
	assert.Equal(t, "{}", maskJSONText("{}"))
}

// A number under a secret key is a secret too, and a malformed text that names a secret key in any
// spelling is masked whole (review round 1 of the 436 account: findings on both).
func TestMaskJSONText_NumericSecretsAreMasked(t *testing.T) {
	got := maskJSONText(`{"auth_pass":12345678,"mqtt":{"pass":9007199254740993},"keep_alive":60}`)
	assert.NotContains(t, got, "12345678")
	assert.NotContains(t, got, "9007199254740993")
	assert.Equal(t, []string{"auth_pass", "mqtt.pass"}, maskedPaths(t, got))
	assert.Contains(t, got, `"keep_alive":60`)

	stored := `{"auth_pass":12345678}`
	back, err := restoreMaskedText(got, stored)
	assert.NoError(t, err)
	assert.Contains(t, back, `"auth_pass":12345678`, "a returned mask takes the stored number")
	assert.NotContains(t, back, secretMask)
}

func TestMaskJSONText_MalformedTextIsMaskedWhole(t *testing.T) {
	bs := string(rune(92)) // a backslash, built here so the escape case below is a real u-escape
	for name, text := range map[string]string{
		"truncated, secret key":    `{"password":"saved-secret"`,
		"truncated, no secret key": `{"name":"nothing here"`,
		"quote splits a key":       `{"pass\"word":"saved-secret"`,
		"u-escaped key":            `{"pa` + bs + `u0073sword":"saved-secret"`,
		"trailing data after JSON": `{"password":"saved-secret"} x`,
		"not JSON at all":          `password=saved-secret`,
	} {
		assert.Equal(t, secretMask, maskJSONText(text), name)
	}
}

func TestRestoreMaskedText_TakesTheStoredValueOrDropsTheMask(t *testing.T) {
	stored := `{"mqtt":{"pass":"kept","server":"old"},"list":[{"token":"t0"}]}`
	back := `{"mqtt":{"pass":"` + secretMask + `","server":"new"},"list":[{"token":"` + secretMask + `"}],"auth":{"pass":"` + secretMask + `"}}`
	got, err := restoreMaskedText(back, stored)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"mqtt":{"pass":"kept","server":"new"},"list":[{"token":"t0"}],"auth":{}}`, got)
	whole, err := restoreMaskedText(secretMask, stored)
	assert.NoError(t, err)
	assert.Equal(t, stored, whole, "the mask standing in for a whole text takes the stored text")
}

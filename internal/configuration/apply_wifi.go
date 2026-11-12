package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// WiFiSettingsClient is the Wi-Fi surface of a Gen1 client: POST /settings/sta for the station and
// POST /settings/ap for the access point. A client without it cannot apply Wi-Fi settings, and
// apply reports that as a failure instead of sending a nested wifi_sta document on POST /settings,
// which Gen1 ignores. The settings are read back through GET /settings (ShellyClient.GetConfig).
type WiFiSettingsClient interface {
	SetWiFiConfig(ctx context.Context, config map[string]interface{}) error
	SetAPConfig(ctx context.Context, config map[string]interface{}) error
}

var errNoWiFiClient = errors.New("client cannot write Wi-Fi settings")

// wifiUnconfirmedWarning is the state, neither success nor failure, of a station network change
// that the device accepted and then stopped answering after.
const wifiUnconfirmedWarning = "Applied, unconfirmed: the device may have changed network and has not answered since"

// gen1StationKeys maps the converted wifi_sta keys to the form parameters of POST /settings/sta.
// GET /settings says mask and gw, where the write form says netmask and gateway.
var gen1StationKeys = []struct{ api, form string }{
	{"enabled", "enabled"}, {"ssid", "ssid"}, {"key", "key"}, {"ipv4_method", "ipv4_method"},
	{"ip", "ip"}, {"mask", "netmask"}, {"gw", "gateway"}, {"dns", "dns"},
}

// gen1AccessPointKeys are the form parameters of POST /settings/ap. The device does not let its
// access point SSID be changed, so the ssid is refused before apply (unsupportedWiFi).
var gen1AccessPointKeys = []struct{ api, form string }{
	{"enabled", "enabled"}, {"key", "key"},
}

// usableWiFiSecret reports whether a key may be sent: an empty or redacted one never is.
func usableWiFiSecret(s string) bool {
	return s != "" && s != "[REDACTED]" && s != "***"
}

// gen1WiFiForms builds the two write forms from the converted configuration. It holds only keys the
// desired configuration set, and never a key that is empty or redacted. Either form is nil when
// there is nothing to write to that route.
func gen1WiFiForms(apiConfig map[string]interface{}) (sta, ap map[string]interface{}) {
	build := func(section string, keys []struct{ api, form string }) map[string]interface{} {
		doc, _ := apiConfig[section].(map[string]interface{})
		form := make(map[string]interface{})
		for _, key := range keys {
			value, present := doc[key.api]
			if !present {
				continue
			}
			switch v := value.(type) {
			case bool:
				// The reference sets the flag as 1.
				if v {
					value = 1
				} else {
					value = 0
				}
			case string:
				if key.api == "key" && !usableWiFiSecret(v) {
					continue
				}
			}
			form[key.form] = value
		}
		if len(form) == 0 {
			return nil
		}
		return form
	}
	return build("wifi_sta", gen1StationKeys), build("wifi_ap", gen1AccessPointKeys)
}

// liveWiFiSection returns one section of a GET /settings document, or nil.
func liveWiFiSection(raw json.RawMessage, section string) map[string]interface{} {
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	var out map[string]interface{}
	if json.Unmarshal(doc[section], &out) != nil {
		return nil
	}
	return out
}

// checkWiFiBeforeWrite refuses, before any section is written, a Wi-Fi change that could take the
// device off its network. The station settings are never written blind, so the device has to be
// readable first. A new network needs its key: the device keeps no key it was not sent, and one
// with no key leaves it unable to join. The same network without a key is a change of the other
// settings and stays allowed.
func checkWiFiBeforeWrite(live json.RawMessage, haveLive bool, readErr error, apiConfig map[string]interface{}) error {
	if err := wifiPrecheck(live, haveLive, readErr, apiConfig); err != nil {
		return &refusedBeforeWriteError{err}
	}
	return nil
}

// refusedBeforeWriteError marks an apply that stopped before the first write. The device still
// holds what it held, so the caller can leave config_applied as it was.
type refusedBeforeWriteError struct{ err error }

func (e *refusedBeforeWriteError) Error() string { return e.err.Error() }
func (e *refusedBeforeWriteError) Unwrap() error { return e.err }

func wifiPrecheck(live json.RawMessage, haveLive bool, readErr error, apiConfig map[string]interface{}) error {
	sta, ap := gen1WiFiForms(apiConfig)
	if len(sta) == 0 && len(ap) == 0 {
		return nil
	}
	if !haveLive {
		return fmt.Errorf("could not read the device's settings, nothing was written: %w", readErr)
	}
	ssid, ok := sta["ssid"].(string)
	if !ok {
		return nil
	}
	if _, hasKey := sta["key"]; hasKey {
		return nil
	}
	if current, _ := liveWiFiSection(live, "wifi_sta")["ssid"].(string); current == ssid {
		return nil
	}
	return fmt.Errorf("%w: wifi.ssid: a new network needs its password, and none that can be sent is set", ErrUnsupportedSettings)
}

// stationNetworkChanged reports whether the station write moves the device's network: its ssid,
// enabled flag, addressing method or static address differs from what the device reported before
// the write. Only then can a silent device mean it is on another network. The name server does not
// move the device, and the key is never read back.
func stationNetworkChanged(sta map[string]interface{}, live map[string]interface{}) bool {
	for _, key := range gen1StationKeys {
		if key.api == "key" || key.api == "dns" {
			continue
		}
		value, written := sta[key.form]
		if !written {
			continue
		}
		if !relayValuesEqual(value, live[key.api]) {
			return true
		}
	}
	return false
}

func (a *ConfigApplier) applyWiFiSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, live json.RawMessage, result *ApplyResult) error {
	sta, ap := gen1WiFiForms(apiConfig)
	if len(sta) == 0 && len(ap) == 0 {
		return nil
	}
	wifiClient, ok := client.(WiFiSettingsClient)
	if !ok {
		section := "wifi_sta"
		if len(sta) == 0 {
			section = "wifi_ap"
		}
		return failSection(result, section, errNoWiFiClient)
	}

	networkChanged := len(sta) > 0 && stationNetworkChanged(sta, liveWiFiSection(live, "wifi_sta"))

	var firstErr error
	written := map[string]map[string]interface{}{}
	for _, route := range []struct {
		section string
		form    map[string]interface{}
		write   func(context.Context, map[string]interface{}) error
	}{
		{"wifi_sta", sta, wifiClient.SetWiFiConfig},
		{"wifi_ap", ap, wifiClient.SetAPConfig},
	} {
		if len(route.form) == 0 {
			continue
		}
		if err := route.write(ctx, route.form); err != nil {
			// The caller logs the returned error, so the returned one is redacted too.
			secret, _ := route.form["key"].(string)
			err = redact(err, secret)
			failWiFi(result, route.section, route.form, err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}
		written[route.section] = route.form
	}
	if len(written) == 0 {
		return firstErr
	}

	after, err := client.GetConfig(ctx)
	if err != nil {
		if networkChanged && written["wifi_sta"] != nil && !deviceAnswered(err) {
			result.Unconfirmed = true
			result.Warnings = append(result.Warnings, wifiUnconfirmedWarning)
			return firstErr
		}
		for section, form := range written {
			failWiFi(result, section, form, "read-back failed: "+err.Error())
		}
		return firstError(firstErr, err)
	}
	for _, section := range []string{"wifi_sta", "wifi_ap"} {
		if form := written[section]; form != nil {
			compareWiFiReadBack(result, section, form, liveWiFiSection(after.Raw, section))
		}
	}
	return firstErr
}

// deviceAnswered reports whether a failed read came from a device that answered: an HTTP status,
// a rejected login or a body that is not JSON all prove it is on its network. A transport error
// (no route, refused, timeout) is no answer.
func deviceAnswered(err error) bool {
	var deviceErr *shelly.DeviceError
	return errors.As(err, &deviceErr) || shelly.IsAuthError(err)
}

// compareWiFiReadBack checks every key just written against what GET /settings reports. The key is
// never reported by the device, so it counts as applied without a read.
func compareWiFiReadBack(result *ApplyResult, section string, form, live map[string]interface{}) {
	keys := gen1StationKeys
	if section == "wifi_ap" {
		keys = gen1AccessPointKeys
	}
	for _, key := range keys {
		value, present := form[key.form]
		if !present {
			continue
		}
		if key.api == "key" || relayValuesEqual(value, live[key.api]) {
			result.AppliedCount++
			continue
		}
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  section + "." + key.form,
			Value: fmt.Sprintf("%v", value),
			Error: fmt.Sprintf("device reports %v after the write", live[key.api]),
		})
		result.FailedCount++
	}
}

// failWiFi records every key of one form as failed. The key's value is never shown.
func failWiFi(result *ApplyResult, section string, form map[string]interface{}, message string) {
	for _, key := range sortedKeys(form) {
		value := fmt.Sprintf("%v", form[key])
		if key == "key" {
			value = "(hidden)"
		}
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  section + "." + key,
			Value: value,
			Error: message,
		})
		result.FailedCount++
	}
}

// unsupportedWiFi returns the Wi-Fi paths of desired that apply cannot write. The access point
// SSID cannot be changed on a Gen1 device, the roam threshold and a separate access point key
// field have no Gen1 setting, and enabling the station disables the access point and the reverse,
// so both cannot be desired on.
func unsupportedWiFi(wifi *WiFiConfiguration) []string {
	if wifi == nil {
		return nil
	}
	var unsupported []string
	if wifi.RoamThreshold != nil {
		unsupported = append(unsupported, "wifi.roam_threshold")
	}
	if ap := wifi.AccessPoint; ap != nil {
		if ap.SSID != nil {
			unsupported = append(unsupported, "wifi.ap.ssid")
		}
		if ap.Key != nil {
			unsupported = append(unsupported, "wifi.ap.key")
		}
		if ap.Enable != nil && *ap.Enable && wifi.Enable != nil && *wifi.Enable {
			unsupported = append(unsupported, "wifi.ap.enable", "wifi.enable")
		}
	}
	return unsupported
}

// withoutWiFiPasswords returns desired as the verify result reports it: the Wi-Fi keys are
// write-only, so the result never echoes them. The caller's configuration is never changed.
func withoutWiFiPasswords(desired *DeviceConfiguration) *DeviceConfiguration {
	if desired == nil || desired.WiFi == nil {
		return desired
	}
	wifi := *desired.WiFi
	wifi.Password = nil
	if wifi.AccessPoint != nil {
		ap := *wifi.AccessPoint
		ap.Password = nil
		ap.Key = nil
		wifi.AccessPoint = &ap
	}
	shown := *desired
	shown.WiFi = &wifi
	return &shown
}

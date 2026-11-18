package configuration

import (
	"context"
	"encoding/json"
	"fmt"
)

// The Gen2 Wi-Fi station. WiFi.SetConfig takes {"config": {"sta": {...}}}. Only the station is
// written: the access point, roaming and the fallback network (sta1) are not, so a setting for
// them is refused before the device is touched, and so is switching the station off, because the
// manager then has no way back to the device.

// gen2WiFiPasswordUsable reports whether a password may be sent: an empty one and a mask is never one.
func gen2WiFiPasswordUsable(s string) bool {
	return usableWiFiSecret(s) && s != redactedSecret
}

// gen2WiFiToDoc returns the keys of the station that the configuration sets. The password is
// write-only, and the device wants it with every ssid it is given.
func gen2WiFiToDoc(w *WiFiConfiguration) map[string]interface{} {
	sta := map[string]interface{}{}
	if w.Enable != nil {
		sta["enable"] = *w.Enable
	}
	if w.SSID != nil {
		sta["ssid"] = *w.SSID
	}
	if w.Password != nil && gen2WiFiPasswordUsable(*w.Password) {
		sta["pass"] = *w.Password
	}
	if w.IPv4Mode != nil {
		sta["ipv4mode"] = *w.IPv4Mode
	}
	if s := w.StaticIP; s != nil {
		for key, value := range map[string]*string{"ip": s.IP, "netmask": s.Netmask, "gw": s.Gateway, "nameserver": s.Nameserver} {
			if value != nil {
				sta[key] = *value
			}
		}
	}
	return sta
}

// gen2WiFiFromDoc reads the station of a Shelly.GetConfig document. A null value is an unset
// setting, and the device returns no password.
func gen2WiFiFromDoc(section interface{}) *WiFiConfiguration {
	wifi, _ := section.(map[string]interface{})
	raw, ok := wifi["sta"].(map[string]interface{})
	if !ok {
		return nil
	}
	w := &WiFiConfiguration{}
	if enable, ok := raw["enable"].(bool); ok {
		w.Enable = BoolPtr(enable)
	}
	if ssid, ok := raw["ssid"].(string); ok && ssid != "" {
		w.SSID = StringPtr(ssid)
	}
	if mode, ok := raw["ipv4mode"].(string); ok && mode != "" {
		w.IPv4Mode = StringPtr(mode)
	}
	static := &StaticIPConfig{}
	for key, field := range map[string]**string{"ip": &static.IP, "netmask": &static.Netmask, "gw": &static.Gateway, "nameserver": &static.Nameserver} {
		if value, ok := raw[key].(string); ok && value != "" {
			*field = StringPtr(value)
		}
	}
	if *static != (StaticIPConfig{}) {
		w.StaticIP = static
	}
	if *w == (WiFiConfiguration{}) {
		return nil
	}
	return w
}

// unmappedWiFiSettings returns the paths of the Wi-Fi settings that the Gen2 apply does not write,
// and the rest of the section for the round trip (without the write-only password), or nil when
// nothing else is set.
func unmappedWiFiSettings(w *WiFiConfiguration) ([]string, *WiFiConfiguration) {
	if w == nil {
		return nil, nil
	}
	rest := *w
	var paths []string
	// The password is write-only: no read returns it, so the round trip cannot compare it, and a
	// section that holds only a password is a station write like any other.
	rest.Password = nil
	if rest.Enable != nil && !*rest.Enable {
		paths, rest.Enable = append(paths, "wifi.enable"), nil
	}
	if ap := rest.AccessPoint; ap != nil {
		for path, set := range map[string]bool{
			"wifi.ap.enable": ap.Enable != nil, "wifi.ap.ssid": ap.SSID != nil,
			"wifi.ap.pass": ap.Password != nil, "wifi.ap.key": ap.Key != nil,
		} {
			if set {
				paths = append(paths, path)
			}
		}
		rest.AccessPoint = nil
	}
	if rest.RoamThreshold != nil {
		paths, rest.RoamThreshold = append(paths, "wifi.roam_threshold"), nil
	}
	if rest == (WiFiConfiguration{}) {
		return paths, nil
	}
	return paths, &rest
}

// gen2WiFiStation returns the station keys of a converted document, or nil.
func gen2WiFiStation(apiConfig map[string]interface{}) map[string]interface{} {
	wifi, _ := apiConfig["wifi"].(map[string]interface{})
	sta, _ := wifi["sta"].(map[string]interface{})
	return sta
}

// refuseUnsafeWiFi stops an apply, before the first write, that would leave the device unable to
// join its network. The station is never written blind, so a desired ssid needs the device to be
// readable. A new network needs its password: the device keeps none it was not sent. The network
// the device is already on needs none, and its ssid is left out of the write, because the device
// wants a password with every ssid it is given.
func refuseUnsafeWiFi(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}) error {
	sta := gen2WiFiStation(apiConfig)
	ssid, ok := sta["ssid"].(string)
	if !ok {
		return nil
	}
	live, err := client.GetConfig(ctx)
	if err != nil {
		return &refusedBeforeWriteError{fmt.Errorf("could not read the device before writing the Wi-Fi station: %w", err)}
	}
	var doc map[string]interface{}
	if err = json.Unmarshal(live.Raw, &doc); err != nil {
		return &refusedBeforeWriteError{fmt.Errorf("could not read the device before writing the Wi-Fi station: %w", err)}
	}
	if _, hasPassword := sta["pass"]; hasPassword {
		return nil
	}
	if current, _ := gen2WiFiStation(doc)["ssid"].(string); current != ssid {
		return &refusedBeforeWriteError{fmt.Errorf("%w: wifi.ssid: a new network needs its password, and none that can be sent is set", ErrUnsupportedSettings)}
	}
	delete(sta, "ssid")
	return nil
}
